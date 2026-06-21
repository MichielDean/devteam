# Feature Specification: Kanban View for Dev Team

**Feature ID**: kanban-view-for-dev-team
**Feature Branch**: `kanban-view-for-dev-team`
**Created**: 2026-06-21
**Priority**: P1
**Intake Path**: Loose Idea
**Status**: Draft

**Input**: I want to be able to have a Kanban view for all of the specs so we can show their current states in progress. Ideally we reuse components for this rather than building everything ourselves bespoke.

---

## Problem Statement

The Dev Team web UI dashboard (feature 002) displays specs as a flat card grid sorted by a single field. To understand pipeline flow — which specs are stuck in which phase, where the bottlenecks are, how work is distributed across the six phases — a team member must mentally reconstruct the pipeline from a sorted list. A Kanban view with one column per phase makes pipeline state visible at a glance: each column is a phase, each card is a spec, card movement across columns IS the pipeline progressing. This is the same data the dashboard already shows, reorganized by phase instead of by sort field.

The feature explicitly calls for component reuse over bespoke construction. The existing dashboard already has `FeatureCard`, `FeatureList`, `FeatureSummary` types, `PHASES`/`PHASE_LABELS` constants, and the `GET /api/features` endpoint. The Kanban view reuses all of these; it adds only the column layout and a view-mode toggle.

---

## Source Discovery

### Sources Reviewed

| Source | Location | Relevance |
|--------|----------|-----------|
| Feature 002 spec (Dev Team Web UI) | `specs/002-dev-team-web-ui/spec.md` | Defines the dashboard, FeatureCard, FeatureList, API contract this feature extends |
| Feature 002 plan | `specs/002-dev-team-web-ui/plan.md` | Documents frontend architecture, component tree, state management, routing |
| Existing UI components | `ui/src/components/`, `ui/src/pages/` | Reusable components: FeatureCard, FeatureList, EmptyState, Toast, ThemeToggle, ConnectionStatus |
| Existing types | `ui/src/types/index.ts` | FeatureSummary, PHASES, PHASE_LABELS, STATUS_LABELS, PRIORITY_LABELS — reused as-is |
| Existing API client | `ui/src/api/client.ts` (implied by Dashboard.tsx) | `listFeatures()` reused; no new endpoint |
| Feature domain types | `internal/feature/types.go` | Source of truth for phases, statuses, priorities |
| AGENTS.md | repo root | Minimal — no conventions to match beyond existing code patterns |
| repos.yaml | repo root | Confirms devteam is the primary repo |

### Standards and RFCs

This feature implements no protocol. No RFC, standard, or external specification governs its behavior. It is a pure UI layout feature over existing data. No test vectors or conformance suites apply.

### Internal Conventions (Binding)

The existing dashboard (feature 002) establishes conventions this feature MUST follow:

1. **Component pattern**: Function components, TypeScript, Tailwind CSS v4 classes, `data-testid` attributes on all interactive elements.
2. **State management**: React Query (`useQuery` with `queryKey: ['features']`) for server state. No new global state stores.
3. **Routing**: React Router v7. Feature detail at `/features/:id` (existing route, reused).
4. **Data source**: `GET /api/features` returns `{ features: FeatureSummary[], total_count: number }`. No new endpoint. No new request shape.
5. **Real-time updates**: Existing SSE hook (`useSSE`) invalidates the `['features']` query cache on phase changes. Kanban view benefits automatically — no new SSE wiring.
6. **Dark mode**: Tailwind `dark:` variants. All new components MUST support dark mode.
7. **Empty state**: `EmptyState` component shown when `features.length === 0` (existing pattern in Dashboard.tsx).
8. **Toasts**: `useToast()` hook for success/error notifications.
9. **Test attributes**: `data-testid` on all interactive elements (existing convention across all components).

---

## Constraint Register

| ID | Source | Type | Constraint | Verification Method |
|----|--------|------|------------|---------------------|
| CON-001 | Feature input ("reuse components") | scope | MUST reuse existing FeatureSummary type, PHASES/PHASE_LABELS constants, and listFeatures API client; MUST NOT add new API endpoints or new data types | Code review: grep for new endpoint definitions (none expected); grep for new type declarations matching FeatureSummary (none expected) |
| CON-002 | Feature 002 spec FR-006 | consistency | Dashboard displays all features with phase, status, priority. Kanban view shows the same fields per card — no field dropped, no field invented. | E2E: Kanban card displays title, status badge, priority badge, gate indicator matching FeatureCard fields |
| CON-003 | Feature 002 spec FR-010 | consistency | Empty state with call-to-action when no features exist. Kanban view MUST show the same empty state, not six empty columns. | E2E: Load Kanban view with 0 features, assert EmptyState renders (not six empty columns) |
| CON-004 | Feature 002 spec FR-036 | consistency | Dark mode support via Tailwind `dark:` variants and persisted toggle. Kanban columns and cards MUST be readable in dark mode. | E2E: Toggle dark mode, assert all column headers and cards have readable text/background |
| CON-005 | Feature 002 spec FR-037 | consistency | Usable on viewports as narrow as 375px without horizontal scrolling. Six columns do not fit at 375px — Kanban view MUST provide a horizontal-scroll container for the columns at narrow widths (column itself never shrinks below readable width). | E2E: Set viewport to 375px, assert columns are individually readable and the board scrolls horizontally (page itself does not scroll) |
| CON-006 | internal/feature/types.go | correctness | Columns correspond exactly to the six phases returned by AllPhases(): inception, planning, construction, review, testing, delivery. No fewer, no more, no different names. | Unit: assert Kanban column headers match PHASES constant exactly (6 columns, in order) |
| CON-007 | internal/feature/types.go | correctness | A feature card appears in exactly one column: the column matching `feature.current_phase`. A feature with `current_phase: "planning"` appears ONLY in the planning column. | Unit: given features with distinct current_phase values, assert each maps to exactly one column; given 2 features in planning, assert planning column has 2 cards |
| CON-008 | Feature 002 spec FR-007 | consistency | Existing dashboard supports sorting. Kanban view cards within a column MUST be sortable by the same fields (phase is implicit by column; priority, status, updated_at remain applicable). [ASSUMPTION: sort controls appear once at board level, apply within-column ordering] | E2E: click sort-by-priority, assert cards within each column reorder by priority |
| CON-009 | Feature 002 (Dashboard.tsx) | consistency | Clicking a card navigates to `/features/:id` (existing route). Kanban cards MUST use the same navigation — no new detail route. | E2E: click a Kanban card, assert navigation to `/features/<id>` (existing FeatureDetail page) |
| CON-010 | Feature 002 (useSSE) | consistency | Real-time updates: when SSE invalidates the `['features']` query, the Kanban view re-renders with updated card positions. No new SSE wiring. | E2E: trigger a phase change via API, assert the card moves to the new column within 5 seconds |
| CON-011 | Feature input | scope | View toggle between existing card-grid (List) and new Kanban (Board). List view MUST remain unchanged. Board view is additive. | E2E: toggle List→Board→List, assert both views render correctly and state persists across toggle |
| CON-012 | Feature 002 (data-testid convention) | testability | All new interactive elements have `data-testid` attributes. | Unit/grep: every button, column, card in KanbanBoard has a data-testid |
| CON-013 | Feature 002 (ConnectionStatus) | consistency | When SSE connection drops, the existing ConnectionStatus banner shows. Kanban view MUST NOT add a second connection indicator. | E2E: drop SSE connection, assert single ConnectionStatus banner (the existing one), no duplicate |
| CON-014 | internal/feature/types.go | correctness | Feature status `cancelled` and `done` are terminal. [ASSUMPTION: cancelled/done features appear in the column matching their current_phase (which may be any phase), shown with a visual terminal indicator. They are NOT hidden.] | E2E: given a cancelled feature at review phase, assert it appears in the review column with a cancelled visual indicator |
| CON-015 | Feature 002 (QuestionBadge) | consistency | Features with pending questions show a QuestionBadge in the card. Kanban cards MUST show the same QuestionBadge when `pending_questions_count > 0`. | E2E: given a feature with pending_questions_count > 0, assert QuestionBadge renders on the Kanban card |

---

## Request Analysis

- **Clarity**: Vague — "Kanban view for specs, show current states, reuse components." Needs structured exploration of column definition, card content, view toggle, mobile behavior, empty state.
- **Request type**: New feature (enhancement to existing dashboard).
- **Scope**: Single repo (devteam), single component area (frontend `ui/`), no backend changes.
- **Complexity**: Simple — clear implementation path: add a view toggle and a KanbanBoard component, reuse existing data and types.

---

## User Scenarios & Testing

### US-001: View specs as a Kanban board by phase (Priority: P1)

A team member opens the dashboard and switches from the card-grid list view to a Kanban board view. The board shows six columns — one per pipeline phase (Inception, Planning, Construction, Review, Testing, Delivery) — with feature cards placed in the column matching each feature's current phase. Each card shows the feature title, status badge, priority badge, and gate result indicator.

**Why this priority**: This is the feature. Without the board view, nothing else matters.

**Independent test**: With at least one feature in each of two distinct phases, switch to Board view and assert each feature appears in the column matching its `current_phase`.

**Acceptance scenarios**:
1. Given the dashboard with features present, when the user clicks the "Board" view toggle, then six columns render with headers Inception, Planning, Construction, Review, Testing, Delivery, and each feature card appears in exactly the column matching its `current_phase`.
2. Given a feature with `current_phase: "planning"`, when the board renders, then the card appears in the Planning column and in no other column.
3. Given the board view, when the user clicks a card, then the app navigates to `/features/:id` (the existing feature detail route).
4. Given the board view, when the user clicks the "List" view toggle, then the existing card-grid dashboard renders unchanged from its pre-002-behavior.

### US-002: See real-time pipeline movement on the board (Priority: P1)

When a feature advances through the pipeline (via CLI, API, or UI action), its card moves from one column to the next within 5 seconds, without a manual page refresh.

**Why this priority**: A static board is a screenshot. The value is watching work flow.

**Independent test**: Start processing a feature via the API, keep the board open, and assert the card moves columns as phases change.

**Acceptance scenarios**:
1. Given the board view with a feature in the Inception column, when the feature advances to planning (via API or UI), then the card moves from the Inception column to the Planning column within 5 seconds, without manual refresh.
2. Given the board view, when the SSE connection drops, then the existing ConnectionStatus banner appears at the top (no duplicate banner) and the board continues showing the last-known state (not a blank page).
3. Given the board view with SSE reconnected, when a state change arrives, then the board updates to the current state.

### US-003: Sort cards within columns (Priority: P2)

A team member can sort the cards within each column by priority, status, or last-updated time. The sort applies within each column independently (the column itself is determined by phase, which is not sortable).

**Why this priority**: A column with 20 cards in random order is as useless as a flat list. Sort makes the board scannable.

**Independent test**: With 3 features in the Planning column with distinct priorities, click sort-by-priority and assert the cards reorder within the Planning column.

**Acceptance scenarios**:
1. Given the board view with multiple cards in a column, when the user clicks the "Priority" sort control, then cards within every column reorder by priority (P1 first).
2. Given the board view with a sort applied, when the user clicks the same sort control again, then the sort direction toggles (asc/desc) within every column.
3. Given the board view, when no sort control is active, then cards within each column appear in the order returned by the API (stable, no shuffle).

### US-004: Use the board on mobile (Priority: P2)

The board is usable on a 375px-wide viewport. Six columns do not fit side-by-side at 375px, so the board scrolls horizontally within its container; the page itself does not scroll horizontally.

**Why this priority**: The dashboard already commits to 375px support (FR-037). The board must not break that commitment.

**Independent test**: Set viewport to 375px, switch to Board view, assert columns are individually readable and the board scrolls horizontally while the page does not.

**Acceptance scenarios**:
1. Given the board view at 375px viewport width, when the user views the board, then each column is at least 250px wide (readable) and the board container scrolls horizontally to reveal off-screen columns.
2. Given the board view at 375px viewport width, when the user views the page, then the page itself has no horizontal scrollbar (only the board container does).
3. Given the board view at 1440px viewport width, when the user views the board, then all six columns are visible without horizontal scrolling.

### US-005: See feature status, priority, and gate on each card (Priority: P1)

Each card on the board shows the same status badge, priority badge, and gate result indicator as the existing FeatureCard component, so a team member can scan the board and identify blocked, failed, or high-priority features without clicking through.

**Why this priority**: A card with only a title is a Kanban in name only. The status/priority/gate triad is what makes the board actionable.

**Independent test**: Given a feature with `status: "gate_blocked"`, `priority: 1`, and a failed gate, switch to Board view and assert the card shows a gate-blocked badge, P1 priority badge, and a failed-gate indicator.

**Acceptance scenarios**:
1. Given a feature with `status: "in_progress"`, `priority: 2`, and no gate result, when the board renders, then its card shows an "In Progress" status badge, a "P2 - Medium" priority badge, and no gate indicator.
2. Given a feature with `status: "gate_blocked"` and a failed gate, when the board renders, then its card shows a gate-blocked status badge and a "✗ Gate failed" indicator.
3. Given a feature with `pending_questions_count > 0`, when the board renders, then its card shows a QuestionBadge (same component as the list view).

### US-006: Empty state on the board (Priority: P1)

When no features exist, the board does not render six empty columns. It shows the existing EmptyState component with the call-to-action to create the first feature.

**Why this priority**: Six empty columns is a hostile first-run experience. The existing EmptyState already solves this.

**Independent test**: With zero features in the system, switch to Board view and assert the EmptyState renders (not six empty columns).

**Acceptance scenarios**:
1. Given zero features in the system, when the user switches to Board view, then the EmptyState component renders with the "create the first feature" call-to-action, and no column headers are shown.
2. Given zero features, when the user switches back to List view, then the same EmptyState renders (consistent behavior).

### US-007: Toggle persists across page reloads (Priority: P3)

The user's chosen view mode (List or Board) persists across page reloads via `localStorage`, so reopening the dashboard lands on the last-used view.

**Why this priority**: A minor convenience. Low priority, but cheap to implement.

**Independent test**: Switch to Board view, reload the page, assert the board renders (not the list).

**Acceptance scenarios**:
1. Given the user has selected Board view, when the user reloads the page, then the board renders (not the list).
2. Given the user has selected List view, when the user reloads the page, then the list renders.
3. Given the user has no stored preference, when the user opens the dashboard, then the List view renders (default; does not change existing behavior).

---

## Edge Cases

| # | Edge Case | Expected Behavior |
|---|---|---|
| 1 | Zero features | EmptyState renders (not six empty columns). |
| 2 | All features in one phase | One column has all cards; other five columns show a column-level empty placeholder ("No specs in <phase>"). The five empty columns are still rendered (the board shows the full pipeline shape). |
| 3 | Feature in `cancelled` status | Card appears in the column matching its `current_phase` with the cancelled status badge (red). Not hidden. |
| 4 | Feature in `done` status at delivery phase | Card appears in the Delivery column with a done badge (green). |
| 5 | Feature with `waiting_for_human` status | Card appears in its current-phase column with the waiting-for-human badge (yellow). |
| 6 | Feature with `current_phase` not matching any known phase | [ASSUMPTION: cannot happen — API guarantees current_phase is one of the six phases. If it did, the card would not render and a console error would log. Defensive code checks phase membership.] |
| 7 | Very long feature titles | Card title truncates with ellipsis (existing FeatureCard pattern: `truncate` Tailwind class). |
| 8 | 100+ features | Board renders all cards. [ASSUMPTION: no virtualization for MVP. 100 cards across 6 columns is ~17 per column, performant. Revisit virtualization if count exceeds 500.] |
| 9 | Viewport at 375px | Board container scrolls horizontally; columns are individually readable (≥250px). Page does not scroll horizontally. |
| 10 | SSE connection drops | Existing ConnectionStatus banner shows. Board shows last-known state. No duplicate banner. |
| 11 | Concurrent CLI action changes a feature's phase | SSE event invalidates `['features']` query; board re-renders; card moves to new column within 5s. |
| 12 | User toggles Board→List mid-SSE-event | View switches immediately; List view reflects the latest data (same query cache). No data loss. |
| 13 | Dark mode | All columns, cards, badges, sort controls readable in dark mode via Tailwind `dark:` variants. |
| 14 | Feature with failed gate in a non-terminal phase | Card shows "✗ Gate failed" indicator in its current-phase column. The feature has not been recirculated yet, so it stays in the column. |

---

## Requirements

### Functional Requirements

**View Toggle**

- **FR-001**: The dashboard MUST provide a view-mode toggle with two options: "List" (existing card grid) and "Board" (Kanban columns).
  Source: US-001, US-007
- **FR-002**: The toggle MUST default to "List" when no persisted preference exists, preserving existing dashboard behavior.
  Source: US-007
- **FR-003**: The selected view mode MUST persist in `localStorage` under the key `devteam-dashboard-view` and be restored on page load.
  Source: US-007
- **FR-004**: Toggling between views MUST NOT refetch data — both views consume the same `['features']` React Query cache.
  Source: US-001

**Board Layout**

- **FR-005**: The Board view MUST render exactly six columns, one per phase, in pipeline order: Inception, Planning, Construction, Review, Testing, Delivery. Column headers use the existing `PHASE_LABELS` constant.
  Source: US-001, CON-006
- **FR-006**: Each feature card MUST appear in exactly the column matching its `current_phase` field. A feature appears in no more and no fewer than one column.
  Source: US-001, CON-007
- **FR-007**: Each column MUST display a count badge showing the number of cards in that column.
  Source: US-001
- **FR-008**: A column with zero features MUST display a "No specs in \<phase\>" placeholder inside the column (the column itself still renders).
  Source: Edge case 2

**Card Content**

- **FR-009**: Each card MUST display: feature title (truncated with ellipsis if long), status badge (using `STATUS_LABELS` and existing badge color classes), priority badge (using `PRIORITY_LABELS`), and gate result indicator ("✓ Gate passed" / "✗ Gate failed" / none) — matching the existing FeatureCard component field-for-field.
  Source: US-005, CON-002
- **FR-010**: Each card with `pending_questions_count > 0` MUST display the existing `QuestionBadge` component.
  Source: US-005, CON-015
- **FR-011**: Clicking a card MUST navigate to `/features/:id` via React Router `<Link>` (same route as the existing FeatureCard).
  Source: US-001, CON-009
- **FR-012**: Card visual treatment for terminal statuses (`cancelled`, `done`) MUST use the existing status color mapping (red for cancelled, green for done). No new color tokens.
  Source: Edge case 3, 4, CON-002

**Sorting**

- **FR-013**: The Board view MUST provide sort controls for priority, status, and updated_at — the same fields as the existing FeatureList, minus phase (phase is implicit by column).
  Source: US-003, CON-008
- **FR-014**: The sort MUST apply within each column independently. Clicking "Priority" reorders cards within every column by priority.
  Source: US-003
- **FR-015**: Repeated clicks on the same sort control MUST toggle direction (asc/desc).
  Source: US-003
- **FR-016**: With no sort control active, cards within a column MUST appear in the order returned by the API (stable).
  Source: US-003

**Responsive Behavior**

- **FR-017**: At viewport widths where six columns do not fit (below ~1200px), the board container MUST scroll horizontally; each column MUST be at least 250px wide.
  Source: US-004, CON-005
- **FR-018**: The page itself MUST NOT scroll horizontally at any viewport width ≥ 375px. Only the board container scrolls.
  Source: US-004, CON-005
- **FR-019**: At viewport widths where six columns fit (≥~1200px), all columns MUST be visible without horizontal scrolling.
  Source: US-004

**Empty State**

- **FR-020**: When `features.length === 0`, the Board view MUST render the existing `EmptyState` component and MUST NOT render column headers.
  Source: US-006, CON-003

**Real-Time Updates**

- **FR-021**: The Board view MUST reflect feature phase changes within 5 seconds, driven by the existing `useSSE` hook invalidating the `['features']` query. No new SSE wiring.
  Source: US-002, CON-010
- **FR-022**: On SSE connection loss, the Board view MUST NOT add its own connection indicator; the existing `ConnectionStatus` banner remains the single source of connection status.
  Source: US-002, CON-013

**Dark Mode**

- **FR-023**: All Board view components (columns, cards, headers, badges, sort controls) MUST support dark mode via Tailwind `dark:` variants, matching the existing dashboard.
  Source: CON-004

**Testability**

- **FR-024**: Every interactive element in the Board view (view toggle, sort controls, cards, columns) MUST have a `data-testid` attribute.
  Source: CON-012

### Key Entities

- **Board**: Container rendering six Columns left-to-right in pipeline order. Reuses `FeatureSummary[]` data; no new entity.
- **Column**: A phase-labeled vertical container holding Cards whose `current_phase` matches the column's phase. Header uses `PHASE_LABELS[phase]`. Count badge shows card count.
- **Card**: A compact feature summary, reusing the fields and visual language of the existing `FeatureCard` component. May be a new component (`KanbanCard`) or the existing `FeatureCard` reused directly — [ASSUMPTION: a new `KanbanCard` component is created to allow column-optimized layout, but it reuses the same data type and badge classes as `FeatureCard`].
- **ViewToggle**: A two-option control (List / Board) that switches the dashboard's main content area. Persists selection to `localStorage`.
- **SortControls**: Reuse the existing sort button pattern from `FeatureList.tsx`, adapted to apply within-column sorting.

No new API entities. No new database entities. No new domain types.

### State Transitions

**View mode state**: `list` ↔ `board`
- `list` → `board`: user clicks Board toggle.
- `board` → `list`: user clicks List toggle.
- Persisted to `localStorage['devteam-dashboard-view']`.
- On load: read `localStorage`; if `board`, start in `board`; if `list` or unset or invalid, start in `list`.
- Invalid transitions: none (only two states).

**Card position state** (derived, not stored):
- A card's column is a pure function of `feature.current_phase`. When `current_phase` changes (via API/SSE), the card's column changes. No local state.
- Invalid transitions: a card cannot be in two columns; a card cannot be in no column (unless `current_phase` is invalid, which is impossible per API contract).

### Non-Functional Requirements

- **NFR-001**: Board view MUST render within 500ms for up to 100 features (same target as dashboard NFR-001).
- **NFR-002**: Toggle between List and Board MUST complete in under 200ms perceived latency (no refetch; same cache).
- **NFR-003**: Card movement on SSE event MUST be visible within 5 seconds of the state change (same as dashboard NFR-003).
- **NFR-004**: The frontend bundle size increase MUST be under 20KB gzipped (the feature is one new component plus a toggle; no new dependencies).
- **NFR-005**: No new npm dependencies. The feature MUST use only packages already in `ui/package.json` (React, React Router, TanStack Query, Tailwind).
  Source: CON-001, "reuse components"

## Success Criteria

- **SC-001**: Given a dashboard with features in at least 3 distinct phases, when the user switches to Board view, then six columns render and each feature appears in the column matching its `current_phase`, verified by visually inspecting the board and asserting each card's column header equals the card's `current_phase` label.
- **SC-002**: Given the Board view, when a feature advances from inception to planning via the API, then its card moves from the Inception column to the Planning column within 5 seconds, without manual refresh.
- **SC-003**: Given the Board view at 375px viewport width, then all six columns are reachable by horizontal scroll and the page itself has no horizontal scrollbar.
- **SC-004**: Given the Board view with zero features, then the EmptyState component renders with a create-feature call-to-action and no column headers are present.
- **SC-005**: Given the user selects Board view and reloads the page, then the Board view renders (preference persisted).
- **SC-006**: Given the Board view in dark mode, then all columns, cards, and badges are readable (text contrast meets WCAG AA against backgrounds).
- **SC-007**: Given the Board view, when the user clicks any card, then navigation occurs to `/features/:id` (existing detail page).
- **SC-008**: The frontend bundle gzipped size increase is under 20KB.

## Error Scenarios

| User Action | Success | Error Condition | Expected Response |
|---|---|---|---|
| Switch to Board view | Board renders with columns and cards | `listFeatures` query is loading | Skeleton/spinner in each column (loading state, not blank) |
| Switch to Board view | Board renders | `listFeatures` query errored | Error message in board area ("Failed to load features: \<message\>"), columns do not render |
| View board with 0 features | EmptyState renders | (not an error) | EmptyState, 200 OK from API with `[]` |
| Click a card | Navigate to `/features/:id` | Feature deleted between render and click | Existing FeatureDetail page shows 404 / "Feature not found" (existing behavior, unchanged) |
| SSE event arrives | Card moves column | SSE event malformed | [ASSUMPTION: existing useSSE hook logs and ignores malformed events; board shows last-known good state] |
| Persist preference | `localStorage` write succeeds | `localStorage` full / disabled | Silently fall back to in-memory state; default to List on next load. Console warning, no user-facing error. |
| Toggle view | View switches | (no error path — pure client state) | N/A |

**Empty state behavior (explicit)**:
- `features: []` → EmptyState component, no columns. 200 OK from API.
- Column with 0 cards (but other columns have cards) → column renders with "No specs in \<phase\>" placeholder. NOT an error.
- `phase_states` empty for a feature → not applicable (board uses `current_phase`, not `phase_states`).

**Boundary conditions**:
- `current_phase`: must be one of 6 phases. API guarantees this. Client defensive check: if a phase is not in `PHASES`, log a console error and skip the card.
- `title`: any length; UI truncates with ellipsis (existing pattern).
- `priority`: 1, 2, or 3. API guarantees. Client renders `PRIORITY_LABELS[priority]` or fallback `P{priority}`.
- `pending_questions_count`: integer ≥ 0. If > 0, QuestionBadge renders.

## Assumptions and Scope Boundaries

### In Scope

- A new `KanbanBoard` component (columns + cards) in `ui/src/components/`.
- A new `ViewToggle` component (List / Board) in `ui/src/components/`.
- A new `KanbanCard` component (or reuse of `FeatureCard` — Architect's call) in `ui/src/components/`.
- View-mode state persisted to `localStorage`.
- Board-level sort controls (within-column sorting).
- Horizontal-scroll behavior for narrow viewports.
- Dark mode support for all new components.
- `data-testid` attributes on all new interactive elements.

### Out of Scope

- Drag-and-drop card movement between columns (the pipeline advances cards via API actions, not user drag — [ASSUMPTION: user wants to OBSERVE flow, not manually move cards. If drag is wanted, it is a separate feature.]).
- Backend API changes (no new endpoints, no new request/response shapes).
- New data types or domain entities.
- New npm dependencies.
- Filtering features by phase, status, or priority (the board is the filter — columns ARE phase groupings. Additional filters are a separate feature.).
- WIP limits per column (Kanban WIP-limit convention is out of scope for MVP).
- Card detail expansion on click (cards navigate to the existing detail page; no inline expansion).
- Column collapse/expand (all six columns always render when features exist).
- Virtualization for large counts ([ASSUMPTION: not needed for ≤100 features. Revisit at 500+.]).
- Authentication / authorization (inherits dashboard's local-only, no-auth model).

### Assumptions

- [ASSUMPTION: "reuse components" means reuse existing data types, API client, badge classes, QuestionBadge, EmptyState, and the general component pattern. A new `KanbanCard` component is acceptable as long as it reuses the same data type (`FeatureSummary`) and visual language (same badge classes, same STATUS_LABELS/PRIORITY_LABELS). The existing `FeatureCard` is grid-optimized; a column-optimized card variant is consistent with "reuse" as long as it's not a from-scratch redesign.]
- [ASSUMPTION: the user wants to OBSERVE pipeline flow, not manually move cards via drag-and-drop. Kanban here means "columns-by-status visualization", not "interactive task board". If drag is wanted, it's a separate feature.]
- [ASSUMPTION: board-level sort controls apply within-column. An alternative (per-column sort controls) is rejected as redundant — one sort control set is simpler and matches the existing dashboard pattern.]
- [ASSUMPTION: the default view remains List (existing behavior) to avoid changing the dashboard for users who don't opt in. Board is opt-in via toggle.]
- [ASSUMPTION: no virtualization is needed for ≤100 features. The dashboard NFR-001 targets 100 features; the board inherits this.]
- [ASSUMPTION: `current_phase` is always one of the 6 known phases per the API contract. Defensive code handles the impossible case by logging and skipping.]
- [ASSUMPTION: cancelled and done features are NOT hidden. They appear in the column matching their `current_phase` with terminal status badges. Hiding them would misrepresent pipeline state.]
- [ASSUMPTION: localStorage key `devteam-dashboard-view` does not collide with existing keys. Checked: Dashboard.tsx uses no localStorage; ThemeToggle uses a theme key. No collision.]