# Tasks: Kanban View

**Feature ID**: kanban-view
**Phase**: Planning → Construction
**Repo**: `devteam` (UI only — `ui/` directory)
**Branch**: `kanban-view`

Tasks are ordered by dependency. T-001 is foundational (types + grouping). T-002 and T-003 build on it. T-004 wires navigation. T-005 adds E2E tests. Linear chain; no parallelism except T-002 and the ViewToggle half of T-004 could overlap once T-001 lands, but the dependency is strict enough that sequential is simpler.

---

## Task T-001 — Add column types and grouping function

**Priority**: P1
**User stories**: US-001, US-002, US-005 (foundation for all)
**Constraints**: CON-001, CON-002, CON-009, CON-004
**Files**:
- `ui/src/types/index.ts` — MODIFY: add `ColumnKey` type and `KANBAN_COLUMNS` constant
- `ui/src/components/groupByColumn.ts` — CREATE: pure grouping function + `columnForFeature`

**Dependencies**: none (first task)

**Done conditions**:
- `ui/src/types/index.ts` exports:
  - `type ColumnKey = 'backlog' | 'inception' | 'planning' | 'construction' | 'review' | 'testing' | 'delivery'`
  - `const KANBAN_COLUMNS: ColumnKey[] = ['backlog','inception','planning','construction','review','testing','delivery']`
  - `const KANBAN_COLUMN_LABELS: Record<ColumnKey, string>` with `backlog: 'Backlog'` and the rest mirroring `PHASE_LABELS`
- `ui/src/components/groupByColumn.ts` exports:
  - `function columnForFeature(f: FeatureSummary): ColumnKey | null` — returns `'backlog'` iff `f.status === 'draft' && f.current_phase === 'inception'`; returns `f.current_phase as ColumnKey` if it's in `KANBAN_COLUMNS`; returns `null` for unknown phases (dropped + warned by caller)
  - `function groupByColumn(features: FeatureSummary[]): Record<ColumnKey, FeatureSummary[]>` — initializes all 7 keys to `[]`, then iterates features calling `columnForFeature`; for `null` result, `console.warn(`[kanban] feature ${f.id} has unknown current_phase "${f.current_phase}", dropped from board`)` and skips
- Verify:
  - `cd ui && npm run build` succeeds (tsc + vite)
  - `groupByColumn([])` returns an object with exactly the 7 `KANBAN_COLUMNS` keys, each `[]` (mentally trace or add a temporary `console.log` in a scratch file — do NOT commit scratch)
  - `groupByColumn([{id:'x',status:'draft',current_phase:'inception',...}])` puts `x` in `backlog` and leaves the rest `[]`
  - `groupByColumn([{id:'y',status:'done',current_phase:'delivery',...}])` puts `y` in `delivery` (terminal status NOT special-cased)
  - `groupByColumn([{id:'z',status:'in_progress',current_phase:'frobnicate',...}])` returns all columns `[]` (dropped)

**Test level**: unit (logic) — covered by E2E in T-005; no unit runner added (see plan Open Questions)

**Agent failure mode checks**:
- [x] JSON arrays are `[]` not null: `groupByColumn` initializes every column to `[]` before any assignment — empty input yields 7 empty arrays, never `undefined`/`null`
- [x] Parsing safety: unknown `current_phase` caught and returned as `null`, caller logs + skips. Never throws.
- [x] No nil-pointer: all field access on `FeatureSummary` is on a typed param; no optional chaining needed inside the pure function

---

## Task T-002 — Create `KanbanColumn` component

**Priority**: P1
**User stories**: US-001 (column rendering), US-005 (per-column empty state)
**Constraints**: CON-001, CON-004, CON-005, CON-008, CON-011
**Files**:
- `ui/src/components/KanbanColumn.tsx` — CREATE

**Dependencies**: T-001 must complete first (needs `ColumnKey`, `KANBAN_COLUMN_LABELS`)

**Done conditions**:
- `KanbanColumn` exports a component with props:
  ```
  interface KanbanColumnProps {
    columnKey: ColumnKey;
    features: FeatureSummary[];
  }
  ```
- Renders:
  - Root `<section data-testid={`kanban-column-${columnKey}`} className="... bg-white dark:bg-gray-800 ...">`
  - Header `<h3>` with the label from `KANBAN_COLUMN_LABELS[columnKey]` and a count `<span>` equal to `features.length`
  - When `features.length > 0`: a `<div>` list mapping each feature to `<FeatureCard feature={f} />` (imports `FeatureCard` from `./FeatureCard`)
  - When `features.length === 0`: `<div data-testid={`kanban-column-empty-${columnKey}`} className="... text-gray-500 dark:text-gray-400 ...">` with text:
    - `backlog` → "No features waiting to start"
    - all other columns → "No features in this phase"
- Verify:
  - `cd ui && npm run build` succeeds
  - `cd ui && npm run lint` succeeds
  - Component is importable from `KanbanBoard` (T-003 will import it)

**Test level**: e2e (verified via T-005 board tests: AC-003, AC-013, AC-CON-011)

**Agent failure mode checks**:
- [x] No null-array map: `features` prop defaults to `[]` if undefined via destructuring default (`features = []`) — defensive even though caller always passes an array
- [x] Dark mode: root and empty-state both have `dark:` variants
- [x] Reuse `FeatureCard` — import statement present, no re-implementation of card markup (AC-CON-005)
- [x] `data-testid` stable: suffix is the `columnKey` directly from the prop, no transformation

---

## Task T-003 — Create `KanbanBoard` page

**Priority**: P1
**User stories**: US-001, US-002, US-004, US-005, US-006
**Constraints**: CON-001, CON-002, CON-003, CON-004, CON-005, CON-008, CON-009, CON-010, CON-011
**Files**:
- `ui/src/pages/KanbanBoard.tsx` — CREATE

**Dependencies**: T-001 and T-002 must complete first

**Done conditions**:
- `KanbanBoard` exports a default React page component
- Uses `useQuery({ queryKey: ['features'], queryFn: listFeatures })` — same key as Dashboard (AD-001)
- Reads `features = data?.features ?? []` and `totalCount = data?.total_count ?? 0` (defensive coalescing — CON-004)
- Calls `groupByColumn(features)` to get the 7-column map
- Renders:
  - Page header: `<h2>Board</h2>` + `feature-count-badge` `<span data-testid="feature-count-badge">{totalCount}</span>` (same testid as Dashboard — CON-010) + `<ViewToggle />` (from T-004; until T-004 lands, render a placeholder `<div data-testid="view-toggle-placeholder" />` and replace in T-004)
  - Loading state: `<div data-testid="kanban-loading">Loading board...</div>` with spinner matching Dashboard pattern (`animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600`)
  - Error state: `<div data-testid="kanban-error" className="text-red-600 dark:text-red-400 py-4">Failed to load features: {error.message}</div>` + columns still render (empty) below the banner
  - Board root: `<div data-testid="kanban-board" className="flex gap-4 overflow-x-auto pb-4">` containing one `<KanbanColumn>` per `KANBAN_COLUMNS` entry, in order, each with `min-w-[280px]` and `flex-1` for the active columns
- Verify:
  - `cd ui && npm run build` succeeds
  - `cd ui && npm run lint` succeeds
  - Source contains `import { listFeatures } from '../api/client'` and `import FeatureCard` is NOT directly present in `KanbanBoard.tsx` (cards come through `KanbanColumn` — if the developer adds a direct `FeatureCard` import here, that's fine too, but the intended path is via `KanbanColumn`)
  - Source contains `useQuery({ queryKey: ['features']` (character-for-character the same key as `Dashboard.tsx` line 18)

**Test level**: e2e (AC-001, AC-002, AC-006, AC-009, AC-011, AC-CON-011, AC-ERR-001)

**Agent failure mode checks**:
- [x] JSON arrays `[]` not null: `data?.features ?? []` + `groupByColumn` initializes all columns to `[]`
- [x] Parsing safety: `groupByColumn` handles unknown phases (logs + drops); board never throws
- [x] No nil deref: `data?.features`, `data?.total_count`, `error?.message` all use optional chaining
- [x] Cache key consistency: `['features']` exactly matches Dashboard — react-query dedupes; no second fetch (CON-010, FR-014)
- [x] Error path: `error` branch renders banner + empty columns, never a blank page or thrown exception

---

## Task T-004 — Wire navigation: route, `ViewToggle`, Dashboard header

**Priority**: P1
**User stories**: US-003
**Constraints**: CON-007, CON-010
**Files**:
- `ui/src/components/ViewToggle.tsx` — CREATE
- `ui/src/App.tsx` — MODIFY: add `<Route path="/kanban" element={<KanbanBoard />} />`
- `ui/src/pages/Dashboard.tsx` — MODIFY: add `<ViewToggle />` in the header next to the `h2`/badge
- `ui/src/pages/KanbanBoard.tsx` — MODIFY: replace the `view-toggle-placeholder` from T-003 with `<ViewToggle />`

**Dependencies**: T-003 must complete first (so the `/kanban` route has a page to render)

**Done conditions**:
- `ViewToggle` exports a component rendering a segmented control:
  ```
  <nav data-testid="view-toggle" className="inline-flex rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
    <Link to="/" data-testid="view-toggle-list" className="...">List</Link>
    <Link to="/kanban" data-testid="view-toggle-board" className="...">Board</Link>
  </nav>
  ```
  - Uses `react-router`'s `Link` (already a dep)
  - Active link can be styled via `useLocation()` — optional, not required by AC. If implemented, active link gets `bg-blue-600 text-white`; inactive gets `text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700`.
- `App.tsx` adds the `/kanban` route *after* the existing `/` and `/features/:id` routes. Existing routes unchanged.
- `Dashboard.tsx` adds `<ViewToggle />` in the header `<div className="flex items-center justify-between mb-6">` — place it inside the left `<div>` after the badge, or in a new middle position. The `feature-count-badge`, `create-feature-button`, and existing layout must remain intact (AC-009 depends on the badge still rendering on the Dashboard).
- `KanbanBoard.tsx` replaces the placeholder with `<ViewToggle />`.
- Verify:
  - `cd ui && npm run build` succeeds
  - `cd ui && npm run lint` succeeds
  - Manual: `cd ui && npm run dev`, open `http://localhost:5173/`, click "Board" → board renders; click "List" → dashboard renders; badge count same on both
  - `git diff main -- internal/api/server.go` is empty (no backend route added — CON-003)

**Test level**: e2e (AC-007, AC-008, AC-009)

**Agent failure mode checks**:
- [x] No new dependency: `ViewToggle` uses only `react-router` `Link` + `useLocation` (already in `dependencies`)
- [x] No backend change: route is client-side only (`react-router` `<Route>`)
- [x] Dashboard regression: existing `feature-count-badge`, `create-feature-button`, `features-loading`, `features-error`, `empty-state` testids all still present (grep `Dashboard.tsx` after edit)
- [x] No nil pointer: `useLocation()` always returns a value in a `<BrowserRouter>` context

---

## Task T-005 — E2E test suite for Kanban

**Priority**: P1
**User stories**: US-001..US-006 (all)
**Constraints**: all (CON-001..CON-011) — each CON has at least one AC exercised here
**Files**:
- `ui/e2e/kanban.spec.ts` — CREATE
- `ui/e2e/app.spec.ts` — unchanged (verify no regression by running it)

**Dependencies**: T-001..T-004 must all complete first

**Done conditions** — `ui/e2e/kanban.spec.ts` contains `test()` blocks covering every AC below, all passing when run against a live backend with seeded features:

- **AC-001** (features in correct columns): seed features in inception/planning/delivery via `POST /api/features` + `POST /api/features/{id}/run`/`/advance` (or use existing workspace features if present); load `/kanban`; for each seeded feature assert `[data-testid="feature-card-{id}"]` is inside `[data-testid="kanban-column-{current_phase}"]`.
- **AC-002** (column order): query `[data-testid^="kanban-column-"]` children of `[data-testid="kanban-board"]`; assert the ordered suffix list equals `['backlog','inception','planning','construction','review','testing','delivery']`.
- **AC-003** (header count): for each `kanban-column-*`, assert header text contains the label and a count integer equal to descendant `feature-card-*` count.
- **AC-004** (draft+inception → backlog): create a feature, do NOT run any phase; load `/kanban`; assert card in `kanban-column-backlog`, NOT in `kanban-column-inception`.
- **AC-005** (in_progress+inception → inception column): create a feature, `POST /api/features/{id}/run`; wait for status `in_progress`; load `/kanban`; assert card in `kanban-column-inception`, NOT in `kanban-column-backlog`.
- **AC-006** (done in delivery): find or seed a `done` feature in `delivery`; load `/kanban`; assert card in `kanban-column-delivery`.
- **AC-007** (Dashboard → Board): load `/`; assert `feature-list` visible; click `view-toggle-board`; assert `kanban-board` visible, `feature-list` not visible.
- **AC-008** (Board → Dashboard): from `/kanban`; click `view-toggle-list`; assert `feature-list` visible, `kanban-board` not visible.
- **AC-009** (badge consistent): load `/`; read `feature-count-badge` text → N; navigate to `/kanban`; assert `feature-count-badge` text is N.
- **AC-010** (card click → detail): seed a feature; load `/kanban`; click `feature-card-{id}`; assert URL path is `/features/{id}` and FeatureDetail renders (`h1` visible).
- **AC-011** (empty board): route-intercept `**/api/features` → `{features:[],total_count:0}`; load `/kanban`; assert all 7 `kanban-column-*` exist, each has 0 `feature-card-*` descendants and a visible `kanban-column-empty-*` child; capture `page.on('console')` errors → assert 0 error-type entries; capture `page.on('pageerror')` → assert 0.
- **AC-013** (partial board): seed 5 features all advanced to `planning`; load `/kanban`; assert `kanban-column-planning` has 5 cards; every other column has 0 cards + visible empty-state.
- **AC-014** (live update): seed a feature in `inception`; load `/kanban`; assert card in `kanban-column-inception`; trigger advance to `planning` via the existing flow (e.g. `POST /api/features/{id}/advance` after gate passes, or `POST /api/features/{id}/process`); wait for `['features']` refetch; assert card now in `kanban-column-planning` and URL unchanged.
- **AC-CON-008** (dark mode): toggle `theme-toggle-button`; load `/kanban`; assert `kanban-board` computed `background-color` matches dark palette (e.g. `rgb(17, 24, 39)` for `bg-gray-900` or `rgb(31, 41, 55)` for `bg-gray-800` — check which class the board root uses and assert the matching rgb).
- **AC-CON-011** (testids exist exactly once): load `/kanban`; for each testid in `['kanban-board','kanban-column-backlog','kanban-column-inception','kanban-column-planning','kanban-column-construction','kanban-column-review','kanban-column-testing','kanban-column-delivery']`, assert `locator(`[data-testid="${t}"]`)` has count 1.
- **AC-ERR-001** (API 500 on load): route-intercept `**/api/features` → 500 `{error:'internal_error',details:'db down'}`; load `/kanban`; assert `kanban-error` visible with text "Failed to load features"; assert `page.on('pageerror')` count 0.
- **AC-ERR-002** (refetch error): load `/kanban` successfully; intercept next `**/api/features` with 500; trigger refetch (invalidate via a mutation — e.g. create a feature through the existing flow, or call `window.__queryClient.invalidateQueries` if exposed — simplest: just reload the page after setting the intercept); assert no `pageerror`; assert `kanban-error` visible.
- **AC-ERR-003** (deleted-while-viewing): seed a feature; load `/kanban`; delete the feature's spec dir via filesystem (or a direct API if available — check `internal/api/server.go` for a delete route; if none, remove `specs/{id}/` via `fs.rmSync` in the test's `beforeAll`); click the card; assert FeatureDetail renders its 404/not-found state; assert no `pageerror`.
- **Unknown phase (negative case from plan)**: route-intercept `**/api/features` → `{features:[{id:'x',title:'X',status:'in_progress',current_phase:'frobnicate',priority:1,updated_at:new Date().toISOString(),gate_result:null,pending_questions_count:0}],total_count:1}`; load `/kanban`; assert no `feature-card-x` in any column; assert `page.on('console')` captured a `warn`-type message containing "frobnicate"; assert `page.on('pageerror')` count 0.

**Integration (non-Playwright) checks** — run as a separate `test.describe` or as shell commands in the test's `beforeAll`/CI:
- **AC-CON-003**: `git diff main -- internal/api/server.go ui/src/api/client.ts` shows no new `mux.HandleFunc` and no new exported function in `client.ts`. (Run as a CI step or assert via `child_process.execSync('git diff main -- ...')` in a Node test; if neither is available, document as a manual review checkbox in the test file header comment.)
- **AC-CON-006**: `git diff main -- ui/package.json` shows no additions in `dependencies`/`devDependencies`. (Same approach.)

**Verify (after writing the suite)**:
- `cd ui && npx playwright test e2e/kanban.spec.ts` — all tests green (with a running backend)
- `cd ui && npx playwright test` (full suite) — `app.spec.ts` still green, no regression
- `cd ui && npm run build` succeeds
- `cd ui && npm run lint` succeeds
- `git diff main -- ui/package.json` is empty (or only lockfile churn)
- `git diff main -- internal/ cmd/ go.mod go.sum` is empty

**Test level**: e2e (primary) + integration (diff checks)

**Agent failure mode checks**:
- [x] Tests verify real system: Playwright hits `http://localhost:8765` (or the playwright `webServer`); no mocked component rendering
- [x] Console error capture: every e2e test that loads a page registers `page.on('console')` + `page.on('pageerror')` and asserts zero errors unless the test is explicitly testing an error path (in which case it asserts the error banner is visible AND no `pageerror`)
- [x] No `test.skip` without justification: AC-006 may skip if no `done` feature exists in the workspace and seeding one end-to-end is impractical — if skipped, document why in a comment
- [x] Route interception restored: every test that calls `page.route()` should use `afterEach` to clear routes (Playwright does this per-page automatically, but be explicit)

---

## Dependency graph

```
T-001 (types + grouping)
  └─ T-002 (KanbanColumn)
       └─ T-003 (KanbanBoard page)
            └─ T-004 (ViewToggle + routes + Dashboard mod)
                 └─ T-005 (E2E suite)
```

Linear. No parallelism. Total: 5 tasks, all P1.

## Coverage cross-check (constraint → task)

| CON-ID | Task(s) |
|--------|---------|
| CON-001 | T-001 (constant), T-002 (column render order via board map), T-005 (AC-002) |
| CON-002 | T-001 (columnForFeature), T-005 (AC-004, AC-005) |
| CON-003 | T-003 (reuses listFeatures), T-005 (AC-CON-003 diff check) |
| CON-004 | T-001 (groupByColumn init), T-003 (defensive `?? []`), T-005 (AC-011, AC-012 via AC-011) |
| CON-005 | T-002 (imports FeatureCard), T-005 (AC-CON-005 source inspection) |
| CON-006 | T-004 (ViewToggle uses only existing deps), T-005 (AC-CON-006 diff check) |
| CON-007 | T-004 (route + ViewToggle), T-005 (AC-007, AC-008) |
| CON-008 | T-002, T-003 (dark: classes), T-005 (AC-CON-008) |
| CON-009 | T-001 (no done/cancelled special-case), T-005 (AC-006) |
| CON-010 | T-003 (shared query key + badge), T-005 (AC-009) |
| CON-011 | T-002 (column testid), T-003 (board testid), T-005 (AC-CON-011) |

Every constraint is covered by ≥1 task and ≥1 test. No constraint is unaddressed.