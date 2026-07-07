package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 18,
		Name:    "settings_and_admin_ui",
		Up:      migration018SettingsAdmin,
	})
}

// migration018SettingsAdmin creates the DB-backed config store backing the
// admin UI. Per bolt-plan rev2 (honoring human input Q7/Q8 strictly), this
// migration is thinned from the original app-design §3 catalog:
//
//   - feature_defaults (global + per-repo default scope/depth/test_strategy/
//     execution_mode) — backs the Defaults tab (Bolt 2, MVP).
//   - server_config (key-value table for DB-backed mutable server keys) —
//     backs the Server tab (Bolt 3).
//   - audit_events.actor (nullable; populated by RecordAuditEventWithActor)
//   - idx_audit_events_type_time (cross-feature audit filter index for the
//     Audit tab — Bolt 4).
//
// NOT in this migration (cut from v1 per Q7/Q8):
//   - llm_providers / llm_models / tier_model_map — owned by the sibling
//     feature multi-provider-llm-configuration (Q7 strict scope cut). This
//     feature surfaces them as a fast-follow integration tab (Bolt 5) once
//     the sibling API is frozen.
//   - cicd_platforms — cut entirely (Q8). A dedicated ci-cd-platform-config
//     feature owns it.
//
// All DDL is additive (CREATE TABLE IF NOT EXISTS / ALTER TABLE ADD COLUMN),
// idempotent, and runs in the single migration transaction the runner
// already wraps each migration in (FR-MIG-01). The down path is a clean DROP
// of these tables/columns/indexes — no existing table is touched except the
// additive actor column on audit_events.
func migration018SettingsAdmin(tx *sql.Tx) error {
	statements := []string{
		// 1. Feature defaults — global (repo IS NULL) + per-repo override.
		// One global row + one row per repo override. The UNIQUE(repo)
		// constraint enforces this: Postgres treats multiple NULLs as
		// distinct, so the single global row must be inserted explicitly
		// (the store layer owns that invariant).
		`CREATE TABLE IF NOT EXISTS feature_defaults (
			id             SERIAL PRIMARY KEY,
			scope          TEXT,
			depth          TEXT,
			test_strategy  TEXT,
			execution_mode TEXT,
			repo           TEXT NULL,
			created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
			CONSTRAINT feature_defaults_repo_unique UNIQUE (repo)
		)`,

		// 2. Server config — key-value table for DB-backed mutable server
		// keys. value is JSONB so each key can carry a typed value without
		// a per-key migration. Validation is in the store layer
		// (go-playground/validator per ADR-D3), not the DB.
		`CREATE TABLE IF NOT EXISTS server_config (
			key        TEXT PRIMARY KEY,
			value      JSONB NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,

		// 3. Audit actor column — additive, nullable for legacy rows
		// (ADR-AUDIT-ACTOR). RecordAuditEventWithActor populates it;
		// the existing RecordAuditEvent calls the overload with actor=""
		// which inserts NULL (FR-AUDIT-ACTOR-03 backward-compat).
		`ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS actor TEXT`,

		// 3a. Drop the audit_events.feature_id → features(id) FK so
		// platform-level config-mutation events (feature_id = "platform")
		// can be recorded without a sentinel feature row. Prior to this
		// feature every audit event was feature-scoped; the admin config
		// surface introduces platform-level events (repos registry, feature
		// defaults, server config) that have no owning feature. The FK is
		// safe to drop because nothing else references it; the existing
		// idx_audit_events_feature index is preserved. This is the one
		// non-additive change in this migration, scoped to a constraint
		// (not a column) so the schema diff remains non-destructive to
		// data.
		`ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS audit_events_feature_id_fkey`,

		// 4. Cross-feature audit filter index (ADR-AUDIT-INDEX).
		// The Audit tab filters by event_type + time range across all
		// features; without this index the query devolves to a seq scan
		// over the whole audit_events table (R-AUDIT-VOLUME). The existing
		// idx_audit_events_feature(feature_id, created_at) is preserved.
		`CREATE INDEX IF NOT EXISTS idx_audit_events_type_time
			ON audit_events (event_type, created_at)`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("executing statement: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}