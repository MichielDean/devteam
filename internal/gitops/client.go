package gitops

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/MichielDean/devteam/internal/github"
)

// GitClient owns local-git operations (clone, branch, commit, push) AND
// bridges the legacy string-based PR-method signatures to the new
// internal/github.GitHubClient interface (feature github-authorization-integration,
// U-05, ADR-14, FR-IFACE-05).
//
// The local-git methods (CreateBranch, StageAll, Commit, Push, CommitAndPush,
// CurrentBranch, HasRemoteBranch, HasStagedChanges, Run) are UNCHANGED
// (C-02, FR-PR-06, NFR-COMPAT-03) — they shell to `git`, not `gh`.
//
// The PR methods (CreatePullRequest, ReadyPullRequest, AddPullRequestComment,
// GetPRNumber) keep their legacy string-based signatures (C-01, R-03) but now
// delegate to the injected ghClient GitHubClient (the interface), not to `gh`
// directly. The interface uses int64 PR numbers + RepoRef; the legacy methods
// bridge by resolving repo + settings + number internally, then calling the
// interface (ADR-14).
//
// A nil ghClient means the bridge is not wired (the pipeline sets it after
// construction); the PR methods return an error in that case (fail-fast, not
// a silent fallback to gh shelling — the gh-free run path is the goal).
type GitClient struct {
	baseDir  string
	ghClient github.GitHubClient
}

// NewGitClient constructs a GitClient with local-git ops only (the legacy
// constructor, preserved for existing callers — pipeline.go's three injection
// sites). The PR methods will fail with "GitHubClient not wired" until
// SetGitHubClient is called.
func NewGitClient(baseDir string) *GitClient {
	return &GitClient{baseDir: baseDir}
}

// NewGitClientWithGitHub constructs a GitClient with the GitHubClient interface
// injected (the post-feature constructor). The pipeline uses this when the
// github: config block is present (FR-IFACE-06).
func NewGitClientWithGitHub(baseDir string, ghClient github.GitHubClient) *GitClient {
	return &GitClient{baseDir: baseDir, ghClient: ghClient}
}

// SetGitHubClient injects the GitHubClient after construction (used by the
// pipeline when it resolves the client from config after the fact — the three
// NewGitClient(baseDir) sites in pipeline.go).
func (g *GitClient) SetGitHubClient(gh github.GitHubClient) {
	g.ghClient = gh
}

// GitHubClient returns the injected GitHubClient (or nil — the pipeline checks
// before calling PR methods).
func (g *GitClient) GitHubClient() github.GitHubClient {
	return g.ghClient
}

func (g *GitClient) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.baseDir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Run is the exported version of run for use by other packages.
func (g *GitClient) Run(args ...string) (string, error) {
	return g.run(args...)
}

func (g *GitClient) CurrentBranch() (string, error) {
	out, err := g.run("branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("getting current branch: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (g *GitClient) HasRemoteBranch(branch string) bool {
	out, err := g.run("ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}

func (g *GitClient) CreateBranch(branch string) error {
	_, err := g.run("checkout", "-b", branch)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			_, err = g.run("checkout", branch)
			if err != nil {
				return fmt.Errorf("checking out existing branch %s: %w", branch, err)
			}
			return nil
		}
		return fmt.Errorf("creating branch %s: %w", branch, err)
	}
	return nil
}

func (g *GitClient) StageAll() error {
	_, err := g.run("add", "-A")
	if err != nil {
		return fmt.Errorf("staging changes: %w", err)
	}
	return nil
}

func (g *GitClient) HasStagedChanges() (bool, error) {
	out, err := g.run("diff", "--cached", "--stat")
	if err != nil {
		return false, fmt.Errorf("checking staged changes: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}

func (g *GitClient) Commit(message string) error {
	_, err := g.run("commit", "-m", message)
	if err != nil {
		return fmt.Errorf("committing: %w", err)
	}
	return nil
}

func (g *GitClient) Push(branch string) error {
	if g.HasRemoteBranch(branch) {
		_, err := g.run("push", "origin", branch)
		if err != nil {
			return fmt.Errorf("pushing to origin/%s: %w", branch, err)
		}
		return nil
	}
	_, err := g.run("push", "-u", "origin", branch)
	if err != nil {
		return fmt.Errorf("pushing -u origin/%s: %w", branch, err)
	}
	return nil
}

func (g *GitClient) CreatePullRequest(branch string, title string, body string) (string, error) {
	if g.ghClient == nil {
		return "", fmt.Errorf("GitHubClient not wired (set github: config block or call SetGitHubClient); see docs/github-app-setup.md")
	}
	repo, err := g.resolveRepoRef()
	if err != nil {
		return "", err
	}
	// Legacy behavior: --base main, --draft (ADR-14 — the bridge preserves the
	// legacy defaults; per-repo settings override is the NativeClient's job via
	// CreatePRWithSettings, not this legacy bridge).
	prRef, err := g.ghClient.CreatePR(context.Background(), repo, "main", branch, title, body, true)
	if err != nil {
		return "", err
	}
	if prRef == nil {
		return "", fmt.Errorf("CreatePR returned nil PRRef")
	}
	return prRef.URL, nil
}

func (g *GitClient) ReadyPullRequest(branch string) error {
	if g.ghClient == nil {
		return fmt.Errorf("GitHubClient not wired (set github: config block or call SetGitHubClient); see docs/github-app-setup.md")
	}
	repo, err := g.resolveRepoRef()
	if err != nil {
		return err
	}
	// Legacy ReadyPullRequest takes a branch; the interface takes a number
	// (ADR-11). Bridge: look up the number by branch via the gh adapter's
	// helper if available, else fail.
	ghAdapter, ok := g.ghClient.(*github.GhCLIClient)
	if !ok {
		return fmt.Errorf("ReadyPullRequest(branch) bridge requires *GhCLIClient; for NativeClient use ReadyPR(ctx, repo, number) directly")
	}
	number, err := ghAdapter.GetPRNumberByBranch(context.Background(), branch)
	if err != nil {
		return err
	}
	return g.ghClient.ReadyPR(context.Background(), repo, number)
}

func (g *GitClient) AddPullRequestComment(branch string, comment string) error {
	if g.ghClient == nil {
		return fmt.Errorf("GitHubClient not wired (set github: config block or call SetGitHubClient); see docs/github-app-setup.md")
	}
	repo, err := g.resolveRepoRef()
	if err != nil {
		return err
	}
	prNumber, err := g.GetPRNumber(branch)
	if err != nil {
		return err
	}
	var number int64
	fmt.Sscanf(prNumber, "%d", &number)
	return g.ghClient.CommentPR(context.Background(), repo, number, comment)
}

func (g *GitClient) GetPRNumber(branch string) (string, error) {
	if g.ghClient == nil {
		return "", fmt.Errorf("GitHubClient not wired (set github: config block or call SetGitHubClient); see docs/github-app-setup.md")
	}
	// Preserve the fragile string-parse verbatim for the GhCLIClient path
	// (ADR-03 "do not improve"). For NativeClient, there's no branch→number
	// lookup in MVP — the kanban-view path that uses this is gh-adapter-only.
	ghAdapter, ok := g.ghClient.(*github.GhCLIClient)
	if !ok {
		return "", fmt.Errorf("GetPRNumber(branch) bridge requires *GhCLIClient; for NativeClient the PR number is returned by CreatePR directly")
	}
	number, err := ghAdapter.GetPRNumberByBranch(context.Background(), branch)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", number), nil
}

// resolveRepoRef derives the RepoRef (owner/name) from the worktree's git remote.
// This is the bridge from the legacy string-based signatures (which take only a
// branch) to the interface's typed RepoRef (ADR-14). It reads `git remote -v`
// to extract the GitHub owner/repo.
func (g *GitClient) resolveRepoRef() (github.RepoRef, error) {
	out, err := g.run("remote", "get-url", "origin")
	if err != nil {
		return github.RepoRef{}, fmt.Errorf("resolving origin remote: %s: %w", strings.TrimSpace(out), err)
	}
	url := strings.TrimSpace(out)
	return parseGitHubURL(url), nil
}

// parseGitHubURL extracts owner/name from a GitHub remote URL (SSH or HTTPS).
func parseGitHubURL(url string) github.RepoRef {
	// SSH: git@github.com:owner/name.git
	// HTTPS: https://github.com/owner/name.git
	var rest string
	if i := strings.Index(url, "github.com"); i >= 0 {
		rest = url[i+len("github.com"):]
	} else {
		return github.RepoRef{}
	}
	rest = strings.TrimPrefix(rest, ":")
	rest = strings.TrimPrefix(rest, "/")
	rest = strings.TrimSuffix(rest, ".git")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 {
		return github.RepoRef{}
	}
	return github.RepoRef{Owner: parts[0], Name: parts[1]}
}

func (g *GitClient) CommitAndPush(branch string, message string) error {
	if err := g.StageAll(); err != nil {
		return err
	}
	hasChanges, err := g.HasStagedChanges()
	if err != nil {
		return err
	}
	if !hasChanges {
		return nil
	}
	if err := g.Commit(message); err != nil {
		return err
	}
	return g.Push(branch)
}