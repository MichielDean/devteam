package intake

import (
	"path/filepath"
	"testing"

	"github.com/MichielDean/devteam/internal/db"
)

// setupTestIntake creates a DB-backed LooseIdeaIntake and ExternalSpecIntake.
func setupTestIntake(t *testing.T) (string, *db.DB) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	database, err := db.Open(db.Config{Driver: "sqlite3", DSN: path}, path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return dir, database
}