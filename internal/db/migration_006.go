package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 6,
		Name:    "feature_scope_depth_columns",
		Up:      migration006ScopeDepth,
	})
}

// migration006ScopeDepth adds scope/depth/test-strategy/autonomy columns to features.
func migration006ScopeDepth(tx *sql.Tx) error {
	columns := []struct {
		name string
		ddl  string
	}{
		{"scope", "TEXT NOT NULL DEFAULT 'feature'"},
		{"depth", "TEXT NOT NULL DEFAULT 'standard'"},
		{"test_strategy", "TEXT NOT NULL DEFAULT 'standard'"},
		{"autonomy_mode", "TEXT NOT NULL DEFAULT ''"},
	}

	for _, col := range columns {
		var colExists int
		row := tx.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`, "features", col.name)
		if err := row.Scan(&colExists); err != nil {
			return fmt.Errorf("checking column %s: %w", col.name, err)
		}
		if colExists == 0 {
			_, err := tx.Exec(fmt.Sprintf("ALTER TABLE features ADD COLUMN %s %s", col.name, col.ddl))
			if err != nil {
				return fmt.Errorf("adding column %s: %w", col.name, err)
			}
		}
	}
	return nil
}