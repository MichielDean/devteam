package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuleLoaderPhaseRules(t *testing.T) {
	tmpDir := t.TempDir()
	phaseDir := filepath.Join(tmpDir, "rules", "aidlc-rule-details", "inception")
	if err := os.MkdirAll(phaseDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phaseDir, "requirements-analysis.md"), []byte("# Requirements Analysis\n\nTest content."), 0644); err != nil {
		t.Fatal(err)
	}

	rl := NewRuleLoader(tmpDir)
	rules, err := rl.PhaseRules("inception")
	if err != nil {
		t.Fatalf("PhaseRules() error: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("expected 1 rule file, got %d", len(rules))
	}
}

func TestRuleLoaderRoleRules(t *testing.T) {
	tmpDir := t.TempDir()
	roleDir := filepath.Join(tmpDir, "roles", "pm")
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(roleDir, "INSTRUCTIONS.md"), []byte("# PM Role\n\nYou are the Product Manager."), 0644); err != nil {
		t.Fatal(err)
	}

	rl := NewRuleLoader(tmpDir)
	rules, err := rl.RoleRules("pm")
	if err != nil {
		t.Fatalf("RoleRules() error: %v", err)
	}
	if len(rules) == 0 {
		t.Error("expected non-empty role rules")
	}
}

func TestRuleLoaderBuildContext(t *testing.T) {
	tmpDir := setupTestRules(t)
	rl := NewRuleLoader(tmpDir)

	ctx, err := rl.BuildContext("inception", "pm", 2)
	if err != nil {
		t.Fatalf("BuildContext() error: %v", err)
	}
	if len(ctx) == 0 {
		t.Error("expected non-empty context")
	}
}

func setupTestRules(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	dirs := []string{
		filepath.Join(tmpDir, "rules", "aidlc"),
		filepath.Join(tmpDir, "rules", "aidlc-rule-details", "inception"),
		filepath.Join(tmpDir, "rules", "aidlc-rule-details", "construction"),
		filepath.Join(tmpDir, "rules", "aidlc-rule-details", "operations"),
		filepath.Join(tmpDir, "roles", "pm"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "rules", "aidlc", "core-workflow.md"), []byte("# Core Workflow\n\nTest workflow."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "rules", "aidlc-rule-details", "inception", "requirements-analysis.md"), []byte("# Requirements Analysis\n\nTest content."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "roles", "pm", "INSTRUCTIONS.md"), []byte("# PM Role\n\nYou are the Product Manager."), 0644); err != nil {
		t.Fatal(err)
	}

	return tmpDir
}