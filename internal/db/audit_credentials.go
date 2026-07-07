package db

import (
	"fmt"
	"time"
)

// RecordCredentialAuditEvent inserts an audit event with credential_touched=1
// (feature github-authorization-integration, U-11, FR-AUDIT-02).
//
// The details string MUST already be redacted by the caller (the redaction
// wrapper at internal/github/redaction.go is the write-boundary guard; this
// helper does NOT re-redact — it trusts the caller). details carries
// actor + action + target + fingerprint ONLY, never a secret value
// (nfr-design-specs §5.3, BR-AUDIT-01/02).
//
// Only this helper writes credential_touched=1. The existing 68-call-site
// RecordAuditEvent continues to write 0 (the column's DEFAULT 0 covers it).
// Static check (nfr-design-specs §5.2): grep internal/ for credential_touched
// writes outside this file → zero hits.
func (db *DB) RecordCredentialAuditEvent(featureID, eventType, stageID, phase, details string) error {
	_, err := db.Exec(
		`INSERT INTO audit_events (feature_id, event_type, stage_id, phase, details, credential_touched, created_at)
		 VALUES (?, ?, ?, ?, ?, 1, ?)`,
		featureID, eventType, stageID, phase, details, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("recording credential audit event %s: %w", eventType, err)
	}
	return nil
}

// RecordRepoSettingsAudit inserts a REPO_SETTINGS_CHANGED audit event with
// credential_touched=0 (settings are not credentials — FR-AUDIT-03). details
// carries the field diff with redacted values; the caller redacts before
// passing (redaction is defensive even though the MVP field set is non-secret).
func (db *DB) RecordRepoSettingsAudit(featureID, repo, details string) error {
	_, err := db.Exec(
		`INSERT INTO audit_events (feature_id, event_type, stage_id, phase, details, credential_touched, created_at)
		 VALUES (?, ?, '', 'repo', ?, 0, ?)`,
		featureID, AuditRepoSettingsChanged, details, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("recording repo settings audit for %s: %w", repo, err)
	}
	return nil
}

// RecordRepoRegistryAudit inserts a REPO_REGISTRY_SYNCED audit event with
// credential_touched=0 (FR-DISC-06, U-04). details carries the sync summary
// (e.g. "added=3 removed=0 managed=2").
func (db *DB) RecordRepoRegistryAudit(featureID, details string) error {
	_, err := db.Exec(
		`INSERT INTO audit_events (feature_id, event_type, stage_id, phase, details, credential_touched, created_at)
		 VALUES (?, ?, '', 'repo', ?, 0, ?)`,
		featureID, AuditRepoRegistrySynced, details, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("recording repo registry audit: %w", err)
	}
	return nil
}

// AuditEventWithCredentialTouched is the extended AuditEvent row shape after
// migration 020 (adds credential_touched). Used by audit-list CLI filtering.
type AuditEventWithCredentialTouched struct {
	AuditEvent
	CredentialTouched int `json:"credential_touched"`
}

// GetAuditEventsWithCredentialTouched returns audit events for a feature,
// including the credential_touched column (post-migration-020 shape).
func (db *DB) GetAuditEventsWithCredentialTouched(featureID string) ([]AuditEventWithCredentialTouched, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, event_type, stage_id, phase, details, created_at, credential_touched
		 FROM audit_events WHERE feature_id = ? ORDER BY created_at ASC, id ASC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting audit events (with credential_touched) for %s: %w", featureID, err)
	}
	defer rows.Close()

	var events []AuditEventWithCredentialTouched
	for rows.Next() {
		var e AuditEventWithCredentialTouched
		if err := rows.Scan(&e.ID, &e.FeatureID, &e.EventType, &e.StageID, &e.Phase, &e.Details, &e.CreatedAt, &e.CredentialTouched); err != nil {
			return nil, fmt.Errorf("scanning audit event with credential_touched: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}