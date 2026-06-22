# Tasks: kanban-view

**Input**: Design documents from `specs/kanban-view/` (plan.md, research.md, data-model.md, contracts/)

**Prerequisites**: plan.md (required), spec.md (required for user stories), acceptance.md (required for AC mapping).

**Tests**: Tests are included — the spec mandates e2e (AC-001..AC-022) and a unit test (AC-011).

**Organization**: Tasks grouped by user story priority (P1: US-001 + US-002, P2: US-003, P3: US-004), with a foundational phase for shared primitives.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1, US2, US3, US4, or SHARED (foundational)
- Exact file paths in descriptions

---

## Phase 1: Foundational (Shared Primitives)

**Purpose**: Primitives that all user stories depend on. Must complete before any user-story implementation.

**⚠️ CRITICAL**: No user story work begins until this phase is complete.

- [ ] T001 [SHARED] Extract `statusColors` map to shared module
  - Files:
    - `ui/src/components/badgeColors.ts` — CREATE. Export `const statusColors: Record<string, string>` (copy the 9-entry map from `FeatureCard.tsx`).
    - `ui/src/components/FeatureCard.tsx` — MODIFY. Remove inline `statusColors`, import from `./badgeColors`. No other change.
  - Constraints: CON-006 (card chrome parity — single source of truth).
  - Done conditions:
    - `badgeColors.ts` exists and exports `statusColors`.
    - `FeatureCard.tsx` imports `statusColors` from `./badgeColors` (grep: `import { statusColors } from './badgeColors'`).
    - No other definition of `statusColors` exists in `ui/src/` (grep `const statusColors` → exactly one match, in `badgeColors.ts`).
    - `npm run dev` loads the Dashboard (click List toggle manually if needed); existing `FeatureCard` renders identically — no visual drift.
  - Test level: smoke (dev server starts, no console error).
  - Agent failure mode checks:
    - [ ] Did not duplicate the map — single source.
    - [ ] `FeatureCard` import path correct (`./badgeColors`).

- [ ] T002 [SHARED] [P] Create `useSessionView` hook
  - Files:
    - `ui/src/hooks/useSessionView.ts` — CREATE.
  - Spec: returns `['board' | 'list', (v: 'board' | 'list') => void]`. Lazy-init from `sessionStorage.getItem('devteam.dashboard.view')`; validate value ∈ {'board','list'}, else default `'board'` (FR-003). On change, `setItem`. try/catch around storage access → fall back to `'board'` on error (private mode).
  - Constraints: FR-002, FR-003.
  - Done conditions:
    - `useSessionView.ts` exists, exports `useSessionView`.
    - Fresh session (no stored value) → returns `'board'` (FR-003).
    - After `setView('list')`, `sessionStorage.getItem('devteam.dashboard.view')` === `'list'` (FR-002).
    - Invalid stored value (e.g. `'garbage'`) → returns `'board'`.
    - Storage access wrapped in try/catch (no throw in private mode).
  - Test level: e2e (AC-004/005 exercise this via the toggle).
  - Agent failure mode checks:
    - [ ] Lazy initializer does not throw on storage error.
    - [ ] No SSR guard needed (Vite SPA) but `typeof window !== 'undefined'` guard is acceptable if defensive.

- [ ] T003 [SHARED] [P] Implement and unit-test `groupFeaturesByPhase`
  - Files:
    - `ui/src/components/KanbanBoard.tsx` — CREATE (initially just the exported pure function + a thin default-export placeholder component; full component built in T006). OR create `ui/src/components/grouping.ts` and re-export from `KanbanBoard.tsx`. **Decision: co-locate in `KanbanBoard.tsx` and export — fewer files.**
    - `ui/src/components/KanbanBoard.test.ts` — CREATE. Vitest unit test.
    - `ui/package.json` — MODIFY. Add `vitest` to devDependencies, add `"test:unit": "vitest run"` script.
  - Spec: `groupFeaturesByPhase(features: FeatureSummary[]): Record<PhaseName | 'other', FeatureSummary[]>`. Initialize all 6 phase buckets + `other` to `[]`. For each feature, if `PHASES.includes(feature.current_phase as PhaseName)` push to that bucket, else push to `other`. Partition invariant: `sum === input.length`.
  - Constraints: CON-009 (unknown phase defensive), CON-008 (no null arrays), FR-006, FR-007.
  - Done conditions:
    - `npm run test:unit` passes.
    - Test cases:
      - Empty input → 6 empty buckets + empty `other`; all buckets are `[]` (not null/undefined).
      - Known-phase feature → placed in correct bucket.
      - Unknown-phase feature (`current_phase: 'weird'`) → placed in `other` bucket; no throw (AC-011, CON-009).
      - Mixed input → partition sum invariant holds: `Object.values(groups).reduce((n, f) => n + f.length, 0) === input.length`.
  - Test level: unit (AC-011).
  - Agent failure mode checks:
    - [ ] Every bucket initialized to `[]` — no `null` arrays (CON-008).
    - [ ] Unknown phase does not crash (CON-009).
    - [ ] Partition invariant — no feature dropped or duplicated.
    - [ ] `package.json` `dependencies` block unchanged — `vitest` is in `devDependencies` only (CON-003).

**Checkpoint**: Foundational primitives ready — `badgeColors`, `useSessionView`, `groupFeaturesByPhase` all exist and tested. User-story implementation can begin.

---

## Phase 2: User Story 1 — Toggle Between List and Kanban Board (Priority: P1) 🎯 MVP

**Goal**: A user can switch between List and Board views; choice persists for the session; Board is default.

**Independent Test**: Load Dashboard, click "Board" → columns render; click "List" → FeatureList renders; reload → Board persists.

### Implementation for User Story 1

- [ ] T004 [US1] Create `ViewToggle` component
  - Files:
    - `ui/src/components/ViewToggle.tsx` — CREATE.
  - Spec: two `<button>` elements, container `data-testid="view-toggle"`, buttons `data-testid="view-toggle-list"` / `"view-toggle-board"`. Active button `aria-pressed="true"`, inactive `aria-pressed="false"`. Props `{ view: 'board' | 'list'; onViewChange: (v) => void }`. Tailwind styling matches existing button aesthetic (blue active, gray inactive, dark: variants).
  - Constraints: FR-001.
  - Done conditions:
    - `ViewToggle.tsx` exists.
    - Renders exactly two buttons with the specified testids.
    - Exactly one button has `aria-pressed="true"` at any time (AC-001).
    - Clicking inactive button calls `onViewChange` with that view.
  - Test level: e2e (AC-001/002/003/004/005 exercise via Dashboard).
  - Agent failure mode checks:
    - [ ] Exactly one `aria-pressed="true"` — never both, never neither.
    - [ ] No nested interactive elements.

- [ ] T005 [US1] Wire toggle + Board/List conditional into `Dashboard.tsx`
  - Files:
    - `ui/src/pages/Dashboard.tsx` — MODIFY.
  - Spec:
    - Import `useSessionView`, `ViewToggle`, `KanbanBoard` (placeholder import OK if T006 not done — but T006 is a dependency; see below).
    - `const [view, setView] = useSessionView();`
    - Render `<ViewToggle view={view} onViewChange={setView} />` **only** inside the `!isLoading && !error && features.length > 0` branch, positioned above the view body (FR-004).
    - In that same branch: `view === 'board' ? <KanbanBoard features={features} /> : <FeatureList features={features} />`.
    - Loading / error / empty branches unchanged (FR-017, CON-008).
  - Dependencies: T002 (useSessionView), T004 (ViewToggle), T006 (KanbanBoard component).
  - Constraints: FR-001, FR-002, FR-003, FR-004, FR-016, FR-017, CON-007, CON-008.
  - Done conditions:
    - `npm run dev` — Dashboard loads with Board visible by default (AC-005).
    - Click "List" → `FeatureList` renders, zero `kanban-column-*` (AC-003).
    - Click "Board" → six columns render, zero `feature-list` (AC-002).
    - Reload → Board still active (AC-004).
    - Route `/api/features` to `{features:[], total_count:0}` → `EmptyState` renders, `view-toggle` count 0 (AC-006/018).
    - Route `/api/features` to 500 → `features-error` visible, `view-toggle` count 0 (AC-015).
    - Route `/api/features` with delay → `features-loading` visible, zero `kanban-column-*` (AC-014).
    - Network tab: exactly one `GET /api/features` when Board renders (AC-016, CON-007).
  - Test level: e2e (AC-001..006, AC-014..016, AC-018) + integration (AC-016).
  - Agent failure mode checks:
    - [ ] Toggle hidden in empty/error/loading states (FR-004).
    - [ ] Single `useQuery(['features'])` call — no second fetch added (CON-007).
    - [ ] Existing loading/error/empty branches not modified (CON-008).

**Checkpoint**: User Story 1 fully functional and independently testable — toggle works, Board default, persistence, empty/loading/error states correct.

---

## Phase 3: User Story 2 — Feature Cards on the Board Show Key State (Priority: P1)

**Goal**: Each feature appears as a card in its phase column with title, badges, gate indicator, and status-flag ring. Click navigates to detail.

**Independent Test**: Load Board, assert each card is in the column matching `PHASE_LABELS[feature.current_phase]` with correct badges.

### Implementation for User Story 2

- [ ] T006 [US2] Implement `KanbanBoard` component (full)
  - Files:
    - `ui/src/components/KanbanBoard.tsx` — MODIFY (was created in T003 with only `groupFeaturesByPhase`).
  - Spec: render columns in `PHASES` order using `KanbanColumn`; append `other` column only when `groups.other.length > 0` (FR-007). Board container `flex gap-4 overflow-x-auto`, height `h-[calc(100vh-8rem)]` (FR-014/015). No `useQuery` — props only (CON-007).
  - Dependencies: T003 (groupFeaturesByPhase), T007 (KanbanColumn).
  - Constraints: FR-005, FR-006, FR-007, FR-013, FR-014, FR-015, FR-016, CON-007, CON-008.
  - Done conditions:
    - Renders exactly 6 `kanban-column-*` elements when no unknown phases (AC-019).
    - Renders 7th `kanban-column-other` only when an unknown phase exists (AC-011/019).
    - Column order matches `PHASES` (AC-019).
    - Board container has `overflow-x-auto` (or `overflow-x-scroll`) — AC-022.
    - No `useQuery` / `fetch` calls in this file (CON-007).
  - Test level: e2e (AC-002/007/011/019/022) + unit (AC-011 via T003).
  - Agent failure mode checks:
    - [ ] `other` column conditional — not always rendered.
    - [ ] No network calls (CON-007).
    - [ ] Column order from `PHASES` constant, not hardcoded.

- [ ] T007 [US2] Implement `KanbanColumn` component
  - Files:
    - `ui/src/components/KanbanColumn.tsx` — CREATE.
  - Spec: props `{ phase: PhaseName | 'other'; label: string; features: FeatureSummary[] }`. Root `data-testid="kanban-column-${phase}"`. Header (fixed) + body (`flex-1 overflow-y-auto`, FR-013). Empty placeholder `data-testid="kanban-column-empty-${phase}"` with muted "No features" when `features.length === 0` (FR-012). Column width `w-60` (240px, FR-015).
  - Dependencies: T008 (KanbanCard).
  - Constraints: FR-012, FR-013, FR-015, CON-008.
  - Done conditions:
    - Empty column renders header + "No features" placeholder (AC-017).
    - Column body `scrollHeight > clientHeight` when overflowing (AC-020).
    - Column `getBoundingClientRect().width >= 240` (AC-022).
    - Header is outside the `overflow-y-auto` element (stays fixed when body scrolls).
  - Test level: e2e (AC-017/020/021/022).
  - Agent failure mode checks:
    - [ ] Empty body renders placeholder, not blank/null.
    - [ ] Header not inside the scroll container.

- [ ] T008 [US2] Implement `KanbanCard` component
  - Files:
    - `ui/src/components/KanbanCard.tsx` — CREATE.
  - Spec: root `<Link to={/features/${id}}>` with `data-testid="kanban-card-${feature.id}"` (FR-010). Title (line-clamp-2). Status badge `data-testid="kanban-card-status"` using `STATUS_LABELS` + `statusColors` from `badgeColors.ts` (CON-005/006). Priority badge `data-testid="kanban-card-priority"` using `PRIORITY_LABELS`. Question badge: local `<span data-testid="question-badge">` (NOT `QuestionBadge` Link — avoids nested anchor) when `pending_questions_count > 0` (FR-008). Gate indicator `data-testid="kanban-card-gate"` with `✓ Gate passed` / `✗ Gate failed` (FR-009, CON-006 — byte-identical to `FeatureCard`). Status-flag ring: `ring-2 ring-red-400` for `gate_blocked` (AC-012), `ring-2 ring-yellow-400` for `waiting_for_human` (AC-013), no ring otherwise. Updated date line.
  - Dependencies: T001 (badgeColors).
  - Constraints: FR-008, FR-009, FR-010, FR-011, CON-005, CON-006.
  - Done conditions:
    - Card in Planning column shows title + "P1 - Critical" + "In Progress" for a `priority=1, status=in_progress` feature (AC-007).
    - `pending_questions_count > 0` → `question-badge` testid visible (AC-008).
    - `gate_result` present → `kanban-card-gate` text `✓ Gate passed` or `✗ Gate failed` (AC-009).
    - Click card → navigates to `/features/:id` (AC-010).
    - `status === 'gate_blocked'` → card class contains `ring-red` (AC-012).
    - `status === 'waiting_for_human'` → card class contains `ring-yellow` (AC-013).
    - No nested `<a>` elements (question badge is `<span>`).
    - Gate indicator text grep: `KanbanCard.tsx` and `FeatureCard.tsx` both contain `✓ Gate passed` and `✗ Gate failed` (CON-006).
  - Test level: e2e (AC-007/008/009/010/012/013).
  - Agent failure mode checks:
    - [ ] Ring class only for the two attention statuses — no accidental ring on normal cards.
    - [ ] Gate text byte-identical to `FeatureCard` (CON-006).
    - [ ] No nested anchors — question badge is `<span>`.
    - [ ] `statusColors` imported from `badgeColors.ts`, not redefined.

**Checkpoint**: User Stories 1 AND 2 both functional — toggle + full board with correct cards, badges, rings, navigation.

---

## Phase 4: User Story 3 — Empty Columns and Empty Board (Priority: P2)

**Goal**: Empty columns show a placeholder; empty board shows `EmptyState` with toggle hidden.

**Independent Test**: Load Board in a workspace where some phases have no features; assert every column renders with placeholder.

### Implementation for User Story 3

- [ ] T009 [US3] Verify empty-column + empty-board behavior (implementation largely in T007/T005)
  - Files: (no new files — verification task)
    - `ui/src/components/KanbanColumn.tsx` — verify empty placeholder (from T007).
    - `ui/src/pages/Dashboard.tsx` — verify `EmptyState` branch hides toggle (from T005).
  - Dependencies: T005, T007.
  - Constraints: FR-004, FR-012, CON-008.
  - Done conditions:
    - Testing column with no features: `kanban-column-testing` header visible, `kanban-column-empty-testing` contains "No features" (AC-017).
    - Zero features: `EmptyState` renders, `view-toggle` count 0 (AC-006/018).
    - All 6 phase columns present regardless of emptiness (AC-019).
    - US-3 scenario 3: create a feature when board was empty → toggle appears, Board renders with stored view (manual e2e or covered by kanban.spec.ts).
  - Test level: e2e (AC-006/017/018/019).
  - Agent failure mode checks:
    - [ ] Empty column body is `[]` + placeholder, never `null`/blank (CON-008).
    - [ ] Toggle hidden in empty state, not just visually-hidden (count 0 in DOM).

**Checkpoint**: US-3 complete — empty states polished, no broken columns.

---

## Phase 5: User Story 4 — Column Overflow Handling (Priority: P3)

**Goal**: Columns scroll vertically independent of each other; board height bounded to viewport; horizontal scroll on narrow viewports.

**Independent Test**: Seed 50 features in one phase; assert that column scrolls without page-level scroll.

### Implementation for User Story 4

- [ ] T010 [US4] Verify overflow CSS (implementation in T006/T007)
  - Files: (no new files — CSS already in T006/T007)
    - `ui/src/components/KanbanBoard.tsx` — board container `h-[calc(100vh-8rem)] overflow-x-auto` (from T006).
    - `ui/src/components/KanbanColumn.tsx` — column body `flex-1 overflow-y-auto`, column `w-60` (from T007).
  - Dependencies: T006, T007.
  - Constraints: FR-013, FR-014, FR-015.
  - Done conditions:
    - Seed 50 features in one phase → that column body `scrollHeight > clientHeight`; page `body.scrollTop === 0` (AC-020).
    - `page.setViewportSize({width:1280, height:400})` → each column body `clientHeight <= 400 - headerHeight` (AC-021).
    - `page.setViewportSize({width:600, height:800})` → board container `overflow-x` auto/scroll; each column `getBoundingClientRect().width >= 240` (AC-022).
  - Test level: e2e (AC-020/021/022).
  - Agent failure mode checks:
    - [ ] Board height uses viewport-relative unit (`100vh`), not a fixed pixel height.
    - [ ] Column body (not column root) has `overflow-y-auto`.
    - [ ] Column min-width `w-60` (240px) — not `w-60` + padding that shrinks below 240.

**Checkpoint**: US-4 complete — overflow polished.

---

## Phase 6: Cross-Cutting — Regression + Full E2E

**Purpose**: Protect existing tests and cover all acceptance criteria.

- [ ] T011 [SHARED] Update `app.spec.ts` for Board-default (CON-004 regression fix)
  - Files:
    - `ui/e2e/app.spec.ts` — MODIFY.
  - Spec: existing list-view tests assert `feature-card-*` on first load. With Board now default (FR-003), those assertions fail. Add a `beforeEach` or inline step: after `page.goto('/')` and `networkidle`, click `view-toggle-list` before asserting `feature-card-*`. Only for tests that assert list-view DOM. Tests that assert API behavior (`request.get`) or the count badge (present in both views? — no, count badge is in the header, above the toggle, always present) are unaffected. Tests that skip when no features remain unchanged.
  - Constraints: CON-004.
  - Done conditions:
    - `npm run test:e2e` — all existing `app.spec.ts` tests pass (CON-004).
    - No existing assertion removed or weakened — only an additive click step.
    - Count-badge tests still pass (badge is in header, not view body).
  - Test level: e2e (regression).
  - Agent failure mode checks:
    - [ ] Did not delete/skip existing tests to make them pass.
    - [ ] Click step waits for `view-toggle-list` to be visible (handles loading state).
    - [ ] Tests that skip on empty workspace still skip correctly.

- [ ] T012 [SHARED] Create `kanban.spec.ts` covering AC-001..AC-022
  - Files:
    - `ui/e2e/kanban.spec.ts` — CREATE.
  - Spec: one test per acceptance criterion (or grouped where one test covers multiple ACs):
    - AC-001: toggle visible, Board active by default.
    - AC-002: click Board → 6 columns, no feature-list.
    - AC-003: click List → feature-list, no columns.
    - AC-004: reload → Board persists (sessionStorage).
    - AC-005: fresh context → Board default.
    - AC-006: empty features → toggle count 0, EmptyState.
    - AC-007: card in Planning column with P1 + In Progress badges.
    - AC-008: question badge visible when `pending_questions_count > 0`.
    - AC-009: gate indicator `✓ Gate passed` / `✗ Gate failed`.
    - AC-010: click card → `/features/:id`.
    - AC-011: (unit — already in T003; reference, do not re-test in e2e).
    - AC-012: `gate_blocked` → `ring-red` class.
    - AC-013: `waiting_for_human` → `ring-yellow` class.
    - AC-014: loading → `features-loading`, zero columns.
    - AC-015: error → `features-error`, zero columns.
    - AC-016: single `GET /api/features` (page.on('request') count).
    - AC-017: empty Testing column → "No features" placeholder.
    - AC-018: (same as AC-006).
    - AC-019: exactly 6 columns (+optional other).
    - AC-020: 50 features in one phase → column scrolls, body doesn't.
    - AC-021: resize shorter → column body height adjusts.
    - AC-022: narrow viewport → horizontal scroll, columns >= 240px.
  - Dependencies: T005, T006, T007, T008, T009, T010.
  - Constraints: CON-001 (Playwright :18765), all FRs/ACs.
  - Done conditions:
    - `npm run test:e2e` — `kanban.spec.ts` all tests pass.
    - Every AC-001..AC-022 has a corresponding test (map in test file comments).
    - Console-error assertion present in Board-view tests (SC-004).
    - All selectors use `data-testid` (no class-based state assertions except the ring-class ACs which explicitly assert class).
  - Test level: e2e + integration (AC-016).
  - Agent failure mode checks:
    - [ ] Tests run on `:18765` (playwright.config.ts), not `:8765` (CON-001).
    - [ ] No flaky selectors — all `data-testid`.
    - [ ] AC-016 request count uses `page.on('request')` with a filter on `/api/features`.
    - [ ] AC-020/021/022 use `evaluate` for `scrollHeight`/`clientHeight`/`getBoundingClientRect`.

**Checkpoint**: Full e2e suite green — `app.spec.ts` + `kanban.spec.ts`. All 22 ACs covered. No regression.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Foundational)**: no dependencies — start immediately. T001, T002, T003 can run in parallel (T002 and T003 are marked [P]; T001 touches `FeatureCard.tsx` which neither T002 nor T003 touches, so also parallel-safe).
- **Phase 2 (US-1)**: T004 (ViewToggle) depends on nothing new (parallel with Phase 1). T005 (Dashboard wiring) depends on T002, T004, T006.
- **Phase 3 (US-2)**: T008 (KanbanCard) depends on T001. T007 (KanbanColumn) depends on T008. T006 (KanbanBoard) depends on T003 + T007.
- **Phase 4 (US-3)**: T009 depends on T005 + T007 (verification only).
- **Phase 5 (US-4)**: T010 depends on T006 + T007 (verification only).
- **Phase 6 (Cross-cutting)**: T011 (app.spec.ts) depends on T005 (Board is default). T012 (kanban.spec.ts) depends on T005/T006/T007/T008/T009/T010.

### Parallel Opportunities

- T001, T002, T003 can all run in parallel (Phase 1).
- T004 (ViewToggle) can run in parallel with Phase 1 (different file, no dependency).
- T008 (KanbanCard) can start once T001 completes — parallel with T002/T003/T004.

### Recommended Execution Order (single developer)

1. T001 → T002 → T003 (parallel) → checkpoint
2. T008 → T007 → T006 → T004 → T005 → checkpoint (US-1 + US-2 functional)
3. T009 → T010 (verification) → checkpoint
4. T011 → T012 (tests) → full green

## Implementation Strategy

### MVP First (US-1 + US-2)

1. Complete Phase 1 (foundational primitives).
2. Complete US-1 (toggle) + US-2 (cards) — together, since T005 depends on T006.
3. **STOP and VALIDATE**: Board renders, toggle works, cards correct, navigation works.
4. Add US-3 (empty states — mostly verification) and US-4 (overflow — CSS verification).
5. Add regression fixture (T011) and full e2e (T012).

## Notes

- Every task references specific files (no "where does this go?" ambiguity).
- Every task has verifiable done conditions tied to ACs.
- Constraint references: T001→CON-006; T002→FR-002/003; T003→CON-008/009; T004→FR-001; T005→FR-001/002/003/004/016/017, CON-007/008; T006→FR-005/006/007/013/014/015/016, CON-007/008; T007→FR-012/013/015, CON-008; T008→FR-008/009/010/011, CON-005/006; T009→FR-004/012, CON-008; T010→FR-013/014/015; T011→CON-004; T012→CON-001, all ACs.
- Commit after each task or logical group.