# Contract: GET /api/features

**Status**: Existing endpoint, unchanged. Documented here because the Kanban board consumes it. No new endpoints are introduced by feature `kanban-view`.

## Request

- **Method**: `GET`
- **Path**: `/api/features`
- **Headers**: none required (no auth boundary for this read endpoint).
- **Query params**: none.
- **Body**: none.

## Responses

### 200 OK

```json
{
  "features": [
    {
      "id": "kanban-view-1750617600",
      "title": "Kanban View",
      "status": "in_progress",
      "priority": 1,
      "current_phase": "planning",
      "updated_at": "2026-06-22T00:00:00Z",
      "gate_result": null,
      "pending_questions_count": 0
    }
  ],
  "total_count": 1
}
```

**Schema**:

| Field | Type | Nullable | Notes |
|---|---|---|---|
| `features` | `FeatureSummary[]` | no | **MUST be `[]` not `null`** when empty (CON-009, AC-007). |
| `total_count` | `number` | no | Integer `>= 0`. |
| `features[].id` | `string` | no | Unique feature id. |
| `features[].title` | `string` | no | UTF-8 text; rendered as text node (no HTML). |
| `features[].status` | `string` | no | One of `STATUS_LABELS` keys or a future value. |
| `features[].priority` | `number` | no | `1 \| 2 \| 3`. |
| `features[].current_phase` | `string` | no | Expected: one of `PHASES`. Unknown values are valid and route to the board's "Other" column (CON-011, FR-013, AC-016). |
| `features[].updated_at` | `string` | no | ISO 8601 timestamp. |
| `features[].gate_result` | `GateResult \| null` | yes | `null` when no gate has run. |
| `features[].gate_result.passed` | `boolean` | no | Drives the `feature-card-gate` indicator. |
| `features[].pending_questions_count` | `number` | no | `>= 0`. Drives `QuestionBadge` rendering. |

### 500 Internal Server Error

```json
{ "error": "internal_error", "details": "Failed to list features" }
```

The Dashboard's existing `features-error` branch renders for any non-2xx response (CON-009, AC-006). The Kanban board is not rendered when the query errors.

## Consumer contract (UI)

- `ui/src/api/client.ts` `listFeatures()` issues this request and returns `FeatureListResponse`.
- `ui/src/pages/Dashboard.tsx` wraps it in `useQuery({ queryKey: ['features'], queryFn: listFeatures })`.
- Both list and kanban views consume `data?.features ?? []`. Toggling views MUST NOT trigger a second request (FR-014, AC-004) — verified by the TanStack Query cache.

## No new contracts

Feature `kanban-view` introduces no new HTTP endpoints, no new request bodies, no new error codes. The only new "interface" is the UI-local `localStorage['devteam.dashboard.view']` key (see `data-model.md`), which is not an API contract.