# Implementation Plan: kanban-view

**Branch**: `kanban-view` | **Date**: 2026-06-22 | **Spec**: `specs/kanban-view/spec.md`

**Input**: Feature specification from `specs/kanban-view/spec.md`

## Summary

Add a Kanban board view to the Dev Team Dashboard. The board renders the existing `GET /api/features` result as six phase columns (Inception → Delivery) with feature cards. A List/Board toggle on the Dashboard switches between the existing `FeatureList` and the new `KanbanBoard`; the choice persists in `sessionStorage` and defaults to **Board**. The board is view-only (no drag-and-drop), reuses the existing react-query cache (zero new HTTP requests), reuses the existing `PHASES`/`PHASE_LABELS`/`STATUS_LABELS`/`PRIORITY_LABELS` constants and `FeatureCard` badge chrome, and adds no new npm or backend dependencies. Technical approach: 4 new UI components + 1 pure grouping function + 1 session-storage hook + 1 Playwright spec + a bounded Dashboard modification + a bounded regression fix to `app.spec.ts`.

## Technical Context

**Language/Version**: TypeScript 5.8, React 19.1, Vite 6.3

**Primary Dependencies**: `react`, `react-dom`, `react-router` v7, `@tanstack/react-query` v5, `tailwindcss` v4 (via `@tailwindcss/vite`). **No new dependencies added** (CON-003).

**Storage**: None new. View preference in `sessionStorage` key `devteam.dashboard.view`. Backend storage unchanged — the board reads the existing SQLite/Postgres-backed `GET /api/features`.

**Testing**: Playwright e2e (`ui/e2e/*.spec.ts`, port `:18765` — CON-001) is the primary test level for this UI feature. One Vitest-style unit test for the pure `groupFeaturesByPhase` function (AC-011, CON-009). No Go backend tests changed.

**Target Platform**: Modern browsers (Chromium targeted by Playwright). Dark mode supported via existing Tailwind `dark:` variant convention.

**Project Type**: Brownfield web app — React SPA consuming a Go HTTP API. Single repo (`devteam`).

**Performance Goals**: SC-001 — single-click view switch renders within 1 frame of query resolve. SC-006 — first contentful paint < 200ms after `features` query resolves (pure CSS + React, no fetch). Board render is O(features) with no virtualization — sufficient for realistic workspace sizes (< 200 features).

**Constraints**:
- No new runtime npm dependency (CON-003).
- No backend change, no new endpoint, no DTO change (FR-016).
- Exactly one `GET /api/features` request per Dashboard mount (CON-007, AC-016).
- Existing `feature-card-*` / `feature-count-badge` e2e assertions must still pass (CON-004) — `app.spec.ts` gets a bounded regression fix (click-to-List fixture where it asserts list chrome).
- Reuse `PHASES`/`PHASE_LABELS`/`STATUS_LABELS`/`PRIORITY_LABELS` — no duplicated label strings (CON-005).
- Board card badge chrome matches `FeatureCard` (CON-006) — extract shared badge styles.

**Scale/Scope**: Single repo, UI-only. 4 new components (~250 LOC total), 1 hook (~15 LOC), 1 pure function + unit test (~40 LOC), 1 e2e spec (~22 tests tracing AC-001..AC-022), Dashboard modification (~30 LOC delta), `app.spec.ts` regression fix (~10 LOC delta). No backend LOC.

## Constitution Check

GATE: Passed during inception (see `spec.md` § Constitution Compliance). Re-verified for planning:

| Principle | Status | Planning note |
|---|---|---|
| I. Spec-Driven | ✅ | Plan derives solely from `spec.md` + `acceptance.md`. No design decisions contradict the spec. |
| II. Six Roles, Fixed Pipeline | ✅ | This plan does not dictate code or tests — it specifies files, done conditions, and test levels for the Developer/Tester. |
| III. Central Spec, Distributed Implementation | ✅ | Single repo. `repos.yaml` declares primary-only scope. |
| IV. Two Intake Paths | ✅ | N/A to planning. |
| V. Proof-of-Work Gates | ✅ | Every task has verifiable done conditions; every constraint has a verification checkpoint. |
| VI. Cross-Repo Coherence | ✅ | N/A — single repo. |
| VII. Self-Bootstrap | ✅ | Feature improves the platform's own UI. |
| VIII. Go, Minimal Dependencies | ✅ | No new npm runtime dep, no new Go dep. Tailwind + existing React/react-query/react-router only. |
| IX. Pipeline Governance | ✅ | Security/resiliency extensions evaluated and documented N/A for view-only UI (no input, no auth, no mutation, no new external call). Error-recovery + overconfidence-prevention applied. |
| X. Learn From Cistern | ✅ | Structured plan with constraint map + consistency matrix. |

No violations. No complexity-tracking entries needed.

## Project Structure

### Documentation (this feature)

```text
specs/kanban-view/
├── plan.md              # this file
├── research.md          # existing-code patterns + rejected alternatives
├── data-model.md        # FeatureSummary (unchanged) + PhaseColumn (UI-only) + ViewPreference
├── contracts/
│   └── GET-api-features.md   # unchanged endpoint, consumed by the board
└── tasks.md             # task decomposition
```

### Source Code (repository root)

```text
ui/src/
├── components/
│   ├── KanbanBoard.tsx        # [CREATE] board container — 6 columns + optional Other, viewport-bounded
│   ├── KanbanColumn.tsx       # [CREATE] single column — header + scrollable body + empty placeholder
│   ├── KanbanCard.tsx         # [CREATE] board card — Link + badges + gate + ring flags
│   ├── ViewToggle.tsx         # [CREATE] two-button segmented control (List/Board)
│   ├── badgeStyles.ts         # [CREATE] extracted shared status-color map (consumed by FeatureCard + KanbanCard)
│   ├── FeatureCard.tsx        # [MODIFY] import statusColors from badgeStyles.ts instead of module-local (CON-006 parity)
│   └── ... (existing components unchanged)
├── hooks/
│   └── useSessionView.ts      # [CREATE] sessionStorage-backed view state (key: devteam.dashboard.view, default: 'board')
├── lib/
│   └── groupFeaturesByPhase.ts # [CREATE] pure grouping function + unit test co-located or in __tests__
├── pages/
│   └── Dashboard.tsx          # [MODIFY] add toggle + branch between FeatureList / KanbanBoard
└── types/
    └── index.ts               # [UNCHANGED] — reuses PHASES, PHASE_LABELS, STATUS_LABELS, PRIORITY_LABELS, FeatureSummary

ui/e2e/
├── kanban.spec.ts             # [CREATE] 22 tests tracing AC-001..AC-022
└── app.spec.ts                # [MODIFY] bounded regression fix — click-to-List where tests assert feature-card-* on /
```

**Structure Decision**: Components under `ui/src/components/`, hook under `ui/src/hooks/`, pure logic under `ui/src/lib/` (new directory — keeps non-component logic out of `components/`). All new test files under `ui/e2e/` (Playwright convention, CON-001) and co-located unit test for the pure function. This follows the existing brownfield layout (CON-002) — no new top-level directories.

## Component Design

### Component: `ViewToggle`
- **Purpose**: Two-button segmented control that switches the Dashboard view between List and Board.
- **Responsibilities**: render two buttons; reflect active state via `aria-pressed`; call `onChange(value)` on click. Stateless — state lives in the parent via `useSessionView`.
- **Interfaces**: props `{ value: 'board'|'list'; onChange: (v) => void }`. Renders `data-testid="view-toggle"`, `data-testid="view-toggle-board"[aria-pressed=true|false]`, `data-testid="view-toggle-list"[aria-pressed=true|false]` (AC-001, AC-002, AC-003, AC-004, AC-005).
- **Dependencies**: none (pure presentational).

### Component: `KanbanBoard`
- **Purpose**: Render six phase columns (+ optional Other) from the `features` array.
- **Responsibilities**: call `groupFeaturesByPhase(features)`; render a `KanbanColumn` per result bucket in order; bound overall height to viewport; horizontally scroll on narrow viewports.
- **Interfaces**: props `{ features: FeatureSummary[] }`. Root `data-testid="kanban-board"`. Container Tailwind: `flex gap-4 overflow-x-auto` + `h-[calc(100vh-<header>px)]` or equivalent viewport-bound height (FR-014, AC-020, AC-021). Column min-width `240px` (FR-015, AC-022).
- **Dependencies**: `groupFeaturesByPhase`, `KanbanColumn`, `PHASES`/`PHASE_LABELS`.

### Component: `KanbanColumn`
- **Purpose**: One column — header + scrollable body + empty placeholder.
- **Responsibilities**: render header with `PHASE_LABELS[phase]` (or "Other"); render cards in body; render muted "No features" placeholder when empty; body scrolls vertically independent of siblings.
- **Interfaces**: props `{ phase: PhaseName|'other'; label: string; features: FeatureSummary[] }`. Root `data-testid="kanban-column-${phase}"`. Empty placeholder `data-testid="kanban-column-empty-${phase}"` (AC-017). Header `data-testid="kanban-column-${phase}-header"`. Body Tailwind: `overflow-y-auto` + `max-h-[...]` (FR-013, AC-020).
- **Dependencies**: `KanbanCard`.

### Component: `KanbanCard`
- **Purpose**: One feature card on the board — clickable, badge parity with `FeatureCard`, ring flags for blocked/waiting.
- **Responsibilities**: render `<Link to="/features/${id}">` root; render title, status badge (shared `statusColors`), priority badge (`PRIORITY_LABELS`), `QuestionBadge` when `pending_questions_count > 0`, gate indicator (`✓ Gate passed`/`✗ Gate failed`) when `gate_result` present, updated date; apply `ring-red-*` class when `status==='gate_blocked'` and `ring-yellow-*` when `status==='waiting_for_human'` (FR-011, AC-012, AC-013).
- **Interfaces**: props `{ feature: FeatureSummary }`. Root `data-testid="kanban-card-${feature.id}"`. Badge testids: `kanban-card-status`, `kanban-card-priority`, `kanban-card-gate` (AC-007, AC-008, AC-009, AC-010). Reuses `question-badge` testid via `QuestionBadge` (AC-008).
- **Dependencies**: `react-router` `Link`, `QuestionBadge`, `badgeStyles` (shared `statusColors`), `STATUS_LABELS`, `PRIORITY_LABELS`.

### Hook: `useSessionView`
- **Purpose**: sessionStorage-backed view state.
- **Responsibilities**: read `devteam.dashboard.view` on init (default `'board'` if absent/invalid — FR-003); write on change; subscribe to updates.
- **Interfaces**: `() => [value, setValue]`. `setValue('board'|'list')` writes sessionStorage then updates state.
- **Dependencies**: none. ~15 LOC.

### Pure function: `groupFeaturesByPhase`
- **Purpose**: Bucket features by `current_phase` into the six known columns + optional `other`.
- **Responsibilities**: produce `Record<PhaseName, FeatureSummary[]>` for the six known phases (always present, even when empty — FR-012, AC-019) plus an `other` bucket populated only when unknown phases exist (FR-007, AC-011). Preserve API order within buckets. **Invariant**: total input count === total output count (SC-002).
- **Interfaces**: `(features: FeatureSummary[]) => { columns: Array<{ phase: PhaseName|'other'; label: string; features: FeatureSummary[] }> }`. Returns an ordered array so the board can render without re-sorting.
- **Dependencies**: `PHASES`, `PHASE_LABELS`.
- **Defensive behavior**: unknown `current_phase` → `other` bucket, no throw (CON-009, AC-011). Missing `current_phase` (undefined/null) → `other` bucket too (belt-and-suspenders; the API contract says required, but the function must not crash on bad data).

### Modified: `Dashboard.tsx`
- Add `const [view, setView] = useSessionView();`.
- Render `<ViewToggle value={view} onChange={setView} />` in the header **only when** `!isLoading && !error && features.length > 0` (FR-004, AC-006, AC-018).
- In the features-present branch, render `<KanbanBoard features={features} />` when `view==='board'`, else `<FeatureList features={features} />` (FR-001, FR-016).
- Loading/error/empty branches unchanged (FR-017, AC-014, AC-015, AC-006).

### Component Dependency Map
```
Dashboard
  ├─ useSessionView
  ├─ ViewToggle
  └─ (features present) ─┬─ KanbanBoard ─┬─ groupFeaturesByPhase
                         │               └─ KanbanColumn ── KanbanCard ─┬─ QuestionBadge
                         │                                            ├─ badgeStyles (shared with FeatureCard)
                         │                                            └─ Link (react-router)
                         └─ FeatureList (unchanged) ── FeatureCard ── badgeStyles (shared)
```
No circular dependencies. `badgeStyles` is a leaf module with no imports.

## API Contracts

See `contracts/GET-api-features.md`. The board consumes the **unchanged** `GET /api/features` endpoint. No new endpoints, no new request bodies, no new error codes. The contract file documents the response schema and the board-specific single-fetch invariant (CON-007, AC-016).

## Constraint Verification Map

| CON-ID | Design Decision | Component(s) | Verification Checkpoint | Test Type |
|--------|-----------------|--------------|------------------------|-----------|
| CON-001 | New e2e tests live in `ui/e2e/kanban.spec.ts` and run under the existing Playwright config on `:18765` — no new config, no `:8765` | `kanban.spec.ts` | `npm run test:e2e` runs kanban.spec.ts against the existing webServer config; no test references `:8765` | E2E |
| CON-002 | New components under `ui/src/components/`; hook under `ui/src/hooks/`; pure logic under `ui/src/lib/`; e2e under `ui/e2e/` | file paths in tasks.md | Reviewer checks every new file path matches the convention | File-path check |
| CON-003 | No new npm runtime dep. Layout via Tailwind utilities; state via `useState`+`sessionStorage`; grouping via pure function. `ui/package.json` diff is empty | all new files | `git diff ui/package.json` is empty after the feature | Review (diff check) |
| CON-004 | `FeatureList` and `FeatureCard` left intact; Board is additive. `app.spec.ts` tests asserting `feature-card-*` on `/` get a bounded click-to-List fixture (the default is now Board) | `app.spec.ts`, `Dashboard.tsx` | `npm run test:e2e` green — both `app.spec.ts` (with fix) and `kanban.spec.ts` pass | E2E (regression) |
| CON-005 | Board imports `PHASES`, `PHASE_LABELS`, `STATUS_LABELS`, `PRIORITY_LABELS` from `ui/src/types`. No new string literals for phase/status names | `KanbanBoard`, `KanbanColumn`, `KanbanCard`, `groupFeaturesByPhase` | `grep -R "Inception\|Planning\|Construction\|Review\|Testing\|Delivery" ui/src/components/Kanban*` returns only `PHASE_LABELS` references, no literals | Review (grep) |
| CON-006 | Extract `statusColors` from `FeatureCard.tsx` into `ui/src/components/badgeStyles.ts`; both `FeatureCard` and `KanbanCard` import it. Badge classes identical | `badgeStyles.ts`, `FeatureCard.tsx`, `KanbanCard.tsx` | Visual/class-name parity in review; `kanban-card-status` has same Tailwind classes as `feature-card-status` for a given status | E2E (AC-007) + review |
| CON-007 | Board consumes the same `useQuery(['features'])` result as the list — no second `useQuery` call in any board component | `Dashboard.tsx`, `KanbanBoard.tsx` | Playwright `page.on('request')` count for `/api/features` === 1 during Board render (AC-016) | Integration |
| CON-008 | Dashboard loading/error/empty branches unchanged; board renders only in the features-present branch; empty columns render placeholder | `Dashboard.tsx`, `KanbanColumn.tsx` | AC-006 (empty→EmptyState, toggle hidden), AC-014 (loading→features-loading), AC-015 (error→features-error), AC-017 (empty column placeholder) | E2E |
| CON-009 | `groupFeaturesByPhase` buckets unknown `current_phase` into `other`; never throws | `groupFeaturesByPhase.ts` | Unit test: input `{current_phase:'weird'}` → output `other` bucket contains it; input `{current_phase:undefined}` → `other`; total count preserved (AC-011) | Unit |

## Cross-Component Consistency Matrix

This feature has multiple UI components that must agree on shared values (badge classes, label maps, testid scheme, navigation target). Tracing every shared value across producers and consumers:

| Shared Value | Producer | Consumer | Consistent? | Verification |
|---|---|---|---|---|
| Status badge Tailwind classes | `badgeStyles.ts` (`statusColors`) | `FeatureCard` (list), `KanbanCard` (board) | YES — single source of truth after extraction (CON-006) | E2E AC-007 asserts board badge text matches `PRIORITY_LABELS`/`STATUS_LABELS`; review asserts identical class strings for a given status |
| Phase labels | `PHASE_LABELS` (`types/index.ts`) | `KanbanColumn` header, `KanbanBoard` column order | YES — both import `PHASE_LABELS` and `PHASES`; no literals (CON-005) | E2E AC-002/AC-019 assert column header text matches `PHASE_LABELS` |
| Status labels | `STATUS_LABELS` (`types/index.ts`) | `KanbanCard` status badge | YES — import, no literal | E2E AC-007 asserts "In Progress" text |
| Priority labels | `PRIORITY_LABELS` (`types/index.ts`) | `KanbanCard` priority badge | YES — import, no literal | E2E AC-007 asserts "P1 - Critical" text |
| Gate indicator text | `KanbanCard` (board), `FeatureCard` (list) | Playwright AC-009 | YES — both render `✓ Gate passed` / `✗ Gate failed` | E2E AC-009 asserts exact string on board card |
| Card navigation target | `KanbanCard` `<Link to="/features/${id}">`, `FeatureCard` `<Link to="/features/${id}">` | `react-router` route `/features/:id` | YES — identical URL pattern (FR-010) | E2E AC-010 clicks `kanban-card-${id}` and asserts `URL =~ /\/features\/${id}/` |
| testid prefix scheme | `KanbanCard` `kanban-card-${id}`, `KanbanColumn` `kanban-column-${phase}` | Playwright `kanban.spec.ts` selectors | YES — plan specifies exact testids; tests assert them | E2E AC-007, AC-010, AC-017, AC-019 |
| View preference key | `useSessionView` writes `devteam.dashboard.view` | `useSessionView` reads same key | YES — single hook owns both (FR-002) | E2E AC-004 (reload preserves Board), AC-005 (fresh session defaults Board) |
| Features data source | `Dashboard` `useQuery(['features'])` | `KanbanBoard` (via props), `FeatureList` (via props) | YES — single query, prop-drilled; no second `useQuery` in board (CON-007) | Integration AC-016 — exactly one `/api/features` request |
| Ring flag classes | `KanbanCard` applies `ring-red-*` (gate_blocked), `ring-yellow-*` (waiting_for_human) | Playwright AC-012/AC-013 | YES — plan pins the classes; tests assert them | E2E AC-012, AC-013 |
| Empty placeholder text | `KanbanColumn` renders "No features" | Playwright AC-017 | YES — plan pins the text; test asserts it | E2E AC-017 |
| Column min-width | `KanbanBoard`/`KanbanColumn` `min-w-[240px]` | Playwright AC-022 | YES — plan pins 240px; test asserts `getBoundingClientRect().width >= 240` | E2E AC-022 |

**Multi-component constraint check**: CON-005 (reuse label maps) applies to **four** components — `KanbanBoard`, `KanbanColumn`, `KanbanCard`, `groupFeaturesByPhase`. All four import from `types/index.ts`; none define local label literals. CON-006 (badge parity) applies to **two** components — `FeatureCard` and `KanbanCard`. Both import `statusColors` from `badgeStyles.ts` after extraction. No component is missed.

## Test Strategy

Per-component testing levels (from the Test Level Selection Matrix: UI components → smoke + integration + e2e + unit):

### `useSessionView`
- **Unit**: defaults to `'board'` on fresh session; persists `'list'`/`'board'` to sessionStorage; reads back on re-init; ignores invalid stored values (falls back to `'board'`).
- **E2E**: AC-004 (reload preserves Board), AC-005 (fresh session defaults Board).
- Quality checkpoints:
  - [ ] hook does not throw when `sessionStorage` is unavailable (SSR/old browser) — falls back to in-memory state with default `'board'`.

### `groupFeaturesByPhase`
- **Unit** (mandatory — only non-trivial logic in the feature):
  - empty input → 6 known buckets, each `features: []`, no `other` bucket (AC-019 invariant).
  - all-known-phases input → features in correct buckets, no `other` bucket, total count preserved (SC-002).
  - unknown `current_phase` → `other` bucket populated, known buckets intact (AC-011, CON-009).
  - `current_phase` undefined/null → `other` bucket (defensive).
  - order preserved within buckets.
- Quality checkpoints:
  - [ ] `sum(out.features.length) === in.length` for every test case (SC-002 invariant).
  - [ ] no throw on any input shape.

### `ViewToggle`
- **E2E**: AC-001 (both buttons visible, Board active by default), AC-002 (click Board), AC-003 (click List).
- Quality checkpoints:
  - [ ] `aria-pressed` reflects active state (AC-001, AC-004, AC-005).
  - [ ] toggle hidden when `features.length === 0` (AC-006, AC-018).

### `KanbanBoard` / `KanbanColumn`
- **E2E**: AC-002 (6 columns render), AC-017 (empty column placeholder), AC-019 (exactly 6 columns + optional Other), AC-020 (column scrolls independently), AC-021 (viewport-resize adjusts), AC-022 (narrow viewport horizontal scroll + 240px min-width).
- Quality checkpoints:
  - [ ] exactly 6 `kanban-column-*` elements when no unknown phases (AC-019).
  - [ ] `other` column appears only when unknown phase exists (AC-019).
  - [ ] column body `scrollHeight > clientHeight` when overflowing (AC-020).
  - [ ] page body does not scroll (AC-020: `scrollTop === 0`).
  - [ ] no console errors (SC-004).

### `KanbanCard`
- **E2E**: AC-007 (title + P1 + In Progress badges), AC-008 (QuestionBadge visible), AC-009 (gate indicator string), AC-010 (click navigates), AC-012 (gate_blocked red ring), AC-013 (waiting_for_human yellow ring).
- Quality checkpoints:
  - [ ] `kanban-card-${id}` testid present and unique.
  - [ ] badge text matches `PRIORITY_LABELS`/`STATUS_LABELS` exactly (no literal drift).
  - [ ] gate text is exactly `✓ Gate passed` or `✗ Gate failed` (AC-009).
  - [ ] `<Link>` navigates to `/features/${id}` (AC-010).

### `Dashboard.tsx` (modified)
- **E2E**: AC-006 (empty → EmptyState, toggle hidden), AC-014 (loading → features-loading, no columns), AC-015 (error → features-error, no columns), AC-016 (single fetch).
- **Integration**: AC-016 (request count === 1).
- Quality checkpoints:
  - [ ] loading branch unchanged — `features-loading` still renders (regression).
  - [ ] error branch unchanged — `features-error` still renders (regression).
  - [ ] empty branch unchanged — `EmptyState` still renders (regression).
  - [ ] `feature-count-badge` still renders in header regardless of view (regression).

### `app.spec.ts` (regression fix)
- **E2E**: existing tests still pass after the default flips to Board. Tests asserting `feature-card-*` on `/` must first click `view-toggle-list` (or assert `kanban-card-*` where appropriate).
- Quality checkpoints:
  - [ ] `npm run test:e2e app.spec.ts` green.
  - [ ] no test deleted or skipped (CON-004).

### Smoke (whole feature)
- [ ] `npm run build` (tsc + vite) succeeds — no type errors.
- [ ] `npm run lint` succeeds.
- [ ] Playwright webServer starts on `:18765`, `/` loads without console errors (SC-004).

### Test level summary
| Component | Smoke | Integration | E2E | Unit |
|---|---|---|---|---|
| `useSessionView` | YES | — | YES | YES |
| `groupFeaturesByPhase` | — | — | — | **YES** |
| `ViewToggle` | YES | — | YES | — |
| `KanbanBoard`/`KanbanColumn` | YES | — | YES | — |
| `KanbanCard` | YES | — | YES | — |
| `Dashboard` (modified) | YES | YES (AC-016) | YES | — |
| `app.spec.ts` (regression) | YES | — | YES | — |

No conformance tests — this feature is not governed by an external standard/RFC (spec § Constraint Register).

## Agent Failure Mode Checks

Tasks an AI agent will implement, mapped to the applicable checks:

| Task | Produces | Failure-mode check |
|---|---|---|
| `groupFeaturesByPhase` | parsing/bucketing logic | **All input shapes caught** — unknown/undefined/null `current_phase` → `other`, never throws (CON-009). Unit test covers each. |
| `useSessionView` | initialization + state | **Nil pointer ordering** — `useState` initializer reads `sessionStorage.getItem` which may throw in restricted environments; wrap in try/catch, fall back to `'board'`. Do NOT dereference before init. |
| `KanbanBoard`/`KanbanColumn`/`KanbanCard` | JSX serialization | **JSON arrays [] not null** — N/A (no JSON serialization in UI), but the parallel: empty `features` array must render the placeholder, not crash on `.map`. Always default `features ?? []` at the bucket level. |
| Dashboard modification | conditional rendering | **State transitions** — view state has only `board`/`list`; no invalid transition. Verify toggle hidden when `features.length === 0` (FR-004) — an `if` guard, not a state transition. |
| `app.spec.ts` regression fix | test modification | **Multi-component consistency** — the fix must be applied to EVERY test that asserts `feature-card-*` on `/`, not just the first. Grep the spec for `feature-card` and fix each assertion. |
| `badgeStyles` extraction | refactor across 2 components | **Multi-component consistency** — after extraction, BOTH `FeatureCard` and `KanbanCard` must import from `badgeStyles.ts`; the module-local `statusColors` in `FeatureCard.tsx` must be removed, not left as dead code. Grep verifies. |
| Tailwind classes | CSS | **Dark mode** — every new color class must have a `dark:` companion (existing convention). Reviewer greps for `bg-`/`text-` without `dark:` in new files. |
| Playwright spec | network mocking | **AC-016 request count** — `page.on('request')` listener must be attached BEFORE `page.goto('/')` or the first request is missed. Known Playwright footgun. |

## Quality Checkpoints (task boundaries)

1. **After T001 (badgeStyles extraction)**: `npm run build` + `npm run test:e2e app.spec.ts` green — proves the refactor didn't break the list view (CON-004, CON-006).
2. **After T002 (groupFeaturesByPhase)**: unit test passes — proves the grouping invariant (SC-002) and the unknown-phase defensive case (CON-009, AC-011) before any UI depends on it.
3. **After T003 (useSessionView)**: unit test passes — proves default `'board'` and persistence before Dashboard wires it up.
4. **After T004 (KanbanCard + KanbanColumn + KanbanBoard)**: `npm run build` green — components compile and type-check. No e2e yet (Dashboard not wired).
5. **After T005 (Dashboard wiring + ViewToggle)**: `npm run test:e2e kanban.spec.ts` — AC-001..AC-019 pass (P1+P2). AC-020..AC-022 (overflow) may pass too but are P3.
6. **After T006 (app.spec.ts regression fix)**: `npm run test:e2e` (full suite) green — both specs pass (CON-004).
7. **After T007 (kanban.spec.ts overflow tests)**: AC-020..AC-022 pass.
8. **Final gate**: `npm run build && npm run lint && npm run test:e2e` all green; `git diff ui/package.json` empty (CON-003); grep shows no new phase/status label literals in board components (CON-005).

## Quickstart Guide for the Developer

```bash
# from repo root
export PATH="$PATH:/usr/local/go/bin"

# 1. Build the backend binary the Playwright webServer needs
go build -o ~/go/bin/devteam ./cmd/devteam/

# 2. UI dev loop
cd ui
npm install
npm run dev          # vite dev server (optional — for manual eyeballing)

# 3. Type-check + build
npm run build        # tsc -b && vite build — must pass with zero errors

# 4. Lint
npm run lint

# 5. Run the full e2e suite (starts webServer on :18765 automatically)
npm run test:e2e
# or target just the new spec:
npx playwright test kanban.spec.ts
npx playwright test app.spec.ts

# 6. Manual smoke: open the app, click List/Board toggle, click a card, verify navigation.
```

**Implementation order** (from tasks.md): T001 → T002 → T003 → T004 → T005 → T006 → T007. T002 and T003 can run in parallel (different files, no deps).

**Files to read before starting**: `ui/src/pages/Dashboard.tsx`, `ui/src/components/FeatureCard.tsx`, `ui/src/components/FeatureList.tsx`, `ui/src/components/QuestionBadge.tsx`, `ui/src/components/EmptyState.tsx`, `ui/src/types/index.ts`, `ui/e2e/app.spec.ts`. All quoted in research.md.

**Don't**:
- Don't add any npm dependency (CON-003).
- Don't add any backend code or API endpoint (FR-016).
- Don't add a second `useQuery` call in any board component (CON-007).
- Don't duplicate `PHASES`/`PHASE_LABELS`/`STATUS_LABELS`/`PRIORITY_LABELS` — import them (CON-005).
- Don't leave the module-local `statusColors` in `FeatureCard.tsx` after extracting to `badgeStyles.ts` (CON-006 — dead code).
- Don't delete or skip existing `app.spec.ts` tests (CON-004) — fix them by clicking the List toggle first.
- Don't add drag-and-drop (out of scope — spec ASSUMPTION).