package db

import (
	"testing"
	"time"
)

func TestTmuxSessionCRUD(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-001")

	// Create
	err := d.CreateTmuxSession("feat-001", "ideation", 0, "devteam-feat-001-ideation", "/tmp/sessions/feat-001/ideation")
	if err != nil {
		t.Fatalf("CreateTmuxSession: %v", err)
	}

	// Get
	sess, err := d.GetTmuxSession("feat-001", "ideation", 0)
	if err != nil {
		t.Fatalf("GetTmuxSession: %v", err)
	}
	if sess == nil {
		t.Fatal("expected session, got nil")
	}
	if sess.SessionName != "devteam-feat-001-ideation" {
		t.Errorf("session name: got %s", sess.SessionName)
	}
	if sess.State != TmuxSessionCreated {
		t.Errorf("state: got %s, want %s", sess.State, TmuxSessionCreated)
	}
	if sess.BoltNumber != 0 {
		t.Errorf("bolt_number: got %d, want 0", sess.BoltNumber)
	}

	// Get by name
	sessByName, err := d.GetTmuxSessionByName("devteam-feat-001-ideation")
	if err != nil || sessByName == nil {
		t.Fatalf("GetTmuxSessionByName: %v", err)
	}
	if sessByName.ID != sess.ID {
		t.Errorf("ID mismatch: %d vs %d", sessByName.ID, sess.ID)
	}

	// Update state
	err = d.UpdateTmuxSessionState("feat-001", "ideation", 0, TmuxSessionRunning, "1.1", "product")
	if err != nil {
		t.Fatalf("UpdateTmuxSessionState: %v", err)
	}
	sess, _ = d.GetTmuxSession("feat-001", "ideation", 0)
	if sess.State != TmuxSessionRunning {
		t.Errorf("state after update: got %s, want %s", sess.State, TmuxSessionRunning)
	}
	if sess.StageID != "1.1" {
		t.Errorf("stage_id: got %s, want 1.1", sess.StageID)
	}
	if sess.LastAgent != "product" {
		t.Errorf("last_agent: got %s, want product", sess.LastAgent)
	}

	// List for feature
	err = d.CreateTmuxSession("feat-001", "inception", 0, "devteam-feat-001-inception", "/tmp/sessions/feat-001/inception")
	if err != nil {
		t.Fatalf("CreateTmuxSession inception: %v", err)
	}
	sessions, err := d.ListTmuxSessionsForFeature("feat-001")
	if err != nil {
		t.Fatalf("ListTmuxSessionsForFeature: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("sessions count: got %d, want 2", len(sessions))
	}

	// Expire phase
	err = d.ExpireTmuxSessionsForPhase("feat-001", "ideation")
	if err != nil {
		t.Fatalf("ExpireTmuxSessionsForPhase: %v", err)
	}
	sess, _ = d.GetTmuxSession("feat-001", "ideation", 0)
	if sess.State != TmuxSessionExpired {
		t.Errorf("state after expire: got %s, want %s", sess.State, TmuxSessionExpired)
	}

	// Delete all for feature
	err = d.DeleteTmuxSessionsForFeature("feat-001")
	if err != nil {
		t.Fatalf("DeleteTmuxSessionsForFeature: %v", err)
	}
	sessions, _ = d.ListTmuxSessionsForFeature("feat-001")
	if len(sessions) != 0 {
		t.Errorf("sessions after delete: got %d, want 0", len(sessions))
	}
}

func TestTmuxSessionBoltNumber(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-002")

	// Create construction bolt sessions
	d.CreateTmuxSession("feat-002", "construction", 1, "devteam-feat-002-construction-bolt1", "/tmp/feat-002/bolt1")
	d.CreateTmuxSession("feat-002", "construction", 2, "devteam-feat-002-construction-bolt2", "/tmp/feat-002/bolt2")

	// Get bolt 1
	sess, err := d.GetTmuxSession("feat-002", "construction", 1)
	if err != nil || sess == nil {
		t.Fatalf("GetTmuxSession bolt 1: %v", err)
	}
	if sess.BoltNumber != 1 {
		t.Errorf("bolt_number: got %d, want 1", sess.BoltNumber)
	}

	// Get bolt 2
	sess, err = d.GetTmuxSession("feat-002", "construction", 2)
	if err != nil || sess == nil {
		t.Fatalf("GetTmuxSession bolt 2: %v", err)
	}
	if sess.BoltNumber != 2 {
		t.Errorf("bolt_number: got %d, want 2", sess.BoltNumber)
	}
}

func TestListActiveTmuxSessions(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-active-a")
	seedFeature(t, d, "feat-active-b")
	seedFeature(t, d, "feat-active-c")

	d.CreateTmuxSession("feat-active-a", "ideation", 0, "devteam-feat-active-a-ideation", "/tmp/a")
	d.CreateTmuxSession("feat-active-b", "inception", 0, "devteam-feat-active-b-inception", "/tmp/b")
	d.CreateTmuxSession("feat-active-c", "construction", 1, "devteam-feat-active-c-construction-bolt1", "/tmp/c")

	// Mark one as done — should not appear in active
	d.UpdateTmuxSessionState("feat-active-c", "construction", 1, TmuxSessionDone, "3.5", "developer")

	active, err := d.ListActiveTmuxSessions()
	if err != nil {
		t.Fatalf("ListActiveTmuxSessions: %v", err)
	}
	// Check that our active sessions are present (other tests may add sessions too)
	activeNames := make(map[string]bool)
	for _, s := range active {
		activeNames[s.SessionName] = true
	}
	if !activeNames["devteam-feat-active-a-ideation"] {
		t.Error("feat-active-a ideation session not in active list")
	}
	if !activeNames["devteam-feat-active-b-inception"] {
		t.Error("feat-active-b inception session not in active list")
	}
	if activeNames["devteam-feat-active-c-construction-bolt1"] {
		t.Error("feat-active-c should not be active (marked done)")
	}
	for _, s := range active {
		if s.State == TmuxSessionDone || s.State == TmuxSessionFailed || s.State == TmuxSessionExpired {
			t.Errorf("found non-active session: %s state=%s", s.SessionName, s.State)
		}
	}
}

func TestUpdateTmuxSessionOutputTimestamp(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-x")
	d.CreateTmuxSession("feat-x", "ideation", 0, "devteam-feat-x-ideation", "/tmp/x")

	before := time.Now().UTC()
	time.Sleep(10 * time.Millisecond)
	d.UpdateTmuxSessionOutputTimestamp("devteam-feat-x-ideation")

	sess, _ := d.GetTmuxSessionByName("devteam-feat-x-ideation")
	if sess.LastOutputAt == nil {
		t.Fatal("last_output_at is nil")
	}
	if sess.LastOutputAt.Before(before) {
		t.Errorf("last_output_at not updated: %v before %v", sess.LastOutputAt, before)
	}
}