package pipeline

import (
	"fmt"
	"log"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/role"
)

// SessionManager manages the lifecycle of persistent tmux sessions.
// Sessions are scoped per feature+phase (or feature+construction-boltN).
// They persist across stage dispatches and are NOT killed when gates open
// or questions are asked.
type SessionManager struct {
	database   *db.DB
	dispatcher *role.Dispatcher
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(database *db.DB, dispatcher *role.Dispatcher) *SessionManager {
	return &SessionManager{
		database:   database,
		dispatcher: dispatcher,
	}
}

// ResolveOrCreateSession finds an existing session for a feature+phase,
// or creates a new one if none exists. Returns the session name and context dir.
// For construction Bolts, pass boltNumber > 0.
func (sm *SessionManager) ResolveOrCreateSession(featureID, phase string, boltNumber int) (sessionName, contextDir string, err error) {
	if sm == nil || sm.database == nil {
		return "", "", fmt.Errorf("session manager requires database")
	}

	tmuxMgr := sm.dispatcher.TmuxManager()

	if boltNumber > 0 && phase == "construction" {
		sessionName = tmuxMgr.SessionNameForBolt(featureID, boltNumber)
		contextDir = tmuxMgr.ContextDirForBolt(featureID, boltNumber)
	} else {
		sessionName = tmuxMgr.SessionNameForPhase(featureID, phase)
		contextDir = tmuxMgr.ContextDirForPhase(featureID, phase)
	}

	// Check if session record exists in DB
	existing, _ := sm.database.GetTmuxSession(featureID, phase, boltNumber)
	if existing != nil {
		log.Printf("SessionManager: found existing session %s (state=%s)", sessionName, existing.State)
		// Check if tmux session is actually alive
		if sm.dispatcher.IsSessionAliveByName(sessionName) {
			log.Printf("SessionManager: session %s is alive — reusing", sessionName)
			return sessionName, existing.ContextDir, nil
		}
		// Session record exists but tmux session is dead — update state to created and recreate tmux
		log.Printf("SessionManager: session %s record exists but tmux dead — recreating tmux", sessionName)
		sm.database.UpdateTmuxSessionState(featureID, phase, boltNumber, db.TmuxSessionCreated, "", "")
		return sessionName, contextDir, nil
	}

	// No DB record — create one
	if err := sm.database.CreateTmuxSession(featureID, phase, boltNumber, sessionName, contextDir); err != nil {
		log.Printf("SessionManager: CreateTmuxSession: %v (may already exist)", err)
	}

	return sessionName, contextDir, nil
}

// SetSessionRunning marks a session as running with the current stage and agent.
func (sm *SessionManager) SetSessionRunning(featureID, phase string, boltNumber int, stageID, agentRole string) error {
	if sm == nil || sm.database == nil {
		return nil
	}
	return sm.database.UpdateTmuxSessionState(featureID, phase, boltNumber, db.TmuxSessionRunning, stageID, agentRole)
}

// SetSessionAwaitingGate marks a session as paused awaiting gate approval.
func (sm *SessionManager) SetSessionAwaitingGate(featureID, phase string, boltNumber int, stageID string) error {
	if sm == nil || sm.database == nil {
		return nil
	}
	return sm.database.UpdateTmuxSessionState(featureID, phase, boltNumber, db.TmuxSessionAwaitingGate, stageID, "")
}

// SetSessionAwaitingQuestion marks a session as paused awaiting human answers.
func (sm *SessionManager) SetSessionAwaitingQuestion(featureID, phase string, boltNumber int, stageID string) error {
	if sm == nil || sm.database == nil {
		return nil
	}
	return sm.database.UpdateTmuxSessionState(featureID, phase, boltNumber, db.TmuxSessionAwaitingQuestion, stageID, "")
}

// SetSessionResuming marks a session as resuming (re-dispatch after question answers).
func (sm *SessionManager) SetSessionResuming(featureID, phase string, boltNumber int) error {
	if sm == nil || sm.database == nil {
		return nil
	}
	return sm.database.UpdateTmuxSessionState(featureID, phase, boltNumber, db.TmuxSessionResuming, "", "")
}

// SetSessionDone marks a session as done (phase complete).
func (sm *SessionManager) SetSessionDone(featureID, phase string, boltNumber int) error {
	if sm == nil || sm.database == nil {
		return nil
	}
	return sm.database.UpdateTmuxSessionState(featureID, phase, boltNumber, db.TmuxSessionDone, "", "")
}

// SetSessionFailed marks a session as failed.
func (sm *SessionManager) SetSessionFailed(featureID, phase string, boltNumber int, stageID string) error {
	if sm == nil || sm.database == nil {
		return nil
	}
	return sm.database.UpdateTmuxSessionState(featureID, phase, boltNumber, db.TmuxSessionFailed, stageID, "")
}

// KillSession kills the tmux session and marks it as expired in DB.
func (sm *SessionManager) KillSession(featureID, phase string, boltNumber int) error {
	if sm == nil {
		return nil
	}

	tmuxMgr := sm.dispatcher.TmuxManager()
	var sessionName string
	if boltNumber > 0 && phase == "construction" {
		sessionName = tmuxMgr.SessionNameForBolt(featureID, boltNumber)
	} else {
		sessionName = tmuxMgr.SessionNameForPhase(featureID, phase)
	}

	tmuxMgr.KillSession(sessionName)

	if sm.database != nil {
		sm.database.UpdateTmuxSessionState(featureID, phase, boltNumber, db.TmuxSessionExpired, "", "")
	}
	return nil
}

// ExpirePhaseSessions marks all sessions for a phase as expired.
func (sm *SessionManager) ExpirePhaseSessions(featureID, phase string) error {
	if sm == nil || sm.database == nil {
		return nil
	}
	return sm.database.ExpireTmuxSessionsForPhase(featureID, phase)
}

// CleanupFeatureSessions kills all tmux sessions and removes DB records for a feature.
func (sm *SessionManager) CleanupFeatureSessions(featureID string) error {
	if sm == nil {
		return nil
	}

	if sm.database != nil {
		sessions, _ := sm.database.ListTmuxSessionsForFeature(featureID)
		tmuxMgr := sm.dispatcher.TmuxManager()
		for _, s := range sessions {
			tmuxMgr.KillSession(s.SessionName)
		}
		sm.database.DeleteTmuxSessionsForFeature(featureID)
	}
	return nil
}

// GetSessionOutput returns the accumulated log output for a session.
// If stageID is provided, returns only that stage's log. Otherwise returns
// the raw capture-pane output.
func (sm *SessionManager) GetSessionOutput(featureID, phase string, boltNumber int, stageID string) (string, error) {
	if sm == nil {
		return "", nil
	}

	tmuxMgr := sm.dispatcher.TmuxManager()
	var sessionName, contextDir string
	if boltNumber > 0 && phase == "construction" {
		sessionName = tmuxMgr.SessionNameForBolt(featureID, boltNumber)
		contextDir = tmuxMgr.ContextDirForBolt(featureID, boltNumber)
	} else {
		sessionName = tmuxMgr.SessionNameForPhase(featureID, phase)
		contextDir = tmuxMgr.ContextDirForPhase(featureID, phase)
	}

	// If stage-specific, read from the log file
	if stageID != "" {
		_ = contextDir // used for log path resolution in future
		// Try to find the log file for this stage
		// We don't know the agent role here, so read the capture-pane instead
		// and let the UI filter. Or we can glob for stageID-*.log
	}

	// Fall back to capture-pane
	if sm.dispatcher.IsSessionAliveByName(sessionName) {
		return tmuxMgr.CapturePane(sessionName)
	}

	// Session not alive — try reading from log files
	// Read all log files in the context dir and concatenate
	return "", nil
}

// CapturePaneRaw returns raw ANSI output for the xterm.js pane viewer.
func (sm *SessionManager) CapturePaneRaw(featureID, phase string, boltNumber int) (string, error) {
	if sm == nil {
		return "", nil
	}

	tmuxMgr := sm.dispatcher.TmuxManager()
	var sessionName string
	if boltNumber > 0 && phase == "construction" {
		sessionName = tmuxMgr.SessionNameForBolt(featureID, boltNumber)
	} else {
		sessionName = tmuxMgr.SessionNameForPhase(featureID, phase)
	}

	if !sm.dispatcher.IsSessionAliveByName(sessionName) {
		return "", fmt.Errorf("session %s is not alive", sessionName)
	}

	return tmuxMgr.CapturePaneRaw(sessionName)
}

// ListSessionsForFeature returns all tmux sessions for a feature.
func (sm *SessionManager) ListSessionsForFeature(featureID string) ([]db.TmuxSessionRow, error) {
	if sm == nil || sm.database == nil {
		return nil, nil
	}
	return sm.database.ListTmuxSessionsForFeature(featureID)
}

// ListActiveSessions returns all active sessions across all features.
func (sm *SessionManager) ListActiveSessions() ([]db.TmuxSessionRow, error) {
	if sm == nil || sm.database == nil {
		return nil, nil
	}
	return sm.database.ListActiveTmuxSessions()
}