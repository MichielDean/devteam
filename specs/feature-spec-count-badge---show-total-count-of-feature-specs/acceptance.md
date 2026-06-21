# Acceptance Criteria: Feature Spec Count Badge

**Spec**: feature-spec-count-badge---show-total-count-of-feature-specs

**Created**: 2026-06-20

---

## US-1: See the total feature count on the dashboard

- **AC-001**: Given the dashboard with 5 features on disk, when the page loads, then a badge element adjacent to the "Features" heading displays the text "5" and the feature list contains 5 rows.
  Test level: e2e
  Verification: Load the dashboard in a browser with 5 features seeded; assert the badge element (located by `data-testid="feature-count-badge"`) has text content "5" and the feature list has 5 child rows.

- **AC-002**: Given the dashboard with 0 features on disk, when the page loads, then the badge displays "0", the empty state renders, and no JavaScript console errors occur.
  Test level: e2e
  Verification: Load the dashboard in a browser with 0 features seeded; assert the badge has text "0", the empty state element is visible, and `page.on('console')` with severity `error` collected no messages.

- **AC-003**: Given the dashboard with N features displayed and the badge showing N, when the user creates a new feature via the intake form, then the badge updates to show N+1 after the features list query refetches.
  Test level: e2e
  Verification: Load dashboard with 2 features, assert badge shows "2"; fill and submit the intake form; wait for the features list query to refetch (React Query invalidation); assert badge text is "3" and the list has 3 rows.

- **AC-004**: Given the dashboard, when the feature list query is loading, then a loading state is shown (existing behavior) and the badge does not render stale or `NaN` content.
  Test level: e2e
  Verification: Load dashboard with slow network (throttle response); during the loading spinner phase, assert the badge is either absent or shows the last known value — never `NaN` or `undefined`.

- **AC-005**: Given the dashboard, when the API returns a response body where `total_count` is missing, then the badge defaults safely (renders "0" or is not rendered) and no console error is logged.
  Test level: e2e
  Verification: Intercept `GET /api/features` and return `{"features": []}` (no `total_count`); assert the page renders without crashing and no console error occurs; assert the badge is either absent or shows "0".

- **AC-006**: Given the dashboard, when the API returns an error (500) for the features list, then the existing "Failed to load features" error message is shown and the badge is not rendered.
  Test level: e2e
  Verification: Intercept `GET /api/features` and return 500; assert the error element (`data-testid="features-error"`) is visible and no badge element is present in the DOM.

- **AC-007**: Given the rendered badge element, when a screen reader or accessibility inspector reads the DOM, then the badge has an accessible name describing its purpose (e.g., `aria-label="Total features: N"`).
  Test level: e2e
  Verification: Load dashboard with 3 features; query the badge element; assert it has `aria-label` matching `/Total features: \d+/`.

---

## US-2: API exposes total_count on the features list endpoint

- **AC-008**: Given 3 features on disk, when a client sends `GET /api/features`, then the response is HTTP 200 with a JSON body containing `"total_count": 3` at the top level alongside `"features"`, and `response.total_count === response.features.length`.
  Test level: integration
  Verification: Seed 3 features; send `GET /api/features` via `httptest` or live server; assert status 200, decode body, assert `resp["total_count"] == 3` and `len(resp["features"]) == 3`.

- **AC-009**: Given 0 features on disk, when a client sends `GET /api/features`, then the response is HTTP 200 with body `{"features": [], "total_count": 0}` (features is an empty array, not null).
  Test level: integration
  Verification: Seed 0 features; send `GET /api/features`; assert status 200, decode body, assert `resp["total_count"] == 0`, assert `resp["features"]` is a non-nil array of length 0 (not null).

- **AC-010**: Given 1 feature on disk, when a client sends `GET /api/features`, then the response is HTTP 200 with `"total_count": 1` and `features` array of length 1.
  Test level: integration
  Verification: Seed 1 feature; send `GET /api/features`; assert status 200, `resp["total_count"] == 1`, `len(resp["features"]) == 1`.

- **AC-011**: Given the features list endpoint, when the backend fails to list features (e.g., spec provider error), then the response is HTTP 500 with body `{"error":"internal_error","details":"Failed to list features"}` and the body does NOT contain a `total_count` field.
  Test level: integration
  Verification: Configure a failing `ListFeatures` (e.g., point spec provider at an unreadable directory); send `GET /api/features`; assert status 500, body has `error` and `details`, and body has no `total_count` key.

- **AC-012**: Given a `GET /api/features` response with N features, when the `total_count` field is compared to the `features` array length, then they are equal for every N in {0, 1, 5, 50}.
  Test level: integration
  Verification: For each N in {0, 1, 5, 50}, seed N features; send `GET /api/features`; assert `resp["total_count"] == len(resp["features"])`.

- **AC-013**: Given the `FeatureListResponse` TypeScript type, when the frontend `listFeatures()` API client function is called, then the returned object has a `total_count: number` field accessible at the top level.
  Test level: unit
  Verification: Inspect `ui/src/types/index.ts` `FeatureListResponse` interface; assert it declares `total_count: number`; inspect `ui/src/api/client.ts` `listFeatures()`; assert its return type is `FeatureListResponse`. (Type-level check; can be enforced via `tsc --noEmit`.)

- **AC-014**: Given the existing `TestListFeaturesEmpty` test, when it runs, then it asserts the response body contains `total_count` equal to 0 in addition to the existing `features` empty-array assertion.
  Test level: unit
  Verification: Run `go test ./internal/api/ -run TestListFeaturesEmpty -v`; assert the test passes and the test source asserts `resp["total_count"] == 0`.

- **AC-015**: Given a populated feature list test, when `GET /api/features` returns N features (N > 0), then the test asserts `total_count == N`.
  Test level: integration
  Verification: Run the populated list integration test (new or existing); assert the test passes and its source asserts `resp["total_count"] == N` for the seeded count.

---

## Summary

| AC | US | Test level | Covers |
|---|---|---|---|
| AC-001 | US-1 | e2e | Happy path, badge renders with count |
| AC-002 | US-1 | e2e | Empty state, badge shows 0, no console errors |
| AC-003 | US-1 | e2e | Count updates after mutation (create) |
| AC-004 | US-1 | e2e | Loading state, no stale/NaN badge |
| AC-005 | US-1 | e2e | Defensive default when `total_count` missing |
| AC-006 | US-1 | e2e | Error path, badge not rendered |
| AC-007 | US-1 | e2e | Accessibility: badge has aria-label |
| AC-008 | US-2 | integration | Field present, equals array length (N=3) |
| AC-009 | US-2 | integration | Empty state: `total_count: 0`, `features: []` not null |
| AC-010 | US-2 | integration | Single feature: `total_count: 1` |
| AC-011 | US-2 | integration | Error path: 500, no `total_count` field |
| AC-012 | US-2 | integration | Consistency across {0,1,5,50} |
| AC-013 | US-2 | unit | TypeScript type declares `total_count: number` |
| AC-014 | US-2 | unit | Existing empty test asserts `total_count: 0` |
| AC-015 | US-2 | integration | Populated list test asserts `total_count == N` |

---