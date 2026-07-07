package github

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v66/github"
)

// GetMergeableState returns the PR's mergeability, polling with bounded
// exponential backoff when GitHub reports `null`/`unknown` (FR-CONFLICT-02,
// ADR-07, U-09). The poll loop is encapsulated INSIDE this method (ADR-07):
// the caller sees only the final state + diagnostic.
//
// Bounds (nc.pollMaxRetries, nc.pollMaxDuration) are constructor config
// (NFR-PERF-02, defaults 5 / 60s, hard ceilings 10 / 300s enforced at config
// load). On exhaustion: return MergeableUnknown + diagnostic — NEVER
// silently Mergeable (R-09, F-6 invariant, ADR-07).
//
// context.Context cancellation is honored mid-poll (the select on ctx.Done()
// during the backoff sleep).
func (nc *NativeClient) GetMergeableState(ctx context.Context, repo RepoRef, number int64) (MergeableState, error) {
	c, err := nc.ensureClientWithFreshToken(ctx)
	if err != nil {
		return MergeableUnknown, err
	}

	var lastSeen string
	deadline := time.Now().Add(nc.pollMaxDuration)
	backoff := 1 * time.Second
	const backoffCap = 5 * time.Second

	for attempt := 0; attempt < nc.pollMaxRetries; attempt++ {
		pr, _, err := c.PullRequests.Get(ctx, repo.Owner, repo.Name, int(number))
		if err != nil {
			return MergeableUnknown, redactTokenURLs(fmt.Errorf("GetMergeableState %s/%s#%d: %w", repo.Owner, repo.Name, number, err))
		}
		state := pr.GetMergeableState()
		lastSeen = state
		switch state {
		case "clean", "has_hooks":
			return Mergeable, nil
		case "dirty", "blocked":
			return Conflicting, nil
		case "behind":
			// "behind" means the PR needs rebase — treat as conflicting for the
			// pipeline's halt decision (the operator must rebase before merge).
			return Conflicting, nil
		case "unstable":
			// "unstable" means status checks are running — poll again (GitHub
			// hasn't reached a definitive mergeable state).
		}
		// state == "" (null) or "unknown" → poll.

		// Check deadline before sleeping.
		if time.Now().After(deadline) {
			return MergeableUnknown, fmt.Errorf("mergeable_state polling exhausted after %d retries / %s; last seen state: %q",
				attempt+1, time.Since(deadline.Add(-nc.pollMaxDuration)).Round(time.Second), lastSeen)
		}

		// Sleep with ctx cancellation.
		select {
		case <-ctx.Done():
			return MergeableUnknown, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < backoffCap {
			backoff *= 2
			if backoff > backoffCap {
				backoff = backoffCap
			}
		}
	}

	return MergeableUnknown, fmt.Errorf("mergeable_state polling exhausted after %d retries / %s; last seen state: %q",
		nc.pollMaxRetries, nc.pollMaxDuration, lastSeen)
}

// _ keeps the github import used even if the API surface narrows.
var _ = github.String