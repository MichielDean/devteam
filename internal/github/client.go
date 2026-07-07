package github

import (
	"context"
	"errors"
)

// GitHubClient is the sole interface for GitHub API access in the devteam
// platform (feature github-authorization-integration, U-01, ADR-01, FR-IFACE-01).
//
// The interface uses domain types (RepoRef, Repository, PRRef, MergeableState),
// NOT go-github types, so callers never transitively import go-github
// (ADR-04). NativeClient translates at its boundary.
//
// 7 methods, all taking context.Context first (ADR-04). The GhCLIClient adapter
// implements 3 (CreatePR, ReadyPR, CommentPR) and returns ErrUnsupported for the
// other 4 (ADR-08) — it is a fallback, not a parallel implementation.
type GitHubClient interface {
	AuthHealthCheck(ctx context.Context) error
	ListRepositories(ctx context.Context) ([]Repository, error)
	CreateBranch(ctx context.Context, repo RepoRef, branch, from string) error
	CreatePR(ctx context.Context, repo RepoRef, base, head, title, body string, draft bool) (*PRRef, error)
	ReadyPR(ctx context.Context, repo RepoRef, number int64) error
	CommentPR(ctx context.Context, repo RepoRef, number int64, body string) error
	GetMergeableState(ctx context.Context, repo RepoRef, number int64) (MergeableState, error)
}

// ErrUnsupported is the sentinel returned by GhCLIClient for interface methods
// it does not back (AuthHealthCheck, ListRepositories, GetMergeableState,
// CreateBranch — ADR-08). Callers check errors.Is(err, ErrUnsupported) and
// either fail with a clear message or fall back to NativeClient.
var ErrUnsupported = errors.New("gh CLI adapter does not support this method")

// NewClient is the factory seam (ADR-02, FR-AUTH-05). It selects NativeClient
// (default) or GhCLIClient from the provider config. Concrete constructors
// live in U-02 (native.go) and U-03 (ghcli.go); this function is the single
// dispatch point the pipeline and CLI use.
//
// Until U-02 lands, provider=native returns a clear "native client not
// implemented" error (U-01 acceptance). With U-02, it returns *NativeClient.
func NewClient(cfg Config) (GitHubClient, error) {
	switch cfg.Provider {
	case "", "native":
		return NewNativeClient(cfg)
	case "gh":
		return NewGhCLIClient(cfg)
	default:
		return nil, errors.New("github.provider: must be 'native' or 'gh'; got '" + cfg.Provider + "'")
	}
}