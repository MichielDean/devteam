package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v66/github"
)

// CreateBranch creates a branch on the remote repo via the GitHub git refs API
// (FR-PR-01). `from` is the source ref (e.g. "main" or a full SHA); the new
// branch points at from's current HEAD.
func (nc *NativeClient) CreateBranch(ctx context.Context, repo RepoRef, branch, from string) error {
	c, err := nc.ensureClientWithFreshToken(ctx)
	if err != nil {
		return err
	}
	// Resolve `from` to a SHA. If it's already a 40-char hex, use it directly;
	// otherwise treat it as a branch name and resolve via the branch endpoint.
	var sha string
	if isFullSHA(from) {
		sha = from
	} else {
		br, _, err := c.Repositories.GetBranch(ctx, repo.Owner, repo.Name, from, 10)
		if err != nil {
			return redactTokenURLs(fmt.Errorf("CreateBranch: resolving source branch %s: %w", from, err))
		}
		sha = br.GetCommit().GetSHA()
	}
	ref := "refs/heads/" + branch
	_, _, err = c.Git.CreateRef(ctx, repo.Owner, repo.Name, &github.Reference{
		Ref: &ref,
		Object: &github.GitObject{
			SHA: &sha,
		},
	})
	if err != nil {
		return redactTokenURLs(fmt.Errorf("CreateBranch: creating %s from %s: %w", branch, from, err))
	}
	return nil
}

// CreatePR creates a pull request targeting `base` (FR-PR-02). `base` and
// `draft` are caller-supplied (ADR-05); the helpers resolveBase/resolveDraft
// read repo_settings and pass the resolved values. Empty base → "main"
// fallback (C-11, I-03 fix).
func (nc *NativeClient) CreatePR(ctx context.Context, repo RepoRef, base, head, title, body string, draft bool) (*PRRef, error) {
	if base == "" {
		base = "main"
	}
	c, err := nc.ensureClientWithFreshToken(ctx)
	if err != nil {
		return nil, err
	}
	newPR := &github.NewPullRequest{
		Base:   &base,
		Head:   &head,
		Title:  &title,
		Body:   &body,
		Draft:  &draft,
	}
	pr, _, err := c.PullRequests.Create(ctx, repo.Owner, repo.Name, newPR)
	if err != nil {
		return nil, redactTokenURLs(fmt.Errorf("CreatePR on %s/%s (base=%s head=%s): %w", repo.Owner, repo.Name, base, head, err))
	}
	return &PRRef{
		Repo:   repo,
		Number: int64(pr.GetNumber()),
		URL:    pr.GetHTMLURL(),
	}, nil
}

// ReadyPR transitions a draft PR to ready-for-review (FR-PR-03) via
// PATCH /repos/{owner}/{repo}/pulls/{pr} with draft=false.
func (nc *NativeClient) ReadyPR(ctx context.Context, repo RepoRef, number int64) error {
	c, err := nc.ensureClientWithFreshToken(ctx)
	if err != nil {
		return err
	}
	_, _, err = c.PullRequests.Edit(ctx, repo.Owner, repo.Name, int(number), &github.PullRequest{
		Draft: github.Bool(false),
	})
	if err != nil {
		return redactTokenURLs(fmt.Errorf("ReadyPR %s/%s#%d: %w", repo.Owner, repo.Name, number, err))
	}
	return nil
}

// CommentPR posts a comment on a PR (FR-PR-04) via
// POST /repos/{owner}/{repo}/pulls/{pr}/comments.
func (nc *NativeClient) CommentPR(ctx context.Context, repo RepoRef, number int64, body string) error {
	c, err := nc.ensureClientWithFreshToken(ctx)
	if err != nil {
		return err
	}
	_, _, err = c.Issues.CreateComment(ctx, repo.Owner, repo.Name, int(number), &github.IssueComment{
		Body: &body,
	})
	if err != nil {
		return redactTokenURLs(fmt.Errorf("CommentPR %s/%s#%d: %w", repo.Owner, repo.Name, number, err))
	}
	return nil
}

// isFullSHA reports whether s is a 40-char lowercase hex SHA.
func isFullSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// resolveBase reads repo_settings.default_branch for the given repo; empty
// (no settings row) → "main" (C-11, FR-PR-02 main fallback). Used by the
// pipeline's PR-creation helper (U-08 pr_helpers.go conceptually; inlined here
// for the NativeClient path because the helper needs the DB + repo ID).
//
// This is a thin wrapper; the SettingsReader (repos.go) is the actual DB
// reader. We keep it here so NativeClient stays self-contained for the
// gh-free run path (NFR-PORT-02) without a separate settings dependency.
func resolveBase(settings *SettingsReader, repo RepoRef) string {
	if settings == nil {
		return "main"
	}
	if b := settings.DefaultBranch(repo); b != "" {
		return b
	}
	return "main"
}

// resolveDraft reads repo_settings.pr_draft_default; no settings row → true
// (matches the legacy `--draft` default, I-03).
func resolveDraft(settings *SettingsReader, repo RepoRef) bool {
	if settings == nil {
		return true
	}
	return settings.PrDraftDefault(repo)
}

// CreatePRWithSettings is the pipeline-facing PR creator that resolves base +
// draft from repo_settings before calling CreatePR (ADR-05, FR-PR-02). The
// interface's CreatePR takes explicit base/draft; this helper is the policy
// site that reads settings. The pipeline calls this, not CreatePR directly,
// when per-repo settings should apply (the gh-free run path, U-08).
func (nc *NativeClient) CreatePRWithSettings(ctx context.Context, settings *SettingsReader, repo RepoRef, head, title, body string) (*PRRef, error) {
	base := resolveBase(settings, repo)
	draft := resolveDraft(settings, repo)
	return nc.CreatePR(ctx, repo, base, head, title, body, draft)
}

// _ is a placeholder to keep the imports used if the helpers above reference
// strings only minimally (avoids "imported and not used" in some build paths).
var _ = strings.TrimSpace