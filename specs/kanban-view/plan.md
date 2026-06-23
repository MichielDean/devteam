# Implementation Plan: kanban-view

**Branch**: `feature/kanban-view` | **Date**: 2026-06-22 | **Spec**: `specs/kanban-view/spec.md`

**Input**: Feature specification from `specs/kanban-view/spec.md`

## Summary

Add a Kanban board view to the Dev Team Dashboard. The board renders features as cards grouped into **six** phase columns (Inception → Delivery), plus a defensive trailing "Other" column for unknown phases. A List/Board toggle switches between the existing `FeatureList` and the new `KanbanBoard`; **"List" is the default** (FR-003, preserves existing behavior), persisted in `sessionStorage` for the session. The board is view-only (no drag-and-drop), consumes the existing `useQuery(['features'])` data (no new fetch, no backend change), and reuses the existing loading/error/empty Dashboard branches. All layout via Tailwind utilities — no new runtime npm dependencies.

**Technical approach**: three new UI components (`KanbanBoard`, `KanbanCard`, `KanbanColumn`) + one shared `badgeColors` module + one `useSessionView` hook + a `ViewToggle` component, wired into `Dashboard.tsx`. A pure `groupFeaturesByPhase` function is extracted for unit testing. New e2e file `kanban.spec.ts` covers AC-001–AC-022; one additive fixture in `app.spec.ts` preserves the existing list-view tests (which already click "List" when Board-default drift is present — see Open Questions).

**Brownfield note**: a prior implementation pass exists in the worktree. It largely matches this plan but **diverges on two spec-mandated points**: (1) `useSessionView.ts` returns `'board'` default — spec FR-003/AC-005 require `'list'`; (2) `kanban.spec.ts` asserts Board default — spec AC-001/005 require List default. Construction must reconcile these to the central spec (Constitution I: spec is the contract). Six columns, vertical scroll, empty-column placeholders, terminal features visible, single fetch, no new deps — all already satisfied by the worktree code.

## Technical Context

**Language/Version**: TypeScript 5.8, React 19.1, Go 1.x (backend — unchanged)

**Primary Dependencies**: `react`, `react-dom`, `react-router` v7, `@tanstack/react-query` v5, `tailwindcss` v4. `vitest` (devDep, already present in worktree) for the unit test. **No new runtime dependencies** (CON-003).

**Storage**: `sessionStorage` (browser) for view preference. No server-side storage. No DB change.

**Testing**: Playwright e2e (`ui/e2e/*.spec.ts`, `:18765` — CON-001) + Vitest unit (`ui/src/**/*.test.ts`, `npm run test:unit`) for `groupFeaturesByPhase` (AC-011) + Go smoke (`internal/api/kanban_smoke_test.go`) pinning the consumed API contract.

**Target Platform**: Web browser (Chrome/Firefox/Safari). Playwright runs on `:18765`.

**Project Type**: Web app (Go backend + React frontend, single repo).

**Performance Goals**: First contentful paint of the Board within 200ms of the features query resolving (SC-006). Pure CSS + React render — no data fetching. Trivially met.

**Constraints**:
- No new runtime npm dependency (CON-003). `vitest` is devOnly.
- No backend change, no new endpoint, no new fetch (CON-007, FR-016).
- Reuse `PHASES`/`PHASE_LABELS`/`STATUS_LABELS`/`PRIORITY_LABELS` — no duplicated strings (CON-005).
- Card chrome parity with `FeatureCard` (CON-006) — shared `badgeColors.ts`, byte-identical gate-indicator text.
- Existing `app.spec.ts` list-view assertions must still pass (CON-004). The worktree already adds a `switchToList` helper — keep it.
- E2E on `:18765` only (CON-001).
- List is the default view (FR-003) — **diverges from worktree's current `'board'` default; construction must fix**.

**Scale/Scope**: Single repo, `ui/` directory only. ~6 new/modified files. Workspaces with 0–50+ features per phase (overflow handled, FR-013).

## Constitution Check

GATE: Passed. Verified against `.specify/memory/constitution.md` (worktree) — same principles govern the central repo.

| Principle | Status | Note |
|---|---|---|
| I. Spec-Driven, Always | ✅ | Plan derives from central spec.md + acceptance.md. Spec is the contract — worktree code that diverges (Board default) is wrong, not the spec. |
| II. Six Roles, Fixed Pipeline | ✅ | This is the Architect's planning output. Does not dictate code or tests beyond constraints. |
| III. Central Spec, Distributed Implementation | ✅ | Single spec in central `devteam` repo. `repos.yaml` declares primary repo only. |
| IV. Two Intake Paths, One Output Format | ✅ | Loose-idea intake; standard spec/acceptance/repos shape. |
| V. Proof-of-Work Gates | ✅ | ACs are Given/When/Then with test levels. Done conditions tie to specific ACs. |
| VI. Cross-Repo Coherence | ✅ | Single-repo feature. N/A. |
| VII. Self-Bootstrap | ✅ | Feature improves the platform's own UI. |
| VIII. Go, Minimal Dependencies | ✅ | No backend change. No new runtime npm dep. `vitest` is devOnly (justified by AC-011 unit-test requirement; already in worktree). |
| IX. Pipeline Governance | ✅ | Security/resiliency extensions N/A — view-only, no input, no auth, no external call, no mutation. Documented in spec. |
| X. Learn From Cistern | ✅ | Structured context; phase gate mechanically enforced. |

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

### Source Code (repository root — `ui/` only; worktree path shown)

```text
ui/
├── src/
│   ├── pages/
│   │   └── Dashboard.tsx           # MODIFY — wire toggle + conditional Board/List
│   ├── components/
│   │   ├── KanbanBoard.tsx         # CREATE — board container; re-exports groupFeaturesByPhase
│   │   ├── KanbanColumn.tsx        # CREATE — header + scrollable body + empty placeholder
│   │   ├── KanbanCard.tsx          # CREATE — vertical card; reuses badgeColors
│   │   ├── groupFeaturesByPhase.ts # CREATE — pure partition fn (AC-011 unit test target)
│   │   ├── groupFeaturesByPhase.test.ts # CREATE — vitest unit test (AC-011)
│   │   ├── ViewToggle.tsx          # CREATE — two-button toggle with aria-pressed
│   │   ├── badgeColors.ts          # CREATE — extracted shared statusColors map (CON-006)
│   │   ├── FeatureCard.tsx         # MODIFY — import statusColors from badgeColors.ts
│   │   └── QuestionBadge.tsx       # unchanged (reused by FeatureCard; KanbanCard uses local <span>)
│   ├── hooks/
│   │   └── useSessionView.ts       # CREATE — sessionStorage-backed view preference (default 'list')
│   └── types/
│       └── index.ts                # unchanged (reuses PHASES, PHASE_LABELS, etc.)
├── e2e/
│   ├── app.spec.ts                 # MODIFY — switchToList helper before list-view assertions (CON-004)
│   └── kanban.spec.ts              # CREATE — AC-001..AC-022
└── package.json                    # MODIFY — add vitest devDep + test:unit script (if not present)
```

**Structure Decision**: Single-project web app (existing layout). New components under `ui/src/components/` (CON-002). New hook under `ui/src/hooks/` (matches existing `useFeatures.ts` / `useSSE.ts` location). No new pages — the board lives on the existing Dashboard route (`/`). `groupFeaturesByPhase` extracted to its own file (not co-located in `KanbanBoard.tsx`) so the unit test imports a pure module with no React side effects — matches the worktree's existing layout.

## Component Design

### `ViewToggle`

- **Purpose**: Two-button segmented control switching between "List" and "Board".
- **Responsibilities**:
  - Render two `<button>` elements with `data-testid="view-toggle-list"` / `"view-toggle-board"`.
  - Container `data-testid="view-toggle"`.
  - Active button carries `aria-pressed="true"`; inactive `aria-pressed="false"` (AC-001/004/005).
  - Call `onViewChange(view)` on click.
- **Interfaces**: props `{ view: 'list' | 'board'; onViewChange: (v) => void }`.
- **Dependencies**: none (pure presentational).
- **Agent failure-mode check**: exactly one button has `aria-pressed="true"` at any time — never both, never neither.

### `useSessionView`

- **Purpose**: Session-scoped persistence of the view preference.
- **Responsibilities**:
  - Lazy-init from `sessionStorage.getItem('devteam.dashboard.view')` (FR-002). Validate against `'list' | 'board'`; invalid/absent → **`'list'`** (FR-003 — conservative, preserves existing behavior).
  - On change, `sessionStorage.setItem('devteam.dashboard.view', next)`.
  - try/catch around storage access → fall back to `'list'` on error (private-mode quota).
- **Interfaces**: `useSessionView(): ['list' | 'board', (v) => void]`.
- **Dependencies**: `sessionStorage` (browser native).
- **Agent failure-mode check**: lazy initializer must not throw if `sessionStorage` access raises — wrap in try/catch.
- **⚠️ Divergence flag**: the worktree's current `useSessionView.ts` returns `'board'` default. **Construction must change this to `'list'`** to satisfy FR-003/AC-005.

### `groupFeaturesByPhase` (pure function)

- **Purpose**: Partition `FeatureSummary[]` into six phase buckets + a defensive `other` bucket.
- **Spec**: `groupFeaturesByPhase(features: FeatureSummary[]): Record<PhaseName | 'other', FeatureSummary[]>`.
  - Initialize all 6 phase buckets + `other` to `[]` (CON-008 — no null arrays).
  - For each feature: if `PHASES.includes(feature.current_phase)`, push to that bucket; else push to `other` (FR-007, CON-009).
  - Partition invariant: `sum(buckets) === input.length` (SC-002, FR-006).
- **Interfaces**: exported from `groupFeaturesByPhase.ts`; re-exported by `KanbanBoard.tsx` for convenience.
- **Dependencies**: `PHASES`, `PhaseName` from `types`.
- **Agent failure-mode checks**:
  - [ ] Every bucket starts as `[]` — no `null`/`undefined`.
  - [ ] Unknown `current_phase` does not crash — routes to `other`.
  - [ ] Partition invariant holds — unit test asserts sum.

### `KanbanBoard`

- **Purpose**: Render six phase columns + optional "Other" column, each populated with `KanbanCard`s.
- **Responsibilities**:
  - Accept `features: FeatureSummary[]` prop.
  - Compute `groupFeaturesByPhase(features)`.
  - Render columns in `PHASES` order; append `'other'` column only when `groups.other.length > 0` (FR-007, AC-019).
  - Board container: `flex gap-4 overflow-x-auto` (FR-015); height bounded via `h-[calc(100vh-8rem)]` (FR-014).
  - No network calls — pure render from props (CON-007).
- **Interfaces**: props `{ features: FeatureSummary[] }`. Re-exports `groupFeaturesByPhase`.
- **Dependencies**: `KanbanColumn`, `PHASES`, `PHASE_LABELS` from `types`, `groupFeaturesByPhase`.
- **Agent failure-mode checks**:
  - [ ] `other` column conditional — not always rendered.
  - [ ] No `useQuery`/`fetch` in this file (CON-007).
  - [ ] Column order from `PHASES` constant, not hardcoded.

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
  - [ ] Header outside the `overflow-y-auto` element (stays fixed when body scrolls).

### `KanbanCard`

- **Purpose**: Vertical card for a single feature on the board.
- **Responsibilities**:
  - Root: `<Link to={/features/:id}>` with `data-testid="kanban-card-${feature.id}"` (FR-010, AC-010).
  - Title (line-clamped to 2 lines).
  - Badge trio: status (`kanban-card-status`), priority (`kanban-card-priority`), using `STATUS_LABELS` / `PRIORITY_LABELS` and the shared `statusColors` map (CON-005/CON-006).
  - Question badge: local `<span data-testid="question-badge">` (NOT `QuestionBadge` `<Link>` — avoids nested-anchor invalid HTML) when `pending_questions_count > 0` (FR-008, AC-008).
  - Gate indicator `kanban-card-gate` when `gate_result` present: `✓ Gate passed` / `✗ Gate failed` (FR-009, AC-009) — **byte-identical to `FeatureCard`** (CON-006).
  - Status-flag ring (FR-011):
    - `status === 'gate_blocked'` → `ring-2 ring-red-400` (AC-012).
    - `status === 'waiting_for_human'` → `ring-2 ring-yellow-400` (AC-013).
    - Otherwise no ring.
  - Updated date line (matches `FeatureCard`).
- **Interfaces**: props `{ feature: FeatureSummary }`.
- **Dependencies**: `Link` from `react-router`, `statusColors` from `badgeColors.ts`, `STATUS_LABELS`/`PRIORITY_LABELS` from `types`.
- **Agent failure-mode checks**:
  - [ ] Ring class only for the two attention statuses — no accidental ring on normal cards.
  - [ ] Gate indicator text byte-identical to `FeatureCard`.
  - [ ] No nested `<a>` — question badge is `<span>`.
  - [ ] `statusColors` imported from `badgeColors.ts`, not redefined.

### `badgeColors` (shared module)

- **Purpose**: Single source of truth for the status → Tailwind class map (CON-006).
- **Responsibilities**: export `statusColors: Record<string, string>` — the map currently inlined in `FeatureCard.tsx` (already extracted in the worktree).
- **Consumers**: `FeatureCard` (modify to import), `KanbanCard` (new).
- **Agent failure-mode check**: verify both consumers import from this module — no re-duplicated map. Grep `const statusColors` → exactly one match.

### `Dashboard` (modify)

- **Changes**:
  - Import `useSessionView`, `ViewToggle`, `KanbanBoard`.
  - `const [view, setView] = useSessionView();`
  - Render `ViewToggle` **only** when `!isLoading && !error && features.length > 0` (FR-004, AC-006).
  - In the `features.length > 0` branch: `view === 'board' ? <KanbanBoard features={features} /> : <FeatureList features={features} />`.
  - Loading / error / empty branches unchanged (FR-017, CON-008).
- **Agent failure-mode checks**:
  - [ ] Toggle hidden in empty state — verify e2e AC-006.
  - [ ] Single `useQuery(['features'])` call remains — no second fetch (CON-007, AC-016).
- **⚠️ Divergence flag**: worktree `Dashboard.tsx` comment says "Board default" — update comment to "List default per FR-003". The render logic (`view === 'board' ? ... : ...`) is already correct; only the hook's default changes.

## API Contracts

See `contracts/GET-api-features.md`. **No new endpoints.** The Board consumes the existing `GET /api/features` response via props from Dashboard. Contract documented read-only and pinned server-side by `internal/api/kanban_smoke_test.go` (asserts `features: []` never `null`, no `/api/kanban*` route exists).

## Constraint Verification Map

| CON-ID | Design Decision | Component(s) | Verification Checkpoint | Test Type |
|---|---|---|---|---|
| CON-001 | New e2e in `ui/e2e/kanban.spec.ts`; `app.spec.ts` switchToList helper. Playwright config unchanged (`:18765`). | kanban.spec.ts, app.spec.ts, playwright.config.ts | `npm run test:e2e` runs against `:18765` webServer; no test references `:8765` | E2E |
| CON-002 | New components in `ui/src/components/`; hook in `ui/src/hooks/`. No new pages. | KanbanBoard/Column/Card/ViewToggle/badgeColors/groupFeaturesByPhase, useSessionView | File-path review: all new files under `ui/src/components/` or `ui/src/hooks/` | Review |
| CON-003 | No new runtime npm dep. `vitest` devDep only (for AC-011 unit test). All layout via Tailwind. | package.json, KanbanBoard/Column/Card | `package.json` diff: `dependencies` block unchanged; `devDependencies` adds `vitest` only (if not already present) | Review |
| CON-004 | `app.spec.ts` list-view tests click `view-toggle-list` before asserting `feature-card-*`. Additive `switchToList` helper, no assertion removed. | app.spec.ts | `npm run test:e2e` green; existing feature-card / count-badge assertions pass after the click-to-List step | E2E (regression) |
| CON-005 | Board imports `PHASES`, `PHASE_LABELS`, `STATUS_LABELS`, `PRIORITY_LABELS` from `types/index.ts`. Column headers via `PHASE_LABELS`; card badges via `STATUS_LABELS`/`PRIORITY_LABELS`. No new string literals. | KanbanBoard, KanbanColumn, KanbanCard | Grep `Kanban*.tsx` for `'Inception'\|'Planning'\|...` / `'In Progress'\|...` → zero matches outside `types/` | Review + grep |
| CON-006 | `statusColors` in `badgeColors.ts`; `FeatureCard` and `KanbanCard` both import it. Gate indicator text byte-identical (`✓ Gate passed` / `✗ Gate failed`). `question-badge` testid reused. | badgeColors.ts, FeatureCard, KanbanCard | Code review: single `statusColors` map; gate text byte-identical; e2e AC-007/008/009 pass | Review + E2E |
| CON-007 | Board receives `features` as prop from Dashboard; Dashboard owns the single `useQuery(['features'])`. Board makes zero fetch calls. | Dashboard, KanbanBoard | E2e AC-016: `page.on('request')` count for `/api/features` === 1 during Board render | Integration |
| CON-008 | Loading (`features-loading`), error (`features-error`), empty (`EmptyState`) branches reused unchanged. Board renders only when `features.length > 0`. Empty columns render `[]` + "No features" placeholder. | Dashboard, KanbanColumn, groupFeaturesByPhase | E2e AC-006/014/015/017/018; `PhaseColumn.features` always `[]` never `null` (code review + unit test AC-011) | E2E + Unit + Review |
| CON-009 | `groupFeaturesByPhase` routes any `current_phase` not in `PHASES` to `other`. No throw, no drop. | groupFeaturesByPhase, KanbanBoard | Unit test AC-011: `groupFeaturesByPhase([{current_phase:'weird',...}])` → `{other:[feature]}`; partition sum invariant | Unit |

**Every constraint has a design decision and a verification checkpoint.** No constraint unaddressed.

## Cross-Component Consistency Matrix

| Shared Value | Producer | Consumer | Consistent? | Verification |
|---|---|---|---|---|
| Phase labels & order | `PHASES` / `PHASE_LABELS` (types) | `KanbanColumn` header, `KanbanBoard` column ordering | YES — single source | E2E AC-019 (6 columns in `PHASES` order); grep no duplicate literals (CON-005) |
| Status labels | `STATUS_LABELS` (types) | `KanbanCard` status badge | YES — single source | E2E AC-007 (badge text "In Progress") |
| Priority labels | `PRIORITY_LABELS` (types) | `KanbanCard` priority badge | YES — single source | E2E AC-007 (badge text "P1 - Critical") |
| Status → Tailwind class map | `badgeColors.ts` (shared module) | `FeatureCard`, `KanbanCard` | YES — both import the same export | Code review; visual parity e2e AC-007 (CON-006) |
| Gate indicator text | `FeatureCard` (`✓ Gate passed` / `✗ Gate failed`) | `KanbanCard` (must match) | YES — byte-identical strings | E2e AC-009; grep both files for the strings |
| `question-badge` testid | `QuestionBadge` (list, `<Link>`), local `<span>` (board) | E2E selectors | YES — same testid, different element type (span not Link, to avoid nested anchors) | E2e AC-008; HTML validity (no nested `<a>`) |
| Features array | Dashboard `useQuery(['features'])` | `FeatureList` (list), `KanbanBoard` (board) | YES — same prop source, no second fetch | Integration AC-016 (CON-007) |
| View preference | `useSessionView` (sessionStorage) | `Dashboard` render branch | YES — single state owner | E2e AC-004/005 (reload + fresh session) |
| Column count | `KanbanBoard` renders `PHASES.length` + conditional `other` | E2E AC-019 assertion (6, +1 only when unknown phase) | YES — driven by `PHASES` constant | E2E AC-019 |
| View default | `useSessionView` returns `'list'` (FR-003) | Dashboard initial render | **MUST BE `'list'`** — worktree currently returns `'board'` (divergence) | E2e AC-001/005 (List active by default); construction must fix `useSessionView.ts` |

**Multi-component note**: the only "N producers" case is the status-color map (2 consumers: `FeatureCard` + `KanbanCard`). Extracting to `badgeColors.ts` guarantees consistency. No provider/consumer divergence possible. The view-default row flags a spec-vs-worktree inconsistency the Developer must reconcile.

## Test Strategy

### Component: `ViewToggle`
- **Smoke**: renders two buttons, active one has `aria-pressed="true"`.
- **E2E**: AC-001 (toggle visible, **List** active by default), AC-002 (click Board → columns), AC-003 (click List → feature-list), AC-004 (reload persists), AC-005 (fresh session → List).
- **Unit**: not required (pure presentational, e2e covers it).

### Component: `useSessionView`
- **E2E**: AC-004 (sessionStorage persistence across reload), AC-005 (fresh session defaults **List**), US-3 scenario 3 (empty → non-empty resumes stored view).
- **Unit**: optional; behavior is trivial and e2e-covered.

### Component: `groupFeaturesByPhase`
- **Unit** (AC-011, mandatory): `groupFeaturesByPhase.test.ts` —
  - Empty input → 6 empty buckets + empty `other`; all `[]` not null (CON-008).
  - Known-phase feature → correct bucket.
  - Unknown-phase feature (`current_phase: 'weird'`) → `other` bucket, no crash (CON-009).
  - Partition invariant: `sum(buckets) === input.length` for mixed input (SC-002, FR-006).
- **E2E**: AC-019 (column count/order), AC-011 end-to-end via board (T019 in worktree kanban.spec.ts).

### Component: `KanbanBoard`
- **Smoke**: renders without crash given `[]` (six empty columns — but Dashboard renders `EmptyState` for `[]`, so this is a dev-only smoke) and given a populated array.
- **E2E**: AC-002 (columns render), AC-007 (card in correct column with badges), AC-016 (single fetch), AC-019 (6 columns + optional other), AC-022 (horizontal scroll).

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
| `groupFeaturesByPhase` (logic) | YES | — | YES | **YES** (AC-011) |
| `KanbanBoard` (UI) | YES | — | YES | — |
| `KanbanCard` (UI) | YES | — | YES | — |
| `KanbanColumn` (UI) | YES | — | YES | — |
| `ViewToggle` (UI) | YES | — | YES | — |
| `useSessionView` (hook) | YES | — | YES | — |
| `Dashboard` (wiring) | YES | YES (AC-016) | YES | — |
| `app.spec.ts` (regression) | YES | — | YES | — |

### Quality Checkpoints (per component)

- [ ] Board renders without console errors (smoke — SC-004)
- [ ] All e2e selectors use `data-testid`, never class names for state (CON convention) — exception: ring-class ACs (AC-012/013) explicitly assert class
- [ ] `PhaseColumn.features` is `[]` not `null` for empty columns (CON-008)
- [ ] No nested `<a>` inside `KanbanCard` (question badge is `<span>`)
- [ ] Gate indicator text byte-identical to `FeatureCard` (CON-006)
- [ ] `statusColors` imported from `badgeColors.ts` in both card components (CON-006)
- [ ] No new string literals for phase/status names in board files (CON-005)
- [ ] `package.json` `dependencies` block unchanged (CON-003)
- [ ] Single `GET /api/features` request during Board render (CON-007, AC-016)
- [ ] `app.spec.ts` list-view tests click "List" first (CON-004)
- [ ] **`useSessionView` returns `'list'` default — NOT `'board'`** (FR-003, AC-005)

## Agent Failure Mode Checks (per task)

| Task | Failure mode | Check |
|---|---|---|
| T-001 (badgeColors extract) | Re-duplicated map | Grep: only one `statusColors` definition; both cards import it |
| T-002 (useSessionView) | sessionStorage throws in private mode; **wrong default** | try/catch → default `'list'`; **verify default is `'list'` not `'board'`** (FR-003) |
| T-003 (groupFeaturesByPhase) | Null arrays; dropped features; crash on unknown phase | Unit test asserts `[]` init, partition sum, unknown-phase bucket |
| T-004 (KanbanCard) | Nested anchors; wrong ring class; gate text drift | HTML validator; ring class only for 2 statuses; grep gate text |
| T-005 (KanbanColumn) | Empty body blank (not placeholder); header scrolls with body | Placeholder testid; header outside `overflow-y-auto` |
| T-006 (KanbanBoard) | Second fetch; wrong column order; `other` column always present | No `useQuery` in board; columns in `PHASES` order; `other` conditional |
| T-007 (Dashboard wiring) | Toggle visible in empty state; loading/error branches broken; **wrong default** | Toggle gated by `features.length > 0`; existing branches untouched; **default view is List** |
| T-008 (ViewToggle) | `aria-pressed` wrong/missing; both buttons active | Assert exactly one `aria-pressed="true"` |
| T-009 (app.spec.ts fixture) | Existing assertions broken; skip-too-aggressive | All existing tests still run; only added a click step |
| T-010 (kanban.spec.ts) | Tests run on `:8765`; selectors use classes; **assert Board default (wrong)** | Config `:18765`; all selectors `data-testid`; **AC-001/005 assert List default** |

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
| Invalid stored view | AC-005 (implicit) | `useSessionView` validates value; invalid → `'list'` | Defaults to List on next load. |
| Storage access throws (private mode) | (defensive) | try/catch in `useSessionView` lazy init → `'list'` | No throw; in-memory state still updates on toggle. |

## Quality Checkpoints at Task Boundaries

1. **After T-001 (badgeColors)**: `FeatureCard` still renders identically — run existing `app.spec.ts` list-view tests (after clicking List). No visual drift.
2. **After T-003 (groupFeaturesByPhase)**: unit test passes (AC-011) before any UI wiring.
3. **After T-006 (KanbanBoard)**: renders standalone in a smoke test (dev server) with mock features — no console errors.
4. **After T-007 (Dashboard wiring)**: e2e AC-001/002/003/006 pass — toggle works, **List default**, empty state hides toggle.
5. **After T-009 (app.spec.ts)**: full existing suite green — no regression (CON-004).
6. **After T-010 (kanban.spec.ts)**: all AC-001..AC-022 covered (every acceptance criterion has a test).

## Quickstart Guide for the Developer

```bash
# From worktree root: ~/source/devteam/worktrees/kanban-view/devteam
cd ui

# 1. Ensure vitest devDep + test:unit script exist (worktree already has them)
#    package.json: "test:unit": "vitest run", devDependencies: "vitest": "^4.1.9"
#    If missing: npm install -D vitest

# 2. CRITICAL FIRST FIX — reconcile spec divergence:
#    ui/src/hooks/useSessionView.ts: change `return 'board'` → `return 'list'` (FR-003)
#    ui/e2e/kanban.spec.ts: AC-001/005 must assert view-toggle-list[aria-pressed="true"]
#      (currently asserts view-toggle-board — WRONG per spec)

# 3. Verify the rest of the worktree code matches the central spec:
#    - Six columns (no Backlog) — KanbanBoard.tsx already correct
#    - groupFeaturesByPhase routes unknown → other — already correct
#    - badgeColors.ts shared, KanbanCard uses <span> question badge — already correct
#    - kanban-column-empty-${phase} placeholder — already correct
#    - Vertical scroll, h-[calc(100vh-8rem)], w-60 — already correct

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
- **`useSessionView` returns `'list'` default — AC-001/005 assert List active on fresh load** (FR-003).

## Open Questions

1. **Worktree divergence on view default**: The worktree's `useSessionView.ts` and `kanban.spec.ts` assert Board default. The central spec's FR-003/AC-001/AC-005 mandate **List** default. Per Constitution I (spec is the contract), construction changes to List default. Documented as the single highest-priority construction fix. No human input needed — spec decided.
2. **`questions.json` from inception**: 8 PM questions exist at `specs/kanban-view/questions.json` (Backlog column, default view, toggle mechanism, persistence duration, drag-drop, terminal features, overflow, empty columns). The spec resolves all via `[ASSUMPTION:]` defaults aligned with the central spec's FRs. No architect-level questions remain — the spec decided every architectural choice. No `questions.json` written by the architect.