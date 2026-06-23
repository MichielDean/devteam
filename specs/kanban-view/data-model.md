# Data Model: kanban-view

This feature adds **no new persisted entities** and **no backend changes**. The data model is the existing `FeatureSummary` plus one UI-only derived shape. Documented here for completeness and to give the Developer a single source of truth for field names and types.

## Entities

### FeatureSummary (existing, unchanged — consumer only)

Source: `GET /api/features` response → `ui/src/types/index.ts`. The board reads this; it does not produce or mutate it.

- **Attributes**:
  - `id`: `string`, required, non-empty. Primary key. Used in `data-testid="kanban-card-${id}"` and in the `<Link to="/features/${id}">` destination (FR-010).
  - `title`: `string`, required. Rendered as the card heading.
  - `status`: `string`, required, one of `draft | in_progress | gate_blocked | passed | failed | done | recirculated | cancelled | waiting_for_human` (keys of `STATUS_LABELS`). Drives the status badge (FR-008) and the ring-flag for `gate_blocked`/`waiting_for_human` (FR-011).
  - `priority`: `number`, required, `1 | 2 | 3`. Drives the priority badge via `PRIORITY_LABELS[priority]` (FR-008).
  - `current_phase`: `string`, required. Expected to be one of `PHASES`; if not, the feature is bucketed under `other` (FR-007, CON-009). This is the **column key**.
  - `updated_at`: `string` (ISO 8601 timestamp), required. Rendered as "Updated {date}" on the card.
  - `gate_result`: `GateResult | null`, optional. When non-null for the feature's current phase, the card shows `✓ Gate passed` / `✗ Gate failed` (FR-009, AC-009).
  - `pending_questions_count`: `number`, required, default `0`. When `> 0`, the `QuestionBadge` renders on the card (FR-008, AC-008).
- **Relationships**: none at the UI layer. Each feature is an independent row from the API.
- **Constraints**: no validation performed by the board — it renders whatever the API returns. Defensive handling: missing `total_count` falls back to `features.length` (already in Dashboard.tsx). Unknown `current_phase` → `other` bucket (FR-007).

### PhaseColumn (UI-only, not persisted, not sent over the wire)

Derived per-render from `PHASES` + the `features` array. Lifecycle: ephemeral — recreated on every render from the `useQuery(['features'])` result. Never stored in state, never serialized.

- **Attributes**:
  - `phase`: `PhaseName | 'other'` — the column key. One of `inception | planning | construction | review | testing | delivery | other`.
  - `label`: `string` — `PHASE_LABELS[phase]` for the six known phases; the literal `"Other"` for the `other` column (FR-007).
  - `features`: `FeatureSummary[]` — features whose `current_phase === phase`, preserving API order. Empty array for columns with no features (FR-012).
- **Relationships**: contains 0..N `FeatureSummary` items.
- **Constraints**:
  - Exactly six known columns always render in `PHASES` order (FR-005, AC-019).
  - The `other` column renders **only** when at least one feature has an unknown `current_phase` (AC-019: "plus optional Other only when an unknown phase exists").
  - A feature appears in **exactly one** column (FR-006, SC-002). No feature is dropped, no feature is duplicated.

### ViewPreference (UI-only, session-scoped)

Persisted in `sessionStorage` under key `devteam.dashboard.view`. Not an entity in the API; not sent to the backend.

- **Attributes**:
  - `value`: `'board' | 'list'`, required.
- **Default**: `'board'` (FR-003, per human input Q-001/009/017).
- **Lifecycle**: written on toggle click; read on Dashboard mount; cleared at session end (browser closes the tab).
- **Constraints**: only persisted when features exist (toggle is hidden in empty state — FR-004 — so there is nothing to persist).

## Relationships

```
FeatureListResponse (GET /api/features)
  └── features: FeatureSummary[]
        └── grouped by current_phase ──► PhaseColumn[]
                                          └── features: FeatureSummary[]  (same refs, no copies of data)

ViewPreference (sessionStorage)  ──► selects which view renders: KanbanBoard | FeatureList
```

## State Transitions

### ViewPreference
- `unset` → `board`: initial Dashboard load with no prior choice (FR-003 default).
- `board` → `list`: user clicks List toggle (FR-001).
- `list` → `board`: user clicks Board toggle (FR-001).
- `*` → `unset`: browser tab closed (sessionStorage cleared by the browser).
- **Invalid transitions**: none. The toggle is a two-state switch. When `features.length === 0`, the toggle is hidden (FR-004) so no transition is possible — the board simply does not render and `EmptyState` shows instead.

### FeatureSummary (from the board's perspective)
The board **never mutates** `FeatureSummary`. It is a read-only projection of API state. Phase transitions happen through the existing pipeline (`/advance`, `/recirculate`) on the detail page, not on the board. When the `useQuery(['features'])` cache invalidates (e.g. after a create mutation or SSE phase_change event), react-query re-renders and the board re-derives columns from the fresh data.

## Data Integrity Rules

1. **No feature dropped**: `sum(column.features.length for column in columns) === features.length` — invariant, unit-tested in `groupFeaturesByPhase` (SC-002).
2. **No feature duplicated**: each feature appears in exactly one column — guaranteed by the grouping function's single-bucket assignment.
3. **Column order is stable**: known columns in `PHASES` order; `other` column appended last (FR-005).
4. **Empty columns still render**: a column with `features: []` is still present in the output array (FR-012, AC-017, AC-019). The grouping function must not filter out empty buckets.
5. **No new API fields**: the board must not require any field not already in `FeatureSummary`. If the API omits `gate_result`, the card omits the gate indicator. If `pending_questions_count` is missing, treat as `0` (defensive — matches existing Dashboard `?? 0` pattern).

## Validation Rules (boundary — none new)

This feature has **no input boundary**. The board renders API output; it does not accept user input. No new validation rules. The existing `GET /api/features` contract (see `contracts/GET-api-features.md`) is unchanged.