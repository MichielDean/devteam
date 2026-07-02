package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 9,
		Name:    "tmux_sessions",
		Up:      migration009TmuxSessions,
	})
}

// migration009TmuxSessions creates the tmux_sessions table for persistent
// per-phase tmux session tracking. Each session is scoped to a feature+phase
// (or feature+construction-boltN for construction Bolts).
func migration009TmuxSessions(tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS tmux_sessions (
			id SERIAL PRIMARY KEY,
			feature_id TEXT NOT NULL,
			phase TEXT NOT NULL,
			bolt_number INTEGER NOT NULL DEFAULT 0,
			stage_id TEXT DEFAULT '',
			session_name TEXT NOT NULL UNIQUE,
			state TEXT NOT NULL DEFAULT 'created',
			context_dir TEXT NOT NULL,
			last_agent TEXT DEFAULT '',
			last_output_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tmux_sessions_feature ON tmux_sessions(feature_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tmux_sessions_state ON tmux_sessions(state)`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("executing statement: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}