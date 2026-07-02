package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListSessionsEndpoint(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	defer s.httpServer.Close()

	fid := insertTestFeature(t, database, "sessions", s)

	// Create a session in DB
	database.CreateTmuxSession(fid, "ideation", 0, "devteam-"+fid+"-ideation", "/tmp/"+fid+"/ideation")

	req := httptest.NewRequest("GET", "/api/features/"+fid+"/sessions", nil)
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var sessions []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &sessions)
	if len(sessions) != 1 {
		t.Errorf("sessions count: got %d, want 1", len(sessions))
	}
	if len(sessions) > 0 {
		if sessions[0]["session_name"] != "devteam-"+fid+"-ideation" {
			t.Errorf("session_name: got %v", sessions[0]["session_name"])
		}
		if sessions[0]["is_alive"] != false {
			t.Errorf("is_alive: got %v, want false (no real tmux session)", sessions[0]["is_alive"])
		}
	}
}

func TestListSessionsEmpty(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	defer s.httpServer.Close()

	fid := insertTestFeature(t, database, "nosessions", s)

	req := httptest.NewRequest("GET", "/api/features/"+fid+"/sessions", nil)
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var sessions []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &sessions)
	if len(sessions) != 0 {
		t.Errorf("sessions count: got %d, want 0", len(sessions))
	}
}

func TestListActiveSessionsEndpoint(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	defer s.httpServer.Close()

	fid1 := insertTestFeature(t, database, "active1", s)
	fid2 := insertTestFeature(t, database, "active2", s)

	database.CreateTmuxSession(fid1, "ideation", 0, "devteam-"+fid1+"-ideation", "/tmp/a")
	database.CreateTmuxSession(fid2, "inception", 0, "devteam-"+fid2+"-inception", "/tmp/b")

	// Mark one as done — should not appear
	database.UpdateTmuxSessionState(fid2, "inception", 0, "done", "2.1", "product")

	req := httptest.NewRequest("GET", "/api/sessions/active", nil)
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var sessions []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &sessions)
	// Should only have the one active session (ideation)
	if len(sessions) != 1 {
		t.Errorf("active sessions: got %d, want 1", len(sessions))
	}
}

func TestKillSessionEndpoint(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	defer s.httpServer.Close()

	fid := insertTestFeature(t, database, "kill", s)
	database.CreateTmuxSession(fid, "ideation", 0, "devteam-"+fid+"-ideation", "/tmp/x")

	req := httptest.NewRequest("POST", "/api/features/"+fid+"/sessions/ideation/kill", nil)
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Verify session state is expired in DB
	sess, _ := database.GetTmuxSession(fid, "ideation", 0)
	if sess == nil {
		t.Fatal("session record missing")
	}
	if sess.State != "expired" {
		t.Errorf("session state: got %s, want expired", sess.State)
	}
}

func TestGetSessionOutputEndpoint(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	defer s.httpServer.Close()

	fid := insertTestFeature(t, database, "output", s)
	database.CreateTmuxSession(fid, "ideation", 0, "devteam-"+fid+"-ideation", "/tmp/x")

	req := httptest.NewRequest("GET", "/api/features/"+fid+"/sessions/ideation/output", nil)
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	// Session is not alive (no real tmux), so should return 204 or empty
	if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 200 or 204", w.Code)
	}
}

func TestGetCapturePaneEndpoint(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	defer s.httpServer.Close()

	fid := insertTestFeature(t, database, "pane", s)
	database.CreateTmuxSession(fid, "ideation", 0, "devteam-"+fid+"-ideation", "/tmp/x")

	req := httptest.NewRequest("GET", "/api/features/"+fid+"/sessions/ideation/pane", nil)
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	// Session not alive — should return 404
	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404 (session not alive)", w.Code)
	}
}