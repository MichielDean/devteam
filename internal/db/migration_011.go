package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 11,
		Name:    "stage_logs_table",
		Up:      migration011StageLogs,
	})
}

func migration011StageLogs(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS stage_logs (
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