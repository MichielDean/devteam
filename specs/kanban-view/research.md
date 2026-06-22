# Research: kanban-view

## Existing Code Patterns (the source of truth for conventions)

### Component shape
- All UI components live in `ui/src/components/`. Pages in `ui/src/pages/`.
- Each component is a single default-exported function component. Props interface declared above. No class components. No HOCs.
- Tailwind utility classes inline; dark mode via `dark:` variants (see `FeatureCard.tsx`, `Dashboard.tsx`).
- Interactive/observable elements carry `data-testid` in `kebab-case` — `feature-card-${id}`, `feature-card-title`, `features-loading`, `features-error`, `dashboard-page`, `feature-count-badge`.

### Dashboard data flow (`ui/src/pages/Dashboard.tsx`)
- `useQuery({ queryKey: ['features'], queryFn: listFeatures })` returns `{ data, isLoading, error }`.
- `data?.features ?? []` → `FeatureSummary[]`. `data?.total_count ?? 0` → count.
- Existing render branches: `isLoading` → `features-loading` spinner; `error` → `features-error`; empty → `EmptyState`; else → `<FeatureList features={features} />`.
- **Implication**: the view toggle must slot in *only* the non-empty success branch. Loading/error/empty paths are already handled and must remain unchanged for Kanban view (FR-013, FR-014, FR-015, AC-009, AC-010, AC-019).

### `FeatureCard` (`ui/src/components/FeatureCard.tsx`)
- Root is a `<Link to={/features/${id}}>` — click navigates. Reuse verbatim inside columns → navigation parity guaranteed (CON-003, FR-009, SC-004, AC-012, AC-013).
- Already truncates title, shows status/phase/priority badges, gate indicator, updated date, question badge. No card markup duplication.

### `FeatureList` (`ui/src/components/FeatureList.tsx`)
- Sorts a *copy* (`[...features].sort(...)`) — never mutates props. The Kanban column sort must follow the same immutable pattern.
- Sort comparator lives inline in the component. For Kanban we extract `orderCards` to a pure helper so AC-018 (unit test) can target it without a DOM.

### Types (`ui/src/types/index.ts`)
- `PHASES = ['inception','planning','construction','review','testing','delivery'] as const` — column set and order source (CON-004, FR-002, AC-CON-004).
- `PHASE_LABELS: Record<PhaseName, string>` — header text source (CON-005, FR-006, AC-CON-005).
- `FeatureSummary.priority` is `number` (1/2/3). `updated_at` is ISO string. `current_phase` is `string` (not typed to `PhaseName`) — unknown phases are possible at runtime, so FR-017 needs an "Other" bucket.

### Routing (`ui/src/App.tsx`)
- `react-router` v7. Routes: `/` → Dashboard, `/features/:id` → FeatureDetail. No nested routes; toggle state stays local to Dashboard, not in the router (matches assumption "back button not trapped").

### E2E (`ui/e2e/app.spec.ts`, `ui/playwright.config.ts`)
- Playwright on port **18765** (CON-002, AC-CON-002). `webServer` config auto-starts `~/go/bin/devteam -http :18765` from repo root.
- Existing pattern: `page.on('console', ...)` to capture console errors and assert empty. Kanban specs follow the same pattern.
- Existing pattern for API interception: `page.route('**/api/features', ...)` to inject loading/empty/error states (used by `feature count badge handles missing total_count defensively` and `absent on API error`). Kanban loading/error ACs (AC-009, AC-010) reuse this pattern.

### Build/lint/test
- `cd ui && npm run build` (`tsc -b && vite build`) — CON-001, AC-CON-001.
- `cd ui && npm run lint` (eslint) — CON-001.
- `cd ui && npx playwright test --reporter=line` — CON-002, AC-CON-002.
- No Vitest currently installed. AC-018 (unit test for `orderCards`) requires either adding Vitest or testing the helper via a Playwright `page.evaluate` that imports the compiled module. **Decision: add Vitest** — it's a devDependency (CON-008 only restricts *runtime* deps), it's the React ecosystem default, and the spec explicitly calls out "Vitest (or existing UI unit test runner)" in AC-018. One new devDependency, zero new runtime deps. See "Alternatives" below.

## Library / Framework Choices

| Need | Choice | Rationale |
|---|---|---|
| View state persistence | `localStorage` (browser native) | Spec assumption + question answer: "Persist in localStorage". No new dep. |
| Segmented toggle UI | Native `<button>` + Tailwind + `aria-pressed` | No toggle library. Matches existing button style in `FeatureList` sort controls. Accessible by default. |
| Column layout | CSS flexbox via Tailwind `flex` + `overflow-x-auto` | Native. Handles FR-011 (horizontal scroll) without a carousel library. Sticky headers via `sticky top-0`. |
| Card | Reuse `FeatureCard` | CON-003. Zero new code for card body. |
| Sort helper | Plain TypeScript function `orderCards(features: FeatureSummary[]): FeatureSummary[]` | Pure, unit-testable (AC-018). No comparator library. |
| Unit tests | Vitest (devDependency) | See build/lint/test above. React ecosystem default. Only way to satisfy AC-018 without a DOM. |

## Alternatives Considered

1. **URL query string for view persistence** (question option B). Rejected — spec assumption + chosen answer say localStorage. URL query would also require router changes and back-button handling. localStorage is simpler and matches the "no router navigation" assumption.
2. **Drag-and-drop between columns** (question option B/C). Rejected — chosen answer is display-only. Would require a DnD library (new runtime dep, violates CON-008 spirit) and a backend phase-change endpoint. Explicitly out of scope per spec assumption.
3. **Hide empty columns** (question option B). Rejected — chosen answer is "always show all six with empty-state message". Hiding would also break FR-010 and AC-011.
4. **Mobile vertical stack** (question option C). Rejected — chosen answer is horizontal scroll everywhere. Simpler, one layout, matches assumption.
5. **Separate `KanbanColumn.tsx` component vs inline columns in `KanbanBoard.tsx`**. Chose separate component — column has its own header/count/empty-state/sort concerns, and the `Other` fallback column (FR-017) is cleaner as a column instance than a special branch. Two small files, not one big one.
6. **Unit test runner: Vitest vs Playwright `page.evaluate`**. Vitest chosen. `page.evaluate` would require shipping the helper as a window global or fetching the built bundle, both hacky. Vitest is one devDependency, runs in Node, and is the standard. CON-008 restricts *runtime* deps; devDependencies for testing are explicitly allowed ("Tester names specific test files" — testing infra is expected).
7. **`useReducer` for view state vs `useState`**. `useState` — two values (list/kanban), no complex transitions. Reducer would be over-engineering (ponytail: no unrequested abstractions).
8. **Custom hook `useDashboardView` vs inline state in Dashboard**. Inline. One state + one effect for localStorage is 6 lines; a hook file for that is scaffolding (ponytail).

## Performance Characteristics

- `listFeatures` already returns all data. Kanban adds an O(n) group-by-phase + O(n log n) sort per column, both bounded by feature count (single repo, tens-to-hundreds of features). No memoization needed for MVP — `[...features].sort()` per render is the existing `FeatureList` pattern and is fine at this scale. `useMemo` added only if a measurable re-render problem appears (YAGNI).
- No new network requests. No SSE subscription. No polling. The board reuses the existing `useQuery(['features'])` cache — toggling views does not refetch.
- Horizontal scroll is GPU-composited (`overflow-x-auto`); no JS scroll logic. Sticky headers are native CSS.

## Spikes / Prototypes

None needed. Every primitive (Link, flexbox, localStorage, FeatureCard, useQuery) is already in use in the codebase. The feature is composition, not invention.