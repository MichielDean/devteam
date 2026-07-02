package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// DB wraps the PostgreSQL database connection and provides access to all stores.
type DB struct {
	conn *sql.DB
	dsn  string
}

// Config holds database connection configuration.
type Config struct {
	DSN string `yaml:"dsn" json:"dsn"` // PostgreSQL connection string
}

// Open opens a PostgreSQL database connection using the provided config.
// If config DSN is empty, uses the given default DSN.
func Open(cfg Config, defaultDSN string) (*DB, error) {
	dsn := cfg.DSN
	if dsn == "" {
		dsn = defaultDSN
	}

	return openPostgres(dsn)
}

func openPostgres(dsn string) (*DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening postgres database: %w", err)
	}

	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("pinging postgres database: %w", err)
	}

	db := &DB{conn: conn, dsn: dsn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	log.Printf("db: opened postgres (dsn hidden for security)")
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

// Placeholder converts ? placeholders to $1, $2, etc. for PostgreSQL.
func (db *DB) Placeholder(query string, args ...interface{}) (string, []interface{}) {
	return convertToPostgresPlaceholders(query), args
}

// convertToPostgresPlaceholders replaces ? with $1, $2, etc.
func convertToPostgresPlaceholders(query string) string {
	var b strings.Builder
	argNum := 1
	for _, ch := range query {
		if ch == '?' {
			b.WriteString(fmt.Sprintf("$%d", argNum))
			argNum++
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// Exec executes a query with PostgreSQL placeholders.
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	q, a := db.Placeholder(query, args...)
	return db.conn.Exec(q, a...)
}

// Query executes a query with PostgreSQL placeholders.
func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	q, a := db.Placeholder(query, args...)
	return db.conn.Query(q, a...)
}

// QueryRow executes a query with PostgreSQL placeholders.
func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	q, a := db.Placeholder(query, args...)
	return db.conn.QueryRow(q, a...)
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
	_, err := db.Exec(
		`INSERT INTO events (feature_id, event_type, phase, details, created_at) VALUES (?, ?, ?, ?, ?)`,
		featureID, eventType, phase, details, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("recording event: %w", err)
	}
	return nil
}

// columnExists checks if a column exists in a table using information_schema.
func (db *DB) columnExists(table, column string) bool {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = ? AND column_name = ?`,
		table, column,
	).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}