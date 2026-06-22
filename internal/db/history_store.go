package db

import (
	"database/sql"
	"fmt"
	"time"
)

// RecirculationRow tracks a phase being sent back for rework.
type RecirculationRow struct {
	ID             int64     `json:"id"`
	FeatureID      string    `json:"feature_id"`
	FromPhase      string    `json:"from_phase"`
	ToPhase        string    `json:"to_phase"`
	Reason         string    `json:"reason"`
	FailureDetails string    `json:"failure_details"`
	CreatedAt      time.Time `json:"created_at"`
}

// AddRecirculation records a recirculation event.
func (db *DB) AddRecirculation(featureID, fromPhase, toPhase, reason, failureDetails string) error {
	_, err := db.conn.Exec(
		`INSERT INTO recirculations (feature_id, from_phase, to_phase, reason, failure_details, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		featureID, fromPhase, toPhase, reason, failureDetails, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("adding recirculation: %w", err)
	}

	// Increment recirculation count on feature
	_, err = db.conn.Exec(
		`UPDATE features SET recirculation_count = recirculation_count + 1, updated_at = ? WHERE id = ?`,
		time.Now().UTC(), featureID,
	)
	if err != nil {
		return fmt.Errorf("incrementing recirculation count: %w", err)
	}
	return nil
}

// GetRecirculations retrieves all recirculations for a feature.
func (db *DB) GetRecirculations(featureID string) ([]RecirculationRow, error) {
	rows, err := db.conn.Query(
		`SELECT id, feature_id, from_phase, to_phase, reason, failure_details, created_at
		 FROM recirculations WHERE feature_id = ? ORDER BY created_at ASC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting recirculations: %w", err)
	}
	defer rows.Close()

	var recirculations []RecirculationRow
	for rows.Next() {
		var r RecirculationRow
		if err := rows.Scan(&r.ID, &r.FeatureID, &r.FromPhase, &r.ToPhase, &r.Reason, &r.FailureDetails, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning recirculation: %w", err)
		}
		recirculations = append(recirculations, r)
	}
	return recirculations, nil
}

// EventRow is an entry in the audit trail.
type EventRow struct {
	ID         int64     `json:"id"`
	FeatureID  string    `json:"feature_id"`
	EventType  string    `json:"event_type"`
	Phase      string    `json:"phase"`
	Details    string    `json:"details"`
	CreatedAt  time.Time `json:"created_at"`
}

// GetEvents retrieves all events for a feature.
func (db *DB) GetEvents(featureID string) ([]EventRow, error) {
	rows, err := db.conn.Query(
		`SELECT id, feature_id, event_type, phase, details, created_at
		 FROM events WHERE feature_id = ? ORDER BY created_at ASC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting events: %w", err)
	}
	defer rows.Close()

	var events []EventRow
	for rows.Next() {
		var e EventRow
		if err := rows.Scan(&e.ID, &e.FeatureID, &e.EventType, &e.Phase, &e.Details, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}

// ChurnMetrics holds metrics about phase churn (recirculations).
type ChurnMetrics struct {
	FeatureID         string `json:"feature_id"`
	TotalRecirculations int   `json:"total_recirculations"`
	ConstructionChurn  int   `json:"construction_churn"`
	ReviewChurn        int   `json:"review_churn"`
	TestingChurn       int   `json:"testing_churn"`
	DeliveryChurn      int   `json:"delivery_churn"`
}

// GetChurnMetrics returns churn metrics for a feature.
func (db *DB) GetChurnMetrics(featureID string) (*ChurnMetrics, error) {
	m := &ChurnMetrics{FeatureID: featureID}

	// Total
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM recirculations WHERE feature_id = ?`, featureID,
	).Scan(&m.TotalRecirculations)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("getting total churn: %w", err)
	}

	// Per-phase
	for _, phase := range []string{"construction", "review", "testing", "delivery"} {
		var count int
		err := db.conn.QueryRow(
			`SELECT COUNT(*) FROM recirculations WHERE feature_id = ? AND from_phase = ?`, featureID, phase,
		).Scan(&count)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("getting churn for %s: %w", phase, err)
		}
		switch phase {
		case "construction":
			m.ConstructionChurn = count
		case "review":
			m.ReviewChurn = count
		case "testing":
			m.TestingChurn = count
		case "delivery":
			m.DeliveryChurn = count
		}
	}

	return m, nil
}