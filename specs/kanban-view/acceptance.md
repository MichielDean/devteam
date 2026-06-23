# Acceptance Criteria — kanban-view

Every criterion follows Given/When/Then with a test level and verification method. Test levels: `smoke` (service starts, no crash), `integration` (API contract), `e2e` (Playwright browser), `unit` (pure logic).

## US-001 — Toggle Between List and Kanban Board

AC-001: Given the Dashboard is loaded with at least one feature, when the user locates the view toggle, then both "List" and "Board" options are present and the active option is "List" (conservative default per FR-003).
  Test level: e2e
  Verification: `await expect(page.locator('[data-testid="view-toggle"]')).toBeVisible(); await expect(page.locator('[data-testid="view-toggle-list"][aria-pressed="true"]')).toBeVisible();`

AC-002: Given the Dashboard is loaded and the active view is "List", when the user clicks the "Board" toggle, then six phase column headers (Inception, Planning, Construction, Review, Testing, Delivery) render and the `FeatureList` grid is no longer present.
  Test level: e2e
  Verification: `page.locator('[data-testid="view-toggle-board"]').click();` then assert each `[data-testid^="kanban-column-"]` header text matches `PHASE_LABELS` and `[data-testid="feature-list"]` has count 0.

AC-003: Given the active view is "Board", when the user clicks the "List" toggle, then the `FeatureList` component renders and no `kanban-column-*` elements are present.
  Test level: e2e
  Verification: `page.locator('[data-testid="view-toggle-list"]').click();` assert `[data-testid="feature-list"]` visible and `[data-testid^="kanban-column-"]` count 0.

AC-004: Given the user has selected "Board", when they reload the Dashboard in the same browser session, then "Board" is still the active view (sessionStorage persistence).
  Test level: e2e
  Verification: Click Board, `page.reload()`, assert `[data-testid="view-toggle-board"][aria-pressed="true"]` visible.

AC-005: Given a fresh browser session with no prior view choice, when the Dashboard loads, then the active view is "List" (conservative default per FR-003).
  Test level: e2e
  Verification: New context, navigate to `/`, assert `[data-testid="view-toggle-list"][aria-pressed="true"]`.

AC-006: Given zero features exist, when the Dashboard loads, then the view toggle is NOT visible and `EmptyState` renders.
  Test level: e2e
  Verification: Route `/api/features` to `{features:[], total_count:0}`; assert `[data-testid="view-toggle"]` count 0 and `EmptyState` text visible.

## US-002 — Feature Cards on the Board Show Key State

AC-007: Given a feature with `current_phase='planning'`, `priority=1`, `status='in_progress'`, when the board renders, then a card with the feature's title is present in the Planning column with a P1 priority badge and an "In Progress" status badge.
  Test level: e2e
  Verification: `page.locator('[data-testid="kanban-column-planning"] [data-testid*="kanban-card-"]')` contains the title; assert badge text via `[data-testid="kanban-card-priority"]` = "P1" and `[data-testid="kanban-card-status"]` = "In Progress".

AC-008: Given a feature with `pending_questions_count > 0`, when its board card renders, then a pending-questions badge is visible on the card.
  Test level: e2e
  Verification: Assert `[data-testid*="kanban-card-"] [data-testid="question-badge"]` is visible for that feature.

AC-009: Given a feature whose current-phase `gate_result` is present, when the card renders, then a passed (✓) or failed (✗) gate indicator is visible.
  Test level: e2e
  Verification: Assert `[data-testid="kanban-card-gate"]` text matches "✓ Gate passed" or "✗ Gate failed" per `gate_result.passed`.

AC-010: Given a board card for feature `:id`, when the user clicks the card, then the browser navigates to `/features/:id`.
  Test level: e2e
  Verification: `page.locator('[data-testid="kanban-card-${id}"]').click(); await expect(page).toHaveURL(/\/features\/${id}/);`

AC-011: Given a feature whose `current_phase` is not one of the six known phases, when the board renders, then the card is placed in a trailing "Other" column (`kanban-column-other`) and no crash occurs.
  Test level: unit
  Verification: `groupFeaturesByPhase([{current_phase:'weird', ...}, ...])` returns `{other: [feature]}`. (Pure function unit test.)

AC-012: Given a feature with `status='gate_blocked'`, when its board card renders, then the card has a distinct visual flag (e.g., red ring border class) vs. a normal card.
  Test level: e2e
  Verification: Assert `[data-testid="kanban-card-${id}"]` has class containing `ring-red` (or equivalent flag class) when status is `gate_blocked`.

AC-013: Given a feature with `status='waiting_for_human'`, when its board card renders, then the card has a distinct visual flag (yellow ring) vs. a normal card.
  Test level: e2e
  Verification: Assert `[data-testid="kanban-card-${id}"]` has class containing `ring-yellow` when status is `waiting_for_human`.

AC-014: Given the features query is loading, when the Board view is active, then the loading indicator renders (`features-loading` testid pattern) and no column bodies render yet.
  Test level: e2e
  Verification: Route `/api/features` with delay; assert `[data-testid="features-loading"]` visible and `[data-testid^="kanban-column-"]` count 0 until resolved.

AC-015: Given the features query returns an error, when the Board view is active, then the error indicator renders (`features-error` testid) and the board does not render.
  Test level: e2e
  Verification: Route `/api/features` to 500; assert `[data-testid="features-error"]` visible and `[data-testid^="kanban-column-"]` count 0.

AC-016: Given the Board view is active, when the network is inspected, then exactly one `GET /api/features` request is made (no second fetch for the board).
  Test level: integration
  Verification: Playwright `page.on('request')` count for `/api/features` === 1 during Board render. (CON-007)

## US-003 — Empty Columns and Empty Board

AC-017: Given no features have `current_phase='testing'`, when the board renders, then the Testing column header is visible and its body contains a muted "No features" placeholder.
  Test level: e2e
  Verification: Assert `[data-testid="kanban-column-testing"]` header visible and `[data-testid="kanban-column-empty-testing"]` contains "No features".

AC-018: Given zero features exist, when the Dashboard loads, then `EmptyState` renders and the view toggle is NOT visible.
  Test level: e2e
  Verification: (Same as AC-006 — listed once under US-1; cross-referenced here for US-3 traceability.)

AC-019: Given the board renders with at least one feature in some column, when the user inspects every column, then all six phase columns are present regardless of whether they have features.
  Test level: e2e
  Verification: Assert exactly 6 `[data-testid^="kanban-column-"]` elements (plus optional "Other" only when an unknown phase exists).

## US-004 — Column Overflow Handling

AC-020: Given a column with more cards than fit the viewport height, when the user scrolls within that column, then the column body scrolls vertically while the column header and other columns remain fixed (no page-level scroll).
  Test level: e2e
  Verification: Seed 50 features in one phase; assert `scrollHeight > clientHeight` on that column's body element and the page body does not scroll (body scrollTop === 0).

AC-021: Given the board is rendered, when the viewport is resized shorter, then each column's scroll area adjusts to the new viewport height (board height bounded to viewport).
  Test level: e2e
  Verification: `page.setViewportSize({width:1280, height:400})`; assert each `[data-testid^="kanban-column-"]` body `clientHeight <= 400 - headerHeight`.

AC-022: Given a viewport narrower than the board's natural width, when the board renders, then the board container scrolls horizontally and each column has a minimum width of 240px.
  Test level: e2e
  Verification: `page.setViewportSize({width:600, height:800})`; assert board container has `overflow-x: auto` (or scroll) and each column `getBoundingClientRect().width >= 240`.

## Constraint Traceability

| Constraint | AC |
|---|---|
| CON-001 (Playwright :18765) | All e2e ACs run via Playwright config |
| CON-002 (file paths) | Verified in review (architect/developer phase) |
| CON-003 (minimal deps) | AC-016 implicitly — no new endpoint; package.json diff check in review |
| CON-004 (no regression) | AC-001, AC-003 preserve the list view as a selectable toggle option; existing app.spec.ts must still pass (default remains List per FR-003, so app.spec.ts requires no change) |
| CON-005 (reuse phase/status constants) | AC-007, AC-019 — column headers and badge labels match `PHASE_LABELS`/`STATUS_LABELS` |
| CON-006 (card chrome parity) | AC-007, AC-008, AC-009 — board card badges match FeatureCard badges |
| CON-007 (single fetch) | AC-016 |
| CON-008 (loading/error/empty states) | AC-006, AC-014, AC-015, AC-017, AC-018 |
| CON-009 (unknown enum defensive) | AC-011 |

## Extension ACs

Security extension: N/A — view-only, no input, no auth, no mutation. Documented in spec.md.

Resiliency extension: N/A — reuses existing react-query error handling. Loading (AC-014) and error (AC-015) states covered. No new external call (AC-016).