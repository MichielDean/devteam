# Research: kanban-view

## Existing code patterns (ui/src/)

- **`pages/Dashboard.tsx`** — owns `useQuery(['features'], listFeatures)`, renders three branches: `features-loading` (spinner), `features-error` (red text), and the happy path which shows `EmptyState` when `features.length === 0` else `<FeatureList features={features} />`. The toggle + localStorage + conditional board/list slots into the happy-path branch. Loading/error/empty branches stay above the view switch so CON-009/CON-010 are preserved by construction.
- **`components/FeatureCard.tsx`** — a `<Link to={/features/:id}>` rendering title, id, status badge, phase badge, priority badge, `QuestionBadge` (when `pending_questions_count > 0`), and a `feature-card-gate` indicator (when `gate_result` present). Already satisfies FR-004, FR-005, FR-010, FR-011, FR-015. **Reuse as the Kanban card — no new `KanbanCard` component.** The phase badge is redundant inside a phase-titled column but harmless and keeps view-switching signal parity (US-003).
- **`components/FeatureList.tsx`** — sortable grid of `FeatureCard`. Stays unchanged (FR-006, SC-005).
- **`components/QuestionBadge.tsx`** — `data-testid="question-badge"`, count text. Reused via `FeatureCard`.
- **`components/EmptyState.tsx`** — `data-testid="empty-state"` + `empty-state-create-button`. Rendered above both views when `features.length === 0` (CON-010).
- **`types/index.ts`** — `PHASES` (readonly tuple), `PHASE_LABELS`, `STATUS_LABELS`, `PRIORITY_LABELS`, `FeatureSummary`, `GateResult`. Single source of truth for phase identifiers + labels (CON-004).
- **`api/client.ts`** — `listFeatures()` → `FeatureListResponse { features: FeatureSummary[], total_count }`. Already returns everything the board needs; no backend change (CON-007, FR-014).
- **`e2e/app.spec.ts`** — existing Playwright suite on `:18765`. Pattern: `page.on('console', ...)` to capture console errors; `page.route('**/api/features', ...)` to mock responses. New `kanban.spec.ts` follows the same pattern. Existing list-view assertions in `app.spec.ts` click `feature-card` selectors directly — they still work because list view remains the default (FR-008), so no modification to `app.spec.ts` is required.

## Library / framework choices

- **React 19 + react-router v7 + TanStack Query v5 + Tailwind v4** — already installed. Board is pure React + Tailwind. No new runtime dependency (CON-007).
- **Playwright** — already installed (`@playwright/test` devDep, config at `ui/playwright.config.ts`, port 18765). Used for ALL tests including the ones the PM's acceptance.md marked "unit / Vitest". See "Test runner decision" below.
- **localStorage** — browser-native, wrapped in try/catch (FR-009). No persistence library.

## Test runner decision (architect override of AC verification method)

acceptance.md AC-010, AC-011, AC-016, AC-017 specify "Vitest" as the verification tool and classify them as `unit`. The repo has **no Vitest (or any JS unit-test runner) installed** — only Playwright. Options:

1. **Add `vitest` + `@testing-library/react` + `jsdom` as devDependencies** (3 new devDeps). CON-007 restricts additions to `dependencies`, not `devDependencies`, so this is technically permitted.
2. **Use Playwright for everything** — already installed, zero new deps. "Unit" cases become Playwright specs with `page.route` mocks and `page.addInitScript` to throw on `localStorage.setItem`/`getItem`. The assertion is identical; only the runner differs.

**Decision: option 2 (Playwright for everything).** Rationale:
- Ponytail: the first lazy solution that works is the right one. Option 2 adds zero deps, zero config files, zero framework setup.
- CON-007 is honored maximally (no `package.json` change at all — stronger than the letter of the constraint).
- The "unit" test level is a logical classification (each test isolates one behavior); the runner is an implementation detail. The Tester phase can still report these as unit-level behavioral checks.
- `groupFeaturesByPhase` is a pure function — covered by a Playwright spec that mocks `/api/features` to return a feature with `current_phase = 'rolling_out'` and asserts the `kanban-column-other` element appears. Same assertion as AC-016, no framework needed.

If the Developer or Reviewer finds a real gap that only a JS DOM test can fill (e.g. asserting React state without a browser), escalate and revisit. Default: Playwright.

## Alternative approaches considered and rejected

- **Separate `KanbanCard` component** — rejected. `FeatureCard` already renders every field US-003 requires and is a `<Link>`. A second card component would duplicate badge/styling logic and drift from the list view. Reuse instead.
- **Separate `KanbanColumn` component** — rejected. A column is a header + a list of cards; inlining it inside `KanbanBoard.tsx` keeps the board to one file. Extract only if a column grows independent behavior (sorting, filtering — explicitly out of scope).
- **Separate `ViewToggle` component** — rejected. Two `<button>` elements with `aria-pressed`. Inlining in `Dashboard.tsx` is shorter than the props interface a separate component would require.
- **Separate `useViewPreference` hook** — rejected. ~8 lines of try/catch around `localStorage.getItem`/`setItem`. Inlining in `Dashboard.tsx` is shorter than a hook file.
- **`sessionStorage`** — rejected. Spec FR-007 mandates `localStorage` with key `devteam.dashboard.view` for cross-session persistence (US-002 AC-008 reloads and expects the board).
- **Drag-and-drop** — rejected. Spec assumption: read-only board.
- **Virtualisation / column-level `overflow-y`** — rejected. Spec assumption: page scroll, no virtualisation (SC-002 requires all 50 cards in the DOM).
- **Add `vitest` devDep** — rejected per "Test runner decision" above.

## Spikes / prototypes

None. The feature reuses existing primitives (`FeatureCard`, `PHASES`, `useQuery(['features'])`, Playwright route mocking). No unfamiliar technology — no spike needed.