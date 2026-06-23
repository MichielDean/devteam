# Feature Specification: Kanban View

**Feature Branch**: `kanban-view`

**Created**: 2026-06-22

**Status**: Draft

**Input**: User description: "Add a Kanban board view to the Dev Team UI that shows features as cards organized by phase. Features displayed in columns (Inception, Planning, Construction, Review, Testing, Delivery). Cards show title, priority, status. Click card to navigate to detail page. Toggle between list view and Kanban board view."

**Priority**: P1

**Request Classification**: New feature · Enhancement · Single repo (UI) · Moderate complexity

## Workspace Summary (Brownfield)

**Existing codebase**: Dev Team platform. Go backend (cmd/devteam, internal/api, internal/feature, internal/pipeline) serving a React/TypeScript frontend (ui/). Frontend stack: Vite, React, react-router, TanStack React Query, Tailwind CSS. Build: `npm run build` / `npm run dev`. E2E: Playwright on port 18765 (config at ui/playwright.config.ts).

**Affected area**: Frontend only. `ui/src/pages/Dashboard.tsx` currently renders `FeatureList` (a sortable grid of `FeatureCard`). `FeatureCard` already displays title, id, status badge, phase badge, priority badge, pending-questions badge, gate result, last-updated. `FeatureSummary` type already carries all fields needed (`current_phase`, `status`, `priority`, `pending_questions_count`, `gate_result`, `updated_at`). API `GET /api/features` returns `FeatureListResponse { features: FeatureSummary[], total_count }` — no backend changes required for read-only board.

**Conventions to follow**:
- Tailwind utility classes, dark-mode `dark:` variants (see existing components).
- `data-testid` attributes on every interactive/rendered element for Playwright selectors (convention used across Dashboard, FeatureCard, FeatureList).
- TanStack Query for server state (`useQuery(['features'], listFeatures)`).
- react-router `<Link>` for in-app navigation to `/features/:id`.
- Phase constants in `ui/src/types/index.ts` (`PHASES`, `PHASE_LABELS`).
- No new runtime dependencies; reuse stdlib/already-installed packages.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View features as a Kanban board organized by phase (Priority: P1)

A user visiting the Dashboard can switch from the existing list/grid view to a Kanban board view that arranges feature cards into six columns labelled Inception, Planning, Construction, Review, Testing, Delivery. Each card shows the feature title, priority badge, and status badge. Cards are placed in the column matching the feature's `current_phase`. Clicking a card navigates to that feature's detail page (`/features/:id`), identical to the existing list view behaviour.

**Why this priority**: This is the core deliverable of the feature request. Without it, nothing else matters. A user who can see the board has a viable MVP — they gain the at-a-glance phase overview that is the entire point of the request.

**Independent Test**: Can be fully tested by navigating to `/`, toggling to board view, and verifying six labelled columns render with feature cards in the columns matching their `current_phase`, and clicking a card navigates to `/features/:id`. Delivers the primary value without depending on other stories.

**Acceptance Scenarios**:

1. **Given** at least one feature exists across multiple phases, **When** the user clicks the Kanban view toggle on the Dashboard, **Then** six columns labelled Inception, Planning, Construction, Review, Testing, Delivery are rendered and each feature card appears in the column matching its `current_phase`.
2. **Given** the Kanban board is displayed, **When** the user clicks a feature card, **Then** the browser navigates to `/features/:id` for that feature.
3. **Given** the Kanban board is displayed, **When** the user clicks the list view toggle, **Then** the board is replaced by the existing `FeatureList` grid.

---

### User Story 2 - Toggle persists between visits and respects user preference (Priority: P2)

The Dashboard remembers which view (list or Kanban) the user last selected via `localStorage`, so revisiting `/` restores that view without requiring the toggle to be pressed again. First-time visitors see the list view (the existing default).

**Why this priority**: Quality-of-life improvement. The board is fully usable without it, but repeatedly re-toggling on every page load is friction. Not blocking for MVP.

**Independent Test**: Can be tested by toggling to Kanban, reloading the page, and verifying the board re-renders without pressing the toggle; clearing localStorage and reloading verifies the fallback to list view.

**Acceptance Scenarios**:

1. **Given** the user has selected Kanban view and `localStorage` is available, **When** the user reloads `/`, **Then** the Kanban board is rendered without further interaction.
2. **Given** no view preference is stored in `localStorage`, **When** the user visits `/`, **Then** the list view is rendered (existing default preserved).
3. **Given** the user has selected Kanban view, **When** `localStorage` is unavailable or throws, **Then** the Dashboard falls back to list view without crashing.

---

### User Story 3 - Kanban cards surface pending questions and gate status (Priority: P3)

Cards on the Kanban board additionally show a pending-questions indicator (reusing the existing `QuestionBadge`) and a gate-passed/gate-failed indicator, matching the information density of the existing `FeatureCard` so switching views does not lose signal.

**Why this priority**: Nice-to-have parity with the list view. The board is useful with title/priority/status alone; these indicators improve at-a-glance triage but are not essential to the core ask.

**Independent Test**: Can be tested by creating a feature with pending questions and a feature with a gate result, toggling to Kanban, and verifying the badge/indicator renders on the card.

**Acceptance Scenarios**:

1. **Given** a feature with `pending_questions_count > 0`, **When** the Kanban board renders, **Then** the card displays a pending-questions badge with the count.
2. **Given** a feature whose latest gate result has `passed: false`, **When** the Kanban board renders, **Then** the card displays a visible gate-failed indicator.
3. **Given** a feature whose latest gate result has `passed: true`, **When** the Kanban board renders, **Then** the card displays a visible gate-passed indicator.

---

### Edge Cases

- **No features exist**: Board renders six empty columns with an empty-state message inside each column (or the existing `EmptyState` component rendered above the board). API returns `200 OK` with `features: []`; the board must not render a blank page or a 404-style error. [ASSUMPTION: empty board shows six empty columns plus the existing EmptyState call-to-action, consistent with list view's empty handling.]
- **All features in one phase**: The other five columns render empty (headers visible, body empty). No column is hidden.
- **Feature with an unrecognised `current_phase` value** (e.g. a future phase added by the backend): The card is placed in a trailing "Other" column appended after Delivery, so no feature is silently dropped. [ASSUMPTION: backend phase enum may extend; UI must degrade gracefully rather than crash.]
- **Feature with `status: gate_blocked` or `recirculated`**: Card stays in the column matching `current_phase`. [ASSUMPTION pending question resolution: default to current_phase column, the conservative choice that keeps the board a faithful view of pipeline state.]
- **API error loading features**: The existing error path in `Dashboard.tsx` (`data-testid="features-error"`) is preserved for both views; the board is not rendered when the query errors.
- **Rapid toggle between views**: Switching view must not re-trigger the `useQuery(['features'])` network request — both views consume the same cached query data.
- **Very wide board on small screens**: See US-1 acceptance plus the responsive handling assumption below.
- **`localStorage` disabled / Safari private mode**: `localStorage.setItem` throws; the toggle must catch and degrade to per-session memory only (US-2).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The Dashboard MUST render a view toggle control offering "List" and "Kanban" options. Source: US-001
- **FR-002**: When "Kanban" is selected, the Dashboard MUST render a board with exactly six columns labelled, in order: Inception, Planning, Construction, Review, Testing, Delivery. Source: US-001
- **FR-003**: Each column MUST display the features whose `current_phase` equals that column's phase, in the order returned by the API (no re-sort within column for v1). Source: US-001
- **FR-004**: Each Kanban card MUST display the feature title, a priority badge, and a status badge. Source: US-001
- **FR-005**: Clicking a Kanban card MUST navigate to `/features/:id` using react-router `<Link>` (no full page reload). Source: US-001
- **FR-006**: When "List" is selected, the Dashboard MUST render the existing `FeatureList` component unchanged. Source: US-001
- **FR-007**: The active view selection MUST be persisted to `localStorage` under a stable key (`devteam.dashboard.view`). Source: US-002
- **FR-008**: On Dashboard mount, if `localStorage` contains a valid view value the Dashboard MUST restore that view; otherwise it MUST default to "list". Source: US-002
- **FR-009**: All `localStorage` access MUST be wrapped so that a thrown exception (disabled storage, private mode) results in a graceful fallback to the default view, never a render crash. Source: US-002
- **FR-010**: A Kanban card for a feature with `pending_questions_count > 0` MUST display a pending-questions badge with the count, reusing the existing `QuestionBadge` component. Source: US-003
- **FR-011**: A Kanban card MUST display a gate-status indicator when `gate_result` is present: a distinct visible treatment for `passed: true` vs `passed: false`. Source: US-003
- **FR-012**: The board MUST render all six columns even when a column has zero features. Source: US-001, Edge Cases
- **FR-013**: A feature whose `current_phase` is not one of the six known phases MUST be placed in a trailing "Other" column rendered after Delivery (if any such features exist); if no such features exist, the "Other" column MUST NOT be rendered. Source: Edge Cases
- **FR-014**: Both views MUST consume the same `useQuery(['features'])` result; toggling views MUST NOT issue an additional network request. Source: Edge Cases
- **FR-015**: The board's column headers MUST use `PHASE_LABELS` from `ui/src/types/index.ts` so labels stay consistent with the rest of the UI. Source: US-001

### Key Entities *(reused, no new backend entities)*

- **FeatureSummary** (existing): `id`, `title`, `status`, `priority` (1|2|3), `current_phase` (string — one of `inception|planning|construction|review|testing|delivery` or a future value), `updated_at`, `gate_result: GateResult | null`, `pending_questions_count`. No schema changes.
- **DashboardView** (new UI-only local enum): `'list' | 'kanban'`. Persisted in `localStorage['devteam.dashboard.view']`. Not sent to the backend.

### State Transitions (UI-only)

```
DashboardView: list ⇄ kanban (toggle)
Default on first visit / missing storage: list
```

No feature state machine is altered. The board is a read-only projection of feature state.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can switch from list to Kanban view with a single click and see features grouped into the correct phase columns within 200ms of the click (no network round-trip).
- **SC-002**: With 50 features across the six phases, the Kanban board renders with no console errors and all 50 cards are present in the DOM (verifiable via `[data-testid^="feature-card-"]` selector returning 50 elements).
- **SC-003**: Clicking any Kanban card navigates to the correct `/features/:id` route in under one navigation event (no intermediate page reload).
- **SC-004**: Reloading `/` after selecting Kanban restores the Kanban view without further user action (verifiable by `localStorage['devteam.dashboard.view'] === 'kanban'` and the board rendering on load).
- **SC-005**: The existing list view remains byte-for-byte behaviourally identical (same sort controls, same `FeatureCard` rendering) when selected after the feature ships.
- **SC-006**: The Playwright E2E suite (`ui/e2e`) passes with new kanban specs added; no existing E2E spec regresses.

## Assumptions

- [ASSUMPTION: Read-only board — no drag-and-drop. Conservative default pending question resolution. Cards move between columns only as the pipeline updates `current_phase`.]
- [ASSUMPTION: Blocked / recirculated features stay in the column of their `current_phase`. A separate "Blocked" column is not added unless the user answers question 1 with that option.]
- [ASSUMPTION: List view remains the default for first-time visitors; Kanban is opt-in. Conservative choice — preserves existing UX. Pending question 2 resolution.]
- [ASSUMPTION: Card information density matches existing `FeatureCard` (title + priority + status + pending questions + gate + updated) so view switching loses no signal. Pending question 3 resolution; US-003 makes the extra indicators explicit.]
- [ASSUMPTION: All six phase columns always render, even when empty, for a consistent board layout. Pending question 4 resolution; conservative default is consistency.]
- [ASSUMPTION: Large columns scroll with the page (column body does not get an independent `overflow-y`); simplest viable behaviour, no virtualisation. Pending question 6 resolution.]
- [ASSUMPTION: On narrow viewports the board scrolls horizontally, preserving the six-column layout. Pending question 7 resolution; conservative default keeps the board intact.]
- [ASSUMPTION: No new runtime npm dependencies. React + react-router + Tailwind + existing project primitives are sufficient.]
- [ASSUMPTION: No backend changes. `GET /api/features` already returns everything the board needs.]
- [ASSUMPTION: The existing `EmptyState` component continues to render above both views when `features.length === 0`, preserving current behaviour.]

## Constraint Register

This feature implements no external protocol/RFC; constraints derive from internal conventions and the Dev Team constitution.

| ID | Source | Section/Vector | Type | Constraint | Verification Method |
|----|--------|----------------|------|------------|---------------------|
| CON-001 | AGENTS.md | "Frontend (UI)" | consistency | Build/lint/test commands used by the UI must match AGENTS.md (`npm run build`, `npm run lint`, `npm run test:e2e`) | Manual: commands run from `ui/` succeed |
| CON-002 | AGENTS.md | "Playwright E2E Tests" | consistency | E2E tests run on port 18765, not 8765; new specs must target 18765 | Playwright config unchanged; new specs use base URL |
| CON-003 | AGENTS.md | "Testing" | correctness | New UI code must not start servers on :8765 (production port) | Code review grep for `8765` in new files returns no matches |
| CON-004 | ui/src/types/index.ts | PHASES/PHASE_LABELS | consistency | Column labels and ordering must derive from `PHASES` / `PHASE_LABELS`, not hardcoded literals that can drift | Code review: imports from `types/index.ts`; no literal `'Inception'` etc. in the new component |
| CON-005 | constitution.md | I. Spec-Driven | process | Implementation proceeds only after spec.md + acceptance.md + repos.yaml exist in this spec directory | Gate check verifies the three files exist |
| CON-006 | constitution.md | V. Proof-of-Work | process | E2E test report names specific files, methods, assertions for the kanban view | Testing-phase gate verifies named test cases |
| CON-007 | constitution.md | VIII. Go, Minimal Dependencies | engineering | Frontend must not add a new npm dependency for the board; reuse React/Tailwind/router | `ui/package.json` diff has no additions to `dependencies` |
| CON-008 | Existing Dashboard.tsx | data-testid convention | consistency | Every new interactive/rendered element must carry a `data-testid` attribute for Playwright selectors | Code review: new elements have `data-testid` |
| CON-009 | Existing Dashboard.tsx | error/loading paths | correctness | API loading and error states (`features-loading`, `features-error`) must remain visible in both views; Kanban does not suppress them | E2E: loading state asserts spinner; mocked 500 asserts error text |
| CON-010 | Existing Dashboard.tsx | Empty state | correctness | When `features.length === 0`, the empty state must render (not a blank board) | E2E: with zero features, `EmptyState` CTA is visible |
| CON-011 | FeatureSummary API contract | current_phase enum | correctness | Unknown `current_phase` values must not crash the board; they land in an "Other" column | Unit test: render board with a feature whose `current_phase = 'rolling_out'`; assert "Other" column appears with the card |

## Constitution Compliance

- [x] **I. Spec-Driven, Always** — Compliant. This spec precedes implementation; plan/tasks come from the Architect.
- [x] **II. Six Roles, Fixed Pipeline** — Compliant. PM produces spec only; no planning/construction artifacts created here.
- [x] **III. Central Spec, Distributed Implementation** — Compliant. Single spec in the devteam repo; `repos.yaml` declares scope (primary repo: devteam/ui).
- [x] **IV. Two Intake Paths, One Output Format** — Compliant. Loose idea intake produces spec.md + acceptance.md + repos.yaml.
- [x] **V. Proof-of-Work Gates** — Compliant. Acceptance criteria are specific Given/When/Then with test levels and verification methods.
- [x] **VI. Cross-Repo Coherence** — Compliant. Single repo affected; no cross-repo coordination required.
- [x] **VII. Self-Bootstrap** — N/A (not a platform self-build feature).
- [x] **VIII. Go, Minimal Dependencies** — Compliant. No new npm dependency (CON-007); no Go changes.
- [x] **IX. Pipeline Governance** — Compliant. Phase rules followed; security/resiliency extensions noted but mostly N/A for a read-only UI view (no new endpoints, no auth boundary changes, no external dependencies).
- [x] **X. Learn From Cistern** — Compliant. Structured context, distinct phase gates.

### Security & Resiliency Extension Notes (P1)

- **Security**: The board is a read-only view of already-authenticated data served by the existing API. No new endpoints, no new user input, no new auth boundary. XSS surface: feature titles are already rendered as text by React (auto-escaped); the Kanban card must continue to render titles as text nodes, never via `dangerouslySetInnerHTML`. [ASSUMPTION: no new security acceptance criteria needed beyond CON-003 and the React text-rendering default.]
- **Resiliency**: No new external dependency. The existing `useQuery(['features'])` failure path (loading spinner, error text) is reused for both views. `localStorage` access is wrapped (FR-009). No timeouts/retries needed — TanStack Query already manages the fetch lifecycle.

## Scope Boundaries

**In scope**:
- New `KanbanBoard` component and a `KanbanCard` variant (or reused `FeatureCard`) in `ui/src/components/`.
- View toggle in `Dashboard.tsx`.
- `localStorage` persistence of the selected view.
- Playwright E2E spec(s) for the board.
- Unit tests (Vitest if configured) for column-grouping logic and unknown-phase handling.

**Out of scope**:
- Drag-and-drop of cards between columns.
- Backend API changes.
- New npm dependencies.
- Card editing, inline phase changes, or any mutation of feature state from the board.
- Column-level filtering, search, or per-column sorting (the existing list-view sort controls do not apply to the board).
- Bulk operations on cards.
- Real-time updates beyond what the existing `useQuery` invalidation already provides (no new SSE channel for the board).