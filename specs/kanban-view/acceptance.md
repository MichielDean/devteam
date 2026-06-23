# Acceptance Criteria — kanban-view

Each criterion follows `AC-NNN: Given / When / Then` with a test level and a specific verification method. Every user story has criteria at every relevant test level (UI changes require smoke, integration, and e2e). Error paths and empty states are covered explicitly.

## US-001 — Kanban board view

**AC-001**: Given the Dashboard is loaded with at least one feature whose `current_phase` is `planning`, when the user clicks the Kanban view toggle (`data-testid="view-toggle-kanban"`), then six columns labelled Inception, Planning, Construction, Review, Testing, Delivery are rendered (`data-testid="kanban-column-inception"`, …`delivery`) and the planning feature's card appears inside the Planning column.
  Test level: e2e
  Verification: Playwright navigates to `/`, clicks `view-toggle-kanban`, asserts each `kanban-column-<phase>` header text matches `PHASE_LABELS`, and asserts `kanban-column-planning` contains an element matching `[data-testid^="feature-card-"]` whose `data-testid` includes the planning feature's id.

**AC-002**: Given the Kanban board is displayed and a feature with id `abc123` has its card rendered, when the user clicks that card (`data-testid="feature-card-abc123"`), then the page navigates to `/features/abc123` via client-side routing (no full document reload).
  Test level: e2e
  Verification: Playwright clicks the card and asserts `page.url()` ends with `/features/abc123`; asserts no `request` event for a full document navigation fired (router-based nav).

**AC-003**: Given the Kanban board is displayed, when the user clicks the List view toggle (`data-testid="view-toggle-list"`), then the board is removed from the DOM and the existing `FeatureList` grid (`data-testid="feature-list"`) is rendered instead.
  Test level: e2e
  Verification: Playwright clicks `view-toggle-list`, asserts `feature-list` is visible, and asserts no `kanban-column-*` elements remain in the DOM.

**AC-004**: Given the Dashboard is loaded with the Kanban view active, when the Dashboard queries `GET /api/features`, then the response is consumed once and toggling between List and Kanban does not issue a second `GET /api/features` request.
  Test level: integration
  Verification: Playwright (or Vitest + MSW) records network requests for `/api/features`; toggles view twice; asserts exactly one network request for `/api/features` occurred after initial load (TanStack Query cache hit).

**AC-005**: Given the Dashboard query for features is still loading, when the user toggles to Kanban view, then the loading indicator (`data-testid="features-loading"`) is rendered inside the board area and no `kanban-column-*` elements are present yet.
  Test level: smoke
  Verification: Render `Dashboard` with a query that never resolves; toggle to Kanban; assert `features-loading` is in the document and no `kanban-column-*` elements exist.

**AC-006**: Given the Dashboard query for features has errored, when the user toggles to Kanban view, then the error message (`data-testid="features-error"`) is rendered and no `kanban-column-*` elements are present.
  Test level: smoke
  Verification: Render `Dashboard` with `listFeatures` mocked to reject; toggle to Kanban; assert `features-error` is visible and no `kanban-column-*` elements exist.

**AC-007**: Given zero features exist (`features: []`, `total_count: 0`), when the user toggles to Kanban view, then six empty columns are rendered AND the existing `EmptyState` call-to-action is visible (not a blank page, not an error).
  Test level: e2e
  Verification: Playwright stubs `/api/features` to return `{features:[],total_count:0}`; toggles to Kanban; asserts six `kanban-column-*` elements exist and the EmptyState CTA button is visible.

## US-002 — View persistence

**AC-008**: Given the user has selected Kanban view and `localStorage` is available, when the user reloads `/`, then the Kanban board is rendered on mount without further user interaction and `localStorage.getItem('devteam.dashboard.view')` equals `'kanban'`.
  Test level: e2e
  Verification: Playwright clicks `view-toggle-kanban`, reloads the page, asserts `kanban-column-inception` is visible on load and `localStorage` value is `'kanban'`.

**AC-009**: Given `localStorage` has no `devteam.dashboard.view` key, when the user visits `/`, then the List view is rendered (existing default) and the `feature-list` element is visible.
  Test level: e2e
  Verification: Playwright clears `localStorage`, loads `/`, asserts `feature-list` is visible and no `kanban-column-*` element exists.

**AC-010**: Given `localStorage.setItem` throws (private mode / disabled storage), when the user toggles to Kanban, then the Kanban board is rendered for the current session and no uncaught exception propagates to the console.
  Test level: unit
  Verification: Vitest mocks `localStorage.setItem` to throw; renders `Dashboard`; toggles view; asserts `kanban-column-*` elements appear and no error was thrown by the component.

**AC-011**: Given `localStorage.getItem` throws, when the Dashboard mounts, then the Dashboard falls back to the List view and renders without crashing.
  Test level: unit
  Verification: Vitest mocks `localStorage.getItem` to throw; renders `Dashboard`; asserts `feature-list` is visible and no error was thrown.

## US-003 — Card information density

**AC-012**: Given a feature with `pending_questions_count = 3`, when the Kanban board renders, then that feature's card displays a pending-questions badge showing `3`.
  Test level: integration
  Verification: Render `KanbanBoard` with a feature having `pending_questions_count: 3`; assert the badge element (`data-testid^="question-badge-"`) has text content `3`.

**AC-013**: Given a feature whose `gate_result.passed === false`, when the Kanban board renders, then that card displays a visible gate-failed indicator (e.g. `data-testid="feature-card-gate"` with failed styling/text).
  Test level: integration
  Verification: Render `KanbanBoard` with `gate_result: { passed: false, checks: [] }`; assert the gate indicator element is present and conveys failure (text or class matching the existing `FeatureCard` failure treatment).

**AC-014**: Given a feature whose `gate_result.passed === true`, when the Kanban board renders, then that card displays a visible gate-passed indicator.
  Test level: integration
  Verification: Render `KanbanBoard` with `gate_result: { passed: true, checks: [] }`; assert the gate indicator element is present and conveys success.

**AC-015**: Given a feature whose `gate_result === null`, when the Kanban board renders, then that card does NOT render a gate indicator element (`data-testid="feature-card-gate"` absent).
  Test level: integration
  Verification: Render `KanbanBoard` with `gate_result: null`; assert no `feature-card-gate` element exists inside that card.

## Edge cases — explicit acceptance

**AC-016**: Given a feature whose `current_phase` is not one of the six known phases (e.g. `'rolling_out'`), when the Kanban board renders, then an "Other" column (`data-testid="kanban-column-other"`) is rendered after Delivery and contains that feature's card.
  Test level: unit
  Verification: Vitest renders `KanbanBoard` with a feature whose `current_phase = 'rolling_out'`; asserts `kanban-column-other` exists and contains the card; asserts the six standard columns also exist.

**AC-017**: Given no features have an unknown `current_phase`, when the Kanban board renders, then the "Other" column is NOT rendered (only the six standard columns appear).
  Test level: unit
  Verification: Vitest renders `KanbanBoard` with features all using known phases; asserts no `kanban-column-other` element exists.

**AC-018**: Given multiple features distributed across all six phases, when the Kanban board renders, then each card is placed in the column matching its `current_phase` and the total number of cards across all columns equals the number of features.
  Test level: integration
  Verification: Render with 6 features (one per phase); assert each column contains exactly one card and total card count is 6.

**AC-019**: Given the Kanban board is displayed with 50 features, when the board renders, then all 50 cards are present in the DOM (no virtualisation truncation) and no console error is emitted.
  Test level: integration
  Verification: Render `KanbanBoard` with 50 features; assert `[data-testid^="feature-card-"]` selector matches 50 elements; assert no console errors captured.

## Traceability summary

| User Story | Acceptance Criteria |
|---|---|
| US-001 (board view, P1) | AC-001, AC-002, AC-003, AC-004, AC-005, AC-006, AC-007 |
| US-002 (persistence, P2) | AC-008, AC-009, AC-010, AC-011 |
| US-003 (card density, P3) | AC-012, AC-013, AC-014, AC-015 |
| Edge cases | AC-016, AC-017, AC-018, AC-019 |

Every constraint from the spec's Constraint Register maps to at least one AC:
- CON-001, CON-002 → verified at delivery gate (build/lint/e2e commands).
- CON-003 → verified by code review (no `8765` in new files).
- CON-004 → verified by code review (imports from `types/index.ts`).
- CON-005, CON-006 → process gates.
- CON-007 → verified by `ui/package.json` diff at review.
- CON-008 → verified by code review; every AC above names a `data-testid`.
- CON-009 → AC-005, AC-006.
- CON-010 → AC-007.
- CON-011 → AC-016, AC-017.