# Implementation Plan: playwright-e2e (GET /api/health)

**Branch**: `spec/playwright-e2e` | **Date**: 2026-06-25 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/playwright-e2e/spec.md`

## Summary

Add a `GET /api/health` liveness endpoint returning `{"status":"ok","version":"<config.Version>"}` with HTTP 200, plus Go `httptest` integration tests and one Playwright E2E spec. Technical approach: 1 route + 1 handler method on `*Server` reusing existing `writeJSON`; version surfaced via a new 1-line `Pipeline.Config()` accessor (laziest path — `Server` already holds `pipeline`, avoids breaking 25+ `NewServer` call sites); 405 for non-GET is free from Go 1.22+ ServeMux method-pattern routing; recovery middleware already covers the route (CON-002 for free).

## Technical Context

- **Language/Version**: Go 1.26.1 (module `github.com/MichielDean/devteam`)
- **Primary Dependencies**: stdlib `net/http`, `encoding/json` only. Playwright `@playwright/test` (already installed in `ui/`).
- **Storage**: N/A — ephemeral response, no persistence
- **Testing**: Go `testing` + `net/http/httptest` (integration); Playwright (E2E)
- **Target Platform**: Linux server (existing devteam web service)
- **Project Type**: web service (brownfield — extending `internal/api`)
- **Performance Goals**: none specified; handler is sub-microsecond (struct + JSON encode)
- **Constraints**: no auth (consistent with existing `/api/*`); liveness only (no DB ping); config-sourced version
- **Scale/Scope**: single endpoint, ~5 LOC handler + 1 LOC accessor + tests. Minimal P3 pipeline-exercise feature.

## Constitution Check

**GATE: Must pass before design work.**

No `constitution.md` at repo root or `.specify/constitution.md` (verified by spec §Constitution Compliance). No constitution principles to check. **PASS** — no violations possible.

## Project Structure

### Documentation (this feature)
```text
specs/playwright-e2e/
├── plan.md              # this file
├── research.md          # existing code patterns, library choices
├── data-model.md        # HealthStatus entity
├── contracts/
│   └── GET-api-health.md
└── tasks.md             # implementation tasks
```

### Source Code (repository root — brownfield, modify in place)
```text
internal/api/
├── server.go            # [MODIFY] add healthResponse struct + healthHandler + route registration
└── server_test.go       # [MODIFY] add health endpoint tests
internal/pipeline/
└── pipeline.go          # [MODIFY] add Config() accessor (1 line)
ui/e2e/
└── health.spec.ts       # [CREATE] Playwright E2E spec
```

**Structure Decision**: Modify existing files in place (brownfield). No new packages — single endpoint fits the existing `internal/api` package. Playwright spec goes under `ui/e2e/` per existing convention (`app.spec.ts`, `questions.spec.ts` live there). No new dirs.

## Architecture

### Components

```
Component: HealthHandler
Purpose: Serve liveness + version on GET /api/health
Responsibilities:
  - Read config.Version via s.pipeline.Config()
  - Emit JSON {"status":"ok","version":"<version>"} with 200
  - Never decode r.Body (GET)
Interfaces:
  - healthHandler(w http.ResponseWriter, r *http.Request) — method on *Server
Dependencies:
  - Pipeline.Config() accessor (NEW) for version
  - writeJSON (existing) for JSON response
```

```
Component: Pipeline.Config() accessor
Purpose: Expose the loaded *config.Config so Server can read Version
Responsibilities:
  - Return p.config (1 line)
Interfaces:
  - Config() *config.Config
Dependencies: none (reads existing struct field)
```

```
Component: Playwright health spec
Purpose: E2E cross-stack verification of /api/health through :18765 test server
Responsibilities:
  - GET /api/health via page.request against baseURL
  - Assert status 200 + JSON {status:"ok", version:"1.0"}
Interfaces: Playwright test file under ui/e2e/
Dependencies: Playwright webServer (existing config), devteam binary on :18765
```

### Component Dependency Map
```
Pipeline.Config()  ← HealthHandler ← Route registration (server.go)
                                      ↑
                          recoveryMiddleware + corsMiddleware (existing, wrap mux)
Playwright health spec → page.request → :18765 webServer → devteam binary (existing)
```
No cycles. No shared-state components. No multi-provider/multi-consumer split → **cross-component consistency matrix is trivial** (see below).

### Service Layer Design
Single handler, no orchestration. Stateless. Same request/response cycle as all existing `/api/*` endpoints.

## Data Model
See `data-model.md`. Single ephemeral entity `HealthStatus` (not persisted). Go struct `healthResponse{Status, Version string}` with JSON tags — field order guarantees byte-exact `{"status":"ok","version":"..."}` (CON-006).

## API Contracts
See `contracts/GET-api-health.md`. Summary:
- `GET /api/health` → 200 `{"status":"ok","version":"1.0"}` (default config)
- POST/PUT/DELETE/PATCH → 405 (stdlib method-pattern, auto)
- `/api/health/` → 404 (exact path only)
- Handler panic → 500 via recoveryMiddleware (CON-002)

## Constraint Verification Map — MANDATORY

| CON-ID | Design Decision | Component(s) | Verification Checkpoint | Test Type |
|--------|-----------------|--------------|------------------------|-----------|
| CON-001 | Route registered via `mux.HandleFunc("GET /api/health", s.healthHandler)` in NewServer, before the `staticFS` catch-all | server.go NewServer | Grep server.go for the HandleFunc call; integration test GET /api/health returns 200 (proves route registered) | Integration (AC-001) |
| CON-002 | Route registered on `mux` which is wrapped by `s.recoveryMiddleware(s.corsMiddleware(mux))` — no bypass possible. Handler panic → recoveryMiddleware emits 500 | recoveryMiddleware, healthHandler | Integration test: induce panic in a health-handler variant, assert 500 + server stays alive (AC-005) | Integration (AC-005) |
| CON-003 | `version` field read from `s.pipeline.Config().Version` (new 1-line accessor on Pipeline); NOT hardcoded in handler | Pipeline.Config(), healthHandler | Integration test: Server with `Config{Version:"9.9.9-test"}` → response body `{"status":"ok","version":"9.9.9-test"}` (AC-003) | Integration (AC-003) |
| CON-004 | Health tests use `httptest.NewServer(s.httpServer.Handler)` + `http.Get/Post` + `json.Decode`, matching `server_test.go` pattern | server_test.go | Test file uses httptest.NewServer; `go test` passes | Integration (AC-001..AC-011) |
| CON-005 | Playwright spec uses `baseURL` (http://localhost:18765 from playwright.config.ts); webServer auto-starts devteam binary on :18765 | ui/e2e/health.spec.ts | `npx playwright test` runs the spec against :18765, not prod :8765 (AC-012, AC-013) | E2E (AC-012, AC-013) |
| CON-006 | `healthResponse` struct with fields ordered `Status` then `Version`; `json.Encoder.Encode` emits keys in struct order → byte-exact `{"status":"ok","version":"1.0"}` for default config | healthResponse, healthHandler | Integration test: byte/string assertion body == `{"status":"ok","version":"1.0"}` (AC-001) | Integration (AC-001) |
| CON-007 | Only `GET /api/health` registered; Go 1.22+ ServeMux method-pattern emits 405 automatically for POST/PUT/DELETE/PATCH with `Allow` header | NewServer route registration | Integration test per method: POST→405, PUT→405, DELETE→405, PATCH→405 (AC-006..009) | Integration (AC-006..009) |

**All 7 constraints have a design decision + verification checkpoint + test.** No constraint unaddressed.

## Cross-Component Consistency Matrix — MANDATORY

This feature has no multi-provider/multi-consumer split (single handler, single config source, single response shape). Matrix is trivial but included for completeness:

| Shared Value | Producer | Consumer | Consistent? | Verification |
|---|---|---|---|---|
| `version` string | `config.Config.Version` (devteam.yaml) → `Pipeline.Config()` → `healthHandler` | HTTP response `version` field | YES — single source, single read, no transform | AC-003 (custom version round-trip) |
| Response JSON shape | `healthResponse` struct (field order: Status, Version) | All tests asserting body | YES — struct field order = JSON key order = byte-exact expectation | AC-001 (byte assertion) |
| 405 method set | Go ServeMux method-pattern (only GET registered) | AC-006..009 expectations | YES — stdlib emits 405 for exactly POST/PUT/DELETE/PATCH | AC-006..009 |
| 500 panic path | `recoveryMiddleware` (existing, server.go:226) | AC-005 expectation | YES — recovery is outermost middleware, covers all routes including health | AC-005 |

No inconsistency possible — single producer per value. The classic multi-component bug (provider A emits X, consumer B rejects X) does not apply here.

## Negative Case Design

For every constraint with a negative test vector, how the implementation rejects it:

| Vector | Expected Rejection | Test |
|---|---|---|
| POST /api/health (CON-007) | 405 Method Not Allowed | AC-006 |
| PUT /api/health (CON-007) | 405 | AC-007 |
| DELETE /api/health (CON-007) | 405 | AC-008 |
| PATCH /api/health (CON-007) | 405 | AC-009 |
| /api/health/ trailing slash (edge case) | 404 Not Found | AC-011 |
| Handler panic (CON-002 negative) | 500, process survives | AC-005 |
| Empty config.Version (edge case) | 200 with `version:""` (faithful reflection, NOT rejected — by design per spec assumption) | Documented in data-model.md; no dedicated test required (AC-003 covers non-empty custom value) |

No external RFC negative vectors (no standard governs this feature). All negative cases are HTTP-semantics + repo-convention derived.

## Test Strategy

### Component: HealthHandler (internal/api/server.go)
Testing levels required:
- **Smoke**: Server starts, `GET /api/health` returns 200 (proves route registered + handler non-panicking)
- **Integration**: 
  - GET → 200, `Content-Type: application/json`, body `{"status":"ok","version":"1.0"}` (default config) — AC-001, CON-006
  - GET with no body → 200 same body (handler does not decode r.Body) — AC-002
  - GET with custom `Config{Version:"9.9.9-test"}` → body `{"status":"ok","version":"9.9.9-test"}` — AC-003, CON-003
  - GET `?cb=123` → 200 standard body (query ignored) — AC-004
  - POST → 405 — AC-006
  - PUT → 405 — AC-007
  - DELETE → 405 — AC-008
  - PATCH → 405 — AC-009
  - GET → 200 (positive control alongside 405s) — AC-010
  - GET `/api/health/` → 404 — AC-011
  - Induced panic → 500, server stays alive — AC-005, CON-002
- **Unit**: none — handler has no isolated business logic (just struct + writeJSON)
- **E2E**: covered by Playwright component below

Quality checkpoints:
- [ ] `go test ./internal/api/` passes with all health tests
- [ ] GET body is byte-exact `{"status":"ok","version":"1.0"}` (not just JSON-equal — field order matters for CON-006)
- [ ] 405 responses have status 405 (not 200, not 404)
- [ ] Panic test confirms subsequent request still succeeds (server alive)
- [ ] No `r.Body` decode in handler (grep-verified)

### Component: Pipeline.Config() accessor (internal/pipeline/pipeline.go)
Testing levels required:
- **Unit**: accessor returns the same `*config.Config` passed to `NewPipeline` — covered transitively by AC-003 (if accessor returned wrong/nil config, version would be wrong/empty)
- No dedicated test needed — 1-line accessor, exercised by every health integration test.

### Component: Playwright health spec (ui/e2e/health.spec.ts)
Testing levels required:
- **E2E**: 
  - `page.request.get(baseURL + '/api/health')` → status 200, json `{status:"ok", version:"1.0"}` — AC-012
  - `npx playwright test` discovers + passes the spec (not skipped) — AC-013
- **Smoke**: webServer starts on :18765 (existing playwright.config.ts handles this)

Quality checkpoints:
- [ ] Spec file lives under `ui/e2e/` (discovered by `testDir: './e2e'`)
- [ ] No `.skip` on the health test
- [ ] Uses `baseURL` (not hardcoded `http://localhost:18765`) so it respects `process.env.BASE_URL`
- [ ] Asserts both `status` AND `version` fields (not just status code)

### Test Level Selection Matrix (applied)
| What changed | Smoke | Integration | E2E | Unit |
|---|---|---|---|---|
| HTTP API handler (healthHandler) | **YES** | **YES** | YES (via Playwright) | — |
| Pipeline.Config() accessor | YES (transitive) | — | — | — (transitive) |
| Playwright spec | YES | — | **YES** | — |

## Quality Checkpoints (task boundaries)
- After T-001 (accessor): `go build ./...` compiles. No test yet.
- After T-002 (handler+route): `go build ./...` compiles; `go test ./internal/api/ -run TestHealth` passes (requires T-003 tests written or run together — see tasks).
- After T-003 (Go tests): all health integration tests pass; `go test ./internal/api/` green.
- After T-004 (Playwright spec): `npx playwright test health.spec.ts` green against :18765.

## NFR Considerations
- **Performance**: no targets; handler is trivial. No concern.
- **Security**: no auth (consistent with existing endpoints, spec FR-007). Health endpoint exposes only `status:"ok"` + config version — version is already public via the running service's behavior; no sensitive data. No input validation needed (GET, no body, no params parsed).
- **Scalability**: stateless handler, no DB. Scales with the server.
- **Reliability**: panic → recovery middleware → 500 (CON-002). No retry/backoff needed (liveness probe, clients retry by nature).

## Quickstart Guide for the Developer

1. **Read first**: `spec.md`, `acceptance.md`, this `plan.md`, `research.md`, `contracts/GET-api-health.md`, `data-model.md`, `tasks.md`.
2. **Read existing code**: `internal/api/server.go` (lines 21-32 struct, 160-202 NewServer, 898-906 writeJSON/writeError, 226-236 recoveryMiddleware), `internal/api/server_test.go` (`setupTestServer`), `internal/pipeline/pipeline.go` (lines 26, 279-292), `ui/playwright.config.ts`, `ui/e2e/app.spec.ts`.
3. **Implement in order**: T-001 (accessor) → T-002 (handler+route) → T-003 (Go tests) → T-004 (Playwright spec). T-001 and T-002 may be one commit; T-003 must be same commit or immediately after (tests alongside code).
4. **Verify**: `go build ./... && go test ./internal/api/` then `cd ui && npx playwright test health.spec.ts`.
5. **Self-verify before signaling done**: run the server (`~/go/bin/devteam -http :8765`), `curl -i http://localhost:8765/api/health` → 200 + expected body. `curl -i -X POST http://localhost:8765/api/health` → 405.
6. **Agent failure mode checks** (from tasks.md): nil-pointer ordering (Server.pipeline is set in NewServer before any request); JSON null vs empty (no arrays in response — N/A); recovery middleware first (already true, no change); parsing safety (no parsing — GET, no body decode); multi-component consistency (single component, N/A).