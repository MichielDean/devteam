package spec

import (
	"path/filepath"
	"testing"

	"github.com/MichielDean/devteam/internal/db"
)

// newTestProvider creates a SpecProvider wired to a temporary SQLite DB.
func newTestProvider(t *testing.T) (*SpecProvider, *db.DB) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	database, err := db.Open(db.Config{Driver: "sqlite3", DSN: path}, path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	sp := NewSpecProvider(dir)
	sp.SetDatabase(database)
	return sp, database
}

// newTestWriter creates a SpecWriter linked to a DB-backed SpecProvider.
func newTestWriter(t *testing.T) (*SpecWriter, *SpecProvider, *db.DB) {
	t.Helper()
	sp, database := newTestProvider(t)
	sw := NewSpecWriter(sp.baseDir)
	sw.SetProvider(sp)
	return sw, sp, database
}