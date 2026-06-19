package role

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoleLoaderLoad(t *testing.T) {
	tmpDir := t.TempDir()
	roleDir := filepath.Join(tmpDir, "roles", "pm")
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		t.Fatal(err)
	}
	instructions := `# Product Manager (PM)

## Identity

You are the Product Manager on the Dev Team. You own the **what** and the **why**.

## Core Responsibilities

1. Intake
2. Explore
3. Clarify
`
	if err := os.WriteFile(filepath.Join(roleDir, "INSTRUCTIONS.md"), []byte(instructions), 0644); err != nil {
		t.Fatal(err)
	}

	rl := NewRoleLoader(tmpDir)
	rd, err := rl.Load("pm")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if rd.Name != "pm" {
		t.Errorf("Name = %s, want pm", rd.Name)
	}
	if len(rd.Instructions) == 0 {
		t.Error("Instructions is empty")
	}
	if len(rd.Description) == 0 {
		t.Error("Description is empty")
	}
}

func TestRoleLoaderValidate(t *testing.T) {
	tmpDir := t.TempDir()
	for _, roleName := range []string{"pm", "architect", "developer", "reviewer", "tester", "ops"} {
		roleDir := filepath.Join(tmpDir, "roles", roleName)
		if err := os.MkdirAll(roleDir, 0755); err != nil {
			t.Fatal(err)
		}
		content := "# " + roleName + " Role\n\nYou are the " + roleName + ".\n"
		if err := os.WriteFile(filepath.Join(roleDir, "INSTRUCTIONS.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	rl := NewRoleLoader(tmpDir)
	if err := rl.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
}

func TestRoleLoaderValidateMissing(t *testing.T) {
	tmpDir := t.TempDir()
	rl := NewRoleLoader(tmpDir)
	if err := rl.Validate(); err == nil {
		t.Error("expected validation error for missing roles, got nil")
	}
}
