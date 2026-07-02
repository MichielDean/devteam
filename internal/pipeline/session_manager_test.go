package pipeline

import (
	"testing"
	"time"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/role"
)

func seedFeatureRow(t *testing.T, d *db.DB, id string) {
	t.Helper()
	_, err := d.Exec(
		`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count) VALUES (?, ?, 'inception', 'in_progress', 3, 'loose_idea', '', ?, ?, 0) ON CONFLICT (id) DO NOTHING`,
		id, id, time.Now().UTC(), time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("seedFeature %s: %v", id, err)
	}
}

func TestSessionManagerResolveOrCreate(t *testing.T) {
	tmpDir := t.TempDir()
	sp, database := newTestProvider(t, tmpDir)
	dispatcher := role.NewDispatcher(tmpDir)
	pipe := NewPipelineWithDispatcher(nil, sp, dispatcher)
	pipe.SetDatabase(database)
	seedFeatureRow(t, database, "feat-test-1")

	sm := pipe.SessionMgr()
	if sm == nil {
		t.Fatal("SessionMgr is nil")
	}

	sessionName, contextDir, err := sm.ResolveOrCreateSession("feat-test-1", "ideation", 0)
	if err != nil {
		t.Fatalf("ResolveOrCreateSession: %v", err)
	}
	if sessionName != "devteam-feat-test-1-ideation" {
		t.Errorf("session name: got %s, want devteam-feat-test-1-ideation", sessionName)
	}
	if contextDir == "" {
		t.Error("context dir is empty")
	}

	sess, err := database.GetTmuxSession("feat-test-1", "ideation", 0)
	if err != nil || sess == nil {
		t.Fatalf("DB record not created: %v", err)
	}

	sessionName2, _, err := sm.ResolveOrCreateSession("feat-test-1", "ideation", 0)
	if err != nil {
		t.Fatalf("ResolveOrCreateSession (2nd call): %v", err)
	}
	if sessionName2 != sessionName {
		t.Errorf("session name changed on reuse: %s vs %s", sessionName2, sessionName)
	}

	sessions, _ := database.ListTmuxSessionsForFeature("feat-test-1")
	if len(sessions) != 1 {
		t.Errorf("sessions count after reuse: got %d, want 1", len(sessions))
	}
}

func TestSessionManagerPerBolt(t *testing.T) {
	tmpDir := t.TempDir()
	sp, database := newTestProvider(t, tmpDir)
	dispatcher := role.NewDispatcher(tmpDir)
	pipe := NewPipelineWithDispatcher(nil, sp, dispatcher)
	pipe.SetDatabase(database)
	seedFeatureRow(t, database, "feat-bolt")
	sm := pipe.SessionMgr()

	name1, dir1, err := sm.ResolveOrCreateSession("feat-bolt", "construction", 1)
	if err != nil {
		t.Fatalf("ResolveOrCreateSession bolt 1: %v", err)
	}
	if name1 != "devteam-feat-bolt-construction-bolt1" {
		t.Errorf("bolt 1 session name: got %s", name1)
	}
	if dir1 == "" {
		t.Error("bolt 1 context dir is empty")
	}

	name2, _, err := sm.ResolveOrCreateSession("feat-bolt", "construction", 2)
	if err != nil {
		t.Fatalf("ResolveOrCreateSession bolt 2: %v", err)
	}
	if name2 != "devteam-feat-bolt-construction-bolt2" {
		t.Errorf("bolt 2 session name: got %s", name2)
	}

	sessions, _ := database.ListTmuxSessionsForFeature("feat-bolt")
	if len(sessions) != 2 {
		t.Errorf("sessions count: got %d, want 2", len(sessions))
	}
}

func TestSessionManagerStateTransitions(t *testing.T) {
	tmpDir := t.TempDir()
	sp, database := newTestProvider(t, tmpDir)
	dispatcher := role.NewDispatcher(tmpDir)
	pipe := NewPipelineWithDispatcher(nil, sp, dispatcher)
	pipe.SetDatabase(database)
	seedFeatureRow(t, database, "feat-state")
	sm := pipe.SessionMgr()

	sm.ResolveOrCreateSession("feat-state", "ideation", 0)

	sm.SetSessionRunning("feat-state", "ideation", 0, "1.1", "product")
	sess, _ := database.GetTmuxSession("feat-state", "ideation", 0)
	if sess.State != "running" {
		t.Errorf("state after running: got %s, want running", sess.State)
	}
	if sess.StageID != "1.1" {
		t.Errorf("stage_id: got %s, want 1.1", sess.StageID)
	}

	sm.SetSessionAwaitingGate("feat-state", "ideation", 0, "1.1")
	sess, _ = database.GetTmuxSession("feat-state", "ideation", 0)
	if sess.State != "awaiting_gate" {
		t.Errorf("state after awaiting gate: got %s, want awaiting_gate", sess.State)
	}

	sm.SetSessionAwaitingQuestion("feat-state", "ideation", 0, "1.1")
	sess, _ = database.GetTmuxSession("feat-state", "ideation", 0)
	if sess.State != "awaiting_question" {
		t.Errorf("state after awaiting question: got %s, want awaiting_question", sess.State)
	}

	sm.SetSessionResuming("feat-state", "ideation", 0)
	sess, _ = database.GetTmuxSession("feat-state", "ideation", 0)
	if sess.State != "resuming" {
		t.Errorf("state after resuming: got %s, want resuming", sess.State)
	}

	sm.SetSessionDone("feat-state", "ideation", 0)
	sess, _ = database.GetTmuxSession("feat-state", "ideation", 0)
	if sess.State != "done" {
		t.Errorf("state after done: got %s, want done", sess.State)
	}
}

func TestSessionManagerExpirePhase(t *testing.T) {
	tmpDir := t.TempDir()
	sp, database := newTestProvider(t, tmpDir)
	dispatcher := role.NewDispatcher(tmpDir)
	pipe := NewPipelineWithDispatcher(nil, sp, dispatcher)
	pipe.SetDatabase(database)
	seedFeatureRow(t, database, "feat-expire")
	sm := pipe.SessionMgr()

	sm.ResolveOrCreateSession("feat-expire", "ideation", 0)
	sm.ResolveOrCreateSession("feat-expire", "inception", 0)

	sm.ExpirePhaseSessions("feat-expire", "ideation")

	sess, _ := database.GetTmuxSession("feat-expire", "ideation", 0)
	if sess.State != "expired" {
		t.Errorf("ideation state: got %s, want expired", sess.State)
	}

	sess, _ = database.GetTmuxSession("feat-expire", "inception", 0)
	if sess.State == "expired" {
		t.Error("inception was expired — should be unaffected")
	}
}

func TestSessionManagerCleanupFeature(t *testing.T) {
	tmpDir := t.TempDir()
	sp, database := newTestProvider(t, tmpDir)
	dispatcher := role.NewDispatcher(tmpDir)
	pipe := NewPipelineWithDispatcher(nil, sp, dispatcher)
	pipe.SetDatabase(database)
	seedFeatureRow(t, database, "feat-cleanup")
	sm := pipe.SessionMgr()

	sm.ResolveOrCreateSession("feat-cleanup", "ideation", 0)
	sm.ResolveOrCreateSession("feat-cleanup", "inception", 0)
	sm.ResolveOrCreateSession("feat-cleanup", "construction", 1)

	sm.CleanupFeatureSessions("feat-cleanup")

	sessions, _ := database.ListTmuxSessionsForFeature("feat-cleanup")
	if len(sessions) != 0 {
		t.Errorf("sessions after cleanup: got %d, want 0", len(sessions))
	}
}

func TestSessionManagerListSessions(t *testing.T) {
	tmpDir := t.TempDir()
	sp, database := newTestProvider(t, tmpDir)
	dispatcher := role.NewDispatcher(tmpDir)
	pipe := NewPipelineWithDispatcher(nil, sp, dispatcher)
	pipe.SetDatabase(database)
	seedFeatureRow(t, database, "feat-list-a")
	seedFeatureRow(t, database, "feat-list-b")
	sm := pipe.SessionMgr()

	sm.ResolveOrCreateSession("feat-list-a", "ideation", 0)
	sm.ResolveOrCreateSession("feat-list-b", "inception", 0)

	sessions, err := sm.ListSessionsForFeature("feat-list-a")
	if err != nil {
		t.Fatalf("ListSessionsForFeature: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("sessions for feat-list-a: got %d, want 1", len(sessions))
	}

	active, err := sm.ListActiveSessions()
	if err != nil {
		t.Fatalf("ListActiveSessions: %v", err)
	}
	if len(active) < 2 {
		t.Errorf("active sessions: got %d, want >= 2", len(active))
	}
}