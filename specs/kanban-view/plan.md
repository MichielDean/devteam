# Implementation Plan: kanban-view

**Branch**: `kanban-view` | **Date**: 2026-06-22 | **Spec**: `specs/kanban-view/spec.md`

**Input**: Feature specification from `specs/kanban-view/spec.md`

## Summary

Add a Kanban board view to the Dev Team Dashboard that groups existing features into six phase columns (Inception, Planning, Construction, Review, Testing, Delivery), reusing the existing `FeatureCard` component for card content and navigation. A List/Kanban segmented toggle persists the user's choice in `localStorage`. **No backend, no API, no Go, no new runtime dependencies.** Entire feature is inside `ui/`. The board is display + navigation only (no drag-and-drop).

## Technical Context

**Language/Version**: TypeScript 5.8 + React 19.1

**Primary Dependencies** (all already installed, no additions):
- react, react-dom, react-router (v7) — routing, `Link`
- @tanstack/react-query — `useQuery(['features'])` (existing Dashboard query, reused)
- tailwindcss (v4) via `@tailwindcss/vite` — styling, dark mode `dark:` variants

**Dev Dependencies** (one addition):
- vitest — for the `orderCards` unit test (AC-018). DevDependency only; no runtime impact. See research.md "Alternatives" for rationale. CON-008 restricts *runtime* deps.

**Storage**: `localStorage` key `devteam-dashboard-view` (values `"list"` | `"kanban"`, default `"list"`). No server storage.

**Testing**:
- `cd ui && npm run build` + `cd ui && npm run lint` (smoke, CON-001)
- `cd ui && npx playwright test --reporter=line` (e2e on port 18765, CON-002)
- `cd ui && npx vitest run` (unit, for AC-018)

**Target Platform**: Modern browsers (Chrome/Firefox/Safari/Edge). SPA via Vite, no SSR.

**Project Type**: Web app frontend (`ui/`).

**Performance Goals**: Toggle switches views in one render frame (no refetch, no spinner beyond existing `listFeatures` load). SC-001. Board groups/sorts O(n log n) at most; feature count is tens-to-hundreds.

**Constraints**:
- No new runtime npm deps (CON-008).
- E2E on port 18765 via existing `ui/playwright.config.ts` (CON-002).
- Build/lint with repo's existing commands (CON-001).
- Card markup reused via `FeatureCard` import (CON-003).
- Column set/order from `PHASES` constant (CON-004); labels from `PHASE_LABELS` (CON-005).
- `data-testid` on all new interactive/observable elements (CON-006).

**Scale/Scope**: Single repo (`.`), single directory subtree (`ui/src/`). 6 new files + 1 modified file + 1 test file. ~150-250 lines of new code.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|---|---|---|
| I. Spec-Driven, Always | ✅ Pass | spec.md, acceptance.md, repos.yaml exist. Plan + tasks follow. |
| II. Six Roles, Fixed Pipeline | ✅ Pass | PM complete; Architect now; Developer/Reviewer/Tester/Ops follow. |
| III. Central Spec, Distributed Implementation | ✅ Pass | Single spec in `specs/kanban-view/`. Single repo scope. |
| IV. Two Intake Paths, One Output Format | ✅ Pass | Loose idea → same artifact shape. |
| V. Proof-of-Work Gates | ✅ Pass | ACs are Given/When/Then with test levels. Tester will name specific Playwright files (CON-007). |
| VI. Cross-Repo Coherence | ✅ N/A | Single-repo feature. |
| VII. Self-Bootstrap | ✅ N/A | Not the platform-building spec. |
| VIII. Go, Minimal Dependencies | ✅ Pass | No Go changes. No new runtime deps. One devDependency (vitest) for unit testing. |
| IX. Pipeline Governance | ✅ Pass | Planning rules followed; constraint map, consistency matrix, test strategy below. |
| X. Learn From Cistern | ✅ Pass | Role identity clear; phase gate mechanically enforced. |

No violations. No complexity tracking entries needed.

## Project Structure

### Documentation (this feature)

```text
specs/kanban-view/
├── plan.md              # This file
├── research.md          # Existing patterns, library choices, alternatives
├── data-model.md        # View-only derived structures (no persisted entities)
├── contracts/
│   ├── GET-api-features.md   # Documents: no new API; existing endpoint consumed
│   └── components.md         # Internal React component prop contracts
└── tasks.md             # Task decomposition (next file)
```

### Source Code (repository root — `ui/` subtree only)

```text
ui/
├── src/
│   ├── components/
│   │   ├── KanbanBoard.tsx     # NEW — board container + orderCards export
│   │   ├── KanbanColumn.tsx    # NEW — single phase column
│   │   ├── ViewToggle.tsx      # NEW — List/Kanban segmented control
│   │   ├── FeatureCard.tsx     # UNCHANGED — reused inside columns
│   │   ├── FeatureList.tsx     # UNCHANGED — existing list view
│   │   └── EmptyState.tsx      # UNCHANGED — zero-features state
│   ├── pages/
│   │   └── Dashboard.tsx       # MODIFIED — add view state + toggle + conditional board
│   └── types/
│       └── index.ts            # MODIFIED — add ViewMode type + key constant (additive)
├── e2e/
│   └── kanban.spec.ts          # NEW — Playwright e2e for AC-001..AC-020 (except AC-018)
└── (vitest config: inline in package.json or minimal vitest.config.ts)
```

**Structure Decision**: New components colocated with existing components in `ui/src/components/` — matches the existing flat structure (no subfolders per feature). No new top-level directories. The smallest diff that delivers the feature.

## Constraint Verification Map

| CON-ID | Design Decision | Component(s) | Verification Checkpoint | Test Type |
|--------|-----------------|--------------|------------------------|-----------|
| CON-001 | Use existing `npm run build` / `npm run lint` in `ui/`. No new build tool. | `ui/` (all) | `cd ui && npm run build` and `cd ui && npm run lint` both exit 0 | smoke (AC-CON-001) |
| CON-002 | E2E run via existing `ui/playwright.config.ts` (port 18765). New spec `ui/e2e/kanban.spec.ts` uses `baseURL` from config; no hardcoded 8765. | `ui/e2e/kanban.spec.ts` | `cd ui && npx playwright test kanban.spec.ts` runs against config's webServer on 18765 | smoke (AC-CON-002) |
| CON-003 | `KanbanColumn` imports `FeatureCard` from `./FeatureCard` and renders `<FeatureCard feature={...} />`. No inline card JSX (title/status/priority/gate markup) in column. | `KanbanColumn.tsx` | Grep `KanbanColumn.tsx` for `import FeatureCard` and `<FeatureCard`; assert no `feature-card-title`/`feature-card-status` literal JSX in column body | unit (AC-CON-003) |
| CON-004 | `KanbanBoard` builds columns by mapping over the imported `PHASES` array; DOM order follows array order. `other` column appended only if needed. | `KanbanBoard.tsx` | Playwright reads `kanban-column-*` testids in DOM order, asserts sequence `inception, planning, construction, review, testing, delivery` | e2e (AC-CON-004) |
| CON-005 | `KanbanColumn` receives `label` prop from `KanbanBoard`, which sources it from `PHASE_LABELS[phase]`. No string literal "Inception"/"Planning"/etc. in component bodies. | `KanbanBoard.tsx`, `KanbanColumn.tsx` | Grep new component source for `PHASE_LABELS` import; assert no hardcoded phase-name string literals used as header text | unit (AC-CON-005) |
| CON-006 | Every new interactive/observable element gets a `data-testid` in `kebab-case`: `view-toggle`, `view-toggle-list`, `view-toggle-kanban`, `kanban-board`, `kanban-column-{phase}`, `kanban-column-header-{phase}`, `kanban-column-count-{phase}`, `kanban-column-empty-state`, `kanban-column-other`. | `ViewToggle.tsx`, `KanbanBoard.tsx`, `KanbanColumn.tsx` | Playwright suite uses only `[data-testid=...]` selectors for these elements; no class/text selectors | e2e (AC-CON-006) |
| CON-007 | Tester's report maps each AC-NNN to a named `describe`/`it` in `ui/e2e/kanban.spec.ts` (or `ui/src/__tests__/orderCards.test.ts` for AC-018) with quoted assertions. | `ui/e2e/kanban.spec.ts`, `ui/src/__tests__/orderCards.test.ts` | test-report.md contains file:line references per AC | process (AC-CON-007) |
| CON-008 | `ui/package.json` gains only `vitest` (devDependency). No new runtime deps. Tailwind/react-router/react-query cover all UI needs. | `ui/package.json` | `git diff main -- ui/package.json ui/package-lock.json` shows only vitest under devDependencies; runtime deps unchanged | smoke (AC-CON-008) |
| CON-009 | Empty known column (count 0) renders header + `(0)` + `kanban-column-empty-state` with non-empty text. Not hidden, not an error. | `KanbanColumn.tsx` | Playwright: seed state with an empty phase, assert column visible + count 0 + empty-state text present | e2e (AC-011, AC-CON-009) |
| CON-010 | Dashboard's existing `isLoading`/`error` branches render `features-loading`/`features-error` BEFORE the view-specific branch. Kanban is only rendered in the non-empty success branch. | `Dashboard.tsx` | Playwright: intercept `/api/features` with delay → `features-loading` visible, `kanban-board` absent; intercept with 500 → `features-error` visible, `kanban-board` absent | e2e (AC-009, AC-010, AC-CON-010) |

## Cross-Component Consistency Matrix

This feature has no multi-provider / producer-consumer standard-conformance surface (no RFC, no multiple signing providers). The "components" are React components sharing the `FeatureSummary` shape and the `PHASES`/`PHASE_LABELS` constants. Consistency checks:

| Shared Value | Producer | Consumer | Consistent? | Verification |
|---|---|---|---|---|
| `FeatureSummary` shape | `GET /api/features` (existing) → `ui/src/types/index.ts` | `Dashboard` (useQuery), `FeatureCard` (props), `KanbanBoard` (grouping), `KanbanColumn` (rendering), `orderCards` (sorting) | YES — all consumers import the same `FeatureSummary` type from `ui/src/types/index.ts`. No redefinition. | tsc compiles (smoke); e2e reads card content in list vs kanban and asserts identical (AC-008) |
| `current_phase` value set | Backend (existing) emits one of `PHASES` (assumption) | `KanbanBoard` grouping (`f.current_phase === phase`), `FeatureCard` label (`PHASE_LABELS[phase]`) | YES for known phases. Unknown phases defensively bucketed to `other` column (FR-017). | e2e: AC-006 (feature in correct column), AC-CON-004 (column order); defensive `other` column tested if a fixture with unknown phase can be seeded (note: backend only emits known phases per assumption, so `other` is verified by code inspection + a unit test of the grouping function if extractable) |
| `PHASES` array order | `ui/src/types/index.ts` (single definition) | `KanbanBoard` (column iteration order) | YES — `KanbanBoard` imports `PHASES` and maps in array order. No reordering. | e2e (AC-CON-004) |
| `PHASE_LABELS` text | `ui/src/types/index.ts` (single definition) | `KanbanBoard` (passes `label` to `KanbanColumn`), `FeatureCard` (phase badge) | YES — both import `PHASE_LABELS`. Column header text and card phase badge text come from the same map. | unit (AC-CON-005) + e2e (AC-007 header label) |
| Card markup | `FeatureCard.tsx` (single component) | `FeatureList` (list view), `KanbanColumn` (kanban view) | YES — both render `<FeatureCard feature={f} />`. No duplicated card JSX. | unit (AC-CON-003) + e2e (AC-008 content parity) |
| Card click → `/features/{id}` | `FeatureCard` root `<Link to={/features/${id}}>` | list view, kanban view | YES — same component, same Link. Navigation identical. | e2e (AC-012, AC-013, SC-004) |
| `priority` sort direction | `orderCards` (priority asc: 1 < 2 < 3) | `KanbanBoard` (passes ordered array to `KanbanColumn`) | YES — single sort helper, single direction. | unit (AC-018) + e2e (AC-016, AC-017) |
| `updated_at` tiebreaker | `orderCards` (desc: newer first) | `KanbanBoard` | YES — single helper. | unit (AC-018) + e2e (AC-017) |
| `viewMode` persistence key | `Dashboard.tsx` (`devteam-dashboard-view`) | `Dashboard.tsx` (read on mount, write on change) | YES — single key, single read site, single write site. | e2e (AC-003) |

**No cross-repo consistency surface** — single repo feature (Constitution VI N/A).

## Test Strategy

### Component: `ViewToggle`
- **Smoke**: renders two buttons with correct testids; active button has `aria-pressed="true"`.
- **Unit**: clicking inactive button calls `onChange` with that value; clicking active button is a no-op (or re-emits same — idempotent). *(Covered in e2e via Dashboard; no separate unit test mandated unless Developer finds it cheap.)*
- **E2E**: AC-001, AC-002, AC-005 — toggle switches views; testids present; aria-pressed reflects active.
- **Quality checkpoints**:
  - [ ] Both testids present in DOM when toggle is mounted
  - [ ] `aria-pressed` toggles correctly on click
  - [ ] Toggle hidden when `features.length === 0` (AC-020)

### Component: `KanbanBoard`
- **Smoke**: renders `kanban-board` container with six `kanban-column-{phase}` children in `PHASES` order.
- **Unit**: `orderCards` returns array sorted priority asc then `updated_at` desc; does not mutate input (AC-018).
- **E2E**: AC-006 (feature in correct column), AC-007 (header label + count), AC-CON-004 (column order).
- **Quality checkpoints**:
  - [ ] `Σ column.count === features.length` for every render (invariant)
  - [ ] Unknown-phase features go to `other` column (FR-017)
  - [ ] No inline card JSX — uses `<FeatureCard>` (CON-003)
  - [ ] `PHASES` imported, not hardcoded (CON-004)

### Component: `KanbanColumn`
- **Smoke**: renders header (label + count) and body; empty column renders `kanban-column-empty-state`.
- **E2E**: AC-011 (empty column visible with count 0 + message), AC-008 (card content parity with list view), AC-014/AC-015 (scroll behavior via board container).
- **Quality checkpoints**:
  - [ ] Header count matches actual card count in body (SC-003)
  - [ ] Empty state has non-empty text (not hidden div)
  - [ ] Header label from `PHASE_LABELS`, not hardcoded (CON-005)
  - [ ] Sticky header does not desync from cards during horizontal scroll (AC-015)

### Component: `Dashboard` (modified)
- **Smoke**: still renders existing `feature-list` by default; still renders `features-loading`/`features-error`/`empty-state` on those branches.
- **E2E**: AC-001..AC-005 (toggle + persistence), AC-009/AC-010 (loading/error passthrough), AC-019/AC-020 (empty state + toggle hidden).
- **Quality checkpoints**:
  - [ ] Default view = list when no localStorage key (AC-004)
  - [ ] localStorage invalid value → list (defensive)
  - [ ] Loading branch renders BEFORE view branch (no flash of empty board) (CON-010)
  - [ ] Toggle only mounted when features non-empty (FR-016)

### Component: `orderCards` (pure helper)
- **Unit**: AC-018 — fixture array, assert output order. Must not mutate input.
- **Quality checkpoints**:
  - [ ] P1 before P2 before P3
  - [ ] Same priority: newer `updated_at` first
  - [ ] Input array unchanged after call

### Test levels required (summary)
| Component | smoke | unit | e2e | conformance |
|---|---|---|---|---|
| ViewToggle | ✅ | (optional) | ✅ | N/A |
| KanbanBoard | ✅ | ✅ (orderCards) | ✅ | N/A |
| KanbanColumn | ✅ | — | ✅ | N/A |
| Dashboard | ✅ | — | ✅ | N/A |
| orderCards | — | ✅ | — | N/A |
| Build/lint | ✅ | — | — | N/A |
| E2E config | ✅ | — | — | N/A |

**No conformance tests** — no RFC/standard implemented. All constraints are internal-consistency (CON-001..CON-010), verified by smoke/e2e/unit as mapped.

## Negative Case Design

This feature has no negative conformance vectors (no protocol, no standard). The "negative cases" are edge/error states, each covered by an AC:

| Edge case | Design | AC | Test |
|---|---|---|---|
| Empty column (count 0) | `KanbanColumn` renders empty-state message, not hidden, not error | AC-011, AC-CON-009 | e2e |
| Zero features total | Dashboard renders `EmptyState`, toggle not mounted | AC-019, AC-020 | e2e |
| `listFeatures` loading | Dashboard renders `features-loading`, board not mounted | AC-009, AC-CON-010 | e2e (route intercept with delay) |
| `listFeatures` error | Dashboard renders `features-error`, board not mounted | AC-010, AC-CON-010 | e2e (route intercept with 500) |
| Unknown `current_phase` | `KanbanBoard` appends `other` column; feature not dropped | FR-017 | e2e (if seedable) + code inspection |
| Corrupt localStorage value | Dashboard defaults to `list` | AC-004 | e2e (`localStorage.setItem('devteam-dashboard-view','garbage')`) |
| Card with pending questions badge | Badge renders, does not block Link click | AC-013 | e2e |
| Narrow viewport (6 cols don't fit) | Board `overflow-x-auto`, columns fixed min-width | AC-014, AC-015 | e2e (set viewport 800x600) |
| Long feature title | `FeatureCard` `truncate` class (existing) | (inherited) | e2e (covered by AC-008 content parity) |

## Agent Failure Mode Checks

For each task an AI agent will implement, the following systematic failure modes are checked:

| Task produces | Failure mode | Check |
|---|---|---|
| `Dashboard` state + effect (localStorage) | nil/undefined ordering, effect running before state init | Initialize `view` state lazily from localStorage read *inside* `useState` initializer; effect only *writes*. Read never throws (try/catch). |
| `KanbanBoard` grouping | mutating input array | Use `.filter()` + `orderCards` (which copies). Never `features.sort()` in place. |
| `orderCards` sort | unstable sort, wrong direction | Compare `a.priority - b.priority` (asc); for tiebreaker `new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()` (desc). Unit test catches direction bugs (AC-018). |
| `KanbanColumn` render | empty state rendered as `null` or hidden div | Empty state is a real `<div data-testid="kanban-column-empty-state">` with text. AC-011 catches. |
| `ViewToggle` aria | `aria-pressed` as string vs boolean | React expects `aria-pressed={view === 'list'}` (boolean); renders `"true"`/`"false"`. Test asserts `aria-pressed="true"` on active. |
| JSON serialization | N/A — no JSON produced by this feature (no API). | — |
| HTTP middleware | N/A — no backend. | — |
| Parsing code | N/A — no parsing of external input. localStorage read is `getItem` (string or null), no parse needed. | — |
| Multi-component consistency | `FeatureCard` used in both `FeatureList` and `KanbanColumn` | CON-003 check: grep for `<FeatureCard` import in `KanbanColumn.tsx`; assert no duplicated badge JSX. |
| Language footguns (TS/React) | `PHASE_LABELS[feature.current_phase]` where `current_phase` is `string` not `PhaseName` → TS error | `FeatureCard` already casts `feature.current_phase as PhaseName`. `KanbanBoard` grouping uses `f.current_phase === phase` (string compare, safe). `KanbanColumn` receives `label` as prop (already resolved). No new cast needed. |
| React key warnings | duplicate or missing keys on column cards | Use `feature.id` as key (unique UUID). Already the pattern in `FeatureList`. |

## Quality Checkpoints (task boundaries)

- **After T001 (types)**: `npm run build` passes; `ViewMode` type exported.
- **After T002 (orderCards + unit test)**: `npx vitest run` passes AC-018; `npm run build` passes.
- **After T003 (ViewToggle)**: `npm run build` + `npm run lint` pass; toggle renders in isolation (manual or first e2e).
- **After T004 (KanbanColumn)**: `npm run build` passes; `FeatureCard` imported (CON-003).
- **After T005 (KanbanBoard)**: `npm run build` passes; six columns in `PHASES` order; `other` column conditional.
- **After T006 (Dashboard wiring)**: full smoke — `npm run build`, `npm run lint`, then run e2e smoke subset (toggle + default view).
- **After T007 (e2e suite)**: `npx playwright test kanban.spec.ts` all green on port 18765.
- **Final gate**: all ACs in acceptance.md traced to a passing test; constraint map fully verified; `package.json` diff shows only vitest devDependency added.

## Quickstart Guide (for the Developer)

```bash
# 1. Work in the ui/ subtree. No backend changes.
cd ui

# 2. Add vitest (devDependency only — CON-008 restricts runtime deps)
npm install -D vitest

# 3. Implement in dependency order (see tasks.md):
#    types → orderCards(+test) → ViewToggle → KanbanColumn → KanbanBoard → Dashboard

# 4. Smoke after each file:
npm run build   # tsc + vite build
npm run lint    # eslint

# 5. Unit test for orderCards:
npx vitest run

# 6. E2E (port 18765, config auto-starts server):
npx playwright install chromium   # first time only
npx playwright test kanban.spec.ts --reporter=line

# 7. Verify constraints:
git diff main -- package.json     # only vitest under devDependencies
grep -r "FeatureCard" src/components/KanbanColumn.tsx  # imported, not reimplemented
grep -r "PHASE_LABELS" src/components/KanbanBoard.tsx  # imported, not hardcoded
```

**Order of implementation**: types → orderCards(+unit test) → ViewToggle → KanbanColumn → KanbanBoard → Dashboard wiring → e2e suite. Each step builds on the previous; no circular deps.

**Do NOT**:
- Add runtime dependencies (CON-008).
- Reimplement `FeatureCard` markup inside columns (CON-003).
- Hardcode phase names as string literals (CON-005).
- Change `FeatureCard`, `FeatureList`, `EmptyState`, or any backend/Go file.
- Add drag-and-drop (explicitly out of scope).
- Introduce a state management library — `useState` + one `useEffect` suffice.