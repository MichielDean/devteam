# Changelog — spec kanban-view

All entries reference spec `kanban-view`. Versioning follows the Dev Team
repository's own release cadence; this changelog records what the
`kanban-view` feature adds relative to `main`.

## [unreleased] - 2026-06-22

### Added

- Kanban board view: a phase-grouped, read-only board that shows every feature
  spec as a card organized into six phase columns (Inception, Planning,
  Construction, Review, Testing, Delivery) in canonical pipeline order, plus a
  defensive trailing "Other" column for unknown `current_phase` values
  (spec kanban-view, US-002, FR-005, FR-006, FR-007, CON-005).
- List/Board toggle: a two-button segmented control in the Dashboard header
  switches between the existing `FeatureList` and the new `KanbanBoard`. The
  default view is **Board** (spec kanban-view, US-001, FR-001, FR-003,
  AC-001/005).
- Session persistence: the selected view is stored in `sessionStorage` key
  `devteam.dashboard.view` and resumes across reloads within the same browser
  session (spec kanban-view, US-001, FR-002, AC-004).
- Kanban cards: each card shows the feature title, priority badge
  (`PRIORITY_LABELS`), status badge (`STATUS_LABELS`), pending-questions badge
  when `pending_questions_count > 0`, and gate result indicator
  (`✓ Gate passed` / `✗ Gate failed`) when `gate_result` is present —
  identical chrome to the existing `FeatureCard` (spec kanban-view, US-002,
  FR-008, FR-009, CON-006, AC-007/008/009).
- Card click navigation: clicking a board card navigates to `/features/{id}`
  via `react-router`'s `Link`, same destination as the list view (spec
  kanban-view, US-002, FR-010, AC-010).
- Attention flags: cards with `status='gate_blocked'` render with a red ring;
  cards with `status='waiting_for_human'` render with a yellow ring (spec
  kanban-view, US-002, FR-011, AC-012/013).
- Empty column placeholder: a phase column with no features renders its header
  plus a muted "No features" placeholder (spec kanban-view, US-003, FR-012,
  AC-017). All six phase columns are always present (AC-019).
- Column overflow scroll: each column body scrolls vertically independent of
  other columns; the board's height is bounded to the viewport; on narrow
  viewports the board scrolls horizontally with a 240px minimum column width
  (spec kanban-view, US-004, FR-013, FR-014, FR-015, AC-020/021/022).
- Shared `badgeColors` module: the status → Tailwind class map is extracted to
  `ui/src/components/badgeColors.ts` and imported by both `FeatureCard` and
  `KanbanCard` — single source of truth (spec kanban-view, CON-006).
- `useSessionView` hook: session-scoped view preference with try/catch around
  `sessionStorage` access (private-mode quota safety), defaulting to `board`
  (spec kanban-view, FR-002, FR-003).
- `groupFeaturesByPhase` pure function: partitions features into six phase
  buckets plus `other`, every bucket initialized to `[]` (never `null`),
  partition invariant `sum === input.length` (spec kanban-view, FR-006,
  FR-007, CON-008, CON-009, AC-011).
- E2E tests (`ui/e2e/kanban.spec.ts`) covering AC-001 through AC-022 (spec
  kanban-view, acceptance.md).
- Unit test (`ui/src/components/groupFeaturesByPhase.test.ts`) covering AC-011,
  CON-008, CON-009, SC-002 (spec kanban-view, acceptance.md).
- Go smoke test (`internal/api/kanban_smoke_test.go`) asserting `features:[]`
  not `null`, terminal statuses stay in their phase column, and no new
  kanban-specific endpoint exists (spec kanban-view, testing phase).

### Changed

- `ui/src/pages/Dashboard.tsx` now hosts a `useSessionView` state and renders
  either the existing `FeatureList` (list view) or the new `KanbanBoard`
  (board view) inside the same page shell. The `ViewToggle` renders only when
  `!isLoading && !error && features.length > 0` (spec kanban-view, FR-001,
  FR-004, FR-016, FR-017, AC-006).
- `ui/src/components/FeatureCard.tsx` now imports `statusColors` from the new
  shared `badgeColors.ts` module instead of inlining the map (spec kanban-view,
  CON-006).
- `ui/e2e/app.spec.ts` list-view tests now call a `switchToList(page)` helper
  that clicks `view-toggle-list` before asserting `feature-card-*` testids,
  since Board is now the default (spec kanban-view, CON-004, AC-001/003).
- `ui/package.json` adds `vitest` as a **devDependency** and a `test:unit`
  script. The `dependencies` block is unchanged (spec kanban-view, CON-003).
- `ui/vite.config.ts` gains a `test` block (vitest config, excludes `e2e/**`).

### Fixed

- None. No defects from prior releases are addressed by this feature.

### Not changed (intentional)

- No backend endpoints added, removed, or modified. `GET /api/features` is the
  sole data source for the board, reused unchanged (spec kanban-view, FR-016,
  CON-007, AC-016).
- No new runtime npm dependencies. `vitest` is devOnly (spec kanban-view,
  CON-003, VIII "Go, Minimal Dependencies").
- No new top-level route. The board lives at `/` alongside the list view (spec
  kanban-view, Assumptions).
- No drag-and-drop, no WIP limits, no per-column search, no swimlane-per-status,
  no "+N more" overflow, no cross-session persistence (spec kanban-view,
  Assumptions).