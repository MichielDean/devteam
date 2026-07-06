package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 12,
		Name:    "execution_mode",
		Up:      migration012ExecutionMode,
	})
}

func migration012ExecutionMode(tx *sql.Tx) error {
	// Check if the column already exists (PostgreSQL doesn't support IF NOT EXISTS on ADD COLUMN directly
	// for all versions, so use information_schema check like migration_006).
	var colExists int
	row := tx.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`, "features", "execution_mode")
	if err := row.Scan(&colExists); err != nil {
		return fmt.Errorf("checking execution_mode column: %w", err)
	}
	if colExists == 0 {
		_, err := tx.Exec(`ALTER TABLE features ADD COLUMN execution_mode TEXT DEFAULT ''`)
		if err != nil {
			return fmt.Errorf("adding execution_mode column: %w", err)
		}
	}
	return nil
}