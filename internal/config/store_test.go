package config

import (
	"fmt"
	"testing"

	"github.com/MichielDean/devteam/internal/db"
)

// testDB opens a fresh test DB, truncates, and re-seeds the provider config.
// Returns the DB and a wired ProviderStore + TierStore.
func testDB(t *testing.T) (*db.DB, *ProviderStore, *TierStore) {
	t.Helper()
	database, err := db.Open(db.Config{DSN: postgresTestDSN}, postgresTestDSN)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	truncateForConfigTest(database)
	t.Cleanup(func() { database.Close() })
	seedProviderConfigForTest(t, database)
	return database, NewProviderStore(database), NewTierStore(database)
}

// postgresTestDSN mirrors the internal/db test DSN. Re-declared here because the
// const is unexported in package db. Kept in sync manually.
const postgresTestDSN = "host=localhost port=5432 user=devteam password=devteam dbname=devteam_test_db sslmode=disable"

func truncateForConfigTest(d *db.DB) {
	tables := []string{
		"role_overrides", "tier_models", "provider_models", "providers", "features",
		"audit_events", "tmux_sessions", "bolts", "feature_stages",
		"spec_artifacts", "outcomes", "notes", "events", "questions",
		"rules", "team_knowledge", "feature_repos", "sessions",
		"phase_states", "gate_results", "recirculations",
	}
	for _, table := range tables {
		d.Conn().Exec("TRUNCATE TABLE " + table + " CASCADE")
	}
}

func seedProviderConfigForTest(t *testing.T, d *db.DB) {
	t.Helper()
	_, err := d.Exec(`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, worktree_dir, created_at, updated_at, recirculation_count)
		VALUES ('__platform__', 'Platform Configuration', 'construction', 'draft', 0, 'platform', 'platform', '', NOW(), NOW(), 0)
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

// TestAC001_ProviderStoreSeed verifies the seeded providers load with correct
// enabled flags, key states, and models. Traces U-DATA-02, U-DATA-03.
func TestAC001_ProviderStoreSeed(t *testing.T) {
	_, store, _ := testDB(t)
	// Control key states via env getter stub.
	prev := osGetenv
	defer func() { osGetenv = prev }()
	osGetenv = func(name string) string {
		switch name {
		case "ANTHROPIC_API_KEY":
			return "sk-ant-test"
		default:
			return ""
		}
	}

	providers, err := store.Providers()
	if err != nil {
		t.Fatalf("Providers: %v", err)
	}
	if len(providers) != 4 {
		t.Fatalf("provider count = %d, want 4", len(providers))
	}
	byName := map[string]ProviderConfig{}
	for _, p := range providers {
		byName[p.Name] = p
	}
	if !byName["anthropic"].Enabled {
		t.Error("anthropic should be enabled")
	}
	if !byName["ollama-cloud"].Enabled {
		t.Error("ollama-cloud should be enabled")
	}
	if byName["openai"].Enabled {
		t.Error("openai should be disabled")
	}
	if byName["copilot"].Enabled {
		t.Error("copilot should be disabled")
	}
	if byName["anthropic"].KeyState != KeyStateSet {
		t.Errorf("anthropic key_state = %s, want set (env stub returns sk-ant-test)", byName["anthropic"].KeyState)
	}
	if byName["ollama-cloud"].KeyState != KeyStateNotSet {
		t.Errorf("ollama-cloud key_state = %s, want not_set (env stub returns '')", byName["ollama-cloud"].KeyState)
	}
	if byName["copilot"].KeyState != KeyStateNotRequired {
		t.Errorf("copilot key_state = %s, want not_required (api_key_env empty)", byName["copilot"].KeyState)
	}
	// Models: anthropic has 1 (claude-opus-4), copilot has 0 (but should be [], not nil).
	if len(byName["anthropic"].Models) != 1 {
		t.Errorf("anthropic models = %d, want 1", len(byName["anthropic"].Models))
	}
	if byName["copilot"].Models == nil {
		t.Error("copilot models should be [] not nil (JSON serialization)")
	}
}

// TestAC001_ProviderStoreCRUD verifies upsert/read/delete round-trip.
func TestAC001_ProviderStoreCRUD(t *testing.T) {
	_, store, _ := testDB(t)

	// Upsert a new custom provider.
	err := store.UpsertProvider(ProviderConfig{
		Name:            "custom-prov",
		DisplayName:     "Custom Provider",
		Enabled:         true,
		BaseURL:         "https://custom.example/v1",
		APIKeyEnv:       "$CUSTOM_API_KEY",
		DefaultModelID:  "custom-model",
		NPMAdapter:      "@ai-sdk/openai-compatible",
		EnvVarSupported: true,
		PresetID:        "custom",
		Models: []ProviderModel{
			{ModelID: "custom-model", FriendlyName: "Custom Model"},
			{ModelID: "custom-model-2", FriendlyName: "Custom Model 2"},
		},
	})
	if err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}

	// Read back.
	p, err := store.Provider("custom-prov")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if p == nil {
		t.Fatal("custom-prov not found after upsert")
	}
	if len(p.Models) != 2 {
		t.Errorf("models = %d, want 2", len(p.Models))
	}

	// Update (replace models with one).
	err = store.UpsertProvider(ProviderConfig{
		Name:           "custom-prov",
		DisplayName:     "Custom Provider Updated",
		Enabled:        false,
		BaseURL:        "https://custom.example/v1",
		APIKeyEnv:       "$CUSTOM_API_KEY",
		DefaultModelID: "custom-model",
		NPMAdapter:     "@ai-sdk/openai-compatible",
		PresetID:       "custom",
		Models: []ProviderModel{
			{ModelID: "only-model", FriendlyName: "Only Model"},
		},
	})
	if err != nil {
		t.Fatalf("UpsertProvider update: %v", err)
	}
	p, _ = store.Provider("custom-prov")
	if len(p.Models) != 1 || p.Models[0].ModelID != "only-model" {
		t.Errorf("after update, models = %+v, want [only-model]", p.Models)
	}
	if p.Enabled {
		t.Error("after update, should be disabled")
	}

	// Delete.
	if err := store.DeleteProvider("custom-prov"); err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}
	p, _ = store.Provider("custom-prov")
	if p != nil {
		t.Error("custom-prov should be gone after delete")
	}
}

// TestAC005_TierResolutionPerProvider verifies opus→anthropic when anthropic
// enabled, and opus→ollama-cloud when only ollama enabled. Traces U-DATA-04.
func TestAC005_TierResolutionPerProvider(t *testing.T) {
	_, store, tierStore := testDB(t)
	providers, err := store.Providers()
	if err != nil {
		t.Fatalf("Providers: %v", err)
	}

	// Both anthropic + ollama enabled: opus → anthropic (alphabetical, anthropic < ollama-cloud).
	p, model := tierStore.ResolveTier("opus", providers)
	if p == nil {
		t.Fatal("opus: expected anthropic, got nil")
	}
	if p.Name != "anthropic" {
		t.Errorf("opus provider = %s, want anthropic", p.Name)
	}
	if model != "claude-opus-4" {
		t.Errorf("opus model = %s, want claude-opus-4", model)
	}

	// Disable anthropic → opus has no tier row for ollama-cloud (iac-designs seed
	// only maps opus→anthropic). ResolveTier returns nil; ResolveProvider's
	// single-provider fallback (step 4) handles the rest (tested separately).
	for i := range providers {
		if providers[i].Name == "anthropic" {
			providers[i].Enabled = false
		}
	}
	p, model = tierStore.ResolveTier("opus", providers)
	if p != nil {
		t.Errorf("opus with anthropic disabled and no ollama tier row: expected nil, got %s", p.Name)
	}

	// The full ResolveProvider should fall back to the single enabled provider
	// (ollama-cloud) via step 4. Verify that separately.
	prev := osGetenv
	defer func() { osGetenv = prev }()
	osGetenv = func(name string) string {
		if name == "OLLAMA_API_KEY" {
			return "test-ollama-key"
		}
		return ""
	}
	rp, err := ResolveProvider("architect", providers, map[string]RoleOverride{}, tierStore, func(role string) (string, error) {
		return "opus", nil
	})
	if err != nil {
		t.Fatalf("ResolveProvider fallback: %v", err)
	}
	if rp == nil {
		t.Fatal("expected single-provider fallback to ollama-cloud, got nil")
	}
	if rp.Model != "glm-5.2:cloud" {
		t.Errorf("fallback model = %s, want glm-5.2:cloud", rp.Model)
	}
	_ = model
}

// TestAC005_TierFallbackToDefault verifies that a tier with no resolved provider
// returns nil (caller falls back to provider default / opencode default).
func TestAC005_TierFallbackToDefault(t *testing.T) {
	_, store, tierStore := testDB(t)
	providers, _ := store.Providers()
	// Disable all providers → no tier resolution.
	for i := range providers {
		providers[i].Enabled = false
	}
	p, _ := tierStore.ResolveTier("opus", providers)
	if p != nil {
		t.Errorf("opus with all disabled: expected nil, got %s", p.Name)
	}
	// Unknown tier → nil.
	p, _ = tierStore.ResolveTier("nonexistent-tier", providers)
	if p != nil {
		t.Errorf("nonexistent tier: expected nil, got %s", p.Name)
	}
}

// TestAC007_RoleOverridePrecedence verifies an override wins over tier resolution.
func TestAC007_RoleOverridePrecedence(t *testing.T) {
	_, store, tierStore := testDB(t)
	providers, _ := store.Providers()

	// Set an override: architect → anthropic/claude-opus-4 (architect is opus tier, normally anthropic anyway;
	// set an override to ollama-cloud to prove it wins).
	if err := tierStore.UpsertRoleOverride("architect", "ollama-cloud", "glm-5.2:cloud"); err != nil {
		t.Fatalf("UpsertRoleOverride: %v", err)
	}
	overrides, err := tierStore.RoleOverrides()
	if err != nil {
		t.Fatalf("RoleOverrides: %v", err)
	}

	prev := osGetenv
	defer func() { osGetenv = prev }()
	osGetenv = func(name string) string {
		if name == "OLLAMA_API_KEY" {
			return "test-ollama-key"
		}
		return ""
	}
	rp, err := ResolveProvider("architect", providers, overrides, tierStore, func(role string) (string, error) {
		return "opus", nil
	})
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	if rp == nil {
		t.Fatal("expected resolved provider, got nil")
	}
	if rp.Model != "glm-5.2:cloud" {
		t.Errorf("override model = %s, want glm-5.2:cloud", rp.Model)
	}
}

// TestAC007_OverrideRemovalReverts verifies that removing an override (provider="")
// reverts to tier resolution. Traces FR-007 acceptance (c).
func TestAC007_OverrideRemovalReverts(t *testing.T) {
	_, store, tierStore := testDB(t)
	providers, _ := store.Providers()

	// Set then remove an override for "developer".
	if err := tierStore.UpsertRoleOverride("developer", "ollama-cloud", "glm-5.2:cloud"); err != nil {
		t.Fatalf("set override: %v", err)
	}
	if err := tierStore.UpsertRoleOverride("developer", "", ""); err != nil {
		t.Fatalf("remove override: %v", err)
	}
	overrides, _ := tierStore.RoleOverrides()
	if _, exists := overrides["developer"]; exists {
		t.Error("override for developer should be removed")
	}

	// Now developer resolves via tier (opus → anthropic).
	prev := osGetenv
	defer func() { osGetenv = prev }()
	osGetenv = func(name string) string {
		if name == "ANTHROPIC_API_KEY" {
			return "sk-ant-test"
		}
		return ""
	}
	rp, err := ResolveProvider("developer", providers, overrides, tierStore, func(role string) (string, error) {
		return "opus", nil
	})
	if err != nil {
		t.Fatalf("ResolveProvider after removal: %v", err)
	}
	if rp == nil {
		t.Fatal("expected tier resolution, got nil")
	}
	if rp.Model != "claude-opus-4" {
		t.Errorf("after removal, model = %s, want claude-opus-4 (tier resolution)", rp.Model)
	}
}

// TestAC004_PerDispatchResolution verifies ResolveProvider reads fresh each call
// (no caching) and produces the right provider+model+key. Traces U-CFG-01.
func TestAC004_PerDispatchResolution(t *testing.T) {
	_, store, tierStore := testDB(t)
	providers, _ := store.Providers()
	overrides, _ := tierStore.RoleOverrides()

	prev := osGetenv
	defer func() { osGetenv = prev }()
	osGetenv = func(name string) string {
		if name == "ANTHROPIC_API_KEY" {
			return "sk-ant-fresh"
		}
		return ""
	}

	// opus-tier role → anthropic/claude-opus-4 with the key resolved fresh.
	rp1, err := ResolveProvider("architect", providers, overrides, tierStore, func(role string) (string, error) {
		return "opus", nil
	})
	if err != nil {
		t.Fatalf("ResolveProvider 1: %v", err)
	}
	if rp1.Model != "claude-opus-4" {
		t.Errorf("model = %s, want claude-opus-4", rp1.Model)
	}
	if rp1.APIKeyValue != "sk-ant-fresh" {
		t.Errorf("key = %q, want sk-ant-fresh", rp1.APIKeyValue)
	}

	// Change the env getter — second call should see the new value (no caching).
	osGetenv = func(name string) string {
		if name == "ANTHROPIC_API_KEY" {
			return "sk-ant-rotated"
		}
		return ""
	}
	rp2, _ := ResolveProvider("architect", providers, overrides, tierStore, func(role string) (string, error) {
		return "opus", nil
	})
	if rp2.APIKeyValue != "sk-ant-rotated" {
		t.Errorf("second call key = %q, want sk-ant-rotated (fresh read)", rp2.APIKeyValue)
	}
}

// TestAC004_FailFastOnMissingKey verifies that a resolved provider with an unset
// api_key_env returns an error (no fallback, no silent default). Traces R-12, NFR-OP-02.
func TestAC004_FailFastOnMissingKey(t *testing.T) {
	_, store, tierStore := testDB(t)
	providers, _ := store.Providers()
	overrides, _ := tierStore.RoleOverrides()

	prev := osGetenv
	defer func() { osGetenv = prev }()
	osGetenv = func(name string) string { return "" } // all env vars unset

	// opus → anthropic, but ANTHROPIC_API_KEY unset → error.
	_, err := ResolveProvider("architect", providers, overrides, tierStore, func(role string) (string, error) {
		return "opus", nil
	})
	if err == nil {
		t.Fatal("expected error for missing ANTHROPIC_API_KEY, got nil")
	}
}

// TestAC009_EmptyProvidersTreatedAsAbsent verifies FR-006: no providers → nil,nil
// (opencode default), no error.
func TestAC009_EmptyProvidersTreatedAsAbsent(t *testing.T) {
	_, _, tierStore := testDB(t)
	// Empty providers list.
	rp, err := ResolveProvider("architect", []ProviderConfig{}, map[string]RoleOverride{}, tierStore, func(role string) (string, error) {
		return "opus", nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if rp != nil {
		t.Errorf("expected nil resolved provider, got %+v", rp)
	}
}

// TestAC009_ConfigMergerDBWins verifies DB wins on name conflict; YAML-only rows added.
func TestAC009_ConfigMergerDBWins(t *testing.T) {
	d, store, _ := testDB(t)
	merger := NewConfigMerger(store)

	// YAML provider "anthropic" (DB has it too) + YAML-only "yaml-only-prov".
	yamlProviders := []ProviderConfig{
		{Name: "anthropic", DisplayName: "YAML Anthropic (should be ignored)", Enabled: false},
		{Name: "yaml-only-prov", DisplayName: "YAML Only Provider", Enabled: true, DefaultModelID: "yaml-model",
			Models: []ProviderModel{{ModelID: "yaml-model", FriendlyName: "YAML Model"}}},
	}
	merged := merger.MergeProviders(yamlProviders)
	byName := map[string]ProviderConfig{}
	for _, p := range merged {
		byName[p.Name] = p
	}
	// DB anthropic should win (display "Anthropic", enabled=true).
	if byName["anthropic"].DisplayName != "Anthropic" {
		t.Errorf("anthropic display = %q, want 'Anthropic' (DB wins)", byName["anthropic"].DisplayName)
	}
	if !byName["anthropic"].Enabled {
		t.Error("anthropic from DB should be enabled")
	}
	// YAML-only provider should be present.
	if _, ok := byName["yaml-only-prov"]; !ok {
		t.Error("yaml-only-prov should be present")
	}
	// Total: 4 DB + 1 YAML-only = 5.
	if len(merged) != 5 {
		t.Errorf("merged count = %d, want 5", len(merged))
	}
	// Verify deterministic order (sorted by name).
	if len(merged) > 1 && merged[0].Name > merged[1].Name {
		t.Errorf("merged not sorted: %s before %s", merged[0].Name, merged[1].Name)
	}
	_ = d
}

// TestAC009_ConfigMergerEmptyEmpty verifies empty DB + empty YAML → empty result.
func TestAC009_ConfigMergerEmptyEmpty(t *testing.T) {
	d, store, _ := testDB(t)
	// Truncate providers so DB is empty (but store is still wired).
	truncateForConfigTest(d)
	merger := NewConfigMerger(store)
	merged := merger.MergeProviders(nil)
	if len(merged) != 0 {
		t.Errorf("empty+empty merged = %d, want 0", len(merged))
	}
}

// TestAC002_SecretsNeverPersisted verifies the store never writes a raw key value:
// upserting a provider with a raw key in APIKeyEnv is rejected at the handler
// layer (regex), but the store itself should never be the leak point. Here we
// verify the store only writes the api_key_env reference column, never a value.
func TestAC002_SecretsNeverPersisted(t *testing.T) {
	d, store, _ := testDB(t)
	// Upsert with a $VAR reference (the only valid form).
	err := store.UpsertProvider(ProviderConfig{
		Name:      "test-secret",
		DisplayName: "Test",
		APIKeyEnv: "$TEST_SECRET_KEY",
		NPMAdapter: "@ai-sdk/openai-compatible",
		PresetID:   "custom",
		Models:     []ProviderModel{},
	})
	if err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	// Read the raw column back from the DB — it should be exactly "$TEST_SECRET_KEY".
	var apiKeyEnv string
	err = d.QueryRow("SELECT api_key_env FROM providers WHERE name = $1", "test-secret").Scan(&apiKeyEnv)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if apiKeyEnv != "$TEST_SECRET_KEY" {
		t.Errorf("api_key_env column = %q, want $TEST_SECRET_KEY", apiKeyEnv)
	}
	// No column exists for the raw value (schema has no api_key_value column).
	cols, err := d.Conn().Query("SELECT column_name FROM information_schema.columns WHERE table_name = 'providers'")
	if err != nil {
		t.Fatalf("schema query: %v", err)
	}
	defer cols.Close()
	for cols.Next() {
		var col string
		cols.Scan(&col)
		if col == "api_key_value" {
			t.Error("providers table should NOT have an api_key_value column")
		}
	}
}

// TestAC002_APIKeyEnvValidation verifies the api_key_env regex ^\$\w+$ is enforced.
// (The handler enforces this; the store trusts its inputs. This test documents
// the convention by showing valid/invalid forms.)
func TestAC002_APIKeyEnvValidation(t *testing.T) {
	valid := []string{"$ANTHROPIC_API_KEY", "$OLLAMA_API_KEY", ""}
	invalid := []string{"sk-ant-raw-key", "ANTHROPIC_API_KEY", "$has spaces", "$"}
	for _, v := range valid {
		if !validAPIKeyEnv(v) {
			t.Errorf("valid api_key_env %q rejected", v)
		}
	}
	for _, v := range invalid {
		if validAPIKeyEnv(v) {
			t.Errorf("invalid api_key_env %q accepted", v)
		}
	}
}

// validAPIKeyEnv returns true if s matches ^\$\w+$ or is empty. ADR-003.
func validAPIKeyEnv(s string) bool {
	if s == "" {
		return true
	}
	if len(s) < 2 || s[0] != '$' {
		return false
	}
	for _, r := range s[1:] {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// TestAC010_TierStoreCRUD verifies tier_models and role_overrides upsert/read.
func TestAC010_TierStoreCRUD(t *testing.T) {
	_, store, tierStore := testDB(t)
	providers, _ := store.Providers()

	// Upsert a new tier mapping.
	if err := tierStore.UpsertTierModel("sonnet", "anthropic", "claude-sonnet-4"); err != nil {
		t.Fatalf("UpsertTierModel: %v", err)
	}
	// Verify it resolves: sonnet with anthropic enabled → anthropic/claude-sonnet-4.
	p, model := tierStore.ResolveTier("sonnet", providers)
	if p == nil || p.Name != "anthropic" || model != "claude-sonnet-4" {
		t.Errorf("sonnet → %s/%s, want anthropic/claude-sonnet-4", nameOr(p), model)
	}

	// Verify ModelExistsForProvider.
	exists, err := tierStore.ModelExistsForProvider("anthropic", "claude-opus-4")
	if err != nil {
		t.Fatalf("ModelExistsForProvider: %v", err)
	}
	if !exists {
		t.Error("claude-opus-4 should exist for anthropic")
	}
	exists, _ = tierStore.ModelExistsForProvider("anthropic", "nonexistent")
	if exists {
		t.Error("nonexistent model should not exist")
	}

	// Verify ProviderEnabled.
	enabled, err := tierStore.ProviderEnabled("anthropic")
	if err != nil || !enabled {
		t.Errorf("anthropic enabled = %v, want true (err=%v)", enabled, err)
	}
	enabled, _ = tierStore.ProviderEnabled("openai")
	if enabled {
		t.Error("openai should not be enabled")
	}
}

func nameOr(p *ProviderConfig) string {
	if p == nil {
		return "<nil>"
	}
	return p.Name
}

var _ = fmt.Sprintf // keep fmt import if unused in future edits