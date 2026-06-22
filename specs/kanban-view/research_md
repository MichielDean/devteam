# Research: kanban-view

## Existing Code Patterns (ui/)

### Dashboard data flow
`ui/src/pages/Dashboard.tsx` is the single consumer of the features query:
- `const { data, isLoading, error } = useQuery({ queryKey: ['features'], queryFn: listFeatures })`
- `features = data?.features ?? []` — defensive coalesce to empty array (handles missing `features` array, but react-query `data` shape is typed `FeatureListResponse` which requires `features`)
- `totalCount = data?.total_count ?? 0` — defensive count fallback (covered by e2e `feature count badge handles missing total_count`)
- Branching: `isLoading` → `features-loading`; `error` → `features-error`; `features.length === 0` → `EmptyState`; `features.length > 0` → `FeatureList`

The Board is additive: it slots into the `features.length > 0` branch alongside `FeatureList`, gated by the toggle. The loading/error/empty branches stay untouched — they satisfy FR-017 without any change.

### FeatureCard chrome (CON-006 parity target)
`ui/src/components/FeatureCard.tsx` owns:
- `statusColors: Record<string, string>` map (Tailwind classes keyed by status). 9 entries: `in_progress`, `done`, `cancelled`, `draft`, `gate_blocked`, `passed`, `failed`, `recirculated`, `waiting_for_human`. All with `dark:` variants.
- Badge trio: status, phase, priority. Each a `<span>` with `data-testid="feature-card-{kind}"`.
- `QuestionBadge` rendered when `pending_questions_count > 0`, absolutely positioned top-right.
- Gate indicator: `feature-card-gate` div, `✓ Gate passed` / `✗ Gate failed`.
- Updated date line.

`KanbanCard` must reuse the **same badge color map** and the **same gate indicator text**. Decision: extract `statusColors` to `ui/src/components/badgeColors.ts` (shared module), imported by both `FeatureCard` and `KanbanCard`. Avoids duplication (CON-005/CON-006) and keeps a single source of truth. No new type, no new string literals — just relocating an existing map.

### FeatureList conventions
- Root `data-testid="feature-list"`
- Sort toolbar above the grid
- Grid: `grid gap-4 sm:grid-cols-2 lg:grid-cols-3`
- Each card keyed by `feature.id`

The Board replaces the grid region, not the sort toolbar — the Board does not need sort controls (columns are already phase-grouped). The toggle lives **above** both views, between the header row and the view body.

### types/index.ts
`PHASES`, `PHASE_LABELS`, `STATUS_LABELS`, `PRIORITY_LABELS` already exist and are the canonical string sources. The Board imports them — no new phase/status strings anywhere (CON-005). `PhaseName` is `typeof PHASES[number]` — typed, closed enum.

### E2E conventions (CON-001)
- `ui/playwright.config.ts`: `webServer` on `:18765`, `reuseExistingServer: true` unless `START_SERVER=1`. Tests run against the real Go binary serving the built UI.
- `app.spec.ts` uses `data-testid` selectors exclusively, captures `consoleErrors`, skips tests gracefully when no features exist (`test.skip`).
- Existing tests assert `feature-card-*` testids on first load. **CON-004 conflict**: with Board now default (FR-003), `feature-card-*` is not rendered until the user clicks "List". Resolution: the kanban test file owns Board assertions; `app.spec.ts` list-view tests must click `view-toggle-list` before asserting `feature-card-*`. This is the only modification to an existing test file, and it's an additive fixture, not a regression.

### API contract
`GET /api/features` → `FeatureListResponse { features: FeatureSummary[], total_count: number }`. `FeatureSummary` already carries every field the Board needs: `id`, `title`, `status`, `priority`, `current_phase`, `updated_at`, `gate_result` (nullable), `pending_questions_count`. **No backend change.** The Board does not call this endpoint — it receives the already-fetched `features` array as a prop from Dashboard (CON-007: single fetch).

## Library / Framework Choices

| Choice | Decision | Rationale |
|---|---|---|
| DnD library | None | Spec: drag-and-drop out of scope. View-only board. (CON-003) |
| CSS approach | Tailwind utilities only | Matches repo convention. No CSS-in-JS. (CON-003) |
| State persistence | `sessionStorage` direct | `sessionStorage` is a native browser API — no library. Key: `devteam.dashboard.view`. Values: `'board' | 'list'`. (FR-002) |
| View state hook | Custom `useSessionView` hook | Thin wrapper: lazy-init from `sessionStorage.getItem`, sync via `sessionStorage.setItem` on change. No external state lib. |
| Column layout | CSS flexbox / grid via Tailwind | `flex gap-4 overflow-x-auto` on board; each column `w-60` (240px, FR-015) with `flex flex-col`; column body `flex-1 overflow-y-auto`. Board height bounded via `h-[calc(100vh-Xpx)]`. (FR-013/014/015) |
| Card component | New `KanbanCard.tsx` | Not a reuse of `FeatureCard` — different layout (vertical, no id truncation, status-flag ring). Shares `statusColors` map and `QuestionBadge` for chrome parity (CON-006). |
| Grouping logic | Pure function `groupFeaturesByPhase` | Exported from `KanbanBoard.tsx` (or a sibling `grouping.ts`) for unit testing (AC-011). Returns `Record<PhaseName | 'other', FeatureSummary[]>`. |

## Alternative Approaches Considered & Rejected

1. **Reuse `FeatureCard` directly inside columns.** Rejected: the list card is a horizontal grid tile; a kanban card is a vertical stack. Forcing one component to serve both creates conditional layout branches and conflicting testid schemes (`feature-card-*` vs `kanban-card-*`). The spec mandates `kanban-card-*` testids (AC-007/010/012/013). Separate component, shared chrome primitives.

2. **Separate `/kanban` route.** Rejected by spec assumption: single `/` Dashboard route, toggle switches views. A second route would require router changes, a nav link, and dual-empty-state handling — more surface for no gain. (FR-001)

3. **`localStorage` for cross-session persistence.** Rejected by spec: no user-preference backend, session scope is sufficient. (FR-002, Assumptions)

4. **"+N more" overflow per column.** Rejected by spec: scroll is one CSS line, +N is a component. (FR-013, Assumptions)

5. **Hide empty columns.** Rejected by spec: the point of the board is to show all six phases. Empty columns get a muted placeholder. (FR-012, Assumptions)

6. **Swimlanes per status.** Rejected by spec: YAGNI, no UX evidence. Status is a badge on the card. (Assumptions)

7. **Global state store (zustand/jotai) for view preference.** Rejected: one boolean in `sessionStorage` does not justify a store. `useState` + lazy init is 3 lines. (CON-003)

## Spikes / Prototypes

No code spike required. The feature is pure UI composition over an existing data source. The riskiest piece — column scroll geometry (FR-013/014) — is a known CSS pattern (`flex-col` + `overflow-y-auto` on the body, fixed header). Verified against Tailwind v4 utility availability: `h-[calc(100vh-8rem)]`, `overflow-y-auto`, `overflow-x-auto`, `w-60` (240px), `ring-2 ring-yellow-400` / `ring-red-400` for status flags (FR-011). All standard Tailwind utilities present in v4.