# Tasks: Kanban View

**Feature ID**: kanban-view
**Phase**: planning
**Repo**: `devteam` (single repo — `ui/` only; no `internal/` changes)

Tasks grouped by user-story priority. Each task lists exact files, constraints, done conditions, test level, and agent failure-mode checks. Dependencies are explicit. `[P]` marks tasks parallelizable with their sibling.

## Ordering & parallelism

```
T001 (P1, grouping fn)
  └─> T002 (P1, board+column components)
        └─> T003 (P1, ViewToggle + Dashboard wiring)
              ├─> T004 [P] (P1+P2 e2e specs)
              └─> T005 [P] (integration specs)
                    └─> T006 (final gate verification)
```

---

## P1 — US-001, US-002, US-003, US-004

### T001 — Create pure grouping function + column constants
**Priority**: P1
**User stories**: US-001, US-002, US-005
**Constraints**: CON-001, CON-002, CON-004, CON-009
**Files**:
- `ui/src/lib/groupFeaturesByColumn.ts` — [CREATE]
**Dependencies**: none
**Done conditions**:
- File exports `ColumnKey` type = `'backlog' | PhaseName` and `COLUMN_KEYS` array `['backlog', ...PHASES]` and `COLUMN_LABELS` (`{ backlog: 'Backlog', ...PHASE_LABELS }`).
- File exports `groupFeaturesByColumn(features: FeatureSummary[]): Record<ColumnKey, FeatureSummary[]>`.
- Function pre-initializes all 7 keys to `[]` before iterating (never returns a missing key).
- Backlog rule: `f.status === 'draft' && f.current_phase === 'inception'` → `backlog`.
- Inception column: `f.current_phase === 'inception' && f.status !== 'draft'` → `inception`.
- Other phases: `f.current_phase === <phase>` → that column, regardless of status (terminal `done`/`cancelled` included).
- Unknown `current_phase` (not in `PHASES`): feature is dropped, no throw.
- `import { PHASES, PHASE_LABELS, PhaseName } from '../types'` — no re-declaration of the 6 phases.
- `npx tsc --noEmit` passes from `ui/`.
**Test level**: smoke (compiles) + e2e (behavior verified indirectly by T004's AC-001/004/005/006/011/013). Direct unit test deferred per plan AD-6.
**Agent failure mode checks**:
- [ ] Null/empty-array: input `[]` returns 7 keys each `[]` — no throw, no missing key.
- [ ] TS footgun: cast `f.current_phase as PhaseName` is guarded by `PHASES.includes(...)` before indexing the Record; a bad cast cannot produce a runtime `undefined` column.
- [ ] Multi-component: this is the single source of truth for grouping — no other component re-implements the rule.

---

### T002 — Create KanbanColumn and KanbanBoard components
**Priority**: P1
**User stories**: US-001, US-002, US-004, US-005
**Constraints**: CON-001, CON-004, CON-005, CON-008, CON-009, CON-011, FR-008, FR-009, FR-010, FR-012, FR-013, FR-014
**Files**:
- `ui/src/components/KanbanColumn.tsx` — [CREATE]
- `ui/src/components/KanbanBoard.tsx` — [CREATE]
**Dependencies**: T001 must complete first (imports `groupFeaturesByColumn`, `COLUMN_KEYS`, `COLUMN_LABELS`, `ColumnKey`).
**Done conditions**:
- `KanbanColumn` props: `{ columnKey: ColumnKey; label: string; features: FeatureSummary[] }`.
- `KanbanColumn` root element has `data-testid={`kanban-column-${columnKey}`}` (all 7: backlog, inception, planning, construction, review, testing, delivery).
- `KanbanColumn` header shows `label` + a numeric count of `features.length` (FR-008).
- `KanbanColumn` renders one `FeatureCard` per feature (imports `FeatureCard` from `./FeatureCard`; uses `<FeatureCard feature={f} />` — CON-005/AC-CON-005).
- `KanbanColumn` empty state: when `features.length === 0`, renders a non-blank message — Backlog: "No features waiting to start"; others: "No features in this phase" (FR-009).
- `KanbanColumn` uses `dark:` variants on container, header, body consistent with `FeatureCard`/`Dashboard` (CON-008/FR-010).
- `KanbanBoard` calls `useQuery({ queryKey: ['features'], queryFn: listFeatures })` (FR-014 — same key as Dashboard).
- `KanbanBoard` reads `data?.features ?? []` (never `data.features` directly — defends undefined while loading).
- `KanbanBoard` calls `groupFeaturesByColumn(...)` and renders one `KanbanColumn` per `COLUMN_KEYS` entry, in order (CON-001).
- `KanbanBoard` root has `data-testid="kanban-board"` (CON-011/FR-012) and a horizontally-scrollable container (`overflow-x-auto` + flex row) so all 7 columns are reachable on narrow viewports (FR-013).
- `KanbanBoard` loading: while `isLoading` and no `data`, render the existing spinner pattern (`animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600`) with `data-testid="kanban-loading"`.
- `KanbanBoard` error: while `error` and no `data`, render a board-level banner containing `"Failed to load features: {error.message}"` with `data-testid="kanban-error"`, and still render the 7 columns with empty buckets (AC-ERR-001). No uncaught throw.
- `KanbanBoard` error + stale data: while `error` and `data` both exist (refetch failure), keep stale cards visible (AC-ERR-002) — react-query default; do NOT clear `data` on error.
- `npm run build` passes from `ui/`.
- No new import from `api/client` other than `listFeatures` (CON-003).
- No new dependency added to `ui/package.json` (CON-006).
**Test level**: smoke (build passes) + e2e (T004 covers behavior) + integration (T005 covers AC-CON-005 source grep, AC-CON-006 dep diff).
**Agent failure mode checks**:
- [ ] Nil/undefined deref: `data?.features ?? []` — verified.
- [ ] Null vs empty array: grouping fn pre-inits all 7 keys; board never maps over a possibly-missing column.
- [ ] Multi-component consistency: `KanbanColumn` does NOT re-filter by status at render time — all features passed in are rendered. Status filtering lives only in `groupFeaturesByColumn` (T001). If a future "hide done" toggle is added, it goes in the grouping fn, not the column.
- [ ] Over-engineering: no drag-drop, no WIP limits, no column collapse, no animation, no virtualization. If new TSX exceeds ~200 lines across both files, re-read done conditions.
- [ ] Reuse: `FeatureCard` imported as-is; no wrapper component that re-implements its markup.

---

### T003 — Create ViewToggle and wire Dashboard
**Priority**: P1
**User stories**: US-003
**Constraints**: CON-007, CON-010, FR-006, FR-007
**Files**:
- `ui/src/components/ViewToggle.tsx` — [CREATE]
- `ui/src/pages/Dashboard.tsx` — [MODIFY]
**Dependencies**: T002 must complete first (Dashboard imports `KanbanBoard`).
**Done conditions**:
- `ViewToggle` props: `{ value: 'list' | 'board'; onChange: (v: 'list' | 'board') => void }`.
- `ViewToggle` renders two buttons "List" and "Board" with `data-testid="view-toggle-list"` and `data-testid="view-toggle-board"`; the active one is visually marked (e.g. `bg-blue-600 text-white`).
- `Dashboard` adds `const [viewMode, setViewMode] = useState<'list' | 'board'>('list')` (list is default — preserves existing behavior).
- `Dashboard` renders `<ViewToggle value={viewMode} onChange={setViewMode} />` in the header row (same row as the "Features" h2 + count badge + "New Feature" button). The count badge stays in this row regardless of `viewMode` (CON-010/AC-009).
- `Dashboard` body switches on `viewMode`:
  - `'list'` → existing `FeatureList`/`EmptyState`/loading/error flow (unchanged).
  - `'board'` → `<KanbanBoard />`.
- The existing `feature-count-badge`, `create-feature-button`, `IntakeForm`, and list-view loading/error markup remain unchanged and still render in `'list'` mode.
- In `'board'` mode, the list-view loading spinner / `features-error` / `EmptyState` do NOT render (the board owns its own loading/error). The count badge still renders (guarded by `!isLoading && !error` referring to the list query — but in board mode the badge should still show; resolution: lift the badge visibility to depend on the shared `['features']` query status, which both views share. Implementation detail: since both views use the same query key, the badge's existing `!isLoading && !error` guard works in both modes because the query state is shared).
- `npm run build` passes.
- Existing `ui/e2e/app.spec.ts` still passes (regression — Dashboard list behavior unchanged).
- No new dependency in `ui/package.json`.
**Test level**: smoke (build) + e2e (T004 AC-007/008/009) + regression (existing app.spec.ts).
**Agent failure mode checks**:
- [ ] Initialization ordering: `useState` initialized before any read; `ViewToggle` is controlled (no internal state that can desync from `viewMode`).
- [ ] Count badge persistence: badge is in the header row, outside the body switch — verified by reading the JSX structure. If the badge is accidentally moved inside the `'list'` branch, AC-009 fails.
- [ ] Over-engineering: no URL query param for view mode, no persistence to localStorage, no animated transition between views. (If a human wants persistence, that's a separate feature.)
- [ ] Regression: the `EmptyState` "Start Your First Feature" button still works in list mode.

---

### T004 [P] — E2E Playwright spec for board behavior
**Priority**: P1 (covers P1 + P2 ACs)
**User stories**: US-001, US-002, US-003, US-004, US-005, US-006
**Constraints**: CON-001, CON-002, CON-004, CON-005, CON-007, CON-008, CON-009, CON-010, CON-011
**Files**:
- `ui/e2e/kanban.spec.ts` — [CREATE]
**Dependencies**: T003 must complete first (board reachable via toggle). Parallelizable with T005.
**Done conditions** — the spec contains passing tests for each of:
- AC-001: seed features in inception/planning/delivery (via `POST /api/features` + `run`/`advance`); load `/`, click `view-toggle-board`; for each seeded feature assert `[data-testid="feature-card-{id}"]` is a descendant of `[data-testid="kanban-column-{current_phase}"]`.
- AC-002: query `[data-testid^="kanban-column-"]` under `kanban-board`; assert ordered suffix list === `['backlog','inception','planning','construction','review','testing','delivery']`.
- AC-003: for each column, header text contains the label (e.g. "Inception") and a count integer equal to the number of `[data-testid^="feature-card-"]` descendants in that column.
- AC-004: create a feature, do NOT run any phase; switch to board; assert card in `kanban-column-backlog` and NOT in `kanban-column-inception`.
- AC-005: create a feature, `POST /api/features/{id}/run`, wait for `status === 'in_progress'`; switch to board; assert card in `kanban-column-inception` and NOT in `kanban-column-backlog`.
- AC-006: seed/find a `done` feature in `delivery`; assert card in `kanban-column-delivery`.
- AC-007: load `/`, assert `feature-list` visible; click `view-toggle-board`; assert `kanban-board` visible and `feature-list` not visible.
- AC-008: from board, click `view-toggle-list`; assert `feature-list` visible and `kanban-board` not visible.
- AC-009: read `feature-count-badge` text → N; switch to board; assert badge still reads N.
- AC-010: seed a feature, switch to board, click `feature-card-{id}`; assert URL path === `/features/{id}` and FeatureDetail renders.
- AC-011: point at a fresh/cleaned specs dir (zero features); switch to board; for each `kanban-column-*` assert a non-empty empty-state message and zero `feature-card-*` descendants; capture `page.on('console')` and assert zero `error` entries.
- AC-013: seed 5 features, advance all to planning; assert `kanban-column-planning` has 5 cards and every other column has 0 cards + visible empty-state.
- AC-014: seed a feature in inception, switch to board, trigger an advance that invalidates `['features']` (e.g. `POST /api/features/{id}/advance` after gate, or a mutation); wait for refetch; assert card moves to `kanban-column-planning` and URL did not change.
- AC-CON-008: toggle dark mode via existing `ThemeToggle`; load board; assert board container + at least one column have dark-palette computed `background-color` (e.g. `rgb(31, 41, 55)` for `bg-gray-800`).
- AC-CON-011: for each testid in `{kanban-board, kanban-column-backlog, kanban-column-inception, kanban-column-planning, kanban-column-construction, kanban-column-review, kanban-column-testing, kanban-column-delivery}`, assert exactly one element exists.
- AC-ERR-003: seed a feature, load board, delete the feature's spec dir (or via a delete call if available), click the card, assert FeatureDetail renders its existing 404/not-found state with no console error.
- `npm run test:e2e` passes (all kanban spec tests green).
**Test level**: e2e (Playwright).
**Agent failure mode checks**:
- [ ] Console-error capture: every board-rendering test registers `page.on('console')` and `page.on('pageerror')` and asserts no errors — catches the null-array crash class.
- [ ] Seeding determinism: tests that need specific phases must drive the API to the target state, not assume pre-existing data. Tests that can't guarantee state (e.g. zero-feature test) must clean the specs dir or skip with a clear message.
- [ ] No brittle selectors: use `data-testid` exclusively (no text-based selectors for structural assertions; text OK for empty-state message assertion).

---

### T005 [P] — Integration spec: no-new-endpoint, no-new-dep, error paths
**Priority**: P1
**User stories**: US-001 (CON-003), US-005 (error paths), CON-006
**Constraints**: CON-003, CON-004, CON-006
**Files**:
- `ui/e2e/kanban-api.spec.ts` — [CREATE]
**Dependencies**: T003 must complete first. Parallelizable with T004.
**Done conditions** — the spec contains passing tests for:
- AC-CON-003 (no new backend endpoint): a Playwright test that (a) greps the served UI bundle or asserts via `request.get('/api/features')` that the board works with the existing endpoint, AND documents that the static check `git diff main -- internal/api/server.go ui/src/api/client.ts` shows no new mux handler / no new client function. The static diff is run as a build-time check or asserted in the test report — implement as a Playwright test that loads the board and asserts only `GET /api/features` is called (via `page.on('request')`) — no other `/api/*` kanban-specific route.
- AC-CON-006 (no new dep): document the `git diff main -- ui/package.json` check in the test report; the Playwright test itself asserts the board renders using only already-bundled modules (indirect — the real check is the diff).
- AC-ERR-001: `page.route('**/api/features', r => r.fulfill({ status: 500, json: {error:'internal_error', details:'db down'} }))`; load `/`, switch to board; assert `kanban-error` banner visible containing "Failed to load features"; assert `page.on('pageerror')` did not fire.
- AC-ERR-002: load board successfully, then intercept the next `GET /api/features` with 500; trigger a refetch (invalidate via a mutation — e.g. create a feature through the IntakeForm in list mode then switch back, or call `queryClient.invalidateQueries` through a test hook if exposed); assert no `pageerror`; assert an error indicator is visible OR stale cards remain (either per AC).
- `npm run test:e2e` passes.
**Test level**: integration (route interception + request observation + diff checks).
**Agent failure mode checks**:
- [ ] Route interception installed BEFORE `page.goto` (otherwise the first request slips through).
- [ ] `page.on('request')` filter scoped to `/api/` to avoid noise from static assets.
- [ ] No false positive on AC-CON-003: the board must not call any `/api/*` path other than `/api/features`. If it does, the test fails — that's the signal for a CON-003 violation.

---

## P2 — US-005 (empty state) — covered by T004 (AC-011, AC-013) and T001 (grouping fn empty-input behavior)

No separate P2 task. US-005's ACs are satisfied by T001's grouping fn (empty input → 7 empty cols, no throw) and T004's AC-011/AC-013 e2e tests. Listing here for traceability:
- AC-011 → T004
- AC-012 → T001 (behavior) + T004 (e2e witness); unit-level reclassified per plan AD-6
- AC-013 → T004

## P3 — US-006 (live updates) — covered by T004 (AC-014) and T002 (FR-014 reuse of `['features']` cache key)

No separate P3 task. US-006's AC-014 is satisfied by T002 (board uses `useQuery(['features'])` — same key as Dashboard, so invalidation propagates) and T004's AC-014 e2e test.

---

### T006 — Final gate verification
**Priority**: P1 (gate)
**User stories**: all
**Constraints**: CON-003, CON-006 (and overall gate)
**Files**: none (verification only)
**Dependencies**: T004 and T005 must complete first.
**Done conditions**:
- `cd ui && npm run build` passes with zero TS errors.
- `cd ui && npm run test:e2e` passes (both `app.spec.ts` and new `kanban.spec.ts` + `kanban-api.spec.ts`).
- `git diff main -- ui/package.json` shows **no additions** in `dependencies` or `devDependencies` (CON-006/AC-CON-006). Lockfile churn acceptable.
- `git diff main -- internal/` is **empty** (CON-003/AC-CON-003 — no backend changes).
- `git diff main -- ui/src/api/client.ts` shows no new function (AC-CON-003).
- `grep -r "kanban" ui/src/components/KanbanBoard.tsx ui/src/components/KanbanColumn.tsx` confirms `FeatureCard` is imported and rendered (AC-CON-005).
- No new files outside `ui/src/components/Kanban*.tsx`, `ui/src/components/ViewToggle.tsx`, `ui/src/lib/groupFeaturesByColumn.ts`, `ui/src/pages/Dashboard.tsx` (modified), `ui/e2e/kanban*.spec.ts`.
**Test level**: smoke + integration (diff checks).
**Agent failure mode checks**:
- [ ] Blast radius: grep the whole repo for any accidental `internal/` or `cmd/` edit. If found, revert — out of scope.
- [ ] Over-engineering: total new TSX/TS lines ≤ ~300. If significantly more, flag for review.

---

## Constraint coverage summary

| CON | Task(s) |
|-----|---------|
| CON-001 | T001, T002, T004 (AC-002) |
| CON-002 | T001, T004 (AC-004, AC-005) |
| CON-003 | T002, T005 (AC-CON-003), T006 |
| CON-004 | T001, T002, T004 (AC-011, AC-013) |
| CON-005 | T002, T005 (AC-CON-005), T006 |
| CON-006 | T002, T005 (AC-CON-006), T006 |
| CON-007 | T003, T004 (AC-007, AC-008) |
| CON-008 | T002, T004 (AC-CON-008) |
| CON-009 | T001, T004 (AC-006) |
| CON-010 | T003, T004 (AC-009) |
| CON-011 | T002, T004 (AC-CON-011) |

Every constraint has at least one task. Every task references its constraints.

## Acceptance criteria coverage summary

| AC | Task |
|----|------|
| AC-001..006 | T004 |
| AC-007..009 | T003, T004 |
| AC-010 | T004 |
| AC-011, AC-013 | T001, T002, T004 |
| AC-012 | T001 (behavior) + T004 (e2e witness) — unit reclassified per plan AD-6 |
| AC-014 | T002 (cache key) + T004 |
| AC-CON-003 | T002, T005, T006 |
| AC-CON-005 | T002, T005, T006 |
| AC-CON-006 | T005, T006 |
| AC-CON-008 | T002, T004 |
| AC-CON-011 | T002, T004 |
| AC-ERR-001, AC-ERR-002 | T002 (error UI), T005 |
| AC-ERR-003 | T004 |

Every acceptance criterion is covered by at least one task.