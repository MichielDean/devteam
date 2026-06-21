package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/feature"
)

type Manager struct {
	baseDir      string
	worktreesDir string
}

func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir:      baseDir,
		worktreesDir: filepath.Join(baseDir, "worktrees"),
	}
}

// CloneRepo clones url into destDir. Idempotent: no-op if destDir already
// has a .git directory. Caller must ensure destDir is exclusive to one
// feature — use GetWorkDir(repoName, featureID) to compute it.
func (m *Manager) CloneRepo(url, branch, destDir string) error {
	if _, err := os.Stat(filepath.Join(destDir, ".git")); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(destDir), 0755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, url, destDir)
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

// CloneForFeature clones (or reuses) a working copy of repo for the given
// feature. The clone lives at worktrees/<featureID>/<repoName> so concurrent
// features on the same repo do not clobber each other's branches.
//
// On reuse: origin is fetched and the local branch matching featureID is
// deleted so CreateFeatureBranch always branches from latest origin/main.
// This is safe because the feature branch is only ever advanced by the
// pipeline that owns this worktree.
//
// Local git identity (user.name/user.email) is pinned on the clone so that
// pre-commit hooks requiring the Lobsterdog identity pass for any agent
// committing inside this worktree.
func (m *Manager) CloneForFeature(repo *config.RepoEntry, featureID string) (string, error) {
	workDir := m.GetWorkDir(repo.Name, featureID)
	if _, err := os.Stat(filepath.Join(workDir, ".git")); err == nil {
		if err := m.FetchOrigin(workDir); err != nil {
			return "", fmt.Errorf("fetching origin for %s: %w", repo.Name, err)
		}
		return workDir, nil
	}
	branch := "main"
	if err := m.CloneRepo(repo.URL, branch, workDir); err != nil {
		return "", fmt.Errorf("cloning %s: %w", repo.Name, err)
	}
	if err := m.pinLocalIdentity(workDir); err != nil {
		return "", fmt.Errorf("pinning identity for %s: %w", repo.Name, err)
	}
	return workDir, nil
}

// pinLocalIdentity sets local user.name/user.email on repoDir so commits
// made inside this worktree satisfy global pre-commit hooks regardless of
// the cloning user's global config. Local config takes precedence over
// global, so this overrides inherited identities.
func (m *Manager) pinLocalIdentity(repoDir string) error {
	for _, kv := range [][2]string{
		{"user.name", "Lobsterdog Contributors"},
		{"user.email", "noreply@lobsterdog.dev"},
	} {
		cmd := exec.Command("git", "-C", repoDir, "config", kv[0], kv[1])
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git config %s: %w\n%s", kv[0], err, string(out))
		}
	}
	return nil
}

// CreateFeatureBranch creates feature/<featureID> in repoDir, branching
// from origin/main (or main if no remote). If the branch already exists it
// is checked out. The caller MUST hold exclusive access to repoDir for the
// feature — see CloneForFeature for the per-feature worktree convention.
func (m *Manager) CreateFeatureBranch(repoDir, featureID string) error {
	branchName := FeatureBranchName(featureID)

	// Ensure we're on a clean starting point (origin/main preferred).
	if err := m.ensureOnMain(repoDir); err != nil {
		// Non-fatal: fall through and try the branch op anyway.
	}

	cmd := exec.Command("git", "checkout", "-b", branchName)
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "already exists") {
			cmd := exec.Command("git", "checkout", branchName)
			cmd.Dir = repoDir
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("git checkout existing branch failed: %w\noutput: %s", err, string(output))
			}
			return nil
		}
		return fmt.Errorf("git checkout -b failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

// ensureOnMain checks out origin/main (or main) without disturbing local
// commits on feature branches. It is best-effort: a dirty tree or missing
// remote is ignored so the caller can still attempt CreateFeatureBranch.
func (m *Manager) ensureOnMain(repoDir string) error {
	// Prefer origin/main if the remote ref exists.
	if out, err := exec.Command("git", "-C", repoDir, "rev-parse", "--verify", "origin/main").Output(); err == nil && strings.TrimSpace(string(out)) != "" {
		if err := exec.Command("git", "-C", repoDir, "checkout", "origin/main").Run(); err != nil {
			return err
		}
		return nil
	}
	_ = exec.Command("git", "-C", repoDir, "checkout", "main").Run()
	return nil
}

func (m *Manager) CommitAll(repoDir, message string) error {
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %w\noutput: %s", err, string(output))
	}

	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Lobsterdog Contributors",
		"GIT_AUTHOR_EMAIL=noreply@lobsterdog.dev",
		"GIT_COMMITTER_NAME=Lobsterdog Contributors",
		"GIT_COMMITTER_EMAIL=noreply@lobsterdog.dev",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("git commit failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

// CommitAcrossRepos commits staged-and-unstaged changes in every provided
// working dir with a consistent message referencing the feature ID. Repos
// with no changes to commit are silently skipped (CommitAll returns nil).
func (m *Manager) CommitAcrossRepos(repos []*RepoWorkDir, featureID string) error {
	message := fmt.Sprintf("feat(%s): implement changes", featureID)
	for _, rwd := range repos {
		if err := m.CommitAll(rwd.Dir, message); err != nil {
			return fmt.Errorf("committing in %s: %w", rwd.Name, err)
		}
	}
	return nil
}

// PushBranch pushes the feature branch for the given featureID to origin
// in repoDir. Sets upstream on first push. Returns nil if there is nothing
// to push (e.g. branch already up-to-date).
func (m *Manager) PushBranch(repoDir, featureID string) error {
	branchName := FeatureBranchName(featureID)

	// Check whether the remote branch exists.
	hasRemote, err := m.hasRemoteBranch(repoDir, branchName)
	if err != nil {
		return fmt.Errorf("checking remote branch in %s: %w", repoDir, err)
	}

	args := []string{"push"}
	if !hasRemote {
		args = append(args, "-u", "origin", branchName)
	} else {
		args = append(args, "origin", branchName)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push failed in %s: %w\noutput: %s", repoDir, err, string(output))
	}
	return nil
}

// PushAcrossRepos pushes the feature branch in every provided working dir.
// Errors from individual repos are collected; the function returns a
// multi-error describing all failures so a single broken push doesn't hide
// successful ones. Repos with no changes are silently skipped.
func (m *Manager) PushAcrossRepos(repos []*RepoWorkDir, featureID string) error {
	var errs []string
	for _, rwd := range repos {
		hasChanges, err := m.hasUnpushedCommits(rwd.Dir, featureID)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: checking unpushed commits: %v", rwd.Name, err))
			continue
		}
		if !hasChanges {
			continue
		}
		if err := m.PushBranch(rwd.Dir, featureID); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", rwd.Name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("push failures: %s", strings.Join(errs, "; "))
	}
	return nil
}

// GetWorkDir returns the per-feature working directory for a repo.
// Layout: <baseDir>/worktrees/<featureID>/<repoName>. This isolates
// concurrent features on the same repo so their feature branches don't
// clobber each other in a shared clone.
func (m *Manager) GetWorkDir(repoName, featureID string) string {
	return filepath.Join(m.worktreesDir, featureID, repoName)
}

// GetWorkDirLegacy returns the shared-by-repo-name working directory used
// by older code paths. Retained for backward compatibility with tests that
// pass only a repo name. New code should use GetWorkDir(repoName, featureID).
func (m *Manager) GetWorkDirLegacy(repoName string) string {
	return filepath.Join(m.worktreesDir, repoName)
}

func (m *Manager) IsRepoCloned(repoName string) bool {
	dir := m.GetWorkDirLegacy(repoName)
	gitDir := filepath.Join(dir, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

// IsRepoClonedFor returns true if the per-feature worktree for repoName
// and featureID already exists on disk.
func (m *Manager) IsRepoClonedFor(repoName, featureID string) bool {
	gitDir := filepath.Join(m.GetWorkDir(repoName, featureID), ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

func (m *Manager) FetchOrigin(repoDir string) error {
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

func (m *Manager) IsBuildable(repoDir string) bool {
	for _, buildFile := range []string{"go.mod", "package.json", "Makefile", "Cargo.toml"} {
		if _, err := os.Stat(filepath.Join(repoDir, buildFile)); err == nil {
			return true
		}
	}
	return false
}

type RepoWorkDir struct {
	Name   string
	URL    string
	Dir    string
	Branch string
}

// PrepareRepos clones (or reuses) every repo in refs, creates
// feature/<featureID> in each, and returns the working dirs. The branch
// is stored on each RepoWorkDir so callers (Pipeline.PushPhaseChanges)
// know which ref to push.
func (m *Manager) PrepareRepos(refs []feature.RepoRef, featureID string) ([]*RepoWorkDir, error) {
	branchName := FeatureBranchName(featureID)
	var workDirs []*RepoWorkDir
	for _, repo := range refs {
		cfgRepo := &config.RepoEntry{
			Name: repo.Name,
			URL:  repo.URL,
		}
		dir, err := m.CloneForFeature(cfgRepo, featureID)
		if err != nil {
			return nil, fmt.Errorf("preparing repo %s: %w", repo.Name, err)
		}
		if err := m.CreateFeatureBranch(dir, featureID); err != nil {
			return nil, fmt.Errorf("creating feature branch in %s: %w", repo.Name, err)
		}
		workDirs = append(workDirs, &RepoWorkDir{
			Name:   repo.Name,
			URL:    repo.URL,
			Dir:    dir,
			Branch: branchName,
		})
	}
	return workDirs, nil
}

// RemoveWorktreeFor removes the per-feature working directory for a repo.
// Called during feature cleanup to avoid accumulating worktrees on disk.
// Safe to call if the directory doesn't exist.
func (m *Manager) RemoveWorktreeFor(repoName, featureID string) error {
	dir := m.GetWorkDir(repoName, featureID)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(dir)
}

// RemoveAllWorktreesFor removes every per-feature working directory for
// the given featureID. Use after a feature is merged or cancelled.
func (m *Manager) RemoveAllWorktreesFor(featureID string) error {
	featureDir := filepath.Join(m.worktreesDir, featureID)
	if _, err := os.Stat(featureDir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(featureDir)
}

func (m *Manager) LoadReposConfig() (*config.ReposConfig, error) {
	path := filepath.Join(m.baseDir, "repos.yaml")
	return config.LoadRepos(path)
}

// FeatureBranchName returns the canonical feature branch name for a feature ID.
// All repos participating in a feature use the same branch name so the PR
// title/body can reference it consistently.
func FeatureBranchName(featureID string) string {
	return "feature/" + featureID
}

// hasRemoteBranch reports whether origin/<branch> exists in repoDir.
func (m *Manager) hasRemoteBranch(repoDir, branch string) (bool, error) {
	cmd := exec.Command("git", "-C", repoDir, "ls-remote", "--heads", "origin", branch)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Remote may be unreachable during local-only tests. Treat as
		// "no remote branch" so PushBranch attempts -u origin.
		return false, nil
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// hasUnpushedCommits reports whether the local feature branch has commits
// worth pushing: either new commits beyond what's already on the remote
// feature branch, or a feature branch that doesn't yet exist on the remote
// AND has commits beyond main. Used by PushAcrossRepos to skip repos with
// no new work. Returns false on any error to avoid pushing garbage.
func (m *Manager) hasUnpushedCommits(repoDir, featureID string) (bool, error) {
	branchName := FeatureBranchName(featureID)

	// Compare the feature branch to origin/main. If they're equal, there
	// are no new commits to push regardless of whether the remote feature
	// branch exists.
	mainRef := "origin/main"
	if out, err := exec.Command("git", "-C", repoDir, "rev-parse", "--verify", mainRef).Output(); err != nil || strings.TrimSpace(string(out)) == "" {
		// No origin/main — fall back to comparing against HEAD's parent.
		mainRef = "HEAD~1"
	}
	cmd := exec.Command("git", "-C", repoDir, "rev-list", "--count", branchName+"..."+mainRef)
	if out, err := cmd.Output(); err == nil {
		if n := strings.TrimSpace(string(out)); n == "" || n == "0" {
			return false, nil
		}
	} else {
		// Branch may not exist yet; nothing to push.
		return false, nil
	}

	// Compare against the upstream tracking branch if it exists. If the
	// remote already has the same commits, no push needed.
	upstream := "origin/" + branchName
	cmd = exec.Command("git", "-C", repoDir, "rev-list", "--count", branchName+"..."+upstream)
	if out, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(out)) != "0", nil
	}
	// No upstream ref yet → first push of new commits.
	return true, nil
}
