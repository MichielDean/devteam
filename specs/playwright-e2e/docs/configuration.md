# Configuration — playwright-e2e

**Spec**: playwright-e2e — *Health Check Endpoint (GET /api/health)*

The health endpoint introduces **no new configuration knobs**. It reuses
the existing `Config.Version` field already loaded from `devteam.yaml`.

## Config file: `devteam.yaml`

The endpoint reads a single existing field:

| Field | Type | Required | Default | Used by `/api/health` |
|---|---|---|---|---|
| `version` | string | no (defaults to empty) | `"1.0"` (shipped config) | Becomes the `version` field in the `HealthStatus` response body. |

Example (current shipped config):

```yaml
version: "1.0"
```

Changing `version` and restarting the service changes the response body —
e.g. setting `version: "1.4.2"` makes `GET /api/health` return
`{"status":"ok","version":"1.4.2"}`. An empty `version` produces
`{"status":"ok","version":""}` (faithful reflection; the endpoint never
fabricates a default version).

## Environment variables

The health endpoint introduces **no new environment variables**. It does
not read `DEVTEAM_DB_DRIVER`, `DEVTEAM_DB_DSN`, or any other env var
directly. Those continue to govern the existing database layer as
documented in `devteam.yaml`.

## Dependencies

### Go (server side)
- Go 1.26.1 (module `github.com/MichielDean/devteam`).
- stdlib only for the handler: `net/http`, `encoding/json`. **No new Go
  dependencies added by this feature.**

### Playwright (E2E)
- `@playwright/test` — already installed in `ui/` before this feature. The
  new spec `ui/e2e/health.spec.ts` uses the existing `baseURL`
  (`http://localhost:18765`) and `webServer` (auto-starts a devteam test
  binary on `:18765`) from `ui/playwright.config.ts`. No new npm
  dependencies.

## Ports

| Port | Purpose | Used by this feature? |
|---|---|---|
| `:8765` | Production devteam HTTP API. | Yes — `/api/health` is served here in production. |
| `:18765` | Playwright `webServer` test port. | Yes — the E2E spec hits `/api/health` here (CON-005). |

The E2E spec uses `baseURL` from `playwright.config.ts` (which resolves to
`:18765`), never the production `:8765` port, so test traffic never reaches
production.

## Database

No DB changes. The `HealthStatus` entity is ephemeral — not persisted, no
table, no migration. Liveness only; the endpoint does not ping the
database.

## Authentication

None. `GET /api/health` requires no auth, consistent with every existing
`/api/*` endpoint (none carry auth middleware). Adding auth solely to the
health probe would break monitoring probes and diverge from convention
(spec FR-007, assumption Q2).

## Caching

No `Cache-Control` or other cache headers are set on the health response
(out of scope, spec assumption). Monitoring tools that append cache-buster
query strings (e.g. `?cb=123`) are handled — query params are ignored and
the response is still 200 with the standard body (AC-004).