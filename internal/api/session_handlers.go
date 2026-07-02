package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/MichielDean/devteam/internal/db"
)

// registerSessionRoutes adds the tmux session management API endpoints.
func (s *Server) registerSessionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/features/{id}/sessions", s.listSessions)
	mux.HandleFunc("POST /api/features/{id}/sessions/{phase}/resume", s.resumeSession)
	mux.HandleFunc("POST /api/features/{id}/sessions/{phase}/kill", s.killSession)
	mux.HandleFunc("GET /api/features/{id}/sessions/{phase}/output", s.getSessionOutput)
	mux.HandleFunc("GET /api/features/{id}/sessions/{phase}/pane", s.getCapturePane)
	mux.HandleFunc("GET /api/sessions/active", s.listActiveSessions)
}

// listSessions returns all tmux sessions for a feature.
func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	if s.db == nil {
		writeError(w, http.StatusInternalServerError, "no_database", "")
		return
	}

	sessions, err := s.db.ListTmuxSessionsForFeature(featureID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query_failed", err.Error())
		return
	}

	// Enrich with alive status from tmux
	type sessionWithAlive struct {
		db.TmuxSessionRow
		IsAlive bool `json:"is_alive"`
	}
	result := make([]sessionWithAlive, 0, len(sessions))
	for _, sess := range sessions {
		alive := s.pipeline.Dispatcher().IsSessionAliveByName(sess.SessionName)
		result = append(result, sessionWithAlive{TmuxSessionRow: sess, IsAlive: alive})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// resumeSession resumes a paused session by re-dispatching the current stage
// with fresh DB context (including answered questions).
func (s *Server) resumeSession(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	phase := r.PathValue("phase")
	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", err.Error())
		return
	}

	// Find the current stage for this feature
	if s.db == nil {
		writeError(w, http.StatusInternalServerError, "no_database", "")
		return
	}

	// Get the session record to find which stage to resume
	boltNumber := 0
	if f.CurrentBolt > 0 && phase == "construction" {
		boltNumber = f.CurrentBolt
	}

	sess, err := s.db.GetTmuxSession(featureID, phase, boltNumber)
	if err != nil || sess == nil {
		writeError(w, http.StatusNotFound, "session_not_found", "No session for this phase")
		return
	}

	stageID := sess.StageID
	if stageID == "" {
		stageID = f.CurrentStage
	}

	// Mark session as resuming
	s.db.UpdateTmuxSessionState(featureID, phase, boltNumber, db.TmuxSessionResuming, stageID, "")

	// Re-dispatch the stage (this regenerates CONTEXT.md from DB with answers included)
	if s.isFeatureActive(featureID) {
		writeError(w, http.StatusConflict, "conflict", "feature already running")
		return
	}

	s.markFeatureActive(featureID)
	defer s.unmarkFeatureActive(featureID)

	result, err := s.pipeline.RunStage(r.Context(), f, stageID, func(line string, isStderr bool) {
		s.broadcastSSE(featureID, "agent_output", fmt.Sprintf(`{"line":%s,"stderr":%v}`, jsonString(line), isStderr))
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "resume_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// killSession kills a tmux session and marks it as expired.
func (s *Server) killSession(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	phase := r.PathValue("phase")

	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", err.Error())
		return
	}

	boltNumber := 0
	if f.CurrentBolt > 0 && phase == "construction" {
		boltNumber = f.CurrentBolt
	}

	if s.pipeline.SessionMgr() != nil {
		if err := s.pipeline.SessionMgr().KillSession(featureID, phase, boltNumber); err != nil {
			writeError(w, http.StatusInternalServerError, "kill_failed", err.Error())
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"killed"}`))
}

// getSessionOutput returns the output for a session (from capture-pane or log files).
func (s *Server) getSessionOutput(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	phase := r.PathValue("phase")
	stageID := r.URL.Query().Get("stage_id")

	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", err.Error())
		return
	}

	boltNumber := 0
	if f.CurrentBolt > 0 && phase == "construction" {
		boltNumber = f.CurrentBolt
	}

	if s.pipeline.SessionMgr() != nil {
		output, err := s.pipeline.SessionMgr().GetSessionOutput(featureID, phase, boltNumber, stageID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "output_failed", err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(output))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// getCapturePane returns raw ANSI tmux capture-pane output for the xterm.js viewer.
func (s *Server) getCapturePane(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	phase := r.PathValue("phase")

	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", err.Error())
		return
	}

	boltNumber := 0
	if f.CurrentBolt > 0 && phase == "construction" {
		boltNumber = f.CurrentBolt
	}

	if s.pipeline.SessionMgr() != nil {
		output, err := s.pipeline.SessionMgr().CapturePaneRaw(featureID, phase, boltNumber)
		if err != nil {
			writeError(w, http.StatusNotFound, "session_not_alive", err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(output))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// listActiveSessions returns all active tmux sessions across all features.
func (s *Server) listActiveSessions(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeError(w, http.StatusInternalServerError, "no_database", "")
		return
	}

	sessions, err := s.db.ListActiveTmuxSessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query_failed", err.Error())
		return
	}

	// Enrich with alive status
	type sessionWithAlive struct {
		db.TmuxSessionRow
		IsAlive bool `json:"is_alive"`
	}
	result := make([]sessionWithAlive, 0, len(sessions))
	for _, sess := range sessions {
		alive := s.pipeline.Dispatcher().IsSessionAliveByName(sess.SessionName)
		result = append(result, sessionWithAlive{TmuxSessionRow: sess, IsAlive: alive})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}