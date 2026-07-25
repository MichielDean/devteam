package intake

import (
	"testing"

	"github.com/MichielDean/devteam/internal/db"
)

// setupTestIntake creates a DB-backed LooseIdeaIntake and ExternalSpecIntake.
// Truncates all data tables for clean test state.
func setupTestIntake(t *testing.T) (string, *db.DB) {
	t.Helper()
	dir := t.TempDir()
	dsn := "host=localhost port=5432 user=devteam password=devteam dbname=devteam_test_intake sslmode=disable"
	database, err := db.Open(db.Config{DSN: dsn}, dsn)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	truncateAllTables(database)
	t.Cleanup(func() { database.Close() })
	return dir, database
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