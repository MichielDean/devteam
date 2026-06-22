# Release & Deployment — Spec kanban-view

## Scope

`kanban-view` is a single-repo, UI-only feature (per `specs/kanban-view/repos.yaml`).
The only affected repository is **devteam** (`https://github.com/MichielDean/devteam`).
There are no shared libraries to release first and no consumers that depend on a
new API contract — the board reuses the existing `GET /api/features` endpoint and
existing `FeatureCard` component unchanged.

## Release order

1. **devteam** — `feature/kanban-view` branch (the primary and only repo)
   - Reason: single-repo feature; no upstream dependencies to release first.
   - Breaking changes: **none**. No backend API changes, no DTO changes, no new
     endpoints, no new query parameters, no removed fields.
   - Migration required: **no**. The change is additive: a new view toggle and
     new UI components. The existing Dashboard list view, `GET /api/features`
     endpoint, and `FeatureCard` component behave exactly as before.
   - Backward compatibility: the board reads only fields the existing
     `FeatureSummary` type already defines. Older clients that never render the
     board are unaffected; the new components are lazily rendered only when the
     user toggles to the Board view.

No other repositories are tagged or released for this feature.

## Build verification

Verified from the implementation worktree
`~/source/devteam/worktrees/kanban-view/devteam` on branch `feature/kanban-view`
at commit `bd10d23`:

### Backend (Go)

```sh
go build -o ~/go/bin/devteam ./cmd/devteam/        # exit 0, binary 11.1M
go test ./... -count=1 -timeout 120s               # 256 passed in 13 packages, exit 0
```

All packages pass: `internal/api`, `internal/config`, `internal/feature`,
`internal/init`, `internal/intake`, `internal/pipeline`, `internal/repo`,
`internal/role`, `internal/rules`, `internal/spec`. No test files (and none
required) in `cmd/devteam`, `internal/gitops`, `internal/plugins`.

### Frontend (UI)

```sh
cd ui && npm run build                             # tsc -b + vite build, exit 0
# 476 modules transformed, dist/index.html + assets emitted
```

No TypeScript errors. No new dependencies installed — `git diff main --
ui/package.json` shows **no additions** in `dependencies` or `devDependencies`
(verified: the diff is empty for `package.json`), satisfying CON-006 / FR-011 /
AC-CON-006.

### E2E / integration (Playwright)

```sh
./run-tests.sh ui                                  # exit 0
# 29 passed, 3 skipped, 0 failed (10.2s)
```

The 3 skipped tests are pre-existing skips in `app.spec.ts`, unrelated to this
feature. All kanban-view acceptance criteria tests pass:
AC-001, AC-002, AC-003, AC-004, AC-005, AC-006, AC-007, AC-008, AC-009, AC-010,
AC-011, AC-013, AC-014, AC-CON-008, AC-CON-011, AC-ERR-003 (e2e); AC-CON-003,
AC-CON-006, AC-ERR-001, AC-ERR-002 (integration). The unit-labeled AC-012 and
AC-CON-005 are covered at integration/e2e level per plan AD-6 (no `vitest`
devDependency was added, by conservative resolution of the CON-006 tension).

## Deployment verification

The binary built from `feature/kanban-view` was started from the implementation
worktree and verified end-to-end against a live browser session:

1. **Build**: `go build -o ~/go/bin/devteam ./cmd/devteam/` succeeded (exit 0).
2. **Start**: `~/go/bin/devteam -http :18765` started from the worktree without
   panicking. `GET /api/features` returned HTTP 200.
3. **UI renders**: loaded `http://localhost:18765/` — the Dashboard list view
   rendered, the **List / Board** toggle was present, and the total feature
   count badge read `5`.
4. **Board renders**: clicked the **Board** toggle. All 7 columns rendered in
   canonical order (Backlog, Inception, Planning, Construction, Review, Testing,
   Delivery) with correct per-column counts and empty-state messages. The count
   badge stayed at `5` across the toggle.
5. **Card navigation**: clicked a card in the Delivery column — the browser
   navigated to `/features/002-dev-team-web-ui` and the Feature Detail page
   rendered.
6. **List toggle back**: clicked **List** — the list view returned, the board
   disappeared, the count badge stayed at `5`.
7. **Dark mode**: toggled dark mode via the existing theme switch, then switched
   to the Board view. `document.documentElement.className` became `dark`; the
   Delivery column's computed background color was `oklch(0.21 0.034 264.665)`,
   i.e. the dark palette, not the light one.
8. **Console errors**: zero `pageerror` events and zero console error messages
   across the entire session (verified via Playwright `page.on('console')`
   capture).

The production `devteam-web.service` systemd unit (port `:8765`) was restarted
against the rebuilt binary; `GET /api/features` returned HTTP 200.

## Configuration

No new configuration is introduced by this feature.

### Environment variables

None added. The board uses only existing configuration:

| Variable | Used by | Purpose | New? |
|----------|---------|---------|------|
| `SERVER_PORT` | `ui/playwright.config.ts`, `run-tests.sh` | Isolates the Playwright test server from the production `:8765` service. Defaults to `8765` if unset. | Existing (the feature only adds the ability to override the port via the env var in `run-tests.sh`; the playwright config already read it). |
| `BASE_URL` | `ui/playwright.config.ts` | Playwright `baseURL`. Defaults to `http://localhost:8765`. | Existing. |
| `START_SERVER` | `ui/playwright.config.ts` | Set to `1` to force Playwright to start a fresh server instead of reusing a running one. | Existing. |
| `SERVER_BINARY` | `ui/playwright.config.ts` | Override the web server command used by Playwright. | Existing. |

### Configuration files

- `devteam.yaml` (repo root): unchanged by this feature. The board does not read
  any new config keys.
- `ui/playwright.config.ts`: the `webServer.command` now derives its `-http`
  flag from `SERVER_PORT` so test runs can isolate on a non-production port. This
  is a test-only change; production deployment is unaffected.

### Dependencies

- **Go** (`go.mod`): unchanged. No new Go modules added.
- **UI** (`ui/package.json`): unchanged — no new runtime or dev dependencies.
  Verified by `git diff main -- ui/package.json` showing no additions in
  `dependencies` or `devDependencies` (CON-006 / FR-011 / AC-CON-006).

### Database migrations

None. The feature introduces no new persistent entities. The board is a derived
view over the existing `FeatureSummary` data returned by `GET /api/features`.

## Terminology consistency check

Documentation terminology was checked against `specs/kanban-view/spec.md` and
`specs/kanban-view/acceptance.md`:

| Term in spec | Used in docs? | Notes |
|--------------|---------------|-------|
| Kanban board | yes | spec.md uses "Kanban board" / "Kanban view" interchangeably; docs use both consistently |
| Backlog column | yes | CON-002 / FR-002 |
| Inception, Planning, Construction, Review, Testing, Delivery | yes | the 6 pipeline phases, in canonical order (CON-001) |
| `current_phase` | yes | wire field name, matched verbatim |
| `status` | yes | wire field name, matched verbatim; `draft` is the Backlog-triggering status |
| `FeatureCard` | yes | the reused component, named as in the codebase and spec CON-005 |
| `FeatureSummary` | yes | the card entity, named as in `ui/src/types/index.ts` and spec |
| `feature-count-badge` | yes | the `data-testid` from the prior count-badge feature, referenced by CON-010 |
| `data-testid` selectors (`kanban-board`, `kanban-column-{key}`, `view-toggle-list`, `view-toggle-board`) | yes | matched verbatim from FR-012 / CON-011 |
| `listFeatures` / `GET /api/features` | yes | the sole data source (CON-003 / FR-004) |
| `['features']` react-query cache key | yes | the shared cache key (FR-014) |
| Terminal statuses (`done`, `cancelled`) stay visible | yes | CON-009 / FR-003 |
| Empty array `[]` not `null` | yes | CON-004 |

No code-internal names (e.g. `viewMode`, `COLUMN_KEYS`, `groupFeaturesByColumn`)
are used in user-facing documentation except where explicitly identified as
implementation details in the API reference. User-facing docs use spec
terminology throughout.

## Quality gate checklist

- [x] Documentation exists for every user story (US-001 through US-006)
- [x] Documentation uses spec terminology, not code-internal names
- [x] Cross-repo release order documented (single-repo feature — only `devteam`,
      no shared libraries, no consumers, no breaking changes, no migration)
- [x] Release notes reference the spec number (`kanban-view` in every changelog
      entry)
- [x] Affected repo builds successfully (Go `go build` + `go test ./...`; UI
      `npm run build`)
- [x] The service starts and responds to HTTP requests (`GET /api/features`
      → 200)
- [x] The frontend loads without console errors (verified via Playwright console
      capture: 0 errors)
- [x] All smoke tests from the testing phase still pass (29 passed, 3 skipped
      pre-existing, 0 failed)
- [x] Configuration is documented (no new config added; existing env vars and
      config files enumerated)
- [x] Breaking changes (if any) are documented with migration steps — **none**,
      no migration needed