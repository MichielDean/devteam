# Research: playwright-e2e (GET /api/health)

## Existing Code Patterns (verified by reading repo)

### Route registration
`internal/api/server.go:160-188` — `NewServer` constructs `http.NewServeMux()` and registers routes via Go 1.22+ method-pattern: `mux.HandleFunc("GET /api/features", s.listFeatures)`. Health route follows same pattern: `mux.HandleFunc("GET /api/health", s.healthHandler)`. Registered before the `staticFS` catch-all (`mux.Handle("/", s.spaHandler(staticFS))` at :191) so `/api/*` wins.

### Handler shape
Handlers are methods on `*Server` taking `(w http.ResponseWriter, r *http.Request)`. JSON via `writeJSON(w, code, data)` (server.go:898) which sets `Content-Type: application/json` and `json.NewEncoder(w).Encode(data)`. Errors via `writeError(w, code, errCode, details)` (server.go:904). Health handler reuses `writeJSON`.

### Middleware chain
`handler := s.recoveryMiddleware(s.corsMiddleware(mux))` (server.go:194). Recovery is OUTERMOST (catches panics → 500). Health route is registered on `mux` and thus automatically covered by both. No bypass possible — no custom wiring needed. CON-002 satisfied for free.

### Config access — KEY FINDING
`Server` struct (server.go:21-32) does NOT hold `*config.Config`. `Pipeline` holds it (`internal/pipeline/pipeline.go:26` `config *config.Config`, unexported). No `Config()` accessor exists. Two options to surface `cfg.Version` to the handler:

| Option | Churn | Verdict |
|---|---|---|
| A. Add `cfg *config.Config` param to `NewServer` | Breaks 25+ test call sites + main.go | Rejected — high churn for P3 |
| B. Add `func (p *Pipeline) Config() *config.Config` accessor (1 line) | 1 line in pipeline.go, 0 call-site changes | **CHOSEN** — laziest, Server already holds `pipeline` |

Health handler reads `s.pipeline.Config().Version`. main.go unchanged. Tests unchanged except the new health tests.

### 405 handling
Go 1.22+ `http.NewServeMux` method-pattern returns 405 automatically for methods not registered on a registered path. Registering only `GET /api/health` means POST/PUT/DELETE/PATCH → 405 automatically (with `Allow: GET, HEAD` header). No custom 405 handler needed. CON-007 satisfied by the mux for free. HEAD is auto-allowed by Go's ServeMux for GET routes (acceptable — HEAD is read-only, semantically a GET variant).

### Trailing slash
`GET /api/health/` is NOT registered → 404 (ServeMux only merges `/api/health/` into `/api/health` if a subtree `{$}` or `/api/health/` pattern is registered). Matches existing endpoints' exact-path convention. AC-011 satisfied for free.

### Test pattern
`internal/api/server_test.go` uses `httptest.NewServer(s.httpServer.Handler)` + `http.Get/Post` + `json.Decode` into `map[string]interface{}`. `setupTestServer(t)` helper builds a minimal `config.Config` with 6 phases, `spec.NewSpecProvider(tmpDir)`, `pipeline.NewPipelineWithDispatcher(cfg, sp, nil)`, `NewServer(":0", ...)`. Health tests follow same pattern. NOTE: `setupTestServer` builds `cfg` with NO `Version` set → `cfg.Version == ""`. AC-001 (default config `"1.0"`) needs a health-specific setup that sets `cfg.Version = "1.0"`, OR the test constructs its own cfg. AC-003 sets `cfg.Version = "9.9.9-test"`.

### Playwright pattern
`ui/playwright.config.ts` — `testDir: './e2e'`, `baseURL: http://localhost:18765`, `webServer` auto-starts `~/go/bin/devteam -http :18765` from repo root (port 18765, NOT prod 8765). Existing specs in `ui/e2e/app.spec.ts` use `page.goto('/')`, `page.locator(...)`, `expect(...).toContainText(...)`. Health spec uses `page.request.get(baseURL + '/api/health')` (Playwright's built-in request context) — no browser navigation needed, just an HTTP probe. `npx playwright test` discovers `*.spec.ts` under `ui/e2e/` automatically. CON-005 satisfied by pointing at baseURL (which is :18765).

## Library/framework choices
- Go stdlib `net/http` only — no new deps. Matches existing handlers.
- Playwright `page.request` API — already installed (`@playwright/test` in `ui/package.json`). No new dep.

## Alternatives considered & rejected
- **Custom 405 handler**: rejected — Go 1.22+ ServeMux method-pattern emits 405 natively. Custom code would be reinventing the stdlib.
- **Subtree pattern `GET /api/health/`** to merge trailing slash: rejected — spec assumes 404 on trailing slash (matches existing exact-path convention).
- **Adding `cfg` param to `NewServer`**: rejected — 25+ call-site breakage for a P3 feature. `Pipeline.Config()` accessor is 1 line.
- **Hardcoding `"1.0"` in handler**: rejected — violates CON-003 / AC-003. Sourcing from config is the conservative choice.
- **DB/dependency readiness check**: rejected — out of scope (liveness only, per spec assumption).

## Performance characteristics
Handler is ~5 lines: struct alloc + 1 JSON encode. Sub-microsecond. No DB, no I/O. No perf goals in spec. No concern.

## Spikes
None needed. All patterns verified by reading existing code.