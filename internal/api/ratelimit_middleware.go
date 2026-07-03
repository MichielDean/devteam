package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/ratelimit"
)

// rateLimitStatusRoute is the only exempt-by-default route (BLM §3.2, F-9 —
// no /health route exists today; this route is NEW and registered by
// ConfigureRateLimiting ONLY when the limiter is armed). It is matched
// exactly as "METHOD /path" (BR-09 — no normalization) and short-circuits
// BEFORE Allow is called (§2.13 — structural exemption, not "check then
// exempt the verdict").
const rateLimitStatusRoute = "GET /health/rate-limit"

// rateLimitMiddleware (U-D) is the request lifecycle (BLM §2.1). When the
// limiter is nil (absent config or enabled:false) it is a pure pass-through:
// exactly one nil check and one next.ServeHTTP — no map lookup, no header
// mutation, no log line, no Allow call. The server is byte-identical to the
// pre-feature build (D7/R12, §2.4, SEC-10/PERF-12).
//
// When armed, the lifecycle is: exempt short-circuit (§2.13) → key extraction
// (BR-10/BR-12) → policy resolve (override or default, BR-17) → Allow
// (§2.1/§2.14) → branch on verdict + dry_run (allow / deny+dry_run / deny+force)
// → malfunction catch (§2.1, two-layer §2.2). Advisory headers on allow
// (§2.10). 429 on deny+enforce (BR-19..BR-21).
//
// §2.2 two-layer recovery contract (BR-35/BR-56, invariant 5): the inner
// defer/recover scope is LIMITER-DOMAIN ONLY — key extraction, policy resolve,
// Allow, and header-setting. next.ServeHTTP is called EXACTLY ONCE, OUTSIDE the
// recover scope, so a handler panic propagates to the outer recoveryMiddleware
// (F-8) → 500. The limiter never swallows, re-invokes, or misattributes a
// handler bug. This preserves the pre-feature error contract for every
// existing endpoint (B-1 fix).
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// PERF-12 / §2.4 — the nil branch is EXACTLY two statements. No
		// header mutation, no log, no Allow. Byte-identical to pre-feature.
		if s.rateLimiter == nil {
			next.ServeHTTP(w, r)
			return
		}

		route := r.Method + " " + r.URL.Path

		// §2.13 — exempt BEFORE Allow. NO Allow call, NO header mutation, NO
		// count increment, NO log. The status endpoint is exempt by route;
		// config exempt overrides are matched here too. Exempt routes call
		// next.ServeHTTP directly — OUTSIDE the limiter-domain recover scope,
		// so a handler panic on an exempt route still reaches the outer
		// recoveryMiddleware (BR-35).
		if route == rateLimitStatusRoute {
			next.ServeHTTP(w, r)
			return
		}
		if s.isExempt(route) {
			next.ServeHTTP(w, r)
			return
		}

		// adjudicate runs the limiter-domain work (key extraction, policy
		// resolve, Allow, header-setting) inside a recover scope. A panic in
		// any of those sites is caught here and routed to the fail-open path
		// (§2.1/§2.2 inner layer). If THIS recovery path itself panics, the
		// outer recoveryMiddleware (F-8) catches it → 500 (two-layer, BR-56).
		//
		// next.ServeHTTP is NEVER called inside adjudicate — the handler is
		// invoked exactly once, after adjudicate returns, OUTSIDE the recover
		// scope. A handler panic therefore propagates to recoveryMiddleware,
		// never to this limiter-domain defer (B-1 fix).
		verdict, failOpen := s.adjudicate(r, route, w)

		if failOpen {
			// §2.1 — malfunction path. The limiter already incremented
			// failures_total inside its own recover (Allow) OR the middleware
			// recover caught a panic in key-extraction/policy/header-setting.
			// Log M4.2 (BR-32), set NO RateLimit-* headers (invariant 4),
			// pass through (D1/US6). next.ServeHTTP is called OUTSIDE the
			// recover scope — a handler panic on the fail-open path still
			// reaches recoveryMiddleware (B-1 fix).
			next.ServeHTTP(w, r)
			return
		}

		dryRun := s.rlCfg.GetDryRun()

		if verdict.Allow {
			// §2.10 — advisory headers on allow (O-9, corrects C-2). Set on
			// w.Header() BEFORE next.ServeHTTP (invariant 1 — stdlib locks
			// the header map at WriteHeader; setting after is a silent no-op).
			// setAdvisoryHeaders ran inside adjudicate's recover scope; the
			// handler call is outside.
			next.ServeHTTP(w, r)
			return
		}

		// Deny path.
		if dryRun {
			// BR-42 — dry-run never rejects. Advisory headers + Retry-After
			// (M5 — even on 200 so clients see what the limiter would say),
			// log M4.3, call next, NO 429, NO rejections_total increment.
			// setAdvisoryHeaders + Retry-After set inside adjudicate; the log
			// + next.ServeHTTP run outside the recover scope.
			next.ServeHTTP(w, r)
			return
		}

		// Clean 429 (BR-19..BR-21). The rejection headers + log +
		// RecordRejection + writeError all ran inside adjudicate's recover
		// scope (a panic in writeError is limiter-domain). NO next.ServeHTTP
		// (invariant 2 — the handler never sees the rejected request). The
		// 429 response is already written; we return without calling next.
	})
}

// adjudicate runs the limiter-domain work and returns the verdict plus a
// failOpen flag. It is the §2.2 inner-layer recover scope: a panic in key
// extraction, policy resolve, Allow, or header-setting is caught here and
// routed to the fail-open path. next.ServeHTTP is NEVER called inside this
// function — the caller invokes the handler exactly once, outside this
// recover scope, so handler panics reach the outer recoveryMiddleware (B-1).
//
// On the allow path, advisory headers are set on w here (inside the recover
// scope, so a header-setting panic is caught). On the deny+enforce path, the
// 429 response is fully written here (headers + log + RecordRejection +
// writeError). On the deny+dry_run path, advisory headers + Retry-After are
// set here and the M4.3 log is emitted here. The caller then calls
// next.ServeHTTP once, outside the recover scope.
//
// Returns (zero Verdict, true) on fail-open; (verdict, false) otherwise.
func (s *Server) adjudicate(r *http.Request, route string, w http.ResponseWriter) (verdict ratelimit.Verdict, failOpen bool) {
	// §2.2 inner-layer recovery — limiter-domain ONLY. A panic in key
	// extraction, policy resolve, Allow, or header-setting is caught here.
	// If THIS recovery path panics, the outer recoveryMiddleware (F-8) catches
	// it → 500 (two-layer, BR-56). next.ServeHTTP is never called here.
	defer func() {
		if rec := recover(); rec != nil {
			s.rateLimiter.RecordFailure()
			log.Printf("rate_limit: internal_error err=%q key=<unknown> route=%s decision=allow fail_mode=%s",
				fmt.Sprintf("%v", rec), route, s.rlCfg.FailMode)
			// Fail-open skips ALL RateLimit-* headers (invariant 4). The
			// caller calls next.ServeHTTP once, outside this recover scope.
			failOpen = true
		}
	}()

	// BR-10/BR-12 — composite key + bare IP. Never panics (BR-13/SEC-02);
	// if it ever does, the recover above catches it (fail-safe).
	compositeKey, _ := s.rlExtractor.Extract(r)

	// BR-17 — resolve matched policy (override or default).
	policy, ok := s.resolvePolicy(route)
	if !ok {
		// An exempt override short-circuited above; a non-exempt override
		// with no limit/window falls back to defaults inside resolvePolicy.
	}

	// §2.1/§2.14 — Allow. On malfunction returns fail-open verdict + err.
	verdict = s.rateLimiter.Allow(compositeKey, policy)

	// §2.1 — malfunction path: the limiter already incremented
	// failures_total inside its own recover. Log M4.2 (BR-32), set NO
	// RateLimit-* headers (invariant 4), pass through (D1/US6).
	if verdict.Err != nil {
		log.Printf("rate_limit: internal_error err=%q key=%s route=%s decision=allow fail_mode=%s",
			verdict.Err.Error(), compositeKey, route, s.rlCfg.FailMode)
		// NO RateLimit-* headers on malfunction (BR-23/BR-54). The caller
		// calls next.ServeHTTP once, outside this recover scope.
		failOpen = true
		return
	}

	dryRun := s.rlCfg.GetDryRun()

	if verdict.Allow {
		// §2.10 — advisory headers on allow (O-9, corrects C-2). Set on
		// w.Header() BEFORE the caller calls next.ServeHTTP (invariant 1).
		s.setAdvisoryHeaders(w, verdict)
		return
	}

	// Deny path.
	if dryRun {
		// BR-42 — dry-run never rejects. Advisory headers + Retry-After
		// (M5 — even on 200 so clients see what the limiter would say),
		// log M4.3, NO 429, NO rejections_total increment. The caller calls
		// next.ServeHTTP once, outside this recover scope.
		s.setAdvisoryHeaders(w, verdict)
		retryAfter := retryAfterSeconds(verdict)
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		log.Printf("rate_limit: dry_run would_reject key=%s route=%s count=%d limit=%d window=%ds retry_after=%ds",
			compositeKey, route, verdict.Count, verdict.Limit, int(verdict.Window.Seconds()), retryAfter)
		return
	}

	// Clean 429 (BR-19..BR-21). Set 429 headers BEFORE writeError calls
	// WriteHeader (invariant 1, F-5: writeJSON calls WriteHeader then
	// encodes). NO next.ServeHTTP (invariant 2 — the handler never sees
	// the rejected request). The 429 is fully written here; the caller
	// returns without calling next.
	s.setRejectionHeaders(w, verdict)
	retryAfter := retryAfterSeconds(verdict)
	// BR-21 — 429 body is ErrorResponse{Error, Details} via the EXISTING
	// writeError (F-5, F-6). Do NOT edit dto.go. details MUST be non-empty
	// (F-6: Details has omitempty; an empty details would omit the field).
	details := fmt.Sprintf("Rate limit exceeded for %s; retry after %ds.", route, retryAfter)
	// Log M4.1 (BR-31) BEFORE writeError so the log is emitted even if
	// writeError somehow panicked (it won't, but the ordering matches the
	// rejection-counter contract).
	log.Printf("rate_limit: rejected key=%s route=%s count=%d limit=%d window=%ds retry_after=%ds",
		compositeKey, route, verdict.Count, verdict.Limit, int(verdict.Window.Seconds()), retryAfter)
	s.rateLimiter.RecordRejection()
	writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded", details)
	return
}

// isExempt returns true if the route has an exempt:true override (BR-09).
// Exemption is structural: the middleware short-circuits before Allow, so an
// exempt route never increments counters, never sets headers, never logs.
func (s *Server) isExempt(route string) bool {
	if s.rlCfg == nil {
		return false
	}
	ov, ok := s.rlCfg.EndpointOverrides[route]
	return ok && ov.Exempt
}

// resolvePolicy returns the Policy for a route (BR-17). Override lookup is
// exact "METHOD /path" match; if an override exists with a limit/window, use
// its values (falling back to defaults for any field it leaves nil). If no
// override, use defaults. The second return is false only when the override
// marked the route exempt — but exempt routes short-circuit earlier in the
// middleware, so by the time resolvePolicy is called the route is not exempt.
func (s *Server) resolvePolicy(route string) (ratelimit.Policy, bool) {
	limit := s.rlCfg.GetDefaultLimit()
	windowSeconds := s.rlCfg.GetDefaultWindowSeconds()
	if ov, ok := s.rlCfg.EndpointOverrides[route]; ok {
		if ov.Limit != nil && *ov.Limit > 0 {
			limit = *ov.Limit
		}
		if ov.WindowSeconds != nil && *ov.WindowSeconds > 0 {
			windowSeconds = *ov.WindowSeconds
		}
	}
	return ratelimit.Policy{
		Limit:  limit,
		Window: time.Duration(windowSeconds) * time.Second,
	}, true
}

// setAdvisoryHeaders sets the four allow-path advisory headers (BR-22, §2.10,
// O-9 — corrects C-2: advisory headers are ALWAYS present on allow-path
// except exempt/malfunction). NO Retry-After on allow-path (nothing to retry).
func (s *Server) setAdvisoryHeaders(w http.ResponseWriter, v ratelimit.Verdict) {
	w.Header().Set("RateLimit-Limit", strconv.Itoa(v.Limit))
	remaining := v.Limit - v.Count
	if remaining < 0 {
		remaining = 0
	}
	w.Header().Set("RateLimit-Remaining", strconv.Itoa(remaining))
	w.Header().Set("RateLimit-Reset", strconv.Itoa(int(v.ResetIn.Round(time.Second)/time.Second)))
	w.Header().Set("X-RateLimit-Policy", fmt.Sprintf("%d;w=%d", v.Limit, int(v.Window.Seconds())))
}

// setRejectionHeaders sets the 429 header set (BR-20). Retry-After is an
// integer delta-seconds ≥1 (BR-24 — delta-seconds, NOT HTTP-date). All headers
// are set on w.Header() BEFORE writeError calls WriteHeader (invariant 1).
func (s *Server) setRejectionHeaders(w http.ResponseWriter, v ratelimit.Verdict) {
	retryAfter := retryAfterSeconds(v)
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	w.Header().Set("RateLimit-Limit", strconv.Itoa(v.Limit))
	w.Header().Set("RateLimit-Remaining", "0") // BR-20 — always 0 on rejection
	w.Header().Set("RateLimit-Reset", strconv.Itoa(retryAfter))
	w.Header().Set("X-RateLimit-Policy", fmt.Sprintf("%d;w=%d", v.Limit, int(v.Window.Seconds())))
}

// retryAfterSeconds returns the integer delta-seconds until the sliding
// window would admit this key again (BR-20, BR-24). Clamped to ≥1 (a 0
// Retry-After would tell the client "retry immediately" which is wrong on a
// 429). When the window is about to roll over, ResetIn already reflects the
// remaining time in the current window; the next request after that boundary
// has a fresh (decayed) count.
func retryAfterSeconds(v ratelimit.Verdict) int {
	s := int(v.ResetIn.Round(time.Second) / time.Second)
	if s < 1 {
		s = 1
	}
	return s
}

// ConfigureRateLimiting (U-W) arms the limiter after NewServer (BLM §4, §2.8).
// NewServer's signature is UNCHANGED (BR-57, F-2 — regression guard
// TestNewServerSignatureUnchanged). The ~10 existing call sites stay valid.
//
// Behavior:
//   - nil cfg OR cfg.Enabled == false → no-op (limiter stays nil → §2.4
//     passthrough; the status route is NOT registered → 404, BR-47/BR-33).
//   - invalid cfg (BR-01..BR-07) → log + leave limiter nil (BR-08, ADR-008,
//     §2.5 — fail-open startup; the server does NOT crash, O-5). The fatal
//     validateConfig path (F-10) is NOT touched.
//   - ratelimit.New failure → log + leave limiter nil (limiter-build bug
//     degrades to passthrough, consistent with D1).
//   - valid cfg → arm: set s.rateLimiter, s.rlCfg, s.rlExtractor, and
//     register GET /health/rate-limit on s.mux (§2.9 — the ONLY route-
//     registration site, gated by the armed condition).
//
// Returns void — main.go cannot mis-handle a returned error into a crash
// (REL-14). configPath is recorded on cfg so the status endpoint can echo
// config_source (BR-46).
func (s *Server) ConfigureRateLimiting(cfg *config.RateLimitConfig, configPath string) {
	if cfg == nil || !cfg.Enabled {
		return // passthrough, D7 (BR-33)
	}
	if err := cfg.Validate(); err != nil {
		// BR-08 / §2.5 — fail-open startup. Do NOT route through the fatal
		// validateConfig path (F-10). Log + leave limiter nil.
		log.Printf("rate_limit: config invalid: %v", err)
		return
	}
	cfg.SetConfigPath(configPath)
	limiter, err := ratelimit.New(
		ratelimit.WithMaxTrackedKeys(cfg.GetMaxTrackedKeys()),
	)
	if err != nil {
		log.Printf("rate_limit: limiter build failed, running without limiter: %v", err)
		return
	}
	s.rateLimiter = limiter
	s.rlCfg = cfg
	s.rlExtractor = ratelimit.KeyExtractor{TrustProxyHeaders: cfg.GetTrustProxyHeaders()}
	// §2.9 — register the status route ONLY when armed. When disabled, the
	// route is absent → 404 (BR-47, byte-identical to pre-feature, SEC-10).
	if s.mux != nil {
		s.mux.HandleFunc(rateLimitStatusRoute, s.handleRateLimitStatus)
	}
}

// rateLimitStatusResponse is the /health/rate-limit JSON body (BLM §3.2,
// BR-27 — stable key order). active_keys is an ARRAY (not map) sorted by
// count DESC; the json tag is "active_keys" (NOT omitempty) and the slice is
// initialized to empty so the empty case marshals to `[]` not `null` (BR-28).
// active_keys_truncated is a BOOL (O-2, corrects C-5 — NOT a synthetic entry).
//
// Field order is LOCKED (BR-27): status, enabled, limiter, config_source,
// trust_proxy_headers, dry_run, fail_mode, defaults, endpoint_overrides,
// active_keys, active_keys_truncated, rejections_total, failures_total,
// generated_at. Go marshals struct fields in declaration order, so the
// struct below defines the wire order.
type rateLimitStatusResponse struct {
	Status              string                      `json:"status"`
	Enabled             bool                        `json:"enabled"`
	Limiter             string                      `json:"limiter"`
	ConfigSource        string                      `json:"config_source"`
	TrustProxyHeaders   bool                        `json:"trust_proxy_headers"`
	DryRun              bool                        `json:"dry_run"`
	FailMode            string                      `json:"fail_mode"`
	Defaults            rateLimitDefaultsResponse   `json:"defaults"`
	EndpointOverrides   []rateLimitOverrideResponse `json:"endpoint_overrides"`
	ActiveKeys          []ratelimit.KeyState        `json:"active_keys"`
	ActiveKeysTruncated bool                        `json:"active_keys_truncated"`
	RejectionsTotal     int64                       `json:"rejections_total"`
	FailuresTotal       int64                       `json:"failures_total"`
	GeneratedAt         string                      `json:"generated_at"`
}

type rateLimitDefaultsResponse struct {
	Limit         int `json:"limit"`
	WindowSeconds int `json:"window_seconds"`
}

type rateLimitOverrideResponse struct {
	Route         string `json:"route"`
	Limit         *int   `json:"limit,omitempty"`
	WindowSeconds *int   `json:"window_seconds,omitempty"`
	Exempt        bool   `json:"exempt,omitempty"`
}

// handleRateLimitStatus (U-F) emits the /health/rate-limit JSON (BLM §3.2).
// It is exempt by route (§2.13) — NEVER calls Allow, NEVER increments
// counters. Always 200, even when failures_total > 0 (the endpoint reporting
// its own fail-open state is itself healthy). One schema for all states
// (§2.12 — no special "broken" variant).
func (s *Server) handleRateLimitStatus(w http.ResponseWriter, r *http.Request) {
	if s.rateLimiter == nil || s.rlCfg == nil {
		// Defensive: the route is only registered when armed (§2.9), so this
		// branch is unreachable in production. If reached, respond as a
		// disabled limiter would (BR-33) rather than panicking.
		writeJSON(w, http.StatusOK, rateLimitStatusResponse{
			Status:              "healthy",
			Enabled:             false,
			Limiter:             "sliding_window",
			ConfigSource:        "",
			TrustProxyHeaders:   false,
			DryRun:              false,
			FailMode:            "fail_open",
			Defaults:            rateLimitDefaultsResponse{Limit: 100, WindowSeconds: 60},
			EndpointOverrides:   []rateLimitOverrideResponse{},
			ActiveKeys:          []ratelimit.KeyState{},
			ActiveKeysTruncated: false,
			RejectionsTotal:     0,
			FailuresTotal:       0,
			GeneratedAt:         time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	// Bounded snapshot (§2.12). maxKeys = the configured cap; the snapshot
	// sorts by count DESC and caps, setting active_keys_truncated on overflow.
	snap := s.rateLimiter.Snapshot(s.rlCfg.GetMaxTrackedKeys())

	// BR-28 — active_keys is [] not null. The snapshot already returns an
	// empty (not nil) slice, but we defend here too.
	if snap.Keys == nil {
		snap.Keys = []ratelimit.KeyState{}
	}

	overrides := buildOverrideResponse(s.rlCfg)

	resp := rateLimitStatusResponse{
		Status:            "healthy",
		Enabled:           s.rlCfg.Enabled,
		Limiter:           "sliding_window",
		ConfigSource:      s.rlCfg.ConfigPath(),
		TrustProxyHeaders: s.rlCfg.GetTrustProxyHeaders(),
		DryRun:            s.rlCfg.GetDryRun(),
		FailMode:          s.rlCfg.FailMode,
		Defaults: rateLimitDefaultsResponse{
			Limit:         s.rlCfg.GetDefaultLimit(),
			WindowSeconds: s.rlCfg.GetDefaultWindowSeconds(),
		},
		EndpointOverrides:   overrides,
		ActiveKeys:          snap.Keys,
		ActiveKeysTruncated: snap.Truncated,
		RejectionsTotal:     snap.RejectionsTotal,
		FailuresTotal:       snap.FailuresTotal,
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339),
	}
	if resp.FailMode == "" {
		// v2 only value; present for forward-compat. Default to fail_open if
		// the operator left it blank (Validate() accepts blank, treating it
		// as "use the v2 default").
		resp.FailMode = "fail_open"
	}

	// Always 200 (BR-26). Use writeJSON (F-5) for Content-Type. NO RateLimit-*
	// headers (the endpoint is exempt — invariant 3).
	writeJSON(w, http.StatusOK, resp)
}

// buildOverrideResponse returns the endpoint_overrides array in a stable,
// scannable order (BR-27 — ARRAY, not map). Routes are sorted alphabetically
// so the response is deterministic across runs (a map's iteration order is
// random in Go; sorting makes the response diff-able).
func buildOverrideResponse(cfg *config.RateLimitConfig) []rateLimitOverrideResponse {
	if cfg == nil || len(cfg.EndpointOverrides) == 0 {
		return []rateLimitOverrideResponse{}
	}
	out := make([]rateLimitOverrideResponse, 0, len(cfg.EndpointOverrides))
	for route, ov := range cfg.EndpointOverrides {
		row := rateLimitOverrideResponse{Route: route}
		if ov.Limit != nil {
			l := *ov.Limit
			row.Limit = &l
		}
		if ov.WindowSeconds != nil {
			ws := *ov.WindowSeconds
			row.WindowSeconds = &ws
		}
		row.Exempt = ov.Exempt
		out = append(out, row)
	}
	// Sort by route for deterministic output.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].Route > out[j].Route; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// rateLimitStatusResponseJSON is a helper for tests that want to decode the
// status response without importing the type; it mirrors the wire shape.
var rateLimitStatusResponseJSON rateLimitStatusResponse

// encodeRateLimitStatusForTest is a test-only helper exposed so tests can
// round-trip the response shape. It is not used in production.
func encodeRateLimitStatusForTest(resp rateLimitStatusResponse) ([]byte, error) {
	return json.Marshal(resp)
}
