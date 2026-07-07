package db

// TruncateAllTables is the exported test helper for cross-package tests
// (e.g. internal/chat) that need a clean DB. It clears all data tables
// including the chat tables + re-seeds the __chat__ sentinel feature row.
//
// This is the exported twin of the package-internal truncateAllTables in
// artifact_store_test.go. Kept in a non-_test.go file so external packages
// can call it from their tests.
func TruncateAllTables(d *DB) {
	tables := []string{
		"chat_messages", "chat_sessions",
		"audit_events", "tmux_sessions", "bolts", "feature_stages",
		"spec_artifacts", "outcomes", "notes", "events", "questions",
		"rules", "team_knowledge", "feature_repos", "sessions",
		"phase_states", "gate_results", "recirculations", "features",
		// repos tables (migration 017) — truncate so repo tests start clean.
		"repo_settings", "repo_operation_config", "repo_registry", "repos",
	}
	for _, table := range tables {
		d.Conn().Exec("TRUNCATE TABLE " + table + " CASCADE")
	}
	// Re-seed the __chat__ sentinel feature row — it's the FK parent for
	// chat_cli_exec audit events with no real feature. Truncate removes it;
	// tests that depend on it need it present.
	d.Exec(`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count, scope, depth, test_strategy)
		VALUES ('__chat__', '__chat__ sentinel', 'operation', 'sentinel', 0, 'loose_idea', '', now(), now(), 0, 'feature', 'minimal', 'standard')
		ON CONFLICT (id) DO NOTHING`)
}

// PostgresTestDSN is the shared test DSN for cross-package tests. Matches
// the const in artifact_store_test.go.
const PostgresTestDSN = "host=localhost port=5432 user=devteam password=devteam dbname=devteam_test_db sslmode=disable"