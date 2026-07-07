package defaults

import (
	"context"
	"testing"

	"github.com/MichielDean/devteam/internal/db"
)

const defaultsTestDSN = "host=localhost port=5432 user=devteam password=devteam dbname=devteam_test_defaults sslmode=disable"

func setupDefaultsTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(db.Config{DSN: defaultsTestDSN}, defaultsTestDSN)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	// Truncate the feature_defaults + audit_events tables for clean test state.
	// Uses a dedicated DB (devteam_test_defaults) to avoid races with the
	// internal/db package tests which truncate the shared devteam_test_db.
	d.Conn().Exec("TRUNCATE TABLE feature_defaults CASCADE")
	d.Conn().Exec("TRUNCATE TABLE audit_events CASCADE")
	t.Cleanup(func() { d.Close() })
	return d
}

func TestStore_GetGlobal_Empty(t *testing.T) {
	s := NewStore(setupDefaultsTestDB(t))
	got, err := s.GetGlobal(context.Background())
	if err != nil {
		t.Fatalf("GetGlobal: %v", err)
	}
	if got.Scope != "" || got.Depth != "" || got.TestStrategy != "" || got.ExecutionMode != "" {
		t.Errorf("empty global = %+v, want zero-value", got)
	}
}

func TestStore_PutGlobal_RoundTrip(t *testing.T) {
	s := NewStore(setupDefaultsTestDB(t))
	in := Defaults{Scope: "feature", Depth: "standard", TestStrategy: "unit", ExecutionMode: "human"}
	got, err := s.PutGlobal(context.Background(), in, "operator")
	if err != nil {
		t.Fatalf("PutGlobal: %v", err)
	}
	if got.Scope != "feature" {
		t.Errorf("scope = %q, want feature", got.Scope)
	}

	// Re-read.
	got2, err := s.GetGlobal(context.Background())
	if err != nil {
		t.Fatalf("GetGlobal after put: %v", err)
	}
	if got2.Scope != "feature" || got2.Depth != "standard" || got2.TestStrategy != "unit" || got2.ExecutionMode != "human" {
		t.Errorf("round-trip = %+v, want %+v", got2, in)
	}
}

func TestStore_PutGlobal_Upsert(t *testing.T) {
	s := NewStore(setupDefaultsTestDB(t))
	// First put inserts.
	_, err := s.PutGlobal(context.Background(), Defaults{Scope: "feature"}, "operator")
	if err != nil {
		t.Fatalf("first PutGlobal: %v", err)
	}
	// Second put updates the same row (not a duplicate).
	_, err = s.PutGlobal(context.Background(), Defaults{Scope: "enterprise"}, "operator")
	if err != nil {
		t.Fatalf("second PutGlobal: %v", err)
	}
	got, err := s.GetGlobal(context.Background())
	if err != nil {
		t.Fatalf("GetGlobal: %v", err)
	}
	if got.Scope != "enterprise" {
		t.Errorf("scope = %q, want enterprise (upsert)", got.Scope)
	}
	// Verify only one global row exists.
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM feature_defaults WHERE repo IS NULL`).Scan(&count)
	if count != 1 {
		t.Errorf("global row count = %d, want 1 (upsert must not duplicate)", count)
	}
}

func TestStore_PutForRepo_RoundTrip(t *testing.T) {
	s := NewStore(setupDefaultsTestDB(t))
	in := Defaults{Scope: "greenfield", Depth: "comprehensive"}
	got, err := s.PutForRepo(context.Background(), "devteam", in, "operator")
	if err != nil {
		t.Fatalf("PutForRepo: %v", err)
	}
	if got.Scope != "greenfield" {
		t.Errorf("scope = %q, want greenfield", got.Scope)
	}
	if got.Repo != "devteam" {
		t.Errorf("repo = %q, want devteam", got.Repo)
	}

	// Re-read.
	got2, err := s.GetForRepo(context.Background(), "devteam")
	if err != nil {
		t.Fatalf("GetForRepo: %v", err)
	}
	if got2.Scope != "greenfield" {
		t.Errorf("round-trip scope = %q, want greenfield", got2.Scope)
	}
}

func TestStore_GetForRepo_NotFound(t *testing.T) {
	s := NewStore(setupDefaultsTestDB(t))
	_, err := s.GetForRepo(context.Background(), "nonexistent")
	if err != ErrNotFound {
		t.Errorf("GetForRepo nonexistent = %v, want ErrNotFound", err)
	}
}

func TestStore_DeleteForRepo(t *testing.T) {
	s := NewStore(setupDefaultsTestDB(t))
	_, err := s.PutForRepo(context.Background(), "devteam", Defaults{Scope: "feature"}, "operator")
	if err != nil {
		t.Fatalf("PutForRepo: %v", err)
	}
	if err := s.DeleteForRepo(context.Background(), "devteam", "operator"); err != nil {
		t.Fatalf("DeleteForRepo: %v", err)
	}
	_, err = s.GetForRepo(context.Background(), "devteam")
	if err != ErrNotFound {
		t.Errorf("GetForRepo after delete = %v, want ErrNotFound", err)
	}
}

func TestStore_DeleteForRepo_NotFound(t *testing.T) {
	s := NewStore(setupDefaultsTestDB(t))
	err := s.DeleteForRepo(context.Background(), "nonexistent", "operator")
	if err != ErrNotFound {
		t.Errorf("DeleteForRepo nonexistent = %v, want ErrNotFound", err)
	}
}

func TestStore_ListPerRepo_Empty(t *testing.T) {
	s := NewStore(setupDefaultsTestDB(t))
	got, err := s.ListPerRepo(context.Background())
	if err != nil {
		t.Fatalf("ListPerRepo: %v", err)
	}
	if got == nil {
		t.Error("ListPerRepo returned nil, want []")
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestStore_ListPerRepo_Populated(t *testing.T) {
	s := NewStore(setupDefaultsTestDB(t))
	s.PutForRepo(context.Background(), "cistern", Defaults{Scope: "feature"}, "operator")
	s.PutForRepo(context.Background(), "devteam", Defaults{Scope: "enterprise"}, "operator")
	// Also put a global row — it must NOT appear in ListPerRepo.
	s.PutGlobal(context.Background(), Defaults{Scope: "security-patch"}, "operator")

	got, err := s.ListPerRepo(context.Background())
	if err != nil {
		t.Fatalf("ListPerRepo: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (per-repo only, global excluded)", len(got))
	}
	// Ordered by repo name → cistern first.
	if got[0].Repo != "cistern" || got[1].Repo != "devteam" {
		t.Errorf("order = %s, %s; want cistern, devteam", got[0].Repo, got[1].Repo)
	}
}

func TestStore_EmitsFeatureDefaultsMutated(t *testing.T) {
	d := setupDefaultsTestDB(t)
	// Clear audit_events so we can count cleanly.
	d.Conn().Exec("TRUNCATE TABLE audit_events CASCADE")
	s := NewStore(d)

	_, err := s.PutGlobal(context.Background(), Defaults{Scope: "feature"}, "operator")
	if err != nil {
		t.Fatalf("PutGlobal: %v", err)
	}

	events, total, err := d.GetAuditEventsFiltered(db.AuditFilter{EventType: db.AuditFeatureDefaultsMutated, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("GetAuditEventsFiltered: %v", err)
	}
	if total != 1 {
		t.Errorf("audit events = %d, want 1", total)
	}
	if len(events) > 0 && events[0].Actor != "operator" {
		t.Errorf("actor = %q, want operator", events[0].Actor)
	}
}