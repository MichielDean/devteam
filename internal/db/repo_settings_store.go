package db

import (
	"database/sql"
	"fmt"
	"time"
)

// RepoSettingsRow is one per-repo settings row (feature github-authorization-integration,
// U-10, FR-SETTINGS-01). MVP field set is fixed (R-08):
//   - default_branch (FR-PR-02, C-11 main fallback)
//   - pr_draft_default
//   - conflict_detection_enabled
//   - provider (native|gh, FR-SETTINGS-05, ADR-17)
type RepoSettingsRow struct {
	ID                        int64     `json:"id"`
	RepoRegistryID            int64     `json:"repo_registry_id"`
	DefaultBranch             string    `json:"default_branch"`
	PrDraftDefault            int       `json:"pr_draft_default"` // 0/1
	ConflictDetectionEnabled  int       `json:"conflict_detection_enabled"` // 0/1
	Provider                  string    `json:"provider"` // "native" | "gh"
	UpdatedAt                 time.Time `json:"updated_at"`
}

// MVPRepoSettingsFields is the fixed set of allowed `repo set` keys (R-08,
// FR-SETTINGS-01). Writes outside this set are rejected at the CLI boundary
// with "not supported in MVP" (FR-SETTINGS-03).
var MVPRepoSettingsFields = []string{
	"default_branch",
	"pr_draft_default",
	"conflict_detection_enabled",
	"provider",
}

// Phase2RepoSettingsFields are explicitly NOT supported in MVP. A `repo set`
// write of any of these fails with exit 2 and the scope-change pointer (R-08).
var Phase2RepoSettingsFields = []string{
	"required_reviewers",
	"labels",
	"branch_protection",
	"merge_strategy",
	"status_checks",
}

// GetRepoSettings reads the settings row for a repo_registry row. Returns
// (nil, nil) if no settings row exists — the caller applies defaults (main,
// draft=true, conflict_detection=true, provider=native) per FR-SETTINGS-01.
func (db *DB) GetRepoSettings(repoRegistryID int64) (*RepoSettingsRow, error) {
	var r RepoSettingsRow
	err := db.QueryRow(
		`SELECT id, repo_registry_id, default_branch, pr_draft_default, conflict_detection_enabled, provider, updated_at
		 FROM repo_settings WHERE repo_registry_id = ?`,
		repoRegistryID,
	).Scan(&r.ID, &r.RepoRegistryID, &r.DefaultBranch, &r.PrDraftDefault, &r.ConflictDetectionEnabled, &r.Provider, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting repo_settings for registry_id %d: %w", repoRegistryID, err)
	}
	return &r, nil
}

// UpsertRepoSettings inserts or updates a settings field. field must be in
// MVPRepoSettingsFields (the caller validates). value is the string form; this
// helper coerces to the column type. Returns the upserted row.
func (db *DB) UpsertRepoSettings(repoRegistryID int64, field, value string) (*RepoSettingsRow, error) {
	// Validate field is in MVP set (defense in depth — the CLI validates first).
	if !contains(MVPRepoSettingsFields, field) {
		return nil, fmt.Errorf("field %q is not in the MVP settings set; supported: %v", field, MVPRepoSettingsFields)
	}

	// Ensure a settings row exists (INSERT if missing), then UPDATE the field.
	// This is a single-statement upsert pattern via ON CONFLICT.
	now := time.Now().UTC()

	// Coerce value to the column type.
	var setClause string
	switch field {
	case "default_branch":
		setClause = "default_branch = EXCLUDED.default_branch"
	case "pr_draft_default":
		coerced, err := coerceBool(value, "pr_draft_default")
		if err != nil {
			return nil, err
		}
		value = coerced
		setClause = "pr_draft_default = EXCLUDED.pr_draft_default"
	case "conflict_detection_enabled":
		coerced, err := coerceBool(value, "conflict_detection_enabled")
		if err != nil {
			return nil, err
		}
		value = coerced
		setClause = "conflict_detection_enabled = EXCLUDED.conflict_detection_enabled"
	case "provider":
		if value != "native" && value != "gh" {
			return nil, fmt.Errorf("provider: must be 'native' or 'gh'; got %q", value)
		}
		setClause = "provider = EXCLUDED.provider"
	}

	// Build the INSERT with the field set, then ON CONFLICT update just that field.
	// We use a dynamic column list to keep the insert minimal.
	var insertCols, insertVals string
	switch field {
	case "default_branch":
		insertCols = "repo_registry_id, default_branch, pr_draft_default, conflict_detection_enabled, provider"
		insertVals = "?, ?, 1, 1, 'native'"
	case "pr_draft_default":
		insertCols = "repo_registry_id, default_branch, pr_draft_default, conflict_detection_enabled, provider"
		insertVals = "?, 'main', ?, 1, 'native'"
	case "conflict_detection_enabled":
		insertCols = "repo_registry_id, default_branch, pr_draft_default, conflict_detection_enabled, provider"
		insertVals = "?, 'main', 1, ?, 'native'"
	case "provider":
		insertCols = "repo_registry_id, default_branch, pr_draft_default, conflict_detection_enabled, provider"
		insertVals = "?, 'main', 1, 1, ?"
	}

	query := `INSERT INTO repo_settings (` + insertCols + `, updated_at) VALUES (` + insertVals + `, ?)
		ON CONFLICT (repo_registry_id) DO UPDATE SET ` + setClause + `, updated_at = EXCLUDED.updated_at`

	_, err := db.Exec(query, repoRegistryID, value, now)
	if err != nil {
		return nil, fmt.Errorf("upserting repo_settings (field=%s, value=%s): %w", field, value, err)
	}

	return db.GetRepoSettings(repoRegistryID)
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// coerceBool converts "true"/"false"/"1"/"0" to "1"/"0" for INTEGER columns.
func coerceBool(value, field string) (string, error) {
	switch value {
	case "true", "1":
		return "1", nil
	case "false", "0":
		return "0", nil
	default:
		return "", fmt.Errorf("%s: must be true|false|1|0; got %q", field, value)
	}
}