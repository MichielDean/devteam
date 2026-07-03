package ratelimit

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultMaxTrackedKeys is the bound on the in-memory key map when no
// max_tracked_keys is configured (BLM §3.4 — default 10000, ~640KB-1.28MB).
const DefaultMaxTrackedKeys = 10000

// minMaxTrackedKeys is the hard sanity floor enforced by config validation
// (BR-06). The Limiter itself does not re-validate; it trusts the config.
const minMaxTrackedKeys = 100

// sweepEvery is the amortization interval for eviction (BLM §3.4 — a sweep
// runs on every 64th Allow call rather than on a background goroutine, so
// there is no shutdown coordination or goroutine leak to manage).
const sweepEvery = 64

// entry is the per-key two-bucket sliding-window state (BLM §1.1, §2.6).
// It is not exported; only the Limiter touches it.
//
// The limiter remembers the last Policy used for each key so the status
// endpoint can report per-key limit/window without a separate lookup. This
// adds two ints per entry — negligible against the ~80B v1 baseline
// (PERF-03 <128B Should-target still holds).
type entry struct {
	prevBucket      int64     // weighted count carried over from the previous window
	currBucket      int64     // requests in the current window so far
	currBucketStart time.Time // the start of the current window
	lastSeen        time.Time // last time Allow touched this key (for LRU eviction)
	limit           int       // last policy limit applied to this key
	windowSeconds   int       // last policy window (seconds) applied to this key
}

// Limiter is the aggregate root (BLM §1.2). No code outside this package
// touches the counter map. The only mutation entry point is Allow; the only
// observation entry point is Snapshot. RejectionsTotal/FailuresTotal are
// atomic counters observed on the status path without taking the mutex
// (BR-44 — go test -race is the gate).
//
// Limiter imports ONLY the Go standard library (BR-60, SEC-14, REL-15). The
// package boundary is the reversibility seam: swap the algorithm = swap one
// package's internals, with no edit to internal/api or internal/config.
type Limiter struct {
	clock Clock

	mu   sync.Mutex // guards keys (F-12 idiom — single mutex, like sseMu)
	keys map[string]*entry

	// maxTracked caps the in-memory key map (BLM §3.4, SEC-07/REL-12).
	maxTracked int

	// callCount is incremented on every Allow; a sweep runs on every
	// sweepEvery-th call. Atomic to avoid taking the mutex just to bump it.
	callCount atomic.Int64

	// Cumulative counters (BR-49 — process-lifetime, monotonic, NOT resettable).
	rejectionsTotal atomic.Int64
	failuresTotal    atomic.Int64
}

// Option configures a Limiter at construction (BLM §1.1 — functional value).
type Option func(*Limiter)

// WithMaxTrackedKeys overrides the default key cap (BLM §3.4). The caller
// (ConfigureRateLimiting) is responsible for validating the value via
// RateLimitConfig.Validate() before passing it here; the limiter clamps to
// the hard floor to defend against an unvalidated caller in tests.
func WithMaxTrackedKeys(n int) Option {
	return func(l *Limiter) {
		if n < minMaxTrackedKeys {
			n = minMaxTrackedKeys
		}
		l.maxTracked = n
	}
}

// WithClock injects a Clock (BLM §2.6 — fake clock for deterministic tests;
// wall clock in production). This is the ONLY seam for time in the package.
func WithClock(c Clock) Option {
	return func(l *Limiter) { l.clock = c }
}

// New constructs a Limiter. It returns an error only if the options produce
// an inconsistent state — in practice the config is validated upstream by
// RateLimitConfig.Validate(), so the arming path treats a non-nil error as
// "log + run without the limiter" (BLM §3.6, §2.5).
//
// New never panics. A nil Clock defaults to the wall clock.
func New(opts ...Option) (*Limiter, error) {
	l := &Limiter{
		clock:       wallClock{},
		maxTracked:  DefaultMaxTrackedKeys,
		keys:        make(map[string]*entry),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(l)
		}
	}
	if l.clock == nil {
		l.clock = wallClock{}
	}
	if l.maxTracked < minMaxTrackedKeys {
		return nil, fmt.Errorf("ratelimit: max_tracked_keys %d below hard floor %d", l.maxTracked, minMaxTrackedKeys)
	}
	return l, nil
}

// Allow is the single mutation entry point (BLM §1.2, §2.1, §2.14). It:
//
//  1. Increments the per-key window counter (BR-11 — increments on allow AND
//     on deny; NOT on exempt and NOT on malfunction, because the exempt and
//     malfunction paths never call Allow).
//  2. Computes the weighted sliding-window count (BLM §2.6 — two-bucket).
//  3. Emits a Verdict. On a clean allow: {Allow:true, OverLimit:false}. On a
//     clean deny: {Allow:false, OverLimit:true} with Count == Limit+1 (BR-14
//     — the over-limit request counts; counter-on-deny is LOCKED).
//  4. On any panic or internal error: returns a fail-open Verdict
//     {Allow:true, OverLimit:false, Exempt:false} with a non-nil err
//     (§2.1 — the ONLY error-path return shape; no fail-closed branch
//     exists in v2, BR-52).
//
// Allow is safe for concurrent use (REL-06 — single mutex serializes the
// map; -race is the gate). The hot path is O(1) map lookup + mutex + compare
// (PERF-01). Eviction (§2.3) is amortized every sweepEvery-th call under the
// same mutex; no background goroutine (REL-04 — structurally verifiable by
// the absence of any `go` keyword in this package).
func (l *Limiter) Allow(key string, policy Policy) (v Verdict) {
	// Defer/recover catches any panic in the window math and routes it to
	// the fail-open path (§2.1 / §2.2 inner layer). The middleware has its
	// own outer defer/recover for panics OUTSIDE Allow (key extraction etc.).
	// This is the ONLY error-path return shape — a developer cannot
	// accidentally add a fail-closed branch without editing this defer.
	defer func() {
		if r := recover(); r != nil {
			l.failuresTotal.Add(1)
			v = Verdict{
				Allow:     true,
				OverLimit: false,
				Exempt:    false,
				Key:       key,
				Err:       fmt.Errorf("ratelimit: internal error: %v", r),
			}
		}
	}()

	now := l.clock.Now()
	window := policy.Window
	if window <= 0 {
		window = 60 * time.Second
	}
	windowSeconds := int(window / time.Second)

	// The locked critical section is wrapped in a closure so a deferred
	// Unlock releases the mutex even if the window math panics (the recover
	// at the top of Allow would catch it, but without releasing the mutex
	// every subsequent Allow would deadlock). The closure also keeps the
	// mutex held for the shortest possible scope — evict (below) takes its
	// own lock and must NOT be called while this lock is held (Mutex is
	// non-reentrant).
	var elapsedInWindow time.Duration
	var allow bool
	var postCount int64
	func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		e, ok := l.keys[key]
		if !ok {
			e = &entry{}
			l.keys[key] = e
		}

		// Rollover the buckets if the current window has elapsed (BLM §2.6).
		if !e.currBucketStart.IsZero() && now.Sub(e.currBucketStart) >= window {
			elapsed := now.Sub(e.currBucketStart)
			fullWindows := int(elapsed / window)
			if fullWindows >= 2 {
				// Both buckets are entirely stale; reset.
				e.prevBucket = 0
				e.currBucket = 0
			} else {
				e.prevBucket = e.currBucket
				e.currBucket = 0
			}
			// Anchor the new bucket to the window boundary just passed.
			e.currBucketStart = e.currBucketStart.Add(time.Duration(fullWindows) * window)
		}
		if e.currBucketStart.IsZero() {
			e.currBucketStart = now
		}

		// Weighted count at `now` (BLM §2.6):
		//   count = prevBucket * (1 - (now - currBucketStart)/window) + currBucket
		elapsedInWindow = now.Sub(e.currBucketStart)
		var decay float64
		if window > 0 {
			decay = float64(elapsedInWindow) / float64(window)
			if decay < 0 {
				decay = 0
			}
			if decay > 1 {
				decay = 1
			}
		}
		weighted := float64(e.prevBucket)*(1-decay) + float64(e.currBucket)

		// BR-14 / §2.14 — the verdict compares the PRE-increment weighted
		// count (demand so far), THEN increments the counter so the over-limit
		// request is counted (counter == limit+1 on deny, matching M4.1:
		// count=301 at limit=300). Verdict.Count is POST-increment.
		preCount := int64(weighted)
		if weighted > float64(preCount) {
			preCount++ // round up a fractional count just over the limit
		}
		allow = preCount < int64(policy.Limit)

		// Increment the counter on allow AND on deny (BR-11 — the over-limit
		// request counts; the counter tracks *demand*, not just admitted traffic).
		e.currBucket++
		weighted++
		postCount = int64(weighted)
		if weighted > float64(postCount) {
			postCount++
		}

		e.lastSeen = now
		e.limit = policy.Limit
		e.windowSeconds = windowSeconds
	}()

	v = Verdict{
		Key:    key,
		Count:  int(postCount),
		Limit:  policy.Limit,
		Window: window,
	}
	if allow {
		v.Allow = true
	} else {
		v.Allow = false
		v.OverLimit = true
	}
	// ResetIn: time until the oldest in-window request expires. With the
	// two-bucket approximation this is the time until the current window
	// boundary (the previous bucket's contribution reaches zero one full
	// window after currBucketStart).
	v.ResetIn = window - elapsedInWindow
	if v.ResetIn < 0 {
		v.ResetIn = 0
	}

	// Amortized eviction (§2.3) — every sweepEvery-th call, under the mutex.
	// Safe to call now: the critical-section lock was released by the closure.
	if l.callCount.Add(1)%sweepEvery == 0 {
		l.evict(now, window)
	}

	return v
}

// evict removes expired keys (lastSeen older than 2*window) and, if the map
// exceeds the cap, drops the oldest by lastSeen down to 90% of the cap. All
// under the existing mutex; no goroutine (REL-04). The cost is O(keys) but
// amortized over sweepEvery requests, so no single request pays a >10×
// spike (PERF-07 — v1 measured 761ns, 3.7× ratio).
func (l *Limiter) evict(now time.Time, window time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	expiryAge := 2 * window
	for k, e := range l.keys {
		if now.Sub(e.lastSeen) > expiryAge {
			delete(l.keys, k)
		}
	}

	if len(l.keys) > l.maxTracked {
		// Evict oldest by lastSeen down to 90% headroom (BLM §3.4).
		type kv struct {
			k string
			t time.Time
		}
		items := make([]kv, 0, len(l.keys))
		for k, e := range l.keys {
			items = append(items, kv{k, e.lastSeen})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].t.Before(items[j].t) })
		target := int(float64(l.maxTracked) * 0.9)
		if target < 1 {
			target = 1
		}
		drop := len(items) - target
		for i := 0; i < drop && i < len(items); i++ {
			delete(l.keys, items[i].k)
		}
	}
}

// Snapshot returns a bounded, sorted view of the limiter (BLM §3.2, §2.12).
// Keys are sorted by Count DESCENDING (hottest first) and capped at maxKeys;
// Truncated is true when the cap was hit (O-2 — a BOOL field, NOT a synthetic
// entry). The returned Keys slice is never nil (BR-28 — len()==0 for the
// empty case). RejectionsTotal/FailuresTotal are read atomically without
// taking the mutex.
//
// The per-key Limit/WindowSeconds reflect the last policy the limiter
// applied to that key (stored on the entry). Count is the sum of the two
// buckets (a close approximation of the weighted count; the exact weighted
// value is what Allow compared, but the status endpoint is a diagnostic
// view, not an accounting record). ResetInSeconds is the time until the
// current window boundary for that key.
//
// Snapshot does NOT increment any counter; it is read-only. It does NOT
// call Allow; the status endpoint is exempt by route (BR-09/BR-26).
func (l *Limiter) Snapshot(maxKeys int) Snapshot {
	if maxKeys < 0 {
		maxKeys = 0
	}
	l.mu.Lock()
	now := l.clock.Now()
	states := make([]KeyState, 0, len(l.keys))
	for k, e := range l.keys {
		window := time.Duration(e.windowSeconds) * time.Second
		if window <= 0 {
			window = 60 * time.Second
		}
		resetIn := e.currBucketStart.Add(window).Sub(now)
		if resetIn < 0 {
			resetIn = 0
		}
		states = append(states, KeyState{
			Key:            k,
			Count:          int(e.currBucket) + int(e.prevBucket),
			Limit:          e.limit,
			WindowSeconds:   e.windowSeconds,
			ResetInSeconds:  int(resetIn.Round(time.Second) / time.Second),
		})
	}
	l.mu.Unlock()

	sort.Slice(states, func(i, j int) bool { return states[i].Count > states[j].Count })

	truncated := false
	if len(states) > maxKeys {
		states = states[:maxKeys]
		truncated = true
	}
	return Snapshot{
		Keys:            states,
		Truncated:       truncated,
		RejectionsTotal:  l.rejectionsTotal.Load(),
		FailuresTotal:   l.failuresTotal.Load(),
	}
}

// RejectionsTotal returns the process-lifetime cumulative 429 count (BR-49).
// Read atomically; no mutex.
func (l *Limiter) RejectionsTotal() int64 { return l.rejectionsTotal.Load() }

// FailuresTotal returns the process-lifetime cumulative limiter-internal-error
// count (BR-49 — monotonic; never decremented; restart to reset).
func (l *Limiter) FailuresTotal() int64 { return l.failuresTotal.Load() }

// RecordRejection increments the rejection counter. The middleware calls
// this on a clean 429 (deny + dry_run=false). NOT called on dry-run rejects
// (BR-42 — dry_run never rejects) and NOT on malfunctions (BR-50 — those
// increment failures_total instead).
func (l *Limiter) RecordRejection() { l.rejectionsTotal.Add(1) }

// RecordFailure increments the malfunction counter. The middleware calls
// this on the fail-open path (§2.1). Exposed so the middleware can record a
// malfunction that occurs OUTSIDE Allow (e.g., during key extraction) without
// having to fabricate a Verdict.
func (l *Limiter) RecordFailure() { l.failuresTotal.Add(1) }

// Len returns the number of tracked keys. For tests and the status endpoint.
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.keys)
}