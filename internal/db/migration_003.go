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
	// PostgreSQL: check if column exists via information_schema.
	var colExists int
	row := tx.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`, "features", "feature_data")
	if err := row.Scan(&colExists); err != nil {
		return fmt.Errorf("checking feature_data column: %w", err)
	}

	if colExists == 0 {
		_, err := tx.Exec(`ALTER TABLE features ADD COLUMN feature_data TEXT DEFAULT ''`)
		if err != nil {
			return fmt.Errorf("adding feature_data column: %w", err)
		}
	}

	return nil
}