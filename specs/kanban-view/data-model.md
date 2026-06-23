# Data Model: kanban-view

No backend changes. No new persisted entities. The board is a read-only projection of the existing `FeatureSummary` API response. The only new state is a browser-local UI preference.

## Entities

### FeatureSummary (existing, unchanged)

- **Source**: `GET /api/features` → `FeatureListResponse.features[]`; TypeScript type at `ui/src/types/index.ts`.
- **Attributes**:
  - `id: string` — required, unique. Used in `data-testid="feature-card-${id}"` and the `/features/:id` route.
  - `title: string` — required. Rendered as text (auto-escaped by React; never `dangerouslySetInnerHTML`).
  - `status: string` — required. One of `STATUS_LABELS` keys or a future value; unknown values fall through to the raw string (existing `FeatureCard` behaviour).
  - `priority: number` — required, `1 | 2 | 3` per existing contract (UI renders via `PRIORITY_LABELS` with `P${n}` fallback).
  - `current_phase: string` — required. Expected: one of `PHASES` (`inception|planning|construction|review|testing|delivery`). Unknown values are valid and route to the "Other" column (CON-011, FR-013).
  - `updated_at: string` — required, ISO timestamp. Rendered via `new Date(...).toLocaleDateString()` (existing `FeatureCard`).
  - `gate_result: GateResult | null` — nullable. When non-null, `FeatureCard` renders `feature-card-gate` with ✓/✗ based on `passed`.
  - `pending_questions_count: number` — required, `>= 0`. When `> 0`, `FeatureCard` renders `QuestionBadge` with the count.
- **Relationships**: none (flat list item).
- **Constraints**: none added. Backend contract unchanged.

### GateResult (existing, unchanged)

- **Attributes**: `phase: string`, `passed: boolean`, `checks: CheckResult[]`.
- Consumed by `FeatureCard` to render the gate indicator (FR-011, AC-013/014/015).

### DashboardView (new, UI-only, not sent to backend)

- **Type**: `'list' | 'kanban'`
- **Attributes**: single value held in `Dashboard.tsx` component state.
- **Persistence**: `localStorage['devteam.dashboard.view']` (FR-007).
- **Default**: `'list'` on first visit / missing key / read error (FR-008, FR-009).
- **Validation**: on read, accept `'kanban'` as kanban; any other value (including `'list'`, malformed, or absent) → `'list'`. Whitelist the known value rather than echoing arbitrary strings.
- **Relationships**: none.
- **State transitions**:
  - `list → kanban`: user clicks `view-toggle-kanban`; persist `'kanban'` (wrapped, FR-009).
  - `kanban → list`: user clicks `view-toggle-list`; persist `'list'`.
  - `* → list` (on mount): `localStorage` missing/invalid/throwing → default `'list'` (FR-008, FR-009, AC-009, AC-011).
  - Invalid transitions: none — toggle is a free two-state switch.

## Phase columns (derived, not persisted)

- **Source**: `PHASES` readonly tuple from `ui/src/types/index.ts` (`inception, planning, construction, review, testing, delivery`).
- **Labels**: `PHASE_LABELS[phase]` (CON-004, FR-015).
- **Ordering**: `PHASES` order is the column order (FR-002).
- **"Other" column**: appended after Delivery **iff** at least one feature has `current_phase` not in `PHASES` (FR-013, AC-016/017). Label: `"Other"`. `data-testid="kanban-column-other"`. Not in `PHASES`/`PHASE_LABELS` (it is a UI fallback, not a pipeline phase) — hardcoded literal in `KanbanBoard.tsx` with a `// ponytail:` comment noting the exception.

## Data integrity rules

- **No mutation**: the board never writes to feature state. All feature data flows from `useQuery(['features'])` (FR-014, AC-004).
- **Single source of truth**: phase labels come from `PHASE_LABELS` (CON-004). No literal `'Inception'`/`'Planning'`/etc. strings in new code.
- **Defensive reads**: `localStorage` access wrapped in try/catch (FR-009, AC-010, AC-011). Component never crashes on storage failure.
- **Unknown-phase safety**: `groupFeaturesByPhase` must not throw on an unrecognised `current_phase`; it routes the feature to "Other" (CON-011, AC-016).
- **Empty state**: when `features.length === 0`, `EmptyState` renders above both views; six empty columns still render in kanban view (FR-012, CON-010, AC-007).