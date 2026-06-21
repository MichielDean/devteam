# Plan: Kanban View

**Feature ID**: kanban-view
**Phase**: planning
**Architect**: architect
**Created**: 2026-06-21

## Summary

Add a read-only Kanban board view to the Dev Team web UI that groups the existing `FeatureSummary` list into 7 columns (Backlog + 6 pipeline phases) by `current_phase`/`status`. Reuses `FeatureCard`, `listFeatures()`, the `['features']` react-query cache, and Tailwind dark-mode variants. No backend changes, no new dependencies. Navigation is a view-toggle in the Dashboard header so the count badge stays mounted across both views (satisfies CON-010 / FR-007).

## Spec Validation

| Check | Result |
|-------|--------|
| Completeness — all FRs trace to user stories | PASS — FR-001..014 map to US-001..006 |
| Constraint register exists, every constraint addressable | PASS — CON-001..011 all covered below |
| Consistency — requirements contradict? | PASS — no contradictions |
| Feasibility with stated stack | PASS — React 19 + react-router 7 + react-query 5 + Tailwind 4 already installed |
| Edge cases defined (empty, error, mid-flight) | PASS — Error Scenarios table + AC-ERR-001..003 + AC-011..013 |
| Negative vectors converted to ACs | N/A — no external standard; "negative vectors" here are the empty-state + error-path ACs (CON-004 → AC-011/012) |
| Ambiguities | No unresolved NEEDS-CLARIFICATION. Architect resolves one open decision: **view-toggle in Dashboard vs separate route** → view-toggle (see Architecture Decision below) |

## Technical Context

| Aspect | Value |
|--------|-------|
| Language | TypeScript (UI), Go (backend — unchanged) |
| Framework | React 19.1, react-router 7.6, @tanstack/react-query 5.80 |
| Styling | Tailwind CSS 4.1 (`dark:` variants already in use) |
| Build | Vite 6.3 |
| Test | Playwright 1.61 (e2e/integration via route interception); **vitest added for unit** (see Open Decision) |
| Backend | Go `devteam` binary serving `GET /api/features` — unchanged |
| New runtime deps | **None** (CON-006/FR-011). vitest is a devDependency — see Open Decision. |

## Project Structure

All changes in `devteam` repo (single-repo feature per `repos.yaml`).

```
ui/src/
  pages/
    Dashboard.tsx          [MODIFY] — add view-toggle state, render KanbanBoard OR FeatureList in same page shell so count badge stays mounted
  components/
    KanbanBoard.tsx        [CREATE] — board container, fetches via useQuery(['features']), renders 7 KanbanColumn, error banner, loading spinner
    KanbanColumn.tsx       [CREATE] — column header (name + count) + card list + empty-state message, data-testid kanban-column-{key}
    ViewToggle.tsx         [CREATE] — segmented control "List | Board", data-testid view-toggle-list / view-toggle-board
  lib/
    groupFeaturesByColumn.ts   [CREATE] — pure grouping function (unit-tested)
    groupFeaturesByColumn.test.ts [CREATE] — vitest unit tests (AC-012, AC-CON-005 contract)
ui/e2e/
  kanban.spec.ts          [CREATE] — all e2e ACs (AC-001..011,013,014, AC-CON-008/011, AC-ERR-003)
  kanban-api.spec.ts      [CREATE] — integration ACs (AC-CON-003, AC-CON-006, AC-ERR-001, AC-ERR-002)
ui/package.json           [MODIFY] — add vitest devDependency ONLY if Open Decision resolves to "add vitest"
ui/vite.config.ts         [MODIFY] — add vitest config block (test environment jsdom) ONLY if Open Decision resolves to add vitest
```

No files under `internal/`, `cmd/`, or `rules/` are touched.

## Architecture Decisions

### AD-1: View-toggle in Dashboard (not separate route)

**Decision**: Render `KanbanBoard` and `FeatureList` inside the same `Dashboard` page, toggled by a `viewMode` state (`'list' | 'board'`). Do NOT add a `/kanban` route.

**Why**: The count badge (`feature-count-badge`) lives in the Dashboard header. Keeping both views in one page shell means the badge stays mounted across toggles → trivially satisfies CON-010/FR-007/AC-009 (badge text remains N). A separate route would require lifting the badge to `App.tsx` and duplicating the loading/error logic, adding code for no benefit.

**Trade-off**: The URL does not distinguish views (`/` for both). Acceptable — the spec explicitly leaves route-vs-toggle to the architect, and the board is an alternate presentation of the same data, not a distinct resource.

### AD-2: Group in a pure function, not inside the component

**Decision**: Extract grouping to `groupFeaturesByColumn(features: FeatureSummary[]): Record<ColumnKey, FeatureSummary[]>`. The component calls it; the function is unit-testable in isolation.

**Why**: AC-012 requires a unit test of the grouping function with `[]` input. Co-locating the logic in the component makes that test require a render. A pure function is the minimal, testable unit.

### AD-3: Column set is a constant derived from `PHASES`

**Decision**: Define `COLUMN_KEYS = ['backlog', ...PHASES] as const` and `COLUMN_LABELS = { backlog: 'Backlog', ...PHASE_LABELS }`. Do not re-declare the 6 phases — spread the canonical `PHASES`/`PHASE_LABELS` from `types/index.ts`.

**Why**: CON-001 is "no invented or reordered columns." Importing the canonical constant guarantees order and values match the source. Re-declaring would let drift slip in.

### AD-4: Backlog rule = `status === 'draft' && current_phase === 'inception'`

**Decision**: Implement exactly the spec's derived grouping rule. A feature with `current_phase === 'inception'` AND `status === 'draft'` → Backlog; same phase but any other status → Inception column. All other phases → column matching `current_phase` regardless of status (CON-009: terminal `done`/`cancelled` stay visible in their phase).

### AD-5: Error/loading states mirror Dashboard

**Decision**: Reuse the existing loading spinner markup and error banner pattern from `Dashboard.tsx`. On `error`, render a board-level banner `"Failed to load features: {message}"` with the 7 columns still rendered empty (AC-ERR-001). On refetch error mid-session, keep stale cards visible (AC-ERR-002 "either is acceptable" — choose stale-data option because react-query keeps `data` populated on refetch error by default).

### AD-6: Open Decision — unit-test runner

**Context**: AC-012 and AC-CON-005 specify **unit** test level for the grouping function and the `FeatureCard` import contract. The repo currently has **no unit-test runner** (only Playwright). Adding vitest means a new devDependency.

**Tension with CON-006/FR-011**: "no new UI dependency added to `package.json`." CON-006 is scoped to **runtime** deps (`dependencies` block) per AC-CON-006 verification: "no additions in the `dependencies` or `devDependencies` blocks." The spec's verification text literally forbids devDependency additions too.

**Conservative resolution**: Do NOT add vitest. Satisfy AC-012 and AC-CON-005 via **Playwright route-interception tests** instead of true unit tests. The grouping function is still extracted as a pure function (AD-2) so it *could* be unit-tested later, but the AC-012 assertion ("no throw on `[]`") is verifiable by loading the board with a mocked empty API response (already covered by AC-011's e2e). AC-CON-005 ("imports FeatureCard") is verifiable by a static source grep/diff — an integration-level check.

**Cost**: The acceptance criteria say "unit" but the constraint register forbids the dep that would make true unit tests possible. This is a spec tension. The architect resolves it conservatively (no new dep) and surfaces it here. The Tester phase should treat AC-012/AC-CON-005 as integration/e2e-level verifiable and note the level reclassification in the test report.

**If the human overrides**: Add `vitest` + `@vitest/ui` + `jsdom` as devDependencies and a `test:unit` script; the pure function is ready to test.

## Component Design

### Component: `groupFeaturesByColumn` (pure function)
**Purpose**: Map a `FeatureSummary[]` into 7 column buckets.
**Responsibilities**:
- Apply the Backlog rule (AD-4).
- Guarantee every column key exists (empty array, never undefined) — defends against null-array crashes (CON-004/AC-012).
- Preserve input order within each column (no re-sort; sorting is out of scope per spec).
**Interface**:
- `groupFeaturesByColumn(features: FeatureSummary[]): Record<ColumnKey, FeatureSummary[]>`
- `ColumnKey = 'backlog' | PhaseName`
**Dependencies**: `PHASES`, `PhaseName` from `types/index.ts`.

### Component: `KanbanBoard`
**Purpose**: Top-level board surface.
**Responsibilities**:
- `useQuery({ queryKey: ['features'], queryFn: listFeatures })` — reuses the existing cache key (FR-014).
- Call `groupFeaturesByColumn(data?.features ?? [])`.
- Render loading spinner while `isLoading`.
- Render error banner `"Failed to load features: {error.message}"` while `error` and no `data`.
- Render 7 `KanbanColumn` children in `COLUMN_KEYS` order inside a horizontally-scrollable flex row.
- Expose `data-testid="kanban-board"`.
**Interface**: no props (fetches its own data).
**Dependencies**: `listFeatures`, `useQuery`, `groupFeaturesByColumn`, `KanbanColumn`.

### Component: `KanbanColumn`
**Purpose**: One column.
**Responsibilities**:
- Header: column label + card count (FR-008).
- Body: list of `FeatureCard` for each feature in `features` (CON-005/FR-005).
- Empty state: non-blank message when `features.length === 0` (FR-009). Backlog uses "No features waiting to start"; others "No features in this phase".
- Dark-mode classes on container, header, body (CON-008/FR-010).
- Expose `data-testid="kanban-column-{key}"`.
**Interface**: `{ columnKey: ColumnKey; label: string; features: FeatureSummary[] }`.
**Dependencies**: `FeatureCard`.

### Component: `ViewToggle`
**Purpose**: Segmented control to switch Dashboard content between list and board.
**Responsibilities**:
- Two buttons "List" / "Board"; active state styled.
- Expose `data-testid="view-toggle-list"`, `data-testid="view-toggle-board"`.
- Controlled component (state owned by Dashboard).
**Interface**: `{ value: 'list' | 'board'; onChange: (v) => void }`.
**Dependencies**: none.

### Component: `Dashboard` (modified)
**Purpose**: Existing page; now hosts the view toggle and switches body content.
**Responsibilities added**:
- `const [viewMode, setViewMode] = useState<'list' | 'board'>('list')`.
- Render `ViewToggle` in the header row next to the count badge.
- Body: `viewMode === 'list'` → existing `FeatureList`/`EmptyState`; `viewMode === 'board'` → `KanbanBoard`.
- Keep the count badge, loading, and error banner at the page level for the **list** view (unchanged). The **board** view owns its own loading/error because it renders from the same `['features']` query — but the badge stays mounted because it's in the header.
**Dependencies added**: `ViewToggle`, `KanbanBoard`.

### Component Dependency Map
```
Dashboard ─┬─> ViewToggle
           └─> KanbanBoard ─┬─> KanbanColumn ─> FeatureCard
                            └─> groupFeaturesByColumn
KanbanColumn ─> FeatureCard (existing)
groupFeaturesByColumn ─> types (PHASES)
```
No cycles. `FeatureCard` is reused unchanged (CON-005).

## Data Model

No new persistent entities (per spec). The board is a derived view.

### Derived entity: Column
```
Column:
  key: ColumnKey ('backlog' | 'inception' | 'planning' | 'construction' | 'review' | 'testing' | 'delivery')
  label: string
  features: FeatureSummary[]   // derived, never null/undefined
```
**Integrity rule**: every `ColumnKey` always present in the `Record`, value always an array (possibly empty). This is the CON-004 defense.

### Grouping rule (authoritative)
```ts
function groupFeaturesByColumn(features: FeatureSummary[]): Record<ColumnKey, FeatureSummary[]> {
  const cols = { backlog: [], inception: [], planning: [], construction: [], review: [], testing: [], delivery: [] } as Record<ColumnKey, FeatureSummary[]>;
  for (const f of features) {
    if (f.status === 'draft' && f.current_phase === 'inception') {
      cols.backlog.push(f);
    } else if (PHASES.includes(f.current_phase as PhaseName)) {
      cols[f.current_phase as ColumnKey].push(f);
    }
    // else: unknown phase — drop (defensive; should not happen given types.go enum)
  }
  return cols;
}
```
Every feature lands in exactly one column (CON-009: terminal statuses fall through to the `current_phase` branch).

### State transitions
None introduced. Feature state machine stays in `internal/feature/feature.go`. The board only observes.

## API Contracts

**No new endpoints** (CON-003/FR-004/AC-CON-003). The board consumes the existing one:

### `GET /api/features` (existing, unchanged)
**Response 200**:
```json
{ "features": FeatureSummary[], "total_count": number }
```
`features` is `[]` (never `null`) when empty — already guaranteed by `internal/api/dto.go` (CON-004).

**Response 500** (error path, AC-ERR-001):
```json
{ "error": "internal_error", "details": "..." }
```
Board renders `"Failed to load features: {details}"` banner.

No request schema (GET). No new error codes. No new DTOs. The board's `listFeatures()` call is the same one `Dashboard` already makes.

## Constraint Verification Map

| CON-ID | Design Decision | Component(s) | Verification Checkpoint | Test Type |
|--------|-----------------|--------------|------------------------|-----------|
| CON-001 | Column set = `['backlog', ...PHASES]` imported from canonical `types/index.ts`; rendered in that order | `groupFeaturesByColumn`, `KanbanBoard`, `KanbanColumn` | AC-002: ordered `data-testid` suffixes == `['backlog','inception','planning','construction','review','testing','delivery']` | e2e |
| CON-002 | Backlog bucket = `status==='draft' && current_phase==='inception'`; Inception bucket = `current_phase==='inception' && status!=='draft'` | `groupFeaturesByColumn` | AC-004 (draft→backlog, not inception) + AC-005 (in_progress→inception, not backlog) | e2e |
| CON-003 | Board imports `listFeatures` from `api/client.ts`; no new route in `internal/api/server.go`; no new client fn | `KanbanBoard` | AC-CON-003: `git diff main -- internal/api/server.go ui/src/api/client.ts` shows no new mux HandleFunc / no new client fn; board source imports only `listFeatures` for data | integration |
| CON-004 | Grouping fn initializes all 7 keys to `[]`; iterates `data?.features ?? []`; never indexes a missing key | `groupFeaturesByColumn`, `KanbanBoard` | AC-012 (no throw on `[]` — verified via e2e empty-state AC-011, level reclassified per AD-6) + AC-011 (all columns render empty-state, zero console errors) | e2e (reclassified from unit — see AD-6) |
| CON-005 | `KanbanColumn` imports and renders existing `FeatureCard` for each card; no re-implementation | `KanbanColumn` | AC-CON-005: board source contains `import FeatureCard` and `<FeatureCard .../>`; verified by source grep | integration (reclassified from unit — see AD-6) |
| CON-006 | Zero new entries in `ui/package.json` `dependencies` or `devDependencies` | `package.json` | AC-CON-006: `git diff main -- ui/package.json` shows no additions in dep blocks | integration |
| CON-007 | `ViewToggle` in Dashboard header toggles `viewMode`; both views reachable from each other | `Dashboard`, `ViewToggle` | AC-007 (list→board) + AC-008 (board→list) | e2e |
| CON-008 | Board/column/card use Tailwind `dark:` variants mirroring `FeatureCard`/`Dashboard` | `KanbanBoard`, `KanbanColumn` | AC-CON-008: dark-mode computed bg on board + column matches dark palette | e2e |
| CON-009 | Terminal statuses (`done`,`cancelled`) fall through to `current_phase` branch — no status filter excludes them | `groupFeaturesByColumn` | AC-006: `done`+`delivery` feature in `kanban-column-delivery` | e2e |
| CON-010 | Count badge lives in Dashboard header, outside the view-toggle body — stays mounted across toggles | `Dashboard` | AC-009: badge text unchanged after list→board switch | e2e |
| CON-011 | Board + 7 columns expose `data-testid` per FR-012 list | `KanbanBoard`, `KanbanColumn` | AC-CON-011: each testid exists exactly once | e2e |

Every constraint has a design decision, a component, and a verification checkpoint with a test.

## Cross-Component Consistency Matrix

This feature is single-repo and single-layer (UI only), but multiple components share values. Tracing them:

| Shared Value | Producer | Consumer | Consistent? | Verification |
|--------------|----------|----------|-------------|--------------|
| Phase wire values (`inception`..`delivery`) | Go `internal/feature/types.go` `Phase` enum → `GET /api/features` `current_phase` | `types/index.ts` `PHASES`; `groupFeaturesByColumn` matches against them | YES — `types/index.ts` `PHASES` already mirrors the Go enum (verified by reading both files); grouping fn imports `PHASES`, not a re-declaration | Static: read both files; e2e: AC-001 seeds features in each phase and asserts column placement |
| Status wire values (`draft`,`in_progress`,...) | Go `Status` enum → API `status` field | `groupFeaturesByColumn` Backlog rule checks `=== 'draft'`; `FeatureCard` `STATUS_LABELS` map | YES — string literal `'draft'` matches the Go `StatusDraft = "draft"` wire value; `STATUS_LABELS` already covers all 9 statuses | e2e: AC-004/AC-005 exercise `draft` vs `in_progress`; AC-006 exercises `done` |
| Column key set | `COLUMN_KEYS = ['backlog', ...PHASES]` | `KanbanColumn` `data-testid="kanban-column-{key}"`; e2e selectors | YES — single source of truth (the constant), columns render from it, testids derive from it | e2e: AC-CON-011 asserts all 7 testids exist exactly once |
| `FeatureSummary` shape | `GET /api/features` → `types/index.ts` `FeatureSummary` | `FeatureCard` props, `groupFeaturesByColumn` field reads (`f.status`, `f.current_phase`) | YES — unchanged; board reads only fields the existing types define | Static: board source reads no fields outside `FeatureSummary`; e2e: AC-010 clicks a card and detail page renders |
| Empty-array contract | `internal/api/dto.go` serializes `features: []` not `null` | `KanbanBoard` `data?.features ?? []`; `groupFeaturesByColumn` initializes all cols to `[]` | YES — double defense: DTO guarantees `[]`, and the `?? []` + pre-init cols mean even a null would not crash | e2e: AC-011 + AC-013 exercise empty + partial-empty |
| react-query cache key | `Dashboard` `useQuery(['features'])` | `KanbanBoard` `useQuery(['features'])` — same key | YES — identical key → shared cache, single fetch, shared invalidation (FR-014) | e2e: AC-014 invalidation moves a card without reload |

No inconsistencies found. The only producer of every shared value is either the Go backend (unchanged) or a single UI constant; all consumers read from that single source.

## Test Strategy

### Component: `groupFeaturesByColumn`
- **Smoke**: N/A (pure fn).
- **Integration**: N/A.
- **E2E**: N/A.
- **Unit (reclassified to e2e per AD-6)**: behavior covered by AC-011 (empty input), AC-013 (partial fill), AC-001 (all phases), AC-004/005 (backlog rule), AC-006 (terminal status).

> If AD-6 is overridden to add vitest: direct unit tests — `[]` → 7 empty cols; one feature per phase → correct bucket; draft+inception → backlog; in_progress+inception → inception column; done+delivery → delivery; unknown phase → dropped, no throw.

### Component: `KanbanBoard`
- **Smoke**: page renders without console error (covered by existing `app.spec.ts` pattern + new kanban spec).
- **Integration**: AC-ERR-001 (500 → banner), AC-ERR-002 (refetch error → no crash), AC-CON-003 (no new endpoint via diff), AC-CON-006 (no new dep via diff).
- **E2E**: AC-001, AC-002, AC-003, AC-009, AC-011, AC-013, AC-014, AC-CON-008, AC-CON-011.
- **Unit**: N/A.

### Component: `KanbanColumn`
- **Smoke**: renders in board.
- **Integration**: AC-CON-005 (source grep for `FeatureCard` import).
- **E2E**: AC-003 (header count), AC-011 (empty-state message), AC-CON-008 (dark mode), AC-CON-011 (testid presence).
- **Unit**: N/A.

### Component: `ViewToggle`
- **Smoke**: renders in Dashboard header.
- **Integration**: N/A.
- **E2E**: AC-007, AC-008 (both toggle directions).
- **Unit**: N/A.

### Component: `Dashboard` (modified)
- **Smoke**: existing `app.spec.ts` still passes (regression guard).
- **Integration**: N/A.
- **E2E**: AC-007..009 (toggle + count badge persistence), AC-ERR-003 (deleted-card click → existing FeatureDetail 404).
- **Unit**: N/A.

### Negative-case / empty-state design
| Vector | Expected rejection/behavior | Test |
|--------|-----------------------------|------|
| `features: []` (CON-004) | 7 columns each render empty-state msg, count 0, no throw | AC-011, AC-012 |
| `GET /api/features` 500 (Error Scenarios) | Board-level banner "Failed to load features: {msg}", columns render empty, no `pageerror` | AC-ERR-001 |
| Refetch error mid-session | Stale cards remain visible (react-query default), no crash | AC-ERR-002 |
| Click deleted card | Navigate to `/features/{id}`; existing FeatureDetail 404 state | AC-ERR-003 |
| Unknown `current_phase` value | Grouping fn drops the feature (defensive); no column for it | Static reasoning + e2e AC-001 (only valid phases seeded) |
| `data.total_count` missing (Dashboard defensive) | Badge shows 0 — existing behavior, unchanged | Existing `app.spec.ts` regression |

## Agent Failure Mode Checks (apply to the Developer)

| Check | Applies to | What to verify |
|-------|-----------|----------------|
| Null vs empty array | `KanbanBoard`, `groupFeaturesByColumn` | `data?.features ?? []`; all 7 column keys pre-initialized to `[]`; never `Object.keys(grouped).map` on a possibly-missing key. No `omitempty`-style gaps. |
| Nil/undefined deref | `KanbanBoard` | `data` may be `undefined` while `isLoading` — guard with `?? []` before grouping. Do NOT call `.map` on `data.features` directly. |
| Parsing-safety | N/A — no parsing of external input; API JSON is already typed by `client.ts`. |
| Multi-component consistency | `KanbanColumn` renders `FeatureCard` for **every** feature in its bucket — no status filtering at render time (filtering is in `groupFeaturesByColumn` only). If a constraint applies to "all columns," verify in all 7, not just Backlog. |
| State machine | N/A — board is read-only, no transitions. |
| Middleware | N/A — no backend changes. |
| Language footguns (TS) | `f.current_phase as PhaseName` cast — guard with `PHASES.includes(...)` before indexing to avoid a runtime `undefined` key. `Record<ColumnKey, ...>` indexed with a non-key returns `undefined` at runtime if the cast lies. |
| Recovery middleware first | N/A — no HTTP handlers added. |
| Over-engineering | No drag-drop, no WIP limits, no per-column search, no animation, no new route, no new dep. If the implementation exceeds ~250 lines of new TSX, stop and re-read done conditions. |

## NFR Considerations

### Performance
- Feature count is small (tens). Client-side grouping is O(n). No pagination, no virtualization needed (spec assumption).
- Single react-query fetch shared with Dashboard (same key) → no extra network cost.

### Security
- No new input handling. Board reads only from authenticated `GET /api/features` (existing auth model unchanged).
- No user input rendered unescaped — `FeatureCard` already renders text via React (auto-escaped).
- No new endpoints to protect.

### Scalability
- N/A for this feature — UI-only, bounded by existing API capacity.

### Reliability
- Error banner on API 500 (AC-ERR-001). Stale-data-on-refetch-error (AC-ERR-002). No unbounded calls (react-query manages retries/timeout via existing client config).

## Quality Checkpoints (task boundaries)

1. After T001 (grouping fn + types): `cd ui && npx tsc --noEmit` passes; function file exists with the exact signature in AD-2.
2. After T002 (KanbanColumn + KanbanBoard): `npm run build` passes; `KanbanBoard` renders 7 `KanbanColumn` in order with correct testids.
3. After T003 (ViewToggle + Dashboard wiring): `npm run build` passes; existing `app.spec.ts` still passes (regression); toggling switches body content without unmounting the count badge.
4. After T004 (e2e spec): `npm run test:e2e` — all kanban ACs green; console-error assertions pass.
5. After T005 (integration spec): `npm run test:e2e` — AC-CON-003/006, AC-ERR-001/002 green.
6. Final gate: `git diff main -- ui/package.json` shows no new deps; `git diff main -- internal/` is empty.

## Quickstart Guide for the Developer

```bash
# from repo root
cd ui
npm install          # no new deps should be added
npm run build        # tsc + vite build — must pass after each task
npm run test:e2e     # play against running devteam binary on :8765
                     # (set START_SERVER=1 to force a fresh server, or reuse existing)
git diff main -- ui/package.json   # MUST show no additions in dependencies/devDependencies
git diff main -- internal/         # MUST be empty
```

**Order**: T001 → T002 → T003 → (T004 ∥ T005) → final gate.
**Do NOT**: add vitest, add a `/kanban` route, add drag-drop, re-implement `FeatureCard`, add any backend route, filter out `done`/`cancelled` features.
**DO**: import `PHASES`/`PhaseName` from `types/index.ts`; reuse `listFeatures`; reuse `useQuery(['features'])`; pre-init all 7 column arrays; render `FeatureCard` as-is.

## Open Questions (for human review, autonomous-safe)

1. **AD-6 — unit test runner**: The acceptance criteria label AC-012/AC-CON-005 as "unit" but CON-006 forbids adding the devDependency (`vitest`) that true unit tests require. The architect resolved this conservatively (no vitest; reclassify those two ACs to e2e/integration). **If a human prefers to add vitest, say so before construction** — the pure grouping function is already structured to be unit-testable.
2. **Empty-state copy**: Spec leaves copy to the architect. Chosen: Backlog → "No features waiting to start"; other 6 columns → "No features in this phase". Override before construction if different copy is wanted.
3. **Default view on first load**: List (existing behavior preserved). Override if board should be default.