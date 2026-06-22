# Tasks: kanban-view

**Input**: Design documents from `specs/kanban-view/` (plan.md, data-model.md, contracts/components.md, research.md)

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: This feature's acceptance criteria require e2e (Playwright) + one unit test (AC-018). Tests ARE required — acceptance.md specifies test levels. Test tasks included.

**Organization**: Tasks grouped by user story priority. P1 stories (US-1, US-2, US-3) form the MVP; P2 (US-4, US-5) are polish; P3 (US-6) is the empty-board edge case. Foundational tasks (types, helper, vitest setup) come first because all stories depend on them.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1..US6, or FOUND for foundational)
- Exact file paths in descriptions

## Path Conventions

Web app: `ui/src/`, `ui/e2e/`. All paths below relative to repo root.

---

## Phase 1: Foundational (Blocking Prerequisites)

**Purpose**: Shared types, the pure sort helper + its unit test, and the vitest devDependency. No user story is implementable until these exist.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T001 [FOUND] Add `ViewMode` type and `DASHBOARD_VIEW_STORAGE_KEY` constant to `ui/src/types/index.ts` (additive — do not modify existing exports).
  - Add: `export type ViewMode = 'list' | 'kanban';`
  - Add: `export const DASHBOARD_VIEW_STORAGE_KEY = 'devteam-dashboard-view';`
  - **Constraints addressed**: CON-008 (no new runtime dep — type/key only)
  - **Done when**: `cd ui && npm run build` exits 0; `ViewMode` and `DASHBOARD_VIEW_STORAGE_KEY` importable from `../types`.
  - **Test level**: smoke (build passes)
  - **Agent failure mode check**: additive export — do not reorder or rename existing exports (would break `FeatureCard`/`FeatureList` imports).

- [ ] T002 [FOUND] Add `vitest` as a devDependency in `ui/package.json` and create `ui/vitest.config.ts` (minimal — reuses vite config) OR inline config in package.json.
  - Run `cd ui && npm install -D vitest`
  - Add `"test:unit": "vitest run"` script to `package.json`
  - **Constraints addressed**: CON-008 (devDependency only — runtime deps unchanged), CON-001 (build/lint commands unchanged)
  - **Done when**: `cd ui && npx vitest run` exits 0 with no tests (or a placeholder); `git diff main -- ui/package.json` shows `vitest` only under `devDependencies`; runtime `dependencies` block unchanged.
  - **Test level**: smoke
  - **Agent failure mode check**: do NOT add vitest to `dependencies` (runtime) — must be `devDependencies`. Verify with grep before committing.

- [ ] T003 [FOUND] [P] Implement `orderCards` helper and export from `ui/src/components/KanbanBoard.tsx` (create the file with the helper first; component body added in T006).
  - Export: `export function orderCards(features: FeatureSummary[]): FeatureSummary[]`
  - Logic: return `[...features].sort((a, b) => { if (a.priority !== b.priority) return a.priority - b.priority; return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime(); })`
  - **Constraints addressed**: FR-012, AC-016, AC-017, AC-018
  - **Done when**: `cd ui && npm run build` exits 0; helper importable.
  - **Test level**: unit (next task)
  - **Agent failure mode check**: must NOT mutate input — use spread copy. Must use `getTime()` for date compare (string compare of ISO dates is lexically correct for UTC but `getTime()` is unambiguous and matches `FeatureList` pattern).

- [ ] T004 [FOUND] Write unit test for `orderCards` at `ui/src/components/__tests__/orderCards.test.ts` (or `ui/src/__tests__/orderCards.test.ts` — follow whatever vitest picks up).
  - Import `orderCards` from `../KanbanBoard`
  - Fixture: `[{id:'a',priority:2,updated_at:'2026-01-01T00:00:00Z',...}, {id:'b',priority:1,...}, {id:'c',priority:2,updated_at:'2026-06-01T00:00:00Z',...}]`
  - Assert order: `[b (P1), c (P2 newer), a (P2 older)]`
  - Assert input array not mutated (deep-equal before/after)
  - **Constraints addressed**: AC-018, CON-007 (named file + assertions)
  - **Done when**: `cd ui && npx vitest run` passes; test names the file and assertions (AC-018 traced to `orderCards.test.ts`).
  - **Test level**: unit
  - **Agent failure mode check**: do not test implementation details (sort comparator internals) — test observable order + immutability only.

**Checkpoint**: Foundation ready — types, helper, unit test green, vitest installed. User story implementation can begin.

---

## Phase 2: User Story 1 — Toggle Between List and Kanban Views (Priority: P1) 🎯 MVP

**Goal**: User can switch between list and Kanban views; choice persists in localStorage.

**Independent Test**: Load Dashboard, click Kanban, confirm board renders, click List, confirm grid returns, reload, confirm persistence.

### Implementation for User Story 1

- [ ] T005 [US1] Implement `ViewToggle` component at `ui/src/components/ViewToggle.tsx`.
  - Props: `{ view: ViewMode; onChange: (v: ViewMode) => void }`
  - Render: container `div[data-testid="view-toggle"][role="group"][aria-label="Dashboard view"]` with two `<button>`s: `view-toggle-list` and `view-toggle-kanban`, each with `aria-pressed={view === 'list'/'kanban'}` and active/inactive Tailwind classes (mirror `FeatureList` sort-button style).
  - **Constraints addressed**: FR-001, FR-005 (default handled by parent), AC-001, AC-002, AC-005, CON-006
  - **Done when**: `npm run build` + `npm run lint` pass; component renders both buttons with correct testids and `aria-pressed` reflecting the `view` prop.
  - **Test level**: e2e (via Dashboard in T007)
  - **Agent failure mode check**: `aria-pressed` must be a boolean expression (`view === 'list'`), not a string literal. Container must have `role="group"`.

- [ ] T006 [US1] Modify `ui/src/pages/Dashboard.tsx` to add view state + localStorage persistence + conditional rendering.
  - Add `useState` initialized lazily: `useState<ViewMode>(() => { try { const v = localStorage.getItem(DASHBOARD_VIEW_STORAGE_KEY); return v === 'kanban' ? 'kanban' : 'list'; } catch { return 'list'; } })`
  - Add `useEffect` watching `view`: `localStorage.setItem(DASHBOARD_VIEW_STORAGE_KEY, view)` (wrap in try/catch)
  - In the non-empty success branch (`!isLoading && !error && features.length > 0`): render `<ViewToggle view={view} onChange={setView} />` above the view body; then render `<FeatureList>` if `view === 'list'` else `<KanbanBoard features={features} />` (import `KanbanBoard` — it will be created in T008; for T006 alone, temporarily render a placeholder `<div data-testid="kanban-board">TODO</div>` and replace in T008, OR implement T008 first. **Recommended order: T008 before T006's final wiring** — see dependency note below).
  - Loading/error/empty branches UNCHANGED — they render before the view branch and do not mount the toggle or board (FR-013, FR-014, FR-015, FR-016, AC-009, AC-010, AC-019, AC-020).
  - **Constraints addressed**: FR-001, FR-003, FR-004, FR-005, FR-013, FR-014, FR-015, FR-016, AC-001, AC-002, AC-003, AC-004, AC-009, AC-010, AC-019, AC-020, CON-010
  - **Done when**: `npm run build` passes; loading Dashboard with no localStorage key shows `feature-list` (AC-004); clicking `view-toggle-kanban` shows `kanban-board` and hides `feature-list` (AC-001); clicking `view-toggle-list` reverses (AC-002); reload after selecting kanban shows kanban (AC-003); `/api/features` delayed → `features-loading` visible, `kanban-board` absent (AC-009); `/api/features` 500 → `features-error` visible, `kanban-board` absent (AC-010); zero features → `empty-state` visible, toggle absent (AC-019, AC-020).
  - **Test level**: e2e (T007)
  - **Agent failure mode check**: lazy `useState` initializer reads localStorage ONCE — do not read in the render body (causes repeated reads). Effect writes on every `view` change — do not gate with a ref (YAGNI). Loading branch MUST come before the view branch in JSX (CON-010). Toggle mounted ONLY in the non-empty branch.

**Dependency note**: T006 imports `KanbanBoard` (T008). Implement T008 (or at minimum the `KanbanBoard` shell with `data-testid="kanban-board"`) before T006's final build. Order within Phase: T005 (ViewToggle) and T008 (KanbanBoard shell) can be done in parallel [P]; T006 (Dashboard wiring) depends on both.

---

## Phase 3: User Story 2 — View Features as Cards in Phase Columns (Priority: P1) 🎯 MVP

**Goal**: Kanban board with six phase columns, cards grouped by `current_phase`, headers with counts.

**Independent Test**: Seed features in multiple phases, load Kanban, verify each feature in the column matching its phase, counts match.

### Implementation for User Story 2

- [ ] T007 [P] [US2] Implement `KanbanColumn` component at `ui/src/components/KanbanColumn.tsx`.
  - Props: `{ phase: PhaseName | 'other'; label: string; features: FeatureSummary[] }`
  - Render: `div[data-testid=kanban-column-${phase}]` containing:
    - header `div[data-testid=kanban-column-header-${phase}]` with `<span>{label}</span>` and `<span data-testid=kanban-column-count-${phase}>({features.length})</span>`; `sticky top-0` + bg classes for visibility during scroll.
    - body: if `features.length === 0` → `<div data-testid="kanban-column-empty-state">No features in {label}</div>`; else `features.map(f => <FeatureCard key={f.id} feature={f} />)`.
  - **Constraints addressed**: FR-006, FR-008, FR-010, FR-017, FR-018, AC-006, AC-007, AC-008, AC-011, CON-003, CON-005, CON-006
  - **Done when**: `npm run build` passes; `import FeatureCard from './FeatureCard'` present (CON-003); no hardcoded phase-name string literal used as header text (CON-005 — `label` comes from prop); empty column renders `kanban-column-empty-state` with non-empty text (AC-011).
  - **Test level**: e2e (T010)
  - **Agent failure mode check**: do NOT inline card JSX (title/status/priority badges) — must use `<FeatureCard>`. Empty state must be a real element with text, not `null` or hidden. `key={f.id}` on cards.

- [ ] T008 [US2] Implement `KanbanBoard` component body at `ui/src/components/KanbanBoard.tsx` (helper exported in T003; now add the component).
  - Props: `{ features: FeatureSummary[] }`
  - Build columns: `const known = PHASES.map(phase => ({ phase, label: PHASE_LABELS[phase], features: orderCards(features.filter(f => f.current_phase === phase)) }))`; `const others = features.filter(f => !(PHASES as readonly string[]).includes(f.current_phase))`; if `others.length > 0` append `{ phase: 'other', label: 'Other', features: orderCards(others) }`.
  - Render: `div[data-testid=kanban-board]` with `className="flex gap-4 overflow-x-auto ..."`; map columns to `<KanbanColumn key={col.phase} ...col} />`.
  - Each column gets a fixed `min-w` via KanbanColumn's own classes (so six columns overflow narrow viewports → FR-011).
  - **Constraints addressed**: FR-002, FR-007, FR-010, FR-011, FR-017, FR-018, AC-006, AC-011, AC-014, AC-015, AC-CON-004, CON-004, CON-006
  - **Done when**: `npm run build` passes; board renders six `kanban-column-{phase}` in `PHASES` order (AC-CON-004); `Σ column.count === features.length`; `other` column appears only when unknown-phase features exist; `overflow-x-auto` present for scroll (AC-014); `PHASES` and `PHASE_LABELS` imported (CON-004, CON-005).
  - **Test level**: e2e (T010)
  - **Agent failure mode check**: map over `PHASES` (imported) — do not hardcode `['inception','planning',...]`. `(PHASES as readonly string[]).includes(...)` for the unknown-phase filter (TS: `PHASES` is `readonly [...]`, `.includes` needs the cast or a type guard). Do NOT mutate `features` — `filter` + `orderCards` (which copies) are safe. `key={col.phase}` on columns.

**Checkpoint**: US-1 + US-2 complete → MVP functional. User can toggle to Kanban and see features grouped by phase. (US-3 click navigation works automatically because `FeatureCard` is reused — verify in e2e.)

---

## Phase 4: User Story 3 — Click a Card to Navigate to Feature Detail (Priority: P1)

**Goal**: Clicking a Kanban card navigates to `/features/{id}`, same as list view.

**Independent Test**: Load Kanban, click a card, confirm URL is `/features/{id}` and detail page renders.

### Implementation for User Story 3

- [ ] T009 [US3] **No new implementation task** — US-3 is satisfied by reusing `FeatureCard` (whose root is a `<Link to={/features/${id}}>`). Verify via e2e.
  - **Constraints addressed**: FR-009, AC-012, AC-013, CON-003, SC-004
  - **Justification for no implementation**: `KanbanColumn` (T007) renders `<FeatureCard feature={f} />` — the exact same component `FeatureList` uses. The Link behavior is inherited. No new code needed. If e2e (T010) reveals navigation is blocked (e.g., a parent element swallows clicks), a fix task is added then.
  - **Test level**: e2e (T010)
  - **Agent failure mode check**: ensure `KanbanColumn` does not wrap `FeatureCard` in an extra `<a>` or `<button>` (would create nested interactive elements or hijack clicks). The card's `Link` is the click target.

**Checkpoint**: US-1, US-2, US-3 all complete → full P1 MVP. User can toggle, view board, click cards to detail.

---

## Phase 5: User Story 4 — Horizontal Scroll for Six Columns (Priority: P2)

**Goal**: On narrow viewports the board scrolls horizontally; headers stay aligned with columns.

**Independent Test**: Set viewport 800x600, load Kanban, scroll right, verify last column visible and header aligned.

### Implementation for User Story 4

- [ ] T010 [US4] **No new implementation task** — US-4 is satisfied by `KanbanBoard`'s `overflow-x-auto` (T008) + `KanbanColumn`'s fixed `min-w` (T007) + `sticky top-0` headers (T007). Verify via e2e.
  - **Constraints addressed**: FR-011, AC-014, AC-015
  - **Justification**: horizontal scroll is a CSS concern, already specified in T007/T008 classes. If e2e (T011) reveals scroll doesn't trigger (columns too narrow / flex shrinking), adjust `min-w` in `KanbanColumn` — that's a tweak to T007, not a new task.
  - **Test level**: e2e (T011)
  - **Agent failure mode check**: columns must NOT shrink to fit (`flex-shrink-0` or fixed `min-w`); otherwise `overflow-x-auto` never triggers. Verify in T011.

---

## Phase 6: User Story 5 — Card Ordering Within Columns (Priority: P2)

**Goal**: Within a column, P1 before P2 before P3; ties broken by most-recently-updated first.

**Independent Test**: Seed three features same phase, priorities 1/2/3, verify vertical order. Seed two same-priority features, verify newer on top.

### Implementation for User Story 5

- [ ] T011 [US5] **No new implementation task** — US-5 is satisfied by `orderCards` (T003) applied per-column in `KanbanBoard` (T008). Unit test (T004) + e2e (T011's test) verify.
  - **Constraints addressed**: FR-012, AC-016, AC-017, AC-018
  - **Justification**: ordering logic implemented in T003, tested in T004 (unit), verified in e2e. No additional code.
  - **Test level**: unit (T004) + e2e (T011 test)

---

## Phase 7: User Story 6 — Empty Board State (Priority: P3)

**Goal**: Zero features → existing `EmptyState`, toggle hidden.

**Independent Test**: Empty repo, load Dashboard, verify `empty-state` visible, toggle absent.

### Implementation for User Story 6

- [ ] T012 [US6] **No new implementation task** — US-6 is satisfied by Dashboard's existing empty branch (`features.length === 0` → `<EmptyState>`), which renders BEFORE the toggle-bearing branch (T006). The toggle is mounted only in the `features.length > 0` branch.
  - **Constraints addressed**: FR-015, FR-016, AC-019, AC-020
  - **Justification**: T006's design mounts `ViewToggle` only inside the non-empty success branch. Empty state branch is unchanged. No new code.
  - **Test level**: e2e (T011)
  - **Agent failure mode check**: verify T006 did not accidentally render the toggle outside the non-empty branch (would show toggle on empty state — AC-020 failure).

---

## Phase 8: E2E Test Suite (covers all user stories)

**Purpose**: Playwright e2e specs tracing every AC (except AC-018, covered by unit in T004) to a named assertion.

- [ ] T013 [P] [US1-US6] Write `ui/e2e/kanban.spec.ts` covering AC-001 through AC-020 (except AC-018) + AC-CON-001 through AC-CON-010.
  - `describe('kanban-view', () => { ... })` with `it`/`test` blocks mapped:
    - AC-001: click `view-toggle-kanban` → `kanban-board` visible, `feature-list` hidden
    - AC-002: click `view-toggle-list` → reverse
    - AC-003: click kanban, `localStorage.getItem("devteam-dashboard-view")` === `"kanban"`, `page.reload()`, board still visible
    - AC-004: `localStorage.clear()`, load, `feature-list` visible, `kanban-board` hidden
    - AC-005: both toggle testids present; active has `aria-pressed="true"`
    - AC-006: for each feature (read via list view first), assert its card inside `kanban-column-{current_phase}`; no card in two columns
    - AC-007: for each of six column testids, header text matches `/^{Label}\s*\(\d+\)$/`
    - AC-008: sample feature — read `feature-card-title`/`feature-card-priority`/`feature-card-status` in list, switch to kanban, assert identical
    - AC-009: `page.route('**/api/features', async route => { await new Promise(r => setTimeout(r, 5000)); await route.continue(); })` — load with view=kanban, assert `features-loading` visible, `kanban-board` absent
    - AC-010: `page.route('**/api/features', route => route.fulfill({ status: 500, json: { error: 'x' } }))` — assert `features-error` visible, `kanban-board` absent
    - AC-011: seed/identify a phase with zero features, assert column visible + count `(0)` + `kanban-column-empty-state` visible with text
    - AC-012: click `feature-card-{id}` in kanban, assert `page.url()` matches `/features/{id}`, detail page visible
    - AC-013: seed feature with `pending_questions_count > 0`, click card, assert navigation
    - AC-014: `page.setViewportSize({width:800,height:600})`, load kanban, assert board `scrollWidth > clientWidth`, scroll right, assert `kanban-column-delivery` in viewport
    - AC-015: at narrow width, for a sample column record header `boundingBox().x` and first card `boundingBox().x`, scroll, re-read, assert x-difference constant (±1px)
    - AC-016: seed 3 features same phase priorities 1/2/3, read `feature-card-priority` + `boundingBox().y` per card in that column, assert y-order P1<P2<P3
    - AC-017: seed 2 features same phase same priority different `updated_at`, assert newer (smaller y) above older
    - AC-019: seed zero features (or skip if workspace has features — follow `app.spec.ts` empty-state pattern), assert `empty-state` visible, `kanban-board` absent
    - AC-020: zero features → neither `view-toggle-list` nor `view-toggle-kanban` visible
    - AC-CON-001: (separate) run `npm run build` + `npm run lint`, assert exit 0 — or assert in test via a build-smoke spec; recommend a separate `describe` with a single `test` that shells out, OR document as a manual gate. **Recommended: document in test-report.md as a smoke gate run before e2e, not a Playwright test** — Playwright shouldn't shell out to builds.
    - AC-CON-002: assert no test in this file hardcodes `8765`; config `baseURL` used (implicit — all `page.goto('/')` use config)
    - AC-CON-004: read all `kanban-column-*` testids in DOM order, assert `[inception, planning, construction, review, testing, delivery]` (other column, if present, is last and not in this assertion)
    - AC-CON-006: all selectors in this file are `[data-testid=...]` — enforce by code inspection (Reviewer)
  - **Constraints addressed**: CON-002, CON-006, CON-007, all ACs
  - **Done when**: `cd ui && npx playwright test kanban.spec.ts --reporter=line` passes on port 18765; every AC-NNN (except AC-018) maps to a named `test` block; test-report.md (Tester phase) traces each AC to file:line.
  - **Test level**: e2e
  - **Agent failure mode check**: use `page.route` for loading/error injection (existing pattern in `app.spec.ts`). Use `page.setViewportSize` for scroll tests. For tests requiring specific feature seeds (AC-016/017), if the live workspace can't be seeded, document the test as conditional-skip (like `app.spec.ts` empty-state pattern) AND add a note for the Tester to verify via API state inspection. Do NOT hardcode feature IDs — read them from the list view first.

**Checkpoint**: All user stories + e2e suite complete. Full feature verified.

---

## Phase 9: Polish & Cross-Cutting

- [ ] T014 [P] Verify `git diff main -- ui/package.json ui/package-lock.json` shows only `vitest` under `devDependencies`. (AC-CON-008)
  - **Done when**: diff confirms no runtime dep additions.
  - **Test level**: smoke

- [ ] T015 [P] Verify `grep -rn "FeatureCard" ui/src/components/KanbanColumn.tsx` shows import + usage; no duplicated card JSX in `KanbanColumn.tsx` or `KanbanBoard.tsx`. (AC-CON-003)
  - **Done when**: grep confirms `<FeatureCard` usage; no `feature-card-title`/`feature-card-status` literal JSX in new files.
  - **Test level**: unit (static)

- [ ] T016 [P] Verify `grep -rn "PHASE_LABELS" ui/src/components/KanbanBoard.tsx` shows import; no hardcoded "Inception"/"Planning"/etc. string literals as headers in any new file. (AC-CON-005)
  - **Done when**: grep confirms import; no header string literals.
  - **Test level**: unit (static)

- [ ] T017 Run quickstart validation: `cd ui && npm run build && npm run lint && npx vitest run && npx playwright test kanban.spec.ts --reporter=line`. All green. (Final gate)
  - **Done when**: all four commands exit 0.
  - **Test level**: smoke + unit + e2e

---

## Dependencies & Execution Order

### Phase Dependencies
- **Phase 1 (Foundational, T001-T004)**: No dependencies — start immediately. BLOCKS all user stories.
- **Phase 2 (US-1, T005-T006)**: Depends on Phase 1. T006 depends on T005 + T008 (KanbanBoard shell).
- **Phase 3 (US-2, T007-T008)**: Depends on Phase 1. T007 and T008 can start in parallel [P] with T005. T006 (Dashboard wiring) depends on T008.
- **Phase 4 (US-3, T009)**: No implementation — satisfied by T007. Verified in T013.
- **Phase 5 (US-4, T010)**: No implementation — satisfied by T007/T008 classes. Verified in T013.
- **Phase 6 (US-5, T011)**: No implementation — satisfied by T003/T004/T008. Verified in T013.
- **Phase 7 (US-6, T012)**: No implementation — satisfied by T006's branch structure. Verified in T013.
- **Phase 8 (E2E, T013)**: Depends on T005-T008 complete.
- **Phase 9 (Polish, T014-T017)**: Depends on all above.

### Recommended execution order (sequential)
1. T001 (types) → T002 (vitest) → T003 (orderCards) → T004 (unit test)
2. T005 (ViewToggle) [P] and T007 (KanbanColumn) [P] and T008 (KanbanBoard body) — parallel
3. T006 (Dashboard wiring) — after T005 + T008
4. T013 (e2e suite) — after T006
5. T014-T017 (polish/gate) — after T013

### Parallel Opportunities
- T005, T007, T008 are [P] (different files, no cross-deps except T008 imports `KanbanColumn` from T007 — so T008 starts after T007's interface is defined, but both can be drafted together).
- T014, T015, T016 are [P] (independent greps).

### Within Each User Story
- (N/A for US-3/4/5/6 — no implementation tasks; verification only.)

---

## Implementation Strategy

### MVP First (US-1 + US-2 + US-3)
1. Phase 1 (Foundation): types, vitest, orderCards + unit test.
2. T005 + T007 + T008 (components).
3. T006 (Dashboard wiring) → MVP: toggle + board + click navigation.
4. T013 (e2e) → verify MVP.

### Incremental Delivery
- MVP (P1) ships toggle + board + navigation.
- P2 (US-4 scroll, US-5 ordering) already baked into MVP implementation — verified in e2e.
- P3 (US-6 empty state) already baked into Dashboard branch structure — verified in e2e.

---

## Notes

- [P] tasks = different files, no dependencies.
- [Story] label maps task to user story.
- Every task lists constraints addressed (CON-NNN) and done conditions with verifiable assertions.
- Every task specifies test level (smoke/unit/e2e).
- Agent failure mode checks specified per task.
- Tasks T009/T010/T011/T012 are "no new implementation" tasks — they exist to make the spec→task→test trace explicit. They justify why no code is needed and point to the e2e verification.
- Commit after each task or logical group.