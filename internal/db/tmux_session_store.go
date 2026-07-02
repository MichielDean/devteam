package db

import (
	"fmt"
	"time"
)

// TmuxSessionRow represents a tmux session record in the DB.
type TmuxSessionRow struct {
	ID           int64      `json:"id"`
	FeatureID    string     `json:"feature_id"`
	Phase        string     `json:"phase"`
	BoltNumber   int        `json:"bolt_number"`
	StageID      string     `json:"stage_id"`
	SessionName  string     `json:"session_name"`
	State        string     `json:"state"`
	ContextDir   string     `json:"context_dir"`
	LastAgent    string     `json:"last_agent"`
	LastOutputAt *time.Time `json:"last_output_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Tmux session state constants
const (
	TmuxSessionCreated          = "created"
	TmuxSessionRunning          = "running"
	TmuxSessionAwaitingGate     = "awaiting_gate"
	TmuxSessionAwaitingQuestion = "awaiting_question"
	TmuxSessionResuming         = "resuming"
	TmuxSessionDone             = "done"
	TmuxSessionFailed           = "failed"
	TmuxSessionExpired          = "expired"
)

// CreateTmuxSession inserts a new tmux session record.
// Uses ON CONFLICT DO NOTHING to handle the case where a record already exists
// (e.g. the tmux session died and we're recreating it).
func (db *DB) CreateTmuxSession(featureID, phase string, boltNumber int, sessionName, contextDir string) error {
	now := time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO tmux_sessions (feature_id, phase, bolt_number, session_name, state, context_dir, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (session_name) DO NOTHING`,
		featureID, phase, boltNumber, sessionName, TmuxSessionCreated, contextDir, now, now,
	)
	if err != nil {
		return fmt.Errorf("creating tmux session: %w", err)
	}
	return nil
}

// GetTmuxSession retrieves a session by feature_id + phase + bolt_number.
func (db *DB) GetTmuxSession(featureID, phase string, boltNumber int) (*TmuxSessionRow, error) {
	row := db.QueryRow(
		`SELECT id, feature_id, phase, bolt_number, stage_id, session_name, state, context_dir,
		        last_agent, last_output_at, created_at, updated_at
		 FROM tmux_sessions WHERE feature_id = ? AND phase = ? AND bolt_number = ?`,
		featureID, phase, boltNumber,
	)
	return scanTmuxSession(row)
}

// GetTmuxSessionByName retrieves a session by its unique session name.
func (db *DB) GetTmuxSessionByName(sessionName string) (*TmuxSessionRow, error) {
	row := db.QueryRow(
		`SELECT id, feature_id, phase, bolt_number, stage_id, session_name, state, context_dir,
		        last_agent, last_output_at, created_at, updated_at
		 FROM tmux_sessions WHERE session_name = ?`,
		sessionName,
	)
	return scanTmuxSession(row)
}

// ListTmuxSessionsForFeature returns all tmux sessions for a feature.
func (db *DB) ListTmuxSessionsForFeature(featureID string) ([]TmuxSessionRow, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, phase, bolt_number, stage_id, session_name, state, context_dir,
		        last_agent, last_output_at, created_at, updated_at
		 FROM tmux_sessions WHERE feature_id = ? ORDER BY created_at`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying tmux sessions: %w", err)
	}
	defer rows.Close()

	var sessions []TmuxSessionRow
	for rows.Next() {
		s, err := scanTmuxSessionRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *s)
	}
	return sessions, nil
}

// ListActiveTmuxSessions returns all sessions across all features that are not done/failed/expired.
func (db *DB) ListActiveTmuxSessions() ([]TmuxSessionRow, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, phase, bolt_number, stage_id, session_name, state, context_dir,
		        last_agent, last_output_at, created_at, updated_at
		 FROM tmux_sessions WHERE state IN (?, ?, ?, ?) ORDER BY updated_at DESC`,
		TmuxSessionCreated, TmuxSessionRunning, TmuxSessionAwaitingGate, TmuxSessionAwaitingQuestion,
	)
	if err != nil {
		return nil, fmt.Errorf("querying active tmux sessions: %w", err)
	}
	defer rows.Close()

	var sessions []TmuxSessionRow
	for rows.Next() {
		s, err := scanTmuxSessionRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *s)
	}
	return sessions, nil
}

// UpdateTmuxSessionState updates the state, stage_id, and last_agent of a session.
func (db *DB) UpdateTmuxSessionState(featureID, phase string, boltNumber int, state, stageID, lastAgent string) error {
	now := time.Now().UTC()
	_, err := db.Exec(
		`UPDATE tmux_sessions SET state = ?, stage_id = ?, last_agent = ?, updated_at = ?,
		         last_output_at = ?
		 WHERE feature_id = ? AND phase = ? AND bolt_number = ?`,
		state, stageID, lastAgent, now, now, featureID, phase, boltNumber,
	)
	if err != nil {
		return fmt.Errorf("updating tmux session state: %w", err)
	}
	return nil
}

// UpdateTmuxSessionOutputTimestamp updates last_output_at for a session.
func (db *DB) UpdateTmuxSessionOutputTimestamp(sessionName string) error {
	now := time.Now().UTC()
	_, err := db.Exec(
		`UPDATE tmux_sessions SET last_output_at = ?, updated_at = ? WHERE session_name = ?`,
		now, now, sessionName,
	)
	if err != nil {
		return fmt.Errorf("updating tmux session output timestamp: %w", err)
	}
	return nil
}

// DeleteTmuxSessionsForFeature removes all session records for a feature.
func (db *DB) DeleteTmuxSessionsForFeature(featureID string) error {
	_, err := db.Exec(`DELETE FROM tmux_sessions WHERE feature_id = ?`, featureID)
	if err != nil {
		return fmt.Errorf("deleting tmux sessions: %w", err)
	}
	return nil
}

// ExpireTmuxSessionsForPhase marks sessions as expired when a phase is advanced past.
func (db *DB) ExpireTmuxSessionsForPhase(featureID, phase string) error {
	now := time.Now().UTC()
	_, err := db.Exec(
		`UPDATE tmux_sessions SET state = ?, updated_at = ?
		 WHERE feature_id = ? AND phase = ? AND state NOT IN (?, ?)`,
		TmuxSessionExpired, now, featureID, phase, TmuxSessionDone, TmuxSessionFailed,
	)
	if err != nil {
		return fmt.Errorf("expiring tmux sessions: %w", err)
	}
	return nil
}

// scanTmuxSession scans a single row into a TmuxSessionRow.
func scanTmuxSession(row interface {
	Scan(dest ...any) error
}) (*TmuxSessionRow, error) {
	var s TmuxSessionRow
	var lastOutputAt *time.Time
	err := row.Scan(
		&s.ID, &s.FeatureID, &s.Phase, &s.BoltNumber, &s.StageID, &s.SessionName,
		&s.State, &s.ContextDir, &s.LastAgent, &lastOutputAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	s.LastOutputAt = lastOutputAt
	return &s, nil
}

// scanTmuxSessionRows scans rows into a TmuxSessionRow (for use with *sql.Rows).
func scanTmuxSessionRows(rows interface {
	Scan(dest ...any) error
}) (*TmuxSessionRow, error) {
	return scanTmuxSession(rows)
}