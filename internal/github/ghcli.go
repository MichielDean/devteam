package github

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GhCLIClient implements GitHubClient by delegating the four PR ops to the
// existing gh-CLI shelling behavior (ADR-02, ADR-03, U-03). It is a fallback,
// not a parallel implementation — the gh-free run path is the feature's goal
// (C-05, FR-PR-05), and this adapter exists so the operator can revert to the
// pre-feature behavior without a code change (provider: gh in config).
//
// Byte-identical legacy behavior (ADR-03, FR-IFACE-03): CLI args, stdout
// parsing, error wrapping, opaque-output treatment are preserved verbatim
// from internal/gitops/client.go. The fragile GetPRNumber string-parse is
// preserved ("do not improve" — RE §3.2 anti-pattern 3).
//
// Net-new methods (AuthHealthCheck, ListRepositories, GetMergeableState,
// CreateBranch) return ErrUnsupported (ADR-08) — the adapter is honest about
// its limits.
type GhCLIClient struct {
	baseDir string // cmd.Dir for gh subprocesses
}

// NewGhCLIClient constructs the fallback adapter. The baseDir is the worktree
// root for gh subprocess cwd (matches the legacy gitops.GitClient.baseDir).
func NewGhCLIClient(cfg Config) (*GhCLIClient, error) {
	return &GhCLIClient{baseDir: cfg.BaseDir}, nil
}

// AuthHealthCheck returns ErrUnsupported (ADR-08). The gh adapter does not
// back discovery/auth-health — those require provider: native.
func (g *GhCLIClient) AuthHealthCheck(ctx context.Context) error {
	// But we DO check gh is on PATH (NFR-PORT-03): if the operator set
	// provider: gh but gh is absent, fail with a typed error pointing at §11.
	if _, err := exec.LookPath("gh"); err != nil {
		return authErrorFromCode(ErrCodeGhCLINotFound,
			"provider: gh requires the gh CLI on PATH; either install gh or set github.provider=native")
	}
	return ErrUnsupported
}

// ListRepositories returns ErrUnsupported (ADR-08).
func (g *GhCLIClient) ListRepositories(ctx context.Context) ([]Repository, error) {
	return nil, ErrUnsupported
}

// GetMergeableState returns ErrUnsupported (ADR-08).
func (g *GhCLIClient) GetMergeableState(ctx context.Context, repo RepoRef, number int64) (MergeableState, error) {
	return MergeableUnknown, ErrUnsupported
}

// CreateBranch returns ErrUnsupported (ADR-08) — the gh adapter does not back
// the native branch-creation path.
func (g *GhCLIClient) CreateBranch(ctx context.Context, repo RepoRef, branch, from string) error {
	return ErrUnsupported
}

// CreatePR delegates to `gh pr create` with the caller-supplied base + draft
// (ADR-05 — the adapter parameterizes the previously-hard-coded --base main /
// --draft). Empty base falls back to "main" (C-11, FR-PR-02 main fallback).
//
// Byte-identical to pre-refactor internal/gitops.GitClient.CreatePullRequest
// except base/draft are now params (the I-03 fix). Output parsing preserved.
func (g *GhCLIClient) CreatePR(ctx context.Context, repo RepoRef, base, head, title, body string, draft bool) (*PRRef, error) {
	if base == "" {
		base = "main"
	}
	args := []string{"pr", "create", "--base", base, "--head", head, "--title", title, "--body", body}
	if draft {
		args = append(args, "--draft")
	}
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = g.baseDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, redactTokenURLs(fmt.Errorf("creating PR: %s: %w", string(out), err))
	}
	url := strings.TrimSpace(string(out))
	// Extract PR number from the URL (gh returns the web URL; parse the trailing /pull/N).
	number := parsePRNumberFromURL(url)
	return &PRRef{Repo: repo, Number: number, URL: url}, nil
}

// ReadyPR delegates to `gh pr ready` (FR-PR-03). The legacy ReadyPullRequest
// took a branch; the interface takes a number (ADR-11). The adapter bridges
// by calling `gh pr ready <number>` (gh accepts the number form).
func (g *GhCLIClient) ReadyPR(ctx context.Context, repo RepoRef, number int64) error {
	cmd := exec.CommandContext(ctx, "gh", "pr", "ready", fmt.Sprintf("%d", number))
	cmd.Dir = g.baseDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return redactTokenURLs(fmt.Errorf("marking PR ready: %s: %w", string(out), err))
	}
	return nil
}

// CommentPR delegates to `gh pr comment <number> --body <body>` (FR-PR-04).
// Byte-identical to the legacy AddPullRequestComment (which looked up the
// number first via GetPRNumber; the interface takes the number directly).
func (g *GhCLIClient) CommentPR(ctx context.Context, repo RepoRef, number int64, body string) error {
	cmd := exec.CommandContext(ctx, "gh", "pr", "comment", fmt.Sprintf("%d", number), "--body", body)
	cmd.Dir = g.baseDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return redactTokenURLs(fmt.Errorf("commenting on PR: %s: %w", string(out), err))
	}
	return nil
}

// GetPRNumberByBranch is a GhCLIClient-specific helper (NOT on the interface)
// used by the legacy gitops.GitClient bridge (ADR-11, ADR-14). It preserves the
// fragile `gh pr list --head <branch> --json number --limit 1` string-parse
// verbatim ("do not improve" — ADR-03, RE §3.2 anti-pattern 3).
func (g *GhCLIClient) GetPRNumberByBranch(ctx context.Context, branch string) (int64, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "list", "--head", branch, "--json", "number", "--limit", "1")
	cmd.Dir = g.baseDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, redactTokenURLs(fmt.Errorf("finding PR for %s: %s: %w", branch, string(out), err))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return 0, fmt.Errorf("no PR found for branch %s", branch)
	}
	parts := strings.Split(lines[0], ":")
	if len(parts) < 2 {
		return 0, fmt.Errorf("unexpected PR list format: %s", lines[0])
	}
	var n int64
	fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &n)
	return n, nil
}

// parsePRNumberFromURL extracts the trailing /pull/<N> from a GitHub PR URL.
// gh's `pr create` returns the web URL; we parse the number for the PRRef.
func parsePRNumberFromURL(url string) int64 {
	idx := strings.LastIndex(url, "/pull/")
	if idx < 0 {
		return 0
	}
	var n int64
	fmt.Sscanf(url[idx+len("/pull/"):], "%d", &n)
	return n
}