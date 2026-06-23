# Kanban View — User Guide (spec kanban-view)

The Kanban view is a new presentation of the Dev Team Dashboard. Where the
existing list view shows features in a single sortable grid, the Kanban view
shows them as cards grouped into six phase columns. Both views show the same
data — switching between them does not refetch, does not lose your place, and
does not change the total feature count.

This guide documents every user story from `specs/kanban-view/spec.md`.

## What it shows

A board with **six phase columns** in this fixed left-to-right pipeline order
(FR-005, CON-005 — column names come from `PHASE_LABELS`):

1. **Inception**
2. **Planning**
3. **Construction**
4. **Review**
5. **Testing**
6. **Delivery**

A trailing **"Other"** column appears only when a feature's `current_phase` is
not one of the six known phases (FR-007, AC-011). This is defensive — the
backend enum is closed, so it should never happen in practice — but the board
will not crash if it does.

Each column has a header with the phase name. Each card shows the feature
title, priority badge, status badge, pending-questions badge (when > 0), gate
result indicator for the current phase (when present), and updated date
(FR-008, FR-009, CON-006). The card chrome is shared with the existing
`FeatureCard` component — the same badge color map and the same gate
indicator text (`✓ Gate passed` / `✗ Gate failed`).

## US-001 — Toggle Between List and Kanban Board

A **List / Board** toggle sits in the Dashboard header. Click **Board** to
switch to the Kanban board; click **List** to switch back to the existing
sortable `FeatureList` (FR-001, AC-002/003).

- **Default view**: when the Dashboard loads for the first time in a session
  with no prior choice, the **Board** view is shown (FR-003, AC-005). Kanban
  is the primary view of the Dashboard.
- **Persistence**: the selected view is remembered for the browser session via
  `sessionStorage` key `devteam.dashboard.view` (FR-002, AC-004). Navigate
  away and return, or reload the page, and the same view is still active.
- **Hidden when empty**: if zero features exist, the toggle is NOT visible and
  the existing `EmptyState` component renders instead (FR-004, AC-006/018).
  Once features are created, the toggle appears and the stored view resumes.

## US-002 — Feature Cards on the Board Show Key State

Every feature appears as a card in **exactly one** column — the column whose
phase equals the feature's `current_phase` (FR-006, SC-002). A feature in the
Review phase appears in the **Review** column; a feature in Delivery appears
in **Delivery**; and so on.

Each card shows:

- **Title** (line-clamped to 2 lines)
- **Priority badge** via `PRIORITY_LABELS` — e.g. `P1 - Critical`, `P2 - Medium`, `P3 - Low` (FR-008, AC-007)
- **Status badge** via `STATUS_LABELS` — e.g. `In Progress`, `Done`, `Cancelled` (FR-008, AC-007)
- **Pending-questions badge** when `pending_questions_count > 0` (FR-008, AC-008)
- **Gate result indicator** when `gate_result` is present: `✓ Gate passed` or `✗ Gate failed` (FR-009, AC-009) — identical text to the list view's `FeatureCard`
- **Updated date**

**Attention flags** (FR-011):

- A feature with `status='gate_blocked'` gets a **red ring** on its card (AC-012).
- A feature with `status='waiting_for_human'` gets a **yellow ring** on its card (AC-013).

**Click to navigate**: clicking a card navigates to `/features/{id}` via
`react-router`'s `Link` — the same destination as the list view's
`FeatureCard` (FR-010, AC-010). The board itself is read-only.

**Terminal features stay visible**: a feature with `status='done'` or
`status='cancelled'` is **not** filtered out. It stays in the column for its
`current_phase`; the status badge communicates the terminal state. This is
intentional — the board shows the state of all specs, including finished ones,
for retrospective.

**Unknown phase (defensive)**: if a feature's `current_phase` is not one of
the six known phases, the card is placed in a trailing **"Other"** column
rather than dropped (FR-007, AC-011). The board does not crash.

## US-003 — Empty Columns and Empty Board

**Empty column**: when a phase has no features, its column still renders with
the header and a muted **"No features"** placeholder in the body (FR-012,
AC-017). All six phase columns are always present regardless of whether they
have features (AC-019) — hiding empty columns would obscure the pipeline
shape, which is the whole point of the board.

**Empty board**: when zero features exist in the workspace, the Dashboard
renders the existing `EmptyState` component instead of the board, and the
List/Board toggle is NOT visible (FR-004, AC-006/018). Once features are
created, the toggle becomes visible and the previously-stored view resumes.

## US-004 — Column Overflow Handling

When a column contains more cards than fit the viewport, the column **scrolls
vertically independent of other columns** (FR-013, AC-020). The board's
overall height is bounded to the viewport so all six column headers remain
visible without page-level scroll (FR-014, AC-021).

On viewports narrower than the board's natural width, the board container
**scrolls horizontally**; each column has a minimum width of 240px (FR-015,
AC-022).

## Switching views

1. Open the Dashboard (`/`).
2. Locate the **List / Board** toggle in the header.
3. Click **Board** to render the Kanban board. Click **List** to return to the
   existing sortable feature list.

The board and list share the same `/` Dashboard route — there is no separate
`/kanban` route. A single toggle control switches them.

## Loading and error states

- **Loading**: while the features query is in flight, the existing
  `features-loading` indicator renders and no column bodies render yet
  (FR-017, AC-014).
- **API error**: the existing `features-error` indicator renders and the board
  does not render (FR-017, AC-015). The toggle is hidden (no data to show).

Both states reuse the existing Dashboard branches unchanged (CON-008). The
board only renders when `!isLoading && !error && features.length > 0`.

## Dark mode

All new elements include `dark:` Tailwind classes matching the existing
palette. Use the existing theme toggle in the app header; every column, card,
badge, and empty-state message re-renders with the dark palette.

## What the board does not do

The following are explicitly out of scope (see `spec.md` "Assumptions"):

- **Drag-and-drop** card movement between columns. The board is view-only;
  phase transitions happen through the existing pipeline (`/advance`,
  `/recirculate`) on the feature detail page.
- **Card creation** from the board. Use the **+ New Feature** button on the
  Dashboard.
- **Filtering, sorting, or search** within columns. The board shows all
  features; sorting and filtering remain list-view features.
- **Swimlane-per-status.** Columns are the six phases, not statuses. Status is
  shown as a badge on the card.
- **A separate `/kanban` route.** The board lives at `/` alongside the list
  view, toggled by the control.
- **Cross-session persistence.** The view choice is `sessionStorage` only; no
  user-preference backend exists.
- **"+N more" overflow truncation.** Overflow is handled by vertical scroll,
  not a truncation component.

## Accessibility

- The **List / Board** toggle exposes two buttons, each with `aria-pressed`
  reflecting the active view (AC-001/004/005).
- Each column header and card title uses semantic heading elements.
- The board and all columns expose stable `data-testid` attributes for
  automated selectors (see below).

## Stable test selectors

For E2E and integration tests, the board exposes:

| Selector | Element |
|----------|---------|
| `[data-testid="view-toggle"]` | toggle container |
| `[data-testid="view-toggle-list"]` | List toggle button |
| `[data-testid="view-toggle-board"]` | Board toggle button |
| `[data-testid^="kanban-column-"]` | each column (e.g. `kanban-column-planning`) |
| `[data-testid="kanban-column-empty-{phase}"]` | per-column "No features" placeholder |
| `[data-testid="kanban-card-{id}"]` | each board card |
| `[data-testid="kanban-card-status"]` | card status badge |
| `[data-testid="kanban-card-priority"]` | card priority badge |
| `[data-testid="kanban-card-gate"]` | card gate indicator |
| `[data-testid="question-badge"]` | card pending-questions badge |
| `[data-testid="features-loading"]` | loading indicator (existing, reused) |
| `[data-testid="features-error"]` | error indicator (existing, reused) |

The existing `feature-count-badge` testid remains mounted in the Dashboard
header across both views.