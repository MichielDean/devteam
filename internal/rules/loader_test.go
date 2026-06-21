package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuleLoaderPhaseRules(t *testing.T) {
	tmpDir := t.TempDir()
	phaseDir := filepath.Join(tmpDir, "rules", "pipeline", "inception")
	if err := os.MkdirAll(phaseDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phaseDir, "rules.md"), []byte("# Inception Rules\n\nTest content."), 0644); err != nil {
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

func TestRuleLoaderBuildContextWithExtensions(t *testing.T) {
	tmpDir := setupTestRules(t)

	extDirs := []string{
		filepath.Join(tmpDir, "rules", "pipeline", "extensions", "error-recovery"),
		filepath.Join(tmpDir, "rules", "pipeline", "extensions", "overconfidence-prevention"),
		filepath.Join(tmpDir, "rules", "pipeline", "extensions", "security"),
		filepath.Join(tmpDir, "rules", "pipeline", "extensions", "resiliency"),
	}
	for _, dir := range extDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	extContent := []struct {
		dir  string
		file string
	}{
		{extDirs[0], "rules.md"},
		{extDirs[1], "rules.md"},
		{extDirs[2], "rules.md"},
		{extDirs[3], "rules.md"},
	}
	for _, ec := range extContent {
		if err := os.WriteFile(filepath.Join(ec.dir, ec.file), []byte("# Extension\n\nTest content."), 0644); err != nil {
			t.Fatal(err)
		}
	}

	rl := NewRuleLoader(tmpDir)

	t.Run("P1 loads all extensions", func(t *testing.T) {
		ctx, err := rl.BuildContext("inception", "pm", 1)
		if err != nil {
			t.Fatalf("BuildContext() error: %v", err)
		}
		if !contains(ctx, "Extension: error-recovery") {
			t.Error("P1 context missing error-recovery extension")
		}
		if !contains(ctx, "Extension: overconfidence-prevention") {
			t.Error("P1 context missing overconfidence-prevention extension")
		}
		if !contains(ctx, "Extension: security") {
			t.Error("P1 context missing security extension")
		}
		if !contains(ctx, "Extension: resiliency") {
			t.Error("P1 context missing resiliency extension")
		}
	})

	t.Run("P2 loads security but not resiliency", func(t *testing.T) {
		ctx, err := rl.BuildContext("inception", "pm", 2)
		if err != nil {
			t.Fatalf("BuildContext() error: %v", err)
		}
		if !contains(ctx, "Extension: error-recovery") {
			t.Error("P2 context missing error-recovery extension")
		}
		if !contains(ctx, "Extension: overconfidence-prevention") {
			t.Error("P2 context missing overconfidence-prevention extension")
		}
		if !contains(ctx, "Extension: security") {
			t.Error("P2 context missing security extension")
		}
		if contains(ctx, "Extension: resiliency") {
			t.Error("P2 context should not include resiliency extension")
		}
	})

	t.Run("P3 loads only always-on extensions", func(t *testing.T) {
		ctx, err := rl.BuildContext("inception", "pm", 3)
		if err != nil {
			t.Fatalf("BuildContext() error: %v", err)
		}
		if !contains(ctx, "Extension: error-recovery") {
			t.Error("P3 context missing error-recovery extension")
		}
		if !contains(ctx, "Extension: overconfidence-prevention") {
			t.Error("P3 context missing overconfidence-prevention extension")
		}
		if contains(ctx, "Extension: security") {
			t.Error("P3 context should not include security extension")
		}
		if contains(ctx, "Extension: resiliency") {
			t.Error("P3 context should not include resiliency extension")
		}
	})
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func setupTestRules(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	dirs := []string{
		filepath.Join(tmpDir, "rules", "pipeline"),
		filepath.Join(tmpDir, "rules", "pipeline", "inception"),
		filepath.Join(tmpDir, "rules", "pipeline", "planning"),
		filepath.Join(tmpDir, "rules", "pipeline", "construction"),
		filepath.Join(tmpDir, "rules", "pipeline", "review"),
		filepath.Join(tmpDir, "rules", "pipeline", "testing"),
		filepath.Join(tmpDir, "rules", "pipeline", "delivery"),
		filepath.Join(tmpDir, "roles", "pm"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "rules", "pipeline", "core-workflow.md"), []byte("# Dev Team Pipeline Governance\n\nTest workflow."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "rules", "pipeline", "inception", "rules.md"), []byte("# Inception Phase Rules\n\nTest content."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "roles", "pm", "INSTRUCTIONS.md"), []byte("# PM Role\n\nYou are the Product Manager."), 0644); err != nil {
		t.Fatal(err)
	}

	return tmpDir
}