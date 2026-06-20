# Dev Team Web UI — API Reference

**Spec**: 002-dev-team-web-ui
**Version**: 1.0.0
**Last Updated**: 2026-06-20

---

## Base URL

```
http://localhost:8080/api
```

The server listens on `localhost` by default. The port is configurable via the `-http` flag (e.g., `-http :8080`).

All API endpoints are prefixed with `/api/`. The root path `/` serves the SPA static files.

---

## Authentication

None. The server operates in single-user local mode. No authentication or authorization is required.

---

## Common Headers

### Request Headers

| Header | Value |
|--------|-------|
| `Content-Type` | `application/json` (for POST/PATCH requests with JSON bodies) |

### Response Headers

All API responses include the following security headers:

| Header | Value |
|--------|-------|
| `Content-Security-Policy` | `default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'` |
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |

No `Strict-Transport-Security` header is set because the server is local-only (no TLS by default).

---

## Error Response Format

All error responses follow this structure:

```json
{
  "error": "error_code",
  "details": "Human-readable message"
}
```

### Error Codes

| HTTP Status | Error Code | Description |
|-------------|-----------|-------------|
| 400 | `validation_error` | General validation error |
| 400 | `invalid_phase` | Invalid or forward phase for recirculation |
| 400 | `invalid_priority` | Priority must be 1, 2, or 3 |
| 400 | `empty_title` | Title is required and cannot be empty |
| 400 | `empty_description` | Description is required for loose idea |
| 400 | `title_too_long` | Title exceeds 200 characters |
| 400 | `description_too_long` | Description exceeds 10,000 characters |
| 404 | `feature_not_found` | Feature ID does not exist |
| 404 | `artifact_not_found` | Artifact has not been generated yet |
| 409 | `duplicate_title` | A feature with a similar title already exists |
| 409 | `already_processing` | Feature is already being processed |
| 500 | `internal_error` | Unexpected server error |

---

## Endpoints

### List Features

```
GET /api/features
```

Returns all features with their current phase, status, and priority.

**Response**: `200 OK`

```json
{
  "features": [
    {
      "id": "001-dev-team-platform",
      "title": "Dev Team Platform",
      "status": "in_progress",
      "priority": 1,
      "current_phase": "planning",
      "updated_at": "2026-06-20T10:30:00Z",
      "gate_result": null
    }
  ]
}
```

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Feature identifier (slug-based) |
| `title` | string | Feature title |
| `status` | string | Current status: `in_progress`, `passed`, `failed`, `cancelled`, `done` |
| `priority` | integer | Priority level: 1 (High), 2 (Medium), 3 (Low) |
| `current_phase` | string | Current pipeline phase |
| `updated_at` | string (ISO 8601) | Last update timestamp |
| `gate_result` | object or null | Most recent gate result, if evaluated |

---

### Create Feature

```
POST /api/features
```

Creates a new feature via loose idea or external spec intake.

**Request (Loose Idea)**:

```json
{
  "type": "loose_idea",
  "title": "We need dark mode",
  "description": "Add dark mode support to the dashboard for better UX in low-light environments",
  "priority": 1
}
```

**Request (External Spec)**:

```json
{
  "type": "external_spec",
  "title": "External PRD",
  "description": "PRD from product team",
  "priority": 2,
  "file_content": "base64-encoded-file-content"
}
```

**Validation Rules**:

| Field | Loose Idea | External Spec |
|-------|-----------|---------------|
| `type` | Required: `"loose_idea"` | Required: `"external_spec"` |
| `title` | Required, max 200 chars | Required, max 200 chars |
| `description` | Required, max 10,000 chars | Required, max 10,000 chars |
| `priority` | Optional, 1–3, default 2 | Optional, 1–3, default 2 |
| `file_content` | Not applicable | Required, base64-encoded |

**Response**: `201 Created` with `FeatureDetailResponse` body

**Error Responses**:

| Status | Code | Condition |
|--------|------|-----------|
| 400 | `empty_title` | Title is empty or whitespace |
| 400 | `empty_description` | Description is empty or whitespace |
| 400 | `title_too_long` | Title exceeds 200 characters |
| 400 | `description_too_long` | Description exceeds 10,000 characters |
| 400 | `invalid_priority` | Priority is not 1, 2, or 3 |
| 409 | `duplicate_title` | A feature with a similar title already exists |

**Important**: Creating a feature does **not** automatically start processing. The feature is created with status `in_progress` and phase `inception`. The user must explicitly click "Run Phase" or "Process" to dispatch the PM agent.

---

### Get Feature Detail

```
GET /api/features/:id
```

Returns the full feature detail including all phase states, artifacts, and gate results.

**Response**: `200 OK`

```json
{
  "id": "001-dev-team-platform",
  "title": "Dev Team Platform",
  "status": "in_progress",
  "priority": 1,
  "intake_path": "loose_idea",
  "created_at": "2026-06-19T00:00:00Z",
  "updated_at": "2026-06-20T10:30:00Z",
  "phase_states": {
    "inception": {
      "phase": "inception",
      "status": "passed",
      "started_at": "2026-06-19T00:00:00Z",
      "completed_at": "2026-06-19T01:00:00Z",
      "artifacts": [
        {
          "type": "spec_md",
          "path": "specs/001-dev-team-platform/spec.md",
          "generated_by": "pm",
          "generated_at": "2026-06-19T01:00:00Z"
        }
      ],
      "gate_result": {
        "phase": "inception",
        "passed": true,
        "checks": [
          {"name": "spec.md exists", "passed": true, "message": "Found spec.md"},
          {"name": "acceptance.md exists", "passed": true, "message": "Found acceptance.md"}
        ]
      }
    },
    "planning": {
      "phase": "planning",
      "status": "in_progress",
      "started_at": "2026-06-20T10:00:00Z",
      "artifacts": [],
      "gate_result": null
    }
  },
  "dependencies": [],
  "repos": [
    {"name": "devteam", "url": "git@github.com:MichielDean/devteam.git", "branch": "main"}
  ]
}
```

**Error Responses**:

| Status | Code | Condition |
|--------|------|-----------|
| 404 | `feature_not_found` | Feature ID does not exist |

---

### Run Phase

```
POST /api/features/:id/run
```

Dispatches the agent for the feature's current phase.

**Response**: `200 OK` with `FeatureDetailResponse` body (updated with phase run results)

**Error Responses**:

| Status | Code | Condition |
|--------|------|-----------|
| 404 | `feature_not_found` | Feature ID does not exist |
| 409 | `already_processing` | Feature is already being processed |

---

### Advance Feature

```
POST /api/features/:id/advance
```

Advances the feature to the next pipeline phase. Only valid when the current phase's gate has passed.

**Response**: `200 OK` with `FeatureDetailResponse` body

**Error Responses**:

| Status | Code | Condition |
|--------|------|-----------|
| 400 | `validation_error` | Gate has not passed, feature is at the delivery phase, or feature is in a terminal state |
| 404 | `feature_not_found` | Feature ID does not exist |

---

### Recirculate Feature

```
POST /api/features/:id/recirculate
```

Sends the feature back to an earlier phase.

**Request**:

```json
{
  "target_phase": "planning"
}
```

The `target_phase` must be a valid phase (`inception`, `planning`, `construction`, `review`, `testing`, `delivery`) that is earlier than the feature's current phase.

**Response**: `200 OK` with `FeatureDetailResponse` body

**Error Responses**:

| Status | Code | Condition |
|--------|------|-----------|
| 400 | `invalid_phase` | Target phase is invalid, not earlier than current, or feature is in a terminal state |
| 404 | `feature_not_found` | Feature ID does not exist |

---

### Cancel Feature

```
POST /api/features/:id/cancel
```

Cancels the feature. Only valid for features that are not already in a terminal state (cancelled or done).

**Response**: `200 OK` with `FeatureDetailResponse` body (status: `"cancelled"`)

**Error Responses**:

| Status | Code | Condition |
|--------|------|-----------|
| 400 | `validation_error` | Feature is already cancelled or already done |
| 404 | `feature_not_found` | Feature ID does not exist |

---

### Process Feature

```
POST /api/features/:id/process
```

Starts autonomous processing for the feature. The pipeline runs all phases automatically, from the current phase to delivery. Progress is streamed via the SSE endpoint.

**Response**: `200 OK` with `FeatureDetailResponse` body

**Error Responses**:

| Status | Code | Condition |
|--------|------|-----------|
| 409 | `already_processing` | Feature is already being processed |
| 404 | `feature_not_found` | Feature ID does not exist |

Processing runs in a background goroutine. Real-time progress updates are available via `GET /api/features/:id/stream`.

---

### Evaluate Gate

```
GET /api/features/:id/gate
```

Evaluates the current phase's gate and returns the result.

**Response**: `200 OK`

```json
{
  "phase": "inception",
  "passed": true,
  "checks": [
    {"name": "spec.md exists", "passed": true, "message": "Found spec.md"},
    {"name": "acceptance.md exists", "passed": true, "message": "Found acceptance.md"}
  ]
}
```

**Error Responses**:

| Status | Code | Condition |
|--------|------|-----------|
| 404 | `feature_not_found` | Feature ID does not exist |

---

### Get Artifact

```
GET /api/features/:id/artifacts/:type
```

Returns the content of a specific artifact as plain text (markdown).

**Supported Types**: `input`, `spec`, `acceptance`, `repos`, `plan`, `tasks`, `review_report`, `test_report`, `docs`

**Response**: `200 OK` with `Content-Type: text/plain; charset=utf-8`

**Error Responses**:

| Status | Code | Condition |
|--------|------|-----------|
| 404 | `artifact_not_found` | Artifact has not been generated yet |

---

### SSE Stream

```
GET /api/features/:id/stream
```

Opens a Server-Sent Events (SSE) connection for real-time processing progress updates.

**Response**: `200 OK` with `Content-Type: text/event-stream`

**Event Types**:

| Event | Description | Data Fields |
|-------|-------------|-------------|
| `phase_change` | Feature moved to a new phase | `feature_id`, `phase`, `status`, `timestamp` |
| `gate_result` | Gate evaluation completed | `feature_id`, `phase`, `passed`, `checks` |
| `agent_dispatch` | Agent dispatched for a role | `feature_id`, `phase`, `role`, `status`, `timestamp` |
| `agent_complete` | Agent finished execution | `feature_id`, `phase`, `role`, `status`, `duration_ms` |
| `processing_complete` | Autonomous processing finished | `feature_id`, `status`, `timestamp` |
| `error` | An error occurred during processing | `feature_id`, `message`, `timestamp` |

**Example Events**:

```
event: phase_change
data: {"feature_id":"001-dev-team-platform","phase":"planning","status":"in_progress","timestamp":"2026-06-20T10:00:00Z"}

event: gate_result
data: {"feature_id":"001-dev-team-platform","phase":"inception","passed":true,"checks":[...]}

event: agent_dispatch
data: {"feature_id":"001-dev-team-platform","phase":"inception","role":"pm","status":"dispatched","timestamp":"2026-06-19T00:05:00Z"}

event: agent_complete
data: {"feature_id":"001-dev-team-platform","phase":"inception","role":"pm","status":"success","duration_ms":120000}

event: processing_complete
data: {"feature_id":"001-dev-team-platform","status":"done","timestamp":"2026-06-20T12:00:00Z"}

event: error
data: {"feature_id":"001-dev-team-platform","message":"Agent dispatch failed: timeout","timestamp":"2026-06-20T10:00:00Z"}
```

**Connection Behavior**:

- The connection stays open until processing completes or the client disconnects.
- Keep-alive comments are sent every 30 seconds to prevent proxy timeouts.
- Multiple concurrent clients can connect to the same feature stream and receive the same events.
- Clients auto-reconnect via the `EventSource` API on disconnect.

---

## Pipeline Phases

The Dev Team pipeline has six phases, processed in order:

| Phase | Description |
|-------|-------------|
| `inception` | Requirements gathering, specification, and acceptance criteria |
| `planning` | Implementation planning and task breakdown |
| `construction` | Code generation and implementation |
| `review` | Code review and quality assurance |
| `testing` | Testing and verification |
| `delivery` | Documentation, deployment, and release |

Valid recirculation targets must be earlier than the current phase. For example, a feature in `review` can recirculate to `inception`, `planning`, or `construction`, but not to `testing` or `delivery`.

---

## Artifact Types

| API Path (`:type`) | File on Disk | Description |
|---------------------|-------------|-------------|
| `input` | `specs/<id>/input.md` | Original idea or external spec |
| `spec` | `specs/<id>/spec.md` | Feature specification |
| `acceptance` | `specs/<id>/acceptance.md` | Acceptance criteria |
| `repos` | `specs/<id>/repos.yaml` | Repository scope |
| `plan` | `specs/<id>/plan.md` | Implementation plan |
| `tasks` | `specs/<id>/tasks.md` | Task breakdown |
| `review_report` | `specs/<id>/review-report.md` | Code review report |
| `test_report` | `specs/<id>/test-report.md` | Test results |
| `docs` | `specs/<id>/docs/` | Generated documentation |

---

## Security Considerations

- The API does not expose secrets, agent prompts, or internal file paths — only feature state and artifact content.
- All input is validated: title length, description length, priority range, phase names.
- Request body size is limited to 1MB.
- Panic recovery middleware catches unexpected errors and returns a generic 500 response.
- CORS is configured for local development (`Access-Control-Allow-Origin: *`).
- The server listens on `localhost` by default and does not use TLS.