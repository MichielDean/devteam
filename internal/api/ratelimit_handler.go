package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/MichielDean/devteam/internal/ratelimit"
)

// writeRateLimitRejection shapes the 429 response (U-E/US4/BR-19..BR-25). Sets
// the LOCKED header set (BR-20) on w.Header() BEFORE writeError calls
// WriteHeader (invariant 1/BR-18 — stdlib locks the header map at WriteHeader
// time; setting after is a silent no-op). The body is the existing
// ErrorResponse{Error, Details} shape ONLY (RC-1/BR-21 — corrects C-1: NO key,
// policy, retry_after_seconds, or request_id in the body; machine data lives in
// headers). The details text includes the matched route and the Retry-After
// value so body-only clients get the retry hint.
//
// No request_id anywhere in v1 (RC-3 — no request-id middleware exists; a
// ` request_id=%s` hook is left in the code comment for the future maintainer).
//
// The caller (rateLimitMiddleware) logs the composite key separately; the key
// is NOT a parameter here because it is not put in the body (BR-21) — passing it
// would be dead weight.
func (s *Server) writeRateLimitRejection(w http.ResponseWriter, v ratelimit.Verdict, route string) {
	retryAfter := secondsStr(v.ResetIn)
	// 429 header set (BR-20). Content-Type is set by writeError/writeJSON.
	w.Header().Set("Retry-After", retryAfter)
	w.Header().Set("RateLimit-Limit", fmt.Sprintf("%d", v.Limit))
	w.Header().Set("RateLimit-Remaining", "0") // always 0 on a rejection (BR-20)
	w.Header().Set("RateLimit-Reset", retryAfter)
	w.Header().Set("X-RateLimit-Policy", fmt.Sprintf("%d;w=%d", v.Limit, int(v.Window.Seconds())))
	// Body: ErrorResponse{Error:"rate_limit_exceeded", Details:"..."} ONLY (BR-21).
	// The details text carries the route + Retry-After for body-only clients.
	details := fmt.Sprintf("Rate limit exceeded for %s; retry after %ss.", route, retryAfter)
	writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded", details)
	// Future: if a request-id middleware is added, echo it here.
	//   w.Header().Set("X-Request-Id", requestID)  // (request_id=%s hook, RC-3)
}

// handleRateLimitStatus is the GET /health/rate-limit status handler (U-F/US5).
// It is exempt by route match BEFORE Allow (BR-09/BR-26) — it NEVER calls Allow
// and NEVER increments counters (exemption is structural, D3). It is always 200,
// even when failures_total > 0 (the endpoint reporting its own fail-open state
// is itself healthy; status is always "healthy" in v1 — the counters are the
// degradation signal, NDP-18). It is registered ONLY when the limiter is armed
// (BR-47 — when disabled, the route 404s).
//
// No IP redaction in v1 (O-4/BR-30 — corrects C-4): full IPs shown in
// active_keys[].key (composite ip|route) because the endpoint is operator-only
// and redaction would destroy the diagnostic value. No redact_ips config field.
// active_keys_truncated is a BOOL field (O-2/BR-29 — corrects C-5: NOT a
// synthetic active_keys entry). active_keys is an ARRAY sorted by count
// DESCENDING (hottest first), never null (json:"active_keys" tag, NOT omitempty,
// BR-28). The cap is max_tracked_keys from config (default 10000).
func (s *Server) handleRateLimitStatus(w http.ResponseWriter, r *http.Request) {
	if s.rateLimiter == nil || s.rlCfg == nil {
		// Defensive: the route is registered only when armed (BR-47), so this
		// should not be reachable. Return 404 to match the disabled state.
		http.NotFound(w, r)
		return
	}

	cfg := s.rlCfg
	cap := cfg.GetMaxTrackedKeys()
	snap := s.rateLimiter.Snapshot(cap)
	// BR-29/O-2: truncated iff the limiter tracked MORE keys than the cap
	// (TotalKeys is the pre-truncation count, observed atomically with Keys
	// under the same lock — no TOCTOU window). NOT len(Keys) >= cap, which
	// false-positives when exactly cap keys exist (no truncation occurred).
	truncated := cap > 0 && snap.TotalKeys > cap

	// Build active_keys view (M2.1 — array, sorted by count desc, bounded by cap).
	activeKeys := make([]activeKeyView, 0, len(snap.Keys))
	for _, ks := range snap.Keys {
		activeKeys = append(activeKeys, activeKeyView{
			Key:            ks.Key,
			Count:          ks.Count,
			Limit:          ks.Limit,
			WindowSeconds:  ks.WindowSeconds,
			ResetInSeconds: ks.ResetInSeconds,
		})
	}

	// endpoint_overrides is an ARRAY (not map) with stable order (M2.1/BR-27).
	overrides := make([]overrideView, 0, len(cfg.EndpointOverrides))
	for _, ov := range cfg.EndpointOverridesList() {
		v := overrideView{Route: ov.Route, Exempt: ov.Exempt}
		if ov.Limit != nil {
			v.Limit = ov.Limit
		}
		if ov.WindowSeconds != nil {
			v.WindowSeconds = ov.WindowSeconds
		}
		overrides = append(overrides, v)
	}

	// defaults view — always present the resolved (default-applied) values.
	defaultsView := defaultsView{
		Limit:         cfg.GetDefaultLimit(),
		WindowSeconds: cfg.GetDefaultWindowSeconds(),
	}

	resp := rateLimitStatusResponse{
		Status:               "healthy",
		Enabled:              cfg.Enabled,
		Limiter:              "sliding_window",
		ConfigSource:         cfg.ConfigSource,
		TrustProxyHeaders:    cfg.GetTrustProxyHeaders(),
		DryRun:               cfg.GetDryRun(),
		FailMode:             s.failMode(),
		Defaults:             defaultsView,
		EndpointOverrides:    overrides,
		ActiveKeys:           activeKeys,
		ActiveKeysTruncated:  truncated,
		RejectionsTotal:      snap.RejectionsTotal,
		FailuresTotal:        snap.FailuresTotal,
		GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// Use a fixed key order via a json struct (Go encodes struct fields in
	// declaration order, which matches the LOCKED M2.1 key order — BR-27).
	_ = json.NewEncoder(w).Encode(resp)
}

// rateLimitStatusResponse is the LOCKED status schema (M2.1/BR-27). Field
// declaration order == JSON key order (Go's encoding/json preserves struct
// field order). active_keys is `json:"active_keys"` (NOT omitempty) so it
// marshals to [] not null when empty (BR-28).
type rateLimitStatusResponse struct {
	Status              string         `json:"status"`
	Enabled             bool           `json:"enabled"`
	Limiter             string         `json:"limiter"`
	ConfigSource        string         `json:"config_source"`
	TrustProxyHeaders   bool           `json:"trust_proxy_headers"`
	DryRun              bool           `json:"dry_run"`
	FailMode            string         `json:"fail_mode"`
	Defaults            defaultsView   `json:"defaults"`
	EndpointOverrides   []overrideView `json:"endpoint_overrides"`
	ActiveKeys          []activeKeyView `json:"active_keys"`
	ActiveKeysTruncated bool           `json:"active_keys_truncated"`
	RejectionsTotal     int64          `json:"rejections_total"`
	FailuresTotal       int64          `json:"failures_total"`
	GeneratedAt         string         `json:"generated_at"`
}

type defaultsView struct {
	Limit         int `json:"limit"`
	WindowSeconds int `json:"window_seconds"`
}

type overrideView struct {
	Route         string `json:"route"`
	Limit         *int   `json:"limit,omitempty"`
	WindowSeconds *int   `json:"window_seconds,omitempty"`
	Exempt        bool   `json:"exempt,omitempty"`
}

type activeKeyView struct {
	Key            string `json:"key"`
	Count          int64  `json:"count"`
	Limit          int    `json:"limit"`
	WindowSeconds  int    `json:"window_seconds"`
	ResetInSeconds int    `json:"reset_in_seconds"`
}