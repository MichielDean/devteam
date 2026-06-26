# Acceptance Criteria — playwright-e2e

Every criterion is testable at a specific level. Each user story has criteria at every relevant test level.

## US-001 — Health Check Probe (P1)

AC-001: Given the server is running with default config, when a client sends `GET /api/health`, then the response status is 200, `Content-Type` header contains `application/json`, and the body equals `{"status":"ok","version":"1.0"}`.
  Test level: smoke
  Verification: `httptest.NewServer` request; assert `resp.StatusCode == 200`, `strings.Contains(resp.Header.Get("Content-Type"), "application/json")`, and body string equals `{"status":"ok","version":"1.0"}`.

AC-002: Given the server is running, when a client sends `GET /api/health` with no request body, then the response is 200 with body `{"status":"ok","version":"1.0"}` (body-less GET must not error).
  Test level: integration
  Verification: `httptest.NewServer` GET with `http.MethodGet`, empty body; assert 200 and body. Confirm handler does not attempt `r.Body` decode.

AC-003: Given the loaded `Config.Version` is `"9.9.9-test"`, when a client sends `GET /api/health`, then the response body is `{"status":"ok","version":"9.9.9-test"}` (version sourced from config, not hardcoded).
  Test level: integration
  Verification: Construct Server with `Config{Version: "9.9.9-test"}`; `httptest.NewServer`; GET `/api/health`; assert body `{"status":"ok","version":"9.9.9-test"}`.

AC-004: Given the server is running, when a client sends `GET /api/health?cb=123`, then the response is 200 with the standard body (query params ignored).
  Test level: integration
  Verification: `httptest` GET with query string; assert 200 and body `{"status":"ok","version":"1.0"}`.

AC-005: Given the health handler panics, when a client sends `GET /api/health`, then the recovery middleware returns HTTP 500 rather than crashing the process.
  Test level: integration
  Verification: Inject a handler variant that panics (or temporarily wrap to force panic); `httptest` GET; assert `resp.StatusCode == 500` and server process stays alive for subsequent requests.

## US-002 — Method Restriction (P2)

AC-006: Given the server is running, when a client sends `POST /api/health`, then the response status is 405.
  Test level: integration
  Verification: `httptest` POST `/api/health` with empty body; assert `resp.StatusCode == 405`.

AC-007: Given the server is running, when a client sends `PUT /api/health`, then the response status is 405.
  Test level: integration
  Verification: `httptest` PUT `/api/health`; assert 405.

AC-008: Given the server is running, when a client sends `DELETE /api/health`, then the response status is 405.
  Test level: integration
  Verification: `httptest` DELETE `/api/health`; assert 405.

AC-009: Given the server is running, when a client sends `PATCH /api/health`, then the response status is 405.
  Test level: integration
  Verification: `httptest` PATCH `/api/health`; assert 405.

AC-010: Given the server is running, when a client sends `GET /api/health`, then the response status is 200 (GET remains the only allowed method alongside the 405s above).
  Test level: integration
  Verification: `httptest` GET `/api/health`; assert 200. (Positive control for AC-006..009.)

AC-011: Given the server is running, when a client sends `GET /api/health/` (trailing slash), then the response status is 404 (only the exact path is registered).
  Test level: integration
  Verification: `httptest` GET `/api/health/`; assert `resp.StatusCode == 404`.

## US-003 — Playwright E2E Coverage (P3)

AC-012: Given the Playwright `webServer` is running on :18765, when the E2E test issues `GET /api/health` (via `page.request.get` or `fetch`), then the response status is 200 and the JSON body has `status === "ok"` and `version === "1.0"`.
  Test level: e2e
  Verification: Playwright spec under `ui/e2e/` asserts `response.status()` === 200 and `await response.json()` yields `{status: "ok", version: "1.0"}`.

AC-013: Given the Playwright suite is executed, when `npx playwright test` runs, then the health E2E test is discovered (not skipped) and passes.
  Test level: e2e
  Verification: `npx playwright test` output shows the health spec file ran with status `passed`; grep test report for the spec name. No `.skip` on the health test.

## Constraint Coverage

| Constraint | Acceptance Criteria |
|---|---|
| CON-001 (method-pattern routing) | AC-001 (endpoint served implies route registered) + code review grep |
| CON-002 (middleware chain) | AC-005 |
| CON-003 (version from config) | AC-003 |
| CON-004 (httptest pattern) | AC-001..AC-011 |
| CON-005 (Playwright :18765) | AC-012, AC-013 |
| CON-006 (exact body) | AC-001 |
| CON-007 (405 for non-GET, RFC 9110 §15.5.5) | AC-006, AC-007, AC-008, AC-009 |