package ratelimit

// SwapClockForTest replaces the limiter's clock with the given one. This is
// a test-only seam; production code never swaps the clock after construction.
// It exists so the api-package integration tests can inject a malfunction
// (via a panicking clock) without exposing the unexported clock field.
func (l *Limiter) SwapClockForTest(c Clock) { l.clock = c }

// RestoreClockForTest restores the real wall clock. Test-only.
func (l *Limiter) RestoreClockForTest() { l.clock = wallClock{} }