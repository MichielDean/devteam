# Research: kanban-view

## Existing Code Patterns (verified by reading the repo)

### Dashboard data flow (`ui/src/pages/Dashboard.tsx`)
- Single `useQuery({ queryKey: ['features'], queryFn: listFeatures })` at the page root.
- `data?.features ?? []` and `data?.total_count ?? 0` — defensive defaults already in place (CON: missing `total_count` handled).
- Three explicit render branches: `isLoading` → `features-loading` testid; `error` → `features-error` testid; `features.length === 0` → `<EmptyState>`; else `<FeatureList>`.
- `FeatureList` is rendered only when `features.length > 0`. The board must slot into this same branch and reuse the same loading/error/empty branches unchanged (FR-017, AC-014, AC-015, AC-006/AC-018).
- `feature-count-badge` and `create-feature-button` live in the header above the view body — they stay regardless of view mode.

### FeatureCard chrome (`ui/src/components/FeatureCard.tsx`)
- Root is a `react-router` `<Link to={/features/${id}}>` with `data-testid="feature-card-${id}"`.
- Badge color map `statusColors` is a module-local `Record<string, string>` keyed by status string. CON-006 requires the board card to reuse the same badge classes — the simplest path is to **export `statusColors`** from `FeatureCard.tsx` (or extract to a shared `ui/src/components/badgeStyles.ts`) and import it in `KanbanCard`.
- Badges: `feature-card-status` (statusColors), `feature-card-phase` (purple), `feature-card-priority` (indigo, `PRIORITY_LABELS[priority]` → "P1 - Critical" etc.).
- `QuestionBadge` component (`ui/src/components/QuestionBadge.tsx`) renders an absolute-positioned yellow circle with `data-testid="question-badge"`. Reusable as-is on the board card.
- Gate indicator (`feature-card-gate`): `✓ Gate passed` / `✗ Gate failed` text. Reuse the exact text on the board card (AC-009 asserts this string).
- Updated date: `feature-card-updated`. Spec ASSUMPTION says last-updated is shown on the board card too — reuse the same formatting.

### Types (`ui/src/types/index.ts`)
- `PHASES = ['inception','planning','construction','review','testing','delivery'] as const` — the column source (FR-005, CON-005).
- `PHASE_LABELS`, `STATUS_LABELS`, `PRIORITY_LABELS` — label maps to reuse, no new string literals (CON-005).
- `FeatureSummary` has every field the board needs: `id`, `title`, `status`, `priority`, `current_phase`, `updated_at`, `gate_result`, `pending_questions_count`. No new fields, no DTO change.

### E2E conventions (`ui/e2e/app.spec.ts`)
- `data-testid` is the selector convention — every new element gets one.
- Console-error assertion pattern: `page.on('console', msg => { if (msg.type()==='error') consoleErrors.push(msg.text()) })` then `expect(consoleErrors).toEqual([])`. New board tests must extend this.
- `page.route('**/api/features', ...)` is the existing pattern for mocking the API in tests — reuse for AC-006 (empty), AC-014 (loading delay), AC-015 (500), AC-016 (request count).
- Existing tests assert `feature-card-*` testids. The board introduces `kanban-card-*` testids. CON-004 (no regression) is satisfied by (a) keeping `FeatureList` and its `feature-card-*` cards intact, (b) defaulting to Board but providing a one-click toggle back to List. **Risk**: existing `app.spec.ts` tests like "feature list loads and shows features" assert `[data-testid*="feature-card"]` count >= 1 on `/` — with Board as the default, those tests will now fail because `feature-card-*` only renders in List view. **Resolution**: the developer task must update `app.spec.ts` to click the List toggle (or assert against `kanban-card-*` where appropriate) before asserting `feature-card-*`. This is a known, bounded regression-fix task, not a spec change. Called out in tasks.md.

### Playwright config
- `webServer` runs on `:18765` (CON-001). New `kanban.spec.ts` runs under the same config — no new config needed.

## Library / Framework Choices

| Need | Choice | Rationale |
|---|---|---|
| Layout | Tailwind utility classes only | CON-003: no new runtime dep. flex/grid + `overflow-y-auto` + `min-w-[240px]` cover FR-013/FR-014/FR-015. |
| State (view toggle) | `sessionStorage` + a small `useSessionView` hook | FR-002. No store lib. `useSyncExternalStore` is overkill — a `useState` initialized from `sessionStorage.getItem('devteam.dashboard.view')` with a setter that writes back is 10 lines. |
| Data | existing `useQuery(['features'])` | FR-016, CON-007, AC-016. No second fetch. |
| Routing | existing `react-router` `<Link>` | FR-010. Same destination as FeatureCard. |
| Grouping logic | pure function `groupFeaturesByPhase(features)` | FR-006/FR-007. Returns `Record<PhaseName \| 'other', FeatureSummary[]>`. Unit-testable (AC-011, CON-009). |

## Alternative Approaches Considered and Rejected

1. **Separate `/kanban` route instead of a toggle.** Rejected: spec ASSUMPTION explicitly chose same-route toggle; a route split doubles the navigation surface and breaks the "one click" SC-001.
2. **URL query param `?view=board` for persistence.** Rejected: spec FR-002 mandates `sessionStorage` key `devteam.dashboard.view`. URL params are shareable but the spec closed this.
3. **Shared `KanbanCard` *is* `FeatureCard` with a `variant` prop.** Rejected after review: `FeatureCard` is a `<Link>` wrapping a flex layout with absolute `QuestionBadge`; the board card needs the same chrome but a different container (column body, narrower width, ring-flag for blocked/waiting). Forcing one component into both shapes risks breaking CON-004. Instead: **extract the shared badge color map + gate indicator text into a tiny shared module** and have `KanbanCard` render its own layout reusing those primitives. Keeps CON-006 (chrome parity) without coupling the two components' layout.
4. **`+N more` overflow truncation.** Rejected: spec ASSUMPTION — scroll is one CSS line, `+N` is a component. YAGNI.
5. **`localStorage` for cross-session persistence.** Rejected: spec ASSUMPTION — sessionStorage is sufficient; no user-preference backend exists.
6. **Drag-and-drop (dnd-kit).** Rejected: spec ASSUMPTION — view-only board. Would require new backend endpoints + gate-aware drop rules. Out of scope.

## Spikes / Prototypes

No code spikes needed — every primitive (Tailwind column scroll, `sessionStorage` hook, `useQuery` reuse, `<Link>` card) is already proven in this repo. The grouping function is the only non-trivial logic and is a 15-line pure function with a unit test.