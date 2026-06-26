# Changelog

All entries reference the spec number `playwright-e2e` (the feature branch
name and spec id). Format follows [Keep a Changelog](https://keepachangelog.com/).

## [1.0] - 2026-06-25

### Added
- Health check endpoint `GET /api/health` returning `{"status":"ok","version":"<version>"}` with HTTP 200, for process liveness and deployed-version verification (spec playwright-e2e, US-001, FR-001, FR-002, FR-003, FR-007).
- `Content-Type: application/json` header on the health response (spec playwright-e2e, FR-003, AC-001).
- `version` field sourced from `Config.Version` (`devteam.yaml`), not hardcoded (spec playwright-e2e, US-001 scenario 3, FR-002, CON-003, AC-003).
- 405 Method Not Allowed for `POST`, `PUT`, `DELETE`, and `PATCH` on `/api/health`, with `Allow: GET, HEAD` header (spec playwright-e2e, US-002, FR-004, CON-007, AC-006..AC-009).
- Playwright E2E spec `ui/e2e/health.spec.ts` asserting `GET /api/health` status 200 and JSON body against the `:18765` test server (spec playwright-e2e, US-003, FR-006, CON-005, AC-012, AC-013).
- `Pipeline.Config()` accessor exposing the loaded `*config.Config` for the health handler (spec playwright-e2e, plan.md).
- Go `httptest` integration tests covering AC-001 through AC-011 (spec playwright-e2e, acceptance.md).

### Changed
- `internal/api/server.go`: registered `mux.HandleFunc("GET /api/health", s.healthHandler)` alongside the existing `/api/*` routes, reusing the existing `recoveryMiddleware` + `corsMiddleware` chain (spec playwright-e2e, FR-005, CON-001, CON-002).
- `internal/pipeline/pipeline.go`: added a one-line `Config()` accessor (spec playwright-e2e, plan.md).

### Fixed
- _None._

### Breaking changes
- _None._ The feature is additive: one new endpoint, no change to existing
  routes, no schema change, no DB migration, no new dependencies.

### Out of scope (deliberately not added)
- Database / dependency readiness checks — liveness only (spec playwright-e2e, scope).
- Authentication on `/api/health` — none, consistent with existing `/api/*` (spec playwright-e2e, FR-007).
- `/api/health/live` vs `/api/health/ready` split — single endpoint only (spec playwright-e2e, scope).
- Cache headers (`Cache-Control`) — not requested (spec playwright-e2e, scope).
- Version sourcing via build-time ldflags — config-sourced only (spec playwright-e2e, scope).