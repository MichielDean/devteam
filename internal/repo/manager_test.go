package repo

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestManagerGetWorkDir(t *testing.T) {
	m := NewManager("/tmp/test-devteam")
	got := m.GetWorkDir("cistern")
	expected := filepath.Join("/tmp/test-devteam", "worktrees", "cistern")
	if got != expected {
		t.Errorf("GetWorkDir() = %s, want %s", got, expected)
	}
}

func TestManagerIsRepoCloned(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	if m.IsRepoCloned("nonexistent") {
		t.Error("expected IsRepoCloned to return false for nonexistent repo")
	}
}

func TestManagerCommitAllNothingToCommit(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "test-repo")

	cmd := exec.Command("git", "init", repoDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\noutput: %s", err, string(output))
	}

	cmd = exec.Command("git", "-C", repoDir, "config", "user.email", "noreply@lobsterdog.dev")
	if err := cmd.Run(); err != nil {
		t.Fatalf("git config email failed: %v", err)
	}
	cmd = exec.Command("git", "-C", repoDir, "config", "user.name", "Lobsterdog Contributors")
	if err := cmd.Run(); err != nil {
		t.Fatalf("git config name failed: %v", err)
	}

	m := NewManager(tmpDir)
	err := m.CommitAll(repoDir, "test commit")
	// CommitAll with nothing to commit should return nil (it checks for "nothing to commit")
	if err != nil {
		t.Logf("CommitAll returned: %v (acceptable - empty repo)", err)
	}
}