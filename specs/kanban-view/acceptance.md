# Acceptance Criteria: Kanban View

**Feature ID**: kanban-view
**Created**: 2026-06-21

Every criterion follows `Given / When / Then` with a test level and verification method. Constraint-driven criteria reference their source CON-NNN from `spec.md`.

## US-001 — See all features organized by pipeline phase

### AC-001
Given a system with at least one feature in each of the `inception`, `planning`, and `delivery` phases, when the user opens the Kanban view, then each feature appears in the column whose key matches its `current_phase` field.
- Test level: e2e
- Verification: Playwright. Seed features via `POST /api/features` then advance selected features to target phases. Load the board, for each seeded feature assert a card with `data-testid="feature-card-{id}"` exists inside `data-testid="kanban-column-{current_phase}"`.
- Source: US-001, CON-001

### AC-002
Given the Kanban view is rendered, when the user inspects the column order, then the columns appear left-to-right as: Backlog, Inception, Planning, Construction, Review, Testing, Delivery.
- Test level: e2e
- Verification: Playwright. Query `[data-testid^="kanban-column-"]` children of `[data-testid="kanban-board"]`, assert the ordered list of their `data-testid` suffixes equals `["backlog","inception","planning","construction","review","testing","delivery"]`.
- Source: CON-001

### AC-003
Given the board is loaded, when the user reads each column header, then every column header displays the column display name and a numeric card count equal to the number of cards in that column.
- Test level: e2e
- Verification: Playwright. For each `kanban-column-*`, assert the header text contains the expected label (e.g. "Inception") and a count integer; assert the count equals the number of `[data-testid^="feature-card-"]` descendants in that column.
- Source: US-001, FR-008

## US-002 — Not-yet-started features appear in Backlog

### AC-004
Given a feature with `status == "draft"` and `current_phase == "inception"` (freshly intake'd, no phase run), when the user opens the Kanban view, then that feature's card appears in `data-testid="kanban-column-backlog"` and NOT in `data-testid="kanban-column-inception"`.
- Test level: e2e
- Verification: Playwright. Create a feature via `POST /api/features` and do not run any phase. Load the board, assert the card is a descendant of `kanban-column-backlog` and is NOT a descendant of `kanban-column-inception`.
- Source: US-002, CON-002, FR-002

### AC-005
Given a feature with `status == "in_progress"` and `current_phase == "inception"` (inception phase has started), when the user opens the Kanban view, then that feature's card appears in `data-testid="kanban-column-inception"` and NOT in `data-testid="kanban-column-backlog"`.
- Test level: e2e
- Verification: Playwright. Create a feature, trigger `POST /api/features/{id}/run` to start inception, wait for status to become `in_progress`. Load the board, assert the card is in `kanban-column-inception` and not in `kanban-column-backlog`.
- Source: US-002, CON-002, FR-002, FR-003

### AC-006
Given a feature with `status == "done"` and `current_phase == "delivery"`, when the user opens the Kanban view, then that feature's card appears in `data-testid="kanban-column-delivery"` (terminal features are NOT hidden).
- Test level: e2e
- Verification: Playwright. Seed or find a done feature in delivery. Load the board, assert the card is in `kanban-column-delivery`.
- Source: CON-009, FR-003

## US-003 — Switch between list view and Kanban view

### AC-007
Given the Dashboard list view is loaded, when the user activates the Kanban view affordance, then the Kanban board renders and the Dashboard list is no longer the primary content.
- Test level: e2e
- Verification: Playwright. Load `/`, assert `data-testid="feature-list"` is visible. Click the Kanban view toggle. Assert `data-testid="kanban-board"` is visible and `data-testid="feature-list"` is not visible.
- Source: US-003, CON-007, FR-006

### AC-008
Given the Kanban view is loaded, when the user activates the list view affordance, then the Dashboard list renders and the Kanban board is no longer the primary content.
- Test level: e2e
- Verification: Playwright. From the Kanban view, click the list view toggle. Assert `data-testid="feature-list"` is visible and `data-testid="kanban-board"` is not visible.
- Source: US-003, CON-007, FR-006

### AC-009
Given the Dashboard shows a total feature count badge of N, when the user switches to the Kanban view, then the total feature count badge on the Kanban view also shows N.
- Test level: e2e
- Verification: Playwright. Load `/`, read `data-testid="feature-count-badge"` text → N. Switch to Kanban. Assert the count badge (same `data-testid="feature-count-badge"`) still reads N.
- Source: CON-010, FR-007

## US-004 — Click a card to open feature detail

### AC-010
Given a feature card on the Kanban board, when the user clicks the card, then the browser navigates to `/features/{id}` for that feature.
- Test level: e2e
- Verification: Playwright. Seed a feature, load the board, click the card with `data-testid="feature-card-{id}"`, assert the current URL path equals `/features/{id}` and the FeatureDetail page renders.
- Source: US-004, CON-005, FR-005

## US-005 — Empty board renders cleanly

### AC-011
Given a system with zero features (`GET /api/features` returns `{"features":[],"total_count":0}`), when the user opens the Kanban view, then all 7 columns render with an empty-state message and no browser console errors occur.
- Test level: e2e
- Verification: Playwright. Point the test at a fresh state with no specs (or clean specs dir). Load the board. For each `kanban-column-*`, assert the column body contains a non-empty empty-state message and zero `feature-card-*` descendants. Capture console messages via Playwright `page.on('console')` and assert zero entries of type `error`.
- Source: US-005, CON-004, FR-009

### AC-012
Given the API returns `features: []` (empty array, not null), when the board renders, then no column throws a "cannot read properties of undefined / map of null" error and the page does not crash.
- Test level: unit
- Verification: Jest/Vitest unit test of the grouping function with input `[]` — assert it returns 7 columns each with an empty cards array, no throw.
- Source: CON-004

### AC-013
Given a board where 5 features all sit in `planning` and every other phase is empty, when the board renders, then the `planning` column shows 5 cards and every other column shows its empty-state message with count 0.
- Test level: e2e
- Verification: Playwright. Seed 5 features, advance all to planning. Load the board, assert `kanban-column-planning` has 5 `feature-card-*` descendants and every other `kanban-column-*` has 0 cards and a visible empty-state message.
- Source: US-005, FR-009

## US-006 — Board reflects live updates during processing

### AC-014
Given the Kanban view is open with a feature in `inception` and the react-query `['features']` cache is valid, when that feature advances to `planning` (via an action that invalidates the `['features']` cache), then the card moves from `kanban-column-inception` to `kanban-column-planning` without a full page reload.
- Test level: e2e
- Verification: Playwright. Seed a feature in inception. Load the board, assert card in `kanban-column-inception`. Trigger an advance (e.g. via `POST /api/features/{id}/advance` after gate passes, or by directly invalidating the query through the existing mutation flow). Wait for the query to refetch. Assert the card is now in `kanban-column-planning` and the URL did not change.
- Source: US-006, FR-014

## Constraint-driven criteria

### AC-CON-003 (no new backend endpoint)
Given the implemented feature, when the codebase is inspected, then no new route is registered in `internal/api/server.go`'s `NewServer` mux and no new function is added to `ui/src/api/client.ts` for kanban-specific data fetching (the board reuses `listFeatures`).
- Test level: integration
- Verification: Diff/grep check — `git diff main -- internal/api/server.go ui/src/api/client.ts` shows no new `mux.HandleFunc` line and no new client function beyond existing ones. Assert `listFeatures` is the sole data source imported by the board component.
- Source: CON-003, FR-004

### AC-CON-005 (reuse FeatureCard)
Given the board component is implemented, when its source is inspected, then it imports and renders the existing `FeatureCard` component for each card (or a thin wrapper that delegates to `FeatureCard`); it does not re-implement card markup from scratch.
- Test level: unit
- Verification: Read the board component source, assert an `import FeatureCard` (or `import ... from '../components/FeatureCard'`) and `<FeatureCard ... />` usage in the render path.
- Source: CON-005, FR-005

### AC-CON-006 (no new UI dependency)
Given the implemented feature, when `ui/package.json` is compared to `main`, then no dependency has been added to `dependencies` or `devDependencies`.
- Test level: integration
- Verification: `git diff main -- ui/package.json` shows no additions in the `dependencies` or `devDependencies` blocks (lockfile churn from reinstall is acceptable; the constraint is on declared deps).
- Source: CON-006, FR-011

### AC-CON-008 (dark mode)
Given the user has enabled dark mode via the existing `ThemeToggle`, when the Kanban view renders, then the board container, each column, and each card render with dark-mode background/text classes (Tailwind `dark:` variants) consistent with the Dashboard.
- Test level: e2e
- Verification: Playwright. Toggle dark mode. Load the board. Assert the board container and at least one column have computed background colors matching the dark palette (e.g. `rgb(31, 41, 55)` for `bg-gray-800`) rather than the light palette. Visual regression snapshot optional.
- Source: CON-008, FR-010

### AC-CON-011 (data-testid stability)
Given the Kanban view is rendered, when an E2E selector queries by `data-testid`, then elements `kanban-board`, `kanban-column-backlog`, `kanban-column-inception`, `kanban-column-planning`, `kanban-column-construction`, `kanban-column-review`, `kanban-column-testing`, `kanban-column-delivery` all exist exactly once.
- Test level: e2e
- Verification: Playwright. Load the board, for each testid assert exactly one element exists.
- Source: CON-011, FR-012

## Error path criteria

### AC-ERR-001
Given `GET /api/features` returns HTTP 500, when the user opens the Kanban view, then the board renders an error banner containing the text "Failed to load features" and does not crash, throw an uncaught exception, or render a blank page.
- Test level: integration
- Verification: Playwright with route interception — `page.route('**/api/features', r => r.fulfill({ status: 500, body: JSON.stringify({error:'internal_error', details:'db down'}) }))`. Load the board. Assert an error banner is visible with "Failed to load features" text. Assert no `page.on('pageerror')` event fired.
- Source: Error Scenarios table, FR-009

### AC-ERR-002
Given the Kanban view is loaded and a query refetch fails mid-session, when the refetch errors, then an error banner appears and the previously-rendered cards remain visible (stale data is better than a blank board) OR the board shows the error banner with empty columns — either is acceptable as long as no uncaught exception occurs.
- Test level: integration
- Verification: Playwright. Load the board successfully, then intercept the next `GET /api/features` with 500. Trigger a refetch (e.g. invalidate via a mutation). Assert no `pageerror` event; assert an error indicator is visible.
- Source: Error Scenarios table

### AC-ERR-003
Given the user clicks a feature card whose `id` was deleted between board load and click, when the browser navigates to `/features/{id}`, then the existing FeatureDetail not-found state is shown (the board does not need to handle this itself).
- Test level: e2e
- Verification: Playwright. Seed a feature, load the board, delete the feature's spec dir via filesystem (or a separate delete call if available), click the card, assert the FeatureDetail page renders its existing 404/not-found state without a console error.
- Source: Error Scenarios table

## Test level summary

| AC IDs | Level |
|--------|-------|
| AC-001, AC-002, AC-003, AC-004, AC-005, AC-006, AC-007, AC-008, AC-009, AC-010, AC-011, AC-013, AC-014, AC-CON-008, AC-CON-011, AC-ERR-003 | e2e |
| AC-CON-003, AC-CON-006, AC-ERR-001, AC-ERR-002 | integration |
| AC-012, AC-CON-005 | unit |

Every user story has at least one criterion per relevant test level. UI changes → smoke + integration + e2e are all represented (e2e via Playwright, integration via route interception + API diff, unit via the grouping function). Error paths and empty states are explicitly covered (AC-011, AC-012, AC-013, AC-ERR-001, AC-ERR-002, AC-ERR-003).