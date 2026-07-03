package ratelimit

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// fakeClock returns a controllable Clock for deterministic time advancement
// (BLM §2.6 — no real wall clock in unit tests).
func fakeClock(start time.Time) *FakeClock { return NewFakeClock(start) }

// newTestLimiter builds a Limiter with a fake clock and small max-tracked for
// eviction tests. Tests that need a specific policy pass it to Allow.
func newTestLimiter(t *testing.T, clock Clock, maxTracked int) *Limiter {
	t.Helper()
	if maxTracked == 0 {
		maxTracked = DefaultMaxTrackedKeys
	}
	l, err := New(WithClock(clock), WithMaxTrackedKeys(maxTracked))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return l
}

func policy(limit int, window time.Duration) Policy {
	return Policy{Limit: limit, Window: window}
}

// TestLimiterAllowsWithinLimit (BR-14, BR-11) — within-limit requests all
// allow; the counter matches the number of requests.
func TestLimiterAllowsWithinLimit(t *testing.T) {
	clock := fakeClock(time.Unix(1000, 0))
	l := newTestLimiter(t, clock, 0)
	p := policy(10, 60*time.Second)

	for i := 0; i < 9; i++ {
		v := l.Allow("k1", p)
		if !v.Allow {
			t.Errorf("request %d: expected allow, got deny (count=%d limit=%d)", i+1, v.Count, v.Limit)
		}
	}
	// 9 requests under the limit — counter should reflect demand (BR-11).
	v := l.Allow("k1", p)
	if v.Count != 10 {
		t.Errorf("expected count=10 after 10 allows, got %d", v.Count)
	}
	if !v.Allow {
		t.Errorf("10th request under limit=10 should allow (count=%d < limit=%d)", v.Count, v.Limit)
	}
}

// TestLimiterDeniesOverLimit (BR-14 LOCKED, §2.14) — the 11th request over
// limit=10 is denied AND the counter increments (counter-on-deny is the
// LOCKED behavior; the over-limit request counts, so counter == limit+1).
// This test MUST assert counter == 11, NOT "counter stays at 10" — that was
// the v1 reversal corrected by BR-14 (reversal correction #7).
func TestLimiterDeniesOverLimit(t *testing.T) {
	clock := fakeClock(time.Unix(1000, 0))
	l := newTestLimiter(t, clock, 0)
	p := policy(10, 60*time.Second)

	for i := 0; i < 10; i++ {
		v := l.Allow("k1", p)
		if !v.Allow {
			t.Fatalf("request %d: expected allow, got deny", i+1)
		}
	}
	// 11th request: over limit, deny, counter == limit+1 (BR-14 correction).
	v := l.Allow("k1", p)
	if v.Allow {
		t.Errorf("11th request: expected deny, got allow")
	}
	if !v.OverLimit {
		t.Errorf("11th request: expected OverLimit=true")
	}
	if v.Count != 11 {
		t.Errorf("11th over-limit request: counter must == limit+1 == 11 (BR-14 LOCKED), got %d", v.Count)
	}
}

// TestLimiterWindowSlides (BR-15) — requests older than the window expire.
// After the fake clock advances past 2*window, both buckets reset and a
// new request is allowed.
func TestLimiterWindowSlides(t *testing.T) {
	clock := fakeClock(time.Unix(1000, 0))
	l := newTestLimiter(t, clock, 0)
	p := policy(3, 60*time.Second)

	// Fill the window.
	for i := 0; i < 3; i++ {
		l.Allow("k1", p)
	}
	// 4th over limit → deny.
	if v := l.Allow("k1", p); v.Allow {
		t.Errorf("4th request should deny")
	}

	// Advance past 2*window — both buckets are fully stale; the rollover
	// resets them and a new request is allowed.
	clock.Advance(121 * time.Second)
	v := l.Allow("k1", p)
	if !v.Allow {
		t.Errorf("after window slide: expected allow, got deny (count=%d)", v.Count)
	}
}

// TestLimiterCompositeKeyIndependent (BR-16) — same IP, different routes get
// independent counters.
func TestLimiterCompositeKeyIndependent(t *testing.T) {
	clock := fakeClock(time.Unix(1000, 0))
	l := newTestLimiter(t, clock, 0)
	p := policy(2, 60*time.Second)

	// Exhaust POST /v1/run for IP X.
	l.Allow("1.2.3.4|POST /v1/run", p)
	l.Allow("1.2.3.4|POST /v1/run", p)
	if v := l.Allow("1.2.3.4|POST /v1/run", p); v.Allow {
		t.Errorf("third request on same key should deny")
	}
	// Same IP, different route — independent counter, should allow.
	if v := l.Allow("1.2.3.4|GET /api/features", p); !v.Allow {
		t.Errorf("different route, same IP should allow (independent counters BR-16), got deny count=%d", v.Count)
	}
}

// TestLimiterConcurrentSafe (BR-44, REL-06, PERF-06) — parallel Allow calls
// under -race must not lose updates or corrupt state.
func TestLimiterConcurrentSafe(t *testing.T) {
	clock := fakeClock(time.Unix(1000, 0))
	l := newTestLimiter(t, clock, 0)
	p := policy(10000, 60*time.Second) // high enough that none deny

	var wg sync.WaitGroup
	const goroutines = 8
	const perG = 1000
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				l.Allow("shared-key", p)
			}
		}()
	}
	wg.Wait()

	got := l.Len()
	if got != 1 {
		t.Errorf("expected 1 tracked key, got %d", got)
	}
}

// TestLimiterConcurrentNoLostUpdates (BR-44, REL-06) — N goroutines × M
// requests produce a known total. We verify via the rejection counter: with
// limit exactly equal to total requests, every request allows and the
// counter reflects all of them.
func TestLimiterConcurrentNoLostUpdates(t *testing.T) {
	clock := fakeClock(time.Unix(1000, 0))
	l := newTestLimiter(t, clock, 0)
	const goroutines = 8
	const perG = 100
	total := goroutines * perG
	p := policy(total, 60*time.Second) // all should allow

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				l.Allow("shared-key", p)
			}
		}()
	}
	wg.Wait()

	// One more request at the limit → deny, counter == total+1.
	v := l.Allow("shared-key", p)
	if v.Allow {
		t.Errorf("request after limit should deny")
	}
	if v.Count != total+1 {
		t.Errorf("expected counter == total+1 == %d, got %d (lost updates? BR-44)", total+1, v.Count)
	}
}

// TestLimiterAllowMalfunctionFailsOpen (BR-50, §2.1, REL-01) — a malfunction
// during Allow yields a fail-open verdict {Allow:true} + non-nil err. The
// counter is NOT incremented (BR-11 — malfunction does not count), and
// failures_total IS incremented (BR-50).
func TestLimiterAllowMalfunctionFailsOpen(t *testing.T) {
	clock := fakeClock(time.Unix(1000, 0))
	l := newTestLimiter(t, clock, 0)
	p := policy(10, 60*time.Second)

	// Prime a key so we have state to observe.
	l.Allow("k1", p)
	if l.FailuresTotal() != 0 {
		t.Fatalf("baseline failures_total should be 0, got %d", l.FailuresTotal())
	}

	// Inject a malfunction via a clock whose Now panics. The defer/recover
	// in Allow must catch it and return the fail-open verdict (§2.1 — the
	// ONLY error-path return shape; no fail-closed branch exists in v2).
	l.clock = panicClock{}
	v := l.Allow("k1", p)

	if !v.Allow {
		t.Errorf("malfunction must fail-open (Allow=true), got deny (BR-50)")
	}
	if v.Err == nil {
		t.Errorf("malfunction must return non-nil err (BR-50)")
	}
	if v.OverLimit {
		t.Errorf("malfunction verdict must not be OverLimit (BR-50)")
	}
	if l.FailuresTotal() != 1 {
		t.Errorf("malfunction must increment failures_total to 1, got %d (BR-50)", l.FailuresTotal())
	}
	// Counter NOT incremented on malfunction (BR-11). The primed key had 1
	// count; the malfunction must not add another. The snapshot Count is
	// the sum of both buckets (an approximation, but a malfunction does
	// not touch the buckets, so it stays at the primed value).
	l.clock = clock
	snap := l.Snapshot(100)
	for _, ks := range snap.Keys {
		if ks.Key == "k1" && ks.Count > 1 {
			t.Errorf("malfunction must not increment counter (BR-11), k1 count=%d", ks.Count)
		}
	}
}

// panicClock is a Clock whose Now panics — used to inject a malfunction
// into Allow to verify the fail-open defer/recover (§2.1).
type panicClock struct{}

func (panicClock) Now() time.Time { panic("injected malfunction for test") }

// TestLimiterMalfunctionPerRequest (BR-53, REL-03) — a malfunction on request
// 1 does not stick; request 2 (no malfunction) evaluates normally.
func TestLimiterMalfunctionPerRequest(t *testing.T) {
	clock := fakeClock(time.Unix(1000, 0))
	l := newTestLimiter(t, clock, 0)
	p := policy(10, 60*time.Second)

	// Request 1: inject malfunction.
	l.clock = panicClock{}
	v1 := l.Allow("k1", p)
	if !v1.Allow || v1.Err == nil {
		t.Fatalf("request 1 should malfunction-fail-open, got %+v", v1)
	}
	if l.FailuresTotal() != 1 {
		t.Fatalf("failures_total should be 1 after malfunction, got %d", l.FailuresTotal())
	}

	// Request 2: restore real clock; evaluates normally.
	l.clock = clock
	v2 := l.Allow("k1", p)
	if v2.Err != nil {
		t.Errorf("request 2 should evaluate normally (no malfunction), got err=%v (BR-53)", v2.Err)
	}
	if l.FailuresTotal() != 1 {
		t.Errorf("failures_total should stay 1 (no new malfunction), got %d (BR-49)", l.FailuresTotal())
	}
}

// TestLimiterFailuresTotalNotReset (BR-49) — failures_total is monotonic and
// is NOT reset when corruption clears.
func TestLimiterFailuresTotalNotReset(t *testing.T) {
	clock := fakeClock(time.Unix(1000, 0))
	l := newTestLimiter(t, clock, 0)
	p := policy(10, 60*time.Second)

	l.clock = panicClock{}
	l.Allow("k1", p) // malfunction → failures_total=1
	if got := l.FailuresTotal(); got != 1 {
		t.Fatalf("after malfunction: failures_total=%d, want 1", got)
	}
	l.clock = clock
	l.Allow("k1", p) // normal → failures_total stays 1
	if got := l.FailuresTotal(); got != 1 {
		t.Errorf("after recovery: failures_total=%d, want 1 (not reset, BR-49)", got)
	}
}

// TestLimiterEvictsExpiredKeys (BLM §3.4, SEC-07) — keys older than 2*window
// are swept on the amortized trigger.
func TestLimiterEvictsExpiredKeys(t *testing.T) {
	clock := fakeClock(time.Unix(1000, 0))
	l := newTestLimiter(t, clock, 0)
	p := policy(100, 60*time.Second)

	// Create a key.
	l.Allow("old-key", p)
	// Advance past 2*window so the key is stale.
	clock.Advance(121 * time.Second)
	// Trigger a sweep by making sweepEvery Allow calls.
	for i := 0; i < sweepEvery; i++ {
		l.Allow("fresh-key", p)
	}
	if l.Len() > 1 {
		t.Errorf("stale key should be evicted, len=%d", l.Len())
	}
}

// TestLimiterEvictsAtCap (BLM §3.4, SEC-07, REL-12, PERF-04) — when the map
// exceeds maxTracked, the oldest by lastSeen are dropped to 90% headroom.
func TestLimiterEvictsAtCap(t *testing.T) {
	clock := fakeClock(time.Unix(1000, 0))
	l := newTestLimiter(t, clock, 200)
	p := policy(10000, 60*time.Second)

	// Create 250 keys, each 1ms apart so their lastSeen differs.
	for i := 0; i < 250; i++ {
		l.Allow(fmt.Sprintf("k-%d", i), p)
		clock.Advance(time.Millisecond)
	}
	// Trigger a sweep.
	for i := 0; i < sweepEvery; i++ {
		l.Allow("trigger", p)
	}
	got := l.Len()
	if got > 200 {
		t.Errorf("cap not enforced: len=%d > maxTracked=200", got)
	}
	target := int(float64(200) * 0.9)
	if got > target+1 { // +1 for the trigger key itself which is fresh
		t.Errorf("expected ~%d keys after eviction to 90%%, got %d", target, got)
	}
}

// TestLimiterSnapshotSortedAndBounded (BR-27, BR-29, §2.12) — snapshot is
// sorted by count DESC and capped at maxKeys with active_keys_truncated=true.
func TestLimiterSnapshotSortedAndBounded(t *testing.T) {
	clock := fakeClock(time.Unix(1000, 0))
	l := newTestLimiter(t, clock, 0)
	p := policy(1000, 60*time.Second)

	// Create 5 keys with different counts.
	for i := 0; i < 5; i++ {
		count := (i + 1) * 10 // k-0 gets 10, k-4 gets 50
		for j := 0; j < count; j++ {
			l.Allow(fmt.Sprintf("k-%d", i), p)
		}
	}

	snap := l.Snapshot(3)
	if len(snap.Keys) != 3 {
		t.Errorf("expected 3 keys (capped), got %d", len(snap.Keys))
	}
	if !snap.Truncated {
		t.Errorf("expected active_keys_truncated=true when capped (BR-29)")
	}
	// Hottest first.
	if snap.Keys[0].Count < snap.Keys[1].Count || snap.Keys[1].Count < snap.Keys[2].Count {
		t.Errorf("snapshot must be sorted by count DESC (BR-27), got %d %d %d",
			snap.Keys[0].Count, snap.Keys[1].Count, snap.Keys[2].Count)
	}
	// k-4 (50 requests) should be first.
	if snap.Keys[0].Key != "k-4" {
		t.Errorf("hottest key should be k-4, got %s", snap.Keys[0].Key)
	}
}

// TestLimiterSnapshotEmptyNotNull (BR-28) — an empty limiter returns an
// empty slice, NOT nil, so the status endpoint serializes `[]` not `null`.
func TestLimiterSnapshotEmptyNotNull(t *testing.T) {
	clock := fakeClock(time.Unix(1000, 0))
	l := newTestLimiter(t, clock, 0)
	snap := l.Snapshot(100)
	if snap.Keys == nil {
		t.Errorf("empty snapshot Keys must be [] not nil (BR-28)")
	}
	if len(snap.Keys) != 0 {
		t.Errorf("empty snapshot should have 0 keys, got %d", len(snap.Keys))
	}
	if snap.Truncated {
		t.Errorf("empty snapshot should not be truncated")
	}
}

// TestLimiterNoGoroutineLeak (REL-04, §2.3) — arming the limiter and running
// requests must not create goroutines. Verified by counting goroutines
// before/after and by the absence of the `go` keyword in the package (the
// real structural guarantee — see TestRatelimitPackageNoInternalDeps which
// also blocks importing anything that might spawn a coordinated goroutine).
func TestLimiterNoGoroutineLeak(t *testing.T) {
	clock := fakeClock(time.Unix(1000, 0))
	l := newTestLimiter(t, clock, 0)
	p := policy(100, 60*time.Second)

	before := runtime.NumGoroutine()
	for i := 0; i < 1000; i++ {
		l.Allow("k1", p)
	}
	// Force eviction sweep to run too.
	clock.Advance(2 * 60 * time.Second)
	for i := 0; i < sweepEvery; i++ {
		l.Allow("trigger", p)
	}
	after := runtime.NumGoroutine()
	// Allow a small delta for runtime-internal goroutines; the limiter must
	// add none. A jump >2 would indicate a leaked goroutine.
	if after-before > 2 {
		t.Errorf("goroutine count jumped %d → %d (limiter must not spawn goroutines, REL-04)", before, after)
	}
}

// BenchmarkRateLimitAllow measures the isolated Limiter.Allow hot path (O(1)
// two-bucket weighted sliding window) under parallel contention. This is the
// PERF-01/PERF-02/PERF-03 load-test surface — the limiter alone, without the
// middleware harness (no httptest recorder allocations). The v1 4.6 load test
// measured 206.7ns median, 0 allocs/op; v2 implements the identical algorithm,
// so the same margins are expected (performance-nfrs §1 PERF-01 carries the
// v1 baseline as the expected v2 result).
//
// v2 NOTE: v2's Allow takes (key, policy) — the policy is per-call (not set at
// New time as in v1). The bench passes a high-limit policy so the verdict is
// always allow (steady-state hot path, no deny branch).
func BenchmarkRateLimitAllow(b *testing.B) {
	l, _ := New(WithMaxTrackedKeys(DefaultMaxTrackedKeys))
	p := Policy{Limit: 1 << 20, Window: 60 * time.Second}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = l.Allow("ip|GET /x", p)
		}
	})
}