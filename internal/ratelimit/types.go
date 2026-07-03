package ratelimit

import "time"

// Policy is the immutable per-verdict resolved policy (BLM §1.1).
// The middleware resolves an override (or default) into a Policy before
// calling Limiter.Allow; the Limiter treats the policy as opaque data
// except for the Limit and Window fields it needs for the window math.
type Policy struct {
	Limit  int
	Window time.Duration
	Exempt bool
}

// Verdict is the outcome of one Allow call (BLM §1.1). On a clean allow or
// clean deny, err is nil. On a malfunction (panic or internal error during
// the window check), Allow returns a fail-open verdict
// {Allow:true, OverLimit:false, Exempt:false} with a non-nil err — the
// middleware decides whether to log and pass through (§2.1 fail-open pattern).
type Verdict struct {
	Allow     bool
	Key       string
	Count     int
	Limit     int
	Window    time.Duration
	ResetIn   time.Duration
	OverLimit bool
	Exempt    bool
	Err       error
}

// KeyState is one row of Snapshot.Keys (BLM §1.1). The composite key is
// echoed verbatim (no redaction — O-4/BR-30; the endpoint is operator-only).
type KeyState struct {
	Key             string `json:"key"`
	Count           int    `json:"count"`
	Limit           int    `json:"limit"`
	WindowSeconds   int    `json:"window_seconds"`
	ResetInSeconds  int    `json:"reset_in_seconds"`
}

// Snapshot is the bounded point-in-time view the status handler reads
// (BLM §1.1, §3.2). Keys is sorted by Count DESCENDING (hottest first) and
// capped at the maxKeys argument to Limiter.Snapshot; Truncated is true when
// the cap was hit and some keys were omitted (O-2 — a BOOL field, NOT a
// synthetic entry inside Keys).
//
// Keys MUST be serialized as an empty array when there are no tracked keys,
// NEVER as null (BR-28 — the json tag is "active_keys" without omitempty,
// and the slice is initialized to empty in the status handler). The caller
// is responsible for initializing Keys to an empty slice in the empty case;
// Snapshot itself returns an empty slice (not nil) when the limiter has no
// keys.
type Snapshot struct {
	Keys            []KeyState `json:"active_keys"`
	Truncated       bool       `json:"active_keys_truncated"`
	RejectionsTotal int64      `json:"rejections_total"`
	FailuresTotal   int64      `json:"failures_total"`
}