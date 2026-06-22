# Changelog — Spec kanban-view

All entries reference spec `kanban-view`. Versioning follows the Dev Team
repository's own release cadence; this changelog records what the
`kanban-view` feature adds relative to `main`.

## [unreleased] - 2026-06-22

### Added

- Kanban board view: a phase-grouped, read-only board that shows every feature
  spec as a card organized into 7 columns (Backlog, Inception, Planning,
  Construction, Review, Testing, Delivery) in canonical pipeline order
  (spec kanban-view, US-001, FR-001, CON-001).
- Backlog column: features that have not started the pipeline (status `draft`
  and current phase `inception`) appear in a dedicated Backlog column, separate
  from features actively in the Inception phase (spec kanban-view, US-002,
  FR-002, CON-002).
- View toggle: a List / Board segmented control in the Dashboard header switches
  between the existing list view and the Kanban board. The total feature count
  badge stays mounted and consistent across both views (spec kanban-view,
  US-003, FR-006, FR-007, CON-007, CON-010).
- Clickable cards: clicking a board card navigates to that feature's detail page
  at `/features/{id}`, reusing the existing `FeatureCard` link behavior
  (spec kanban-view, US-004, FR-005, CON-005).
- Per-column header counts and empty-state messages: every column shows its card
  count in its header and renders a non-blank empty-state message when it has
  zero cards. Backlog reads "No features waiting to start"; the six phase columns
  read "No features in this phase" (spec kanban-view, US-005, FR-008, FR-009,
  CON-004).
- Dark mode support: the board, columns, and cards render with the app's existing
  Tailwind `dark:` variants, consistent with the rest of the UI (spec kanban-view,
  FR-010, CON-008).
- Horizontal scroll on narrow viewports: all 7 columns remain reachable without
  overlap or clipping (spec kanban-view, FR-013).
- Live updates: the board shares the existing react-query `['features']` cache
  with the Dashboard list, so mutations that invalidate that cache (create, run,
  advance, recirculate, cancel) move cards between columns without a full page
  reload (spec kanban-view, US-006, FR-014).
- Stable `data-testid` selectors for board and columns
  (`kanban-board`, `kanban-column-backlog`, `kanban-column-inception`,
  `kanban-column-planning`, `kanban-column-construction`,
  `kanban-column-review`, `kanban-column-testing`, `kanban-column-delivery`,
  `view-toggle-list`, `view-toggle-board`) (spec kanban-view, FR-012, CON-011).
- E2E tests (`ui/e2e/kanban.spec.ts`) covering AC-001 through AC-014,
  AC-CON-008, AC-CON-011, and AC-ERR-003 (spec kanban-view, acceptance.md).
- Integration tests (`ui/e2e/kanban-api.spec.ts`) covering AC-CON-003,
  AC-CON-006, AC-ERR-001, and AC-ERR-002 (spec kanban-view, acceptance.md).
- Backend smoke test (`internal/api/kanban_smoke_test.go`) exercising the board's
  data contract end-to-end (spec kanban-view, testing phase).
- Test helper script `run-tests.sh` and isolated Playwright port (`SERVER_PORT`
  in `ui/playwright.config.ts`) so test runs do not collide with the production
  `:8765` service.

### Changed

- `ui/src/pages/Dashboard.tsx` now hosts a `viewMode` state and renders either the
  existing `FeatureList` (list view, default) or the new `KanbanBoard` (board view)
  inside the same page shell, so the `feature-count-badge` stays mounted across
  toggles (spec kanban-view, FR-006, FR-007, CON-010).

### Fixed

- None. No defects from prior releases are addressed by this feature.

### Not changed (intentional)

- No backend endpoints added, removed, or modified. `GET /api/features` is the
  sole data source for the board, reused unchanged (spec kanban-view, FR-004,
  CON-003, AC-CON-003).
- No new runtime or dev dependencies added to `ui/package.json`
  (spec kanban-view, FR-011, CON-006, AC-CON-006).
- No new top-level route added. The board lives at `/` alongside the list view
  (spec kanban-view, plan AD-1).
- No drag-and-drop, no WIP limits, no per-column search, no mobile collapsed
  layout (spec kanban-view, "Out of scope").