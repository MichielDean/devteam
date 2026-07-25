package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RepoOperationConfigRow is a per-repo operation-phase configuration record.
// JSONB fields are json.RawMessage to preserve the operator's exact bytes
// (unknown keys, nested shapes, key ordering for human-readable diffs). The
// platform treats JSONB as opaque (C-A5, ADR-03) — it never unmarshals these
// fields except the API's one-time "is valid JSON" check. Empty JSONB
// serializes as {}, never null (DR-R6, Agent Failure Mode Awareness).
type RepoOperationConfigRow struct {
	RepoName         string          `json:"repo_name"`
	CiPlatform       string          `json:"ci_platform"`
	CdPlatform       string          `json:"cd_platform"`
	Environments     json.RawMessage `json:"environments"`
	Observability    json.RawMessage `json:"observability"`
	IncidentResponse json.RawMessage `json:"incident_response"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// SaveOperationConfig upserts a per-repo operation config row.
// Idempotent: re-saving the same row advances updated_at but does not
// duplicate (ON CONFLICT(repo_name) DO UPDATE — FR-STORE-02). created_at is
// set on insert and left unchanged on update (excluded.created_at re-writes
// it, but since we pass `now` for both, the row's created_at only differs
// from updated_at on the first insert when the table sets it via DEFAULT;
// here we pass now for both columns, so on insert both equal now, on update
// updated_at advances and created_at is overwritten with the same now —
// which is acceptable since the store is the only writer and the operator
// does not rely on created_at being immutable across upserts for this
// single-writer table).
func (db *DB) SaveOperationConfig(row RepoOperationConfigRow) error {
	now := time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO repo_operation_config
		   (repo_name, ci_platform, cd_platform, environments,
		    observability, incident_response, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(repo_name) DO UPDATE SET
		   ci_platform       = excluded.ci_platform,
		   cd_platform       = excluded.cd_platform,
		   environments      = excluded.environments,
		   observability     = excluded.observability,
		   incident_response = excluded.incident_response,
		   updated_at        = excluded.updated_at`,
		row.RepoName, row.CiPlatform, row.CdPlatform,
		ensureJSONB(row.Environments), ensureJSONB(row.Observability), ensureJSONB(row.IncidentResponse),
		now, now,
	)
	if err != nil {
		return fmt.Errorf("saving operation config for %s: %w", row.RepoName, err)
	}
	return nil
}

// GetOperationConfig returns the config for a single repo.
// If no row exists, returns a zero-value row (not an error) — C-A7 backward
// compat / FR-STORE-05. Empty JSONB columns are normalized to []byte("{}") so
// the caller never sees null (DR-R6).
func (db *DB) GetOperationConfig(repoName string) (RepoOperationConfigRow, error) {
	row := RepoOperationConfigRow{
		RepoName:         repoName,
		Environments:     []byte("{}"),
		Observability:    []byte("{}"),
		IncidentResponse: []byte("{}"),
	}
	err := db.QueryRow(
		`SELECT ci_platform, cd_platform, environments, observability,
		        incident_response, created_at, updated_at
		 FROM repo_operation_config WHERE repo_name = ?`,
		repoName,
	).Scan(&row.CiPlatform, &row.CdPlatform, &row.Environments,
		&row.Observability, &row.IncidentResponse,
		&row.CreatedAt, &row.UpdatedAt)
	if err == sql.ErrNoRows {
		return row, nil // zero-value, no error (FR-STORE-05)
	}
	if err != nil {
		return row, fmt.Errorf("getting operation config for %s: %w", repoName, err)
	}
	normalizeEmptyJSONB(&row.Environments)
	normalizeEmptyJSONB(&row.Observability)
	normalizeEmptyJSONB(&row.IncidentResponse)
	return row, nil
}

// GetAllOperationConfigs returns all configured repos ordered by repo_name.
// Empty table → non-nil, zero-length slice (FR-STORE-03, DR-R6 — never null).
func (db *DB) GetAllOperationConfigs() ([]RepoOperationConfigRow, error) {
	rows, err := db.Query(
		`SELECT repo_name, ci_platform, cd_platform, environments,
		        observability, incident_response, created_at, updated_at
		 FROM repo_operation_config ORDER BY repo_name ASC`)
	if err != nil {
		return nil, fmt.Errorf("getting all operation configs: %w", err)
	}
	defer rows.Close()

	out := []RepoOperationConfigRow{} // EMPTY, not nil (DR-R6)
	for rows.Next() {
		var r RepoOperationConfigRow
		if err := rows.Scan(&r.RepoName, &r.CiPlatform, &r.CdPlatform,
			&r.Environments, &r.Observability, &r.IncidentResponse,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning operation config: %w", err)
		}
		normalizeEmptyJSONB(&r.Environments)
		normalizeEmptyJSONB(&r.Observability)
		normalizeEmptyJSONB(&r.IncidentResponse)
		out = append(out, r)
	}
	return out, nil
}

// GetOperationConfigsByRepoNames returns configs for the named repos in a
// single query. Used by the dispatch bridge to avoid N+1 (NFR-PERF-02 — the
// bridge makes one GetFeatureRepos query + this one query = ≤2 total). Repos
// with no row are omitted from the result; the caller treats absence as "no
// config" (FR-BRIDGE-04).
func (db *DB) GetOperationConfigsByRepoNames(names []string) ([]RepoOperationConfigRow, error) {
	if len(names) == 0 {
		return []RepoOperationConfigRow{}, nil
	}
	placeholders := make([]string, len(names))
	args := make([]any, len(names))
	for i, n := range names {
		placeholders[i] = "?"
		args[i] = n
	}
	query := `SELECT repo_name, ci_platform, cd_platform, environments,
	                 observability, incident_response, created_at, updated_at
	          FROM repo_operation_config WHERE repo_name IN (` +
		strings.Join(placeholders, ", ") + `) ORDER BY repo_name ASC`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("getting operation configs by repo names: %w", err)
	}
	defer rows.Close()

	out := []RepoOperationConfigRow{}
	for rows.Next() {
		var r RepoOperationConfigRow
		if err := rows.Scan(&r.RepoName, &r.CiPlatform, &r.CdPlatform,
			&r.Environments, &r.Observability, &r.IncidentResponse,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning operation config: %w", err)
		}
		normalizeEmptyJSONB(&r.Environments)
		normalizeEmptyJSONB(&r.Observability)
		normalizeEmptyJSONB(&r.IncidentResponse)
		out = append(out, r)
	}
	return out, nil
}

// DeleteOperationConfig removes a repo's config row. Deleting a non-existent
// row is a no-op (not an error) — matches DeleteTeamKnowledge semantics.
func (db *DB) DeleteOperationConfig(repoName string) error {
	_, err := db.Exec(`DELETE FROM repo_operation_config WHERE repo_name = ?`, repoName)
	if err != nil {
		return fmt.Errorf("deleting operation config for %s: %w", repoName, err)
	}
	return nil
}

// ensureJSONB normalizes the incoming JSONB bytes for a save: nil/empty/null
// becomes {} so the DB never stores NULL and the API never returns null
// (DR-R6). Valid JSON passes through unchanged.
func ensureJSONB(b json.RawMessage) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("{}")
	}
	// Postgres JSONB does not accept the literal Go "null" as a column
	// default replacement cleanly across all driver versions; normalize
	// to {} for storage consistency.
	if string(b) == "null" {
		return json.RawMessage("{}")
	}
	return b
}

// normalizeEmptyJSONB replaces nil/empty/`null` json.RawMessage with {} so
// the caller never serializes null (DR-R6 / empty-not-null contract). This is
// the single point that enforces empty-not-null on every read path.
func normalizeEmptyJSONB(b *json.RawMessage) {
	if b == nil || len(*b) == 0 || string(*b) == "null" {
		*b = json.RawMessage("{}")
	}
}
