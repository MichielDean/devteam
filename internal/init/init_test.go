package init

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit_CreatesDirectoryStructure(t *testing.T) {
	tmpDir := t.TempDir()
	initializer := NewInitializer(tmpDir)

	if err := initializer.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	expectedDirs := []string{
		"specs",
		"roles/pm",
		"roles/architect",
		"roles/developer",
		"roles/reviewer",
		"roles/tester",
		"roles/ops",
		"rules/aidlc",
		"rules/aidlc-rule-details/inception",
		"rules/aidlc-rule-details/construction",
		"rules/aidlc-rule-details/operations",
		"rules/aidlc-rule-details/extensions/security/baseline",
		"rules/aidlc-rule-details/extensions/resiliency/baseline",
		"rules/aidlc-rule-details/extensions/testing/property-based",
		"constitution",
	}

	for _, dir := range expectedDirs {
		path := filepath.Join(tmpDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist", dir)
		}
	}
}

func TestInit_CreatesConfigFiles(t *testing.T) {
	tmpDir := t.TempDir()
	initializer := NewInitializer(tmpDir)

	if err := initializer.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	configFiles := []string{
		"devteam.yaml",
		"repos.yaml",
	}

	for _, file := range configFiles {
		path := filepath.Join(tmpDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", file)
		}
	}
}

func TestInit_CreatesRoleInstructions(t *testing.T) {
	tmpDir := t.TempDir()
	initializer := NewInitializer(tmpDir)

	if err := initializer.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	roles := []string{"pm", "architect", "developer", "reviewer", "tester", "ops"}
	for _, role := range roles {
		path := filepath.Join(tmpDir, "roles", role, "INSTRUCTIONS.md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected role instructions for %s to exist", role)
		}
	}
}

func TestInit_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	initializer := NewInitializer(tmpDir)

	if err := initializer.Init(); err != nil {
		t.Fatalf("First Init() error: %v", err)
	}

	if err := initializer.Init(); err != nil {
		t.Fatalf("Second Init() error: %v", err)
	}
}

func TestInit_DoesNotOverwriteExisting(t *testing.T) {
	tmpDir := t.TempDir()
	initializer := NewInitializer(tmpDir)

	configPath := filepath.Join(tmpDir, "devteam.yaml")
	existingContent := "custom: config\n"
	if err := os.WriteFile(configPath, []byte(existingContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := initializer.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != existingContent {
		t.Error("Init() should not overwrite existing devteam.yaml")
	}
}