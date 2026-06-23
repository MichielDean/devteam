package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the database connection and provides access to all stores.
// Supports SQLite (local) and PostgreSQL (shared/multi-user) backends.
type DB struct {
	conn     *sql.DB
	driver   string // "sqlite3" or "postgres"
	path     string // for sqlite: file path; for postgres: connection string
}

// Config holds database connection configuration.
type Config struct {
	Driver string `yaml:"driver" json:"driver"` // "sqlite3" (default) or "postgres"
	DSN    string `yaml:"dsn" json:"dsn"`       // connection string
}

// Open opens a database connection using the provided config.
// If config is empty, defaults to SQLite at the given defaultPath.
func Open(cfg Config, defaultPath string) (*DB, error) {
	driver := cfg.Driver
	dsn := cfg.DSN

	if driver == "" {
		driver = "sqlite3"
	}
	if dsn == "" {
		dsn = defaultPath
	}

	switch driver {
	case "sqlite3":
		return openSQLite(dsn)
	case "postgres", "postgresql":
		return openPostgres(dsn)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s (use 'sqlite3' or 'postgres')", driver)
	}
}

func openSQLite(path string) (*DB, error) {
	dir := filepath.Dir(path)
	if dir != "." && dir != "/" {
		if err := mkdirAll(dir); err != nil {
			return nil, fmt.Errorf("creating db directory: %w", err)
		}
	}

	dsn := path + "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on"
	conn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}

	conn.SetMaxOpenConns(1) // SQLite doesn't handle concurrent writes well

	db := &DB{conn: conn, driver: "sqlite3", path: path}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	log.Printf("db: opened sqlite at %s", path)
	return db, nil
}

func openPostgres(dsn string) (*DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening postgres database: %w", err)
	}

	conn.SetMaxOpenConns(25) // PostgreSQL handles concurrent connections
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("pinging postgres database: %w", err)
	}

	db := &DB{conn: conn, driver: "postgres", path: dsn}
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

// Driver returns the database driver name ("sqlite3" or "postgres").
func (db *DB) Driver() string {
	return db.driver
}

// Placeholder returns the correct placeholder for the current driver.
// SQLite uses ? and PostgreSQL uses $1, $2, etc.
// Use this for queries that need to work across both drivers.
func (db *DB) Placeholder(query string, args ...interface{}) (string, []interface{}) {
	if db.driver == "postgres" {
		return convertToPostgresPlaceholders(query), args
	}
	return query, args
}

// convertToPostgresPlaceholders replaces ? with $1, $2, etc. for PostgreSQL.
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

// Exec executes a query with driver-appropriate placeholders.
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	q, a := db.Placeholder(query, args...)
	return db.conn.Exec(q, a...)
}

// Query executes a query with driver-appropriate placeholders.
func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	q, a := db.Placeholder(query, args...)
	return db.conn.Query(q, a...)
}

// QueryRow executes a query with driver-appropriate placeholders.
func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	q, a := db.Placeholder(query, args...)
	return db.conn.QueryRow(q, a...)
}

func mkdirAll(dir string) error {
	return osMkdirAll(dir, 0755)
}

// osMkdirAll is a variable for testing (can be overridden).
var osMkdirAll = func(dir string, perm os.FileMode) error {
	return os.MkdirAll(dir, perm)
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