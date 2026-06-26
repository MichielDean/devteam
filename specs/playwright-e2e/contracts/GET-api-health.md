# Contract: GET /api/health

Liveness probe. No persistence, no auth, no request body decode.

## Request

- **Method**: `GET` (only). `HEAD` auto-allowed by Go ServeMux (read-only, acceptable).
- **Path**: `/api/health` (exact; trailing slash `/api/health/` → 404, not registered).
- **Headers**: none required. No auth.
- **Body**: none expected. Handler MUST NOT attempt `r.Body` decode (GET has no body; AC-002).
- **Query params**: ignored (AC-004). e.g. `?cb=123` still returns 200 + standard body.

## Responses

### 200 OK — success
```
HTTP/1.1 200 OK
Content-Type: application/json

{"status":"ok","version":"1.0"}
```
- `status`: string, always `"ok"`
- `version`: string, value of loaded `config.Config.Version` (from `devteam.yaml`). Default config → `"1.0"`. Empty config → `""`.
- Body is byte-exact `{"status":"ok","version":"<version>"}` + trailing newline from `json.Encoder` (CON-006).

### 405 Method Not Allowed — non-GET method
```
HTTP/1.1 405 Method Not Allowed
Allow: GET, HEAD
Content-Type: text/plain; charset=utf-8

Method Not Allowed
```
- Emitted automatically by Go 1.22+ `http.NewServeMux` for POST/PUT/DELETE/PATCH on the registered path.
- Body is Go's default `Method Not Allowed\n` plain text (not JSON). Spec accepts "empty or JSON-error body" — plain text satisfies "empty or". No custom 405 handler.
- `Allow` header set by stdlib.

### 404 Not Found — trailing slash or unregistered path
```
HTTP/1.1 404 Not Found
Content-Type: text/plain; charset=utf-8

404 page not found
```
- `/api/health/` (trailing slash) → 404 (AC-011). Only exact `/api/health` registered.

### 500 Internal Server Error — handler panic
```
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{"error":"internal_error","details":"An unexpected error occurred"}
```
- Emitted by `recoveryMiddleware` (server.go:226-236). Handler panic does not crash the process (AC-005, CON-002).

## Example Requests

```bash
# Happy path
curl -i http://localhost:8765/api/health
# → 200, {"status":"ok","version":"1.0"}

# Query param ignored
curl -i 'http://localhost:8765/api/health?cb=123'
# → 200, {"status":"ok","version":"1.0"}

# Non-GET → 405
curl -i -X POST http://localhost:8765/api/health
# → 405

# Trailing slash → 404
curl -i http://localhost:8765/api/health/
# → 404
```

## Constraint Trace
| Constraint | How satisfied in this contract |
|---|---|
| CON-001 | Route registered as `mux.HandleFunc("GET /api/health", s.healthHandler)` |
| CON-002 | 500 response shape comes from `recoveryMiddleware`; route is on `mux` wrapped by it |
| CON-003 | `version` field sourced from `s.pipeline.Config().Version` |
| CON-006 | 200 body byte-exact `{"status":"ok","version":"1.0"}` for default config |
| CON-007 | 405 for POST/PUT/DELETE/PATCH via stdlib method-pattern |