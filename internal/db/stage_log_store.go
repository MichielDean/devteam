package db

import (
	"fmt"
	"time"
)

// SaveStageLog upserts the agent output log for a specific stage.
// Called after the tmux dispatch completes to persist the full output.
func (db *DB) SaveStageLog(featureID, stageID, agentRole, content string) error {
	now := time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO stage_logs (feature_id, stage_id, agent_role, content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT (feature_id, stage_id) DO UPDATE SET content = EXCLUDED.content, agent_role = EXCLUDED.agent_role, updated_at = EXCLUDED.updated_at`,
		featureID, stageID, agentRole, content, now, now,
	)
	if err != nil {
		return fmt.Errorf("saving stage log: %w", err)
	}
	return nil
}

// AppendStageLog appends content to an existing stage log (or creates it).
func (db *DB) AppendStageLog(featureID, stageID, agentRole, content string) error {
	now := time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO stage_logs (feature_id, stage_id, agent_role, content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT (feature_id, stage_id) DO UPDATE SET content = stage_logs.content || EXCLUDED.content, updated_at = EXCLUDED.updated_at`,
		featureID, stageID, agentRole, content, now, now,
	)
	if err != nil {
		return fmt.Errorf("appending stage log: %w", err)
	}
	return nil
}

// GetStageLog retrieves the agent output log for a specific stage.
func (db *DB) GetStageLog(featureID, stageID string) (string, error) {
	var content string
	err := db.QueryRow(
		`SELECT content FROM stage_logs WHERE feature_id = ? AND stage_id = ?`,
		featureID, stageID,
	).Scan(&content)
	if err != nil {
		return "", err
	}
	return content, nil
}

// GetStageLogMeta returns metadata about stage logs for a feature.
func (db *DB) GetStageLogMeta(featureID string) ([]struct {
	StageID   string    `json:"stage_id"`
	AgentRole string    `json:"agent_role"`
	Size      int       `json:"size"`
	UpdatedAt time.Time `json:"updated_at"`
}, error) {
	rows, err := db.Query(
		`SELECT stage_id, agent_role, length(content), updated_at
		 FROM stage_logs WHERE feature_id = ? ORDER BY stage_id`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying stage logs: %w", err)
	}
	defer rows.Close()

	result := []struct {
		StageID   string    `json:"stage_id"`
		AgentRole string    `json:"agent_role"`
		Size      int       `json:"size"`
		UpdatedAt time.Time `json:"updated_at"`
	}{}
	for rows.Next() {
		var r struct {
			StageID   string    `json:"stage_id"`
			AgentRole string    `json:"agent_role"`
			Size      int       `json:"size"`
			UpdatedAt time.Time `json:"updated_at"`
		}
		if err := rows.Scan(&r.StageID, &r.AgentRole, &r.Size, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning stage log: %w", err)
		}
		result = append(result, r)
	}
	return result, nil
}