package db

import (
	"database/sql"
	"fmt"
	"time"
)

// SessionRow is the database representation of an agent session.
type SessionRow struct {
	ID           int64      `json:"id"`
	FeatureID    string     `json:"feature_id"`
	Phase        string     `json:"phase"`
	Role         string     `json:"role"`
	TmuxSession  string     `json:"tmux_session"`
	DurationMs   int64      `json:"duration_ms"`
	OutputLength int        `json:"output_length"`
	Success      bool       `json:"success"`
	Error        string     `json:"error"`
	LogPath      string     `json:"log_path"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
}

// CreateSession records the start of an agent session.
func (db *DB) CreateSession(s SessionRow) (int64, error) {
	var id int64
	err := db.QueryRow(
		`INSERT INTO sessions (feature_id, phase, role, tmux_session, duration_ms, output_length, success, error, log_path, started_at, ended_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		s.FeatureID, s.Phase, s.Role, s.TmuxSession, s.DurationMs, s.OutputLength, boolToInt(s.Success), s.Error, s.LogPath, s.StartedAt, s.EndedAt,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("creating session: %w", err)
	}
	return id, nil
}

// CompleteSession records the end of an agent session with results.
func (db *DB) CompleteSession(id int64, durationMs int64, outputLength int, success bool, errorMsg, logPath string) error {
	now := time.Now().UTC()
	_, err := db.Exec(
		`UPDATE sessions SET duration_ms = ?, output_length = ?, success = ?, error = ?, log_path = ?, ended_at = ? WHERE id = ?`,
		durationMs, outputLength, boolToInt(success), errorMsg, logPath, now, id,
	)
	if err != nil {
		return fmt.Errorf("completing session: %w", err)
	}
	return nil
}

// GetSessions retrieves all sessions for a feature.
func (db *DB) GetSessions(featureID string) ([]SessionRow, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, phase, role, tmux_session, duration_ms, output_length, success, error, log_path, started_at, ended_at
		 FROM sessions WHERE feature_id = ? ORDER BY started_at ASC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SessionRow
	for rows.Next() {
		var s SessionRow
		var successInt int
		var endedAt sql.NullTime
		if err := rows.Scan(&s.ID, &s.FeatureID, &s.Phase, &s.Role, &s.TmuxSession, &s.DurationMs, &s.OutputLength, &successInt, &s.Error, &s.LogPath, &s.StartedAt, &endedAt); err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}
		s.Success = successInt == 1
		if endedAt.Valid {
			s.EndedAt = &endedAt.Time
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// GetSessionsForPhase retrieves sessions for a specific phase.
func (db *DB) GetSessionsForPhase(featureID, phase string) ([]SessionRow, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, phase, role, tmux_session, duration_ms, output_length, success, error, log_path, started_at, ended_at
		 FROM sessions WHERE feature_id = ? AND phase = ? ORDER BY started_at ASC`,
		featureID, phase,
	)
	if err != nil {
		return nil, fmt.Errorf("getting sessions for phase: %w", err)
	}
	defer rows.Close()

	var sessions []SessionRow
	for rows.Next() {
		var s SessionRow
		var successInt int
		var endedAt sql.NullTime
		if err := rows.Scan(&s.ID, &s.FeatureID, &s.Phase, &s.Role, &s.TmuxSession, &s.DurationMs, &s.OutputLength, &successInt, &s.Error, &s.LogPath, &s.StartedAt, &endedAt); err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}
		s.Success = successInt == 1
		if endedAt.Valid {
			s.EndedAt = &endedAt.Time
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// SessionMetrics holds aggregate metrics for agent sessions.
type SessionMetrics struct {
	TotalSessions  int     `json:"total_sessions"`
	SuccessRate    float64 `json:"success_rate"`
	AvgDurationMs  int64   `json:"avg_duration_ms"`
	TotalOutput    int     `json:"total_output"`
}

// GetSessionMetrics returns aggregate metrics across all sessions.
func (db *DB) GetSessionMetrics() (*SessionMetrics, error) {
	var m SessionMetrics
	err := db.QueryRow(
		`SELECT COUNT(*), AVG(success), AVG(duration_ms), SUM(output_length) FROM sessions WHERE ended_at IS NOT NULL`,
	).Scan(&m.TotalSessions, &m.SuccessRate, &m.AvgDurationMs, &m.TotalOutput)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("getting session metrics: %w", err)
	}
	return &m, nil
}