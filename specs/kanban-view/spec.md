# Feature Specification: Kanban View

**Feature Branch**: `feature/kanban-view`

**Created**: 2026-06-22

**Status**: Draft

**Input**: Loose idea — "Add a Kanban board view to the Dev Team UI that shows features as cards organized by phase. Features displayed in columns (Inception, Planning, Construction, Review, Testing, Delivery). Cards show title, priority, status. Click card to navigate to detail page. Toggle between list view and Kanban board view."

**Feature ID**: kanban-view

**Priority**: P1

## Workspace Summary (Brownfield)

This feature is added to an existing codebase.

**Existing codebase**: Dev Team platform — Go backend (`cmd/devteam/`, `internal/`) with React/TypeScript frontend (`ui/`). Build: `go build`, `npm run build` in `ui/`. Tests: Go unit tests, Playwright E2E (`ui/e2e/`) on port **18765** (NOT 8765 — production). Config in `devteam.yaml`, runtime data in `.devteam.db` (SQLite).

**Existing UI structure**:
- `ui/src/App.tsx` — router with two routes: `/` (Dashboard) and `/features/:id` (FeatureDetail)
- `ui/src/pages/Dashboard.tsx` — fetches `listFeatures()`, renders `<FeatureList>` or `<EmptyState>`
- `ui/src/components/FeatureList.tsx` — sort controls (phase/priority/status/updated) + responsive grid of `<FeatureCard>`
- `ui/src/components/FeatureCard.tsx` — `<Link to={/features/${id}}>` card showing title, id, status badge, phase badge, priority badge, gate result, updated date, question badge
- `ui/src/components/EmptyState.tsx` — shown when no features exist

**Existing data/API**:
- `GET /api/features` → `FeatureListResponse { features: FeatureSummary[], total_count: number }`
- `FeatureSummary`: `{ id, title, status, priority, current_phase, updated_at, gate_result, pending_questions_count }`
- No new API endpoints needed for read-only Kanban MVP — the existing `GET /api/features` payload already contains `current_phase`, `priority`, `status`, and `id` (enough to place cards in columns and link to detail).

**Conventions to follow**:
- Tailwind utility classes (existing pattern), dark mode via `dark:` variants
- `data-testid` attributes on all interactive/verifiable elements (existing pattern in `FeatureCard`, `FeatureList`, `Dashboard`)
- `@tanstack/react-query` for data fetching (existing `useQuery({ queryKey: ['features'] })`)
- React Router `<Link>` for navigation
- TypeScript types in `ui/src/types/index.ts`
- Phase ordering and labels already defined: `PHASES = ['inception','planning','construction','review','testing','delivery']`, `PHASE_LABELS`, `STATUS_LABELS`, `PRIORITY_LABELS`

## Request Analysis

- **Clarity**: Vague — high-level idea, no detail on drag-drop, view persistence, empty states, filtering
- **Type**: New feature (UI enhancement to existing Dashboard)
- **Scope**: Single component area (`ui/`) — frontend only, no backend changes for read-only MVP
- **Complexity**: Simple-to-moderate — new view component + toggle, reuses existing data flow

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Toggle Between List and Kanban Views (Priority: P1)

As a Dev Team user, I can toggle between the existing list view and a new Kanban board view on the Dashboard, so that I can choose the layout that best suits how I want to scan my features.

**Why this priority**: The toggle is the entry point to the entire feature. Without it the Kanban board is unreachable. It is the smallest independently-testable slice that delivers value on its own — a user can flip the toggle and confirm the board appears. P1 because it gates access to everything else.

**Independent Test**: Load the Dashboard with at least one feature present, click the "Kanban" toggle button, verify the board layout renders (columns + cards), click "List" toggle, verify the existing list layout returns. No other story required.

**Acceptance Scenarios**:

1. **Given** the Dashboard is loaded with one or more features and the default view is List, **When** the user clicks the "Kanban" view toggle, **Then** the Kanban board renders with six phase columns and feature cards placed in the column matching each feature's `current_phase`, and the Kanban toggle shows an active/selected state.
2. **Given** the Kanban board is currently displayed, **When** the user clicks the "List" view toggle, **Then** the existing `FeatureList` (sortable grid) renders and the List toggle shows an active/selected state.
3. **Given** the user has selected the Kanban view and reloads the page, **When** the Dashboard re-renders, **Then** the previously-selected view is restored (Kanban), because the preference is persisted.
4. **Given** the Dashboard is loading features from the API, **When** the toggle is rendered, **Then** both toggle buttons are disabled until features have loaded, to avoid a flash of empty board/list.

---

### User Story 2 - View Features as Kanban Cards Organized by Phase (Priority: P1)

As a Dev Team user, I can see feature cards organized into phase columns (Inception, Planning, Construction, Review, Testing, Delivery), so that I can see at a glance how my features are distributed across the pipeline.

**Why this priority**: This is the core value proposition of the feature — the phase-organized board. Independently testable: render the board with a known set of features and verify each card lands in the correct column. P1 because without it the toggle is meaningless.

**Independent Test**: Seed features with known `current_phase` values (one per phase), render the Kanban view, assert each column contains exactly the feature(s) whose `current_phase` matches that column, and assert all six column headers are visible in pipeline order.

**Acceptance Scenarios**:

1. **Given** features exist with `current_phase` values spanning all six phases, **When** the Kanban board renders, **Then** each feature appears in the column whose phase matches its `current_phase`, columns appear left-to-right in pipeline order (Inception → Planning → Construction → Review → Testing → Delivery), and each column header displays the phase label from `PHASE_LABELS`.
2. **Given** a feature with `current_phase = 'construction'`, **When** the board renders, **Then** that feature's card appears only in the Construction column and in no other column.
3. **Given** features exist, **When** a Kanban card renders, **Then** the card displays the feature title, a priority badge (using `PRIORITY_LABELS`), and a status badge (using `STATUS_LABELS`) — matching the information shown on the existing `FeatureCard` list view.
4. **Given** a feature has `pending_questions_count > 0`, **When** its Kanban card renders, **Then** the question badge is shown on the card (consistent with the list view `FeatureCard`).
5. **Given** a Kanban card is rendered, **When** the user clicks the card, **Then** the app navigates to `/features/:id` for that feature (same destination as the list view).

---

### User Story 3 - Column Count Badges and Empty Columns (Priority: P2)

As a Dev Team user, each phase column header shows how many features are in that phase, and empty columns display an empty-state message, so that I can quickly assess pipeline load and distinguish "no features in this phase" from a rendering bug.

**Why this priority**: Improves scannability and prevents the "is it broken or empty?" confusion. Independently testable: render the board with a known distribution (some empty columns, some with counts) and assert badges + empty messages. P2 because the board is functional without it, but it materially improves usability.

**Independent Test**: Seed features such that at least two phases have zero features and at least two phases have ≥1 feature. Render the Kanban view. Assert each column header shows a count matching the number of cards in that column, and assert empty columns show an empty-state message (e.g., "No features") rather than rendering blank.

**Acceptance Scenarios**:

1. **Given** the Inception column contains 3 features and the Delivery column contains 0 features, **When** the board renders, **Then** the Inception column header displays "Inception (3)" and the Delivery column header displays "Delivery (0)".
2. **Given** a phase column has zero features, **When** the board renders, **Then** the column body displays a visible empty-state message (not a blank space, not an error) such as "No features in this phase".
3. **Given** no features exist at all (Dashboard empty state), **When** the user opens the Kanban view, **Then** all six columns render with count "(0)" and the empty-state message in each column body (the Dashboard-level `<EmptyState>` with "Create Feature" CTA may also render above the board).

---

### User Story 4 - Horizontal Scroll for Narrow Viewports (Priority: P2)

As a Dev Team user on a narrow viewport (tablet or small window), the Kanban board scrolls horizontally rather than squishing columns into an unusable stack, so that each column remains wide enough to read cards.

**Why this priority**: Six columns do not fit comfortably below ~1024px. Horizontal scroll keeps the board usable on smaller screens without breaking the desktop layout. Independently testable: set a narrow viewport, render the board, assert columns retain a minimum width and the board container scrolls horizontally.

**Independent Test**: Render the Kanban view at a 768px viewport width. Assert the board container has a horizontal scrollbar (or overflow-x auto), each column has a minimum width ≥ 240px, and columns do not collapse to a stacked vertical layout.

**Acceptance Scenarios**:

1. **Given** the viewport is narrower than the combined minimum width of six columns, **When** the Kanban board renders, **Then** the board container scrolls horizontally (`overflow-x-auto`) and each column retains its minimum width (≥ 240px).
2. **Given** the viewport is wide enough to show all six columns (≥ 1280px), **When** the board renders, **Then** all six columns are visible without horizontal scrolling and fill the available width.

---

### User Story 5 - Priority Filter on the Board (Priority: P3)

As a Dev Team user, I can filter the Kanban board by priority (All / P1 / P2 / P3), so that I can focus on critical features when the board is crowded.

**Why this priority**: Convenience feature, not essential to the core board. Independently testable: select a priority filter and assert only matching cards remain visible across all columns. P3 — nice-to-have.

**Independent Test**: Seed features with mixed priorities. Render the Kanban view. Select "P1" in the priority filter. Assert every visible card has priority 1, and cards of other priorities are not rendered in any column. Select "All" and assert all cards reappear.

**Acceptance Scenarios**:

1. **Given** the board displays features of priorities 1, 2, and 3 across multiple columns, **When** the user selects "P1" in the priority filter, **Then** only cards with `priority === 1` are rendered in any column, and column count badges update to reflect only the visible (filtered) cards.
2. **Given** the priority filter is set to "P1", **When** the user selects "All", **Then** all cards reappear in their correct columns and count badges return to their unfiltered values.
3. **Given** the priority filter is set to "P1" and a column has zero P1 features, **When** the board renders, **Then** that column shows count "(0)" and the empty-state message.

---

### Edge Cases

- **No features exist at all**: Dashboard renders its existing `<EmptyState>` with the "Create Feature" CTA; if the user manages to toggle to Kanban (toggle should be hidden or disabled in the empty state per AC-001.4 of US-1), the board shows six empty columns with "(0)" counts. [ASSUMPTION: when zero features exist, the toggle is hidden and only the `<EmptyState>` CTA is shown — the empty state is not view-specific.]
- **A feature's `current_phase` is not one of the six known phases** (data corruption / future phase): the card is placed in an "Unknown" bucket or omitted. [ASSUMPTION: place unknown-phase features in a trailing "Unknown" column so they are not silently lost; this is defensive and should never happen with current data.]
- **Feature with `status === 'cancelled'`**: [ASSUMPTION: cancelled features are hidden from the Kanban board (and from the list view's useful working set) — they clutter the board without value. Cancelled features remain accessible via the detail page URL.]
- **Feature with `status === 'done'` (delivered)**: [ASSUMPTION: shown in the Delivery column so users can see completed work; a future enhancement may collapse/hide them.]
- **API error loading features**: the existing Dashboard error state (`data-testid="features-error"`) is shown for both views — the board does not render a partial/broken state.
- **API returns features but `current_phase` is empty string**: treated as unknown phase → "Unknown" column.
- **Very long feature titles**: card title truncates with ellipsis (existing `FeatureCard` uses `truncate` class) — Kanban card reuses this behavior.
- **Rapid toggle clicking**: toggling view multiple times in quick succession must not cause overlapping renders or stale state — React's state batching handles this; no debounce needed.
- **View preference persistence when localStorage is unavailable** (private mode / disabled): fall back to default (List) silently — do not throw.
- **Horizontal scroll on touch devices**: `overflow-x-auto` provides native touch scrolling; no custom drag-to-scroll for MVP.
- **Board rendered while features are still loading** (toggle clicked during fetch): show the existing loading spinner (`data-testid="features-loading"`) in both views; toggle buttons disabled during load.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The Dashboard MUST display a view toggle control with two options: "List" and "Kanban". *Source: US-1*
- **FR-002**: The default view MUST be "List" (the existing `FeatureList` layout) when no prior preference is stored. *Source: US-1*
- **FR-003**: Selecting "Kanban" MUST render a board with six columns labeled Inception, Planning, Construction, Review, Testing, Delivery, in that left-to-right order, using `PHASES` and `PHASE_LABELS` from `ui/src/types/index.ts`. *Source: US-2*
- **FR-004**: The system MUST place each feature card in the column whose phase matches the feature's `current_phase` field. *Source: US-2*
- **FR-005**: Each Kanban card MUST display the feature title, a priority badge, and a status badge, using the same label maps (`PRIORITY_LABELS`, `STATUS_LABELS`) as the existing list `FeatureCard`. *Source: US-2*
- **FR-006**: Each Kanban card MUST show the question badge when `pending_questions_count > 0`, consistent with the list view. *Source: US-2*
- **FR-007**: Clicking a Kanban card MUST navigate to `/features/:id` (the existing FeatureDetail route). *Source: US-2*
- **FR-008**: The view preference MUST persist across page reloads via a URL query parameter `?view=kanban|list` taking precedence, falling back to `localStorage` key `devteam-dashboard-view`, falling back to the default ("list"). *Source: US-1*
- **FR-009**: Each phase column header MUST display a count badge showing the number of feature cards currently rendered in that column, formatted as "PhaseLabel (N)". *Source: US-3*
- **FR-010**: Each phase column whose rendered card count is zero MUST display an empty-state message in the column body (e.g., "No features in this phase"). *Source: US-3*
- **FR-011**: The Kanban board container MUST scroll horizontally (`overflow-x-auto`) when the combined column widths exceed the viewport, and each column MUST maintain a minimum width of 240px. *Source: US-4*
- **FR-012**: The board MUST display all six phase columns regardless of whether they contain features (columns are never hidden). *Source: US-3, US-4*
- **FR-013**: The board MUST hide features with `status === 'cancelled'` from all columns. *Source: Edge cases*
- **FR-014**: Features whose `current_phase` is not one of the six known phases MUST be placed in a trailing "Unknown" column. *Source: Edge cases*
- **FR-015**: The board MUST provide a priority filter control with options "All", "P1", "P2", "P3". Selecting a priority MUST hide cards not matching that priority across all columns and MUST update column count badges to reflect only visible cards. *Source: US-5*
- **FR-016**: The view toggle and priority filter MUST be disabled while features are loading from the API. *Source: US-1, US-5*
- **FR-017**: When the API returns an error, the Dashboard MUST show the existing error state (`data-testid="features-error"`) and MUST NOT render a partial board. *Source: Edge cases*
- **FR-018**: When no features exist, the Dashboard MUST show the existing `<EmptyState>` CTA and MUST NOT render the board or toggle. *Source: Edge cases*
- **FR-019**: The URL query parameter `?view=kanban` MUST be set in the browser address bar when the user selects Kanban, and removed (or set to `?view=list`) when List is selected, so the view state is shareable/bookmarkable. *Source: US-1*

### Key Entities *(include if feature involves data)*

This feature introduces **no new backend entities**. It is a read-only presentation layer over the existing `FeatureSummary` entity.

- **FeatureSummary** (existing, unchanged): `{ id: string, title: string, status: string, priority: number, current_phase: string, updated_at: string, gate_result: GateResult | null, pending_questions_count: number }`
  - Used to place cards in columns via `current_phase`
  - Used to filter via `priority`
  - Used to hide via `status === 'cancelled'`

- **ViewPreference** (new, client-only, non-persisted entity — UI state):
  - `view: 'list' | 'kanban'` — which Dashboard layout is active
  - `priorityFilter: 'all' | 'p1' | 'p2' | 'p3'` — active priority filter on the board (irrelevant in list view)
  - Persistence: URL query param `view` (precedence) → `localStorage['devteam-dashboard-view']` → default `'list'`

No state transitions for ViewPreference (it is ephemeral UI state, not a domain entity with a lifecycle).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can switch from the list view to the Kanban board view in a single click and see their features reorganized into six phase columns within 1 render cycle (no additional API call — uses already-loaded `listFeatures` data).
- **SC-002**: A user can visually identify which pipeline phase any feature is in without opening the feature detail page (the column placement conveys `current_phase`).
- **SC-003**: The Kanban board renders correctly with up to 100 features in under 2 seconds (measured from features-loaded to board-painted) on a standard laptop — no virtualization needed for MVP volumes, but rendering must not block the main thread for > 200ms.
- **SC-004**: The view preference survives a full page reload — reloading a URL with `?view=kanban` lands the user back on the Kanban board, not the list.
- **SC-005**: Every phase column displays a live count badge that matches the actual number of cards rendered in that column (count badge N === number of card elements in column body), including when the priority filter changes the visible set.
- **SC-006**: The Kanban board is usable on a 768px-wide viewport via horizontal scroll — no column collapses below 240px and no card text is clipped horizontally.
- **SC-007**: All E2E Playwright tests for the Kanban board pass on the test server (port 18765), including toggle, column placement, count badges, empty columns, card navigation, priority filter, and view persistence.

## Assumptions

- [ASSUMPTION: The board is **read-only for MVP** — no drag-and-drop to change a feature's phase. Clicking a card navigates to the detail page, same as the list view. Drag-and-drop phase changes would require backend support (phase transition API, validation) and are out of scope. This is the conservative interpretation per error-recovery guidance and the "no unjustified scope expansion" overconfidence-prevention rule.]
- [ASSUMPTION: View preference persistence uses URL query param `?view=` taking precedence, falling back to `localStorage['devteam-dashboard-view']`, falling back to default 'list'. This gives shareable URLs AND per-browser defaulting without over-engineering a user-profile settings API.]
- [ASSUMPTION: All six phase columns are always shown, even when empty, with a count badge and an empty-state message in the body. Hiding empty columns was rejected because it makes the pipeline shape unstable and breaks the "scan the whole pipeline" use case.]
- [ASSUMPTION: Features with `status === 'cancelled'` are hidden from the board (and from the list view's working set). Cancelled features clutter the board without value. They remain reachable via direct `/features/:id` URLs.]
- [ASSUMPTION: Features with `status === 'done'` are shown in the Delivery column so completed work is visible. A future enhancement may collapse or filter them.]
- [ASSUMPTION: Features with an unrecognized `current_phase` (not one of the six known phases) are placed in a trailing "Unknown" column. This is defensive — current data should never produce this — and prevents silent data loss.]
- [ASSUMPTION: No new backend API endpoints are needed for MVP. The existing `GET /api/features` returns all fields required (`id`, `title`, `status`, `priority`, `current_phase`, `pending_questions_count`). The board is a pure client-side reorganization of existing data.]
- [ASSUMPTION: The board reuses the existing `useQuery({ queryKey: ['features'] })` cache. Toggling views does NOT refetch — it re-renders from cached data. The existing SSE invalidation continues to refresh both views.]
- [ASSUMPTION: When zero features exist, the existing Dashboard `<EmptyState>` CTA is shown and the view toggle is hidden — the empty state is not view-specific.]
- [ASSUMPTION: Minimum column width of 240px is sufficient to read a truncated title + badges. Tunable in implementation if empirically too narrow.]
- [ASSUMPTION: Mobile phones (< 640px) are supported via horizontal scroll only — no special mobile layout for MVP. The Dev Team UI is a desktop-first operator console.]
- [ASSUMPTION: The priority filter applies to the Kanban view only for MVP. The list view retains its existing sort controls and does not gain a priority filter in this feature.]
- [ASSUMPTION: The toggle control is a segmented button group (two buttons: List | Kanban) placed near the existing "Features" heading / sort controls, consistent with the existing Tailwind button styling.]
- [ASSUMPTION: No server-side pagination is needed for MVP — the existing `listFeatures` returns all features in a single response. If feature volume grows beyond ~500, pagination/virtualization becomes a separate feature.]

## Constraint Register

This feature does **not** implement an external protocol, RFC, or standard. It is a UI presentation change over an existing internal API. There are no external test vectors, conformance suites, or error taxonomies to conform to.

The governing constraints are **internal conventions** from the existing codebase:

| ID | Source | Section/Vector | Type | Constraint | Verification Method |
|----|--------|----------------|------|------------|---------------------|
| CON-001 | AGENTS.md | "Frontend (UI)" | consistency | Build with `npm run build` in `ui/`; lint with `npm run lint`; E2E with `npx playwright test` on port 18765 (NOT 8765). Never start a test server on :8765. | Build + lint + E2E commands run clean |
| CON-002 | AGENTS.md | "Testing" / Playwright config | consistency | E2E tests use port **18765**. The Playwright `webServer` config auto-starts a test server from the repo root (where `devteam.yaml` lives). | E2E test config references :18765; tests run against test server, not production |
| CON-003 | AGENTS.md | "Project Structure" | consistency | New UI components go in `ui/src/components/`; new pages in `ui/src/pages/`; shared types in `ui/src/types/index.ts`. | File paths match existing layout |
| CON-004 | existing `FeatureCard.tsx` / `FeatureList.tsx` | patterns | consistency | Interactive/verifiable elements MUST carry `data-testid` attributes (existing pattern: `feature-card-${id}`, `feature-card-title`, `sort-by-phase`, etc.). Kanban elements follow the same convention. | Grep `data-testid` in new components; E2E selectors use them |
| CON-005 | existing `types/index.ts` | `PHASES`, `PHASE_LABELS`, `STATUS_LABELS`, `PRIORITY_LABELS` | consistency | The board MUST use these existing label maps rather than redefining phase/status/priority labels. Phase column order MUST follow the `PHASES` array order. | Import from `types/index.ts`; no duplicate label definitions |
| CON-006 | existing `Dashboard.tsx` | react-query usage | consistency | Data fetching uses `useQuery({ queryKey: ['features'], queryFn: listFeatures })` from `@tanstack/react-query`. New views reuse the same query key — no duplicate fetches. | Single `queryKey: ['features']` shared by list + kanban |
| CON-007 | existing `FeatureCard.tsx` | navigation | consistency | Card click navigates via React Router `<Link to={/features/${id}}>` — no `window.location` or imperative navigation. | Kanban card uses `<Link>` |
| CON-008 | existing UI | dark mode | consistency | All new UI MUST support dark mode via Tailwind `dark:` variants (existing pattern: `dark:bg-gray-800`, `dark:text-white`, etc.). | Render in dark mode; no unstyled light-only elements |
| CON-009 | overconfidence-prevention extension | Pattern 2 (happy path only) | correctness | Empty state, API error state, loading state, and unknown-phase state MUST all be explicitly handled — not just the happy path with a full board. | One AC per state in `acceptance.md` |
| CON-010 | error-recovery extension | Inception §"Ambiguous requirements" | process | Unanswered clarifying questions (autonomous mode, no human) MUST be resolved as documented `[ASSUMPTION:]` markers with the conservative choice, not silently dropped. | Every question in `questions.json` has a matching `[ASSUMPTION:]` in this spec |

## Constitution Compliance

No `constitution.md` exists at the repo root or `.specify/constitution.md`. **Constitution check: N/A — no constitution present.** No principles to verify against.

## Out of Scope

- Drag-and-drop to move cards between phase columns (would mutate `current_phase` — requires backend phase-transition API + validation). Future enhancement.
- Backend changes of any kind (new endpoints, schema changes, phase-transition API). The MVP is pure client-side.
- Server-side pagination or virtualization of the board (not needed at current feature volumes).
- A mobile-first stacked layout (mobile uses horizontal scroll for MVP).
- Filtering by status, gate result, or date on the board (priority filter only for MVP).
- Bulk actions on cards (select multiple, reassign phase, etc.).
- Card reordering within a column (cards render in API response order or by `updated_at` desc — deterministic, not user-orderable for MVP).
- Real-time card movement animation when SSE pushes a phase change (the board re-renders on query invalidation; no animated transitions for MVP).
- Applying the priority filter to the existing list view (list view keeps its current sort controls).