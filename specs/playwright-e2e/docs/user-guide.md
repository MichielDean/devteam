# User Guide — Health Check Endpoint

**Spec**: playwright-e2e — *Health Check Endpoint (GET /api/health)*
**Spec number**: playwright-e2e

This feature adds a health check endpoint to the devteam web service. It is
a minimal P3 feature whose primary goal is to exercise the full Dev Team
pipeline (inception → planning → construction → review → testing → delivery)
end-to-end. The user-facing capability is a single liveness probe.

Terminology matches `specs/playwright-e2e/spec.md`: **Health Check Probe**,
**Method Restriction**, **End-to-End Playwright Coverage**, `HealthStatus`,
`Config.Version`.

---

## User Story 1 — Health Check Probe (P1)

> An operator or monitoring system can send `GET /api/health` and receive a
> JSON body `{"status":"ok","version":"1.0"}` with HTTP 200, so that process
> liveness and deployed version can be verified without hitting business
> endpoints.

### What you get

- A `GET /api/health` endpoint on the devteam web service.
- HTTP 200 with `Content-Type: application/json`.
- Body `{"status":"ok","version":"<version>"}` where `<version>` is the
  `version` field from `devteam.yaml` (`Config.Version`). With the default
  config this is `{"status":"ok","version":"1.0"}`.
- No authentication required (consistent with all existing `/api/*`
  endpoints — none have auth middleware).
- Liveness only: the endpoint reports whether the process is up and serving
  HTTP. It does **not** ping the database or any other dependency
  (readiness checks are out of scope).

### Common workflows

Point a monitoring tool / load balancer / Kubernetes liveness probe at
`GET /api/health` and treat HTTP 200 as "process alive". Parse the `version`
field to confirm the deployed build.

```bash
curl -s http://localhost:8765/api/health
# {"status":"ok","version":"1.0"}
```

### Version is sourced from config, not hardcoded

The `version` field in the response equals `Config.Version` from
`devteam.yaml`. If you change the `version` field in `devteam.yaml` and
restart the service, the response changes accordingly. An empty
`Config.Version` produces an empty `version` field (`""`) — the endpoint
reflects config faithfully rather than fabricating a default.

### Edge cases covered

- **No request body on GET**: returns 200 with the standard body. GET has
  no body; the handler does not attempt to decode one.
- **Query parameters** (e.g. `?cb=123`): ignored; still returns 200 with
  the standard body. Monitoring tools commonly append cache-busters.
- **Trailing slash** (`/api/health/`): returns 404. Only the exact
  `/api/health` path is registered.
- **Empty config version**: returns 200 with `version: ""`.

### Error messages

| Scenario | Response | Meaning |
|---|---|---|
| Handler panic | 500 `{"error":"internal_error","details":"An unexpected error occurred"}` | Internal failure caught by `recoveryMiddleware`; process stays alive. |

---

## User Story 2 — Method Restriction on Health Endpoint (P2)

> An operator can rely on `/api/health` being a read-only GET-only endpoint,
> so that non-GET methods receive a deterministic error response rather than
> a 200 or a 405-with-body that confuses probes.

### What you get

- `POST`, `PUT`, `DELETE`, and `PATCH` to `/api/health` each return HTTP
  405 Method Not Allowed.
- `GET` remains the only method that returns 200.
- The 405 body is Go's default plain text `Method Not Allowed`, with an
  `Allow: GET, HEAD` header. No custom 405 handler, no JSON-error body that
  could confuse a probe parser.

### Examples

```bash
curl -i -X POST http://localhost:8765/api/health   # → 405
curl -i -X PUT  http://localhost:8765/api/health   # → 405
curl -i -X DELETE http://localhost:8765/api/health # → 405
curl -i -X PATCH http://localhost:8765/api/health  # → 405
curl -i http://localhost:8765/api/health           # → 200
```

### Error messages

| Scenario | Response | Meaning |
|---|---|---|
| Non-GET method | 405 `Method Not Allowed` (plain text) | The endpoint is GET-only; `Allow` header lists `GET, HEAD`. |

---

## User Story 3 — End-to-End Playwright Coverage (P3)

> A developer can run the existing Playwright E2E suite and have it include
> a test that hits `/api/health` through the test web server, so that the
> health endpoint is verified through the real HTTP stack the same way the
> UI is.

### What you get

- A Playwright spec under `ui/e2e/` (`health.spec.ts`) that issues
  `GET /api/health` against the test web server on `:18765` and asserts
  status 200 plus the JSON body `{status: "ok", version: "1.0"}`.
- The spec is discovered by `npx playwright test` (not skipped) and passes.

### Running the E2E test

```bash
cd ui
npx playwright test health.spec.ts
```

The Playwright `webServer` (configured in `ui/playwright.config.ts`)
auto-starts a devteam test binary on `:18765`; the spec uses `baseURL`
(`http://localhost:18765`), not the production `:8765` port. This keeps E2E
verification on the test stack the same way the UI is verified.

### Why E2E

The smoke + integration Go tests already verify the endpoint via
`httptest`. The E2E test adds cross-stack confidence: the health endpoint is
exercised through the real HTTP server, middleware chain, and Playwright
`webServer` lifecycle — the same path the UI takes. The feature is named
`playwright-e2e`; the E2E coverage is the point.