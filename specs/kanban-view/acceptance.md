# Acceptance Criteria — kanban-view

Every criterion is Given/When/Then with a test level and a specific verification. UI feature → e2e via Playwright (port 18765) is the primary level; unit for pure sort logic.

## US-1 — Toggle Between List and Kanban Views

AC-001: Given the Dashboard is loaded with at least one feature, when the user clicks the "Kanban" toggle control, then the feature grid is replaced by a Kanban board containing six phase columns.
  Test level: e2e
  Verification: Playwright — locate `[data-testid="view-toggle-kanban"]`, click, assert `[data-testid="kanban-board"]` is visible and `[data-testid="feature-list"]` is not visible.

AC-002: Given the Kanban board is displayed, when the user clicks the "List" toggle control, then the board is replaced by the existing feature grid (`FeatureList`).
  Test level: e2e
  Verification: Playwright — locate `[data-testid="view-toggle-list"]`, click, assert `[data-testid="feature-list"]` is visible and `[data-testid="kanban-board"]` is not visible.

AC-003: Given the user has selected Kanban view, when the page is reloaded, then the Kanban view is displayed again without any user interaction.
  Test level: e2e
  Verification: Playwright — click Kanban toggle, `page.reload()`, assert `[data-testid="kanban-board"]` is visible. Confirm `localStorage.getItem("devteam-dashboard-view")` === `"kanban"` before and after reload.

AC-004: Given a user has never toggled before (no localStorage key), when they load the Dashboard, then the List view is shown.
  Test level: e2e
  Verification: Playwright — `page.evaluate(() => localStorage.clear())`, load Dashboard, assert `[data-testid="feature-list"]` is visible and `[data-testid="kanban-board"]` is not visible.

AC-005: Given the view-toggle control is rendered, then both toggle options have `data-testid` attributes (`view-toggle-list`, `view-toggle-kanban`) and the currently-active option is marked with an aria-pressed or visually-distinct state.
  Test level: e2e
  Verification: Playwright — assert both testids exist; assert the active option has `aria-pressed="true"` or a distinct active class.

## US-2 — View Features as Cards in Phase Columns

AC-006: Given features exist in multiple phases, when the Kanban view is displayed, then each feature appears in exactly one column and that column's phase matches the feature's `current_phase`.
  Test level: e2e
  Verification: Playwright — read features via the list view (each card has `data-testid="feature-card-{id}"` with `data-testid="feature-card-phase"` text), switch to Kanban, for each feature assert its card appears inside the column `[data-testid="kanban-column-{phase}"]` matching the phase read earlier. Assert no card appears in two columns.

AC-007: Given the Kanban view is displayed, then each column header shows the phase label from `PHASE_LABELS` (Inception, Planning, Construction, Review, Testing, Delivery) and a count.
  Test level: e2e
  Verification: Playwright — for each of the six testids `kanban-column-inception` ... `kanban-column-delivery`, assert the header text contains the expected label and a count (e.g. `/^Inception\s*\(\d+\)$/`).

AC-008: Given the Kanban view is displayed, then each card shows the feature title, priority badge, and status badge with the same content as the list-view card for that feature.
  Test level: e2e
  Verification: Playwright — for a sample feature, read `feature-card-title`, `feature-card-priority`, `feature-card-status` in list view, switch to Kanban, read the same testids inside that feature's column, assert text content is identical.

AC-009: Given the features list is still loading, when the Kanban view is the selected view, then the existing loading indicator is shown (`[data-testid="features-loading"]`) and no board is rendered.
  Test level: e2e
  Verification: Playwright — intercept `/api/features` with a delayed response, load Dashboard with view=kanban, assert `[data-testid="features-loading"]` is visible and `[data-testid="kanban-board"]` is not rendered.

AC-010: Given the features fetch returned an error, when the Kanban view is the selected view, then the existing error indicator is shown (`[data-testid="features-error"]`) and no board is rendered.
  Test level: e2e
  Verification: Playwright — intercept `/api/features` with a 500, load Dashboard with view=kanban, assert `[data-testid="features-error"]` is visible and `[data-testid="kanban-board"]` is not rendered.

AC-011: Given no features exist in a phase, when that column is rendered, then the column is still visible with its header, a count of 0, and an in-column empty-state message.
  Test level: e2e
  Verification: Playwright — seed state where at least one phase has zero features, load Kanban, assert `[data-testid="kanban-column-{empty-phase}"]` is visible, its header shows count `(0)`, and `[data-testid="kanban-column-empty-state"]` inside it is visible with non-empty text.

## US-3 — Click a Card to Navigate to Feature Detail

AC-012: Given the Kanban view is displayed with a card for feature X, when the user clicks the card, then the browser navigates to `/features/{id-of-X}` and the FeatureDetail page renders.
  Test level: e2e
  Verification: Playwright — in Kanban view, click `[data-testid="feature-card-{X.id}"]`, assert `page.url()` matches `/features/${X.id}`, assert `[data-testid="feature-detail-page"]` (or the existing detail root) is visible.

AC-013: Given a card has a pending-questions badge, when the user clicks the card body, then navigation still occurs (badge does not block the link).
  Test level: e2e
  Verification: Playwright — seed a feature with `pending_questions_count > 0`, switch to Kanban, click the card, assert navigation to `/features/{id}` succeeds.

## US-4 — Horizontal Scroll for Six Columns

AC-014: Given the viewport is narrower than the combined width of six columns, when the Kanban view is displayed, then the board scrolls horizontally and an off-screen column becomes visible after scrolling.
  Test level: e2e
  Verification: Playwright — `page.setViewportSize({width: 800, height: 600})`, load Kanban, assert `[data-testid="kanban-board"]` has `scrollWidth > clientWidth`, assert column `kanban-column-delivery` is not fully in the viewport initially; scroll the board right (`scrollLeft = scrollWidth`), assert `kanban-column-delivery` bounding box is now within the viewport.

AC-015: Given the board is scrolled horizontally, then each column's header remains aligned above its cards (header does not detach or desync).
  Test level: e2e
  Verification: Playwright — load Kanban at narrow width, for a sample column read the header's `boundingBox().x` and the first card's `boundingBox().x`, scroll the board, re-read both, assert the x-difference between header and card stays constant (within 1px).

## US-5 — Card Ordering Within Columns

AC-016: Given a column contains features of priorities P1, P2, and P3, when the column is rendered, then P1 cards appear above P2 cards, which appear above P3 cards.
  Test level: e2e
  Verification: Playwright — seed three features in the same phase with priorities 1, 2, 3, load Kanban, read the `data-testid="feature-card-priority"` text and `boundingBox().y` for each card inside `kanban-column-{phase}`, assert y-order matches priority order (P1 smallest y).

AC-017: Given a column contains two features of the same priority with different `updated_at`, when the column is rendered, then the more recently updated card appears above the older one.
  Test level: e2e
  Verification: Playwright — seed two features same phase, same priority, feature A `updated_at` newer than B, load Kanban, assert A's card `boundingBox().y` < B's card `boundingBox().y`.

AC-018: Given a column's card ordering logic, when the ordering function is invoked with an unsorted feature list, then it returns cards ordered by priority asc then `updated_at` desc.
  Test level: unit
  Verification: Vitest (or existing UI unit test runner) — import the sort helper, call with a fixture array, assert output order. (If the helper is inlined into the component, extract a pure `orderCards(features)` function so it is unit-testable — no other refactor required.)

## US-6 — Empty Board State

AC-019: Given there are zero features, when the Dashboard loads, then the existing `EmptyState` component is rendered (not six empty columns).
  Test level: e2e
  Verification: Playwright — seed empty repo state, load Dashboard, assert `[data-testid="empty-state"]` is visible and `[data-testid="kanban-board"]` is not in the DOM.

AC-020: Given there are zero features, then the view-toggle control is not visible (or is disabled).
  Test level: e2e
  Verification: Playwright — seed empty state, load Dashboard, assert neither `[data-testid="view-toggle-list"]` nor `[data-testid="view-toggle-kanban"]` is visible (or both have `aria-disabled="true"` / `disabled` attribute).

## Constraint-Register Acceptance Criteria

AC-CON-001: Given the implementation is built, then it builds and lints with the repo's existing commands — `npm run build` and `npm run lint` in `ui/`.
  Test level: smoke
  Verification: Run `cd ui && npm run build` and `cd ui && npm run lint`; both exit 0. No new build tool introduced.

AC-CON-002: Given e2e tests are run, then they execute against port 18765 via the existing `ui/playwright.config.ts`.
  Test level: smoke
  Verification: `cd ui && npx playwright test --reporter=line` runs against the config's `webServer` on 18765; no test hardcodes 8765.

AC-CON-003: Given the Kanban column renders cards, then the card markup is produced by the existing `FeatureCard` component (imported, not re-implemented inline).
  Test level: unit
  Verification: Grep the new Kanban component source — it imports `FeatureCard` from `./FeatureCard` and renders `<FeatureCard ... />`; no duplicated card JSX (title/status/priority/gate markup) in the column body.

AC-CON-004: Given the board renders columns, then the column set and order match `PHASES` exactly: inception, planning, construction, review, testing, delivery.
  Test level: e2e
  Verification: Playwright — read `data-testid` of all `kanban-column-*` elements in DOM order, assert the sequence is `kanban-column-inception`, `...-planning`, `...-construction`, `...-review`, `...-testing`, `...-delivery`.

AC-CON-005: Given column headers are rendered, then the label text is derived from `PHASE_LABELS` (imported), not a hardcoded string literal.
  Test level: unit
  Verification: Grep the new component source — `PHASE_LABELS` is imported from `../types`; no string literal "Inception"/"Planning"/etc. used as a header label.

AC-CON-006: Given any new interactive element is added (toggles, columns, cards), then it carries a `data-testid` attribute following the existing `kebab-case` naming pattern.
  Test level: e2e
  Verification: Playwright — every selector used in the test suite is a `[data-testid=...]` selector; no class-based or text-based selectors for interactive elements.

AC-CON-007: Given the testing phase runs, then the Tester's report names specific Playwright spec files and specific assertions traced to user stories (AC-NNN).
  Test level: process
  Verification: The test-report.md maps each AC-NNN to a `describe`/`it` block in a named `ui/e2e/kanban.spec.ts` (or similar) file with quoted assertion lines. No "works as expected" claims.

AC-CON-008: Given the implementation is complete, then `ui/package.json` has no new runtime dependencies (or, if one was unavoidable, the plan.md justifies it).
  Test level: smoke
  Verification: `git diff main -- ui/package.json ui/package-lock.json` shows no added runtime deps, or the added dep is documented in plan.md with rationale. Tailwind / react-router / react-query cover all needs.

AC-CON-009: Given a column has zero features, then the column shows an empty-state message (not an error, not hidden).
  Test level: e2e
  Verification: (= AC-011) — covered by the empty-column assertion there.

AC-CON-010: Given `listFeatures` is loading or errored, then the Kanban view shows the same loading/error UI as the list view (not a blank board).
  Test level: e2e
  Verification: (= AC-009, AC-010) — covered there.

## Coverage Matrix

| User Story | ACs | Test Levels |
|---|---|---|
| US-1 Toggle | AC-001..AC-005 | e2e |
| US-2 Cards in columns | AC-006..AC-011 | e2e |
| US-3 Click to detail | AC-012, AC-013 | e2e |
| US-4 Horizontal scroll | AC-014, AC-015 | e2e |
| US-5 Card ordering | AC-016, AC-017, AC-018 | e2e, unit |
| US-6 Empty board | AC-019, AC-020 | e2e |
| Constraints | AC-CON-001..AC-CON-010 | smoke, e2e, unit, process |

Every user story has at least one smoke/e2e criterion (UI change → e2e required). US-5 has a unit criterion for the pure sort logic. Error/loading paths (AC-009, AC-010) and empty states (AC-011, AC-019, AC-020) are explicitly covered.