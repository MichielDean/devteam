# Review Report: Feature Spec Count Badge

**Spec**: feature-spec-count-badge---show-total-count-of-feature-specs
**Phase**: review
**Reviewer**: reviewer (adversarial)
**Date**: 2026-06-21
**Commits reviewed**: `b2eb494` (feat), `79ee384` (test coverage), `2ed6faf` (test fix), `21aa2f3` (flaky fix), `1023993` (playwright config)
**Branch**: `feat/spec-count-badge` @ `e354868`

---

## Summary

- **Acceptance criteria**: 15 total — 14 MET, 1 NOT MET
- **Findings**: 0 critical, 1 required, 1 noted
- **Production code diff**: 15 lines (dto.go +1/-1, Dashboard.tsx +13/-1, types/index.ts +1) — plan target <30. No over-engineering.
- **Backend verification**: `go build ./...` succeeds. `go test ./internal/api/ -v` → 57 passed (4 target tests verified: `TestListFeaturesEmpty`, `TestListFeaturesTotalCountPopulated`, `TestListFeaturesTotalCountConsistency` subtests N∈{0,1,5,50}, `TestSmokeCreateAndGetFeature`, `TestErrorResponseShape`, `TestListFeaturesErrorResponseHasNoTotalCount`).
- **Frontend verification**: `npm run build` succeeds (tsc -b + vite). E2E: `npx playwright test --grep "feature count badge"` → 4 passed, 0 failed. Full suite: 9 passed, 0 failed, 2 skipped.
- **Live verification**: `curl http://localhost:8765/api/features | jq '{total_count, feature_count: (.features | length)}'` → `{total_count: 5, feature_count: 5}` — invariant holds against the running systemd service.
- **Gate status**: BLOCKED — 1 required finding (F-001: AC-003 e2e test for badge update after intake form submission is absent). Implementation supports the behavior but it is unverified at the e2e level, and the AC explicitly names this scenario at test level e2e.

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
- **Evidence**: `ui/e2e/app.spec.ts:132-154` `test('feature count badge renders with total count')` asserts `[data-testid="feature-count-badge"]` visible, text matches `/^\d+$/`, and `body.total_count === parseInt(badgeText)`. `ui/src/pages/Dashboard.tsx:45-51` renders `<span data-testid="feature-count-badge" ...>{totalCount}</span>`.
- **Explanation**: Test verifies badge==API invariant against whatever features happen to be on disk rather than seeding exactly 5 and asserting literal "5". Stronger contract check (badge always matches API), but does not match the literal AC wording. Rendering path correct. The literal N=5 scenario is not independently seeded — noted, not blocking. The invariant is strictly stronger than the literal assertion.

### AC-002: Given 0 features, badge "0", empty state, no console errors
- **Status**: MET WITH NOTE
- **Evidence**: `ui/e2e/app.spec.ts:166-192` `test('feature count badge handles missing total_count defensively')` intercepts `GET /api/features`, strips `total_count`, fulfills 200 with `{"features": []}` (effectively 0 features), asserts `consoleErrors` empty and `badge` visible with text `'0'` (commit `2ed6faf` hardened this from vacuous to `toHaveText('0')`). `Dashboard.tsx:84-86` renders `EmptyState` when `features.length === 0`. Existing `test('feature list handles empty state')` (`app.spec.ts:22-30`) skips when the workspace has features — it is environment-dependent, not a deterministic 0-feature assertion.
- **Explanation**: The 0-feature + missing-field combination is covered. The dedicated 0-feature empty-state assertion is environment-skipped (no seeding harness). Defensive default `?? 0` verified end-to-end. Noted for completeness; not blocking.

### AC-003: Given N features, create via intake form, badge updates to N+1 after refetch
- **Status**: NOT MET
- **Evidence**: `ui/e2e/app.spec.ts` contains no test that fills the intake form, submits, and asserts the badge increments. `app.spec.ts:53-61` (`test('new feature button opens form')`) only asserts the form opens, not submission. The implementation supports the behavior: `Dashboard.tsx:20-26` `createMutation.onSuccess` → `queryClient.invalidateQueries({ queryKey: ['features'] })` → refetch → `data.total_count` updates → badge re-renders (badge gated on `!isLoading && !error`, `Dashboard.tsx:44`).
- **Explanation**: The verification specified in acceptance.md ("Load dashboard with 2 features… fill and submit the intake form; wait for the features list query to refetch; assert badge text is '3' and the list has 3 rows") is not implemented. The codepath is correct but unverified at the e2e level. This is a required gap — AC-003 explicitly names this scenario at test level e2e.

### AC-004: During loading, badge does not render stale/NaN
- **Status**: MET
- **Evidence**: `Dashboard.tsx:44` `{!isLoading && !error && (...)}` gates the badge on `!isLoading`. During loading `data` is `undefined` (React Query semantics), so `data?.total_count ?? 0` would yield `0` — but the badge is not rendered at all because the gate short-circuits. NaN/undefined are impossible by construction. `Dashboard.tsx:71-76` renders the loading spinner during `isLoading`.
- **Explanation**: Implementation is correct — badge is absent during loading, never stale, never NaN. The acceptance criterion's verification ("throttle response; during loading spinner phase, assert badge absent or last known value — never NaN or undefined") specifies an e2e test that does not exist. However, the AC's core requirement is "the badge does not render stale or NaN content" — the implementation guarantees this by construction (gate prevents rendering). Marking MET on the implementation; the absent e2e test is noted (F-002) but the criterion itself is satisfied by the code.

### AC-005: Missing total_count → safe default, no console error
- **Status**: MET
- **Evidence**: `ui/e2e/app.spec.ts:166-192` intercepts `**/api/features`, deletes `body.total_count`, fulfills 200; asserts `consoleErrors` empty, `features-error` count 0, badge visible with text `'0'`. `Dashboard.tsx:37` `const totalCount = data?.total_count ?? 0;` provides the defensive default.
- **Explanation**: FR-009 verified end-to-end.

### AC-006: API 500 → "Failed to load features" shown, badge not rendered
- **Status**: MET
- **Evidence**: `ui/e2e/app.spec.ts:194-203` `test('feature count badge absent on API error')` intercepts 500, asserts `[data-testid="features-error"]` visible and `[data-testid="feature-count-badge"]` count 0. `Dashboard.tsx:44` gate (`!error`) and `78-82` error block.
- **Explanation**: Error path verified. Badge absent on error confirmed.

### AC-007: Badge has aria-label matching /Total features: \d+/
- **Status**: MET
- **Evidence**: `ui/e2e/app.spec.ts:156-164` `test('feature count badge has accessible aria-label')` asserts `ariaLabel.match(/Total features: \d+/)`. `Dashboard.tsx:47` `aria-label={\`Total features: ${totalCount}\`}`.
- **Explanation**: NFR-003 satisfied. Verified end-to-end.

### AC-008: 3 features → total_count==3, ==features.length
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:84-143` `TestListFeaturesTotalCountPopulated` creates 3 features via POST, GETs `/api/features`, asserts `resp["total_count"] == float64(3)` and `resp["total_count"] == float64(len(features))`. Test passes.
- **Explanation**: N=3 case with invariant assertion verified.

### AC-009: 0 features → total_count==0, features [] not null
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:73-77` extends `TestListFeaturesEmpty`: `if resp["total_count"] != float64(0) { t.Errorf(...) }`. Existing assertions confirm `features` is an array of length 0. `internal/api/dto.go:90` `summaries := make([]FeatureSummaryResponse, 0, len(features))` — non-nil empty slice serializes as `[]` not `null`. Test passes.
- **Explanation**: FR-005 (features [] not null) preserved. Empty state verified.

### AC-010: 1 feature → total_count==1, features length 1
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:453-478` `TestSmokeCreateAndGetFeature` extension: after POST creates 1 feature, GETs `/api/features`, asserts `body["total_count"] == float64(1)` and `len(features) == 1`. Test passes.
- **Explanation**: N=1 case verified.

### AC-011: 500 error → no total_count field
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:561-621` `TestListFeaturesErrorResponseHasNoTotalCount` points SpecProvider at chmod-0000 dir, GETs `/api/features`, asserts status 500, `bytes.Contains(body, "total_count")` is false, decoded map has no `total_count` key, has `error` key. Additionally `internal/api/server_test.go:147-170` `TestErrorResponseShape` marshals `ErrorResponse{Error: "internal_error", Details: "Failed to list features"}` and asserts JSON omits `total_count`. `internal/api/dto.go:11-14` `ErrorResponse` struct has only `error` and `details` fields. `internal/api/server.go:131` `writeError` uses `ErrorResponse`. Both tests pass (verified as non-root uid 1000).
- **Explanation**: FR-003 verified by live 500 (chmod trick works as non-root) AND by construction (ErrorResponse struct shape). Doubly covered.

### AC-012: total_count == features.length for N in {0, 1, 5, 50}
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:481-559` `TestListFeaturesTotalCountConsistency` parametrizes over `[]int{0, 1, 5, 50}`, creates N features, GETs `/api/features`, asserts `int(tc) == n`, `len(feats) == n`, `int(tc) == len(feats)`, AND guards the null-array regression with `bytes.Contains(raw, "\"features\":null")`. All 4 subtests pass.
- **Explanation**: Literal {0,1,5,50} set exercised. Invariant `total_count == len(features)` enforced at all four N values. Null-array regression guard included. This AC is fully covered — previous review's note about N=5/N=50 is resolved.

### AC-013: FeatureListResponse type declares total_count: number; listFeatures returns it
- **Status**: MET
- **Evidence**: `ui/src/types/index.ts:14-17` `export interface FeatureListResponse { features: FeatureSummary[]; total_count: number; }` — required, not optional. `ui/src/api/client.ts:51-52` `export async function listFeatures(): Promise<FeatureListResponse> { return request<FeatureListResponse>('/features'); }`. `npm run build` succeeds (tsc -b passes).
- **Explanation**: Type-level contract declared. `total_count` required (not `?:`), matching FR-002.

### AC-014: TestListFeaturesEmpty asserts total_count == 0
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:73-77` `if resp["total_count"] != float64(0) { t.Errorf("expected total_count 0, got %v", resp["total_count"]) }`. Uses correct `float64` comparison (JSON numbers decode to `float64` in `map[string]interface{}`). Test passes.
- **Explanation**: Existing test extended with the new assertion. Correct type comparison.

### AC-015: Populated list test asserts total_count == N
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:130-131` `TestListFeaturesTotalCountPopulated` asserts `resp["total_count"] == float64(3)` for N=3. Plus `server_test.go:469-470` asserts `total_count == float64(1)` in the smoke test. Both pass.
- **Explanation**: Populated case verified with invariant assertion at N=3 and literal at N=1.

---

## Step 3: Key Checks

### Null Pointer Safety — PASS
- `internal/api/server.go:127-137` `listFeatures`: calls `s.pipeline.ListFeatures()`, checks `err` before building response. No nil deref.
- `internal/api/dto.go:95` `if questionStore != nil` guards the `PendingCount` call. `questionStore` may be nil (passed as nil in `TestListFeaturesEmpty` setup at `server_test.go:55`).
- `internal/api/dto.go:90` `summaries := make([]FeatureSummaryResponse, 0, len(features))` — initialized empty, never nil.
- `Dashboard.tsx:36-37` uses optional chaining (`data?.features`, `data?.total_count`) — safe when `data` is `undefined` during loading.
- No new struct fields, no new initialization ordering. No nil pointer risk introduced.

### JSON Serialization — PASS
- `features` serializes as `[]` not `null` on empty state: `make([]FeatureSummaryResponse, 0, ...)` produces a non-nil empty slice (`dto.go:90`). `TestListFeaturesEmpty` confirms length 0 array. `TestListFeaturesTotalCountConsistency` explicitly guards `"features":null` with `bytes.Contains` (`server_test.go:531`). FR-005 preserved and regression-guarded.
- `total_count` is a map key (`map[string]interface{}{"total_count": len(summaries)}` at `dto.go:105`) — always serializes, even when 0. No `omitempty` possible on map keys. FR-002 satisfied by construction.
- `rg -n "omitempty" internal/api/dto.go` shows `omitempty` only on `ErrorResponse.Details`, `CreateFeatureRequest.FileContent`, `GateResult`, `StartedAt`, `CompletedAt`, `CheckResultResponse.Message` — none on `total_count` or `features`. PASS.

### Error Path Coverage — PASS
- 500 path (`server.go:128-132`): `ListFeatures` error → `writeError(w, 500, "internal_error", "Failed to list features")` → `ErrorResponse` (no `total_count`). Verified by `TestListFeaturesErrorResponseHasNoTotalCount` (live 500) and `TestErrorResponseShape` (struct shape).
- This endpoint takes no input and reads local state — no 400/404/409 paths apply (confirmed in plan.md API Contracts: "No other status codes are possible from this endpoint"). Existing 400/404 handlers on other endpoints unchanged.
- Empty state returns 200 with `{"features": [], "total_count": 0}` — verified by `TestListFeaturesEmpty` and `TestListFeaturesTotalCountConsistency` N=0 subtest.

### Middleware Chain — PASS
- `internal/api/server.go:64` `handler := s.recoveryMiddleware(s.corsMiddleware(mux))` — recovery is outermost (catches panics in CORS and all handlers). Unchanged by this feature.
- `server.go:83-93` `corsMiddleware` sets `Access-Control-Allow-Origin: *` — pre-existing, not this feature's scope (spec assumes local-only mode). No change required.
- `server.go:96-106` `recoveryMiddleware` catches panics, logs, returns 500 via `ErrorResponse`. Unchanged.
- No new middleware added. No body size limit change (existing `MaxBytesReader` on POST at `server.go:140`; GET has no body).

### Over-Engineering Check — PASS
- Production code: 15 lines (dto.go +1/-1, Dashboard.tsx +13/-1, types/index.ts +1). Plan target <30 lines (`tasks.md:207`). Well under.
- No pagination, no filtering, no sorting. No new endpoint. No new pipeline method (`git diff internal/pipeline/` clean for the feature commit). No separate badge component file. No custom hook. No React context. No SSE. No memoized selector.
- Badge is inline `<span>` in `Dashboard.tsx` per plan instruction. Tailwind classes consistent with existing `QuestionBadge.tsx` pattern (`rounded-full text-xs font-bold`).

### Missing Implementation Check — PASS
All 10 FRs covered:
- FR-001 (total_count top-level int) → `dto.go:105`
- FR-002 (present on every 200 incl empty) → map key always serializes (no `omitempty`)
- FR-003 (absent on error) → `ErrorResponse` struct + `TestErrorResponseShape` + `TestListFeaturesErrorResponseHasNoTotalCount`
- FR-004 (== len(features)) → `len(summaries)` by construction; verified at N∈{0,1,5,50}
- FR-005 (features [] not null) → `make([]..., 0, ...)` preserved; regression-guarded in `TestListFeaturesTotalCountConsistency`
- FR-006 (badge adjacent to heading) → `Dashboard.tsx:42-53`
- FR-007 (badge shows 0 on empty) → `totalCount ?? 0`, always rendered when `!isLoading && !error`
- FR-008 (updates on refetch) → React Query invalidation in `createMutation.onSuccess` (`Dashboard.tsx:22-23`)
- FR-009 (safe default when missing) → `?? 0`, verified by e2e
- FR-010 (non-interactive) → `<span>` with no `onClick`/`<Link>`/`role` (`Dashboard.tsx:45-52`)

All NFRs covered: NFR-001 (`len()` O(1)), NFR-002 (`min-w-[2.5rem]` at `Dashboard.tsx:48`), NFR-003 (`aria-label` at `Dashboard.tsx:47`), NFR-004 (~15 bytes), NFR-005 (additive field + `?? 0` default).

### Security Review (P2 — recommended) — PASS
- No new attack surface. `total_count` is output-only, derived from `len(summaries)` (data already in the response). No new information disclosed — count is inferable from `features.length` today.
- No new inputs → no input validation needed.
- No auth change — endpoint remains unauthenticated (local-only mode per spec assumption).
- No secrets in logs — `server.go:130` logs the error message, not feature data.
- CORS `*` is pre-existing, not this feature's scope.
- No SQL injection, XSS, CSRF, IDOR, mass assignment risk introduced (no new input binding).

### Constitution Compliance — PASS
- I. Spec-Driven: implementation traces to spec.md + acceptance.md. PASS.
- VIII. Go, Minimal Dependencies: no new Go deps (stdlib only). PASS.
- All other principles N/A for this feature (per plan.md Constitution Check).

---

## Findings

### F-001: Missing e2e test for badge update after feature creation (AC-003)
- **Severity**: Required (blocks gate)
- **Criterion**: AC-003
- **Code**: `ui/e2e/app.spec.ts` (absent test)
- **Description**: acceptance.md AC-003 specifies an e2e test: "Load dashboard with 2 features, assert badge shows '2'; fill and submit the intake form; wait for the features list query to refetch; assert badge text is '3' and the list has 3 rows." No such test exists in `ui/e2e/app.spec.ts`. The existing `test('new feature button opens form')` (`app.spec.ts:53-61`) only asserts the form opens — it does not submit it, does not wait for refetch, and does not assert the badge increments. The implementation supports the behavior (`Dashboard.tsx:20-26` `createMutation.onSuccess` invalidates the `['features']` query → refetch → `data.total_count` updates → badge re-renders), but it is unverified at the e2e level. Add a test that seeds N features (or accepts the current count), reads the badge value, fills the intake form, submits, waits for the features list query to refetch (React Query invalidation), and asserts the badge incremented to N+1.

### F-002: Missing e2e test for loading-state badge behavior (AC-004)
- **Severity**: Noted
- **Criterion**: AC-004
- **Code**: `ui/e2e/app.spec.ts` (absent test)
- **Description**: acceptance.md AC-004 specifies throttling the network and asserting the badge is absent or shows last known value (never NaN) during loading. No such test exists. The implementation is correct (`Dashboard.tsx:44` gates badge on `!isLoading`), so this is a test-coverage gap, not a code defect. The AC's core requirement ("badge does not render stale or NaN content") is satisfied by construction (gate prevents rendering during loading), hence MET on implementation. Recommend adding a test that delays `/api/features` and asserts `[data-testid="feature-count-badge"]` has count 0 during the loading spinner phase for full e2e parity with the AC's verification step.

---

## Quality Gate

| # | Criterion | Status |
|---|---|---|
| 1 | Every AC checked with quoted evidence | PASS (15/15) |
| 2 | "No issues" backed by evidence | PASS — null safety, JSON, error paths, middleware all verified with file:line |
| 3 | Security review complete | PASS (P2 — no new attack surface) |
| 4 | Constitution compliance verified | PASS |
| 5 | Null pointer safety verified | PASS |
| 6 | Error paths verified | PASS (500 path tested live + by construction; no 400/404/409 apply to this endpoint) |
| 7 | Middleware chain verified | PASS (recovery outermost, unchanged) |
| 8 | Over-engineering check completed | PASS (15 lines production, target <30) |
| 9 | Missing implementation check completed | PASS (all 10 FRs + 5 NFRs covered) |

**Gate decision**: BLOCKED — 1 required finding (F-001: AC-003 e2e test missing) must be resolved before advancing to the Testing phase. F-002 is noted and recommended but does not block (implementation is correct; test gap only).

---

## Recirculate / Fix Guidance

### F-001 (Required — recirculate to Construction)
Add to `ui/e2e/app.spec.ts` inside the existing `test.describe('Dev Team Web UI', ...)` block:

```typescript
test('feature count badge updates after creating a feature via intake form', async ({ page }) => {
  await page.goto('/');

  const badge = page.locator('[data-testid="feature-count-badge"]');
  await expect(badge).toBeVisible();
  const beforeText = (await badge.textContent()) ?? '0';
  const before = parseInt(beforeText, 10);

  await page.locator('button:has-text("New Feature")').click();
  await expect(page.locator('form, [data-testid="create-form"]')).toBeVisible();

  // Fill the intake form — use a unique title to avoid duplicate_title 409.
  const unique = `Badge e2e ${Date.now()}`;
  await page.locator('input[name="title"], input[type="text"]').first().fill(unique);
  await page.locator('textarea').first().fill('e2e verification of badge increment');
  // Select priority P2 if a select is present; otherwise accept the default.
  await page.locator('button[type="submit"], button:has-text("Create")').click();

  // Wait for the features list query to refetch (React Query invalidation).
  await expect(badge).toHaveText(String(before + 1));

  // Sanity: the new feature appears in the list.
  await expect(page.locator(`text=${unique}`)).toBeVisible();
});
```

Note: the existing `test('new feature button opens form')` only verifies the form opens. The new test must complete the submit→refetch→badge-increment loop. Adjust selectors to match the actual IntakeForm component (`ui/src/components/IntakeForm.tsx`).

### F-002 (Noted — optional)
Add to `ui/e2e/app.spec.ts`:

```typescript
test('feature count badge absent during loading state', async ({ page }) => {
  await page.route('**/api/features', async route => {
    await new Promise(r => setTimeout(r, 500));
    await route.continue();
  });
  await page.goto('/');
  // During the loading spinner phase, the badge must not be in the DOM.
  await expect(page.locator('[data-testid="features-loading"]')).toBeVisible();
  await expect(page.locator('[data-testid="feature-count-badge"]')).toHaveCount(0);
});
```

---

## Verification Commands Run

| Command | Result |
|---|---|
| `go build ./...` | Success |
| `go test ./internal/api/ -v` | 57 passed, 0 failed |
| `go test ./internal/api/ -run TestListFeaturesTotalCountConsistency -v` | 4 subtests pass (N=0,1,5,50) |
| `go test ./internal/api/ -run TestListFeaturesErrorResponseHasNoTotalCount -v` | Pass (non-root uid 1000) |
| `cd ui && npm run build` | tsc -b + vite build succeed |
| `cd ui && npx playwright test --reporter=line` | 9 passed, 0 failed, 2 skipped |
| `cd ui && npx playwright test --grep "feature count badge"` | 4 passed, 0 failed |
| `curl -s http://localhost:8765/api/features \| jq '{total_count, feature_count: (.features \| length)}'` | `{total_count: 5, feature_count: 5}` |

All verifications performed against commit `e354868` on branch `feat/spec-count-badge` with the systemd-managed `devteam-web.service` running on `:8765`.