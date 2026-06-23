# Research: kanban-view

## Existing Code Patterns (Brownfield)

The repo is a Go backend (`internal/`, `cmd/`) + React 19 / TS / Vite / Tailwind v4 frontend (`ui/`). Worktree at `~/source/devteam/worktrees/kanban-view/devteam` on branch `feature/kanban-view` — a prior implementation pass exists there; this plan targets the **central spec** (`~/source/devteam/specs/kanban-view/spec.md`), which differs from the worktree's stale 7-column plan in two ways: **List** is the default (FR-003, not Board), and **six** phase columns (no Backlog). The worktree's `useSessionView.ts` returns `'board'` and `kanban.spec.ts` asserts Board default — both **diverge from this spec** and must be corrected in construction.

### Dashboard data flow (unchanged)
`ui/src/pages/Dashboard.tsx` owns one `useQuery(['features'])` → `listFeatures()` → `FeatureListResponse { features: FeatureSummary[], total_count }`. Existing branches: `isLoading` → `features-loading`; `error` → `features-error`; `features.length === 0` → `EmptyState`; else → `FeatureList`. The board slots into the final branch behind a toggle. No new fetch.

### Card chrome (`ui/src/components/FeatureCard.tsx`)
Renders `<Link to={/features/:id}>` with: title, id, status badge (via inline `statusColors` map), phase badge, priority badge, gate indicator (`✓ Gate passed` / `✗ Gate failed`), updated date. `QuestionBadge` (a `<Link>`) overlays a count bubble. The worktree already extracted `statusColors` to `ui/src/components/badgeColors.ts` and `KanbanCard` imports it — **CON-006 satisfied by extraction; the board card must reuse `badgeColors` and match the gate-indicator strings byte-for-byte**.

### Test conventions
- E2E: Playwright, `ui/e2e/*.spec.ts`, `:18765` only (CON-001). `data-testid` for all selectors. Console-error assertion pattern: `page.on('console')` + `page.on('pageerror')`.
- Unit: `vitest` (`ui/src/**/*.test.ts`, `npm run test:unit`). Worktree has `groupFeaturesByPhase.test.ts` — pure-function unit tests are the established pattern.
- Backend smoke: `internal/api/kanban_smoke_test.go` pins the `GET /api/features` contract the board depends on (`features: []` never `null`, no new kanban endpoint).

### Types (`ui/src/types/index.ts`)
`PHASES = ['inception','planning','construction','review','testing','delivery'] as const`, `PHASE_LABELS`, `STATUS_LABELS`, `PRIORITY_LABELS`, `FeatureSummary`. All board phase/status/priority strings MUST come from here (CON-005).

## Library / Framework Choices

| Choice | Decision | Rationale |
|---|---|---|
| Drag-and-drop | None (view-only) | Spec assumption: out of scope. No dnd-kit / react-dnd. CON-003. |
| Layout | Tailwind utilities only | `flex`, `overflow-x-auto`, `overflow-y-auto`, `w-60` (240px), `h-[calc(100vh-8rem)]`. No CSS-in-JS. |
| State persistence | `sessionStorage` (browser native) | FR-002. No localStorage — spec assumption. |
| Data fetching | Existing `@tanstack/react-query` `useQuery(['features'])` | FR-016, CON-007. Board receives `features` as prop. |
| Unit test runner | `vitest` (already a devDep in worktree) | Vite-native, zero-config. AC-011 requires a unit test. |
| Routing | Existing `react-router` v7 `<Link>` | FR-010. No new route. |

## Alternative Approaches Considered & Rejected

1. **Separate `/kanban` route** — rejected. Spec assumption: single Dashboard route, in-page toggle. A route would require nav-link plumbing and break the "toggle remembers choice" UX.
2. **Swimlanes by status** — rejected. Spec assumption: columns are phases, status is a badge. YAGNI — no UX evidence for swimlanes.
3. **"+N more" overflow per column** — rejected. Spec assumption: vertical scroll. One CSS line vs a component. Pending human Q-007 but default is scroll.
4. **Hide empty columns** — rejected. Spec assumption: render all six. Hiding obscures pipeline shape — the board's whole point.
5. **localStorage (cross-session)** — rejected. Spec assumption: sessionStorage only. No user-preference backend exists.
6. **Backlog column** — rejected for THIS spec. The central spec explicitly assumes six phase columns; a draft/unstarted feature appears in Inception. (The worktree's stale plan had seven with Backlog — superseded.)
7. **Filter out done/cancelled features** — rejected. Spec assumption: terminal features remain visible in their `current_phase` column for retrospective.
8. **New `GET /api/kanban` endpoint** — rejected. CON-007 / FR-016: no new fetch, no backend change. `GET /api/features` returns every field the board needs.

## Spikes / Prototypes

No new spikes required — the worktree already contains a working implementation of every component (`KanbanBoard`, `KanbanColumn`, `KanbanCard`, `ViewToggle`, `useSessionView`, `groupFeaturesByPhase` + unit test, `kanban.spec.ts`, `app.spec.ts` fixture, backend `kanban_smoke_test.go`). The spike proved the approach viable. **The construction task is to reconcile the existing code with the central spec's two divergences** (List default; six columns) and verify against the central spec's 22 ACs. No technology risk remains.

## Performance Characteristics

- Board render: pure CSS + React from already-resolved query data. SC-006 target 200ms FCP — trivially met (no fetch, no virtualization needed for ≤50 features/phase).
- `groupFeaturesByPhase`: O(n) single pass. Partition invariant `sum === input.length`.
- No re-render storms: board is pure functional, props from Dashboard. react-query cache is the single source.