# Dev Team Refactor Plan — Simplify Dispatch & Status

## Diagnosis: Why It Doesn't Work

After reading `pipeline.go`, `process.go`, `dispatcher.go`, `tmux.go`, `server.go`, `gate.go`, `outcome.go`, and `state.go`:

### Dispatch is broken

1. **Output capture loses most output.** `tmux.go:169` uses `capture-pane -S -500` — only last 500 lines. Long agent runs scroll past this. `pipe-pane` writes full output to `logs/<phase>-<role>.log` but `getCapturedOutput` (`server.go:801`) reads from `capture-pane`, not the log file. The audit trail exists; the UI never sees it.

2. **Output diffing is incorrect.** `tmux.go:230` slices `captured[lastCaptureLen:]` to find new content. Pane reflows, line wrapping, or scrollback reordering makes `lastCaptureLen` point at the wrong byte offset. Lines get dropped or duplicated. The `default:` non-blocking send on `lineCh` (`tmux.go:241`) drops lines when the consumer is slow.

3. **Liveness check conflates "no stdout" with "hung."** `staleTimeout = 5 * time.Minute` kills sessions that produce no output. LLM calls (thinking, tool use) emit nothing for minutes. Real agent runs get killed mid-thought.

4. **Exit code ignored.** `tmux.go:206` sets `result.Success = true` whenever any output was captured, regardless of process exit status. `dispatchDirect` has the same bug (`dispatcher.go:208`). A crashed agent reports success.

5. **`dispatchDirect` is dead code.** `NewDispatcher` always constructs a `TmuxSessionManager` (`dispatcher.go:53`). The `if d.tmux != nil` branch (`dispatcher.go:80`) is always taken. ~130 lines of unused pipe-based dispatch.

6. **`DispatchCrossRepo` is unused.** `dispatcher.go:248` appends a string to context. No caller in production paths.

7. **Temp context dir lifecycle is fragile.** `dispatcher.go:109` and `tmux.go:54` do `defer os.RemoveAll(contextDir)`. For tmux path this is fine (function blocks until session ends), but the pattern invites a future bug where the session outlives the dir.

### Status is fragmented across 6 code paths

The "dispatch → gate → advance" sequence is reimplemented in **six places** with subtly different behavior:

| Location | What it does | Difference |
|---|---|---|
| `process.go:ProcessAsync` | Autopilot loop | Re-evaluates gate after `RunPhaseWithAgentStreaming` already did |
| `pipeline.go:RunPhaseWithAgentStreaming` | Recursive auto-advancer | Recursively calls itself for advance/recirculate — unbounded stack |
| `server.go:createFeature` goroutine | Auto-start inception | Doesn't auto-advance past inception |
| `server.go:runPhase` goroutine | Single-phase run | Passes `autoAdvance=true` despite being "single-phase" |
| `server.go:processFeature` goroutine | Autopilot via ProcessAsync | Duplicates event broadcasting |
| `server.go:answerQuestion` | Resume after Q&A | Reimplements the full dispatch+gate+advance sequence inline |
| `server.go:resumeOrphanedFeatures` | Restart on boot | Blindly re-dispatches, circuit breaker is the only guard |

Each duplicates: SSE broadcasting, gate evaluation, feature reload, state save, error handling. Bugs fixed in one path often miss the others.

### Gate evaluation is broken and duplicated

1. **Gate runs 2–3 times per phase.** `RunPhaseWithAgentStreaming` evaluates via `ParseOutcome` (agent's `outcome.txt`), then again via `NewGateEvaluatorWithCommit` as "safety check" (`pipeline.go:923`). `ProcessAsync` evaluates a **third** time (`process.go:256`). They can disagree; the agent's self-reported `pass` overrides the gate failure.

2. **Gate checks are substring matching.** `gate.go:97` checks `"spec.md contains at least one user story"` via `strings.Contains(content, "User Stor")`. An agent writing the literal string "User Stor" anywhere passes. Every check in the 600-line `evaluateDesc` switch is the same pattern. The gate is security theater.

3. **Build/test gates run in the wrong dir.** `checkBuildCompiles` runs `go build ./...` in `workDirOr(f)`, which is the spec worktree for spec-only features (no `go.mod`) or the first impl repo for multi-repo features (second repo never built). `checkTestSuitePasses` runs `go test ./...` with a 120s timeout — large projects time out and the gate fails for unrelated reasons.

4. **`cleanPhaseArtifacts` deletes DB artifacts before each run** (`pipeline.go:539`). On recirculation, the agent re-running a phase loses access to its own prior artifacts via `BuildCrossRepoContext`. Inputs are wiped before the agent starts.

### State is in 4 places, inconsistent

| Storage | What | Source of truth for |
|---|---|---|
| SQLite `.devteam.db` | features, questions, notes, events, gate_results, artifacts | API reads (questions, notes, artifacts) |
| `specs/<id>/.devteam-state.yaml` (worktree) | Feature struct (phase, status, prepared repos) | Agent reads via CLI |
| `specs/<id>/.devteam-state.yaml` (primary checkout) | Mirror copy | `ListFeatures` walks this |
| tmux session `devteam-<id>` | Agent liveness | `IsSessionAlive`, `CaptureOutput` |
| `logs/<phase>-<role>.log` | Full agent output | Nothing reads it |

`syncFeatureToDB` does `INSERT OR REPLACE` on every state change, but `ListFeatures` reads from disk. DB and disk drift. Restarting the server `resumeOrphanedFeatures` walks disk features, finds `in_progress` ones, re-dispatches — even if DB says something different.

### Resume logic is racy and dangerous

- `RestoreActiveProcesses` (`server.go:51`) restores `activeProcess` from tmux sessions, then `resumeOrphanedFeatures` finds features with `status=in_progress` and no tmux session, and re-dispatches them in `single-phase` mode. But `single-phase` mode passes `autoAdvance=true` to `RunPhaseWithAgentStreaming` (`server.go:144`), so it's not actually single-phase.
- Circuit breaker: `ResumeCount >= 3` fails the feature. But `resumeOrphanedFeatures` increments `ResumeCount` (`server.go:112`) without saving the feature first — the increment is lost if the goroutine crashes before `SaveFeatureState`.
- No idempotency: if the server crashes during a phase, restart re-runs the phase from scratch, re-dispatching the agent, re-spending LLM credits, potentially duplicating work the agent already committed.

---

## Refactor Plan

### Guiding principles

1. **One code path** for dispatch → gate → advance. No six reimplementations.
2. **One source of truth** for feature state. SQLite is it; YAML mirror deleted.
3. **Agent is the evaluator.** Drop the substring gate. The agent signals pass/recirculate via CLI; the platform trusts it. Safety net is a single smoke check (did the agent touch any files?), not 40 substring checks.
4. **Output is a log file, not a tmux pane.** Read the file for status; tmux is only for live tail.
5. **No recursive auto-advance.** One phase per dispatch. The loop in `ProcessAsync` advances explicitly.

### Phase 1 — Consolidate dispatch (the big one)

**Delete:**
- `dispatcher.go:dispatchDirect` (dead code, ~130 lines)
- `dispatcher.go:DispatchCrossRepo` (unused)
- `pipeline.go:RunPhaseWithAgent` (only called by tests; rewrite tests to use the streaming version)
- `server.go:createFeature` auto-start goroutine — replace with a call to the unified processor (Phase 3)
- `server.go:runPhase` goroutine body — replace with unified processor call
- `server.go:answerQuestion` inline dispatch logic — replace with unified processor call
- `server.go:resumeOrphanedFeatures` inline dispatch — replace with unified processor call

**Rewrite `tmux.go:DispatchStreaming`:**

```
- Drop capture-pane polling entirely.
- tmux session runs: `opencode run ... 2>&1 | tee -a <logfile>`
- pipe-pane logs to the same file (belt and suspenders).
- Dispatcher waits for session to exit via `tmux wait-for` (or poll `has-session` every 1s — simpler).
- On exit, read exit code from a marker file the wrapper writes:
    `opencode run ... ; echo $? > <contextDir>/exit_code`
- Set result.Success = (exit_code == "0").
- Stream output by tailing the log file (follow mode), not by diffing capture-pane.
- Drop the 5-minute stale timeout. Replace with: kill only on ctx cancellation.
    The agent is the evaluator; if it hangs, the user cancels via UI.
- Result.Output = full log file content (capped at e.g. 256KB for the API response).
```

**Add `RunPhase` — the single entry point** (replaces `RunPhaseWithAgentStreaming`):

```go
// RunPhase dispatches the agent for the current phase, waits for it to exit,
// reads the outcome, and returns. Does NOT auto-advance, does NOT recurse.
// The caller (ProcessAsync or single-phase handler) decides what to do next.
func (p *Pipeline) RunPhase(ctx context.Context, f *feature.Feature, onOutput OutputLineCallback) (*RunResult, error)
```

- Build context once (role instructions + rules + spec context + notes + revision notes + outcome instructions).
- Dispatch via `dispatcher.DispatchStreaming`.
- Read outcome via `ParseOutcome` (agent CLI signal). If no outcome file, default to `pass` — **drop the safety gate re-evaluation**. The agent is the evaluator.
- Save state once. Return.

**Result:** dispatch is one function, ~150 lines. Output is complete. Exit code is respected.

### Phase 2 — Collapse the gate

**Delete `gate.go` entirely** (~1000 lines of substring matching).

**Replace with `smoke.go` (~80 lines):**

```go
// SmokeCheck runs after the agent signals pass. It catches the obvious failure
// mode: the agent claimed pass but did nothing. Returns nil if the agent touched
// files or produced artifacts; returns a list of reasons otherwise.
func (p *Pipeline) SmokeCheck(f *feature.Feature, phase feature.Phase, preDispatchCommit string) []string
```

Checks (phase-dependent):
- **Inception/Planning:** required artifacts exist in DB (non-empty content). That's it.
- **Construction:** `git diff --name-only <preDispatchCommit>` returns at least one non-spec file.
- **Review:** `review_report` artifact exists and mentions at least one file path.
- **Testing:** `test_report` artifact exists AND `git diff` shows new test files.
- **Delivery:** `docs` artifact exists.

If smoke check fails, treat as recirculate (back to current phase with the smoke failures as revision notes). No 40-check gauntlet. The agent is trusted; the smoke check catches "agent did nothing" and "agent wrote a stub report."

**Delete `GateDefinitions` in `state.go`** — no more `ValidationDescs`. Keep `RequiredArts` only (used by smoke check).

**Delete `outcome.go:formatGateFailureAsNotes`** — no gate result to format. Smoke failures format directly.

### Phase 3 — Unify the processing loop

**Replace `ProcessAsync` + the 5 inline dispatch sites with one processor:**

```go
// Processor runs phases for a feature until it stops (pass without auto-advance,
// recirculate, needs_feedback, failed, or delivery complete).
// Emits events to eventCh. The caller owns the loop's lifecycle.
func (p *Pipeline) Process(ctx context.Context, f *feature.Feature, eventCh chan<- ProcessEvent, onOutput OutputLineCallback) error
```

Loop body:
1. If `f.Status == waiting_for_feedback`: check pending questions; if zero, resume; else return (wait for user).
2. Prepare impl repos if entering construction (existing logic, keep).
3. Emit `agent_dispatch`.
4. `RunPhase(ctx, f, onOutput)` — single call, no recursion.
5. Read outcome.
6. Emit `agent_complete` + `gate_result` (smoke check result).
7. Branch on outcome:
   - `pass` + not delivery → advance, loop.
   - `pass` + delivery → mark done, create PR, return.
   - `recirculate` → write revision notes, set `f.Current = target`, loop.
   - `needs_feedback` → `WaitForHuman`, return (user answers via API, server restarts processor).
   - `failed` → return error.
   - smoke failed → treat as recirculate to current phase.
8. Reload feature from DB (single source of truth — see Phase 4).

**All six server goroutines become:**

```go
go func() {
    defer s.activeProcess.Delete(id)
    eventCh := make(chan pipeline.ProcessEvent, 100)
    onOutput := s.makeSSEOutputBroadcaster(id)
    err := s.pipeline.Process(ctx, f, eventCh, onOutput)
    s.drainEvents(eventCh)  // broadcast remaining
    if err != nil { s.broadcastSSE(id, "error", ...) }
}()
```

One code path. No duplication.

**Auto-advance behavior:** `Process` always auto-advances on `pass`. The `single-phase` vs `autopilot` distinction goes away — a "single phase run" is just `Process` that the user stops after one phase (via cancel). Simpler mental model.

If the user wants explicit per-phase control: add a `maxPhases int` parameter (0 = unlimited). One-phase run = `maxPhases=1`. Drop the mode string entirely.

### Phase 4 — Single source of truth for state

**SQLite is the only store for feature state.** Delete the YAML mirror.

- `ListFeatures` → `SELECT * FROM features ORDER BY created_at`.
- `GetFeature` → `SELECT * FROM features WHERE id = ?`.
- `SaveFeature` → `UPDATE features SET ...`.
- `syncFeatureToDB` → becomes `SaveFeature` (no more "sync" — it's the save).
- `EnsureSpecWorktree` → stop writing `.devteam-state.yaml` to primary checkout and worktree. The worktree gets spec artifacts only (via DB read into CONTEXT.md), not state files.

**Migration:** add a one-time migration that reads all `specs/<id>/.devteam-state.yaml` files and inserts them into the `features` table (idempotent — skip rows that exist). Then delete the YAML files.

**`PreparedRepos` storage:** currently on the YAML struct. Move to a `feature_repos` table: `(feature_id, name, url, dir, branch)`. `PrepareImplRepos` writes to this table; `dispatchWorkingDirForPhase` reads from it.

**Questions, notes, events, gate_results, artifacts:** already in SQLite. Keep.

**What stays on disk:**
- `specs/<id>/CONTEXT.md` — written per dispatch, read by agent. Ephemeral, not state.
- `specs/<id>/REVISION_NOTES.md` — written on recirculate, read by next dispatch. Ephemeral.
- `specs/<id>/GATE_FAILURE.md` — delete this entirely (smoke failures go to DB as revision notes).
- `logs/<phase>-<role>.log` — agent output audit trail. Keep.

### Phase 5 — Status checking that works

**`getCapturedOutput` (`GET /api/features/{id}/output`):**
- If processing: return last N lines of the log file (default 200, query param `?lines=`). Read from `logs/<current-phase>-<current-role>.log`.
- If not processing: return empty (or the last log if we want history — configurable).

**SSE:**
- Keep `broadcastSSE`. Drop the 200-event buffer for `agent_output` lines (they're ephemeral; late joiners read the log file via `/output` endpoint). Keep buffer for lifecycle events (`phase_change`, `gate_result`, `agent_dispatch`, `agent_complete`).
- Fix `addSSEClient`/`removeSSEClient` — replace the `sync.Map` + mutex hack with a plain `map[string][]chan SSEMessage` guarded by a single `sync.Mutex` field on `Server`. Simpler, correct.

**`RestoreActiveProcesses`:**
- Drop `resumeOrphanedFeatures` entirely. On startup:
  - Restore `activeProcess` from tmux sessions (existing logic).
  - For features with `status=in_progress` and no tmux session: mark `status=failed` with a note "interrupted by server restart." User re-runs manually. **Do not auto-resume.** Auto-resume burns credits and duplicates work; the user should decide.
- Circuit breaker becomes unnecessary (no auto-resume).

**`IsProcessing`:** unchanged — `activeProcess` map lookup.

### Phase 6 — Clean up the API surface

**Endpoints to keep (unchanged behavior, simplified handlers):**
- `GET /api/features`, `POST /api/features`, `GET /api/features/{id}`
- `POST /api/features/{id}/run` — calls `Process` with `maxPhases=1`
- `POST /api/features/{id}/process` — calls `Process` with `maxPhases=0`
- `POST /api/features/{id}/advance` — manual advance (no auto-run)
- `POST /api/features/{id}/recirculate` — manual recirculate
- `POST /api/features/{id}/cancel` — kills tmux session, marks cancelled
- `GET /api/features/{id}/gate` — returns last smoke check result from DB
- `GET /api/features/{id}/output` — reads log file (Phase 5)
- `GET /api/features/{id}/stream` — SSE (Phase 5)
- Artifact, question, note, signal endpoints — unchanged

**Endpoints to drop:**
- `GET /api/features/{id}/sessions` — tmux session list is internal, not useful to UI
- `GET /api/metrics/sessions` — unused

**CLI (`devteam` binary):** no public-facing changes. `signal`, `questions`, `notes`, `artifact` commands work the same. Internal `run`/`gate`/`advance` commands call the same handlers.

---

## Execution Order

Do in this order. Each phase is independently shippable.

1. **Phase 1 (dispatch)** — biggest behavior fix. Output capture works, exit codes respected. ~2 days.
2. **Phase 4 (state)** — unblocks Phase 3 (single source of truth). ~1 day + migration.
3. **Phase 2 (gate)** — delete gate.go, add smoke.go. ~1 day.
4. **Phase 3 (unify loop)** — depends on 1, 2, 4. Replace 6 dispatch sites with `Process`. ~2 days.
5. **Phase 5 (status)** — depends on 1 (log file) and 3 (event flow). ~1 day.
6. **Phase 6 (API cleanup)** — depends on 3. ~0.5 day.

**Total: ~7-8 days.** Each phase lands as a separate PR/commit, tests pass between phases.

---

## What gets simpler (measurable)

| Metric | Before | After |
|---|---|---|
| Code paths for "dispatch + gate + advance" | 6 | 1 |
| Gate check functions | ~40 substring checks (~1000 lines) | 5 smoke checks (~80 lines) |
| Feature state stores | 4 (DB + 2 YAML + tmux) | 2 (DB + tmux) |
| `RunPhaseWithAgentStreaming` | ~350 lines, recursive | `RunPhase` ~150 lines, non-recursive |
| Output capture | 500-line pane, diffed by length | Full log file, tailed |
| Auto-resume on restart | Yes (racy, credit-burning) | No (user-driven) |
| Dead code removed | `dispatchDirect`, `DispatchCrossRepo`, `RunPhaseWithAgent`, `formatGateFailureAsNotes`, `writeGateFailure`, `checkFrontendTests` (unused), `GateDefinitions.ValidationDescs` | — |

---

## Risks & Mitigations

1. **Dropping the gate gauntlet lets bad agents pass.** Mitigation: smoke check catches "did nothing." For higher assurance, the reviewer agent (phase 4) is the real gate — adversarial review is stronger than substring matching ever was. The platform never had real gates; it had the illusion of gates. Removing the illusion is honest.

2. **No auto-resume frustrates users.** Mitigation: UI shows "interrupted by restart" with a "Re-run" button. One click. Better than silent credit burn and duplicated work.

3. **Migration risk (YAML → DB).** Mitigation: migration is idempotent and runs once. If it fails, the YAML files are still on disk — re-run migration. Add a `devteam migrate-state` CLI command for manual runs.

4. **`Process` loop change breaks existing in-flight features.** Mitigation: ship Phase 4 (state) first — features resume from DB regardless of code path. Then Phase 3. A feature mid-pipeline when the refactor lands will resume on next dispatch via the new `Process` loop.

5. **Tmux `wait-for` requires tmux 3.0+.** Mitigation: poll `has-session` every 1s as fallback (current behavior, minus the capture-pane diffing). Detect tmux version on startup.

---

## Decisions (confirmed)

1. **Keep all 6 phases.** No phase merging. Smoke checks adapt per phase.

2. **No local file writes for state.** Delete `outcome.txt`, `.devteam-state.yaml`, `CONTEXT.md`, `REVISION_NOTES.md`, `GATE_FAILURE.md` from disk. Agent signals via `devteam signal` CLI → DB column. Pipeline reads outcome from DB. Revision notes stored in DB `notes` table, injected into next dispatch's prompt string. CONTEXT is the `req.Context` string passed to opencode (already works this way — the disk write was redundant).

3. **Build/test every prepared repo.** Smoke check iterates `feature_repos` table, runs the project's build/test command in each repo's worktree dir. Slow but correct — no repo skipped.

4. **Drop autopilot on backend.** Delete `ProcessAsync`, `POST /process` endpoint, and `activeProcess` mode string. Server dispatches one phase per `POST /run` call. UI drives multi-phase progression by calling `/run` repeatedly (client-side loop). Server stays stateless per-phase. `POST /process` endpoint removed.