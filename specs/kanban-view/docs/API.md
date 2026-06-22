# API Documentation — Kanban View (Spec kanban-view)

The Kanban view is a UI-only feature. It introduces **no new backend endpoints**.
It consumes the single existing endpoint below. This documentation describes the
contract as the board relies on it, matching the terminology defined in
`specs/kanban-view/spec.md`.

## `GET /api/features`

**Purpose**: List all feature specs as `FeatureSummary` cards. The Kanban board
groups this single response into 7 columns (Backlog + the 6 pipeline phases:
Inception, Planning, Construction, Review, Testing, Delivery). This endpoint is
unchanged by the kanban-view feature; the board reuses it as-is (CON-003, FR-004).

**Request**: no request body, no query parameters. GET only.

**Response 200**:
```json
{
  "features": [FeatureSummary, ...],
  "total_count": number
}
```

| Field | Type | Description |
|-------|------|-------------|
| `features` | `FeatureSummary[]` | Every feature spec in the system. **Always an array, never `null`** — serializes as `[]` when empty (CON-004). The board initializes all 7 columns to empty arrays before grouping, so even a null would not crash the page. |
| `total_count` | `number` | Count of features in `features`. Drives the `feature-count-badge` shown in the Dashboard header; the badge stays mounted across list/board toggles (CON-010, FR-007). |

### `FeatureSummary`

| Field | Type | Spec terminology | Board use |
|-------|------|------------------|-----------|
| `id` | `string` | feature id | Card identity; clicking a card navigates to `/features/{id}` (US-004) |
| `title` | `string` | feature title | Rendered by the reused `FeatureCard` (CON-005) |
| `status` | `string` enum: `draft`, `in_progress`, `gate_blocked`, `passed`, `failed`, `done`, `recirculated`, `cancelled`, `waiting_for_human` | status | Drives the Backlog grouping rule (CON-002): `status === 'draft'` AND `current_phase === 'inception'` → Backlog column. All other statuses fall through to the `current_phase` column, including terminal `done` and `cancelled` (CON-009). |
| `priority` | `number` enum: `1` (P1 - Critical), `2` (P2 - Medium), `3` (P3 - Low) | priority | Rendered as priority badge by `FeatureCard` |
| `current_phase` | `string` enum: `inception`, `planning`, `construction`, `review`, `testing`, `delivery` (empty string for a feature that has never entered a phase) | current phase | Drives column placement (CON-001, FR-003). The board column set is derived from the canonical `PHASES` constant in `ui/src/types/index.ts`, which mirrors `internal/feature/types.go`. |
| `updated_at` | `string` (RFC 3339 / ISO 8601) | updated date | Rendered by `FeatureCard` |
| `gate_result` | `GateResult \| null` | gate indicator | Rendered by `FeatureCard` (✓ Gate passed / ✗ Gate failed) |
| `pending_questions_count` | `number` | pending questions count | Drives the `QuestionBadge` rendered by `FeatureCard` |

### Column grouping rule (derived, client-side only)

```
backlog      := features where status == 'draft' AND current_phase == 'inception'
inception    := features where current_phase == 'inception' AND NOT (status == 'draft')
planning     := features where current_phase == 'planning'
construction := features where current_phase == 'construction'
review       := features where current_phase == 'review'
testing      := features where current_phase == 'testing'
delivery     := features where current_phase == 'delivery'
```

Every feature appears in exactly one column. A feature with an unknown
`current_phase` is dropped defensively by the grouping function (it cannot
happen given the Go enum, but the guard prevents a runtime crash if the wire
value ever drifts). This grouping is performed entirely in the browser by the
pure function `ui/src/lib/groupFeaturesByColumn.ts` — there is no server-side
filtering, no per-phase query param, and no new endpoint (AC-CON-003).

**Response 500** (error path, AC-ERR-001):
```json
{ "error": "internal_error", "details": "..." }
```
The board renders a top-level error banner: `Failed to load features: {details}`,
keeps all 7 columns rendered (empty), and does not throw an uncaught exception.

## Endpoints NOT introduced by this feature

The kanban-view feature explicitly does **not** add any of the following
(verified by `git diff main -- internal/api/server.go ui/src/api/client.ts`
showing no new `mux.HandleFunc` and no new client function):

- No `GET /api/features/kanban`
- No `GET /api/features/grouped`
- No per-phase filtering query parameter
- No new client function in `ui/src/api/client.ts` beyond the existing
  `listFeatures()` (which calls `GET /api/features`)

## Authentication

Unchanged. `GET /api/features` is served by the existing Dev Team API server with
its existing auth model. The Kanban view adds no new input handling, no new
endpoints to protect, and no user input rendered unescaped (React auto-escapes
`FeatureCard` text).

## Cache behavior

The board calls `useQuery({ queryKey: ['features'], queryFn: listFeatures })` —
the **same cache key** the existing Dashboard list view uses (FR-014). This means:

- One network fetch is shared between list and board views.
- Any mutation that invalidates `['features']` (create, run phase, advance,
  recirculate, cancel) propagates to the board automatically — cards move columns
  without a full page reload (US-006, AC-014).
- On a refetch error mid-session, react-query keeps the previous `data`
  populated by default, so stale cards remain visible rather than the board
  going blank (AC-ERR-002).