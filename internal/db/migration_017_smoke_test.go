package db

import (
	"fmt"
	"testing"
)

// TestMigration017_Applies verifies migration 017 creates the
// repo_operation_config table with the correct columns and no FK.
func TestMigration017_Applies(t *testing.T) {
	d := setupTestDB(t)
	// setupTestDB already ran migrations (including 017) on connect.
	// Verify columns.
	rows, err := d.Conn().Query(`SELECT column_name, data_type, column_default FROM information_schema.columns WHERE table_name = 'repo_operation_config' ORDER BY ordinal_position`)
	if err != nil {
		t.Fatalf("query columns: %v", err)
	}
	defer rows.Close()

	type col struct {
		name, dtype, def string
	}
	got := []col{}
	for rows.Next() {
		var c col
		var def any
		if err := rows.Scan(&c.name, &c.dtype, &def); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if def != nil {
			c.def = fmt.Sprintf("%v", def)
		}
		got = append(got, c)
	}
	want := []col{
		{"repo_name", "text", ""},
		{"ci_platform", "text", "''::text"},
		{"cd_platform", "text", "''::text"},
		{"environments", "jsonb", "'{}'::jsonb"},
		{"observability", "jsonb", "'{}'::jsonb"},
		{"incident_response", "jsonb", "'{}'::jsonb"},
		{"created_at", "timestamp with time zone", ""},
		{"updated_at", "timestamp with time zone", ""},
	}
	if len(got) != len(want) {
		t.Fatalf("columns: got %d, want %d — %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].name != w.name {
			t.Errorf("col[%d] name = %q, want %q", i, got[i].name, w.name)
		}
		if got[i].dtype != w.dtype {
			t.Errorf("col[%d] type = %q, want %q", i, got[i].dtype, w.dtype)
		}
	}
	// Verify no FK.
	var fkCount int
	d.Conn().QueryRow(`SELECT COUNT(*) FROM information_schema.table_constraints WHERE table_name = 'repo_operation_config' AND constraint_type = 'FOREIGN KEY'`).Scan(&fkCount)
	if fkCount != 0 {
		t.Errorf("FK count = %d, want 0 (C-D3)", fkCount)
	}
	// Verify row count is 0 (FR-SCHEMA-06).
	var n int
	d.Conn().QueryRow(`SELECT count(*) FROM repo_operation_config`).Scan(&n)
	if n != 0 {
		t.Errorf("row count = %d, want 0 (table starts empty)", n)
	}
}