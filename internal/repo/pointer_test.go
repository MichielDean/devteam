package repo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadPointer(t *testing.T) {
	tmpDir := t.TempDir()

	pointer := &DevTeamPointer{
		SpecRepo:  "devteam",
		FeatureID: "001-test-feature",
		SpecPath:  "specs/001-test-feature",
		Branch:    "feature/001-test-feature",
	}

	if err := WritePointer(tmpDir, pointer); err != nil {
		t.Fatalf("WritePointer() error: %v", err)
	}

	pointerPath := filepath.Join(tmpDir, ".devteam", "pointer.yaml")
	if _, err := os.Stat(pointerPath); os.IsNotExist(err) {
		t.Fatal("expected .devteam/pointer.yaml to exist")
	}

	read, err := ReadPointer(tmpDir)
	if err != nil {
		t.Fatalf("ReadPointer() error: %v", err)
	}

	if read.SpecRepo != pointer.SpecRepo {
		t.Errorf("expected SpecRepo %s, got %s", pointer.SpecRepo, read.SpecRepo)
	}
	if read.FeatureID != pointer.FeatureID {
		t.Errorf("expected FeatureID %s, got %s", pointer.FeatureID, read.FeatureID)
	}
	if read.Branch != pointer.Branch {
		t.Errorf("expected Branch %s, got %s", pointer.Branch, read.Branch)
	}
}

func TestWritePointersForFeature(t *testing.T) {
	tmpDir := t.TempDir()

	repoDir := filepath.Join(tmpDir, "worktrees", "test-repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	m := NewManager(tmpDir)
	workDirs := []*RepoWorkDir{
		{Name: "test-repo", URL: "https://github.com/example/test.git", Dir: repoDir},
	}

	if err := m.WritePointersForFeature("001-test-feature", workDirs); err != nil {
		t.Fatalf("WritePointersForFeature() error: %v", err)
	}

	pointerPath := filepath.Join(repoDir, ".devteam", "pointer.yaml")
	if _, err := os.Stat(pointerPath); os.IsNotExist(err) {
		t.Fatal("expected .devteam/pointer.yaml to exist in repo dir")
	}

	read, err := ReadPointer(repoDir)
	if err != nil {
		t.Fatalf("ReadPointer() error: %v", err)
	}

	if read.FeatureID != "001-test-feature" {
		t.Errorf("expected FeatureID 001-test-feature, got %s", read.FeatureID)
	}
}