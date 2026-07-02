package db

import (
	"fmt"
	"time"
)

// TeamKnowledgeRow is a per-agent team knowledge entry.
type TeamKnowledgeRow struct {
	ID        int64     `json:"id"`
	AgentName string    `json:"agent_name"`
	Topic     string    `json:"topic"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SaveTeamKnowledge inserts or updates team knowledge for an agent+topic.
func (db *DB) SaveTeamKnowledge(agentName, topic, content string) error {
	now := time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO team_knowledge (agent_name, topic, content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(agent_name, topic) DO UPDATE SET content = excluded.content, updated_at = excluded.updated_at`,
		agentName, topic, content, now, now,
	)
	if err != nil {
		return fmt.Errorf("saving team knowledge %s/%s: %w", agentName, topic, err)
	}
	return nil
}

// GetTeamKnowledge returns all team knowledge entries for an agent.
func (db *DB) GetTeamKnowledge(agentName string) ([]TeamKnowledgeRow, error) {
	rows, err := db.Query(
		`SELECT id, agent_name, topic, content, created_at, updated_at
		 FROM team_knowledge WHERE agent_name = ? ORDER BY topic ASC`,
		agentName,
	)
	if err != nil {
		return nil, fmt.Errorf("getting team knowledge for %s: %w", agentName, err)
	}
	defer rows.Close()

	var entries []TeamKnowledgeRow
	for rows.Next() {
		var e TeamKnowledgeRow
		if err := rows.Scan(&e.ID, &e.AgentName, &e.Topic, &e.Content, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning team knowledge: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// DeleteTeamKnowledge removes a team knowledge entry.
func (db *DB) DeleteTeamKnowledge(agentName, topic string) error {
	_, err := db.Exec(`DELETE FROM team_knowledge WHERE agent_name = ? AND topic = ?`, agentName, topic)
	if err != nil {
		return fmt.Errorf("deleting team knowledge %s/%s: %w", agentName, topic, err)
	}
	return nil
}

// RuleRow is a learned behavioral rule from the learning loop.
type RuleRow struct {
	ID              int64     `json:"id"`
	FeatureID       string    `json:"feature_id"`
	AgentName       string    `json:"agent_name"`
	StageID         string    `json:"stage_id"`
	RuleText        string    `json:"rule_text"`
	SourceRejection string    `json:"source_rejection"`
	CreatedAt       time.Time `json:"created_at"`
}

// SaveRule inserts a learned rule.
func (db *DB) SaveRule(featureID, agentName, stageID, ruleText, sourceRejection string) error {
	_, err := db.Exec(
		`INSERT INTO rules (feature_id, agent_name, stage_id, rule_text, source_rejection, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		featureID, agentName, stageID, ruleText, sourceRejection, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("saving rule: %w", err)
	}
	return nil
}

// GetRulesForAgent returns rules applicable to an agent (global + feature-specific).
func (db *DB) GetRulesForAgent(agentName, featureID string) ([]RuleRow, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, agent_name, stage_id, rule_text, source_rejection, created_at
		 FROM rules WHERE agent_name = ? AND (feature_id = '' OR feature_id = ?)
		 ORDER BY created_at ASC`,
		agentName, featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting rules for %s: %w", agentName, err)
	}
	defer rows.Close()

	var rules []RuleRow
	for rows.Next() {
		var r RuleRow
		if err := rows.Scan(&r.ID, &r.FeatureID, &r.AgentName, &r.StageID, &r.RuleText, &r.SourceRejection, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// GetRulesForFeature returns all rules for a feature.
func (db *DB) GetRulesForFeature(featureID string) ([]RuleRow, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, agent_name, stage_id, rule_text, source_rejection, created_at
		 FROM rules WHERE feature_id = ? OR feature_id = '' ORDER BY created_at ASC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting rules for feature %s: %w", featureID, err)
	}
	defer rows.Close()

	var rules []RuleRow
	for rows.Next() {
		var r RuleRow
		if err := rows.Scan(&r.ID, &r.FeatureID, &r.AgentName, &r.StageID, &r.RuleText, &r.SourceRejection, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// DeleteRule removes a rule.
func (db *DB) DeleteRule(ruleID int64) error {
	_, err := db.Exec(`DELETE FROM rules WHERE id = ?`, ruleID)
	if err != nil {
		return fmt.Errorf("deleting rule %d: %w", ruleID, err)
	}
	return nil
}