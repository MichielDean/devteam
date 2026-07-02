package ratelimit

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Clock is the time source used by a Limiter. The zero-value Limiter uses the
// wall clock; tests inject a fake via WithClock for deterministic time
// advancement (E14 — no real wall clock in unit tests).
type Clock interface {
	Now() time.Time
}

// wallClock implements Clock using time.Now.
type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

// Policy is the immutable per-verdict rate policy: a limit and a window. The
// Exempt flag is part of the policy value object but is enforced structurally in
// the middleware (route-match exemption before Allow, BR-09); an exempt policy
// is never passed to Allow in the MVP wiring.
type Policy struct {
	Limit  int
	Window time.Duration
	Exempt bool
}

// Verdict is the outcome of one Allow call. On a clean allow or deny, err is nil
// and the fields describe the matched policy. On malfunction (D1/ADR-002), Allow
// returns Verdict{Allow:true, OverLimit:false, Exempt:false} and a non-nil err —
// the verdict is final (allow), the error is the signal. A clean deny has
// OverLimit=true, err=nil; a malfunction has Allow=true, err!=nil. Never both.
type Verdict struct {
	Allow     bool
	Key       string
	Count     int64
	Limit     int
	Window    time.Duration
	ResetIn   time.Duration
	OverLimit bool
	Exempt    bool
}

// KeyState is one row of Snapshot.Keys.
type KeyState struct {
	Key            string
	Count          int64
	Limit          int
	WindowSeconds  int
	ResetInSeconds int
	LastSeen       time.Time
}

// Snapshot is a point-in-time view of the limiter for the status endpoint. Keys
// is sorted by Count DESCENDING (hottest first) and bounded by the maxKeys
// argument to Snapshot. Keys is never nil (it is initialized to an empty
// slice so it marshals to [] not null — BR-28). TotalKeys is the total number of
// tracked keys BEFORE truncation, observed atomically under the same lock as
// Keys — the status handler uses it to compute active_keys_truncated without a
// TOCTOU window (BR-29/O-2).
type Snapshot struct {
	Keys            []KeyState
	TotalKeys       int
	RejectionsTotal int64
	FailuresTotal   int64
}

// entry is the per-key two-bucket window state. It is NOT exported; it lives
// inside Limiter.keys. Kept small for cache efficiency (PERF-03 target ≤128B).
type entry struct {
	prevBucket       int64
	currBucket       int64
	currBucketStart  time.Time
	lastSeen         time.Time
	limit            int
	window           time.Duration
}

// Limiter is the aggregate root that owns all counter state, policy
// resolution, verdict emission, and snapshots. The zero value is NOT usable;
// use New. The only mutation entry point is Allow; the only observation entry
// point is Snapshot. No code outside this package touches the counter map
// (business-logic-model §1.2).
type Limiter struct {
	mu              sync.Mutex
	clock           Clock
	keys            map[string]*entry
	defaults        Policy
	maxTracked      int
	allowCount      int64 // incremented on every Allow call; sweep on every 64th
	rejectionsTotal atomic.Int64
	failuresTotal    atomic.Int64
}

// Option configures a Limiter at construction time.
type Option func(*limiterConfig)

type limiterConfig struct {
	maxTracked int
	clock      Clock
}

// WithMaxTrackedKeys sets the maximum number of tracked keys (default 10000,
// hard floor 100, BR-06). When the map exceeds the cap, the oldest keys by
// lastSeen are evicted to maxTrackedKeys * 0.9 (10% headroom to avoid thrash).
func WithMaxTrackedKeys(n int) Option {
	return func(c *limiterConfig) { c.maxTracked = n }
}

// WithClock injects a time source for deterministic time advancement in tests.
// The zero-value Limiter uses the wall clock.
func WithClock(clk Clock) Option {
	return func(c *limiterConfig) { c.clock = clk }
}

// DefaultMaxTrackedKeys is the LOCKED default cap on tracked keys (O-7).
const DefaultMaxTrackedKeys = 10000

// New constructs a Limiter with the given default policy and options. The
// overrides map (post-MVP, US8) is accepted here for forward-compat but is not
// used by the MVP middleware; the MVP uses defaults only.
func New(defaults Policy, opts ...Option) (*Limiter, error) {
	cfg := limiterConfig{
		maxTracked: DefaultMaxTrackedKeys,
		clock:      wallClock{},
	}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.maxTracked < 100 {
		return nil, fmt.Errorf("ratelimit: max_tracked_keys must be >= 100, got %d", cfg.maxTracked)
	}
	if defaults.Window <= 0 {
		return nil, fmt.Errorf("ratelimit: defaults.Window must be > 0, got %s", defaults.Window)
	}
	if defaults.Limit <= 0 {
		return nil, fmt.Errorf("ratelimit: defaults.Limit must be > 0, got %d", defaults.Limit)
	}
	return &Limiter{
		clock:      cfg.clock,
		keys:       make(map[string]*entry),
		defaults:   defaults,
		maxTracked: cfg.maxTracked,
	}, nil
}

// sweepInterval is the LOCKED amortization interval (every 64th Allow call).
const sweepInterval = 64

// Allow adjudicates one request against the limiter. On a clean allow it
// returns Verdict{Allow:true,...} with err nil and increments the key's count.
// On a clean deny it returns Verdict{Allow:false, OverLimit:true,...} with err
// nil and STILL increments the count (invariant 6, BR-11 — the counter tracks
// demand, so the over-limit request is counted: count == limit+1 on rejection).
// On malfunction it returns Verdict{Allow:true, OverLimit:false, Exempt:false}
// and a non-nil err (D1/ADR-002/BR-50 — fail-open); the count is NOT incremented
// (count is indeterminate mid-error, invariant 6).
func (l *Limiter) Allow(key string) (verdict Verdict, err error) {
	// Two-layer recovery (NDP-02): the middleware has its own defer/recover, but
	// Allow also guards against internal panics so a corrupt entry can't crash it.
	// On malfunction we fail open (D1/ADR-002/BR-50): return an allow verdict + a
	// non-nil err so the middleware can log + increment failures_total. The count
	// is NOT incremented (invariant 6 — count is indeterminate mid-error).
	defer func() {
		if r := recover(); r != nil {
			// fail-open (NDP-01): count not incremented, verdict is allow + err.
			// The middleware owns the failures_total increment (BR-50) — we do NOT
			// increment here to avoid double-counting when the middleware's err
			// path also fires. The err we return is the signal the middleware logs
			// and counts.
			verdict = Verdict{Allow: true, Key: key}
			err = fmt.Errorf("ratelimit: internal error: %v", r)
		}
	}()

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	policy := l.defaults

	e, ok := l.keys[key]
	if !ok {
		e = &entry{
			limit:           policy.Limit,
			window:          policy.Window,
			currBucketStart: now,
		}
		l.keys[key] = e
	}

	// Rollover if the window has elapsed since the current bucket started.
	window := e.window
	if window <= 0 {
		window = policy.Window
	}
	elapsed := now.Sub(e.currBucketStart)
	if elapsed >= window {
		e.prevBucket = e.currBucket
		e.currBucket = 0
		e.currBucketStart = e.currBucketStart.Add(window)
		// If a long time has passed, keep advancing the bucket boundary until it
		// catches up to now. This prevents a single rollover from carrying
		// stale prevBucket weight across many windows.
		for now.Sub(e.currBucketStart) >= window {
			e.prevBucket = 0
			e.currBucketStart = e.currBucketStart.Add(window)
		}
		elapsed = now.Sub(e.currBucketStart)
	}

	// Weighted sliding-window count (D5).
	count := e.currBucket
	if elapsed > 0 && elapsed < window {
		weight := float64(e.prevBucket) * (1.0 - float64(elapsed)/float64(window))
		// Round weight to nearest int (prevBucket is an int64 count); avoid float
		// drift by adding 0.5 and truncating.
		count += int64(weight + 0.5)
	} else if elapsed >= window {
		// prevBucket is fully expired (boundary case).
		count = e.currBucket
	}

	limit := e.limit
	allow := count < int64(limit)

	// Invariant 6 (BR-11): increment on allow AND on deny, NOT on malfunction.
	// The over-limit request counts (count == limit+1 on rejection, M4.1).
	e.currBucket++
	e.lastSeen = now

	l.allowCount++
	if l.allowCount%sweepInterval == 0 {
		l.sweepLocked(now)
	}

	resetIn := window - elapsed
	if resetIn < 0 {
		resetIn = 0
	}

	return Verdict{
		Allow:     allow,
		Key:       key,
		Count:     count + 1, // reflect this request's increment for the caller
		Limit:     limit,
		Window:    window,
		ResetIn:   resetIn,
		OverLimit: !allow,
	}, nil
}

// sweepLocked evicts keys whose lastSeen is older than 2 * window, and if the
// map exceeds maxTracked, evicts oldest by lastSeen to maxTracked * 0.9 (10%
// headroom to avoid thrash). Must be called with l.mu held.
func (l *Limiter) sweepLocked(now time.Time) {
	// Expire keys older than 2 * window. Collect first, delete second to avoid
	// map iteration/mutation ordering pitfalls.
	expired := make([]string, 0)
	for k, e := range l.keys {
		if now.Sub(e.lastSeen) >= 2*e.window {
			expired = append(expired, k)
		}
	}
	for _, k := range expired {
		delete(l.keys, k)
	}
	// Cap enforcement: evict oldest by lastSeen to 90% of maxTracked.
	if len(l.keys) > l.maxTracked {
		target := int(float64(l.maxTracked) * 0.9)
		type kv struct {
			key     string
			lastSeen time.Time
		}
		all := make([]kv, 0, len(l.keys))
		for k, e := range l.keys {
			all = append(all, kv{k, e.lastSeen})
		}
		sort.Slice(all, func(i, j int) bool {
			return all[i].lastSeen.Before(all[j].lastSeen)
		})
		// Evict the oldest (lowest lastSeen) until we reach the target.
		toEvict := len(l.keys) - target
		for i := 0; i < toEvict && i < len(all); i++ {
			delete(l.keys, all[i].key)
		}
	}
}

// Snapshot returns a bounded point-in-time view of the limiter state for the
// status endpoint. Keys is sorted by Count DESCENDING (hottest first) and
// truncated at maxKeys. Keys is never nil (initialized to an empty slice so it
// marshals to [] not null — BR-28). RejectionsTotal and FailuresTotal are the
// process-lifetime cumulative counters (atomic, observed without the mutex).
func (l *Limiter) Snapshot(maxKeys int) Snapshot {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	states := make([]KeyState, 0, len(l.keys))
	for k, e := range l.keys {
		window := e.window
		elapsed := now.Sub(e.currBucketStart)
		if elapsed >= window {
			elapsed = window
		}
		resetIn := window - elapsed
		if resetIn < 0 {
			resetIn = 0
		}
		// Recompute the weighted count for display (matches Allow's calculation).
		count := e.currBucket
		if elapsed > 0 && elapsed < window {
			weight := float64(e.prevBucket) * (1.0 - float64(elapsed)/float64(window))
			count += int64(weight + 0.5)
		}
		states = append(states, KeyState{
			Key:            k,
			Count:          count,
			Limit:          e.limit,
			WindowSeconds:  int(window.Seconds()),
			ResetInSeconds: int(resetIn.Seconds()),
			LastSeen:       e.lastSeen,
		})
	}

	// Sort by Count DESCENDING (hottest first).
	sort.Slice(states, func(i, j int) bool {
		if states[i].Count != states[j].Count {
			return states[i].Count > states[j].Count
		}
		// Stable tiebreak: by key ascending for deterministic output.
		return states[i].Key < states[j].Key
	})

	// Cap at maxKeys. A negative maxKeys disables the cap. TotalKeys is the
	// pre-truncation count, observed atomically under the same lock as Keys so
	// the status handler can compute active_keys_truncated without a TOCTOU
	// window (BR-29/O-2 — truncated iff TotalKeys > maxKeys, NOT len(Keys) >=
	// maxKeys, which false-positives when exactly maxKeys exist).
	totalKeys := len(states)
	if maxKeys >= 0 && len(states) > maxKeys {
		states = states[:maxKeys]
	}

	return Snapshot{
		Keys:            states,
		TotalKeys:       totalKeys,
		RejectionsTotal: l.rejectionsTotal.Load(),
		FailuresTotal:   l.failuresTotal.Load(),
	}
}

// RecordRejection atomically increments the process-lifetime rejection counter.
// Called by the middleware on a clean 429 (dry_run=false). Not incremented on
// exempt, malfunction, or dry-run would-reject paths.
func (l *Limiter) RecordRejection() {
	l.rejectionsTotal.Add(1)
}

// RecordFailure atomically increments the process-lifetime malfunction counter.
// Called by the middleware on malfunction (err != nil from Allow, or a
// recovered panic). Not reset when corruption clears (BR-49 — process-lifetime
// cumulative; restart to reset).
func (l *Limiter) RecordFailure() {
	l.failuresTotal.Add(1)
}

// RejectionsTotal returns the process-lifetime cumulative rejection count
// (atomic, observed without the mutex).
func (l *Limiter) RejectionsTotal() int64 {
	return l.rejectionsTotal.Load()
}

// FailuresTotal returns the process-lifetime cumulative malfunction count
// (atomic, observed without the mutex).
func (l *Limiter) FailuresTotal() int64 {
	return l.failuresTotal.Load()
}

// NumKeys returns the current number of tracked keys. Used by tests; not on the
// status hot path (Snapshot is).
func (l *Limiter) NumKeys() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.keys)
}