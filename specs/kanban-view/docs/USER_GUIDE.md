# Kanban View — User Guide (spec `kanban-view`)

The Kanban view is a new presentation of the Dev Team Dashboard. Where the
existing list view shows features in a single sortable grid, the Kanban view
shows them as cards grouped into six phase columns. Both views show the same
data — switching between them does not refetch, does not lose your place, and
does not change the total feature count.

This guide documents every user story from `specs/kanban-view/spec.md`.

## What it shows

A board with **six phase columns** in this fixed left-to-right pipeline order
(FR-002, FR-015 — column labels come from `PHASE_LABELS`):

1. **Inception**
2. **Planning**
3. **Construction**
4. **Review**
5. **Testing**
6. **Delivery**

A trailing **"Other"** column appears only when a feature's `current_phase` is
not one of the six known phases (FR-013, AC-016). This is defensive — the
backend enum is closed, so it should never happen in practice — but the board
will not crash if it does.

Each column has a header with the phase name. Each Kanban card shows the
feature title, a priority badge, and a status badge (FR-004). Cards for
features with `pending_questions_count > 0` additionally show a
pending-questions badge with the count (FR-010, US-003). Cards whose
`gate_result` is present show a visible gate-status indicator — gate-passed
or gate-failed (FR-011, US-003). The card chrome is shared with the existing
`FeatureCard` component — the same badge map and the same gate indicator
treatment, so switching views loses no signal.

## US-001 — View features as a Kanban board organized by phase

A **List / Kanban** toggle sits on the Dashboard. Click **Kanban** to switch
to the Kanban board; click **List** to switch back to the existing sortable
`FeatureList` grid (FR-001, FR-006, AC-003).

When you click **Kanban**, six phase columns render in pipeline order, and
each feature card appears in the column matching its `current_phase`
(AC-001). All six columns always render, even when a column has no features
(FR-012).

Click a Kanban card to navigate to that feature's detail page
(`/features/:id`) via client-side routing — no full page reload (FR-005,
AC-002). The destination is identical to clicking a card in the existing list
view.

### Common workflow

1. Visit `/`. The list view renders (existing default — see US-002).
2. Click **Kanban**. Six phase columns render with features grouped by
   `current_phase`.
3. Click any card. The browser navigates to `/features/{id}` without a full
   reload.
4. Click **List** to return to the existing sortable grid.

### Error / loading states

- While `GET /api/features` is loading, the existing `features-loading`
  indicator renders and no `kanban-column-*` elements appear (CON-009,
  AC-005).
- If `GET /api/features` errors, the existing `features-error` message
  renders and no `kanban-column-*` elements appear (CON-009, AC-006).
- If there are zero features, six empty columns render **and** the existing
  `EmptyState` call-to-action is visible above the board (CON-010, AC-007).

### Edge cases

- **All features in one phase**: the other five columns render empty
  (headers visible, body empty). No column is hidden.
- **Unknown `current_phase`**: the card lands in the trailing "Other" column
  so no feature is silently dropped (FR-013, AC-016).
- **`status: gate_blocked` or `recirculated`**: the card stays in the column
  matching its `current_phase` (spec Assumptions).
- **Rapid toggle**: switching views does not refetch — both views consume the
  same cached query data (FR-014, AC-004).
- **Narrow viewports**: the board scrolls horizontally, preserving the
  six-column layout (spec Assumptions).

## US-002 — Toggle persists between visits and respects user preference

The Dashboard remembers which view (list or Kanban) you last selected via
`localStorage`, so revisiting `/` restores that view without pressing the
toggle again (FR-007, AC-008).

**First-time visitors see the list view** — the existing default (FR-008,
AC-009). Kanban is opt-in.

If `localStorage` is unavailable (private mode, disabled storage), the
Dashboard falls back to the list view without crashing (FR-009, AC-010,
AC-011).

### Common workflow

1. Visit `/`. List view renders (default).
2. Click **Kanban**. The selection is written to
   `localStorage['devteam.dashboard.view']`.
3. Reload the page. The Kanban board re-renders on mount without further
   interaction.
4. To revert: click **List**, or clear `localStorage` and reload — the list
   view renders.

## US-003 — Kanban cards surface pending questions and gate status

Kanban cards show the same information density as the existing `FeatureCard`
so switching views loses no signal:

- **Pending questions**: a feature with `pending_questions_count > 0` shows a
  pending-questions badge with the count, reusing the existing `QuestionBadge`
  component (FR-010, AC-012).
- **Gate status**: a feature whose `gate_result` is present shows a visible
  gate-status indicator — gate-passed when `passed: true`, gate-failed when
  `passed: false` (FR-011, AC-013/014). A feature whose `gate_result` is
  `null` shows no gate indicator (AC-015).

## Accessibility & data-testid

Every new rendered element carries a `data-testid` attribute for assistive
selection and Playwright targeting (CON-008):

- `view-toggle-list`, `view-toggle-kanban` — the toggle buttons.
- `kanban-column-inception`, `kanban-column-planning`,
  `kanban-column-construction`, `kanban-column-review`,
  `kanban-column-testing`, `kanban-column-delivery` — the six phase columns.
- `kanban-column-other` — the defensive trailing column (only when non-empty).
- `feature-card-{id}` — each card (shared with the existing list view).

## Out of scope

The board is read-only (spec Scope Boundaries). There is no drag-and-drop, no
inline card editing, no column-level filtering or sorting, and no backend
mutation. Cards move between columns only as the pipeline updates a feature's
`current_phase`.