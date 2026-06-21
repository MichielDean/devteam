package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
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
	if len(cfg.Pipeline.Phases) != 6 {
		t.Errorf("expected 6 phases, got %d", len(cfg.Pipeline.Phases))
	}
	if _, ok := cfg.Roles["pm"]; !ok {
		t.Error("missing pm role")
	}
	if _, ok := cfg.Roles["architect"]; !ok {
		t.Error("missing architect role")
	}
}

func TestLoadConfigValidation(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `
version: "1.0"
pipeline:
  phases:
    - name: inception
      roles: [pm]
      gate: spec_approved
      artifacts: [spec.md]
      rules: rules/aidlc/core-workflow.md
roles:
  pm:
    name: PM
    description: Product Manager
    instructions: roles/pm/INSTRUCTIONS.md
    phase_rules: rules/aidlc-rule-details/inception/
`
	cfgPath := filepath.Join(tmpDir, "devteam.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	_, err := LoadConfig(cfgPath)
	if err == nil {
		t.Fatal("expected validation error for missing phases and roles, got nil")
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
