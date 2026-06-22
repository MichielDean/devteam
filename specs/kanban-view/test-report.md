# Test Report — kanban-view

**Feature**: kanban-view
**Phase**: testing
**Tester role**: Dev Team Tester
**Date**: 2026-06-22
**Implementation worktree**: `~/source/devteam/worktrees/kanban-view/devteam` (branch `feature/kanban-view`)
**Spec repo worktree**: `~/worktrees/devteam-specs/kanban-view` (branch `spec/kanban-view`)

---

## Outcome

**ALL TESTS PASS.** 31 e2e (Playwright) + 4 unit (vitest) + 7 Go smoke/integration = **42 tests green, 0 failures.** 3 e2e tests skipped (workspace-state-dependent empty/detail assertions that guard their own skip condition).

No recirculate findings against the current implementation. One drift finding was discovered and **fixed during testing** (see §1, Finding F-1) — the pre-existing `kanban.spec.ts` / `kanban-api.spec.ts` described a *different* implementation than what shipped. Those tests were rewritten to match the spec and the shipped code.

---

## 1. Spec-Implementation Drift Verification

Compared `specs/kanban-view/spec.md` (PM) + `acceptance.md` (AC-001..AC-022) + `plan.md` (Architect) against the shipped code in `~/source/devteam/worktrees/kanban-view/devteam/ui/src/`.

### F-1 (FIXED): Pre-existing test suite described a phantom implementation

**Finding.** The `kanban.spec.ts` and `kanban-api.spec.ts` files present in the worktree at testing-phase start did NOT match the shipped implementation or the spec:

| Aspect | Pre-existing test asserted | Spec / shipped impl |
|---|---|---|
| Column set | 7 columns incl. `backlog` | 6 phase columns (`PHASES`) + conditional `other` (FR-005, FR-007) |
| Card testid | `feature-card-${id}` | `kanban-card-${id}` (FR-010, plan §KanbanCard) |
| Default view | List (tests called `switchToBoard` from `/`) | Board (FR-003, AC-005) |
| Error testid | `kanban-error` | `features-error` (FR-017, existing Dashboard branch reused) |
| Grouping | status-based (`draft`→backlog) | phase-based via `current_phase` (FR-006) |

The pre-existing tests would have **all failed** against the shipped code. They appear to have been written against an imagined "status swimlane + backlog" design that the spec explicitly rejected (spec Assumptions: "Columns are the six phases, not statuses. … Swimlane-per-status is out of scope").

**Resolution.** Rewrote `ui/e2e/kanban.spec.ts` from scratch, tracing every acceptance criterion (AC-001..AC-022) to a named test against the real implementation selectors. Deleted `ui/e2e/kanban-api.spec.ts` (its single-fetch + error-path concerns are now covered by `[T015]` AC-016 and `[T014]` AC-015 in `kanban.spec.ts`, and by the pre-existing API tests in `app.spec.ts`). This is a test-only fix; no implementation code was touched by the Tester.

### F-2 (FIXED): app.spec.ts broken by Board-default (CON-004 regression)

**Finding.** `app.spec.ts` list-view tests asserted `[data-testid*="feature-card"]` on `/` directly. With Board now the default (FR-003), the board renders `kanban-card-*` not `feature-card-*`, so those assertions see 0 cards. The spec's CON-004 traceability note explicitly called this out: *"app.spec.ts may need a click-to-List fixture if it asserts the default — architect to verify."*

**Resolution.** Added a `switchToList(page)` helper that clicks `view-toggle-list` (guarded by visibility, so empty-state tests are unaffected) before list-view assertions. Applied to: "feature list loads and shows features", "feature list handles empty state", "feature detail page renders correctly", "phase progress indicators render". Also fixed a pre-existing dead assertion (`text=Pipeline Progress`) that has never existed in `FeatureDetail.tsx` on `main` — replaced with `[data-testid="feature-detail-page"]` + `h1`.

### Drift checks performed (no findings beyond F-1/F-2)

- **PM → Architect**: every US-001..US-004 addressed by plan tasks. ✓
- **Architect → Developer**: every plan component (`KanbanBoard`, `KanbanColumn`, `KanbanCard`, `ViewToggle`, `useSessionView`, `badgeColors`, `groupFeaturesByPhase`) exists at the planned path. ✓
- **Developer → Tester**: every AC-001..AC-022 now has a test (see §6 traceability). ✓
- **Frontend-Backend contract**: board consumes `useQuery(['features'])` only; `[T015]` AC-016 verifies exactly one `GET /api/features` request. ✓
- **Constraint register**: CON-001..CON-009 each have a test (see §7). ✓

---

## 2. Test Infrastructure Discovered

| Command | Purpose | Location |
|---|---|---|
| `npm run test:e2e` | Playwright e2e (`ui/e2e/*.spec.ts`) | `ui/package.json` |
| `npm run test:unit` | Vitest unit (`src/**/*.test.ts`) — **added this phase** | `ui/package.json` |
| `npm run build` | `tsc -b && vite build` (type-check + bundle) | `ui/package.json` |
| `go test ./internal/api/... -run TestKanban` | Go smoke/integration for backend contract | `internal/api/kanban_smoke_test.go` |
| `go build -o ~/go/bin/devteam ./cmd/devteam` | Build server binary for Playwright `webServer` | repo root |

**Playwright config** (`ui/playwright.config.ts`): `webServer` launches `devteam` binary; `SERVER_PORT` env var selects the port. Per CON-001 the spec mandates `:18765` (never `:8765`). All e2e in this report ran with `SERVER_PORT=18765 START_SERVER=1 BASE_URL=http://localhost:18765`.

**Vitest added as devDependency** (plan §Testing decision). `package.json` `dependencies` block unchanged (CON-003); `devDependencies` adds `vitest@^4.1.9` only. `vite.config.ts` gains a `test` block (excludes `e2e/**`) so vitest doesn't pick up Playwright specs.

---

## 3. Smoke Test Results (Level 1)

### 3a. Backend smoke — Go

Command: `cd ~/source/devteam/worktrees/kanban-view/devteam && go test ./internal/api/... -run TestKanban -count=1 -v`

Result: **7 passed, 0 failed.** (`Go test: 7 passed in 1 packages`)

Tests run (file `internal/api/kanban_smoke_test.go`):
- `TestKanbanSmokeEmptyFeaturesArrayNotNull` — `GET /api/features` on empty system returns `"features":[]` not `"features":null`. Status 200. `total_count: 0`. (CON-004 #1 agent-generated bug pinned at backend.)
- `TestKanbanSmokeTerminalStatusFeaturesStayInPhaseColumn` — a `done`+`delivery` feature is returned by `GET /api/features` with `current_phase=delivery` intact (not filtered). Seeds via `SpecProvider`, asserts list contains it with status `done` + phase `delivery`.
- `TestKanbanSmokeNoKanbanSpecificEndpoint` — `/api/kanban`, `/api/kanban/features`, `/api/board`, `/api/features/kanban` all return 4xx (no new route added — CON-003/AC-CON-003). 4 subtests.

### 3b. Frontend smoke — UI build + e2e load

Command: `cd ui && npm run build`

Result: **✓ built in 2.48s, 479 modules transformed.** `tsc -b` passes (no type errors → no phantom method calls, no missing imports). Bundles: `index-*.js` 239 KB, `vendor-*` chunks unchanged.

Command: `SERVER_PORT=18765 START_SERVER=1 BASE_URL=http://localhost:18765 npm run test:e2e`

Result: server started on `:18765`, every e2e test loaded `/` in Chromium without `pageerror`. No nil-pointer-equivalent (no uncaught exceptions). See §5 for per-test evidence.

### 3c. Console-error check (SC-004)

Every kanban e2e test installs `page.on('pageerror')` and `page.on('console', … 'error')` and asserts both empty (after filtering browser-level `Failed to load resource` network spam from intentional 500 responses). All 22 kanban tests pass the no-console-errors assertion.

---

## 4. Unit Test Results (Level 4)

Command: `cd ui && npm run test:unit`
Output: `Test Files 1 passed (1) — Tests 4 passed (4) — Duration 169ms`

File: `ui/src/components/groupFeaturesByPhase.test.ts` (vitest). Tests the pure `groupFeaturesByPhase` function exported from `KanbanBoard.tsx` → `groupFeaturesByPhase.ts`.

| Test | AC | Assertion |
|---|---|---|
| empty input → all buckets `[]` not null | AC-011, CON-008 | `for p of PHASES: Array.isArray(g[p]) && g[p].length === 0`; `g.other` likewise |
| partition invariant sum === input.length | FR-006, SC-002 | 4 mixed features → sum of all 7 bucket lengths === 4; `planning` holds 2 |
| unknown phase → `other`, no crash, no drop | AC-011, CON-009 | `current_phase:'not-a-real-phase'` → `g.other[0].id==='weird'`; `g.review.length===1`; sum invariant holds |
| one feature per known phase, none dropped/duplicated | SC-002 | one feature in each of the 6 phases → each bucket length 1, `other` length 0 |

**Agent failure mode coverage**: CON-008 (empty buckets `[]` never `null`) is explicitly asserted. The partition invariant (SC-002: zero features dropped or duplicated) is asserted twice. Unknown-enum defensiveness (CON-009) asserted with a synthetic `'not-a-real-phase'` string.

---

## 5. E2E Test Results (Level 3)

Command: `SERVER_PORT=18765 START_SERVER=1 BASE_URL=http://localhost:18765 npm run test:e2e`
Result (final stable run): **31 passed, 3 skipped, 0 failed.** (6.7s)

Skipped tests are workspace-state-dependent (`test.skip(count > 0, …)` / `if (!firstFeature.isVisible()) test.skip()`); they self-skip when the live workspace doesn't meet their precondition. No failure is hidden behind a skip.

### kanban.spec.ts — 22 tests, one per AC

| Test ID | US | AC | Type | Scenario | Result |
|---|---|---|---|---|---|
| T001 | US-001 | AC-001 | e2e | toggle visible, Board `aria-pressed=true` by default, list absent | ✓ |
| T002 | US-001 | AC-002 | e2e | click Board → 6 phase columns in `PHASES` order, headers match `PHASE_LABELS`, list gone | ✓ |
| T003 | US-001 | AC-003 | e2e | click List → `feature-list` visible, zero `kanban-column-*` | ✓ |
| T004 | US-001 | AC-004 | e2e | List→Board→reload → Board still active; `sessionStorage.getItem('devteam.dashboard.view')==='board'` | ✓ |
| T005 | US-001 | AC-005 | e2e | fresh browser context → Board default, no prior sessionStorage | ✓ |
| T006 | US-001 | AC-006 | e2e | `features:[]` → `view-toggle` count 0, "No features" visible, board count 0 | ✓ |
| T007 | US-002 | AC-007 | e2e | `current_phase:'planning'`, priority 1, `in_progress` → card in planning column, `kanban-card-priority`="P1 - Critical", `kanban-card-status`="In Progress" | ✓ |
| T008 | US-002 | AC-008 | e2e | `pending_questions_count:3` → `question-badge` visible, text "3" | ✓ |
| T009 | US-002 | AC-009 | e2e | `gate_result.passed:true` → "✓ Gate passed"; `passed:false` → "✗ Gate failed" | ✓ |
| T010 | US-002 | AC-010 | e2e | click `kanban-card-nav1` → URL `/features/nav1` | ✓ |
| T011 | US-002 | AC-012 | e2e | `status:'gate_blocked'` → card class contains `ring-red` | ✓ |
| T012 | US-002 | AC-013 | e2e | `status:'waiting_for_human'` → card class contains `ring-yellow` | ✓ |
| T013 | US-002 | AC-014 | e2e | delayed `/api/features` → `features-loading` visible, 0 columns | ✓ |
| T014 | US-002 | AC-015 | e2e | `/api/features` 500 → `features-error` visible, 0 columns, 0 board, no pageerror | ✓ |
| T015 | US-002 | AC-016 | integration | `page.on('request')` count for `/api/features` === 1 during Board render (CON-007) | ✓ |
| T016 | US-003 | AC-017 | e2e | all features in planning → testing column header visible, `kanban-column-empty-testing` contains "No features", 0 cards | ✓ |
| T017 | US-003 | AC-018 | e2e | zero features → `view-toggle` count 0, "No features" visible (cross-ref AC-006) | ✓ |
| T018 | US-003 | AC-019 | e2e | one feature in planning → exactly 6 `kanban-column-*`, each phase present once, `kanban-column-other` count 0 | ✓ |
| T019 | US-002 | AC-011→e2e | e2e | `current_phase:'not-a-real-phase'` → `kanban-column-other` count 1, card visible inside it, 7 total columns (CON-009 end-to-end) | ✓ |
| T020 | US-004 | AC-020 | e2e | 50 features in planning, 600px viewport → column `scrollHeight > clientHeight`, `document.body.scrollTop === 0` | ✓ |
| T021 | US-004 | AC-021 | e2e | 400px viewport → column body `clientHeight <= 400` | ✓ |
| T022 | US-004 | AC-022 | e2e | 600px viewport → board `overflowX` ∈ {auto, scroll}, first column `getBoundingClientRect().width >= 240` | ✓ |

### app.spec.ts — regression (CON-004)

12 tests, all pass (3 skip on workspace state). The 4 list-view tests now call `switchToList(page)` first. API-shape tests (`API returns valid JSON with arrays not null`, `API 404`, `API 400`, count-badge tests) unchanged and green.

---

## 6. Acceptance-Criterion Traceability

Every AC in `acceptance.md` has ≥1 test:

| AC | Test(s) | Level |
|---|---|---|
| AC-001 | T001 | e2e |
| AC-002 | T002 | e2e |
| AC-003 | T003 | e2e |
| AC-004 | T004 | e2e |
| AC-005 | T005 | e2e |
| AC-006 | T006, T017 | e2e |
| AC-007 | T007 | e2e |
| AC-008 | T008 | e2e |
| AC-009 | T009 | e2e |
| AC-010 | T010 | e2e |
| AC-011 | unit (groupFeaturesByPhase.test.ts "unknown phase") + T019 | unit + e2e |
| AC-012 | T011 | e2e |
| AC-013 | T012 | e2e |
| AC-014 | T013 | e2e |
| AC-015 | T014 | e2e |
| AC-016 | T015 | integration |
| AC-017 | T016 | e2e |
| AC-018 | T017 (cross-ref AC-006) | e2e |
| AC-019 | T018, T019 | e2e |
| AC-020 | T020 | e2e |
| AC-021 | T021 | e2e |
| AC-022 | T022 | e2e |

No AC is untested.

---

## 7. Constraint Register Verification

| CON | Constraint | Test | Result |
|---|---|---|---|
| CON-001 | e2e on `:18765` never `:8765` | All e2e run with `SERVER_PORT=18765` | ✓ |
| CON-002 | new components under `ui/src/components/`, hook under `ui/src/hooks/` | file-path check: `KanbanBoard.tsx`, `KanbanCard.tsx`, `KanbanColumn.tsx`, `ViewToggle.tsx`, `badgeColors.ts`, `groupFeaturesByPhase.ts` in `components/`; `useSessionView.ts` in `hooks/` | ✓ (verified by inspection) |
| CON-003 | no new runtime npm dep | `package.json` `dependencies` block unchanged; `vitest` is devDep only; `TestKanbanSmokeNoKanbanSpecificEndpoint` asserts no new backend route | ✓ |
| CON-004 | existing `feature-card-*` / `feature-count-badge` tests still pass | `app.spec.ts` green after `switchToList` fixture; count-badge tests unchanged | ✓ |
| CON-005 | board reuses `PHASES`/`PHASE_LABELS`/`STATUS_LABELS`/`PRIORITY_LABELS` | T002 asserts column headers match `PHASE_LABELS`; T007 asserts badge text from `STATUS_LABELS`/`PRIORITY_LABELS` | ✓ |
| CON-006 | card chrome parity with `FeatureCard` | `badgeColors.ts` shared module imported by both; T007/T008/T009 assert same badge/gate text | ✓ |
| CON-007 | single `GET /api/features` | T015 (AC-016) — request count === 1 | ✓ |
| CON-008 | empty/loading/error states covered | T006/T013/T014/T016/T017 + unit "empty buckets `[]`" | ✓ |
| CON-009 | unknown `current_phase` defensive | unit "unknown phase → other" + T019 end-to-end | ✓ |

---

## 8. Agent Failure Mode Verification

| Failure mode | Check | Evidence |
|---|---|---|
| Nil pointer chains | start server, hit endpoints | Go smoke tests start `httptest.NewServer(s.httpServer.Handler)` and hit `/api/features` + 4 kanban paths; no panic. e2e loads `/` with no `pageerror`. |
| Null vs empty arrays | `features` field `[]` not `null` | `TestKanbanSmokeEmptyFeaturesArrayNotNull` asserts body contains `"features":[]` and NOT `"features":null`. Unit test asserts every `groupFeaturesByPhase` bucket is `[]` not `null`. |
| Phantom method calls | code compiles AND runs | `tsc -b` clean; `vite build` clean (479 modules); e2e runs the real bundle against the real binary. |
| Over-engineering | line-count smell | `KanbanBoard.tsx` 34 lines, `KanbanColumn.tsx` 37, `KanbanCard.tsx` 76, `ViewToggle.tsx` 34, `useSessionView.ts` 31, `badgeColors.ts` 13, `groupFeaturesByPhase.ts` 32. Total ~257 lines for the whole feature. Test suite (`kanban.spec.ts` 447 + unit 71) is larger than the impl — healthy ratio. |
| Missing error paths | 400/404/409/empty/malformed | T006 (empty), T013 (loading), T014 (error 500); `app.spec.ts` retains `API 404`, `API 400`, `feature count badge absent on API error`. Backend 404 for missing feature is covered by `app.spec.ts:114`. |
| Constraint violations | every CON has a test | §7 — all 9 CONs covered. |
| Multi-component inconsistency | constraints across all consumers | `badgeColors.ts` consumed by `FeatureCard` + `KanbanCard`; T007 verifies board card badge text matches the same `STATUS_LABELS`/`PRIORITY_LABELS` the list view uses. |
| Language-specific footguns | TS edge cases | Unit test passes a synthetic `'not-a-real-phase'` string (CON-009 enum drift) and an empty array (CON-008 null-vs-empty); both handled. `useSessionView` wraps `sessionStorage` access in try/catch (private-mode quota throw) — verified by inspection, e2e T004/T005 exercise the happy path. |

---

## 9. Exact Commands to Reproduce

```bash
# From the implementation worktree root:
cd ~/source/devteam/worktrees/kanban-view/devteam

# 1. Build the Go binary (Playwright webServer uses it)
export PATH=$PATH:/usr/local/go/bin:~/go/bin
go build -o ~/go/bin/devteam ./cmd/devteam

# 2. Go smoke/integration tests (Level 1)
go test ./internal/api/... -run TestKanban -count=1 -v

# 3. UI type-check + build (phantom-method / nil-deref check at compile time)
cd ui
npm run build

# 4. Unit tests (Level 4) — AC-011, CON-008, CON-009, SC-002
npm run test:unit

# 5. E2E (Level 3) — AC-001..AC-022, CON-001/004/005/006/007
#    SERVER_PORT=18765 honors CON-001. START_SERVER=1 forces a fresh binary.
SERVER_PORT=18765 START_SERVER=1 BASE_URL=http://localhost:18765 npm run test:e2e
```

---

## 10. Files Changed by the Tester

All in the implementation worktree (`~/source/devteam/worktrees/kanban-view/devteam`):

| File | Change |
|---|---|
| `ui/e2e/kanban.spec.ts` | **Rewritten** — 22 tests tracing AC-001..AC-022 against the shipped implementation selectors (was: phantom 7-column/backlog design that would have failed) |
| `ui/e2e/kanban-api.spec.ts` | **Deleted** — redundant with `kanban.spec.ts` T014/T015 + `app.spec.ts` API tests |
| `ui/e2e/app.spec.ts` | **Modified** — added `switchToList` helper (CON-004); applied to 4 list-view tests; replaced dead `text=Pipeline Progress` assertion with `feature-detail-page` testid |
| `ui/src/components/groupFeaturesByPhase.test.ts` | **Created** — vitest unit test for `groupFeaturesByPhase` (AC-011, CON-008, CON-009, SC-002) |
| `ui/vite.config.ts` | **Modified** — added `test` block (vitest config, excludes `e2e/**`) + `/// <reference types="vitest/config" />` |
| `ui/package.json` | **Modified** — added `vitest` devDep, `test:unit` script |
| `ui/package-lock.json` | **Modified** — vitest install |

No implementation source files (`KanbanBoard.tsx`, `KanbanCard.tsx`, `KanbanColumn.tsx`, `ViewToggle.tsx`, `useSessionView.ts`, `badgeColors.ts`, `groupFeaturesByPhase.ts`, `Dashboard.tsx`, `types/index.ts`) were modified by the Tester. No Go files modified. No spec artifacts (`spec.md`, `acceptance.md`, `plan.md`, `tasks.md`) modified.

---

## 11. Quality Gate

| Gate criterion | Status |
|---|---|
| Conformance tests pass | N/A — no RFC/standard; constraint register is internal conventions, all covered (§7) |
| `go test ./...` passes | ✓ (kanban subset: 7 passed) |
| `npm test` / `npx playwright test` passes | ✓ (31 passed, 3 skipped, 0 failed) |
| Smoke tests pass: service starts, endpoints respond | ✓ (Go httptest + Playwright webServer on :18765) |
| Integration tests pass: full HTTP cycles, JSON shapes `[]` not null | ✓ (`TestKanbanSmokeEmptyFeaturesArrayNotNull`, T015 single-fetch, app.spec.ts API-shape test) |
| E2E tests pass: frontend loads, renders, no console errors | ✓ (22 kanban + 12 app tests, every test asserts `pageErrors===[]`) |
| State machine verified | N/A — feature is view-only, no state transitions (spec: "Drag-and-drop out of scope. The board is view-only.") |
| Spec drift checked: every US has a test | ✓ (§1, §6) |
| Every constraint has ≥1 test | ✓ (§7) |
| Every AC has ≥1 test | ✓ (§6) |
| No nil pointer panics, no null-vs-empty mismatches | ✓ (§8) |
| Multi-component constraints tested across all components | ✓ (`badgeColors` in both `FeatureCard` + `KanbanCard`, T007) |
| Language-specific footguns tested | ✓ (unknown-enum string, empty array, sessionStorage try/catch) |

**Gate: PASS.** No recirculate findings against the shipped implementation. The two drift findings (F-1, F-2) were against the *test suite itself* and were fixed during this phase.