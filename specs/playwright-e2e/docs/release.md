# Release Order — playwright-e2e

**Spec**: playwright-e2e
**Date**: 2026-06-25

## Repos affected

Per `specs/playwright-e2e/repos.yaml`, this feature spans a single repo:

| Repo | Role | Changes |
|---|---|---|
| `devteam` (module `github.com/MichielDean/devteam`) | primary | Add `GET /api/health` handler + route in `internal/api/server.go` (Go 1.22+ method-pattern ServeMux). Add `httptest` integration tests in `internal/api/server_test.go`. Add one Playwright E2E spec under `ui/e2e/`. Add a one-line `Pipeline.Config()` accessor in `internal/pipeline/pipeline.go`. No DB changes, no new dependencies. |

## Release order

Single-repo feature — no cross-repo ordering required. The "shared library
first, consumers second, frontend last" ladder collapses to one step:

1. **`devteam`** — v`Config.Version` (default `1.0`)
   - Reason: Only repo touched. Contains both the API handler and the
     Playwright E2E spec that consumes it; no external shared library or
     separate frontend repo.
   - Breaking changes: none. Additive endpoint; existing routes, handlers,
     and response shapes are unchanged.
   - Migration required: no. No DB migration, no config schema change, no
     dependency bump.
   - Tag: `v1.0` (matches `Config.Version` in `devteam.yaml`).

If the project later splits the UI into a separate frontend repo, the order
would become: (1) `devteam` API repo, (2) frontend repo that points its
health-probe consumer at the new endpoint. Not applicable today.

## Deployment verification (per delivery gate)

Documentation-only delivery phase. The Testing phase already verified, with
named evidence in `specs/playwright-e2e/test-report.md`:

- `go build ./...` succeeds.
- `go test ./internal/api/` passes all health integration tests (AC-001
  through AC-011).
- `npx playwright test` discovers and passes `ui/e2e/health.spec.ts`
  (AC-012, AC-013) against the `:18765` test server.
- The devteam binary starts and `GET /api/health` returns 200 with
  `{"status":"ok","version":"1.0"}`.

This phase does not re-run those commands. See `test-report.md` for the
proof of work.

## Configuration to verify at deploy time

- `devteam.yaml` `version` field — drives the response `version` field.
  Default `1.0`. Changing it changes the response (no rebuild needed beyond
  a restart).
- No new environment variables introduced.
- No new dependencies introduced (stdlib `net/http`, `encoding/json` only
  on the Go side; `@playwright/test` already installed in `ui/`).
- No DB migrations.

See `docs/configuration.md` for full configuration documentation.

## `.devteam/` pointer

The spec is marked delivered once the delivery gate passes. The pipeline
updates `.devteam-state.yaml`; this phase does not write that file.