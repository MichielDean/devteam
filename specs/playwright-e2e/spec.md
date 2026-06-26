# Feature Specification: Health Check Endpoint (GET /api/health)

**Feature Branch**: `spec/playwright-e2e`

**Created**: 2026-06-25

**Status**: Draft

**Input**: User description: "Add a simple health check endpoint at GET /api/health that returns {"status":"ok","version":"1.0"}. This is a minimal feature to test the full pipeline end-to-end."

**Priority**: P3 — minimal pipeline-exercise feature, not a user-facing capability.

## Workspace Summary (Brownfield)

Target repo: `devteam` (primary). Go 1.26.1 module `github.com/MichielDean/devteam`.

- **HTTP API**: `internal/api/server.go` uses Go 1.22+ `http.NewServeMux` method-pattern routing (e.g. `mux.HandleFunc("GET /api/features", s.listFeatures)`). Routes registered in `NewServer` / constructor around line 160-188.
- **Middleware**: `s.recoveryMiddleware(s.corsMiddleware(mux))` wraps all routes (server.go:194). No auth middleware exists on any current endpoint.
- **Config**: `internal/config/config.go` defines `Config` with `Version string` field (`yaml:"version"`), loaded from `devteam.yaml` (currently `version: "1.0"`). Server holds the loaded `Config` and exposes it to handlers.
- **Tests**: `internal/api/server_test.go` (~31KB) uses `httptest` in-process server testing. Pattern: construct `Server`, spin `httptest.NewServer`, hit endpoints, assert JSON via `encoding/json`.
- **E2E**: `ui/e2e/` Playwright suite, config `ui/playwright.config.ts` on port :18765. `webServer` auto-starts a test binary from repo root.
- **Conventions**: AGENTS.md forbids phase instructions from hardcoding build/test commands or ports — but spec/implementation-level specifics are fine. No `constitution.md` exists at repo root or `.specify/`.
- **No existing `/api/health` endpoint** (grep confirmed).

Conventions to follow: Go 1.22+ method-pattern `mux.HandleFunc`; handler method on `*Server`; JSON via `encoding/json` (matching existing handlers); `httptest` for tests; Playwright spec file under `ui/e2e/` for E2E.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Health Check Probe (Priority: P1)

An operator or monitoring system can send `GET /api/health` and receive a JSON body `{"status":"ok","version":"1.0"}` with HTTP 200, so that process liveness and deployed version can be verified without hitting business endpoints.

**Why this priority**: This is the entire feature. Without it, nothing exists. P1 because it is the must-have MVP slice; implementing only this story yields a viable, demonstrable health endpoint.

**Independent Test**: Can be fully tested by issuing `GET /api/health` against a running server and asserting status 200 + JSON body `{"status":"ok","version":"1.0"}`. No other story needed.

**Acceptance Scenarios**:

1. **Given** the server is running, **When** a client sends `GET /api/health`, **Then** the response is HTTP 200 with header `Content-Type: application/json` and body `{"status":"ok","version":"1.0"}`.
2. **Given** the server is running, **When** a client sends `GET /api/health` with no request body, **Then** the response is still 200 with the same body (body-less GET must not error).
3. **Given** the devteam.yaml `version` field is `"1.0"`, **When** `GET /api/health` is invoked, **Then** the `version` field in the response equals the config version, not a separately hardcoded literal.

---

### User Story 2 - Method Restriction on Health Endpoint (Priority: P2)

An operator can rely on `/api/health` being a read-only GET-only endpoint, so that non-GET methods receive a deterministic error response rather than a 200 or a 405-with-body that confuses probes.

**Why this priority**: Hardens the endpoint but is not required for the happy-path MVP. The pipeline-exercise goal (US-1) does not depend on method restriction.

**Independent Test**: Send POST/PUT/DELETE to `/api/health` and assert each returns 405 with an empty or JSON-error body (and never 200). GET still returns 200.

**Acceptance Scenarios**:

1. **Given** the server is running, **When** a client sends `POST /api/health`, **Then** the response is HTTP 405.
2. **Given** the server is running, **When** a client sends `PUT /api/health`, **Then** the response is HTTP 405.
3. **Given** the server is running, **When** a client sends `DELETE /api/health`, **Then** the response is HTTP 405.
4. **Given** the server is running, **When** a client sends `GET /api/health`, **Then** the response is HTTP 200 (GET remains the only allowed method).

---

### User Story 3 - End-to-End Playwright Coverage (Priority: P3)

A developer can run the existing Playwright E2E suite and have it include a test that hits `/api/health` through the test web server, so that the health endpoint is verified through the real HTTP stack the same way the UI is.

**Why this priority**: Nice-to-have. The feature is named `playwright-e2e` and the repo already has Playwright infra, but the smoke + integration Go tests are sufficient for verification. E2E adds the cross-stack confidence the feature name implies.

**Independent Test**: Run `npx playwright test` and confirm a spec under `ui/e2e/` issues `GET /api/health` against :18765 and asserts the 200 + JSON body. Test passes without US-2.

**Acceptance Scenarios**:

1. **Given** the Playwright test server is running on :18765, **When** the E2E test issues `GET /api/health` via `page.request` or `fetch`, **Then** the response status is 200 and the JSON body has `status === "ok"` and `version === "1.0"`.
2. **Given** the Playwright suite is executed, **When** `npx playwright test` runs, **Then** the health E2E test is discovered and passes (not skipped).

---

### Edge Cases

- **Missing/no request body on GET**: must return 200 (GET has no body; handler must not attempt body decode). Covered by US-1 scenario 2.
- **Trailing slash** (`/api/health/`): Go 1.22+ ServeMux does NOT automatically merge `/api/health/` into `/api/health` unless a subtree pattern is registered. [ASSUMPTION: `/api/health/` (trailing slash) should return 404, not 200 — only the exact `/api/health` path is served. Aligns with existing endpoints which register exact paths like `GET /api/features`.]
- **Query parameters** (`/api/health?foo=bar`): must return 200 with the standard body. Query params are ignored. [ASSUMPTION: health probe ignores query strings; monitoring tools commonly append cache-busters.]
- **Empty state**: not applicable — no collection/list. The response is always a single fixed-shape object.
- **Config version field empty/missing**: [ASSUMPTION: if `config.Version` is empty string, response `version` field is the empty string `""`. The endpoint reflects config faithfully rather than fabricating a default. This matches "version sourced from config" decision.]

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system SHALL expose an HTTP endpoint at exact path `/api/health` accepting the `GET` method. *Source: US-001, US-002*
- **FR-002**: The system SHALL respond to `GET /api/health` with HTTP status 200 and a JSON body `{"status":"ok","version":"<version>"}` where `<version>` is the value of the loaded `Config.Version` (from `devteam.yaml`). *Source: US-001*
- **FR-003**: The system SHALL set the `Content-Type: application/json` response header on `GET /api/health`. *Source: US-001*
- **FR-004**: The system SHALL respond to `POST`, `PUT`, `DELETE`, and `PATCH` on `/api/health` with HTTP status 405. *Source: US-002*
- **FR-005**: The system SHALL register the health route using the existing Go 1.22+ `http.NewServeMux` method-pattern routing convention (e.g. `mux.HandleFunc("GET /api/health", s.healthHandler)`), consistent with `internal/api/server.go`. *Source: US-001, workspace conventions*
- **FR-006**: The system SHALL include a Playwright E2E test under `ui/e2e/` that issues `GET /api/health` against the test server and asserts status 200 and the JSON body. *Source: US-003*
- **FR-007**: The system SHALL NOT require authentication for `GET /api/health` (consistent with all existing `/api/*` endpoints, which have no auth middleware). *Source: US-001, workspace conventions*

### Key Entities *(include if feature involves data)*

- **HealthStatus**: ephemeral response entity (no persistence). Attributes: `status` (string, fixed value `"ok"`), `version` (string, sourced from `Config.Version`). No relationships, no lifecycle, no state transitions. Not stored.

## Error Scenarios

| User Action | Success | Error Condition | Expected Response |
|---|---|---|---|
| `GET /api/health` | 200 `{"status":"ok","version":"1.0"}` | — (no failure path for liveness probe; panic caught by recovery middleware → 500) | 500 via recovery middleware if handler panics |
| `POST /api/health` | n/a | non-GET method | 405 |
| `PUT /api/health` | n/a | non-GET method | 405 |
| `DELETE /api/health` | n/a | non-GET method | 405 |
| `PATCH /api/health` | n/a | non-GET method | 405 |
| `GET /api/health/` (trailing slash) | n/a | path not registered | 404 |

## Constraint Register

No external RFC or standard governs this feature. Sources discovered: repo conventions (AGENTS.md, existing `internal/api/server.go` patterns, `internal/config/config.go`), existing test patterns (`server_test.go` httptest, `ui/e2e/` Playwright). Constraints derived from internal conventions:

| ID | Source | Section/Vector | Type | Constraint | Verification Method |
|----|--------|----------------|------|------------|---------------------|
| CON-001 | repo convention | server.go:160-188 | consistency | Route registered via Go 1.22+ method-pattern `mux.HandleFunc("GET /api/health", ...)` | Grep server.go for the HandleFunc call; integration test hits endpoint |
| CON-002 | repo convention | server.go:194 | consistency | Health route is covered by existing `recoveryMiddleware` + `corsMiddleware` chain (no custom middleware bypass) | Integration test: recovery middleware returns 500 not panic crash on induced handler panic |
| CON-003 | repo convention | config.go:11 | consistency | `version` response field sourced from loaded `Config.Version`, not a hardcoded literal separate from config | Unit/integration test: load config with non-"1.0" version, assert response version matches |
| CON-004 | repo convention | server_test.go | consistency | New endpoint tested with `httptest` in-process server pattern (no external process) | Go test file uses httptest.NewServer |
| CON-005 | repo convention | ui/e2e, AGENTS.md :18765 | consistency | E2E test runs against Playwright `webServer` on :18765, not production :8765 | Playwright config webServer URL assertion |
| CON-006 | input.md | idea | correctness | Response body is exactly `{"status":"ok","version":"1.0"}` for the default config | Byte-level JSON assertion in integration test |
| CON-007 | HTTP semantics | RFC 9110 §15.5.5 | correctness | Non-GET methods on a GET-only resource return 405 Method Not Allowed | Integration test per method |

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `GET /api/health` against a running server returns HTTP 200 with JSON body `{"status":"ok","version":"1.0"}` (default config) — verified by at least one integration test that performs a byte/string assertion on the body.
- **SC-002**: `POST`, `PUT`, `DELETE`, and `PATCH` to `/api/health` each return HTTP 405 — verified by one integration test per method.
- **SC-003**: The `version` field in the response equals `Config.Version` when config is changed to a non-default value — verified by a unit or integration test that loads a custom version and asserts the response.
- **SC-004**: A Playwright E2E test under `ui/e2e/` exists, is discovered by `npx playwright test`, and passes, asserting status 200 and the JSON body against the :18765 test server.
- **SC-005**: The full Dev Team pipeline (inception → planning → construction → review → testing → delivery) completes end-to-end for this feature without manual intervention — the meta-goal of the feature.

## Assumptions

- [ASSUMPTION: `version` is sourced from `devteam.yaml` `config.Version` (currently `"1.0"`), not hardcoded as a separate literal in the handler. The input idea's `"version":"1.0"` matches the current config value; sourcing from config is the conservative choice that stays correct when config changes. Question Q1 was asked but unanswered; this assumption resolves it.]
- [ASSUMPTION: `GET /api/health` requires no authentication. All existing `/api/*` endpoints have no auth middleware (server.go:160-188, :194). Adding auth solely to health would break monitoring probes and diverge from convention. Q2 was asked but unanswered; this assumption resolves it.]
- [ASSUMPTION: Only `GET` is accepted on `/api/health`; all other methods return 405. Health probes are read-only by convention. Q3/Q4 were asked but unanswered; this assumption resolves them.]
- [ASSUMPTION: The endpoint reports process liveness only (is the server up and serving HTTP), not dependency/readiness health (no database ping). The input idea specifies only `status` and `version` fields — adding a DB check would expand scope and response shape beyond the idea. Q5 was asked but unanswered; this assumption resolves it with the minimal-scope conservative default.]
- [ASSUMPTION: A Playwright E2E test IS included, because the feature is explicitly named `playwright-e2e` and the repo has existing Playwright infra at `ui/e2e/`. The feature's stated purpose is to exercise the full pipeline including E2E. Q6 was asked but unanswered; this assumption resolves it.]
- [ASSUMPTION: `/api/health/` with a trailing slash returns 404 (only the exact path is registered), matching how existing endpoints register exact paths.]
- [ASSUMPTION: Query parameters on `/api/health` are ignored and still return 200, accommodating monitoring tools that append cache-busters.]
- [ASSUMPTION: No `constitution.md` exists (verified at repo root and `.specify/`), so no constitution compliance check is required.]

## Scope Boundaries

**In scope**:
- New `GET /api/health` route + handler in `internal/api/server.go`.
- 405 responses for non-GET methods on `/api/health`.
- Go integration/unit tests using `httptest`.
- One Playwright E2E spec under `ui/e2e/`.

**Out of scope**:
- Database/dependency readiness checks (liveness only).
- Authentication or authorization on the health endpoint.
- Metrics, tracing, or structured-logging integration beyond what the handler naturally produces.
- A `/api/health/live` vs `/api/health/ready` split (single endpoint only).
- UI/dashboard visualization of health status.
- Caching headers (`Cache-Control`) — [ASSUMPTION: no cache headers added; not requested.]
- Version sourcing from build-time ldflags — config-sourced only.

## Constitution Compliance

No `constitution.md` exists at repo root or `.specify/constitution.md`. No constitution compliance check applicable.