package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 17,
		Name:    "repo_operation_config",
		Up:      migration017RepoOperationConfig,
	})
}

// migration017RepoOperationConfig creates the per-repo operation-phase
// configuration table. It is keyed by repo_name (the same identifier used in
// feature_repos and repos.yaml) but is NOT foreign-keyed to feature_repos —
// operation config is a permanent per-repo fact; feature_repos rows are
// transient per-feature worktree coords (C-D3). JSONB columns carry the typed
// payload so schema evolution is a shape change, not ALTER TABLE (C-D4).
// ci_platform / cd_platform are free-text labels, not an enum (C-D5).
func migration017RepoOperationConfig(tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS repo_operation_config (
			repo_name         TEXT PRIMARY KEY,
			ci_platform       TEXT DEFAULT '',
			cd_platform       TEXT DEFAULT '',
			environments       JSONB DEFAULT '{}',
			observability     JSONB DEFAULT '{}',
			incident_response JSONB DEFAULT '{}',
			created_at        TIMESTAMPTZ NOT NULL,
			updated_at        TIMESTAMPTZ NOT NULL
		)`,
	}
	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("executing statement: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}