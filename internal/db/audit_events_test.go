package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAuditEventConstants(t *testing.T) {
	// Verify key event types are defined and non-empty
	events := []string{
		AuditWorkflowStart, AuditWorkflowComplete,
		AuditPhaseStart, AuditPhaseComplete,
		AuditStageStart, AuditStageCompleted, AuditStageSkipped,
		AuditGateApproved, AuditGateRejected, AuditGateAcceptAsIs,
		AuditRuleLearned, AuditRuleApplied,
		AuditBoltStarted, AuditBoltCompleted,
		AuditSubagentCompleted,
		AuditJumpToStage, AuditJumpToPhase,
	}
	for _, e := range events {
		if e == "" {
			t.Error("audit event constant is empty")
		}
	}
}

func TestRecordAuditEvent(t *testing.T) {
	d := openTestDB(t)
	err := d.RecordAuditEvent("feat-1", AuditStageStart, "1.1", "ideation", "test details")
	if err != nil {
		t.Fatalf("RecordAuditEvent: %v", err)
	}

	events, err := d.GetAuditEvents("feat-1")
	if err != nil {
		t.Fatalf("GetAuditEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != AuditStageStart {
		t.Errorf("EventType = %s, want %s", events[0].EventType, AuditStageStart)
	}
	if events[0].StageID != "1.1" {
		t.Errorf("StageID = %s, want 1.1", events[0].StageID)
	}
}

func TestGetAuditEventsForStage(t *testing.T) {
	d := openTestDB(t)
	d.RecordAuditEvent("feat-1", AuditStageStart, "1.1", "ideation", "")
	d.RecordAuditEvent("feat-1", AuditStageCompleted, "1.1", "ideation", "")
	d.RecordAuditEvent("feat-1", AuditStageStart, "1.2", "ideation", "")

	events, err := d.GetAuditEventsForStage("feat-1", "1.1")
	if err != nil {
		t.Fatalf("GetAuditEventsForStage: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events for stage 1.1, got %d", len(events))
	}
}

func TestGetAuditEventsChronological(t *testing.T) {
	d := openTestDB(t)
	d.RecordAuditEvent("feat-1", AuditStageStart, "1.1", "ideation", "first")
	d.RecordAuditEvent("feat-1", AuditStageCompleted, "1.1", "ideation", "second")

	events, err := d.GetAuditEvents("feat-1")
	if err != nil {
		t.Fatalf("GetAuditEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	// Should be ordered by created_at ASC, id ASC
	if events[0].Details != "first" {
		t.Errorf("first event should be 'first', got %s", events[0].Details)
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(Config{Driver: "sqlite3", DSN: dbPath}, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	// Insert a feature row to satisfy FK constraints
	_, err = db.Exec(`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"feat-1", "Test Feature", "ideation", "draft", 3, "loose_idea", "specs/feat-1", time.Now().UTC(), time.Now().UTC())
	if err != nil {
		t.Fatalf("inserting test feature: %v", err)
	}
	return db
}