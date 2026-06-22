# API Contract: None (UI-only feature)

**Feature**: kanban-view
**HTTP endpoints added**: 0
**HTTP endpoints modified**: 0
**Backend changes**: 0

This feature is a pure frontend presentation layer over the existing `GET /api/features` endpoint. That endpoint is **already implemented and unchanged**. Its contract is documented here for reference so the Developer and Tester know the exact shape the Kanban consumes.

---

## Existing Endpoint (consumed, not modified)

### `GET /api/features`

**Existing response** (from `ui/src/types/index.ts` `FeatureListResponse`):

```json
{
  "features": [
    {
      "id": "string (UUID)",
      "title": "string",
      "status": "in_progress | done | cancelled | draft | gate_blocked | passed | failed | recirculated | waiting_for_human",
      "priority": 1 | 2 | 3,
      "current_phase": "inception | planning | construction | review | testing | delivery",
      "updated_at": "ISO-8601 string",
      "gate_result": { "phase": "string", "passed": "boolean", "checks": [...] } | null,
      "pending_questions_count": "number"
    }
  ],
  "total_count": "number"
}
```

**Status codes** (existing, unchanged):
- `200 OK` — features array (possibly empty `[]`, never `null` — see existing `app.spec.ts` "API returns valid JSON with arrays not null")
- `500` — `{ "error": "...", "details": "..." }` (handled by Dashboard `features-error` testid)

**What the Kanban consumes from this response**:
- `features[].id` → `feature-card-${id}` testid, `/features/${id}` navigation
- `features[].title` → card title (via `FeatureCard`)
- `features[].status` → status badge (via `FeatureCard`)
- `features[].priority` → priority badge + column sort key 1
- `features[].current_phase` → column assignment (group-by key)
- `features[].updated_at` → column sort key 2 (tiebreaker, descending)
- `features[].gate_result`, `features[].pending_questions_count` → rendered by `FeatureCard`, passed through untouched
- `total_count` → not directly used by Kanban (Dashboard badge uses it). The invariant `Σ column.count === features.length` is checked against `features.length`, not `total_count` (they are equal per existing API).

**No new query params, no new headers, no new request body.** The Kanban calls `listFeatures()` exactly as `FeatureList` does today.

---

## Internal Component Contracts

See `contracts/components.md` for the prop contracts of:
- `ViewToggle` (new)
- `KanbanBoard` (new)
- `KanbanColumn` (new)
- `orderCards` (pure helper exported from `KanbanBoard.tsx`)

These are not HTTP contracts but they ARE the contracts the Developer implements against and the Reviewer/Tester verify against.