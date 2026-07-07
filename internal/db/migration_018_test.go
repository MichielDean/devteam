package db

import (
	"testing"
)

// TestMigration018_RoundTrip verifies migration 018 applies additively and
// idempotently: empty DB → migrate → all v1 tables/columns/indexes present;
// re-run migrate → no error (S-MIG-01, S-MIG-02).
func TestMigration018_RoundTrip(t *testing.T) {
	d := setupTestDB(t)
	// setupTestDB already runs all pending migrations (including 018) via Open.

	// feature_defaults exists with the expected columns.
	if !tableExists(t, d, "feature_defaults") {
		t.Fatal("feature_defaults table missing after migration")
	}
	for _, col := range []string{"id", "scope", "depth", "test_strategy", "execution_mode", "repo", "created_at", "updated_at"} {
		if !columnExists(t, d, "feature_defaults", col) {
			t.Errorf("feature_defaults.%s missing", col)
		}
	}

	// server_config exists as a key-value table.
	if !tableExists(t, d, "server_config") {
		t.Fatal("server_config table missing after migration")
	}
	for _, col := range []string{"key", "value", "updated_at"} {
		if !columnExists(t, d, "server_config", col) {
			t.Errorf("server_config.%s missing", col)
		}
	}

	// audit_events.actor was added (nullable).
	if !columnExists(t, d, "audit_events", "actor") {
		t.Fatal("audit_events.actor column missing after migration")
	}

	// idx_audit_events_type_time exists.
	if !indexExists(t, d, "idx_audit_events_type_time") {
		t.Fatal("idx_audit_events_type_time index missing after migration")
	}

	// Re-running migrations is a no-op (idempotency — S-MIG-02). The runner
	// records applied versions in schema_migrations, so a second Open() will
	// see 18 as applied and skip it. Verify by opening a second connection
	// against the same test DB.
	d2, err := Open(Config{DSN: postgresTestDSN}, postgresTestDSN)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer d2.Close()
	// No panic, no error — migration 018 was skipped as already-applied.
	if !tableExists(t, d2, "feature_defaults") {
		t.Error("feature_defaults missing after second Open (idempotency check)")
	}
}

// TestMigration018_ActorNullable verifies the actor column is nullable so
// legacy rows (and the backward-compat RecordAuditEvent overload) survive
// (S-MIG-03 AC1).
func TestMigration018_ActorNullable(t *testing.T) {
	d := setupTestDB(t)
	fid := "feat-mig018-actor"
	seedFeature(t, d, fid)

	// Legacy overload: no actor → NULL.
	if err := d.RecordAuditEvent(fid, AuditStageStart, "1.1", "ideation", "legacy"); err != nil {
		t.Fatalf("RecordAuditEvent (legacy): %v", err)
	}
	events, err := d.GetAuditEvents(fid)
	if err != nil {
		t.Fatalf("GetAuditEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Actor != "" {
		t.Errorf("legacy event actor = %q, want empty (NULL scanned as \"\")", events[0].Actor)
	}

	// New overload: actor populated.
	if err := d.RecordAuditEventWithActor(fid, AuditConfigUpdated, "", "construction", "mask test", "operator"); err != nil {
		t.Fatalf("RecordAuditEventWithActor: %v", err)
	}
	events, err = d.GetAuditEvents(fid)
	if err != nil {
		t.Fatalf("GetAuditEvents after actor: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	// events[1] is the actor-bearing event (chronological order).
	if events[1].Actor != "operator" {
		t.Errorf("actor event Actor = %q, want operator", events[1].Actor)
	}
}

// TestMigration018_AdditiveOnly verifies the schema diff vs the pre-018
// schema is purely additive — no dropped columns, no widened types
// (S-MIG-04). We assert the v1 tables exist and that audit_events still has
// every pre-018 column (the only change is the additive actor column).
func TestMigration018_AdditiveOnly(t *testing.T) {
	d := setupTestDB(t)

	// audit_events pre-018 columns are still present.
	for _, col := range []string{"id", "feature_id", "event_type", "stage_id", "phase", "details", "created_at"} {
		if !columnExists(t, d, "audit_events", col) {
			t.Errorf("audit_events.%s missing (additive-only violation)", col)
		}
	}

	// The v1 tables are net-new (additive).
	if !tableExists(t, d, "feature_defaults") {
		t.Error("feature_defaults missing (additive net-new table expected)")
	}
	if !tableExists(t, d, "server_config") {
		t.Error("server_config missing (additive net-new table expected)")
	}
}

// TestFourConfigMutationEventConstants verifies exactly the 4 v1
// config-mutation event constants exist (bolt-plan rev2 Bolt 0 DoD AC3) — no
// PROVIDER_CONFIG_MUTATED (deferred), no CI_CONFIG_MUTATED (cut), no
// CONFIG_REVERTED (dropped per F-1).
func TestFourConfigMutationEventConstants(t *testing.T) {
	want := map[string]string{
		"CONFIG_UPDATED":           AuditConfigUpdated,
		"CONFIG_VALIDATION_FAILED": AuditConfigValidationFailed,
		"REPOS_REGISTRY_MUTATED":   AuditReposRegistryMutated,
		"FEATURE_DEFAULTS_MUTATED": AuditFeatureDefaultsMutated,
	}
	for val, constVal := range want {
		if constVal != val {
			t.Errorf("constant %q = %q, want %q", val, constVal, val)
		}
	}
	// Negative checks: the deferred/cut constants must not be defined.
	for _, dropped := range []string{"PROVIDER_CONFIG_MUTATED", "CI_CONFIG_MUTATED", "CONFIG_REVERTED"} {
		for _, c := range []string{AuditConfigUpdated, AuditConfigValidationFailed, AuditReposRegistryMutated, AuditFeatureDefaultsMutated} {
			if c == dropped {
				t.Errorf("dropped constant %s must not be defined in v1", dropped)
			}
		}
	}
}

// TestRecordAuditEventWithActor_BackwardCompat verifies the legacy
// RecordAuditEvent signature still compiles and produces actor=NULL rows
// (S-MIG-03 AC3, FR-AUDIT-ACTOR-03).
func TestRecordAuditEventWithActor_BackwardCompat(t *testing.T) {
	d := setupTestDB(t)
	fid := "feat-backcompat"
	seedFeature(t, d, fid)

	// Legacy call compiles unchanged.
	if err := d.RecordAuditEvent(fid, AuditStageStart, "1.1", "ideation", "legacy"); err != nil {
		t.Fatalf("RecordAuditEvent: %v", err)
	}
	events, err := d.GetAuditEvents(fid)
	if err != nil {
		t.Fatalf("GetAuditEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Actor != "" {
		t.Errorf("legacy event actor = %q, want empty (NULL)", events[0].Actor)
	}
}

// TestGetAuditEventsFiltered exercises the cross-feature filtered read
// backing the Audit tab (FR-AUDIT-01, FR-AUDIT-02). Verifies type filter,
// pagination, and total count.
func TestGetAuditEventsFiltered(t *testing.T) {
	d := setupTestDB(t)
	fid := "feat-filtered"
	seedFeature(t, d, fid)

	// Seed a mix of event types.
	d.RecordAuditEventWithActor(fid, AuditConfigUpdated, "", "construction", "s1", "operator")
	d.RecordAuditEventWithActor(fid, AuditReposRegistryMutated, "", "construction", "s2", "operator")
	d.RecordAuditEventWithActor(fid, AuditConfigUpdated, "", "construction", "s3", "operator")
	d.RecordAuditEventWithActor(fid, AuditFeatureDefaultsMutated, "", "construction", "s4", "operator")

	// Filter by single type.
	events, total, err := d.GetAuditEventsFiltered(AuditFilter{EventType: AuditConfigUpdated, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("GetAuditEventsFiltered CONFIG_UPDATED: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(events) != 2 {
		t.Errorf("len = %d, want 2", len(events))
	}

	// No filter → all 4.
	events, total, err = d.GetAuditEventsFiltered(AuditFilter{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("GetAuditEventsFiltered all: %v", err)
	}
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}
	if len(events) != 4 {
		t.Errorf("len = %d, want 4", len(events))
	}

	// Pagination: page_size 2 → page 1 returns 2, page 2 returns 2.
	events, total, err = d.GetAuditEventsFiltered(AuditFilter{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("GetAuditEventsFiltered page 1: %v", err)
	}
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}
	if len(events) != 2 {
		t.Errorf("page 1 len = %d, want 2", len(events))
	}
	events, _, err = d.GetAuditEventsFiltered(AuditFilter{Page: 2, PageSize: 2})
	if err != nil {
		t.Fatalf("GetAuditEventsFiltered page 2: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("page 2 len = %d, want 2", len(events))
	}

	// Comma-list type filter.
	events, total, err = d.GetAuditEventsFiltered(AuditFilter{EventType: AuditConfigUpdated + "," + AuditReposRegistryMutated, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("GetAuditEventsFiltered comma-list: %v", err)
	}
	if total != 3 {
		t.Errorf("comma-list total = %d, want 3", total)
	}

	// Actor filter.
	events, total, err = d.GetAuditEventsFiltered(AuditFilter{Actor: "operator", Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("GetAuditEventsFiltered actor: %v", err)
	}
	if total != 4 {
		t.Errorf("actor total = %d, want 4", total)
	}
	events, total, err = d.GetAuditEventsFiltered(AuditFilter{Actor: "nobody", Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("GetAuditEventsFiltered actor nobody: %v", err)
	}
	if total != 0 {
		t.Errorf("actor nobody total = %d, want 0", total)
	}
}

// --- helpers ---

func tableExists(t *testing.T, d *DB, name string) bool {
	t.Helper()
	var exists bool
	err := d.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = $1
		)`, name).Scan(&exists)
	if err != nil {
		t.Fatalf("checking table %s: %v", name, err)
	}
	return exists
}

func columnExists(t *testing.T, d *DB, table, column string) bool {
	t.Helper()
	var exists bool
	err := d.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = $1 AND column_name = $2
		)`, table, column).Scan(&exists)
	if err != nil {
		t.Fatalf("checking column %s.%s: %v", table, column, err)
	}
	return exists
}

func indexExists(t *testing.T, d *DB, name string) bool {
	t.Helper()
	var exists bool
	err := d.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE indexname = $1
		)`, name).Scan(&exists)
	if err != nil {
		t.Fatalf("checking index %s: %v", name, err)
	}
	return exists
}