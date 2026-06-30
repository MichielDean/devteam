package db

import (
	"fmt"
	"time"
)

// OutcomeRow is a recorded agent outcome signal.
type OutcomeRow struct {
	ID        int64     `json:"id"`
	FeatureID string    `json:"feature_id"`
	Phase     string    `json:"phase"`
	Outcome   string    `json:"outcome"`
	Target    string    `json:"target"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}

// SaveOutcome inserts an outcome signal for a feature/phase.
// Called by the signal API when the agent runs `devteam signal <id> <outcome>`.
func (db *DB) SaveOutcome(featureID, phase, outcome, target, notes string) error {
	_, err := db.Exec(
		`INSERT INTO outcomes (feature_id, phase, outcome, target, notes, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		featureID, phase, outcome, target, notes, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("saving outcome: %w", err)
	}
	return nil
}

// GetLatestOutcome returns the most recent outcome for a feature/phase.
// Returns nil with no error if no outcome exists yet.
func (db *DB) GetLatestOutcome(featureID, phase string) (*OutcomeRow, error) {
	row := db.QueryRow(
		`SELECT id, feature_id, phase, outcome, target, notes, created_at
		 FROM outcomes WHERE feature_id = ? AND phase = ?
		 ORDER BY created_at DESC LIMIT 1`,
		featureID, phase,
	)
	var o OutcomeRow
	err := row.Scan(&o.ID, &o.FeatureID, &o.Phase, &o.Outcome, &o.Target, &o.Notes, &o.CreatedAt)
	if err != nil {
		return nil, nil // no outcome yet — not an error
	}
	return &o, nil
}

// DeleteOutcomesForPhase removes all outcomes for a feature/phase.
// Called after the pipeline has consumed the outcome so a re-run starts clean.
func (db *DB) DeleteOutcomesForPhase(featureID, phase string) error {
	_, err := db.Exec(
		`DELETE FROM outcomes WHERE feature_id = ? AND phase = ?`,
		featureID, phase,
	)
	if err != nil {
		return fmt.Errorf("deleting outcomes for %s/%s: %w", featureID, phase, err)
	}
	return nil
}