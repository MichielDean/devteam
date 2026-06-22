# Contract: GET /api/features

> **No change to this endpoint.** Documented here because the Kanban Board consumes its response. The Board adds zero new endpoints and zero new requests (CON-007, FR-016). This contract is read-only from the Board's perspective.

## Request

```
GET /api/features
```

**Headers**: none required (no auth on this endpoint — view-only read).

**Query params**: none.

**Body**: none.

## Response

### 200 OK

```json
{
  "features": [
    {
      "id": "feat-abc123",
      "title": "Kanban View",
      "status": "in_progress",
      "priority": 1,
      "current_phase": "planning",
      "updated_at": "2026-06-22T16:57:08Z",
      "gate_result": {
        "phase": "inception",
        "passed": true,
        "checks": [
          { "name": "artifact_spec_md_exists", "passed": true, "message": "..." }
        ]
      },
      "pending_questions_count": 0
    }
  ],
  "total_count": 1
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `features` | `FeatureSummary[]` | yes | **MUST be `[]` not `null`** when empty. The Board treats `[]` as the empty-board state → `EmptyState` renders (FR-004). |
| `total_count` | `number` | yes | Count of all features. Dashboard derives the badge from this; falls back to `features.length` when missing (defensive, CON-008). |

### `FeatureSummary` schema

| Field | Type | Required | Nullable | Validation | Board use |
|---|---|---|---|---|---|
| `id` | `string` | yes | no | unique | Card key, link target `/features/:id` |
| `title` | `string` | yes | no | non-empty | Card title |
| `status` | `string` | yes | no | one of `STATUS_LABELS` keys (defensive: unknown tolerated) | Status badge + ring flag |
| `priority` | `number` | yes | no | `1 \| 2 \| 3` | Priority badge |
| `current_phase` | `string` | yes | no | one of `PHASES` (defensive: unknown → "Other" column) | Column assignment |
| `updated_at` | `string` (ISO 8601) | yes | no | parseable date | Card updated line |
| `gate_result` | `GateResult \| null` | no | yes | — | Gate indicator when present |
| `pending_questions_count` | `number` | yes | no | `>= 0` | Question badge when `> 0` |

### `GateResult` schema

| Field | Type | Required | Notes |
|---|---|---|---|
| `phase` | `string` | yes | Phase the gate ran on |
| `passed` | `boolean` | yes | `true` → `✓ Gate passed`, `false` → `✗ Gate failed` |
| `checks` | `CheckResult[]` | yes | Individual check results (not rendered on the card) |

## Error Responses

| Status | Body | Board behavior |
|---|---|---|
| `500` | `{ "error": "internal_error", "details": "..." }` | Dashboard renders `features-error`; Board does not render (FR-017, AC-015). Toggle hidden. |
| `502` / `503` | `{ "error": "...", "details": "..." }` | Same as 500 — react-query error branch. |
| Network failure | (no response) | Same error branch. |

The Board itself never handles HTTP errors — it only renders when `!isLoading && !error && features.length > 0`. Error handling is owned by Dashboard's existing branches (CON-008).

## Examples

### Happy path — features present

```http
GET /api/features
```

```json
{
  "features": [
    {
      "id": "feat-1",
      "title": "Kanban View",
      "status": "in_progress",
      "priority": 1,
      "current_phase": "planning",
      "updated_at": "2026-06-22T16:57:08Z",
      "gate_result": null,
      "pending_questions_count": 2
    },
    {
      "id": "feat-2",
      "title": "Count Badge",
      "status": "done",
      "priority": 3,
      "current_phase": "delivery",
      "updated_at": "2026-06-20T10:00:00Z",
      "gate_result": { "phase": "testing", "passed": true, "checks": [] },
      "pending_questions_count": 0
    }
  ],
  "total_count": 2
}
```

Board renders: `feat-1` in Planning column (yellow ring for `waiting_for_human`? no — status is `in_progress`; question badge shows `2`); `feat-2` in Delivery column with `✓ Gate passed`.

### Empty board

```json
{ "features": [], "total_count": 0 }
```

Dashboard renders `EmptyState`; toggle hidden (FR-004, AC-006/018).

### Defensive — unknown `current_phase`

```json
{
  "features": [
    { "id": "feat-x", "title": "Weird", "status": "draft", "priority": 2,
      "current_phase": "retro", "updated_at": "2026-06-22T00:00:00Z",
      "gate_result": null, "pending_questions_count": 0 }
  ],
  "total_count": 1
}
```

Board renders six known columns (all empty placeholders) plus a trailing "Other" column containing `feat-x` (FR-007, AC-011). No crash.