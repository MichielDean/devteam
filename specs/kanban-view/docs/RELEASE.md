# Release & Deployment — spec `kanban-view`

## Scope

`kanban-view` is a single-repo, UI-only feature (per
`specs/kanban-view/repos.yaml`). The only affected repository is **devteam**
(the primary repo at `.`). There are no shared libraries to release first and
no consumers that depend on a new API contract — the board reuses the existing
`GET /api/features` endpoint and the existing `FeatureCard` chrome unchanged.

## Release Order

1. **devteam** — `feature/kanban-view` branch (the primary and only repo)
   - Reason: single-repo feature; no upstream shared libraries or API
     consumers to release first.
   - Breaking changes: **none**. No backend API changes, no DTO changes, no
     new endpoints, no new query parameters, no removed fields.
   - Migration required: **no**. The change is additive: a new view toggle,
     one new UI component (`KanbanBoard`), a modified Dashboard, and one new
     Playwright spec. The existing Dashboard list view, `GET /api/features`
     endpoint, and `FeatureCard` component behave exactly as before.
   - Backward compatibility: the board reads only fields the existing
     `FeatureSummary` type already defines. The new components render only
     when the user toggles to the Kanban view; the list view remains the
     default (FR-008).

No other repositories are tagged or released for this feature. This section
is documented for completeness; the cross-repo release order is trivial
because the feature is single-repo.

## Build verification

Build and deployment verification was performed by the Construction and
Testing phases from the implementation worktree on branch
`feature/kanban-view`. See `specs/kanban-view/test-report.md` for full
evidence. The Delivery phase does not re-run builds or tests.

### Frontend (UI)

From `ui/`, per `AGENTS.md` (CON-001):

```sh
npm run lint        # exit 0
npm run build       # exit 0
npm run test:e2e    # Playwright on :18765 (CON-002), all kanban specs + app.spec.ts pass
```

### Backend (Go)

No backend changes. `GET /api/features` is unchanged. The Go binary is not
rebuild-attributable to this feature.

## Configuration

### Environment variables

None introduced. The Kanban view introduces no new env vars. Existing Dev
Team backend/UI env vars (e.g. `DEVTEAM_HTTP_ADDR`, Vite dev port) are
unchanged.

### Configuration files

No new config files. `ui/playwright.config.ts` is unchanged (CON-002 — E2E
baseURL stays `:18765`).

### Dependencies

**No new npm dependencies** — runtime or dev (CON-007). The board is built
with the existing stack: React 19.1, react-router v7, `@tanstack/react-query`
v5, tailwindcss v4. `ui/package.json` has zero additions for this feature.

### Browser-side storage

- **Key**: `devteam.dashboard.view`
- **Location**: `localStorage` (browser only — not server-side, not a cookie)
- **Accepted values**: `'kanban'` → Kanban board; `'list'` or absent → list
  view (the default, FR-008)
- **Failure mode**: all `localStorage` access is wrapped in try/catch; if
  `localStorage` throws (private mode, disabled storage), the Dashboard
  falls back to the list view for the session and never crashes (FR-009,
  AC-010, AC-011).

### Database migrations

None. No schema changes, no new entities (spec Key Entities — `FeatureSummary`
and `GateResult` are reused unchanged).

## Deployment

Standard Dev Team UI deployment: build the frontend, serve the Go binary,
which serves the built UI assets and proxies `/api/*` to the backend. No
deployment topology change. The feature is additive and renders only when a
user opts into the Kanban view.

## Breaking changes & migration

None. No migration steps required.