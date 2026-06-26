# Review Report

**Feature**: playwright-e2e (GET /api/health)
**Reviewer**: Code Reviewer
**Phase**: review
**Date**: 2026-06-25

## Summary

- Acceptance criteria: 13 total, 12 MET, 1 NOT MET
- Findings: 1 critical (BLOCKING), 0 required, 0 noted
- Implementation LOC: ~30 (handler 14 + accessor 1 + Playwright 9) — minimal, no over-engineering
- **Gate: BLOCKED** — AC-001..AC-011 (Go integration tests) have no implementation. T002/T004 from tasks.md were not executed.

---

## Phase 1: Constraint Register Review (MANDATORY FIRST)

### CON-001 — Route via Go 1.22+ method-pattern `mux.HandleFunc("GET /api/health", ...)`
- **Source**: repo convention, server.go:160-188
- **Status**: MET
- **Trace**:
  1. `NewServer` line 160: `mux := http.NewServeMux()`
  2. Line 190: `mux.HandleFunc("GET /api/health", s.healthHandler)` — exact method-pattern form
  3. Route registered BEFORE staticFS catch-all (line 192-194) ✓
- **Evidence**: `internal/api/server.go:190` — `mux.HandleFunc("GET /api/health", s.healthHandler)`

### CON-002 — Health route covered by recoveryMiddleware + corsMiddleware
- **Source**: repo convention, server.go:194
- **Status**: MET (code); NOT VERIFIABLE (no test for panic→500)
- **Trace**:
  1. Line 190: health route registered on `mux`
  2. Line 196: `handler := s.recoveryMiddleware(s.corsMiddleware(mux))` — wraps entire mux
  3. `recoveryMiddleware` (server.go:228-238) has `defer recover()` → `writeError(w, 500, ...)` ✓
  4. Recovery is outermost (after CORS) ✓
- **Evidence**: `internal/api/server.go:196` — `handler := s.recoveryMiddleware(s.corsMiddleware(mux))`; `:228-238` — recovery with `writeError(w, http.StatusInternalServerError, ...)`
- **Note**: Code path is correct; AC-005 test (induced panic → 500) was NOT implemented (see F-001).

### CON-003 — version sourced from Config, not hardcoded
- **Source**: repo convention, config.go
- **Status**: MET (code); NOT VERIFIABLE (no test)
- **Trace**:
  1. Handler `healthHandler` server.go:919-924 calls `s.pipeline.Config().Version`
  2. `Pipeline.Config()` pipeline.go:44 returns `p.config`
  3. `p.config` set in all 3 constructors (NewPipeline:285, NewPipelineWithDispatcher:319, NewPipelineWithQuestionStore:334) ✓
  4. No hardcoded "1.0" literal in handler — `Status: "ok"` is the only literal, version is dynamic ✓
- **Evidence**: `internal/api/server.go:922` — `Version: s.pipeline.Config().Version`; `internal/pipeline/pipeline.go:44` — `func (p *Pipeline) Config() *config.Config { return p.config }`

### CON-004 — httptest in-process server pattern
- **Source**: repo convention, server_test.go
- **Status**: NOT MET — no Go tests added
- **Evidence**: `git diff main...HEAD -- internal/api/server_test.go` shows no changes; `grep TestHealth internal/api/server_test.go` returns 0 matches
- **Explanation**: tasks.md T002 + T004 mandate 11 `TestHealth*` functions in `server_test.go`; none exist. AC-001..AC-011 have no Go-level verification.

### CON-005 — E2E on :18765 via Playwright baseURL
- **Source**: repo convention, ui/e2e
- **Status**: MET
- **Trace**:
  1. `ui/e2e/health.spec.ts:4` — `request.get('/api/health')` (relative)
  2. `ui/playwright.config.ts:16` — `baseURL: process.env.BASE_URL || 'http://localhost:18765'`
  3. `request` fixture resolves against baseURL → :18765 ✓
- **Evidence**: `ui/e2e/health.spec.ts:4`; `ui/playwright.config.ts:16`

### CON-006 — exact body `{"status":"ok","version":"1.0"}`
- **Source**: input.md idea
- **Status**: MET (code); NOT VERIFIABLE (no byte-assertion test)
- **Trace**:
  1. `healthResponse` struct fields ordered `Status` then `Version` (server.go:912-915)
  2. JSON tags `"status"`, `"version"` ✓
  3. `json.NewEncoder(w).Encode(data)` (writeJSON:903) emits keys in struct field order → `{"status":"ok","version":"1.0"}` (with trailing newline) ✓
- **Evidence**: `internal/api/server.go:912-915` — struct with `Status` before `Version`; `:903` — `json.NewEncoder(w).Encode(data)`
- **Note**: byte-exactness is structurally guaranteed but no integration test asserts it (F-001).

### CON-007 — 405 for non-GET (RFC 9110 §15.5.5)
- **Source**: HTTP semantics
- **Status**: MET (code); NOT VERIFIABLE (no tests)
- **Trace**:
  1. Only `GET /api/health` registered (server.go:190)
  2. Go 1.22+ ServeMux method-pattern: POST/PUT/DELETE/PATCH → 405 automatically with `Allow: GET, HEAD` header
  3. `/api/health/` (trailing slash) not registered → 404 ✓
- **Evidence**: `internal/api/server.go:190` — single `GET /api/health` registration; no other method routes

---

## Phase 2: Acceptance Criteria Review

### AC-001: GET /api/health → 200, application/json, body `{"status":"ok","version":"1.0"}`
- **Status**: MET (impl) / NOT MET (test)
- **Evidence**: `server.go:190` route, `:919-924` handler, `:901` sets Content-Type, struct order guarantees body. **No integration test exists** (`server_test.go` unchanged).
- **Explanation**: Implementation correct; verification artifact missing.

### AC-002: GET with no body → 200 same body (no r.Body decode)
- **Status**: MET
- **Evidence**: `server.go:919-924` — `healthHandler` never references `r.Body`
- **Explanation**: Handler reads only config; body-less GET returns 200. (No dedicated test — F-001.)

### AC-003: custom Config.Version="9.9.9-test" → body reflects it
- **Status**: MET (impl) / NOT VERIFIABLE (no test)
- **Evidence**: `server.go:922` — `Version: s.pipeline.Config().Version` (dynamic)
- **Explanation**: Version is config-sourced, not hardcoded. No test exercises custom config.

### AC-004: GET /api/health?cb=123 → 200 standard body
- **Status**: MET
- **Evidence**: handler ignores `r.URL.RawQuery`; no query parsing. ServeMux matches path regardless of query string.
- **Explanation**: Query params ignored by omission. (No dedicated test — F-001.)

### AC-005: handler panic → 500, server survives
- **Status**: NOT MET (no test)
- **Evidence**: `server.go:228-238` recovery middleware exists and wraps mux, but **no test induces panic + asserts 500 + subsequent 200**.
- **Explanation**: Code path present; AC requires verification of induced-panic path per acceptance.md verification text. Not implemented.

### AC-006: POST → 405
### AC-007: PUT → 405
### AC-008: DELETE → 405
### AC-009: PATCH → 405
- **Status**: MET (impl, via stdlib) / NOT MET (no tests)
- **Evidence**: only `GET /api/health` registered → stdlib emits 405. No `TestHealth*Returns405` functions exist.

### AC-010: GET → 200 (positive control)
- **Status**: MET (impl) / NOT MET (no test)

### AC-011: GET /api/health/ (trailing slash) → 404
- **Status**: MET (impl) / NOT MET (no test)
- **Evidence**: exact-path registration; trailing-slash path not registered → ServeMux 404. No test.

### AC-012: Playwright E2E asserts 200 + `{status:"ok", version:"1.0"}`
- **Status**: MET
- **Evidence**: `ui/e2e/health.spec.ts:4-8` — asserts `response.status()===200`, `body.status==='ok'`, `body.version==='1.0'`
- **Explanation**: All three required assertions present.

### AC-013: suite discovers + passes health spec (no .skip)
- **Status**: MET (structural)
- **Evidence**: `ui/e2e/health.spec.ts` uses `test(...)` (not `test.skip`); file under `ui/e2e/` matches `testDir: './e2e'` (config:10)
- **Explanation**: Will be discovered. (Execution pass is Tester's job, not Reviewer's.)

---

## Phase 3: Negative Test Vector Verification

| Vector | Impl rejects? | Test? |
|---|---|---|
| POST /api/health → 405 | YES (stdlib) | **NO** — TestHealthPOSTReturns405 missing |
| PUT → 405 | YES | **NO** |
| DELETE → 405 | YES | **NO** |
| PATCH → 405 | YES | **NO** |
| /api/health/ → 404 | YES (exact path) | **NO** |
| handler panic → 500 | YES (recovery) | **NO** — AC-005 test missing |

All rejection logic correct in code; **zero negative-vector tests present**. This is F-001.

---

## Phase 4: Cross-Component Consistency

Single producer per value. Matrix trivial per plan.md.
- `version`: `config.Config.Version` → `Pipeline.Config()` → `healthHandler` → response. Single path, no transform. **Consistent.**
- Response shape: struct field order = JSON key order = expected body. **Consistent.**
- No multi-provider split. No findings.

---

## Phase 5: Language-Specific Footgun Review

**Go**: No nil-map writes. `s.pipeline.Config()` could return nil only if constructors didn't set `config:` — verified all 3 set it (pipeline.go:285,319,334). `s.pipeline` set in NewServer before handler invocation (server.go:153). No overflow, no modulo. **No footguns.**

**TypeScript** (Playwright spec): no `any`, uses `===` via `expect().toBe()`, no optional chaining. **No footguns.**

---

## Findings

### F-001: Go integration tests NOT implemented (BLOCKING)
- **Severity**: needs fixing — BLOCKING
- **Criterion**: AC-001..AC-011, CON-004, tasks.md T002 + T004
- **Code**: `internal/api/server_test.go` — UNCHANGED (`git diff main...HEAD` empty for this file)
- **Description**: tasks.md T002 mandates 5 tests (`TestHealthGETReturns200AndBody`, `TestHealthGETNoRequestBody`, `TestHealthVersionFromConfig`, `TestHealthGETIgnoresQueryParams`, `TestHealthPanicReturns500`) and T004 mandates 6 more (`TestHealthPOSTReturns405`, `TestHealthPUTReturns405`, `TestHealthDELETEReturns405`, `TestHealthPATCHReturns405`, `TestHealthGETStill200After405s`, `TestHealthTrailingSlash404`) plus a `setupHealthTestServer` helper. **None exist.** `grep TestHealth internal/api/server_test.go` = 0 matches. This violates CON-004 (httptest pattern) and leaves 11 of 13 acceptance criteria unverified at their specified test level (smoke/integration). Spec.md FR-006 + acceptance.md test levels mandate these.
- **Fix needed**: Implement T002 + T004 tests in `internal/api/server_test.go` using `httptest.NewServer(s.httpServer.Handler)` pattern matching existing `setupTestServer`. The implementation code is correct; only the verification artifact is missing.

### No other findings
- Over-engineering: NONE. Handler 14 LOC, accessor 1 LOC, spec 9 LOC — matches plan estimate (~5 LOC handler + 1 LOC accessor).
- Missing implementation (non-test): NONE. FR-001..FR-007 all addressed (route, handler, version-from-config, Content-Type, 405 auto, Playwright spec, no auth).
- Security: N/A — P3 feature, no auth by design (FR-007), no input to validate (GET, no body decode).
- Constitution: no `constitution.md` exists — N/A.
- Null safety: `s.pipeline` + `p.config` both set before use in all constructors. Safe.
- Error paths: 405 (stdlib), 404 (exact path), 500 (recovery middleware) all present in code.
- Middleware chain: recovery outermost (server.go:196), CORS present. Unchanged by feature. Correct.

---

## Quality Gate Status

| Gate item | Status |
|---|---|
| Every constraint checked w/ evidence | DONE (7/7) |
| Every AC checked w/ evidence | DONE (13/13) |
| Negative vectors verified | CODE-ONLY — no tests (F-001) |
| Cross-component consistency | DONE (trivial, consistent) |
| Security review | N/A (P3) |
| Null safety | DONE (safe) |
| Error paths | DONE (405/404/500 in code) |
| Middleware chain | DONE (unchanged, correct) |
| Over-engineering | DONE (minimal) |
| Footguns | DONE (none) |
| Execution paths traced | DONE |

**GATE: NOT PASSED** — F-001 (BLOCKING). 11 acceptance criteria lack the Go integration tests mandated by their test level and by tasks.md T002/T004. The implementation code is correct and minimal; the gap is purely the missing test artifact.

---

## Recommendation

**RECIRCULATE to construction** with note: "Go integration tests for AC-001..AC-011 missing. `internal/api/server_test.go` unchanged. Implement tasks.md T002 (5 tests: TestHealthGETReturns200AndBody, TestHealthGETNoRequestBody, TestHealthVersionFromConfig, TestHealthGETIgnoresQueryParams, TestHealthPanicReturns500) and T004 (6 tests: TestHealthPOSTReturns405, TestHealthPUTReturns405, TestHealthDELETEReturns405, TestHealthPATCHReturns405, TestHealthGETStill200After405s, TestHealthTrailingSlash404) + setupHealthTestServer helper. Implementation code (handler, accessor, route) is correct — only tests missing."

Alternatively, since the implementation is correct and the missing artifact is a test file (Tester's domain overlaps), construction may implement T002/T004 or the Tester phase may be directed to add them. Per tasks.md ownership, T002/T004 are construction-phase deliverables.