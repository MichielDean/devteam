# Review Report: Feature Spec Count Badge

**Spec**: feature-spec-count-badge---show-total-count-of-feature-specs
**Phase**: review
**Reviewer**: reviewer (glm-5.2:cloud)
**Date**: 2026-06-21
**Commits reviewed**: `b2eb494` (feat), `79ee384` (test additions), `2ed6faf` (missing-field assertion fix)

---

## Summary

- **Acceptance criteria**: 15 total — 9 MET, 3 MET WITH NOTE, 1 NOT MET
- **Findings**: 0 critical, 1 required (AC-003 e2e gap), 3 noted
- **Production code diff**: 15 lines across 3 files — well within plan's "<30 lines" target. No over-engineering.
- **Backend verification**: `go build ./...` succeeds; 6 targeted Go tests pass (`TestListFeaturesEmpty`, `TestListFeaturesTotalCountPopulated`, `TestErrorResponseShape`, `TestSmokeCreateAndGetFeature`, `TestListFeaturesTotalCountConsistency` {0,1,5,50}, `TestListFeaturesErrorResponseHasNoTotalCount`).
- **Null/empty-array safety**: PASS — `features` serializes as `[]` not `null` (byte-scan guard in `TestListFeaturesTotalCountConsistency/N=0`); `total_count` is a map key (no `omitempty`, always serializes as `0`); error responses omit `total_count` (verified at both DTO-marshal and live-500 levels).
- **Middleware chain**: unchanged — `recoveryMiddleware(corsMiddleware(mux))` (`server.go:64`). Recovery outermost catches panics; CORS present. Not modified by this feature.
- **Security**: P2 feature, no new attack surface. `total_count` is output-only derived from existing data. No new inputs, no new auth. Not applicable beyond existing posture.
- **Gate status**: BLOCKED — 1 required finding (AC-003 e2e gap) must be resolved before advancing to delivery.

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

### AC-001: Given dashboard with 5 features, badge shows "5", list has 5 rows (e2e)
- **Status**: MET WITH NOTE
- **Evidence**: `ui/e2e/app.spec.ts:132-154` `test('feature count badge renders with total count')` asserts `[data-testid="feature-count-badge"]` visible, text matches `/^\d+$/`, and `body.total_count === parseInt(badgeText, 10)`. `ui/src/pages/Dashboard.tsx:45-52` renders `<span data-testid="feature-count-badge" ...>{totalCount}</span>`.
- **Explanation**: Test verifies badge==API invariant for whatever N the live server returns, not a literal "5 features seeded → badge shows '5'". This is a stronger contract check but does not match the literal AC wording ("5 features… displays '5'… list has 5 rows"). The rendering path is correct. The literal N=5 scenario is not independently seeded at e2e level.

### AC-002: Given 0 features, badge "0", empty state, no console errors (e2e)
- **Status**: MET WITH NOTE
- **Evidence**: `ui/e2e/app.spec.ts:166-192` `test('feature count badge handles missing total_count defensively')` intercepts the list response and asserts no console errors; `Dashboard.tsx:84-86` renders `EmptyState` when `features.length === 0`. Integration-level `TestListFeaturesEmpty` (`server_test.go:51-78`) asserts `total_count == 0` and `features` is `[]` length 0.
- **Explanation**: No dedicated e2e test seeds 0 features and asserts badge=="0" + empty-state element visible. The empty-state contract is verified at integration level (backend) and the defensive-default path is verified at e2e level (different test). The literal "0 features on disk" e2e scenario is not scripted.

### AC-003: Given N features, create via intake form, badge updates to N+1 after refetch (e2e)
- **Status**: NOT MET
- **Evidence**: `ui/e2e/app.spec.ts:132-203` (all 4 badge tests) contains no test that fills the intake form, submits, waits for refetch, and asserts badge increments. `Dashboard.tsx:20-26` implements the path: `createMutation.onSuccess` → `queryClient.invalidateQueries({ queryKey: ['features'] })` → refetch → `data.total_count` updates → badge re-renders. The implementation is correct; the e2e verification specified in acceptance.md ("Load dashboard with 2 features… fill and submit the intake form; wait for refetch; assert badge text is '3' and the list has 3 rows") is absent.
- **Explanation**: The AC explicitly names test level e2e and specifies the create→refetch→increment scenario. The codepath is correct but unverified at the e2e level. Required gap.

### AC-004: Loading state, badge does not render stale or NaN (e2e)
- **Status**: MET
- **Evidence**: `ui/src/pages/Dashboard.tsx:44` `{!isLoading && !error && (...)}` gates the badge — during `isLoading` the badge is not in the DOM. `Dashboard.tsx:37` `const totalCount = data?.total_count ?? 0;` — `data` is `undefined` during load, so `?? 0` yields `0`, but the badge is not rendered during loading so no `0`/`NaN` flash. `app.spec.ts:194-203` error-path test confirms badge absent when `error` truthy (analogous gate).
- **Explanation**: Badge gating prevents any stale/NaN render. No `NaN`/`undefined` can reach the DOM. Verified by code inspection + the error-path e2e (which exercises the `!error` half of the gate).

### AC-005: Defensive default when total_count missing (e2e)
- **Status**: MET
- **Evidence**: `ui/e2e/app.spec.ts:166-192` intercepts `GET /api/features`, deletes `total_count`, asserts `consoleErrors` empty, badge visible with text `'0'` (`app.spec.ts:188-189` `await expect(badge).toHaveText('0')`). `Dashboard.tsx:37` `data?.total_count ?? 0`. Commit `2ed6faf` tightened this test from a vacuous conditional to a hard assertion.
- **Explanation**: Full match. Badge renders "0", no console error, no crash.

### AC-006: Error path, badge not rendered (e2e)
- **Status**: MET
- **Evidence**: `ui/e2e/app.spec.ts:194-203` intercepts → 500 `{error, details}`, asserts `features-error` visible and `feature-count-badge` count is 0 (absent). `Dashboard.tsx:44` `!error` gate hides badge; `Dashboard.tsx:78-82` renders `features-error`.
- **Explanation**: Full match.

### AC-007: Badge has aria-label matching /Total features: \d+/ (e2e)
- **Status**: MET
- **Evidence**: `ui/e2e/app.spec.ts:156-164` asserts `aria-label` matches `/Total features: \d+/`. `Dashboard.tsx:47` `aria-label={\`Total features: ${totalCount}\`}`.
- **Explanation**: Full match.

### AC-008: 3 features → total_count: 3, equals array length (integration)
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:80-145` `TestListFeaturesTotalCountPopulated` creates 3 features via POST, GET /api/features → 200, asserts `resp["total_count"] == float64(3)`, `len(features) == 3`, and `total_count == float64(len(features))`.
- **Explanation**: Full match.

### AC-009: 0 features → {"features": [], "total_count": 0}, array not null (integration)
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:51-78` `TestListFeaturesEmpty` asserts `resp["total_count"] == float64(0)` and `resp["features"]` is a `[]interface{}` of length 0 (type-asserted, not nil). `server_test.go:534` `TestListFeaturesTotalCountConsistency/N=0` byte-scans the raw body for `"features":null` and fails if found. `dto.go:90` `summaries := make([]FeatureSummaryResponse, 0, len(features))` — initialized non-nil.
- **Explanation**: Full match. Two independent guards against the null-array regression.

### AC-010: 1 feature → total_count: 1, array length 1 (integration)
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:457-479` `TestSmokeCreateAndGetFeature` extended: after POST creates 1 feature, GET /api/features → 200, asserts `body["total_count"] == float64(1)` and `len(features) == 1`.
- **Explanation**: Full match.

### AC-011: Error response 500, no total_count field (integration)
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:561-621` `TestListFeaturesErrorResponseHasNoTotalCount` makes specs dir unreadable (`chmod 0000`), GET /api/features → 500, asserts body does NOT contain substring `total_count` (byte scan) and decoded map lacks `total_count` key. `internal/api/server_test.go:147-169` `TestErrorResponseShape` marshals `ErrorResponse{Error, Details}` directly and asserts no `total_count` key. `dto.go:11-14` `ErrorResponse` struct has only `error` and `details` fields.
- **Explanation**: Full match — verified both via live 500 (integration) and DTO marshal (unit). `writeError` (`server.go:131`) returns before `FeaturesToSummaryResponse` runs, so `total_count` cannot leak into error responses.

### AC-012: total_count == len(features) for N in {0, 1, 5, 50} (integration)
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:481-559` `TestListFeaturesTotalCountConsistency` parametrized over N ∈ {0, 1, 5, 50}, seeds N features, asserts `int(tc) == n`, `len(feats) == n`, and `int(tc) == len(feats)` (FR-004 invariant). Also guards `"features":null` byte scan.
- **Explanation**: Full match — exact N set from the AC.

### AC-013: FeatureListResponse TypeScript type declares total_count: number (unit)
- **Status**: MET
- **Evidence**: `ui/src/types/index.ts:14-17` `export interface FeatureListResponse { features: FeatureSummary[]; total_count: number; }`. `ui/src/api/client.ts:51-53` `listFeatures(): Promise<FeatureListResponse>`. Field is required (not `?:`).
- **Explanation**: Full match. `tsc -b` (run by `npm run build`) enforces this at the type level. Test report confirms build succeeds with no `total_count`-related type errors.

### AC-014: TestListFeaturesEmpty asserts total_count == 0 (unit)
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:75-77` `if resp["total_count"] != float64(0) { t.Errorf("expected total_count 0, got %v", resp["total_count"]) }`. Ran `go test ./internal/api/ -run TestListFeaturesEmpty -v` → PASS.
- **Explanation**: Full match. Uses `float64(0)` correctly (JSON number decode into `map[string]interface{}`).

### AC-015: Populated list test asserts total_count == N (integration)
- **Status**: MET
- **Evidence**: `internal/api/server_test.go:130` `if resp["total_count"] != float64(3) { ... }` in `TestListFeaturesTotalCountPopulated` (N=3). Also `server_test.go:142-144` asserts the invariant `total_count == float64(len(features))`.
- **Explanation**: Full match — asserts both literal N and the invariant.

---

## Step 3: Over-Engineering Check

| File | Lines changed | Plan target | Verdict |
|---|---|---|---|
| `internal/api/dto.go` | +1/-1 (return statement) | ~5 | PASS — minimal |
| `ui/src/types/index.ts` | +1 | ~1 | PASS |
| `ui/src/pages/Dashboard.tsx` | +13/-1 | ~15 | PASS |
| `ui/e2e/app.spec.ts` | +75 (tests) | tests, not production | PASS |
| `internal/api/server_test.go` | +268 (tests) | tests, not production | PASS |
| **Production total** | **15 lines** | **<30** | **PASS** |

No over-engineering. No pagination, filtering, sorting, new endpoints, new pipeline methods, new components/hooks/context, SSE, or CLI changes. Scope boundaries respected.

---

## Step 4: Missing Implementation Check

| User story | Implemented? | Evidence |
|---|---|---|
| US-1 (badge) | YES | `Dashboard.tsx:42-53` renders the badge; `types/index.ts:16` declares the field |
| US-2 (API field) | YES | `dto.go:105` returns `{"features": summaries, "total_count": len(summaries)}` |

All functional requirements (FR-001..FR-010) have corresponding code:
- FR-001..FR-005 (API): `dto.go:105`
- FR-006..FR-010 (UI): `Dashboard.tsx:37,44-52`

---

## Step 5: Quality Gate Checks

### Null Pointer Safety
- `FeaturesToSummaryResponse` (`dto.go:89-106`): no new pointers. `questionStore` nil-check exists at line 95. `len(summaries)` on a `make`-initialized slice is safe.
- `Dashboard.tsx`: `data?.total_count ?? 0` — null-safe. `isLoading`/`error` are boolean/truthy checks, no deref.
- No new struct fields, no new initialization ordering. PASS.

### JSON Serialization
- `features` → `[]FeatureSummaryResponse` initialized via `make([]..., 0, len(features))` (`dto.go:90`) → serializes as `[]` not `null`. Guarded by `TestListFeaturesTotalCountConsistency/N=0` byte scan.
- `total_count` → map key (int), no struct tag, no `omitempty` → always serializes, even when 0. Verified by `grep omitempty internal/api/dto.go` (7 matches, none on `total_count` — it's a map key, not a struct field).
- Error responses: `ErrorResponse` has only `error` + `details` → no `total_count` leak. Guarded by `TestListFeaturesErrorResponseHasNoTotalCount` + `TestErrorResponseShape`.
- PASS.

### Error Path Coverage
- 400 (invalid input): `server.go:144-198` `writeError(w, 400, ...)` — unchanged, not affected by this feature.
- 404 (missing feature): `server.go:214` `writeError(w, 404, "feature_not_found", ...)` — unchanged.
- 500 (list failure): `server.go:131` `writeError(w, 500, "internal_error", "Failed to list features")` — returns before DTO builder, so `total_count` never appears on error. Verified live.
- 500 (panic): `server.go:96-106` `recoveryMiddleware` catches panics, writes `internal_error`. Unchanged.
- Empty state: `total_count: 0`, `features: []` — verified.
- PASS.

### Middleware Chain
- `server.go:64` `s.recoveryMiddleware(s.corsMiddleware(mux))` — recovery outermost, CORS inside, mux innermost. Unchanged by this feature. `TestRecoveryMiddleware` (`server_test.go:220` per test report) confirms panic recovery. PASS.
- CORS is `*` (`server.go:85`) — pre-existing, not introduced by this feature. Not a regression. Noted (not a finding — local-only mode per spec).

### Security (P2 — not mandatory, but reviewed)
- No new inputs (output-only derived field). No new endpoints. No new auth surface. No new information disclosed (`total_count` is inferable from `len(features)` which already exists in the response). No secrets in logs (`server.go:130` logs the error, not the response body). PASS — no security concerns introduced.

---

## Findings

### F-001: Missing e2e test for AC-003 (create → refetch → badge increment)
- **Severity**: REQUIRED
- **Criterion**: AC-003
- **Code**: `ui/e2e/app.spec.ts` (absent test)
- **Description**: The acceptance criterion explicitly specifies an e2e test: "Load dashboard with 2 features, assert badge shows '2'; fill and submit the intake form; wait for the features list query to refetch (React Query invalidation); assert badge text is '3' and the list has 3 rows." No such test exists. The implementation path is correct (`Dashboard.tsx:22-23` invalidates `['features']` query on mutation success, triggering refetch and badge re-render), but the scenario is unverified at the e2e level.
- **Fix**: Add a Playwright test that: (1) loads `/`, (2) captures initial badge text, (3) clicks `+ New Feature`, (4) fills and submits the intake form, (5) waits for the badge to update (`await expect(badge).not.toHaveText(initialText)` or `toHaveText(String(initial+1))`), (6) asserts the list row count incremented. Must use `page.route` to intercept POST if deterministic seeding is needed, or seed via API before navigation.

### F-002: AC-001 uses contract check instead of literal N=5 seeding (noted)
- **Severity**: NOTED
- **Criterion**: AC-001
- **Code**: `ui/e2e/app.spec.ts:132-154`
- **Description**: Test asserts `badgeText == API total_count` rather than seeding exactly 5 features and asserting literal "5". This is arguably stronger (verifies the invariant for any N) but does not match the literal AC verification ("5 features seeded… text content '5'… 5 child rows"). Not blocking — the contract is verified.

### F-003: AC-002 empty-state e2e not independently scripted (noted)
- **Severity**: NOTED
- **Criterion**: AC-002
- **Code**: `ui/e2e/app.spec.ts` (no dedicated 0-feature test)
- **Description**: No e2e test seeds 0 features and asserts badge=="0" + empty-state element visible + no console errors. The empty-state contract is covered at integration level (`TestListFeaturesEmpty`) and the defensive-default path is covered at e2e level (different test), but the literal "0 features on disk" e2e scenario is absent. Not blocking — the code path (`Dashboard.tsx:84-86`) is correct and integration-tested.

### F-004: CORS is `*` (noted, pre-existing)
- **Severity**: NOTED
- **Criterion**: N/A (not an AC)
- **Code**: `internal/api/server.go:85` `w.Header().Set("Access-Control-Allow-Origin", "*")`
- **Description**: Pre-existing permissive CORS. Not introduced by this feature (the diff does not touch `server.go`). Spec states local-only mode. Not blocking for this review — flagging for awareness only.

---

## Quality Gate Decision

| # | Criterion | Status |
|---|---|---|
| 1 | Every AC checked with quoted evidence | PASS (15/15) |
| 2 | "No issues found" backed by evidence | N/A (issues found) |
| 3 | Security review complete (P1) | N/A (P2 — reviewed, no concerns) |
| 4 | Constitution compliance verified | PASS (plan §Constitution Check) |
| 5 | Null pointer safety verified | PASS |
| 6 | Error paths verified | PASS |
| 7 | Middleware chain verified | PASS (unchanged) |
| 8 | Over-engineering check completed | PASS (15 lines < 30 target) |
| 9 | Missing implementation check completed | PASS (both user stories implemented) |

**Gate result: BLOCKED.** One required finding (F-001: AC-003 e2e gap) must be resolved before advancing to delivery. The remaining findings (F-002, F-003, F-004) are noted, not blocking.

---

## Recommendation

Recirculate to construction to add the AC-003 e2e test (F-001). The test is ~20 lines of Playwright and exercises an already-correct codepath — low risk, high verification value. Once added, all 15 ACs will be MET or MET WITH NOTE and the gate passes.