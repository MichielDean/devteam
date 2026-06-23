# Release & Deployment — spec kanban-view

## Scope

`kanban-view` is a single-repo, UI-only feature (per
`specs/kanban-view/repos.yaml`). The only affected repository is **devteam**
(`https://github.com/MichielDean/devteam`). There are no shared libraries to
release first and no consumers that depend on a new API contract — the board
reuses the existing `GET /api/features` endpoint and the existing
`FeatureCard` badge chrome unchanged.

## Release order

1. **devteam** — `feature/kanban-view` branch (the primary and only repo)
   - Reason: single-repo feature; no upstream dependencies to release first.
   - Breaking changes: **none**. No backend API changes, no DTO changes, no
     new endpoints, no new query parameters, no removed fields.
   - Migration required: **no**. The change is additive: a new view toggle,
     new UI components, and one devDependency (`vitest`). The existing
     Dashboard list view, `GET /api/features` endpoint, and `FeatureCard`
     component behave exactly as before.
   - Backward compatibility: the board reads only fields the existing
     `FeatureSummary` type already defines. Older clients that never render
     the board are unaffected; the new components render only when the user
     toggles to the Board view.

No other repositories are tagged or released for this feature.

## Build verification

Verified by the Construction and Testing phases from the implementation
worktree on branch `feature/kanban-view` (see `specs/kanban-view/test-report.md`
for full evidence).

### Backend (Go)

```sh
go build -o ~/go/bin/devteam ./cmd/devteam/        # exit 0
go test ./internal/api/... -run TestKanban -count=1 -v   # 7 passed, 0 failed
```

The Go smoke test `internal/api/kanban_smoke_test.go` asserts:
- `GET /api/features` on an empty system returns `"features":[]` not
  `"features":null` (CON-008, the #1 agent-generated serialization bug).
- A `done` feature in `delivery` is returned with `current_phase=delivery`
  intact (not filtered).
- `/api/kanban`, `/api/kanban/features`, `/api/board`, `/api/features/kanban`
  all return 4xx — no new route added (CON-007, AC-016).

### Frontend (UI)

```sh
cd ui && npm run build                             # tsc -b + vite build, exit 0
```

No TypeScript errors. `package.json` `dependencies` block unchanged (CON-003);
`devDependencies` adds `vitest` only.

### Unit tests

```sh
cd ui && npm run test:unit                         # 4 passed (vitest)
```

`ui/src/components/groupFeaturesByPhase.test.ts` covers AC-011, CON-008
(empty buckets `[]` never `null`), CON-009 (unknown phase → `other`), and
SC-002 (partition invariant).

### E2E / integration (Playwright)

```sh
SERVER_PORT=18765 START_SERVER=1 BASE_URL=http://localhost:18765 npm run test:e2e
# 31 passed, 3 skipped, 0 failed
```

Per CON-001, e2e runs on `:18765`, never `:8765`. The 3 skipped tests are
pre-existing workspace-state-dependent skips in `app.spec.ts`; no failure is
hidden behind a skip. All 22 kanban-view acceptance criteria tests pass
(AC-001 through AC-022).

## Deployment verification

The Testing phase verified the binary from `feature/kanban-view` end-to-end
against a live browser session (see `test-report.md` §3-5):

1. **Build**: `go build` succeeded.
2. **Start**: the `devteam` binary started on `:18765` without panicking;
   `GET /api/features` returned HTTP 200.
3. **UI renders**: the Dashboard loaded; the List/Board toggle was present;
   the feature-count badge rendered.
4. **Board renders**: clicking Board rendered six phase columns in canonical
   order with correct per-column counts and "No features" placeholders for
   empty columns.
5. **Card navigation**: clicking a card navigated to `/features/{id}`.
6. **Toggle back**: clicking List returned the list view; the board
   disappeared.
7. **Console errors**: zero `pageerror` events and zero console error
   messages across the session (verified via Playwright `page.on('console')`
   capture).

## Configuration

No new production configuration is introduced by this feature.

### Environment variables

None added. The board uses only existing configuration:

| Variable | Used by | Purpose | New? |
|----------|---------|---------|------|
| `SERVER_PORT` | `ui/playwright.config.ts`, `run-tests.sh` | Isolates the Playwright test server from the production `:8765` service. Per CON-001 the spec mandates `:18765` for e2e. | Existing. |
| `BASE_URL` | `ui/playwright.config.ts` | Playwright `baseURL`. Defaults to `http://localhost:8765`. | Existing. |
| `START_SERVER` | `ui/playwright.config.ts` | Set to `1` to force Playwright to start a fresh server instead of reusing a running one. | Existing. |
| `SERVER_BINARY` | `ui/playwright.config.ts` | Override the web server command used by Playwright. | Existing. |

### Configuration files

- `devteam.yaml` (repo root): unchanged by this feature. The board does not
  read any new config keys.
- `ui/playwright.config.ts`: test-only; `webServer.command` honors
  `SERVER_PORT` so test runs isolate on `:18765`. Production deployment is
  unaffected.
- `ui/vite.config.ts`: gains a vitest `test` block (excludes `e2e/**`).
  Build-only; production deployment is unaffected.

### Dependencies

- **Go** (`go.mod`): unchanged. No new Go modules added.
- **UI** (`ui/package.json`): `dependencies` block unchanged (CON-003).
  `devDependencies` adds `vitest` only (for the AC-011 unit test). No new
  runtime npm dependency (CON-003, VIII "Go, Minimal Dependencies").

### Database migrations

None. The feature introduces no new persistent entities. The board is a
derived view over the existing `FeatureSummary` data returned by
`GET /api/features`.

## Terminology consistency check

Documentation terminology was checked against `specs/kanban-view/spec.md` and
`specs/kanban-view/acceptance.md`:

| Term in spec | Used in docs? | Notes |
|--------------|---------------|-------|
| Kanban board / Kanban view | yes | spec.md uses both; docs use both consistently |
| Inception, Planning, Construction, Review, Testing, Delivery | yes | the six pipeline phases, in canonical order (FR-005, CON-005) |
| "Other" column | yes | defensive trailing column for unknown `current_phase` (FR-007, AC-011) |
| `current_phase` | yes | wire field name, matched verbatim |
| `status` | yes | wire field name, matched verbatim |
| `priority` | yes | wire field name, matched verbatim |
| `pending_questions_count` | yes | wire field name, matched verbatim |
| `gate_result` | yes | wire field name, matched verbatim |
| `FeatureCard` | yes | the reused component, named as in the codebase and spec CON-006 |
| `FeatureSummary` | yes | the card entity, named as in `ui/src/types/index.ts` and spec |
| `feature-count-badge` | yes | the `data-testid` from the prior count-badge feature |
| `data-testid` selectors (`view-toggle`, `view-toggle-list`, `view-toggle-board`, `kanban-column-{phase}`, `kanban-card-{id}`, `kanban-card-status`, `kanban-card-priority`, `kanban-card-gate`, `question-badge`, `features-loading`, `features-error`) | yes | matched verbatim from FR / AC |
| `listFeatures` / `GET /api/features` | yes | the sole data source (FR-016, CON-007, AC-016) |
| `['features']` react-query cache key | yes | the shared cache key (FR-016) |
| `sessionStorage` key `devteam.dashboard.view` | yes | FR-002 |
| Terminal statuses (`done`, `cancelled`) stay visible | yes | spec Assumptions |
| Empty array `[]` not `null` | yes | CON-008 |
| `PHASES` / `PHASE_LABELS` / `STATUS_LABELS` / `PRIORITY_LABELS` | yes | CON-005 |
| `badgeColors` / `statusColors` | yes | CON-006 shared module |
| `groupFeaturesByPhase` | yes | the pure grouping function (AC-011) |

No code-internal names are used in user-facing documentation except where
explicitly identified as implementation details in the API reference.
User-facing docs use spec terminology throughout.

## Quality gate checklist

- [x] Documentation exists for every user story (US-001 through US-004)
- [x] Documentation uses spec terminology, not code-internal names
- [x] Cross-repo release order documented (single-repo feature — only
      `devteam`, no shared libraries, no consumers, no breaking changes, no
      migration)
- [x] Release notes reference the spec (`kanban-view` in every changelog
      entry)
- [x] Affected repo builds successfully (Go `go build` + Go smoke tests; UI
      `npm run build` + `npm run test:unit`)
- [x] The service starts and responds to HTTP requests (`GET /api/features`
      → 200)
- [x] The frontend loads without console errors (verified via Playwright
      console capture: 0 errors)
- [x] All e2e tests from the testing phase pass (31 passed, 3 skipped
      pre-existing, 0 failed)
- [x] Configuration is documented (no new production config; existing env
      vars and config files enumerated)
- [x] Breaking changes (if any) documented with migration steps — **none**,
      no migration needed