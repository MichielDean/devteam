package pipeline

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/repo"
)

// minimalTestConfig returns a Config with all 6 phases and 6 roles required
// by validateConfig. Tests that need a Pipeline must pass a valid config.
func minimalTestConfig() *config.Config {
	phases := []config.PhaseConfig{
		{Name: "inception", Roles: []string{"pm"}, Gate: "spec_approved", Artifacts: []string{"spec.md", "acceptance.md", "repos.yaml"}},
		{Name: "planning", Roles: []string{"architect"}, Gate: "plan_approved", Artifacts: []string{"plan.md", "tasks.md"}},
		{Name: "construction", Roles: []string{"developer"}, Gate: "tasks_complete"},
		{Name: "review", Roles: []string{"reviewer"}, Gate: "criteria_met", Artifacts: []string{"review_report"}},
		{Name: "testing", Roles: []string{"tester"}, Gate: "tests_pass", Artifacts: []string{"test_report"}},
		{Name: "delivery", Roles: []string{"ops"}, Gate: "docs_match_spec", Artifacts: []string{"docs"}},
	}
	roles := map[string]config.RoleConfig{
		"pm":        {Name: "Product Manager"},
		"architect": {Name: "Architect"},
		"developer": {Name: "Developer"},
		"reviewer":  {Name: "Reviewer"},
		"tester":    {Name: "Tester"},
		"ops":       {Name: "Ops"},
	}
	return &config.Config{
		Version:  "1.0",
		Pipeline: config.PipelineConfig{Phases: phases},
		Roles:    roles,
	}
}

// makeBareRemote creates a bare git repo at tmpDir/<name>.git with one
// commit on main. Returns the path to the bare repo (suitable as a clone
// URL — origin will point back at it so pushes land there).
func makeBareRemote(t *testing.T, tmpDir, name string) string {
	t.Helper()
	seed := filepath.Join(tmpDir, name+"-seed")
	bare := filepath.Join(tmpDir, name+".git")
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
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("# "+name+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", seed, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, string(out))
	}
	if out, err := exec.Command("git", "-C", seed, "commit", "-m", "init").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, string(out))
	}
	if out, err := exec.Command("git", "clone", "--bare", seed, bare).CombinedOutput(); err != nil {
		t.Fatalf("git clone --bare: %v\n%s", err, string(out))
	}
	return bare
}

// TestPrepareImplRepos_NoReposYaml verifies that a feature with no
// repos.yaml is treated as spec-only: PreparedRepos stays nil, no error.
func TestPrepareImplRepos_NoReposYaml(t *testing.T) {
	tmpDir := t.TempDir()
	provider, _ := newTestProvider(t, tmpDir)
	writer := newTestWriter(provider)

	f := feature.NewFeature("001-spec-only", "Spec Only", 2, feature.IntakeLooseIdea)
	if err := writer.CreateFeatureDir(f.ID); err != nil {
		t.Fatal(err)
	}
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}

	p := NewPipeline(minimalTestConfig(), provider)
	if err := p.PrepareImplRepos(f); err != nil {
		t.Fatalf("PrepareImplRepos on spec-only feature: %v", err)
	}
	if len(f.PreparedRepos) != 0 {
		t.Errorf("expected no prepared repos for spec-only feature, got %d", len(f.PreparedRepos))
	}
}

// TestPrepareImplRepos_WithReposYaml verifies that PrepareImplRepos clones
// every repo in repos.yaml, persists PreparedRepos on the feature state,
// and that a second call is a no-op reusing the existing worktrees.
func TestPrepareImplRepos_WithReposYaml(t *testing.T) {
	tmpDir := t.TempDir()
	bareA := makeBareRemote(t, tmpDir, "repoA")
	bareB := makeBareRemote(t, tmpDir, "repoB")

	provider, _ := newTestProvider(t, tmpDir)
	writer := newTestWriter(provider)

	f := feature.NewFeature("001-multi", "Multi Repo", 2, feature.IntakeLooseIdea)
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}
	// Write repos.yaml declaring two impl repos.
	reposYAML := "feature: 001-multi\nrepos:\n  - name: repoA\n    url: " + bareA + "\n    branch: feature/001-multi\n  - name: repoB\n    url: " + bareB + "\n    branch: feature/001-multi\n"
	if err := writer.WriteArtifact(f.ID, feature.ArtifactReposYAML, []byte(reposYAML)); err != nil {
		t.Fatal(err)
	}

	p := NewPipeline(minimalTestConfig(), provider)
	if err := p.PrepareImplRepos(f); err != nil {
		t.Fatalf("PrepareImplRepos: %v", err)
	}

	if len(f.PreparedRepos) != 2 {
		t.Fatalf("expected 2 prepared repos, got %d", len(f.PreparedRepos))
	}

	// Verify each worktree exists and is on the feature branch.
	for _, pr := range f.PreparedRepos {
		if _, err := os.Stat(filepath.Join(pr.Dir, ".git")); err != nil {
			t.Errorf("prepared repo %s missing .git at %s: %v", pr.Name, pr.Dir, err)
			continue
		}
		if pr.Branch != "feature/001-multi" {
			t.Errorf("expected branch feature/001-multi, got %s", pr.Branch)
		}
		out, err := exec.Command("git", "-C", pr.Dir, "branch", "--show-current").Output()
		if err != nil {
			t.Errorf("branch --show-current for %s: %v", pr.Name, err)
			continue
		}
		if got := strings.TrimSpace(string(out)); got != "feature/001-multi" {
			t.Errorf("expected checked-out branch feature/001-multi for %s, got %q", pr.Name, got)
		}
	}

	// Verify PreparedRepos was persisted to feature state on disk.
	loaded, err := provider.LoadFeatureState(f.ID)
	if err != nil {
		t.Fatalf("LoadFeatureState: %v", err)
	}
	if len(loaded.PreparedRepos) != 2 {
		t.Errorf("expected 2 persisted prepared repos, got %d", len(loaded.PreparedRepos))
	}

	// Second call is a no-op: same worktrees, no re-clone.
	firstDir := f.PreparedRepos[0].Dir
	if err := p.PrepareImplRepos(f); err != nil {
		t.Fatalf("second PrepareImplRepos: %v", err)
	}
	if f.PreparedRepos[0].Dir != firstDir {
		t.Errorf("second PrepareImplRepos changed worktree dir: was %s, now %s", firstDir, f.PreparedRepos[0].Dir)
	}
}

// TestDispatchWorkingDirForPhase verifies that spec-only phases use the
// spec repo as CWD and impl phases use the first prepared repo.
func TestDispatchWorkingDirForPhase(t *testing.T) {
	tmpDir := t.TempDir()
	provider, _ := newTestProvider(t, tmpDir)
	p := NewPipeline(minimalTestConfig(), provider)

	f := feature.NewFeature("001-dir", "Dir Test", 2, feature.IntakeLooseIdea)

	// No prepared repos: all phases fall back to spec repo base dir.
	for _, phase := range feature.AllPhases() {
		got := p.dispatchWorkingDirForPhase(f, phase)
		if got != tmpDir {
			t.Errorf("phase %s with no prepared repos: expected %s, got %s", phase, tmpDir, got)
		}
	}

	// With prepared repos: impl phases use first prepared repo dir.
	f.PreparedRepos = []feature.PreparedRepo{
		{Name: "primary", Dir: filepath.Join(tmpDir, "wt-primary")},
	}
	specPhases := []feature.Phase{feature.PhaseInception, feature.PhasePlanning}
	implPhases := []feature.Phase{feature.PhaseConstruction, feature.PhaseReview, feature.PhaseTesting, feature.PhaseDelivery}

	for _, phase := range specPhases {
		got := p.dispatchWorkingDirForPhase(f, phase)
		if got != tmpDir {
			t.Errorf("spec phase %s: expected spec repo %s, got %s", phase, tmpDir, got)
		}
	}
	for _, phase := range implPhases {
		got := p.dispatchWorkingDirForPhase(f, phase)
		if got != f.PreparedRepos[0].Dir {
			t.Errorf("impl phase %s: expected %s, got %s", phase, f.PreparedRepos[0].Dir, got)
		}
	}
}

// TestImplRepoContext verifies that the CONTEXT.md fragment for impl repos
// is empty for spec-only phases and contains worktree paths for impl phases.
func TestImplRepoContext(t *testing.T) {
	tmpDir := t.TempDir()
	provider, _ := newTestProvider(t, tmpDir)
	p := NewPipeline(minimalTestConfig(), provider)

	f := feature.NewFeature("001-ctx", "Ctx Test", 2, feature.IntakeLooseIdea)

	// No prepared repos: empty for all phases.
	for _, phase := range feature.AllPhases() {
		if got := p.implRepoContext(f, phase); got != "" {
			t.Errorf("phase %s with no prepared repos: expected empty context, got %q", phase, got)
		}
	}

	f.PreparedRepos = []feature.PreparedRepo{
		{Name: "repoA", Dir: "/tmp/wt-a", Branch: "feature/001-ctx", URL: "git@github.com:foo/a.git"},
		{Name: "repoB", Dir: "/tmp/wt-b", Branch: "feature/001-ctx", URL: "git@github.com:foo/b.git"},
	}

	// Spec phases: still empty (no impl repo context for inception/planning).
	for _, phase := range []feature.Phase{feature.PhaseInception, feature.PhasePlanning} {
		if got := p.implRepoContext(f, phase); got != "" {
			t.Errorf("spec phase %s should have empty impl repo context, got %q", phase, got)
		}
	}

	// Impl phases: contains both repo paths and the feature branch.
	for _, phase := range []feature.Phase{feature.PhaseConstruction, feature.PhaseReview, feature.PhaseTesting, feature.PhaseDelivery} {
		got := p.implRepoContext(f, phase)
		if !strings.Contains(got, "/tmp/wt-a") {
			t.Errorf("phase %s: context missing repoA path", phase)
		}
		if !strings.Contains(got, "/tmp/wt-b") {
			t.Errorf("phase %s: context missing repoB path", phase)
		}
		if !strings.Contains(got, "feature/001-ctx") {
			t.Errorf("phase %s: context missing feature branch name", phase)
		}
		if !strings.Contains(got, "PRIMARY") {
			t.Errorf("phase %s: context should mark primary repo for impl phases", phase)
		}
	}
}

// TestPushPhaseChanges_PerRepo verifies that PushPhaseChanges commits and
// pushes each prepared repo's feature branch to its origin, and that
// repos with no new commits are skipped.
func TestPushPhaseChanges_PerRepo(t *testing.T) {
	tmpDir := t.TempDir()
	bareA := makeBareRemote(t, tmpDir, "repoA")
	bareB := makeBareRemote(t, tmpDir, "repoB")

	provider, _ := newTestProvider(t, tmpDir)
	writer := newTestWriter(provider)

	f := feature.NewFeature("001-push", "Push Test", 2, feature.IntakeLooseIdea)
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}
	reposYAML := "feature: 001-push\nrepos:\n  - name: repoA\n    url: " + bareA + "\n    branch: feature/001-push\n  - name: repoB\n    url: " + bareB + "\n    branch: feature/001-push\n"
	if err := writer.WriteArtifact(f.ID, feature.ArtifactReposYAML, []byte(reposYAML)); err != nil {
		t.Fatal(err)
	}

	p := NewPipeline(minimalTestConfig(), provider)
	if err := p.PrepareImplRepos(f); err != nil {
		t.Fatalf("PrepareImplRepos: %v", err)
	}

	// Commit only in repoA.
	dirA := f.PreparedRepos[0].Dir
	if err := os.WriteFile(filepath.Join(dirA, "new.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := p.repoManager.CommitAll(dirA, "feat(001-push): add new.go"); err != nil {
		t.Fatalf("CommitAll A: %v", err)
	}

	// PushPhaseChanges should push repoA and skip repoB (no new commits).
	// Note: PushPhaseChanges also attempts to commit/push the spec repo
	// (the devteam repo itself) via gitClient. In this test the spec repo
	// is tmpDir which is not a git repo, so that step will fail and log a
	// warning. The impl repo push must still succeed. We ignore the
	// spec-repo warning by checking the bare remotes directly.
	_ = p.PushPhaseChanges(f, feature.PhaseConstruction)

	// repoA bare should have feature/001-push.
	out, err := exec.Command("git", "-C", bareA, "branch", "--list").Output()
	if err != nil {
		t.Fatalf("branch --list A: %v", err)
	}
	if !strings.Contains(string(out), "feature/001-push") {
		t.Errorf("expected feature/001-push on repoA bare, got: %s", string(out))
	}
	// repoB bare should NOT have feature/001-push (no commits).
	out, err = exec.Command("git", "-C", bareB, "branch", "--list").Output()
	if err != nil {
		t.Fatalf("branch --list B: %v", err)
	}
	if strings.Contains(string(out), "feature/001-push") {
		t.Errorf("repoB should not have feature/001-push, got: %s", string(out))
	}
}

// TestPushPhaseChanges_SpecOnly verifies that features without prepared
// repos fall back to the legacy gitClient path (commits/pushes spec repo
// on feat/<id> branch). We don't have a real origin here so we just verify
// the path doesn't panic and returns an error we can ignore.
func TestPushPhaseChanges_SpecOnly(t *testing.T) {
	tmpDir := t.TempDir()
	provider, _ := newTestProvider(t, tmpDir)

	f := feature.NewFeature("001-spec", "Spec Only Push", 2, feature.IntakeLooseIdea)

	p := NewPipeline(minimalTestConfig(), provider)
	// No prepared repos → legacy path. tmpDir is not a git repo, so this
	// will fail. We just verify it doesn't panic and the code path is
	// reachable.
	_ = p.PushPhaseChanges(f, feature.PhaseInception)
}

// TestCleanupImplRepos verifies that cleanup removes worktree directories
// and clears PreparedRepos on the feature.
func TestCleanupImplRepos(t *testing.T) {
	tmpDir := t.TempDir()
	bare := makeBareRemote(t, tmpDir, "repo")

	provider, _ := newTestProvider(t, tmpDir)
	writer := newTestWriter(provider)

	f := feature.NewFeature("001-clean", "Cleanup Test", 2, feature.IntakeLooseIdea)
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}
	reposYAML := "feature: 001-clean\nrepos:\n  - name: repo\n    url: " + bare + "\n    branch: feature/001-clean\n"
	if err := writer.WriteArtifact(f.ID, feature.ArtifactReposYAML, []byte(reposYAML)); err != nil {
		t.Fatal(err)
	}

	p := NewPipeline(minimalTestConfig(), provider)
	if err := p.PrepareImplRepos(f); err != nil {
		t.Fatalf("PrepareImplRepos: %v", err)
	}
	if len(f.PreparedRepos) != 1 {
		t.Fatalf("expected 1 prepared repo, got %d", len(f.PreparedRepos))
	}
	dir := f.PreparedRepos[0].Dir
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("worktree should exist before cleanup: %v", err)
	}

	p.CleanupImplRepos(f)

	if len(f.PreparedRepos) != 0 {
		t.Errorf("expected PreparedRepos cleared, got %d", len(f.PreparedRepos))
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("expected worktree dir removed, stat err=%v", err)
	}
	// Feature worktrees dir should be gone too.
	featureWorktreesDir := filepath.Join(tmpDir, "worktrees", "001-clean")
	if _, err := os.Stat(featureWorktreesDir); !os.IsNotExist(err) {
		t.Errorf("expected feature worktrees dir removed, stat err=%v", err)
	}
}

// TestSpecProviderLoadFeatureRepos verifies the repos.yaml parser returns
// the declared RepoRefs.
func TestSpecProviderLoadFeatureRepos(t *testing.T) {
	tmpDir := t.TempDir()
	provider, _ := newTestProvider(t, tmpDir)
	writer := newTestWriter(provider)

	f := feature.NewFeature("001-repos", "Repos Test", 2, feature.IntakeLooseIdea)
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}
	reposYAML := "feature: 001-repos\nrepos:\n  - name: alpha\n    url: git@github.com:foo/alpha.git\n    branch: feature/001-repos\n  - name: beta\n    url: git@github.com:foo/beta.git\n    branch: feature/001-repos\n"
	if err := writer.WriteArtifact(f.ID, feature.ArtifactReposYAML, []byte(reposYAML)); err != nil {
		t.Fatal(err)
	}

	refs, err := provider.LoadFeatureRepos(f.ID)
	if err != nil {
		t.Fatalf("LoadFeatureRepos: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].Name != "alpha" || refs[1].Name != "beta" {
		t.Errorf("expected alpha,beta got %s,%s", refs[0].Name, refs[1].Name)
	}
}

// TestSpecProviderLoadFeatureRepos_Missing verifies that a missing
// repos.yaml returns an empty slice, not an error.
func TestSpecProviderLoadFeatureRepos_Missing(t *testing.T) {
	tmpDir := t.TempDir()
	provider, _ := newTestProvider(t, tmpDir)
	writer := newTestWriter(provider)

	f := feature.NewFeature("001-none", "No Repos", 2, feature.IntakeLooseIdea)
	if err := writer.CreateFeatureDir(f.ID); err != nil {
		t.Fatal(err)
	}

	refs, err := provider.LoadFeatureRepos(f.ID)
	if err != nil {
		t.Fatalf("LoadFeatureRepos on missing repos.yaml should not error: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs, got %d", len(refs))
	}
}

// TestFeatureBranchNamePipeline verifies the pipeline uses the canonical
// feature/<id> branch name (not feat/<id>) for impl repos. The spec repo
// still uses feat/<id> for legacy compatibility.
func TestFeatureBranchNamePipeline(t *testing.T) {
	if got := repo.FeatureBranchName("001-foo"); got != "feature/001-foo" {
		t.Errorf("FeatureBranchName = %q, want feature/001-foo", got)
	}
}
