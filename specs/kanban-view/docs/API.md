# API Documentation — Kanban View (spec kanban-view)

The Kanban view is a **UI-only** feature. It introduces **no new backend
endpoints**, no new query parameters, and no new request bodies. The board
consumes the single existing endpoint documented below, using the terminology
defined in `specs/kanban-view/spec.md` and the contract in
`specs/kanban-view/contracts/GET-api-features.md`.

## `GET /api/features`

**Purpose**: List every feature spec as a `FeatureSummary`. The Kanban board
groups this single response into six phase columns (Inception, Planning,
Construction, Review, Testing, Delivery) plus an optional trailing "Other"
column for defensive handling of unknown `current_phase` values (FR-005,
FR-006, FR-007). This endpoint is **unchanged** by the kanban-view feature;
the board reuses it as-is (FR-016, CON-007).

**Request**: no request body, no query parameters, no headers required. GET
only.

**Response 200**:

```json
{
  "features": [FeatureSummary, ...],
  "total_count": number
}
```

| Field | Type | Description |
|-------|------|-------------|
| `features` | `FeatureSummary[]` | Every feature spec in the system. **MUST be `[]` not `null`** when empty (CON-008, the #1 agent-generated serialization bug). The board treats `[]` as the empty-board state — Dashboard renders `EmptyState` and the toggle is hidden (FR-004, AC-006/018). |
| `total_count` | `number` | Count of all features. Drives the `feature-count-badge` in the Dashboard header. Defensive: when missing, Dashboard falls back to `features.length` (CON-008, existing e2e). |

### `FeatureSummary`

| Field | Type | Spec terminology | Board use |
|-------|------|------------------|-----------|
| `id` | `string` | feature id | Card key; clicking a card navigates to `/features/{id}` (FR-010, AC-010) |
| `title` | `string` | feature title | Card title (line-clamped to 2 lines) |
| `status` | `string` enum: `draft`, `in_progress`, `gate_blocked`, `passed`, `failed`, `done`, `recirculated`, `cancelled`, `waiting_for_human` | status | Status badge via `STATUS_LABELS` (FR-008); `gate_blocked` → red ring (FR-011, AC-012); `waiting_for_human` → yellow ring (FR-011, AC-013) |
| `priority` | `number` enum: `1` (P1 - Critical), `2` (P2 - Medium), `3` (P3 - Low) | priority | Priority badge via `PRIORITY_LABELS` (FR-008, AC-007) |
| `current_phase` | `string` enum: `inception`, `planning`, `construction`, `review`, `testing`, `delivery` | current phase | Drives column placement (FR-006). Unknown values → "Other" column (FR-007, AC-011). Column set and order come from the canonical `PHASES` constant in `ui/src/types/index.ts`. |
| `updated_at` | `string` (ISO 8601) | updated date | Card updated line |
| `gate_result` | `GateResult \| null` | gate result | When present, card shows `✓ Gate passed` or `✗ Gate failed` (FR-009, AC-009) — identical text to `FeatureCard` (CON-006) |
| `pending_questions_count` | `number` | pending questions count | When `> 0`, card shows the pending-questions badge (FR-008, AC-008) |

### `GateResult`

| Field | Type | Description |
|-------|------|-------------|
| `phase` | `string` | Phase the gate ran on |
| `passed` | `boolean` | `true` → `✓ Gate passed`, `false` → `✗ Gate failed` |
| `checks` | `CheckResult[]` | Individual check results (not rendered on the card) |

### Column grouping rule (client-side only, pure function)

```
inception    := features where current_phase == 'inception'
planning     := features where current_phase == 'planning'
construction := features where current_phase == 'construction'
review       := features where current_phase == 'review'
testing      := features where current_phase == 'testing'
delivery     := features where current_phase == 'delivery'
other        := features where current_phase not in PHASES  (defensive, FR-007)
```

Every feature appears in **exactly one** column (FR-006, SC-002). The
partition invariant `sum(buckets) === input.length` holds for any input. The
grouping is performed entirely in the browser by the pure function
`groupFeaturesByPhase` (exported from `KanbanBoard.tsx`, extracted as
`ui/src/components/groupFeaturesByPhase.ts`). There is no server-side
filtering, no per-phase query parameter, and no new endpoint (CON-007, AC-016).

### Error responses

| Status | Body | Board behavior |
|--------|------|----------------|
| `500` | `{ "error": "internal_error", "details": "..." }` | Dashboard renders the existing `features-error` branch; the board does not render; the toggle is hidden (FR-017, AC-015). |
| `502` / `503` | `{ "error": "...", "details": "..." }` | Same as 500 — react-query error branch. |
| Network failure | (no response) | Same error branch. |

The board itself never handles HTTP errors — it only renders when
`!isLoading && !error && features.length > 0`. Error handling is owned by
Dashboard's existing branches, reused unchanged (CON-008, FR-017).

## Endpoints NOT introduced by this feature

The kanban-view feature explicitly does **not** add any of the following
(verified by the Go smoke test `TestKanbanSmokeNoKanbanSpecificEndpoint` in
`internal/api/kanban_smoke_test.go`):

- No `GET /api/features/kanban`
- No `GET /api/features/grouped`
- No per-phase filtering query parameter
- No new client function in `ui/src/api/client.ts` beyond the existing
  `listFeatures()` (which calls `GET /api/features`)

## Authentication

Unchanged. `GET /api/features` is served by the existing Dev Team API server
with its existing auth model. The Kanban view adds no new input handling, no
new endpoints to protect, and no user input rendered unescaped (React
auto-escapes card text). The security extension is N/A per the spec.

## Cache behavior

The board calls `useQuery({ queryKey: ['features'], queryFn: listFeatures })` —
the **same cache key** the existing Dashboard list view uses (FR-016, CON-007).
This means:

- **One** network fetch is shared between list and board views (AC-016).
- Any mutation that invalidates `['features']` (create, run phase, advance,
  recirculate, cancel) propagates to the board automatically — cards move
  columns without a full page reload.
- On a refetch error mid-session, react-query keeps the previous `data`
  populated by default, so stale cards remain visible rather than the board
  going blank.