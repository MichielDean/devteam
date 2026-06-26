# Review Report

**Feature**: playwright-e2e (GET /api/health)
**Reviewer**: Code Reviewer
**Phase**: review
**Date**: 2026-06-25
**Reviewed commit**: HEAD (cd8ccd5) — supersedes stale report deleted in working tree.

## Summary

- Acceptance criteria: 13 total, 13 MET, 0 NOT MET
- Constraints: 7 total, 7 MET, 0 NOT MET
- Findings: 0 blocking, 0 required, 1 noted
- Implementation LOC: ~30 (handler 14 + struct 4 + accessor 1 + route 1 + Playwright 9) + 248 test LOC — minimal, no over-engineering
- **Gate: PASSED**

---

## Phase 1: Constraint Register Review (MANDATORY FIRST)

### CON-001 — Route via Go 1.22+ method-pattern `mux.HandleFunc("GET /api/health", ...)`
- **Source**: repo convention, server.go:160-188
- **Status**: MET
- **Trace**:
  1. `NewServer` constructs `mux := http.NewServeMux()` then registers feature routes (:176-188)
  2. `server.go:190` — `mux.HandleFunc("GET /api/health", s.healthHandler)` — exact Go 1.22+ method-pattern form
  3. Registered BEFORE `staticFS` catch-all at `:192-194` (`mux.Handle("/", s.spaHandler(staticFS))`) — order correct, no shadowing ✓
- **Evidence**: `internal/api/server.go:190` — `mux.HandleFunc("GET /api/health", s.healthHandler)`

### CON-002 — Health route covered by recoveryMiddleware + corsMiddleware
- **Source**: repo convention, server.go:194
- **Status**: MET
- **Trace**:
  1. `:190` health route registered on `mux`
  2. `:196` — `handler := s.recoveryMiddleware(s.corsMiddleware(mux))` — wraps entire mux; health route cannot bypass ✓
  3. `recoveryMiddleware` (:228-237) has `defer recover()` → on panic: `log.Printf` + `writeError(w, http.StatusInternalServerError, "internal_error", ...)` ✓
  4. Recovery is OUTERMOST (wraps CORS which wraps mux) — panics in CORS or handler both caught ✓
  5. Test `TestHealthPanicReturns500` (server_test.go) induces panic via a `panicMux` wrapped by the same middleware chain, asserts 500, then asserts a fresh non-panicking server returns 200 (server alive) ✓
- **Evidence**: `internal/api/server.go:196`, `:228-237`; `internal/api/server_test.go` `TestHealthPanicReturns500`

### CON-003 — version sourced from Config, not hardcoded
- **Source**: repo convention, config.go
- **Status**: MET
- **Trace**:
  1. `healthHandler` `:919-923` builds `healthResponse{Status: "ok", Version: s.pipeline.Config().Version}`
  2. `Pipeline.Config()` `pipeline.go:44` — `func (p *Pipeline) Config() *config.Config { return p.config }`
  3. `p.config` set in ALL 3 constructors: `NewPipeline:285`, `NewPipelineWithDispatcher:319`, `NewPipelineWithQuestionStore:334` — each `config: cfg` ✓
  4. No hardcoded "1.0" literal in handler — only `Status: "ok"` is literal; version is dynamic ✓
  5. Test `TestHealthVersionFromConfig` constructs `Config{Version:"9.9.9-test"}` via `setupHealthTestServer` and asserts body `{"status":"ok","version":"9.9.9-test"}` ✓
- **Evidence**: `internal/api/server.go:922`; `internal/pipeline/pipeline.go:44,285,319,334`; `server_test.go` `TestHealthVersionFromConfig`

### CON-004 — httptest in-process server pattern
- **Source**: repo convention, server_test.go
- **Status**: MET
- **Trace**:
  1. `setupHealthTestServer` builds `httptest.NewServer(s.httpServer.Handler)` — in-process, no external spawn ✓
  2. All 11 `TestHealth*` functions use it (or build their own `httptest.NewServer` in the panic test) ✓
  3. Helper `healthBody` reads body via `io.ReadAll` + `strings.TrimSpace` (trims json.Encoder trailing newline) ✓
- **Evidence**: `internal/api/server_test.go` `setupHealthTestServer`, `healthBody`, `TestHealthGETReturns200AndBody`..`TestHealthTrailingSlash404`

### CON-005 — E2E on :18765 via Playwright baseURL
- **Source**: repo convention, ui/e2e, AGENTS.md :18765
- **Status**: MET
- **Trace**:
  1. `ui/e2e/health.spec.ts:4` — `await request.get('/api/health')` (relative URL)
  2. `request` fixture resolves against `baseURL`; `ui/playwright.config.ts:16` — `baseURL: process.env.BASE_URL || 'http://localhost:18765'` ✓
  3. `webServer` (:19-31) starts `~/go/bin/devteam -http :18765` on port 18765 (not prod :8765) ✓
  4. No hardcoded `http://localhost:18765` in the spec — uses baseURL as plan required ✓
- **Evidence**: `ui/e2e/health.spec.ts:4`; `ui/playwright.config.ts:16,24,26`

### CON-006 — exact body `{"status":"ok","version":"1.0"}`
- **Source**: input.md idea
- **Status**: MET
- **Trace**:
  1. `healthResponse` struct `:912-915` — fields ordered `Status` then `Version`, JSON tags `"status"`, `"version"`
  2. `writeJSON:900-904` — `json.NewEncoder(w).Encode(data)` emits keys in struct field order → `{"status":"ok","version":"1.0"}` (+ trailing newline) ✓
  3. Test `TestHealthGETReturns200AndBody` asserts `healthBody(...) != \`{"status":"ok","version":"1.0"}\`` — byte-exact (after trim) ✓
- **Evidence**: `internal/api/server.go:912-915`, `:900-904`; `server_test.go` `TestHealthGETReturns200AndBody`

### CON-007 — 405 for non-GET (RFC 9110 §15.5.5)
- **Source**: HTTP semantics
- **Status**: MET
- **Trace**:
  1. Only `GET /api/health` registered (`:190`); no other method routes
  2. Go 1.22+ ServeMux method-pattern: POST/PUT/DELETE/PATCH → 405 automatically (with `Allow` header) ✓
  3. `/api/health/` (trailing slash) not registered → 404 ✓
  4. Tests: `TestHealthPOSTReturns405`, `TestHealthPUTReturns405`, `TestHealthDELETEReturns405`, `TestHealthPATCHReturns405` each assert `http.StatusMethodNotAllowed`; `TestHealthTrailingSlash404` asserts `http.StatusNotFound` ✓
- **Evidence**: `internal/api/server.go:190`; `server_test.go` `TestHealthPOSTReturns405`, `TestHealthPUTReturns405`, `TestHealthDELETEReturns405`, `TestHealthPATCHReturns405`, `TestHealthTrailingSlash404`

---

## Phase 2: Acceptance Criteria Review

### AC-001: GET /api/health → 200, application/json, byte-exact body `{"status":"ok","version":"1.0"}`
- **Status**: MET
- **Evidence**: `server.go:190` route; `:919-924` handler; `:901` `w.Header().Set("Content-Type", "application/json")`; struct field order guarantees body. `server_test.go` `TestHealthGETReturns200AndBody` asserts 200 + `Content-Type` contains `application/json` + byte-exact body.

### AC-002: GET with no body → 200 same body (no r.Body decode)
- **Status**: MET
- **Evidence**: `server.go:919-923` — `healthHandler` never references `r.Body`. `TestHealthGETNoRequestBody` issues GET with `nil` body, asserts 200 + standard body.

### AC-003: custom Config.Version="9.9.9-test" → body reflects it
- **Status**: MET
- **Evidence**: `server.go:922` — `Version: s.pipeline.Config().Version` (dynamic). `TestHealthVersionFromConfig` builds `Config{Version:"9.9.9-test"}`, asserts body `{"status":"ok","version":"9.9.9-test"}`.

### AC-004: GET /api/health?cb=123 → 200 standard body (query ignored)
- **Status**: MET
- **Evidence**: `healthHandler` ignores `r.URL.RawQuery`; ServeMux matches path regardless of query string. `TestHealthGETIgnoresQueryParams` hits `?cb=123`, asserts 200 + `{"status":"ok","version":"1.0"}`.

### AC-005: handler panic → 500, server survives
- **Status**: MET
- **Evidence**: `server.go:228-237` recovery middleware wraps mux (`:196`). `TestHealthPanicReturns500` builds a `panicMux` with a panicking handler, wraps via `s.recoveryMiddleware(s.corsMiddleware(panicMux))`, asserts 500, then hits a fresh non-panicking server and asserts 200 (process alive).

### AC-006: POST → 405
- **Status**: MET — `TestHealthPOSTReturns405` asserts `http.StatusMethodNotAllowed`.

### AC-007: PUT → 405
- **Status**: MET — `TestHealthPUTReturns405` asserts 405.

### AC-008: DELETE → 405
- **Status**: MET — `TestHealthDELETEReturns405` asserts 405.

### AC-009: PATCH → 405
- **Status**: MET — `TestHealthPATCHReturns405` asserts 405.

### AC-010: GET → 200 (positive control)
- **Status**: MET — `TestHealthGETStill200After405s` asserts 200.

### AC-011: GET /api/health/ (trailing slash) → 404
- **Status**: MET — `TestHealthTrailingSlash404` asserts `http.StatusNotFound`. Exact-path registration confirmed at `:190`.

### AC-012: Playwright E2E asserts 200 + `{status:"ok", version:"1.0"}`
- **Status**: MET
- **Evidence**: `ui/e2e/health.spec.ts:4-8` — asserts `response.status()===200`, `body.status==='ok'`, `body.version==='1.0'`. Uses `request` fixture (baseURL-resolved).

### AC-013: suite discovers + passes health spec (no .skip)
- **Status**: MET (structural — execution is Tester's job)
- **Evidence**: `ui/e2e/health.spec.ts:3` uses `test(...)` (not `test.skip`); file under `ui/e2e/` matches `testDir: './e2e'` (`playwright.config.ts:10`). Will be discovered.

---

## Phase 3: Negative Test Vector Verification

| Vector | Impl rejects? | Test? | Status |
|---|---|---|---|
| POST /api/health → 405 | YES (stdlib method-pattern) | `TestHealthPOSTReturns405` | MET |
| PUT → 405 | YES | `TestHealthPUTReturns405` | MET |
| DELETE → 405 | YES | `TestHealthDELETEReturns405` | MET |
| PATCH → 405 | YES | `TestHealthPATCHReturns405` | MET |
| /api/health/ (trailing slash) → 404 | YES (exact path) | `TestHealthTrailingSlash404` | MET |
| handler panic → 500 | YES (recovery middleware) | `TestHealthPanicReturns500` | MET |

All negative vectors rejected with correct response; each has a dedicated test.

---

## Phase 4: Cross-Component Consistency

Feature has single producer per shared value (per plan.md matrix). Verified:
- **`version` string**: `config.Config.Version` → `Pipeline.Config()` → `healthHandler` → response. Single path, no transform. Consistent. Verified by `TestHealthVersionFromConfig`.
- **Response JSON shape**: `healthResponse` struct field order (Status, Version) = JSON key order = byte-exact expectation. Consistent. Verified by `TestHealthGETReturns200AndBody`.
- **405 method set**: stdlib emits 405 for exactly POST/PUT/DELETE/PATCH (only GET registered). Consistent with AC-006..009.
- **500 panic path**: `recoveryMiddleware` outermost, covers all routes. Consistent with AC-005.

No multi-provider split → no cross-component inconsistency possible. No findings.

---

## Phase 5: Language-Specific Footgun Review

**Go**:
- Nil map writes: none — handler does no map writes.
- `s.pipeline.Config()` could return nil only if a constructor didn't set `config:` — verified all 3 set it (`pipeline.go:285,319,334`). Safe.
- `s.pipeline` set in `NewServer` before any request served. Safe.
- `Config().Version` deref: if `Config()` returned nil this would panic, but all constructors set `config: cfg` with a non-nil cfg (callers pass `&config.Config{...}`). Safe.
- Integer overflow / modulo: none — handler does no arithmetic.
- No nil-channel/interface-nil pitfalls.

**TypeScript** (Playwright spec):
- No `any` — `body` typed via `await response.json()`.
- Uses `expect().toBe()` (strict equality, no `==`).
- No optional chaining hiding null.

**No footguns.** No findings.

---

## Phase 6: Spec-Implementation Drift (Plan vs Spec)

- Every user story in spec.md has corresponding tasks in tasks.md and code: US-1 → handler+route+tests, US-2 → 405 tests, US-3 → Playwright spec.
- Every acceptance criterion has a done condition and implementation/test evidence.
- No tasks trace to no user story (no scope creep).
- No user stories lack tasks (no missing implementation).

No drift.

---

## Over-Engineering Check

| Component | Plan estimate | Actual | Verdict |
|---|---|---|---|
| healthHandler | ~5 LOC | 6 LOC (919-924) | minimal |
| healthResponse struct | — | 4 LOC (912-915) | minimal, needed for CON-006 field order |
| Pipeline.Config() accessor | 1 LOC | 1 LOC (pipeline.go:44) | minimal |
| Route registration | 1 line | 1 line (server.go:190) | minimal |
| Playwright spec | ~9 LOC | 9 LOC | minimal |
| Go tests | 11 funcs + 2 helpers | 248 LOC | appropriate — one test per AC, no redundant suites |

No abstractions, no factories, no speculative config, no dead code. Implementation is the minimum that satisfies the done conditions. `setupHealthTestServer` helper reuses the existing `setupTestServer` shape rather than inventing a new pattern — correct.

---

## Security Review

P3 feature — security extension not mandatory. Reviewed for common issues:
- **Auth**: none on `/api/health`, consistent with all existing `/api/*` endpoints (FR-007). No auth bypass — there's no auth to bypass. Correct by design.
- **Input validation**: GET, no body decode, no query parsing. Nothing to validate. Handler reads only `config.Version`. Correct.
- **Output filtering**: response exposes only `status:"ok"` + config version. Version is already public via service behavior. No sensitive data leak.
- **Error messages**: recovery middleware returns generic `"An unexpected error occurred"` (`:233`), no stack trace in response (only in `log.Printf`). Correct.
- **CORS**: `corsMiddleware` present in chain (`:196`). Unchanged by feature.
- **Rate limiting**: N/A — liveness probe; not in scope.

No security findings.

---

## Constitution Compliance

No `constitution.md` at repo root or `.specify/constitution.md` (verified by spec §Constitution Compliance). No principles to check. N/A.

---

## Null Pointer Safety

- `s.pipeline`: set in `NewServer` before handlers reachable. Verified.
- `s.pipeline.Config()` → `p.config`: set in all 3 Pipeline constructors. Verified.
- `Config().Version`: string field, zero-value `""` if config has empty version — spec assumption says response reflects `""` faithfully. Not a nil deref.
- No JSON arrays/maps in response (single flat object). No null-vs-empty-array bug possible.
- No map writes in handler. No nil-map panic possible.

No null-safety findings.

---

## Error Path Coverage

| Path | Code | Test |
|---|---|---|
| 200 happy GET | `:920` `writeJSON(w, 200, ...)` | `TestHealthGETReturns200AndBody` |
| 400 invalid input | N/A — GET, no input parsed | N/A |
| 404 missing path (`/api/health/`) | exact-path registration → ServeMux 404 | `TestHealthTrailingSlash404` |
| 405 wrong method | stdlib method-pattern | `TestHealthPOSTReturns405` etc. |
| 500 panic | `recoveryMiddleware:233` | `TestHealthPanicReturns500` |
| Empty state | N/A — single object, no collection | N/A |

All applicable error paths covered.

---

## Middleware Chain

- `:196` — `handler := s.recoveryMiddleware(s.corsMiddleware(mux))`
- Recovery is OUTERMOST (catches panics in CORS + all handlers including health) ✓
- CORS present (unchanged by feature) ✓
- Request body size limit: not set in this feature — pre-existing repo state, not a regression introduced here.
- Health route registered on `mux` before the `staticFS` catch-all (`:192-194`) — not shadowed ✓

No middleware findings.

---

## Findings

### F-001 (noted): `Config()` returns `*config.Config` with no nil guard
- **Severity**: noted — does not need fixing
- **Criterion**: CON-003 (consistency)
- **Code**: `internal/pipeline/pipeline.go:44` — `func (p *Pipeline) Config() *config.Config { return p.config }`
- **Description**: If a caller constructed a `Pipeline` without going through the 3 existing constructors, `p.config` would be nil and `healthHandler`'s `s.pipeline.Config().Version` would panic (caught by recovery middleware → 500, so still safe at runtime). All 3 existing constructors set `config: cfg`, so under current usage this is unreachable. The recovery middleware converts any such panic to 500, so no crash risk.
- **Why noted, not required**: All production paths use the constructors. The panic path is covered by recovery middleware (CON-002). Adding a nil guard would be defensive code for an unreachable state — ponytail: YAGNI.

### No other findings
- 0 blocking, 0 required.

---

## Quality Gate Status

| Gate item | Status |
|---|---|
| Every constraint checked w/ evidence + trace | DONE (7/7 MET) |
| Every AC checked w/ evidence | DONE (13/13 MET) |
| Negative vectors verified (reject + test) | DONE (6/6) |
| Cross-component consistency | DONE (trivial, consistent) |
| Security review | DONE (N/A P3, no findings) |
| Null safety | DONE (safe) |
| Error paths | DONE (200/404/405/500) |
| Middleware chain | DONE (recovery outermost, unchanged) |
| Over-engineering | DONE (minimal) |
| Footguns | DONE (none) |
| Execution paths traced | DONE (per constraint) |
| Spec-implementation drift | DONE (none) |
| Constitution | N/A (no constitution.md) |

**GATE: PASSED** — all 13 acceptance criteria and all 7 constraints MET with quoted evidence. No blocking or required findings. Implementation is minimal and correct.

---

## Recommendation

**PASS** — signal `devteam signal playwright-e2e pass`.

The implementation correctly satisfies every acceptance criterion and constraint. The handler is minimal (6 LOC), version is config-sourced (not hardcoded), 405s come free from Go 1.22+ method-pattern routing, the recovery middleware covers the route, and all 11 Go integration tests + 1 Playwright E2E spec are present and structurally correct. One noted finding (F-001) is unreachable under current constructor usage and covered by recovery middleware regardless — not action-required.