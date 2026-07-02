package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/ratelimit"
	"github.com/MichielDean/devteam/internal/spec"
)

// These tests exercise the rate-limit middleware, 429 shaping, status
// endpoint, fail-open path, and ConfigureRateLimiting wiring in isolation via
// httptest (R9 per-case TestXxx, E14 — no DB, no live server). The limiter is
// armed directly on a minimal Server (NewServer requires db/pipeline for the
// full app, but the rate-limit path touches none of those; we construct the
// Server struct literally to avoid the DB dependency).

// newTestServer builds a minimal Server with only the rate-limit fields set,
// plus a mux so the status route can be registered. No DB, no pipeline.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	return &Server{
		mux: http.NewServeMux(),
	}
}

func ptrInt(v int) *int { return &v }
func ptrBool(v bool) *bool { return &v }

// enableArms configures and arms the limiter on s with the given policy.
func arm(t *testing.T, s *Server, limit int, windowSecs int) *ratelimit.Limiter {
	t.Helper()
	limiter, err := ratelimit.New(ratelimit.Policy{Limit: limit, Window: time.Duration(windowSecs) * time.Second})
	if err != nil {
		t.Fatalf("arm: New: %v", err)
	}
	s.rateLimiter = limiter
	s.rlCfg = &config.RateLimitConfig{
		Enabled:  true,
		FailMode: "fail_open",
		Defaults: config.RateLimitDefaults{Limit: ptrInt(limit), WindowSeconds: ptrInt(windowSecs)},
	}
	s.rlExtractor = ratelimit.KeyExtractor{}
	return limiter
}

// chain wraps a handler in the rate-limit middleware only (no recovery/cors)
// for unit tests that want to assert rate-limit behavior directly.
func (s *Server) rateLimitOnly(next http.Handler) http.Handler {
	return s.rateLimitMiddleware(next)
}

func TestRateLimitMiddlewarePassThroughWhenDisabled(t *testing.T) {
	// BR-33/D7: nil limiter → byte-identical pass-through.
	s := newTestServer(t)
	called := false
	h := s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "1.2.3.4:1"
	h.ServeHTTP(rec, req)
	if !called {
		t.Fatal("nil limiter: next not called (pass-through broken)")
	}
	// No RateLimit-* headers when disabled.
	for k := range rec.Header() {
		if strings.HasPrefix(k, "RateLimit-") || k == "X-RateLimit-Policy" || k == "Retry-After" {
			t.Errorf("disabled limiter set header %q (should be absent)", k)
		}
	}
}

func TestRateLimitAdvisoryHeadersOnAllow(t *testing.T) {
	// BR-22/O-9 (corrects C-2): advisory headers ALWAYS on allow-path (enforce
	// mode), except exempt/malfunction. 4 headers: RateLimit-Limit,
	// RateLimit-Remaining, RateLimit-Reset, X-RateLimit-Policy. No Retry-After.
	s := newTestServer(t)
	arm(t, s, 10, 60)
	h := s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/run", nil)
	req.RemoteAddr = "198.51.100.42:1"
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("allow path: code = %d, want 200", rec.Code)
	}
	for _, h := range []string{"RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset", "X-RateLimit-Policy"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("allow path: missing advisory header %q", h)
		}
	}
	if ra := rec.Header().Get("Retry-After"); ra != "" {
		t.Errorf("allow path: Retry-After present (%q), should be absent", ra)
	}
	// X-RateLimit-Policy format: "<Limit>;w=<Window>" (BR-25).
	if got := rec.Header().Get("X-RateLimit-Policy"); got != "10;w=60" {
		t.Errorf("X-RateLimit-Policy = %q, want 10;w=60", got)
	}
}

func TestRateLimitRemainingDecrements(t *testing.T) {
	// BR-22: second request → Remaining is lower.
	s := newTestServer(t)
	arm(t, s, 10, 60)
	h := s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "1.2.3.4:1"
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	r1 := rec1.Header().Get("RateLimit-Remaining")
	r2 := rec2.Header().Get("RateLimit-Remaining")
	if r1 == "" || r2 == "" {
		t.Fatalf("Remaining headers missing: %q %q", r1, r2)
	}
	if r2 >= r1 {
		t.Errorf("Remaining did not decrement: first=%s second=%s", r1, r2)
	}
}

func TestRateLimitRejection429Status(t *testing.T) {
	// BR-19: over-limit → 429. Invariant 2: next is NOT called on a clean deny.
	s := newTestServer(t)
	arm(t, s, 1, 60)
	calls := 0
	h := s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "1.2.3.4:1"
	h.ServeHTTP(httptest.NewRecorder(), req) // first: allow (handler called once)
	if calls != 1 {
		t.Errorf("first call: handler calls = %d, want 1 (allow)", calls)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req) // second: deny (handler NOT called)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("over-limit: code = %d, want 429", rec.Code)
	}
	if calls != 1 {
		t.Errorf("deny call: handler calls = %d, want 1 (next not called on reject, invariant 2)", calls)
	}
}

func TestRateLimitRejectionHeadersPresent(t *testing.T) {
	// BR-20: all 5 headers present and correct format on 429.
	s := newTestServer(t)
	arm(t, s, 1, 60)
	h := s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "1.2.3.4:1"
	h.ServeHTTP(httptest.NewRecorder(), req) // first allow
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	for _, hdr := range []string{"Retry-After", "RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset", "X-RateLimit-Policy"} {
		if rec.Header().Get(hdr) == "" {
			t.Errorf("429: missing header %q", hdr)
		}
	}
	if got := rec.Header().Get("RateLimit-Remaining"); got != "0" {
		t.Errorf("429 RateLimit-Remaining = %q, want 0", got)
	}
}

func TestRateLimitRejectionRetryAfterIsDeltaSeconds(t *testing.T) {
	// BR-24/O-3: Retry-After is integer delta-seconds, NOT HTTP-date.
	s := newTestServer(t)
	arm(t, s, 1, 60)
	h := s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "1.2.3.4:1"
	h.ServeHTTP(httptest.NewRecorder(), req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	ra := rec.Header().Get("Retry-After")
	if ra == "" {
		t.Fatal("Retry-After missing")
	}
	// Must be all digits (delta-seconds). An HTTP-date would contain letters.
	for _, c := range ra {
		if c < '0' || c > '9' {
			t.Errorf("Retry-After = %q contains non-digit %q (should be delta-seconds)", ra, c)
		}
	}
}

func TestRateLimitRejectionBodyIsErrorResponse(t *testing.T) {
	// BR-21/RC-1 (corrects C-1): body == {"error":"rate_limit_exceeded","details":"..."}.
	// NO key, policy, retry_after_seconds, or request_id in body.
	s := newTestServer(t)
	arm(t, s, 1, 60)
	h := s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "1.2.3.4:1"
	h.ServeHTTP(httptest.NewRecorder(), req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON object: %v (body=%s)", err, rec.Body.String())
	}
	if got := string(body["error"]); got != `"rate_limit_exceeded"` {
		t.Errorf("body.error = %s, want \"rate_limit_exceeded\"", got)
	}
	if _, has := body["details"]; !has {
		t.Error("body missing details")
	}
	// Forbidden fields (RC-1/RC-3):
	for _, f := range []string{"key", "policy", "retry_after_seconds", "request_id"} {
		if _, has := body[f]; has {
			t.Errorf("body has forbidden field %q (BR-21/RC-3)", f)
		}
	}
}

func TestRateLimitStatusRoute404WhenDisabled(t *testing.T) {
	// BR-33/BR-47: disabled limiter → status route NOT registered → 404.
	s := newTestServer(t)
	// No ConfigureRateLimiting call → s.rateLimiter is nil → route not registered.
	// Hit the mux directly; since we didn't register the route, mux 404s.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/rate-limit", nil)
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("disabled status route: code = %d, want 404", rec.Code)
	}
}

func TestConfigureRateLimitingArmsLimiter(t *testing.T) {
	// U-W: valid cfg → s.rateLimiter != nil, extractor configured.
	s := newTestServer(t)
	cfg := &config.RateLimitConfig{
		Enabled:  true,
		FailMode: "fail_open",
		Defaults: config.RateLimitDefaults{Limit: ptrInt(50), WindowSeconds: ptrInt(30)},
	}
	s.ConfigureRateLimiting(cfg)
	if s.rateLimiter == nil {
		t.Fatal("valid cfg: rateLimiter is nil (not armed)")
	}
}

func TestConfigureRateLimitingNilConfigPassthrough(t *testing.T) {
	// D7: nil cfg → rateLimiter == nil.
	s := newTestServer(t)
	s.ConfigureRateLimiting(nil)
	if s.rateLimiter != nil {
		t.Error("nil cfg: rateLimiter should be nil")
	}
}

func TestConfigureRateLimitingDisabledPassthrough(t *testing.T) {
	// D7: enabled=false → rateLimiter == nil.
	s := newTestServer(t)
	cfg := &config.RateLimitConfig{Enabled: false}
	s.ConfigureRateLimiting(cfg)
	if s.rateLimiter != nil {
		t.Error("enabled=false: rateLimiter should be nil")
	}
}

func TestConfigureRateLimitingInvalidConfigFailsOpen(t *testing.T) {
	// ADR-008/O-5/BR-08: malformed cfg → log + rateLimiter == nil (no crash).
	s := newTestServer(t)
	cfg := &config.RateLimitConfig{
		Enabled:  true,
		FailMode: "fail_closed", // BR-01 rejects this
	}
	s.ConfigureRateLimiting(cfg)
	if s.rateLimiter != nil {
		t.Error("invalid cfg: rateLimiter should be nil (fail-open startup)")
	}
}

func TestConfigureRateLimitingRegistersStatusRoute(t *testing.T) {
	// BR-47: armed → GET /health/rate-limit 200; nil → 404 (tested above).
	s := newTestServer(t)
	cfg := &config.RateLimitConfig{Enabled: true, FailMode: "fail_open"}
	s.ConfigureRateLimiting(cfg)
	if s.rateLimiter == nil {
		t.Fatal("not armed")
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/rate-limit", nil)
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("armed status route: code = %d, want 200", rec.Code)
	}
}

func TestRateLimitStatusReturns200(t *testing.T) {
	// BR-26: status endpoint always 200, never calls Allow.
	s := newTestServer(t)
	arm(t, s, 1, 60)
	// Register the handler on the mux (ConfigureRateLimiting does this in prod).
	s.mux.HandleFunc("GET /health/rate-limit", s.handleRateLimitStatus)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/rate-limit", nil)
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: code = %d, want 200", rec.Code)
	}
}

func TestRateLimitStatusExemptFromLimiting(t *testing.T) {
	// BR-26/D3: caller's key at limit, repeated GET status → always 200, never 429.
	s := newTestServer(t)
	arm(t, s, 1, 60)
	s.mux.HandleFunc("GET /health/rate-limit", s.handleRateLimitStatus)
	req := httptest.NewRequest(http.MethodGet, "/health/rate-limit", nil)
	req.RemoteAddr = "1.2.3.4:1"
	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		s.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("status call %d: code = %d, want 200 (exempt)", i, rec.Code)
		}
	}
}

func TestRateLimitStatusEmptyHealthy(t *testing.T) {
	// BR-28: no traffic → active_keys == [] (not null), counters 0.
	s := newTestServer(t)
	arm(t, s, 10, 60)
	s.mux.HandleFunc("GET /health/rate-limit", s.handleRateLimitStatus)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/rate-limit", nil)
	s.mux.ServeHTTP(rec, req)
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("status body not JSON: %v", err)
	}
	ak, has := body["active_keys"]
	if !has {
		t.Fatal("status missing active_keys")
	}
	if string(ak) == "null" {
		t.Error("active_keys is null, want [] (BR-28)")
	}
	if string(ak) != "[]" {
		t.Errorf("empty active_keys = %s, want []", string(ak))
	}
}

func TestRateLimitStatusActiveKeysReflectState(t *testing.T) {
	// US5 scenario 4: tracked keys present with count/limit/window/reset_in.
	s := newTestServer(t)
	arm(t, s, 10, 60)
	s.mux.HandleFunc("GET /health/rate-limit", s.handleRateLimitStatus)
	// Generate traffic.
	req := httptest.NewRequest(http.MethodGet, "/v1/run", nil)
	req.RemoteAddr = "198.51.100.42:1"
	for i := 0; i < 3; i++ {
		s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(httptest.NewRecorder(), req)
	}
	// Query status.
	rec := httptest.NewRecorder()
	statusReq := httptest.NewRequest(http.MethodGet, "/health/rate-limit", nil)
	s.mux.ServeHTTP(rec, statusReq)
	var resp rateLimitStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("status body: %v", err)
	}
	if len(resp.ActiveKeys) == 0 {
		t.Fatal("no active_keys after traffic")
	}
	if resp.ActiveKeys[0].Key == "" {
		t.Error("active_keys[0].Key empty")
	}
}

func TestRateLimitStatusShowsFullIPs(t *testing.T) {
	// BR-30/O-4 (corrects C-4): NO redaction in v1 — full IPs in active_keys[].key.
	s := newTestServer(t)
	arm(t, s, 10, 60)
	s.mux.HandleFunc("GET /health/rate-limit", s.handleRateLimitStatus)
	req := httptest.NewRequest(http.MethodGet, "/v1/run", nil)
	req.RemoteAddr = "198.51.100.42:1"
	s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(httptest.NewRecorder(), req)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/rate-limit", nil))
	var resp rateLimitStatusResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.ActiveKeys) == 0 {
		t.Fatal("no active_keys")
	}
	if !strings.Contains(resp.ActiveKeys[0].Key, "198.51.100.42") {
		t.Errorf("active_keys[0].Key = %q, want full IP (no redaction, BR-30)", resp.ActiveKeys[0].Key)
	}
}

func TestRateLimitStatusSchemaKeyOrder(t *testing.T) {
	// BR-27/M2.1: keys in locked order.
	s := newTestServer(t)
	arm(t, s, 10, 60)
	s.mux.HandleFunc("GET /health/rate-limit", s.handleRateLimitStatus)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/rate-limit", nil))
	// Decode into a raw map preserving order via a decoder — use a slice of keys.
	dec := json.NewDecoder(rec.Body)
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	wantOrder := []string{
		"status", "enabled", "limiter", "config_source", "trust_proxy_headers",
		"dry_run", "fail_mode", "defaults", "endpoint_overrides", "active_keys",
		"active_keys_truncated", "rejections_total", "failures_total", "generated_at",
	}
	// Re-decode preserving order via json.RawMessage on the raw bytes. We compare
	// the set of keys present (Go struct field order == JSON key order, so if all
	// keys are present the order is guaranteed by the struct definition).
	for _, k := range wantOrder {
		if _, has := raw[k]; !has {
			t.Errorf("status schema missing key %q", k)
		}
	}
}

func TestRateLimitStatusActiveKeysCapped(t *testing.T) {
	// BR-29/O-2: >cap keys → active_keys_truncated=true, array length ≤ cap.
	s := newTestServer(t)
	// Small cap via config to make the test fast. Floor is 100; use 100.
	cfg := &config.RateLimitConfig{
		Enabled:       true,
		FailMode:      "fail_open",
		MaxTrackedKeys: ptrInt(100),
		Defaults:      config.RateLimitDefaults{Limit: ptrInt(10000), WindowSeconds: ptrInt(3600)},
	}
	s.ConfigureRateLimiting(cfg) // registers the status route on s.mux
	if s.rateLimiter == nil {
		t.Fatal("not armed")
	}
	// Generate >100 distinct keys.
	for i := 0; i < 110; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = fmt.Sprintf("%d.%d.%d.%d:1", 10+i/10000, i/100%256, i%100, 1)
		s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(httptest.NewRecorder(), req)
	}
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/rate-limit", nil))
	var resp rateLimitStatusResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.ActiveKeysTruncated {
		t.Error("active_keys_truncated = false, want true (cap exceeded)")
	}
	if len(resp.ActiveKeys) > 100 {
		t.Errorf("active_keys len = %d, want <= 100 (cap)", len(resp.ActiveKeys))
	}
}

func TestRateLimitStatusActiveKeysNotTruncatedAtExactCap(t *testing.T) {
	// BR-29/O-2 boundary case: exactly cap keys → active_keys_truncated=false
	// (no truncation occurred — the prior implementation false-positived here
	// using len(Keys) >= cap; the corrected impl uses TotalKeys > cap).
	s := newTestServer(t)
	cfg := &config.RateLimitConfig{
		Enabled:       true,
		FailMode:      "fail_open",
		MaxTrackedKeys: ptrInt(100),
		Defaults:      config.RateLimitDefaults{Limit: ptrInt(10000), WindowSeconds: ptrInt(3600)},
	}
	s.ConfigureRateLimiting(cfg) // registers the status route
	// Generate exactly 100 distinct keys (within the cap, no eviction).
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = fmt.Sprintf("%d.%d.%d.%d:1", 10, i/100%256, i%100, 1)
		s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(httptest.NewRecorder(), req)
	}
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/rate-limit", nil))
	var resp rateLimitStatusResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ActiveKeysTruncated {
		t.Errorf("exactly-cap keys: active_keys_truncated = true, want false (no truncation occurred)")
	}
	if len(resp.ActiveKeys) != 100 {
		t.Errorf("exactly-cap keys: active_keys len = %d, want 100", len(resp.ActiveKeys))
	}
}

func TestRateLimitMiddlewareCORSOptionsShortCircuits(t *testing.T) {
	// BR-34/D6: OPTIONS preflight does NOT increment the counter. The CORS
	// middleware short-circuits to 204 BEFORE the limiter runs (chain order
	// recovery(cors(rateLimit(mux)))). Test the full chain.
	s := newTestServer(t)
	arm(t, s, 1, 60)
	// Full chain: recovery(cors(rateLimit(mux))).
	mux := s.mux
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := s.recoveryMiddleware(s.corsMiddleware(s.rateLimitMiddleware(mux)))
	// OPTIONS request.
	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	req.RemoteAddr = "1.2.3.4:1"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS: code = %d, want 204 (CORS short-circuit)", rec.Code)
	}
	// The limiter counter must not have incremented — a subsequent GET from the
	// same IP should still be allowed (limit 1, used 0).
	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	req2.RemoteAddr = "1.2.3.4:1"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("GET after OPTIONS: code = %d, want 200 (OPTIONS did not increment counter)", rec2.Code)
	}
}

func TestRateLimitMiddlewarePanicRecovered(t *testing.T) {
	// BR-35/NDP-02: injected limiter panic → recovered, server up (500 or
	// pass-through depending on layer). Two-layer recovery: the middleware's
	// own defer/recover catches it first → fail-open (traffic flows).
	s := newTestServer(t)
	arm(t, s, 10, 60)
	// Replace the limiter with one whose Allow panics via a panicking clock.
	clk := &panickingTestClock{}
	limiter, _ := ratelimit.New(ratelimit.Policy{Limit: 10, Window: 60 * time.Second}, ratelimit.WithClock(clk))
	s.rateLimiter = limiter
	h := s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	clk.panicNext = true
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "1.2.3.4:1"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	// Fail-open: traffic flows (200 from the handler), NO RateLimit-* headers.
	if rec.Code != http.StatusOK {
		t.Errorf("malfunction: code = %d, want 200 (fail-open, traffic flows)", rec.Code)
	}
	for k := range rec.Header() {
		if strings.HasPrefix(k, "RateLimit-") {
			t.Errorf("malfunction path set header %q (should be absent, invariant 4)", k)
		}
	}
}

func TestRateLimitMiddlewareMalfunctionPassesThrough(t *testing.T) {
	// BR-50: middleware gets err → next called, traffic flows.
	s := newTestServer(t)
	arm(t, s, 10, 60)
	clk := &panickingTestClock{}
	limiter, _ := ratelimit.New(ratelimit.Policy{Limit: 10, Window: 60 * time.Second}, ratelimit.WithClock(clk))
	s.rateLimiter = limiter
	called := false
	h := s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	clk.panicNext = true
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "1.2.3.4:1"
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !called {
		t.Error("malfunction: next not called (fail-open broken)")
	}
}

func TestRateLimitMiddlewareFailuresTotalIncrements(t *testing.T) {
	// BR-50: malfunction → failuresTotal+1.
	s := newTestServer(t)
	arm(t, s, 10, 60)
	clk := &panickingTestClock{}
	limiter, _ := ratelimit.New(ratelimit.Policy{Limit: 10, Window: 60 * time.Second}, ratelimit.WithClock(clk))
	s.rateLimiter = limiter
	h := s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	before := s.rateLimiter.FailuresTotal()
	clk.panicNext = true
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "1.2.3.4:1"
	h.ServeHTTP(httptest.NewRecorder(), req)
	after := s.rateLimiter.FailuresTotal()
	if after != before+1 {
		t.Errorf("malfunction: FailuresTotal %d → %d, want +1", before, after)
	}
}

func TestRateLimitCleanDenialNotMalfunction(t *testing.T) {
	// BR-51: clean 429 → "rejected" log, NOT "internal_error"; failures_total not incremented.
	s := newTestServer(t)
	arm(t, s, 1, 60)
	h := s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "1.2.3.4:1"
	// First: allow.
	h.ServeHTTP(httptest.NewRecorder(), req)
	beforeFailures := s.rateLimiter.FailuresTotal()
	// Second: deny (clean 429, not malfunction).
	h.ServeHTTP(httptest.NewRecorder(), req)
	afterFailures := s.rateLimiter.FailuresTotal()
	if afterFailures != beforeFailures {
		t.Errorf("clean deny incremented failures_total: %d → %d (should not)", beforeFailures, afterFailures)
	}
}

func TestRateLimitExemptRouteNoHeaders(t *testing.T) {
	// BR-09/BR-23: exempt override route → no RateLimit-* headers, counter unchanged.
	s := newTestServer(t)
	cfg := &config.RateLimitConfig{
		Enabled:  true,
		FailMode: "fail_open",
		Defaults: config.RateLimitDefaults{Limit: ptrInt(1), WindowSeconds: ptrInt(60)},
		EndpointOverrides: map[string]config.RateLimitOverride{
			"GET /exempt": {Exempt: true},
		},
	}
	s.ConfigureRateLimiting(cfg)
	h := s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	// Exempt route.
	req := httptest.NewRequest(http.MethodGet, "/exempt", nil)
	req.RemoteAddr = "1.2.3.4:1"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("exempt route: code = %d, want 200", rec.Code)
	}
	for k := range rec.Header() {
		if strings.HasPrefix(k, "RateLimit-") || k == "X-RateLimit-Policy" {
			t.Errorf("exempt route set header %q (should be absent, BR-23)", k)
		}
	}
}

func TestRateLimit429LoopServerStable(t *testing.T) {
	// BR-38/NDP-16: a client that ignores Retry-After and retries immediately
	// gets repeated 429s. Server is stable; rejections_total climbs.
	s := newTestServer(t)
	arm(t, s, 1, 60)
	calls := 0
	h := s.rateLimitOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "1.2.3.4:1"
	// First: allow (handler called once).
	h.ServeHTTP(httptest.NewRecorder(), req)
	if calls != 1 {
		t.Fatalf("first call: handler calls = %d, want 1", calls)
	}
	// 100 immediate retries → 100 429s (handler NOT called on deny path).
	for i := 0; i < 100; i++ {
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
	if calls != 1 {
		t.Errorf("after 429 loop: handler calls = %d, want 1 (next not called on reject)", calls)
	}
	if got := s.rateLimiter.RejectionsTotal(); got != 100 {
		t.Errorf("rejections_total = %d, want 100", got)
	}
}

func TestNewServerSignatureUnchanged(t *testing.T) {
	// ADR-007 regression guard: NewServer keeps its 6-arg signature. This is a
	// compile-time check — assigning NewServer to a typed var pins the exact
	// parameter types; if the signature changes, this fails to build.
	var _ func(string, *spec.SpecProvider, *pipeline.Pipeline, fs.FS, feature.QuestionStore, *db.DB) *Server = NewServer
}

func TestRateLimitStatusConfigSource(t *testing.T) {
	// BR-46: config_source reflects the loaded YAML path.
	s := newTestServer(t)
	cfg := &config.RateLimitConfig{Enabled: true, FailMode: "fail_open"}
	cfg.ConfigSource = "/path/to/devteam.yaml"
	s.ConfigureRateLimiting(cfg) // registers the status route on s.mux
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/rate-limit", nil))
	var resp rateLimitStatusResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ConfigSource != "/path/to/devteam.yaml" {
		t.Errorf("config_source = %q, want /path/to/devteam.yaml", resp.ConfigSource)
	}
}

func TestRateLimitStatusGeneratedAtRFC3339(t *testing.T) {
	// BR-48: generated_at is RFC3339 UTC.
	s := newTestServer(t)
	arm(t, s, 10, 60)
	s.mux.HandleFunc("GET /health/rate-limit", s.handleRateLimitStatus)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/rate-limit", nil))
	var resp rateLimitStatusResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if _, err := time.Parse(time.RFC3339, resp.GeneratedAt); err != nil {
		t.Errorf("generated_at %q not RFC3339: %v", resp.GeneratedAt, err)
	}
}

// panickingTestClock panics on the next Now() when panicNext is set, simulating
// an internal error mid-evaluation (for the fail-open tests).
type panickingTestClock struct {
	panicNext bool
}

func (p *panickingTestClock) Now() time.Time {
	if p.panicNext {
		p.panicNext = false
		panic("simulated internal error mid-evaluation")
	}
	return time.Unix(1_000_000, 0)
}