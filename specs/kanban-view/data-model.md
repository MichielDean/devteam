# Data Model: kanban-view

## Scope

This feature introduces **no new persisted entities** and **no backend changes**. The board is a derived, ephemeral view over the existing `FeatureSummary` data returned by `GET /api/features`. All entities below are UI-only, recomputed on every render from the react-query cache.

## Entities

### FeatureSummary (existing, unchanged)

The source entity. Already defined in `ui/src/types/index.ts`. Consumed read-only by the Board.

- **Attributes**:
  - `id`: `string`, required, unique. PK.
  - `title`: `string`, required.
  - `status`: `string`, required. One of the keys in `STATUS_LABELS` (`draft`, `in_progress`, `gate_blocked`, `passed`, `failed`, `done`, `recirculated`, `cancelled`, `waiting_for_human`). Defensive: unknown values fall back to the raw string for the badge label; the status-flag ring only matches the two known attention statuses (`gate_blocked`, `waiting_for_human`).
  - `priority`: `number`, required. `1 | 2 | 3`. Indexed via `PRIORITY_LABELS`.
  - `current_phase`: `string`, required. Expected to be one of `PHASES`. Defensive: unknown values route to the "Other" column (FR-007, CON-009).
  - `updated_at`: `string` (ISO 8601), required.
  - `gate_result`: `GateResult | null`, optional. When present, card shows `✓ Gate passed` / `✗ Gate failed` (FR-009).
  - `pending_questions_count`: `number`, required, default `0`. Badge shown when `> 0` (FR-008).
- **Relationships**: none directly; each `FeatureSummary` is a projection of a `FeatureDetail` (not loaded by the Board).
- **Constraints**: none added. All constraints are existing backend invariants.
- **State transitions**: none. The Board does not mutate features.

### PhaseColumn (UI-only, not persisted)

A derived view entity representing one kanban column. Built by `groupFeaturesByPhase(features)`.

- **Attributes**:
  - `phase`: `PhaseName | 'other'`, required. One of the six `PHASES` values, or `'other'` for the defensive trailing column.
  - `label`: `string`, required. Derived from `PHASE_LABELS[phase]` for known phases; `'Other'` for the trailing column.
  - `features`: `FeatureSummary[]`, required, default `[]`. The features whose `current_phase === phase` (or not in `PHASES` for `'other'`). **MUST be `[]` not `null`** when empty (CON-008 empty-state, agent failure-mode: null-array bug).
- **Relationships**:
  - `1 : N` with `FeatureSummary` (one column holds many features).
  - `N : 1` derived from the `features` array passed into the Board.
- **Constraints**:
  - Exactly six `PhaseColumn` instances for the known phases, always rendered in `PHASES` order (FR-005).
  - A seventh `'other'` column renders **only** when at least one feature has an unknown `current_phase` (FR-007, AC-019).
  - A feature appears in **exactly one** column (FR-006). No duplicates, no omissions — `groupFeaturesByPhase` must be a pure partition: `sum(column.features.length) === input.length`.

### ViewPreference (UI-only, session-scoped)

The user's toggle choice. Persisted in `sessionStorage`.

- **Attributes**:
  - `view`: `'board' | 'list'`, required, default `'board'` (FR-003).
- **Storage**: `sessionStorage` key `devteam.dashboard.view` (FR-002). Serialized as the bare string value (`'board'` or `'list'`). Cleared automatically when the browser session ends.
- **Lifecycle**:
  - **Read**: on Dashboard mount, lazy-init `useState` from `sessionStorage.getItem('devteam.dashboard.view')`. If absent or invalid, default to `'board'` (FR-003).
  - **Write**: on every toggle click, `sessionStorage.setItem('devteam.dashboard.view', next)`.
  - **Invalidation**: a value that is not `'board'` or `'list'` is treated as absent → default `'board'`.
- **Visibility**: the toggle is rendered **only** when `features.length > 0` (FR-004). When `features.length === 0`, `EmptyState` renders and the toggle is absent. The stored preference is preserved across the empty → non-empty transition (US-3 scenario 3).

## Relationships Summary

```
FeatureListResponse (from useQuery)
  └── features: FeatureSummary[]   (1 : N)
        └── grouped by groupFeaturesByPhase()
              └── PhaseColumn[]    (1 : N, partition)
                    └── features: FeatureSummary[]  (1 : N, subset)

ViewPreference (sessionStorage) ── controls ──> Dashboard renders FeatureList | KanbanBoard
```

## Data Integrity Rules

1. **Partition invariant**: `groupFeaturesByPhase` must place every input feature in exactly one column. Verification: unit test asserts `sum of column lengths === input length` for inputs including unknown phases, empty arrays, and all-known-phase inputs (AC-011).
2. **Order invariant**: the six known columns render in `PHASES` order. The `'other'` column, when present, is always last. Verification: e2e AC-019 asserts column order.
3. **No-null-arrays**: `PhaseColumn.features` is always an array, never `null` or `undefined`. Empty columns carry `[]` and the UI renders the "No features" placeholder (FR-012, CON-008). Verification: code review + e2e AC-017.
4. **Single source of truth for labels**: column headers use `PHASE_LABELS`; card badges use `STATUS_LABELS` and `PRIORITY_LABELS`. No duplicated string literals (CON-005). Verification: grep for phase/status name literals in board component files.
5. **Single fetch**: the Board receives `features` as a prop from Dashboard, which owns the single `useQuery(['features'])` call. The Board makes zero network requests (CON-007, FR-016). Verification: integration AC-016.

## State Transitions

### ViewPreference

| From | To | Trigger | Notes |
|---|---|---|---|
| (none / invalid) | `board` | Dashboard mount, no stored value | Default (FR-003) |
| `board` | `list` | User clicks "List" toggle | Persisted to sessionStorage |
| `list` | `board` | User clicks "Board" toggle | Persisted to sessionStorage |
| any | (hidden) | `features.length === 0` | Toggle not rendered; preference retained in storage |
| (hidden) | last stored | `features.length > 0` again | Toggle re-renders; stored view resumes (US-3 scenario 3) |

Invalid transitions: none possible — the toggle is a two-state control. An invalid stored value is normalized to `board` on read.

### PhaseColumn

No state transitions. The column set is recomputed on every render from the current `features` array. A feature "moving" between columns is a consequence of `current_phase` changing in the backend (via `/advance` or `/recirculate`), which invalidates the react-query cache and triggers re-render — not a Board-internal state machine.