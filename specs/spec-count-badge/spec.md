# Spec: Feature Spec Count Badge

## Feature ID
spec-count-badge

## Priority
2

## Summary
Display a badge on the features list page (Dashboard) showing the total number of feature specs. The count is derived client-side from the existing `GET /api/features` response — no new API endpoint and no API response schema change required.

## Request Analysis
- **Request type**: New feature (enhancement to existing UI)
- **Clarity**: Clear — single, well-defined UI addition
- **Scope**: Single component (UI Dashboard / FeatureList)
- **Complexity**: Trivial — render a number derived from an already-loaded array

## User Stories

### US-001: See total feature spec count
**As a** user viewing the features page
**When** I load the features list and at least one feature exists
**Then** I see a badge showing the total number of features (e.g., "3 features")
Priority: P1

### US-002: See zero count in empty state
**As a** user viewing the features page
**When** I load the features list and no features exist
**Then** I see the count badge showing "0 features" alongside the existing empty state
Priority: P1

## Functional Requirements

- **FR-001**: The system shall render a count badge on the features list page that displays the total number of features, formatted as "{N} feature(s)".
  Source: US-001

- **FR-002**: The system shall derive the count from `features.length`, where `features` is the array returned by the existing `GET /api/features` response field `features`.
  Source: US-001

- **FR-003**: The system shall display "0 features" when the `features` array is empty.
  Source: US-002

- **FR-004**: The system shall display "1 feature" (singular) when the `features` array contains exactly one element, and "{N} features" (plural) for N != 1.
  Source: US-001
  [ASSUMPTION: Pluralization is in scope — trivially cheap and prevents a "1 features" grammar bug. Conservative specificity over vague criteria.]

- **FR-005**: The count badge shall be visible in all three Dashboard render states: loading complete with features, loading complete with zero features (alongside EmptyState), and during loading (showing "Loading..." or the last known count).
  Source: US-001, US-002
  [ASSUMPTION: Badge is rendered as a static header element above the loading/error/empty/list branches, so it remains visible across states. If the badge were inside the FeatureList branch it would disappear in empty state.]

- **FR-006**: The count badge shall update automatically when features are created or removed (the existing react-query invalidation on `['features']` already triggers refetch and re-render).
  Source: US-001

- **FR-007**: The count badge shall reflect the count even when the features query is in an error state (badge shows last successful count or "0 features" if never loaded).
  Source: US-001
  [ASSUMPTION: On query error, the Dashboard currently renders an error block and no features list. The badge will render using `data?.features?.length ?? 0`. This means: never-loaded error → "0 features"; previously-loaded error → stale count. Conservative: badge visible in error state with stale/zero count, not hidden.]

## Key Entities and Relationships

- **FeatureSummary** (existing): id, title, status, priority, current_phase, updated_at, gate_result, pending_questions_count
- **FeatureListResponse** (existing): `{ features: FeatureSummary[] }` — the source of the count
- **CountBadge** (new, UI-only): a display element with no separate persistence. Derived value: `count = features.length`

No new backend entities. No state transitions (the badge is a pure derived view of existing data).

## State Transitions
None. The count badge has no lifecycle of its own — it re-renders whenever the `features` query data changes.

## Success Criteria

- **SC-001**: Given a features list with 3 features, when the page loads, then the badge text contains "3 features".
- **SC-002**: Given a features list with 0 features, when the page loads, then the badge text contains "0 features".
- **SC-003**: Given a features list with 1 feature, when the page loads, then the badge text contains "1 feature" (singular).
- **SC-004**: Given the page is in the error state (query failed, never loaded successfully), when the page renders, then the badge text contains "0 features" and the existing error block is still displayed.
- **SC-005**: Given a user creates a new feature via the IntakeForm, when the create mutation succeeds and the `['features']` query invalidates, then the badge count increments by 1 within one re-render cycle.

## Error Scenarios

| User Action / State | Success | Error Condition | Expected Response |
|---|---|---|---|
| Load features page | 200 OK; badge shows "{N} features" matching array length | `GET /api/features` returns 500 | Badge shows "0 features" (never loaded) or last known count; existing error block renders below |
| Load features page | 200 OK `[]`; badge shows "0 features" | n/a — empty is not an error | (empty state is success, covered by SC-002) |
| Load features page | 200 OK `[1 feature]`; badge shows "1 feature" | n/a | (covered by SC-003) |
| Create feature then page reloads | New feature appears, badge increments | Create returns 409 `duplicate_title` | Badge does not change; existing toast shows error; no console errors |
| Create feature then page reloads | New feature appears, badge increments | Create returns 400 `validation_error` | Badge does not change; existing toast shows error; no console errors |
| Network failure during refetch | Badge shows last known count | Query transitions to error state | Badge shows last known count (`data?.features?.length ?? 0`); existing error block renders |

[ASSUMPTION: No 404 path exists for `GET /api/features` — the endpoint always returns 200 with an array (possibly empty). 404 is only applicable to specific-feature endpoints (`/api/features/{id}`), which this feature does not touch.]

## Empty State Behavior

- **API**: `GET /api/features` returns `200 OK` with `{"features": []}` when no features exist. No change required.
- **UI**: The Dashboard currently renders `<EmptyState>` when `features.length === 0`. The count badge renders "0 features" and remains visible alongside (or above) the `EmptyState` component.
- **Null safety**: The existing API contract serializes `features` as an array, never `null`. The UI uses `data?.features ?? []` defensively. The badge uses `features.length` after this coalescing, so it is null-safe.

## Assumptions and Scope Boundaries

### In Scope
- Rendering a count badge on the features list page (Dashboard)
- Deriving the count from the existing `GET /api/features` response (`features.length`)
- Pluralization ("1 feature" vs "{N} features")
- Badge visible across loading, error, empty, and populated states
- Badge updates automatically on react-query invalidation

### Out of Scope
- No new API endpoint
- No change to the `GET /api/features` response schema (no `total_count` field added — the original skeleton AC-003 mentioned `total_count`, but that would be an API change. [RESOLVED: choose less scope — derive client-side. The client already has the array; a separate `total_count` field would be redundant and would require a backend change, DTO change, type change, and test changes for zero benefit.])
- No pagination, filtering, or sorting changes (FeatureList already has client-side sort; not touched)
- No per-phase counts, per-status counts, or breakdowns (just total)
- No count on the FeatureDetail page
- No SSE / real-time count updates beyond the existing react-query refetch invalidation
- No internationalization beyond simple English pluralization

### Assumptions
- [ASSUMPTION: The `features` field in `GET /api/features` is always an array, never `null` — verified in `dto.go:90` which initializes `summaries := make([]FeatureSummary, 0, len(features))`.]
- [ASSUMPTION: Pluralization rule is English-only: "1 feature", "0 features", "N features" for N != 1. No locale-aware pluralization framework introduced.]
- [ASSUMPTION: The badge is rendered in the Dashboard header (next to the "Features" h2 / "New Feature" button row), not inside FeatureList, so it remains visible in all states. This is a UI placement decision deferred to the Architect; PM specifies only that the badge is visible in all states.]
- [ASSUMPTION: "Visible in all states" means the badge element is in the DOM and renders the count text in loading, error, empty, and populated states. During the initial loading state before first data arrives, `data` is undefined, so `data?.features?.length ?? 0` yields 0 — badge shows "0 features" briefly. Acceptable; alternatively shows "Loading..." — Architect decides. Either is in scope as long as it does not throw and does not disappear the layout.]

## Technical Context (Brownfield Workspace Analysis)

### Existing Structure
- **Backend**: Go 1.x, `net/http` with `mux.HandleFunc`, JSON responses. API in `internal/api/`.
- **Frontend**: React + TypeScript + Vite + TanStack Query (react-query). UI in `ui/src/`.
- **Build**: `go build` for backend; `npm` + Vite for frontend; Playwright for e2e.

### Existing Patterns
- API responses are plain structs serialized via `writeJSON` (sets `Content-Type: application/json`).
- Arrays are always initialized non-nil (`make([]T, 0, len(...))`) — no `null` serialization bug here.
- Frontend fetches via `api/client.ts` `request<T>` wrapper; react-query `useQuery({ queryKey: ['features'], queryFn: listFeatures })`.
- Dashboard (`ui/src/pages/Dashboard.tsx`) already has all the state branches: `isLoading`, `error`, `features.length === 0` (EmptyState), `features.length > 0` (FeatureList).
- Test infra: Go integration tests in `internal/api/server_test.go`, Playwright e2e in `ui/e2e/`.

### Integration Points
- `GET /api/features` — existing, unchanged
- `listFeatures()` in `ui/src/api/client.ts` — existing, unchanged
- `useQuery({ queryKey: ['features'] })` in Dashboard — existing, unchanged; the `data.features` array is the data source

### Existing Tests
- `TestListFeaturesEmpty` (server_test.go:47) — verifies empty array returns 200 with `[]`
- Playwright e2e exists in `ui/e2e/` — can add a count badge assertion

### What Changes
- `ui/src/pages/Dashboard.tsx`: add badge element in the header row, derive count from `features.length`
- Possibly a small helper for pluralization (inline ternary is fine — no new file needed unless Architect prefers)

### What's New
- One new UI element (badge) in Dashboard header
- No new components, no new files, no new API, no new types

### Impact Scope
- **Blast radius**: single file (`Dashboard.tsx`) plus its e2e test. No backend changes. No API contract changes. No type changes. No other UI components affected.