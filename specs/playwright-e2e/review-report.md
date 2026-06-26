# Review Report — playwright-e2e

## Summary

- **Acceptance criteria**: 13 total, 3 MET, 10 NOT MET
- **Findings**: 1 critical (recirculate), 0 required, 0 noted
- **Outcome**: RECIRCULATE TO CONSTRUCTION — implementation code is correct and minimal, but 10 of 13 acceptance criteria's mandated Go integration tests (AC-001..AC-011) were never written. Only the Playwright E2E spec (AC-012, AC-013) and the handler/route itself are present.

The implementation code (`internal/api/server.go` +18 lines, `internal/pipeline/pipeline.go` +3 lines, `ui/e2e/health.spec.ts` +9 lines) is correct and minimal — it satisfies the *behavioral* requirements. But the spec, acceptance.md, plan.md, and tasks.md all mandate 11 Go `httptest` integration tests (T002: `TestHealthGETReturns200AndBody`, `TestHealthGETNoRequestBody`, `TestHealthVersionFromConfig`, `TestHealthGETIgnoresQueryParams`, `TestHealthPanicReturns500`; T004: `TestHealthPOSTReturns405`, `TestHealthPUTReturns405`, `TestHealthDELETEReturns405`, `TestHealthPATCHReturns405`, `TestHealthGETStill200After405s`, `TestHealthTrailingSlash404`). `grep -ni "health" internal/api/server_test.go` returns ZERO matches. `git diff main...HEAD -- internal/api/server_test.go` is empty. The test file was never modified.

---

## Phase 1: Constraint Register Review

### CON-001 — Route registered via Go 1.22+ method-pattern `mux.HandleFunc("GET /api/health", ...)`
- **Source**: repo convention, server.go:160-188
- **Status**: MET
- **Trace**:
  1. Entry: `NewServer` at `internal/api/server.go:150`
  2. Route registered at `internal/api/server.go:190`: `mux.HandleFunc("GET /api/health", s.healthHandler)`
  3. Placed after `/api/metrics/sessions` (line 188) and before `staticFS` catch-all (line 192) — correct ordering, catch-all cannot shadow it.
- **Evidence**: `internal/api/server.go:190` — `mux.HandleFunc("GET /api/health", s.healthHandler)`
- **Explanation**: Exact method-pattern registration matching existing endpoint convention. 1 grep match.

### CON-002 — Health route covered by existing recoveryMiddleware + corsMiddleware chain
- **Source**: repo convention, server.go:194
- **Status**: MET
- **Trace**:
  1. `mux` receives the health route at line 190.
  2. `handler := s.recoveryMiddleware(s.corsMiddleware(mux))` at `internal/api/server.go:196` wraps the entire mux.
  3. `s.httpServer = &http.Server{Handler: handler}` at line 198 — the wrapped handler is what serves requests.
  4. Recovery is outermost: `recoveryMiddleware(corsMiddleware(mux))` — panic in `healthHandler` → caught at `internal/api/server.go:231` `recover()` → `writeError(w, 500, ...)` at line 233. Process survives.
- **Evidence**: `internal/api/server.go:196` — `handler := s.recoveryMiddleware(s.corsMiddleware(mux))`; `internal/api/server.go:228-237` — recoveryMiddleware with `defer recover()`.
- **Explanation**: No bypass possible — health route is on the same `mux` wrapped by the existing chain. CON-002 satisfied for free, as the plan predicted.

### CON-003 — `version` response field sourced from `Config.Version`, not hardcoded
- **Source**: repo convention, config.go:11
- **Status**: MET
- **Trace**:
  1. `healthHandler` at `internal/api/server.go:919` reads `s.pipeline.Config().Version` at line 922.
  2. `Pipeline.Config()` at `internal/pipeline/pipeline.go:44` returns `p.config`.
  3. `p.config` is the `*config.Config` passed to `NewPipeline`/`NewPipelineWithDispatcher` (field `config` set at construction, line 26).
  4. `config.Config.Version` is loaded from `devteam.yaml` `version:` (`internal/config/config.go:11`, `devteam.yaml:3` = `"1.0"`).
  5. `Status: "ok"` IS hardcoded — that is correct, `status` is a fixed literal per spec, only `version` is config-sourced.
- **Evidence**: `internal/api/server.go:920-923` — `writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Version: s.pipeline.Config().Version})`; `internal/pipeline/pipeline.go:44` — `func (p *Pipeline) Config() *config.Config { return p.config }`
- **Explanation**: Version flows config → Pipeline → Server → response. No hardcoded version literal in handler. Nil-safety: `p.config` is set in all constructors (verified field assignment exists in struct); `s.pipeline` set at `server.go:153` before `httpServer` created at line 198.

### CON-004 — New endpoint tested with `httptest` in-process server pattern
- **Source**: repo convention, server_test.go
- **Status**: NOT MET
- **Trace**:
  1. Required: Go integration tests using `httptest.NewServer` for the health endpoint.
  2. `rg -ni "health" internal/api/server_test.go` → 0 matches.
  3. `git diff main...HEAD -- internal/api/server_test.go` → empty (no changes to test file).
  4. No `TestHealth*` functions exist anywhere.
- **Evidence**: `internal/api/server_test.go` — no health-related code; diff vs main is empty.
- **Explanation**: Zero Go integration tests exist. This constraint is unmet. AC-001..AC-011 (all integration/smoke level) depend on these tests.

### CON-005 — E2E test runs against Playwright `webServer` on :18765, not production :8765
- **Source**: repo convention, ui/e2e, AGENTS.md :18765
- **Status**: MET
- **Trace**:
  1. `ui/e2e/health.spec.ts:3` — `const response = await request.get('/api/health');`
  2. Playwright `request` fixture resolves against `baseURL` from `ui/playwright.config.ts:16` — `baseURL: process.env.BASE_URL || 'http://localhost:18765'`.
  3. `webServer` config at `ui/playwright.config.ts:19-31` starts devteam binary on port 18765 (`port: parseInt(process.env.SERVER_PORT || '18765')`).
  4. Spec uses relative URL `/api/health`, not hardcoded `http://localhost:8765` — respects `baseURL`.
- **Evidence**: `ui/e2e/health.spec.ts:3`; `ui/playwright.config.ts:16,26`.
- **Explanation**: Spec hits :18765 via baseURL, not prod port. Constraint met.

### CON-006 — Response body is exactly `{"status":"ok","version":"1.0"}` for default config
- **Source**: input.md idea
- **Status**: MET (code-level); verification test NOT MET
- **Trace**:
  1. `healthResponse` struct at `internal/api/server.go:912-915` — fields ordered `Status` then `Version`, JSON tags `"status"` / `"version"`.
  2. `json.NewEncoder(w).Encode(data)` at `internal/api/server.go:903` emits keys in struct field order → `{"status":"ok","version":"1.0"}`.
  3. No `omitempty` on either field → both always present.
  4. `json.Encoder.Encode` appends a trailing `\n` — body is actually `{"status":"ok","version":"1.0"}\n`. Byte-exact assertion must use `strings.TrimSpace` (tasks.md T002 notes this).
- **Evidence**: `internal/api/server.go:912-915` struct; `internal/api/server.go:900-904` writeJSON.
- **Explanation**: Struct field order guarantees byte-exact JSON key order. Code correct. BUT no test performs the byte/string assertion (AC-001) — verification gap, see AC-001 finding.

### CON-007 — Non-GET methods return 405 (RFC 9110 §15.5.5)
- **Source**: HTTP semantics, RFC 9110 §15.5.5
- **Status**: MET (code-level, via stdlib); verification tests NOT MET
- **Trace**:
  1. Only `GET /api/health` registered at `internal/api/server.go:190`.
  2. Go 1.22+ `http.NewServeMux` method-pattern: a request with method not matching the registered pattern → 405 Method Not Allowed with `Allow` header (stdlib behavior, no custom handler needed).
  3. POST/PUT/DELETE/PATCH → none match `GET /api/health` → stdlib emits 405.
  4. `/api/health/` (trailing slash) → no pattern registered → 404.
- **Evidence**: `internal/api/server.go:190` — only `GET /api/health` registered; no other method registered for this path.
- **Explanation**: 405/404 come free from stdlib method-pattern routing. Code correct. BUT no integration test verifies each method (AC-006..AC-011) — verification gap.

---

## Phase 2: Acceptance Criteria Review

### AC-001: GET /api/health → 200, Content-Type application/json, body `{"status":"ok","version":"1.0"}`
- **Status**: NOT MET
- **Evidence**: No test. `rg -ni "health" internal/api/server_test.go` → 0 matches. The handler code at `internal/api/server.go:919-923` would satisfy this behaviorally, but the mandated smoke test (`TestHealthGETReturns200AndBody` per tasks.md T002) does not exist.
- **Explanation**: AC requires a `httptest.NewServer` test asserting 200 + Content-Type + byte-exact body. Test missing. Code is correct; verification is absent.

### AC-002: GET /api/health with no body → 200 same body (no r.Body decode)
- **Status**: NOT MET
- **Evidence**: No test `TestHealthGETNoRequestBody`. Handler at `internal/api/server.go:919-923` does not reference `r.Body` (grep confirms 0 `r.Body` matches in healthHandler) — code is correct, but no test verifies it.
- **Explanation**: Test missing. Code correct.

### AC-003: Config{Version:"9.9.9-test"} → body `{"status":"ok","version":"9.9.9-test"}`
- **Status**: NOT MET
- **Evidence**: No test `TestHealthVersionFromConfig`. Handler sources version from `s.pipeline.Config().Version` (server.go:922) — code correct, but no test loads a custom version and asserts the round-trip.
- **Explanation**: Test missing. This is the primary verification for CON-003. Without it, "version from config" is unverified.

### AC-004: GET /api/health?cb=123 → 200 standard body (query ignored)
- **Status**: NOT MET
- **Evidence**: No test `TestHealthGETIgnoresQueryParams`. Handler does not read `r.URL.RawQuery` — query params ignored by default. Code correct, no test.
- **Explanation**: Test missing.

### AC-005: Handler panic → recovery middleware returns 500, process survives
- **Status**: NOT MET
- **Evidence**: No test `TestHealthPanicReturns500`. Recovery middleware at `internal/api/server.go:228-237` catches panics and returns 500 — existing `TestRecoveryMiddleware` at server_test.go:224 covers the general middleware, but no health-specific panic test exists (tasks.md T002 mandates one).
- **Explanation**: Test missing. This is the primary verification for CON-002.

### AC-006: POST /api/health → 405
- **Status**: NOT MET
- **Evidence**: No test `TestHealthPOSTReturns405`. Stdlib method-pattern emits 405 — code correct, no test.
- **Explanation**: Test missing. CON-007 verification gap.

### AC-007: PUT /api/health → 405
- **Status**: NOT MET
- **Evidence**: No test `TestHealthPUTReturns405`. Code correct (stdlib 405), no test.
- **Explanation**: Test missing.

### AC-008: DELETE /api/health → 405
- **Status**: NOT MET
- **Evidence**: No test `TestHealthDELETEReturns405`. Code correct, no test.
- **Explanation**: Test missing.

### AC-009: PATCH /api/health → 405
- **Status**: NOT MET
- **Evidence**: No test `TestHealthPATCHReturns405`. Code correct, no test.
- **Explanation**: Test missing.

### AC-010: GET /api/health → 200 (positive control for AC-006..009)
- **Status**: NOT MET
- **Evidence**: No test `TestHealthGETStill200After405s`. Code correct, no test.
- **Explanation**: Test missing.

### AC-011: GET /api/health/ (trailing slash) → 404
- **Status**: NOT MET
- **Evidence**: No test `TestHealthTrailingSlash404`. No subtree pattern registered → stdlib 404. Code correct, no test.
- **Explanation**: Test missing.

### AC-012: Playwright E2E GET /api/health → 200, JSON {status:"ok", version:"1.0"}
- **Status**: MET
- **Evidence**: `ui/e2e/health.spec.ts:2-8`:
  ```typescript
  test('GET /api/health returns 200 with status ok and version 1.0', async ({ request }) => {
    const response = await request.get('/api/health');
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.status).toBe('ok');
    expect(body.version).toBe('1.0');
  });
  ```
  Asserts status 200, `body.status === 'ok'`, `body.version === '1.0'`. Uses `request` fixture (resolves against baseURL :18765).
- **Explanation**: Spec asserts both fields and status code. Matches AC-012 exactly.

### AC-013: npx playwright test discovers + passes health spec (not skipped)
- **Status**: MET
- **Evidence**: `ui/e2e/health.spec.ts` — single `test(...)` call, no `.skip`, no `.only`. File lives under `ui/e2e/` (matches `testDir: './e2e'` in playwright.config.ts:10). Test name is a plain string, discovered by default.
- **Explanation**: No skip markers. File in the discovered directory. (Whether `npx playwright test` actually passes is the Tester's job to verify by running it — code review confirms the spec is discoverable and not skipped.)

---

## Phase 3: Negative Test Vector Verification

| Vector | Expected | Implementation rejects? | Test verifies? |
|---|---|---|---|
| POST /api/health | 405 | YES (stdlib) | NO — test missing |
| PUT /api/health | 405 | YES (stdlib) | NO — test missing |
| DELETE /api/health | 405 | YES (stdlib) | NO — test missing |
| PATCH /api/health | 405 | YES (stdlib) | NO — test missing |
| /api/health/ trailing slash | 404 | YES (no subtree registered) | NO — test missing |
| Handler panic | 500, survives | YES (recovery middleware) | NO — test missing |

All negative vectors are correctly *handled* by the implementation, but NONE are *verified* by tests. This is the core finding.

---

## Phase 4: Cross-Component Consistency Review

Per plan.md's matrix, this feature has a single producer per value — no multi-component split.

| Shared Value | Producer | Consumer | Consistent? |
|---|---|---|---|
| `version` string | `config.Config.Version` → `Pipeline.Config()` → `healthHandler` | HTTP response `version` field | YES — single source, single read, no transform |
| Response JSON shape | `healthResponse` struct (Status, Version order) | Tests asserting body | YES (struct order = JSON key order) |
| 405 method set | Go ServeMux method-pattern (only GET) | AC-006..009 expectations | YES — stdlib emits 405 for POST/PUT/DELETE/PATCH |
| 500 panic path | `recoveryMiddleware` (outermost) | AC-005 expectation | YES |

No cross-component inconsistency. Single-component feature — the classic multi-provider bug does not apply.

---

## Phase 5: Language-Specific Footgun Review (Go)

- **Nil map writes**: No maps written in health code. N/A.
- **Nil channel**: No channels. N/A.
- **Interface nil isn't nil**: No interface comparisons. N/A.
- **Nil pointer deref**: `s.pipeline.Config().Version` — `s.pipeline` set at `server.go:153` before `httpServer` (line 198); `p.config` set in all Pipeline constructors (field at `pipeline.go:26`, assigned in `NewPipeline`/`NewPipelineWithDispatcher`/`NewPipelineWithQuestionStore`). Safe.
- **Integer overflow / modulo**: No arithmetic. N/A.
- **JSON null vs empty array**: `healthResponse` has no slice/map fields — both are `string`. N/A. No `omitempty` on response fields (correct — would break byte-exactness).

No Go footguns found in the implementation.

---

## Phase: Spec-Implementation Drift / Plan Coverage

| Spec/Plan item | Implemented? |
|---|---|
| T001 — `Pipeline.Config()` accessor | YES (`pipeline.go:44`) |
| T002 — Go integration tests (5 funcs) | **NO — entirely missing** |
| T003 — healthHandler + route | YES (`server.go:190, 910-924`) |
| T004 — Method-restriction tests (6 funcs) | **NO — entirely missing** |
| T005 — No impl (stdlib 405/404) | YES (correctly no code) |
| T006 — Playwright health spec | YES (`ui/e2e/health.spec.ts`) |
| T007 — Full verification | NOT VERIFIED (tests missing) |

tasks.md mandates T002 (5 tests) and T004 (6 tests) = 11 Go test functions. ZERO were written. This is the gap.

---

## Over-Engineering Check

Implementation is minimal — laudably so:
- `internal/api/server.go`: +18 lines (struct + handler + route) — plan predicted ~5 LOC handler + route; actual is ~15 LOC including the struct and doc comments. On target.
- `internal/pipeline/pipeline.go`: +3 lines (1-line accessor + 2-line doc comment) — exactly as planned.
- `ui/e2e/health.spec.ts`: +9 lines — matches plan's 8-line template exactly.

No over-engineering. No speculative abstractions, no factories, no config-for-constants. The developer correctly resisted adding a custom 405 handler (T005 — stdlib does it). Ponytail-clean.

---

## Missing Implementation

- **11 Go integration tests** mandated by acceptance.md AC-001..AC-011 and tasks.md T002/T004. None exist. `internal/api/server_test.go` was not modified on this branch.
  - `TestHealthGETReturns200AndBody` (AC-001, CON-001, CON-004, CON-006)
  - `TestHealthGETNoRequestBody` (AC-002)
  - `TestHealthVersionFromConfig` (AC-003, CON-003)
  - `TestHealthGETIgnoresQueryParams` (AC-004)
  - `TestHealthPanicReturns500` (AC-005, CON-002)
  - `TestHealthPOSTReturns405` (AC-006, CON-007)
  - `TestHealthPUTReturns405` (AC-007)
  - `TestHealthDELETEReturns405` (AC-008)
  - `TestHealthPATCHReturns405` (AC-009)
  - `TestHealthGETStill200After405s` (AC-010)
  - `TestHealthTrailingSlash404` (AC-011)

---

## Security Review

Feature is P3 (not P1), so security review is recommended, not mandatory. Findings:

- **Auth**: No auth on `/api/health` — consistent with all existing `/api/*` endpoints (spec FR-007, assumption). Correct.
- **Input validation**: GET, no body decode, no params parsed — no input to validate. Correct.
- **Output filtering**: Response exposes only `status:"ok"` + config version. Version is already observable via service behavior; not sensitive. Correct.
- **CORS**: `corsMiddleware` sets `Access-Control-Allow-Origin: *` (server.go:217). This is pre-existing, not introduced by this feature. Permissive but consistent with the rest of the API. Not a finding for this feature.
- **Error leakage**: `recoveryMiddleware` (server.go:233) returns generic `"An unexpected error occurred"` on panic — no stack trace leakage. Correct.

No security findings introduced by this feature.

---

## Constitution Compliance

No `constitution.md` exists at repo root or `.specify/constitution.md` (verified in spec §Constitution Compliance). No constitution principles to check. N/A.

---

## Null Pointer Safety

- `s.pipeline` — set at `server.go:153` in `NewServer` struct literal before `httpServer` creation at line 198. Safe.
- `p.config` — field at `pipeline.go:26`; set in all constructors (NewPipeline, NewPipelineWithDispatcher, NewPipelineWithQuestionStore per tasks.md T001 verification). Safe.
- `s.pipeline.Config()` returns `*config.Config` — if it returned nil, `.Version` would panic. But `p.config` is always set. Safe.
- No JSON arrays/maps in response — null-vs-empty-array bug not applicable.

No null pointer safety issues.

---

## Error Path Verification

| Path | Expected | Code handles? |
|---|---|---|
| GET happy path | 200 + body | YES (server.go:920) |
| POST/PUT/DELETE/PATCH | 405 | YES (stdlib method-pattern) |
| `/api/health/` trailing slash | 404 | YES (no subtree registered) |
| Handler panic | 500, process survives | YES (recoveryMiddleware:228-237) |
| Empty config.Version | 200 with `version:""` | YES (faithful reflection — `Config().Version` returns `""`, no defaulting) |

All error paths handled correctly in code. Verification tests for these paths are missing (see findings).

---

## Middleware Chain Verification

- `handler := s.recoveryMiddleware(s.corsMiddleware(mux))` (server.go:196).
- Recovery is OUTERMOST — catches panics in corsMiddleware AND all handlers including health. Correct ordering.
- CORS sets headers + handles OPTIONS preflight (server.go:215-226).
- Health route on `mux` is inside both wrappers. No bypass.
- No request body size limit — pre-existing, not introduced here. N/A for this feature.

Middleware chain correct.

---

## Findings

### F-001: Missing all 11 mandated Go integration tests
- **Severity**: CRITICAL — recirculate to construction
- **Criterion**: AC-001, AC-002, AC-003, AC-004, AC-005, AC-006, AC-007, AC-008, AC-009, AC-010, AC-011; CON-004; tasks.md T002, T004
- **Code**: `internal/api/server_test.go` (no diff vs main — file unchanged)
- **Description**: The spec, acceptance.md, plan.md, and tasks.md all mandate 11 Go `httptest` integration tests for the health endpoint. None were written. `git diff main...HEAD -- internal/api/server_test.go` is empty. `rg -ni "health" internal/api/server_test.go` returns 0 matches. The implementation *code* is correct and would pass these tests, but the tests themselves do not exist. This leaves 10 of 13 acceptance criteria unverifiable. Construction gate requires "code compiles, service starts, no stubs, independently buildable" — the developer shipped the handler but skipped the test tasks (T002, T004) entirely. Recirculate to construction with instruction to add the 11 missing test functions exactly as specified in tasks.md T002 and T004, using the `setupHealthTestServer` helper pattern.
- **Fix needed**: Add the 11 `TestHealth*` functions to `internal/api/server_test.go` per tasks.md T002 (5 tests) and T004 (6 tests). Use `httptest.NewServer(s.httpServer.Handler)`. Byte-exact body assertion must use `strings.TrimSpace` to handle `json.Encoder`'s trailing newline.

---

## Quality Gate Status

1. ✅ Constraint register reviewed with execution path traces (CON-001..CON-007)
2. ✅ Every acceptance criterion checked with quoted evidence (AC-001..AC-013)
3. ⚠️ Negative test vectors: implementation handles them correctly, but NO tests verify them
4. ✅ Cross-component consistency verified (trivial — single component)
5. ✅ Security review complete (no findings)
6. ✅ Constitution compliance checked (N/A — no constitution)
7. ✅ Null pointer safety verified
8. ✅ Error paths verified in code (tests missing)
9. ✅ Middleware chain verified end-to-end
10. ✅ Over-engineering check complete (minimal implementation)
11. ✅ Missing implementation check complete (11 tests missing)
12. ✅ Language-specific footguns checked (none found)
13. ✅ Execution paths traced for each constraint
14. ✅ Multi-component constraints N/A (single component)

**Gate: NOT PASSED** — critical finding F-001 (11 missing mandated tests). Recirculate to construction.