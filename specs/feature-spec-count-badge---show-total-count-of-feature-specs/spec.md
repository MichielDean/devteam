# Feature Specification: Feature Spec Count Badge

**Feature ID**: feature-spec-count-badge---show-total-count-of-feature-specs

**Created**: 2026-06-20

**Status**: Draft

**Priority**: P2

**Input**: Show the total count of feature specs on the features list page. Add a `total_count` field to the `GET /api/features` response. Show a badge on the UI. No new endpoints needed.

---

## Problem Statement

The Dev Team dashboard lists features but does not surface how many exist at a glance. A team member scanning the dashboard has to count rows manually to gauge pipeline volume. Adding a total count badge next to the "Features" heading gives immediate, at-a-glance visibility into pipeline size without reading the full list. The backend already computes the feature list; surfacing the count is a one-field addition to the existing `GET /api/features` response.

---

## Request Analysis

- **Clarity**: Clear — specific endpoint, specific field, specific UI element. Minimal clarification needed.
- **Type**: Enhancement — improving an existing feature (the features list page).
- **Scope**: Single component surface, two layers — backend API DTO + frontend display. Touches `internal/api/dto.go` (response shape) and `ui/src/pages/Dashboard.tsx` (badge rendering) plus the TypeScript types in `ui/src/types/index.ts`.
- **Complexity**: Trivial — additive change, no new endpoints, no new state, no new persistence. Existing `FeaturesToSummaryResponse` already iterates the feature slice; `len(features)` is the count.

---

## Workspace Analysis (Brownfield)

### What exists

- **Backend**: `GET /api/features` handler (`internal/api/server.go:127 listFeatures`) calls `s.pipeline.ListFeatures()` and returns `FeaturesToSummaryResponse(features, s.questionStore)` (`internal/api/dto.go:89`). That helper builds a `map[string]interface{}{"features": summaries}`. It iterates `features` already, so the count is `len(features)` — available without a new query.
- **DTO**: `FeatureSummaryResponse` (`internal/api/dto.go:28`) is the per-feature summary. The list response is an untyped `map[string]interface{}` with a single `features` key.
- **Tests**: `internal/api/server_test.go:47 TestListFeaturesEmpty` asserts the empty response has `features` as an array of length 0. `internal/api/server_test.go:200` does an HTTP GET against the live server. These tests will need extending to assert the new `total_count` field.
- **Frontend**: `ui/src/api/client.ts:51 listFeatures()` returns `FeatureListResponse`. `ui/src/types/index.ts:14 FeatureListResponse` has a single `features: FeatureSummary[]` field. `ui/src/pages/Dashboard.tsx:36` reads `data?.features ?? []` and renders a header `<h2>Features</h2>` followed by a `+ New Feature` button.

### What changes

- **Backend**: `FeaturesToSummaryResponse` adds a `total_count` integer key to the returned map equal to `len(summaries)`. No new handler, no new route, no new query.
- **DTO/contract**: The list response gains a top-level `total_count: int` field. `FeatureSummaryResponse` is unchanged.
- **Frontend types**: `FeatureListResponse` gains `total_count: number`.
- **Frontend UI**: `Dashboard.tsx` renders a badge element next to the "Features" heading showing `total_count`.

### What's new

- One new response field (`total_count`) on an existing endpoint.
- One new UI element (count badge) on an existing page.

### Impact scope (blast radius)

- `internal/api/dto.go` — `FeaturesToSummaryResponse` return value.
- `internal/api/server_test.go` — existing list tests assert the new field.
- `ui/src/types/index.ts` — `FeatureListResponse` interface.
- `ui/src/pages/Dashboard.tsx` — header rendering.
- No persistence changes. No state machine changes. No new endpoints. No CLI changes (CLI uses `ListFeatures` directly, not the HTTP response).

---

## User Scenarios & Testing

### User Story 1 — See the total feature count on the dashboard (Priority: P1)

A team member opens the Dev Team dashboard. Next to the "Features" heading, a badge shows the total number of feature specs in the system. The count matches the number of rows in the feature list. When features are created or cancelled, the badge updates on the next refresh.

**Why this priority**: This is the feature. Without the badge visible, the feature does not exist.

**Independent Test**: With 3 features on disk, load the dashboard and verify the badge shows "3" and the list has 3 rows.

**Acceptance Scenarios**:

1. **Given** the dashboard with 5 features, **When** the page loads, **Then** a badge next to "Features" displays "5" and the list contains 5 rows
2. **Given** the dashboard with 0 features, **When** the page loads, **Then** the badge displays "0" and the empty state is shown (no console errors)
3. **Given** the dashboard, **When** a new feature is created via the intake form, **Then** the badge updates from N to N+1 after the list refetches
4. **Given** the dashboard, **When** a feature is cancelled, **Then** the badge still shows the total count of all features (cancelled features remain in the list — see Assumptions)

---

### User Story 2 — API exposes total_count on the features list endpoint (Priority: P1)

The `GET /api/features` response includes a top-level `total_count` integer field equal to the number of features in the `features` array. This is the contract the frontend badge consumes.

**Why this priority**: The UI badge depends on the API field. Backend ships first.

**Independent Test**: `curl GET /api/features` and assert the JSON response has `total_count` equal to the length of the `features` array.

**Acceptance Scenarios**:

1. **Given** N features on disk, **When** a client sends `GET /api/features`, **Then** the response body contains `"total_count": N` at the top level alongside `"features"`
2. **Given** 0 features on disk, **When** a client sends `GET /api/features`, **Then** the response body contains `"total_count": 0` and `"features": []` (not null)
3. **Given** the features list endpoint, **When** the backend fails to list features, **Then** the response is `500` with `{"error":"internal_error","details":"Failed to list features"}` and no `total_count` field (existing error path unchanged)

---

## Edge Cases

| # | Edge Case | Expected Behavior |
|---|---|---|
| 1 | Zero features | `total_count: 0`, `features: []`, UI shows empty state with badge "0" |
| 2 | Single feature | `total_count: 1`, badge shows "1" |
| 3 | Many features (100+) | `total_count` reflects actual count; no pagination in this feature (see Out of Scope) |
| 4 | Backend list error | 500 response with existing error shape; no `total_count` field emitted on error |
| 5 | `features` array empty but `total_count` non-zero | Must never happen — `total_count` is always `len(features)` by construction |
| 6 | Frontend receives response missing `total_count` (e.g., older backend) | Badge renders "0" or is hidden; UI does not crash, no console errors (defensive default) |
| 7 | Network error during list fetch | Existing error path applies — Dashboard shows "Failed to load features" error, badge not rendered |
| 8 | Concurrent feature creation while dashboard open | Badge updates on next React Query refetch (existing invalidation behavior); no special handling |
| 9 | Cancelled features | Cancelled features remain in the list (existing behavior — `ListFeatures` returns all features regardless of status); `total_count` includes them |

---

## Requirements

### Functional Requirements

**API**

- **FR-001**: The `GET /api/features` response SHALL include a top-level `total_count` integer field equal to the number of entries in the `features` array.
  Source: US-2

- **FR-002**: The `total_count` field SHALL be present on every successful `GET /api/features` response, including the empty state (`total_count: 0`).
  Source: US-2

- **FR-003**: The `total_count` field SHALL NOT appear on error responses (400/404/500); error responses retain the existing `{"error": "...", "details": "..."}` shape.
  Source: US-2

- **FR-004**: The `total_count` value SHALL equal `len(features)` exactly — computed from the same slice used to build the `features` array, never from a separate query.
  Source: US-2

- **FR-005**: The `features` array SHALL serialize as `[]` (empty array), not `null`, when no features exist. (Existing behavior — preserved, not changed.)
  Source: US-2

**UI**

- **FR-006**: The Dashboard page SHALL render a count badge adjacent to the "Features" heading displaying the `total_count` value from the API response.
  Source: US-1

- **FR-007**: The badge SHALL display `0` when `total_count` is 0 and the list shows the empty state.
  Source: US-1

- **FR-008**: The badge SHALL update to reflect the latest `total_count` whenever the features list query refetches (e.g., after creating or cancelling a feature).
  Source: US-1

- **FR-009**: The badge SHALL render a safe default (e.g., not rendered, or "0") when the API response omits `total_count`, so the UI does not crash and no console errors occur.
  Source: US-1, edge case 6

- **FR-010**: The badge SHALL be a non-interactive display element (no click handler, no link). It is informational only.
  Source: US-1

### Key Entities

- **FeatureListResponse** (modified): Existing response object gains a `total_count: integer` field.
- **CountBadge** (new UI element): Inline display element showing an integer next to the "Features" heading.

### State Transitions

No new entities with state. The `total_count` field is derived state — computed per request from `len(features)`. No persistence, no lifecycle.

### Non-Functional Requirements

- **NFR-001**: The `total_count` field adds no measurable latency to `GET /api/features` — it is `len()` of an already-computed slice.
- **NFR-002**: The badge must not cause layout shift on first paint — its width should accommodate at least 3 digits without reflow.
- **NFR-003**: The badge must meet accessibility baseline: has an accessible name (e.g., `aria-label="Total features: N"`) and is readable by screen readers.
- **NFR-004**: The response size increase is negligible (one integer field — ~15 bytes).
- **NFR-005**: The change is backward-compatible at the API level — existing clients that ignore unknown fields are unaffected. Frontend that reads the new field degrades gracefully when it is absent.

### Security

This feature adds a derived integer count to an existing read-only endpoint. No new inputs, no new endpoints, no new auth surface. Threat model is unchanged from the existing `GET /api/features`:

- **Information disclosure**: `total_count` reveals the number of features. This is already inferable from `len(features)` in the existing response — no new information is exposed.
- **No new input validation**: The field is output-only.
- **No auth change**: The endpoint remains unauthenticated (local-only mode, per the existing spec assumption).

[ASSUMPTION: No new security acceptance criteria are required because no new attack surface is introduced. The existing `GET /api/features` security posture is preserved.]

---

## Success Criteria

- **SC-001**: Given 3 features on disk, When the dashboard loads, Then the badge shows "3" and the list has 3 rows.
- **SC-002**: Given 0 features on disk, When `GET /api/features` is called, Then the response is `{"features": [], "total_count": 0}` (200 OK).
- **SC-003**: Given N features on disk, When `GET /api/features` is called, Then `response.total_count === response.features.length`.
- **SC-004**: Given the dashboard, When a feature is created via the intake form, Then the badge increments by 1 after the list refetches.
- **SC-005**: Given the dashboard, When the API returns a response without `total_count` (older backend), Then the UI does not crash and no console errors appear.

---

## Error Scenarios

| User Action | Success | Error Condition | Expected Response |
|---|---|---|---|
| `GET /api/features` | 200 OK `{"features": [...], "total_count": N}` | Backend fails to list features | 500 `{"error":"internal_error","details":"Failed to list features"}` (no `total_count` field) |
| Load dashboard | Badge renders with count | API unreachable | Existing error path: "Failed to load features" message shown, badge not rendered |
| Load dashboard | Badge renders with count | API returns malformed/missing `total_count` | Badge defaults safely, no console error |

---

## Assumptions

- **[ASSUMPTION: `total_count` is `len(features)`]** — the count reflects every feature returned by `Pipeline.ListFeatures()`, including cancelled and done features. No filtering or pagination is introduced by this feature.
- **[ASSUMPTION: No pagination]** — the existing endpoint returns all features in one response. This feature does not add pagination. If pagination is added later, `total_count` should be redefined to mean "total matching count" vs "page count" — out of scope here.
- **[ASSUMPTION: Count is per-request derived, not persisted]** — `total_count` is computed on each request from the in-memory slice. No new storage field, no migration.
- **[ASSUMPTION: Existing tests updated, not replaced]** — `TestListFeaturesEmpty` and the HTTP-level list test will be extended to assert `total_count`. New tests added for the badge rendering and the field's presence on a populated list.
- **[ASSUMPTION: Badge styling follows existing Tailwind conventions]** — the badge uses the same design language as existing UI elements (e.g., the priority badge pattern). No new CSS framework or design tokens.
- **[ASSUMPTION: Single repo]** — this feature touches only the `devteam` repo (backend DTO + frontend). No cross-repo coordination.

---

## Scope Boundaries

### In Scope

- Add `total_count` integer field to `GET /api/features` response.
- Render a count badge next to the "Features" heading on the Dashboard.
- Update `FeatureListResponse` TypeScript type.
- Update existing backend list tests to assert the new field.
- Add frontend test for badge rendering (empty and populated states).

### Out of Scope

- Pagination, filtering, or sorting of the features list (existing behavior preserved).
- Counting features by status (e.g., "3 in progress, 2 done") — only the total.
- A new endpoint — no new routes are added.
- CLI changes — the CLI uses `ListFeatures()` directly, not the HTTP response.
- Persisting the count — it is always derived.
- Badge click behavior or navigation — the badge is display-only.
- Real-time count updates via SSE — the badge updates on the next React Query refetch, which is the existing behavior for list mutations. SSE-driven live count is a separate feature.
- Auth or access control on the count field.

---