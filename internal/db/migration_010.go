package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 10,
		Name:    "stage_descriptions",
		Up:      migration010StageDescriptions,
	})
}

// migration010StageDescriptions adds a description column to stage_definitions
// for existing databases (migration 005 now includes the column in CREATE TABLE,
// but older databases that already ran migration 005 need an ALTER TABLE).
func migration010StageDescriptions(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE stage_definitions ADD COLUMN IF NOT EXISTS description TEXT DEFAULT ''`)
	if err != nil {
		return fmt.Errorf("adding description column: %w", err)
	}
	return nil
}