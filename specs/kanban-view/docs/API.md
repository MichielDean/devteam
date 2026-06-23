# API Documentation — Kanban View (spec `kanban-view`)

The Kanban view is a **UI-only** feature. It introduces **no new backend
endpoints**, no new query parameters, and no new request bodies. The board
consumes the single existing endpoint documented below, using the terminology
defined in `specs/kanban-view/spec.md` and the contract in
`specs/kanban-view/contracts/GET-api-features.md`.

## `GET /api/features`

**Purpose**: List every feature as a `FeatureSummary`. The Kanban board groups
this single response into six phase columns (Inception, Planning,
Construction, Review, Testing, Delivery) plus an optional trailing "Other"
column for defensive handling of unknown `current_phase` values (FR-002,
FR-013, AC-016). This endpoint is **unchanged** by the `kanban-view` feature;
the board reuses it as-is (FR-014, CON-007).

**Request**: no request body, no query parameters, no headers required. GET
only.

**Response 200**:

```json
{
  "features": [FeatureSummary, ...],
  "total_count": 123
}
```

| Field | Type | Description |
|-------|------|-------------|
| `features` | `FeatureSummary[]` | Every feature in the system. **MUST be `[]` not `null`** when empty (the #1 agent-generated serialization bug). The board treats `[]` as the empty-board state — Dashboard renders the existing `EmptyState` call-to-action (CON-010, AC-007). |
| `total_count` | `number` | Count of all features. |

### `FeatureSummary`

| Field | Type | Spec terminology | Board use |
|-------|------|------------------|-----------|
| `id` | `string` | feature id | Card key; clicking a Kanban card navigates to `/features/{id}` (FR-005, AC-002) |
| `title` | `string` | feature title | Kanban card title — rendered as a text node, never via `dangerouslySetInnerHTML` (spec Security notes) |
| `status` | `string` | status | Status badge via `STATUS_LABELS` (FR-004) |
| `priority` | `number` enum: `1` (P1), `2` (P2), `3` (P3) | priority | Priority badge via `PRIORITY_LABELS` (FR-004) |
| `current_phase` | `string` enum: `inception`, `planning`, `construction`, `review`, `testing`, `delivery` | current phase | Drives column placement (FR-002, FR-003). Unknown values route to the trailing "Other" column (FR-013, AC-016). Column set and order come from the canonical `PHASES` constant in `ui/src/types/index.ts` (FR-015, CON-004). |
| `updated_at` | `string` (ISO 8601) | updated date | Card updated line |
| `gate_result` | `GateResult \| null` | gate result | When present, the Kanban card shows a visible gate-status indicator: `passed: true` → gate-passed; `passed: false` → gate-failed (FR-011, AC-013/014). Absent (`null`) → no indicator (AC-015). |
| `pending_questions_count` | `number` | pending questions count | When `> 0`, the Kanban card shows a pending-questions badge with the count, reusing the existing `QuestionBadge` (FR-010, AC-012). |

### `GateResult`

| Field | Type | Description |
|-------|------|-------------|
| `passed` | `boolean` | `true` → gate-passed indicator; `false` → gate-failed indicator (FR-011). |
| `checks` | `CheckResult[]` | Individual check results (not rendered on the Kanban card). |

### Column grouping rule (client-side only, pure function)

```
inception    := features where current_phase == 'inception'
planning     := features where current_phase == 'planning'
construction := features where current_phase == 'construction'
review       := features where current_phase == 'review'
testing      := features where current_phase == 'testing'
delivery     := features where current_phase == 'delivery'
other        := features where current_phase not in PHASES  (rendered iff non-empty)
```

All six phase columns always render, even when empty (FR-012, AC-007). The
"Other" column renders **only** when at least one feature has an unknown
`current_phase` (FR-013, AC-016/017). Within each column, features stay in the
order returned by the API — no re-sort (FR-003).

**Response 500**:

```json
{ "error": "internal_error", "details": "Failed to list features" }
```

The Dashboard's existing `features-error` branch renders for any non-2xx
response (CON-009, AC-006). The Kanban board is **not** rendered when the
query errors.

## No new endpoints

Feature `kanban-view` introduces no new HTTP endpoints, no new request bodies,
no new query parameters, and no new error codes. The only new "interface" is
the UI-local `localStorage['devteam.dashboard.view']` key (see
`data-model.md`), which is not an API contract.

## Network behaviour on view toggle

Both the List view and the Kanban view consume the same `useQuery(['features'])`
result. Toggling views MUST NOT issue a second `GET /api/features` request
(FR-014, AC-004) — verified by the TanStack React Query cache.