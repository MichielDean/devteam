# Feature Specification: Kanban View

**Feature ID**: kanban-view
**Feature Branch**: `kanban-view`
**Created**: 2026-06-21
**Status**: Inception
**Priority**: P1
**Intake Path**: Loose Idea

## Description

Add a Kanban board view to the Dev Team web UI that visualizes all feature specs as cards organized into columns by their current pipeline phase. Features that have not yet started the pipeline appear in a "Backlog" column. The view reuses existing UI components (FeatureCard, feature data, Tailwind styles) and the existing `GET /api/features` endpoint rather than introducing new backend APIs or building bespoke board infrastructure from scratch.

The Kanban view is an alternative presentation of the same data already shown by the Dashboard's `FeatureList`. It adds a phase-grouped board layout so users can see pipeline progress across all specs at a glance.

## Source Discovery

### Governing Sources

This feature is a UI presentation layer over existing Dev Team data. There is no external RFC, protocol standard, or conformance test vector that governs a Kanban board. The governing sources are internal conventions:

| Source | What it governs |
|--------|-----------------|
| `ui/src/types/index.ts` | `FeatureSummary` shape, `PHASES` constant, `STATUS_LABELS`, `PRIORITY_LABELS` — the canonical phase and status enums the board must use |
| `ui/src/api/client.ts` | `listFeatures()` returns `FeatureListResponse { features: FeatureSummary[], total_count: number }` — the single data source for the board |
| `ui/src/components/FeatureCard.tsx` | Existing card component to reuse for board cards |
| `internal/feature/types.go` | Phase enum (`inception`, `planning`, `construction`, `review`, `testing`, `delivery`) and Status enum (`draft`, `in_progress`, `gate_blocked`, `passed`, `failed`, `done`, `recirculated`, `cancelled`, `waiting_for_human`) — wire values the API returns |
| `internal/api/dto.go` + `server.go` | `GET /api/features` returns `{"features":[...],"total_count":N}` with empty `features` as `[]` (never null) |

### Constraint Register

| ID | Source | Type | Constraint | Verification |
|----|--------|------|------------|-------------|
| CON-001 | `ui/src/types/index.ts` `PHASES` | correctness | Board columns are the 6 pipeline phases in canonical order: inception, planning, construction, review, testing, delivery — no invented or reordered columns | Column order assertion |
| CON-002 | Feature input | correctness | A "Backlog" column contains features whose pipeline has not started (phase = inception AND status = draft, i.e. no phase has entered in_progress) | Backlog grouping test |
| CON-003 | `ui/src/api/client.ts` `listFeatures` | correctness | Board data comes exclusively from the existing `GET /api/features` response; no new backend endpoint is introduced | Endpoint inventory check |
| CON-004 | `internal/api/dto.go` | correctness | Empty feature list serializes as `[]` not `null`; board renders empty columns when no features exist in a phase | Empty state test |
| CON-005 | `ui/src/components/FeatureCard.tsx` | reuse | Feature cards on the board reuse the existing `FeatureCard` component (or its visual contract: title, status badge, phase badge, priority badge, gate indicator, updated date) | Component import check |
| CON-006 | Feature input | reuse | Reuse existing components and Tailwind styling patterns instead of building bespoke board infrastructure; no new UI dependency added to `package.json` | Dependency diff check |
| CON-007 | `ui/src/App.tsx` routing | consistency | Kanban view is reachable via navigation (route or view toggle) alongside the existing Dashboard list view | Navigation test |
| CON-008 | Existing dark mode support (`ThemeToggle`) | consistency | Board supports dark mode via existing Tailwind `dark:` variants, matching the rest of the UI | Dark mode render test |
| CON-009 | `internal/feature/types.go` Status enum | correctness | A feature with terminal status (`done`, `cancelled`) is placed in its `current_phase` column, not hidden — the board shows all features regardless of status | Terminal status placement test |
| CON-010 | `ui/src/pages/Dashboard.tsx` `feature-count-badge` | consistency | Total feature count badge remains visible and correct when Kanban view is active | Count badge assertion |
| CON-011 | Existing `data-testid` convention | testability | Board and columns expose stable `data-testid` attributes for E2E selectors (e.g. `kanban-board`, `kanban-column-{phase}`, `kanban-column-backlog`) | Testid presence check |

## User Scenarios & Testing

### User Story 1 - See all features organized by pipeline phase (Priority: P1)

As a developer using Dev Team, I want to view a Kanban board where each column is a pipeline phase and each card is a feature, so I can see the state of all specs and what kind of progress they have at a glance.

**Why this priority**: The feature request is explicitly this. Without the board, the feature does not exist.

**Independent Test**: With at least one feature in each of inception, planning, and delivery phases, load the Kanban view and verify each feature appears in the column matching its `current_phase`.

### User Story 2 - Not-yet-started features appear in Backlog (Priority: P1)

As a developer, I want features that have not started the pipeline to appear in a "Backlog" column, separate from features actively in a phase, so I can distinguish unstarted work from in-progress work.

**Why this priority**: Explicitly called out in the feature input ("Anything not started yet should be in the backlog").

**Independent Test**: Create a feature but do not run any phase (status = `draft`, current_phase = `inception`). Load the Kanban view and verify the feature appears in the Backlog column, not the Inception column.

### User Story 3 - Switch between list view and Kanban view (Priority: P1)

As a developer, I want to toggle between the existing list/dashboard view and the new Kanban view, so I can choose the layout that suits my current task without losing access to either.

**Why this priority**: The Kanban view is additive — it must not replace the existing Dashboard. Users need both.

**Independent Test**: From the Dashboard, navigate to the Kanban view and back, verifying both views render their expected content and the total feature count badge stays consistent.

### User Story 4 - Click a card to open feature detail (Priority: P1)

As a developer, I want to click a feature card on the Kanban board and navigate to that feature's detail page, so I can inspect or act on a feature directly from the board.

**Why this priority**: Cards are useless if they don't link to the work. This matches the existing `FeatureCard` behavior (it renders a `<Link>`).

**Independent Test**: With at least one feature on the board, click its card and verify navigation to `/features/{id}`.

### User Story 5 - Empty board renders cleanly with no console errors (Priority: P2)

As a developer with zero features, I want the Kanban view to render all columns as empty with an empty-state message, so the board doesn't break or show a blank page when there's no data.

**Why this priority**: Empty state correctness prevents the #1 agent-generated UI bug (null vs empty array) and a blank-page regression. P2 because it only triggers when the system has no features, which is rare after first use.

**Independent Test**: With zero features in the system, load the Kanban view and verify every column renders with an empty-state message and no browser console errors.

### User Story 6 - Board reflects live updates during processing (Priority: P3)

As a developer, when a feature advances phases while I'm viewing the Kanban board, the card moves to the new column without a full page reload, so the board stays current during autonomous processing.

**Why this priority**: Nice-to-have. The existing Dashboard already invalidates queries on mutations; the board can piggyback on the same `useQuery` cache. P3 because manual refresh already works and this is a polish improvement.

**Independent Test**: With the board open and a feature processing, trigger a phase advance and verify the card moves columns without a manual reload.

## Functional Requirements

- **FR-001**: The system shall render a Kanban board with 7 columns: Backlog, Inception, Planning, Construction, Review, Testing, Delivery, in that left-to-right order. (Source: US-001, US-002, CON-001)
- **FR-002**: The system shall place a feature in the Backlog column when its `status` is `draft` and `current_phase` is `inception` (i.e. no phase has entered `in_progress`). (Source: US-002, CON-002)
- **FR-003**: The system shall place a feature in the column matching its `current_phase` (inception → delivery) when it is not in Backlog (status is anything other than `draft`-with-`inception`, including `done`, `cancelled`, `in_progress`, `gate_blocked`, `passed`, `failed`, `recirculated`, `waiting_for_human`). (Source: US-001, CON-009)
- **FR-004**: The system shall source all board data from the existing `listFeatures()` API client function, which calls `GET /api/features`. No new backend endpoint shall be introduced. (Source: US-001, CON-003)
- **FR-005**: Each feature card on the board shall reuse the existing `FeatureCard` component (title, status badge, phase badge, priority badge, gate indicator, updated date, link to detail). (Source: US-004, CON-005)
- **FR-006**: The system shall provide a navigation affordance (view toggle or route) on the Dashboard to switch to the Kanban view, and an affordance on the Kanban view to return to the Dashboard list. (Source: US-003, CON-007)
- **FR-007**: The system shall preserve the total feature count badge across both views. (Source: US-003, CON-010)
- **FR-008**: The system shall render each column with a header showing the column name and a count of cards in that column. (Source: US-001)
- **FR-009**: The system shall render an empty-state message in each column that contains zero features (e.g. "No features in this phase"). (Source: US-005, CON-004)
- **FR-010**: The system shall support dark mode on the board using existing Tailwind `dark:` variants consistent with the rest of the UI. (Source: CON-008)
- **FR-011**: The board shall not add any new runtime dependency to `ui/package.json`; it must be built from existing React, react-router, @tanstack/react-query, and Tailwind primitives. (Source: CON-006)
- **FR-012**: The board and its columns shall expose stable `data-testid` attributes: `kanban-board`, `kanban-column-backlog`, `kanban-column-inception`, `kanban-column-planning`, `kanban-column-construction`, `kanban-column-review`, `kanban-column-testing`, `kanban-column-delivery`. (Source: CON-011)
- **FR-013**: The board shall remain horizontally scrollable on narrow viewports so all 7 columns are reachable without overlapping or clipping. (Source: US-001)
- **FR-014**: The board shall refresh its data via the existing react-query `useQuery(['features'])` cache, so mutations that invalidate that cache (create, advance, recirculate, cancel) propagate to the board. (Source: US-006)

## Key Entities and Relationships

This feature introduces no new persistent entities. It is a view over existing data:

- **FeatureSummary** (existing, from `GET /api/features`): the card entity.
  - `id`, `title`, `status`, `priority`, `current_phase`, `updated_at`, `gate_result`, `pending_questions_count`
- **Column**: a derived grouping, not a stored entity. A column is identified by a phase key (or `backlog`) and contains the subset of `FeatureSummary[]` whose `current_phase` and `status` map to that key.
- **Board**: the set of all 7 columns, derived from a single `FeatureListResponse`.

### Derived grouping rule

```
backlog      := features where status == 'draft' AND current_phase == 'inception'
inception    := features where current_phase == 'inception' AND NOT (status == 'draft')
planning     := features where current_phase == 'planning'
construction := features where current_phase == 'construction'
review       := features where current_phase == 'review'
testing      := features where current_phase == 'testing'
delivery     := features where current_phase == 'delivery'
```

Every feature appears in exactly one column. A feature in `delivery` with `status == 'done'` still appears in the Delivery column (CON-009).

### State transitions

This feature does not change feature state. Feature state transitions remain governed by `internal/feature/feature.go`:
- draft → in_progress → gate_blocked/passed/failed → recirculated → ... → done | cancelled

The board only observes and reflects these transitions; it does not cause them.

## Success Criteria

- **SC-001**: Given a system with features spread across inception, planning, and delivery phases, when the user opens the Kanban view, then each feature appears in the column matching its `current_phase`, and the Backlog column contains only features with `status == 'draft'` and `current_phase == 'inception'`.
- **SC-002**: Given the Dashboard, when the user activates the Kanban view affordance, then the board renders with 7 columns in the order Backlog, Inception, Planning, Construction, Review, Testing, Delivery, and the total feature count badge matches the Dashboard count.
- **SC-003**: Given a feature card on the Kanban board, when the user clicks it, then the browser navigates to `/features/{id}`.
- **SC-004**: Given a system with zero features, when the user opens the Kanban view, then all 7 columns render with an empty-state message, the board does not crash, and the browser console has no errors.
- **SC-005**: Given the UI dependency list, when the Kanban view is implemented, then `ui/package.json` has no new dependencies added compared to the pre-feature state.
- **SC-006**: Given the board in dark mode, when the user toggles the existing theme switch, then all columns and cards render with dark-mode styling consistent with the rest of the app.

## Error Scenarios

| User Action | Success | Error Condition | Expected Response |
|---|---|---|---|
| Open Kanban view | 200, board renders with columns and cards | `GET /api/features` returns 500 | Board renders columns with a per-board error banner: "Failed to load features: {message}" and a retry affordance; no blank page, no uncaught exception |
| Open Kanban view (empty system) | 200, all columns render empty-state message | (no error — empty is success) | 200, `features: []`, each column shows "No features in this phase" |
| Click a feature card | Navigate to `/features/{id}` | Feature `id` no longer exists (deleted between load and click) | Navigate to `/features/{id}`; existing FeatureDetail page handles 404 with its own not-found state (unchanged behavior) |
| Toggle to Kanban while a query is in flight | Board shows loading state (spinner per existing pattern) | Query error mid-flight | Error banner as above; columns render empty |
| Process a feature (advance) while board open | Card moves to new column after cache invalidation | Phase advance API returns 409 / gate blocked | Existing toast/error handling from Dashboard applies; board card stays in current column, gate badge reflects failure |

## Empty State Behavior

- **No features at all**: `features: []` from API. Board renders all 7 columns, each with "No features in this phase" and a count of 0. The total count badge shows 0. No console errors.
- **No features in a given phase, but features exist elsewhere**: that specific column shows "No features in this phase" with count 0; other columns render their cards normally.
- **Backlog empty**: Backlog column shows "No features waiting to start" with count 0.

[ASSUMPTION: exact empty-state copy is left to the Architect/Developer; the constraint is that each column has a non-blank, non-error empty state. Suggested copy is documented above but not mandatory verbatim.]

## Assumptions and Scope Boundaries

### In scope
- New React page/component `KanbanBoard` (or equivalent) under `ui/src/`.
- Navigation affordance between Dashboard list and Kanban board (view toggle in the Dashboard header or a dedicated route — Architect decides).
- Column headers with per-column card counts.
- Reuse of `FeatureCard` for cards.
- Dark mode support.
- E2E (Playwright) tests for board rendering, navigation, empty state.
- `data-testid` attributes for all board elements.

### Out of scope
- Drag-and-drop card movement between columns (the board is read-only; phase changes happen via the existing Run/Advance/Recirculate actions on the detail page).
- Card creation directly from the board (intake stays on the Dashboard / detail page).
- Filtering or search within columns (the existing FeatureList sort controls are not required on the board).
- Per-column WIP limits.
- Backend API changes. No new endpoints, no DTO changes, no new query params.
- Mobile-native app or non-web clients.
- Real-time card animation beyond standard react-query refetch behavior.

### Assumptions
- [ASSUMPTION: The existing `GET /api/features` response shape (`FeatureListResponse { features: FeatureSummary[], total_count }`) is sufficient for the board. No per-phase server-side filtering is needed because the feature count is small (tens, not thousands) and client-side grouping is fast enough.]
- [ASSUMPTION: "Not started" means `status == 'draft'` AND `current_phase == 'inception'`. A freshly intake'd feature has both per `internal/feature/feature.go` line 82–93. If the team later adds a pre-inception phase, the Backlog rule must be revisited.]
- [ASSUMPTION: Terminal features (`done`, `cancelled`) remain visible on the board in their `current_phase` column. If the team wants to hide them, that's a separate feature.]
- [ASSUMPTION: The board reuses the existing react-query `['features']` cache key so it shares data with the Dashboard and stays in sync without a second fetch.]
- [ASSUMPTION: Navigation is a view toggle (e.g. a "Board / List" segmented control in the Dashboard header) rather than a separate top-level route. Either is acceptable; the Architect picks. The constraint is that both views remain reachable from each other.]
- [ASSUMPTION: Horizontal scroll is acceptable on narrow viewports. A responsive collapsed-column design is out of scope for this feature.]