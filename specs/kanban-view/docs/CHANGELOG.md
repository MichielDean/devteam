# Changelog — spec `kanban-view`

All entries reference spec `kanban-view`. Versioning follows the Dev Team
repository's own release cadence; this changelog records what the
`kanban-view` feature adds relative to `main`.

## [unreleased] - 2026-06-22

### Added

- Kanban board view: a phase-grouped, read-only board that shows every feature
  as a card organized into six phase columns (Inception, Planning,
  Construction, Review, Testing, Delivery) in canonical pipeline order, plus a
  defensive trailing "Other" column for unknown `current_phase` values (spec
  `kanban-view`, US-001, FR-002, FR-012, FR-013, AC-001, AC-016).
- List/Kanban view toggle: a two-button control on the Dashboard offering
  "List" and "Kanban" options. The default view is **list** (the existing
  default preserved) (spec `kanban-view`, US-001, FR-001, FR-006, FR-008,
  AC-003, AC-009).
- View persistence: the selected view is stored in `localStorage` under the
  stable key `devteam.dashboard.view` and resumes across reloads. First-time
  visitors see the list view. `localStorage` failures fall back to list
  without crashing (spec `kanban-view`, US-002, FR-007, FR-008, FR-009,
  AC-008, AC-010, AC-011).
- Kanban card information density: each card shows the feature title, a
  priority badge, and a status badge. Cards with `pending_questions_count > 0`
  show a pending-questions badge with the count (reusing the existing
  `QuestionBadge`). Cards whose `gate_result` is present show a visible
  gate-status indicator — gate-passed or gate-failed (spec `kanban-view`,
  US-003, FR-004, FR-010, FR-011, AC-012, AC-013, AC-014, AC-015).
- Card click navigation: clicking a Kanban card navigates to `/features/{id}`
  via `react-router`'s `Link`, same destination as the list view, no full page
  reload (spec `kanban-view`, US-001, FR-005, AC-002).
- Empty board handling: when there are zero features, six empty columns render
  **and** the existing `EmptyState` call-to-action is visible (spec
  `kanban-view`, Edge Cases, FR-012, CON-010, AC-007).
- Playwright E2E specs covering AC-001..AC-019 (spec `kanban-view`, CON-006,
  SC-006).

### Changed

- The Dashboard happy-path branch now conditionally renders `<KanbanBoard>`
  or the existing `<FeatureList>` based on the persisted view. The loading,
  error, and empty branches are unchanged and stay above the view switch
  (spec `kanban-view`, CON-009, AC-005, AC-006).

### Notes

- No backend changes. `GET /api/features` is reused unchanged (spec
  `kanban-view`, FR-014, CON-007).
- No new npm dependencies — runtime or dev (spec `kanban-view`, CON-007).
- No new HTTP endpoints, request bodies, query parameters, or error codes
  (spec `kanban-view`, contracts/GET-api-features.md).
- The existing list view remains behaviourally identical when selected (spec
  `kanban-view`, SC-005).