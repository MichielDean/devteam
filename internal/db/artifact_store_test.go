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
		"chat_messages", "chat_sessions",
		"audit_events", "tmux_sessions", "bolts", "feature_stages",
		"spec_artifacts", "outcomes", "notes", "events", "questions",
		"rules", "team_knowledge", "feature_repos", "sessions",
		"phase_states", "gate_results", "recirculations", "features",
		// repos tables (migration 017) — truncate so repo tests start clean.
		"repo_settings", "repo_operation_config", "repo_registry", "repos",
		"stage_logs",
	}
	for _, table := range tables {
		d.Conn().Exec("TRUNCATE TABLE " + table + " CASCADE")
	}
	// Re-seed the __chat__ sentinel feature row — it's the FK parent for
	// chat_cli_exec audit events with no real feature. Truncate removes it;
	// tests that depend on it need it present.
	seedChatSentinel(d)
}

// seedChatSentinel re-inserts the __chat__ sentinel feature row.
func seedChatSentinel(d *DB) {
	d.Exec(`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count, scope, depth, test_strategy)
		VALUES ('__chat__', '__chat__ sentinel', 'operation', 'sentinel', 0, 'loose_idea', '', now(), now(), 0, 'feature', 'minimal', 'standard')
		ON CONFLICT (id) DO NOTHING`)
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

func TestPerBoltStageTracking(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-bolt")
	// Seed the stage definitions the per-bolt logic needs (3.1-3.5, 1.1).
	// We can't import internal/stage here (circular), so seed directly.
	for _, s := range []StageDefinition{
		{ID: "1.1", Phase: "ideation", Name: "Intent Capture", LeadAgent: "product", Scopes: []string{"feature"}, SortOrder: 1},
		{ID: "3.1", Phase: "construction", Name: "Functional Design", LeadAgent: "architect", Scopes: []string{"feature"}, SortOrder: 20},
		{ID: "3.2", Phase: "construction", Name: "NFR Requirements", LeadAgent: "architect", Scopes: []string{"feature"}, SortOrder: 21},
		{ID: "3.3", Phase: "construction", Name: "NFR Design", LeadAgent: "architect", Scopes: []string{"feature"}, SortOrder: 22},
		{ID: "3.4", Phase: "construction", Name: "Infra Design", LeadAgent: "platform", Scopes: []string{"feature"}, SortOrder: 23},
		{ID: "3.5", Phase: "construction", Name: "Code Generation", LeadAgent: "developer", Scopes: []string{"feature"}, SortOrder: 24},
	} {
		if err := d.UpsertStageDefinition(s); err != nil {
			t.Fatalf("UpsertStageDefinition %s: %v", s.ID, err)
		}
	}

	// Init non-construction stages (bolt_number=0).
	if err := d.InitFeatureStages("feat-bolt", "feature"); err != nil {
		t.Fatalf("InitFeatureStages: %v", err)
	}

	// Init per-Bolt stage rows for bolt 1 and bolt 2.
	if err := d.InitBoltStages("feat-bolt", 1, "feature"); err != nil {
		t.Fatalf("InitBoltStages bolt 1: %v", err)
	}
	if err := d.InitBoltStages("feat-bolt", 2, "feature"); err != nil {
		t.Fatalf("InitBoltStages bolt 2: %v", err)
	}

	// 3.1 for bolt 1 should be a distinct row from 3.1 for bolt 2.
	fs1, err := d.GetFeatureStageForBolt("feat-bolt", "3.1", 1)
	if err != nil || fs1 == nil {
		t.Fatalf("GetFeatureStageForBolt 3.1/1: %v", err)
	}
	if fs1.BoltNumber != 1 || fs1.Status != "not_started" {
		t.Errorf("bolt 1 row = %+v, want BoltNumber=1 Status=not_started", fs1)
	}

	fs2, err := d.GetFeatureStageForBolt("feat-bolt", "3.1", 2)
	if err != nil || fs2 == nil {
		t.Fatalf("GetFeatureStageForBolt 3.1/2: %v", err)
	}
	if fs2.BoltNumber != 2 {
		t.Errorf("bolt 2 row BoltNumber = %d, want 2", fs2.BoltNumber)
	}

	// Mark bolt 1's 3.1 complete; bolt 2's 3.1 should stay not_started.
	if err := d.UpdateFeatureStageForBolt("feat-bolt", "3.1", 1, "completed", 0, nil, nil); err != nil {
		t.Fatalf("UpdateFeatureStageForBolt: %v", err)
	}
	fs1After, _ := d.GetFeatureStageForBolt("feat-bolt", "3.1", 1)
	fs2After, _ := d.GetFeatureStageForBolt("feat-bolt", "3.1", 2)
	if fs1After.Status != "completed" {
		t.Errorf("bolt 1 status = %s, want completed", fs1After.Status)
	}
	if fs2After.Status != "not_started" {
		t.Errorf("bolt 2 status = %s, want not_started (should be unaffected)", fs2After.Status)
	}

	// Non-construction stage (1.1) should be at bolt_number=0.
	fs11, err := d.GetFeatureStage("feat-bolt", "1.1")
	if err != nil || fs11 == nil {
		t.Fatalf("GetFeatureStage 1.1: %v", err)
	}
	if fs11.BoltNumber != 0 {
		t.Errorf("1.1 BoltNumber = %d, want 0 (non-construction)", fs11.BoltNumber)
	}

	// InitBoltStages is idempotent — re-init bolt 1 should not error or duplicate.
	if err := d.InitBoltStages("feat-bolt", 1, "feature"); err != nil {
		t.Fatalf("InitBoltStages bolt 1 (repeat): %v", err)
	}
}

func TestPerBoltStageLogs(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-log")

	// Bolt 1's 3.5 log and bolt 2's 3.5 log should be distinct rows.
	if err := d.SaveStageLogForBolt("feat-log", "3.5", 1, "developer", "bolt 1 output"); err != nil {
		t.Fatalf("SaveStageLogForBolt 1: %v", err)
	}
	if err := d.SaveStageLogForBolt("feat-log", "3.5", 2, "developer", "bolt 2 output"); err != nil {
		t.Fatalf("SaveStageLogForBolt 2: %v", err)
	}

	log1, err := d.GetStageLogForBolt("feat-log", "3.5", 1)
	if err != nil {
		t.Fatalf("GetStageLogForBolt 1: %v", err)
	}
	if log1 != "bolt 1 output" {
		t.Errorf("bolt 1 log = %q, want %q", log1, "bolt 1 output")
	}

	log2, err := d.GetStageLogForBolt("feat-log", "3.5", 2)
	if err != nil {
		t.Fatalf("GetStageLogForBolt 2: %v", err)
	}
	if log2 != "bolt 2 output" {
		t.Errorf("bolt 2 log = %q, want %q", log2, "bolt 2 output")
	}

	// Non-construction stage at bolt_number=0 via the legacy signature.
	if err := d.SaveStageLog("feat-log", "1.1", "product", "ideation output"); err != nil {
		t.Fatalf("SaveStageLog 1.1: %v", err)
	}
	log11, err := d.GetStageLog("feat-log", "1.1")
	if err != nil {
		t.Fatalf("GetStageLog 1.1: %v", err)
	}
	if log11 != "ideation output" {
		t.Errorf("1.1 log = %q, want %q", log11, "ideation output")
	}

	// Overwriting bolt 1's log must not affect bolt 2's log.
	if err := d.SaveStageLogForBolt("feat-log", "3.5", 1, "developer", "bolt 1 revised"); err != nil {
		t.Fatalf("SaveStageLogForBolt 1 (revise): %v", err)
	}
	log2After, _ := d.GetStageLogForBolt("feat-log", "3.5", 2)
	if log2After != "bolt 2 output" {
		t.Errorf("bolt 2 log after bolt 1 revise = %q, want %q (should be unaffected)", log2After, "bolt 2 output")
	}
}