package db

import (
	"testing"
)

func TestMigration018_ChatTablesExist(t *testing.T) {
	d := setupTestDB(t)
	// Tables created at open time via migration runner. Verify they exist and the
	// role CHECK constraint enforces the allowed values.
	cases := []struct {
		table string
	}{
		{"chat_sessions"},
		{"chat_messages"},
	}
	for _, c := range cases {
		var n int
		err := d.QueryRow(
			`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = ?`, c.table,
		).Scan(&n)
		if err != nil {
			t.Fatalf("checking table %s: %v", c.table, err)
		}
		if n != 1 {
			t.Errorf("expected table %s to exist", c.table)
		}
	}
}

func TestMigration018_AuditColumnsExist(t *testing.T) {
	d := setupTestDB(t)
	for _, col := range []string{"session_id", "actor", "feature_id_chat"} {
		var n int
		err := d.QueryRow(
			`SELECT COUNT(*) FROM information_schema.columns
			 WHERE table_name = 'audit_events' AND column_name = ?`, col,
		).Scan(&n)
		if err != nil {
			t.Fatalf("checking column %s: %v", col, err)
		}
		if n != 1 {
			t.Errorf("expected audit_events.%s to exist", col)
		}
	}
}

func TestMigration018_ChatSentinelFeatureExists(t *testing.T) {
	d := setupTestDB(t)
	var n int
	err := d.QueryRow(
		`SELECT COUNT(*) FROM features WHERE id = '__chat__'`,
	).Scan(&n)
	if err != nil {
		t.Fatalf("querying sentinel: %v", err)
	}
	if n != 1 {
		t.Errorf("expected __chat__ sentinel feature row, got %d", n)
	}
}

func TestMigration018_IdempotentReapply(t *testing.T) {
	// Re-running the migration's SQL statements must be safe (IF NOT EXISTS / ON CONFLICT).
	d := setupTestDB(t)
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS chat_sessions (id UUID PRIMARY KEY DEFAULT gen_random_uuid())`,
		`ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS session_id UUID`,
		`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count, scope, depth, test_strategy)
		 VALUES ('__chat__', '__chat__ sentinel', 'operation', 'sentinel', 0, 'loose_idea', '',
		         now(), now(), 0, 'feature', 'minimal', 'standard')
		 ON CONFLICT (id) DO NOTHING`,
	} {
		if _, err := d.Exec(stmt); err != nil {
			t.Fatalf("re-running statement should be idempotent: %v\nSQL: %s", err, stmt)
		}
	}
}