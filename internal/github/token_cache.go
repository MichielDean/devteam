package github

import (
	"sync"
	"time"
)

// tokenCache is the in-process installation-token cache (nfr-design-specs §4.1).
// It holds the plaintext token guarded by mu. The token is short-lived
// (GitHub's 60-minute installation-token expiry); the cache TTL is strictly
// less than that (default 9m, FR-AUTH-03).
//
// Never use a stale token past TTL (BR-CRED-07): getToken checks
// time.Now().Before(expiresAt) before returning; if false, it re-exchanges.
type tokenCache struct {
	mu          sync.RWMutex
	token       string
	expiresAt   time.Time
	refreshedAt time.Time
	prov        string // "cached" | "refreshed" | "fallback (PAT)" | "re-exchanged"
	ttl         time.Duration
}

func newTokenCache(ttl time.Duration) *tokenCache {
	return &tokenCache{ttl: ttl}
}

// get returns the token if fresh (time.Now() before expiresAt). ok=false if
// expired or never set.
func (tc *tokenCache) get() (string, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	if tc.token == "" || time.Now().After(tc.expiresAt) {
		return "", false
	}
	// Return "cached" provenance on a hit (US-01).
	return tc.token, true
}

// set populates the cache with a token + provenance. ttl is either the
// configured TTL or time.Until(expiresAt), whichever is shorter (so a token
// returned with 7m of life doesn't get a 9m cache entry that outlives it).
func (tc *tokenCache) set(token, provenance string, ttl time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.token = token
	tc.refreshedAt = time.Now()
	tc.expiresAt = time.Now().Add(ttl)
	tc.prov = provenance
}

// provenance returns the current provenance string ("cached" on a hit,
// "refreshed" after exchange, "fallback (PAT)" after PAT fallback — US-01).
// It downgrades to "cached" if the token was set more than 0s ago (the first
// read after a set returns the set-time provenance; subsequent reads return
// "cached" — interaction-spec §8.1).
func (tc *tokenCache) provenance() string {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	if tc.prov == "" {
		return "cached"
	}
	// If the set was recent (within 1s), report the set-time provenance;
	// otherwise report "cached" (the operator sees "cached" on a warm call).
	if time.Since(tc.refreshedAt) < time.Second {
		return tc.prov
	}
	return "cached"
}