# Data Model: kanban-view

## Scope Note

This feature introduces **no new persisted entities**. It is a pure presentation layer over the existing `FeatureSummary` returned by `GET /api/features` (already implemented, shape unchanged). The "entities" below are **view-only derived structures** — TypeScript types and runtime-derived groupings, not database rows or API resources.

There is no backend data model change. No migrations. No new API contracts. The only contracts are internal component prop contracts (see `contracts/`).

## Existing Entity (unchanged, consumed)

### FeatureSummary
- **Source**: `ui/src/types/index.ts`, returned by `listFeatures()` in `ui/src/api/client.ts`
- **Attributes**:
  | Field | Type | Nullable | Notes |
  |---|---|---|---|
  | `id` | string | no | UUID; used in `feature-card-${id}` testid and `/features/${id}` route |
  | `title` | string | no | Rendered truncated by `FeatureCard` |
  | `status` | string | no | One of `STATUS_LABELS` keys; unknown values fall through to raw string |
  | `priority` | number | no | 1 (P1), 2 (P2), 3 (P3). Lower = higher priority. Sort key 1. |
  | `current_phase` | string | no | Should be one of `PHASES`; unknown values handled by FR-017 "Other" column |
  | `updated_at` | string | no | ISO 8601 timestamp; sort key 2 (descending) |
  | `gate_result` | `GateResult \| null` | yes | Rendered by `FeatureCard` if present |
  | `pending_questions_count` | number | no | Drives `QuestionBadge` rendering |
- **Relationships**: none at the view layer. Each feature is independent.
- **Constraints**: none added. Existing API constraints (UUID id, ISO timestamp) unchanged.

## New View-Only Structures (not persisted)

### ViewMode
- **Type**: TypeScript union `'list' | 'kanban'`
- **Persistence**: `localStorage` key `devteam-dashboard-view`. Values: `"list"` | `"kanban"`. Absent key → default `"list"` (FR-005, AC-004).
- **Validation on read**: if `localStorage.getItem` returns a value not in `{"list","kanban"}`, treat as `"list"` (defensive; never render a broken state from a corrupt stored value).
- **Transitions**: `list ⇄ kanban`, both directions always valid. No state machine beyond the binary toggle.

### PhaseColumn (derived, per-render)
- **Derived from**: `PHASES` constant + `FeatureSummary[]`.
- **Attributes**:
  | Field | Type | Notes |
  |---|---|---|
  | `phase` | `PhaseName` (or `"other"`) | Key for grouping |
  | `label` | string | From `PHASE_LABELS[phase]` for known phases; `"Other"` for the fallback column |
  | `features` | `FeatureSummary[]` | Filtered to `current_phase === phase`, then ordered by `orderCards` (priority asc, `updated_at` desc) |
  | `count` | number | `features.length`; shown in header (FR-006, AC-007) |
  | `testid` | string | `kanban-column-${phase}` (CON-006) |
- **Lifecycle**: created fresh each render of `KanbanBoard`. Not memoized across renders (YAGNI — feature count is small, regrouping is O(n)).
- **Constraints**:
  - Every `FeatureSummary` in the input appears in exactly one column (FR-007, SC-002). A feature whose `current_phase` is a known phase goes to that column; otherwise to the `other` column (FR-017).
  - Sum of all `column.count` values === `features.length` === `data.total_count` (SC-003, AC-CON-004 verification).
  - Column order for known phases is fixed by `PHASES` array order (FR-002, AC-CON-004). The `other` column, if present, renders last.

### OtherColumn (fallback, conditional)
- **Derived from**: features whose `current_phase` is not in `PHASES`.
- **Rendered**: only when at least one such feature exists (FR-017). Empty "Other" column is NOT rendered (unlike phase columns which are always shown per FR-010/AC-011 — the "Other" column is a defensive bucket, not a guaranteed column).
- **testid**: `kanban-column-other`
- **Label**: `"Other"`

## orderCards helper (pure function)

- **Signature**: `orderCards(features: FeatureSummary[]): FeatureSummary[]`
- **Location**: `ui/src/components/KanbanBoard.tsx` (exported) or `ui/src/lib/orderCards.ts`. Decision: colocate in `KanbanBoard.tsx` and export — avoids a new file for a 6-line function (ponytail: fewest files). If a second consumer appears, extract.
- **Behavior**: returns a new array sorted by:
  1. `priority` ascending (1 before 2 before 3) — FR-012, AC-016
  2. `updated_at` descending (most recent first) as tiebreaker — FR-012, AC-017
- **Purity**: must not mutate input (mirrors `FeatureList` `[...features].sort()` pattern).
- **Stability**: not required — the tiebreaker fully determines order when priorities differ or timestamps differ. Equal priority + equal timestamp order is unspecified and acceptable.
- **Unit test target**: AC-018.

## Relationships Diagram (textual)

```
GET /api/features  →  FeatureListResponse { features: FeatureSummary[], total_count }
                                   │
                                   ▼
                        Dashboard (useQuery)
                                   │
                  ┌────────────────┼────────────────┐
                  ▼                ▼                 ▼
            isLoading         error           features[] (non-empty)
                  │                │                 │
                  ▼                ▼                 ▼
          features-loading   features-error    ┌── viewMode ──┐
          (unchanged)        (unchanged)       │              │
                                               ▼              ▼
                                          'list'         'kanban'
                                               │              │
                                               ▼              ▼
                                          FeatureList    KanbanBoard
                                          (unchanged)         │
                                                              ▼
                                                   PhaseColumn[] (6 + optional other)
                                                              │
                                                              ▼
                                                   FeatureCard (reused, unchanged)
```

## Validation Rules Summary

| Rule | Where enforced | AC |
|---|---|---|
| `viewMode` invalid stored value → default `list` | Dashboard read from localStorage | AC-004 |
| Unknown `current_phase` → `other` column | KanbanBoard grouping | FR-017, AC-CON-004 |
| Sum of column counts === total features | KanbanBoard grouping (invariant) | SC-003 |
| Column order === `PHASES` order | KanbanBoard render | AC-CON-004 |
| Card order: priority asc, updated_at desc | `orderCards` | AC-016, AC-017, AC-018 |
| Empty known column still renders | KanbanColumn | FR-010, AC-011 |
| Zero features → EmptyState, toggle hidden | Dashboard | FR-015, FR-016, AC-019, AC-020 |