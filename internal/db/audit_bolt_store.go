package db

import (
	"encoding/json"
	"fmt"
	"time"
)

// AuditEvent is one entry in the 68-event audit trail.
type AuditEvent struct {
	ID         int64     `json:"id"`
	FeatureID  string    `json:"feature_id"`
	EventType  string    `json:"event_type"`
	StageID    string    `json:"stage_id,omitempty"`
	Phase      string    `json:"phase,omitempty"`
	Details    string    `json:"details,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// RecordAuditEvent inserts an audit event.
func (db *DB) RecordAuditEvent(featureID, eventType, stageID, phase, details string) error {
	_, err := db.Exec(
		`INSERT INTO audit_events (feature_id, event_type, stage_id, phase, details, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		featureID, eventType, stageID, phase, details, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("recording audit event %s: %w", eventType, err)
	}
	return nil
}

// GetAuditEvents returns the full audit trail for a feature, chronological.
func (db *DB) GetAuditEvents(featureID string) ([]AuditEvent, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, event_type, stage_id, phase, details, created_at
		 FROM audit_events WHERE feature_id = ? ORDER BY created_at ASC, id ASC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting audit events for %s: %w", featureID, err)
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.FeatureID, &e.EventType, &e.StageID, &e.Phase, &e.Details, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning audit event: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}

// GetAuditEventsForStage returns events for a specific stage of a feature.
func (db *DB) GetAuditEventsForStage(featureID, stageID string) ([]AuditEvent, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, event_type, stage_id, phase, details, created_at
		 FROM audit_events WHERE feature_id = ? AND stage_id = ? ORDER BY created_at ASC, id ASC`,
		featureID, stageID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting audit events for %s/%s: %w", featureID, stageID, err)
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.FeatureID, &e.EventType, &e.StageID, &e.Phase, &e.Details, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning audit event: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}

// BoltRow is a construction Bolt record.
type BoltRow struct {
	ID                int64     `json:"id"`
	FeatureID         string    `json:"feature_id"`
	BoltNumber        int       `json:"bolt_number"`
	UnitIDs           []string  `json:"unit_ids"`
	Status            string    `json:"status"`         // pending, in_progress, completed, failed
	IsWalkingSkeleton bool      `json:"is_walking_skeleton"`
	CreatedAt         time.Time `json:"created_at"`
}

// CreateBolt inserts a Bolt record.
func (db *DB) CreateBolt(featureID string, boltNumber int, unitIDs []string, isWalkingSkeleton bool) error {
	units, _ := json.Marshal(unitIDs)
	ws := 0
	if isWalkingSkeleton {
		ws = 1
	}
	_, err := db.Exec(
		`INSERT INTO bolts (feature_id, bolt_number, unit_ids, status, is_walking_skeleton, created_at) VALUES (?, ?, ?, 'pending', ?, ?)`,
		featureID, boltNumber, string(units), ws, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("creating bolt %d for %s: %w", boltNumber, featureID, err)
	}
	return nil
}

// GetBolts returns all Bolts for a feature ordered by bolt_number.
func (db *DB) GetBolts(featureID string) ([]BoltRow, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, bolt_number, unit_ids, status, is_walking_skeleton, created_at
		 FROM bolts WHERE feature_id = ? ORDER BY bolt_number ASC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting bolts for %s: %w", featureID, err)
	}
	defer rows.Close()

	var bolts []BoltRow
	for rows.Next() {
		var b BoltRow
		var unitIDs string
		var wsInt int
		if err := rows.Scan(&b.ID, &b.FeatureID, &b.BoltNumber, &unitIDs, &b.Status, &wsInt, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning bolt: %w", err)
		}
		json.Unmarshal([]byte(unitIDs), &b.UnitIDs)
		b.IsWalkingSkeleton = wsInt == 1
		bolts = append(bolts, b)
	}
	return bolts, nil
}

// UpdateBoltStatus updates the status of a Bolt.
func (db *DB) UpdateBoltStatus(featureID string, boltNumber int, status string) error {
	_, err := db.Exec(
		`UPDATE bolts SET status = ? WHERE feature_id = ? AND bolt_number = ?`,
		status, featureID, boltNumber,
	)
	if err != nil {
		return fmt.Errorf("updating bolt %d for %s: %w", boltNumber, featureID, err)
	}
	return nil
}