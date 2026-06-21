# API Documentation — Spec 003: Human Interaction Points

This documents every endpoint added by Spec 003: Human Interaction Points. All endpoints operate on the `Question` artifact and the `waiting_for_human` feature status defined in the spec.

Terminology is taken from `spec.md` (Spec 003). The `Question`, `waiting_for_human` status, `pending`/`answered`/`assumed` question statuses, and the `inception`/`planning` phases all refer to the entities defined in the spec.

All endpoints are served by the Dev Team web server (default address configurable via the `-http` flag). All responses use `application/json`. Error responses use the existing Dev Team error envelope:

```json
{ "error": "<code>", "details": "<human-readable message>" }
```

---

## Question Resource

A `Question` represents a clarification, decision, or priority question surfaced by an agent (PM or Architect) during the `inception` or `planning` phase for human input.

### Question object

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier within the feature, format `Q-{NNN}` (e.g., `Q-001`). Auto-generated, immutable. |
| `feature_id` | string | The feature this question belongs to. |
| `phase` | enum | Phase that generated the question: `inception` or `planning`. |
| `role` | enum | Role that generated the question: `pm` or `architect`. |
| `question` | string | The question text. 1–2000 characters. |
| `type` | enum | Question type: `clarification`, `decision`, or `priority`. |
| `options` | array of strings | Suggested answers. 0–10 items, each 1–500 characters. `[]` when no options exist (never `null`). |
| `answer` | string or null | The human's answer. `null` until answered. Max 5000 characters. |
| `assumption` | string or null | The auto-generated assumption when timeout expires. `null` until assumed. Max 5000 characters. |
| `status` | enum | Question status: `pending`, `answered`, or `assumed`. |
| `created_at` | timestamp (ISO 8601) | Set on creation, immutable. |
| `answered_at` | timestamp or null (ISO 8601) | Set when the human answers or when the timeout expires and the question is assumed. `null` until then. |

### Question status state machine

- `pending` → `answered`: human provides an answer via PATCH
- `pending` → `assumed`: timeout expires without a human response
- `answered` → (terminal): cannot be changed
- `assumed` → (terminal): cannot be changed

---

## GET /api/features/{id}/questions

**Purpose**: Returns all questions for a feature. (Spec FR-003, US-001, US-002)

**Path parameters**:
- `id` (string, required): the feature ID

**Response 200**: array of Question objects. Empty state returns `[]` (not `null`, not 404).

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
    "created_at": "2026-06-20T15:30:00-06:00",
    "answered_at": null
  }
]
```

**Response 404**: feature not found.

```json
{ "error": "not_found", "details": "Feature abc not found" }
```

---

## GET /api/features/{id}/questions/pending

**Purpose**: Returns only questions with `status: "pending"` for a feature. (Spec FR-003, US-001)

**Path parameters**:
- `id` (string, required): the feature ID

**Response 200**: array of Question objects filtered to `status: "pending"`. Empty state returns `[]` (not `null`, not 404).

**Response 404**: feature not found.

```json
{ "error": "not_found", "details": "Feature abc not found" }
```

---

## POST /api/features/{id}/questions

**Purpose**: Creates a new question for a feature. The question is stored with an auto-generated `id` (format `Q-{NNN}`), `status: "pending"`, and `created_at` set. (Spec FR-003, US-005)

**Path parameters**:
- `id` (string, required): the feature ID

**Request body**:

| Field | Type | Required | Validation |
|---|---|---|---|
| `phase` | enum | required | one of `inception`, `planning` |
| `role` | enum | required | one of `pm`, `architect` |
| `question` | string | required | 1–2000 characters |
| `type` | enum | required | one of `clarification`, `decision`, `priority` |
| `options` | array of strings | optional | 0–10 items, each 1–500 characters |

```json
{
  "phase": "inception",
  "role": "pm",
  "question": "What is the target audience?",
  "type": "clarification",
  "options": ["Internal developers", "External users"]
}
```

**Response 201**: the created Question object with server-generated `id`, `status: "pending"`, and `created_at` set.

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
  "created_at": "2026-06-20T15:30:00-06:00",
  "answered_at": null
}
```

**Response 400**: invalid request body.

| Condition | `details` |
|---|---|
| Request body is not valid JSON | `Invalid JSON body` |
| Missing `question` field (or empty/whitespace) | `question is required` |
| `question` exceeds 2000 characters | `question must be 1-2000 characters` |
| Invalid `phase` value | `phase must be one of: inception, planning` |
| Invalid `role` value | `role must be one of: pm, architect` |
| Invalid `type` value | `type must be one of: clarification, decision, priority` |
| More than 10 `options` | `options must have at most 10 items` |

```json
{ "error": "validation_error", "details": "question is required" }
```

**Response 404**: feature not found.

```json
{ "error": "not_found", "details": "Feature abc not found" }
```

---

## PATCH /api/features/{id}/questions/{questionId}

**Purpose**: Answers a pending question. Sets `status` to `answered`, stores the `answer`, and sets `answered_at`. Uses optimistic concurrency: if the question is no longer `pending` when the update is attempted, returns 409. (Spec FR-003, FR-012, US-001, US-002)

**Path parameters**:
- `id` (string, required): the feature ID
- `questionId` (string, required): the question ID (e.g., `Q-001`)

**Request body**:

| Field | Type | Required | Validation |
|---|---|---|---|
| `answer` | string | required | 1–5000 characters (empty string rejected) |

```json
{ "answer": "I want option A" }
```

**Response 200**: the updated Question object.

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
  "created_at": "2026-06-20T15:30:00-06:00",
  "answered_at": "2026-06-20T15:45:00-06:00"
}
```

**Response 400**: invalid request body.

| Condition | `details` |
|---|---|
| Missing `answer` field, empty string, or whitespace-only | `answer must be 1-5000 characters` |
| Answer exceeds 5000 characters | `answer must be 1-5000 characters` |
| Request body is not valid JSON | `Invalid JSON body` |

```json
{ "error": "validation_error", "details": "answer must be 1-5000 characters" }
```

**Response 404**: feature not found or question not found.

```json
{ "error": "not_found", "details": "Question Q-999 not found" }
```

**Response 409**: question is already answered or already assumed (status is not `pending`). The first answer wins; subsequent attempts receive this conflict.

```json
{ "error": "conflict", "details": "Question Q-001 is already answered" }
```

---

## Feature Summary Extension

The existing `GET /api/features` and `GET /api/features/{id}` responses now include a `pending_questions_count` field on each feature summary.

| Field | Type | Description |
|---|---|---|
| `pending_questions_count` | integer | Count of questions with `status: "pending"` for the feature. Always present; `0` when the feature has no pending questions. |

```json
{
  "id": "003-human-interaction-points",
  "title": "Human Interaction Points",
  "status": "waiting_for_human",
  "priority": 1,
  "current_phase": "inception",
  "updated_at": "2026-06-20T15:30:00-06:00",
  "pending_questions_count": 3
}
```

The `status` field on feature summaries can now also be `waiting_for_human` (see "Feature status extension" below).

---

## Feature Status Extension

A new feature status value is introduced: `waiting_for_human`.

Valid transitions:

| From | To | Condition |
|---|---|---|
| `in_progress` | `waiting_for_human` | Questions exist for the feature and the feature is in `inception` or `planning` phase |
| `waiting_for_human` | `in_progress` | All questions answered, or timeout expires |
| `waiting_for_human` | `cancelled` | User cancels the feature |
| `waiting_for_human` | `recirculated` | User recirculates the feature (questions are cleared) |

Invalid transitions:

| From | To | Reason |
|---|---|---|
| `waiting_for_human` | `waiting_for_human` | No self-transition |
| `waiting_for_human` | `passed` | Must return to `in_progress` first |
| `waiting_for_human` | `gate_blocked` | Must return to `in_progress` first |
| `in_progress` (construction, review, testing, delivery) | `waiting_for_human` | Only `inception` and `planning` support human interaction |
| `draft` | `waiting_for_human` | Feature must be started first |
| `done` | `waiting_for_human` | Feature is terminal |
| `cancelled` | `waiting_for_human` | Feature is terminal |

### POST /api/features/{id}/advance — new rejection

Advancing a feature that is in `waiting_for_human` status is rejected. (Spec FR-008, AC-019)

**Response 400**:

```json
{ "error": "validation_error", "details": "Cannot advance feature in waiting_for_human status" }
```

---

## Server-Sent Events

The pipeline broadcasts SSE events for human-interaction status changes on the existing `GET /api/features/{id}/stream` endpoint. The following new event types are emitted:

| Event type | When emitted |
|---|---|
| `waiting_for_human` | Feature transitions to `waiting_for_human` after question detection |
| `questions_answered` | All pending questions for a feature have been answered and the feature resumes to `in_progress` |
| `questions_assumed` | Timeout expired; pending questions were assumed and the feature resumes to `in_progress` |

Clients subscribed to the feature stream receive these events and can refresh the question UI accordingly.

---

## Concurrency

The PATCH answer endpoint uses optimistic concurrency. If two users answer the same `pending` question simultaneously, the first request wins (200) and the second receives 409 Conflict. (Spec FR-012, AC-086)

---

## Error Envelope Reference

All error responses use the standard Dev Team envelope:

```json
{ "error": "<code>", "details": "<human-readable message>" }
```

Error codes used by the question endpoints:

| Code | HTTP status | Meaning |
|---|---|---|
| `validation_error` | 400 | Request body failed validation (missing/invalid fields) |
| `not_found` | 404 | Feature or question does not exist |
| `conflict` | 409 | Question is no longer pending (already answered or assumed) |