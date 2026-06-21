# Acceptance Criteria: Kanban View for Dev Team

**Spec**: kanban-view-for-dev-team
**Created**: 2026-06-21

---

## US-001: View specs as a Kanban board by phase

- **AC-001**: Given the dashboard with at least one feature present, when the user clicks the "Board" view toggle (`data-testid="view-toggle-board"`), then six columns render with headers "Inception", "Planning", "Construction", "Review", "Testing", "Delivery" in that order, and each column's header has `data-testid="kanban-column-header-<phase>"`.
  Test level: e2e
  Verification: Load the dashboard with seeded features, click the Board toggle, assert 6 column headers in pipeline order with the correct testids.

- **AC-002**: Given a feature with `current_phase: "planning"`, when the Board view renders, then the feature's card appears in the Planning column (`data-testid="kanban-column-planning"`) and in no other column.
  Test level: unit
  Verification: Render KanbanBoard with `[{ id: 'f1', current_phase: 'planning', ... }]`, assert the card is in the planning column's card list and absent from all other columns.

- **AC-003**: Given multiple features with distinct `current_phase` values (one each in inception, planning, construction, review, testing, delivery), when the Board view renders, then each card appears in exactly the column matching its `current_phase`, and every column has exactly one card.
  Test level: unit
  Verification: Render KanbanBoard with 6 features in 6 distinct phases, assert each column contains exactly its matching card.

- **AC-004**: Given the Board view, when the user clicks a card (`data-testid="kanban-card-<id>"`), then the app navigates to `/features/<id>` (the existing FeatureDetail route).
  Test level: e2e
  Verification: Render the board, click a card, assert `window.location.pathname` equals `/features/<id>` and the FeatureDetail page renders.

- **AC-005**: Given the Board view, when the user clicks the "List" view toggle (`data-testid="view-toggle-list"`), then the existing card-grid dashboard renders (`data-testid="feature-list"` present) and the board (`data-testid="kanban-board"`) is absent.
  Test level: e2e
  Verification: Switch to Board, assert board present; click List toggle, assert `feature-list` present and `kanban-board` absent.

- **AC-006**: Given the dashboard with features present, when the Board view renders, then each column displays a count badge (`data-testid="kanban-column-count-<phase>"`) showing the number of cards in that column.
  Test level: e2e
  Verification: Seed 3 features in planning and 1 in inception, switch to Board, assert the planning column count badge reads "3" and the inception badge reads "1".

## US-002: See real-time pipeline movement on the board

- **AC-007**: Given the Board view with a feature in the Inception column, when the feature's `current_phase` changes to "planning" (via API call `POST /api/features/:id/advance` succeeding), then the card moves from the Inception column to the Planning column within 5 seconds, without manual refresh.
  Test level: e2e
  Verification: Open board, note card in inception column, call advance API, assert within 5s the card is in the planning column and absent from inception.

- **AC-008**: Given the Board view, when the SSE connection drops, then the existing ConnectionStatus banner (`data-testid="connection-status-banner"`) appears exactly once at the top of the page and the board continues showing the last-known card positions (not blank).
  Test level: e2e
  Verification: Open board, simulate SSE disconnect, assert exactly one ConnectionStatus banner, assert cards still visible at their last-known columns.

- **AC-009**: Given the Board view with SSE disconnected then reconnected, when a phase-change event arrives after reconnection, then the board updates to reflect the new card position.
  Test level: e2e
  Verification: Disconnect SSE, advance a feature via CLI (so SSE misses it), reconnect SSE, assert board updates to current state within 5s of reconnect.

## US-003: Sort cards within columns

- **AC-010**: Given the Board view with 3 features in the Planning column with priorities 3, 1, 2, when the user clicks the "Priority" sort control (`data-testid="sort-by-priority"`), then the cards within every column reorder so P1 appears first, then P2, then P3.
  Test level: e2e
  Verification: Seed 3 features in planning with priorities 3/1/2, switch to Board, click sort-by-priority, assert the planning column's cards are ordered P1, P2, P3 (read card priorities top-to-bottom).

- **AC-011**: Given the Board view with sort-by-priority active (asc, P1 first), when the user clicks "Priority" again, then the sort direction toggles to desc (P3 first) within every column.
  Test level: e2e
  Verification: With sort-by-priority asc active, click sort-by-priority again, assert cards reorder to P3, P2, P1 within each column.

- **AC-012**: Given the Board view with no sort control active, when the board renders, then cards within each column appear in the order returned by the API (the order of the `features` array from `listFeatures`).
  Test level: unit
  Verification: Render KanbanBoard with features in a known array order and no sort applied, assert within-column order matches input array order.

- **AC-013**: Given the Board view with sort-by-status active, when the user clicks sort-by-updated (`data-testid="sort-by-updated"`), then the active sort switches to updated_at and cards reorder by updated_at within each column.
  Test level: e2e
  Verification: With sort-by-status active, click sort-by-updated, assert cards reorder by updated_at within columns.

## US-004: Use the board on mobile

- **AC-014**: Given the Board view at viewport width 375px, when the user views the board, then each column is at least 250px wide and the board container (`data-testid="kanban-board-scroll"`) has a horizontal scrollbar.
  Test level: e2e
  Verification: Set viewport to 375x800, switch to Board, measure each column's `getBoundingClientRect().width` (≥250px), assert the board scroll container has `scrollWidth > clientWidth`.

- **AC-015**: Given the Board view at viewport width 375px, when the user views the page, then `document.documentElement.scrollWidth` is ≤ 375 (the page itself does not scroll horizontally; only the board container does).
  Test level: e2e
  Verification: At 375px viewport, assert `document.documentElement.scrollWidth <= 375` and the board container's `scrollWidth > clientWidth`.

- **AC-016**: Given the Board view at viewport width 1440px, when the user views the board, then all six columns are visible without horizontal scrolling (board container `scrollWidth === clientWidth`).
  Test level: e2e
  Verification: At 1440px viewport, assert board container `scrollWidth <= clientWidth` and all 6 column headers are in the viewport.

## US-005: See feature status, priority, and gate on each card

- **AC-017**: Given a feature with `status: "in_progress"`, `priority: 2`, and `gate_result: null`, when the Board view renders, then its card shows a status badge with text "In Progress" (`data-testid="kanban-card-status"`), a priority badge with text "P2 - Medium" (`data-testid="kanban-card-priority"`), and no gate indicator.
  Test level: e2e
  Verification: Seed the feature, switch to Board, assert badge texts and absence of `data-testid="kanban-card-gate"`.

- **AC-018**: Given a feature with `status: "gate_blocked"` and `gate_result.passed: false`, when the Board view renders, then its card shows a "Gate Blocked" status badge and a gate indicator with text "✗ Gate failed" (`data-testid="kanban-card-gate"`).
  Test level: e2e
  Verification: Seed the feature, switch to Board, assert status badge text "Gate Blocked" and gate indicator text "✗ Gate failed".

- **AC-019**: Given a feature with `pending_questions_count: 2`, when the Board view renders, then its card shows a QuestionBadge (`data-testid="question-badge"`).
  Test level: e2e
  Verification: Seed the feature, switch to Board, assert `question-badge` is present on the card.

- **AC-020**: Given a feature with `status: "cancelled"`, when the Board view renders, then its card shows a "Cancelled" status badge with the cancelled color class (red) — the same color class as the existing FeatureCard uses for cancelled status.
  Test level: e2e
  Verification: Seed a cancelled feature, switch to Board, assert status badge text "Cancelled" and the badge element has the red background class (`bg-red-100` or `dark:bg-red-900`).

- **AC-021**: Given a feature with a long title (200 characters), when the Board view renders, then the card title is truncated with an ellipsis and does not overflow the column width.
  Test level: e2e
  Verification: Seed a feature with a 200-char title, switch to Board, assert the title element's `scrollWidth > clientWidth` (truncation active) and the card does not exceed the column width.

## US-006: Empty state on the board

- **AC-022**: Given zero features in the system (`GET /api/features` returns `{ features: [], total_count: 0 }`), when the user switches to Board view, then the existing `EmptyState` component renders (`data-testid="empty-state"`) with the create-feature call-to-action, and no column headers are present.
  Test level: e2e
  Verification: With 0 features, click Board toggle, assert `empty-state` present and no `kanban-column-header-*` elements exist.

- **AC-023**: Given zero features, when the user switches back to List view, then the same `EmptyState` component renders (consistent empty-state behavior across views).
  Test level: e2e
  Verification: With 0 features, toggle Board→List, assert `empty-state` still present.

- **AC-024**: Given features present but one phase has zero features (e.g., 3 features all in planning, none in review), when the Board view renders, then the Review column renders with its header and a "No specs in Review" placeholder (`data-testid="kanban-column-empty-review"`), and the other columns render their cards normally.
  Test level: e2e
  Verification: Seed 3 features all in planning, switch to Board, assert Review column header present, Review column empty placeholder present, Planning column has 3 cards.

## US-007: Toggle persists across page reloads

- **AC-025**: Given the user has selected Board view, when the user reloads the page, then the Board view renders on load (`data-testid="kanban-board"` present, `data-testid="feature-list"` absent).
  Test level: e2e
  Verification: Switch to Board, `page.reload()`, assert `kanban-board` present and `feature-list` absent.

- **AC-026**: Given the user has selected List view, when the user reloads the page, then the List view renders (`data-testid="feature-list"` present, `data-testid="kanban-board"` absent).
  Test level: e2e
  Verification: Switch to List, `page.reload()`, assert `feature-list` present and `kanban-board` absent.

- **AC-027**: Given the user has no stored view preference (`localStorage['devteam-dashboard-view']` unset), when the user opens the dashboard, then the List view renders (default; existing behavior unchanged).
  Test level: e2e
  Verification: Clear localStorage, load dashboard, assert `feature-list` present and `kanban-board` absent.

## Constraint-Driven Acceptance Criteria

- **AC-CON-001**: Given the Kanban view implementation, when the code is reviewed, then no new API endpoint is defined in `internal/api/` and no new TypeScript type duplicating `FeatureSummary` is defined — the board reuses the existing `listFeatures` client and `FeatureSummary` type.
  Test level: unit (static analysis)
  Verification: `grep -r "api/features" ui/src/components/Kanban*.tsx` returns no new endpoint paths; `grep -r "interface FeatureSummary" ui/src/` returns only `types/index.ts`; KanbanBoard imports `FeatureSummary` from `../types`.

- **AC-CON-002**: Given the Kanban view implementation, when the code is reviewed, then no new npm dependency is added to `ui/package.json` (diff the dependencies block before and after).
  Test level: unit (static analysis)
  Verification: `git diff main -- ui/package.json` shows no additions in `dependencies` or `devDependencies`.

- **AC-CON-003**: Given the Board view, when columns render, then there are exactly 6 columns with headers matching the `PHASES` constant (`['inception','planning','construction','review','testing','delivery']`) in order, no more, no fewer.
  Test level: unit
  Verification: Render KanbanBoard, query all column headers, assert length === 6 and texts equal `PHASES.map(p => PHASE_LABELS[p])` in order.

- **AC-CON-004**: Given the Board view in dark mode, when the user toggles dark mode on, then every column header, card, badge, and sort control has a readable text color against its dark background (no light-on-light or dark-on-dark).
  Test level: e2e
  Verification: Toggle dark mode, for each of (column header, card title, status badge, priority badge, sort button), assert `getComputedStyle(element).color` differs from the background color (manual visual check or contrast ratio ≥ 4.5:1 via axe-core if available).

- **AC-CON-005**: Given the Board view, when a feature with `current_phase: "review"` and `status: "cancelled"` is in the data, then its card appears in the Review column (not hidden) with a "Cancelled" status badge.
  Test level: e2e
  Verification: Seed a cancelled feature at review phase, switch to Board, assert card is in the review column and shows a cancelled badge.

- **AC-CON-006**: Given the Board view, when the SSE connection drops, then exactly one ConnectionStatus banner is present on the page (the existing one) and no new connection indicator was added by the Kanban view.
  Test level: e2e
  Verification: Switch to Board, simulate SSE drop, assert exactly one element with `data-testid="connection-status-banner"` exists in the DOM.

- **AC-CON-007**: Given the Board view, when the user toggles List→Board→List rapidly, then both views render correctly on each toggle and the `['features']` React Query cache is not refetched (no new network request to `/api/features` between toggles).
  Test level: e2e
  Verification: Start network observer, toggle List→Board→List, assert only the initial `GET /api/features` request was made (no refetch on toggle).

## Error Path Acceptance Criteria

- **AC-ERR-001**: Given the `listFeatures` query is loading (first load), when the user switches to Board view, then each column shows a loading skeleton/spinner (`data-testid="kanban-column-loading"`) and no cards render until data arrives.
  Test level: e2e
  Verification: Throttle or stub `listFeatures` to delay, switch to Board, assert 6 loading skeletons render and no cards present; resolve the query, assert cards render.

- **AC-ERR-002**: Given the `listFeatures` query has errored, when the user switches to Board view, then an error message renders in the board area with text "Failed to load features: \<message\>" (`data-testid="features-error"`) and no columns render.
  Test level: e2e
  Verification: Stub `listFeatures` to reject, switch to Board, assert `features-error` present and no `kanban-column-header-*` elements.

- **AC-ERR-003**: Given `localStorage` is disabled or full, when the user toggles to Board view, then the view switches in-memory for the session and a console warning is logged; no user-facing error is shown; on next page load, the default List view renders.
  Test level: unit
  Verification: Mock `localStorage.setItem` to throw, toggle to Board, assert view switches (in-memory), assert `console.warn` called, reload (with localStorage still failing), assert List view renders (default fallback).

- **AC-ERR-004**: Given a feature with `current_phase` set to an unknown value (e.g., "deploy" — defensive case, should not occur per API contract), when the Board view renders, then the card does not appear in any column and a console error is logged with the feature ID and invalid phase.
  Test level: unit
  Verification: Render KanbanBoard with a feature having `current_phase: "deploy"`, assert no column contains the card and `console.error` was called with a message containing the feature ID and "deploy".

## Dark Mode Acceptance Criteria

- **AC-DM-001**: Given the dashboard in light mode, when the user toggles to Board view, then all column headers have light-mode backgrounds (`bg-gray-50` or similar) and dark text.
  Test level: e2e
  Verification: In light mode, switch to Board, assert column header background and text classes match light-mode Tailwind classes.

- **AC-DM-002**: Given the dashboard in dark mode, when the user toggles to Board view, then all column headers have dark-mode backgrounds (`dark:bg-gray-800` or similar) and light text, and all cards have dark card backgrounds (`dark:bg-gray-800`) with light text.
  Test level: e2e
  Verification: Toggle dark mode, switch to Board, assert column headers and cards have `dark:` classes applied and text is readable.