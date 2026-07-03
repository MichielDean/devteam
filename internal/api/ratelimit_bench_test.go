package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/ratelimit"
)

// These benchmarks are the Performance Validation (4.6) load-test surface for
// rate-limiting-v2. They realize the benchmarks named in performance-nfrs §1
// that were not authored during construction (no BenchmarkRateLimitAllow exists
// in the v2 internal/ratelimit package — construction did not add it). They
// measure the real allow-path cost (middleware wrap via httptest, E14 — no DB),
// the disabled passthrough, the eviction sweep, the snapshot build, and the
// configure (arm) cost, so the NFR validation matrix has measured numbers for
// PERF-01/02/04/05/07/08/09/10/11/12.
//
// v2 inherits the v1 algorithm verbatim (the 3.1 functional design LOCKED it,
// business-logic-model §0 authority rule); the v1 measured baselines
// (206.7ns hot path, 0 allocs/op, 72.99ns disabled, 761ns eviction, 2.74ms
// snapshot) are the EXPECTED v2 results. The v2 bench file re-authors the
// surface for the v2 codebase conventions (ptrIntAPI, ConfigureRateLimiting
// 2-arg signature, s.rlExtractor field) — the v1 bench file used ptrInt and
// the 1-arg ConfigureRateLimiting, neither of which exist in v2.
//
// All run via `go test -bench -benchmem ./internal/api/`. The p99 latency is
// derived from ns/op (single-sample is enough for an O(1) hot path; the
// benchmark is the load test for a single-process in-memory limiter — there is
// no production-like environment to load beyond the process itself, per
// NG-PERF-3 / C-01).

// newTestServerB is the bench variant of newRateLimitTestServer (takes *testing.B,
// no DB, no httptest.Server — the bench wraps its own handler).
func newTestServerB(b *testing.B) *Server {
	b.Helper()
	return &Server{mux: http.NewServeMux()}
}

// BenchmarkRateLimitMiddlewareAllow measures the full allow path: middleware
// wrap → key extract → Allow → advisory header set → next. This is the real
// hot path PERF-01/PERF-02/PERF-10 target. Pre-populates 256 keys (warmup) so
// every measured iteration is a map HIT (steady state) on the allow path with
// a limit high enough that exhaustion is unreachable. Pre-builds the requests
// so -benchmem reflects only the middleware hot path, not the harness.
func BenchmarkRateLimitMiddlewareAllow(b *testing.B) {
	s := newTestServerB(b)
	armB(b, s, 1<<30, 60)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := s.rateLimitMiddleware(next)

	// Pre-build 256 requests with distinct IPs and warm the limiter map.
	const nkeys = 256
	reqs := make([]*http.Request, nkeys)
	for i := 0; i < nkeys; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/ping", nil)
		req.RemoteAddr = "10.0.0." + strconv.Itoa(i+1) + ":1234"
		reqs[i] = req
		// warmup: put each key in the map (allow path, under limit).
		h.ServeHTTP(httptest.NewRecorder(), req)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Fresh recorder per call (harness alloc — see PERF-02 note: the
			// isolated BenchmarkRateLimitAllow measures 0 allocs/op for the
			// limiter itself; this bench measures the full middleware + recorder
			// path so harness allocs are expected).
			h.ServeHTTP(httptest.NewRecorder(), reqs[i])
			i++
			if i == nkeys {
				i = 0
			}
		}
	})
}

// BenchmarkRateLimitMiddlewareDisabled measures the nil-limiter passthrough
// (PERF-08/PERF-12 — zero-overhead vs pre-feature chain, single nil check).
func BenchmarkRateLimitMiddlewareDisabled(b *testing.B) {
	s := newTestServerB(b) // s.rateLimiter is nil
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := s.rateLimitMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/v1/ping", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
		}
	})
}

// BenchmarkRateLimitEviction measures the sweep-bearing Allow cost (PERF-07).
// Populates to the cap then triggers the sweep path; p99 of the sweep-bearing
// call must be within 10× the p99 allow latency.
//
// v2 NOTE: v2's Allow takes (key, policy); New takes ...Option (no Policy arg).
func BenchmarkRateLimitEviction(b *testing.B) {
	l, _ := ratelimit.New(ratelimit.WithMaxTrackedKeys(200))
	p := ratelimit.Policy{Limit: 1 << 20, Window: 60 * time.Second}
	// populate to cap with distinct keys so subsequent Allows trigger cap
	// enforcement (oldest evicted to 90%).
	for i := 0; i < 200; i++ {
		_ = l.Allow(ratelimitKeyB(i), p)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l.Allow(ratelimitKeyB(2000+i), p) // new keys, sweep fires on 64th
	}
}

// BenchmarkRateLimitSnapshot measures the status-endpoint snapshot build
// (PERF-09 — Should target < 1ms at default cap). Populates 10000 keys then
// snapshots. Not on the request hot path (status endpoint is exempt).
func BenchmarkRateLimitSnapshot(b *testing.B) {
	l, _ := ratelimit.New()
	p := ratelimit.Policy{Limit: 100, Window: 60 * time.Second}
	for i := 0; i < 10000; i++ {
		_ = l.Allow(ratelimitKeyB(i), p)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l.Snapshot(10000)
	}
}

// BenchmarkConfigureRateLimiting measures the arm path: ConfigureRateLimiting
// (validate + ratelimit.New + status route registration) on a fresh Server.
// NOTE: the per-iteration http.NewServeMux() inflates this number (see
// load-test-results §3.6 / FO-1) — the true validation+New cost is sub-µs; this
// bench reports the mux-construction cost too. Re-authoring to arm on a
// pre-built Server is a next-cycle bench-hygiene item (FO-1).
//
// v2 NOTE: ConfigureRateLimiting takes (cfg, configPath) — the v2 wiring
// contract (BR-59). The v1 bench used the 1-arg variant; this v2 bench uses the
// 2-arg signature, measuring PERF-11 (the setter-based arming cost).
func BenchmarkConfigureRateLimiting(b *testing.B) {
	cfg := &config.RateLimitConfig{
		Enabled:  true,
		FailMode: "fail_open",
		Defaults: config.RateLimitDefaults{
			Limit:         ptrIntAPI(100),
			WindowSeconds: ptrIntAPI(60),
		},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := &Server{mux: http.NewServeMux()}
		s.ConfigureRateLimiting(cfg, "devteam.yaml")
	}
}

func ratelimitKeyB(i int) string {
	return "10.0.0." + strconv.Itoa(i) + "|GET /v1/ping"
}

// armB is the bench variant of arm (uses b, not t). Reuses ptrIntAPI from
// ratelimit_middleware_test.go (same package — no duplicate helper).
func armB(b *testing.B, s *Server, limit, windowSecs int) *ratelimit.Limiter {
	b.Helper()
	limiter, err := ratelimit.New()
	if err != nil {
		b.Fatalf("arm: New: %v", err)
	}
	s.rateLimiter = limiter
	s.rlCfg = &config.RateLimitConfig{
		Enabled:  true,
		FailMode: "fail_open",
		Defaults: config.RateLimitDefaults{Limit: ptrIntAPI(limit), WindowSeconds: ptrIntAPI(windowSecs)},
	}
	s.rlExtractor = ratelimit.KeyExtractor{}
	return limiter
}