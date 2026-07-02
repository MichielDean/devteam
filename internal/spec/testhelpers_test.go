package spec

import (
	"testing"

	"github.com/MichielDean/devteam/internal/db"
)

// newTestProvider creates a SpecProvider wired to a test PostgreSQL DB.
// Truncates all data tables before each test for clean state.
func newTestProvider(t *testing.T) (*SpecProvider, *db.DB) {
	t.Helper()
	dir := t.TempDir()
	dsn := "host=localhost port=5432 user=devteam password=devteam dbname=devteam_test_spec sslmode=disable"
	database, err := db.Open(db.Config{DSN: dsn}, dsn)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	truncateAllTables(database)
	t.Cleanup(func() { database.Close() })
	sp := NewSpecProvider(dir)
	sp.SetDatabase(database)
	return sp, database
}

// truncateAllTables clears all data tables for clean test state.
func truncateAllTables(database *db.DB) {
	tables := []string{
		"audit_events", "tmux_sessions", "bolts", "feature_stages",
		"spec_artifacts", "outcomes", "notes", "events", "questions",
		"rules", "team_knowledge", "feature_repos", "sessions",
		"phase_states", "gate_results", "recirculations", "features",
	}
	for _, table := range tables {
		database.Conn().Exec("TRUNCATE TABLE " + table + " CASCADE")
	}
}

// newTestWriter creates a SpecWriter linked to a DB-backed SpecProvider.
func newTestWriter(t *testing.T) (*SpecWriter, *SpecProvider, *db.DB) {
	t.Helper()
	sp, database := newTestProvider(t)
	sw := NewSpecWriter(sp.baseDir)
	sw.SetProvider(sp)
	return sw, sp, database
}