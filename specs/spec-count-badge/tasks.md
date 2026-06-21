# Tasks: Feature Spec Count Badge

Tasks grouped by user story priority. Single repo (`devteam`), single UI source file modified, one new e2e spec. No backend changes, no new types, no new components, no new dependencies.

Dependency order: **T-001 → T-002 → T-003**. T-001 and T-002 are sequential (tests assert the badge the source change introduces). T-003 is the verification gate, not an implementation task.

---

## P1 — US-001 (See total feature spec count) + US-002 (See zero count in empty state)

P1 covers both user stories because they share a single implementation (the badge renders in all states; the count is `features.length`).

---

### Task: T-001 [MODIFY] Render count badge in Dashboard header

Priority: P1
User stories: US-001, US-002
Acceptance criteria covered: AC-001, AC-002, AC-003, AC-006, AC-007, AC-014, AC-015

Files:
- `ui/src/pages/Dashboard.tsx` — MODIFY

Dependencies: none (first task)

What to change (exact):
1. After the existing `const features: FeatureSummary[] = data?.features ?? [];` (line 36), add:
   ```ts
   const count = features.length;
   const badgeLabel = count === 1 ? '1 feature' : `${count} features`;
   ```
2. Inside the header `<div className="flex items-center justify-between mb-6">` (lines 40-49), between the `<h2>Features</h2>` and the `+ New Feature` `<button>`, insert a badge `<span>`:
   ```tsx
   <span
     data-testid="feature-count-badge"
     className="inline-flex items-center rounded-full bg-blue-100 px-3 py-1 text-sm font-medium text-blue-700 dark:bg-blue-900 dark:text-blue-200"
   >
     {badgeLabel}
   </span>
   ```
   The `<span>` is a sibling of the `<h2>` and the `<button>`, inside the same flex header row. It is NOT inside any of the `isLoading` / `error` / empty / list conditional branches below — this guarantees FR-005 (visible in all states).

Do NOT:
- Create a new component file (no `CountBadge.tsx`).
- Create a new helper file (no `pluralize.ts`).
- Add a new npm dependency.
- Change any other file (`client.ts`, `types/index.ts`, any `internal/` file).
- Add a `total_count` field to any response.
- Move the badge inside any conditional branch.
- Wrap the badge in `isLoading && (...)` or `!error && (...)`.

Done conditions (each must be verified — proof of work, not claims):
- [ ] `cd ui && npm run build` exits 0 with zero TypeScript errors. Paste the final build output line as evidence.
- [ ] The Dashboard header contains an element with `data-testid="feature-count-badge"`. Verify by reading the rendered `Dashboard.tsx` and confirming the `<span>` is a direct child of the header flex div and a sibling of the `<h2>` and `<button>`.
- [ ] The badge renders "0 features" when `data` is undefined (initial load) — verify by reasoning: `data?.features ?? []` yields `[]`, `length === 0`, ternary → "0 features". (This satisfies AC-015's null-safety intent at the source level; the e2e in T-002 verifies it in a browser.)
- [ ] The badge renders "1 feature" (singular) for `features.length === 1` — verify the ternary `count === 1 ? '1 feature' : ...` (AC-014).
- [ ] The badge renders "{N} features" (plural) for `features.length !== 1` including 0 — verify the ternary's else branch (AC-014).
- [ ] The badge is NOT inside any `isLoading`, `error`, empty, or `FeatureList` conditional branch — verify by reading the JSX that the `<span>` sits above line 59 (the first conditional) and is unconditionally rendered.
- [ ] No `NaN` / `undefined` / throw path: `features` is always an array (`?? []`), so `count` is always a non-negative integer. Confirm there is no code path where `count` could be `undefined` or `NaN`.

Test level: smoke (build + render) + e2e (T-002 covers the full e2e). This task's own verification is smoke-level (build passes, source is correct by inspection).

Agent failure mode checks:
- [x] Nil pointer ordering: N/A — no initialization code, no new struct, no new component lifecycle. `features` is already safely derived at line 36; `count` and `badgeLabel` are computed after it. Order is correct.
- [x] JSON serialization null vs empty arrays: N/A — this task produces no JSON serialization. The existing `features` array contract (never null) is unchanged. Regression guard is the existing `TestListFeaturesEmpty` + AC-012 (verified in T-002/T-003).
- [x] Recovery middleware first in chain: N/A — no HTTP handlers, no middleware, no backend change.
- [x] State machine transitions: N/A — no state machine (badge is a pure derived view; spec §"State Transitions: None").

Quality verification steps after this task:
1. `cd ui && npm run build` — must pass.
2. Read `Dashboard.tsx` end-to-end and confirm: badge `<span>` is in the header div, has `data-testid="feature-count-badge"`, renders `{badgeLabel}`, and `badgeLabel` is derived from `features.length` via the ternary. Confirm no other file was modified (`git status` shows only `ui/src/pages/Dashboard.tsx`).

---

### Task: T-002 [CREATE] E2E spec for the count badge

Priority: P1
User stories: US-001, US-002
Acceptance criteria covered: AC-001, AC-002, AC-003, AC-004, AC-005, AC-006, AC-007, AC-008, AC-009, AC-010, AC-011, AC-014 (via e2e), AC-015 (via e2e)

Files:
- `ui/e2e/count-badge.spec.ts` — CREATE

Dependencies: T-001 must complete first (the badge element must exist in the DOM before tests can assert on it).

What to create (exact):
A Playwright spec following the patterns in `ui/e2e/app.spec.ts` (`import { test, expect } from '@playwright/test'`). Use `page.route('**/api/features', ...)` to mock `GET /api/features` with deterministic counts. Mock responses MUST respect the real contract: `features` is an array (possibly empty), never `null`. Use synthetic `FeatureSummary` objects with all required fields (id, title, status, priority, current_phase, updated_at, gate_result, pending_questions_count).

Required test cases (one per acceptance criterion; group as a `test.describe('Count badge', () => { ... })`):

1. **AC-001** — `test('badge shows total count (3 features)')`: `page.route` to fulfill `{ status: 200, contentType: 'application/json', body: JSON.stringify({ features: [f1, f2, f3] }) }` with 3 synthetic FeatureSummary objects; `page.goto('/')`; `await expect(page.locator('[data-testid="feature-count-badge"]')).toHaveText('3 features')`.
2. **AC-002 / AC-014** — `test('badge shows singular (1 feature)')`: mock 1 feature; assert badge text `'1 feature'`.
3. **AC-003 / AC-014** — `test('badge shows plural (5 features)')`: mock 5 features; assert badge text `'5 features'`.
4. **AC-006 / AC-014** — `test('badge shows 0 features in empty state')`: mock `{ features: [] }`; assert badge text `'0 features'` AND assert the empty-state element (`[data-testid="empty-state"]`, per `EmptyState.tsx:7`) is visible (AC-007).
5. **AC-004** — `test('no console errors on badge render')`: in any populated-count test, attach `page.on('console', ...)` capturing errors and warnings; assert zero messages referencing the badge (and zero errors overall, matching the existing `app.spec.ts` pattern).
6. **AC-008 / AC-015** — `test('badge shows 0 features on initial-load 500')`: `page.route` to fulfill `{ status: 500, ... }` (or `route.abort('failed')`); `page.goto('/')`; assert badge text `'0 features'` AND assert `[data-testid="features-error"]` is visible.
7. **AC-009** — `test('badge keeps last known count on refetch error')`: first `page.route` returns 2 features; `page.goto('/')`; assert badge `'2 features'`; then change the route handler (use a mutable flag or `route.continue`/`route.fulfill` swap) to return 500 on subsequent requests; trigger a refetch (e.g., `await page.evaluate(() => window.location.reload())` or invalidate via the UI); assert badge STILL reads `'2 features'` AND `[data-testid="features-error"]` becomes visible. Note: react-query keeps `data` from the last successful fetch while transitioning to error — verify this is the actual behavior; if react-query v5 nulls `data` on error in this app's config, document it and adjust the assertion to match the real behavior (conservative: assert the badge renders a number, not NaN/undefined, and the error block is present).
8. **AC-010** — `test('badge unchanged on create 409')`: mock list returning 1 feature; `page.goto('/');` assert badge `'1 feature'`; `page.route('**/api/features', route => { if (route.request().method() === 'POST') return route.fulfill({ status: 409, contentType: 'application/json', body: JSON.stringify({ error: 'duplicate_title', details: '...' }) }); return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ features: [f1] }) }); })`; open IntakeForm, fill valid-but-duplicate title, submit; assert badge STILL `'1 feature'`; assert a toast with duplicate-title text appears (locate via existing toast testid if present, else by text match consistent with `Dashboard.tsx:29`).
9. **AC-011** — `test('badge unchanged on create 400')`: as AC-010 but POST returns 400 `{ error: 'validation_error', details: '...' }`; assert badge unchanged and zero console errors.
10. **AC-005** — `test('badge increments on successful create')`: mock list returning 3 features; `page.goto('/');` assert badge `'3 features'`; set route to return 4 features on the next `GET /api/features`; open IntakeForm, submit valid input; `POST` returns 201 with a feature object; assert badge text becomes `'4 features'` (use `await expect(locator).toHaveText('4 features')` which auto-waits the re-render). No full page reload — assert via the SPA navigation not triggering (Playwright stays on `/`).

Do NOT:
- Add Vitest, Jest, or any unit test runner (per AD-5).
- Seed real features via the live API for count-specific assertions — counts must be deterministic via `page.route`.
- Assert exact badge text against live (un-seeded) DB state.
- Modify `app.spec.ts` (existing tests remain valid as regression; new assertions go in the new file).
- Introduce a new Playwright config or change `webServer` — reuse the existing config.
- Skip the error-path tests (AC-008, AC-009) — these are mandatory, not nice-to-have.

Done conditions (each must be verified — proof of work, not claims):
- [ ] `ui/e2e/count-badge.spec.ts` exists and imports `{ test, expect } from '@playwright/test'`.
- [ ] All 10 test cases above are present (one per AC, grouped under `test.describe('Count badge', ...)`). Name them so the Tester can map each to its AC.
- [ ] `cd ui && npx playwright test e2e/count-badge.spec.ts` exits 0 with all tests passing. Paste the Playwright summary line (e.g., "  10 passed") as evidence.
- [ ] Existing e2e tests still pass: `cd ui && npx playwright test e2e/app.spec.ts` exits 0. Paste the summary line.
- [ ] AC-001 verified: the test asserting `'3 features'` passes.
- [ ] AC-002/AC-014 verified: the `'1 feature'` singular test passes.
- [ ] AC-003/AC-014 verified: the `'5 features'` plural test passes.
- [ ] AC-006/AC-007 verified: the `'0 features'` + `[data-testid="empty-state"]` test passes.
- [ ] AC-008/AC-015 verified: the initial-load-500 `'0 features'` + `[data-testid="features-error"]` test passes.
- [ ] AC-009 verified: the refetch-error "last known count" test passes (or, if react-query v5 behavior differs, the test documents the real behavior and still asserts the badge renders a finite number with the error block present — quote the actual assertion used).
- [ ] AC-010 verified: the create-409 "badge unchanged" test passes.
- [ ] AC-011 verified: the create-400 "badge unchanged + no console errors" test passes.
- [ ] AC-005 verified: the successful-create "badge increments without reload" test passes.
- [ ] AC-004 verified: the console-error capture in at least one test asserts zero errors.
- [ ] Mocks respect the contract: every mocked `GET /api/features` body has `features` as a JSON array (empty or populated), never `null`. Verify by reading each `route.fulfill` body.
- [ ] Synthetic FeatureSummary objects include all required fields (`id`, `title`, `status`, `priority`, `current_phase`, `updated_at`, `gate_result`, `pending_questions_count`) — verify by reading the mock fixture.

Test level: e2e (this task IS the e2e test suite).

Agent failure mode checks:
- [x] Nil pointer ordering: N/A — test code only; no app initialization. (If a test sets up a route then navigates, the route must be registered before `page.goto` — verify ordering in each test: `page.route(...)` comes before `page.goto('/')`.)
- [x] JSON arrays are [] not null: the mocked `features` field must be an array in every mock — explicitly checked in done conditions. Do not write `body: JSON.stringify({ features: null })` even in error tests (error tests return a 500 status, not a 200 with null features).
- [x] Recovery middleware first in chain: N/A — no middleware.
- [x] State machine transitions: N/A — no state machine. (AC-009's refetch behavior is a react-query cache behavior, not a state machine; verify the real behavior empirically, do not assume.)

Quality verification steps after this task:
1. `cd ui && npx playwright test e2e/count-badge.spec.ts` — must pass all cases.
2. `cd ui && npx playwright test e2e/app.spec.ts` — must still pass (regression).
3. Read the spec file and confirm every mock body has `features` as an array.

Parallel opportunities: none. T-001 must complete before T-002 (tests assert the badge T-001 introduces).

---

### Task: T-003 [VERIFY] Quality gate — full build, all tests, regression

Priority: P1
User stories: US-001, US-002 (both — gate covers the whole feature)
Acceptance criteria covered: all (this is the verification gate, not new implementation)

Files:
- none (verification only — no new code)

Dependencies: T-001 and T-002 must both complete.

What to do:
1. `cd ui && npm run build` — confirm zero TypeScript errors, Vite build succeeds.
2. `cd ui && npx playwright test` — run the full Playwright suite (both `app.spec.ts` and `count-badge.spec.ts`); all tests must pass.
3. `go test ./internal/api/...` — confirm backend tests pass unchanged (regression; this feature makes no backend change, so all existing tests must pass).
4. Manual smoke (proof of work): start the dev server, open the Dashboard in a browser, confirm: (a) the badge is visible in the header next to "Features", (b) the count matches the number of feature cards, (c) creating a feature increments the badge without a page reload, (d) the browser console has zero errors. Quote the exact badge text observed.

Done conditions:
- [ ] `npm run build` exits 0 — paste the final output line.
- [ ] `npx playwright test` exits 0 — paste the summary (e.g., "  X passed").
- [ ] `go test ./internal/api/...` exits 0 — paste the summary.
- [ ] Manual smoke: badge visible, count matches cards, increments on create, zero console errors — quote observed badge text.
- [ ] `git status` shows exactly these changed/new files: `ui/src/pages/Dashboard.tsx` (modified), `ui/e2e/count-badge.spec.ts` (new). No other files changed. No backend files touched. Confirm with `git status --short`.

Test level: smoke + integration + e2e (this task runs all levels).

Agent failure mode checks:
- [x] This is a verification task, not implementation — the checks above confirm the implementation tasks did not introduce failure modes. Specifically: build passing rules out type errors; Playwright passing rules out runtime/render panics and covers all error-path ACs; `go test` passing rules out accidental backend changes (if any `internal/` file appears in `git status`, that's a finding — this feature must not touch the backend).

---

## Summary Table

| Task | Type | Files | Depends on | Test level | ACs covered |
|---|---|---|---|---|---|
| T-001 | MODIFY | `ui/src/pages/Dashboard.tsx` | — | smoke (build) | AC-001,002,003,006,007,014,015 (source) |
| T-002 | CREATE | `ui/e2e/count-badge.spec.ts` | T-001 | e2e | AC-001..011,014,015 |
| T-003 | VERIFY | none | T-001, T-002 | all | all (gate) |

## Cross-Repo Boundaries
None. Single repo (`devteam`), single branch (`main`). No cross-repo dependencies, no release ordering concerns.

## Out of Scope (do not implement)
- New API endpoint
- `total_count` field in any response
- New component file, new helper file, new npm dependency, new test runner (Vitest/Jest)
- Pagination, filtering, sorting, per-phase counts, per-status counts
- Count badge on FeatureDetail page
- SSE / real-time count beyond existing react-query invalidation
- Internationalization beyond English pluralization
- Any backend (`internal/`) change