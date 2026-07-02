package db

import (
	"fmt"
	"time"
)

// GateCheckRow is a single check within a gate evaluation.
type GateCheckRow struct {
	FeatureID    string    `json:"feature_id"`
	Phase        string    `json:"phase"`
	CheckName    string    `json:"check_name"`
	Passed       bool      `json:"passed"`
	CheckMessage string    `json:"check_message"`
	EvaluatedAt  time.Time `json:"evaluated_at"`
}

// RecordGateResult records all checks from a gate evaluation.
// Each check is a separate row — enables querying "which checks fail most often".
func (db *DB) RecordGateResult(featureID, phase string, passed bool, checks []GateCheckRow) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	now := time.Now().UTC()
	for _, check := range checks {
		passedInt := 0
		if check.Passed {
			passedInt = 1
		}
		_, err := tx.Exec(
			`INSERT INTO gate_results (feature_id, phase, passed, check_name, check_passed, check_message, evaluated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			featureID, phase, boolToInt(passed), check.CheckName, passedInt, check.CheckMessage, now,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("inserting gate check: %w", err)
		}
	}

	// If no checks but overall pass/fail, record a single row
	if len(checks) == 0 {
		_, err := tx.Exec(
			`INSERT INTO gate_results (feature_id, phase, passed, check_name, check_passed, check_message, evaluated_at)
			 VALUES (?, ?, ?, 'overall', ?, '', ?)`,
			featureID, phase, boolToInt(passed), boolToInt(passed), now,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("inserting overall gate result: %w", err)
		}
	}

	return tx.Commit()
}

// GetGateHistory returns all gate evaluations for a feature, ordered by time.
func (db *DB) GetGateHistory(featureID string) ([]GateCheckRow, error) {
	rows, err := db.Query(
		`SELECT feature_id, phase, check_name, check_passed, check_message, evaluated_at
		 FROM gate_results WHERE feature_id = ? ORDER BY evaluated_at ASC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting gate history: %w", err)
	}
	defer rows.Close()

	results := []GateCheckRow{}
	for rows.Next() {
		var r GateCheckRow
		var passedInt int
		if err := rows.Scan(&r.FeatureID, &r.Phase, &r.CheckName, &passedInt, &r.CheckMessage, &r.EvaluatedAt); err != nil {
			return nil, fmt.Errorf("scanning gate check: %w", err)
		}
		r.Passed = passedInt == 1
		results = append(results, r)
	}
	return results, nil
}

// GetGateHistoryForPhase returns gate evaluations for a specific phase.
func (db *DB) GetGateHistoryForPhase(featureID, phase string) ([]GateCheckRow, error) {
	rows, err := db.Query(
		`SELECT feature_id, phase, check_name, check_passed, check_message, evaluated_at
		 FROM gate_results WHERE feature_id = ? AND phase = ? ORDER BY evaluated_at ASC`,
		featureID, phase,
	)
	if err != nil {
		return nil, fmt.Errorf("getting gate history for phase: %w", err)
	}
	defer rows.Close()

	results := []GateCheckRow{}
	for rows.Next() {
		var r GateCheckRow
		var passedInt int
		if err := rows.Scan(&r.FeatureID, &r.Phase, &r.CheckName, &passedInt, &r.CheckMessage, &r.EvaluatedAt); err != nil {
			return nil, fmt.Errorf("scanning gate check: %w", err)
		}
		r.Passed = passedInt == 1
		results = append(results, r)
	}
	return results, nil
}

// GetFailedChecks returns all failed checks across all features (for metrics).
func (db *DB) GetFailedChecks() ([]GateCheckRow, error) {
	rows, err := db.Query(
		`SELECT feature_id, phase, check_name, check_passed, check_message, evaluated_at
		 FROM gate_results WHERE check_passed = 0 ORDER BY evaluated_at DESC LIMIT 100`,
	)
	if err != nil {
		return nil, fmt.Errorf("getting failed checks: %w", err)
	}
	defer rows.Close()

	results := []GateCheckRow{}
	for rows.Next() {
		var r GateCheckRow
		var passedInt int
		if err := rows.Scan(&r.FeatureID, &r.Phase, &r.CheckName, &passedInt, &r.CheckMessage, &r.EvaluatedAt); err != nil {
			return nil, fmt.Errorf("scanning failed check: %w", err)
		}
		r.Passed = false
		results = append(results, r)
	}
	return results, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}