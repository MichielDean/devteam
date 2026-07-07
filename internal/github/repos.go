package github

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/go-github/v66/github"
)

// ListRepositories discovers the repos the GitHub App installation can access
// (FR-DISC-01, U-04). Calls GET /installation/repositories, returns []Repository
// (domain type, ADR-04) filtered to the configured installation (C-09 single-
// installation MVP — FR-DISC-03).
//
// The caller (the pipeline or `devteam repo list`) persists the result to
// repo_registry via RepoRegistryStore; this method is the API read only.
func (nc *NativeClient) ListRepositories(ctx context.Context) ([]Repository, error) {
	c, err := nc.ensureClientWithFreshToken(ctx)
	if err != nil {
		return nil, err
	}
	var out []Repository
	opts := &github.ListOptions{PerPage: 100}
	for {
		repos, resp, err := c.Apps.ListRepos(ctx, opts)
		if err != nil {
			return nil, redactTokenURLs(fmt.Errorf("ListRepositories: %w", err))
		}
		for _, r := range repos.Repositories {
			out = append(out, Repository{
				Ref: RepoRef{
					Owner: r.GetOwner().GetLogin(),
					Name:  r.GetName(),
				},
				DefaultBranch:  r.GetDefaultBranch(),
				InstallationID: nc.installationID,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// SettingsReader is a concrete struct (ADR-13), DB-backed, read by
// NativeClient (resolveBase/resolveDraft) and config loaders (U-07
// materialization). Tests use a real test-DB-backed SettingsReader or
// construct one with a mock *sql.DB (the existing internal/db/ pattern).
//
// NOT an interface — single implementation, no premature abstraction
// (principle 4, ADR-13). If Phase 2 adds a cache, the struct becomes an
// interface then; the change is localized.
//
// Callers pass a *SettingsReader (pointer) so nil can represent "no settings
// available" (the resolveBase/resolveDraft fallback path).
type SettingsReader struct {
	db *sql.DB
}

// NewSettingsReader constructs a DB-backed *SettingsReader.
func NewSettingsReader(db *sql.DB) *SettingsReader {
	return &SettingsReader{db: db}
}

// DefaultBranch returns the per-repo default_branch from repo_settings,
// or "" if no settings row exists (the caller applies the "main" fallback).
func (sr *SettingsReader) DefaultBranch(repo RepoRef) string {
	if sr == nil || sr.db == nil {
		return ""
	}
	var b string
	err := sr.db.QueryRow(
		`SELECT s.default_branch FROM repo_settings s
		 JOIN repo_registry r ON r.id = s.repo_registry_id
		 WHERE r.owner = ? AND r.name = ?`,
		repo.Owner, repo.Name,
	).Scan(&b)
	if err != nil {
		return ""
	}
	return b
}

// PrDraftDefault returns the per-repo pr_draft_default (true if no settings row).
func (sr *SettingsReader) PrDraftDefault(repo RepoRef) bool {
	if sr == nil || sr.db == nil {
		return true
	}
	var v int
	err := sr.db.QueryRow(
		`SELECT s.pr_draft_default FROM repo_settings s
		 JOIN repo_registry r ON r.id = s.repo_registry_id
		 WHERE r.owner = ? AND r.name = ?`,
		repo.Owner, repo.Name,
	).Scan(&v)
	if err != nil {
		return true
	}
	return v == 1
}

// ConflictDetectionEnabled returns whether conflict detection is on for a repo.
func (sr *SettingsReader) ConflictDetectionEnabled(repo RepoRef) bool {
	if sr == nil || sr.db == nil {
		return true
	}
	var v int
	err := sr.db.QueryRow(
		`SELECT s.conflict_detection_enabled FROM repo_settings s
		 JOIN repo_registry r ON r.id = s.repo_registry_id
		 WHERE r.owner = ? AND r.name = ?`,
		repo.Owner, repo.Name,
	).Scan(&v)
	if err != nil {
		return true
	}
	return v == 1
}

// Provider returns the per-repo provider override ("native"|"gh") or "" if
// no override (global provider applies — FR-SETTINGS-05, ADR-17).
func (sr *SettingsReader) Provider(repo RepoRef) string {
	if sr == nil || sr.db == nil {
		return ""
	}
	var p string
	err := sr.db.QueryRow(
		`SELECT s.provider FROM repo_settings s
		 JOIN repo_registry r ON r.id = s.repo_registry_id
		 WHERE r.owner = ? AND r.name = ?`,
		repo.Owner, repo.Name,
	).Scan(&p)
	if err != nil {
		return ""
	}
	return p
}