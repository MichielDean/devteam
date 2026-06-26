# API Documentation — Health Check Endpoint

**Spec**: playwright-e2e — *Health Check Endpoint (GET /api/health)*
**Spec number**: playwright-e2e
**Status**: Delivered

This feature adds a single HTTP liveness endpoint, `GET /api/health`, to the
existing devteam web service. The endpoint lets an operator or monitoring
system verify process liveness and the deployed version without hitting
business endpoints.

Terminology below follows `specs/playwright-e2e/spec.md`:
`HealthStatus` (ephemeral response entity), `Config.Version` (version string
sourced from `devteam.yaml`), `recoveryMiddleware` (existing outermost
middleware), `corsMiddleware`.

---

## Endpoints

### GET /api/health

**Purpose**: Report process liveness and the deployed version. Returns a
`HealthStatus` JSON body with HTTP 200. No authentication required
(consistent with all existing `/api/*` endpoints — no auth middleware on any
of them). No request body is read; the handler does not decode `r.Body`
(US-001 scenario 2, FR-002).

**Request**:
- Method: `GET` (only). `HEAD` is auto-allowed by Go's `http.ServeMux`
  (read-only, acceptable — no body is produced on HEAD).
- Path: `/api/health` (exact). The trailing-slash variant `/api/health/` is
  **not** registered and returns `404 Not Found` (AC-011, scope assumption).
- Headers: none required. No auth header.
- Body: none expected. Must not be decoded.
- Query parameters: ignored. e.g. `GET /api/health?cb=123` still returns the
  standard body (AC-004, scope assumption — monitoring tools commonly append
  cache-busters).

**Response 200** — success (AC-001, AC-002, AC-003, AC-004, AC-010):

| Field    | Type   | Description |
|----------|--------|-------------|
| `status` | string | Always `"ok"`. Fixed value; no validation (constant). |
| `version`| string | Value of the loaded `Config.Version` from `devteam.yaml`. Default config → `"1.0"`. Empty config → `""` (faithful reflection, never fabricated). |

Headers:
- `Content-Type: application/json`

Body is byte-exact:
```
{"status":"ok","version":"1.0"}
```
(with a trailing newline from the JSON encoder). The `status` key precedes
`version` (CON-006 — field order guarantees the byte-exact body for the
default config).

**Response 405 Method Not Allowed** — non-GET method (AC-006, AC-007,
AC-008, AC-009, FR-004, CON-007, RFC 9110 §15.5.5):

Emitted automatically by Go 1.22+ `http.ServeMux` method-pattern routing
when the route is registered as `GET /api/health`. Applies to `POST`, `PUT`,
`DELETE`, and `PATCH`.

Headers:
- `Allow: GET, HEAD`
- `Content-Type: text/plain; charset=utf-8`

Body: Go stdlib default `Method Not Allowed\n` (plain text, not JSON). The
spec accepts "empty or JSON-error body"; plain text satisfies this.

**Response 404 Not Found** — trailing slash / unregistered path (AC-011):

Headers:
- `Content-Type: text/plain; charset=utf-8`

Body: `404 page not found\n` (Go stdlib default).

**Response 500 Internal Server Error** — handler panic (AC-005, CON-002):

Emitted by the existing `recoveryMiddleware` (outermost middleware, wraps
every route including `/api/health`). The process stays alive; a subsequent
request still succeeds.

Headers:
- `Content-Type: application/json`

Body:
```json
{"error":"internal_error","details":"An unexpected error occurred"}
```

---

## Example Requests

```bash
# Happy path — process liveness + version
curl -i http://localhost:8765/api/health
# HTTP/1.1 200 OK
# Content-Type: application/json
# {"status":"ok","version":"1.0"}

# Query parameter ignored (cache-buster)
curl -i 'http://localhost:8765/api/health?cb=123'
# → 200, {"status":"ok","version":"1.0"}

# Non-GET method → 405
curl -i -X POST http://localhost:8765/api/health
# → 405, Allow: GET, HEAD

# Trailing slash → 404
curl -i http://localhost:8765/api/health/
# → 404
```

---

## Error Scenarios

| User Action | Success | Error Condition | Expected Response |
|---|---|---|---|
| `GET /api/health` | 200 `{"status":"ok","version":"1.0"}` | — (no failure path for liveness probe) | 500 via `recoveryMiddleware` if handler panics |
| `POST /api/health` | n/a | non-GET method | 405 |
| `PUT /api/health` | n/a | non-GET method | 405 |
| `DELETE /api/health` | n/a | non-GET method | 405 |
| `PATCH /api/health` | n/a | non-GET method | 405 |
| `GET /api/health/` (trailing slash) | n/a | path not registered | 404 |

---

## Constraint Trace

| Constraint | How satisfied |
|---|---|
| CON-001 | Route registered as `mux.HandleFunc("GET /api/health", s.healthHandler)`, consistent with the Go 1.22+ method-pattern convention used by every existing `/api/*` endpoint. |
| CON-002 | Route is on the same `mux` wrapped by `s.recoveryMiddleware(s.corsMiddleware(mux))`; a handler panic is converted to 500, process survives. |
| CON-003 | `version` field sourced from `Config.Version` via a `Pipeline.Config()` accessor; not hardcoded in the handler. Changing `devteam.yaml` `version` changes the response. |
| CON-006 | 200 body is byte-exact `{"status":"ok","version":"1.0"}` for the default config (struct field order `status` then `version`). |
| CON-007 | 405 for `POST`/`PUT`/`DELETE`/`PATCH` via Go 1.22+ ServeMux method-pattern (RFC 9110 §15.5.5). |