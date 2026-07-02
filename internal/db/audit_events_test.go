package db

import (
	"fmt"
	"testing"
	"time"
)

func TestAuditEventConstants(t *testing.T) {
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
	d, fid := openTestDBWithFeature(t)
	err := d.RecordAuditEvent(fid, AuditStageStart, "1.1", "ideation", "test details")
	if err != nil {
		t.Fatalf("RecordAuditEvent: %v", err)
	}

	events, err := d.GetAuditEvents(fid)
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
	d, fid := openTestDBWithFeature(t)
	d.RecordAuditEvent(fid, AuditStageStart, "1.1", "ideation", "")
	d.RecordAuditEvent(fid, AuditStageCompleted, "1.1", "ideation", "")
	d.RecordAuditEvent(fid, AuditStageStart, "1.2", "ideation", "")

	events, err := d.GetAuditEventsForStage(fid, "1.1")
	if err != nil {
		t.Fatalf("GetAuditEventsForStage: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events for stage 1.1, got %d", len(events))
	}
}

func TestGetAuditEventsChronological(t *testing.T) {
	d, fid := openTestDBWithFeature(t)
	d.RecordAuditEvent(fid, AuditStageStart, "1.1", "ideation", "first")
	d.RecordAuditEvent(fid, AuditStageCompleted, "1.1", "ideation", "second")

	events, err := d.GetAuditEvents(fid)
	if err != nil {
		t.Fatalf("GetAuditEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Details != "first" {
		t.Errorf("first event should be 'first', got %s", events[0].Details)
	}
}

// openTestDBWithFeature creates a test DB connection and inserts a unique feature row.
// Truncates all data for clean test state.
func openTestDBWithFeature(t *testing.T) (*DB, string) {
	t.Helper()
	database, err := Open(Config{DSN: postgresTestDSN}, postgresTestDSN)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	truncateAllTables(database)
	t.Cleanup(func() { database.Close() })
	fid := fmt.Sprintf("feat-audit-%s", t.Name())
	now := time.Now().UTC()
	_, err = database.Exec(`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (id) DO NOTHING`,
		fid, "Test Feature", "ideation", "draft", 3, "loose_idea", "specs/"+fid, now, now)
	if err != nil {
		t.Fatalf("inserting test feature: %v", err)
	}
	return database, fid
}