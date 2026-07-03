package ratelimit

import "time"

// Clock is the time source for the Limiter. The wall clock is used by default;
// a fake clock is injected in unit tests for deterministic time advancement
// (BLM §2.6 — Clock injection; F-13: pure stdlib, no internal imports).
type Clock interface {
	Now() time.Time
}

// wallClock reads the real time. Production default.
type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

// FakeClock is a controllable Clock for unit tests. Advance moves time forward
// deterministically so tests can exercise window rollover and eviction
// without sleeping (BLM §2.6 — "no real wall clock in unit tests").
type FakeClock struct {
	t time.Time
}

// NewFakeClock returns a FakeClock anchored at the given start time.
func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{t: start}
}

func (c *FakeClock) Now() time.Time { return c.t }

// Advance moves the fake clock forward by d. Not safe for concurrent use
// (tests are single-threaded per the fake-clock contract).
func (c *FakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

// Set replaces the fake clock's current time. Useful for exact-edge tests.
func (c *FakeClock) Set(t time.Time) { c.t = t }