package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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
	Actor      string    `json:"actor,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// RecordAuditEvent inserts an audit event. The actor column is left NULL
// (legacy behavior — FR-AUDIT-ACTOR-03 backward-compat). New config-mutation
// call sites should use RecordAuditEventWithActor to populate the operator
// identity (ADR-AUDIT-ACTOR-IDENTITY).
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

// RecordAuditEventWithActor inserts an audit event with the operator identity
// in the actor column (added in migration 018 — ADR-AUDIT-ACTOR). An empty
// actor string inserts NULL, preserving backward compatibility with legacy
// rows (FR-AUDIT-ACTOR-03). Config-mutation handlers (repos, defaults, server)
// pass the operator identity here; the single-operator v1 default is
// "operator" (configurable via DEVTEAM_OPERATOR_NAME — ADR-AUDIT-ACTOR-IDENTITY).
func (db *DB) RecordAuditEventWithActor(featureID, eventType, stageID, phase, details, actor string) error {
	var actorArg interface{}
	if actor == "" {
		actorArg = nil
	} else {
		actorArg = actor
	}
	_, err := db.Exec(
		`INSERT INTO audit_events (feature_id, event_type, stage_id, phase, details, actor, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		featureID, eventType, stageID, phase, details, actorArg, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("recording audit event %s: %w", eventType, err)
	}
	return nil
}

// GetAuditEvents returns the full audit trail for a feature, chronological.
func (db *DB) GetAuditEvents(featureID string) ([]AuditEvent, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, event_type, stage_id, phase, details, actor, created_at
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
		var actor sql.NullString
		if err := rows.Scan(&e.ID, &e.FeatureID, &e.EventType, &e.StageID, &e.Phase, &e.Details, &actor, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning audit event: %w", err)
		}
		e.Actor = actor.String
		events = append(events, e)
	}
	return events, nil
}

// GetAuditEventsForStage returns events for a specific stage of a feature.
func (db *DB) GetAuditEventsForStage(featureID, stageID string) ([]AuditEvent, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, event_type, stage_id, phase, details, actor, created_at
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
		var actor sql.NullString
		if err := rows.Scan(&e.ID, &e.FeatureID, &e.EventType, &e.StageID, &e.Phase, &e.Details, &actor, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning audit event: %w", err)
		}
		e.Actor = actor.String
		events = append(events, e)
	}
	return events, nil
}

// AuditFilter is the filter parameter for GetAuditEventsFiltered — the
// cross-feature audit read backing the Audit tab (FR-AUDIT-01). Zero values
// mean "no filter on this field." EventType is a single event type; pass
// comma-separated values in EventType to match any of them. Page is 1-based;
// PageSize defaults to 50 and is capped at 200 (FR-AUDIT-02).
type AuditFilter struct {
	EventType string    // single type or comma-list; empty = all
	FeatureID string    // empty = all features
	Actor     string    // empty = all actors
	From      time.Time // zero = no lower bound
	To        time.Time // zero = no upper bound
	Page      int
	PageSize  int
}

// GetAuditEventsFiltered returns a page of audit events matching the filter,
// backed by idx_audit_events_type_time (created in migration 018). The query
// is the cross-feature audit read (FR-AUDIT-01); the index prevents a seq scan
// when filtering by event_type + time range across the whole table
// (R-AUDIT-VOLUME). total is the count of matching rows ignoring pagination.
//
// Read-only, unguarded (FR-ROUTE-02) — the Audit tab is read-only.
func (db *DB) GetAuditEventsFiltered(filter AuditFilter) ([]AuditEvent, int, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 50
	}
	if filter.PageSize > 200 {
		filter.PageSize = 200
	}

	// Build the WHERE clause. eventType may be a comma-list — expand to IN.
	var (
		conds  []string
		args   []interface{}
	)
	if filter.EventType != "" {
		types := splitCSV(filter.EventType)
		if len(types) == 1 {
			conds = append(conds, "event_type = ?")
			args = append(args, types[0])
		} else {
			placeholders := make([]string, len(types))
			for i, t := range types {
				placeholders[i] = "?"
				args = append(args, t)
			}
			conds = append(conds, "event_type IN ("+joinStrings(placeholders, ", ")+")")
		}
	}
	if filter.FeatureID != "" {
		conds = append(conds, "feature_id = ?")
		args = append(args, filter.FeatureID)
	}
	if filter.Actor != "" {
		conds = append(conds, "actor = ?")
		args = append(args, filter.Actor)
	}
	if !filter.From.IsZero() {
		conds = append(conds, "created_at >= ?")
		args = append(args, filter.From)
	}
	if !filter.To.IsZero() {
		conds = append(conds, "created_at <= ?")
		args = append(args, filter.To)
	}

	where := ""
	if len(conds) > 0 {
		where = " WHERE " + joinStrings(conds, " AND ")
	}

	// Count query (no pagination).
	var total int
	countErr := db.QueryRow("SELECT COUNT(*) FROM audit_events"+where, args...).Scan(&total)
	if countErr != nil {
		return nil, 0, fmt.Errorf("counting filtered audit events: %w", countErr)
	}

	// Page query — order by created_at desc so the newest events surface first.
	offset := (filter.Page - 1) * filter.PageSize
	pageArgs := append(append([]interface{}{}, args...), filter.PageSize, offset)
	rows, err := db.Query(
		"SELECT id, feature_id, event_type, stage_id, phase, details, actor, created_at FROM audit_events"+where+
			" ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?",
		pageArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("querying filtered audit events: %w", err)
	}
	defer rows.Close()

	events := []AuditEvent{}
	for rows.Next() {
		var e AuditEvent
		var actor sql.NullString
		if err := rows.Scan(&e.ID, &e.FeatureID, &e.EventType, &e.StageID, &e.Phase, &e.Details, &actor, &e.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scanning audit event: %w", err)
		}
		e.Actor = actor.String
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating audit events: %w", err)
	}
	return events, total, nil
}

// splitCSV splits a comma-separated value list, trimming whitespace and
// dropping empties. "a, b ,, c" -> ["a","b","c"].
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// joinStrings joins a slice of strings with a separator. Avoids pulling in
// strings.Join for a single use site; kept local and unexported.
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += sep + p
	}
	return out
}

// BoltRow is a construction Bolt record.
type BoltRow struct {
	ID                int64     `json:"id"`
	FeatureID         string    `json:"feature_id"`
	BoltNumber        int       `json:"bolt_number"`
	UnitIDs           []string  `json:"unit_ids"`
	DependsOn         []int     `json:"depends_on"` // bolt numbers this Bolt depends on (empty = ready)
	Status            string    `json:"status"`     // pending, in_progress, completed, failed
	IsWalkingSkeleton bool      `json:"is_walking_skeleton"`
	CreatedAt         time.Time `json:"created_at"`
}

// CreateBolt inserts a Bolt record. dependsOn is the list of bolt numbers this
// Bolt depends on (empty for the walking skeleton or ready-to-run bolts).
func (db *DB) CreateBolt(featureID string, boltNumber int, unitIDs []string, dependsOn []int, isWalkingSkeleton bool) error {
	units, _ := json.Marshal(unitIDs)
	deps, _ := json.Marshal(dependsOn)
	ws := 0
	if isWalkingSkeleton {
		ws = 1
	}
	_, err := db.Exec(
		`INSERT INTO bolts (feature_id, bolt_number, unit_ids, depends_on, status, is_walking_skeleton, created_at) VALUES (?, ?, ?, ?, 'pending', ?, ?)`,
		featureID, boltNumber, string(units), string(deps), ws, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("creating bolt %d for %s: %w", boltNumber, featureID, err)
	}
	return nil
}

// GetBolts returns all Bolts for a feature ordered by bolt_number.
func (db *DB) GetBolts(featureID string) ([]BoltRow, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, bolt_number, unit_ids, depends_on, status, is_walking_skeleton, created_at
		 FROM bolts WHERE feature_id = ? ORDER BY bolt_number ASC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting bolts for %s: %w", featureID, err)
	}
	defer rows.Close()

	bolts := []BoltRow{}
	for rows.Next() {
		var b BoltRow
		var unitIDs, dependsOn string
		var wsInt int
		if err := rows.Scan(&b.ID, &b.FeatureID, &b.BoltNumber, &unitIDs, &dependsOn, &b.Status, &wsInt, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning bolt: %w", err)
		}
		json.Unmarshal([]byte(unitIDs), &b.UnitIDs)
		json.Unmarshal([]byte(dependsOn), &b.DependsOn)
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