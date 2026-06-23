# Data Model: kanban-view

This feature introduces **no new persistent entities**. It is a view over existing `GET /api/features` data. Two UI-only, ephemeral, derived "entities" are documented for implementation clarity.

## Entities

### FeatureSummary (existing, unchanged — consumed read-only)

- **Source**: `GET /api/features` → `FeatureListResponse.features[]`. Go DTO: `internal/api/dto.go` `FeatureSummaryResponse`. TS type: `ui/src/types/index.ts` `FeatureSummary`.
- **Attributes**:

| Field | Type | Required | Nullable | Default | Validation | Board use |
|---|---|---|---|---|---|---|
| `id` | `string` | yes | no | — | unique | Card key; `<Link to={/features/:id}>` |
| `title` | `string` | yes | no | — | non-empty | Card title (line-clamp-2) |
| `status` | `string` | yes | no | — | one of `STATUS_LABELS` keys; unknown tolerated (defensive) | Status badge + ring flag (FR-008/011) |
| `priority` | `number` | yes | no | — | `1 \| 2 \| 3` | Priority badge (FR-008) |
| `current_phase` | `string` | yes | no | — | one of `PHASES`; unknown → "Other" column (FR-007, CON-009) | Column assignment (FR-006) |
| `updated_at` | `string` (ISO 8601) | yes | no | — | parseable date | Card updated line |
| `gate_result` | `GateResult \| null` | no | yes | `null` | — | Gate indicator when present (FR-009) |
| `pending_questions_count` | `number` | yes | no | `0` | `>= 0` | Question badge when `> 0` (FR-008) |

- **Relationships**: none (flat summary; detail lives in `FeatureDetail` via `GET /api/features/:id`).
- **Constraints**: server-side `features` array MUST be `[]` not `null` (CON-008 — pinned by `internal/api/kanban_smoke_test.go`).

### GateResult (existing, unchanged — nested inside FeatureSummary)

| Field | Type | Required | Notes |
|---|---|---|---|
| `phase` | `string` | yes | Phase the gate ran on |
| `passed` | `boolean` | yes | `true` → `✓ Gate passed`; `false` → `✗ Gate failed` (CON-006: byte-identical to `FeatureCard`) |
| `checks` | `CheckResult[]` | yes | Not rendered on the card |

### PhaseColumn (UI-only, ephemeral, not persisted)

- **Source**: derived in `groupFeaturesByPhase(features)` — recomputed every render from the query result.
- **Attributes**:

| Field | Type | Required | Notes |
|---|---|---|---|
| `phase` | `PhaseName \| 'other'` | yes | Key; one of `PHASES` or the defensive `'other'` bucket |
| `label` | `string` | yes | `PHASE_LABELS[phase]` for known phases; `'Other'` for the defensive column |
| `features` | `FeatureSummary[]` | yes | Subset whose `current_phase` maps to this phase. **Always `[]`, never `null`** (CON-008). |

- **Relationships**: 1 Board : 6..7 PhaseColumns; 1 PhaseColumn : 0..N FeatureSummary.
- **Constraints**: partition invariant — `sum(column.features.length for all columns) === input features.length`. No feature in two columns, no feature dropped (SC-002, FR-006).

### ViewPreference (UI-only, session-scoped)

- **Source**: `useSessionView` hook; persisted in `sessionStorage` under `devteam.dashboard.view`.
- **Attributes**:

| Field | Type | Required | Default | Validation |
|---|---|---|---|---|
| `value` | `'list' \| 'board'` | yes | `'list'` (FR-003) | Anything else in storage → default `'list'`. Storage access failure (private mode) → `'list'` via try/catch. |

- **Lifecycle**: session-scoped. Cleared when browser tab closes. No cross-session persistence.
- **State transitions**: `'list' ⇄ 'board'` via toggle click. No other states.

## Relationships

```
Dashboard
  └── useQuery(['features']) ── FeatureListResponse
                                  └── FeatureSummary[] ──┬── FeatureList (view='list')
                                                          └── KanbanBoard (view='board')
                                                                └── groupFeaturesByPhase
                                                                      └── PhaseColumn[] (6 + optional 'other')
                                                                            └── KanbanCard per FeatureSummary
```

## State Transitions

**No feature state transitions are introduced or altered.** The board only observes feature state; transitions remain governed by `internal/feature/feature.go` (`draft → in_progress → gate_blocked/passed/failed → recirculated → ... → done | cancelled`). The board reflects the current state read-only.

**ViewPreference state machine** (the only new state):

```
[list] ──click Board──> [board]
[board] ──click List──> [list]
[board] ──reload (same session)──> [board]   (FR-002, AC-004)
[fresh session] ──load──> [list]              (FR-003, AC-005)
[invalid stored value] ──load──> [list]       (defensive)
[storage access throws] ──load──> [list]      (private mode, try/catch)
```

Invalid transitions: none possible (two-state toggle).

## Data Integrity Rules

1. **Partition invariant**: every `FeatureSummary` in the input appears in exactly one `PhaseColumn`. `sum === input.length`. Unit-tested (AC-011, CON-009).
2. **No null arrays**: every `PhaseColumn.features` initialized to `[]`. `FeatureListResponse.features` is `[]` not `null` from the API (CON-008, pinned server-side).
3. **Unknown phase routing**: `current_phase` not in `PHASES` → `other` bucket, not dropped, no throw (CON-009, FR-007).
4. **Terminal features visible**: `status` of `done` / `cancelled` does NOT filter the feature out — it stays in its `current_phase` column (spec assumption, CON-009 acceptance).
5. **Single source of truth for labels**: `PHASE_LABELS` / `STATUS_LABELS` / `PRIORITY_LABELS` from `types/index.ts`; no duplicated string literals in board components (CON-005).
6. **Single source of truth for status colors**: `badgeColors.ts` `statusColors` map; consumed by both `FeatureCard` and `KanbanCard` (CON-006).