package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 1,
		Name:    "initial_schema",
		Up:      migration001InitialSchema,
	})
}

// migration001InitialSchema creates all initial tables.
func migration001InitialSchema(tx *sql.Tx) error {
	statements := []string{
		// features — replaces .devteam-state.yaml
		`CREATE TABLE IF NOT EXISTS features (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			current_phase TEXT NOT NULL DEFAULT 'inception',
			status TEXT NOT NULL DEFAULT 'draft',
			priority INTEGER NOT NULL DEFAULT 3,
			intake_path TEXT NOT NULL DEFAULT 'loose_idea',
			spec_dir TEXT NOT NULL,
			worktree_dir TEXT DEFAULT '',
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			recirculation_count INTEGER NOT NULL DEFAULT 0
		)`,

		// phase_states — per-phase tracking
		`CREATE TABLE IF NOT EXISTS phase_states (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			phase TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'draft',
			started_at TIMESTAMP,
			completed_at TIMESTAMP,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE,
			UNIQUE(feature_id, phase)
		)`,

		// gate_results — every gate evaluation with per-check pass/fail
		`CREATE TABLE IF NOT EXISTS gate_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			phase TEXT NOT NULL,
			passed INTEGER NOT NULL,
			check_name TEXT NOT NULL,
			check_passed INTEGER NOT NULL,
			check_message TEXT DEFAULT '',
			evaluated_at TIMESTAMP NOT NULL,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		)`,

		// questions — interactive Q&A (replaces questions.json)
		`CREATE TABLE IF NOT EXISTS questions (
			id TEXT PRIMARY KEY,
			feature_id TEXT NOT NULL,
			phase TEXT NOT NULL,
			role TEXT NOT NULL,
			question TEXT NOT NULL,
			question_type TEXT NOT NULL DEFAULT 'multiple_choice',
			options TEXT DEFAULT '[]',
			answer TEXT DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			assumed INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL,
			answered_at TIMESTAMP,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		)`,

		// notes — inter-phase communication (Cistern cataractae_notes pattern)
		`CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			phase TEXT NOT NULL,
			role TEXT NOT NULL,
			note_type TEXT NOT NULL DEFAULT 'summary',
			content TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		)`,

		// sessions — agent dispatch metadata
		`CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			phase TEXT NOT NULL,
			role TEXT NOT NULL,
			tmux_session TEXT DEFAULT '',
			duration_ms INTEGER DEFAULT 0,
			output_length INTEGER DEFAULT 0,
			success INTEGER NOT NULL DEFAULT 0,
			error TEXT DEFAULT '',
			log_path TEXT DEFAULT '',
			started_at TIMESTAMP NOT NULL,
			ended_at TIMESTAMP,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		)`,

		// recirculations — track churn
		`CREATE TABLE IF NOT EXISTS recirculations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			from_phase TEXT NOT NULL,
			to_phase TEXT NOT NULL,
			reason TEXT NOT NULL DEFAULT 'gate_failed',
			failure_details TEXT DEFAULT '',
			created_at TIMESTAMP NOT NULL,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		)`,

		// events — audit trail
		`CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			phase TEXT DEFAULT '',
			details TEXT DEFAULT '',
			created_at TIMESTAMP NOT NULL,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		)`,

		// indexes
		`CREATE INDEX IF NOT EXISTS idx_features_status ON features(status)`,
		`CREATE INDEX IF NOT EXISTS idx_features_priority ON features(priority)`,
		`CREATE INDEX IF NOT EXISTS idx_phase_states_feature ON phase_states(feature_id)`,
		`CREATE INDEX IF NOT EXISTS idx_gate_results_feature ON gate_results(feature_id, phase)`,
		`CREATE INDEX IF NOT EXISTS idx_questions_feature ON questions(feature_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_notes_feature ON notes(feature_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_feature ON sessions(feature_id, phase)`,
		`CREATE INDEX IF NOT EXISTS idx_recirculations_feature ON recirculations(feature_id)`,
		`CREATE INDEX IF NOT EXISTS idx_events_feature ON events(feature_id)`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("executing statement: %w\nSQL: %s", err, stmt)
		}
	}

	return nil
}