# Tasks: kanban-view

**Input**: Design documents from `specs/kanban-view/` (plan.md, research.md, data-model.md, contracts/)

**Prerequisites**: spec.md, acceptance.md, repos.yaml (all present, inception gate passed)

**Organization**: Tasks grouped by user story priority. P1 (US-001 toggle, US-002 cards) first, then P2 (US-003 empty states), then P3 (US-004 overflow). A shared-infra phase precedes the user stories (the `badgeStyles` extraction is a refactor that both list and board depend on, and the pure grouping function + hook are leaf modules every later task consumes).

**Test level legend**: `smoke` = build/lint/start; `e2e` = Playwright browser; `unit` = pure-logic Vitest-style; `integration` = network/request assertion.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependencies)
- **[Story]**: US1, US2, US3, US4, or `SHARED` for cross-story infrastructure

## Path Conventions

Single repo, web app. UI source under `ui/src/`; e2e under `ui/e2e/`. Paths below are relative to repo root.

---

## Phase 1: Shared Infrastructure (blocking prerequisite for all user stories)

**Purpose**: Extract shared badge styles (so the board card and list card stay in parity — CON-006), build the pure grouping function (CON-009, AC-011), and build the session-storage hook (FR-002, FR-003). These are leaf modules with no UI dependencies.

**⚠️ CRITICAL**: US-001..US-004 all depend on T002 and T003. T001 must land first because both `FeatureCard` (existing) and `KanbanCard` (T004) import from `badgeStyles.ts`.

### [ ] T001 [SHARED] Extract shared badge color map into `ui/src/components/badgeStyles.ts`

**Files**:
- `ui/src/components/badgeStyles.ts` — [CREATE] export `statusColors: Record<string, string>` (the map currently module-local in `FeatureCard.tsx`).
- `ui/src/components/FeatureCard.tsx` — [MODIFY] remove the module-local `statusColors` const; `import { statusColors } from './badgeStyles'`. No other change to `FeatureCard`.

**Constraints**: CON-006 (card chrome parity — both consumers import the same map).

**Done conditions**:
- `ui/src/components/badgeStyles.ts` exists and exports `statusColors` with the exact same key→class mapping as the current `FeatureCard.tsx` const.
- `FeatureCard.tsx` imports `statusColors` from `./badgeStyles` and contains no module-local `statusColors` definition (grep `const statusColors` in `FeatureCard.tsx` → 0 matches).
- `npm run build` succeeds (tsc + vite).
- `npm run lint` succeeds.
- `npm run test:e2e app.spec.ts` green — the refactor did not change any rendered class on the list view (CON-004 regression check).

**Test level**: smoke (build + lint + existing e2e green).

**Agent failure mode checks**:
- [ ] Multi-component consistency: after extraction, BOTH `FeatureCard` and (later) `KanbanCard` must import from `badgeStyles.ts`. The module-local const must be removed, not left as dead code.
- [ ] No behavior change: the exported map must be byte-identical to the existing const (same keys, same Tailwind class strings, same `dark:` variants).

**Dependencies**: none — first task.

---

### [ ] T002 [SHARED] [P] Create `groupFeaturesByPhase` pure function + unit test

**Files**:
- `ui/src/lib/groupFeaturesByPhase.ts` — [CREATE] pure function. Signature: `(features: FeatureSummary[]) => Array<{ phase: PhaseName | 'other'; label: string; features: FeatureSummary[] }>`. Returns 6 known buckets in `PHASES` order (each always present, `features: []` when empty) plus an `other` bucket **only** when at least one feature has a `current_phase` not in `PHASES` (or undefined/null). `label` = `PHASE_LABELS[phase]` for known phases, `"Other"` for the `other` bucket.
- `ui/src/lib/groupFeaturesByPhase.test.ts` — [CREATE] unit test covering: (a) empty input → 6 known buckets, no `other`; (b) all-known-phases → correct buckets, no `other`, total count preserved; (c) unknown `current_phase` → `other` bucket populated, known buckets intact; (d) `current_phase` undefined → `other` bucket; (e) order preserved within buckets; (f) invariant `sum(out.features.length) === in.length` for every case.

**Constraints**: CON-009 (unknown enum defensive — no throw), CON-005 (reuse `PHASES`/`PHASE_LABELS`, no literals).

**Done conditions**:
- `groupFeaturesByPhase([], [])` returns an array of length 6, each `features: []`, no `other` entry.
- `groupFeaturesByPhase([{current_phase:'weird', ...}])` returns 7 entries; the 7th is `{phase:'other', label:'Other', features:[<the feature>]}` (AC-011, CON-009).
- `groupFeaturesByPhase([{current_phase:undefined, ...}])` → `other` bucket contains it (defensive — no throw).
- For every test case: `sum(result.map(c => c.features.length)) === input.length` (SC-002 invariant).
- Function never throws on any input shape (unknown/undefined/null `current_phase`, missing fields).
- No phase/status string literals in the function — only `PHASE_LABELS` references (CON-005 grep).
- Unit test passes: `npx vitest run ui/src/lib/groupFeaturesByPhase.test.ts` (or `npm test` if a vitest script exists; if no vitest is installed, add a minimal `vitest` devDependency — **check first**: the repo may use a different runner. If adding vitest, that is a devDependency, not a runtime dep, so CON-003 still holds. Prefer the existing test runner if one exists; otherwise co-locate the test as a `*.test.ts` and run via `npx vitest`).

**Test level**: unit.

**Agent failure mode checks**:
- [ ] Parsing/bucketing safety: all input shapes caught — unknown/undefined/null `current_phase` → `other`, never throws (CON-009). Unit test covers each.
- [ ] No silent drops: the invariant test catches any feature that falls through.

**Dependencies**: none — leaf module. **[P] with T003** (different files, no overlap).

---

### [ ] T003 [SHARED] [P] Create `useSessionView` hook + unit test

**Files**:
- `ui/src/hooks/useSessionView.ts` — [CREATE] hook. `() => [view: 'board'|'list', setView: (v) => void]`. On init, reads `sessionStorage.getItem('devteam.dashboard.view')`; if `'list'` returns `'list'`, else (absent/invalid/any other value) returns `'board'` (FR-003 default). `setView` writes `sessionStorage.setItem('devteam.dashboard.view', v)` then updates state. Wrap `sessionStorage` access in try/catch — fall back to in-memory state if `sessionStorage` is unavailable (SSR/restricted browser).
- `ui/src/hooks/useSessionView.test.ts` — [CREATE] unit test: (a) fresh `sessionStorage` → `'board'`; (b) stored `'list'` → `'list'`; (c) stored garbage → `'board'`; (d) `setView('list')` writes key and updates state; (e) `setView('board')` writes key; (f) `sessionStorage` throws on access → still returns `'board'`, no throw.

**Constraints**: FR-002 (sessionStorage key `devteam.dashboard.view`), FR-003 (default `'board'`).

**Done conditions**:
- Hook returns `'board'` when `sessionStorage` has no key (AC-005).
- Hook returns `'list'` when `sessionStorage` has `'list'` (AC-004 round-trip).
- `setView('list')` then re-init → `'list'` (persistence).
- Invalid stored value (e.g. `'garbage'`) → `'board'` (defensive).
- Hook does not throw when `sessionStorage` is unavailable.
- Unit test passes.

**Test level**: unit.

**Agent failure mode checks**:
- [ ] Nil pointer / init ordering: `useState` initializer reads `sessionStorage.getItem` which may throw in restricted environments — wrap in try/catch, fall back to `'board'`. Do NOT dereference before init.

**Dependencies**: none — leaf module. **[P] with T002**.

**Checkpoint**: T001/T002/T003 complete → shared infra ready. User-story implementation can begin.

---

## Phase 2: User Story 1 — Toggle Between List and Kanban Board (Priority: P1) 🎯 MVP

**Goal**: A user can switch between the existing list view and a new board view; choice persists for the session; Board is the default.

**Independent Test**: Load Dashboard, click Board → six columns render; click List → FeatureList renders; reload → Board still active.

### [ ] T004 [US1] Create `KanbanCard`, `KanbanColumn`, `KanbanBoard` components

**Files**:
- `ui/src/components/KanbanCard.tsx` — [CREATE] renders `<Link to={/features/${feature.id}}>` root with `data-testid="kanban-card-${feature.id}"`. Renders title, status badge (`statusColors` from `badgeStyles.ts` + `STATUS_LABELS`), priority badge (`PRIORITY_LABELS[feature.priority]`), `QuestionBadge` when `pending_questions_count > 0`, gate indicator (`✓ Gate passed`/`✗ Gate failed`) when `feature.gate_result` present, updated date. Applies `ring-2 ring-red-500 dark:ring-red-400` when `status==='gate_blocked'` (AC-012) and `ring-2 ring-yellow-500 dark:ring-yellow-400` when `status==='waiting_for_human'` (AC-013). Badge testids: `kanban-card-status`, `kanban-card-priority`, `kanban-card-gate`.
- `ui/src/components/KanbanColumn.tsx` — [CREATE] props `{ phase, label, features }`. Root `data-testid="kanban-column-${phase}"`. Header `data-testid="kanban-column-${phase}-header"` shows `label`. Body `data-testid="kanban-column-${phase}-body"` with `overflow-y-auto` and a `max-h` tied to viewport (FR-013). When `features.length === 0`, render `data-testid="kanban-column-empty-${phase}"` with muted text "No features" (FR-012, AC-017). Maps `features` to `KanbanCard`.
- `ui/src/components/KanbanBoard.tsx` — [CREATE] props `{ features: FeatureSummary[] }`. Root `data-testid="kanban-board"`. Calls `groupFeaturesByPhase(features)`, renders a `KanbanColumn` per result entry. Container: `flex gap-4 overflow-x-auto` + viewport-bounded height (FR-014, AC-021). Each column wrapper: `min-w-[240px] flex-1` (FR-015, AC-022).

**Constraints**: CON-005 (reuse label maps), CON-006 (badge parity via `badgeStyles`), FR-005/FR-006/FR-007/FR-008/FR-009/FR-010/FR-011/FR-012/FR-013/FR-014/FR-015.

**Done conditions**:
- `npm run build` succeeds (all three components type-check).
- `KanbanCard` root is a `react-router` `<Link to="/features/${id}">` (FR-010) — verify by grep `to={\`/features/` in `KanbanCard.tsx`.
- `KanbanCard` imports `statusColors` from `badgeStyles.ts` (CON-006) — verify by grep.
- No phase/status label literals in any of the three files — only `PHASE_LABELS`/`STATUS_LABELS`/`PRIORITY_LABELS` references (CON-005 grep).
- Every `dark:` color class has a light-mode companion (review).
- `KanbanBoard` renders exactly 6 `KanbanColumn` instances for an all-known-phases input, +1 `other` when an unknown phase exists (AC-019) — verified in T005 e2e.
- `KanbanCard` applies `ring-red-*` for `gate_blocked` and `ring-yellow-*` for `waiting_for_human` (AC-012, AC-013) — verified in T005 e2e.

**Test level**: smoke (build). E2E validation happens in T005 (Dashboard wiring).

**Agent failure mode checks**:
- [ ] Empty array safety: `features ?? []` at the bucket level — empty buckets render the placeholder, not crash on `.map`.
- [ ] Dark mode: every new `bg-`/`text-`/`ring-` class has a `dark:` companion.
- [ ] No second `useQuery` in any board component (CON-007) — board receives `features` via props only.
- [ ] testid uniqueness: `kanban-card-${id}` is unique per feature; `kanban-column-${phase}` is unique per column.

**Dependencies**: T001 (badgeStyles), T002 (groupFeaturesByPhase).

---

### [ ] T005 [US1] Create `ViewToggle`, wire Dashboard to toggle + board/list branching

**Files**:
- `ui/src/components/ViewToggle.tsx` — [CREATE] props `{ value: 'board'|'list'; onChange: (v) => void }`. Root `data-testid="view-toggle"`. Two buttons: `data-testid="view-toggle-list"[aria-pressed={value==='list'}]` and `data-testid="view-toggle-board"[aria-pressed={value==='board'}]`. Tailwind segmented-control styling with `dark:` variants.
- `ui/src/pages/Dashboard.tsx` — [MODIFY] add `const [view, setView] = useSessionView();` (import from `../hooks/useSessionView`). In the header row, render `<ViewToggle value={view} onChange={setView} />` **only when** `!isLoading && !error && features.length > 0` (FR-004, AC-006, AC-018). In the features-present branch (currently `<FeatureList features={features} />`), branch: `view === 'board' ? <KanbanBoard features={features} /> : <FeatureList features={features} />` (FR-001, FR-016). Loading/error/empty branches unchanged (FR-017).
- `ui/e2e/kanban.spec.ts` — [CREATE] Playwright spec with tests tracing AC-001..AC-019 (P1+P2). Cover: toggle visibility + default Board (AC-001, AC-005); click Board→columns (AC-002); click List→FeatureList (AC-003); reload persistence (AC-004); empty state hides toggle (AC-006, AC-018); card badges (AC-007); QuestionBadge (AC-008); gate indicator (AC-009); card click navigation (AC-010); gate_blocked red ring (AC-012); waiting_for_human yellow ring (AC-013); loading state (AC-014); error state (AC-015); single fetch (AC-016); empty column placeholder (AC-017); exactly 6 columns (AC-019). Use `page.route('**/api/features', ...)` to mock empty/loading/error/500 responses. For AC-016, attach `page.on('request')` BEFORE `page.goto('/')`.

**Constraints**: CON-001 (Playwright :18765), CON-007 (single fetch — AC-016), CON-004 (existing list view intact), FR-001/FR-002/FR-003/FR-004/FR-016/FR-017, all P1+P2 ACs.

**Done conditions**:
- `npm run build` + `npm run lint` succeed.
- `npm run test:e2e kanban.spec.ts` green — AC-001..AC-019 pass.
- AC-016: `page.on('request')` count for `/api/features` === 1 during a Board render (attach listener before `goto`).
- AC-006: when `/api/features` returns `{features:[], total_count:0}`, `[data-testid="view-toggle"]` count is 0 and `EmptyState` visible.
- AC-014: when `/api/features` route delays, `[data-testid="features-loading"]` visible and `[data-testid^="kanban-column-"]` count 0.
- AC-015: when `/api/features` returns 500, `[data-testid="features-error"]` visible and `[data-testid^="kanban-column-"]` count 0.
- No console errors in any Board-view test (SC-004) — extend the existing `page.on('console')` assertion pattern.
- `git diff ui/package.json` empty (CON-003).

**Test level**: smoke + e2e + integration (AC-016 is an integration-level request-count assertion).

**Agent failure mode checks**:
- [ ] AC-016 footgun: `page.on('request')` listener MUST be attached before `page.goto('/')` or the first request is missed.
- [ ] FR-004 guard: toggle hidden when `features.length === 0` — verify the `!isLoading && !error && features.length > 0` guard wraps the toggle render.
- [ ] State transitions: view state is `board`/`list` only; no invalid transition. Reload preserves via sessionStorage (AC-004).
- [ ] No second `useQuery` — the board/list branch is inside the existing features-present branch; the single `useQuery(['features'])` stays at the Dashboard root (CON-007).

**Dependencies**: T001, T002, T003, T004.

**Checkpoint**: P1 (US-001 + US-002) and P2 (US-003) functionally complete. Remaining: regression fix (T006) and P3 overflow tests (T007).

---

## Phase 3: User Story 1 (Regression Fix) — Keep existing list-view e2e green

**Purpose**: The default view flipped from List to Board (FR-003). Existing `app.spec.ts` tests that assert `feature-card-*` on `/` now fail because `feature-card-*` only renders in List view. Fix by clicking the List toggle first (or asserting `kanban-card-*` where appropriate). This is a **bounded regression fix**, not a spec change.

### [ ] T006 [US1] Fix `ui/e2e/app.spec.ts` for Board-default

**Files**:
- `ui/e2e/app.spec.ts` — [MODIFY] For every test that asserts `[data-testid*="feature-card"]` on `/` (grep `feature-card` in the file): either (a) click `[data-testid="view-toggle-list"]` before the assertion, or (b) assert `[data-testid*="kanban-card"]` instead if the test only cares that cards render. Do NOT delete or skip any test (CON-004). The "feature list loads and shows features" test, "feature detail page renders correctly" test, "phase progress indicators render" test, and "feature count badge" tests need review. The "feature count badge" tests assert `[data-testid="feature-count-badge"]` which lives in the header above the view body — those should still pass unchanged; verify but likely no edit needed.

**Constraints**: CON-004 (no regression — existing assertions continue to pass).

**Done conditions**:
- `npm run test:e2e app.spec.ts` green — every existing test passes.
- No test deleted, no test skipped (CON-004).
- `grep -n "feature-card" ui/e2e/app.spec.ts` — every match is either preceded by a `view-toggle-list` click or replaced by a `kanban-card` assertion.

**Test level**: e2e (regression).

**Agent failure mode checks**:
- [ ] Multi-component consistency: the fix must be applied to EVERY test that asserts `feature-card-*` on `/`, not just the first. Grep the spec for `feature-card` and fix each assertion.
- [ ] Don't break the test's intent: if a test navigates to detail via a card click, clicking List first preserves the flow; if a test asserts count, `kanban-card-*` works equivalently.

**Dependencies**: T005 (Board is the default, so the regression exists only after T005).

**Checkpoint**: Full e2e suite (`app.spec.ts` + `kanban.spec.ts`) green. P1 + P2 complete and no regressions.

---

## Phase 4: User Story 2 — Feature Cards on the Board Show Key State (Priority: P1)

**Goal**: Cards in the right columns with correct badges, gate indicator, navigation, and ring flags for blocked/waiting.

**Note**: US-002 is implemented by T004 (KanbanCard/KanbanColumn/KanbanBoard) and verified by T005 (kanban.spec.ts AC-007..AC-013, AC-016). No additional task — the work is covered by Phase 2. Listed here for traceability so the Reviewer can confirm every US-002 AC maps to a task.

**AC traceability**:
- AC-007 (P1 badge + In Progress badge in Planning column) → T004 + T005
- AC-008 (QuestionBadge) → T004 (reuses `QuestionBadge`) + T005
- AC-009 (gate indicator string) → T004 + T005
- AC-010 (card click → /features/:id) → T004 (`<Link>`) + T005
- AC-011 (unknown phase → Other column) → T002 (unit) + T005 (e2e)
- AC-012 (gate_blocked red ring) → T004 + T005
- AC-013 (waiting_for_human yellow ring) → T004 + T005
- AC-016 (single fetch) → T005 (integration)

---

## Phase 5: User Story 3 — Empty Columns and Empty Board (Priority: P2)

**Goal**: Empty columns render a placeholder; empty board renders EmptyState and hides the toggle.

**Note**: US-003 is implemented by T004 (KanbanColumn empty placeholder) + T005 (Dashboard toggle-hidden guard + AC-006/AC-017/AC-018/AC-019 e2e). No additional task. Listed for traceability.

**AC traceability**:
- AC-017 (empty column placeholder) → T004 + T005
- AC-018 (empty board → EmptyState, toggle hidden) → T005 (same as AC-006)
- AC-019 (exactly 6 columns + optional Other) → T002 (unit invariant) + T005 (e2e)

---

## Phase 6: User Story 4 — Column Overflow Handling (Priority: P3)

**Goal**: Column bodies scroll independently; board height bounded to viewport; narrow viewports scroll horizontally with 240px min column width.

### [ ] T007 [US4] Add overflow e2e tests to `ui/e2e/kanban.spec.ts`

**Files**:
- `ui/e2e/kanban.spec.ts` — [MODIFY] add tests for AC-020, AC-021, AC-022.
  - AC-020: seed 50 features in one phase (mock `/api/features` with 50 features all `current_phase:'construction'`); assert that column body `scrollHeight > clientHeight` and `document.body.scrollTop === 0` (page doesn't scroll). Use `page.evaluate` to read `scrollHeight`/`clientHeight` on `[data-testid="kanban-column-construction-body"]`.
  - AC-021: `page.setViewportSize({width:1280, height:400})`; assert each `[data-testid^="kanban-column-"]` body `clientHeight <= 400 - headerHeight` (measure header height via `getBoundingClientRect`).
  - AC-022: `page.setViewportSize({width:600, height:800})`; assert board container has `overflow-x: auto|scroll` (via `getComputedStyle`) and each column `getBoundingClientRect().width >= 240`.

**Constraints**: FR-013/FR-014/FR-015 (already implemented in T004 via Tailwind classes); these tests verify the CSS held.

**Done conditions**:
- `npm run test:e2e kanban.spec.ts` green — AC-020, AC-021, AC-022 pass.
- No new component code needed — if a test fails, the fix is in the Tailwind classes on `KanbanBoard`/`KanbanColumn` (T004 files), not new components.
- No console errors (SC-004).

**Test level**: e2e.

**Agent failure mode checks**:
- [ ] Viewport size must be set BEFORE measuring — `setViewportSize` then wait for re-render (`await page.waitForTimeout(100)` or `await expect(column).toBeVisible()`).
- [ ] `getBoundingClientRect`/`getComputedStyle` must run via `page.evaluate` (browser context), not Node.

**Dependencies**: T005 (kanban.spec.ts exists), T004 (overflow CSS in place).

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final gate verification before handoff to Review.

### [ ] T008 [SHARED] Final verification gate

**Files**: no code changes — verification only.

**Done conditions** (all must pass — proof of work, not claims):
- `npm run build` succeeds (tsc + vite, zero errors).
- `npm run lint` succeeds (zero errors).
- `npm run test:e2e` (full suite: `app.spec.ts` + `kanban.spec.ts`) green.
- `git diff ui/package.json` is empty (CON-003 — no new runtime dep; vitest if added is devDependency).
- `git diff ui/package-lock.json` only contains devDependency additions if any (verify no runtime dep added).
- Grep: `grep -RIn "Inception\|Planning\|Construction\|Review\|Testing\|Delivery" ui/src/components/Kanban* ui/src/lib/groupFeaturesByPhase.ts` returns only `PHASE_LABELS` references, no literals (CON-005).
- Grep: `grep -n "const statusColors" ui/src/components/FeatureCard.tsx` → 0 matches (T001 cleaned up — CON-006).
- Grep: `grep -n "useQuery" ui/src/components/Kanban*.tsx` → 0 matches (CON-007 — board consumes via props, no second fetch).
- `npx playwright test kanban.spec.ts --reporter=line` output shows all 22 tests (AC-001..AC-022) passing.

**Test level**: smoke + e2e + integration + unit (all levels exercised).

**Dependencies**: T006, T007.

---

## Dependencies & Execution Order

### Task Dependency Graph

```
T001 (badgeStyles) ─┬─► T004 (board components) ──┐
T002 (groupFn) [P] ─┘                              ├─► T005 (Dashboard + ViewToggle + kanban.spec) ──► T006 (app.spec fix) ──► T008 (gate)
T003 (useSessionView) [P] ─────────────────────────┘                                                    T007 (overflow tests) ──┘
```

### Phase Dependencies
- **Phase 1 (T001/T002/T003)**: no dependencies — start immediately. T002 and T003 run in parallel ([P]).
- **Phase 2 (T004/T005)**: depends on Phase 1 complete. T005 depends on T004.
- **Phase 3 (T006)**: depends on T005 (Board-default creates the regression).
- **Phase 6 (T007)**: depends on T005 (kanban.spec.ts exists). Can run in parallel with T006 ([P] — different test concerns, same file but non-overlapping test names; if serializing is safer, run after T006).
- **Phase 7 (T008)**: depends on T006 + T007.

### Parallel Opportunities
- T002 ∥ T003 (Phase 1 — different files, no deps).
- T006 ∥ T007 (Phase 3/6 — different test concerns; if the e2e runner serializes, run sequentially).

### Within Each User Story
- Shared infra (Phase 1) before any UI.
- Components (T004) before Dashboard wiring (T005).
- Dashboard wiring before regression fix (T006) — the regression only exists once Board is default.

---

## Implementation Strategy

### MVP First (P1 only)
1. Complete Phase 1 (T001/T002/T003) → shared infra ready.
2. Complete T004 + T005 → Board renders, toggle works, P1+P2 ACs pass.
3. Complete T006 → no regressions.
4. **STOP and VALIDATE**: `npm run test:e2e` green; P1 (US-001 + US-002) and P2 (US-003) delivered.

### Then P3
5. Complete T007 → overflow ACs pass.
6. Complete T008 → final gate.

---

## Notes

- **[P]** tasks = different files, no dependencies.
- **[Story]** label maps task to user story for traceability (SHARED for cross-story infra).
- Every task references specific files and specific ACs/CONs.
- Commit after each task or logical group (per `rules.md` commit discipline).
- Stop at any checkpoint to validate independently.
- **Avoid**: vague tasks, same-file conflicts, cross-story dependencies that break independence.