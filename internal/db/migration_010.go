package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 10,
		Name:    "stage_descriptions_and_logs",
		Up:      migration010StageDescriptionsAndLogs,
	})
}

func migration010StageDescriptionsAndLogs(tx *sql.Tx) error {
	// Add description column to stage_definitions if it doesn't exist
	_, err := tx.Exec(`ALTER TABLE stage_definitions ADD COLUMN IF NOT EXISTS description TEXT DEFAULT ''`)
	if err != nil {
		return fmt.Errorf("adding description column: %w", err)
	}

	// Create stage_logs table for persistent agent output per stage
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS stage_logs (
		id SERIAL PRIMARY KEY,
		feature_id TEXT NOT NULL,
		stage_id TEXT NOT NULL,
		agent_role TEXT NOT NULL,
		content TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(feature_id, stage_id)
	)`)
	if err != nil {
		return fmt.Errorf("creating stage_logs table: %w", err)
	}

	return nil
}