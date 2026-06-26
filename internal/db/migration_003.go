package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 3,
		Name:    "feature_data_column",
		Up:      migration003FeatureData,
	})
}

// migration003FeatureData adds a feature_data column to store the full Feature
// struct as JSON. This replaces .devteam-state.yaml on disk — the rich nested
// data (PhaseStates with Artifacts/GateResult, Dependencies, Repos, PreparedRepos)
// lives here while scalar columns (status, current_phase, priority) stay for
// queryability.
func migration003FeatureData(tx *sql.Tx) error {
	// SQLite: add column with default empty (ALTER TABLE ... ADD COLUMN is
	// idempotent-safe here because we check if the column exists first).
	// PostgreSQL: same approach.
	var colExists int
	row := tx.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('features') WHERE name = 'feature_data'`)
	if err := row.Scan(&colExists); err != nil {
		// PostgreSQL doesn't have pragma_table_info — try a different check
		row = tx.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'features' AND column_name = 'feature_data'`)
		if err := row.Scan(&colExists); err != nil {
			return fmt.Errorf("checking feature_data column: %w", err)
		}
	}

	if colExists == 0 {
		_, err := tx.Exec(`ALTER TABLE features ADD COLUMN feature_data TEXT DEFAULT ''`)
		if err != nil {
			return fmt.Errorf("adding feature_data column: %w", err)
		}
	}

	return nil
}