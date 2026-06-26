package pipeline

import (
	"path/filepath"
	"testing"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/spec"
)

// newTestProvider creates a DB-backed SpecProvider for pipeline tests.
func newTestProvider(t *testing.T, baseDir string) (*spec.SpecProvider, *db.DB) {
	t.Helper()
	dbPath := filepath.Join(baseDir, "test.db")
	database, err := db.Open(db.Config{Driver: "sqlite3", DSN: dbPath}, dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	sp := spec.NewSpecProvider(baseDir)
	sp.SetDatabase(database)
	return sp, database
}

// newTestWriter creates a SpecWriter linked to a DB-backed provider.
func newTestWriter(sp *spec.SpecProvider) *spec.SpecWriter {
	sw := spec.NewSpecWriter(sp.BaseDir())
	sw.SetProvider(sp)
	return sw
}