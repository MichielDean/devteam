# Implementation Plan: kanban-view

**Branch**: `kanban-view` | **Date**: 2026-06-22 | **Spec**: `specs/kanban-view/spec.md`

**Input**: Feature specification from `specs/kanban-view/spec.md`

## Summary

Add a Kanban board view to the Dev Team Dashboard. The board renders features as cards grouped into six phase columns (Inception → Delivery), plus a defensive "Other" column for unknown phases. A List/Board toggle switches between the existing `FeatureList` and the new `KanbanBoard`; "Board" is the default, persisted in `sessionStorage` for the session. The board is view-only (no drag-and-drop), consumes the existing `useQuery(['features'])` data (no new fetch, no backend change), and reuses the existing loading/error/empty Dashboard branches. All layout via Tailwind utilities — no new npm dependencies.

Technical approach: three new UI components (`KanbanBoard`, `KanbanCard`, `KanbanColumn`) + one shared badge-color module + one `useSessionView` hook + a `ViewToggle` component, wired into `Dashboard.tsx`. A pure `groupFeaturesByPhase` function is extracted for unit testing. New e2e file `kanban.spec.ts` covers AC-001–AC-022; one additive fixture added to `app.spec.ts` to click "List" before list-view assertions (CON-004 regression fix).

## Technical Context

**Language/Version**: TypeScript 5.8, React 19.1, Go 1.x (backend — unchanged)

**Primary Dependencies**: `react`, `react-dom`, `react-router` v7, `@tanstack/react-query` v5, `tailwindcss` v4. **No new dependencies added** (CON-003).

**Storage**: `sessionStorage` (browser) for view preference. No server-side storage. No DB change.

**Testing**: Playwright e2e (`ui/e2e/*.spec.ts`, `:18765`) + Vitest-free unit tests via a runnable self-check for `groupFeaturesByPhase`. The repo has no JS unit-test runner installed; per ponytail/CON-003, the unit test for `groupFeaturesByPhase` (AC-011) is a co-located `KanbanBoard.test.ts` using a minimal hand-rolled assert harness OR a `vitest` devDependency — **decision: add `vitest` as a devDependency**. Rationale: the repo already has `@playwright/test`, `typescript`, `vite` as devDeps; `vitest` is Vite-native, zero-config, and the spec mandates a unit test (AC-011, test level `unit`). One devDep, minimal surface. If the developer finds an existing vitest setup, use it instead.

**Target Platform**: Web browser (Chrome/Firefox/Safari). Playwright runs on `:18765`.

**Project Type**: Web app (Go backend + React frontend, single repo).

**Performance Goals**: First contentful paint of the Board within 200ms of the features query resolving (SC-006). Pure CSS + React render — no data fetching. Trivially met.

**Constraints**:
- No new runtime npm dependency (CON-003). `vitest` is devOnly.
- No backend change, no new endpoint, no new fetch (CON-007, FR-016).
- Reuse `PHASES`/`PHASE_LABELS`/`STATUS_LABELS`/`PRIORITY_LABELS` — no duplicated strings (CON-005).
- Card chrome parity with `FeatureCard` (CON-006).
- Existing `app.spec.ts` list-view assertions must still pass (CON-004) — requires clicking "List" first since Board is now default.
- E2E on `:18765` only (CON-001).

**Scale/Scope**: Single repo, `ui/` directory only. ~6 new/modified files. Workspaces with 0–50+ features per phase (overflow handled, FR-013).

## Constitution Check

GATE: Passed. The spec's constitution compliance table is accepted. Key principles re-verified:

| Principle | Status | Note |
|---|---|---|
| I. Spec-Driven | ✅ | Plan derives from spec.md + acceptance.md. |
| VII. Self-Bootstrap | ✅ | Feature improves the platform's own UI. |
| VIII. Go, Minimal Dependencies | ✅ | No backend change. `vitest` is the only new devDep (justified by AC-011 unit-test requirement). No new runtime dep. |
| IX. Pipeline Governance | ✅ | Security/resiliency extensions N/A (view-only, no input, no auth, no external call). Documented in spec. |

No violations. No complexity-tracking entries needed.

## Project Structure

### Documentation (this feature)

```text
specs/kanban-view/
├── plan.md              # this file
├── research.md          # existing-pattern analysis + alternatives
├── data-model.md        # ephemeral UI entities (PhaseColumn, ViewPreference)
├── contracts/
│   └── GET-api-features.md   # read-only contract for the consumed endpoint
└── tasks.md             # task breakdown
```

### Source Code (repository root — `ui/` only)

```text
ui/
├── src/
│   ├── pages/
│   │   └── Dashboard.tsx           # MODIFY — wire toggle + conditional Board/List
│   ├── components/
│   │   ├── KanbanBoard.tsx         # CREATE — board container + groupFeaturesByPhase export
│   │   ├── KanbanColumn.tsx        # CREATE — single column (header + scrollable body + empty placeholder)
│   │   ├── KanbanCard.tsx          # CREATE — vertical card; reuses badgeColors + QuestionBadge
│   │   ├── KanbanBoard.test.ts     # CREATE — unit test for groupFeaturesByPhase (AC-011)
│   │   ├── ViewToggle.tsx          # CREATE — two-button toggle with aria-pressed
│   │   ├── badgeColors.ts          # CREATE — extracted shared statusColors map (CON-006)
│   │   ├── FeatureCard.tsx         # MODIFY — import statusColors from badgeColors.ts
│   │   └── QuestionBadge.tsx       # unchanged (reused by KanbanCard)
│   ├── hooks/
│   │   └── useSessionView.ts       # CREATE — sessionStorage-backed view preference
│   └── types/
│       └── index.ts                # unchanged (reuses PHASES, PHASE_LABELS, etc.)
├── e2e/
│   ├── app.spec.ts                 # MODIFY — click "List" before list-view assertions (CON-004)
│   └── kanban.spec.ts              # CREATE — AC-001..AC-022
└── package.json                    # MODIFY — add vitest devDep + test:unit script
```

**Structure Decision**: Single-project web app (existing layout). New components under `ui/src/components/` (CON-002). New hook under `ui/src/hooks/` (matches the existing `useFeatures.ts` location). No new pages — the board lives on the existing Dashboard route.

## Component Design

### `ViewToggle`

- **Purpose**: Two-button segmented control switching between "List" and "Board".
- **Responsibilities**:
  - Render two `<button>` elements with `data-testid="view-toggle-list"` / `"view-toggle-board"`.
  - Container `data-testid="view-toggle"`.
  - Active button carries `aria-pressed="true"`; inactive `aria-pressed="false"` (AC-001/004/005).
  - Call `onViewChange(view)` on click.
- **Interfaces**: props `{ view: 'board' | 'list'; onViewChange: (v) => void }`.
- **Dependencies**: none (pure presentational).

### `useSessionView`

- **Purpose**: Session-scoped persistence of the view preference.
- **Responsibilities**:
  - Lazy-init from `sessionStorage.getItem('devteam.dashboard.view')` (FR-002). Validate against `'board' | 'list'`; invalid/absent → `'board'` (FR-003).
  - On change, `sessionStorage.setItem('devteam.dashboard.view', next)`.
  - SSR-safe guard (typeof window check) — not strictly needed (Vite SPA) but cheap.
- **Interfaces**: `useSessionView(): ['board' | 'list', (v) => void]`.
- **Dependencies**: `sessionStorage` (browser native).
- **Agent failure-mode check**: lazy initializer must not throw if `sessionStorage` access raises (private-mode quota) — wrap in try/catch, fall back to `'board'`.

### `KanbanBoard`

- **Purpose**: Render six phase columns + optional "Other" column, each populated with `KanbanCard`s.
- **Responsibilities**:
  - Accept `features: FeatureSummary[]` prop.
  - Compute `groupFeaturesByPhase(features)` → `Record<PhaseName | 'other', FeatureSummary[]>`.
  - Render columns in `PHASES` order; append `'other'` column only when `groups.other.length > 0` (FR-007, AC-019).
  - Board container: `flex gap-4 overflow-x-auto` (FR-015); height bounded via `h-[calc(100vh-8rem)]` (FR-014).
  - No network calls — pure render from props (CON-007).
- **Interfaces**: props `{ features: FeatureSummary[] }`. Exports `groupFeaturesByPhase` for unit testing.
- **Dependencies**: `KanbanColumn`, `PHASES`, `PHASE_LABELS` from `types`.
- **`groupFeaturesByPhase` spec** (pure function, exported):
  - Input: `FeatureSummary[]`.
  - Output: `{ [phase in PhaseName]: FeatureSummary[] } & { other: FeatureSummary[] }`.
  - Invariant: partition — every input feature appears in exactly one bucket. `sum === input.length`.
  - Unknown `current_phase` → `other` bucket (FR-007, CON-009, AC-011).
  - Each bucket initialized to `[]` (no null arrays — CON-008 agent failure-mode).
- **Agent failure-mode checks**:
  - [ ] No `null` arrays — every bucket starts as `[]`.
  - [ ] Partition invariant holds — unit test asserts sum.
  - [ ] Unknown phase does not crash — unit test with synthetic `'weird'` phase.

### `KanbanColumn`

- **Purpose**: One column — header + scrollable body + empty placeholder.
- **Responsibilities**:
  - Container `data-testid="kanban-column-${phase}"` (e.g. `kanban-column-planning`, `kanban-column-other`).
  - Header: `PHASE_LABELS[phase]` (or `'Other'`), `data-testid="kanban-column-header-${phase}"`.
  - Body: `flex-1 overflow-y-auto` (FR-013), renders `KanbanCard` per feature.
  - Empty: when `features.length === 0`, render `data-testid="kanban-column-empty-${phase}"` with muted "No features" text (FR-012, AC-017).
  - Column width: `w-60` (240px, FR-015).
- **Interfaces**: props `{ phase: PhaseName | 'other'; label: string; features: FeatureSummary[] }`.
- **Dependencies**: `KanbanCard`.
- **Agent failure-mode checks**:
  - [ ] Empty body renders placeholder, not `null`/blank.
  - [ ] Column header stays fixed when body scrolls (header outside the `overflow-y-auto` element).

### `KanbanCard`

- **Purpose**: Vertical card for a single feature on the board.
- **Responsibilities**:
  - Root: `<Link to={/features/:id}>` with `data-testid="kanban-card-${feature.id}"` (FR-010, AC-010).
  - Title (line-clamped to 2 lines).
  - Badge trio: status (`kanban-card-status`), priority (`kanban-card-priority`), using `STATUS_LABELS` / `PRIORITY_LABELS` and the shared `statusColors` map (CON-005/CON-006).
  - `QuestionBadge` when `pending_questions_count > 0` (FR-008, AC-008).
  - Gate indicator `kanban-card-gate` when `gate_result` present: `✓ Gate passed` / `✗ Gate failed` (FR-009, AC-009) — **identical text to `FeatureCard`** (CON-006).
  - Status-flag ring (FR-011):
    - `status === 'gate_blocked'` → `ring-2 ring-red-400` (AC-012).
    - `status === 'waiting_for_human'` → `ring-2 ring-yellow-400` (AC-013).
    - Otherwise no ring.
  - Updated date line (matches `FeatureCard`).
- **Interfaces**: props `{ feature: FeatureSummary }`.
- **Dependencies**: `Link` from `react-router`, `QuestionBadge`, `statusColors` from `badgeColors.ts`, `STATUS_LABELS`/`PRIORITY_LABELS` from `types`.
- **Agent failure-mode checks**:
  - [ ] Ring class only applied for the two attention statuses — no accidental ring on normal cards.
  - [ ] Gate indicator text exactly matches `FeatureCard` (`✓ Gate passed` / `✗ Gate failed`).
  - [ ] Card is a single `<Link>` — no nested interactive elements (QuestionBadge is a `<Link>` today; it must NOT be nested inside the card `<Link>`. **Decision**: on the board card, render the question count as a non-link `<span>` badge styled identically, to avoid nested-anchor invalid HTML. `QuestionBadge` stays as-is for `FeatureCard`; `KanbanCard` uses a local `<span data-testid="question-badge">`. Same testid, same visual, valid HTML. Documented in tasks.)

### `badgeColors` (shared module)

- **Purpose**: Single source of truth for the status → Tailwind class map (CON-006).
- **Responsibilities**: export `statusColors: Record<string, string>` — the map currently inlined in `FeatureCard.tsx`.
- **Consumers**: `FeatureCard` (modify to import), `KanbanCard` (new).
- **Agent failure-mode check**: verify both consumers import from this module — no re-duplicated map.

### `Dashboard` (modify)

- **Changes**:
  - Import `useSessionView`, `ViewToggle`, `KanbanBoard`.
  - `const [view, setView] = useSessionView();`
  - Render `ViewToggle` **only** when `!isLoading && !error && features.length > 0` (FR-004, AC-006).
  - In the `features.length > 0` branch, conditionally render `<KanbanBoard features={features} />` (view === 'board') or `<FeatureList features={features} />` (view === 'list').
  - Loading / error / empty branches unchanged (FR-017, CON-008).
- **Agent failure-mode checks**:
  - [ ] Toggle hidden in empty state — verify e2e AC-006.
  - [ ] Single `useQuery(['features'])` call remains — no second fetch (CON-007, AC-016).

## API Contracts

See `contracts/GET-api-features.md`. **No new endpoints.** The Board consumes the existing `GET /api/features` response via props from Dashboard. Contract documented read-only.

## Constraint Verification Map

| CON-ID | Design Decision | Component(s) | Verification Checkpoint | Test Type |
|---|---|---|---|---|
| CON-001 | New e2e in `ui/e2e/kanban.spec.ts`; `app.spec.ts` modified fixture. Playwright config unchanged (`:18765`). | kanban.spec.ts, app.spec.ts, playwright.config.ts | `npm run test:e2e` runs against `:18765` webServer; no test references `:8765` | E2E |
| CON-002 | New components in `ui/src/components/`; hook in `ui/src/hooks/`. No new pages. | KanbanBoard/Column/Card/ViewToggle/badgeColors, useSessionView | File-path review: all new files under `ui/src/components/` or `ui/src/hooks/` | Review |
| CON-003 | No new runtime npm dep. `vitest` added as devDep only (for AC-011 unit test). All layout via Tailwind. | package.json, KanbanBoard/Column/Card | `package.json` diff: dependencies block unchanged; devDependencies adds `vitest` only | Review |
| CON-004 | `app.spec.ts` list-view tests updated to click `view-toggle-list` before asserting `feature-card-*` (Board is now default). Additive fixture, no assertion removed. | app.spec.ts | `npm run test:e2e` green; existing feature-card / count-badge assertions pass after the click-to-List step | E2E (regression) |
| CON-005 | Board imports `PHASES`, `PHASE_LABELS`, `STATUS_LABELS`, `PRIORITY_LABELS` from `types/index.ts`. Column headers via `PHASE_LABELS`; card badges via `STATUS_LABELS`/`PRIORITY_LABELS`. No new string literals. | KanbanBoard, KanbanColumn, KanbanCard | Grep `kanban-*.tsx` for `'Inception'\|'Planning'\|...` / `'In Progress'\|...` → zero matches outside `types/` | Review + grep |
| CON-006 | `statusColors` extracted to `badgeColors.ts`; `FeatureCard` and `KanbanCard` both import it. Gate indicator text identical (`✓ Gate passed` / `✗ Gate failed`). QuestionBadge testid reused. | badgeColors.ts, FeatureCard, KanbanCard | Code review: single `statusColors` map; gate text byte-identical; e2e AC-007/008/009 pass | Review + E2E |
| CON-007 | Board receives `features` as prop from Dashboard; Dashboard owns the single `useQuery(['features'])`. Board makes zero fetch calls. | Dashboard, KanbanBoard | E2e AC-016: `page.on('request')` count for `/api/features` === 1 during Board render | Integration |
| CON-008 | Loading (`features-loading`), error (`features-error`), empty (`EmptyState`) branches reused unchanged from Dashboard. Board renders only in the `features.length > 0` branch. Empty columns render `[]` + "No features" placeholder. | Dashboard, KanbanColumn | E2e AC-006/014/015/017/018; `PhaseColumn.features` always `[]` never `null` (code review) | E2E + Review |
| CON-009 | `groupFeaturesByPhase` routes any `current_phase` not in `PHASES` to the `other` bucket. No throw, no drop. | KanbanBoard (groupFeaturesByPhase) | Unit test AC-011: `groupFeaturesByPhase([{current_phase:'weird',...}])` → `{other:[feature]}`; partition sum invariant | Unit |

## Cross-Component Consistency Matrix

| Shared Value | Producer | Consumer | Consistent? | Verification |
|---|---|---|---|---|
| Phase labels | `PHASE_LABELS` (types) | `KanbanColumn` header, `KanbanBoard` column ordering | YES — single source | E2E AC-019 (6 columns in `PHASES` order); grep no duplicate literals (CON-005) |
| Status labels | `STATUS_LABELS` (types) | `KanbanCard` status badge | YES — single source | E2E AC-007 (badge text "In Progress") |
| Priority labels | `PRIORITY_LABELS` (types) | `KanbanCard` priority badge | YES — single source | E2E AC-007 (badge text "P1 - Critical") |
| Status → Tailwind class map | `badgeColors.ts` (new shared module) | `FeatureCard`, `KanbanCard` | YES — both import the same export | Code review; visual parity e2e AC-007 (CON-006) |
| Gate indicator text | `KanbanCard` (hardcoded `✓ Gate passed` / `✗ Gate failed`) | (matches `FeatureCard` text) | YES — byte-identical strings | E2E AC-009; grep both files for the strings |
| `question-badge` testid | `QuestionBadge` (list), local `<span>` (board) | E2E selectors | YES — same testid, different element (span not Link) | E2E AC-008; HTML validity check (no nested anchors) |
| Features array | Dashboard `useQuery(['features'])` | `FeatureList` (list), `KanbanBoard` (board) | YES — same prop source, no second fetch | Integration AC-016 (CON-007) |
| View preference | `useSessionView` (sessionStorage) | `Dashboard` render branch | YES — single state owner | E2E AC-004/005 (reload + fresh session) |
| Column count | `KanbanBoard` (renders `PHASES.length` + optional `other`) | E2E AC-019 assertion (6, +1 only when unknown phase) | YES — driven by `PHASES` constant | E2E AC-019 |

**Multi-component note**: the only "N producers" case is the status-color map (2 consumers: `FeatureCard` + `KanbanCard`). Extracting to `badgeColors.ts` guarantees consistency. No provider/consumer divergence possible.

## Test Strategy

### Component: `ViewToggle`
- **Smoke**: renders two buttons, active one has `aria-pressed="true"`.
- **E2E**: AC-001 (toggle visible, Board active by default), AC-002 (click Board → columns), AC-003 (click List → feature-list), AC-004 (reload persists), AC-005 (fresh session → Board).
- **Unit**: not required (pure presentational, e2e covers it).

### Component: `useSessionView`
- **E2E**: AC-004 (sessionStorage persistence across reload), AC-005 (fresh session defaults Board), US-3 scenario 3 (empty → non-empty resumes stored view).
- **Unit**: optional; behavior is trivial and e2e-covered.

### Component: `KanbanBoard` (+ `groupFeaturesByPhase`)
- **Smoke**: renders without crash given `[]` (six empty columns) and given a populated array.
- **Unit** (AC-011, mandatory): `KanbanBoard.test.ts` —
  - `groupFeaturesByPhase([])` → six empty buckets + empty `other`.
  - `groupFeaturesByPhase([{current_phase:'planning'},...])` → correct bucket.
  - `groupFeaturesByPhase([{current_phase:'weird'}])` → `other` bucket, no crash (CON-009).
  - Partition invariant: `sum(buckets) === input.length` for a mixed input.
- **E2E**: AC-002 (columns render), AC-007 (card in correct column with badges), AC-016 (single fetch), AC-019 (6 columns + optional other).

### Component: `KanbanColumn`
- **E2E**: AC-017 (empty column placeholder), AC-019 (column count), AC-020/021 (overflow scroll), AC-022 (min-width 240).
- **Unit**: not required (layout-only).

### Component: `KanbanCard`
- **E2E**: AC-007 (title + badges), AC-008 (question badge), AC-009 (gate indicator), AC-010 (click → navigate), AC-012 (gate_blocked ring), AC-013 (waiting_for_human ring).
- **Unit**: not required (presentational).

### Component: `Dashboard` (modified)
- **Smoke**: page loads, no console errors (existing `app.spec.ts` console-error assertion extended to Board view).
- **E2E**: AC-001/006 (toggle visibility rules), AC-014 (loading state), AC-015 (error state), AC-018 (empty state).
- **Integration**: AC-016 (single fetch via `page.on('request')`).

### Test Level Selection Matrix (applied)

| What changed | Smoke | Integration | E2E | Unit |
|---|---|---|---|---|
| `KanbanBoard` + grouping | YES | — | YES | **YES** (AC-011) |
| `KanbanCard` (UI) | YES | — | YES | — |
| `KanbanColumn` (UI) | YES | — | YES | — |
| `ViewToggle` (UI) | YES | — | YES | — |
| `useSessionView` (hook) | YES | — | YES | — |
| `Dashboard` (wiring) | YES | YES (AC-016) | YES | — |
| `app.spec.ts` (regression) | YES | — | YES | — |

### Quality Checkpoints (per component)

- [ ] Board renders without console errors (smoke — SC-004)
- [ ] All e2e selectors use `data-testid`, never class names for state (CON convention)
- [ ] `PhaseColumn.features` is `[]` not `null` for empty columns (CON-008)
- [ ] No nested `<a>` inside `KanbanCard` (question badge is `<span>`)
- [ ] Gate indicator text byte-identical to `FeatureCard` (CON-006)
- [ ] `statusColors` imported from `badgeColors.ts` in both card components (CON-006)
- [ ] No new string literals for phase/status names in board files (CON-005)
- [ ] `package.json` dependencies block unchanged (CON-003)
- [ ] Single `GET /api/features` request during Board render (CON-007, AC-016)
- [ ] `app.spec.ts` list-view tests click "List" first (CON-004)

## Agent Failure Mode Checks (per task)

| Task | Failure mode | Check |
|---|---|---|
| T-001 (badgeColors extract) | Re-duplicated map | Grep: only one `statusColors` definition; both cards import it |
| T-002 (useSessionView) | sessionStorage throws in private mode | try/catch → default `'board'` |
| T-003 (groupFeaturesByPhase) | Null arrays; dropped features; crash on unknown phase | Unit test asserts `[]` init, partition sum, unknown-phase bucket |
| T-004 (KanbanCard) | Nested anchors; wrong ring class; gate text drift | HTML validator; ring class only for 2 statuses; grep gate text |
| T-005 (KanbanColumn) | Empty body blank (not placeholder); header scrolls with body | Placeholder testid; header outside `overflow-y-auto` |
| T-006 (KanbanBoard) | Second fetch; wrong column order; `other` column always present | No `useQuery` in board; columns in `PHASES` order; `other` conditional |
| T-007 (Dashboard wiring) | Toggle visible in empty state; loading/error branches broken | Toggle gated by `features.length > 0`; existing branches untouched |
| T-008 (ViewToggle) | `aria-pressed` wrong/missing; both buttons active | Assert exactly one `aria-pressed="true"` |
| T-009 (app.spec.ts fixture) | Existing assertions broken; skip-too-aggressive | All existing tests still run; only added a click step |
| T-010 (kanban.spec.ts) | Tests run on `:8765`; selectors use classes | Config `:18765`; all selectors `data-testid` |

## Negative Case Design

The constraint register has no RFC conformance vectors. The "negative" cases are defensive edge cases, each mapped to an AC:

| Edge case (CON) | AC | Design | Rejection behavior |
|---|---|---|---|
| Unknown `current_phase` (CON-009) | AC-011 | `groupFeaturesByPhase` checks `PHASES.includes(phase)`; else → `other` bucket | Feature placed in "Other" column, no crash, no drop. Unit test verifies. |
| Empty board (CON-008) | AC-006/018 | Dashboard renders `EmptyState` when `features.length === 0`; toggle hidden | Board never renders; no empty-column rendering needed. |
| Empty column (CON-008) | AC-017 | `KanbanColumn` renders `kanban-column-empty-${phase}` placeholder when `features.length === 0` | Muted "No features" text; column header still visible. |
| Loading state (CON-008) | AC-014 | Dashboard existing `features-loading` branch; Board not rendered | Spinner visible, zero `kanban-column-*`. |
| Error state (CON-008) | AC-015 | Dashboard existing `features-error` branch; Board not rendered | Error text visible, zero `kanban-column-*`. |
| Missing `total_count` (CON-008) | (existing e2e) | Dashboard `data?.total_count ?? 0` — unchanged | Badge shows `0`; no crash. |
| Invalid stored view | AC-005 (implicit) | `useSessionView` validates value; invalid → `'board'` | Defaults to Board on next load. |

## Quality Checkpoints at Task Boundaries

1. **After T-001 (badgeColors)**: `FeatureCard` still renders identically — run existing `app.spec.ts` list-view tests (after clicking List). No visual drift.
2. **After T-003 (groupFeaturesByPhase)**: unit test passes (AC-011) before any UI wiring.
3. **After T-006 (KanbanBoard)**: renders standalone in a smoke test (dev server) with mock features — no console errors.
4. **After T-007 (Dashboard wiring)**: e2e AC-001/002/003/006 pass — toggle works, empty state hides toggle.
5. **After T-009 (app.spec.ts)**: full existing suite green — no regression (CON-004).
6. **After T-010 (kanban.spec.ts)**: all AC-001..AC-022 covered (every acceptance criterion has a test).

## Quickstart Guide for the Developer

```bash
# From repo root
cd ui

# 1. Add vitest devDep
npm install -D vitest

# 2. Add test:unit script to package.json
#    "test:unit": "vitest run"

# 3. Implement in dependency order (see tasks.md):
#    badgeColors → useSessionView → groupFeaturesByPhase (+ unit test)
#    → KanbanCard → KanbanColumn → KanbanBoard → ViewToggle
#    → Dashboard wiring → app.spec.ts fixture → kanban.spec.ts

# 4. Run unit test
npm run test:unit          # AC-011

# 5. Run e2e (needs the Go binary serving :18765)
START_SERVER=1 npm run test:e2e    # all ACs

# 6. Dev smoke
npm run dev                # http://localhost:5173 — click around, check console
```

**Verify before declaring done**:
- `npm run test:unit` green (AC-011).
- `npm run test:e2e` green (all kanban.spec.ts + app.spec.ts).
- `package.json` `dependencies` block unchanged (CON-003).
- Grep `ui/src/components/Kanban*.tsx` for phase/status name literals → zero (CON-005).
- `ui/src/components/badgeColors.ts` is the only `statusColors` definition (CON-006).
- Browser devtools Network tab: one `GET /api/features` when Board renders (CON-007).