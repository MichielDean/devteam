# Feature Specification: Kanban View

**Feature Branch**: `kanban-view`

**Created**: 2026-06-22

**Status**: Draft

**Input**: User description: "Add a Kanban board view to the Dev Team UI that shows features as cards organized by phase. Features should be displayed in columns representing their current phase (Inception, Planning, Construction, Review, Testing, Delivery). Cards should show feature title, priority, and status. Users should be able to click a card to navigate to the feature detail page. Add a toggle between the current list view and the new Kanban board view."

## Workspace Summary (Brownfield)

**Project**: Dev Team — AI-DLC platform. Go backend + React/TypeScript frontend.

**Frontend stack** (the only code this feature touches):
- React 18 + TypeScript
- react-router (client routing; `Link` / `useNavigate`)
- @tanstack/react-query (server state; `useQuery(['features'])` on Dashboard)
- Tailwind CSS (utility classes, dark mode via `dark:` variant)
- Vite build
- Playwright e2e in `ui/e2e/` (port 18765)

**Existing relevant code**:
- `ui/src/pages/Dashboard.tsx` — fetches `listFeatures`, renders `FeatureList` or `EmptyState`. Has "Features" heading + count badge + "+ New Feature" button.
- `ui/src/components/FeatureList.tsx` — sort controls + responsive grid (`sm:grid-cols-2 lg:grid-cols-3`) of `FeatureCard`.
- `ui/src/components/FeatureCard.tsx` — `Link` to `/features/{id}`, shows title, id, status badge, phase badge, priority badge, gate result, updated date, question badge.
- `ui/src/components/EmptyState.tsx` — empty-features message.
- `ui/src/types/index.ts` — `FeatureSummary` (id, title, status, priority, current_phase, updated_at, gate_result, pending_questions_count), `PHASES` const (6 phases), `PHASE_LABELS`, `STATUS_LABELS`, `PRIORITY_LABELS`.

**Data source**: `listFeatures()` already returns everything the Kanban needs (current_phase, status, priority). **No backend / API / Go changes required.** Pure frontend presentation feature.

**Conventions to follow**:
- `data-testid` attributes on all interactive/observable elements (matches existing pattern — see `feature-card-${id}`, `dashboard-page`).
- Dark mode via `dark:` Tailwind variants.
- Existing badge color classes (status/phase/priority) should be reused for visual consistency.
- Existing `FeatureCard` component should be reused in the Kanban columns (single source of card truth).

## Request Analysis

- **Clarity**: Vague → needs clarification (6 questions asked).
- **Type**: New feature (UI enhancement).
- **Scope**: Single component (UI only — `ui/` repo).
- **Complexity**: Simple → clear implementation path. No new data, no backend, no state machine.
- **Priority**: P1 (declared in input).

## Source Discovery

**External standards/RFCs**: None. This is a pure UI presentation feature; no protocol, no data format standard.

**Test vectors**: None governing the feature. Existing Playwright e2e suite in `ui/e2e/` defines the testing convention to follow.

**Internal conventions**: AGENTS.md (build/test commands, Playwright port 18765, never build from worktree), FeatureCard/FeatureList patterns, `data-testid` convention.

**Error taxonomies**: None — UI feature has no protocol error codes. The only "error" path is the existing `listFeatures` failure already handled by Dashboard (`features-error` testid).

**Security constraints**: Feature renders data already returned to the authenticated UI session. No new endpoints, no new input vectors. Title is already rendered safely by React (no `dangerouslySetInnerHTML`). No new security surface.

## User Scenarios & Testing

### User Story 1 — Toggle Between List and Kanban Views (Priority: P1)

A user viewing the Dashboard can switch between the existing list/grid view of features and a new Kanban board view, and their choice persists across reloads.

**Why this priority**: The toggle is the entry point to the entire feature. Without it the Kanban board is unreachable and no other story delivers value. It is also the smallest independently-testable slice — a user can toggle and confirm the board appears, delivering immediate value even before polish.

**Independent Test**: Can be fully tested by loading the Dashboard, clicking the Kanban toggle, confirming the board renders, clicking List, confirming the grid returns, reloading, and confirming the last choice persisted. Delivers value: user can choose their preferred view.

**Acceptance Scenarios**:

1. **Given** the Dashboard is loaded with features present, **When** the user clicks the "Kanban" toggle control, **Then** the view switches from the feature grid to a Kanban board with six phase columns.
2. **Given** the Kanban board is displayed, **When** the user clicks the "List" toggle control, **Then** the view switches back to the existing feature grid.
3. **Given** the user has selected Kanban view, **When** the page is reloaded, **Then** the Kanban view is displayed again (choice persisted).
4. **Given** a user has never toggled before, **When** they load the Dashboard, **Then** the list view is shown (default).

---

### User Story 2 — View Features as Cards in Phase Columns (Priority: P1)

A user viewing the Kanban board sees all features rendered as cards grouped into six columns — one per pipeline phase (Inception, Planning, Construction, Review, Testing, Delivery). Each card shows the feature title, priority, and status. Each column has a header with the phase name and a count of features in that column.

**Why this priority**: This is the core visual deliverable — the board itself. Without it the toggle has nothing to show. Paired with US-1 it forms the minimum viable feature.

**Independent Test**: Create features in different phases (or seed via existing pipeline runs), load the Kanban view, and verify each feature appears in the column matching its `current_phase`, that card content (title/priority/status) matches the list view, and that column counts match the features present. Delivers value: at-a-glance phase distribution.

**Acceptance Scenarios**:

1. **Given** features exist in multiple phases, **When** the Kanban view is displayed, **Then** each feature appears in exactly one column corresponding to its `current_phase`.
2. **Given** the Kanban view is displayed, **Then** each column header shows the phase label (from `PHASE_LABELS`) and a feature count for that column.
3. **Given** the Kanban view is displayed, **Then** each card shows the feature title, priority badge, and status badge (same content as the existing `FeatureCard`).
4. **Given** no features exist in a phase, **When** that column is rendered, **Then** the column is still visible with its header and a count of 0 and an empty-state message inside.

---

### User Story 3 — Click a Card to Navigate to Feature Detail (Priority: P1)

A user can click any card in the Kanban board to navigate to that feature's detail page, identical to clicking a card in the list view.

**Why this priority**: The input explicitly requires click-to-navigate, and without it the board is a dead-end. Reuses the existing `FeatureCard` `Link` behavior — the test is whether the same navigation works inside a column.

**Independent Test**: Load Kanban view, click a card, confirm the URL changes to `/features/{id}` and the detail page renders. Delivers value: the board is a navigation surface, not just a display.

**Acceptance Scenarios**:

1. **Given** the Kanban view is displayed with a card for feature X, **When** the user clicks the card, **Then** the browser navigates to `/features/{id-of-X}` and the FeatureDetail page renders.
2. **Given** a card with pending questions (question badge present), **When** the user clicks the card, **Then** navigation still occurs (badge does not block the link).

---

### User Story 4 — Horizontal Scroll for Six Columns (Priority: P2)

A user on a viewport too narrow to show all six columns side-by-side can scroll the board horizontally to see off-screen columns, with column headers and counts remaining visible while scrolling.

**Why this priority**: Polish/responsive concern. The board is functional without it (US-2) but cramped on laptops. P2 because the core value (seeing features by phase) is delivered by US-1/US-2; this makes it usable on real screens.

**Independent Test**: Resize the browser to a width where 6 columns don't fit, confirm the board scrolls horizontally, confirm a column initially off-screen becomes visible after scrolling, confirm headers stay aligned with their columns. Delivers value: usable on common laptop widths.

**Acceptance Scenarios**:

1. **Given** the viewport is narrower than the combined width of six columns, **When** the Kanban view is displayed, **Then** the board is horizontally scrollable and off-screen columns become visible by scrolling.
2. **Given** the board is scrolled horizontally, **Then** each column's header and count remain aligned above their respective cards (no header/card desync).

---

### User Story 5 — Card Ordering Within Columns (Priority: P2)

Within a column, cards are ordered by priority (P1 first, then P2, then P3); ties are broken by most-recently-updated first. This makes the most important and active features visually dominant within each phase.

**Why this priority**: Ordering is a refinement of US-2. The board is useful unordered, but priority ordering is the expected Kanban convention and makes the board scannable. P2 because it improves usability without blocking core value.

**Independent Test**: Seed features in the same phase with mixed priorities and updated_at values, load Kanban, confirm the visual order matches priority-then-recency. Delivers value: important features surface first within each column.

**Acceptance Scenarios**:

1. **Given** a column contains features of priorities P1, P2, and P3, **When** the column is rendered, **Then** P1 cards appear above P2 cards, which appear above P3 cards.
2. **Given** a column contains two P1 features with different `updated_at` values, **When** the column is rendered, **Then** the more recently updated card appears above the older one.

---

### User Story 6 — Empty Board State (Priority: P3)

When there are zero features total, the Kanban toggle is hidden (or disabled) and the existing `EmptyState` component is shown, so the user is not presented with six empty columns.

**Why this priority**: Edge case. The existing Dashboard already handles the zero-features case with `EmptyState`; this story ensures the Kanban path does not regress that. P3 because it only fires in the empty-repo state and the existing empty state already handles the default list view.

**Independent Test**: With no features in the system, load Dashboard, confirm `EmptyState` is shown and the Kanban toggle is not active/visible. Delivers value: no confusing empty board on first run.

**Acceptance Scenarios**:

1. **Given** there are zero features, **When** the Dashboard loads, **Then** the existing `EmptyState` component is rendered (not six empty columns).
2. **Given** there are zero features, **Then** the Kanban toggle is hidden or disabled so the user cannot navigate to an empty board.

---

### Edge Cases

- **Feature in an unrecognized phase**: If `current_phase` is not one of the six known phases, the card must still render — in an "Unknown" bucket or appended to the last column. [ASSUMPTION: backend only ever emits one of the six PHASES, so an unknown phase is treated as a bug to surface, not a normal path. Card will render in a trailing "Other" column with a count, so no feature is silently dropped.]
- **Features list is still loading**: Kanban view should show the same loading spinner the list view uses (`features-loading`), not an empty board.
- **Features fetch errored**: Kanban view should show the same error message the list view uses (`features-error`), not an empty board.
- **Single feature across all phases**: board renders six columns, five with count 0, one with count 1 — covered by US-2 AC-4.
- **Very long feature titles**: cards must truncate the title (existing `FeatureCard` uses `truncate` class) — reusing `FeatureCard` inherits this.
- **Dark mode**: all new UI must support `dark:` variants consistent with existing components.
- **URL shareability**: [ASSUMPTION: view choice persisted in localStorage (see questions.json). If human picks URL query string instead, US-1 AC-3 changes accordingly — currently written for localStorage.]
- **Browser back/forward**: toggling view should not push duplicate history entries that trap the back button. [ASSUMPTION: toggle uses local state + localStorage, not router navigation, so back goes to the previous page, not the previous view.]
- **Pending-questions badge on a card inside a column**: must still render and must not block the card's click navigation (covered US-3 AC-2).

## Requirements

### Functional Requirements

- **FR-001**: The Dashboard MUST display a view-toggle control with at least two options: "List" and "Kanban".
  *Source: US-1*
- **FR-002**: Selecting "Kanban" MUST replace the feature grid with a Kanban board consisting of exactly six columns, one per phase in pipeline order: Inception, Planning, Construction, Review, Testing, Delivery.
  *Source: US-2*
- **FR-003**: Selecting "List" MUST restore the existing `FeatureList` component view.
  *Source: US-1*
- **FR-004**: The selected view MUST persist across page reloads via localStorage.
  *Source: US-1*
- **FR-005**: The default view (no prior selection) MUST be the List view.
  *Source: US-1*
- **FR-006**: Each Kanban column MUST display a header containing the phase label (from `PHASE_LABELS`) and the count of features currently in that phase.
  *Source: US-2*
- **FR-007**: Each feature MUST appear in exactly one column — the column whose phase equals the feature's `current_phase`.
  *Source: US-2*
- **FR-008**: Each card in a column MUST reuse the existing `FeatureCard` component (title, priority badge, status badge, gate indicator, updated date, question badge).
  *Source: US-2, US-3*
- **FR-009**: Clicking a card in the Kanban board MUST navigate to `/features/{id}`, identical to clicking a card in the list view.
  *Source: US-3*
- **FR-010**: Columns with zero features MUST still render with their header, a count of 0, and an empty-state message inside the column body.
  *Source: US-2*
- **FR-011**: When the combined width of six columns exceeds the viewport, the board MUST scroll horizontally; column headers MUST remain aligned with their columns during scroll.
  *Source: US-4*
- **FR-012**: Within a column, cards MUST be ordered by priority ascending (P1 before P2 before P3); ties MUST be broken by `updated_at` descending (most recent first).
  *Source: US-5*
- **FR-013**: When the features list is loading, the Kanban view MUST show the existing loading indicator (not an empty board).
  *Source: Edge cases*
- **FR-014**: When the features fetch errors, the Kanban view MUST show the existing error indicator (not an empty board).
  *Source: Edge cases*
- **FR-015**: When there are zero features, the Dashboard MUST show the existing `EmptyState` component and MUST NOT show the Kanban board with six empty columns.
  *Source: US-6*
- **FR-016**: When there are zero features, the view-toggle control MUST be hidden or disabled.
  *Source: US-6*
- **FR-017**: If a feature's `current_phase` is not one of the six known phases, the card MUST render in a trailing "Other" column (so no feature is silently dropped) and the column MUST follow the same header/count/empty-state rules as phase columns.
  *Source: Edge cases*
- **FR-018**: All new UI MUST support dark mode via Tailwind `dark:` variants consistent with existing components.
  *Source: Edge cases*

### Key Entities

This feature introduces **no new entities**. It is a presentation layer over the existing `FeatureSummary`:

- **FeatureSummary** (existing, unchanged): id, title, status, priority, current_phase, updated_at, gate_result, pending_questions_count. Source: `ui/src/types/index.ts`.
- **Phase column** (view-only, not persisted): derived from the `PHASES` constant + the set of features. Has a label (`PHASE_LABELS`), an ordered card list (filtered + sorted from `FeatureSummary[]`), and a count. Not an entity in the data model — a derived view.
- **View mode** (UI state only): enum { list, kanban }. Persisted in localStorage key (e.g. `devteam-dashboard-view`). Not a backend entity.

No state transitions to document — the only state is the view-mode toggle (list ⇄ kanban), both directions always valid.

## Success Criteria

### Measurable Outcomes

- **SC-001**: A user can switch from list to Kanban view in a single click and see all features grouped by phase within 1 render frame (no spinner beyond the existing listFeatures load).
- **SC-002**: Every feature visible in the list view is visible in exactly one Kanban column (no features dropped, no duplicates across columns).
- **SC-003**: Column counts shown in the Kanban headers match the actual number of features in each column (verifiable by summing counts and comparing to `total_count`).
- **SC-004**: Clicking any Kanban card navigates to the same URL as clicking the corresponding list-view card (`/features/{id}`) — 100% parity.
- **SC-005**: A user who selects Kanban, reloads the page, and sees Kanban again — persistence works on first reload (100% of attempts).
- **SC-006**: On a 1280px-wide viewport the board scrolls horizontally to reveal all six columns; no column is permanently clipped.
- **SC-007**: The Kanban view renders correctly in dark mode with no unthemed elements (no white-on-white or black-on-black text/backgrounds).
- **SC-008**: With zero features, the user never sees six empty columns — the existing EmptyState is shown (100% of empty-repo loads).

## Assumptions

- [ASSUMPTION: view choice persisted in localStorage, not URL query string. Default = list. Will revise if human picks URL query string in questions.json.]
- [ASSUMPTION: no drag-and-drop — cards are display-only; phase changes happen only through the pipeline. The board is an observation/navigation surface, not an editing surface. Will revise if human picks drag-and-drop.]
- [ASSUMPTION: all six phase columns are always shown, including empty ones, with an in-column empty-state message. Will revise if human picks "hide empty columns".]
- [ASSUMPTION: card ordering within a column is priority asc, then updated_at desc. Will revise if human picks a different ordering.]
- [ASSUMPTION: horizontal scroll for overflow on all viewport sizes (no separate mobile stacking layout). Will revise if human picks responsive wrap or mobile-stack.]
- [ASSUMPTION: column headers show feature counts. Will revise if human picks "headers minimal".]
- [ASSUMPTION: backend only ever emits one of the six known phase values for `current_phase`. Unknown phases are handled defensively (FR-017) but treated as a backend bug, not a normal path.]
- [ASSUMPTION: the existing `listFeatures` response shape does not need to change — it already returns `current_phase`, `status`, `priority`, `title`, `updated_at`, `gate_result`, `pending_questions_count`.]
- [ASSUMPTION: no new API endpoints, no backend changes, no Go changes. Entire feature is inside `ui/`.]
- [ASSUMPTION: existing Playwright e2e infrastructure (port 18765, `ui/e2e/`) is the test harness for this feature; no new test runner.]
- [ASSUMPTION: the existing `FeatureCard` component is reused verbatim inside columns so card appearance is identical between list and Kanban views.]

## Constraint Register

This feature implements no protocol and no external standard, so there are no RFC/standard-derived constraints. The constraints below are derived from internal conventions and the feature's own requirements; each has a matching acceptance criterion.

| ID | Source | Section/Vector | Type | Constraint | Verification Method |
|----|--------|----------------|------|------------|---------------------|
| CON-001 | AGENTS.md | "Frontend (UI)" | consistency | Build/test commands match repo: `npm run build`, `npm run lint`, `npx playwright test`. Do not invent commands. | AC-CON-001 |
| CON-002 | AGENTS.md | "Playwright E2E Tests" | consistency | E2E tests run on port 18765, not 8765. Config at `ui/playwright.config.ts`. | AC-CON-002 |
| CON-003 | Existing code | FeatureCard.tsx | consistency | Card markup reused via the `FeatureCard` component — no duplicated card markup in the Kanban column. | AC-CON-003 |
| CON-004 | Existing code | types/index.ts `PHASES` | consistency | Column set and order match the `PHASES` constant exactly (inception, planning, construction, review, testing, delivery). | AC-CON-004 |
| CON-005 | Existing code | types/index.ts `PHASE_LABELS` | consistency | Column header text comes from `PHASE_LABELS`, not hardcoded strings. | AC-CON-005 |
| CON-006 | Existing code | Dashboard.tsx `data-testid` | consistency | New interactive elements carry `data-testid` attributes matching the existing naming pattern. | AC-CON-006 |
| CON-007 | Constitution §V | Proof-of-Work Gates | consistency | Tester names specific Playwright test files and assertions traced to user stories — no "it works" claims. | AC-CON-007 |
| CON-008 | Constitution §VIII | Go, Minimal Dependencies | consistency | No new runtime dependency added to `ui/package.json` unless required; prefer existing Tailwind/react-router/react-query primitives. | AC-CON-008 |
| CON-009 | overconfidence-prevention | "Empty state behavior" | correctness | Empty column (count 0) is a valid state with a message, not an error and not a hidden column (per assumption pending question answer). | AC-006 / AC-CON-009 |
| CON-010 | overconfidence-prevention | "Happy Path Only" | correctness | Loading and error states for `listFeatures` are handled in Kanban view identically to list view. | AC-009 / AC-010 |

## Constitution Compliance

| Principle | Status | Rationale |
|---|---|---|
| I. Spec-Driven, Always | Compliant | This spec is the contract. No implementation begins without it + acceptance.md + repos.yaml. |
| II. Six Roles, Fixed Pipeline | Compliant | Feature enters via PM (inception). Architect/Developer/Reviewer/Tester/Ops will each run their phase. |
| III. Central Spec, Distributed Implementation | Compliant | Single spec in `specs/kanban-view/`. repos.yaml declares scope (single repo: `ui/`). No spec duplication. |
| IV. Two Intake Paths, One Output Format | Compliant | Loose idea → same spec.md + acceptance.md + repos.yaml shape as any other feature. |
| V. Proof-of-Work Gates | Compliant | Acceptance criteria are Given/When/Then with test levels; Tester will name specific Playwright files/assertions (CON-007). |
| VI. Cross-Repo Coherence | Compliant (N/A) | Single-repo feature (`ui/` only). No cross-repo coherence burden. |
| VII. Self-Bootstrap | Compliant (N/A) | Not the platform-building spec; ordinary feature. |
| VIII. Go, Minimal Dependencies | Compliant | No Go changes. UI must avoid new npm deps (CON-008); use existing Tailwind/react-router/react-query. |
| IX. Pipeline Governance | Compliant | Inception rules followed: source discovery documented, constraint register present, ACs have test levels, assumptions marked, brownfield workspace analyzed. |
| X. Learn From Cistern | Compliant | Role identity clear (PM, inception). Phase gate will be mechanically enforced by orchestrator. Convergence via acceptance criteria. |

No violations. No justifications needed.