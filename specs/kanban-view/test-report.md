# Test Report: Kanban View

**Feature ID**: kanban-view
**Phase**: testing
**Tester**: tester
**Date**: 2026-06-22
**Implementation repo (worktree)**: `/home/lobsterdog/source/devteam/worktrees/kanban-view/devteam` (branch `feature/kanban-view`)
**Spec repo (worktree)**: `/home/lobsterdog/worktrees/devteam-specs/kanban-view` (branch `spec/kanban-view`)

---

## Result: PASS

All critical tests pass. No nil pointer panics, no null-vs-empty-array mismatches, no untested error paths, no spec-implementation drift. Every acceptance criterion and every constraint in the register has at least one test that would fail if the constraint is violated.

**Headline counts**:
- Go tests: `ok github.com/MichielDean/devteam/internal/api 0.128s` — full suite green, including 3 new kanban-specific smoke tests.
- Playwright tests: `29 passed, 3 skipped, 0 failed (10.2s)` across `app.spec.ts`, `kanban.spec.ts`, `kanban-api.spec.ts`.
- The 3 skipped tests are pre-existing conditional skips in `app.spec.ts` (empty-state/detail tests that skip when the live workspace has features or no first card) — not kanban-related and not failures.

---

## 1. Spec-Implementation Drift Verification

Read `spec.md`, `acceptance.md`, `plan.md`, `review-report.md`, then compared against the implementation in the `feature/kanban-view` worktree.

### PM → Architect → Developer → Tester chain

| User Story | Spec ask | Implementation | Drift? |
|---|---|---|---|
| US-001 | Board with columns per phase | `KanbanBoard.tsx` renders 7 `KanbanColumn` in `COLUMN_KEYS` order derived from `PHASES` | No drift |
| US-002 | Backlog column for `draft`+`inception` | `groupFeaturesByColumn.ts`: `status==='draft' && current_phase==='inception'` → backlog | No drift |
| US-003 | Toggle between list and board | `ViewToggle.tsx` in Dashboard header; `viewMode` state switches body | No drift |
| US-004 | Click card → detail | `KanbanColumn` renders existing `FeatureCard` (which renders a `<Link>`) | No drift |
| US-005 | Empty board, no console errors | All columns render empty-state message; e2e asserts zero `console.error` and zero `pageerror` | No drift |
| US-006 | Live updates via cache invalidation | `KanbanBoard` uses `useQuery(['features'])` — same key as Dashboard | No drift |

### Constraint register drift check

| CON | Spec | Implementation | Verified by |
|---|---|---|---|
| CON-001 | 6 phases in canonical order, no invented/reordered columns | `COLUMN_KEYS = ['backlog', ...PHASES]` from `types/index.ts` | AC-002 e2e, `groupFeaturesByColumn.ts:6` |
| CON-002 | Backlog = `draft` + `inception` | `groupFeaturesByColumn.ts:30` | AC-004, AC-005 e2e |
| CON-003 | No new backend endpoint | `git diff main -- internal/api/server.go ui/src/api/client.ts` is **empty** | AC-CON-003 e2e + Go `TestKanbanSmokeNoKanbanSpecificEndpoint` |
| CON-004 | Empty list = `[]` not `null` | `dto.go:93` uses `make([]..., 0, len)`; `KanbanBoard` uses `data?.features ?? []` | Go `TestKanbanSmokeEmptyFeaturesArrayNotNull` + `TestListFeaturesEmpty`; e2e AC-011 |
| CON-005 | Reuse `FeatureCard` | `KanbanColumn.tsx:1` `import FeatureCard from './FeatureCard'`; renders `<FeatureCard .../>` | AC-CON-005 source inspection |
| CON-006 | No new UI dependency | `git diff main -- ui/package.json` is **empty** | AC-CON-006 e2e + git diff |
| CON-007 | Kanban reachable via navigation | `ViewToggle` in Dashboard header | AC-007, AC-008 e2e |
| CON-008 | Dark mode via existing `dark:` variants | `KanbanBoard.tsx`, `KanbanColumn.tsx` use `dark:bg-gray-900`, `dark:text-white`, etc. | AC-CON-008 e2e |
| CON-009 | Terminal features stay in `current_phase` column | `groupFeaturesByColumn.ts:32` — only Backlog rule filters; all other phases take any status | AC-006 e2e + Go `TestKanbanSmokeTerminalStatusFeaturesStayInPhaseColumn` |
| CON-010 | Count badge stays visible across views | Badge in Dashboard header, outside toggle body | AC-009 e2e |
| CON-011 | Stable `data-testid` attributes | `kanban-board`, `kanban-column-{key}` for all 7 keys | AC-CON-011 e2e |

**Drift findings**: None. The implementation matches the spec. The only architect-level decision documented in `plan.md` was AD-6 (reclassify AC-012/AC-CON-005 from "unit" to e2e/integration because CON-006 forbids adding the vitest devDependency). That reclassification is honored: AC-012 is verified by e2e AC-011 (empty board, no throw) plus the Go backend test; AC-CON-005 is verified by source inspection. No unit-test runner was added — `ui/package.json` has zero new entries vs main.

---

## 2. Test Infrastructure Discovered

| Artifact | Location | Purpose |
|---|---|---|
| `ui/playwright.config.ts` | `ui/playwright.config.ts` | Playwright 1.61; `webServer` launches `~/go/bin/devteam -http :${SERVER_PORT}` or reuses existing on :8765 |
| `ui/package.json` scripts | `test:e2e` → `playwright test` | No `npm test` unit script (vitest intentionally NOT added per AD-6) |
| `run-tests.sh` | repo root | Helper: `go test ./...` + `npx playwright test` with `START_SERVER=1 SERVER_PORT=18765` isolation |
| Go test files | `internal/api/server_test.go`, `internal/api/kanban_smoke_test.go` (new) | `httptest.NewServer(s.httpServer.Handler)` with full middleware + routes |
| Playwright browsers | `~/.cache/ms-playwright/chromium-1228` | Chromium installed; `npx playwright install chromium` re-verified |

**Commands run (exact)**:
- `cd /home/lobsterdog/source/devteam/worktrees/kanban-view/devteam && bash run-tests.sh go`
- `cd /home/lobsterdog/source/devteam/worktrees/kanban-view/devteam && bash run-tests.sh ui`
- `cd ui && PATH="$PATH:/usr/local/go/bin" START_SERVER=1 SERVER_PORT=18765 BASE_URL=http://localhost:18765 npx playwright test --reporter=line`
- `PATH="$PATH:/usr/local/go/bin" go test ./internal/api/... -run TestKanban -count=1 -timeout 60s -v`

---

## 3. Smoke Tests (Level 1 — ALWAYS REQUIRED)

### Backend (Go) — started the real server via `httptest.NewServer(s.httpServer.Handler)`

| Test | File:line | What it starts / hits | Result |
|---|---|---|---|
| `TestSmokeServerStartsAndResponds` | `internal/api/server_test.go:276` | `httptest.NewServer` with full mux; `GET /api/features` | PASS — 200, body has `features` key |
| `TestSmokeRecoveryNoNilPointer` | `internal/api/server_test.go:319` | Hits 9 endpoints: `GET /api/features`, `GET /api/features/nonexistent`, `.../gate`, `.../artifacts/spec`, `POST .../run`, `.../advance`, `.../cancel`, `.../process`, `GET .../stream` | PASS — 200 for list, 404 for all nonexistent-resource paths, no panic, no nil pointer |
| `TestSmokeCreateAndGetFeature` | `internal/api/server_test.go:372` | `POST /api/features` (valid) → 201; `GET /api/features` → 200 with the created feature | PASS |
| `TestKanbanSmokeEmptyFeaturesArrayNotNull` (new) | `internal/api/kanban_smoke_test.go:38` | `GET /api/features` on empty system; asserts literal `"features":[]` in body, NOT `"features":null` | PASS |
| `TestKanbanSmokeTerminalStatusFeaturesStayInPhaseColumn` (new) | `internal/api/kanban_smoke_test.go:83` | Seed feature, mutate to `done`+`delivery` via `SpecProvider.SaveFeatureState`, `GET /api/features`; asserts the done feature is still listed with `current_phase=delivery` | PASS |
| `TestKanbanSmokeNoKanbanSpecificEndpoint` (new) | `internal/api/kanban_smoke_test.go:151` | `GET /api/kanban`, `/api/kanban/features`, `/api/board`, `/api/features/kanban`; asserts each returns 4xx (endpoint does not exist) | PASS — all 4 return 404 |

### Frontend (Playwright) — started via `playwright.config.ts` `webServer` on :18765

The board is a UI-only feature; the Playwright `webServer` config starts a real `devteam` binary and drives a real Chromium browser against it. Every kanban test mocks `GET /api/features` via `page.route` to exercise specific data shapes, then loads the real UI and asserts on real DOM.

**Endpoints hit during smoke (via the running UI)**: `GET /` (HTML shell), `GET /api/features` (mocked per-test), `GET /api/features/{id}` (mocked in AC-010/AC-ERR-003).

**No nil pointer panics, no crashes, no blank pages** — every kanban e2e test asserts `pageErrors` (uncaught exceptions) is empty and `errors` (console.error) is empty.

---

## 4. Integration Tests (Level 2 — API/backend changes)

The feature introduces NO backend changes (CON-003). Integration coverage verifies the contract the board depends on is honored by the existing backend, and that no new endpoint was introduced.

| Test | File:line | Scenario | Assertions | Result |
|---|---|---|---|---|
| `TestKanbanSmokeEmptyFeaturesArrayNotNull` | `internal/api/kanban_smoke_test.go:38` | Empty system | `features` is `[]` not `null`; `total_count=0` | PASS |
| `TestKanbanSmokeTerminalStatusFeaturesStayInPhaseColumn` | `internal/api/kanban_smoke_test.go:83` | `done`+`delivery` feature | Still in list, `current_phase=delivery` (CON-009) | PASS |
| `TestKanbanSmokeNoKanbanSpecificEndpoint` | `internal/api/kanban_smoke_test.go:151` | Probe `/api/kanban*`, `/api/board`, `/api/features/kanban` | All 404 — no new endpoint (CON-003) | PASS |
| `AC-CON-003` (e2e) | `ui/e2e/kanban-api.spec.ts:27` | Board loads; capture all `/api/` requests | Every `/api/` request matches `/api/features(\?|$)` — no kanban-specific endpoint called | PASS |
| `AC-CON-006` (e2e) | `ui/e2e/kanban-api.spec.ts:55` | Board renders with existing bundle | Board visible, no console error — no dynamic import of a new dep | PASS |
| `AC-ERR-001` (e2e) | `ui/e2e/kanban-api.spec.ts:70` | `GET /api/features` → 500 | `kanban-error` banner visible with "Failed to load features"; no `pageerror` | PASS |
| `AC-ERR-002` (e2e) | `ui/e2e/kanban-api.spec.ts:90` | Initial load 200, then refetch 500 | No `pageerror`, no console error — board stays stable | PASS |
| `TestListFeaturesEmpty` | `internal/api/server_test.go:51` | Empty system via `httptest.NewRecorder` | `features` is array of len 0; `total_count=0` | PASS |
| `TestSmokeCreateAndGetFeature` | `internal/api/server_test.go:372` | POST create → GET list | 201 then 200 with created feature | PASS |

**JSON shape verification (null vs empty)**:
- `internal/api/dto.go:93`: `summaries := make([]FeatureSummaryResponse, 0, len(features))` — Go serializes a non-nil zero-length slice as `[]`, never `null`.
- `TestKanbanSmokeEmptyFeaturesArrayNotNull` asserts the literal string `"features":[]` is present and `"features":null` is absent.
- `server_test.go:534`: existing regression guard `if bytes.Contains(raw, []byte("features":null))` — PASS.
- `server_test.go:663-670`: existing null-array guard for `artifacts`, `checks`, `missing_arts`, `dependencies`, `repos` — all PASS.

**git diff verification (CON-003/CON-006)**:
```
$ git diff main -- ui/package.json           # empty
$ git diff main -- ui/src/api/client.ts      # empty
$ git diff main -- internal/api/server.go    # empty
```
Zero new runtime or dev dependencies. Zero new client functions. Zero new mux routes.

---

## 5. E2E Tests (Level 3 — UI changes)

All e2e tests in `ui/e2e/kanban.spec.ts` (16 tests) and `ui/e2e/kanban-api.spec.ts` (4 tests). Every test captures `console.error` and `pageerror` and asserts both are empty.

| AC | Test | File:line | Scenario | Assertions | Result |
|---|---|---|---|---|---|
| AC-001 | `AC-001: features land in column matching current_phase` | `kanban.spec.ts:108` | 3 features in inception/planning/delivery | Each card in `kanban-column-{current_phase}`; 0 console errors | PASS |
| AC-002 | `AC-002: columns render in canonical order` | `kanban.spec.ts:78` | Empty board | 7 columns, testid suffixes == `['backlog','inception','planning','construction','review','testing','delivery']` | PASS |
| AC-003 | `AC-003: column header shows label and card count` | `kanban.spec.ts:129` | 3 features across 3 columns | Header text contains label; count == number of `feature-card-*` descendants | PASS |
| AC-004 | `AC-004: draft+inception → backlog not inception` | `kanban.spec.ts:152` | 1 draft+inception feature | Card in `kanban-column-backlog`, NOT in `kanban-column-inception` | PASS |
| AC-005 | `AC-005: in_progress+inception → inception not backlog` | `kanban.spec.ts:168` | 1 in_progress+inception feature | Card in `kanban-column-inception`, NOT in `kanban-column-backlog` | PASS |
| AC-006 | `AC-006: done+delivery stays in delivery` | `kanban.spec.ts:184` | 1 done+delivery feature | Card in `kanban-column-delivery` (CON-009) | PASS |
| AC-007 | `AC-007: list → board toggle` | `kanban.spec.ts:197` | Load `/`, click board toggle | `kanban-board` visible, `feature-list` count 0 | PASS |
| AC-008 | `AC-008: board → list toggle` | `kanban.spec.ts:209` | From board, click list toggle | `feature-list` visible, `kanban-board` count 0 | PASS |
| AC-009 | `AC-009: count badge stays consistent` | `kanban.spec.ts:220` | 3 features, read badge, toggle to board | Badge text unchanged after toggle (CON-010) | PASS |
| AC-010 | `AC-010: clicking a card navigates to detail` | `kanban.spec.ts:240` | 1 feature, mock `GET /api/features/nav1` | URL matches `/features/nav1$` | PASS |
| AC-011 | `AC-011: empty board, all columns empty-state, no console errors` | `kanban.spec.ts:271` | `features: []` | Each column has `kanban-column-empty-{key}` visible, 0 cards; 0 console errors, 0 pageerror | PASS |
| AC-013 | `AC-013: partial fill — one column has cards, others empty` | `kanban.spec.ts:285` | 5 features all in planning | planning has 5 cards; other 6 columns 0 cards + empty-state visible | PASS |
| AC-014 | `AC-014: cache invalidation moves a card without reload` | `kanban.spec.ts:306` | Feature in inception, wait past staleTime (5s), toggle list→board to trigger refetch, mock now returns planning | Card moves to `kanban-column-planning`; URL unchanged (no `page.reload()`) | PASS |
| AC-CON-008 | `AC-CON-008: dark mode renders dark-palette backgrounds` | `kanban.spec.ts:349` | Enable dark mode via `ThemeToggle`, load board | Column bg is NOT light palette (asserts `isLight=false`) | PASS |
| AC-CON-011 | `AC-CON-011: each testid exists exactly once` | `kanban.spec.ts:95` | Empty board | `kanban-board` count 1; each `kanban-column-{key}` count 1 | PASS |
| AC-ERR-003 | `AC-ERR-003: deleted feature card → detail 404 state` | `kanban.spec.ts:379` | Card exists, `GET /api/features/gone1` mocked 404 | URL `/features/gone1`; 0 pageerror | PASS |

**AC-014 note**: The original test (committed by the developer) used `await page.reload()` which violates AC-014's "without a full page reload" requirement. The tester fixed the test to use a real react-query refetch path: wait past `staleTime` (5000ms, `main.tsx:13`), then toggle list→board (remounts the board, re-subscribes to the `['features']` query, triggers background refetch). URL never changes, no `page.reload()` called. This is a test fix, not an implementation fix — the implementation correctly re-renders on cache invalidation.

---

## 6. Unit Tests (Level 4)

Per architect decision AD-6, no unit-test runner (vitest) was added because CON-006/FR-011 forbids new devDependencies and AC-CON-006's verification text literally forbids devDependency additions. AC-012 and AC-CON-005 were reclassified to e2e/integration:

- **AC-012** (grouping fn does not throw on `[]`): verified by e2e AC-011 (empty board renders, no `pageerror`) + Go `TestKanbanSmokeEmptyFeaturesArrayNotNull` (backend emits `[]`).
- **AC-CON-005** (board imports `FeatureCard`): verified by source inspection — `KanbanColumn.tsx:1` reads `import FeatureCard from './FeatureCard';` and `KanbanColumn.tsx:35` renders `<FeatureCard key={f.id} feature={f} />`. No re-implementation.

The grouping function `groupFeaturesByColumn` (`ui/src/lib/groupFeaturesByColumn.ts`) is a pure function that initializes all 7 column keys to `[]` (`emptyColumns()` at line 13-23) and never indexes a missing key — so even a null input to `data?.features ?? []` cannot crash it. This is verified behaviorally by every e2e test that loads the board.

---

## 7. State Machine Verification

This feature introduces no state machine. The board is read-only; feature state transitions remain in `internal/feature/feature.go`. The board only observes. Verified by:
- `git diff main -- internal/feature/` shows only a 1-line comment change in `state.go` (gate label "test suite passes" → "go test suite passes") — no transition logic changed.
- No new state field on any DTO.

Existing Go tests (`internal/feature/`, `internal/pipeline/`) cover the state machine and all pass: `ok github.com/MichielDean/devteam/internal/feature 0.010s`, `ok github.com/Michieldog/devteam/internal/pipeline 0.212s`.

---

## 8. Agent Failure Mode Verification

| Failure mode | How tested | Result |
|---|---|---|
| **Nil pointer chains** | `TestSmokeRecoveryNoNilPointer` hits 9 endpoints via full middleware chain; every kanban e2e test asserts `pageErrors` empty | No panics |
| **Null vs empty arrays** | `TestKanbanSmokeEmptyFeaturesArrayNotNull` asserts literal `[]`; `server_test.go:534` regression guard; e2e AC-011 loads empty board | `[]` everywhere, never `null` |
| **Phantom method calls** | `go build` + `go test ./...` pass; `npm run build` (tsc + vite) passes; Playwright loads the real bundle | No phantom methods |
| **Over-engineering** | New TSX: `KanbanBoard.tsx` 47 lines, `KanbanColumn.tsx` 40 lines, `ViewToggle.tsx` 34 lines, `groupFeaturesByColumn.ts` 38 lines = **159 lines**. Plan budget was ~250. No drag-drop, no WIP limits, no new route, no new dep. | Within budget |
| **Missing error paths** | AC-ERR-001 (500 → banner), AC-ERR-002 (refetch error → stable), AC-ERR-003 (deleted card → 404), `TestSmokeRecoveryNoNilPointer` (404 for nonexistent), `TestListFeaturesErrorResponseHasNoTotalCount` (500 has no total_count) | All error paths covered |
| **Constraint violations** | Every CON-001..011 has a test (see §1 table) | All constraints verified |
| **Multi-component inconsistency** | Single-repo, UI-only feature. CON-004 (empty `[]`) tested at backend (`dto.go`), frontend guard (`?? []`), and e2e (AC-011). CON-009 (terminal visible) tested at backend (Go test) and e2e (AC-006). | Consistent across all components |
| **Language footguns (TS)** | `groupFeaturesByColumn.ts:32` guards the `f.current_phase as PhaseName` cast with `PHASES.includes(...)` before indexing — verified by reading source. Unknown phase is dropped, not indexed into a missing key. | No runtime `undefined` key |

---

## 9. Proof of Work — Exact Reproduction

### Go tests
```bash
$ cd /home/lobsterdog/source/devteam/worktrees/kanban-view/devteam
$ bash run-tests.sh go
=== Running Go tests ===
?      github.com/MichielDean/devteam/cmd/devteam  [no test files]
ok     github.com/MichielDean/devteam/internal/api  0.128s
ok     github.com/MichielDean/devteam/internal/config  0.006s
ok     github.com/MichielDean/devteam/internal/feature  0.010s
?      github.com/MichielDean/devteam/internal/gitops  [no test files]
ok     github.com/MichielDean/devteam/internal/init  0.008s
ok     github.com/MichielDean/devteam/internal/intake  0.007s
ok     github.com/MichielDean/devteam/internal/pipeline  0.212s
?      github.com/MichielDean/devteam/internal/plugins  [no test files]
ok     github.com/MichielDean/devteam/internal/repo  1.747s
ok     github.com/MichielDean/devteam/internal/role  0.003s
ok     github.com/MichielDean/devteam/internal/rules  0.005s
ok     github.com/MichielDean/devteam/internal/spec  0.006s
=== Go tests: exit 0 ===

$ PATH="$PATH:/usr/local/go/bin" go test ./internal/api/... -run TestKanban -count=1 -timeout 60s -v
=== RUN   TestKanbanSmokeEmptyFeaturesArrayNotNull
--- PASS: TestKanbanSmokeEmptyFeaturesArrayNotNull (0.00s)
=== RUN   TestKanbanSmokeTerminalStatusFeaturesStayInPhaseColumn
--- PASS: TestKanbanSmokeTerminalStatusFeaturesStayInPhaseColumn (0.00s)
=== RUN   TestKanbanSmokeNoKanbanSpecificEndpoint
=== RUN   TestKanbanSmokeNoKanbanSpecificEndpoint/GET_/api/kanban
=== RUN   TestKanbanSmokeNoKanbanSpecificEndpoint/GET_/api/kanban/features
=== RUN   TestKanbanSmokeNoKanbanSpecificEndpoint/GET_/api/board
=== RUN   TestKanbanSmokeNoKanbanSpecificEndpoint/GET_/api/features/kanban
--- PASS: TestKanbanSmokeNoKanbanSpecificEndpoint (0.00s)
    --- PASS: TestKanbanSmokeNoKanbanSpecificEndpoint/GET_/api/kanban (0.00s)
    --- PASS: TestKanbanSmokeNoKanbanSpecificEndpoint/GET_/api/kanban/features (0.00s)
    --- PASS: TestKanbanSmokeNoKanbanSpecificEndpoint/GET_/api/board (0.00s)
    --- PASS: TestKanbanSmokeNoKanbanSpecificEndpoint/GET_/api/features/kanban (0.00s)
PASS
ok      github.com/MichielDean/devteam/internal/api  0.006s
```

### Playwright tests
```bash
$ cd /home/lobsterdog/source/devteam/worktrees/kanban-view/devteam
$ bash run-tests.sh ui
=== Running UI tests ===
...
Running 32 tests using 1 worker
...
  3 skipped
  29 passed (10.2s)
=== Playwright: exit 0 ===
```

The 3 skipped tests are pre-existing conditional skips in `app.spec.ts`:
- `feature list handles empty state` — skips when workspace has features (`test.skip(count > 0, ...)`)
- `feature detail page renders correctly` — skips when no first card visible
- `phase progress indicators render` — skips when no first card visible

These are not kanban tests and not failures. The kanban-specific suite (`kanban.spec.ts` + `kanban-api.spec.ts`) is **20 passed, 0 skipped, 0 failed**.

### git diff verification (CON-003 / CON-006)
```bash
$ git diff main -- ui/package.json           # (no output — empty)
$ git diff main -- ui/src/api/client.ts      # (no output — empty)
$ git diff main -- internal/api/server.go    # (no output — empty)
```

---

## 10. Test Traceability

Every acceptance criterion has at least one test. Every constraint has at least one test that would fail if violated.

| AC | Test(s) | Level | Pass |
|---|---|---|---|
| AC-001 | `kanban.spec.ts:108` AC-001 | e2e | ✅ |
| AC-002 | `kanban.spec.ts:78` AC-002 | e2e | ✅ |
| AC-003 | `kanban.spec.ts:129` AC-003 | e2e | ✅ |
| AC-004 | `kanban.spec.ts:152` AC-004 | e2e | ✅ |
| AC-005 | `kanban.spec.ts:168` AC-005 | e2e | ✅ |
| AC-006 | `kanban.spec.ts:184` AC-006 + `kanban_smoke_test.go:83` TestKanbanSmokeTerminalStatusFeaturesStayInPhaseColumn | e2e + Go smoke | ✅ |
| AC-007 | `kanban.spec.ts:197` AC-007 | e2e | ✅ |
| AC-008 | `kanban.spec.ts:209` AC-008 | e2e | ✅ |
| AC-009 | `kanban.spec.ts:220` AC-009 | e2e | ✅ |
| AC-010 | `kanban.spec.ts:240` AC-010 | e2e | ✅ |
| AC-011 | `kanban.spec.ts:271` AC-011 + `kanban_smoke_test.go:38` TestKanbanSmokeEmptyFeaturesArrayNotNull | e2e + Go smoke | ✅ |
| AC-012 | `kanban.spec.ts:271` AC-011 (no throw on `[]`) + `server_test.go:51` TestListFeaturesEmpty | e2e + Go (reclassified per AD-6) | ✅ |
| AC-013 | `kanban.spec.ts:285` AC-013 | e2e | ✅ |
| AC-014 | `kanban.spec.ts:306` AC-014 | e2e | ✅ |
| AC-CON-003 | `kanban-api.spec.ts:27` + `kanban_smoke_test.go:151` TestKanbanSmokeNoKanbanSpecificEndpoint + git diff | integration | ✅ |
| AC-CON-005 | source inspection: `KanbanColumn.tsx:1,35` | integration (reclassified per AD-6) | ✅ |
| AC-CON-006 | `kanban-api.spec.ts:55` + git diff `ui/package.json` empty | integration | ✅ |
| AC-CON-008 | `kanban.spec.ts:349` AC-CON-008 | e2e | ✅ |
| AC-CON-011 | `kanban.spec.ts:95` AC-CON-011 | e2e | ✅ |
| AC-ERR-001 | `kanban-api.spec.ts:70` AC-ERR-001 | integration | ✅ |
| AC-ERR-002 | `kanban-api.spec.ts:90` AC-ERR-002 | integration | ✅ |
| AC-ERR-003 | `kanban.spec.ts:379` AC-ERR-003 | e2e | ✅ |

| CON | Test that would fail if violated |
|---|---|
| CON-001 | AC-002 (column order assertion) |
| CON-002 | AC-004 + AC-005 (backlog rule) |
| CON-003 | AC-CON-003 + `TestKanbanSmokeNoKanbanSpecificEndpoint` + git diff |
| CON-004 | `TestKanbanSmokeEmptyFeaturesArrayNotNull` + AC-011 |
| CON-005 | `KanbanColumn.tsx` source inspection (imports `FeatureCard`) |
| CON-006 | git diff `ui/package.json` empty + AC-CON-006 |
| CON-007 | AC-007 + AC-008 (both toggle directions) |
| CON-008 | AC-CON-008 (dark palette assertion) |
| CON-009 | AC-006 + `TestKanbanSmokeTerminalStatusFeaturesStayInPhaseColumn` |
| CON-010 | AC-009 (badge unchanged across toggle) |
| CON-011 | AC-CON-011 (each testid exactly once) |

---

## 11. Commit Discipline

Test files committed to the `feature/kanban-view` worktree (NOT pushed — pipeline handles pushes):

```
f3b9695 chore(kanban-view): untrack ui/node_modules symlink
87035f8 test(kanban-view): add backend smoke tests + fix AC-014 refetch trigger
a375f0b feat(kanban-view): add Kanban board view with toggle
```

New/modified test artifacts in the implementation worktree:
- `internal/api/kanban_smoke_test.go` (new — 191 lines, 3 Go smoke tests)
- `ui/e2e/kanban.spec.ts` (modified — AC-014 refetch trigger fix, 25 lines changed)
- `ui/playwright.config.ts` (modified — SERVER_PORT isolation, 6 lines changed)
- `run-tests.sh` (new — test runner helper)

---

## 12. Findings

**No findings requiring recirculation.** The implementation matches the spec, all tests pass, no nil pointer panics, no null-vs-empty-array mismatches, no untested error paths.

One non-blocking note: AC-014's original test (committed by the developer) used `page.reload()`, which technically violated the "without a full page reload" wording of AC-014. The tester fixed the test (not the implementation) to use a real react-query refetch path. The implementation was already correct — it re-renders on cache invalidation as specified.