# Implementation Plan: kanban-view

**Branch**: `kanban-view` | **Date**: 2026-06-22 | **Spec**: `specs/kanban-view/spec.md`

**Input**: Feature specification from `specs/kanban-view/spec.md`

## Summary

Add a Kanban board view to the Dev Team Dashboard. The board renders features as cards grouped into six phase columns (Inception → Delivery) plus a defensive "Other" column for unknown phases. A List/Kanban toggle switches between the existing `FeatureList` and the new `KanbanBoard`; **list view remains the default**, the selection persists in `localStorage['devteam.dashboard.view']` (wrapped, falls back to list on any error). The board is read-only (no drag-and-drop), consumes the existing `useQuery(['features'])` data (no new fetch, no backend change), and reuses the existing loading/error/empty Dashboard branches. All layout via Tailwind utilities — no new npm dependencies (runtime or dev).

Technical approach: **two files touched**. `ui/src/components/KanbanBoard.tsx` (CREATE) — board container, pure `groupFeaturesByPhase` helper, column rendering, reuses `FeatureCard` as the card. `ui/src/pages/Dashboard.tsx` (MODIFY) — inline view toggle + `localStorage`-backed view state, conditional render of `<KanbanBoard>` vs `<FeatureList>`. One new Playwright spec `ui/e2e/kanban.spec.ts` covers AC-001–AC-019. `FeatureCard`, `FeatureList`, `EmptyState`, `QuestionBadge`, types, and the API client are reused unchanged.

## Technical Context

**Language/Version**: TypeScript 5.8, React 19.1. Go 1.x backend (unchanged).

**Primary Dependencies**: `react`, `react-dom`, `react-router` v7, `@tanstack/react-query` v5, `tailwindcss` v4. **No new dependencies** (CON-007 — no `package.json` change at all).

**Storage**: `localStorage` (browser) under key `devteam.dashboard.view`. No server-side storage. No DB change.

**Testing**: Playwright e2e (`ui/e2e/*.spec.ts`, port `18765`). Used for every AC including the ones acceptance.md labels "unit" — the repo has no JS unit-test runner installed and CON-007 favours zero new deps; Playwright `page.route` + `page.addInitScript` covers the same assertions (see `research.md` "Test runner decision"). No Vitest added.

**Target Platform**: Web browser (Chrome/Firefox/Safari). Playwright on `:18765` (CON-002).

**Project Type**: Web app (Go backend + React frontend, single repo, frontend-only change).

**Performance Goals**: Board renders within 200ms of the features query resolving (SC-001). Pure CSS + React render, no fetch. Trivially met.

**Constraints**:
- No new npm dependency, runtime or dev (CON-007). Maximally honors constitution VIII.
- No backend change, no new endpoint, no new fetch (FR-014, AC-004).
- Reuse `PHASES`/`PHASE_LABELS`/`STATUS_LABELS`/`PRIORITY_LABELS` — no duplicated phase strings (CON-004, FR-015).
- Card chrome parity with `FeatureCard` (FR-004, FR-010, FR-011, US-003) — achieved by **reusing `FeatureCard`**.
- Existing `app.spec.ts` list-view assertions still pass unmodified (list is the default, FR-008).
- E2E on `:18765` only; no `8765` references in new files (CON-001, CON-002, CON-003).
- Every new rendered element carries a `data-testid` (CON-008).

**Scale/Scope**: Single repo, `ui/` only. 1 new source file, 1 modified source file, 1 new test file. Workspaces with 0–50+ features (SC-002: all 50 cards in the DOM, no virtualisation).

## Constitution Check

GATE: Passed. Constitution at `.specify/memory/constitution.md` (v1.1, ratified 2026-06-19).

| Principle | Status | Note |
|---|---|---|
| I. Spec-Driven | ✅ | Plan derives from `spec.md` + `acceptance.md` + `repos.yaml` (CON-005). |
| II. Six Roles, Fixed Pipeline | ✅ | Architect produces plan/tasks only; no construction/review artifacts created. |
| III. Central Spec, Distributed Implementation | ✅ | Single spec in devteam repo; `repos.yaml` declares `ui/` scope. |
| IV. Two Intake Paths, One Output Format | ✅ | Loose-idea intake produced the spec artifacts. |
| V. Proof-of-Work Gates | ✅ | Done conditions name specific `data-testid` assertions; E2E spec names files (CON-006). |
| VI. Cross-Repo Coherence | ✅ | Single repo; no cross-repo coordination. |
| VII. Self-Bootstrap | ✅ | Feature improves the platform's own UI. |
| VIII. Go, Minimal Dependencies | ✅ | No backend change. **Zero `package.json` change** — stronger than the letter of CON-007. |
| IX. Pipeline Governance | ✅ | Security/resiliency extensions N/A (read-only view, no input, no auth boundary, no external call); documented in spec. |
| X. Learn From Cistern | ✅ | Structured context, distinct phase gates. |

No violations. No complexity-tracking entries.

## Project Structure

### Documentation (this feature)

```text
specs/kanban-view/
├── plan.md              # this file
├── research.md          # existing-pattern analysis + alternatives + test-runner decision
├── data-model.md        # FeatureSummary (existing) + DashboardView (new UI-only)
├── contracts/
│   └── GET-api-features.md   # read-only contract for the consumed endpoint
└── tasks.md             # task breakdown
```

### Source code (repository root — `ui/` only)

```text
ui/
├── src/
│   ├── pages/
│   │   └── Dashboard.tsx        # MODIFY — toggle + localStorage + conditional Board/List
│   ├── components/
│   │   ├── KanbanBoard.tsx      # CREATE — groupFeaturesByPhase + columns + reuses FeatureCard
│   │   ├── FeatureCard.tsx      # REUSE — already covers FR-004/005/010/011/015
│   │   ├── FeatureList.tsx      # REUSE — unchanged (FR-006, SC-005)
│   │   ├── EmptyState.tsx       # REUSE — rendered above both views (CON-010)
│   │   └── QuestionBadge.tsx    # REUSE — via FeatureCard
│   └── types/index.ts           # REUSE — PHASES, PHASE_LABELS, FeatureSummary
└── e2e/
    ├── app.spec.ts              # REUSE — unchanged (list is default)
    └── kanban.spec.ts           # CREATE — AC-001..AC-019
```

**Structure decision**: minimum-diff brownfield. One new component, one modified page, one new test. No new directories, no abstraction layers. A separate `KanbanCard`/`KanbanColumn`/`ViewToggle`/`useViewPreference` would each be one-purpose files shorter than their props boilerplate — rejected (see `research.md`).

## Component Design

### KanbanBoard (CREATE — `ui/src/components/KanbanBoard.tsx`)

**Purpose**: render features grouped into phase columns.

**Responsibilities**:
- Group features by `current_phase` using a pure helper `groupFeaturesByPhase(features): { phase: string; label: string; features: FeatureSummary[] }[]`.
- Render six columns in `PHASES` order, each with `data-testid="kanban-column-<phase>"`, a header using `PHASE_LABELS[phase]`, and the column's features as `<FeatureCard>` elements.
- Append an "Other" column (`data-testid="kanban-column-other"`, label `"Other"`) **iff** any feature has `current_phase` not in `PHASES` (FR-013, AC-016/017).
- Preserve API order within each column (FR-003 — no re-sort).
- Horizontally scrollable container for narrow viewports (`overflow-x-auto`); columns are flex items with a fixed minimum width.

**Interfaces**:
- Props: `{ features: FeatureSummary[] }` → renders the board. Pure on props.
- Exports `groupFeaturesByPhase` (named) for direct testing via Playwright `page.addInitScript` if needed (AC-016/017/018 assert through the rendered DOM, which exercises the helper end-to-end).

**Dependencies**:
- `FeatureCard` (reuse) — renders each card.
- `PHASES`, `PHASE_LABELS`, `FeatureSummary` from `../types`.

**Agent failure mode checks**:
- [ ] **Null/undefined `features`**: caller (`Dashboard.tsx`) guards with `features.length === 0` branch above; `KanbanBoard` still defends with `features ?? []` at the top of `groupFeaturesByPhase`.
- [ ] **Unknown `current_phase`**: must not throw — routes to "Other" (CON-011, AC-016). No `switch` without a default; use a Map/Set membership check.
- [ ] **Empty columns": six columns always render even with zero features (FR-012, AC-007). Do not filter out empty columns.
- [ ] **JSON/`null` arrays**: the board consumes `FeatureSummary[]` already validated by `Dashboard`; no serialization produced. N/A.
- [ ] **`data-testid` coverage** (CON-008): board root, each column, each card (cards already tagged by `FeatureCard`). No literal phase strings as testids — use the phase identifier from `PHASES`.

### Dashboard (MODIFY — `ui/src/pages/Dashboard.tsx`)

**Purpose**: own the view toggle and conditional render.

**Responsibilities** (additions to existing):
- Hold `view: 'list' | 'kanban'` state, initialised lazily from `localStorage['devteam.dashboard.view']` via a wrapped read (FR-008, FR-009, AC-009, AC-011).
- Render a two-button toggle (`view-toggle-list`, `view-toggle-kanban`) with `aria-pressed` on the active button (CON-008, FR-001).
- On toggle, `setView` and persist to `localStorage` in a try/catch (FR-007, FR-009, AC-010).
- In the happy-path branch (`!isLoading && !error && features.length > 0`), render `<KanbanBoard features={features} />` when `view === 'kanban'` else `<FeatureList features={features} />`.
- Loading, error, and empty branches stay **above** the view switch and are unchanged (CON-009, CON-010, AC-005/006/007). The toggle is only rendered in the happy path (or always rendered but disabled while loading — pick the simpler: render toggle only when `!isLoading && !error`; empty state keeps its own CTA, no toggle needed there).

**Interfaces**:
- No new props (top-level page).
- `localStorage` key: `devteam.dashboard.view`. Accepted values: `'kanban'` → kanban; anything else (including `'list'`, malformed, absent) → `'list'`.

**Dependencies**:
- `KanbanBoard` (new), `FeatureList` (existing), `EmptyState` (existing).

**Agent failure mode checks**:
- [ ] **`localStorage` throws on read** (private mode): wrapped in try/catch, defaults to `'list'` (FR-009, AC-011). No uncaught exception.
- [ ] **`localStorage` throws on write**: wrapped in try/catch, view still updates in-memory for the session (FR-009, AC-010).
- [ ] **Nil/undefined state**: `useState` initialised with a lazy initializer that never throws.
- [ ] **Rapid toggle**: no fetch re-trigger (FR-014, AC-004) — both views consume the same `useQuery(['features'])` result; verified by network-request count in `kanban.spec.ts`.
- [ ] **Existing list-view regression**: `FeatureList` rendering path is byte-for-byte unchanged (SC-005). Only the wrapping conditional is added.

## API Contracts

See `contracts/GET-api-features.md`. No new endpoints. The board consumes the existing `GET /api/features` response via the existing `listFeatures()` client.

## Constraint Verification Map

| CON-ID | Design Decision | Component(s) | Verification Checkpoint | Test Type |
|--------|-----------------|--------------|------------------------|-----------|
| CON-001 | Use `npm run build` / `npm run lint` / `npm run test:e2e` from `ui/` per AGENTS.md. No new commands. | `ui/` (build) | `cd ui && npm run build && npm run lint && npm run test:e2e` all succeed (tasks.md T-FINAL) | Smoke + e2e |
| CON-002 | Playwright config unchanged; new `kanban.spec.ts` uses `baseURL` (`:18765`) from config. No port literal in new files. | `ui/e2e/kanban.spec.ts` | `grep -r 18765 ui/e2e/kanban.spec.ts` returns 0 (uses `page.goto('/')`); suite runs on `:18765` | E2E |
| CON-003 | No `8765` literal in new files. | `KanbanBoard.tsx`, `kanban.spec.ts` | `grep -rn 8765 ui/src/components/KanbanBoard.tsx ui/e2e/kanban.spec.ts ui/src/pages/Dashboard.tsx` returns 0 | Conformance (grep) |
| CON-004 | `KanbanBoard.tsx` imports `PHASES`/`PHASE_LABELS` from `../types`; no literal `'Inception'`/`'Planning'`/etc. (exception: `"Other"` fallback, commented). | `KanbanBoard.tsx` | `grep -nE "'(Inception|Planning|Construction|Review|Testing|Delivery)'" ui/src/components/KanbanBoard.tsx` returns 0 | Conformance (grep) |
| CON-005 | Plan proceeds only after `spec.md` + `acceptance.md` + `repos.yaml` exist. They exist (verified at planning gate). | spec dir | Gate check passed before this plan was written | Process |
| CON-006 | E2E test report (testing phase) names specific files, methods, assertions — `kanban.spec.ts` test titles reference AC-IDs. | `ui/e2e/kanban.spec.ts` | Testing-phase gate verifies named test cases | Process |
| CON-007 | Zero `package.json` change. No new runtime or dev dependency. Playwright used for all tests. | `ui/package.json` | `git diff ui/package.json` empty after implementation | Conformance (diff) |
| CON-008 | Every new rendered element has a `data-testid`: board root, columns, toggle buttons. Cards reuse `FeatureCard`'s `feature-card-<id>`. | `KanbanBoard.tsx`, `Dashboard.tsx` | Code review + `kanban.spec.ts` selects every element by `data-testid` | E2E |
| CON-009 | Loading (`features-loading`) and error (`features-error`) branches stay above the view switch; board not rendered while loading/erroring. | `Dashboard.tsx` | AC-005, AC-006 in `kanban.spec.ts` | Smoke + e2e |
| CON-010 | `EmptyState` renders when `features.length === 0` (existing branch unchanged); kanban view additionally renders six empty columns. | `Dashboard.tsx`, `KanbanBoard.tsx` | AC-007 in `kanban.spec.ts` | E2E |
| CON-011 | `groupFeaturesByPhase` routes unknown `current_phase` to a trailing "Other" column; never throws. | `KanbanBoard.tsx` | AC-016, AC-017 in `kanban.spec.ts` | E2E (behavioral unit) |

## Cross-Component Consistency Matrix

This feature is single-repo, read-only, with no protocol/standard and no producer/consumer pair beyond the existing API → UI flow. The matrix is trivial but documented for completeness.

| Shared Value | Producer | Consumer | Consistent? | Verification |
|---|---|---|---|---|
| `current_phase` enum | Backend `GET /api/features` | `KanbanBoard.groupFeaturesByPhase` | YES — UI accepts any string; known `PHASES` → named columns, unknown → "Other" (CON-011). No rejection path. | `kanban.spec.ts` AC-016 (`'rolling_out'` → Other column) |
| Phase labels | `PHASE_LABELS` in `types/index.ts` | `KanbanBoard` column headers, `FeatureCard` phase badge | YES — both import the same constant (CON-004). | `grep` confirms no literal phase strings in `KanbanBoard.tsx` |
| `FeatureSummary` shape | `api/client.ts` `listFeatures()` | `Dashboard.tsx` → `KanbanBoard` / `FeatureList` → `FeatureCard` | YES — unchanged type; both views consume the same `useQuery(['features'])` result (FR-014). | `kanban.spec.ts` AC-004 (one fetch on toggle) |
| `data-testid` namespace | `FeatureCard` (`feature-card-<id>`) | `kanban.spec.ts` selectors | YES — board reuses `FeatureCard`, so card testids are identical across views. | `kanban.spec.ts` AC-001/002/018 use `[data-testid^="feature-card-"]` |
| `localStorage` key | `Dashboard.tsx` write | `Dashboard.tsx` read on mount | YES — single constant `DEVTEAM_DASHBOARD_VIEW = 'devteam.dashboard.view'` in `Dashboard.tsx`; same key read/written. | AC-008 (reload restores), AC-009 (clear → list) |

No multi-component producer/consumer inconsistency risk: there is exactly one producer (the existing API) and one consumer (the Dashboard query), shared by both views.

## Test Strategy

### Component: KanbanBoard

Testing levels required:
- **Smoke**: board renders without console errors when `features` is empty, has one feature, or has 50 features (SC-002, AC-019).
- **Integration**: column grouping matches `current_phase` for one feature per phase (AC-018); 50 features all present in the DOM (AC-019); pending-questions badge and gate indicator render via `FeatureCard` (AC-012/013/014/015).
- **E2E**: user clicks `view-toggle-kanban` and sees six labelled columns with the right cards (AC-001); clicks a card → `/features/:id` (AC-002); clicks `view-toggle-list` → `feature-list` returns (AC-003).
- **Unit (behavioral, via Playwright)**: unknown `current_phase` → "Other" column (AC-016); no unknown phases → no "Other" column (AC-017).

Quality checkpoints:
- [ ] Board renders six `kanban-column-<phase>` elements for `PHASES` regardless of feature distribution (FR-012).
- [ ] "Other" column appears iff any feature has unknown `current_phase` (FR-013).
- [ ] No literal phase label strings in source (CON-004).
- [ ] No console errors on any fixture (SC-002, AC-019).

### Component: Dashboard (toggle + persistence)

Testing levels required:
- **Smoke**: loading and error branches still render `features-loading` / `features-error` and no `kanban-column-*` while loading/erroring (AC-005, AC-006).
- **Integration**: toggling views does not issue a second `GET /api/features` (AC-004).
- **E2E**: toggle to kanban → board; toggle to list → `feature-list` (AC-003); reload restores kanban (AC-008); clear localStorage → list (AC-009).
- **Unit (behavioral, via Playwright)**: `localStorage.setItem` throws → board still renders for the session, no uncaught exception (AC-010); `localStorage.getItem` throws → list view, no crash (AC-011).

Quality checkpoints:
- [ ] `localStorage` access wrapped in try/catch on both read and write paths (FR-009).
- [ ] Default view is `'list'` (FR-008, AC-009).
- [ ] Existing `app.spec.ts` passes unmodified (SC-006, CON-001).
- [ ] No `8765` literal in modified `Dashboard.tsx` (CON-003).

### Test level selection (per planning matrix)

| What changed | Smoke | Integration | E2E | Unit |
|---|---|---|---|---|
| Frontend/UI components (`KanbanBoard`, `Dashboard` toggle) | **YES** | **YES** | **YES** | YES (behavioral via Playwright) |

No HTTP API handlers, state machines, middleware, or DB operations are touched — those rows are N/A.

## Negative Case Design

The constraint register has no RFC/standard negative vectors. The negative cases are UI edge cases, each converted to a Playwright spec:

| Case | Expected rejection / fallback | Test |
|---|---|---|
| Unknown `current_phase` (CON-011, AC-016) | Feature routed to "Other" column, not dropped, no crash | `kanban.spec.ts` mocks `/api/features` with `current_phase: 'rolling_out'`; asserts `kanban-column-other` contains the card and six standard columns also exist |
| No unknown phases (AC-017) | "Other" column absent | `kanban.spec.ts` mocks all-known-phases response; asserts `kanban-column-other` count is 0 |
| `localStorage.setItem` throws (FR-009, AC-010) | View updates in-memory, no uncaught exception | `kanban.spec.ts` `page.addInitScript` overrides `localStorage.setItem` to throw; toggles to kanban; asserts `kanban-column-*` appear and no `pageerror` fired |
| `localStorage.getItem` throws (FR-009, AC-011) | Defaults to list, no crash | `kanban.spec.ts` `page.addInitScript` overrides `localStorage.getItem` to throw; loads `/`; asserts `feature-list` visible, no `pageerror` |
| API error (CON-009, AC-006) | `features-error` visible, no `kanban-column-*` | `kanban.spec.ts` mocks `/api/features` → 500; toggles to kanban; asserts `features-error` visible and `kanban-column-*` count 0 |
| API loading (CON-009, AC-005) | `features-loading` visible, no `kanban-column-*` | `kanban.spec.ts` mocks `/api/features` to never resolve; toggles to kanban; asserts `features-loading` visible and `kanban-column-*` count 0 |
| Empty features (CON-010, AC-007) | `EmptyState` CTA visible + six empty columns | `kanban.spec.ts` mocks `/api/features` → `{features:[],total_count:0}`; toggles to kanban; asserts `empty-state-create-button` visible and six `kanban-column-*` |

## Quality Checkpoints (task boundaries)

1. **After T001 (`KanbanBoard.tsx` create)**: `npm run build` succeeds; `grep` for literal phase strings returns 0; `grep` for `8765` returns 0; manual `npm run dev` + toggle shows six columns with a seeded feature.
2. **After T002 (`Dashboard.tsx` modify)**: toggle switches views; reload restores kanban; `localStorage` throw path does not crash (manual `addInitScript` check); `app.spec.ts` still passes (list default).
3. **After T003 (`kanban.spec.ts` create)**: `npm run test:e2e` passes all new specs AND `app.spec.ts` unmodified; AC-001..AC-019 each traced to a named test.
4. **T-FINAL (gate)**: `npm run lint && npm run build && npm run test:e2e` all green; `git diff ui/package.json` empty; `git diff ui/playwright.config.ts` empty; no `8765` in new/modified files.

## Quickstart Guide for the Developer

```bash
cd ui
npm install            # one-time; no new deps added
npm run dev            # dev server; backend proxy on :8080

# In another terminal, run the backend on :8080 so /api/features resolves:
PATH="$PATH:/usr/local/go/bin" go build -o ~/go/bin/devteam ./cmd/devteam/
~/go/bin/devteam -http :8080    # or use the running production binary

# Verify manually:
# 1. Open http://localhost:5173 (Vite dev) — list view is default.
# 2. Click "Kanban" — six phase columns render with features.
# 3. Click a card — navigates to /features/<id>.
# 4. Click "List" — feature-list grid returns.
# 5. Reload after selecting Kanban — board restores.

# Run the full gate:
npm run lint
npm run build
npm run test:e2e         # uses :18765 per playwright.config.ts
```

**Implementation order**: T001 (KanbanBoard) → T002 (Dashboard wiring) → T003 (kanban.spec.ts) → T-FINAL (gate). T001 and T003 can be drafted in parallel once the `data-testid` contract is fixed in T001, but T003 assertions only pass after T002 lands.

**Key files to read first**:
- `ui/src/pages/Dashboard.tsx` (where the toggle goes)
- `ui/src/components/FeatureCard.tsx` (the reused card — do not duplicate)
- `ui/src/types/index.ts` (`PHASES`, `PHASE_LABELS`, `FeatureSummary`)
- `ui/e2e/app.spec.ts` (Playwright pattern to follow in `kanban.spec.ts`)
- `specs/kanban-view/acceptance.md` (AC-001..AC-019 — every AC maps to a `kanban.spec.ts` test)