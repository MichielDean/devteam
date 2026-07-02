package config

import (
	"os"
	"path/filepath"
	"testing"
)

func ptrInt(v int) *int { return &v }
func ptrBool(v bool) *bool { return &v }

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `
version: "2.0"
pipeline:
  human_interaction_timeout_minutes: 30
  phases:
    - name: ideation
      roles: [product]
      rules: rules/aidlc/core-workflow.md
    - name: inception
      roles: [product]
      rules: rules/aidlc-rule-details/inception/
    - name: construction
      roles: [developer]
      rules: rules/aidlc-rule-details/construction/code-generation.md
roles:
  product:
    name: Product Agent
    description: Requirements, user stories, market research
    instructions: roles/product/INSTRUCTIONS.md
  architect:
    name: Architect Agent
    description: App design, domain modeling, NFRs
    instructions: roles/architect/INSTRUCTIONS.md
  developer:
    name: Developer Agent
    description: Code implementation
    instructions: roles/developer/INSTRUCTIONS.md
extensions:
  security:
    opt_in: true
    load_for_priority: [1]
    rules: rules/aidlc-rule-details/extensions/security/baseline/security-baseline.md
intake:
  loose_idea:
    description: Rough idea
    output: [spec.md, acceptance.md, repos.yaml]
  external_spec:
    description: PRD or roadmap
    output: [spec.md, acceptance.md, repos.yaml]
spec_repo:
  path: .
  specs_dir: specs/
  constitution_dir: constitution/
`
	cfgPath := filepath.Join(tmpDir, "devteam.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if len(cfg.Pipeline.Phases) != 3 {
		t.Errorf("expected 3 phases, got %d", len(cfg.Pipeline.Phases))
	}
	if _, ok := cfg.Roles["product"]; !ok {
		t.Error("missing product role")
	}
	if _, ok := cfg.Roles["architect"]; !ok {
		t.Error("missing architect role")
	}
}

func TestLoadConfigValidation(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `
version: "2.0"
pipeline:
  human_interaction_timeout_minutes: -5
roles:
  product:
    name: Product
    description: Product Agent
    instructions: roles/product/INSTRUCTIONS.md
`
	cfgPath := filepath.Join(tmpDir, "devteam.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	_, err := LoadConfig(cfgPath)
	if err == nil {
		t.Fatal("expected validation error for negative timeout, got nil")
	}
}

func TestConfig_DefaultTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `
version: "1.0"
pipeline:
  phases:
    - name: inception
      roles: [pm, architect]
      gate: spec_approved
      artifacts: [spec.md, acceptance.md, repos.yaml]
      rules: rules/aidlc/core-workflow.md
    - name: planning
      roles: [architect]
      gate: plan_approved
      artifacts: [plan.md, tasks.md]
      rules: rules/aidlc-rule-details/construction/
    - name: construction
      roles: [developer]
      gate: tasks_complete
      artifacts: [implementation]
      rules: rules/aidlc-rule-details/construction/code-generation.md
    - name: review
      roles: [reviewer]
      gate: criteria_met
      artifacts: [review_report]
      rules: rules/aidlc-rule-details/construction/functional-design.md
    - name: testing
      roles: [tester]
      gate: tests_pass
      artifacts: [test_report]
      rules: rules/aidlc-rule-details/construction/build-and-test.md
    - name: delivery
      roles: [ops]
      gate: docs_match_spec
      artifacts: [docs, release]
      rules: rules/aidlc-rule-details/operations/operations.md
roles:
  pm:
    name: Product Manager
    description: Owns the what and why
    instructions: roles/pm/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/inception/
  architect:
    name: Architect
    description: Owns the how
    instructions: roles/architect/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/functional-design.md
  developer:
    name: Developer
    description: Writes code
    instructions: roles/developer/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/code-generation.md
  reviewer:
    name: Code Reviewer
    description: Adversarial review
    instructions: roles/reviewer/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/build-and-test.md
  tester:
    name: Tester
    description: Writes and runs tests
    instructions: roles/tester/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/build-and-test.md
  ops:
    name: Release Engineer
    description: Owns deployment and docs
    instructions: roles/ops/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/operations/operations.md
`
	cfgPath := filepath.Join(tmpDir, "devteam.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	// Default timeout should be 30 when not specified
	timeout := cfg.Pipeline.GetHumanInteractionTimeoutMinutes()
	if timeout != 30 {
		t.Errorf("GetHumanInteractionTimeoutMinutes() = %d, want 30 (default)", timeout)
	}
}

func TestConfig_ZeroTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `
version: "1.0"
pipeline:
  human_interaction_timeout_minutes: 0
  phases:
    - name: inception
      roles: [pm, architect]
      gate: spec_approved
      artifacts: [spec.md, acceptance.md, repos.yaml]
      rules: rules/aidlc/core-workflow.md
    - name: planning
      roles: [architect]
      gate: plan_approved
      artifacts: [plan.md, tasks.md]
      rules: rules/aidlc-rule-details/construction/
    - name: construction
      roles: [developer]
      gate: tasks_complete
      artifacts: [implementation]
      rules: rules/aidlc-rule-details/construction/code-generation.md
    - name: review
      roles: [reviewer]
      gate: criteria_met
      artifacts: [review_report]
      rules: rules/aidlc-rule-details/construction/functional-design.md
    - name: testing
      roles: [tester]
      gate: tests_pass
      artifacts: [test_report]
      rules: rules/aidlc-rule-details/construction/build-and-test.md
    - name: delivery
      roles: [ops]
      gate: docs_match_spec
      artifacts: [docs, release]
      rules: rules/aidlc-rule-details/operations/operations.md
roles:
  pm:
    name: PM
    description: Product Manager
    instructions: roles/pm/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/inception/
  architect:
    name: Architect
    description: Architect
    instructions: roles/architect/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/functional-design.md
  developer:
    name: Developer
    description: Developer
    instructions: roles/developer/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/code-generation.md
  reviewer:
    name: Reviewer
    description: Reviewer
    instructions: roles/reviewer/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/build-and-test.md
  tester:
    name: Tester
    description: Tester
    instructions: roles/tester/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/build-and-test.md
  ops:
    name: Ops
    description: Ops
    instructions: roles/ops/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/operations/operations.md
`
	cfgPath := filepath.Join(tmpDir, "devteam.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	timeout := cfg.Pipeline.GetHumanInteractionTimeoutMinutes()
	if timeout != 0 {
		t.Errorf("GetHumanInteractionTimeoutMinutes() = %d, want 0 (fully autonomous)", timeout)
	}
}

func TestConfig_NegativeOneTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `
version: "1.0"
pipeline:
  human_interaction_timeout_minutes: -1
  phases:
    - name: inception
      roles: [pm, architect]
      gate: spec_approved
      artifacts: [spec.md, acceptance.md, repos.yaml]
      rules: rules/aidlc/core-workflow.md
    - name: planning
      roles: [architect]
      gate: plan_approved
      artifacts: [plan.md, tasks.md]
      rules: rules/aidlc-rule-details/construction/
    - name: construction
      roles: [developer]
      gate: tasks_complete
      artifacts: [implementation]
      rules: rules/aidlc-rule-details/construction/code-generation.md
    - name: review
      roles: [reviewer]
      gate: criteria_met
      artifacts: [review_report]
      rules: rules/aidlc-rule-details/construction/functional-design.md
    - name: testing
      roles: [tester]
      gate: tests_pass
      artifacts: [test_report]
      rules: rules/aidlc-rule-details/construction/build-and-test.md
    - name: delivery
      roles: [ops]
      gate: docs_match_spec
      artifacts: [docs, release]
      rules: rules/aidlc-rule-details/operations/operations.md
roles:
  pm:
    name: PM
    description: Product Manager
    instructions: roles/pm/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/inception/
  architect:
    name: Architect
    description: Architect
    instructions: roles/architect/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/functional-design.md
  developer:
    name: Developer
    description: Developer
    instructions: roles/developer/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/code-generation.md
  reviewer:
    name: Reviewer
    description: Reviewer
    instructions: roles/reviewer/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/build-and-test.md
  tester:
    name: Tester
    description: Tester
    instructions: roles/tester/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/build-and-test.md
  ops:
    name: Ops
    description: Ops
    instructions: roles/ops/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/operations/operations.md
`
	cfgPath := filepath.Join(tmpDir, "devteam.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	timeout := cfg.Pipeline.GetHumanInteractionTimeoutMinutes()
	if timeout != -1 {
		t.Errorf("GetHumanInteractionTimeoutMinutes() = %d, want -1 (wait forever)", timeout)
	}
}

func TestConfig_CustomTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `
version: "1.0"
pipeline:
  human_interaction_timeout_minutes: 5
  phases:
    - name: inception
      roles: [pm, architect]
      gate: spec_approved
      artifacts: [spec.md, acceptance.md, repos.yaml]
      rules: rules/aidlc/core-workflow.md
    - name: planning
      roles: [architect]
      gate: plan_approved
      artifacts: [plan.md, tasks.md]
      rules: rules/aidlc-rule-details/construction/
    - name: construction
      roles: [developer]
      gate: tasks_complete
      artifacts: [implementation]
      rules: rules/aidlc-rule-details/construction/code-generation.md
    - name: review
      roles: [reviewer]
      gate: criteria_met
      artifacts: [review_report]
      rules: rules/aidlc-rule-details/construction/functional-design.md
    - name: testing
      roles: [tester]
      gate: tests_pass
      artifacts: [test_report]
      rules: rules/aidlc-rule-details/construction/build-and-test.md
    - name: delivery
      roles: [ops]
      gate: docs_match_spec
      artifacts: [docs, release]
      rules: rules/aidlc-rule-details/operations/operations.md
roles:
  pm:
    name: PM
    description: Product Manager
    instructions: roles/pm/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/inception/
  architect:
    name: Architect
    description: Architect
    instructions: roles/architect/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/functional-design.md
  developer:
    name: Developer
    description: Developer
    instructions: roles/developer/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/code-generation.md
  reviewer:
    name: Reviewer
    description: Reviewer
    instructions: roles/reviewer/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/build-and-test.md
  tester:
    name: Tester
    description: Tester
    instructions: roles/tester/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/construction/build-and-test.md
  ops:
    name: Ops
    description: Ops
    instructions: roles/ops/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/operations/operations.md
`
	cfgPath := filepath.Join(tmpDir, "devteam.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	timeout := cfg.Pipeline.GetHumanInteractionTimeoutMinutes()
	if timeout != 5 {
		t.Errorf("GetHumanInteractionTimeoutMinutes() = %d, want 5", timeout)
	}
}

func TestLoadRepos(t *testing.T) {
	tmpDir := t.TempDir()
	reposContent := `
repos:
  - name: devteam
    url: git@github.com:MichielDean/devteam.git
    description: Dev Team platform
    primary: true
  - name: cistern
    url: git@github.com:MichielDean/cistern.git
    description: Workflow orchestrator
`
	reposPath := filepath.Join(tmpDir, "repos.yaml")
	if err := os.WriteFile(reposPath, []byte(reposContent), 0644); err != nil {
		t.Fatalf("writing repos: %v", err)
	}

	repos, err := LoadRepos(reposPath)
	if err != nil {
		t.Fatalf("LoadRepos() error: %v", err)
	}
	if len(repos.Repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(repos.Repos))
	}
	if repos.Repos[0].Name != "devteam" {
		t.Errorf("expected first repo to be devteam, got %s", repos.Repos[0].Name)
	}
	if !repos.Repos[0].Primary {
		t.Error("expected devteam to be primary")
	}
}

// --- rate_limit config tests (rate-limiting-middleware feature) ---
//
// Per-case TestXxx (R9), stdlib testing, no DB (E14). The YAML key is
// `rate_limit` (NOT `rate_limiting` — that was a 2.6 reversal corrected by the
// LOCKED 2.5 M3.1 decision; see business-logic-model §0 C-6).

func TestLoadConfigAbsentRateLimitSection(t *testing.T) {
	// R12: absent rate_limit block → zero value, Enabled=false (passthrough).
	cfg := mustLoadConfig(t, ``)
	if cfg.RateLimit.Enabled {
		t.Error("absent rate_limit block: Enabled should be false (passthrough)")
	}
}

func TestLoadConfigPresentRateLimitDefaults(t *testing.T) {
	cfg := mustLoadConfig(t, `
rate_limit:
  enabled: true
  defaults:
    limit: 300
    window_seconds: 60
`)
	if !cfg.RateLimit.Enabled {
		t.Error("enabled: true parsed as false")
	}
	if cfg.RateLimit.GetDefaultLimit() != 300 {
		t.Errorf("default limit = %d, want 300", cfg.RateLimit.GetDefaultLimit())
	}
	if cfg.RateLimit.GetDefaultWindowSeconds() != 60 {
		t.Errorf("default window = %d, want 60", cfg.RateLimit.GetDefaultWindowSeconds())
	}
}

func TestLoadConfigOptionalFieldsDefaultViaGetter(t *testing.T) {
	// R5: omitted optional fields → documented default via getter.
	cfg := mustLoadConfig(t, `
rate_limit:
  enabled: true
`)
	if cfg.RateLimit.GetTrustProxyHeaders() {
		t.Error("omitted trust_proxy_headers: getter should return false (D2 default)")
	}
	if cfg.RateLimit.GetDryRun() {
		t.Error("omitted dry_run: getter should return false")
	}
	if got := cfg.RateLimit.GetMaxTrackedKeys(); got != 10000 {
		t.Errorf("omitted max_tracked_keys: getter = %d, want 10000 (O-7)", got)
	}
	if got := cfg.RateLimit.GetDefaultLimit(); got != 100 {
		t.Errorf("omitted defaults.limit: getter = %d, want 100 (O-1)", got)
	}
	if got := cfg.RateLimit.GetDefaultWindowSeconds(); got != 60 {
		t.Errorf("omitted defaults.window_seconds: getter = %d, want 60 (O-1)", got)
	}
}

func TestLoadConfigEndpointOverridesParse(t *testing.T) {
	cfg := mustLoadConfig(t, `
rate_limit:
  enabled: true
  endpoint_overrides:
    "POST /v1/run":
      limit: 300
      window_seconds: 60
`)
	ov, ok := cfg.RateLimit.EndpointOverrides["POST /v1/run"]
	if !ok {
		t.Fatal("endpoint override POST /v1/run not parsed")
	}
	if ov.Limit == nil || *ov.Limit != 300 {
		t.Error("override limit not parsed")
	}
}

func TestValidateRateLimitRejectsBadFailMode(t *testing.T) {
	// BR-01: fail_mode must be fail_open.
	c := &RateLimitConfig{FailMode: "fail_closed"}
	if err := c.Validate(); err == nil {
		t.Error("fail_mode=fail_closed: expected validation error")
	}
}

func TestValidateRateLimitRejectsNegativeLimit(t *testing.T) {
	// BR-02: defaults.limit <= 0 → error.
	c := &RateLimitConfig{Defaults: RateLimitDefaults{Limit: ptrInt(-5)}}
	if err := c.Validate(); err == nil {
		t.Error("defaults.limit=-5: expected validation error")
	}
}

func TestValidateRateLimitRejectsZeroLimit(t *testing.T) {
	c := &RateLimitConfig{Defaults: RateLimitDefaults{Limit: ptrInt(0)}}
	if err := c.Validate(); err == nil {
		t.Error("defaults.limit=0: expected validation error")
	}
}

func TestValidateRateLimitRejectsZeroWindow(t *testing.T) {
	// BR-03: defaults.window_seconds <= 0 → error.
	c := &RateLimitConfig{Defaults: RateLimitDefaults{WindowSeconds: ptrInt(0)}}
	if err := c.Validate(); err == nil {
		t.Error("defaults.window_seconds=0: expected validation error")
	}
}

func TestValidateRateLimitRejectsNegativeWindow(t *testing.T) {
	c := &RateLimitConfig{Defaults: RateLimitDefaults{WindowSeconds: ptrInt(-1)}}
	if err := c.Validate(); err == nil {
		t.Error("defaults.window_seconds=-1: expected validation error")
	}
}

func TestValidateRateLimitRejectsBadOverrideKey(t *testing.T) {
	// BR-04: override keys must match "METHOD /path".
	cases := []string{"post /v1/run", "/v1/run", "GET v1/run"}
	for _, k := range cases {
		c := &RateLimitConfig{EndpointOverrides: map[string]RateLimitOverride{k: {Limit: ptrInt(100)}}}
		if err := c.Validate(); err == nil {
			t.Errorf("bad override key %q: expected validation error", k)
		}
	}
	// Valid key passes (given a non-empty override).
	c := &RateLimitConfig{EndpointOverrides: map[string]RateLimitOverride{"GET /v1/run": {Limit: ptrInt(100)}}}
	if err := c.Validate(); err != nil {
		t.Errorf("valid override key: unexpected error %v", err)
	}
}

func TestValidateRateLimitRejectsEmptyOverride(t *testing.T) {
	// BR-05: empty override → error.
	c := &RateLimitConfig{EndpointOverrides: map[string]RateLimitOverride{"POST /v1/run": {}}}
	if err := c.Validate(); err == nil {
		t.Error("empty override: expected validation error")
	}
}

func TestValidateRateLimitRejectsExemptWithLimit(t *testing.T) {
	// BR-07: exempt + limit on the same override → error.
	c := &RateLimitConfig{EndpointOverrides: map[string]RateLimitOverride{
		"GET /health": {Exempt: true, Limit: ptrInt(100)},
	}}
	if err := c.Validate(); err == nil {
		t.Error("exempt+limit override: expected validation error")
	}
}

func TestValidateRateLimitRejectsZeroMaxTrackedKeys(t *testing.T) {
	// BR-06: max_tracked_keys < 100 → error (hard floor 100).
	c := &RateLimitConfig{MaxTrackedKeys: ptrInt(0)}
	if err := c.Validate(); err == nil {
		t.Error("max_tracked_keys=0: expected validation error")
	}
}

func TestValidateRateLimitRejectsLowMaxTrackedKeys(t *testing.T) {
	// BR-06: max_tracked_keys=10 (< 100 floor) → error.
	c := &RateLimitConfig{MaxTrackedKeys: ptrInt(10)}
	if err := c.Validate(); err == nil {
		t.Error("max_tracked_keys=10: expected validation error")
	}
}

func TestLoadConfigInvalidRateLimitDoesNotCrash(t *testing.T) {
	// ADR-008/O-5: a malformed rate_limit block must NOT crash LoadConfig. The
	// structural parse may still fail on bad YAML (that's a different section's
	// concern), but a semantically-invalid rate_limit block (e.g. bad fail_mode)
	// must NOT be rejected by LoadConfig — validation is deferred to
	// RateLimitConfig.Validate(), called by ConfigureRateLimiting (U-W). This
	// keeps the existing fatal validateConfig path untouched.
	cfg := mustLoadConfig(t, `
rate_limit:
  enabled: true
  fail_mode: fail_closed
`)
	// LoadConfig succeeded (no error); the invalid fail_mode is NOT checked here.
	if cfg == nil {
		t.Fatal("LoadConfig returned nil for semantically-invalid rate_limit block")
	}
	// Validate surfaces the error (this is the U-W call site).
	if err := cfg.RateLimit.Validate(); err == nil {
		t.Error("Validate should reject fail_mode=fail_closed")
	}
}

func mustLoadConfig(t *testing.T, rateLimitBlock string) *Config {
	t.Helper()
	tmpDir := t.TempDir()
	// Minimal valid config plus the rate_limit block under test.
	cfgContent := "version: \"2.0\"\npipeline:\n  human_interaction_timeout_minutes: 30\nroles:\n  product:\n    name: Product\n    description: x\n    instructions: x\n" + rateLimitBlock
	cfgPath := filepath.Join(tmpDir, "devteam.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	return cfg
}
