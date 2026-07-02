package db

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
)

// Migration represents a single database migration.
type Migration struct {
	Version int
	Name    string
	Up      func(tx *sql.Tx) error
}

// migrations is the registered list of all migrations, sorted by version.
var migrations []Migration

// RegisterMigration adds a migration to the list. Called from init() in migration files.
func RegisterMigration(m Migration) {
	migrations = append(migrations, m)
}

// migrate runs all pending migrations in order.
func (db *DB) migrate() error {
	_, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	// Get applied migrations
	applied := make(map[int]bool)
	rows, err := db.conn.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("querying applied migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("scanning migration version: %w", err)
		}
		applied[version] = true
	}

	// Sort migrations by version
	sortedMigrations := make([]Migration, len(migrations))
	copy(sortedMigrations, migrations)
	sort.Slice(sortedMigrations, func(i, j int) bool {
		return sortedMigrations[i].Version < sortedMigrations[j].Version
	})

	// Run pending migrations
	for _, m := range sortedMigrations {
		if applied[m.Version] {
			continue
		}

		log.Printf("db: running migration %d: %s", m.Version, m.Name)

		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("beginning migration %d: %w", m.Version, err)
		}

		if err := m.Up(tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d (%s): %w", m.Version, m.Name, err)
		}

		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, m.Version, m.Name); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %d: %w", m.Version, err)
		}

		log.Printf("db: migration %d complete", m.Version)
	}

	log.Printf("db: migrations complete (%d total, %d pending)", len(sortedMigrations), len(sortedMigrations)-len(applied))
	return nil
}