# Contract: GET /api/features

**Status**: UNCHANGED. This endpoint already exists and is the sole data source for the Kanban board. Documented here because the board consumes it and the constraint register (CON-007, AC-016) requires verifying the board makes **exactly one** call to it. No new endpoint is introduced by `kanban-view`.

## Method & Path

`GET /api/features`

## Request

### Headers
- `Accept: application/json` (optional; server defaults to JSON)

### Query Parameters
None.

### Body
None.

## Responses

### 200 OK

```json
{
  "features": [
    {
      "id": "kanban-view-2026-06-22",
      "title": "Kanban View",
      "status": "in_progress",
      "priority": 1,
      "current_phase": "planning",
      "updated_at": "2026-06-22T23:05:00-06:00",
      "gate_result": null,
      "pending_questions_count": 2
    }
  ],
  "total_count": 1
}
```

#### Schema

| Field | Type | Required | Notes |
|---|---|---|---|
| `features` | `FeatureSummary[]` | yes | MUST be `[]` not `null` when empty (CON-008 empty state). Existing app.spec.ts asserts `Array.isArray(body.features)`. |
| `total_count` | `number` | no (defensive) | Board falls back to `features.length` if missing (existing Dashboard pattern; e2e `feature count badge handles missing total_count`). |
| `FeatureSummary.id` | `string` | yes | non-empty. Used in `kanban-card-${id}` testid + `<Link to="/features/${id}">`. |
| `FeatureSummary.title` | `string` | yes | Rendered as card heading. |
| `FeatureSummary.status` | `string` (enum) | yes | One of `draft\|in_progress\|gate_blocked\|passed\|failed\|done\|recirculated\|cancelled\|waiting_for_human`. Drives status badge + ring flag. |
| `FeatureSummary.priority` | `number` | yes | `1\|2\|3`. Drives `PRIORITY_LABELS[priority]` badge. |
| `FeatureSummary.current_phase` | `string` | yes | Expected one of `PHASES`; unknown values → `other` column (FR-007). |
| `FeatureSummary.updated_at` | `string` (ISO 8601) | yes | Rendered as "Updated {date}". |
| `FeatureSummary.gate_result` | `GateResult \| null` | no | When non-null and for the current phase, card shows `✓ Gate passed` / `✗ Gate failed`. |
| `GateResult.phase` | `string` | yes | |
| `GateResult.passed` | `boolean` | yes | |
| `GateResult.checks` | `CheckResult[]` | yes | MUST be `[]` not `null`. |
| `FeatureSummary.pending_questions_count` | `number` | yes (default 0) | `> 0` → `QuestionBadge` renders. |

### 500 Internal Server Error

```json
{
  "error": "internal_error",
  "details": "Failed to list features"
}
```

The board does not handle this directly — the existing Dashboard `error` branch renders `features-error` testid and the board/toggle are not rendered (FR-017, AC-015).

## Board-Specific Consumption Notes

- **Single fetch (CON-007, AC-016)**: the board consumes the same `useQuery(['features'])` result as the list view. react-query dedupes by query key, so exactly one HTTP request is made per Dashboard mount. Verification: Playwright `page.on('request')` count for `/api/features` === 1 during Board render.
- **No mutation**: the board is read-only. No POST/PUT/DELETE originates from board code.
- **No new fields**: if the board needs a field not in `FeatureSummary`, the plan is wrong — escalate, do not extend the API.