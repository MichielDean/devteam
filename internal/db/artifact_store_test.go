package db

import (
	"testing"
	"time"
)

const postgresTestDSN = "host=localhost port=5432 user=devteam password=devteam dbname=devteam_test_db sslmode=disable"

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(Config{DSN: postgresTestDSN}, postgresTestDSN)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	truncateAllTables(d)
	t.Cleanup(func() { d.Close() })
	return d
}

// truncateAllTables clears all data tables for clean test state.
func truncateAllTables(d *DB) {
	tables := []string{
		"audit_events", "tmux_sessions", "bolts", "feature_stages",
		"spec_artifacts", "outcomes", "notes", "events", "questions",
		"rules", "team_knowledge", "feature_repos", "sessions",
		"phase_states", "gate_results", "recirculations", "features",
		"repos",
	}
	for _, table := range tables {
		d.Conn().Exec("TRUNCATE TABLE " + table + " CASCADE")
	}
}

func seedFeature(t *testing.T, d *DB, id string) {
	t.Helper()
	now := time.Now().UTC()
	_, err := d.Exec(
		`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count) VALUES (?, ?, 'inception', 'in_progress', 3, 'loose_idea', '', ?, ?, 0) ON CONFLICT (id) DO NOTHING`,
		id, id, now, now)
	if err != nil {
		t.Fatalf("seedFeature %s: %v", id, err)
	}
}

func TestSaveAndGetArtifact(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-1")

	if err := d.SaveArtifact("feat-1", "spec", "# Spec content"); err != nil {
		t.Fatalf("SaveArtifact: %v", err)
	}

	a, err := d.GetArtifact("feat-1", "spec")
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if a.Content != "# Spec content" {
		t.Errorf("content = %q, want %q", a.Content, "# Spec content")
	}
	if a.FeatureID != "feat-1" {
		t.Errorf("feature_id = %q, want feat-1", a.FeatureID)
	}
	if a.ArtifactType != "spec" {
		t.Errorf("artifact_type = %q, want spec", a.ArtifactType)
	}
}

func TestSaveArtifactUpserts(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-1")

	if err := d.SaveArtifact("feat-1", "plan", "v1"); err != nil {
		t.Fatalf("SaveArtifact v1: %v", err)
	}
	if err := d.SaveArtifact("feat-1", "plan", "v2"); err != nil {
		t.Fatalf("SaveArtifact v2: %v", err)
	}

	a, err := d.GetArtifact("feat-1", "plan")
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if a.Content != "v2" {
		t.Errorf("after upsert content = %q, want v2", a.Content)
	}
}

func TestGetArtifactNotFound(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-1")

	_, err := d.GetArtifact("feat-1", "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing artifact")
	}
}

func TestListArtifacts(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-1")
	seedFeature(t, d, "feat-2")

	d.SaveArtifact("feat-1", "spec", "s")
	d.SaveArtifact("feat-1", "plan", "p")
	d.SaveArtifact("feat-2", "spec", "other")

	arts, err := d.ListArtifacts("feat-1")
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(arts))
	}
}

func TestDeleteArtifact(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-1")

	d.SaveArtifact("feat-1", "spec", "content")
	d.DeleteArtifact("feat-1", "spec")

	_, err := d.GetArtifact("feat-1", "spec")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}