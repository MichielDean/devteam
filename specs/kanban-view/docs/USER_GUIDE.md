# Kanban View — User Guide (Spec kanban-view)

The Kanban view is an alternative presentation of the Dev Team feature list.
Where the Dashboard shows features in a single sortable list, the Kanban view
shows them as cards grouped into columns by their current pipeline phase. Both
views show the same data — switching between them does not refetch, does not
lose your place, and does not change the total feature count.

## What it shows

A board with 7 columns, in this fixed left-to-right order:

1. **Backlog** — features that have not started the pipeline yet
2. **Inception**
3. **Planning**
4. **Construction**
5. **Review**
6. **Testing**
7. **Delivery**

The column order matches the Dev Team pipeline phase order. It is not
configurable. The 6 phase columns (Inception through Delivery) use the canonical
phase names from `internal/feature/types.go`; the board does not invent its own
column names.

Each column has a header with the column name and a count of the cards in that
column. Each card reuses the existing `FeatureCard` component: title, status
badge, phase badge, priority badge, gate indicator (✓ Gate passed / ✗ Gate
failed), and updated date.

## User stories covered

This guide documents every user story from `specs/kanban-view/spec.md`.

### US-001 — See all features organized by pipeline phase

Open the Dashboard and switch to the **Board** view (see "Switching views"
below). Every feature in the system appears as a card in exactly one column. A
feature lands in the column matching its `current_phase` — for example, a
feature currently in the Review phase appears in the **Review** column.

### US-002 — Not-yet-started features appear in Backlog

A feature that has been created but has not had any phase run yet appears in the
**Backlog** column, not in the **Inception** column. The rule is: if the
feature's status is `draft` and its current phase is `inception`, it goes to
Backlog. Once inception starts (status becomes `in_progress`), the card moves
to the **Inception** column.

### US-003 — Switch between list view and Kanban view

A **List / Board** segmented control sits in the Dashboard header, next to the
total feature count badge. Click **Board** to switch to the Kanban view; click
**List** to switch back. The total feature count badge stays mounted across both
views, so the count you see is identical in either view.

### US-004 — Click a card to open feature detail

Every card on the board is a link. Click a card to navigate to that feature's
detail page at `/features/{id}`. From there you can run phases, advance, view
artifacts, and answer pending questions — the same actions available from the
list view. The board itself is read-only: you cannot drag cards between columns,
create features from the board, or change a feature's phase by interacting with
the board. Phase changes happen on the detail page and propagate back to the
board through the shared data cache.

### US-005 — Empty board renders cleanly

If the system has zero features, every column renders with an empty-state
message and a count of 0:

- **Backlog**: "No features waiting to start"
- **Inception, Planning, Construction, Review, Testing, Delivery**:
  "No features in this phase"

The board does not crash, does not show a blank page, and produces no browser
console errors. The total count badge shows 0.

### US-006 — Board reflects live updates during processing

If a feature advances phases while the board is open (because you triggered an
action from its detail page, or because autonomous processing is running), the
card moves to the new column without a full page reload. The board and the
Dashboard list share the same data cache, so any change visible on the list is
visible on the board on the next cache invalidation.

## Switching views

1. Open the Dashboard (`/`).
2. In the header, next to the **Features** heading and the count badge, locate
   the segmented control labeled **List | Board**.
3. Click **Board** to render the Kanban board. Click **List** to return to the
   existing sortable feature list.

The default view on first load is **List** (the existing behavior is preserved).

## Terminal features stay visible

A feature that has finished the pipeline (status `done`) or been cancelled
(status `cancelled`) is **not** hidden from the board. It stays in the column
matching its `current_phase`. For example, a feature that completed Delivery
remains in the **Delivery** column with a "Done" status badge on its card. This
is intentional: the board shows the state of all specs, including finished ones.

## Dark mode

The board supports dark mode via the existing theme toggle in the app header.
Toggle the theme and every column, card, badge, and empty-state message
re-renders with the app's dark palette. No separate board-specific theme setting
exists.

## Error and loading states

- **Loading**: while the first fetch is in flight, each column renders empty and
  the board shows a loading spinner. Columns are always present (the board
  renders 7 columns even before data arrives).
- **API error on first load**: the board shows a red banner
  `Failed to load features: {message}` at the top. The 7 columns still render,
  each empty. No uncaught exception, no blank page.
- **API error on refetch (mid-session)**: the board keeps the previously-loaded
  cards visible — stale data is better than a blank board. No uncaught
  exception. Trigger a refresh (e.g. by creating or advancing a feature) to
  retry.
- **Clicking a card whose feature was deleted between load and click**: the
  browser navigates to `/features/{id}` and the existing Feature Detail page
  shows its own not-found state. The board does not need to handle this itself.

## What the board does not do

The following are explicitly out of scope (see `spec.md` "Out of scope"):

- **Drag-and-drop** card movement between columns. The board is read-only; phase
  changes happen on the feature detail page.
- **Card creation** from the board. Use the **+ New Feature** button on the
  Dashboard (available in both list and board views).
- **Filtering or search** within columns. The board shows all features; sorting
  and filtering remain list-view features.
- **Per-column WIP limits.**
- **A separate `/kanban` route.** The board lives at `/` alongside the list view,
  toggled by the segmented control. The URL does not distinguish the two views.
- **Mobile collapsed-column layout.** On narrow viewports the board scrolls
  horizontally so all 7 columns remain reachable.

## Accessibility

- The **List / Board** toggle is a `role="group"` with two buttons, each with
  `aria-pressed` reflecting the active view.
- The total feature count badge has an `aria-label` of `Total features: N`.
- Each column header is an `<h3>`; each card title is an `<h3>` inside a link.
- The error banner uses `role="alert"`.
- The board and all columns expose stable `data-testid` attributes for automated
  selectors (see the testability section of the spec, CON-011).

## Stable test selectors

For E2E and integration tests, the board exposes:

| Selector | Element |
|----------|---------|
| `[data-testid="kanban-board"]` | the board container |
| `[data-testid="kanban-column-backlog"]` | Backlog column |
| `[data-testid="kanban-column-inception"]` | Inception column |
| `[data-testid="kanban-column-planning"]` | Planning column |
| `[data-testid="kanban-column-construction"]` | Construction column |
| `[data-testid="kanban-column-review"]` | Review column |
| `[data-testid="kanban-column-testing"]` | Testing column |
| `[data-testid="kanban-column-delivery"]` | Delivery column |
| `[data-testid="kanban-column-count-{key}"]` | per-column card count |
| `[data-testid="kanban-column-empty-{key}"]` | per-column empty-state message |
| `[data-testid="view-toggle-list"]` | List view toggle button |
| `[data-testid="view-toggle-board"]` | Board view toggle button |
| `[data-testid="feature-card-{id}"]` | each card (inherited from `FeatureCard`) |
| `[data-testid="kanban-error"]` | board-level error banner |
| `[data-testid="kanban-loading"]` | board-level loading spinner |

The existing `feature-count-badge` testid remains mounted in the Dashboard
header across both views.