# Feature Specification: Kanban View

**Feature Branch**: `kanban-view`

**Created**: 2026-06-22

**Status**: Draft

**Input**: User description: "Add a Kanban board view to the Dev Team UI that shows features as cards organized by phase. Features displayed in columns (Inception, Planning, Construction, Review, Testing, Delivery). Cards show title, priority, status. Click card to navigate to detail page. Toggle between list view and Kanban board view."

## Workspace Summary

**Brownfield** — Dev Team platform repository at `/home/lobsterdog/source/devteam`.

**Stack**:
- Backend: Go (`internal/`, `cmd/devteam/`), SQLite store (`internal/db/`)
- Frontend: React 18 + TypeScript + Vite (`ui/`), Tailwind CSS, `@tanstack/react-query`, `react-router`
- Tests: Playwright E2E (`ui/e2e/app.spec.ts`), Go tests (`*_test.go`)

**Existing relevant code**:
- `ui/src/pages/Dashboard.tsx` — landing page, renders `FeatureList`, has "+ New Feature" button, feature-count badge
- `ui/src/components/FeatureList.tsx` — grid of `FeatureCard`, sort controls (phase/priority/status/updated_at)
- `ui/src/components/FeatureCard.tsx` — card as `<Link to={/features/:id}>`, shows title, id, status badge, phase badge, priority badge, gate result, updated date, QuestionBadge for pending questions
- `ui/src/api/client.ts` — `listFeatures()` returns `FeatureListResponse{features: FeatureSummary[], total_count}`. No new endpoint needed — Kanban uses the same `GET /api/features` response.
- `ui/src/types/index.ts` — `FeatureSummary` (id, title, status, priority, current_phase, updated_at, gate_result, pending_questions_count), `PHASES` const, `PHASE_LABELS`, `STATUS_LABELS`, `PRIORITY_LABELS`
- `ui/src/components/EmptyState.tsx` — empty-features state

**Conventions to follow**:
- `data-testid` attributes on every interactive/observable element (pattern: `feature-card-${id}`, `feature-list`, `dashboard-page`)
- Dark mode via `dark:` Tailwind variants
- Status colors: `bg-{color}-100 text-{color}-800 dark:bg-{color}-900 dark:text-{color}-200`
- Existing phase enum order: inception → planning → construction → review → testing → delivery
- Status enum: draft, in_progress, gate_blocked, passed, failed, done, recirculated, cancelled, waiting_for_human

**No constitution.md** found in repo root or `.specify/`. Constitution compliance check: N/A.

**No external RFC/standard** governs a Kanban board UI. Constraint register derives from internal UI conventions only (see below).

## User Scenarios & Testing

### User Story 1 — Toggle to Kanban board view (Priority: P1)

A user viewing the Dashboard can switch from the existing list/grid layout to a Kanban board layout that renders features as cards organized into columns by current phase. The user can switch back to the list view at any time. The selected view preference is remembered across page reloads.

**Why this priority**: The toggle is the entry point to the entire feature. Without it, the Kanban board is unreachable. Independently testable: a user can toggle, see columns render, toggle back — no dependency on card content fidelity.

**Independent Test**: Load Dashboard, click "Board" toggle, verify six phase columns render, click "List" toggle, verify the original FeatureList grid renders.

**Acceptance Scenarios**:
1. **Given** the Dashboard has loaded with features present, **When** the user clicks the "Board" view toggle, **Then** the page renders a Kanban board with six columns labelled Inception, Planning, Construction, Review, Testing, Delivery.
2. **Given** the Kanban board is displayed, **When** the user clicks the "List" view toggle, **Then** the page renders the existing FeatureList grid with sort controls.
3. **Given** the user has selected "Board" view, **When** the user reloads the page, **Then** the board is displayed by default (preference persisted in localStorage).

---

### User Story 2 — Features appear in their phase column as cards (Priority: P1)

When the Kanban board is displayed, every non-draft feature is rendered as a card inside the column matching its `current_phase`. Each card shows the feature title, priority badge, status badge, pending-questions badge (if any), and is clickable to navigate to the feature detail page. Draft features appear in a "Backlog" column prepended to the board.

**Why this priority**: This is the core value of the feature — visualizing pipeline state. Independently testable: a user can look at the board and verify each feature appears in the column matching its API-returned `current_phase`.

**Independent Test**: Load the board, for each feature returned by `GET /api/features` verify a card exists in the column labelled with that feature's `current_phase` (or Backlog for draft features).

**Acceptance Scenarios**:
1. **Given** features exist across multiple phases, **When** the board renders, **Then** each feature appears in the column whose label matches its `current_phase`.
2. **Given** a feature with status `draft`, **When** the board renders, **Then** the feature appears in the "Backlog" column (shown before Inception).
3. **Given** a feature card is displayed, **When** the user clicks the card, **Then** the browser navigates to `/features/:id`.
4. **Given** a feature has `pending_questions_count > 0`, **When** its card renders, **Then** the QuestionBadge is visible on the card.

---

### User Story 3 — Blocked and completed features are visually distinguishable (Priority: P2)

Cards whose status indicates a blocked state (`gate_blocked`, `failed`, `recirculated`, `waiting_for_human`) render with a distinct visual marker (amber/red left border or badge) so the user can scan the board for stuck features. Cards whose status is `done` render with a "Done" visual treatment (green/gray, strikethrough title optional) in the Delivery column. Cancelled features are excluded from the board.

**Why this priority**: Distinguishing blocked vs. in-progress is what makes a Kanban board more useful than a flat list. Independently testable: seed features in blocked and done states, verify visual markers.

**Independent Test**: Render the board with one `gate_blocked` feature and one `done` feature; verify the blocked card has the blocked visual treatment and the done card has the done visual treatment.

**Acceptance Scenarios**:
1. **Given** a feature with status `gate_blocked`, `failed`, `recirculated`, or `waiting_for_human`, **When** its card renders, **Then** the card displays a distinct blocked-state visual marker (amber/red border or badge) not present on `in_progress`/`passed` cards.
2. **Given** a feature with status `done` in the Delivery column, **When** its card renders, **Then** the card displays a "Done" visual treatment distinguishing it from in-progress cards in the same column.
3. **Given** a feature with status `cancelled`, **When** the board renders, **Then** no card for that feature is displayed.

---

### User Story 4 — Empty columns and empty board states (Priority: P2)

When a phase column has no features, the column renders with a visible empty state ("No features in {phase}") rather than disappearing or rendering blank. When the board has no features at all (all features cancelled or none exist), the board renders an empty-state message consistent with the existing `EmptyState` component, and the view toggle remains accessible.

**Why this priority**: Empty states are a known agent-failure-mode (rendering `null` instead of `[]`). Independently testable: load the board in a workspace with zero non-cancelled features and verify the empty state.

**Independent Test**: Load Dashboard in a workspace with no features; toggle to Board; verify the empty board state renders and the toggle back to List still works.

**Acceptance Scenarios**:
1. **Given** a phase column has zero features, **When** the board renders, **Then** the column displays "No features in {phase label}" text inside the column body.
2. **Given** the workspace has zero non-cancelled features, **When** the user toggles to Board view, **Then** the board area displays an empty-state message and the "List" toggle remains clickable.

---

### User Story 5 — View preference persistence and accessibility (Priority: P3)

The view toggle is keyboard accessible (tab-focusable, Enter/Space to activate), has an ARIA label describing its purpose, and the selected view is persisted to `localStorage` under a stable key (`devteam:view-mode`). The board layout is responsive: on narrow viewports the six phase columns scroll horizontally rather than collapsing the board.

**Why this priority**: Polish — persistence and a11y. Independently testable: tab to the toggle, activate via keyboard, reload, verify preference persisted.

**Independent Test**: Tab through the Dashboard header to reach the view toggle, activate with Enter, reload the page, verify the persisted view is active.

**Acceptance Scenarios**:
1. **Given** the Dashboard has loaded, **When** the user presses Tab until focus reaches the view toggle, **Then** the toggle is focusable and operable with Enter/Space.
2. **Given** the user selects "Board", **When** the page is reloaded, **Then** `localStorage.getItem('devteam:view-mode')` returns `'board'` and the board is displayed.
3. **Given** the viewport is narrower than the combined width of six columns, **When** the board renders, **Then** the board container scrolls horizontally and all six columns remain at their fixed minimum width.

---

### Edge Cases

- **Feature in `waiting_for_human` status**: Treated as blocked — card shows blocked visual marker in its current phase column. Pending-questions badge also visible.
- **Feature with `gate_result` present**: Card may show the gate pass/fail indicator consistent with existing `FeatureCard` behavior.
- **Feature with extremely long title**: Card title truncates (existing `truncate` class on FeatureCard title).
- **Feature with `priority` outside 1-3**: Card renders with the raw `P{priority}` fallback (existing behavior in FeatureCard).
- **Many features in one column (e.g. 50+ in Inception)**: Column body scrolls vertically within the column; column header is sticky so the phase label remains visible. All cards render (no virtualization for v1).
- **API returns an error (GET /api/features fails)**: Existing Dashboard error state (`features-error` testid) is shown regardless of selected view mode; the view toggle may remain visible but the board/list area shows the error.
- **API returns features but `current_phase` is not a known phase**: Card is placed in an "Unknown" trailing column with the raw phase string as the label. [ASSUMPTION: backend always returns a valid phase; this is a defensive fallback.]
- **User toggles while features are loading**: Toggle is disabled or the loading spinner persists in the board area until the query resolves. [ASSUMPTION: disable toggle while `isLoading` is true.]
- **Cancelled features**: Excluded from the board entirely (US-3 AC-3).

## Requirements

### Functional Requirements

- **FR-001**: The Dashboard MUST provide a view toggle control allowing the user to switch between "List" and "Board" view modes.
  Source: US-001

- **FR-002**: The selected view mode MUST be persisted to `localStorage` under the key `devteam:view-mode` and restored on subsequent page loads.
  Source: US-001, US-005

- **FR-003**: When "Board" view is active, the Dashboard MUST render a Kanban board with columns labelled, in order: Backlog (only if any draft features exist), Inception, Planning, Construction, Review, Testing, Delivery.
  Source: US-002

- **FR-004**: Each non-draft feature MUST be rendered as a card in the column matching its `current_phase` (one of: inception, planning, construction, review, testing, delivery).
  Source: US-002

- **FR-005**: Each feature with status `draft` MUST be rendered as a card in the Backlog column.
  Source: US-002

- **FR-006**: Each feature with status `cancelled` MUST NOT be rendered on the board.
  Source: US-003

- **FR-007**: Each card MUST display the feature title, priority badge, status badge, and pending-questions badge (when `pending_questions_count > 0`), reusing the existing `FeatureCard` visual conventions.
  Source: US-002

- **FR-008**: Clicking a card MUST navigate to `/features/:id` (the existing feature detail page).
  Source: US-002

- **FR-009**: Cards with status `gate_blocked`, `failed`, `recirculated`, or `waiting_for_human` MUST display a distinct blocked-state visual marker not present on `in_progress` or `passed` cards.
  Source: US-003

- **FR-010**: Cards with status `done` in the Delivery column MUST display a distinct "Done" visual treatment.
  Source: US-003

- **FR-011**: Columns with zero features MUST display an in-column empty-state message naming the phase.
  Source: US-004

- **FR-012**: When the board has zero non-cancelled features, the board area MUST display an empty-state message and the view toggle MUST remain operable.
  Source: US-004

- **FR-013**: The board container MUST scroll horizontally when the viewport is narrower than the combined width of the phase columns; each column MUST maintain a minimum fixed width.
  Source: US-005

- **FR-014**: Column headers MUST be sticky so the phase label remains visible while the column body scrolls vertically.
  Source: US-005

- **FR-015**: The view toggle MUST be keyboard accessible (focusable, operable with Enter and Space) and expose an ARIA label describing its purpose.
  Source: US-005

- **FR-016**: Cards within each column MUST be ordered by priority ascending (P1 before P2 before P3), then by `updated_at` descending (most recent first) for equal priority.
  Source: US-002 (implicit ordering — see Assumptions)

- **FR-017**: If a feature's `current_phase` does not match any known phase, the card MUST be placed in a trailing "Unknown" column labelled with the raw phase string.
  Source: Edge Cases

### Key Entities

- **FeatureSummary** (existing): id, title, status, priority (1-3), current_phase (enum: inception|planning|construction|review|testing|delivery), updated_at, gate_result, pending_questions_count. No schema changes — the board consumes the existing `GET /api/features` response.
- **ViewMode** (new, UI-only): enum `list` | `board`. Persisted in `localStorage['devteam:view-mode']`. No backend representation.
- **KanbanColumn** (new, UI-only): derived entity — phase label + ordered list of FeatureSummary cards. Columns are derived client-side by grouping `FeatureSummary[]` by `current_phase` (or "Backlog" for draft, "Unknown" for unknown phase). No persistence.

No new API endpoints. No backend changes. No database changes.

## Constraint Register

| ID | Source | Section/Vector | Type | Constraint | Verification Method |
|----|--------|----------------|------|------------|---------------------|
| CON-001 | Existing UI convention | data-testid pattern | consistency | Every interactive/observable board element carries a `data-testid` (e.g. `kanban-board`, `kanban-column-${phase}`, `kanban-card-${id}`, `view-toggle-board`, `view-toggle-list`) | E2E: `page.locator('[data-testid="kanban-board"]')` resolves; column/card testids present |
| CON-002 | Existing UI convention | dark mode | consistency | All board styling uses Tailwind `dark:` variants mirroring existing card styles | Smoke: toggle dark mode, verify no unstyled elements |
| CON-003 | Existing API contract | `GET /api/features` response | correctness | Board consumes `FeatureListResponse{features: FeatureSummary[], total_count}` unchanged — no new endpoint, no schema change | Integration: existing `listFeatures()` call drives the board; response shape unchanged |
| CON-004 | Existing phase enum | `feature.AllPhases()` / `PHASES` const | correctness | Column order and labels MUST match the existing 6-phase enum: inception, planning, construction, review, testing, delivery | Unit: assert column order equals `PHASES` constant from `types/index.ts` |
| CON-005 | Existing status enum | `feature.Status*` constants | correctness | Blocked-state classification (`gate_blocked`, `failed`, `recirculated`, `waiting_for_human`) and done-state classification (`done`) MUST match the existing status string values exactly | Unit: assert blocked-status set equals the four strings; assert done-status equals `done` |
| CON-006 | Existing FeatureCard behavior | Link to `/features/:id` | correctness | Card click navigation MUST use the same `<Link to={/features/${id}}>` pattern as existing `FeatureCard` | E2E: click a board card, assert URL is `/features/:id` |
| CON-007 | Existing QuestionBadge | pending-questions indicator | consistency | Cards with `pending_questions_count > 0` MUST render the existing `QuestionBadge` component | E2E: seed a feature with pending questions, assert QuestionBadge visible on its board card |
| CON-008 | AGENTS.md | "No specific build/test commands in phase instructions" | n/a | No impact — this is a feature spec, not phase instructions. Noted for compliance. | Manual review |

Every constraint has a corresponding acceptance criterion (see acceptance.md AC-CON-001 through AC-CON-007).

## Success Criteria

### Measurable Outcomes

- **SC-001**: A user can switch from list view to board view in a single click and see all non-cancelled features grouped by phase within 1 second of clicking the toggle (assuming the features query has already resolved).
- **SC-002**: Every feature returned by `GET /api/features` (excluding cancelled) appears in exactly one column on the board; zero features are silently dropped or duplicated.
- **SC-003**: A user can visually distinguish a blocked feature card from an in-progress feature card without reading the status badge text (via the blocked visual marker).
- **SC-004**: The board renders without JavaScript console errors on initial load and after toggling views (parity with existing E2E console-error assertion in `app.spec.ts`).
- **SC-005**: The user's chosen view mode persists across a full page reload (no flash of the wrong view before the correct one renders).
- **SC-006**: Empty columns and the empty board state render an explicit message, never a blank area.

## Assumptions

- [ASSUMPTION: The board is read-only — no drag-and-drop to change phase. Phase transitions happen via the existing detail-page controls (advance/recirculate/process). Drag-and-drop would require mapping arbitrary column-to-column moves onto the restricted advance/recirculate API, which is out of scope for v1.]
- [ASSUMPTION: Draft features appear in a "Backlog" column shown before Inception. The input idea did not specify where draft features belong.]
- [ASSUMPTION: Completed (`done`) features remain visible in the Delivery column with a distinct "Done" visual treatment rather than being filtered out by default. A future "hide completed" toggle is out of scope.]
- [ASSUMPTION: All six phase columns are always rendered with horizontal scroll on narrow viewports, rather than adaptively hiding columns. Kanban boards conventionally show all columns.]
- [ASSUMPTION: Cards within a column are sorted by priority ascending then `updated_at` descending, with no user-facing sort controls. The list view's sort controls are not reproduced on the board — priority ordering is the Kanban convention.]
- [ASSUMPTION: High-volume columns (50+ cards) render all cards with vertical scroll within the column body and a sticky column header. No virtualization or "show more" expander for v1 — YAGNI pending observed performance issues.]
- [ASSUMPTION: The view toggle is disabled while the features query is loading to prevent rendering an empty board from a not-yet-resolved query.]
- [ASSUMPTION: The backend always returns a valid `current_phase` for non-draft features. The "Unknown" trailing column (FR-017) is a defensive fallback, not an expected state.]
- [ASSUMPTION: No new API endpoints are required. The existing `GET /api/features` response contains all fields the board needs. If the Architect determines additional fields are needed (e.g. a per-phase count for column headers), that is a planning-phase decision.]
- [ASSUMPTION: No authentication or authorization changes are required — the board is served by the same unauthenticated local UI as the existing Dashboard. The security extension's threat-modeling checklist was reviewed: the board handles no user input, performs no state-changing operations, and renders only data already authorized by the existing list endpoint. No new security acceptance criteria are warranted beyond CON-003 (unchanged API contract).]
- [ASSUMPTION: No new backend error paths are introduced. The board surfaces the existing `features-error` state from Dashboard when `GET /api/features` fails. No resilience acceptance criteria are warranted — the board adds no external dependencies beyond the one the Dashboard already uses.]

## Scope Boundaries

**In scope**:
- New `KanbanBoard` React component and `KanbanColumn` subcomponent in `ui/src/components/`
- View toggle control on `Dashboard.tsx`
- `localStorage` persistence of view mode
- Reuse of existing `FeatureCard` (or a board-card variant sharing its badge logic)
- Grouping logic: `current_phase` → column, `draft` → Backlog, unknown → Unknown
- Blocked/done visual treatments
- Empty column and empty board states
- Sticky column headers, horizontal scroll on narrow viewports
- Playwright E2E coverage in `ui/e2e/app.spec.ts`
- Unit tests for grouping/ordering logic

**Out of scope**:
- Drag-and-drop card movement between columns
- Backend API changes, new endpoints, or schema changes
- Per-column WIP limits
- Card content editing inline
- Filtering by priority/status on the board
- A "hide completed features" toggle
- Virtualization or pagination of cards within a column
- Real-time board updates via SSE (the board uses the same react-query cache as the list; SSE-driven refresh is a separate existing feature)
- Mobile-specific responsive layout beyond horizontal scroll