# Review Report: kanban-view

**Feature ID**: kanban-view
**Phase**: review
**Reviewer**: reviewer
**Date**: 2026-06-22
**Priority**: P1
**Branch reviewed**: `feature/kanban-view` at `6a7756d` ("feat(kanban-view): align board impl with spec/plan")
**Worktree**: `/home/lobsterdog/source/devteam/worktrees/kanban-view/devteam`

## Summary

- Acceptance criteria: 22 total (AC-001..AC-022)
- MET: 18
- NOT MET: 3 (AC-011 unit test, AC-014/AC-015 via stale e2e selectors — see F-003)
- UNVERIFIABLE-by-review: 1 (AC-016 single-fetch — integration, Tester verifies via network)
- Findings: 2 Blocking, 2 Required, 2 Noted

**Gate status: BLOCKED** — 2 Blocking findings (CON-001 port regression, stale e2e suite mismatches spec-compliant impl) must be resolved before advancing. The implementation source code itself is spec-compliant and minimal; the blockers are configuration drift and test-code drift from a mid-flight re-alignment commit (`6a7756d`) that rewrote the impl to match the spec but left the tests encoding the prior divergent interpretation.

---

## Phase 1: Constraint Register Review (with execution-path traces)

### CON-001 — E2E on `:18765`, never `:8765`
**Source**: AGENTS.md "Frontend (UI)"
**Status**: NOT MET (Blocking)
**Trace**:
1. `ui/playwright.config.ts:10` — `baseURL: process.env.BASE_URL || 'http://localhost:8765'`
2. `ui/playwright.config.ts:21` — `port: parseInt(process.env.SERVER_PORT || '8765')`
3. `ui/playwright.config.ts:20` — command defaults to `-http :8765`
**Evidence**: `git diff main -- ui/playwright.config.ts` shows the default changed FROM `:18765` TO `:8765` on this branch. The env-var override path exists (`SERVER_PORT`), but the constraint is that the **default** test port is `:18765`. This branch regressed it to the production port.
**Explanation**: Direct violation. Production server and test server now share a port by default — tests can collide with the live `devteam-web` service. Must revert default to `:18765`.

### CON-002 — New components under `ui/src/components/`, hooks under `ui/src/hooks/`
**Source**: AGENTS.md "Project Structure"
**Status**: MET
**Trace**:
1. New files: `KanbanBoard.tsx`, `KanbanCard.tsx`, `KanbanColumn.tsx`, `ViewToggle.tsx`, `badgeColors.ts`, `groupFeaturesByPhase.ts` — all under `ui/src/components/`
2. New hook: `useSessionView.ts` under `ui/src/hooks/` (sibling to existing `useFeatures.ts`, `useSSE.ts`)
3. Modified: `Dashboard.tsx` (page), `FeatureCard.tsx` (component) — both in their existing canonical locations
**Evidence**: `git diff main --name-only -- ui/` lists 12 files; every new file is under `ui/src/components/` or `ui/src/hooks/`. No new pages.

### CON-003 — Minimal deps; no new runtime npm dep
**Source**: constitution.md VIII
**Status**: MET (with note)
**Trace**:
1. `ui/package.json:14-22` `dependencies` block: `@tanstack/react-query`, `highlight.js`, `react`, `react-dom`, `react-markdown`, `react-router`, `rehype-highlight` — unchanged from main
2. `ui/package.json:23-32` `devDependencies`: `@playwright/test`, `@tailwindcss/vite`, `@types/react`, `@types/react-dom`, `@vitejs/plugin-react`, `tailwindcss`, `typescript`, `vite` — **no `vitest` added**
3. Board uses only `react`, `react-router` `Link`, `@tanstack/react-query` (via Dashboard), Tailwind classes — all already in deps
**Evidence**: `git diff main -- ui/package.json` is empty (0 lines changed).
**Note**: The plan (`plan.md:1838`) called for adding `vitest` as a devDep to satisfy AC-011's unit-test requirement. It was not added. This keeps CON-003 strictly satisfied but leaves AC-011 unimplemented (see F-004). The plan's "minimal deps" choice and the AC-011 requirement conflict; resolution is a Required finding, not a CON-003 violation.

### CON-004 — Existing `feature-card-*` / `feature-count-badge` assertions still pass
**Source**: existing e2e `app.spec.ts`
**Status**: NOT MET (Blocking)
**Trace**:
1. `Dashboard.tsx:52` — `hasFeatures = !isLoading && !error && features.length > 0`
2. `Dashboard.tsx:70` — `{hasFeatures && <ViewToggle value={view} onChange={setView} />}`
3. `Dashboard.tsx:106-110` — `{hasFeatures && (view === 'board' ? <KanbanBoard/> : <FeatureList/>)}`
4. `useSessionView.ts:15` — default is `'board'`
5. ∴ on first load with features, `KanbanBoard` renders. Board cards emit `kanban-card-*` (`KanbanCard.tsx:25`), NOT `feature-card-*`.
6. `app.spec.ts:13-17` — `await page.goto('/'); ... page.locator('[data-testid*="feature-card"]').count()` expects `>= 1`. With Board as default, board cards are `kanban-card-*` → count is 0 → assertion fails.
7. `app.spec.ts` was **not modified** on this branch (`git diff main -- ui/e2e/app.spec.ts` is empty).
**Evidence**: `app.spec.ts:5-19` "feature list loads and shows features" asserts `feature-card` count ≥ 1 on `/` load. Board default means `feature-card-*` only appears after clicking "List". The plan (T-009, `plan.md:2116`) explicitly assigned the developer to add a click-to-List fixture. Not done.
**Explanation**: CON-004 regression. The existing suite breaks because the default view flipped to Board and `app.spec.ts` was never updated to click "List" first. The plan called this out (T-009) and the developer skipped it.

### CON-005 — Reuse `PHASES`/`PHASE_LABELS`/`STATUS_LABELS`/`PRIORITY_LABELS`, no duplicated strings
**Source**: existing UI `types/index.ts`
**Status**: MET
**Trace**:
1. `KanbanBoard.tsx:2` — `import { PHASES, PHASE_LABELS } from '../types'`
2. `KanbanBoard.tsx:21-27` — `(PHASES as readonly PhaseName[]).map(phase => <KanbanColumn label={PHASE_LABELS[phase]} .../>)` — column order and labels derive from constants
3. `KanbanColumn.tsx:21` — renders `{label}` (passed from parent); no literal phase name in column
4. `KanbanCard.tsx:3` — `import { STATUS_LABELS, PRIORITY_LABELS } from '../types'`
5. `KanbanCard.tsx:18-19` — `STATUS_LABELS[feature.status]`, `PRIORITY_LABELS[feature.priority]`
6. Grep `Kanban*.tsx` for `'Inception'|'Planning'|'In Progress'` → 0 matches (only `groupFeaturesByPhase.ts:6` references `'other'` which is the defensive bucket, not a phase/status label)
**Evidence**: No phase/status name literals in board components. All via the `types/` constants.

### CON-006 — Card chrome parity with `FeatureCard`
**Source**: `FeatureCard.tsx`
**Status**: MET
**Trace**:
1. `badgeColors.ts:3-13` — single `statusColors` map, extracted from FeatureCard
2. `FeatureCard.tsx:6` — `import { statusColors } from './badgeColors'`; `FeatureCard.tsx:37` uses `statusColors[feature.status]`
3. `KanbanCard.tsx:4` — `import { statusColors } from './badgeColors'`; `KanbanCard.tsx:37` uses `statusColors[feature.status]`
4. Gate text: `KanbanCard.tsx:64` `✓ Gate passed`, `:66` `✗ Gate failed` — byte-identical to `FeatureCard.tsx:59`/`:61`
5. `question-badge` testid: `KanbanCard.tsx:53` local `<span data-testid="question-badge">` (not the `QuestionBadge` `<Link>`, avoiding nested anchors inside the card `<Link>`). Same testid as `QuestionBadge.tsx:15`, valid HTML.
**Evidence**: Single shared map; both consumers import. Gate strings identical. Question badge testid reused without nested-anchor violation.

### CON-007 — Board consumes same `useQuery(['features'])`, no second fetch
**Source**: `Dashboard.tsx` data flow
**Status**: MET
**Trace**:
1. `Dashboard.tsx:21-24` — single `useQuery({ queryKey: ['features'], queryFn: listFeatures })`
2. `Dashboard.tsx:50` — `const features = data?.features ?? []`
3. `Dashboard.tsx:108` — `<KanbanBoard features={features} />` (props-only)
4. `KanbanBoard.tsx:12` — `function KanbanBoard({ features })` — no `useQuery`, no `useEffect`, no `fetch`. Pure render from props.
5. `KanbanBoard.tsx:13` — `groupFeaturesByPhase(features)` is a pure function (`groupFeaturesByPhase.ts:20-32`, no side effects, no I/O)
**Evidence**: Zero network calls in board component tree. Single query owned by Dashboard.
**Note**: AC-016 (assert exactly one `GET /api/features` via `page.on('request')`) is an integration assertion — verifiable by Tester, not by code review. Code path confirms the precondition.

### CON-008 — Loading/error/empty states explicitly covered
**Source**: overconfidence-prevention Pattern 2
**Status**: MET
**Trace**:
1. Loading: `Dashboard.tsx:89-94` — `{isLoading && <div data-testid="features-loading">...}` — board not rendered (gated by `hasFeatures` at `:106`)
2. Error: `Dashboard.tsx:96-100` — `{error && <div data-testid="features-error">...}` — board not rendered
3. Empty board: `Dashboard.tsx:102-104` — `{!isLoading && !error && features.length === 0 && <EmptyState/>}` — board not rendered, toggle hidden (`hasFeatures` is false at `:70`)
4. Empty column: `KanbanColumn.tsx:24-30` — `features.length === 0 ? <p data-testid="kanban-column-empty-${phase}">No features</p> : cards`
5. Null-array guard: `groupFeaturesByPhase.ts:8-13` `emptyGroups()` initializes every bucket to `[]` — no `null` arrays
**Evidence**: All three states (loading/error/empty) reuse existing Dashboard branches; board renders only when `hasFeatures`. Empty columns render placeholder, never blank/null.

### CON-009 — Unknown `current_phase` handled defensively
**Source**: overconfidence-prevention Pattern 1
**Status**: MET (code); NOT MET (test — see F-004)
**Trace**:
1. `groupFeaturesByPhase.ts:24-29`:
   ```
   for (const f of features) {
     if ((PHASES as readonly string[]).includes(f.current_phase)) {
       groups[f.current_phase as PhaseName].push(f);
     } else {
       groups.other.push(f);
     }
   }
   ```
2. Path A: `current_phase === 'planning'` → `PHASES.includes('planning')` true → pushed to `groups.planning`
3. Path B: `current_phase === 'weird'` → `PHASES.includes('weird')` false → pushed to `groups.other` (no throw, no drop)
4. Path C: `current_phase === undefined` → `.includes(undefined)` false → `groups.other` (no crash)
5. `KanbanBoard.tsx:14` — `showOther = groups.other.length > 0` → Other column renders only when needed (AC-019)
6. Partition invariant: every input feature goes to exactly one bucket; `sum(buckets) === input.length` by construction (no `continue`, no early return)
**Evidence**: Code handles unknown phases correctly. **No unit test exists** to verify AC-011 (`KanbanBoard.test.ts` missing, `vitest` not in devDeps) — see F-004.

---

## Phase 2: Acceptance Criteria Review

### AC-001: Toggle visible, Board active by default
**Status**: MET
**Evidence**: `Dashboard.tsx:70` `{hasFeatures && <ViewToggle value={view} onChange={setView} />}`; `ViewToggle.tsx:13` `data-testid="view-toggle"`; `ViewToggle.tsx:28` `aria-pressed={value === 'board'}`; `useSessionView.ts:15` default `'board'`.
**Explanation**: Toggle renders when features exist; Board button has `aria-pressed="true"` on fresh session (default `'board'`).

### AC-002: Click Board → 6 phase columns, FeatureList gone
**Status**: MET
**Evidence**: `Dashboard.tsx:107-108` `view === 'board' ? <KanbanBoard/> : <FeatureList/>`; `KanbanBoard.tsx:21` maps `PHASES` (6 entries, `types/index.ts:171`); `KanbanBoard.tsx:29-31` appends Other only when `groups.other.length > 0`. Clicking Board sets `view='board'` (`ViewToggle.tsx:26`), FeatureList unmounts.
**Explanation**: 6 columns in `PHASES` order (Inception→Delivery); conditional Other. List view unmounts.

### AC-003: Click List → FeatureList renders, no kanban columns
**Status**: MET
**Evidence**: `Dashboard.tsx:108-109` `view === 'list'` branch renders `<FeatureList features={features}/>`. `FeatureList.tsx:50` `data-testid="feature-list"`. Board unmounts (conditional render).
**Explanation**: Toggling to List mounts FeatureList; board not rendered.

### AC-004: Board persists across reload (sessionStorage)
**Status**: MET
**Evidence**: `useSessionView.ts:10` `sessionStorage.getItem('devteam.dashboard.view')`; `:21-24` `set` writes `sessionStorage.setItem`; `:19` `useState(readStored)` lazy-init reads on mount. Key `devteam.dashboard.view` (FR-002).
**Explanation**: Reload re-mounts Dashboard, `useSessionView` lazy-inits from sessionStorage, prior choice restored.

### AC-005: Fresh session → Board default
**Status**: MET
**Evidence**: `useSessionView.ts:7-16` `readStored()` returns `'board'` when stored value is absent or invalid (`if (v === 'board' || v === 'list') return v; ... return 'board'`).
**Explanation**: Fresh sessionStorage → `getItem` returns null → falls through to `'board'`.

### AC-006: Zero features → toggle hidden, EmptyState renders
**Status**: MET
**Evidence**: `Dashboard.tsx:52` `hasFeatures = !isLoading && !error && features.length > 0`; `:70` `{hasFeatures && <ViewToggle/>}` (hidden when false); `:102-104` `{!isLoading && !error && features.length === 0 && <EmptyState onCreateClick=.../>}`.
**Explanation**: Toggle gated by `hasFeatures`; empty branch renders `EmptyState`.

### AC-007: Card in correct column with P1 + "In Progress" badges
**Status**: MET
**Evidence**: `KanbanCard.tsx:18` `STATUS_LABELS['in_progress']` → `"In Progress"` (`types/index.ts:215`); `:19` `PRIORITY_LABELS[1]` → `"P1 - Critical"` (`types/index.ts:226`); `:38` `data-testid="kanban-card-status"`; `:44` `data-testid="kanban-card-priority"`. Column placement via `groupFeaturesByPhase` → `groups[feature.current_phase]` (`KanbanBoard.tsx:26`).
**Explanation**: Card renders in Planning column with correct badge text from shared constants.

### AC-008: Pending-questions badge when count > 0
**Status**: MET
**Evidence**: `KanbanCard.tsx:50-58` `{feature.pending_questions_count > 0 && <span data-testid="question-badge">...}`.
**Explanation**: Badge renders only when count > 0; testid `question-badge` matches spec.

### AC-009: Gate indicator (✓/✗) when gate_result present
**Status**: MET
**Evidence**: `KanbanCard.tsx:61-69` `{feature.gate_result && <div data-testid="kanban-card-gate">{feature.gate_result.passed ? <span>✓ Gate passed</span> : <span>✗ Gate failed</span>}</div>}`. Text byte-identical to `FeatureCard.tsx:57-64`.
**Explanation**: Gate indicator renders with correct pass/fail text.

### AC-010: Click card → navigate to `/features/:id`
**Status**: MET
**Evidence**: `KanbanCard.tsx:23-26` `<Link to={`/features/${feature.id}`} data-testid={`kanban-card-${feature.id}`}>`. Same destination as `FeatureCard.tsx:18-19`.
**Explanation**: Card is a `react-router` `Link`; click navigates to detail page.

### AC-011: Unknown `current_phase` → "Other" column, no crash (unit test)
**Status**: NOT MET (Required)
**Evidence**: `groupFeaturesByPhase.ts:24-29` handles unknown phases correctly (CON-009 trace). **But no unit test exists**: `KanbanBoard.test.ts` is absent (`ls ui/src/components/` shows no `.test.ts`), `vitest` is not in `package.json` devDeps, no `test:unit` script.
**Explanation**: The code is correct; the required verification artifact is missing. AC-011 test level is explicitly `unit`. The plan (T-003, `plan.md:2159-2163`) and constraint register mandate a unit test for `groupFeaturesByPhase`. Either the developer or tester must add it; currently neither exists on the branch.

### AC-012: `gate_blocked` → red ring
**Status**: MET
**Evidence**: `KanbanCard.tsx:11-14` `ringClass('gate_blocked')` → `'ring-2 ring-red-400'`; `:26` className includes `${ring}`.
**Explanation**: Red ring applied only for `gate_blocked`.

### AC-013: `waiting_for_human` → yellow ring
**Status**: MET
**Evidence**: `KanbanCard.tsx:13` `if (status === 'waiting_for_human') return 'ring-2 ring-yellow-400'`.
**Explanation**: Yellow ring applied only for `waiting_for_human`.

### AC-014: Loading state → `features-loading`, no columns
**Status**: MET (impl); NOT MET (e2e — see F-003)
**Evidence**: `Dashboard.tsx:89-94` loading branch renders `features-loading`; board gated by `hasFeatures` (false while loading) so no `kanban-column-*` renders.
**Explanation**: Implementation correct. The e2e test `kanban.spec.ts` does not exercise this AC with the spec-compliant selectors (it uses `feature-card-*` and a `backlog` column that don't exist).

### AC-015: Error state → `features-error`, no board
**Status**: MET (impl); NOT MET (e2e — see F-003)
**Evidence**: `Dashboard.tsx:96-100` error branch renders `features-error`; board gated by `hasFeatures` (false on error).
**Explanation**: Implementation correct. `kanban-api.spec.ts:80,81` asserts `kanban-error` testid which does not exist in the impl (Dashboard uses `features-error`).

### AC-016: Exactly one `GET /api/features` during Board render
**Status**: UNVERIFIABLE-by-review (integration)
**Evidence**: Code path supports it (CON-007 trace — single `useQuery`, board is props-only). Verifiable by Tester via `page.on('request')`.
**Explanation**: No code-level second fetch exists. Final verification is Tester's network-level assertion.

### AC-017: Empty column → header + "No features" placeholder
**Status**: MET
**Evidence**: `KanbanColumn.tsx:17-22` header with `data-testid="kanban-column-header-${phase}"` renders unconditionally; `:24-30` `features.length === 0 ? <p data-testid="kanban-column-empty-${phase}">No features</p> : cards`.
**Explanation**: Empty columns render header + muted placeholder.

### AC-018: Zero features → EmptyState, toggle hidden
**Status**: MET (same as AC-006)
**Evidence**: See AC-006 trace.
**Explanation**: Cross-referenced with AC-006; identical code path.

### AC-019: All 6 phase columns present regardless of fill
**Status**: MET
**Evidence**: `KanbanBoard.tsx:21-28` `(PHASES as readonly PhaseName[]).map(phase => <KanbanColumn .../>)` — always renders all 6; `:29-31` Other column only when `groups.other.length > 0`.
**Explanation**: 6 columns always render; +1 Other only when unknown phase exists.

### AC-020: Column overflow → vertical scroll, header fixed
**Status**: MET
**Evidence**: `KanbanColumn.tsx:17-22` header has `shrink-0` (won't shrink); `:23` body div has `overflow-y-auto flex-1` (scrolls independently). `KanbanBoard.tsx:19` container `h-[calc(100vh-8rem)]` bounds board height; `KanbanColumn.tsx:15` `max-h-full` constrains column to board height.
**Explanation**: Body scrolls; header fixed; board height bounded.

### AC-021: Resize shorter → scroll area adjusts
**Status**: MET
**Evidence**: `KanbanBoard.tsx:19` `h-[calc(100vh-8rem)]` is viewport-relative; `KanbanColumn.tsx:15` `max-h-full` + `:23` `flex-1` body adjusts to column height which tracks viewport.
**Explanation**: Height tracks viewport via CSS calc; columns follow.

### AC-022: Narrow viewport → horizontal scroll, min-width 240px
**Status**: MET
**Evidence**: `KanbanBoard.tsx:19` `overflow-x-auto`; `KanbanColumn.tsx:15` `w-60` (Tailwind `w-60` = 15rem = 240px) + `shrink-0` (prevents compression).
**Explanation**: Board scrolls horizontally; columns fixed at 240px.

---

## Phase 3: Negative Test Vector Verification

The constraint register has no RFC conformance vectors. Defensive edge cases:

| Edge case | Impl behavior | Status |
|---|---|---|
| Unknown `current_phase` (CON-009/AC-011) | `groupFeaturesByPhase` routes to `other` bucket; no throw | MET (code); NOT MET (no unit test) — F-004 |
| Empty board (CON-008/AC-006) | Dashboard renders `EmptyState`, toggle hidden | MET |
| Empty column (CON-008/AC-017) | `KanbanColumn` renders placeholder | MET |
| Loading (CON-008/AC-014) | `features-loading` branch, board unmounted | MET (code) |
| Error (CON-008/AC-015) | `features-error` branch, board unmounted | MET (code) |
| Missing `total_count` | `Dashboard.tsx:51` `data?.total_count ?? 0` | MET (pre-existing) |
| Invalid stored view | `useSessionView.ts:11` validates `=== 'board' \|\| === 'list'`, else default | MET |

---

## Phase 4: Cross-Component Consistency

| Shared value | Producer | Consumer(s) | Consistent? |
|---|---|---|---|
| Phase labels | `PHASE_LABELS` (types) | `KanbanColumn` header, `KanbanBoard` ordering | YES — single import |
| Status labels | `STATUS_LABELS` (types) | `KanbanCard` status badge | YES |
| Priority labels | `PRIORITY_LABELS` (types) | `KanbanCard` priority badge | YES |
| Status→class map | `badgeColors.ts` | `FeatureCard`, `KanbanCard` (both import) | YES — single source |
| Gate text | hardcoded in both cards | `FeatureCard:59/61`, `KanbanCard:64/66` | YES — byte-identical |
| `question-badge` testid | `QuestionBadge` (Link), local `<span>` in `KanbanCard` | E2E selectors | YES — same testid, valid HTML (no nested anchors) |
| Features array | Dashboard `useQuery(['features'])` | `FeatureList`, `KanbanBoard` (props) | YES — single query |
| View preference | `useSessionView` | `Dashboard` render branch | YES — single owner |

**Multi-component check**: The only N-consumer case is `statusColors` (2 consumers). Both import from `badgeColors.ts`. No divergence.

---

## Phase 5: Language-Specific Footgun Review (TypeScript)

- `any` type: No `any` in new code. `statusColors: Record<string, string>` (`badgeColors.ts:3`). `Record<GroupKey, FeatureSummary[]>` (`groupFeaturesByPhase.ts:22`). ✓
- `==` vs `===`: `useSessionView.ts:11` uses `===`. `KanbanCard.tsx:11-13` uses `===`. No `==` found in new code. ✓
- Optional chaining hiding null: `Dashboard.tsx:50` `data?.features ?? []` — handles undefined `data` (loading) correctly. `:51` `data?.total_count ?? 0`. ✓
- Null array from `.map`/grouping: `groupFeaturesByPhase.ts:8-13` `emptyGroups()` pre-initializes all buckets to `[]`. ✓
- `String.repeat(n)` with n<0: Not used. ✓
- Integer overflow: N/A (JS numbers; no integer arithmetic on large values). ✓

No footgun findings.

---

## Phase 6: Over-Engineering Check

| File | Lines | Expected | Verdict |
|---|---|---|---|
| `KanbanBoard.tsx` | 34 | ~40 | Minimal |
| `KanbanColumn.tsx` | 37 | ~40 | Minimal |
| `KanbanCard.tsx` | 76 | ~80 | Minimal |
| `ViewToggle.tsx` | 34 | ~35 | Minimal |
| `badgeColors.ts` | 13 | ~15 | Minimal (extracted map) |
| `groupFeaturesByPhase.ts` | 32 | ~35 | Minimal |
| `useSessionView.ts` | 31 | ~30 | Minimal |
| `Dashboard.tsx` net diff | +23/-13 | ~+20 | Minimal |

Total new logic: ~257 lines across 7 files. Plan estimated ~6 files, ~250 lines. On target. No speculative abstractions, no factories, no config-for-constants. `PHASE_GROUP_KEYS` export in `groupFeaturesByPhase.ts:6` is unused by the board (board maps `PHASES` directly and appends `other` conditionally) — **Noted**, not a finding (cheap, could be used by tests).

---

## Phase 7: Missing Implementation

| Spec/Plan requirement | Status |
|---|---|
| All FR-001..FR-017 | Implemented in code |
| T-009: `app.spec.ts` click-to-List fixture (CON-004) | **MISSING** — F-001 |
| T-003/T-010: `KanbanBoard.test.ts` unit test + `vitest` devDep (AC-011) | **MISSING** — F-004 |
| `kanban.spec.ts` covering AC-001..AC-022 with spec-compliant selectors | **STALE** — F-003 (encodes prior divergent impl) |
| `kanban-api.spec.ts` with spec-compliant selectors | **STALE** — F-003 |
| Backend changes | None required, none made ✓ |

---

## Phase 8: Security Review (P1)

Spec explicitly documents security extension as N/A: "View-only UI. No new endpoint, no input handling, no auth surface, no data mutation." Verified:

- No new API endpoint (`package.json` no new runtime dep; `internal/` unchanged on this branch's UI diff — backend changes in the broader diff are from other commits, not the kanban-view feature)
- No user input handling in new components (board is read-only render from props)
- No auth surface touched (board reuses already-authenticated `useQuery(['features'])`)
- No state mutations (no `useMutation` in board components)
- `Link` destinations are `/features/:id` where `:id` comes from API response (not user input) — no open-redirect
- `sessionStorage` value validated against allowlist (`useSessionView.ts:11`) — no injection
- No `dangerouslySetInnerHTML` in new code
- No secrets in new code

No security findings.

---

## Phase 9: Constitution Compliance

| Principle | Status | Evidence |
|---|---|---|
| I. Spec-Driven | ✅ | Impl derives from spec.md + acceptance.md + plan.md |
| VII. Self-Bootstrap | ✅ | Improves platform's own UI |
| VIII. Minimal Dependencies | ✅ | No new runtime dep; `package.json` deps unchanged |
| IX. Pipeline Governance | ✅ | Security/resiliency N/A (view-only), documented in spec |

---

## Findings

### F-001: `app.spec.ts` not updated — CON-004 regression (Blocking)
- **Severity**: Blocking
- **Criterion**: CON-004, AC-001/AC-003 (regression), plan T-009
- **Code**: `ui/e2e/app.spec.ts` (unchanged from main; `git diff main -- ui/e2e/app.spec.ts` empty)
- **Description**: Board is now the default view (`useSessionView.ts:15`). `app.spec.ts:5-19` "feature list loads and shows features" asserts `[data-testid*="feature-card"]` count ≥ 1 on `/` load. With Board default, board cards emit `kanban-card-*` (`KanbanCard.tsx:25`), so `feature-card-*` count is 0 → test fails. Plan T-009 (`plan.md:2116`) explicitly assigned the developer to add a click-to-List fixture before list-view assertions. Not done. This breaks the existing suite (CON-004: "existing `feature-card-*` testids and `feature-count-badge` assertions continue to pass unchanged").
- **Fix**: Add `await page.locator('[data-testid="view-toggle-list"]').click();` after `page.goto('/')` in every `app.spec.ts` test that asserts `feature-card-*` or `feature-list`. Additive, no assertion removed.

### F-002: Playwright config default port regressed to `:8765` — CON-001 violation (Blocking)
- **Severity**: Blocking
- **Criterion**: CON-001 ("E2E on `:18765`, never `:8765`")
- **Code**: `ui/playwright.config.ts:10` (`baseURL: ... 'http://localhost:8765'`), `:21` (`port: ... '8765'`), `:20` (`-http :8765`)
- **Description**: `git diff main` shows the default port changed FROM `:18765` TO `:8765` on this branch. The env-var override exists, but the constraint is that the **default** test port is `:18765` to avoid colliding with the production `devteam-web` service on `:8765`. This is a direct constraint violation.
- **Fix**: Revert `playwright.config.ts` defaults to `:18765` (baseURL, port, command). Keep the `SERVER_PORT` env override.

### F-003: `kanban.spec.ts` and `kanban-api.spec.ts` encode the prior divergent implementation (Blocking for test suite, Noted for review)
- **Severity**: Blocking (tests will fail against spec-compliant impl) / Noted (test rewrite is Tester's job)
- **Criterion**: AC-002, AC-007, AC-011, AC-014, AC-015, AC-019, CON-005
- **Code**: `ui/e2e/kanban.spec.ts:3-11` (`EXPECTED_COLUMNS` includes `'backlog'` — 7 columns, spec says 6 + conditional Other); `:13-21` (`COLUMN_LABELS.backlog = 'Backlog'` — no such column in spec); `:119,122,144,159,162,175,178,191,265,279,294,299,319,339,342,366,389` (all use `feature-card-*` testids for board cards — impl emits `kanban-card-*`); `:145` (asserts `kanban-column-count-${key}` — no such testid in `KanbanColumn.tsx`); `:80` (`mockFeatures(page, [])` then `switchToBoard` — but Dashboard hides toggle and renders `EmptyState` when features empty, so `view-toggle-board` won't exist); `:152-166` ("AC-004: draft+inception → backlog" — spec has NO Backlog column; FR-006 says feature goes to column matching `current_phase`); `ui/e2e/kanban-api.spec.ts:80,81` (asserts `kanban-error` testid — impl uses `features-error`); `:45` (asserts `feature-card-k1` on board — impl emits `kanban-card-k1`).
- **Description**: Commit `6a7756d` rewrote the implementation to match the spec (replaced `groupFeaturesByColumn` with `groupFeaturesByPhase`, added `kanban-card-*` testids, removed Backlog column, made Board default) but did NOT update the e2e tests. The commit message admits this: "Existing kanban.spec.ts/kanban-api.spec.ts encode the prior divergent interpretation... Tester owns test rewrite." The tests as they stand will fail against the spec-compliant implementation. This is a blocker for the Testing phase (which will start from these tests) and a review finding because the tests are on the feature branch claiming to cover AC-001..AC-022.
- **Fix**: Tester must rewrite `kanban.spec.ts` and `kanban-api.spec.ts` to use spec-compliant selectors (`kanban-card-*`, 6 phase columns + conditional Other, `features-error`/`features-loading` testids, no `backlog`, no `kanban-column-count`, no `kanban-error`).

### F-004: AC-011 unit test missing — `KanbanBoard.test.ts` and `vitest` devDep absent (Required)
- **Severity**: Required
- **Criterion**: AC-011 (test level `unit`), CON-009, plan T-003/T-010
- **Code**: No `KanbanBoard.test.ts` or `groupFeaturesByPhase.test.ts` in `ui/src/components/`; `ui/package.json:23-32` has no `vitest`; no `test:unit` script in `package.json:6-13`
- **Description**: AC-011 explicitly requires a unit test: `groupFeaturesByPhase([{current_phase:'weird', ...}])` returns `{other: [feature]}`. The plan (T-003, `plan.md:2159-2163`; `plan.md:1838`) decided to add `vitest` as a devDep and create `KanbanBoard.test.ts`. Neither exists. The implementation code is correct (CON-009 trace), but the required verification artifact is missing. CON-009's verification method is "Unit test with synthetic unknown phase" — unsatisfied.
- **Fix**: Add `vitest` devDep, `test:unit` script, and `KanbanBoard.test.ts` (or co-located `groupFeaturesByPhase.test.ts`) covering: empty input → 6 empty buckets + empty other; known phase → correct bucket; unknown phase → other; partition sum invariant.

### F-005: Unused `PHASE_GROUP_KEYS` export (Noted)
- **Severity**: Noted (doesn't need fixing)
- **Criterion**: Over-engineering check
- **Code**: `groupFeaturesByPhase.ts:6` `export const PHASE_GROUP_KEYS: GroupKey[] = [...PHASES, 'other']`
- **Description**: Exported but not imported by `KanbanBoard.tsx` (which maps `PHASES` directly and appends `other` conditionally). Could be used by the missing unit test. Harmless but dead code today.
- **Fix**: None required. If unit test (F-004) is added and uses it, it becomes live. Otherwise delete.

### F-006: `kanban-api.spec.ts:80` asserts non-existent `kanban-error` testid (Noted — subsumed by F-003)
- **Severity**: Noted
- **Criterion**: AC-015
- **Code**: `ui/e2e/kanban-api.spec.ts:80,81`
- **Description**: Dashboard error branch uses `features-error` (`Dashboard.tsx:97`), not `kanban-error`. Test will fail. Subsumed by F-003's test-rewrite requirement.

---

## Quality Gate Checklist

| Gate item | Status |
|---|---|
| Every constraint checked with execution-path trace | ✅ CON-001..CON-009 traced |
| Every AC checked with quoted evidence | ✅ AC-001..AC-022 |
| Negative test vectors verified | ✅ (defensive edges; CON-009 code correct, test missing — F-004) |
| Cross-component consistency verified | ✅ all producers/consumers agree |
| "No issues" backed by evidence | N/A — issues found |
| Security review (P1) | ✅ N/A documented + verified |
| Null pointer safety | ✅ all buckets `[]` init, `data?.features ?? []`, `data?.total_count ?? 0` |
| Error paths verified | ✅ loading/error/empty branches traced |
| Middleware chain | N/A (no backend change) |
| Over-engineering check | ✅ minimal, ~257 lines |
| Missing implementation check | ✅ F-001, F-004 |
| Language footguns | ✅ no `any`, `===`, no `==`, no negative repeat |
| Execution paths traced | ✅ per constraint |
| Multi-component constraints across ALL components | ✅ `statusColors` in both cards |

---

## Verdict

**BLOCKED — 2 Blocking findings must be resolved before advancing to Testing.**

The implementation source code (`KanbanBoard.tsx`, `KanbanCard.tsx`, `KanbanColumn.tsx`, `ViewToggle.tsx`, `badgeColors.ts`, `groupFeaturesByPhase.ts`, `useSessionView.ts`, `Dashboard.tsx` wiring) is **spec-compliant and minimal**. All functional requirements FR-001..FR-017 are implemented correctly. The 18 reviewable ACs are MET at the code level.

The blockers are configuration/test drift from the mid-flight re-alignment commit `6a7756d`:
1. **F-002**: Playwright default port regressed to `:8765` (CON-001 violation) — revert to `:18765`.
2. **F-001**: `app.spec.ts` not updated for Board-default (CON-004 regression) — add click-to-List fixture (T-009, plan-assigned to developer).

Required (must address before Testing can pass):
3. **F-004**: AC-011 unit test + `vitest` devDep missing.
4. **F-003**: `kanban.spec.ts` / `kanban-api.spec.ts` encode the prior divergent impl — Tester owns rewrite, but flagged here so the Testing phase knows the starting tests are stale.

Once F-001 and F-002 are fixed, the feature can advance to Testing. F-003 and F-004 will surface as Testing-phase failures if not addressed first.