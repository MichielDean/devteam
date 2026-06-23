# Acceptance Criteria — kanban-view

Every criterion follows Given/When/Then with a test level and verification method. Constraints (CON-NNN) each have a paired AC-CON-NNN.

---

## US-001 — Toggle to Kanban board view

AC-001: Given the Dashboard has loaded with at least one feature, when the user clicks the "Board" view toggle, then the page renders a Kanban board with six phase columns labelled Inception, Planning, Construction, Review, Testing, Delivery (plus a Backlog column only if any draft features exist).
  Test level: e2e
  Verification: Playwright `page.goto('/')`, click `[data-testid="view-toggle-board"]`, assert `[data-testid="kanban-board"]` visible, assert six `[data-testid^="kanban-column-"]` elements with the expected phase labels in order.

AC-002: Given the Kanban board is displayed, when the user clicks the "List" view toggle, then the page renders the existing FeatureList grid with sort controls (`[data-testid="feature-list"]`).
  Test level: e2e
  Verification: Playwright click `[data-testid="view-toggle-list"]`, assert `[data-testid="kanban-board"]` not visible, assert `[data-testid="feature-list"]` visible.

AC-003: Given the user has selected "Board" view, when the user reloads the page, then the board is displayed by default without requiring the user to re-toggle.
  Test level: e2e
  Verification: Playwright click `[data-testid="view-toggle-board"]`, `page.reload()`, assert `[data-testid="kanban-board"]` visible and `[data-testid="view-toggle-board"]` reflects the active state. Assert `localStorage.getItem('devteam:view-mode')` === `'board'`.

AC-004: Given no view preference is stored in localStorage, when the Dashboard loads for the first time, then the list view is shown by default (existing behavior preserved).
  Test level: e2e
  Verification: Playwright `page.evaluate(() => localStorage.removeItem('devteam:view-mode'))`, `page.goto('/')`, assert `[data-testid="feature-list"]` visible.

---

## US-002 — Features appear in their phase column as cards

AC-005: Given features exist across multiple phases (inception, planning, construction at minimum), when the board renders, then each feature appears in exactly one column whose `[data-testid]` equals `kanban-column-${feature.current_phase}` and no feature appears in more than one column.
  Test level: e2e
  Verification: Playwright `page.goto('/')`, click board toggle, for each feature returned by `GET /api/features` assert a `[data-testid="kanban-card-${id}"]` exists inside `[data-testid="kanban-column-${current_phase}"]`. Cross-check against the API response fetched via `page.request.get('/api/features')`.

AC-006: Given a feature with status `draft`, when the board renders, then the feature appears in the `[data-testid="kanban-column-backlog"]` column which is rendered before the Inception column.
  Test level: e2e
  Verification: Seed (or select) a draft feature, assert its card is inside `kanban-column-backlog` and that `kanban-column-backlog` precedes `kanban-column-inception` in DOM order.

AC-007: Given a feature card is displayed on the board, when the user clicks the card, then the browser navigates to `/features/:id`.
  Test level: e2e
  Verification: Playwright click `[data-testid="kanban-card-${id}"]`, assert `page.url()` matches `/features/${id}`.

AC-008: Given a feature with `pending_questions_count > 0`, when its board card renders, then the existing QuestionBadge component is visible on the card.
  Test level: e2e
  Verification: Seed a feature with pending questions, assert `[data-testid^="question-badge"]` is visible inside `[data-testid="kanban-card-${id}"]`.

AC-009: Given the features query returns a non-empty list, when the board renders, then each card displays the feature title, a priority badge, and a status badge (text content matches existing `PRIORITY_LABELS` and `STATUS_LABELS`).
  Test level: smoke
  Verification: Playwright assert `[data-testid="kanban-card-${id}"]` contains the title text and contains `[data-testid="feature-card-priority"]` and `[data-testid="feature-card-status"]` (reusing existing testids from FeatureCard).

AC-010: Given the features query is loading (`isLoading` true), when the user toggles to Board view, then the board area shows a loading indicator and no partial/empty board is rendered until the query resolves.
  Test level: smoke
  Verification: Playwright throttle the `/api/features` response, toggle to board, assert a loading indicator is visible and `[data-testid="kanban-board"]` is not yet populated with cards.

---

## US-003 — Blocked and completed features visually distinguishable

AC-011: Given a feature with status `gate_blocked`, `failed`, `recirculated`, or `waiting_for_human`, when its board card renders, then the card has a `data-blocked="true"` attribute and a CSS class applying a distinct blocked visual marker (amber/red border or badge) that is not present on cards with status `in_progress` or `passed`.
  Test level: e2e
  Verification: Seed features in each blocked status and in `in_progress`/`passed`. Assert blocked cards have `data-blocked="true"`; assert in-progress/passed cards do not. Assert the blocked card's computed border color differs from the in-progress card's.

AC-012: Given a feature with status `done` in the Delivery column, when its card renders, then the card has a `data-done="true"` attribute and a distinct "Done" visual treatment (e.g. reduced opacity, green accent, or "Done" pill) not present on in-progress cards in the same column.
  Test level: e2e
  Verification: Seed one `done` and one `in_progress` feature in delivery. Assert the done card has `data-done="true"` and a CSS class the in-progress card lacks.

AC-013: Given a feature with status `cancelled`, when the board renders, then no card with that feature's id is present in any column.
  Test level: e2e
  Verification: Seed a cancelled feature, assert `[data-testid="kanban-card-${cancelled_id}"]` count is 0 across the board.

---

## US-004 — Empty columns and empty board states

AC-014: Given a phase column has zero features (e.g. no features in Review), when the board renders, then the column body displays "No features in {phase label}" text and the column header remains visible.
  Test level: e2e
  Verification: Seed features only in inception/planning. Assert `[data-testid="kanban-column-review"]` is visible and contains text matching /No features in Review/i.

AC-015: Given the workspace has zero non-cancelled features (either no features at all or all cancelled), when the user toggles to Board view, then the board area displays an empty-state message and the "List" view toggle remains clickable.
  Test level: e2e
  Verification: In a workspace with no features, toggle to board, assert an empty-state element (e.g. `[data-testid="kanban-empty-state"]`) is visible, assert `[data-testid="view-toggle-list"]` is enabled and clickable.

---

## US-005 — View preference persistence and accessibility

AC-016: Given the Dashboard has loaded, when the user presses Tab until focus reaches the view toggle, then the toggle receives visible focus and is operable with both Enter and Space.
  Test level: e2e
  Verification: Playwright `page.locator('[data-testid="view-toggle-board"]').focus()`, press Enter, assert board visible. Reload, focus the list toggle, press Space, assert list visible.

AC-017: Given the view toggle is rendered, when inspected, then it has an accessible name (aria-label or visible text) describing its purpose (e.g. "Board view" / "List view").
  Test level: smoke
  Verification: Playwright `expect(page.locator('[data-testid="view-toggle-board"]')).toHaveAttribute('aria-label', /.+/)` or assert visible text content.

AC-018: Given the viewport is narrower than the combined width of the six phase columns (e.g. 400px wide), when the board renders, then the board container scrolls horizontally (overflow-x) and each column maintains a minimum fixed width (no column collapses to zero width).
  Test level: e2e
  Verification: Playwright `page.setViewportSize({width: 400, height: 800})`, toggle to board, assert `[data-testid="kanban-board"]` has `scrollWidth > clientWidth`, assert each `[data-testid^="kanban-column-"]` `boundingBox().width` >= the configured minimum column width.

AC-019: Given a column with more cards than fit the viewport height, when the user scrolls within the column body, then the column header remains visible (sticky) while the card list scrolls.
  Test level: e2e
  Verification: Seed 30+ features in one phase, toggle to board, scroll the column body, assert the column header's bounding box top remains within the viewport.

---

## Edge cases

AC-020: Given a feature whose `current_phase` is not one of the six known phases, when the board renders, then the card appears in a trailing `[data-testid="kanban-column-unknown"]` column whose header label is the raw `current_phase` string.
  Test level: unit
  Verification: Unit test the grouping function with a feature whose `current_phase` is `"weird"`; assert it lands in the "unknown" bucket.

AC-021: Given the `GET /api/features` request fails, when the Dashboard renders, then the existing `[data-testid="features-error"]` element is shown regardless of selected view mode and the board is not rendered.
  Test level: integration
  Verification: Playwright intercept `/api/features` and reply 500, `page.goto('/')`, assert `[data-testid="features-error"]` visible and `[data-testid="kanban-board"]` not visible.

---

## Constraint coverage

AC-CON-001: Given the board is rendered, when queried by testid, then `[data-testid="kanban-board"]`, one `[data-testid="kanban-column-${phase}"]` per phase, and one `[data-testid="kanban-card-${id}"]` per visible feature all resolve.
  Test level: e2e
  Verification: Covered by AC-001, AC-005.

AC-CON-002: Given the user toggles dark mode on, when the board renders, then all board elements are styled with dark-mode Tailwind variants (no unstyled white backgrounds, no contrast violations).
  Test level: smoke
  Verification: Playwright click the existing ThemeToggle, screenshot the board, assert no element with `bg-white` lacks a `dark:bg-*` counterpart via computed-style check on column and card backgrounds.

AC-CON-003: Given the board is rendered, when the network is inspected, then the only features request made is the existing `GET /api/features` — no new endpoint is called.
  Test level: integration
  Verification: Playwright network spy, assert exactly one request to `/api/features` and zero requests to any `/api/...kanban...` or other new path.

AC-CON-004: Given the board is rendered, when the columns are enumerated left-to-right, then their phase values match `['inception','planning','construction','review','testing','delivery']` exactly (Backlog, if present, precedes Inception; Unknown, if present, trails Delivery).
  Test level: unit
  Verification: Unit test the column-derivation function; assert output order equals the `PHASES` constant with backlog prepended and unknown appended only when relevant.

AC-CON-005: Given a card whose status is `gate_blocked`, `failed`, `recirculated`, or `waiting_for_human`, when classified by the board, then it is treated as blocked. Given a card whose status is `done`, when classified, then it is treated as done. No other status is classified into either bucket.
  Test level: unit
  Verification: Unit test the status-classification helper with every status string; assert the blocked set and done set match CON-005 exactly.

AC-CON-006: Given a board card is clicked, when the navigation occurs, then the URL is `/features/:id` (same route as the existing FeatureCard link).
  Test level: e2e
  Verification: Covered by AC-007.

AC-CON-007: Given a feature with `pending_questions_count > 0`, when its board card renders, then the existing `QuestionBadge` component is rendered inside the card.
  Test level: e2e
  Verification: Covered by AC-008.