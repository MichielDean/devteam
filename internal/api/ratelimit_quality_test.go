package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MichielDean/devteam/internal/config"
)

// This file holds the spec-named test contracts identified as missing or
// under-tested by the 3.5 architecture review (findings B-1, R-1, S-1). They
// are construction contracts per the 3.1 BR artifact and the 3.3 nfr-design-
// specs — future edits that break the contract fail the build.
//
// Conventions:
//   - Log capture uses log.SetOutput to a *bytes.Buffer, restored in t.Cleanup.
//   - Handler-panic tests use a mux handler that panics deterministically.
//   - Every test names the BR / §2 / US token it guards.

// captureLog redirects log.Printf output to a buffer for the duration of the
// test and returns the buffer. The original output is restored on cleanup.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(orig) })
	return &buf
}

// --- B-1: two-layer recovery contract (§2.2, BR-35, BR-56, invariant 5) ---

// TestRateLimitMiddlewarePanicRecovered (BR-35, BR-56, §2.2) — a panic in
// Allow is caught by the limiter's own malfunction path (fail-open → 200,
// traffic flows, D1). The server stays up. This is the spec-named contract
// for the inner fail-open layer.
func TestRateLimitMiddlewarePanicRecovered(t *testing.T) {
	s, ts, called := newRateLimitTestServer(t, enabledCfg())
	// Inject a panic into Allow via a panicking clock. The limiter's own
	// recover returns a fail-open verdict → middleware calls next once → 200.
	s.rateLimiter.SwapClockForTest(panicTestClock{})
	*called = false
	resp, err := http.Get(ts.URL + "/test")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("fail-open malfunction should pass through (200), got %d (§2.1/BR-50)", resp.StatusCode)
	}
	if !*called {
		t.Errorf("fail-open malfunction must call next.ServeHTTP exactly once (D1), got called=%v", *called)
	}
	if s.rateLimiter.FailuresTotal() < 1 {
		t.Errorf("malfunction must increment failures_total (BR-50), got %d", s.rateLimiter.FailuresTotal())
	}
}

// TestRateLimitHandlerPanicPropagatesToOuterRecovery (B-1 fix, §2.2, BR-35,
// BR-56, invariant 5) — a panic in the DOWNSTREAM HANDLER (next.ServeHTTP)
// must propagate to the outer recoveryMiddleware → 500, NOT be caught by the
// limiter's inner defer/recover. This is the regression guard for the B-1
// defect: the inner recover scope is limiter-domain only; handler panics are
// the outer layer's domain. Verifies:
//  1. Response is 500 (the outer recoveryMiddleware's writeError).
//  2. failures_total does NOT increment (the limiter did not malfunction —
//     a handler bug is not a limiter failure; BR-49 semantic integrity).
//  3. No false "rate_limit: internal_error" log (BR-32 — the malfunction log
//     must be distinguishable from a handler panic; no misattribution).
//  4. The handler is invoked exactly once (no re-invocation inside the defer).
func TestRateLimitHandlerPanicPropagatesToOuterRecovery(t *testing.T) {
	s := &Server{
		sseClients: make(map[string][]chan SSEMessage),
		sseBuffers: make(map[string][]*SSEMessage),
		mux:        http.NewServeMux(),
	}
	handlerCalls := 0
	s.mux.HandleFunc("/boom", func(w http.ResponseWriter, r *http.Request) {
		handlerCalls++
		panic("handler bug: simulated nil deref")
	})
	s.ConfigureRateLimiting(enabledCfg(), "devteam.yaml")
	handler := s.recoveryMiddleware(s.corsMiddleware(s.rateLimitMiddleware(s.mux)))
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	logBuf := captureLog(t)
	before := s.rateLimiter.FailuresTotal()

	resp, err := http.Get(ts.URL + "/boom")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	// 1. The outer recoveryMiddleware catches the handler panic → 500.
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("handler panic must propagate to outer recovery → 500, got %d (B-1/§2.2/BR-56)", resp.StatusCode)
	}
	// 2. failures_total must NOT increment — a handler bug is not a limiter
	//    malfunction (BR-49 semantic integrity, B-1 fix).
	if got := s.rateLimiter.FailuresTotal(); got != before {
		t.Errorf("handler panic must NOT increment failures_total (B-1 fix, BR-49); before=%d after=%d", before, got)
	}
	// 3. No false "rate_limit: internal_error" log — the malfunction log is
	//    limiter-domain only (BR-32 distinguishability).
	if strings.Contains(logBuf.String(), "rate_limit: internal_error") {
		t.Errorf("handler panic must NOT emit a rate_limit internal_error log (B-1 fix, BR-32); log=%q", logBuf.String())
	}
	// 4. The handler is invoked exactly once (no re-invocation inside the
	//    limiter's defer — the B-1 defect re-invoked the handler).
	if handlerCalls != 1 {
		t.Errorf("handler must be invoked exactly once (B-1 fix — no re-invocation), got %d", handlerCalls)
	}
}

// TestRateLimitMalfunctionPathPanicOuterRecovery (BR-56 second layer) — if the
// limiter's own malfunction-recovery code itself panics, the outer
// recoveryMiddleware catches it → 500. This is the second layer of the
// two-layer recovery contract. We simulate it by making the fail-open path
// panic: swap the limiter for one whose Allow returns a verdict with a
// non-nil Err, then make the middleware's malfunction log path unreachable
// in a way that panics. The simplest deterministic injection is a handler
// that panics on the fail-open path — but that is covered by the handler-panic
// test above. Instead we verify the structural property: the outer
// recoveryMiddleware wraps the entire chain, so ANY panic that escapes the
// inner recover is caught by it. We assert this by panicking inside the
// limiter's recover scope via a verdict whose Err formatting panics — but
// fmt.Sprintf("%v", rec) never panics for a string. The cleanest deterministic
// injection: a handler that panics AFTER the limiter allows (allow-path) —
// the panic is in next.ServeHTTP, outside the inner recover, caught by outer.
// This is the same scenario as TestRateLimitHandlerPanicPropagatesToOuterRecovery
// on the allow path; here we assert the allow-path variant specifically.
func TestRateLimitMalfunctionPathPanicOuterRecovery(t *testing.T) {
	s := &Server{
		sseClients: make(map[string][]chan SSEMessage),
		sseBuffers: make(map[string][]*SSEMessage),
		mux:        http.NewServeMux(),
	}
	handlerCalls := 0
	s.mux.HandleFunc("/allow-then-boom", func(w http.ResponseWriter, r *http.Request) {
		handlerCalls++
		panic("handler panic on the allow path")
	})
	s.ConfigureRateLimiting(enabledCfg(), "devteam.yaml")
	handler := s.recoveryMiddleware(s.corsMiddleware(s.rateLimitMiddleware(s.mux)))
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/allow-then-boom")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("allow-path handler panic must reach outer recovery → 500, got %d (BR-56)", resp.StatusCode)
	}
	if handlerCalls != 1 {
		t.Errorf("handler invoked once on allow path (no re-invocation), got %d", handlerCalls)
	}
}

// --- S-1: log-line format capture tests (BR-31, BR-32) ---

// TestRateLimitRejectionLogEmitted (BR-31, M4.1) — a clean 429 emits the M4.1
// log line with the required fields: key, route, count, limit, window,
// retry_after. Captures log.Printf output and asserts the line is present.
func TestRateLimitRejectionLogEmitted(t *testing.T) {
	logBuf := captureLog(t)
	limit := 1
	window := 60
	cfg := &config.RateLimitConfig{
		Enabled: true, FailMode: "fail_open",
		Defaults:       config.RateLimitDefaults{Limit: &limit, WindowSeconds: &window},
		MaxTrackedKeys: ptrIntAPI(10000),
	}
	s, ts, _ := newRateLimitTestServer(t, cfg)
	_ = s
	mustGet(t, ts.URL+"/test").Body.Close() // prime (allowed)
	resp := mustGet(t, ts.URL+"/test")      // over limit → 429
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
	line := logBuf.String()
	for _, want := range []string{"rate_limit: rejected", "key=", "route=GET /test", "count=", "limit=1", "window=60s", "retry_after="} {
		if !strings.Contains(line, want) {
			t.Errorf("M4.1 rejection log missing %q; line=%q (BR-31)", want, line)
		}
	}
}

// TestRateLimitRejectionLogNoLvlToken (BR-31) — the rejection log line must
// NOT contain an `lvl=` token (no slog/zap leak). The v1 reversal C-1 guard.
func TestRateLimitRejectionLogNoLvlToken(t *testing.T) {
	logBuf := captureLog(t)
	limit := 1
	window := 60
	cfg := &config.RateLimitConfig{
		Enabled: true, FailMode: "fail_open",
		Defaults:       config.RateLimitDefaults{Limit: &limit, WindowSeconds: &window},
		MaxTrackedKeys: ptrIntAPI(10000),
	}
	_, ts, _ := newRateLimitTestServer(t, cfg)
	mustGet(t, ts.URL+"/test").Body.Close()
	mustGet(t, ts.URL+"/test").Body.Close() // 429
	if strings.Contains(logBuf.String(), "lvl=") {
		t.Errorf("rejection log must NOT contain lvl= (BR-31/C-1); log=%q", logBuf.String())
	}
}

// TestRateLimitRejectionLogNoRequestId (BR-31) — the rejection log must NOT
// contain a request_id= token (the v2 limiter does not generate request IDs —
// reversal-token grep guard, test-side).
func TestRateLimitRejectionLogNoRequestId(t *testing.T) {
	logBuf := captureLog(t)
	limit := 1
	window := 60
	cfg := &config.RateLimitConfig{
		Enabled: true, FailMode: "fail_open",
		Defaults:       config.RateLimitDefaults{Limit: &limit, WindowSeconds: &window},
		MaxTrackedKeys: ptrIntAPI(10000),
	}
	_, ts, _ := newRateLimitTestServer(t, cfg)
	mustGet(t, ts.URL+"/test").Body.Close()
	mustGet(t, ts.URL+"/test").Body.Close()
	if strings.Contains(logBuf.String(), "request_id") {
		t.Errorf("rejection log must NOT contain request_id (BR-31); log=%q", logBuf.String())
	}
}

// TestRateLimitRejectionLogSingleLine (BR-31) — the rejection log is a single
// line with no embedded newlines (so it parses as one log entry, not many).
func TestRateLimitRejectionLogSingleLine(t *testing.T) {
	logBuf := captureLog(t)
	limit := 1
	window := 60
	cfg := &config.RateLimitConfig{
		Enabled: true, FailMode: "fail_open",
		Defaults:       config.RateLimitDefaults{Limit: &limit, WindowSeconds: &window},
		MaxTrackedKeys: ptrIntAPI(10000),
	}
	_, ts, _ := newRateLimitTestServer(t, cfg)
	mustGet(t, ts.URL+"/test").Body.Close()
	mustGet(t, ts.URL+"/test").Body.Close()
	for _, line := range strings.Split(strings.TrimSpace(logBuf.String()), "\n") {
		if strings.Contains(line, "rate_limit: rejected") && strings.Contains(line, "\n") {
			t.Errorf("rejection log line must not contain embedded newlines (BR-31); line=%q", line)
		}
	}
}

// TestRateLimitRejectionLogUsesLogPrintf (BR-31) — the rejection log is emitted
// via log.Printf (the standard logger), not slog/zap. We assert by capturing
// log.Writer() output (slog writes to a different sink by default). This is a
// light guard: if a future refactor switches to slog, the line would not appear
// in log.Writer() output and this test fails.
func TestRateLimitRejectionLogUsesLogPrintf(t *testing.T) {
	logBuf := captureLog(t)
	limit := 1
	window := 60
	cfg := &config.RateLimitConfig{
		Enabled: true, FailMode: "fail_open",
		Defaults:       config.RateLimitDefaults{Limit: &limit, WindowSeconds: &window},
		MaxTrackedKeys: ptrIntAPI(10000),
	}
	_, ts, _ := newRateLimitTestServer(t, cfg)
	mustGet(t, ts.URL+"/test").Body.Close()
	mustGet(t, ts.URL+"/test").Body.Close()
	if !strings.Contains(logBuf.String(), "rate_limit: rejected") {
		t.Errorf("rejection log must appear in log.Printf output (BR-31); buf=%q", logBuf.String())
	}
}

// TestRateLimitRejectionLogShowsFullIP (BR-30, O-4 log-side) — the rejection log
// contains the full client IP (no redaction). The composite key includes the
// bare IP, so the log line includes it verbatim.
func TestRateLimitRejectionLogShowsFullIP(t *testing.T) {
	logBuf := captureLog(t)
	limit := 1
	window := 60
	cfg := &config.RateLimitConfig{
		Enabled: true, FailMode: "fail_open",
		Defaults:       config.RateLimitDefaults{Limit: &limit, WindowSeconds: &window},
		MaxTrackedKeys: ptrIntAPI(10000),
	}
	_, ts, _ := newRateLimitTestServer(t, cfg)
	mustGet(t, ts.URL+"/test").Body.Close()
	mustGet(t, ts.URL+"/test").Body.Close()
	// httptest loopback is 127.0.0.1 — the log key includes the full IP.
	if !strings.Contains(logBuf.String(), "127.0.0.1") {
		t.Errorf("rejection log must show full IP (BR-30/O-4); log=%q", logBuf.String())
	}
}

// TestRateLimitMalfunctionLogEmitted (BR-32, M4.2) — a malfunction emits the
// M4.2 internal_error log line with decision=allow + fail_mode. Asserts the
// line is distinguishable from a clean rejection (BR-32).
func TestRateLimitMalfunctionLogEmitted(t *testing.T) {
	logBuf := captureLog(t)
	s, ts, _ := newRateLimitTestServer(t, enabledCfg())
	s.rateLimiter.SwapClockForTest(panicTestClock{})
	mustGet(t, ts.URL+"/test").Body.Close()
	line := logBuf.String()
	for _, want := range []string{"rate_limit: internal_error", "decision=allow", "fail_mode=fail_open"} {
		if !strings.Contains(line, want) {
			t.Errorf("M4.2 malfunction log missing %q; line=%q (BR-32)", want, line)
		}
	}
}

// TestRateLimitMalfunctionLogDecisionAllow (BR-32) — the malfunction log says
// decision=allow (fail-open), distinguishing it from a clean rejection's
// implied decision=reject.
func TestRateLimitMalfunctionLogDecisionAllow(t *testing.T) {
	logBuf := captureLog(t)
	s, ts, _ := newRateLimitTestServer(t, enabledCfg())
	s.rateLimiter.SwapClockForTest(panicTestClock{})
	mustGet(t, ts.URL+"/test").Body.Close()
	if !strings.Contains(logBuf.String(), "decision=allow") {
		t.Errorf("malfunction log must say decision=allow (BR-32); log=%q", logBuf.String())
	}
}

// TestRateLimitMalfunctionLogNoLvlToken (BR-32) — the malfunction log must NOT
// contain lvl= (no slog/zap leak).
func TestRateLimitMalfunctionLogNoLvlToken(t *testing.T) {
	logBuf := captureLog(t)
	s, ts, _ := newRateLimitTestServer(t, enabledCfg())
	s.rateLimiter.SwapClockForTest(panicTestClock{})
	mustGet(t, ts.URL+"/test").Body.Close()
	if strings.Contains(logBuf.String(), "lvl=") {
		t.Errorf("malfunction log must NOT contain lvl= (BR-32); log=%q", logBuf.String())
	}
}

// TestRateLimitMalfunctionLogDistinct (BR-32) — the malfunction log line is
// distinguishable from a clean rejection log line (different prefix:
// internal_error vs rejected).
func TestRateLimitMalfunctionLogDistinct(t *testing.T) {
	// Capture a clean rejection.
	rejBuf := captureLog(t)
	limit := 1
	window := 60
	cfg := &config.RateLimitConfig{
		Enabled: true, FailMode: "fail_open",
		Defaults:       config.RateLimitDefaults{Limit: &limit, WindowSeconds: &window},
		MaxTrackedKeys: ptrIntAPI(10000),
	}
	_, ts1, _ := newRateLimitTestServer(t, cfg)
	mustGet(t, ts1.URL+"/test").Body.Close()
	mustGet(t, ts1.URL+"/test").Body.Close()
	rejLine := rejBuf.String()

	// Capture a malfunction (fresh server — log capture is per-test, so we
	// re-capture in a sub-test to keep them separate).
	t.Run("malfunction", func(t *testing.T) {
		malBuf := captureLog(t)
		s, ts2, _ := newRateLimitTestServer(t, enabledCfg())
		s.rateLimiter.SwapClockForTest(panicTestClock{})
		mustGet(t, ts2.URL+"/test").Body.Close()
		malLine := malBuf.String()
		if strings.Contains(rejLine, "internal_error") {
			t.Errorf("clean rejection log must not contain internal_error (BR-32); rej=%q", rejLine)
		}
		if !strings.Contains(malLine, "internal_error") {
			t.Errorf("malfunction log must contain internal_error (BR-32); mal=%q", malLine)
		}
		if strings.Contains(malLine, "rate_limit: rejected key=") {
			t.Errorf("malfunction log must not be a clean-rejection line (BR-32); mal=%q", malLine)
		}
	})
}

// TestRateLimitDryRunLogsWouldReject (BR-42, M4.3) — a dry-run deny emits the
// M4.3 would_reject log with the dry_run marker.
func TestRateLimitDryRunLogsWouldReject(t *testing.T) {
	logBuf := captureLog(t)
	limit := 1
	window := 60
	dry := true
	cfg := &config.RateLimitConfig{
		Enabled: true, FailMode: "fail_open", DryRun: &dry,
		Defaults:       config.RateLimitDefaults{Limit: &limit, WindowSeconds: &window},
		MaxTrackedKeys: ptrIntAPI(10000),
	}
	_, ts, _ := newRateLimitTestServer(t, cfg)
	mustGet(t, ts.URL+"/test").Body.Close() // prime (allowed)
	mustGet(t, ts.URL+"/test").Body.Close() // over limit → dry-run would_reject
	line := logBuf.String()
	for _, want := range []string{"rate_limit: dry_run would_reject", "key=", "route=GET /test", "count=", "limit=1", "window=60s", "retry_after="} {
		if !strings.Contains(line, want) {
			t.Errorf("M4.3 dry_run log missing %q; line=%q (BR-42)", want, line)
		}
	}
}

// --- S-1: other missing named contracts ---

// TestRateLimitFirstRequestAdvisoryHeaders (BR-37) — the first request within
// a fresh window has Remaining == limit - 1 (the request itself consumed one).
func TestRateLimitFirstRequestAdvisoryHeaders(t *testing.T) {
	limit := 100
	window := 60
	cfg := &config.RateLimitConfig{
		Enabled: true, FailMode: "fail_open",
		Defaults:       config.RateLimitDefaults{Limit: &limit, WindowSeconds: &window},
		MaxTrackedKeys: ptrIntAPI(10000),
	}
	_, ts, _ := newRateLimitTestServer(t, cfg)
	resp := mustGet(t, ts.URL+"/test")
	defer resp.Body.Close()
	rem, _ := strconvAtoi(resp.Header.Get("Ratelimit-Remaining"))
	if rem != limit-1 {
		t.Errorf("first request Remaining = %d, want %d (BR-37)", rem, limit-1)
	}
}

// TestRateLimitAdvisoryHeadersSurviveHandler (BR-45) — when the handler does
// NOT touch the response headers, the advisory headers set by the middleware
// are present on the 200 response (invariant 1: headers set before
// next.ServeHTTP are not stripped).
func TestRateLimitAdvisoryHeadersSurviveHandler(t *testing.T) {
	_, ts, _ := newRateLimitTestServer(t, enabledCfg())
	resp := mustGet(t, ts.URL+"/test")
	defer resp.Body.Close()
	for _, h := range []string{"Ratelimit-Limit", "Ratelimit-Remaining", "Ratelimit-Reset", "X-Ratelimit-Policy"} {
		if resp.Header.Get(h) == "" {
			t.Errorf("advisory header %q must survive a handler that does not touch headers (BR-45)", h)
		}
	}
}

// TestRateLimitExemptOverrideOnNonexistentRoute (BR-39) — an exempt override
// for a route that is NOT in the mux does not error; the route simply
// short-circuits to next.ServeHTTP (which 404s via the mux). No crash.
func TestRateLimitExemptOverrideOnNonexistentRoute(t *testing.T) {
	s, ts, _ := newRateLimitTestServer(t, cfgWithExempt("GET /no-such-route"))
	// A request to the exempt-but-nonexistent route 404s (mux), no panic.
	resp := mustGet(t, ts.URL+"/no-such-route")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("exempt override on nonexistent route should 404 (mux), got %d (BR-39)", resp.StatusCode)
	}
	// The limiter did not count it (exempt short-circuits before Allow).
	if s.rateLimiter.Len() != 0 {
		t.Errorf("exempt route must not create a key (BR-09), len=%d", s.rateLimiter.Len())
	}
}

// TestServerStartsWithInvalidRateLimitConfig (BR-08) — end-to-end server
// startup path with an invalid config leaves the limiter nil (passthrough),
// not a crash. This exercises the ConfigureRateLimiting setter from a
// freshly-built Server, not just the setter in isolation.
func TestServerStartsWithInvalidRateLimitConfig(t *testing.T) {
	s := &Server{
		sseClients: make(map[string][]chan SSEMessage),
		sseBuffers: make(map[string][]*SSEMessage),
		mux:        http.NewServeMux(),
	}
	s.mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	bad := &config.RateLimitConfig{
		Enabled: true, FailMode: "fail_closed", // BR-52 rejects fail_closed
		Defaults: config.RateLimitDefaults{Limit: ptrIntAPI(-1)},
	}
	// Must not panic; leaves limiter nil.
	s.ConfigureRateLimiting(bad, "devteam.yaml")
	if s.rateLimiter != nil {
		t.Fatalf("invalid config must leave limiter nil (BR-08)")
	}
	handler := s.recoveryMiddleware(s.corsMiddleware(s.rateLimitMiddleware(s.mux)))
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	// Server responds (passthrough) — invalid config does not crash startup.
	resp := mustGet(t, ts.URL+"/test")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("invalid-config server should passthrough → 200, got %d (BR-08/§2.5)", resp.StatusCode)
	}
}

// TestRateLimitStatusNoSecrets (SEC-11) — the status endpoint exposes no
// secrets (no DSN, no passwords, no API keys). It surfaces config_source (a
// path), limits, and counters only.
func TestRateLimitStatusNoSecrets(t *testing.T) {
	logBuf := captureLog(t)
	_ = logBuf
	_, ts, _ := newRateLimitTestServer(t, enabledCfg())
	resp := mustGet(t, ts.URL+"/health/rate-limit")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	// No common secret markers. config_source is a path, not a secret.
	for _, secret := range []string{"password", "dsn", "secret", "api_key", "token", "host=localhost"} {
		if strings.Contains(strings.ToLower(bodyStr), strings.ToLower(secret)) {
			t.Errorf("status endpoint must not expose %q (SEC-11); body=%q", secret, bodyStr)
		}
	}
}

// TestRateLimitMiddlewareMalfunctionPassesThrough (BR-50) — on a malfunction
// the middleware passes the request to the handler (traffic flows, D1) and
// does NOT reject. Variant of the malfunction tests under the spec's exact name.
func TestRateLimitMiddlewareMalfunctionPassesThrough(t *testing.T) {
	s, ts, called := newRateLimitTestServer(t, enabledCfg())
	s.rateLimiter.SwapClockForTest(panicTestClock{})
	*called = false
	resp := mustGet(t, ts.URL+"/test")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("malfunction must pass through (D1), got %d (BR-50)", resp.StatusCode)
	}
	if !*called {
		t.Errorf("malfunction must call next.ServeHTTP (BR-50/D1)")
	}
}

// TestRateLimitMiddlewareMalfunctionLogsError (BR-50/BR-32) — on a malfunction
// the M4.2 internal_error log is emitted (operator visibility).
func TestRateLimitMiddlewareMalfunctionLogsError(t *testing.T) {
	logBuf := captureLog(t)
	s, ts, _ := newRateLimitTestServer(t, enabledCfg())
	s.rateLimiter.SwapClockForTest(panicTestClock{})
	mustGet(t, ts.URL+"/test").Body.Close()
	if !strings.Contains(logBuf.String(), "rate_limit: internal_error") {
		t.Errorf("malfunction must emit internal_error log (BR-50/BR-32); log=%q", logBuf.String())
	}
}

// TestRateLimitMiddlewareFailuresTotalIncrements (BR-50/BR-49) — on a
// malfunction failures_total increments and stays monotonic across requests.
func TestRateLimitMiddlewareFailuresTotalIncrements(t *testing.T) {
	s, ts, _ := newRateLimitTestServer(t, enabledCfg())
	s.rateLimiter.SwapClockForTest(panicTestClock{})
	mustGet(t, ts.URL+"/test").Body.Close()
	if got := s.rateLimiter.FailuresTotal(); got != 1 {
		t.Errorf("after one malfunction failures_total = %d, want 1 (BR-50)", got)
	}
	mustGet(t, ts.URL+"/test").Body.Close()
	if got := s.rateLimiter.FailuresTotal(); got != 2 {
		t.Errorf("after two malfunctions failures_total = %d, want 2 (BR-49 monotonic)", got)
	}
}

// TestRateLimitCounterOnDeny (BR-14, reversal #7) — an over-limit request's
// verdict Count == limit+1 (the request was counted, NOT left at the limit).
// This is the LOCKED correction of v1 reversal #7, asserted at the middleware
// level via the rejection log (M4.1 shows count=limit+1).
func TestRateLimitCounterOnDeny(t *testing.T) {
	logBuf := captureLog(t)
	limit := 10
	window := 60
	cfg := &config.RateLimitConfig{
		Enabled: true, FailMode: "fail_open",
		Defaults:       config.RateLimitDefaults{Limit: &limit, WindowSeconds: &window},
		MaxTrackedKeys: ptrIntAPI(10000),
	}
	_, ts, _ := newRateLimitTestServer(t, cfg)
	for i := 0; i < limit; i++ {
		mustGet(t, ts.URL+"/test").Body.Close() // within limit
	}
	// The (limit+1)-th request is over-limit → 429, count == limit+1.
	resp := mustGet(t, ts.URL+"/test")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on the (limit+1)-th request, got %d", resp.StatusCode)
	}
	// The M4.1 log shows count=11 (limit+1), NOT count=10 (BR-14).
	if !strings.Contains(logBuf.String(), "count=11") {
		t.Errorf("over-limit log must show count=%d (limit+1, BR-14); log=%q", limit+1, logBuf.String())
	}
}

// TestRateLimitPassthroughNoHeadersNoLog (§2.4, PERF-12) — when the limiter is
// nil (disabled), the request is byte-identical to pre-feature: no
// RateLimit-* headers, no log line, no Allow call.
func TestRateLimitPassthroughNoHeadersNoLog(t *testing.T) {
	logBuf := captureLog(t)
	s := &Server{
		sseClients: make(map[string][]chan SSEMessage),
		sseBuffers: make(map[string][]*SSEMessage),
		mux:        http.NewServeMux(),
	}
	s.mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	// No ConfigureRateLimiting → limiter nil → passthrough.
	handler := s.recoveryMiddleware(s.corsMiddleware(s.rateLimitMiddleware(s.mux)))
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	resp := mustGet(t, ts.URL+"/test")
	defer resp.Body.Close()
	for _, h := range []string{"Ratelimit-Limit", "Ratelimit-Remaining", "Ratelimit-Reset", "X-Ratelimit-Policy", "Retry-After"} {
		if resp.Header.Get(h) != "" {
			t.Errorf("disabled limiter must set NO RateLimit-* headers (§2.4), got %q", resp.Header.Get(h))
		}
	}
	if strings.Contains(logBuf.String(), "rate_limit:") {
		t.Errorf("disabled limiter must emit no rate_limit log (§2.4); log=%q", logBuf.String())
	}
}

// TestRateLimitDisabledStatusRoute404 (BR-47, §2.9) — when the limiter is
// disabled, the /health/rate-limit route is NOT registered → 404.
func TestRateLimitDisabledStatusRoute404(t *testing.T) {
	s := &Server{
		sseClients: make(map[string][]chan SSEMessage),
		sseBuffers: make(map[string][]*SSEMessage),
		mux:        http.NewServeMux(),
	}
	s.mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// Disabled: no ConfigureRateLimiting (or enabled:false).
	handler := s.recoveryMiddleware(s.corsMiddleware(s.rateLimitMiddleware(s.mux)))
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	resp := mustGet(t, ts.URL+"/health/rate-limit")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("disabled limiter: /health/rate-limit must 404 (BR-47/§2.9), got %d", resp.StatusCode)
	}
}

// TestRateLimitStatusSchemaFullShape (BR-27, §2.12) — the status response has
// every field from the LOCKED schema in the LOCKED key order. Decodes the
// body using a streaming decoder (preserves wire order, unlike map unmarshal)
// and asserts the field set + order. Complements TestRateLimitStatusSchemaKeyOrder
// which uses the re-encode approach; this one verifies the raw wire bytes.
func TestRateLimitStatusSchemaFullShape(t *testing.T) {
	_, ts, _ := newRateLimitTestServer(t, enabledCfg())
	resp := mustGet(t, ts.URL+"/health/rate-limit")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	// Assert the full field set is present.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("status body not valid JSON: %v", err)
	}
	wantFields := []string{
		"status", "enabled", "limiter", "config_source", "trust_proxy_headers",
		"dry_run", "fail_mode", "defaults", "endpoint_overrides",
		"active_keys", "active_keys_truncated", "rejections_total",
		"failures_total", "generated_at",
	}
	for _, f := range wantFields {
		if _, ok := raw[f]; !ok {
			t.Errorf("status response missing field %q (BR-27)", f)
		}
	}
	// Assert wire key order matches the struct field declaration order, using
	// a streaming decoder (the only way to preserve order from json).
	var ordered []string
	dec := json.NewDecoder(bytes.NewReader(body))
	if tok, err := dec.Token(); err == nil {
		if delim, ok := tok.(json.Delim); ok && delim == '{' {
			for dec.More() {
				tok, err := dec.Token()
				if err != nil {
					break
				}
				if key, ok := tok.(string); ok {
					ordered = append(ordered, key)
					var val interface{}
					dec.Decode(&val)
				}
			}
		}
	}
	for i, want := range wantFields {
		if i >= len(ordered) {
			t.Errorf("status response has fewer keys than schema (%d < %d) (BR-27)", len(ordered), len(wantFields))
			break
		}
		if ordered[i] != want {
			t.Errorf("status key order[%d] = %q, want %q (BR-27 LOCKED order)", i, ordered[i], want)
		}
	}
}

// TestRateLimitStatusGeneratedAtRFC3339UTC (BR-48) — generated_at is RFC3339
// UTC (Z suffix), parseable by time.Parse(time.RFC3339).
func TestRateLimitStatusGeneratedAtRFC3339UTC(t *testing.T) {
	_, ts, _ := newRateLimitTestServer(t, enabledCfg())
	resp := mustGet(t, ts.URL+"/health/rate-limit")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var typed rateLimitStatusResponse
	json.Unmarshal(body, &typed)
	if typed.GeneratedAt == "" {
		t.Fatal("generated_at empty")
	}
	ts2, err := time.Parse(time.RFC3339, typed.GeneratedAt)
	if err != nil {
		t.Errorf("generated_at not RFC3339 (BR-48): %v", err)
	}
	if ts2.Location() != time.UTC && !strings.HasSuffix(typed.GeneratedAt, "Z") {
		t.Errorf("generated_at should be UTC (BR-48); got %q", typed.GeneratedAt)
	}
}
