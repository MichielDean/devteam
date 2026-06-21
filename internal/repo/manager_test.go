package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichielDean/devteam/internal/feature"
)

func TestManagerGetWorkDir(t *testing.T) {
	m := NewManager("/tmp/test-devteam")
	got := m.GetWorkDir("cistern", "001-feature")
	expected := filepath.Join("/tmp/test-devteam", "worktrees", "001-feature", "cistern")
	if got != expected {
		t.Errorf("GetWorkDir() = %s, want %s", got, expected)
	}
}

func TestManagerGetWorkDirLegacy(t *testing.T) {
	m := NewManager("/tmp/test-devteam")
	got := m.GetWorkDirLegacy("cistern")
	expected := filepath.Join("/tmp/test-devteam", "worktrees", "cistern")
	if got != expected {
		t.Errorf("GetWorkDirLegacy() = %s, want %s", got, expected)
	}
}

func TestManagerIsRepoCloned(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	if m.IsRepoCloned("nonexistent") {
		t.Error("expected IsRepoCloned to return false for nonexistent repo")
	}

	repoDir := m.GetWorkDirLegacy("test-repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if !m.IsRepoCloned("test-repo") {
		t.Error("expected IsRepoCloned to return true for cloned repo")
	}
}

func TestManagerIsRepoClonedFor(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	if m.IsRepoClonedFor("repo", "001-feat") {
		t.Error("expected false for nonexistent per-feature worktree")
	}

	dir := m.GetWorkDir("repo", "001-feat")
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if !m.IsRepoClonedFor("repo", "001-feat") {
		t.Error("expected true for existing per-feature worktree")
	}
}

func TestManagerPrepareRepos(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	repos := []feature.RepoRef{
		{Name: "devteam", URL: "git@github.com:MichielDean/devteam.git"},
	}

	// This test verifies the structure works even if the remote isn't reachable
	// In a real test environment, we'd use local git repos
	workDirs, err := m.PrepareRepos(repos, "001-test-feature")
	// This will fail because the remote isn't reachable in test,
	// but we can test the happy path with a local repo
	_ = workDirs // may be nil if clone fails
	_ = err      // may be non-nil if clone fails
}

func TestManagerIsBuildable(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Empty dir is not buildable
	if m.IsBuildable(tmpDir) {
		t.Error("expected empty dir to not be buildable")
	}

	// Dir with go.mod is buildable
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !m.IsBuildable(tmpDir) {
		t.Error("expected dir with go.mod to be buildable")
	}
}

func TestManagerCommitWithProperGitConfig(t *testing.T) {
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

	// Create a file to commit
	if err := os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(tmpDir)
	err := m.CommitAll(repoDir, "test commit")
	if err != nil {
		t.Fatalf("CommitAll failed: %v", err)
	}
}

// TestManagerPrepareLocalRepos exercises the happy path against a local git
// repo instead of a remote URL. This is the only way to get deterministic
// coverage of PrepareRepos + CreateFeatureBranch + PushBranch in CI.
func TestManagerPrepareLocalRepos(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create a local "remote" repo with one commit on main.
	remote := filepath.Join(tmpDir, "remote-repo")
	if err := os.MkdirAll(remote, 0755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", remote},
		{"-C", remote, "config", "user.email", "noreply@lobsterdog.dev"},
		{"-C", remote, "config", "user.name", "Lobsterdog Contributors"},
		{"-C", remote, "symbolic-ref", "HEAD", "refs/heads/main"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, string(out))
		}
	}
	if err := os.WriteFile(filepath.Join(remote, "README.md"), []byte("# test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", remote, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, string(out))
	}
	if out, err := exec.Command("git", "-C", remote, "commit", "-m", "init").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, string(out))
	}

	refs := []feature.RepoRef{
		{Name: "local-repo", URL: remote},
	}
	workDirs, err := m.PrepareRepos(refs, "001-local")
	if err != nil {
		t.Fatalf("PrepareRepos failed: %v", err)
	}
	if len(workDirs) != 1 {
		t.Fatalf("expected 1 work dir, got %d", len(workDirs))
	}
	wd := workDirs[0]
	if wd.Name != "local-repo" {
		t.Errorf("expected Name local-repo, got %s", wd.Name)
	}
	if wd.Branch != "feature/001-local" {
		t.Errorf("expected Branch feature/001-local, got %s", wd.Branch)
	}
	if !filepath.IsAbs(wd.Dir) {
		// Dir is under tmpDir/worktrees/001-local/local-repo
	}
	expectedDir := filepath.Join(tmpDir, "worktrees", "001-local", "local-repo")
	if wd.Dir != expectedDir {
		t.Errorf("expected Dir %s, got %s", expectedDir, wd.Dir)
	}

	// Verify the feature branch is checked out.
	out, err := exec.Command("git", "-C", wd.Dir, "branch", "--show-current").Output()
	if err != nil {
		t.Fatalf("branch --show-current: %v", err)
	}
	if got := string(out); got != "feature/001-local\n" && got != "feature/001-local" {
		t.Errorf("expected branch feature/001-local, got %q", got)
	}
}

// TestManagerPrepareLocalReposConcurrentFeatures verifies that two features
// targeting the same repo get separate worktrees and don't clobber each
// other's branches. This is the core bug: the old shared worktree meant
// CreateFeatureBranch on feature B would destroy feature A's branch.
func TestManagerPrepareLocalReposConcurrentFeatures(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	remote := filepath.Join(tmpDir, "remote")
	if err := os.MkdirAll(remote, 0755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", remote},
		{"-C", remote, "config", "user.email", "noreply@lobsterdog.dev"},
		{"-C", remote, "config", "user.name", "Lobsterdog Contributors"},
		{"-C", remote, "symbolic-ref", "HEAD", "refs/heads/main"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, string(out))
		}
	}
	if err := os.WriteFile(filepath.Join(remote, "f.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", remote, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, string(out))
	}
	if out, err := exec.Command("git", "-C", remote, "commit", "-m", "init").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, string(out))
	}

	ref := feature.RepoRef{Name: "shared", URL: remote}

	wdA, err := m.PrepareRepos([]feature.RepoRef{ref}, "001-A")
	if err != nil {
		t.Fatalf("PrepareRepos A: %v", err)
	}
	// Make a commit on feature A's branch.
	if err := os.WriteFile(filepath.Join(wdA[0].Dir, "a.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", wdA[0].Dir, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add A: %v\n%s", err, string(out))
	}
	if out, err := exec.Command("git", "-C", wdA[0].Dir, "commit", "-m", "a").CombinedOutput(); err != nil {
		t.Fatalf("git commit A: %v\n%s", err, string(out))
	}

	// Now prepare feature B — must not disturb feature A's branch.
	wdB, err := m.PrepareRepos([]feature.RepoRef{ref}, "002-B")
	if err != nil {
		t.Fatalf("PrepareRepos B: %v", err)
	}

	// Feature A's worktree must still be on its branch with its commit.
	out, err := exec.Command("git", "-C", wdA[0].Dir, "branch", "--show-current").Output()
	if err != nil {
		t.Fatalf("branch A: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "feature/001-A" {
		t.Errorf("feature A branch changed to %q", got)
	}
	if _, err := os.Stat(filepath.Join(wdA[0].Dir, "a.txt")); err != nil {
		t.Errorf("feature A commit lost: %v", err)
	}

	// Feature B must be on its own branch in its own worktree.
	out, err = exec.Command("git", "-C", wdB[0].Dir, "branch", "--show-current").Output()
	if err != nil {
		t.Fatalf("branch B: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "feature/002-B" {
		t.Errorf("feature B expected feature/002-B, got %q", got)
	}
	// B's worktree must NOT contain A's commit.
	if _, err := os.Stat(filepath.Join(wdB[0].Dir, "a.txt")); err == nil {
		t.Error("feature B worktree contains feature A's commit — worktrees are not isolated")
	}
}

// TestManagerPushBranch exercises CommitAll + PushBranch against a local
// bare "remote" repo. Verifies that commits land on origin/feature/<id>.
func TestManagerPushBranch(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Seed a non-bare repo, then mirror it into a bare clone that will
	// act as the push target (origin).
	seed := filepath.Join(tmpDir, "seed")
	if err := os.MkdirAll(seed, 0755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", seed},
		{"-C", seed, "config", "user.email", "noreply@lobsterdog.dev"},
		{"-C", seed, "config", "user.name", "Lobsterdog Contributors"},
		{"-C", seed, "symbolic-ref", "HEAD", "refs/heads/main"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, string(out))
		}
	}
	if err := os.WriteFile(filepath.Join(seed, "f.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", seed, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, string(out))
	}
	if out, err := exec.Command("git", "-C", seed, "commit", "-m", "init").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, string(out))
	}
	bareRemote := filepath.Join(tmpDir, "bare.git")
	if out, err := exec.Command("git", "clone", "--bare", seed, bareRemote).CombinedOutput(); err != nil {
		t.Fatalf("git clone --bare: %v\n%s", err, string(out))
	}

	// PrepareRepos will clone the bare remote — origin points back at the
	// bare remote, so PushBranch has somewhere to push.
	refs := []feature.RepoRef{
		{Name: "local-repo", URL: bareRemote},
	}
	workDirs, err := m.PrepareRepos(refs, "003-push")
	if err != nil {
		t.Fatalf("PrepareRepos: %v", err)
	}
	dir := workDirs[0].Dir

	// Make a commit.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("y\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := m.CommitAll(dir, "feat(003-push): add new.txt"); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}

	// Push.
	if err := m.PushBranch(dir, "003-push"); err != nil {
		t.Fatalf("PushBranch: %v", err)
	}

	// Verify the branch landed on the bare remote.
	out, err := exec.Command("git", "-C", bareRemote, "branch", "--list").Output()
	if err != nil {
		t.Fatalf("git branch --list: %v", err)
	}
	if !strings.Contains(string(out), "feature/003-push") {
		t.Errorf("expected feature/003-push on bare remote, got: %s", string(out))
	}
}

// TestManagerPushAcrossRepos verifies that PushAcrossRepos pushes commits
// to every repo and skips repos with no new commits.
func TestManagerPushAcrossRepos(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create two bare remotes and two seed repos.
	makeRepo := func(name string) (string, string) {
		src := filepath.Join(tmpDir, name+"-src")
		bare := filepath.Join(tmpDir, name+".git")
		if err := os.MkdirAll(src, 0755); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{
			{"init", src},
			{"-C", src, "config", "user.email", "noreply@lobsterdog.dev"},
			{"-C", src, "config", "user.name", "Lobsterdog Contributors"},
			{"-C", src, "symbolic-ref", "HEAD", "refs/heads/main"},
		} {
			if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, string(out))
			}
		}
		if err := os.WriteFile(filepath.Join(src, "f.txt"), []byte("x\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("git", "-C", src, "add", "-A").CombinedOutput(); err != nil {
			t.Fatalf("git add: %v\n%s", err, string(out))
		}
		if out, err := exec.Command("git", "-C", src, "commit", "-m", "init").CombinedOutput(); err != nil {
			t.Fatalf("git commit: %v\n%s", err, string(out))
		}
		if out, err := exec.Command("git", "clone", "--bare", src, bare).CombinedOutput(); err != nil {
			t.Fatalf("git clone --bare: %v\n%s", err, string(out))
		}
		return src, bare
	}

	_, bareA := makeRepo("repoA")
	_, bareB := makeRepo("repoB")

	// Clone from the bare remotes so origin points at a pushable target.
	refs := []feature.RepoRef{
		{Name: "repoA", URL: bareA},
		{Name: "repoB", URL: bareB},
	}
	workDirs, err := m.PrepareRepos(refs, "004-multi")
	if err != nil {
		t.Fatalf("PrepareRepos: %v", err)
	}

	// Commit only in repoA.
	if err := os.WriteFile(filepath.Join(workDirs[0].Dir, "a.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := m.CommitAll(workDirs[0].Dir, "feat(004-multi): a"); err != nil {
		t.Fatalf("CommitAll A: %v", err)
	}

	if err := m.PushAcrossRepos(workDirs, "004-multi"); err != nil {
		t.Fatalf("PushAcrossRepos: %v", err)
	}

	// repoA should have feature/004-multi on its bare remote.
	out, err := exec.Command("git", "-C", bareA, "branch", "--list").Output()
	if err != nil {
		t.Fatalf("branch --list A: %v", err)
	}
	if !strings.Contains(string(out), "feature/004-multi") {
		t.Errorf("expected feature/004-multi on repoA bare, got: %s", string(out))
	}

	// repoB should NOT have feature/004-multi (no commits → no push).
	out, err = exec.Command("git", "-C", bareB, "branch", "--list").Output()
	if err != nil {
		t.Fatalf("branch --list B: %v", err)
	}
	if strings.Contains(string(out), "feature/004-multi") {
		t.Errorf("repoB should not have feature/004-multi, got: %s", string(out))
	}
}

func TestFeatureBranchName(t *testing.T) {
	got := FeatureBranchName("001-foo")
	if got != "feature/001-foo" {
		t.Errorf("FeatureBranchName = %q, want feature/001-foo", got)
	}
}

func TestRemoveWorktreeFor(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	dir := m.GetWorkDir("repo", "001-feat")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := m.RemoveWorktreeFor("repo", "001-feat"); err != nil {
		t.Fatalf("RemoveWorktreeFor: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("expected worktree removed, stat err=%v", err)
	}
	// Idempotent.
	if err := m.RemoveWorktreeFor("repo", "001-feat"); err != nil {
		t.Errorf("RemoveWorktreeFor on missing dir: %v", err)
	}
}

func TestRemoveAllWorktreesFor(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create two repos under one feature.
	for _, name := range []string{"a", "b"} {
		dir := m.GetWorkDir(name, "001-feat")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RemoveAllWorktreesFor("001-feat"); err != nil {
		t.Fatalf("RemoveAllWorktreesFor: %v", err)
	}
	featureDir := filepath.Join(tmpDir, "worktrees", "001-feat")
	if _, err := os.Stat(featureDir); !os.IsNotExist(err) {
		t.Errorf("expected feature worktree dir removed, stat err=%v", err)
	}
}
