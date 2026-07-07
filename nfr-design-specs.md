# NFR Design Specs — Stream Agent Output to Database Instead of Files

**Feature:** stream-agent-output-to-database-instead-of-files
**Stage:** 3.3 NFR Design
**Role:** architect
**Date:** 2026-07-06
**Depth:** standard

---

## Purpose

This artifact specifies the **technical approaches** that realize the non-functional requirements enumerated in stage 3.2 (`security-nfrs`, `performance-nfrs`, `reliability-nfrs`). Stage 3.2 stated *what* the feature must satisfy and *how to verify* it; this stage specifies *how the code is shaped* to satisfy it — the patterns, the structures, the test scaffolding, and the verification procedures the construction bolts (3.4+) will implement.

The feature is a wiring refactor on a modular monolith with one local DB and one local SSE transport. The NFR design is correspondingly conservative: the dominant controls are **structural** (ADR-1 removes the second writer; ADR-2 removes the dual-write path; ADR-7 inverts the dependency), not bolted-on guards. This artifact specifies the structural controls, the defense-in-depth tests that back them, and the small number of runtime guards (config validation, retry-once, reconnect reconcile) the structure leaves.

Every design block traces to a 3.2 NFR (NFR-S/P/R-N), an ADR, and a constraint (C-N) or requirement (FR-N). The construction bolts implement these specs; the quality gates (3.6) verify them.

---

## 1. Security Design

The threat model is single-actor (Operator), single-trust-boundary (local process). The feature is a net security improvement (less shell, no new surface, no world-readable files). The security design is therefore mostly **enforcement design** — specifying how the code review, the import graph, and the test scaffolding guarantee the structural invariants the architecture already established.

### D-S1 — No-new-surface enforcement (carries NFR-S1, ADR-9, C-1)

**Design:** The feature adds no import, no route, no datastore. Enforcement is via three construction-gate checks:

1. **`go.mod` / `go.sum` invariant:** a construction-bolt test (`internal/config/no_new_deps_test.go` or equivalent) asserts the feature diff introduces no new direct require entry. The test runs `go mod tidy` against a clean checkout and asserts the resulting `go.mod` matches the feature-branch `go.mod` byte-for-byte (modulo ordering, which `go mod tidy` normalizes). A new datastore driver (e.g., `github.com/lib/pq` already present — anything else) would surface here.
2. **Route registration invariant:** a grep-based check in the construction review asserts no new `http.HandleFunc` / `router.HandleFunc` / `mux.Handle` call site exists in the feature diff that is not wrapped by the existing auth middleware. The check is a `grep -nE 'HandleFunc|\.Handle\('` over the diff, with manual review of each hit. Automated where possible; manual where the router shape varies.
3. **External-client import invariant:** `grep -rnE 'langfuse|loki|datadog|otel|langsmith|clickhouse' internal/` returns no matches. A construction-bolt test wraps this grep as a Go test (`internal/security/no_external_clients_test.go`) so it runs in CI, not just at review time.

**Why this shape:** the threats NFR-S1 guards against are all *additive* (a new dep, a new route, a new client). Additive threats are caught by diff-invariant tests, not by runtime guards. The design is "make the absence visible in CI," not "build a runtime sandbox."

**Verification procedure (for quality gate 3.6):**
- `go test ./internal/security/...` passes (the no-external-clients test).
- `git diff origin/main...HEAD -- go.mod go.sum` shows no new require entry (manual; the construction review records the verdict).
- `git diff origin/main...HEAD -- internal/api/` shows no new route registration outside existing middleware (manual; recorded in the review).

### D-S2 — Arg-vector `exec.Cmd` enforcement (carries NFR-S2, FR-1, C-8, ADR-1)

**Design:** `DispatchStreaming` constructs the agent command as `exec.CommandContext(ctx, binary, args...)` — an arg slice, never a shell string. The removed `tee | PIPESTATUS` shell pipe does not return. Enforcement:

1. **Grep gate:** `grep -nE 'sh -c|bash -c' internal/role/tmux.go internal/api/agent_handlers.go` returns no matches in the dispatch path (context-file writes via shell are out of scope, C-6 — those calls are in `prepareContextDir`, not the dispatch path, and are flagged as such in review).
2. **Construction review:** confirms the `exec.Cmd` is built from internal state (role, featureID, stageID, bolt) and that no arg derives from operator runtime input. The args are the agent binary path (from config) and a fixed set of flags; none is a free-form string.
3. **Test:** a unit test in `internal/role/streamer_test.go` (or `tmux_test.go`) constructs a dispatch with a feature/stage ID containing shell metacharacters (`;`, `$(...)`, backticks) and asserts the command is invoked with those as literal args (no shell expansion). This is a positive test that the arg-vector construction is shell-injection-proof by construction.

**Why this shape:** shell injection is prevented structurally (no shell). The test is a regression guard — it proves the structural property holds and prevents a future edit from reintroducing a shell string.

### D-S3 — No-output-files enforcement (carries NFR-S3, FR-1, NFR-10, ADR-1)

**Design:** the write path creates no `.log`, no `exit_code`, no `proxy.log`, no `logs/` directory. Enforcement:

1. **Grep gate:** `grep -nE 'tee |logPath|exitCodePath|proxy\.log|os\.Create.*\.log|MkdirAll.*logs' internal/role/tmux.go internal/api/agent_handlers.go` returns no matches in the write path.
2. **Integration test:** a test in `internal/role/streamer_test.go` runs a dispatch against a fake agent that emits known stdout, then asserts the ephemeral context dir contains ONLY the context input files (`CONTEXT.md`, `agents/<role>.md`, `opencode.json`, `AGENTS.md`, `.bashrc`) and NO `.log` / `exit_code` file. The test uses `os.ReadDir(ctxDir)` and asserts the file set matches the allowlist.
3. **Dead-code removal gate:** `grep -rnE 'getCapturePane|CapturePaneRaw|getCapturedOutput' internal/ ui/src/` returns no matches in non-test source (NFR-10 / FR-18). A construction-bolt test wraps this grep.

**Why this shape:** the write path is the feature's purpose; making its cleanliness a CI-checked invariant, not a review-time hope, is the design. The allowlist test is the strong form — it asserts the positive (only context files remain), not just the negative (no log files).

### D-S4 — SSE auth inheritance (carries NFR-S4, ADR-4)

**Design:** the SSE handler and the `getStageLog` handler are unchanged in their middleware wrapping. The feature adds a *consumer* (`TmuxPaneViewer`), not a route. Enforcement:

1. **Route audit:** the construction review records, for each `http.Handle` / `HandleFunc` call site in `internal/api/server.go`, whether it is wrapped by the auth middleware. The diff must not change any wrapping and must not add an unwrapped route.
2. **No-new-route grep:** `git diff origin/main...HEAD -- internal/api/server.go internal/api/sse.go` shows no new `Handle` / `HandleFunc` call site (only modifications to existing handler bodies — the additive `source` field on `getStageLog`).

**Why this shape:** auth is inherited, not re-implemented. The design is "prove nothing changed in the wrapping," not "add a new auth check for the new consumer." The new consumer uses the existing authenticated endpoint.

### D-S5 — Data locality (carries NFR-S5, C-1, NFR-4)

**Design:** the output path is `agent stdout → in-process batcher → local Postgres → local SSE`. No leg leaves the machine. Enforcement is covered by D-S1's no-external-client grep (any egress client would be a new import). Additionally:

1. **Egress grep:** `grep -rnE 'http\.Post|http\.Client|grpc\.Dial|net\.Dial' internal/role/streamer.go internal/role/tmux.go internal/api/sse.go` returns no matches in the output path (the SSE handler writes to an `http.ResponseWriter` of an inbound request; it does not make an outbound call).
2. **Composition-root review:** the `FlushFunc` closure (ADR-7) is confirmed to call only `db.AppendStageLogForBolt` — no second call, no fan-out to a remote.

**Why this shape:** data locality is the absence of egress. The grep proves the absence. The composition-root review confirms the one DI seam does not secretly widen.

### D-S6 — Config read-only enforcement (carries NFR-S6, NFR-P5, C-9, ADR-1)

**Design:** the `Streaming` config block (`log_file_fallback`, `flush_interval_ms`, `flush_bytes`, `render_cap_lines`) is loaded once at process start from `devteam.yaml` via the existing config-load path in `internal/config`. No handler, no request body, no query param, no env var overrides it at runtime. Enforcement:

1. **Config-load-path grep:** `grep -rn 'flush_interval_ms\|flush_bytes\|log_file_fallback\|render_cap_lines' internal/` returns matches ONLY in `internal/config/` (load), `internal/role/streamer.go` (read of the loaded struct), `internal/api/server.go` (read of `log_file_fallback` in the `getStageLog` fallback), and the config schema comment. No match in any `http.Handler` body reading from a request.
2. **Struct immutability:** the `Streaming` struct fields are not exported as setters; the config load returns a value, not a pointer-to-mutable-shared-state. (This is a Go convention, not a language-enforced invariant — the review confirms it.)

**Why this shape:** there is no injection surface because there is no runtime input path to the config. The grep proves the config keys are read-only outside the load path.

### D-S7 — Parameterized-query confirmation (carries NFR-S7, DR-1, C-2)

**Design:** `AppendStageLogForBolt` and `GetStageLogForBolt` are unchanged (C-2 / C-4). They already use `$1, $2, ...` placeholders. The feature adds no new SQL. Enforcement:

1. **No-new-SQL grep:** `git diff origin/main...HEAD -- internal/db/stage_log_store.go` shows no functional change (at most a comment).
2. **No-Sprintf-in-SQL grep:** `grep -nE 'Sprintf.*SELECT|Sprintf.*INSERT|Sprintf.*UPDATE' internal/db/stage_log_store.go` returns no matches.

**Why this shape:** the store is unchanged; this NFR is confirmatory. The design is "prove the store wasn't touched and doesn't string-concat SQL."

### D-S8 — Dead-route removal enforcement (carries NFR-S8, NFR-10, FR-10, FR-18, ADR-4)

**Design:** the removed handlers (`getCapturePane`, `CapturePaneRaw`, `getCapturedOutput`) and their route registrations are deleted in the feature PR. Enforcement is covered by D-S3's dead-code grep (point 3). Additionally:

1. **Route-registration grep:** `grep -nE 'getCapturePane|CapturePaneRaw|getCapturedOutput' internal/api/server.go internal/api/session_handlers.go internal/pipeline/session_manager.go` returns no matches — neither the handler definition nor the route registration.

**Why this shape:** a dead route is a stale auth/data-flow assumption. Removal is the guard; the grep proves removal.

---

## 2. Performance Design

The performance envelope is bounded (one stage at a time per feature, ~10² stages/feature, one human-watching UI). The performance design is **threshold calibration + batcher structure + render cap + backpressure**, not a metrics pipeline.

### D-P1 — Live-latency budget enforcement (carries NFR-P1, NFR-1, C-3, C-4, ADR-8)

**Design:** the end-to-end latency budget (P50 ≤200 ms, P99 ≤400 ms) is bounded by the batch flush threshold (200 ms default) plus the append cost plus the SSE write. The design decomposes the budget:

| Leg | Budget term | Default | Owner |
|-----|-------------|---------|-------|
| Agent stdout → batcher read | negligible (in-process `io.Reader`) | <1 ms | batcher |
| Batcher buffer → flush trigger | `flush_interval_ms` (time) OR line boundary (immediate) OR `flush_bytes` (size) | 200 ms / immediate / 8 KB | batcher |
| `FlushFn` (DB append) | ≤5 ms P99 at ≤1 MB row (NFR-P3) | 5 ms | store |
| `lineCh` send (SSE consumer drain) | human-pace, buffered 100 | <1 ms typical | SSE handler |
| SSE write → browser `onmessage` | local HTTP, single hop | <10 ms | transport |

The line-boundary flush is the fast path (common case: line-by-line output flushes immediately, latency dominated by the SSE hop). The time-trigger (200 ms) is the bound for long single-line bursts. The design is "the threshold IS the budget" — there is no separate latency governor.

**Calibration procedure (D-5 micro-benchmark, executed in planning or in the first construction bolt):**
1. A Go benchmark (`internal/db/stage_log_store_bench_test.go`) inserts/appends at row sizes 1 KB, 10 KB, 100 KB, 1 MB, 10 MB, measuring P50/P99 latency over 100 appends per size.
2. Results recorded in the planning artifact (or, if deferred, in the first construction bolt's commit message).
3. If 1 MB P99 >5 ms: the result is logged to raid-log R-4 (the chunk-table trigger, C-11) and the thresholds ship at the preliminary values with a documented deferral. The chunk-table refactor is NOT implemented in this feature (C-11 out of scope).

**Runtime verification procedure (one-shot, no metrics pipeline per ADR-9):**
1. A manual trace under a real stage: instrument the batcher flush with `time.Now()` before the flush and the UI `onmessage` handler with `performance.now()`. Assert P50 ≤200 ms, P99 ≤400 ms over the stage's chunks.
2. The trace is a one-shot check recorded in the construction review, not a continuous metric. If P50 >200 ms, the D-5 benchmark is re-run; the chunk-table (C-11) is the escalation, not a threshold relaxation.

**Why this shape:** the budget is enforced by the threshold (structural) and verified by a one-shot trace (manual). A continuous metrics pipeline is out of scope (C-1, ADR-9). The design accepts that a latent regression won't be auto-detected; the footer (FR-13) and the periodic D-5 re-run are the mitigations.

### D-P2 — Batched-flush structure (carries NFR-P2, NFR-1, C-3, FR-3)

**Design:** the batcher's flush logic is a single switch on three triggers, evaluated per read:

```go
// pseudo-structure (planning finalizes signatures)
func (b *batcher) maybeFlush() {
    hasLineBoundary := bytes.Contains(b.pending, []byte('\n'))
    timeUp := time.Since(b.lastFlush) >= b.cfg.FlushIntervalMs*time.Millisecond
    sizeUp := len(b.pending) >= b.cfg.FlushBytes
    if hasLineBoundary || timeUp || sizeUp {
        b.flush()  // FlushFn + lineCh send (ADR-8)
    }
}
```

- **Line boundary** (always-on, not configurable): the common case flushes immediately on `\n`. This is the fast path for line-by-line output.
- **Time trigger** (`flush_interval_ms`): bounds latency for long single-line output (a 5 s single-line burst flushes every 200 ms, not at line-end). Caps append rate at 5/sec under this trigger alone.
- **Size trigger** (`flush_bytes`): bounds buffer memory and caps a single append's payload. May exceed 5 appends/sec during a burst, but a burst means large appends, so the per-byte rewrite cost (not the count) is the bound.

**No per-line flush path:** the flush is always threshold-gated. The line-boundary trigger fires on `\n`, but a line with no `\n` (a long partial line) does not flush until time or size fires. This is the intended behavior (R-1 product call: line OR timeout OR size).

**Test procedure:**
1. A unit test emits 1,000 short lines (each ending `\n`) and asserts each line triggers a flush (line-boundary fast path) — append count ≈ 1,000 (or batched if lines arrive faster than the flush call, but each `\n` is a trigger).
2. A unit test emits a single 100 KB line (no `\n`) with `flush_bytes=8192` and asserts the append count is ≈ 100_000/8_192 ≈ 12 (size trigger), not 1 (no line boundary) and not 500 (time trigger at 200ms over the burst duration).
3. A unit test emits nothing for 1 s and asserts NO append occurs (no idle flush — the time trigger fires only when there is pending data; an empty buffer does not flush).

**Why this shape:** the three-trigger switch is the complete specification of the flush policy. The tests pin each trigger and the no-idle-flush invariant. The structure prevents the degenerate per-line path by construction.

### D-P3 — DB append latency monitoring (carries NFR-P3, C-11, AS-1)

**Design:** the append-latency bound (≤5 ms P99 at ≤1 MB) is monitored by the D-5 micro-benchmark (D-P1), not by a runtime metric. The design is explicitly **monitor, don't refactor** (C-11): if the bound breaks, the chunk-table refactor is a *future feature*, not an in-feature escalation.

**Escape hatch design (documented, not implemented):** the `FlushFunc` injection (ADR-7) is the seam. A future chunk-table refactor swaps the closure body from `db.AppendStageLogForBolt(...)` to `db.AppendStageLogChunk(...)` (a new method on a new `stage_log_chunks` table) without touching the batcher. This is the design-for-change payoff: the batcher is chunk-table-ready by construction.

**Why this shape:** C-11 is a monitored risk, not an in-feature work item. The design makes the future swap cheap (the seam exists) and the current bound visible (the benchmark), and stops there. Over-engineering the chunk-table now would violate "design for change, not for reuse" (the change hasn't been triggered).

### D-P4 — UI render cap (carries NFR-P4, FR-15, O-8)

**Design:** `AgentOutputLive` caps the rendered DOM to `render_cap_lines` (default 5,000) lines. Output beyond the cap is truncated from the top (oldest lines dropped). A `[Load all N]` affordance renders the full output (or routes to server-search via `GET /log/{stageId}?q=...`).

**Implementation structure:**
1. The component holds the full received content in a ref (not state — no re-render on append).
2. The rendered state is `content.slice(-render_cap_lines * avgLineLen)` (a tail slice). The slice is recomputed on append ONLY if the new content crosses the cap boundary.
3. The affordance is a button that, on click, sets a `showAll` flag that bypasses the slice. For very large outputs (>50 K lines), `showAll` routes to a server-search view (`GET /log/{stageId}?q=...`) instead of rendering the full DOM.

**Test procedure:**
1. A component test renders 50,000 lines and asserts the DOM node count for the output region is ≤5,000 (plus the affordance).
2. A component test renders 50,000 lines, clicks `[Load all N]`, and asserts the full output renders (or the server-search view mounts) without the tab freezing (measured by the test completing within a timeout).

**Why this shape:** the cap is a UI concern; it does not affect the backend contract (the full output is in the DB and in `result.Output`). The design is "bound the DOM, not the data." The server-search escape hatch handles the "I need to see a specific old line" case without unbounded rendering.

### D-P5 — Config validation at load (carries NFR-P5, NFR-S6, C-3, FR-9)

**Design:** the `Streaming` config block is validated in the config-load path (`internal/config`), not at use time. Validation rules:

| Field | Rule | Error |
|-------|------|-------|
| `flush_interval_ms` | `> 0` | `"streaming.flush_interval_ms must be > 0 (got %d); 0 causes a busy-loop"` |
| `flush_bytes` | `> 0` | `"streaming.flush_bytes must be > 0 (got %d); 0 never flushes"` |
| `render_cap_lines` | `> 0` | `"streaming.render_cap_lines must be > 0 (got %d); 0 renders nothing"` |
| `log_file_fallback` | (bool, no range check) | n/a |

On validation failure, the config-load function returns an error; `main` exits with a non-zero code and prints the error. The process does NOT start the batcher with degenerate thresholds.

**Test procedure:**
1. A test loads a config with `flush_interval_ms: 0` and asserts the load returns an error naming the field and the value.
2. A test loads a config with `flush_bytes: -1` and asserts the same.
3. A test loads a config with `render_cap_lines: 0` and asserts the same.
4. A test loads a config with the defaults (no `streaming` block) and asserts the defaults are applied (200 / 8192 / 5000 / false) and no error occurs.

**Why this shape:** a degenerate threshold (0 ms = busy-loop, 0 bytes = never flushes) is a silent failure mode worse than a startup crash. The design fails fast at the boundary where the config enters the process, not at the point of use where the batcher is already running.

### D-P6 — Backpressure structure (carries NFR-P6, ADR-8, C-4, C-12)

**Design:** the batcher's `lineCh` send is synchronous (blocks until the SSE consumer drains). The `lineCh` buffer is 100 (existing, unchanged). The backpressure chain is:

```
SSE consumer stalls → lineCh fills (100) → batcher's lineCh<- blocks
  → batcher stops reading stdout → stdout pipe fills → agent's stdout write blocks
  → agent slows down (its own write loop blocks)
```

The design is **slow down, don't drop** (ADR-8). No drop path exists in the batcher — the channel send is unconditional (after the DB flush, persist-then-push).

**Test procedure:**
1. A test fills `lineCh` to capacity (100), then attempts one more send and asserts it blocks (a `select` with a short timeout observes the block, not a drop).
2. A test fills `lineCh`, drains it after a pause, and asserts all chunks are delivered in order (no loss, no reordering).

**Why this shape:** backpressure is structural (synchronous send + bounded buffer). The design does not add a drop policy because dropping is a C-12/NFR-R2 violation, not a backpressure behavior. The test pins the block-and-resume invariant.

---

## 3. Reliability Design

The dominant reliability control is structural (ADR-2: remove the second writer). The reliability design specifies the structural invariants, the drain ordering, the failure-mode handlers (retry-once, reconnect reconcile, re-dispatch reset), and the accepted residuals.

### D-R1 — Graceful drain structure (carries NFR-R1, NFR-2, C-12, FR-6, ADR-2)

**Design:** the batcher's `StreamOutput` function drains on shutdown as follows:

```go
// pseudo-structure
func StreamOutput(ctx, r, lineCh, flushFn, ...) (buffer string, err error) {
    for {
        select {
        case <-ctx.Done():
            b.finalFlush()   // flush buffered tail to DB + lineCh
            return b.buffer, ctx.Err()
        default:
        }
        n, err := r.Read(buf)
        if n > 0 { b.append(buf[:n]); b.maybeFlush() }
        if err == io.EOF {
            b.finalFlush()   // flush buffered tail to DB + lineCh
            return b.buffer, nil
        }
        if err != nil { b.finalFlush(); return b.buffer, err }
    }
}
```

The `finalFlush` is unconditional on every exit path (ctx cancel, EOF, read error). No buffered tail is lost on graceful shutdown.

**Hard-crash residual (accepted, documented):** on SIGKILL/power loss, the batcher goroutine dies with up to one batch's worth of buffered bytes (≤200 ms / ≤8 KB). This is the accepted residual (R-3 / NFR-R2). It is documented in the `Streaming` config schema comment and in the README. No test (SIGKILL is not testable in-process); the bound is ≤ the flush threshold by construction.

**Test procedure:**
1. A test emits output, cancels ctx mid-batch (with buffered tail < `flush_bytes` and no line boundary), and asserts `GetStageLogForBolt` returns all bytes produced before cancellation PLUS the buffered tail.
2. A test emits output, lets the agent exit (EOF), and asserts the full output is in the DB (including the final partial batch).

**Why this shape:** the drain is structural (finalFlush on every exit path). The test pins the cancellation and EOF paths. The hard-crash residual is documented as a bound, not tested (it's untestable in-process and bounded by construction).

### D-R2 — Immutability-after-completion enforcement (carries NFR-R2, NFR-9, ADR-2)

**Design:** after a stage reaches `complete`, no write occurs to its `stage_logs` row. This holds because:
1. `SaveStageLogForBolt` is removed from the completion path (`stage_runner.go:203`) — ADR-2.
2. The batcher's final flush (in `StreamOutput`) is the last write before return.
3. No other writer exists (the re-dispatch reset is at `DispatchStreaming` entry, before `cmd.Start()` — D-R7).

**Test procedure:**
1. A test completes a stage, then polls `GetStageLogForBolt` every 100 ms for 2 s and asserts byte-identical content on every poll (no post-completion write).
2. A test completes a stage, then asserts no `AppendStageLogForBolt` call occurs for the next 2 s (instrumented via a counting `FlushFunc` — no call = no write).

**Code-review gate:** `grep -n 'SaveStageLogForBolt' internal/pipeline/stage_runner.go` returns no match in the post-`DispatchStreaming` completion path. The only `SaveStageLogForBolt` call is at `DispatchStreaming` entry (the re-dispatch reset, D-R7).

**Why this shape:** immutability is the absence of a writer. ADR-2 removes the writer; the test and the grep prove the absence.

### D-R3 — Drain ordering invariant (carries NFR-R3, NFR-5, C-5, R-1, ADR-2)

**Design:** the ordering chain (app-design §6.1) holds by construction:

```
final DB flush (in batcher goroutine)
  → StreamOutput returns
    → DispatchStreaming returns
      → stage_runner closes lineCh
        → lineCh drains → streamDone closes
```

The final flush is strictly before `streamDone` close. ADR-2 removes the second writer (`SaveStageLogForBolt` at completion), so no writer remains to race.

**Defense-in-depth test (navigator-authored, NFR-5):**
1. A dedicated race test in `internal/role/streamer_race_test.go` (or `internal/pipeline/stage_runner_race_test.go`) forces a flush during the drain window (emit output, trigger completion mid-batch, assert no loss, no duplication, no stale-clobber).
2. The test runs under `go test -race ./internal/role/... ./internal/pipeline/...` (C-13).
3. The test is authored by the navigator independently of the driver (mob-composition §6) — a different author catches different assumptions.

**Why this shape:** the invariant is structural (the return chain); the race test is defense-in-depth (catches a future edit that breaks the chain). The design does not rely on the test as the primary control — the structure is the control, the test is the regression guard.

### D-R4 — Dual-fan-out co-location (carries NFR-R4, C-4, FR-4, ADR-8)

**Design:** the DB append (`FlushFn`) and the `lineCh` send occur in the same flush, in the same goroutine, with DB-first ordering (persist-then-push, ADR-8):

```go
// inside the batcher's flush
if err := b.flushFn(ctx, fid, sid, bolt, chunk); err != nil {
    // log the error; retry once (D-R6); if still failing, log and continue
    // DO NOT skip the channel send — the live stream must not lose a chunk
}
b.lineCh <- role.OutputLine{Content: chunk, ...}  // synchronous send (D-P6)
```

No second goroutine, no async DB write, no "fire and forget" channel send.

**Test procedure:**
1. A unit test instruments a `FlushFunc` that records call order and a `lineCh` receiver that records receive order; after a run, asserts every flush has a matching send in the same order (DB-first).
2. The race test (D-R3) asserts under concurrent stress that neither the DB append nor the channel send occurs without the other in the same flush cycle.

**Code-review gate:** the batcher's flush block contains both `flushFn(...)` and `lineCh <- ...` in the same function, in that order. `grep -n 'flushFn\|lineCh <-' internal/role/streamer.go` shows them co-located.

**Why this shape:** co-location is what makes the live stream and the persisted log stay in lockstep. Splitting them (a second goroutine for DB, an async send) re-introduces the C-5 race surface and breaks reconcile-on-reconnect (D-R5). The design forbids the split by structure and verifies it by grep + test.

### D-R5 — SSE reconnect reconcile (carries NFR-R5, FR-17, ADR-8)

**Design:** the UI's `useSSE`-based reconcile logic (in `AgentOutputLive` and `TmuxPaneViewer`):

```typescript
// pseudo-structure
onerror: () => { setSseStatus('disconnected'); /* retain content */ }
onopen:  () => {
    // on reconnect, re-read the full DB content and reconcile
    fetch(`/log/${stageId}`).then(r => r.json()).then(({content}) => {
        const held = contentRef.current;
        // prefix-match dedup: find the longest prefix of `content` that matches a prefix of `held`
        // append the missing tail (content after the matched prefix)
        if (content.startsWith(held)) {
            setContent(content);  // held is a prefix of DB; append the tail
        } else {
            // held has content the DB doesn't (a live-only chunk the DB missed — D-R6 residual)
            // OR the DB has content held doesn't (a chunk received during disconnect)
            // reconcile by appending the DB tail after the longest common prefix
            const tail = content.slice(longestCommonPrefix(held, content));
            setContent(held + tail);
        }
    });
    setSseStatus('connected');
}
```

The DB is the reconciliation authority (NFR-R2). The reconcile appends the missing tail; it does not re-show content already displayed (dedup by content-prefix match).

**Test procedure:**
1. A component test simulates an SSE disconnect mid-stream (emit 3 chunks, disconnect, emit 2 more chunks server-side, reconnect), then asserts the UI shows all 5 chunks with no duplication and no gap.
2. A component test simulates a disconnect with no server-side progress (reconnect to the same state) and asserts no duplication (the held content equals the DB content; the tail is empty).

**Why this shape:** the DB has the full content (NFR-R2); the UI has a subset (what it received before disconnect). Reconcile = diff and append. The prefix-match dedup handles the common case (held is a prefix of DB). The longest-common-prefix fallback handles the rare case (a live-only chunk the DB missed, D-R6 residual — the UI keeps it and appends the DB tail after the common prefix).

### D-R6 — DB append failure: retry-once, log, continue (carries NFR-R6, O-10, ADR-8)

**Design:** the batcher's flush handles `FlushFn` errors with a single retry, then log-and-continue:

```go
// inside the batcher's flush
if err := b.flushFn(ctx, fid, sid, bolt, chunk); err != nil {
    // retry once with a short backoff
    time.Sleep(50 * time.Millisecond)
    if err2 := b.flushFn(ctx, fid, sid, bolt, chunk); err2 != nil {
        // log both errors; the DB row will have a gap for this chunk (accepted residual)
        log.Printf("streamer: DB append failed (retry): %v; %v (chunk gap accepted)", err, err2)
    }
    // DO NOT skip the channel send — the live stream must not lose a chunk
}
b.lineCh <- role.OutputLine{Content: chunk, ...}
```

The retry is a single attempt (not a circuit breaker, ADR-10). The channel send is unconditional (the live stream stays live even if the DB has a transient). The DB gap is the accepted residual (ADR-2 consequence).

**Hard-DB-failure boundary (ADR-10):** if the DB is down hard (Postgres not up), the `FlushFunc` fails on both attempts; the batcher logs and continues (the live stream stays live, the DB row has gaps). This is NOT a feature-level resilience pattern — a hard DB failure is a process-level event; the orchestrator's existing DB-connection failure handling applies. The feature does not add a circuit breaker.

**Test procedure:**
1. A test injects a `FlushFunc` that fails once then succeeds; asserts the batcher retries, the DB gets the chunk on retry, and the channel send occurs.
2. A test injects a `FlushFunc` that always fails; asserts the batcher logs, the channel send still occurs, and the DB row is missing that chunk (accepted residual).
3. A test asserts the channel send occurs EVEN on a DB failure (the live stream must not lose a chunk — the worse failure mode would be dropping the send).

**Why this shape:** retry-once bounds the latency cost of a transient (50 ms + one retry). The unconditional channel send prioritizes the live stream (a DB gap is recoverable on reload via D-R5 reconcile; a live drop is not). The hard-failure boundary is documented (ADR-10) — the design does not pretend to be more resilient than its environment.

### D-R7 — Re-dispatch reset (carries NFR-R7, R-6, ADR-2)

**Design:** at `DispatchStreaming` entry, before `cmd.Start()`, the dispatcher resets the row:

```go
// inside DispatchStreaming, before cmd.Start()
// reset the row so a re-dispatch does not concatenate stale + new output (R-6)
if err := sm.saveStageLog(featureID, stageID, bolt, ""); err != nil {
    return result, fmt.Errorf("re-dispatch reset failed: %w", err)
}
// ... cmd.Start(); StreamOutput(...) ...
```

`saveStageLog` is `SaveStageLogForBolt` (the full-replace upsert) — the one retained use (ADR-2). It runs before any batcher activity, so there is no race. The reset is unconditional (every dispatch, not just detected re-dispatches) — a first dispatch resets an empty row (no-op); a re-dispatch resets a completed row (the intended effect). Unconditional is simpler and avoids a "is this a re-dispatch?" check that could be wrong.

**Test procedure:**
1. A test completes a stage (row has the old output), re-dispatches it (new output), and asserts `GetStageLogForBolt` returns ONLY the new run's output (no prefix of the old run).
2. A test dispatches a stage for the first time (no row) and asserts the reset is a no-op (the row starts empty, the batcher appends from scratch).

**Code-review gate:** `grep -n 'SaveStageLogForBolt' internal/role/tmux.go` shows exactly one call site, at `DispatchStreaming` entry, before `cmd.Start()`. No other call site in the dispatch or completion path.

**Why this shape:** the reset is structural (runs before the batcher, no race). Unconditional is simpler than a re-dispatch detector and has the same effect (a first dispatch resets an empty row). The test pins the "no stale concatenation" invariant; the grep pins the single call site.

### D-R8 — Config flag read-path-only enforcement (carries NFR-R8, NFR-8, C-9, ADR-1, ADR-5)

**Design:** `log_file_fallback` governs the READ-path legacy fallback ONLY (ADR-1 / ADR-5). No write path consults it. The write path is always Shape B. Enforcement:

1. **Read-path-only grep:** `grep -rn 'LogFileFallback\|log_file_fallback' internal/` returns matches ONLY in:
   - `internal/config/` (load + default),
   - `internal/api/server.go` (the `getStageLog` fallback read),
   - the config schema comment / `devteam.yaml` comment.
   No match in `internal/role/streamer.go`, `internal/role/tmux.go`, or `internal/api/agent_handlers.go` (the write paths).
2. **No-write-path-file grep:** `grep -nE 'tee |logPath|exitCodePath|proxy\.log' internal/role/tmux.go internal/api/agent_handlers.go` returns no matches (the Shape A write path is gone, not flagged).

**Why this shape:** the flag's scope is the absence of write-path references. The grep proves the absence. The design is "the flag is read-only config for a read-only fallback; the write path is unconditional Shape B."

---

## 4. Observability Design (carries ADR-9, NFR-4, C-1)

The feature adds no metrics endpoint, no trace pipeline, no log shipper (C-1, ADR-9). The observability surface is:

| Signal | Source | Mechanism |
|--------|--------|-----------|
| Read-path source (db vs file-legacy) | `getStageLog` response `source` field | Additive JSON field; UI footer (FR-13) renders it |
| SSE status (connected / disconnected / reconnected) | `useSSE` hook state | UI footer (FR-13) renders it |
| "Received Xs ago" | Client-side `performance.now()` delta on last chunk | UI footer (FR-13) renders it |
| Batcher flush errors | `log.Printf` at debug level in the batcher flush | Existing logger; no pipeline |
| DB append errors (retry failed) | `log.Printf` at debug level in the retry branch | Existing logger; no pipeline |
| Stage row size (future) | `GetStageLogMeta` (existing, unused) | Out of scope; a future `/metrics` endpoint is the escape hatch |

**No structured logging pipeline.** The existing `log.Printf` is the logger. Debug level for flush/append errors (not info — too noisy at 5 flushes/sec). The operator inspects via `journalctl` or the process's stdout.

**No aggregated metrics.** If the operator wants "average flush latency across the last 100 stages," they write a SQL query against `stage_logs` (e.g., `SELECT feature_id, stage_id, length(content), updated_at - created_at AS duration FROM stage_logs`). The DB is directly queryable; the operator is a developer.

**Why this shape:** C-1 forbids the SaaS options; ADR-9 forbids the self-hosted stack (operational burden disproportionate for a P2 internal refactor). The footer is the operator's real-time signal; the DB is the operator's retrospective signal. The design does not add a metrics pipeline because no consumer needs one today.

---

## 5. Resilience Pattern Summary (carries ADR-10, C-1)

| Pattern | In scope? | Rationale |
|---------|-----------|-----------|
| Circuit breaker | No | No fallback path (the DB is the only store, C-1); a tripped breaker with no fallback is a dropped chunk, which is worse than a slow retry (ADR-10) |
| Bulkhead (goroutine pool) | No | The batcher IS single-goroutine by design (C-5); a pool re-introduces the race surface |
| Retry with backoff | **Yes — single retry, 50 ms** | Bounds the latency cost of a transient; the channel send is unconditional (the live stream stays live) |
| Reconnect reconcile | **Yes — UI-side** | The DB is the reconciliation authority; the UI appends the missing tail on reconnect (FR-17) |
| Graceful drain | **Yes — structural** | `finalFlush` on every exit path (ctx cancel, EOF, read error); no buffered tail lost on graceful shutdown |
| Hard-crash residual | **Accepted — ≤ one batch** | ≤200 ms / ≤8 KB lost on SIGKILL; documented, bounded, untestable in-process (R-3) |

**Why no circuit breaker:** the feature has no downstream service to break against. The DB is local; a DB failure is a process-level event, not a feature-level transient to isolate. The retry-once is the only resilience mechanism; it is a single retry, not a breaker.

---

## 6. Test Strategy Summary

The NFR test strategy is a layered set of unit, race, component, and integration tests, each pinning a specific NFR invariant.

| Test | NFR | Type | Author | Gate |
|------|-----|------|--------|------|
| `streamer_test.go` — flush triggers (line/time/size), no-idle-flush | NFR-P2 | unit | driver | `go test` |
| `streamer_test.go` — drain on cancel/EOF, no buffered tail lost | NFR-R1 | unit | driver | `go test` |
| `streamer_test.go` — dual-fan-out co-location, DB-first ordering | NFR-R4 | unit | driver | `go test` |
| `streamer_test.go` — backpressure: fill lineCh, block, drain, no loss | NFR-P6 | unit | driver | `go test` |
| `streamer_test.go` — retry-once on FlushFunc failure, channel send unconditional | NFR-R6 | unit | driver | `go test` |
| `streamer_test.go` — no output files in context dir (allowlist) | NFR-S3 | integration | driver | `go test` |
| `streamer_test.go` — shell-metacharacter args are literal (no shell) | NFR-S2 | unit | driver | `go test` |
| `streamer_race_test.go` — flush during drain window, no loss/dup/clobber | NFR-R3 / NFR-5 | race | navigator | `go test -race` |
| `stage_log_store_test.go` — append-after-append concatenation, idempotent upsert | FR-2 / NFR-S7 | unit | navigator | `go test` |
| `stage_log_store_bench_test.go` — append latency at 1K/10K/100K/1M/10M row sizes | NFR-P3 | benchmark | driver | `go test -bench` |
| `config_test.go` — `flush_interval_ms: 0` / `flush_bytes: -1` / `render_cap_lines: 0` fail fast | NFR-P5 | unit | driver | `go test` |
| `config_test.go` — defaults applied when `streaming` block absent | NFR-P5 | unit | driver | `go test` |
| `server_test.go` — `getStageLog` returns `source: "db"` / `"file-legacy"` | NFR-S1 / FR-13 | unit | navigator | `go test` |
| `tmux_test.go` — re-dispatch resets row (no stale concatenation) | NFR-R7 | integration | driver | `go test` |
| `tmux_test.go` — immutability after completion (no post-completion write) | NFR-R2 | integration | navigator | `go test` |
| `AgentOutputLive.test.tsx` — render cap at 5,000 lines, `[Load all N]` works | NFR-P4 | component | researcher | `vitest` / `playwright` |
| `AgentOutputLive.test.tsx` — SSE disconnect/reconnect reconcile, no dup/gap | NFR-R5 | component | researcher | `vitest` / `playwright` |
| `TmuxPaneViewer.test.tsx` — renders output from SSE/DB after re-route | FR-10 / NFR-S8 | component | researcher | `vitest` / `playwright` |
| `no_external_clients_test.go` — grep for langfuse/loki/datadog/otel/etc. | NFR-S1 | grep-test | driver | `go test` |
| `dead_code_test.go` — grep for getCapturePane/CapturePaneRaw/getCapturedOutput | NFR-S8 / NFR-10 | grep-test | driver | `go test` |
| `no_new_deps_test.go` — `go.mod` diff has no new require entry | NFR-S1 / NFR-4 | diff-test | driver | `go test` (or manual at review) |

**Race gate (C-13):** `go test -race ./internal/role/... ./internal/pipeline/...` passes. This is the binding gate for the concurrency invariants (NFR-R3, NFR-R4, NFR-P6). A race failure is a blocking defect (C-13).

**Why this shape:** the tests pin invariants, not coverage. Each test maps to a specific NFR's verification procedure. The grep-tests (no-external-clients, dead-code, no-new-deps) make the *absence* threats CI-checkable, not just review-checkable. The race test is authored by the navigator (not the driver) to catch different assumptions (mob-composition §6).

---

## 7. Accepted Residuals

The design accepts two bounded residuals, both documented in the raid-log (R-3) and in the config/README:

| Residual | Bound | Source | Documentation |
|----------|-------|--------|---------------|
| Hard-crash tail loss | ≤ one batch (≤200 ms / ≤8 KB) | R-3 / NFR-R1 | `Streaming` config schema comment; README |
| DB-transient chunk gap (if retry fails) | ≤ one chunk | NFR-R6 / ADR-2 | `log.Printf` at the retry-failure branch; the live UI retains the chunk (from `lineCh`) |

Both residuals are bounded by the flush threshold and are recoverable in the live UI (the chunk was sent to `lineCh` before the DB failure — D-R4). The persisted DB row has the gap; the operator sees the full output live. This is the intended tradeoff (ADR-8: persist-then-push, but the channel send is unconditional on DB failure — the live stream is the priority).

No other residuals are accepted. Data loss on graceful shutdown, stale-concatenation on re-dispatch, immutability violation, shell-injection, external egress — all are blocking defects, not residuals.

---

## 8. Traceability Matrix (Design → NFR → Priors)

| Design block | NFR | ADR | Constraint / FR | Test |
|--------------|-----|-----|-----------------|------|
| D-S1 | NFR-S1 | ADR-9 | C-1, NFR-4 | no_external_clients_test, no_new_deps_test |
| D-S2 | NFR-S2 | ADR-1 | C-8, FR-1 | shell-metachar test |
| D-S3 | NFR-S3 | ADR-1 | FR-1, C-9, NFR-10 | allowlist test, dead-code grep |
| D-S4 | NFR-S4 | ADR-4 | app-design §7 | route audit (review) |
| D-S5 | NFR-S5 | ADR-9 | C-1, NFR-4 | egress grep |
| D-S6 | NFR-S6 | ADR-1 | C-9, NFR-8 | config-load-path grep |
| D-S7 | NFR-S7 | — | DR-1, C-2 | no-new-SQL grep |
| D-S8 | NFR-S8 | ADR-4 | NFR-10, FR-10, FR-18 | dead-code grep |
| D-P1 | NFR-P1 | ADR-8 | C-3, C-4, NFR-1 | D-5 benchmark + runtime trace |
| D-P2 | NFR-P2 | — | C-3, FR-3 | flush-trigger tests |
| D-P3 | NFR-P3 | ADR-7 | C-11, AS-1 | D-5 benchmark |
| D-P4 | NFR-P4 | — | FR-15, O-8 | component test |
| D-P5 | NFR-P5 | ADR-1 | C-3, FR-9 | config-validation tests |
| D-P6 | NFR-P6 | ADR-8 | C-4, C-12 | channel-fill test |
| D-R1 | NFR-R1 | ADR-2 | C-12, FR-6 | cancellation + EOF tests |
| D-R2 | NFR-R2 | ADR-2 | NFR-9 | immutability test + grep |
| D-R3 | NFR-R3 | ADR-2 | C-5, R-1, NFR-5 | race test (navigator) |
| D-R4 | NFR-R4 | ADR-8 | C-4, FR-4 | co-location test + grep |
| D-R5 | NFR-R5 | ADR-8 | FR-17 | reconnect-reconcile component test |
| D-R6 | NFR-R6 | ADR-8, ADR-10 | O-10 | inject-failure tests |
| D-R7 | NFR-R7 | ADR-2 | R-6 | re-dispatch test + grep |
| D-R8 | NFR-R8 | ADR-1, ADR-5 | C-9, NFR-8 | read-path-only grep |

---

## 9. Notes for Construction (stage 3.4+)

1. **The structure is the control; the tests are the regression guard.** The drain ordering (D-R3), the dual-write elimination (ADR-2), the no-shell construction (D-S2), the no-output-files path (D-S3) — these are structural. The tests pin them so a future edit doesn't silently break them. Construction must not substitute a test for a structural control (e.g., don't keep `SaveStageLogForBolt` at completion and "just add a test that it doesn't race" — ADR-2 removed it for a reason).

2. **The grep-tests are CI-checkable invariants.** `no_external_clients_test.go`, `dead_code_test.go`, `no_new_deps_test.go` wrap a grep in a Go test so the absence threat runs on every CI build, not just at review time. Construction should implement these as Go tests (using `os.ReadFile` + `strings.Contains` or `filepath.Walk`), not as shell scripts in a Makefile.

3. **The D-5 micro-benchmark is the single pre-flight measurement.** If planning (2.x) did not run it, the first construction bolt runs it and records the results in its commit message. The preliminary thresholds (200 ms / 8 KB) ship with a documented deferral if the benchmark is not run before the batcher is implemented.

4. **The race test is navigator-authored.** The driver implements the batcher; the navigator authors the race test independently. This is a mob-composition constraint (§6), not a suggestion — a different author catches different assumptions about the drain window.

5. **The retry-once is a single retry, not a policy.** 50 ms backoff, one attempt, then log-and-continue. No exponential backoff, no retry count config, no circuit breaker. The feature has no downstream service to break against (ADR-10); a hard DB failure is a process-level event.

6. **The reconnect reconcile is UI-side.** The backend does not track "what the UI missed." The UI re-reads the DB on reconnect and reconciles by content-prefix match. The DB is the authority (NFR-R2).

7. **The `source` field is the only API shape change.** `getStageLog` returns `{content, source}` additively. No version bump, no deprecation. The UI defaults to `"db"` if `source` is absent (back-compat for any uncached client).

8. **The escape hatches are documented, not implemented.** The chunk-table refactor (C-11) swaps the `FlushFunc` closure body (ADR-7) — the seam exists. The `/metrics` endpoint (ADR-9) wires `GetStageLogMeta` — the method exists. Both are future features, not in-feature work.