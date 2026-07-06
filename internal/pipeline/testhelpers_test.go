package pipeline

import (
	"testing"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/spec"
)

// newTestProvider creates a DB-backed SpecProvider for pipeline tests.
// Truncates all data tables before each test for clean state.
func newTestProvider(t *testing.T, baseDir string) (*spec.SpecProvider, *db.DB) {
	t.Helper()
	dsn := "host=localhost port=5432 user=devteam password=devteam dbname=devteam_test_pipeline sslmode=disable"
	database, err := db.Open(db.Config{DSN: dsn}, dsn)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	truncateAllTables(database)
	t.Cleanup(func() { database.Close() })
	sp := spec.NewSpecProvider(baseDir)
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
		"repo_operation_config",
	}
	for _, table := range tables {
		database.Conn().Exec("TRUNCATE TABLE " + table + " CASCADE")
	}
}

// newTestWriter creates a SpecWriter linked to a DB-backed provider.
func newTestWriter(sp *spec.SpecProvider) *spec.SpecWriter {
	sw := spec.NewSpecWriter(sp.BaseDir())
	sw.SetProvider(sp)
	return sw
}