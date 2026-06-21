---

description: "Task list for Feature Spec Count Badge implementation"
---

# Tasks: Feature Spec Count Badge

**Input**: Design documents from `specs/feature-spec-count-badge---show-total-count-of-feature-specs/plan.md`

**Prerequisites**: plan.md (required), spec.md (required for user stories), acceptance.md (required for verification criteria)

**Organization**: Tasks grouped by user story priority. Backend (US-2) ships first because the frontend (US-1) consumes the API field.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US-1, US-2)
- Include exact file paths in descriptions

## Path Conventions

- **Single repo**: `internal/` (Go backend) and `ui/src/` (React frontend) at repository root
- Paths assume the project structure from plan.md

---

## Phase 1: Backend — API exposes total_count (Priority: P1, US-2)

**Goal**: `GET /api/features` returns a top-level `total_count` integer equal to `len(features)`.

**Independent Test**: `curl /api/features | jq '.total_count == (.features | length)'` returns `true` for any feature count.

**Why first**: The frontend badge (US-1) consumes this field. Backend contract must exist before frontend can render against it.

### Implementation for User Story 2

- [ ] T-001 [US-2] Add `total_count` to the features list response in `internal/api/dto.go` — MODIFY the `FeaturesToSummaryResponse` function (currently at line 89). The function already builds `summaries := make([]FeatureSummaryResponse, 0, len(features))` and returns `map[string]interface{}{"features": summaries}`. Change the return to add a second key: `map[string]interface{}{"features": summaries, "total_count": len(summaries)}`. Use `len(summaries)` (the slice actually serialized), NOT `len(features)` (the input). Do NOT add `omitempty` anywhere. Do NOT change the function signature. Do NOT add a new pipeline method. Do NOT touch `FeatureSummaryResponse`. This is a one-line change to the return statement.
  Files:
    - `internal/api/dto.go` — modify (line ~105, the `return` statement inside `FeaturesToSummaryResponse`)
  Dependencies: none
  Done conditions:
    - `go build ./...` succeeds with no errors
    - `FeaturesToSummaryResponse(nil, nil)` returns a map where `map["total_count"] == 0` and `map["features"]` is a `[]FeatureSummaryResponse` of length 0 (verifiable by a unit test calling the function directly)
    - `FeaturesToSummaryResponse` called with a 3-element feature slice returns `map["total_count"] == 3`
    - The returned map has exactly two keys: `"features"` and `"total_count"` — no more, no less
    - `grep -n "omitempty" internal/api/dto.go` does NOT show `total_count` tagged with `omitempty`
    - No new methods are added to `*pipeline.Pipeline` (verify with `git diff internal/pipeline/`)
  Test level: unit (direct function call) + smoke (service starts and endpoint responds)
  Agent failure mode checks:
    - [ ] JSON arrays are [] not null — verify `features` still serializes as `[]` when empty (existing behavior; the change must not regress this). Run `TestListFeaturesEmpty` and confirm `resp["features"]` is a non-nil `[]interface{}` of length 0.
    - [ ] No phantom methods — no new `CountFeatures()` or `TotalCount()` method invented on `*Pipeline`. Count comes from `len(summaries)` inside the DTO builder only.
    - [ ] No `omitempty` on `total_count` — it must serialize as `0` on the empty state, not be omitted.

- [ ] T-002 [US-2] Extend backend tests to assert `total_count` in `internal/api/server_test.go` — MODIFY two existing tests and ADD one new test in the same file.
    1. MODIFY `TestListFeaturesEmpty` (currently at line 47): after the existing `len(features) != 0` assertion, add `if resp["total_count"] != float64(0) { t.Errorf("expected total_count 0, got %v", resp["total_count"]) }`. Note JSON numbers decode to `float64` in a `map[string]interface{}`.
    2. MODIFY `TestSmokeCreateAndGetFeature` (currently at line 273): after the existing POST creates a feature and before the end of the test, perform a `GET /api/features` against `ts.URL` and assert `body["total_count"] == float64(1)` and `len(body["features"]) == 1`.
    3. ADD `TestListFeaturesTotalCountPopulated`: create 3 features via POST, then `GET /api/features`, assert status 200, decode body, assert `resp["total_count"] == float64(3)` and `len(resp["features"]) == 3`. Also assert `resp["total_count"] == float64(len(resp["features"]))` (the invariant, not just the literal).
    4. ADD `TestListFeaturesErrorHasNoTotalCount`: this verifies FR-003. Configure a failing `ListFeatures` — the simplest approach is to point the spec provider at an unreadable directory OR (if that's hard to set up in-test) assert the contract by inspecting the `writeError` path: confirm that `writeError` produces an `ErrorResponse` with only `error` and `details` keys and no `total_count`. If inducing a real 500 is impractical without a failing provider, document this in a comment and instead assert the error response shape via a direct `writeError` call in a separate unit test (`TestErrorResponseShape`) that encodes an `ErrorResponse` and confirms `"total_count"` is absent from the JSON.
  Files:
    - `internal/api/server_test.go` — modify (extend `TestListFeaturesEmpty`, extend `TestSmokeCreateAndGetFeature`, add `TestListFeaturesTotalCountPopulated`, add `TestListFeaturesErrorHasNoTotalCount` or `TestErrorResponseShape`)
  Dependencies: T-001 must complete first (the field must exist before tests can assert it)
  Done conditions:
    - `go test ./internal/api/ -run TestListFeaturesEmpty -v` passes and the test source asserts `total_count == 0`
    - `go test ./internal/api/ -run TestListFeaturesTotalCountPopulated -v` passes and asserts `total_count == 3` AND `total_count == len(features)`
    - `go test ./internal/api/ -run TestSmokeCreateAndGetFeature -v` passes and includes a `total_count` assertion after a GET
    - `go test ./internal/api/ -v` passes overall (no regressions in existing tests)
    - An error-path test confirms `total_count` does NOT appear in a 500 response body
  Test level: integration (HTTP request/response via httptest) + unit (direct function assertions)
  Agent failure mode checks:
    - [ ] JSON number decoding — agent must compare against `float64(N)` not `int(N)` when decoding into `map[string]interface{}`. A common bug is `resp["total_count"] != 0` which always fails because the decoded value is `float64(0)`.
    - [ ] Error path isolation — the test must confirm `total_count` is absent on 500, not just that the error shape is correct. Use `_, exists := resp["total_count"]; assert !exists`.
    - [ ] No test deletion — do not remove or skip existing assertions; only extend them.

**Checkpoint**: Backend ships `total_count` on every 200 response, absent on 500, equal to `len(features)`. `go test ./internal/api/ -v` is green. ✓

---

## Phase 2: Frontend — Badge renders total_count (Priority: P1, US-1)

**Goal**: The Dashboard renders a display-only badge next to "Features" showing `total_count`, with a safe default and accessibility label.

**Independent Test**: Load the dashboard with N features; the `[data-testid="feature-count-badge"]` element shows `N`.

**Why second**: Depends on T-001 (the API field must exist for the type to declare it and the UI to read it, even though the UI has a defensive default for when it's absent).

### Implementation for User Story 1

- [ ] T-003 [US-1] [P-with-T-004] Add `total_count` to the TypeScript `FeatureListResponse` type in `ui/src/types/index.ts` — MODIFY the `FeatureListResponse` interface (currently at line 14). It currently has one field: `features: FeatureSummary[]`. Add a second field: `total_count: number;`. Make it required (not optional with `?`) — the API always returns it per FR-002. The defensive default for a missing field lives in the UI component (T-004), not in the type (the type describes the current API contract).
  Files:
    - `ui/src/types/index.ts` — modify (line ~14, the `FeatureListResponse` interface)
  Dependencies: T-001 must complete first (the type describes the API contract that T-001 establishes)
  Done conditions:
    - `FeatureListResponse` has exactly two fields: `features: FeatureSummary[]` and `total_count: number`
    - `npm run build` (which runs `tsc -b && vite build`) succeeds with no TypeScript errors
    - `grep -n "total_count" ui/src/types/index.ts` shows the field declared on `FeatureListResponse`
    - No other type in the file is modified
  Test level: unit (type-level, enforced by `tsc --noEmit` / `tsc -b`)
  Agent failure mode checks:
    - [ ] Required vs optional — do not make this `total_count?: number`. The API always returns it. Making it optional would let the UI silently treat a backend regression as normal.
    - [ ] No type proliferation — do not add a new `CountBadgeProps` interface or a `FeatureListResponseV2` type. One field on the existing interface.

- [ ] T-004 [US-1] [P-with-T-003] Render the count badge in `ui/src/pages/Dashboard.tsx` — MODIFY the header `<div>` (currently at line 40). It currently renders `<h2>Features</h2>` and the "+ New Feature" button. Add a display-only `<span>` badge adjacent to the `<h2>` (inside the same flex container, after the heading). The badge:
    - Reads `const totalCount = data?.total_count ?? 0;` (defensive default — FR-009, AC-005). Compute this once near the existing `const features = data?.features ?? [];` line (line 36).
    - Has `data-testid="feature-count-badge"` (for E2E targeting — AC-001, AC-002).
    - Has `aria-label={\`Total features: ${totalCount}\`}` (accessibility — NFR-003, AC-007).
    - Renders the count as text content: `{totalCount}`.
    - Uses Tailwind classes consistent with existing badges (see `ui/src/components/QuestionBadge.tsx` for the pattern: `rounded-full`, `text-xs`, `font-bold`, a bg color). Use a neutral/info color (e.g., `bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200`) since this is informational, not a warning like the question badge.
    - Has `inline-flex items-center justify-center` and a `min-w` that accommodates 3 digits (NFR-002 — e.g., `min-w-[2.5rem]`) to prevent layout shift.
    - Has NO `onClick`, NO `<Link>`, NO `role="button"` (FR-010 — display-only).
    - Is always rendered (even when count is 0 — FR-007). Do NOT conditionally hide it when `totalCount === 0`; the spec says the badge shows "0" on empty state.
    - Is NOT rendered while `isLoading` is true OR while `error` is truthy (AC-004, AC-006). Place the badge inside the header, and the header is always rendered, so gate the badge itself: render the badge only when `!isLoading && !error`. During loading the header shows just the heading + button (same as today). On error the header shows heading + button and the error block renders below (badge absent).
  Files:
    - `ui/src/pages/Dashboard.tsx` — modify (header `<div>` ~line 40, and add the `totalCount` const ~line 36)
  Dependencies: T-001 must complete first (API field). T-003 should complete first or in parallel (type declaration) — if T-003 is not done, `data?.total_count` is a type error.
  Done conditions:
    - `npm run build` succeeds (TypeScript compiles, Vite builds)
    - Dashboard renders a `<span data-testid="feature-count-badge">` adjacent to the "Features" heading
    - With 5 features loaded, the badge text content is "5"
    - With 0 features loaded (empty state), the badge text content is "0" and `EmptyState` renders
    - The badge has `aria-label` matching `/Total features: \d+/`
    - During the loading state, the badge is NOT in the DOM (no stale/NaN value)
    - On API error, the badge is NOT in the DOM and `features-error` is visible
    - When `total_count` is missing from the API response (intercepted to omit it), the badge shows "0" and no console error occurs
    - The badge has no `onClick`, no `<Link>`, no `role="button"`
    - No new file is created (the badge is inline in `Dashboard.tsx`)
  Test level: e2e (Playwright, covers UI component per the selection matrix)
  Agent failure mode checks:
    - [ ] Defensive default — must use `?? 0`, not just `data.total_count`. A missing field must render `0`, never `NaN` or `undefined`.
    - [ ] No click handler — the badge is display-only (FR-010). Adding an `<a>` or `<button>` or `onClick` is a scope violation.
    - [ ] Layout shift — the badge must have a min-width so growing from 1 to 3 digits doesn't reflow the header.
    - [ ] Loading-state staleness — during `isLoading`, `data` is `undefined`, so `data?.total_count` is `undefined` and `?? 0` yields `0`. But the badge must NOT render during loading (AC-004) — gate it on `!isLoading && !error`, otherwise the user sees "0" flash before the real count loads.
    - [ ] Over-engineering — do not extract the badge into a separate component file. It is one `<span>`. Do not add a custom hook. Do not add a React context.

**Checkpoint**: Dashboard renders the badge with the correct count on happy and empty states, hides it on loading/error, degrades safely on missing field, is accessible. `npm run build` and `npm run test:e2e` pass. ✓

---

## Phase 3: E2E Verification (Priority: P1, US-1)

**Goal**: Playwright tests verify the badge contract end-to-end, covering happy, empty, missing-field, error, and accessibility paths.

- [ ] T-005 [US-1] Add E2E tests for the count badge in `ui/e2e/app.spec.ts` — MODIFY the existing test file. Add tests inside the existing `test.describe('Dev Team Web UI', ...)` block:
    1. ADD `test('feature count badge renders with total count')`: load `/`; assert `[data-testid="feature-count-badge"]` is visible; assert its text matches `/^\d+$/`; separately fetch `/api/features` via `request.get` and assert `body.total_count === parseInt(badgeText)` (the badge matches the API).
    2. ADD `test('feature count badge has accessible aria-label')`: load `/`; locate the badge; assert `await expect(badge).toHaveAttribute('aria-label', /Total features: \d+/)` (or use `getAttribute('aria-label')` and a regex test).
    3. ADD `test('feature count badge handles missing total_count defensively')`: use `page.route('**/api/features', async route => { const response = await route.fetch(); const body = await response.json(); delete body.total_count; await route.fulfill({ status: 200, json: body }); });` to intercept and strip `total_count`; load `/`; assert no console errors; assert the badge either is absent OR shows "0" (per AC-005 — either is acceptable, but no crash). Assert `features-error` is NOT visible (it's a 200, just a missing field).
    4. ADD `test('feature count badge absent on API error')`: use `page.route('**/api/features', route => route.fulfill({ status: 500, json: { error: 'internal_error', details: 'Failed to list features' } }));` to intercept and force a 500; load `/`; assert `[data-testid="features-error"]` is visible; assert `[data-testid="feature-count-badge"]` has count 0 (is absent from the DOM).
  Files:
    - `ui/e2e/app.spec.ts` — modify (add 4 tests inside the existing describe block)
  Dependencies: T-004 must complete first (the badge must exist in the DOM before tests can target it)
  Done conditions:
    - `npm run test:e2e` runs the 4 new tests and all pass
    - The happy-path test asserts `badgeText` is numeric AND equals the API's `total_count`
    - The aria-label test asserts the label matches `/Total features: \d+/`
    - The missing-field test asserts NO console errors occur (use the same `page.on('console')` pattern as the existing tests at the top of the file)
    - The error test asserts the badge is absent and `features-error` is visible
    - No existing E2E test is modified or removed
  Test level: e2e
  Agent failure mode checks:
    - [ ] Route interception ordering — `page.route()` must be set up BEFORE `page.goto('/')`. A common bug is intercepting after navigation, which misses the request.
    - [ ] Console error capture — the missing-field test MUST capture console errors using the same `page.on('console', msg => { if (msg.type() === 'error') consoleErrors.push(msg.text()); })` pattern the existing tests use, and assert `consoleErrors` is empty.
    - [ ] No test flakiness — do not use `waitForTimeout`. Use Playwright's auto-waiting locators (`await expect(locator).toBeVisible()`).
    - [ ] Numeric comparison — when comparing badge text to API value, parse the badge text to a number (`parseInt(text)`), do not compare string `"5"` to number `5`.

**Checkpoint**: E2E suite covers happy path, accessibility, defensive default, and error path for the badge. All green. ✓

---

## Dependency Graph

```
T-001 (backend DTO) ──┬──▶ T-002 (backend tests)
                      │
                      ├──▶ T-003 (frontend type) ──┐
                      │                            ├──▶ T-004 (frontend badge) ──▶ T-005 (E2E)
                      └────────────────────────────┘
```

- T-001 is the root. Nothing can start before it.
- T-002 depends on T-001 (tests assert the field T-001 adds).
- T-003 depends on T-001 (type describes the API contract T-001 establishes).
- T-004 depends on T-001 and T-003 (reads the field T-001 returns; uses the type T-003 declares).
- T-005 depends on T-004 (targets the DOM element T-004 renders).

## Parallel Opportunities

- T-002 and T-003 can run in parallel (backend tests vs frontend type — different files, no overlap).
- T-002 and T-004 can run in parallel once T-001 is done (backend tests vs frontend badge — different repos/languages, no overlap).

## Checkpoint Summary

| After | Verify |
|---|---|
| T-001 | `go build ./...` passes; `FeaturesToSummaryResponse` returns `total_count` |
| T-002 | `go test ./internal/api/ -v` green; empty/populated/error cases all assert `total_count` correctly |
| T-003 | `npm run build` passes; `FeatureListResponse` has `total_count: number` |
| T-004 | `npm run build` passes; Dashboard renders badge with correct count on happy/empty/loading/error |
| T-005 | `npm run test:e2e` green; 4 new badge tests pass; no existing test regresses |

## Quality Verification Steps (run after ALL tasks complete)

1. `go build ./...` — backend compiles
2. `go test ./internal/api/ -v` — backend tests green
3. `npm run build` — frontend compiles (tsc + vite)
4. `npm run test:e2e` — E2E green
5. Manual smoke: start the server, `curl /api/features | jq '{total_count, feature_count: (.features | length)}'` — the two values are equal
6. Manual smoke: load the dashboard in a browser, verify the badge shows the count, verify the `aria-label` via DevTools inspector, verify no console errors
7. Diff size check: `git diff --stat` should show ~5 files changed, <60 lines of production code added. If significantly more, flag for review as potential over-engineering.

## Scope Enforcement Reminder

The following are explicitly OUT OF SCOPE (per spec.md). If the Developer implements any of these, the Reviewer must flag it as a scope violation:

- Pagination, filtering, sorting of the features list
- Counting by status (e.g., "3 in progress")
- A new endpoint or route
- CLI changes
- Persisting the count
- Badge click behavior or navigation
- Real-time/SSE count updates
- Auth or access control on the count field
- A separate React component file for the badge
- A custom hook or React context for the count