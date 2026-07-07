package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// TestStreamingConfigDefaults verifies the streaming config block defaults
// when the block is absent (U-BK-01).
func TestStreamingConfigDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `
version: "2.0"
pipeline:
  human_interaction_timeout_minutes: 30
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

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if cfg.Pipeline.Streaming.GetLogFileFallback() {
		t.Error("GetLogFileFallback() = true, want false (default-off, ADR-1)")
	}
	if got := cfg.Pipeline.Streaming.GetFlushIntervalMs(); got != 200 {
		t.Errorf("GetFlushIntervalMs() = %d, want 200 (NFR-1 default)", got)
	}
	if got := cfg.Pipeline.Streaming.GetFlushBytes(); got != 8192 {
		t.Errorf("GetFlushBytes() = %d, want 8192", got)
	}
	if got := cfg.Pipeline.Streaming.GetRenderCapLines(); got != 5000 {
		t.Errorf("GetRenderCapLines() = %d, want 5000 (FR-15)", got)
	}
}

// TestStreamingConfigExplicit verifies explicit values are honored (U-BK-01).
func TestStreamingConfigExplicit(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `
version: "2.0"
pipeline:
  human_interaction_timeout_minutes: 30
  streaming:
    log_file_fallback: true
    flush_interval_ms: 100
    flush_bytes: 4096
    render_cap_lines: 1000
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

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if !cfg.Pipeline.Streaming.GetLogFileFallback() {
		t.Error("GetLogFileFallback() = false, want true (explicit)")
	}
	if got := cfg.Pipeline.Streaming.GetFlushIntervalMs(); got != 100 {
		t.Errorf("GetFlushIntervalMs() = %d, want 100", got)
	}
	if got := cfg.Pipeline.Streaming.GetFlushBytes(); got != 4096 {
		t.Errorf("GetFlushBytes() = %d, want 4096", got)
	}
	if got := cfg.Pipeline.Streaming.GetRenderCapLines(); got != 1000 {
		t.Errorf("GetRenderCapLines() = %d, want 1000", got)
	}
}

// TestStreamingConfigZeroUsesDefault verifies that a value of 0 means "use default"
// (not "never flush") — the getter substitutes the default for <= 0 (U-BK-01).
func TestStreamingConfigZeroUsesDefault(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `
version: "2.0"
pipeline:
  streaming:
    flush_interval_ms: 0
    flush_bytes: 0
    render_cap_lines: 0
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

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if got := cfg.Pipeline.Streaming.GetFlushIntervalMs(); got != 200 {
		t.Errorf("GetFlushIntervalMs() with 0 = %d, want 200 (default)", got)
	}
	if got := cfg.Pipeline.Streaming.GetFlushBytes(); got != 8192 {
		t.Errorf("GetFlushBytes() with 0 = %d, want 8192 (default)", got)
	}
	if got := cfg.Pipeline.Streaming.GetRenderCapLines(); got != 5000 {
		t.Errorf("GetRenderCapLines() with 0 = %d, want 5000 (default)", got)
	}
}

// TestStreamingConfigRejectsNegative verifies negative threshold values are rejected at load
// (U-BK-01 / app-design §10 config-misparse guard).
func TestStreamingConfigRejectsNegative(t *testing.T) {
	cases := []struct {
		name    string
		block   string
		wantSub string
	}{
		{
			name:    "negative flush_interval_ms",
			block:  "streaming:\n    flush_interval_ms: -1\n",
			wantSub: "flush_interval_ms",
		},
		{
			name:    "negative flush_bytes",
			block:  "streaming:\n    flush_bytes: -1\n",
			wantSub: "flush_bytes",
		},
		{
			name:    "negative render_cap_lines",
			block:  "streaming:\n    render_cap_lines: -1\n",
			wantSub: "render_cap_lines",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfgContent := "version: \"2.0\"\npipeline:\n  " + c.block + "roles:\n  product:\n    name: Product\n    description: x\n    instructions: x\n"
			cfgPath := filepath.Join(tmpDir, "devteam.yaml")
			if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
				t.Fatalf("writing config: %v", err)
			}
			_, err := LoadConfig(cfgPath)
			if err == nil {
				t.Fatalf("expected validation error for %s, got nil", c.name)
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Errorf("error %q does not mention %q", err.Error(), c.wantSub)
			}
		})
	}
}
