package db

import (
	"encoding/json"
	"fmt"
	"time"
)

// StageDefinition is one of the 32 AIDLC v2 stages, seeded into the DB.
type StageDefinition struct {
	ID               string   `json:"id"`    // "1.1", "2.3", etc.
	Phase            string   `json:"phase"` // "ideation", "inception", etc.
	Name             string   `json:"name"`
	Description      string   `json:"description"` // human-readable purpose of this stage
	LeadAgent        string   `json:"lead_agent"`
	SupportingAgents []string `json:"supporting_agents"`
	KeyArtifacts     []string `json:"key_artifacts"`
	Condition        string   `json:"condition"` // ALWAYS, CONDITIONAL, BROWNFIELD, etc.
	Scopes           []string `json:"scopes"`    // which scopes execute this stage
	Reviewer         string   `json:"reviewer"`  // reviewer agent slug, or ""
	SortOrder        int      `json:"sort_order"`
}

// FeatureStage is the per-feature state for one stage.
// For construction stages 3.1-3.5, BoltNumber identifies which Bolt this row
// tracks (1, 2, 3, ...). For all other stages, BoltNumber is 0 (single row).
type FeatureStage struct {
	ID            int64      `json:"id"`
	FeatureID     string     `json:"feature_id"`
	StageID       string     `json:"stage_id"`
	BoltNumber    int        `json:"bolt_number"`
	Status        string     `json:"status"` // not_started, in_progress, awaiting_approval, revising, completed, skipped
	RevisionCount int        `json:"revision_count"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

// UpsertStageDefinition inserts or replaces a stage definition (seeding).
func (db *DB) UpsertStageDefinition(s StageDefinition) error {
	supporting, _ := json.Marshal(s.SupportingAgents)
	artifacts, _ := json.Marshal(s.KeyArtifacts)
	scopes, _ := json.Marshal(s.Scopes)
	_, err := db.Exec(
		`INSERT INTO stage_definitions (id, phase, name, description, lead_agent, supporting_agents, key_artifacts, condition, scopes, reviewer, sort_order)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   phase = excluded.phase, name = excluded.name, description = excluded.description, lead_agent = excluded.lead_agent,
		   supporting_agents = excluded.supporting_agents, key_artifacts = excluded.key_artifacts,
		   condition = excluded.condition, scopes = excluded.scopes, reviewer = excluded.reviewer,
		   sort_order = excluded.sort_order`,
		s.ID, s.Phase, s.Name, s.Description, s.LeadAgent, string(supporting), string(artifacts), s.Condition, string(scopes), s.Reviewer, s.SortOrder,
	)
	if err != nil {
		return fmt.Errorf("upserting stage definition %s: %w", s.ID, err)
	}
	return nil
}

// GetAllStageDefinitions returns all 32 stages ordered by sort_order.
func (db *DB) GetAllStageDefinitions() ([]StageDefinition, error) {
	rows, err := db.Query(
		`SELECT id, phase, name, description, lead_agent, supporting_agents, key_artifacts, condition, scopes, reviewer, sort_order
		 FROM stage_definitions ORDER BY sort_order ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("getting stage definitions: %w", err)
	}
	defer rows.Close()

	var stages []StageDefinition
	for rows.Next() {
		var s StageDefinition
		var supporting, artifacts, scopes string
		if err := rows.Scan(&s.ID, &s.Phase, &s.Name, &s.Description, &s.LeadAgent, &supporting, &artifacts, &s.Condition, &scopes, &s.Reviewer, &s.SortOrder); err != nil {
			return nil, fmt.Errorf("scanning stage definition: %w", err)
		}
		json.Unmarshal([]byte(supporting), &s.SupportingAgents)
		json.Unmarshal([]byte(artifacts), &s.KeyArtifacts)
		json.Unmarshal([]byte(scopes), &s.Scopes)
		stages = append(stages, s)
	}
	return stages, nil
}

// GetStageDefinition returns a single stage by ID.
func (db *DB) GetStageDefinition(id string) (*StageDefinition, error) {
	row := db.QueryRow(
		`SELECT id, phase, name, description, lead_agent, supporting_agents, key_artifacts, condition, scopes, reviewer, sort_order
		 FROM stage_definitions WHERE id = ?`, id,
	)
	var s StageDefinition
	var supporting, artifacts, scopes string
	err := row.Scan(&s.ID, &s.Phase, &s.Name, &s.Description, &s.LeadAgent, &supporting, &artifacts, &s.Condition, &scopes, &s.Reviewer, &s.SortOrder)
	if err != nil {
		return nil, fmt.Errorf("stage definition %s: %w", id, err)
	}
	json.Unmarshal([]byte(supporting), &s.SupportingAgents)
	json.Unmarshal([]byte(artifacts), &s.KeyArtifacts)
	json.Unmarshal([]byte(scopes), &s.Scopes)
	return &s, nil
}

// GetStagesForScope returns stages that execute for the given scope, ordered.
func (db *DB) GetStagesForScope(scope string) ([]StageDefinition, error) {
	all, err := db.GetAllStageDefinitions()
	if err != nil {
		return nil, err
	}
	var result []StageDefinition
	for _, s := range all {
		for _, sc := range s.Scopes {
			if sc == scope {
				result = append(result, s)
				break
			}
		}
	}
	return result, nil
}

// InitFeatureStages creates feature_stages rows for all NON-construction stages
// in the scope (bolt_number=0). Construction stages 3.1-3.5 are created per-Bolt
// by InitBoltStages. Idempotent — skips stages that already have rows.
func (db *DB) InitFeatureStages(featureID, scope string) error {
	stages, err := db.GetStagesForScope(scope)
	if err != nil {
		return fmt.Errorf("getting stages for scope %s: %w", scope, err)
	}
	for _, s := range stages {
		// Construction per-Bolt stages are created by InitBoltStages.
		// 3.6 and 3.7 run once, so they stay at bolt_number=0.
		if isPerBoltStage(s.ID) {
			continue
		}
		_, err := db.Exec(
			`INSERT INTO feature_stages (feature_id, stage_id, bolt_number, status) VALUES (?, ?, 0, 'not_started')
			 ON CONFLICT (feature_id, stage_id, bolt_number) DO NOTHING`,
			featureID, s.ID,
		)
		if err != nil {
			return fmt.Errorf("init feature stage %s/%s: %w", featureID, s.ID, err)
		}
	}
	return nil
}

// InitBoltStages creates feature_stages rows for construction stages 3.1-3.5
// for one Bolt (bolt_number = boltNum). Idempotent — skips rows that exist.
func (db *DB) InitBoltStages(featureID string, boltNum int, scope string) error {
	stages, err := db.GetStagesForScope(scope)
	if err != nil {
		return fmt.Errorf("getting stages for scope %s: %w", scope, err)
	}
	for _, s := range stages {
		if !isPerBoltStage(s.ID) {
			continue
		}
		_, err := db.Exec(
			`INSERT INTO feature_stages (feature_id, stage_id, bolt_number, status) VALUES (?, ?, ?, 'not_started')
			 ON CONFLICT (feature_id, stage_id, bolt_number) DO NOTHING`,
			featureID, s.ID, boltNum,
		)
		if err != nil {
			return fmt.Errorf("init bolt stage %s/%s bolt %d: %w", featureID, s.ID, boltNum, err)
		}
	}
	return nil
}

// isPerBoltStage reports whether a stage runs per-Bolt (3.1-3.5).
func isPerBoltStage(stageID string) bool {
	switch stageID {
	case "3.1", "3.2", "3.3", "3.4", "3.5":
		return true
	}
	return false
}

// GetFeatureStages returns all stage states for a feature, ordered by
// sort_order then bolt_number. Construction per-Bolt rows appear grouped
// under their stage in bolt order.
func (db *DB) GetFeatureStages(featureID string) ([]FeatureStage, error) {
	rows, err := db.Query(
		`SELECT fs.id, fs.feature_id, fs.stage_id, fs.bolt_number, fs.status, fs.revision_count, fs.started_at, fs.completed_at
		 FROM feature_stages fs
		 JOIN stage_definitions sd ON fs.stage_id = sd.id
		 WHERE fs.feature_id = ?
		 ORDER BY sd.sort_order ASC, fs.bolt_number ASC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting feature stages for %s: %w", featureID, err)
	}
	defer rows.Close()

	var stages []FeatureStage
	for rows.Next() {
		var fs FeatureStage
		var startedAt, completedAt *time.Time
		if err := rows.Scan(&fs.ID, &fs.FeatureID, &fs.StageID, &fs.BoltNumber, &fs.Status, &fs.RevisionCount, &startedAt, &completedAt); err != nil {
			return nil, fmt.Errorf("scanning feature stage: %w", err)
		}
		fs.StartedAt = startedAt
		fs.CompletedAt = completedAt
		stages = append(stages, fs)
	}
	return stages, nil
}

// GetFeatureStage returns the state for one stage of a feature at bolt_number=0
// (non-construction stages). For construction per-Bolt stages, use
// GetFeatureStageForBolt.
func (db *DB) GetFeatureStage(featureID, stageID string) (*FeatureStage, error) {
	row := db.QueryRow(
		`SELECT id, feature_id, stage_id, bolt_number, status, revision_count, started_at, completed_at
		 FROM feature_stages WHERE feature_id = ? AND stage_id = ? AND bolt_number = 0`,
		featureID, stageID,
	)
	var fs FeatureStage
	var startedAt, completedAt *time.Time
	err := row.Scan(&fs.ID, &fs.FeatureID, &fs.StageID, &fs.BoltNumber, &fs.Status, &fs.RevisionCount, &startedAt, &completedAt)
	if err != nil {
		return nil, nil // not found — not an error
	}
	fs.StartedAt = startedAt
	fs.CompletedAt = completedAt
	return &fs, nil
}

// GetFeatureStageForBolt returns the state for one stage of a feature at a
// specific bolt_number. Use bolt_number=0 for non-construction stages.
func (db *DB) GetFeatureStageForBolt(featureID, stageID string, boltNumber int) (*FeatureStage, error) {
	row := db.QueryRow(
		`SELECT id, feature_id, stage_id, bolt_number, status, revision_count, started_at, completed_at
		 FROM feature_stages WHERE feature_id = ? AND stage_id = ? AND bolt_number = ?`,
		featureID, stageID, boltNumber,
	)
	var fs FeatureStage
	var startedAt, completedAt *time.Time
	err := row.Scan(&fs.ID, &fs.FeatureID, &fs.StageID, &fs.BoltNumber, &fs.Status, &fs.RevisionCount, &startedAt, &completedAt)
	if err != nil {
		return nil, nil // not found — not an error
	}
	fs.StartedAt = startedAt
	fs.CompletedAt = completedAt
	return &fs, nil
}

// UpdateFeatureStage updates the state of one stage for a feature at
// bolt_number=0 (non-construction stages). For construction per-Bolt stages,
// use UpdateFeatureStageForBolt.
func (db *DB) UpdateFeatureStage(featureID, stageID, status string, revisionCount int, startedAt, completedAt *time.Time) error {
	_, err := db.Exec(
		`UPDATE feature_stages SET status = ?, revision_count = ?, started_at = ?, completed_at = ?
		 WHERE feature_id = ? AND stage_id = ? AND bolt_number = 0`,
		status, revisionCount, startedAt, completedAt, featureID, stageID,
	)
	if err != nil {
		return fmt.Errorf("updating feature stage %s/%s: %w", featureID, stageID, err)
	}
	return nil
}

// UpdateFeatureStageForBolt updates the state of one stage for a feature at a
// specific bolt_number. Use bolt_number=0 for non-construction stages.
func (db *DB) UpdateFeatureStageForBolt(featureID, stageID string, boltNumber int, status string, revisionCount int, startedAt, completedAt *time.Time) error {
	_, err := db.Exec(
		`UPDATE feature_stages SET status = ?, revision_count = ?, started_at = ?, completed_at = ?
		 WHERE feature_id = ? AND stage_id = ? AND bolt_number = ?`,
		status, revisionCount, startedAt, completedAt, featureID, stageID, boltNumber,
	)
	if err != nil {
		return fmt.Errorf("updating feature stage %s/%s bolt %d: %w", featureID, stageID, boltNumber, err)
	}
	return nil
}
