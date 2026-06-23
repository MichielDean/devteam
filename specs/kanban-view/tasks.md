# Tasks: kanban-view

**Input**: Design documents from `specs/kanban-view/` (plan.md, spec.md, research.md, data-model.md, contracts/GET-api-features.md).

**Prerequisites**: `spec.md`, `acceptance.md`, `repos.yaml` exist (CON-005 — verified). Constitution check passed (plan.md).

**Tests**: This feature is UI-only and acceptance.md mandates tests for every AC. Tests are NOT optional — every user story has Playwright specs in `ui/e2e/kanban.spec.ts`.

**Organization**: Tasks grouped by user story priority (P1 → P2 → P3), then a final gate. Single repo (`devteam`), `ui/` only.

**Path conventions**: paths are relative to repo root. `ui/src/...` for source, `ui/e2e/...` for Playwright specs.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependencies)
- **[Story]**: US1, US2, US3, or EDGE (edge cases), or SHARED

---

## Phase 1: Shared Foundation

**Purpose**: Create the new board component that both US1 (render) and EDGE (unknown phase) depend on. No user story work can start until this exists.

- [ ] T001 [SHARED] Create `KanbanBoard` component in `ui/src/components/KanbanBoard.tsx`
  - **CREATE** `ui/src/components/KanbanBoard.tsx`
  - **Reuse** `FeatureCard` (`ui/src/components/FeatureCard.tsx`), `PHASES`/`PHASE_LABELS`/`FeatureSummary` (`ui/src/types/index.ts`).
  - Export a pure helper `groupFeaturesByPhase(features: FeatureSummary[]): Array<{ phase: string; label: string; features: FeatureSummary[] }>`:
    - Returns one entry per `PHASES` element, in `PHASES` order, each with `label = PHASE_LABELS[phase]` and the features whose `current_phase === phase` (API order preserved, no re-sort — FR-003).
    - If any feature has `current_phase` not in `PHASES`, append a trailing entry `{ phase: 'other', label: 'Other', features: [...] }` (FR-013, AC-016/017). `// ponytail: "Other" is a UI fallback, not a pipeline phase; intentionally not in PHASE_LABELS.`
    - Must not throw on any input (CON-011). Guard `features ?? []` at the top.
  - Render: a horizontally-scrollable container (`overflow-x-auto`, `data-testid="kanban-board"`), inside it a flex row (`flex gap-4 min-w-max`) of columns. Each column:
    - `<div data-testid={\`kanban-column-${entry.phase}\`}>` with a header (`<h3>`) showing `entry.label` and a body containing `entry.features.map(f => <FeatureCard feature={f} />)`.
    - Fixed min width (`min-w-[16rem]`), full-height flex column.
  - **Constraints addressed**: CON-004 (import `PHASES`/`PHASE_LABELS`, no literal phase strings), CON-008 (every element has `data-testid`), CON-011 (unknown phase → "Other", no throw).
  - **Done conditions**:
    - `cd ui && npm run build` succeeds (TypeScript + Vite).
    - `grep -nE "'(Inception|Planning|Construction|Review|Testing|Delivery)'" ui/src/components/KanbanBoard.tsx` returns 0 (CON-004).
    - `grep -n "8765" ui/src/components/KanbanBoard.tsx` returns 0 (CON-003).
    - Every rendered element has a `data-testid` (board root, each column, each card via `FeatureCard`).
    - Six `kanban-column-<phase>` elements always render regardless of feature distribution (FR-012).
    - "Other" column renders iff at least one feature has `current_phase` not in `PHASES` (FR-013).
  - **Test level**: smoke (build) + unit (behavioral, via `kanban.spec.ts` AC-016/017/018 in T003).
  - **Agent failure mode checks**:
    - [ ] No `switch` without a default — use `Set<string>` from `PHASES` for membership; unknown values fall through to "Other".
    - [ ] `features ?? []` guard prevents nil deref if caller passes `undefined`.
    - [ ] No `JSON.stringify`/serialization produced — N/A for null-array check.
    - [ ] No HTTP middleware / state machine — N/A.
    - [ ] `data-testid` uses the phase identifier (`entry.phase`), not a label string.

**Checkpoint**: `KanbanBoard` builds and renders standalone (manual `npm run dev` + temporary wire-up). Foundation ready for US1/US2/US3.

---

## Phase 2: User Story 1 — Kanban board view (Priority: P1) 🎯 MVP

**Goal**: User can toggle to a Kanban board on the Dashboard, see six phase columns with the right cards, click a card to navigate, and toggle back to list.

**Independent Test**: Navigate to `/`, click Kanban, verify six labelled columns with cards in the matching `current_phase` column, click a card → `/features/:id`, click List → `feature-list` returns.

### Implementation for User Story 1

- [ ] T002 [US1] Wire view toggle + conditional render into `ui/src/pages/Dashboard.tsx`
  - **MODIFY** `ui/src/pages/Dashboard.tsx`
  - Add `const VIEW_STORAGE_KEY = 'devteam.dashboard.view';` module constant.
  - Add `type DashboardView = 'list' | 'kanban';` (local — matches `data-model.md`).
  - Replace the happy-path branch (`!isLoading && !error && features.length > 0 && <FeatureList .../>`) with:
    - A view toggle (two `<button>`s): `view-toggle-list` and `view-toggle-kanban`, each with `aria-pressed={view === 'list'}` / `aria-pressed={view === 'kanban'}` (CON-008). Toggle only rendered when `!isLoading && !error` (regardless of empty state — but the empty branch already short-circuits above; simplest: render toggle only in the `features.length > 0` happy path alongside the view switch).
    - Conditional: `view === 'kanban' ? <KanbanBoard features={features} /> : <FeatureList features={features} />`.
  - `view` state: `useState<DashboardView>(() => readView())` where `readView` wraps `localStorage.getItem(VIEW_STORAGE_KEY)` in try/catch and returns `'kanban'` only if the stored value equals `'kanban'`, else `'list'` (FR-008, FR-009, AC-009, AC-011). Whitelist the known value; never echo arbitrary strings.
  - On toggle click: `setView(next)` then call `writeView(next)` which wraps `localStorage.setItem(VIEW_STORAGE_KEY, next)` in try/catch and swallows errors (FR-007, FR-009, AC-010).
  - **Keep the loading (`features-loading`), error (`features-error`), and empty (`EmptyState`) branches exactly where they are** — above the happy-path view switch (CON-009, CON-010, AC-005/006/007).
  - **Constraints addressed**: CON-008 (toggle testids), CON-009 (loading/error unchanged), CON-010 (empty state unchanged), FR-001 (toggle), FR-006 (list unchanged), FR-007/008/009 (localStorage), FR-014 (same `useQuery` result — no new fetch).
  - **Done conditions**:
    - `cd ui && npm run build` succeeds.
    - `grep -n "8765" ui/src/pages/Dashboard.tsx` returns 0 (CON-003).
    - Manual: toggle to Kanban → six columns render; toggle to List → `feature-list` renders; reload after Kanban → board restores; clear localStorage + reload → list.
    - `localStorage.setItem` throwing (manual `addInitScript` or DevTools override) does not crash the toggle; view still switches in-memory.
    - `localStorage.getItem` throwing does not crash mount; list view renders.
    - Existing `ui/e2e/app.spec.ts` passes unmodified (list is default — no regression).
  - **Test level**: smoke (build + manual) + e2e (`kanban.spec.ts` AC-001/003/005/006/007 in T003) + integration (`kanban.spec.ts` AC-004).
  - **Agent failure mode checks**:
    - [ ] `localStorage` read wrapped in try/catch — no uncaught exception on mount (AC-011).
    - [ ] `localStorage` write wrapped in try/catch — no uncaught exception on toggle (AC-010).
    - [ ] `useState` lazy initializer — never throws during render.
    - [ ] No new `useQuery` call — both views share `data` from the existing query (FR-014, AC-004).
    - [ ] `FeatureList` import/render path unchanged (SC-005).

**Checkpoint**: User Story 1 fully functional and testable independently. `npm run dev` + manual toggle works end-to-end.

---

## Phase 3: User Story 2 — View persistence (Priority: P2)

**Goal**: Dashboard remembers the user's last view via `localStorage`; first-time visitors see list; storage failures degrade gracefully.

**Independent Test**: Toggle to Kanban, reload, verify board renders without re-toggling; clear localStorage, reload, verify list; break `localStorage` with `addInitScript`, verify no crash.

### Implementation for User Story 2

- [ ] T003 [US2] Verify persistence + storage-failure fallbacks (covered by `kanban.spec.ts`)
  - **No new source code** — US2 is implemented entirely within T002's `readView`/`writeView` wrappers in `Dashboard.tsx`.
  - **This task exists to make the US2 acceptance criteria explicit in the test file** (T004 below) and to record that no additional implementation task is needed.
  - **Constraints addressed**: FR-007, FR-008, FR-009; AC-008, AC-009, AC-010, AC-011.
  - **Done conditions** (verified by `kanban.spec.ts` in T004):
    - AC-008: reload after Kanban → `kanban-column-inception` visible, `localStorage['devteam.dashboard.view'] === 'kanban'`.
    - AC-009: clear localStorage + load `/` → `feature-list` visible, no `kanban-column-*`.
    - AC-010: `localStorage.setItem` throws → toggling still renders `kanban-column-*`, no `pageerror`.
    - AC-011: `localStorage.getItem` throws → `feature-list` visible, no `pageerror`.
  - **Test level**: e2e + unit (behavioral, via Playwright).
  - **Agent failure mode checks**: documented in T002; no new code here.

**Checkpoint**: US2 verifiable via `kanban.spec.ts`. No standalone implementation — folded into T002.

---

## Phase 4: User Story 3 — Card information density (Priority: P3)

**Goal**: Kanban cards show pending-questions badge and gate indicator, matching `FeatureCard` parity.

**Independent Test**: Render a feature with `pending_questions_count > 0` and one with `gate_result`, toggle to Kanban, verify badge + gate indicator render.

### Implementation for User Story 3

- [ ] T004 [US3] Confirm card density parity (covered by `FeatureCard` reuse)
  - **No new source code** — US3 is satisfied by reusing `FeatureCard` inside `KanbanBoard` (T001). `FeatureCard` already renders `QuestionBadge` when `pending_questions_count > 0` (FR-010) and `feature-card-gate` when `gate_result` is present (FR-011).
  - **This task exists to make the US3 acceptance criteria explicit in `kanban.spec.ts`** (T005 below).
  - **Constraints addressed**: FR-010, FR-011; AC-012, AC-013, AC-014, AC-015.
  - **Done conditions** (verified by `kanban.spec.ts` in T005):
    - AC-012: feature with `pending_questions_count: 3` → `question-badge` text is `3`.
    - AC-013: `gate_result.passed === false` → `feature-card-gate` present with failed treatment.
    - AC-014: `gate_result.passed === true` → `feature-card-gate` present with passed treatment.
    - AC-015: `gate_result === null` → no `feature-card-gate` element inside that card.
  - **Test level**: integration (Playwright with mocked `/api/features`).
  - **Agent failure mode checks**: N/A (no new code).

**Checkpoint**: US3 verifiable via `kanban.spec.ts`. No standalone implementation — folded into T001's `FeatureCard` reuse.

---

## Phase 5: Edge Cases (cross-story)

**Purpose**: Unknown-phase handling, empty board, rapid toggle, large board.

- [ ] T005 [EDGE] Create Playwright spec covering AC-001..AC-019 in `ui/e2e/kanban.spec.ts`
  - **CREATE** `ui/e2e/kanban.spec.ts`
  - Follow the pattern in `ui/e2e/app.spec.ts`: `import { test, expect } from '@playwright/test';` capture console errors via `page.on('console', ...)`; mock `/api/features` via `page.route('**/api/features', ...)`.
  - One `test(...)` per AC, titled with the AC ID for traceability (CON-006): e.g. `test('AC-001: Kanban toggle renders six phase columns with correct cards', ...)`.
  - **AC-001**: mock `/api/features` with one feature per phase; `page.goto('/')`; click `view-toggle-kanban`; assert each `kanban-column-<phase>` header text matches `PHASE_LABELS`; assert `kanban-column-planning` contains a `[data-testid^="feature-card-"]` for the planning feature.
  - **AC-002**: with AC-001 fixture, click `feature-card-<id>`; assert `page.url()` ends with `/features/<id>`; assert no full-document navigation (`page.on('framenavigated')` only the client-side route).
  - **AC-003**: with board visible, click `view-toggle-list`; assert `feature-list` visible and `kanban-column-*` count is 0.
  - **AC-004**: with board visible, set up a request counter on `/api/features`; toggle list↔kanban twice; assert exactly one `/api/features` request after initial load.
  - **AC-005**: mock `/api/features` to never resolve (hang); toggle to kanban; assert `features-loading` visible and `kanban-column-*` count 0.
  - **AC-006**: mock `/api/features` → 500; toggle to kanban; assert `features-error` visible and `kanban-column-*` count 0.
  - **AC-007**: mock `/api/features` → `{features:[],total_count:0}`; toggle to kanban; assert six `kanban-column-*` and `empty-state-create-button` visible.
  - **AC-008**: click `view-toggle-kanban`; `page.reload()`; assert `kanban-column-inception` visible and `await page.evaluate(() => localStorage.getItem('devteam.dashboard.view'))` === `'kanban'`.
  - **AC-009**: `page.evaluate(() => localStorage.clear())`; `page.goto('/')`; assert `feature-list` visible and `kanban-column-*` count 0.
  - **AC-010**: `page.addInitScript(() => { const s = localStorage.setItem; localStorage.setItem = () => { throw new Error('blocked'); }; })`; load; click `view-toggle-kanban`; assert `kanban-column-*` appear and no `pageerror` event fired.
  - **AC-011**: `page.addInitScript(() => { localStorage.getItem = () => { throw new Error('blocked'); }; })`; `page.goto('/')`; assert `feature-list` visible and no `pageerror`.
  - **AC-012**: mock `/api/features` with a feature `pending_questions_count: 3`; toggle to kanban; assert `question-badge` text is `3`.
  - **AC-013**: mock with `gate_result: { phase: 'planning', passed: false, checks: [] }`; toggle to kanban; assert `feature-card-gate` present and conveys failure (text or class matching `FeatureCard`).
  - **AC-014**: mock with `gate_result.passed: true`; assert `feature-card-gate` present and conveys success.
  - **AC-015**: mock with `gate_result: null`; assert no `feature-card-gate` inside that card.
  - **AC-016**: mock with a feature `current_phase: 'rolling_out'`; toggle to kanban; assert `kanban-column-other` exists, contains the card, and six standard columns also exist.
  - **AC-017**: mock all features with known phases; assert `kanban-column-other` count 0.
  - **AC-018**: mock 6 features (one per phase); assert each column has exactly one card and total card count is 6.
  - **AC-019**: mock 50 features; assert `[data-testid^="feature-card-"]` count is 50 and no console errors.
  - **Constraints addressed**: CON-002 (uses `:18765` via config baseURL, no port literal), CON-006 (test titles name AC-IDs), CON-008 (selectors use `data-testid`), all CONs verified end-to-end.
  - **Done conditions**:
    - `cd ui && npm run test:e2e -- kanban.spec.ts` passes all 19 tests.
    - `grep -n "8765" ui/e2e/kanban.spec.ts` returns 0 (CON-003).
    - `grep -nE "'(Inception|Planning|Construction|Review|Testing|Delivery)'" ui/e2e/kanban.spec.ts` returns 0 (assert against `PHASE_LABELS` via the rendered header text, not hardcoded literals — or if literal strings appear in assertions, they're test expectations, not source-of-truth labels; prefer importing nothing and asserting against the known label text since the test is verifying the label renders correctly). **Architect note**: a test asserting the header shows "Inception" is *verifying* `PHASE_LABELS` rendered correctly — the literal in the test is the expected value, not a duplicated source label. This is acceptable. CON-004's grep targets the *component*, not the test.
    - Existing `ui/e2e/app.spec.ts` passes unmodified.
  - **Test level**: e2e (covers all ACs including the ones acceptance.md labeled unit/integration — see `research.md` "Test runner decision").
  - **Agent failure mode checks**:
    - [ ] Each test captures `page.on('console', ...)` and `page.on('pageerror', ...)` where the AC demands no errors.
    - [ ] `page.route` mocks restore between tests (Playwright creates a fresh page per test by default — verify no cross-test leakage).
    - [ ] `page.addInitScript` for `localStorage` overrides runs before app load.
    - [ ] No `8765` literal — use `page.goto('/')` and the config baseURL.

**Checkpoint**: All 19 ACs verified. `npm run test:e2e` green.

---

## Phase 6: Final Gate

**Purpose**: Confirm the whole feature ships cleanly.

- [ ] T-FINAL [SHARED] Run full quality gate
  - **No file changes** — verification only.
  - **Done conditions**:
    - `cd ui && npm run lint` succeeds (0 errors).
    - `cd ui && npm run build` succeeds (TypeScript + Vite).
    - `cd ui && npm run test:e2e` succeeds — both `app.spec.ts` (unmodified) and `kanban.spec.ts` pass.
    - `git diff --stat ui/package.json` is empty (CON-007 — zero new deps).
    - `git diff --stat ui/playwright.config.ts` is empty (CON-002 — config unchanged).
    - `grep -rn "8765" ui/src/components/KanbanBoard.tsx ui/src/pages/Dashboard.tsx ui/e2e/kanban.spec.ts` returns 0 (CON-003).
    - `grep -nE "'(Inception|Planning|Construction|Review|Testing|Delivery)'" ui/src/components/KanbanBoard.tsx` returns 0 (CON-004).
    - Manual smoke: `npm run dev`, toggle Kanban/List, click a card, reload — all work.
  - **Constraints addressed**: CON-001, CON-002, CON-003, CON-004, CON-007 — all verified.
  - **Test level**: smoke + e2e + conformance (greps + diff).
  - **Agent failure mode checks**: final review against all per-task checks.

---

## Dependencies & Execution Order

```
T001 (KanbanBoard) ──► T002 (Dashboard wiring) ──► T005 (kanban.spec.ts) ──► T-FINAL
                          │
                          ├── US2 (AC-008..011) verified by T005 — no separate impl
                          └── US3 (AC-012..015) verified by T005 — no separate impl
```

- **T001** blocks T002 (Dashboard imports `KanbanBoard`).
- **T002** blocks T005 (spec exercises the toggle + board end-to-end).
- **T003 (US2) and T004 (US3)** are **not separate implementation tasks** — their acceptance criteria are satisfied by T001 + T002 and verified by T005. They are listed for traceability only.
- **T-FINAL** blocks delivery; runs after T005 is green.

### Parallel opportunities

- None meaningful at implementation time: T001 → T002 → T005 is a strict chain (different files, but each depends on the previous).
- T005 *drafting* (writing test skeletons) can begin in parallel with T002 once T001 fixes the `data-testid` contract — but the tests only pass after T002 lands. Not worth the merge-conflict risk for a 3-file feature; do them sequentially.

## Implementation strategy

**MVP first (US1 only)**: Complete T001 + T002, manually verify the board toggles and navigates. This delivers the entire P1 value. Then T005 verifies US1 + US2 + US3 + edge cases in one test file. T-FINAL gates.

**Why US2 and US3 have no separate impl tasks**: ponytail/minimum-diff. US2 (persistence) is ~8 lines of try/catch around `localStorage` inside T002 — a separate task would be shorter than its props. US3 (card density) is satisfied by `FeatureCard` reuse in T001 — a separate `KanbanCard` component was rejected in `research.md`. Both stories' acceptance criteria are fully covered by `kanban.spec.ts` (T005).

## Notes

- Every task names exact file paths.
- Every task references the constraints it addresses (or justifies having none — T003/T004 justify "no new code" because the implementation is folded into T001/T002).
- Every task specifies a test level.
- Done conditions are specific verifiable assertions (grep commands, Playwright tests, build success).
- Agent failure mode checks are listed per task where code is produced (T001, T002, T005).
- `kanban.spec.ts` is the single source of proof-of-work for the Testing phase (CON-006).