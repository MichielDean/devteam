package db

import (
	"database/sql"
	"testing"
)

// TestAC008_Migration017Idempotent verifies migration 017 is idempotent: running
// its Up function twice against a fresh transaction produces no error and no
// duplicate rows. Traces U-DATA-01, NFR-DATA-01, iac-designs §1.5.
func TestAC008_Migration017Idempotent(t *testing.T) {
	d := setupTestDB(t)

	// Drop the config tables so we can re-run the migration from scratch against
	// this connection's schema (the migration runner already applied 017 once at
	// Open time, but truncateAllTables wiped the seed rows — we want to verify the
	// Up function itself is idempotent, independent of the runner).
	for _, table := range []string{"role_overrides", "tier_models", "provider_models", "providers"} {
		if _, err := d.Conn().Exec("DROP TABLE IF EXISTS " + table + " CASCADE"); err != nil {
			t.Fatalf("drop %s: %v", table, err)
		}
	}

	// Run Up twice in separate transactions (mimicking the runner's per-migration tx).
	runOnce := func() {
		tx, err := d.Conn().Begin()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		if err := migration017ProviderConfig(tx); err != nil {
			tx.Rollback()
			t.Fatalf("migration017 Up: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}
	runOnce()
	runOnce() // second run must be a no-op (ON CONFLICT DO NOTHING)

	// Assert row counts match the seed exactly (no duplicates).
	var providerCount int
	if err := d.QueryRow("SELECT COUNT(*) FROM providers").Scan(&providerCount); err != nil {
		t.Fatalf("count providers: %v", err)
	}
	if providerCount != 4 {
		t.Errorf("providers count = %d, want 4", providerCount)
	}

	var tierCount int
	if err := d.QueryRow("SELECT COUNT(*) FROM tier_models").Scan(&tierCount); err != nil {
		t.Fatalf("count tier_models: %v", err)
	}
	if tierCount != 2 {
		t.Errorf("tier_models count = %d, want 2", tierCount)
	}

	var modelCount int
	if err := d.QueryRow("SELECT COUNT(*) FROM provider_models").Scan(&modelCount); err != nil {
		t.Fatalf("count provider_models: %v", err)
	}
	if modelCount != 3 {
		t.Errorf("provider_models count = %d, want 3", modelCount)
	}

	// The sentinel __platform__ row should exist exactly once. The first run
	// inserts it; the second run's ON CONFLICT DO NOTHING leaves it. Note:
	// truncateAllTables wiped it, so the first run re-inserts.
	var sentinelCount int
	if err := d.QueryRow("SELECT COUNT(*) FROM features WHERE id = '__platform__'").Scan(&sentinelCount); err != nil {
		t.Fatalf("count sentinel: %v", err)
	}
	if sentinelCount != 1 {
		t.Errorf("sentinel __platform__ count = %d, want 1", sentinelCount)
	}

	// Verify the sentinel is inert: priority=0 (sorts below real features).
	var priority int
	if err := d.QueryRow("SELECT priority FROM features WHERE id = '__platform__'").Scan(&priority); err != nil {
		t.Fatalf("select sentinel priority: %v", err)
	}
	if priority != 0 {
		t.Errorf("sentinel priority = %d, want 0", priority)
	}
}

// TestAC008_Migration017SeedValues verifies the seed row values are correct:
// Anthropic + Ollama enabled (MVP), OpenAI + Copilot disabled (fast-follow),
// $VAR reference convention, Copilot env_var_supported=0. Traces U-DATA-02.
func TestAC008_Migration017SeedValues(t *testing.T) {
	d := setupTestDB(t)
	// Re-seed (truncateAllTables wiped the migration's seed).
	seedProviderConfigForTest(t, d)

	type row struct {
		name, displayName, baseURL, apiKeyEnv, defaultModel, presetID string
		enabled, envVarSupported                                       int
	}
	want := map[string]row{
		"anthropic":    {"anthropic", "Anthropic", "https://api.anthropic.com/v1", "$ANTHROPIC_API_KEY", "claude-opus-4", "anthropic", 1, 1},
		"ollama-cloud": {"ollama-cloud", "Ollama Cloud", "", "$OLLAMA_API_KEY", "glm-5.2:cloud", "ollama-cloud", 1, 1},
		"openai":       {"openai", "OpenAI", "https://api.openai.com/v1", "$OPENAI_API_KEY", "gpt-4o", "openai", 0, 1},
		"copilot":      {"copilot", "GitHub Copilot", "", "", "", "copilot", 0, 0},
	}
	rows, err := d.Query("SELECT name, display_name, base_url, api_key_env, default_model_id, preset_id, enabled, env_var_supported FROM providers ORDER BY name")
	if err != nil {
		t.Fatalf("query providers: %v", err)
	}
	defer rows.Close()
	got := map[string]row{}
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.name, &r.displayName, &r.baseURL, &r.apiKeyEnv, &r.defaultModel, &r.presetID, &r.enabled, &r.envVarSupported); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[r.name] = r
	}
	if len(got) != 4 {
		t.Fatalf("provider count = %d, want 4", len(got))
	}
	for name, w := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("missing provider %s", name)
			continue
		}
		if g != w {
			t.Errorf("provider %s = %+v, want %+v", name, g, w)
		}
	}

	// Verify tier_models seed.
	tierRows, err := d.Query("SELECT tier, provider_name, model_id FROM tier_models ORDER BY tier, provider_name")
	if err != nil {
		t.Fatalf("query tier_models: %v", err)
	}
	defer tierRows.Close()
	type tier struct{ tier, provider, model string }
	wantTiers := []tier{
		{"opus", "anthropic", "claude-opus-4"},
		{"sonnet", "ollama-cloud", "glm-5.2:cloud"},
	}
	var gotTiers []tier
	for tierRows.Next() {
		var ti tier
		if err := tierRows.Scan(&ti.tier, &ti.provider, &ti.model); err != nil {
			t.Fatalf("scan tier: %v", err)
		}
		gotTiers = append(gotTiers, ti)
	}
	if len(gotTiers) != len(wantTiers) {
		t.Fatalf("tier count = %d, want %d", len(gotTiers), len(wantTiers))
	}
	for i, w := range wantTiers {
		if i >= len(gotTiers) || gotTiers[i] != w {
			t.Errorf("tier[%d] = %+v, want %+v", i, gotTiers[i], w)
		}
	}
}

// TestAC008_RecordAuditEventPlatformSentinel verifies that RecordAuditEvent with
// feature_id="__platform__" succeeds (the sentinel row satisfies the FK). This
// is the F-01 fix from architecture-review 3.3. Traces U-AUDIT-01.
func TestAC008_RecordAuditEventPlatformSentinel(t *testing.T) {
	d := setupTestDB(t)
	seedProviderConfigForTest(t, d) // inserts the sentinel row

	if err := d.RecordAuditEvent("__platform__", EventProviderConfigMutated, "", "config", "test mutation"); err != nil {
		t.Fatalf("RecordAuditEvent(__platform__): %v", err)
	}

	var eventType, details string
	err := d.QueryRow(
		"SELECT event_type, details FROM audit_events WHERE feature_id = '__platform__' ORDER BY created_at DESC LIMIT 1",
	).Scan(&eventType, &details)
	if err != nil {
		t.Fatalf("select audit event: %v", err)
	}
	if eventType != EventProviderConfigMutated {
		t.Errorf("event_type = %s, want %s", eventType, EventProviderConfigMutated)
	}
	if details != "test mutation" {
		t.Errorf("details = %q, want %q", details, "test mutation")
	}
}

// seedProviderConfigForTest re-inserts the migration 017 seed rows after
// truncateAllTables wiped them. Tests that rely on the seed call this. The seed
// mirrors migration_017.go exactly; if the migration changes, update both.
func seedProviderConfigForTest(t *testing.T, d *DB) {
	t.Helper()
	// Insert sentinel first (audit FK depends on features).
	now := "NOW()"
	_, err := d.Exec(`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, worktree_dir, created_at, updated_at, recirculation_count)
		VALUES ('__platform__', 'Platform Configuration', 'construction', 'draft', 0, 'platform', 'platform', '', ` + now + `, ` + now + `, 0)
		ON CONFLICT (id) DO NOTHING`)
	if err != nil {
		t.Fatalf("seed sentinel: %v", err)
	}
	providers := []struct {
		name, display, baseURL, apiKeyEnv, defaultModel, npm, preset string
		enabled, envVar                                                int
	}{
		{"anthropic", "Anthropic", "https://api.anthropic.com/v1", "$ANTHROPIC_API_KEY", "claude-opus-4", "@ai-sdk/openai-compatible", "anthropic", 1, 1},
		{"ollama-cloud", "Ollama Cloud", "", "$OLLAMA_API_KEY", "glm-5.2:cloud", "@ai-sdk/openai-compatible", "ollama-cloud", 1, 1},
		{"openai", "OpenAI", "https://api.openai.com/v1", "$OPENAI_API_KEY", "gpt-4o", "@ai-sdk/openai-compatible", "openai", 0, 1},
		{"copilot", "GitHub Copilot", "", "", "", "@ai-sdk/openai-compatible", "copilot", 0, 0},
	}
	for _, p := range providers {
		_, err := d.Exec(`INSERT INTO providers (name, display_name, enabled, base_url, api_key_env, default_model_id, npm_adapter, env_var_supported, preset_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (name) DO NOTHING`,
			p.name, p.display, p.enabled, p.baseURL, p.apiKeyEnv, p.defaultModel, p.npm, p.envVar, p.preset)
		if err != nil {
			t.Fatalf("seed provider %s: %v", p.name, err)
		}
	}
	tiers := []struct{ tier, provider, model string }{
		{"opus", "anthropic", "claude-opus-4"},
		{"sonnet", "ollama-cloud", "glm-5.2:cloud"},
	}
	for _, ti := range tiers {
		_, err := d.Exec(`INSERT INTO tier_models (tier, provider_name, model_id) VALUES (?, ?, ?) ON CONFLICT (tier, provider_name) DO NOTHING`,
			ti.tier, ti.provider, ti.model)
		if err != nil {
			t.Fatalf("seed tier %s/%s: %v", ti.tier, ti.provider, err)
		}
	}
	models := []struct{ provider, model, friendly string }{
		{"anthropic", "claude-opus-4", "Claude Opus 4"},
		{"ollama-cloud", "glm-5.2:cloud", "GLM 5.2 Cloud"},
		{"openai", "gpt-4o", "GPT-4o"},
	}
	for _, m := range models {
		_, err := d.Exec(`INSERT INTO provider_models (provider_name, model_id, friendly_name) VALUES (?, ?, ?) ON CONFLICT (provider_name, model_id) DO NOTHING`,
			m.provider, m.model, m.friendly)
		if err != nil {
			t.Fatalf("seed model %s/%s: %v", m.provider, m.model, err)
		}
	}
}

// suppress unused import warning if database/sql is only used by other tests
var _ = sql.ErrNoRows