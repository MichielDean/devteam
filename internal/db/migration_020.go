package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 20,
		Name:    "audit_credential_touched",
		Up:      migration020AuditCredentialTouched,
	})
}

// migration020AuditCredentialTouched adds the credential_touched column to
// audit_events (feature github-authorization-integration, U-11, FR-AUDIT-01).
//
// Existing rows keep default 0 (BR-AUDIT-03, C-18 — additive, no backfill). Only
// RecordCredentialAuditEvent (new helper in U-11) writes credential_touched=1;
// the existing 68 RecordAuditEvent call sites continue to write 0 (NFR-COMPAT-01).
//
// The index on (feature_id, credential_touched) backs the P-3 auditor's primary
// filter `--credential-touched` (interaction-spec §7, US-11).
//
// Migration version is 020 (architecture-review B1: feature migrations start at 018).
//
// Idempotence: ALTER TABLE ... ADD COLUMN IF NOT EXISTS (Postgres 9.6+, used by
// migration_010 — nfr-design-specs §8.3). The stale comment in migration_012
// claiming PG doesn't support IF NOT EXISTS is a code-quality finding, not a
// blocker; migration_010 is the precedent that proves it works.
func migration020AuditCredentialTouched(tx *sql.Tx) error {
	statements := []string{
		`ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS credential_touched INTEGER NOT NULL DEFAULT 0`,
		`CREATE INDEX IF NOT EXISTS idx_audit_events_cred_touched ON audit_events(feature_id, credential_touched)`,
	}
	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("executing audit_credential_touched statement: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}