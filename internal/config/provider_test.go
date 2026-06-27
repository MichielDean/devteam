package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// baseValidConfigYAML returns a minimal valid devteam.yaml body (6 phases,
// 6 roles) with no providers: section — the pre-feature / backward-compat
// shape. Tests append provider snippets to this to exercise the new schema.
func baseValidConfigYAML() string {
	return `
version: "1.0"
pipeline:
  phases:
    - {name: inception, roles: [pm, architect], gate: spec_approved, artifacts: [spec.md], rules: r}
    - {name: planning, roles: [architect], gate: plan_approved, artifacts: [plan.md], rules: r}
    - {name: construction, roles: [developer], gate: tasks_complete, artifacts: [impl], rules: r}
    - {name: review, roles: [reviewer], gate: criteria_met, artifacts: [review_report], rules: r}
    - {name: testing, roles: [tester], gate: tests_pass, artifacts: [test_report], rules: r}
    - {name: delivery, roles: [ops], gate: docs_match_spec, artifacts: [docs], rules: r}
roles:
  pm: {name: PM, description: pm, instructions: r, phase_rules: r}
  architect: {name: Architect, description: a, instructions: r, phase_rules: r}
  developer: {name: Developer, description: d, instructions: r, phase_rules: r}
  reviewer: {name: Reviewer, description: rv, instructions: r, phase_rules: r}
  tester: {name: Tester, description: t, instructions: r, phase_rules: r}
  ops: {name: Ops, description: o, instructions: r, phase_rules: r}
`
}

func writeCfg(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "devteam.yaml")
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	return p
}

// AC-005 [US-1] [CON-007/F R-007] duplicate provider name rejected at load.
func TestProviders_DuplicateNameRejected(t *testing.T) {
	body := baseValidConfigYAML() + `
providers:
  - {name: p1, base_url: http://x, api_key_env: K, model: m1}
  - {name: p1, base_url: http://y, api_key_env: K2, model: m2}
`
	_, err := LoadConfig(writeCfg(t, body))
	if err == nil {
		t.Fatal("expected duplicate-provider error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate provider name: p1") {
		t.Errorf("error %q does not mention 'duplicate provider name: p1'", err.Error())
	}
}

// AC-006 [US-1] [FR-007] role references unknown provider rejected at load.
func TestProviders_UnknownProviderReferenceRejected(t *testing.T) {
	body := baseValidConfigYAML() + `
providers:
  - {name: p1, base_url: http://x, api_key_env: K, model: m1}
role_providers:
  developer: {provider: ghost}
`
	_, err := LoadConfig(writeCfg(t, body))
	if err == nil {
		t.Fatal("expected unknown-provider error, got nil")
	}
	want := "role 'developer' references unknown provider 'ghost'"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

// AC-007 [US-1] [FR-007] empty base_url rejected at load.
func TestProviders_EmptyBaseURLRejected(t *testing.T) {
	body := baseValidConfigYAML() + `
providers:
  - {name: p1, base_url: "", api_key_env: K, model: m1}
`
	_, err := LoadConfig(writeCfg(t, body))
	if err == nil {
		t.Fatal("expected empty base_url error, got nil")
	}
	want := "provider 'p1' has empty base_url"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

// AC-008 [US-1] [FR-007] no model on role and no default on provider rejected.
func TestProviders_NoModelAnywhereRejected(t *testing.T) {
	body := baseValidConfigYAML() + `
providers:
  - {name: p1, base_url: http://x, api_key_env: K}
role_providers:
  pm: {provider: p1}
`
	_, err := LoadConfig(writeCfg(t, body))
	if err == nil {
		t.Fatal("expected no-model error, got nil")
	}
	want := "role 'pm' has no model and provider 'p1' has no default model"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

// AC-009 [US-4] [FR-006] empty providers map treated as absent (no error).
// The implementation uses []ProviderConfig (a slice), so `providers: {}` in
// YAML deserializes to an empty slice. Verify load succeeds and dispatch
// falls back (ResolveProvider returns nil).
func TestProviders_EmptyMapTreatedAsAbsent(t *testing.T) {
	body := baseValidConfigYAML() + `
providers: []
`
	cfg, err := LoadConfig(writeCfg(t, body))
	if err != nil {
		t.Fatalf("empty providers should load, got: %v", err)
	}
	rp, err := cfg.ResolveProvider("pm")
	if err != nil {
		t.Fatalf("ResolveProvider on empty providers: %v", err)
	}
	if rp != nil {
		t.Errorf("expected nil ResolvedProvider for empty providers, got %+v", rp)
	}
}

// AC-003 [US-1] [CON-010, FR-006] no providers: key → load OK, resolve nil.
func TestProviders_AbsentSectionFallsBack(t *testing.T) {
	cfg, err := LoadConfig(writeCfg(t, baseValidConfigYAML()))
	if err != nil {
		t.Fatalf("no-providers config should load, got: %v", err)
	}
	for _, role := range []string{"pm", "architect", "developer", "reviewer", "tester", "ops"} {
		rp, err := cfg.ResolveProvider(role)
		if err != nil {
			t.Errorf("role %s: ResolveProvider err = %v, want nil", role, err)
		}
		if rp != nil {
			t.Errorf("role %s: expected nil fallback, got %+v", role, rp)
		}
	}
}

// AC-004 [US-1] [CON-010, FR-006] role with no mapping → fallback (nil).
func TestProviders_UnmappedRoleFallsBack(t *testing.T) {
	body := baseValidConfigYAML() + `
providers:
  - {name: p1, base_url: http://x, api_key_env: K, model: m1}
role_providers:
  pm: {provider: p1}
`
	cfg, err := LoadConfig(writeCfg(t, body))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	rp, err := cfg.ResolveProvider("reviewer")
	if err != nil {
		t.Fatalf("ResolveProvider(reviewer): %v", err)
	}
	if rp != nil {
		t.Errorf("unmapped reviewer: expected nil, got %+v", rp)
	}
}

// AC-001 [US-1] [CON-002, CON-006, FR-003] mapped role resolves to the
// provider's base_url + the role's model.
func TestResolveProvider_MappedRoleUsesRoleModel(t *testing.T) {
	body := baseValidConfigYAML() + `
providers:
  - {name: p1, base_url: http://x, api_key_env: TESTPROV_K, model: default-model}
role_providers:
  pm: {provider: p1, model: m1}
`
	t.Setenv("TESTPROV_K", "sk-test")
	cfg, err := LoadConfig(writeCfg(t, body))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	rp, err := cfg.ResolveProvider("pm")
	if err != nil {
		t.Fatalf("ResolveProvider(pm): %v", err)
	}
	if rp == nil {
		t.Fatal("expected non-nil resolved provider")
	}
	if rp.BaseURL != "http://x" {
		t.Errorf("BaseURL = %q, want http://x", rp.BaseURL)
	}
	if rp.Model != "m1" {
		t.Errorf("Model = %q, want m1 (role override)", rp.Model)
	}
	if rp.APIKeyEnv != "TESTPROV_K" {
		t.Errorf("APIKeyEnv = %q, want TESTPROV_K", rp.APIKeyEnv)
	}
	if rp.APIKeyValue != "sk-test" {
		t.Errorf("APIKeyValue = %q, want sk-test", rp.APIKeyValue)
	}
}

// AC-002 [US-1] [CON-006, FR-002] two providers, two roles → different
// base_urls per role.
func TestResolveProvider_PerRoleResolution(t *testing.T) {
	body := baseValidConfigYAML() + `
providers:
  - {name: anthropic, base_url: http://a, api_key_env: TESTPROV_A, model: ma}
  - {name: local, base_url: http://b, api_key_env: TESTPROV_B, model: mb}
role_providers:
  pm: {provider: anthropic}
  developer: {provider: local}
`
	t.Setenv("TESTPROV_A", "ka")
	t.Setenv("TESTPROV_B", "kb")
	cfg, err := LoadConfig(writeCfg(t, body))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	pm, err := cfg.ResolveProvider("pm")
	if err != nil {
		t.Fatalf("pm: %v", err)
	}
	dev, err := cfg.ResolveProvider("developer")
	if err != nil {
		t.Fatalf("developer: %v", err)
	}
	if pm.BaseURL != "http://a" {
		t.Errorf("pm BaseURL = %q, want http://a", pm.BaseURL)
	}
	if dev.BaseURL != "http://b" {
		t.Errorf("developer BaseURL = %q, want http://b", dev.BaseURL)
	}
	if pm.Model != "ma" {
		t.Errorf("pm Model = %q, want ma", pm.Model)
	}
	if dev.Model != "mb" {
		t.Errorf("developer Model = %q, want mb", dev.Model)
	}
}

// AC-002 supplementary: role model overrides provider default model.
func TestResolveProvider_RoleModelOverridesDefault(t *testing.T) {
	body := baseValidConfigYAML() + `
providers:
  - {name: p1, base_url: http://x, api_key_env: TESTPROV_K, model: provider-default}
role_providers:
  pm: {provider: p1, model: role-override}
`
	t.Setenv("TESTPROV_K", "k")
	cfg, _ := LoadConfig(writeCfg(t, body))
	rp, _ := cfg.ResolveProvider("pm")
	if rp.Model != "role-override" {
		t.Errorf("Model = %q, want role-override", rp.Model)
	}
}

// AC-002 supplementary: empty role model falls back to provider default.
func TestResolveProvider_EmptyRoleModelUsesProviderDefault(t *testing.T) {
	body := baseValidConfigYAML() + `
providers:
  - {name: p1, base_url: http://x, api_key_env: TESTPROV_K, model: provider-default}
role_providers:
  pm: {provider: p1}
`
	t.Setenv("TESTPROV_K", "k")
	cfg, _ := LoadConfig(writeCfg(t, body))
	rp, _ := cfg.ResolveProvider("pm")
	if rp.Model != "provider-default" {
		t.Errorf("Model = %q, want provider-default", rp.Model)
	}
}

// AC-012 [US-2] [CON-005, FR-005] missing env var → error, no spawn.
// ResolveProvider is the fail-fast gate before opencode is spawned.
func TestResolveProvider_MissingEnvVarFailsFast(t *testing.T) {
	body := baseValidConfigYAML() + `
providers:
  - {name: p1, base_url: http://x, api_key_env: TESTPROV_MISSING, model: m1}
role_providers:
  pm: {provider: p1}
`
	// Ensure unset (t.Setenv with empty would make it set-but-empty; use os.Unsetenv).
	os.Unsetenv("TESTPROV_MISSING")
	cfg, err := LoadConfig(writeCfg(t, body))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	rp, err := cfg.ResolveProvider("pm")
	if err == nil {
		t.Fatal("expected missing-env error, got nil")
	}
	if rp != nil {
		t.Errorf("expected nil resolved provider on error, got %+v", rp)
	}
	want := "api key env var 'TESTPROV_MISSING' is not set"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

// AC-013 [US-2] [FR-005] set-but-empty env var treated as missing.
func TestResolveProvider_EmptyEnvVarFailsFast(t *testing.T) {
	body := baseValidConfigYAML() + `
providers:
  - {name: p1, base_url: http://x, api_key_env: TESTPROV_EMPTY, model: m1}
role_providers:
  pm: {provider: p1}
`
	t.Setenv("TESTPROV_EMPTY", "")
	cfg, _ := LoadConfig(writeCfg(t, body))
	rp, err := cfg.ResolveProvider("pm")
	if err == nil {
		t.Fatal("expected empty-env error, got nil")
	}
	if rp != nil {
		t.Errorf("expected nil resolved provider, got %+v", rp)
	}
	if !strings.Contains(err.Error(), "TESTPROV_EMPTY") {
		t.Errorf("error %q should name TESTPROV_EMPTY", err.Error())
	}
}

// AC-010 [US-2] [CON-004, FR-004] config file never contains the key value,
// only the env var name. (Verified by inspecting the YAML we write — the
// schema only has api_key_env, never a key-value field.)
func TestProviders_ConfigSchemaStoresOnlyEnvVarName(t *testing.T) {
	body := baseValidConfigYAML() + `
providers:
  - {name: p1, base_url: http://x, api_key_env: TESTPROV_K, model: m1}
role_providers:
  pm: {provider: p1}
`
	t.Setenv("TESTPROV_K", "sk-NEVER-IN-CONFIG")
	cfg, err := LoadConfig(writeCfg(t, body))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(cfg.Providers))
	}
	p := cfg.Providers[0]
	if p.APIKeyEnv != "TESTPROV_K" {
		t.Errorf("APIKeyEnv = %q, want TESTPROV_K", p.APIKeyEnv)
	}
	// No field on ProviderConfig holds a key value.
	// Sanity: the resolved value is read from env, not config.
	rp, _ := cfg.ResolveProvider("pm")
	if rp.APIKeyValue != "sk-NEVER-IN-CONFIG" {
		t.Errorf("APIKeyValue = %q, want sk-NEVER-IN-CONFIG", rp.APIKeyValue)
	}
}

// AC-021 [US-1] [CON-009, FR-009] model is a plain string; no model-listing.
// (Structural: ProviderConfig.Model is string, already verified by compile.
// This test asserts the default-model path works with arbitrary strings.)
func TestResolveProvider_FreeFormModelString(t *testing.T) {
	body := baseValidConfigYAML() + `
providers:
  - {name: p1, base_url: http://x, api_key_env: TESTPROV_K, model: "some-vend-or/weird-model-id-v1.2"}
role_providers:
  pm: {provider: p1}
`
	t.Setenv("TESTPROV_K", "k")
	cfg, _ := LoadConfig(writeCfg(t, body))
	rp, _ := cfg.ResolveProvider("pm")
	if rp.Model != "some-vend-or/weird-model-id-v1.2" {
		t.Errorf("Model = %q, want free-form string passthrough", rp.Model)
	}
}

// Edge case: all six roles mapped to the same provider (spec says valid).
func TestProviders_AllRolesSameProvider(t *testing.T) {
	body := baseValidConfigYAML() + `
providers:
  - {name: p1, base_url: http://x, api_key_env: TESTPROV_K, model: m1}
role_providers:
  pm: {provider: p1}
  architect: {provider: p1}
  developer: {provider: p1}
  reviewer: {provider: p1}
  tester: {provider: p1}
  ops: {provider: p1}
`
	t.Setenv("TESTPROV_K", "k")
	cfg, err := LoadConfig(writeCfg(t, body))
	if err != nil {
		t.Fatalf("all-roles-same-provider should load: %v", err)
	}
	for _, role := range []string{"pm", "architect", "developer", "reviewer", "tester", "ops"} {
		rp, err := cfg.ResolveProvider(role)
		if err != nil {
			t.Errorf("role %s: %v", role, err)
		}
		if rp == nil || rp.BaseURL != "http://x" {
			t.Errorf("role %s: expected http://x, got %+v", role, rp)
		}
	}
}

// Edge case: role mapping with empty provider string → fallback, not error.
func TestProviders_EmptyProviderMappingFallsBack(t *testing.T) {
	body := baseValidConfigYAML() + `
providers:
  - {name: p1, base_url: http://x, api_key_env: TESTPROV_K, model: m1}
role_providers:
  pm: {provider: ""}
`
	cfg, err := LoadConfig(writeCfg(t, body))
	if err != nil {
		t.Fatalf("empty-provider mapping should load: %v", err)
	}
	rp, err := cfg.ResolveProvider("pm")
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	if rp != nil {
		t.Errorf("empty provider mapping: expected nil fallback, got %+v", rp)
	}
}