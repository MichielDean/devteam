# API Reference — Spec 003: Human Interaction Points

This document describes the REST API endpoints added by **Spec 003: Human Interaction Points**. These endpoints let a human answer clarification questions and design decisions surfaced by the PM (inception) and Architect (planning) agents through the web UI.

All endpoints are served by the same `devteam -http :<addr>` binary introduced in Spec 002. The endpoints share the existing `{"error": "code", "details": "message"}` error response format, the existing recovery/CORS middleware chain, and the existing SSE mechanism (`broadcastSSE`).

---

## Data Model

### Question

A clarification or decision that an agent (PM or Architect) surfaces for human input during inception or planning.

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | auto | Unique identifier within the feature, format `Q-{NNN}` (e.g., `Q-001`). Auto-generated, immutable. |
| `feature_id` | string | auto | The feature this question belongs to. |
| `phase` | enum | yes | Which phase generated this question: `"inception"` or `"planning"`. |
| `role` | enum | yes | Which role generated this question: `"pm"` or `"architect"`. |
| `question` | string | yes | The question text. 1–2000 characters. |
| `type` | enum | yes | One of `"clarification"`, `"decision"`, `"priority"`. |
| `options` | array of strings | optional | 0–10 suggested answers, each 1–500 characters. Defaults to `[]`. |
| `answer` | string or null | auto | `null` until a human responds. Max 5000 characters. Immutable once set. |
| `assumption` | string or null | auto | `null` until the timeout expires without a human response. Max 5000 characters. Immutable once set. |
| `status` | enum | auto | One of `"pending"`, `"answered"`, `"assumed"`. |
| `created_at` | timestamp (ISO 8601) | auto | Set on creation. Immutable. |
| `answered_at` | timestamp or null (ISO 8601) | auto | Set when the human responds or the timeout expires. `null` until then. |

### Question Status State Machine

```
pending → answered   (human provides an answer via PATCH)
pending → assumed    (timeout expires, auto-assumed)
answered → (terminal, no further transitions)
assumed → (terminal, no further transitions)
```

### Feature Summary Extension

The existing `GET /api/features` response now includes a `pending_questions_count` field on each feature summary:

| Field | Type | Description |
|---|---|---|
| `pending_questions_count` | int | Count of questions with `status: "pending"` for this feature. Always present; `0` when the feature has no pending questions. |

---

## Endpoints

### GET /api/features/{id}/questions

**Purpose**: Returns all questions for a feature (Spec 003, FR-003).

**Path parameters**:
- `id` (string, required): Feature ID.

**Response 200**:
```json
[
  {
    "id": "Q-001",
    "feature_id": "003-human-interaction-points",
    "phase": "inception",
    "role": "pm",
    "question": "What is the target audience for this feature?",
    "type": "clarification",
    "options": ["Internal developers", "External users", "Both"],
    "answer": null,
    "assumption": null,
    "status": "pending",
    "created_at": "2026-06-20T15:30:00Z",
    "answered_at": null
  }
]
```

**Empty state**: Returns `[]` (not `null`, not 404) when the feature has no questions.

**Response 404**:
```json
{"error": "not_found", "details": "Feature abc not found"}
```

---

### POST /api/features/{id}/questions

**Purpose**: Creates a new question for a feature (Spec 003, FR-003). Typically called by the pipeline after an agent dispatch detects a `questions.json` artifact, but also accessible via the API for testing and tooling.

**Path parameters**:
- `id` (string, required): Feature ID.

**Request body**:
```json
{
  "phase": "inception",
  "role": "pm",
  "question": "What is the target audience?",
  "type": "clarification",
  "options": ["Internal developers", "External users"]
}
```

| Field | Type | Required | Validation |
|---|---|---|---|
| `phase` | enum | yes | One of `"inception"`, `"planning"`. |
| `role` | enum | yes | One of `"pm"`, `"architect"`. |
| `question` | string | yes | 1–2000 characters. |
| `type` | enum | yes | One of `"clarification"`, `"decision"`, `"priority"`. |
| `options` | array of strings | optional | 0–10 items, each 1–500 characters. |

**Response 201**:
```json
{
  "id": "Q-001",
  "feature_id": "003-human-interaction-points",
  "phase": "inception",
  "role": "pm",
  "question": "What is the target audience?",
  "type": "clarification",
  "options": ["Internal developers", "External users"],
  "answer": null,
  "assumption": null,
  "status": "pending",
  "created_at": "2026-06-20T15:30:00Z",
  "answered_at": null
}
```

The `id` is auto-generated as `Q-{NNN}` (sequential within the feature). The `status` is set to `"pending"`, `created_at` is set to the current timestamp, and `answer`/`assumption`/`answered_at` are `null`.

**Response 400** (validation failure):
```json
{"error": "validation_error", "details": "question is required"}
```
Other validation messages include:
- `"phase must be one of: inception, planning"`
- `"role must be one of: pm, architect"`
- `"type must be one of: clarification, decision, priority"`
- `"question must be 1-2000 characters"`
- `"options must have at most 10 items"`
- `"option must be 1-500 characters"`

**Response 404** (feature not found):
```json
{"error": "not_found", "details": "Feature abc not found"}
```

---

### PATCH /api/features/{id}/questions/{questionId}

**Purpose**: Answers a pending question (Spec 003, FR-003, FR-012). Uses optimistic concurrency: if the question is no longer `"pending"`, returns 409 Conflict.

**Path parameters**:
- `id` (string, required): Feature ID.
- `questionId` (string, required): Question ID (e.g., `Q-001`).

**Request body**:
```json
{"answer": "I want option A"}
```

| Field | Type | Required | Validation |
|---|---|---|---|
| `answer` | string | yes | 1–5000 characters. |

**Response 200**:
```json
{
  "id": "Q-001",
  "feature_id": "003-human-interaction-points",
  "phase": "inception",
  "role": "pm",
  "question": "What is the target audience?",
  "type": "clarification",
  "options": ["Internal developers", "External users"],
  "answer": "I want option A",
  "assumption": null,
  "status": "answered",
  "created_at": "2026-06-20T15:30:00Z",
  "answered_at": "2026-06-20T15:45:00Z"
}
```

On success, `status` transitions `pending → answered`, `answer` is stored, and `answered_at` is set. The transition is terminal — no subsequent modifications are allowed.

**Response 400** (validation failure):
```json
{"error": "validation_error", "details": "answer must be 1-5000 characters"}
```

**Response 404** (feature or question not found):
```json
{"error": "not_found", "details": "Question Q-999 not found"}
```
Also returned when the feature is not found: `{"error": "not_found", "details": "Feature abc not found"}`.

**Response 409** (question already answered or assumed):
```json
{"error": "conflict", "details": "Question Q-001 is already answered"}
```
This is the concurrent-answer handling contract (Spec 003, FR-012): when two users answer the same question simultaneously, the first PATCH wins and the second receives 409.

---

### GET /api/features/{id}/questions/pending

**Purpose**: Returns only pending (unanswered) questions for a feature (Spec 003, FR-003).

**Path parameters**:
- `id` (string, required): Feature ID.

**Response 200**: Same shape as `GET /api/features/{id}/questions`, filtered to questions with `status: "pending"`. Returns `[]` when there are no pending questions (not `null`, not 404).

**Response 404**:
```json
{"error": "not_found", "details": "Feature abc not found"}
```

---

## Feature Status Extension

Spec 003 adds a new feature status value: `waiting_for_human`.

### Valid Transitions

| From | To | Condition |
|---|---|---|
| `in_progress` | `waiting_for_human` | Questions exist for the feature AND the current phase is inception or planning |
| `waiting_for_human` | `in_progress` | All questions answered by a human OR the timeout expires |
| `waiting_for_human` | `cancelled` | User cancels the feature |
| `waiting_for_human` | `recirculated` | User recirculates the feature (questions are cleared) |

### Invalid Transitions

| From | To | Reason |
|---|---|---|
| `waiting_for_human` | `waiting_for_human` | No self-transition |
| `waiting_for_human` | `passed` | Must return to `in_progress` first |
| `waiting_for_human` | `gate_blocked` | Must return to `in_progress` first |
| `in_progress` (construction, review, testing, delivery) | `waiting_for_human` | Only inception and planning support human interaction |
| `draft` | `waiting_for_human` | Feature must be started first |
| `done` | `waiting_for_human` | Feature is terminal |
| `cancelled` | `waiting_for_human` | Feature is terminal |

### Interaction with Existing Endpoints

- `POST /api/features/{id}/advance` on a feature in `waiting_for_human` status returns **400 Bad Request**:
  ```json
  {"error": "validation_error", "details": "Cannot advance feature in waiting_for_human status"}
  ```
- `POST /api/features/{id}/recirculate` on a feature in `waiting_for_human` status is valid; the feature transitions to `recirculated`, all questions for that feature are deleted, and the re-run may generate new questions with new IDs.
- `POST /api/features/{id}/cancel` on a feature in `waiting_for_human` status is valid; the feature transitions to `cancelled`.

---

## SSE Events

The pipeline broadcasts these new SSE events through the existing `GET /api/features/{id}/stream` endpoint (Spec 002):

| Event | Trigger | Payload |
|---|---|---|
| `waiting_for_human` | Feature transitions to `waiting_for_human` status | `{"feature_id": "...", "status": "waiting_for_human", "phase": "inception", "timestamp": "..."}` |
| `questions_answered` | All questions for a feature are answered by a human | `{"feature_id": "...", "status": "in_progress", "timestamp": "..."}` |
| `questions_assumed` | Timeout expires and unanswered questions are auto-assumed | `{"feature_id": "...", "status": "in_progress", "timestamp": "..."}` |

---

## Empty State Behavior

Per Spec 003, the following empty states are guaranteed (not `null`, not 404):

- `GET /api/features/{id}/questions` returns `[]` when the feature has no questions.
- `GET /api/features/{id}/questions/pending` returns `[]` when there are no pending questions.
- Question `options` is `[]` when no suggested options exist.
- Question `answer` is `null` (not `""`) until answered.
- Question `assumption` is `null` (not `""`) until assumed.
- The feature list badge is hidden (not shown with "0") when a feature has no pending questions.
- The feature detail page question section is hidden when the feature has no questions.

---

## Input Validation Summary

| Endpoint | Field | Rule |
|---|---|---|
| `POST /api/features/{id}/questions` | `phase` | required, enum `["inception", "planning"]` |
| `POST /api/features/{id}/questions` | `role` | required, enum `["pm", "architect"]` |
| `POST /api/features/{id}/questions` | `question` | required, 1–2000 chars |
| `POST /api/features/{id}/questions` | `type` | required, enum `["clarification", "decision", "priority"]` |
| `POST /api/features/{id}/questions` | `options` | optional, 0–10 items, each 1–500 chars |
| `PATCH /api/features/{id}/questions/{questionId}` | `answer` | required, 1–5000 chars |

---

## Error Response Format

All error responses follow the existing format from Spec 002:

```json
{"error": "error_code", "details": "human-readable message"}
```

Error codes used by Spec 003:
- `400` — `validation_error` (input validation failures)
- `404` — `not_found` (feature or question not found)
- `409` — `conflict` (question already answered or assumed)