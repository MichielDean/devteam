# Review Report: kanban-view

**Feature ID**: kanban-view
**Phase**: review
**Reviewer**: reviewer
**Date**: 2026-06-21
**Priority**: P1

## Summary

- Acceptance criteria: 22 total (AC-001..014, AC-CON-003/005/006/008/011, AC-ERR-001/002/003)
- MET: 21
- NOT MET: 1 (AC-014 — live-update test uses `page.reload()`, violating "without a full page reload")
- Findings: 0 critical, 1 required, 2 noted

Implementation: single commit `a375f0b` on `feature/kanban-view` in `/home/lobsterdog/source/devteam/worktrees/kanban-view/devteam`. Diff is 7 files, 707 insertions / 27 deletions — all under `ui/`. No `internal/`, `cmd/`, `rules/` changes. No `package.json` changes. Build passes (`npm run build` verified — 476 modules transformed).

Implementation is the minimal lazy solution. New TSX/TS logic: 155 lines across 4 files (KanbanBoard 46, KanbanColumn 39, ViewToggle 33, groupFeaturesByColumn 37) — well under the plan's ~250-line ceiling. No drag-drop, no WIP limits, no new route, no new deps, no re-implementation of FeatureCard. The shortest path to done.

---

## Phase 1: Constraint Register Review

### CON-001 — Board columns are the 6 pipeline phases in canonical order
**Status**: MET
**Trace**:
1. `types/index.ts:171` — `PHASES = ['inception','planning','construction','review','testing','delivery']` (canonical source)
2. `groupFeaturesByColumn.ts:6` — `COLUMN_KEYS = ['backlog', ...PHASES]` → imports, does not re-declare
3. `KanbanBoard.tsx:36` — `COLUMN_KEYS.map(key => <KanbanColumn .../>)` → renders in that order
4. `KanbanColumn.tsx:17` — `data-testid={`kanban-column-${columnKey}`}` → testids derive from the same constant

No invented/reordered columns. Order flows from the single canonical constant.
**Evidence**: `groupFeaturesByColumn.ts:6`, `KanbanBoard.tsx:36`, `types/index.ts:171`

### CON-002 — Backlog = phase inception AND status draft
**Status**: MET
**Trace**:
1. `groupFeaturesByColumn.ts:30` — `if (f.status === 'draft' && f.current_phase === 'inception') { cols.backlog.push(f); }`
2. `groupFeaturesByColumn.ts:32-33` — else branch: `PHASES.includes(f.current_phase)` → pushes to `cols[f.current_phase]`; a feature in `inception` with `status !== 'draft'` falls to the `inception` column.
3. Paths traced: draft+inception → backlog ✓; in_progress+inception → inception ✓; any other status+inception → inception ✓; done+delivery → delivery ✓ (CON-009).

Rule matches spec's derived grouping exactly.
**Evidence**: `groupFeaturesByColumn.ts:30-34`

### CON-003 — Board data exclusively from GET /api/features; no new endpoint
**Status**: MET
**Trace**:
1. `KanbanBoard.tsx:2` — `import { listFeatures } from '../api/client'` (sole data import)
2. `KanbanBoard.tsx:11-14` — `useQuery({ queryKey: ['features'], queryFn: listFeatures })`
3. `git diff main...HEAD -- internal/ cmd/ rules/` → empty (verified: no backend diff)
4. `git diff main...HEAD -- ui/src/api/client.ts` → empty (no new client function)
5. E2E guard `kanban-api.spec.ts:37-50` — `page.on('request')` filters `/api/`, asserts every URL matches `/\/api\/features(\?|$)/`

No new endpoint. Only `listFeatures` consumed.
**Evidence**: `KanbanBoard.tsx:2,11-14`; `kanban-api.spec.ts:27-53`; git diff (empty for internal/ + client.ts)

### CON-004 — Empty features serialize as `[]` not `null`; board renders empty columns
**Status**: MET
**Trace**:
1. `internal/api/dto.go:93` — `summaries := make([]FeatureSummaryResponse, 0, len(features))` → empty slice, not nil, serializes to `[]`
2. `KanbanBoard.tsx:16` — `groupFeaturesByColumn(data?.features ?? [])` → defends undefined while loading
3. `groupFeaturesByColumn.ts:13-23` — `emptyColumns()` pre-initializes all 7 keys to `[]`
4. `groupFeaturesByColumn.ts:28` — `const cols = emptyColumns()` before iteration → no missing key possible
5. `KanbanColumn.tsx:30-33` — when `features.length === 0`, renders empty-state `<p>` (non-blank)

Double defense: DTO guarantees `[]`; `?? []` + pre-init cols defend even a null. No throw path on empty input.
**Evidence**: `internal/api/dto.go:93`, `KanbanBoard.tsx:16`, `groupFeaturesByColumn.ts:13-23,28`, `KanbanColumn.tsx:30-33`

### CON-005 — Reuse existing FeatureCard
**Status**: MET
**Trace**:
1. `KanbanColumn.tsx:1` — `import FeatureCard from './FeatureCard'`
2. `KanbanColumn.tsx:35` — `features.map(f => <FeatureCard key={f.id} feature={f} />)`
3. `FeatureCard.tsx` unchanged (not in diff) — renders title, status/phase/priority badges, gate indicator, updated date, `<Link to={/features/${id}}>` (line 29-30)

No re-implementation. Card markup reused as-is.
**Evidence**: `KanbanColumn.tsx:1,35`; `FeatureCard.tsx:29` (existing, unchanged)

### CON-006 — No new UI dependency
**Status**: MET
**Trace**:
1. `git diff main...HEAD -- ui/package.json` → empty (verified)
2. `git diff main...HEAD -- ui/vite.config.ts` → empty (no vitest config added — AD-6 resolved conservatively, no vitest)
3. `kanban-api.spec.ts:55-68` — AC-CON-006 test asserts board renders (only possible with existing bundle)

Zero new declared deps. Lockfile churn only from reinstall if any (not in diff).
**Evidence**: git diff (empty for package.json + vite.config.ts)

### CON-007 — Kanban reachable via navigation alongside Dashboard
**Status**: MET
**Trace**:
1. `Dashboard.tsx:15` — `useState<'list' | 'board'>('list')`
2. `Dashboard.tsx:68` — `<ViewToggle value={viewMode} onChange={setViewMode} />` in header
3. `Dashboard.tsx:87-112` — `viewMode === 'board' ? <KanbanBoard /> : <>...FeatureList...</>`
4. `ViewToggle.tsx:16,25` — `data-testid="view-toggle-list"` / `data-testid="view-toggle-board"` both rendered; either click sets `viewMode` via `onChange`

Both directions reachable. List is default (existing behavior preserved).
**Evidence**: `Dashboard.tsx:15,68,87-112`; `ViewToggle.tsx:16,25`

### CON-008 — Dark mode via existing Tailwind `dark:` variants
**Status**: MET
**Trace**:
1. `KanbanBoard.tsx:23,32` — `dark:text-red-400 dark:bg-red-900/30 dark:border-red-800` / `dark:text-gray-400`
2. `KanbanColumn.tsx:18,20,23,24,31` — `dark:bg-gray-900 dark:border-gray-700 dark:text-white dark:text-gray-300 dark:text-gray-400`
3. `ViewToggle.tsx:10,13` — `dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600 dark:bg-gray-800 dark:border-gray-700`
4. `FeatureCard.tsx` (unchanged) already uses `dark:` variants throughout
5. E2E `kanban.spec.ts:342-370` — AC-CON-008 toggles ThemeToggle, asserts column computed bg is not light palette

Consistent with existing `dark:` usage in Dashboard/FeatureCard.
**Evidence**: `KanbanBoard.tsx:23,32`; `KanbanColumn.tsx:18,20,23,31`; `ViewToggle.tsx:10,13`; `kanban.spec.ts:342-370`

### CON-009 — Terminal status features placed in current_phase column, not hidden
**Status**: MET
**Trace**:
1. `groupFeaturesByColumn.ts:30-34` — only the backlog branch checks `status === 'draft'`; the else branch indexes by `current_phase` with NO status filter
2. Path: `status === 'done' && current_phase === 'delivery'` → fails backlog branch (status ≠ draft) → `PHASES.includes('delivery')` true → `cols.delivery.push(f)` ✓
3. Same for `cancelled`: no status filter in the else branch → placed in current_phase column
4. E2E `kanban.spec.ts:184-195` — AC-006 verifies `done`+`delivery` lands in `kanban-column-delivery`

Terminal features visible. No exclusion filter.
**Evidence**: `groupFeaturesByColumn.ts:30-34`; `kanban.spec.ts:184-195`

### CON-010 — Total feature count badge remains visible and correct in Kanban view
**Status**: MET
**Trace**:
1. `Dashboard.tsx:58-66` — `feature-count-badge` is rendered in the header row, OUTSIDE the `viewMode === 'board' ? ... : ...` body switch (line 87)
2. `Dashboard.tsx:49` — `totalCount = data?.total_count ?? 0` derived from the shared `['features']` query
3. The badge visibility guard `!isLoading && !error` (line 58) reads the shared query state — both views use the same query key, so the badge renders identically in both modes
4. E2E `kanban.spec.ts:220-238` — AC-009 reads badge text before toggle, asserts same text after toggle

Badge stays mounted; count stays correct.
**Evidence**: `Dashboard.tsx:49,58-66,87`; `kanban.spec.ts:220-238`

### CON-011 — Stable `data-testid` attributes
**Status**: MET
**Trace**:
1. `KanbanBoard.tsx:19` — `data-testid="kanban-board"`
2. `KanbanColumn.tsx:17` — `data-testid={`kanban-column-${columnKey}`}` → derives from `COLUMN_KEYS` = backlog + 6 phases
3. All 8 required testids produced: `kanban-board`, `kanban-column-backlog`, `kanban-column-inception`, `kanban-column-planning`, `kanban-column-construction`, `kanban-column-review`, `kanban-column-testing`, `kanban-column-delivery`
4. E2E `kanban.spec.ts:95-106` — AC-CON-011 asserts each testid exists exactly once

All required testids present, exactly once each.
**Evidence**: `KanbanBoard.tsx:19`; `KanbanColumn.tsx:17`; `groupFeaturesByColumn.ts:6`; `kanban.spec.ts:95-106`

---

## Phase 2: Acceptance Criteria Review

### AC-001: features land in column matching current_phase
**Status**: MET
**Evidence**: `groupFeaturesByColumn.ts:30-34` (grouping rule); `kanban.spec.ts:108-127` (e2e: seeds features in inception/planning/delivery, asserts each card in matching column)
**Explanation**: Grouping fn routes non-backlog features to `cols[f.current_phase]`. Test verifies for 3 phases.

### AC-002: columns render in canonical order
**Status**: MET
**Evidence**: `KanbanBoard.tsx:36` (`COLUMN_KEYS.map`); `groupFeaturesByColumn.ts:6` (`COLUMN_KEYS = ['backlog', ...PHASES]`); `kanban.spec.ts:78-93` (asserts ordered suffixes === `['backlog','inception','planning','construction','review','testing','delivery']`)
**Explanation**: Render order driven by the constant; test asserts exact order.

### AC-003: column header shows label + card count
**Status**: MET
**Evidence**: `KanbanColumn.tsx:20-27` (header renders `<h3>{label}</h3>` + count span `{features.length}`); `kanban.spec.ts:129-150` (asserts header text contains label and count equals `feature-card-*` descendants)
**Explanation**: Count is `features.length` (line 26), test asserts it equals descendant card count for all 7 columns.

### AC-004: draft+inception → backlog, not inception
**Status**: MET
**Evidence**: `groupFeaturesByColumn.ts:30-31` (`if (f.status === 'draft' && f.current_phase === 'inception') cols.backlog.push(f)`); `kanban.spec.ts:152-166` (asserts card in `kanban-column-backlog`, NOT in `kanban-column-inception`)
**Explanation**: Rule explicitly routes draft+inception to backlog. Test verifies exclusion from inception column.

### AC-005: in_progress+inception → inception, not backlog
**Status**: MET
**Evidence**: `groupFeaturesByColumn.ts:30-33` (fails draft check → else branch → `cols.inception.push`); `kanban.spec.ts:168-182` (asserts card in `kanban-column-inception`, NOT in backlog)
**Explanation**: in_progress ≠ draft, so feature falls to the else branch and is placed by current_phase.

### AC-006: done+delivery → delivery (terminal not hidden)
**Status**: MET
**Evidence**: `groupFeaturesByColumn.ts:32-33` (else branch has no status filter); `kanban.spec.ts:184-195`
**Explanation**: See CON-009 trace. Terminal features routed by current_phase only.

### AC-007: list → board toggle
**Status**: MET
**Evidence**: `Dashboard.tsx:87-112` (viewMode switch); `ViewToggle.tsx:25` (`view-toggle-board` button); `kanban.spec.ts:197-207` (asserts `feature-list` visible → click → `kanban-board` visible, `feature-list` count 0)
**Explanation**: Toggle switches body content; test asserts visibility transition.

### AC-008: board → list toggle
**Status**: MET
**Evidence**: `Dashboard.tsx:87-112`; `ViewToggle.tsx:16` (`view-toggle-list` button); `kanban.spec.ts:209-218` (asserts `feature-list` visible, `kanban-board` count 0)
**Explanation**: Reverse toggle works symmetrically.

### AC-009: count badge stays consistent across toggle
**Status**: MET
**Evidence**: `Dashboard.tsx:58-66` (badge in header, outside body switch); `kanban.spec.ts:220-238` (reads badge text before, asserts same text after toggle)
**Explanation**: Badge is in the header row which is never unmounted by the body switch. Same query → same count.

### AC-010: clicking a card navigates to /features/{id}
**Status**: MET
**Evidence**: `KanbanColumn.tsx:35` (`<FeatureCard feature={f} />`); `FeatureCard.tsx:29-30` (`<Link to={`/features/${feature.id}`}>`); `kanban.spec.ts:240-269` (clicks card, asserts URL `/features/nav1`)
**Explanation**: FeatureCard renders a `<Link>`; clicking it uses react-router navigation. Test verifies URL.

### AC-011: empty board renders all columns with empty-state, no console errors
**Status**: MET
**Evidence**: `groupFeaturesByColumn.ts:13-23` (pre-init all 7 cols to `[]`); `KanbanColumn.tsx:30-33` (empty-state `<p>`); `kanban.spec.ts:271-283` (mocks `features: []`, asserts empty-state visible + zero `feature-card-*` for all 7 columns + zero console errors/pageerrors)
**Explanation**: Empty input produces 7 empty columns each with non-blank empty-state message. Console captured and asserted clean.

### AC-012: features:[] does not throw "map of null"
**Status**: MET
**Evidence**: `KanbanBoard.tsx:16` (`data?.features ?? []`); `groupFeaturesByColumn.ts:13-23,28` (pre-init + `emptyColumns()` call before iteration)
**Explanation**: Level reclassified from unit to e2e per plan AD-6 (CON-006 forbids adding vitest devDep). Behavior verified by AC-011's e2e (empty array path exercised, no throw, no console error). The grouping function never indexes a missing key and never calls `.map` on a possibly-null array.

### AC-013: partial fill — one column has cards, others empty
**Status**: MET
**Evidence**: `kanban.spec.ts:285-304` (seeds 5 in planning, asserts planning has 5 cards, every other column has 0 cards + visible empty-state)
**Explanation**: Grouping fn routes all 5 to planning bucket; other buckets stay `[]` → empty-state renders.

### AC-014: cache invalidation moves card without full page reload
**Status**: NOT MET
**Evidence**: `kanban.spec.ts:306-340` — the test calls `await page.reload()` (line 329) and re-clicks the board toggle (line 330) to observe the moved card. The AC text requires the card to move "without a full page reload."
**Explanation**: The implementation itself supports this — `KanbanBoard` uses `useQuery(['features'])` (KanbanBoard.tsx:11-14), the same cache key as Dashboard, so a real invalidation propagates without reload. The PRODUCTION CODE is correct. But the TEST does not verify the AC's actual constraint: it reloads the page, which is exactly what the AC forbids. The test comment (lines 323-328) acknowledges this ("in spirit", "the test exercises the data path, not the invalidation trigger") — but an AC verification that violates the AC's own constraint is not a verification. A correct test would trigger `queryClient.invalidateQueries(['features'])` via a real mutation (e.g. create a second feature through the IntakeForm, which invalidates the cache) or expose a test hook, then assert the card moved with `page.url()` unchanged and no `page.reload()` call. This is a TEST gap, not an implementation gap — the board code satisfies FR-014.

### AC-CON-003: no new backend endpoint
**Status**: MET
**Evidence**: git diff (empty for `internal/`, `ui/src/api/client.ts`); `kanban-api.spec.ts:27-53` (asserts every `/api/` request matches `/\/api\/features(\?|$)/`)
**Explanation**: No new mux handler, no new client function, board only calls `listFeatures`. Test guards against any new endpoint at runtime.

### AC-CON-005: reuse FeatureCard
**Status**: MET
**Evidence**: `KanbanColumn.tsx:1` (`import FeatureCard from './FeatureCard'`); `KanbanColumn.tsx:35` (`<FeatureCard key={f.id} feature={f} />`)
**Explanation**: Level reclassified from unit to integration per plan AD-6. Source grep confirms import + render usage. No card markup re-implemented.

### AC-CON-006: no new UI dependency
**Status**: MET
**Evidence**: git diff (empty for `ui/package.json`); `kanban-api.spec.ts:55-68` (board renders with existing bundle)
**Explanation**: Zero additions in dependencies or devDependencies.

### AC-CON-008: dark mode renders dark-palette backgrounds
**Status**: MET
**Evidence**: `KanbanColumn.tsx:18` (`dark:bg-gray-900`); `KanbanBoard.tsx:23` (`dark:bg-red-900/30`); `kanban.spec.ts:342-370` (toggles ThemeToggle, asserts column bg is not light palette)
**Explanation**: Dark variants applied to board + columns + toggle. Test asserts computed bg is not the light palette.

### AC-CON-011: each testid exists exactly once
**Status**: MET
**Evidence**: `KanbanBoard.tsx:19` (`kanban-board`); `KanbanColumn.tsx:17` (`kanban-column-${columnKey}`); `kanban.spec.ts:95-106` (asserts `toHaveCount(1)` for board + all 7 columns)
**Explanation**: All 8 required testids present exactly once.

### AC-ERR-001: API 500 renders error banner, no pageerror
**Status**: MET
**Evidence**: `KanbanBoard.tsx:20-28` (`error && !data` → banner with `data-testid="kanban-error"` containing `Failed to load features: {error.message}`); `kanban-api.spec.ts:70-88` (mocks 500, asserts banner visible + contains "Failed to load features" + `pageErrors` empty)
**Explanation**: Error banner renders on 500 with no data. Columns still render (grouped from `[]` fallback). No uncaught exception.

### AC-ERR-002: refetch error keeps board stable, no pageerror
**Status**: MET
**Evidence**: `KanbanBoard.tsx:20` (`error && !data` — when data exists, banner does NOT render; stale cards remain via `groupFeaturesByColumn(data?.features ?? [])`); `kanban-api.spec.ts:90-120` (loads successfully, then fails next fetch, asserts `pageErrors` empty + `errors` empty)
**Explanation**: react-query keeps `data` populated on refetch error by default; the `!data` guard means stale cards stay visible. Test asserts no uncaught exception. AC allows "either" stale-or-banner.

### AC-ERR-003: deleted card click → FeatureDetail 404 state
**Status**: MET
**Evidence**: `FeatureCard.tsx:29` (`<Link to={`/features/${feature.id}`}>`); `kanban.spec.ts:372-387` (mocks `/api/features/gone1` with 404, clicks card, asserts URL `/features/gone1` + no console error)
**Explanation**: Board does not handle the 404 itself — navigates to detail page, which renders its existing not-found state. Test asserts no console error.

---

## Phase 3: Negative Test Vector Verification

The constraint register has no external-standard negative vectors (no RFC). The "negative vectors" are the empty-state + error-path ACs (per plan spec validation row). All verified:

| Vector | Expected behavior | Verified by |
|--------|-------------------|-------------|
| `features: []` (CON-004) | 7 empty cols, no throw | AC-011 e2e — MET |
| `GET /api/features` 500 | error banner, no crash | AC-ERR-001 — MET |
| Refetch error mid-session | stale cards or banner, no crash | AC-ERR-002 — MET |
| Click deleted card | detail page 404 state | AC-ERR-003 — MET |
| Unknown `current_phase` | dropped, no throw | `groupFeaturesByColumn.ts:32-35` — `PHASES.includes(...)` guard; else branch is a no-op (comment line 35). Static reasoning: a feature with `current_phase = 'foo'` fails the backlog check and fails `PHASES.includes`, so it is silently dropped. No throw, no missing-key index. Not exercised by e2e (only valid phases seeded) but the guard is present and correct. |

No negative vector accepted or thrown.

---

## Phase 4: Cross-Component Consistency Review

| Shared Value | Producer | Consumer | Consistent? |
|--------------|----------|----------|-------------|
| Phase wire values | Go `internal/feature/types.go` → API `current_phase` | `types/index.ts:171` `PHASES`; `groupFeaturesByColumn.ts:32` matches against them | YES — `PHASES` mirrors Go enum; grouping imports `PHASES`, not a re-declaration |
| Status wire values | Go Status enum → API `status` | `groupFeaturesByColumn.ts:30` checks `=== 'draft'`; `FeatureCard.tsx:13` uses `STATUS_LABELS` | YES — string literal `'draft'` matches Go `StatusDraft = "draft"`; `STATUS_LABELS` covers all 9 statuses |
| Column key set | `groupFeaturesByColumn.ts:6` `COLUMN_KEYS` | `KanbanColumn.tsx:17` testid; `KanbanBoard.tsx:36` render order; e2e selectors | YES — single source of truth |
| `FeatureSummary` shape | `GET /api/features` → `types/index.ts:3` | `FeatureCard.tsx` props; `groupFeaturesByColumn.ts` reads `f.status`, `f.current_phase` | YES — board reads only fields the existing types define |
| Empty-array contract | `internal/api/dto.go:93` (`make([]..., 0, ...)`) | `KanbanBoard.tsx:16` (`?? []`); `groupFeaturesByColumn.ts:13-23` (pre-init) | YES — double defense |
| react-query cache key | `Dashboard.tsx:21` `useQuery(['features'])` | `KanbanBoard.tsx:12` `useQuery(['features'])` | YES — identical key → shared cache (FR-014) |

No inconsistencies. Every shared value has a single producer; all consumers read from it.

---

## Phase 5: Language-Specific Footgun Review (TypeScript)

| Footgun | Location | Risk | Verdict |
|---------|----------|------|---------|
| `any` type | none in new code | — | Clean: all types from `types/index.ts` |
| `==` vs `===` | `groupFeaturesByColumn.ts:30` uses `===` | none | Correct — strict equality on string literals |
| Optional chaining hiding null | `KanbanBoard.tsx:16` `data?.features ?? []` | guarded with `?? []` | Correct — `data` may be undefined while loading; `?? []` ensures grouping always gets an array |
| `as` cast producing undefined key | `groupFeaturesByColumn.ts:32-33` `f.current_phase as PhaseName` then `cols[f.current_phase as ColumnKey]` | could index Record with a non-key → runtime `undefined` → `.push` on undefined throws | **Guarded**: `PHASES.includes(f.current_phase as PhaseName)` (line 32) gates the cast+index. If the cast lies (phase not in PHASES), the else branch is skipped (line 35 comment) — no index on a missing key. Safe. |
| `.map` on possibly-null | none — `groupFeaturesByColumn` uses `for...of`; `KanbanBoard` maps `COLUMN_KEYS` (constant, never null) | none | Correct |
| Mutable default args | none (no functions with default args) | — | N/A |

No language footgun produces wrong behavior. The `as` cast is correctly guarded by `PHASES.includes` — exactly as the plan's agent failure mode checks required.

---

## Phase 6: Over-Engineering Check

| File | Lines | Expected | Verdict |
|------|-------|----------|---------|
| `groupFeaturesByColumn.ts` | 37 | ~30 | Minimal — pure fn + 2 constants + emptyColumns helper |
| `KanbanBoard.tsx` | 46 | ~50 | Minimal — query + grouping + render 7 columns + error/loading |
| `KanbanColumn.tsx` | 39 | ~40 | Minimal — header + count + card list + empty-state |
| `ViewToggle.tsx` | 33 | ~30 | Minimal — 2 buttons + active styling |
| `Dashboard.tsx` diff | +39/-27 | ~+30 | Minimal — adds viewMode state + toggle + body switch |
| **Total new logic** | **155** | plan ceiling ~250 | **Under budget** |

No speculative abstractions. No interface with one implementation. No factory. No config for constant values. No drag-drop, no WIP limits, no column collapse, no animation, no virtualization, no URL query param, no localStorage persistence. The implementation is the lazy minimum that satisfies every done condition.

Test files: `kanban.spec.ts` (388 lines) + `kanban-api.spec.ts` (121 lines) = 509 lines. These are test-only, one test per AC, no shared fixture extravagance. Appropriate for 22 ACs.

---

## Phase 7: Missing Implementation Check

| User Story | Implemented? | Evidence |
|------------|--------------|----------|
| US-001 (board by phase) | YES | KanbanBoard + grouping fn + AC-001/002/003 |
| US-002 (backlog) | YES | Backlog rule + AC-004/005 |
| US-003 (toggle) | YES | ViewToggle + Dashboard wiring + AC-007/008/009 |
| US-004 (click card) | YES | FeatureCard reuse + AC-010 |
| US-005 (empty state) | YES | emptyColumns + empty-state msg + AC-011/012/013 |
| US-006 (live updates) | YES (code) / TEST GAP (AC-014) | `useQuery(['features'])` shares cache (KanbanBoard.tsx:12) — code satisfies FR-014. Test reloads page instead of invalidating. |

No missing implementation. Every user story has corresponding code. The only gap is the AC-014 test method, not the implementation.

---

## Phase 8: Security Review (P1)

| Check | Status | Evidence |
|-------|--------|----------|
| Authentication | N/A — no new endpoints; board uses existing `GET /api/features` with existing auth model | No backend changes (git diff empty for `internal/`) |
| Authorization | N/A — no new resources; board is read-only over existing data | `KanbanBoard.tsx` only reads |
| Input validation | N/A — board sends no user input to backend; `listFeatures()` is a GET with no params | `api/client.ts:51-52` unchanged |
| Output filtering | N/A — board renders `FeatureSummary` fields already exposed by the existing API | No new fields rendered |
| Error messages | OK — error banner shows `error.message` from the API response, not stack traces or file paths | `KanbanBoard.tsx:26` |
| CORS | N/A — no backend changes | Unchanged |
| Rate limiting | N/A — no new endpoints | Unchanged |
| Logging | OK — no secrets logged; board is client-only | No new logging code |
| Security headers | N/A — no backend changes | Unchanged |
| XSS | OK — all feature data rendered via React JSX (auto-escaped); `FeatureCard` uses `{feature.title}` etc. | `FeatureCard.tsx:39,51,57,63,78` |

No security issues. The feature adds no attack surface — it is a read-only view over existing authenticated data, rendering via React's auto-escaping.

---

## Phase 9: Constitution / Pipeline Principles

- **Quality built in**: grouping fn defends empty/null at the source; error banner + stale-data path handle API failure.
- **Proof of work**: every MET claim above cites specific file:line + test:line.
- **Agent failure modes checked**: null-array (CON-004), nil-deref (`?? []`), `as` cast guarded by `PHASES.includes`, multi-component consistency (single grouping fn, no render-time filtering).
- **Conservative defaults**: AD-6 resolved the unit-test-runner tension conservatively (no vitest devDep, violating CON-006 would be worse). Empty-state copy chosen per plan. List is default view.

---

## Findings

### F-001: AC-014 test uses `page.reload()`, violating the AC's "without a full page reload" constraint
- **Severity**: required (test gap, not implementation defect)
- **Criterion**: AC-014
- **Code**: `ui/e2e/kanban.spec.ts:329` — `await page.reload();`
- **Description**: The test reloads the page to observe the card move from inception to planning. AC-014 requires the card to move "without a full page reload." The production code supports this (shared `['features']` cache key, FR-014), but the test does not verify the actual constraint. The test's own comment (lines 323-328) acknowledges the deviation.
- **Fix needed**: Replace `page.reload()` with a real cache invalidation — e.g. create a second feature via the IntakeForm (which calls `queryClient.invalidateQueries(['features'])` in `Dashboard.tsx:31`), or expose a test hook, then assert the card moved with `page.url()` unchanged. The Tester phase should catch this during testing — flagging here so it is not missed.

### F-002 (noted): AC-012 and AC-CON-005 test levels reclassified from unit to e2e/integration
- **Severity**: noted (spec tension, resolved conservatively per plan AD-6)
- **Criterion**: AC-012, AC-CON-005
- **Code**: plan.md AD-6
- **Description**: acceptance.md labels these as "unit" but CON-006 forbids adding the `vitest` devDep that true unit tests require. The architect resolved this conservatively (no vitest) and surfaced it as an open question. The implementation is structured correctly (pure grouping fn is unit-testable if vitest is later added). This is a spec-level tension, not an implementation defect. The Tester phase should note the reclassification in the test report.

### F-003 (noted): Unknown `current_phase` silently dropped — no test covers the defensive guard
- **Severity**: noted (defensive code is correct, but untested)
- **Criterion**: plan agent failure mode check ("Unknown `current_phase` value → dropped, no throw")
- **Code**: `groupFeaturesByColumn.ts:32-35` — `PHASES.includes(...)` guard + comment
- **Description**: The grouping fn correctly drops features with an unknown `current_phase` (no throw, no missing-key index). This is defensive code for a case that "should not happen given types.go enum." No e2e test seeds an unknown phase to verify the drop. The guard is present and correct by static reasoning; a unit test would cover it if vitest were added (see F-002).

---

## Quality Gate

| Gate item | Status |
|-----------|--------|
| Every constraint checked with execution path trace + quoted evidence | ✅ CON-001..011 all traced |
| Every acceptance criterion checked with quoted evidence | ✅ AC-001..014, AC-CON-*, AC-ERR-* all checked |
| Every negative test vector verified | ✅ All empty/error paths verified; unknown-phase drop verified by static reasoning |
| Cross-component consistency verified across all producers/consumers | ✅ 6 shared values, all consistent |
| "No issues" includes evidence | ✅ Every MET cites file:line + test:line |
| Security review complete (P1) | ✅ No attack surface added |
| Constitution compliance verified | ✅ Conservative defaults, proof of work, agent failure modes checked |
| Null pointer safety verified | ✅ `?? []`, pre-init cols, `PHASES.includes` guard |
| Error paths verified | ✅ 500 (AC-ERR-001), refetch error (AC-ERR-002), 404 (AC-ERR-003), empty (AC-011/013) |
| Middleware chain | N/A — no backend changes |
| Execution paths traced | ✅ Every constraint has an input-to-output trace |
| Language footguns checked | ✅ `as` cast guarded, `===` used, `?? []` defends undefined |
| Multi-component constraints across ALL components | ✅ Single grouping fn is the only place filtering happens; all 7 columns render from it |
| Over-engineering check | ✅ 155 lines new logic vs ~250 ceiling |
| Missing implementation check | ✅ All 6 user stories implemented |

**Gate result**: PASS with one required finding (F-001: AC-014 test method). The implementation is correct; the test must be fixed to verify the AC's actual constraint. No critical findings. No implementation defects. F-001 is a test-quality issue for the Tester phase to address, not a recirculate-to-construction issue.

**Recommendation**: Advance to testing phase. The Tester must fix the AC-014 test to use cache invalidation instead of `page.reload()` before the testing gate can pass.