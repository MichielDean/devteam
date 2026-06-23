# Acceptance Criteria — Kanban View

Every criterion follows Given/When/Then with an explicit test level and verification method. `data-testid` values are named here so the Tester and E2E author can rely on them.

---

## US-1 — Toggle Between List and Kanban Views

**AC-001**: Given the Dashboard is loaded with at least one feature and no `?view=` query param and no localStorage preference, when the Dashboard renders, then the List view is rendered by default and the "List" toggle button shows an active/selected state (`data-testid="view-toggle-list"` has the active class) and the "Kanban" toggle button (`data-testid="view-toggle-kanban"`) does not.
  Test level: e2e
  Verification: Playwright — navigate to `/`, assert `data-testid="feature-list"` is visible and `data-testid="kanban-board"` is not; assert `view-toggle-list` has active styling, `view-toggle-kanban` does not.

**AC-002**: Given the List view is displayed, when the user clicks `data-testid="view-toggle-kanban"`, then the Kanban board (`data-testid="kanban-board"`) renders, the `FeatureList` (`data-testid="feature-list"`) is no longer visible, and `view-toggle-kanban` shows the active state while `view-toggle-list` does not.
  Test level: e2e
  Verification: Playwright — click `view-toggle-kanban`, assert `kanban-board` visible, `feature-list` not visible, toggle active states flipped.

**AC-003**: Given the Kanban board is displayed, when the user clicks `data-testid="view-toggle-list"`, then the `FeatureList` renders, the `kanban-board` is no longer visible, and `view-toggle-list` shows the active state.
  Test level: e2e
  Verification: Playwright — click `view-toggle-list`, assert `feature-list` visible, `kanban-board` not visible, toggle active states flipped.

**AC-004**: Given the user has selected the Kanban view, when the page is reloaded, then the Kanban board is rendered again (preference persisted) and the URL contains `?view=kanban`.
  Test level: e2e
  Verification: Playwright — click `view-toggle-kanban`, assert URL contains `?view=kanban`, `page.reload()`, assert `kanban-board` is visible and URL still contains `?view=kanban`.

**AC-005**: Given the browser navigates directly to `/?view=kanban` with no localStorage preference, when the Dashboard renders, then the Kanban board is shown (URL query param takes precedence over the default).
  Test level: e2e
  Verification: Playwright — `page.goto('/?view=kanban')`, assert `kanban-board` visible, `feature-list` not visible.

**AC-006**: Given localStorage has `devteam-dashboard-view = 'kanban'` and the URL has no `?view=` param, when the Dashboard renders, then the Kanban board is shown (localStorage fallback).
  Test level: e2e
  Verification: Playwright — `page.addInitScript` to set localStorage, `page.goto('/')`, assert `kanban-board` visible.

**AC-007**: Given the URL has `?view=kanban` and localStorage has `devteam-dashboard-view = 'list'`, when the Dashboard renders, then the Kanban board is shown (URL precedence over localStorage).
  Test level: e2e
  Verification: Playwright — set localStorage to 'list' via init script, `page.goto('/?view=kanban')`, assert `kanban-board` visible.

**AC-008**: Given the features query is still loading (isLoading true), when the Dashboard renders, then both `view-toggle-list` and `view-toggle-kanban` are disabled (have the `disabled` attribute).
  Test level: e2e
  Verification: Playwright — intercept `/api/features` with a delayed response, navigate to `/`, assert both toggle buttons have `disabled` attribute while loading spinner (`features-loading`) is visible.

**AC-009**: Given no features exist (empty Dashboard), when the Dashboard renders, then the existing `<EmptyState>` (`data-testid="empty-state"` or existing empty-state testid) is shown and neither the view toggle nor the Kanban board is rendered.
  Test level: e2e
  Verification: Playwright — mock `/api/features` returning `{"features":[],"total_count":0}`, navigate to `/`, assert empty-state CTA visible, assert `view-toggle-list` and `view-toggle-kanban` and `kanban-board` are not in the DOM.

---

## US-2 — View Features as Kanban Cards Organized by Phase

**AC-010**: Given features exist whose `current_phase` values span all six phases (one feature per phase), when the Kanban board renders, then six columns are present with headers reading "Inception", "Planning", "Construction", "Review", "Testing", "Delivery" left-to-right, each column has `data-testid="kanban-column-${phase}"`, and each column contains exactly one card (`data-testid="kanban-card-${feature.id}"`) whose feature's `current_phase` equals that column's phase.
  Test level: e2e
  Verification: Playwright — seed features via API or DB, navigate to `/?view=kanban`, for each phase assert `kanban-column-${phase}` header text matches `PHASE_LABELS[phase]` and the column contains exactly the card(s) whose `current_phase === phase`.

**AC-011**: Given a feature with `current_phase = 'construction'`, when the board renders, then that feature's card appears inside `data-testid="kanban-column-construction"` and inside no other column.
  Test level: e2e
  Verification: Playwright — assert `kanban-card-${id}` is a descendant of `kanban-column-construction` and not a descendant of any other `kanban-column-*`.

**AC-012**: Given a feature with `priority = 1` and `status = 'in_progress'`, when its Kanban card renders, then the card displays the priority badge text matching `PRIORITY_LABELS[1]` ("P1 - Critical") and the status badge text matching `STATUS_LABELS['in_progress']` ("In Progress"), using `data-testid="kanban-card-priority"` and `data-testid="kanban-card-status"`.
  Test level: e2e
  Verification: Playwright — assert badge text content matches the existing label maps.

**AC-013**: Given a feature with `pending_questions_count > 0`, when its Kanban card renders, then a question badge is visible on the card (`data-testid="kanban-card-question-badge"`), consistent with the list view `FeatureCard`.
  Test level: e2e
  Verification: Playwright — seed a feature with `pending_questions_count = 2`, assert the question badge is visible on the kanban card.

**AC-014**: Given a Kanban card is rendered for feature with id `X`, when the user clicks the card, then the app navigates to `/features/X` (the existing FeatureDetail route).
  Test level: e2e
  Verification: Playwright — click `kanban-card-${X}`, assert `page.url()` ends with `/features/X` and the feature detail page (`data-testid="feature-detail-page"` or existing detail testid) is visible.

**AC-015**: Given a feature with `status = 'cancelled'`, when the board renders, then no card for that feature appears in any column.
  Test level: e2e
  Verification: Playwright — seed a cancelled feature, navigate to `/?view=kanban`, assert no `kanban-card-${id}` for that feature exists in the DOM.

**AC-016**: Given a feature with `current_phase = 'something_unknown'` (not one of the six known phases), when the board renders, then the card appears in a trailing column with header "Unknown" (`data-testid="kanban-column-unknown"`).
  Test level: e2e
  Verification: Playwright — seed a feature with an unrecognized phase, assert `kanban-column-unknown` exists and contains the card, and that it appears after the Delivery column.

**AC-017**: Given the existing `useQuery({ queryKey: ['features'] })` cache is populated, when the user toggles from List to Kanban, then no new HTTP request to `/api/features` is made (the board renders from cached data).
  Test level: integration
  Verification: Playwright — listen to network requests, navigate to `/`, wait for initial `/api/features`, click `view-toggle-kanban`, assert no second `/api/features` request is fired.

---

## US-3 — Column Count Badges and Empty Columns

**AC-018**: Given the Inception column contains 3 visible cards and the Delivery column contains 0 visible cards, when the board renders, then the Inception column header (`data-testid="kanban-column-header-inception"`) displays text containing "Inception" and "(3)", and the Delivery column header (`data-testid="kanban-column-header-delivery"`) displays text containing "Delivery" and "(0)".
  Test level: e2e
  Verification: Playwright — seed 3 inception features and 0 delivery features, assert header text contents.

**AC-019**: Given a phase column has zero visible cards, when the board renders, then the column body displays an empty-state message element (`data-testid="kanban-column-empty-${phase}"`) with visible text (e.g., "No features in this phase"), not a blank space.
  Test level: e2e
  Verification: Playwright — seed no features in the Review phase, assert `kanban-column-empty-review` is visible and has non-empty text.

**AC-020**: Given the priority filter is set to "P1" and a column has 2 features but only 1 is P1, when the board renders, then that column's count badge shows "(1)" (counts reflect the filtered/visible set, not the total).
  Test level: e2e
  Verification: Playwright — seed 2 features in Inception (one P1, one P2), select priority filter "P1", assert Inception header shows "(1)".

**AC-021**: Given zero features exist at all but the empty state is bypassed (e.g., via direct `?view=kanban` navigation when the API returns an empty list), when the board renders, then all six column headers show "(0)" and each column body shows the empty-state message.
  Test level: e2e
  Verification: Playwright — mock `/api/features` returning empty, `page.goto('/?view=kanban')` (if the board renders in the empty state per the implementation note in spec edge cases), assert each column header shows "(0)" and each `kanban-column-empty-*` is visible. If the implementation hides the board entirely in the empty state, this AC is satisfied by AC-009 instead — document which path the implementation took.

---

## US-4 — Horizontal Scroll for Narrow Viewports

**AC-022**: Given the viewport is 768px wide, when the Kanban board renders, then the board container (`data-testid="kanban-board"`) has a scrollable overflow on the x-axis (`overflow-x` computed style is `auto` or `scroll`) and each column (`data-testid="kanban-column-*"`) has a computed width of at least 240px.
  Test level: e2e
  Verification: Playwright — `page.setViewportSize({width:768,height:1024})`, navigate to `/?view=kanban`, assert `kanban-board` computed `overflow-x` is `auto`/`scroll`, assert each column `boundingBox().width >= 240`.

**AC-023**: Given the viewport is 1280px wide, when the Kanban board renders, then all six columns are visible without horizontal scrolling (the board's `scrollWidth <= clientWidth`).
  Test level: e2e
  Verification: Playwright — `page.setViewportSize({width:1280,height:1024})`, navigate to `/?view=kanban`, assert `kanban-board` `scrollWidth <= clientWidth`.

**AC-024**: Given a narrow viewport, when the user scrolls the board horizontally, then columns that were off-screen become visible (horizontal scroll reaches all six columns).
  Test level: e2e
  Verification: Playwright — 768px viewport, scroll `kanban-board` to the right by `scrollWidth`, assert the Delivery column header is now in the viewport (`boundingBox().x` within `[0, viewportWidth]`).

---

## US-5 — Priority Filter on the Board

**AC-025**: Given the board displays features of priorities 1, 2, and 3 across multiple columns, when the user selects "P1" in the priority filter (`data-testid="priority-filter"`), then only cards with `priority === 1` remain visible in every column and cards with priority 2 or 3 are not present in the DOM.
  Test level: e2e
  Verification: Playwright — seed mixed-priority features, select "P1" in `priority-filter`, assert every `kanban-card-*` visible corresponds to a P1 feature and no P2/P3 cards exist in the DOM.

**AC-026**: Given the priority filter is set to "P1", when the user selects "All", then all cards reappear in their correct columns and the column count badges return to their unfiltered values.
  Test level: e2e
  Verification: Playwright — select "P1", record counts, select "All", assert all cards visible and counts match the unfiltered counts.

**AC-027**: Given the priority filter is set to "P1" and a column has zero P1 features, when the board renders, then that column's header shows "(0)" and the column body shows the empty-state message (`kanban-column-empty-${phase}`).
  Test level: e2e
  Verification: Playwright — seed no P1 features in Review, select "P1", assert Review header shows "(0)" and `kanban-column-empty-review` visible.

**AC-028**: Given the priority filter control is rendered, when the user inspects it, then it offers exactly the options "All", "P1", "P2", "P3" and defaults to "All".
  Test level: e2e
  Verification: Playwright — navigate to `/?view=kanban`, assert `priority-filter` is visible and its current value/selected option is "All"; open it and assert the four options are present.

**AC-029**: Given the list view is active (not Kanban), when the Dashboard renders, then the priority filter control is NOT visible (the filter is Kanban-only for MVP).
  Test level: e2e
  Verification: Playwright — navigate to `/` (default list view), assert `priority-filter` is not in the DOM.

---

## Error / Loading / Edge States

**AC-030**: Given the API returns an error (e.g., 500), when the Dashboard renders, then the existing error state (`data-testid="features-error"`) is shown and `kanban-board`, `feature-list`, the view toggle, and the priority filter are NOT rendered.
  Test level: e2e
  Verification: Playwright — mock `/api/features` returning 500, navigate to `/`, assert `features-error` visible, assert `kanban-board`, `feature-list`, `view-toggle-*`, `priority-filter` not in DOM.

**AC-031**: Given the features query is loading, when the Dashboard renders, then the existing loading spinner (`data-testid="features-loading"`) is shown and the Kanban board is not rendered.
  Test level: e2e
  Verification: Playwright — delay `/api/features` response, navigate to `/`, assert `features-loading` visible and `kanban-board` not visible.

**AC-032**: Given the view toggle is rendered and the user is on the Kanban view, when the SSE pushes a `phase_change` event that invalidates the `['features']` query (existing mechanism), then the board re-renders with updated card placements without a full page reload.
  Test level: integration
  Verification: Manual or Playwright — trigger a phase change via the API for a seeded feature while the board is open, assert the card moves to the new phase column after the query refetch completes (assert `kanban-card-${id}` is now in the new column).

**AC-033**: Given dark mode is active (existing `ThemeToggle`), when the Kanban board renders, then every new element (columns, cards, headers, badges, toggle, filter, empty-state message) is legible in dark mode — no unstyled light-only elements, no invisible text (e.g., dark text on dark background).
  Test level: e2e
  Verification: Playwright — toggle dark mode via existing `ThemeToggle`, navigate to `/?view=kanban`, screenshot the board, assert no element has a contrast failure (manual visual check or `data-testid` elements visible with non-zero bounding boxes and readable text color). At minimum: assert all `kanban-column-header-*` and `kanban-card-*` text is visible (non-empty bounding box within viewport after scroll).

---

## Constraint Verification Map (CON → AC)

| CON | AC(s) |
|-----|-------|
| CON-001 (build/lint/E2E on :18765) | AC-001 through AC-033 (all E2E run on :18765) |
| CON-002 (Playwright port 18765) | All e2e-level ACs |
| CON-003 (file layout) | Verified at construction gate, not an AC |
| CON-004 (data-testid convention) | Every E2E AC relies on named testids |
| CON-005 (existing label maps) | AC-010, AC-012 |
| CON-006 (shared query key, no refetch) | AC-017 |
| CON-007 (`<Link>` navigation) | AC-014 |
| CON-008 (dark mode) | AC-033 |
| CON-009 (empty/error/loading/unknown states) | AC-009, AC-015, AC-016, AC-019, AC-021, AC-030, AC-031 |
| CON-010 (assumptions documented for autonomous mode) | Verified by the Assumptions section of spec.md — every question in questions.json has a matching [ASSUMPTION:] |

---

## Test Level Coverage Summary

| User Story | smoke | integration | e2e | unit |
|------------|-------|-------------|-----|------|
| US-1 (toggle) | — | AC-017 (no-refetch) | AC-001..AC-009 | — |
| US-2 (board/cards) | — | AC-017 | AC-010..AC-016 | — |
| US-3 (counts/empty) | — | — | AC-018..AC-021 | — |
| US-4 (scroll) | — | — | AC-022..AC-024 | — |
| US-5 (priority filter) | — | — | AC-025..AC-029 | — |
| Edge states | — | AC-032 | AC-009, AC-030, AC-031, AC-033 | — |

Note on unit tests: the board's card-placement logic (grouping features by `current_phase`, filtering by priority, hiding cancelled) is pure derivation from the `FeatureSummary[]` input. The Architect may add a unit-tested pure function for this grouping during Planning; it is not mandated at the spec level but is recommended. If implemented, an additional unit-level AC will be derived in the plan.