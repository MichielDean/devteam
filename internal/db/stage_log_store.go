package db

import (
	"fmt"
	"strings"
	"time"
)

// sanitizeForPostgres removes bytes that cause Postgres UTF-8 encoding errors
// (notably null bytes 0x00 which tmux/terminal output can contain).
func sanitizeForPostgres(s string) string {
	return strings.ReplaceAll(s, "\x00", "")
}

// SaveStageLog upserts the agent output log for a specific stage at
// bolt_number=0 (non-construction stages). For per-Bolt construction stages
// 3.1-3.5, use SaveStageLogForBolt.
func (db *DB) SaveStageLog(featureID, stageID, agentRole, content string) error {
	return db.SaveStageLogForBolt(featureID, stageID, 0, agentRole, content)
}

// SaveStageLogForBolt upserts the agent output log for a specific stage at a
// specific bolt_number. Use bolt_number=0 for non-construction stages.
// Null bytes and other invalid UTF-8 sequences are stripped to prevent
// Postgres encoding errors (pq: invalid byte sequence for encoding "UTF8").
func (db *DB) SaveStageLogForBolt(featureID, stageID string, boltNumber int, agentRole, content string) error {
	now := time.Now().UTC()
	content = sanitizeForPostgres(content)
	_, err := db.Exec(
		`INSERT INTO stage_logs (feature_id, stage_id, bolt_number, agent_role, content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (feature_id, stage_id, bolt_number) DO UPDATE SET content = EXCLUDED.content, agent_role = EXCLUDED.agent_role, updated_at = EXCLUDED.updated_at`,
		featureID, stageID, boltNumber, agentRole, content, now, now,
	)
	if err != nil {
		return fmt.Errorf("saving stage log: %w", err)
	}
	return nil
}

// AppendStageLog appends content to an existing stage log at bolt_number=0
// (or creates it). For per-Bolt stages, use AppendStageLogForBolt.
func (db *DB) AppendStageLog(featureID, stageID, agentRole, content string) error {
	return db.AppendStageLogForBolt(featureID, stageID, 0, agentRole, content)
}

// AppendStageLogForBolt appends content to an existing stage log at a specific
// bolt_number (or creates it). Use bolt_number=0 for non-construction stages.
func (db *DB) AppendStageLogForBolt(featureID, stageID string, boltNumber int, agentRole, content string) error {
	now := time.Now().UTC()
	content = sanitizeForPostgres(content)
	_, err := db.Exec(
		`INSERT INTO stage_logs (feature_id, stage_id, bolt_number, agent_role, content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (feature_id, stage_id, bolt_number) DO UPDATE SET content = stage_logs.content || EXCLUDED.content, updated_at = EXCLUDED.updated_at`,
		featureID, stageID, boltNumber, agentRole, content, now, now,
	)
	if err != nil {
		return fmt.Errorf("appending stage log: %w", err)
	}
	return nil
}

// GetStageLog retrieves the agent output log for a specific stage at
// bolt_number=0. For per-Bolt stages, use GetStageLogForBolt.
func (db *DB) GetStageLog(featureID, stageID string) (string, error) {
	return db.GetStageLogForBolt(featureID, stageID, 0)
}

// GetStageLogForBolt retrieves the agent output log for a specific stage at a
// specific bolt_number. Use bolt_number=0 for non-construction stages.
func (db *DB) GetStageLogForBolt(featureID, stageID string, boltNumber int) (string, error) {
	var content string
	err := db.QueryRow(
		`SELECT content FROM stage_logs WHERE feature_id = ? AND stage_id = ? AND bolt_number = ?`,
		featureID, stageID, boltNumber,
	).Scan(&content)
	if err != nil {
		return "", err
	}
	return content, nil
}

// StageLogMeta is metadata about a stage log entry.
type StageLogMeta struct {
	StageID    string    `json:"stage_id"`
	BoltNumber int       `json:"bolt_number"`
	AgentRole  string    `json:"agent_role"`
	Size       int       `json:"size"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// GetStageLogMeta returns metadata about stage logs for a feature, ordered by
// stage_id then bolt_number.
func (db *DB) GetStageLogMeta(featureID string) ([]StageLogMeta, error) {
	rows, err := db.Query(
		`SELECT stage_id, bolt_number, agent_role, length(content), updated_at
		 FROM stage_logs WHERE feature_id = ? ORDER BY stage_id, bolt_number`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying stage logs: %w", err)
	}
	defer rows.Close()

	result := []StageLogMeta{}
	for rows.Next() {
		var r StageLogMeta
		if err := rows.Scan(&r.StageID, &r.BoltNumber, &r.AgentRole, &r.Size, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning stage log: %w", err)
		}
		result = append(result, r)
	}
	return result, nil
}
