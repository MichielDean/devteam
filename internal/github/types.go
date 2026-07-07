package github

import (
	"time"
)

// Config carries the resolved github: block values for client construction
// (feature github-authorization-integration, U-01). The config loader
// (internal/config) validates these; this struct is the construction-time
// shape passed to NewClient and the concrete constructors.
type Config struct {
	Provider               string        // "native" (default) | "gh"
	AppID                  int64         // GitHub App ID
	InstallationID          int64         // App installation ID
	PrivateKeyPath         string        // path to App private key PEM (NOT the master key)
	TokenCacheTTL          time.Duration // < 60m; default 9m
	PATFallbackEnabled     bool          // FR-AUTH-02; PAT stored via `devteam auth store-pat`
	ConflictPollMaxRetries int           // NFR-PERF-02; default 5, ceiling 10
	ConflictPollMaxDuration time.Duration // NFR-PERF-02; default 60s, ceiling 300s
	BaseDir                string        // worktree base dir (for GhCLIClient's cmd.Dir)
	Credstore              *Credstore    // for NativeClient key/token read+persist; nil for GhCLIClient
}

// RepoRef identifies a repository by owner + name (ADR-04 domain type).
// Callers construct this; it does not carry installation or settings state.
type RepoRef struct {
	Owner string
	Name  string
}

// Repository is the discovery result (FR-DISC-01, ADR-04). Carries only the
// fields the feature needs — no leaky abstraction of go-github's full struct.
type Repository struct {
	Ref             RepoRef
	DefaultBranch   string
	InstallationID  int64
}

// PRRef identifies a pull request (ADR-04). URL is the human-readable web URL
// returned by CreatePR; Number is the API identifier for subsequent ops.
type PRRef struct {
	Repo   RepoRef
	Number int64
	URL    string
}

// MergeableState is the typed enum for a PR's mergeability (FR-CONFLICT-01,
// ADR-07). The third value, MergeableUnknown, is returned when the poll bound
// is exhausted — it is NEVER silently promoted to Mergeable (R-09, F-6).
type MergeableState int

const (
	// Mergeable means GitHub reports the PR can be merged cleanly.
	Mergeable MergeableState = iota
	// Conflicting means GitHub reports the PR has merge conflicts.
	Conflicting
	// MergeableUnknown means the poll bound was exhausted before GitHub
	// returned a definitive state. The pipeline treats this as "needs
	// investigation" and halts (interaction-spec §6.2, US-05 criterion 3).
	MergeableUnknown
)

// String returns the lowercase string form used in banners and CLI output
// (interaction-spec §8.4: "mergeable", "conflicting", "unknown").
func (m MergeableState) String() string {
	switch m {
	case Mergeable:
		return "mergeable"
	case Conflicting:
		return "conflicting"
	default:
		return "unknown"
	}
}