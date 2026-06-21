# Plan: Kanban View

**Feature ID**: kanban-view
**Phase**: Planning
**Created**: 2026-06-21
**Spec**: `specs/kanban-view/spec.md`
**Acceptance**: `specs/kanban-view/acceptance.md`
**Repos**: `devteam` only (UI-only change)

## Spec Validation

- **Completeness**: All 14 FRs trace to user stories US-001..US-006. ✓
- **Constraint register**: 11 constraints (CON-001..CON-011) all addressable by UI changes. ✓
- **Consistency**: No FR contradicts another. FR-002 (Backlog rule) and FR-003 (phase-column rule) are mutually exclusive by construction — a feature is either `draft+inception` (Backlog) or not (its `current_phase` column). ✓
- **Feasibility**: All FRs satisfiable with existing React + react-router + @tanstack/react-query + Tailwind. No new dependency required. ✓
- **Edge cases**: Empty state (zero features), per-column empty state, API 500, mid-flight refetch error, deleted-while-viewing — all defined in spec Error Scenarios. ✓
- **Negative vectors**: No external standard governs this feature; no conformance vectors. The "negative cases" are the error-path ACs (AC-ERR-001..003), addressed in Test Strategy. ✓
- **Ambiguities**: All [ASSUMPTION] markers from spec are accepted as planning inputs. One architecture decision is required from the spec's open assumption: **view toggle vs. dedicated route**. Decision below (§Architecture Decisions, AD-002).

## Technical Context

- **Language**: TypeScript (UI), Go (backend — unchanged)
- **UI Framework**: React 19, react-router 7, @tanstack/react-query 5, Tailwind 4 (via `@tailwindcss/vite`)
- **Build**: Vite 6, `tsc -b`
- **Test runner (E2E)**: Playwright 1.61, config `ui/playwright.config.ts`, tests `ui/e2e/*.spec.ts`, baseURL `http://localhost:8765`
- **Test runner (unit)**: No existing unit test runner in `ui/package.json`. **Decision: add none.** The single unit-testable artifact is the pure grouping function `groupByColumn`. It will be verified via (a) an E2E test that exercises the same inputs through the rendered DOM (AC-012 is covered by AC-011/AC-013 which render the same empty/partial states) and (b) a `__main__`-style self-check is not idiomatic for TS. Instead the grouping function gets a colocated Vitest-free assertion: a typed `assert`-based demo is YAGNI for a pure function with full E2E coverage. **If the reviewer/tester demands unit coverage, add Vitest as a devDep — flagged in Open Questions.**
- **Backend**: `GET /api/features` returns `{"features": FeatureSummary[], "total_count": number}`. Empty list serializes as `[]` (verified by existing test `app.spec.ts` "API returns valid JSON with arrays not null"). **No backend change.**
- **Existing reusable components**:
  - `ui/src/components/FeatureCard.tsx` — card with title, status/phase/priority badges, gate indicator, updated date, `<Link to={/features/{id}}>`. Reused as-is.
  - `ui/src/components/EmptyState.tsx` — page-level empty state; **not reused** for per-column empty state (different contract: no "create" CTA inside a column). A small inline per-column empty message is used instead.
  - `ui/src/components/Toast.tsx`, `ThemeToggle.tsx`, `ConnectionStatus.tsx` — untouched.
  - `ui/src/types/index.ts` — `PHASES`, `PHASE_LABELS`, `STATUS_LABELS`, `FeatureSummary`, `FeatureListResponse` — reused, extended with one constant (`KANBAN_COLUMNS`).

## Project Structure

All changes under `ui/src/` and `ui/e2e/`. No `internal/`, `cmd/`, or Go changes.

```
ui/src/
  App.tsx                          [MODIFY] — add /kanban route
  pages/
    Dashboard.tsx                  [MODIFY] — add view-toggle control in header
    KanbanBoard.tsx                [CREATE] — board page; uses useQuery(['features']); renders columns
  components/
    KanbanColumn.tsx               [CREATE] — single column: header (name + count), card list, empty-state msg
    ViewToggle.tsx                 [CREATE] — segmented control "List | Board"; uses react-router navigate
    FeatureCard.tsx                [unchanged] — reused
  types/
    index.ts                       [MODIFY] — add KANBAN_COLUMNS constant + ColumnKey type
  api/
    client.ts                      [unchanged] — reuses listFeatures
ui/e2e/
  kanban.spec.ts                   [CREATE] — all kanban E2E ACs
  app.spec.ts                      [unchanged]
ui/package.json                    [unchanged — no new deps]
```

## Data Model

No new entities. The board is a derived view over `FeatureSummary[]`.

### Derived grouping (canonical rule from spec §Derived grouping rule)

```
type ColumnKey = 'backlog' | 'inception' | 'planning' | 'construction' | 'review' | 'testing' | 'delivery';

function columnForFeature(f: FeatureSummary): ColumnKey {
  if (f.status === 'draft' && f.current_phase === 'inception') return 'backlog';
  return f.current_phase as ColumnKey; // current_phase ∈ PHASES for any non-draft-or-non-inception feature
}
```

**Edge case — `current_phase` not in PHASES**: defensive fallback. If a feature has an unknown `current_phase` (future phase added backend-side), it falls through to a catch-all. **Decision: log to console.warn and place in the `delivery` column is WRONG (misleading). Instead: render an "Unknown phase" column is OUT OF SCOPE (spec says 7 columns fixed). Correct conservative behavior: drop the card from the board and `console.warn` with the feature id + phase.** Documented in `KanbanBoard.tsx`. This is the only parsing-style failure path and it is caught + logged, never thrown (agent failure mode: parsing-safety).

### Column ordering

`KANBAN_COLUMNS: ColumnKey[] = ['backlog','inception','planning','construction','review','testing','delivery']` — single source of truth for both render order and `data-testid` suffixes.

### State transitions

The board introduces no state transitions. Feature state remains governed by `internal/feature/feature.go`. The board only observes.

## API Contracts

**No new API.** The board consumes the existing endpoint:

```
GET /api/features
Response 200:
  { "features": FeatureSummary[], "total_count": number }
Response 500:
  { "error": "internal_error", "details": string }
```

`FeatureSummary` shape (from `ui/src/types/index.ts`):
```
{ id: string, title: string, status: string, priority: number,
  current_phase: string, updated_at: string,
  gate_result: GateResult | null, pending_questions_count: number }
```

The board uses `listFeatures()` from `ui/src/api/client.ts` and the existing `useQuery({ queryKey: ['features'], queryFn: listFeatures })` — **same cache key as Dashboard**, so the two views share data and stay in sync without a second fetch (CON-010, FR-007, FR-014).

## Architecture Decisions

- **AD-001 — Single shared `['features']` cache.** Board and Dashboard both call `useQuery({ queryKey: ['features'] })`. No separate fetch, no separate cache entry. Mutations that invalidate `['features']` (create/advance/recirculate/cancel) propagate to both views automatically. Satisfies FR-014, CON-010.
- **AD-002 — Dedicated `/kanban` route, not a view toggle in shared header.** Spec left this open ([ASSUMPTION: view toggle vs route]). Decision: **route**. Rationale: (a) react-router is already a dependency and already routes `/` and `/features/:id`; (b) a route is deep-linkable, bookmarkable, and back-button friendly; (c) a segmented control in the Dashboard header forces the toggle to live on one page only, requiring the Board to re-implement the toggle to switch back — a route lets each page link to the other. **A `ViewToggle` component is still created** but it renders as a pair of `<Link>`s (List ↔ Board), placed in the page header of *both* Dashboard and KanbanBoard so the user can switch from either side. Satisfies FR-006, CON-007.
- **AD-003 — Board is read-only.** No drag-and-drop, no inline edit. Phase changes happen on the detail page (existing). Stated out-of-scope in spec.
- **AD-004 — Horizontal scroll on narrow viewports.** Board container uses `overflow-x-auto` with a min-width inner flex row. Each column has a fixed min-width (e.g. `min-w-[280px]`). No responsive collapse (out of scope per spec [ASSUMPTION]).
- **AD-005 — Per-column empty state is inline text, not the `EmptyState` component.** `EmptyState` has a "create" CTA which is wrong inside a column. Each column renders `<div data-testid="kanban-column-empty-{key}">No features in this phase</div>` (Backlog: "No features waiting to start"). Satisfies FR-009, CON-004.
- **AD-006 — Error banner is board-level, not column-level.** When `useQuery` returns `error`, render a board-level banner `data-testid="kanban-error"` with "Failed to load features: {message}" and columns render empty (no cards). Matches existing Dashboard error pattern (`features-error` testid). Satisfies AC-ERR-001, AC-ERR-002.
- **AD-007 — Unknown `current_phase` handling.** See Data Model. Card dropped + `console.warn`. Never throws.

## Constraint Verification Map

| CON-ID | Design Decision | Component(s) | Verification Checkpoint | Test Type |
|--------|-----------------|--------------|------------------------|-----------|
| CON-001 | `KANBAN_COLUMNS` constant in `types/index.ts` is the single source of truth for column order; `KanbanBoard` maps over it in order. No reordering logic. | `types/index.ts`, `KanbanBoard.tsx` | E2E AC-002: column `data-testid` order equals `['backlog','inception','planning','construction','review','testing','delivery']` | e2e |
| CON-002 | `columnForFeature(f)` returns `'backlog'` iff `f.status==='draft' && f.current_phase==='inception'`; else returns `f.current_phase`. Pure function, colocated with board. | `KanbanBoard.tsx` (or `utils/groupByColumn.ts`) | E2E AC-004 (draft+inception → backlog) and AC-005 (in_progress+inception → inception column) | e2e + unit-via-e2e |
| CON-003 | Board imports `listFeatures` from `api/client.ts` and calls `useQuery(['features'])`. No new client function, no new backend route. | `KanbanBoard.tsx`, `api/client.ts` (unchanged) | Integration AC-CON-003: `git diff main -- internal/api/server.go ui/src/api/client.ts` shows no new route registration and no new client function | integration (diff check) |
| CON-004 | Board reads `data.features ?? []` (defensive) and grouping function returns 7 columns each with an empty array when input is `[]`. No `.map` on possibly-null. | `KanbanBoard.tsx` | E2E AC-011 (zero features → 7 columns, empty-state each, no console error) + AC-012 (grouping `[]` returns 7 empty columns, no throw) | e2e + unit |
| CON-005 | Board imports `FeatureCard` and renders `<FeatureCard feature={f} />` per card. No re-implementation of card markup. | `KanbanBoard.tsx`, `KanbanColumn.tsx` | Unit AC-CON-005: source contains `import FeatureCard` and `<FeatureCard` usage | unit (source inspection) |
| CON-006 | `ui/package.json` `dependencies` and `devDependencies` blocks unchanged. Board uses only React, react-router, @tanstack/react-query, Tailwind. | `package.json` (unchanged) | Integration AC-CON-006: `git diff main -- ui/package.json` shows no additions in dep blocks | integration (diff check) |
| CON-007 | `ViewToggle` component renders `<Link to="/">` and `<Link to="/kanban">`; placed in headers of both Dashboard and KanbanBoard. | `ViewToggle.tsx`, `Dashboard.tsx`, `KanbanBoard.tsx`, `App.tsx` | E2E AC-007 (Dashboard → Board) and AC-008 (Board → Dashboard) | e2e |
| CON-008 | Board and columns use Tailwind `dark:` variants mirroring Dashboard (e.g. `bg-white dark:bg-gray-800`, `text-gray-900 dark:text-white`). No custom CSS. | `KanbanBoard.tsx`, `KanbanColumn.tsx` | E2E AC-CON-008: in dark mode, board container + one column have dark palette computed bg | e2e |
| CON-009 | `columnForFeature` does NOT special-case `done`/`cancelled` — they fall through to the `current_phase` branch. Terminal features appear in their phase column. | `KanbanBoard.tsx` | E2E AC-006: `done` feature in `delivery` appears in `kanban-column-delivery` | e2e |
| CON-010 | Board reads `data.total_count ?? 0` from the same `useQuery(['features'])` as Dashboard and renders the same `feature-count-badge` testid. | `KanbanBoard.tsx` | E2E AC-009: badge text matches Dashboard badge text after switch | e2e |
| CON-011 | Board root has `data-testid="kanban-board"`; each column has `data-testid="kanban-column-{key}"` for key in KANBAN_COLUMNS. | `KanbanBoard.tsx`, `KanbanColumn.tsx` | E2E AC-CON-011: each of the 8 testids exists exactly once | e2e |

## Cross-Component Consistency Matrix

This feature has one producer (the `GET /api/features` endpoint, unchanged) and one consumer (the new `KanbanBoard`). The Dashboard remains a second consumer of the same endpoint. Consistency checks:

| Shared Value | Producer | Consumer(s) | Consistent? | Verification |
|--------------|----------|-------------|-------------|-------------|
| `FeatureSummary` shape | Go `internal/api/dto.go` (unchanged) | `KanbanBoard`, `Dashboard`, `FeatureCard` | YES — board imports the existing `FeatureSummary` type from `types/index.ts`; no redefinition | tsc compile + e2e render |
| `current_phase` enum values | Go `internal/feature/types.go` (unchanged): inception, planning, construction, review, testing, delivery | `KanbanBoard.columnForFeature` | YES — board's `ColumnKey` is `backlog` ∪ `PHASES`; unknown values are dropped+warned, not misclassified | e2e AC-001 (features in inception/planning/delivery land in correct columns) |
| `status` enum values | Go `internal/feature/types.go` (unchanged) | `KanbanBoard.columnForFeature` (only checks `==='draft'`) | YES — only `draft` is special-cased; all other statuses fall through to phase column. Any new status added backend-side still renders in its phase column | e2e AC-005, AC-006 |
| `total_count` | Go `dto.go` `FeatureListResponse.total_count` | `KanbanBoard` badge, `Dashboard` badge | YES — both read from the same `useQuery(['features'])` response; single source of truth | e2e AC-009 |
| `['features']` query cache | `useQuery(['features'])` in Dashboard (existing) and KanbanBoard (new) | Both views | YES — same query key → react-query dedupes; both views share data; invalidation propagates to both | e2e AC-014 |
| Empty array serialization | Go `dto.go` returns `features: []` not null (existing test `app.spec.ts` verifies) | `KanbanBoard` reads `data.features ?? []` | YES — consumer is defensive even though producer already emits `[]` | e2e AC-011, AC-012 |
| `data-testid` for feature cards | `FeatureCard` (unchanged): `feature-card-{id}` | `KanbanBoard` E2E selectors | YES — board reuses `FeatureCard` so testids are identical | e2e AC-001, AC-010 |
| Column `data-testid` suffixes | `KANBAN_COLUMNS` constant | `KanbanColumn` render + E2E selectors | YES — single constant drives both | e2e AC-CON-011 |

**No multi-component constraint applies** (no "all providers" style constraint in this feature's register). The feature is a single-component UI addition.

## Test Strategy

### Component: `KanbanBoard` (page)

Testing levels required:
- **Smoke**: page renders at `/kanban` without crash; `kanban-board` testid present; no `pageerror` events.
- **Integration**: `GET /api/features` 500 → board renders `kanban-error` banner, no crash (AC-ERR-001). `git diff main -- ui/package.json` shows no new deps (AC-CON-006). `git diff main -- internal/api/server.go ui/src/api/client.ts` shows no new route/fn (AC-CON-003).
- **E2E**: all AC-001..AC-014, AC-CON-008, AC-CON-011, AC-ERR-003 via Playwright.
- **Unit (source inspection)**: `KanbanBoard.tsx` imports `FeatureCard` and `listFeatures` (AC-CON-005). Grouping function handles `[]` without throw (AC-012 — covered by e2e AC-011; if unit runner exists, add a direct test; see Open Questions).

Quality checkpoints:
- [ ] `cd ui && npm run build` (tsc + vite) succeeds
- [ ] `cd ui && npm run lint` succeeds (existing eslint config)
- [ ] `cd ui && npx playwright test e2e/kanban.spec.ts` all green
- [ ] `cd ui && npx playwright test` (full suite incl. `app.spec.ts`) still green — no regression
- [ ] No new entries in `ui/package.json` `dependencies`/`devDependencies`
- [ ] No new route in `internal/api/server.go`, no new fn in `ui/src/api/client.ts`
- [ ] Board renders with zero features without `pageerror`
- [ ] Board renders with `GET /api/features` 500 without `pageerror`
- [ ] Dark mode toggles board bg (visual or computed-style assertion)

### Component: `KanbanColumn`

- **Smoke**: renders header with name + count; renders empty-state when cards=[].
- **E2E**: AC-003 (header count == card count), AC-013 (one column has 5, others empty-state).
- Quality checkpoints:
  - [ ] Empty column shows non-blank empty-state message
  - [ ] Count is integer equal to descendant `feature-card-*` count

### Component: `ViewToggle`

- **E2E**: AC-007 (Dashboard → Board), AC-008 (Board → Dashboard).
- Quality checkpoints:
  - [ ] Both links present on both pages
  - [ ] Active page's own link is not navigable (or is visually marked active — not required by AC, skip)

### Negative Case Design

This feature has no external-standard negative test vectors. The "negative cases" are error paths:

| Negative Case | Design | Verification |
|---------------|--------|--------------|
| API 500 on board load | Board reads `useQuery` `error`; renders `kanban-error` banner; columns render with empty-state (no cards). Never throws. | AC-ERR-001 (Playwright route interception 500) |
| API 500 on refetch mid-session | react-query keeps stale data by default; `error` state triggers banner; stale cards remain. Never throws. | AC-ERR-002 |
| Card clicked → feature deleted between load and click | Board does not handle; navigation to `/features/{id}` proceeds; existing FeatureDetail 404 state handles it. | AC-ERR-003 |
| `features: []` (empty array) | Grouping returns 7 columns each `cards: []`; columns render empty-state. No `map of null`. | AC-011, AC-012 |
| `features` field missing or null (defensive) | Board reads `data?.features ?? []`. Never throws. | Covered by AC-011 (smoke + defensive coalescing) |
| Unknown `current_phase` value | `columnForFeature` drops card + `console.warn`. Board still renders 7 columns. No throw. | New e2e assertion in `kanban.spec.ts`: seed feature with mocked `current_phase: 'unknown'`, assert card absent from all columns, assert `console.warn` captured, no `pageerror`. |

### Agent Failure Mode Checks (per task)

- **JSON serialization / null arrays**: Board reads `data?.features ?? []` and `data?.total_count ?? 0`. Grouping function initializes all 7 columns to `[]` before assigning cards. No `omitempty`-style bug possible (no serialization is produced by the board — it only consumes).
- **Parsing safety (unknown phase)**: `columnForFeature` catches unknown `current_phase` and drops + warns. Never throws. Tested by the new e2e assertion above.
- **Nil pointer / null deref (TS equivalent)**: all field access on `data` uses optional chaining; `FeatureSummary` fields are accessed only after the object is known to exist (it comes from a typed API response).
- **Multi-component consistency**: only one component produces the grouping (`KanbanBoard`); no "all providers" pattern. N/A.
- **State machine logic**: none introduced. N/A.
- **HTTP middleware**: none introduced. N/A.
- **Initialization ordering**: `KANBAN_COLUMNS` is a module-level constant; no init ordering hazard.

## Quality Checkpoints (task boundaries)

- After T-001 (types + grouping): `npm run build` passes; grouping function imported by board.
- After T-002 (KanbanColumn): renders in isolation (smoke via board mount).
- After T-003 (KanbanBoard): `/kanban` route reachable; `kanban-board` testid present; smoke e2e green.
- After T-004 (ViewToggle + Dashboard + App route): navigation both directions works; AC-007, AC-008 green.
- After T-005 (E2E suite): all kanban ACs green; full `app.spec.ts` still green; no new deps; no backend diff.

## Quickstart for the Developer

1. Branch from `main` (worktree already set up at `worktrees/kanban-view` if the pipeline created one; otherwise `git checkout -b kanban-view`).
2. All work is under `ui/`. No Go changes. No `internal/` changes.
3. Read `specs/kanban-view/spec.md` and `specs/kanban-view/acceptance.md` first.
4. Implement tasks T-001 → T-005 in order (dependencies are linear).
5. After each task, run `cd ui && npm run build` to typecheck.
6. To run E2E: start the backend (`cd .. && ~/go/bin/devteam -http :8765` from `ui/`) in one terminal, then `cd ui && START_SERVER=1 npx playwright test e2e/kanban.spec.ts`. Or rely on the playwright config's `webServer` which starts it for you.
7. Verify no new deps: `git diff main -- ui/package.json` should be empty (or only lockfile churn).
8. Verify no backend diff: `git diff main -- internal/ cmd/` should be empty.
9. Hand off to Reviewer when all tasks done and all e2e green.

## Open Questions

- **Unit test runner**: `ui/package.json` has no Vitest/Jest. The single unit-testable artifact (`groupByColumn`) is fully covered by E2E (AC-011/AC-012/AC-013). **Plan: no unit runner added.** If the Tester demands a unit-level test, add Vitest as a devDep — but this violates CON-006's "no new dependency" only if added to `dependencies`; devDep additions are also forbidden by AC-CON-006 ("no dependency has been added to `dependencies` or `devDependencies`"). **Therefore: do NOT add Vitest. Unit coverage is provided via E2E.** Flagged for Tester review.
- **Unknown `current_phase` e2e**: requires mocking the API response (Playwright route interception) to inject a feature with `current_phase: 'unknown'`. This is straightforward but the Developer must remember to add it — it is listed in T-005's done conditions.