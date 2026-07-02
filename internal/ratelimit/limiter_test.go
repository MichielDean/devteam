package ratelimit

import (
	"sync"
	"testing"
	"time"
)

// fakeClock implements Clock for deterministic time advancement in tests (E14).
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(start time.Time) *fakeClock {
	return &fakeClock{now: start}
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

func TestLimiterAllowsWithinLimit(t *testing.T) {
	clk := newFakeClock(time.Unix(1_000_000, 0))
	l, err := New(Policy{Limit: 10, Window: 60 * time.Second}, WithClock(clk))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for i := 0; i < 9; i++ {
		v, err := l.Allow("ip|GET /x")
		if err != nil {
			t.Fatalf("Allow %d: %v", i, err)
		}
		if !v.Allow {
			t.Fatalf("request %d: expected allow, got deny", i)
		}
	}
	// 9th allow: count should be 9 (9 admitted, counter incremented on each allow).
	v, _ := l.Allow("ip|GET /x")
	if !v.Allow {
		t.Fatalf("10th request: expected allow, got deny")
	}
	if v.Count != 10 {
		t.Errorf("after 10 allows, count = %d, want 10", v.Count)
	}
}

func TestLimiterDeniesOverLimit(t *testing.T) {
	clk := newFakeClock(time.Unix(1_000_000, 0))
	l, _ := New(Policy{Limit: 10, Window: 60 * time.Second}, WithClock(clk))
	for i := 0; i < 10; i++ {
		l.Allow("ip|GET /x")
	}
	// 11th request: deny. Counter increments on deny (invariant 6, BR-11/BR-14):
	// count == limit+1 == 11 on rejection. (Reversal correction #7 — the 2.7
	// "counter stays at 10" assertion is REVERSED behavior.)
	v, err := l.Allow("ip|GET /x")
	if err != nil {
		t.Fatalf("11th Allow err: %v", err)
	}
	if v.Allow {
		t.Fatal("11th request: expected deny, got allow")
	}
	if !v.OverLimit {
		t.Error("11th request: expected OverLimit=true")
	}
	if v.Count != 11 {
		t.Errorf("11th request count = %d, want 11 (limit+1, counter increments on deny)", v.Count)
	}
}

func TestLimiterWindowSlides(t *testing.T) {
	clk := newFakeClock(time.Unix(1_000_000, 0))
	l, _ := New(Policy{Limit: 5, Window: 60 * time.Second}, WithClock(clk))
	for i := 0; i < 5; i++ {
		l.Allow("ip|GET /x")
	}
	// At limit. Advance past 2*window → the prior bucket fully decays and old
	// requests expire (the weighted count drops to 0).
	clk.Advance(2*60*time.Second + 1*time.Second)
	v, err := l.Allow("ip|GET /x")
	if err != nil {
		t.Fatalf("Allow after window slide: %v", err)
	}
	if !v.Allow {
		t.Fatal("after window slide: expected allow, got deny (old requests should expire)")
	}
}

func TestLimiterCompositeKeyIndependent(t *testing.T) {
	clk := newFakeClock(time.Unix(1_000_000, 0))
	l, _ := New(Policy{Limit: 1, Window: 60 * time.Second}, WithClock(clk))
	v1, _ := l.Allow("1.2.3.4|POST /v1/run")
	if !v1.Allow {
		t.Fatal("first key: expected allow")
	}
	// Same IP, different endpoint → independent counter (BR-16).
	v2, _ := l.Allow("1.2.3.4|GET /status")
	if !v2.Allow {
		t.Fatal("different endpoint same IP: expected allow (independent counter)")
	}
	// First key now at limit → deny.
	v3, _ := l.Allow("1.2.3.4|POST /v1/run")
	if v3.Allow {
		t.Fatal("first key at limit: expected deny")
	}
}

func TestLimiterEvictsExpiredKeys(t *testing.T) {
	clk := newFakeClock(time.Unix(1_000_000, 0))
	l, _ := New(Policy{Limit: 100, Window: 60 * time.Second}, WithClock(clk))
	// Add one key, then advance past 2*window so it's expired.
	l.Allow("expired|GET /x")
	clk.Advance(2*60*time.Second + 1*time.Second)
	// Trigger a sweep (every 64th Allow). Issue 63 more calls on a different key,
	// then the 64th triggers the sweep that evicts the expired key.
	for i := 0; i < 64; i++ {
		l.Allow("live|GET /y")
	}
	if n := l.NumKeys(); n != 1 {
		t.Errorf("after sweep, NumKeys = %d, want 1 (expired key evicted)", n)
	}
}

func TestLimiterEvictsAtCap(t *testing.T) {
	clk := newFakeClock(time.Unix(1_000_000, 0))
	// Small cap to make the test fast; floor is 100.
	l, _ := New(Policy{Limit: 1000, Window: 60 * time.Second}, WithClock(clk), WithMaxTrackedKeys(100))
	// Fill past the cap with distinct keys, all in-window.
	for i := 0; i < 150; i++ {
		l.Allow(stringKey(i))
	}
	// Trigger a sweep.
	for i := 0; i < 64; i++ {
		l.Allow("trigger|GET /t")
	}
	if n := l.NumKeys(); n > 100 {
		t.Errorf("after cap sweep, NumKeys = %d, want <= 100", n)
	}
	if n := l.NumKeys(); n > 90+1 { // 90% target + the trigger key
		// The cap sweep targets 90% of maxTracked. Allow a small tolerance for the
		// trigger key that's also in the map. The strict invariant is <= maxTracked.
		t.Logf("NumKeys = %d (target ~90 + trigger key)", n)
	}
}

func stringKey(i int) string {
	// Cheap unique key without fmt to avoid import churn in the hot loop.
	b := make([]byte, 0, 20)
	b = append(b, 'k')
	for j := i; j > 0; j /= 10 {
		b = append(b, byte('0'+j%10))
	}
	b = append(b, '|', 'G', 'E', 'T', ' ', '/', 'x')
	return string(b)
}

func TestLimiterConcurrentSafe(t *testing.T) {
	// R9: go test -race is the gate. This test runs many parallel Allow calls on
	// the same key; a race detector failure means the single mutex is wrong.
	l, _ := New(Policy{Limit: 1 << 20, Window: 60 * time.Second})
	var wg sync.WaitGroup
	const goroutines, perG = 16, 200
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				_, _ = l.Allow("shared|GET /x")
			}
		}()
	}
	wg.Wait()
	// No assertion on count — concurrency safety is the point. The race detector
	// (run with -race) is the gate.
}

func TestLimiterConcurrentNoLostUpdates(t *testing.T) {
	clk := newFakeClock(time.Unix(1_000_000, 0))
	l, _ := New(Policy{Limit: 1 << 24, Window: 60 * time.Second}, WithClock(clk))
	const goroutines, perG = 8, 500
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				_, _ = l.Allow("shared|GET /x")
			}
		}()
	}
	wg.Wait()
	// All goroutines incremented the same key's current bucket within one window.
	// The counter must equal goroutines*perG (no lost updates, BR-44).
	v, _ := l.Allow("noop|GET /noop")
	_ = v
	// Read the entry's currBucket under the lock by issuing no-op snapshot... use
	// Snapshot which recomputes from prev/curr buckets. Within the same window,
	// prevBucket=0 so Snapshot count == currBucket == total.
	snap := l.Snapshot(10)
	var found *KeyState
	for i := range snap.Keys {
		if snap.Keys[i].Key == "shared|GET /x" {
			found = &snap.Keys[i]
			break
		}
	}
	if found == nil {
		t.Fatal("shared key not in snapshot")
	}
	want := int64(goroutines * perG)
	if found.Count != want {
		t.Errorf("concurrent count = %d, want %d (lost updates)", found.Count, want)
	}
}

func TestLimiterMalfunctionFailsOpen(t *testing.T) {
	// US6/BR-50: an injected internal error → the limiter fails open (allow +
	// non-nil err). We exercise the fail-open path by constructing a Limiter with
	// a clock that panics on the second Now() call, simulating an internal error
	// mid-evaluation. Allow's own defer/recover converts the panic to a fail-open
	// verdict (Allow=true + non-nil err). The middleware-level test asserts next
	// is called; here we assert the fail-open contract at the limiter level.
	clk := &panickingClock{base: newFakeClock(time.Unix(1_000_000, 0))}
	l, _ := New(Policy{Limit: 10, Window: 60 * time.Second}, WithClock(clk))
	clk.panicNext = true
	v, err := l.Allow("ip|GET /x")
	if !v.Allow {
		t.Error("malfunction: expected Allow=true (fail-open), got false")
	}
	if err == nil {
		t.Error("malfunction: expected non-nil err (fail-open signal)")
	}
	// The middleware owns the failures_total increment (BR-50). At the limiter
	// level, Allow returns the fail-open verdict but does NOT increment
	// failures_total itself (the middleware does, to avoid double-counting).
	// So FailuresTotal is 0 here; the middleware-level test asserts the +1.
	if got := l.FailuresTotal(); got != 0 {
		t.Errorf("limiter-level: FailuresTotal = %d, want 0 (middleware owns the increment)", got)
	}
	// A subsequent normal call (clock reset) must still work — malfunction is
	// per-request, not sticky (BR-53).
	clk.panicNext = false
	v2, err2 := l.Allow("ip|GET /x")
	if err2 != nil {
		t.Fatalf("normal Allow after malfunction: err = %v", err2)
	}
	if !v2.Allow {
		t.Error("normal Allow after malfunction: expected allow (limiter recovered)")
	}
}

type panickingClock struct {
	base      *fakeClock
	panicNext bool // when true, the next Now() call panics
}

func (p *panickingClock) Now() time.Time {
	if p.panicNext {
		p.panicNext = false
		panic("simulated internal error mid-evaluation")
	}
	return p.base.Now()
}

func TestLimiterSnapshotSortedAndBounded(t *testing.T) {
	clk := newFakeClock(time.Unix(1_000_000, 0))
	l, _ := New(Policy{Limit: 1000, Window: 60 * time.Second}, WithClock(clk))
	// Create keys with distinct counts.
	for i := 0; i < 5; i++ {
		for j := 0; j <= i; j++ {
			l.Allow(stringKey(i))
		}
	}
	snap := l.Snapshot(3)
	if len(snap.Keys) != 3 {
		t.Fatalf("Snapshot len = %d, want 3 (bounded)", len(snap.Keys))
	}
	// Sorted by count descending (hottest first).
	for i := 1; i < len(snap.Keys); i++ {
		if snap.Keys[i].Count > snap.Keys[i-1].Count {
			t.Errorf("Snapshot not sorted desc: Keys[%d].Count=%d > Keys[%d].Count=%d",
				i, snap.Keys[i].Count, i-1, snap.Keys[i-1].Count)
		}
	}
}

func TestLimiterSnapshotEmptyIsNotNullSlice(t *testing.T) {
	l, _ := New(Policy{Limit: 10, Window: 60 * time.Second})
	snap := l.Snapshot(10)
	if snap.Keys == nil {
		t.Fatal("empty Snapshot.Keys is nil, want [] (BR-28)")
	}
	if len(snap.Keys) != 0 {
		t.Errorf("empty Snapshot.Keys len = %d, want 0", len(snap.Keys))
	}
}

func TestNewRejectsBadConfig(t *testing.T) {
	if _, err := New(Policy{Limit: 10, Window: 60 * time.Second}, WithMaxTrackedKeys(50)); err == nil {
		t.Error("New with maxTrackedKeys=50: expected error (hard floor 100)")
	}
	if _, err := New(Policy{Limit: 0, Window: 60 * time.Second}); err == nil {
		t.Error("New with Limit=0: expected error")
	}
	if _, err := New(Policy{Limit: 10, Window: 0}); err == nil {
		t.Error("New with Window=0: expected error")
	}
}

func TestLimiterRecordRejectionAndFailure(t *testing.T) {
	l, _ := New(Policy{Limit: 10, Window: 60 * time.Second})
	l.RecordRejection()
	l.RecordRejection()
	if got := l.RejectionsTotal(); got != 2 {
		t.Errorf("RejectionsTotal = %d, want 2", got)
	}
	l.RecordFailure()
	if got := l.FailuresTotal(); got != 1 {
		t.Errorf("FailuresTotal = %d, want 1", got)
	}
}

func TestLimiterFailuresTotalNotReset(t *testing.T) {
	// BR-49: failures_total is process-lifetime cumulative; not decremented when
	// corruption clears. A subsequent normal request does not reset it.
	l, _ := New(Policy{Limit: 10, Window: 60 * time.Second})
	l.RecordFailure()
	// Normal request does not touch failures_total.
	_, _ = l.Allow("ip|GET /x")
	if got := l.FailuresTotal(); got != 1 {
		t.Errorf("after normal request, FailuresTotal = %d, want 1 (not reset)", got)
	}
}

func BenchmarkRateLimitAllow(b *testing.B) {
	l, _ := New(Policy{Limit: 1 << 20, Window: 60 * time.Second})
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = l.Allow("ip|GET /x")
		}
	})
}