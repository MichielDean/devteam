package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 19,
		Name:    "repo_registry_and_settings",
		Up:      migration019RepoRegistryAndSettings,
	})
}

// migration019RepoRegistryAndSettings creates the repo_registry and
// repo_settings tables (feature github-authorization-integration, U-04 + U-10).
//
// repo_registry: the canonical set of repos the GitHub App installation manages
// (FR-DISC-02). Includes the `managed` column added by the 2.7 amendment (F3
// resolution — the MANAGED/AVAILABLE-BUT-UNMANAGED split and `repo manage`
// transition require it). `managed=0` = discovered but not curated; `managed=1`
// = the operator has marked it managed via `devteam repo manage`.
//
// repo_settings: per-repo MVP field set (FR-SETTINGS-01, R-08 fixed set):
//   - default_branch (TEXT, default 'main' — FR-PR-02 main fallback, C-11)
//   - pr_draft_default (INTEGER 0/1, default 1)
//   - conflict_detection_enabled (INTEGER 0/1, default 1)
//   - provider (TEXT 'native'|'gh', default 'native' — FR-SETTINGS-05, ADR-17)
//
// Phase-2 fields (required_reviewers, labels, branch_protection, merge_strategy,
// status_checks) are explicitly NOT in this schema — writes are rejected at the
// CLI boundary with "not supported in MVP" (FR-SETTINGS-03, R-08).
//
// Migration version is 019 (architecture-review B1: 017 is the orphan
// `repos_registry` row already applied; feature migrations start at 018).
//
// Idempotence: CREATE TABLE IF NOT EXISTS (nfr-design-specs §8.3).
func migration019RepoRegistryAndSettings(tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS repo_registry (
			id              SERIAL PRIMARY KEY,
			owner           TEXT NOT NULL,
			name            TEXT NOT NULL,
			full_name       TEXT NOT NULL,
			default_branch  TEXT NOT NULL DEFAULT 'main',
			installation_id BIGINT NOT NULL,
			managed         INTEGER NOT NULL DEFAULT 0,
			discovered_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(owner, name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_repo_registry_installation ON repo_registry(installation_id)`,
		`CREATE INDEX IF NOT EXISTS idx_repo_registry_managed ON repo_registry(managed)`,

		`CREATE TABLE IF NOT EXISTS repo_settings (
			id                         SERIAL PRIMARY KEY,
			repo_registry_id           INTEGER NOT NULL UNIQUE REFERENCES repo_registry(id) ON DELETE CASCADE,
			default_branch             TEXT NOT NULL DEFAULT 'main',
			pr_draft_default           INTEGER NOT NULL DEFAULT 1,
			conflict_detection_enabled INTEGER NOT NULL DEFAULT 1,
			provider                   TEXT NOT NULL DEFAULT 'native',
			updated_at                 TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_repo_settings_repo ON repo_settings(repo_registry_id)`,
	}
	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("executing repo_registry/settings statement: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}