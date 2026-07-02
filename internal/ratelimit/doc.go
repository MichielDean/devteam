// Package ratelimit implements a single-process, in-memory rate limiter for the
// Dev Team server.
//
// # Boundary rule (LOCKED — app-design §1.1, business-logic-model §1.2)
//
// This package imports ONLY the Go standard library (sync, time, sort, net, log).
// It MUST NOT import internal/api or internal/config. This makes the algorithm
// unit-testable in isolation (E14 — no DB, no HTTP) and makes the reversibility
// boundary C-01/D5/D8 real: swap the algorithm = swap one package's internals.
// The gate is: `go list -deps ./internal/ratelimit` shows nothing outside stdlib.
//
// # Algorithm (D5 — two-bucket weighted sliding window)
//
// At time t, the weighted count for a key is:
//
//	count = previousBucket * (1 - (t - currentBucketStart)/window) + currentBucket
//
// When t - currentBucketStart >= window: rollover
// (previousBucket = currentBucket; currentBucket = 0; currentBucketStart = bucket boundary).
// O(1) time, O(1) memory per key. Precision loss vs. a true sorted-set sliding
// window is negligible for dev-tool-scale limits (100/300-scale, not microsecond quotas).
//
// # Count semantics (LOCKED — invariant 6, BR-11)
//
// The count reflects requests the limiter adjudicated in the window. On allow:
// counter += 1 (this request admitted). On deny: counter += 1 (this request was
// evaluated and rejected — it consumed a slot in the evaluation even though it
// wasn't admitted, so the window reflects "how many were attempted"). This
// matches count == limit+1 on the over-limit request. The counter tracks demand,
// not just admitted traffic, so an operator reading active_keys[].count on the
// status endpoint sees how hard a key is being hammered, not just how many got
// through.
//
// # Eviction (NDP-05 — amortized in-band sweep, no goroutine)
//
// A sweep runs on every 64th Allow call under the existing mutex (amortized, NOT
// a goroutine) and evicts keys whose lastSeen is older than 2 * window OR when
// the map exceeds maxTrackedKeys (evict oldest by lastSeen to maxTrackedKeys * 0.9
// — 10% headroom to avoid thrash). No background goroutine means no shutdown
// coordination, no leak risk, no goroutine-lifecycle test complexity.
//
// # Fail-open (D1, ADR-002, BR-50)
//
// Any panic or internal error during evaluation returns
// (Verdict{Allow:true, OverLimit:false, Exempt:false}, err). The verdict is final
// (allow); the error is the signal. The middleware consumes both: err != nil →
// log M4.2, failuresTotal++, no RateLimit-* headers, next.ServeHTTP. Fail-closed
// is structurally unreachable in v1 (fail_mode validated to "fail_open" only).
package ratelimit