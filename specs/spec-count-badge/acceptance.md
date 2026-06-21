# Acceptance Criteria — Spec Count Badge

Traced to user stories in `spec.md`. Every criterion uses Given/When/Then with a test level and verification.

## US-001 — See total feature spec count

AC-001: Given the features list page with 3 existing features, when the page loads successfully, then a count badge is visible in the page header and its text content is exactly "3 features".
  Test level: e2e
  Verification: In a browser, seed 3 features, navigate to the features page, assert an element with `data-testid="feature-count-badge"` is present and has textContent "3 features".

AC-002: Given the features list page with 1 existing feature, when the page loads successfully, then the count badge text content is exactly "1 feature" (singular).
  Test level: e2e
  Verification: Seed 1 feature, load the page, assert `data-testid="feature-count-badge"` textContent equals "1 feature".

AC-003: Given the features list page with 5 existing features, when the page loads successfully, then the count badge text content is exactly "5 features" (plural).
  Test level: e2e
  Verification: Seed 5 features, load the page, assert `data-testid="feature-count-badge"` textContent equals "5 features".

AC-004: Given the features list page has loaded with N features, when the user opens browser DevTools console, then no JavaScript console errors or warnings are emitted related to the count badge render.
  Test level: e2e
  Verification: Load the page with seeded features, capture console messages, assert zero errors and zero warnings referencing the count badge.

AC-005: Given a loaded features page showing "3 features" and the user submits the IntakeForm with a valid unique title, when the create mutation succeeds and the `['features']` query invalidates, then the badge text updates from "3 features" to "4 features" without a full page reload.
  Test level: e2e
  Verification: Seed 3 features, load page, assert badge "3 features", fill IntakeForm with valid input, submit, wait for `data-testid="feature-count-badge"` textContent to become "4 features".

## US-002 — See zero count in empty state

AC-006: Given a features store with 0 features, when the features page loads successfully, then the count badge text content is exactly "0 features".
  Test level: e2e
  Verification: Ensure empty features store, load the page, assert `data-testid="feature-count-badge"` textContent equals "0 features".

AC-007: Given a features store with 0 features, when the features page loads successfully, then the existing empty state (`EmptyState` component / `data-testid` per existing convention) is still rendered and the count badge is also rendered (both visible simultaneously).
  Test level: e2e
  Verification: Empty store, load page, assert both `data-testid="feature-count-badge"` ("0 features") and the existing empty-state element are present in the DOM.

## Error paths

AC-008: Given the `GET /api/features` request fails with HTTP 500 on initial load (no prior successful fetch), when the features page renders, then the count badge text content is "0 features" and the existing error block (`data-testid="features-error"`) is also rendered.
  Test level: e2e
  Verification: Intercept/network-fail `GET /api/features` to return 500, load page, assert `data-testid="feature-count-badge"` textContent equals "0 features" and `data-testid="features-error"` is present.

AC-009: Given the features page previously loaded successfully with 2 features and then a refetch fails with a network error, when the page re-renders, then the count badge still displays "2 features" (last known count) and the error block is rendered.
  Test level: e2e
  Verification: Seed 2 features, load page (badge "2 features"), force subsequent `GET /api/features` to fail, assert badge still reads "2 features" and `data-testid="features-error"` is present.

AC-010: Given the user submits the IntakeForm with a duplicate title, when `POST /api/features` returns 409 `duplicate_title`, then the count badge text does not change and the existing duplicate-title toast is displayed.
  Test level: e2e
  Verification: Seed feature titled "Foo", load page (badge "1 feature"), submit IntakeForm with title "Foo", assert badge still "1 feature" and a toast with duplicate-title text appears.

AC-011: Given the user submits the IntakeForm with an empty title, when `POST /api/features` returns 400, then the count badge text does not change and no console errors occur.
  Test level: e2e
  Verification: Load page, submit IntakeForm with empty title (client-side or by bypassing), assert badge unchanged and console has no errors.

## Empty state (API contract — regression guard)

AC-012: Given the features store is empty, when `GET /api/features` is called, then the response is HTTP 200 with body `{"features": []}` (not 404, not `{"features": null}`).
  Test level: integration
  Verification: With empty store, `GET /api/features`, assert status 200 and `response.body.features` is an empty array `[]` (JSON parse, `Array.isArray`, `length === 0`).

AC-013: Given the existing `TestListFeaturesEmpty` integration test runs, when executed, then it passes unchanged (this feature makes no backend change).
  Test level: integration
  Verification: Run `go test ./internal/api/ -run TestListFeaturesEmpty`, assert pass.

## Unit-level correctness

AC-014: Given a count value N, when the badge label is computed, then the label is "1 feature" for N === 1 and "{N} features" for N !== 1 (including N === 0).
  Test level: unit
  Verification: Unit test the pluralization helper with inputs 0, 1, 2, 5, 100; assert exact strings "0 features", "1 feature", "2 features", "5 features", "100 features".

AC-015: Given `data` is undefined (initial load, no data yet), when the badge count is derived, then it evaluates to 0 (not NaN, not undefined, not thrown error).
  Test level: unit
  Verification: Unit test the count derivation `data?.features?.length ?? 0` with `data = undefined`, `data = {features: []}`, `data = {features: [1,2,3]}`; assert 0, 0, 3 respectively.