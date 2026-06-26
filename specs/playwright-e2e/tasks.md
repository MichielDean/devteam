# Tasks: playwright-e2e (GET /api/health)

**Input**: Design documents from `/specs/playwright-e2e/` (plan.md, research.md, data-model.md, contracts/GET-api-health.md)

**Prerequisites**: plan.md (required), spec.md (required), acceptance.md (required)

**Tests**: REQUIRED — spec.md FR-006 and acceptance.md AC-001..AC-013 mandate Go integration tests + one Playwright E2E spec.

**Organization**: Tasks grouped by user story priority. Single repo (`devteam`, path `.`).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1 / US2 / US3
- Exact file paths in descriptions

## Path Conventions
Single project (Go module at repo root). Paths relative to repo root:
- `internal/api/server.go`, `internal/api/server_test.go`
- `internal/pipeline/pipeline.go`
- `ui/e2e/health.spec.ts`

---

## Phase 1: Foundational (Blocking Prerequisites)

**Purpose**: Expose config to the handler so US-1 can source version from config.

**⚠️ CRITICAL**: US-1 (the MVP) depends on this — handler needs `Pipeline.Config()` accessor.

- [ ] T001 [US1] Add `Config()` accessor to Pipeline in `internal/pipeline/pipeline.go` — [MODIFY]
  - Add method: `func (p *Pipeline) Config() *config.Config { return p.config }` (1 line, placed near other accessors around line 39-44)
  - **Constraints**: CON-003 (enables version-from-config)
  - **Done conditions**:
    - `go build ./...` compiles with no errors
    - `Pipeline.Config()` returns the `*config.Config` passed to `NewPipeline`/`NewPipelineWithDispatcher` (verified transitively by T003 AC-003)
  - **Test level**: unit (transitive — exercised by T003 integration tests)
  - **Agent failure mode checks**:
    - [x] Nil pointer ordering: `p.config` is set in all constructors (NewPipeline:282, NewPipelineWithDispatcher:316, NewPipelineWithQuestionStore:331) before any request — no nil deref possible. Verify by grep that all 3 constructors set `config:`.
    - [x] No JSON serialization in this task
    - [x] No middleware change
    - [x] No parsing code

**Checkpoint**: Pipeline exposes config. Handler can be written.

---

## Phase 2: User Story 1 — Health Check Probe (Priority: P1) 🎯 MVP

**Goal**: `GET /api/health` returns 200 `{"status":"ok","version":"<config.Version>"}`.

**Independent Test**: `curl http://localhost:8765/api/health` → 200 + expected JSON.

### Tests for User Story 1 (write FIRST, ensure they FAIL before implementation)

- [ ] T002 [US1] Add Go integration tests for health endpoint in `internal/api/server_test.go` — [MODIFY]
  - Add a `setupHealthTestServer(t, version string)` helper (or extend `setupTestServer` with a version param) that builds `config.Config{Version: version, Pipeline: ...6 phases...}` + `NewServer` + `httptest.NewServer(s.httpServer.Handler)`. NOTE: existing `setupTestServer` does NOT set `Version` — health tests need it set.
  - Tests (one `func` each, names exact):
    - `TestHealthGETReturns200AndBody` — GET `/api/health` with `cfg.Version="1.0"` → assert 200, `Content-Type` contains `application/json`, body string equals `{"status":"ok","version":"1.0"}` (byte-exact, CON-006). Covers AC-001, CON-001, CON-004, CON-006.
    - `TestHealthGETNoRequestBody` — GET with explicit empty body → 200 + same body. Covers AC-002.
    - `TestHealthVersionFromConfig` — `cfg.Version="9.9.9-test"` → body `{"status":"ok","version":"9.9.9-test"}`. Covers AC-003, CON-003.
    - `TestHealthGETIgnoresQueryParams` — GET `?cb=123` → 200 + `{"status":"ok","version":"1.0"}`. Covers AC-004.
    - `TestHealthPanicReturns500` — register a panicking handler variant (e.g. temporarily replace via a test-only helper or use a separate mux with a panicking handler wrapped by `s.recoveryMiddleware`) → GET → 500; then a second GET to the real endpoint → 200 (proves server alive). Covers AC-005, CON-002.
  - **Constraints**: CON-003, CON-004, CON-006
  - **Done conditions**:
    - Tests compile (`go test ./internal/api/ -run TestHealth` does not error with undefined symbols)
    - Tests FAIL before T003 implements the handler (RED phase)
    - After T003: all 5 tests PASS
    - All tests use `httptest.NewServer` (grep `httptest.NewServer` in the new test funcs)
  - **Test level**: integration
  - **Agent failure mode checks**:
    - [x] JSON null vs empty arrays: N/A — response has no arrays
    - [x] Recovery middleware first: panic test confirms recovery wraps health route
    - [x] No parsing code in tests
    - [x] Byte-exact assertion: use `strings.TrimSpace(body) == `{"status":"ok","version":"1.0"}`` (TrimSpace removes the trailing newline from `json.Encoder`)

### Implementation for User Story 1

- [ ] T003 [US1] Implement health handler + route in `internal/api/server.go` — [MODIFY]
  - Add `healthResponse` struct (field order: `Status` then `Version`, JSON tags `"status"` / `"version"`) near other response types
  - Add method `func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request)`:
    - `writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Version: s.pipeline.Config().Version})`
    - MUST NOT read `r.Body` (GET; AC-002)
  - Register route in `NewServer` BEFORE the `if staticFS != nil` block: `mux.HandleFunc("GET /api/health", s.healthHandler)` (place after the existing `/api/metrics/sessions` line, before staticFS catch-all at :190)
  - **Constraints**: CON-001 (method-pattern route), CON-002 (covered by existing middleware chain), CON-003 (version from `Pipeline.Config()`), CON-006 (struct field order → byte-exact JSON), CON-007 (only GET registered → 405 auto)
  - **Done conditions**:
    - `go build ./...` compiles
    - `go test ./internal/api/ -run TestHealth` — all 5 tests from T002 PASS
    - `grep 'mux.HandleFunc("GET /api/health", s.healthHandler)' internal/api/server.go` returns 1 match (CON-001)
    - `grep 'r.Body' internal/api/server.go` in `healthHandler` returns 0 matches (AC-002 — handler does not decode body)
    - Manual: start server `~/go/bin/devteam -http :8765`, `curl -i http://localhost:8765/api/health` → 200 + `{"status":"ok","version":"1.0"}`
  - **Test level**: integration (T002 tests validate) + smoke (server starts, endpoint responds)
  - **Agent failure mode checks**:
    - [x] Nil pointer ordering: `s.pipeline` is set in `NewServer` (server.go:151-158) before `httpServer` is created and before any request — `s.pipeline.Config()` safe. Verify `s.pipeline` assignment exists in the struct literal.
    - [x] JSON null vs empty arrays: response has no arrays — N/A
    - [x] Recovery middleware first: no change to middleware chain (server.go:194 unchanged) — health route on `mux` is wrapped by existing `recoveryMiddleware(corsMiddleware(mux))`. CON-002 satisfied.
    - [x] State machine logic: none
    - [x] Parsing code: none (GET, no body decode, query params ignored by default since handler doesn't read `r.URL.RawQuery`)
    - [x] Multi-component consistency: single component — N/A
    - [x] Go footgun: `s.pipeline.Config().Version` — if `Config()` returned nil this would panic, but T001 guarantees `p.config` is set in all constructors. No nil-map writes.

**Checkpoint**: US-1 fully functional. `GET /api/health` works, tested, MVP demoable.

---

## Phase 3: User Story 2 — Method Restriction (Priority: P2)

**Goal**: POST/PUT/DELETE/PATCH → 405; trailing slash → 404.

**Independent Test**: `curl -X POST http://localhost:8765/api/health` → 405.

### Tests for User Story 2

- [ ] T004 [US2] Add method-restriction integration tests in `internal/api/server_test.go` — [MODIFY]
  - Tests (one `func` each):
    - `TestHealthPOSTReturns405` — POST `/api/health` empty body → 405 (AC-006, CON-007)
    - `TestHealthPUTReturns405` — PUT → 405 (AC-007)
    - `TestHealthDELETEReturns405` — DELETE → 405 (AC-008)
    - `TestHealthPATCHReturns405` — PATCH → 405 (AC-009)
    - `TestHealthGETStill200After405s` — GET → 200 (AC-010, positive control)
    - `TestHealthTrailingSlash404` — GET `/api/health/` → 404 (AC-011)
  - Use the `setupHealthTestServer` helper from T002.
  - **Constraints**: CON-007 (405 for non-GET, RFC 9110 §15.5.5)
  - **Done conditions**:
    - All 6 tests PASS against T003 implementation (no new impl code needed — 405/404 come free from Go ServeMux method-pattern)
    - Each 405 test asserts `resp.StatusCode == 405` (not 200, not 404)
  - **Test level**: integration
  - **Agent failure mode checks**:
    - [x] No JSON serialization in tests (asserting status codes only)
    - [x] No parsing code
    - [x] Negative case coverage: POST/PUT/DELETE/PATCH are the negative vectors from CON-007

### Implementation for User Story 2

- [ ] T005 [US2] No implementation task — 405 and 404 are emitted by Go 1.22+ `http.NewServeMux` method-pattern routing automatically (T003 registers only `GET /api/health`; non-GET → 405 with `Allow` header; `/api/health/` not registered → 404).
  - **Constraints addressed**: CON-007 (via stdlib), AC-011 (via exact-path registration)
  - **Justification for no code**: Go 1.22+ ServeMux method-pattern emits 405 natively for methods not matching the registered pattern. Adding a custom 405 handler would reinvent the stdlib (ponytail: stdlib does it → use it). Verified in research.md.
  - **Done conditions** (verified by T004 tests passing):
    - POST/PUT/DELETE/PATCH → 405
    - `/api/health/` → 404
  - **Test level**: integration (T004)

**Checkpoint**: US-1 AND US-2 both work independently. Method restriction verified.

---

## Phase 4: User Story 3 — Playwright E2E Coverage (Priority: P3)

**Goal**: `npx playwright test` discovers + passes a health E2E spec against :18765.

**Independent Test**: `cd ui && npx playwright test health.spec.ts` → green.

### Implementation for User Story 3

- [ ] T006 [P] [US3] Create Playwright E2E spec at `ui/e2e/health.spec.ts` — [CREATE]
  - Single test (no `.skip`):
    ```typescript
    import { test, expect } from '@playwright/test';

    test('GET /api/health returns 200 with status ok and version 1.0', async ({ request }) => {
      const response = await request.get('/api/health');
      expect(response.status()).toBe(200);
      const body = await response.json();
      expect(body.status).toBe('ok');
      expect(body.version).toBe('1.0');
    });
    ```
  - Uses Playwright's `request` fixture (resolves against `baseURL` = `http://localhost:18765` from `playwright.config.ts`). No `page.goto` needed — pure HTTP probe.
  - **Constraints**: CON-005 (uses baseURL :18765 via playwright.config.ts webServer, not prod :8765)
  - **Done conditions**:
    - File exists at `ui/e2e/health.spec.ts`
    - `cd ui && npx playwright test health.spec.ts` passes (AC-012, AC-013)
    - `grep '.skip' ui/e2e/health.spec.ts` returns 0 matches (AC-013 — not skipped)
    - Test discovered by `npx playwright test` (full suite run shows it as `passed`)
    - Asserts `status === "ok"` AND `version === "1.0"` (both fields, AC-012)
  - **Test level**: e2e
  - **Agent failure mode checks**:
    - [x] No JSON serialization (test consumes JSON, doesn't produce it)
    - [x] No parsing code (Playwright parses JSON via `response.json()`)
    - [x] No Go footguns (TypeScript file)
    - [x] Uses `request` fixture (not `page.request`) — cleaner, no browser context needed for a pure API probe
  - **Dependencies**: T003 must complete first (endpoint must exist for E2E to hit it). Can run in parallel with T002/T004 (different file, no conflict) once T003 is done.

**Checkpoint**: All user stories independently functional. E2E coverage verified.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Final verification across all stories.

- [ ] T007 Run full verification suite
  - `go build ./...`
  - `go test ./internal/api/` — all health tests + existing tests green
  - `cd ui && npx playwright test` — full suite green including health.spec.ts
  - Manual smoke: `~/go/bin/devteam -http :8765` then `curl -i http://localhost:8765/api/health` → 200 + body; `curl -i -X POST http://localhost:8765/api/health` → 405
  - **Done conditions**: all above pass; no regressions in existing tests
  - **Test level**: smoke + integration + e2e

---

## Dependencies & Execution Order

### Phase Dependencies
- **Phase 1 (T001)**: No deps. BLOCKS T002, T003 (handler needs accessor).
- **Phase 2 (T002, T003)**: T001 done. T002 (tests) and T003 (impl) are tight-loop TDD — write T002 first (RED), then T003 (GREEN). May be one commit.
- **Phase 3 (T004, T005)**: T003 done. T004 tests only (no impl — stdlib provides 405/404).
- **Phase 4 (T006)**: T003 done. Parallel with T002/T004 (different file).
- **Phase 5 (T007)**: All above done.

### Task Dependency Graph
```
T001 ──→ T002 ──→ T003 ──→ T004 (tests only)
                ↓
                └────→ T006 (Playwright, parallel with T004)
                          ↓
                       T007 (final verify)
```

### Parallel Opportunities
- T006 (Playwright spec) can run parallel with T004 (Go method-restriction tests) — different files, no conflict, both depend only on T003.
- Within a story, no parallelism (single-file edits).

## Implementation Strategy: MVP First
1. T001 → T002 → T003 → **STOP, VALIDATE**: US-1 works (`go test ./internal/api/ -run TestHealth` green, manual curl 200).
2. T004 → validate US-2 (405s, 404).
3. T006 → validate US-3 (`npx playwright test health.spec.ts` green).
4. T007 → full suite green.

## Notes
- Tests written FIRST (T002 before T003) per TDD — ensures RED before GREEN.
- Commit after each task or logical group (T001+T002+T003 can be one commit if tight-loop).
- No new dependencies (Go stdlib + existing @playwright/test).
- No DB changes, no config changes (version already `"1.0"` in devteam.yaml).
- `staticFS` catch-all at server.go:190 must remain AFTER the health route registration — verify T003 places the HandleFunc before that block.