package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 2,
		Name:    "spec_artifacts_table",
		Up:      migration002SpecArtifacts,
	})
}

// migration002SpecArtifacts creates a table for storing spec document content in the database.
// This replaces writing spec.md, plan.md, tasks.md, etc. to disk.
// Agents submit content via the API/CLI, and it's stored here.
func migration002SpecArtifacts(tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS spec_artifacts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE,
			UNIQUE(feature_id, artifact_type)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_spec_artifacts_feature ON spec_artifacts(feature_id, artifact_type)`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("executing statement: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}