# Plan: Feature Spec Count Badge

## Feature ID
spec-count-badge

## Priority
2

## Summary
Render a count badge on the features list page (Dashboard) showing the total number of feature specs, derived client-side from the existing `GET /api/features` response array. No backend change, no new endpoint, no new types.

---

## 1. Spec Validation

Spec is technically feasible. Verified against the codebase:

- **Completeness**: All FR-001..FR-007 trace to US-001/US-002. Error scenarios and empty state explicitly covered. No [NEEDS CLARIFICATION] markers remain. ✓
- **Consistency**: FR-004 (pluralization) and FR-005 (visible in all states) are consistent. The [RESOLVED] note on `total_count` correctly removes an API change from scope. ✓
- **Feasibility**: `GET /api/features` returns `{ features: FeatureSummary[] }`; `data?.features ?? []` already exists at `Dashboard.tsx:36`. `features.length` is the count source. Pure derived view, no backend touch. ✓
- **Edge cases**: Empty (0 features), singular (1 feature), plural (N features), error state (stale/zero), loading (no data yet) — all specified in spec SC-001..SC-005 and AC-006..AC-011. ✓
- **Ambiguity**: PM deferred badge placement to Architect (assumption in spec.md). Resolved below in §6. ✓

### One spec-level finding to flag for the Reviewer (not a blocker)
`acceptance.md` AC-014 and AC-015 are labeled `Test level: unit`. The repo has **no JavaScript unit test runner configured** (only `@playwright/test` for e2e — see `ui/package.json`). Two options:

- **Option A (chosen — conservative, matches existing infra)**: Cover AC-014/AC-015 via Playwright by asserting the rendered badge text across counts 0, 1, 2, 5 using `page.route` interception. This validates the same logic in a real browser without introducing a new toolchain. The pluralization helper is exercised end-to-end.
- **Option B (rejected for this feature)**: Add Vitest. Out of scope for a trivial UI feature — introducing a new test runner is unjustified scope expansion (overconfidence prevention, Pattern 4). The PM's "unit" label was an aspiration; the test selection matrix in planning rules makes E2E the mandatory level for UI changes, and unit only "YES" (recommended, not required). E2E covers it.

Decision documented as an architectural assumption. The Tester will verify AC-014/AC-015 at the e2e level via the rendered badge text, not via a JS unit test. This is flagged here so the Reviewer and Tester do not mark AC-014/AC-015 as unverified.

---

## 2. Technical Context

### Language / Framework / Dependencies
- **Backend (unchanged)**: Go 1.x, `net/http` with `mux.HandleFunc`, JSON via `writeJSON`. API in `internal/api/`.
- **Frontend (changed)**: React 19 + TypeScript 5.8 + Vite 6 + TanStack Query v5. UI in `ui/src/`.
- **Build (frontend)**: `npm run build` (`tsc -b && vite build`). Type-check enforced by `tsc -b`.
- **E2E test**: Playwright 1.61, config at `ui/playwright.config.ts`. Tests in `ui/e2e/`. `webServer` launches the Go binary on port 8766; `baseURL` defaults to `http://localhost:8765` (note: config has a port mismatch — `webServer.port` is 8766 but `baseURL` is 8765; pre-existing, not this feature's concern, flagged for awareness).
- **Existing test infra**: Go integration tests in `internal/api/server_test.go` (incl. `TestListFeaturesEmpty`). Playwright e2e in `ui/e2e/app.spec.ts`.

### Dependencies this feature uses (all existing, no new packages)
- `useQuery({ queryKey: ['features'], queryFn: listFeatures })` — already at `Dashboard.tsx:15-18`.
- `data?.features ?? []` — already at `Dashboard.tsx:36`.
- React-query cache invalidation on `['features']` — already at `Dashboard.tsx:23`.

---

## 3. Project Structure (where files go)

All changes in the `devteam` repo. Single repo, single UI file + one test file.

```
ui/src/pages/Dashboard.tsx        — MODIFY (add badge in header row)
ui/e2e/count-badge.spec.ts        — CREATE (new e2e spec for the badge)
ui/e2e/app.spec.ts                — NO CHANGE (existing tests remain valid)
internal/api/                     — NO CHANGE (no backend touch)
```

No new components, no new types, no new files under `ui/src/`. The pluralization helper is an inline expression inside `Dashboard.tsx` (see §6 for rationale — extracting a new file for a one-line ternary is over-engineering).

---

## 4. Data Model

No new entities. No state transitions. The badge is a pure derived view.

### Entities (all existing, unchanged)
- **FeatureSummary** (`ui/src/types/index.ts:3-12`): id, title, status, priority, current_phase, updated_at, gate_result, pending_questions_count.
- **FeatureListResponse** (`ui/src/types/index.ts:14-16`): `{ features: FeatureSummary[] }`.
- **CountBadge** (new, UI-only, no persistence): derived value `count = features.length`. Label = `count === 1 ? "1 feature" : "${count} features"`.

### Derivation (the only "logic" in this feature)
```ts
const features: FeatureSummary[] = data?.features ?? [];   // existing, line 36
const count = features.length;                              // new
const label = count === 1 ? "1 feature" : `${count} features`;  // new
```

### Null safety (architectural decision — explicit)
- `data` is `FeatureListResponse | undefined` (react-query: undefined before first successful fetch).
- `data?.features` is `FeatureSummary[] | undefined`.
- `data?.features ?? []` guarantees an array → `.length` is always a number, never NaN/undefined.
- The existing API contract serializes `features` as an array, never `null` (verified: `internal/api/dto.go` initializes with `make([]FeatureSummary, 0, len(features))`; asserted by `TestListFeaturesEmpty`).
- **Therefore**: `count` is always a finite non-negative integer. No `NaN`, no `undefined`, no throw. This satisfies AC-015.

---

## 5. API Contracts

**No new or changed endpoints.** The single existing endpoint this feature reads is documented here as a regression guard (the feature must not break it).

### `GET /api/features` (existing, unchanged)
```
Request: none (GET, no query params, no body)

Response 200:
  {
    "features": FeatureSummary[]   // array, NEVER null; empty array [] when no features
  }

Error responses (existing, unchanged — feature does not touch these):
  500: { "error": "internal_error", "details": "..." }
```

**Quality decision (architectural)**: The `features` field MUST serialize as `[]` (empty array), never `null`. This is already the case (`make([]FeatureSummary, 0, len(...))`). The Tester must verify this is unchanged by this feature (regression guard — AC-012, AC-013). No `omitempty` is introduced anywhere.

---

## 6. Architecture Decisions

### AD-1: Badge placement — Dashboard header row, outside conditional branches
**Decision**: Render the badge inside the existing header `<div className="flex items-center justify-between mb-6">` at `Dashboard.tsx:40`, as a sibling of the `<h2>Features</h2>` and the `+ New Feature` button. NOT inside any of the `isLoading` / `error` / `empty` / `FeatureList` branches.

**Why**: FR-005 requires the badge visible in all states (loading, error, empty, populated). The header row renders unconditionally (it is above all the conditional branches at lines 59-78). Placing the badge there guarantees visibility across states without any conditional logic. This resolves the PM's deferred placement assumption (spec.md line 117).

**Layout**: Insert the badge between the `<h2>` and the button, as a styled `<span>` with a pill/rounded style consistent with existing Tailwind classes. Exact styling is the Developer's discretion but must use existing Tailwind utility classes (no new CSS file, no new dependency).

### AD-2: Pluralization as inline expression, no new file
**Decision**: `const label = count === 1 ? "1 feature" : \`${count} features\`;` inline in `Dashboard.tsx`. No new `CountBadge.tsx` component, no new `pluralize.ts` helper.

**Why**: The logic is one ternary. Extracting a component or helper file for a single one-line expression is over-engineering (overconfidence prevention, Pattern 4 — "is this needed to satisfy a done condition?"). The spec's "Possibly a small helper... inline ternary is fine" (line 145) explicitly allows inline. Conservative: inline.

### AD-3: Badge text during initial loading
**Decision**: Before the first successful fetch, `data` is `undefined`, so `count = 0` and the badge shows "0 features" briefly. This is acceptable per spec.md line 118.

**Rationale**: The spec allows either "0 features" or "Loading...". Showing "0 features" is simpler (no extra conditional, no flicker between "Loading..." and the real count), is null-safe by construction, and matches the error-state behavior (FR-007: "0 features" if never loaded). Consistency > a transient "Loading..." label. The `features-loading` spinner (existing, line 60-64) already signals the loading state, so the badge showing "0 features" during load does not mislead.

### AD-4: E2E tests use `page.route` interception for deterministic counts
**Decision**: The new e2e spec (`ui/e2e/count-badge.spec.ts`) will use Playwright's `page.route('**/api/features', ...)` to mock the `GET /api/features` response with deterministic counts (0, 1, 2, 3, 5). This makes the count assertions deterministic regardless of live DB state.

**Why**: Existing e2e tests in `app.spec.ts` read live state and `test.skip()` when empty — they cannot deterministically assert a specific count. The badge's count assertions (AC-001, AC-002, AC-003, AC-006, AC-008, AC-009) require exact counts, which requires controlled responses. `page.route` is the established Playwright pattern for this; no new infra needed.

**Mock shape**: `page.route('**/api/features', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ features: [...] }) }))` with synthetic `FeatureSummary` objects. The mock must respect the real contract: `features` is an array (possibly empty), never `null`. Error-path tests (AC-008, AC-009) use `route.fulfill({ status: 500, ... })` or `route.abort('failed')`.

### AD-5: No unit test runner added
**Decision**: Do not add Vitest/Jest. AC-014 (pluralization) and AC-015 (null-safe count) are verified at the e2e level via the mocked-count tests (AD-4), which exercise the same inline expression against inputs 0, 1, 2, 3, 5.

**Why**: Introducing a unit test runner for a one-line ternary is unjustified scope expansion. The test selection matrix marks Unit as "YES" (recommended) not "**YES**" (mandatory) for UI components, while E2E is "**YES**" (mandatory). E2E via `page.route` provides real-browser verification of the exact strings "0 features", "1 feature", "2 features", "5 features" — covering AC-014's intent. AC-015 (null-safe derivation) is covered by the error-state e2e (count=0 when data undefined). This is flagged for the Tester so AC-014/AC-015 are not marked unverified.

---

## 7. Component Design

### Component: Dashboard (modified)
**Purpose**: Features list page — renders header, intake form, and state-conditional content. Now also renders a count badge in the header.

**Responsibilities**:
- Fetch features via `useQuery(['features'])` (existing)
- Derive `count = features.length` and `label` (new)
- Render count badge in header (new)
- Render loading / error / empty / list states (existing, unchanged)

**Interfaces**:
- Reads: `useQuery(['features'])` → `data?.features ?? []` → `count` → `label` (all internal, no new props, no new exports)

**Dependencies**:
- `listFeatures` from `../api/client` (existing)
- `FeatureSummary` type (existing)
- Existing react-query cache invalidation on `['features']` (existing)

### Component dependency map
```
Dashboard.tsx
  ├─ useQuery(['features']) → listFeatures (api/client.ts)  [existing]
  ├─ features.length → count → label                        [new, internal]
  └─ <span data-testid="feature-count-badge">{label}</span> [new, inline]
```
No new components, no new files, no circular dependencies, no cross-repo dependencies. Single-file change in source.

---

## 8. Test Strategy

### Component: Dashboard count badge
**Testing levels required**:
- **Smoke**: Frontend builds (`tsc -b && vite build`) with no type errors; dev server starts and the Dashboard renders without runtime errors; `GET /api/features` still returns 200 with `features` array (regression — no backend change).
- **Integration**: `GET /api/features` returns 200 with `{"features": []}` when empty (regression guard, AC-012) and 200 with a populated array when features exist. Existing `TestListFeaturesEmpty` passes unchanged (AC-013). No new integration test needed — this feature adds no backend code; the existing tests are the regression guard.
- **E2E (mandatory for UI)**: New `ui/e2e/count-badge.spec.ts` covering AC-001 through AC-011, AC-014, AC-015 via `page.route` interception:
  - Count 3 → badge text "3 features" (AC-001)
  - Count 1 → badge text "1 feature" singular (AC-002, AC-014)
  - Count 5 → badge text "5 features" plural (AC-003, AC-014)
  - Count 0 → badge text "0 features" + empty-state element present (AC-006, AC-007, AC-014)
  - No console errors/warnings on render (AC-004)
  - Initial-load 500 → badge "0 features" + `features-error` present (AC-008, AC-015)
  - Prior success (count 2) then refetch 500/network-fail → badge still "2 features" + `features-error` present (AC-009)
  - Create via IntakeForm returns 409 → badge unchanged + toast (AC-010)
  - Create via IntakeForm returns 400 → badge unchanged + no console errors (AC-011)
  - Create success → badge increments without reload (AC-005)
- **Unit**: Not added (see AD-5). AC-014/AC-015 intent is covered by the e2e count assertions above. The inline pluralization expression is exercised in a real browser against inputs 0, 1, 2, 3, 5.

**Quality checkpoints (per the planning rules template)**:
- [ ] `npm run build` succeeds with no TypeScript errors (smoke — frontend builds)
- [ ] Dev server starts and Dashboard renders without runtime errors (smoke)
- [ ] `GET /api/features` still returns 200 with `features` as an array, never `null` (regression — integration; AC-012)
- [ ] Existing `go test ./internal/api/ -run TestListFeaturesEmpty` passes unchanged (integration; AC-013)
- [ ] Badge element has `data-testid="feature-count-badge"` and renders in all four states (loading, error, empty, populated) (e2e)
- [ ] Badge text is exactly "3 features" / "1 feature" / "5 features" / "0 features" for mocked counts 3/1/5/0 (e2e; AC-001, AC-002, AC-003, AC-006, AC-014)
- [ ] Badge shows last known count (not 0, not NaN) when a refetch fails after a prior success (e2e; AC-009)
- [ ] Badge shows "0 features" when initial fetch fails (e2e; AC-008, AC-015)
- [ ] No console errors or warnings during any badge render (e2e; AC-004)
- [ ] Badge increments without full page reload after a successful create (e2e; AC-005)
- [ ] Badge does not change when create returns 409 or 400 (e2e; AC-010, AC-011)
- [ ] Empty-state element and badge are both present when count is 0 (e2e; AC-007)

### Test level selection (from planning rules matrix)
| What changed | Smoke | Integration | E2E | Unit |
|---|---|---|---|---|
| Frontend/UI component (Dashboard badge) | **YES** | **YES** (regression) | **YES** | recommended (covered via e2e) |

---

## 9. Non-Functional Requirements

### Performance
- **No new network requests**: count derived from the already-fetched `features` array. Zero added latency, zero added API calls.
- **No re-render risk**: badge re-renders only when `['features']` query data changes (existing behavior). No new subscriptions, no new state, no polling.
- **No memoization needed**: `count` and `label` are trivial derivations computed inline on each render. Premature memoization would be over-engineering.

### Security
- **No new user input**: badge is read-only derived display. No input validation, no XSS surface (count is a number rendered as text; React escapes text content by default).
- **No secrets**: count is a non-sensitive integer. No data classification changes.
- P2 priority — security extension is recommended, not mandatory. No security-specific acceptance criteria needed (no auth, no input, no sensitive data). The existing `GET /api/features` auth posture (if any) is unchanged.

### Scalability
- **Not applicable**: count is `O(1)` on the already-loaded array. No new data structures. Works identically with 0 or 100,000 features (the array is already in memory for the list render).

### Reliability
- **Null-safe by construction** (AD-4): `data?.features ?? []` → `.length` is always a number. No panic/throw path in the badge render.
- **Error-state degradation**: on query error, badge shows last known count or "0 features" (FR-007). This is graceful degradation, not a failure.
- **No retry/circuit-breaker needed**: the feature adds no I/O. Existing react-query retry config (if any) governs the `['features']` query; unchanged.

---

## 10. Quality Gate Self-Check (Architect's planning-phase overconfidence checks)

- [x] Does every task have done conditions with specific verifiable assertions? (see tasks.md)
- [x] Does every component have a test strategy section? (§8)
- [x] Have I identified agent failure mode checks for each task? (see tasks.md, each task)
- [x] Have I considered what happens when each external dependency fails? (§8 — error-state e2e covers query failure; backend unchanged so no new failure modes)
- [x] Is my implementation plan the minimum needed, or am I over-engineering? (AD-2, AD-5 — inline ternary, no new test runner; single-file source change)

### Overconfidence prevention (planning checks)
- [x] Empty state behavior: defined (count 0 → "0 features", badge + empty-state both visible)
- [x] Large result sets: not applicable (count is O(1) on loaded array; no pagination needed — explicitly out of scope per spec)
- [x] Filtering/sorting: out of scope, unchanged
- [x] Error responses: defined (500 on initial load → "0 features"; 500 on refetch → last known count)
- [x] Concurrent access: not applicable (read-only derived view; react-query handles cache)

---

## 11. Quickstart Guide for the Developer

1. **Read first**: `specs/spec-count-badge/spec.md`, `specs/spec-count-badge/acceptance.md`, this `plan.md`, and `specs/spec-count-badge/tasks.md`.
2. **Read the code**: `ui/src/pages/Dashboard.tsx` (the only file you modify), `ui/src/api/client.ts` (reference — do not change), `ui/src/types/index.ts` (reference — `FeatureSummary`, `FeatureListResponse`).
3. **Implement in dependency order** (see tasks.md): T-001 (badge in Dashboard) before T-002 (e2e tests). T-001 and T-002 are the only two tasks; T-003 is the verification gate.
4. **Build check after T-001**: `cd ui && npm run build` must pass with zero TypeScript errors. Do not proceed to T-002 if the build fails — fix the compile error, do not rewrite.
5. **E2E check after T-002**: `cd ui && npx playwright test e2e/count-badge.spec.ts` must pass all assertions. Existing tests (`app.spec.ts`) must still pass.
6. **Self-verify (proof of work, not claims)**:
   - Start the dev server, open the Dashboard, confirm the badge renders in the header next to "Features".
   - Create a feature via the IntakeForm, confirm the badge increments without a reload.
   - Check the browser console — zero errors, zero warnings.
   - Quote the exact badge text for counts 0, 1, 2, 3, 5 (use devtools or a temporary mock).
7. **Do NOT**: add a new component file, add a new helper file, add a new npm dependency, add Vitest/Jest, change any backend file, change `client.ts`, change any types, add a `total_count` field. All of these are out of scope (overconfidence prevention, Pattern 4).

---

## 12. Open Questions

None unresolved. The two PM-deferred placement/loading-text decisions are resolved by AD-1 and AD-3. The unit-test-runner question is resolved by AD-5 and flagged for the Reviewer/Tester.