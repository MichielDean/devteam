package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database connection and provides access to all stores.
type DB struct {
	conn *sql.DB
	path string
}

// Open opens (or creates) the SQLite database at the given path.
func Open(path string) (*DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	conn.SetMaxOpenConns(1) // SQLite doesn't handle concurrent writes well

	db := &DB{conn: conn, path: path}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	log.Printf("db: opened %s", path)
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the raw database connection for direct queries.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

func (db *DB) migrate() error {
	migrations := []string{
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
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			recirculation_count INTEGER NOT NULL DEFAULT 0
		);`,

		// phase_states — per-phase tracking (status, gate result, timestamps)
		`CREATE TABLE IF NOT EXISTS phase_states (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			phase TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'draft',
			started_at DATETIME,
			completed_at DATETIME,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE,
			UNIQUE(feature_id, phase)
		);`,

		// gate_results — every gate evaluation with pass/fail per check
		`CREATE TABLE IF NOT EXISTS gate_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			phase TEXT NOT NULL,
			passed INTEGER NOT NULL,
			check_name TEXT NOT NULL,
			check_passed INTEGER NOT NULL,
			check_message TEXT DEFAULT '',
			evaluated_at DATETIME NOT NULL,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		);`,

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
			created_at DATETIME NOT NULL,
			answered_at DATETIME,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		);`,

		// notes — inter-phase communication (replaces NOTES.md, Cistern cataractae_notes pattern)
		`CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			phase TEXT NOT NULL,
			role TEXT NOT NULL,
			note_type TEXT NOT NULL DEFAULT 'summary',
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		);`,

		// sessions — agent dispatch metadata (tmux session, duration, outcome)
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
			started_at DATETIME NOT NULL,
			ended_at DATETIME,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		);`,

		// recirculations — track churn (how many times a phase ran, why it failed)
		`CREATE TABLE IF NOT EXISTS recirculations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			from_phase TEXT NOT NULL,
			to_phase TEXT NOT NULL,
			reason TEXT NOT NULL DEFAULT 'gate_failed',
			failure_details TEXT DEFAULT '',
			created_at DATETIME NOT NULL,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		);`,

		// events — audit trail for every state transition
		`CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			phase TEXT DEFAULT '',
			details TEXT DEFAULT '',
			created_at DATETIME NOT NULL,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		);`,

		// indexes for query performance
		`CREATE INDEX IF NOT EXISTS idx_features_status ON features(status);`,
		`CREATE INDEX IF NOT EXISTS idx_features_priority ON features(priority);`,
		`CREATE INDEX IF NOT EXISTS idx_phase_states_feature ON phase_states(feature_id);`,
		`CREATE INDEX IF NOT EXISTS idx_gate_results_feature ON gate_results(feature_id, phase);`,
		`CREATE INDEX IF NOT EXISTS idx_questions_feature ON questions(feature_id, status);`,
		`CREATE INDEX IF NOT EXISTS idx_notes_feature ON notes(feature_id);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_feature ON sessions(feature_id, phase);`,
		`CREATE INDEX IF NOT EXISTS idx_recirculations_feature ON recirculations(feature_id);`,
		`CREATE INDEX IF NOT EXISTS idx_events_feature ON events(feature_id);`,
	}

	for _, m := range migrations {
		if _, err := db.conn.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
		}
	}

	log.Printf("db: migrations complete (%d statements)", len(migrations))
	return nil
}

// Event types for the events table
const (
	EventPhaseStart       = "phase_start"
	EventPhaseComplete    = "phase_complete"
	EventGatePass         = "gate_pass"
	EventGateFail         = "gate_fail"
	EventRecirculate      = "recirculate"
	EventAdvance          = "advance"
	EventQuestionAsked    = "question_asked"
	EventQuestionAnswered = "question_answered"
	EventQuestionAssumed  = "question_assumed"
	EventMarkDone         = "mark_done"
	EventCancel           = "cancel"
	EventPRCreated        = "pr_created"
	EventSessionStart     = "session_start"
	EventSessionEnd       = "session_end"
	EventLivenessKill     = "liveness_kill"
)

// RecordEvent inserts an event into the audit trail.
func (db *DB) RecordEvent(featureID, eventType, phase, details string) error {
	_, err := db.conn.Exec(
		`INSERT INTO events (feature_id, event_type, phase, details, created_at) VALUES (?, ?, ?, ?, ?)`,
		featureID, eventType, phase, details, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("recording event: %w", err)
	}
	return nil
}