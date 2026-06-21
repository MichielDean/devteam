# Review Report: Feature Spec Count Badge

**Spec**: feature-spec-count-badge---show-total-count-of-feature-specs
**Phase**: review
**Reviewer**: reviewer
**Date**: 2026-06-20
**Commit reviewed**: `b2eb494` (`feat: add total_count to features list and dashboard count badge`)

---

## Summary

- **Acceptance criteria**: 15 total — 11 MET, 2 NOT MET, 2 MET WITH NOTE
- **Findings**: 0 critical, 1 required, 3 noted
- **Production code diff**: 15 lines (dto.go +1, Dashboard.tsx +13, types/index.ts +1) — plan target <30. No over-engineering.
- **Backend verification**: `go build ./...` succeeds; `go test ./internal/api/ -v` → 23 passed, 4 target tests (TestListFeaturesEmpty, TestListFeaturesTotalCountPopulated, TestErrorResponseShape, TestSmokeCreateAndGetFeature) pass.
- **Frontend verification**: TypeScript types and Dashboard code inspected and conform to plan. `npm run build` / `npm run test:e2e` could not be re-executed in this review environment (node_modules install produces empty package dirs — environment limitation, not a code defect). Developer self-verification recorded in construction gate; code inspection confirms contract conformance.
- **Gate status**: BLOCKED — 1 required finding (missing e2e test for AC-003) must be resolved before advancing to testing.

---

## Step 1: Spec Review — Plan vs Spec

| Check | Result | Evidence |
|---|---|---|
| Every user story has corresponding tasks | PASS | US-1 (badge) → T-003, T-004, T-005; US-2 (API field) → T-001, T-002 (`tasks.md:27,78,138`) |
| Every AC has a done condition | PASS | AC-001..AC-015 trace to done conditions in T-001..T-005 |
| Orphan tasks (no user story) | NONE | All tasks trace to US-1 or US-2 |
| Missing implementation (user story with no tasks) | NONE | Both user stories covered |

No spec-implementation drift at the plan level.

---

## Step 2: Acceptance Criteria Review

### AC-001: Given dashboard with 5 features, badge shows "5", list has 5 rows
- **Status**: MET WITH NOTE
- **Evidence**: `ui/e2e/app.spec.ts:129-149` `test('feature count badge renders with total count')` asserts `[data-testid="feature-count-badge"]` visible, text matches `/^\d+$/`, and `body.total_count === parseInt(badgeText)`. `ui/src/pages/Dashboard.tsx:45-51` renders `<span data-testid="feature-count-badge" ...>{totalCount}</span>`.
- **Explanation**: The test verifies the badge==API invariant rather than seeding exactly 5 features and asserting literal "5". This is a stronger contract check (badge always matches API) but does not match the literal AC wording ("5 features… displays '5'"). The rendering path is correct; the literal N=5 scenario is not independently seeded.

### AC-002: Given 0 features, badge "0", empty state, no console errors
- **Status**: MET WITH NOTE
- **Evidence**: `ui/e2e/app.spec.ts:178-203` `test('feature count badge handles missing total_count defensively')` intercepts `GET /api/features`, strips `total_count`, returns `{"features": []}` (effectively 0 features + missing field), asserts `consoleErrors` empty and badge absent or "0". `Dashboard.tsx:84-86` renders `EmptyState` when `features.length === 0`.
- **Explanation**: Covered indirectly via the missing-field test (which also exercises the 0-feature empty state). No dedicated 0-feature test that asserts the empty-state element is visible. The empty-state code path (`Dashboard.tsx:84`) is correct.

### AC-003: Given N features, create via intake form, badge updates to N+1 after refetch
- **Status**: NOT MET
- **Evidence**: No e2e test fills and submits the intake form and asserts the badge increments. `ui/e2e/app.spec.ts:129-203` (4 new tests) contains no mutation-then-refetch badge assertion. The implementation supports it: `Dashboard.tsx:20-26` `createMutation.onSuccess` → `queryClient.invalidateQueries({ queryKey: ['features'] })` → refetch → `data.total_count` updates → badge re-renders.
- **Explanation**: The verification specified in acceptance.md ("Load dashboard with 2 features… fill and submit the intake form; wait for refetch; assert badge text is '3'") is not implemented. The codepath is correct but unverified at the e2e level. This is a required gap — the AC explicitly names this scenario at test level e2e.

### AC-004: During loading, badge does not render stale/NaN
- **Status**: NOT MET
- **Evidence**: No e2e test throttles the network to inspect the loading state. `Dashboard.tsx:44` gates the badge on `{!isLoading && !error && (...)}` so the badge is absent during loading (NaN impossible by construction — `totalCount` is only computed from resolved `data`). `Dashboard.tsx:71-76` renders the loading spinner.
- **Explanation**: Implementation is correct (badge gated on `!isLoading`), but the verification specified in acceptance.md ("throttle response; during loading spinner phase, assert badge absent or last known value — never NaN") is not implemented. Required gap for e2e coverage.

### AC-005: Missing total_count → safe default, no console error
- **Status**: MET
- **Evidence**: `ui/e2e/app.spec.ts:178-203` intercepts `**/api/features`, deletes `body.total_count`, fulfills 200; asserts `consoleErrors` empty, `features-error` count 0, badge absent or "0". `Dashboard.tsx:37` `const totalCount = data?.total_count ?? 0` provides the defensive default.
- **Explanation**: Defensive default verified end-to-end. FR-009 satisfied.

### AC-006: API 500 → "Failed to load features" shown, badge not rendered
- **Status**: MET
- **Evidence**: `ui/e2e/app.spec.ts:205-214` `test('feature count badge absent on API error')` intercepts 500, asserts `[data-testid="features-error"]` visible and `[data-testid="feature-count-badge"]` count 0. `Dashboard.tsx:44` gate (`!error`) and `78-82` error block.
- **Explanation**: Error path verified. Badge absent on error confirmed.

### AC-007: Badge has aria-label matching /Total features: \d+/
- **Status**: MET
- **Evidence**: `ui/e2e/app.spec.ts:166-174` `test('feature count badge has accessible aria-label')` asserts `ariaLabel.match(/Total features: \d+/)`. `Dashboard.tsx:47` `aria-label={\`Total features: ${totalCount}\`}`.
- **Explanation**: Accessibility verified. NFR-003 satisfied.

### AC-008: 3 features → total_count==3, ==features.length
- **Status**: MET
- **Evidence**: `internal/api/server_test.go` `TestListFeaturesTotalCountPopulated` creates 3 features via POST, GETs `/api/features`, asserts `resp["total_count"] == float64(3)` and `resp["total_count"] == float64(len(features))`. Test passes (`go test ./internal/api/ -run TestListFeaturesTotalCountPopulated -v`).
- **Explanation**: N=3 case with invariant assertion verified.

### AC-009: 0 features → total_count==0, features [] not null
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:67-69` extends `TestListFeaturesEmpty`: `if resp["total_count"] != float64(0) { t.Errorf(...) }`. Existing assertions confirm `features` is an array of length 0. `internal/api/dto.go:90` `summaries := make([]FeatureSummaryResponse, 0, len(features))` — empty slice serializes as `[]` not `null`. Test passes.
- **Explanation**: Empty state verified. FR-005 (features [] not null) preserved.

### AC-010: 1 feature → total_count==1, features length 1
- **Status**: MET
- **Evidence**: `internal/api/server_test.go` `TestSmokeCreateAndGetFeature` extension (lines ~445-472): after POST creates 1 feature, GETs `/api/features`, asserts `body["total_count"] == float64(1)` and `len(features) == 1`. Test passes.
- **Explanation**: N=1 case verified.

### AC-011: 500 error → no total_count field
- **Status**: MET
- **Evidence**: `internal/api/server_test.go` `TestErrorResponseShape` marshals `ErrorResponse{Error: "internal_error", Details: "Failed to list features"}`, asserts JSON does not contain `"total_count"` and decoded map has no `total_count` key. `internal/api/dto.go:11-14` `ErrorResponse` struct has only `error` and `details` fields. `internal/api/server.go:131` `writeError` uses `ErrorResponse`. Test passes.
- **Explanation**: FR-003 verified by construction (ErrorResponse has no total_count field) and by unit test. T-002 explicitly permitted this approach when inducing a live 500 was impractical.

### AC-012: total_count == features.length for N in {0,1,5,50}
- **Status**: MET WITH NOTE
- **Evidence**: Invariant asserted at N=0 (`TestListFeaturesEmpty`), N=1 (`TestSmokeCreateAndGetFeature`), N=3 (`TestListFeaturesTotalCountPopulated` — explicitly asserts `total_count == len(features)`). `internal/api/dto.go:105` `return map[string]interface{}{"features": summaries, "total_count": len(summaries)}` — both values derived from the same `summaries` slice, so equality is structurally guaranteed.
- **Explanation**: N=5 and N=50 are not literally tested. The invariant `total_count == len(summaries)` is enforced by construction (single slice, single `len()` call) and verified at three N values. Adding N=5 and N=50 would not exercise a new code path — there is no branch in `FeaturesToSummaryResponse` that depends on N. Note only; no action required.

### AC-013: FeatureListResponse type declares total_count: number; listFeatures returns it
- **Status**: MET
- **Evidence**: `ui/src/types/index.ts:14-17` `export interface FeatureListResponse { features: FeatureSummary[]; total_count: number; }` — required, not optional. `ui/src/api/client.ts:51` `export async function listFeatures(): Promise<FeatureListResponse>`.
- **Explanation**: Type-level contract declared. `total_count` is required (not `?:`), matching FR-002 (always present on 200).

### AC-014: TestListFeaturesEmpty asserts total_count == 0
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:67-69` `if resp["total_count"] != float64(0) { t.Errorf("expected total_count 0, got %v", resp["total_count"]) }`. Test passes.
- **Explanation**: Existing test extended with the new assertion. Correct `float64` comparison (JSON numbers decode to `float64` in `map[string]interface{}`).

### AC-015: Populated list test asserts total_count == N
- **Status**: MET
- **Evidence**: `internal/api/server_test.go` `TestListFeaturesTotalCountPopulated` asserts `resp["total_count"] == float64(3)` for N=3. Test passes.
- **Explanation**: Populated case verified with invariant assertion.

---

## Step 3: Key Checks

### Null Pointer Safety — PASS
- `internal/api/server.go:127-137` `listFeatures`: calls `s.pipeline.ListFeatures()`, checks `err` before building response. No nil deref.
- `internal/api/dto.go:95` `if questionStore != nil` guards the `PendingCount` call. `questionStore` may be nil (passed as nil in `TestListFeaturesEmpty` setup).
- `internal/api/dto.go:90` `summaries := make([]FeatureSummaryResponse, 0, len(features))` — initialized empty, never nil.
- `Dashboard.tsx:36-37` uses optional chaining (`data?.features`, `data?.total_count`) — safe when `data` is `undefined` during loading.
- No new struct fields, no new initialization ordering. No nil pointer risk introduced.

### JSON Serialization — PASS
- `features` serializes as `[]` not `null` on empty state: `make([]FeatureSummaryResponse, 0, ...)` produces a non-nil empty slice (`dto.go:90`). `TestListFeaturesEmpty` confirms length 0 array. FR-005 preserved.
- `total_count` is a map key (`map[string]interface{}{"total_count": len(summaries)}`) — always serializes, even when 0. No `omitempty` possible on map keys. FR-002 satisfied by construction.
- `grep -n "omitempty" internal/api/dto.go` shows `omitempty` only on `ErrorResponse.Details`, `CreateFeatureRequest.FileContent`, `GateResult`, `StartedAt`, `CompletedAt`, `CheckResultResponse.Message` — none on `total_count` or `features`. PASS.

### Error Path Coverage — PASS
- 500 path (`server.go:128-132`): `ListFeatures` error → `writeError(w, 500, "internal_error", "Failed to list features")` → `ErrorResponse` (no `total_count`). Verified by `TestErrorResponseShape`.
- This endpoint takes no input and reads local state — no 400/404/409 paths apply (confirmed in plan.md API Contracts: "No other status codes are possible from this endpoint"). Existing 400/404 handlers on other endpoints unchanged.
- Empty state returns 200 with `{"features": [], "total_count": 0}` — verified by `TestListFeaturesEmpty`.

### Middleware Chain — PASS
- `internal/api/server.go:64` `handler := s.recoveryMiddleware(s.corsMiddleware(mux))` — recovery is outermost (catches panics in CORS and all handlers). Unchanged by this feature.
- `server.go:83-93` `corsMiddleware` sets `Access-Control-Allow-Origin: *` — pre-existing, not this feature's scope (spec assumes local-only mode). No change required.
- No new middleware added. No body size limit change (existing `MaxBytesReader` on POST, line 140; GET has no body).

### Over-Engineering Check — PASS
- Production code: 15 lines (dto.go +1, Dashboard.tsx +13, types/index.ts +1). Plan target <30 lines. Well under.
- No pagination, no filtering, no sorting. No new endpoint. No new pipeline method (`git diff internal/pipeline/` clean for this commit). No separate badge component file. No custom hook. No React context. No SSE. No memoized selector.
- Badge is inline `<span>` in `Dashboard.tsx` per plan instruction. Tailwind classes consistent with existing `QuestionBadge.tsx` pattern (`rounded-full text-xs font-bold`).

### Missing Implementation Check — PASS
- All 10 FRs covered:
  - FR-001 (total_count top-level int) → `dto.go:105`
  - FR-002 (present on every 200 incl empty) → map key always serializes
  - FR-003 (absent on error) → `ErrorResponse` struct, `TestErrorResponseShape`
  - FR-004 (== len(features)) → `len(summaries)` by construction
  - FR-005 (features [] not null) → `make([]..., 0, ...)` preserved
  - FR-006 (badge adjacent to heading) → `Dashboard.tsx:42-53`
  - FR-007 (badge shows 0 on empty) → `totalCount ?? 0`, always rendered when `!isLoading && !error`
  - FR-008 (updates on refetch) → React Query invalidation in `createMutation.onSuccess`
  - FR-009 (safe default when missing) → `?? 0`, verified by e2e
  - FR-010 (non-interactive) → `<span>` with no `onClick`/`Link`/`role`
- All NFRs covered: NFR-001 (len() O(1)), NFR-002 (`min-w-[2.5rem]`), NFR-003 (`aria-label`), NFR-004 (~15 bytes), NFR-005 (additive field + `?? 0` default).

### Security Review (P2 — recommended) — PASS
- No new attack surface. `total_count` is output-only, derived from `len(summaries)` (data already in the response). No new information disclosed — count is inferable from `features.length` today.
- No new inputs → no input validation needed.
- No auth change — endpoint remains unauthenticated (local-only mode per spec assumption).
- No secrets in logs — `server.go:130` logs the error, not feature data.
- CORS `*` is pre-existing, not this feature's scope.
- No SQL injection, XSS, CSRF, IDOR, mass assignment risk introduced (no new input binding).

### Constitution Compliance — PASS
- I. Spec-Driven: implementation traces to spec.md + acceptance.md. PASS.
- VIII. Go, Minimal Dependencies: no new Go deps (stdlib only). PASS.
- All other principles N/A for this feature (per plan.md Constitution Check).

---

## Findings

### F-001: Missing e2e test for badge update after feature creation
- **Severity**: Required (blocks gate)
- **Criterion**: AC-003
- **Code**: `ui/e2e/app.spec.ts` (absent test)
- **Description**: acceptance.md AC-003 specifies an e2e test: "Load dashboard with 2 features, assert badge shows '2'; fill and submit the intake form; wait for the features list query to refetch; assert badge text is '3' and the list has 3 rows." No such test exists in `ui/e2e/app.spec.ts`. The implementation supports the behavior (`createMutation.onSuccess` invalidates the query → refetch → badge updates), but it is unverified at the e2e level. Add a test that seeds N features, submits the intake form, waits for the list query refetch, and asserts the badge increments to N+1.

### F-002: Missing e2e test for loading-state badge behavior
- **Severity**: Noted
- **Criterion**: AC-004
- **Code**: `ui/e2e/app.spec.ts` (absent test)
- **Description**: acceptance.md AC-004 specifies throttling the network and asserting the badge is absent or shows last known value (never NaN) during loading. No such test exists. The implementation is correct (`Dashboard.tsx:44` gates badge on `!isLoading`), so this is a test-coverage gap, not a code defect. Recommend adding a test that delays the `/api/features` response and asserts `[data-testid="feature-count-badge"]` has count 0 during the loading spinner phase.

### F-003: AC-012 does not literally test N=5 and N=50
- **Severity**: Noted
- **Criterion**: AC-012
- **Code**: `internal/api/server_test.go` `TestListFeaturesTotalCountPopulated` (N=3 only)
- **Description**: The invariant `total_count == len(features)` is structurally guaranteed (`dto.go:105` uses a single `len(summaries)` call) and verified at N=0, 1, 3. N=5 and N=50 would not exercise a new code path (no N-dependent branch exists). No action required unless strict literal compliance with the AC's {0,1,5,50} set is demanded.

### F-004: AC-001/AC-002 e2e tests verify invariant rather than literal seeded counts
- **Severity**: Noted
- **Criterion**: AC-001, AC-002
- **Code**: `ui/e2e/app.spec.ts:129-149` (render test), `178-203` (missing-field test)
- **Description**: The "renders with total count" test asserts `badgeText == body.total_count` against whatever features happen to be on disk, rather than seeding exactly 5 and asserting "5". The missing-field test exercises the 0-feature state indirectly (via stripped response) rather than a dedicated 0-feature seed. Both tests verify the correct contract (badge matches API), which is stronger than a literal count assertion, but neither matches the literal AC wording. No code change needed; test strengthening is optional.

---

## Quality Gate

| # | Criterion | Status |
|---|---|---|
| 1 | Every AC checked with quoted evidence | PASS (15/15) |
| 2 | "No issues" backed by evidence | PASS — null safety, JSON, error paths, middleware all verified with file:line |
| 3 | Security review complete | PASS (P2 — no new attack surface) |
| 4 | Constitution compliance verified | PASS |
| 5 | Null pointer safety verified | PASS |
| 6 | Error paths verified | PASS (500 path; no 400/404/409 apply to this endpoint) |
| 7 | Middleware chain verified | PASS (recovery outermost, unchanged) |
| 8 | Over-engineering check completed | PASS (15 lines, target <30) |
| 9 | Missing implementation check completed | PASS (all FRs/NFRs covered) |

**Gate decision**: BLOCKED — 1 required finding (F-001: AC-003 e2e test missing) must be resolved before advancing to the Testing phase. F-002 is noted and recommended but does not block (implementation is correct; test gap only).

---

## Recirculate / Fix Guidance

F-001 is a missing test, not a code defect. The Developer should add one e2e test to `ui/e2e/app.spec.ts` covering AC-003 (create feature → badge increments after refetch). This is a construction-phase fix (add test), not a recirculate to planning. Once F-001 is resolved, this review passes.