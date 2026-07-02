package role

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoleLoaderLoad(t *testing.T) {
	tmpDir := t.TempDir()
	roleDir := filepath.Join(tmpDir, "roles", "product")
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		t.Fatal(err)
	}
	instructions := `# Product Agent

Senior product manager and business analyst. Requirements, user stories, market research, scope.

## Core Responsibilities

1. Elicit requirements
2. Structure stories
3. Prioritize scope
`
	if err := os.WriteFile(filepath.Join(roleDir, "INSTRUCTIONS.md"), []byte(instructions), 0644); err != nil {
		t.Fatal(err)
	}

	rl := NewRoleLoader(tmpDir)
	rd, err := rl.Load("product")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if rd.Name != "product" {
		t.Errorf("Name = %s, want product", rd.Name)
	}
	if len(rd.Instructions) == 0 {
		t.Error("Instructions is empty")
	}
	if len(rd.Description) == 0 {
		t.Error("Description is empty")
	}
	if rd.ModelTier != "opus" {
		t.Errorf("ModelTier = %s, want opus", rd.ModelTier)
	}
	if rd.IsReviewer {
		t.Error("product should not be a reviewer")
	}
}

func TestRoleLoaderLoadReviewer(t *testing.T) {
	tmpDir := t.TempDir()
	roleDir := filepath.Join(tmpDir, "roles", "architecture-reviewer")
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "# Architecture Reviewer\n\nReviews technical design artifacts.\n"
	if err := os.WriteFile(filepath.Join(roleDir, "INSTRUCTIONS.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	rl := NewRoleLoader(tmpDir)
	rd, err := rl.Load("architecture-reviewer")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !rd.IsReviewer {
		t.Error("architecture-reviewer should be a reviewer")
	}
	if rd.ModelTier != "sonnet" {
		t.Errorf("ModelTier = %s, want sonnet", rd.ModelTier)
	}
}

func TestRoleLoaderValidate(t *testing.T) {
	tmpDir := t.TempDir()
	for _, roleName := range AllRoles() {
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

func TestAgentRosterComplete(t *testing.T) {
	agents := Agents()
	if len(agents) != 10 {
		t.Errorf("Agents() returned %d, want 10", len(agents))
	}
	reviewers := Reviewers()
	if len(reviewers) != 2 {
		t.Errorf("Reviewers() returned %d, want 2", len(reviewers))
	}
	all := AllRoles()
	if len(all) != 12 {
		t.Errorf("AllRoles() returned %d, want 12", len(all))
	}
}