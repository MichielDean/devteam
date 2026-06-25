# API Contract: PATCH /api/features/{id}/questions/{questionId}

The backend endpoint is unchanged (UI-only feature). This contract documents the existing behavior the wizard depends on.

## Method & Path
`PATCH /api/features/{id}/questions/{questionId}`

## Path Parameters
- `id` (string, required): feature id (UUID)
- `questionId` (string, required): question id (UUID)

## Request
- **Headers**: `Content-Type: application/json`
- **Body**: `AnswerQuestionRequest` (`ui/src/types/index.ts:255`)
  ```json
  { "answer": "string (1–5000 chars after trim)" }
  ```
- **Body limit**: 1 MiB (server `MaxBytesReader`).

## Responses

### 200 OK — answered
Returns the updated `Question` (`QuestionToResponse`).
```json
{
  "id": "q-uuid",
  "feature_id": "f-uuid",
  "phase": "inception",
  "role": "pm",
  "question": "Which option?",
  "type": "decision",
  "options": ["A","B","Other"],
  "answer": "B",
  "assumption": null,
  "status": "answered",
  "created_at": "2026-06-24T12:00:00Z",
  "answered_at": "2026-06-24T12:05:00Z"
}
```
**Side effects** (server):
- Broadcasts SSE `question_answered` `{feature_id, question_id, status:"answered"}`.
- Goroutine checks `PendingCount`; if 0 and feature `status == waiting_for_human`:
  - `single-phase` mode → clears `activeProcess`, sets feature status to `in_progress` (user advances manually). No agent dispatch.
  - otherwise (`autopilot`) → sets status to `in_progress` and auto-resumes the pipeline.

### 400 `validation_error`
- Missing `id` or `questionId` path value.
- Invalid JSON body.
- `answer` empty after `TrimSpace`, or `len(req.Answer) > 5000`.
```json
{ "error": "validation_error", "details": "answer must be 1-5000 characters" }
```
**Wizard behavior (CON-010 / AC-016)**: toast shows the `details` message; wizard stays on the current step; no navigation.

### 404 `not_found`
- Feature `id` not found, OR question `questionId` not found / not owned by feature.
```json
{ "error": "not_found", "details": "Feature <id> not found" }
```
**Wizard behavior (CON-012 / AC-018)**: toast says the question was not found.

### 409 `conflict`
- Question already `answered` or `assumed` (`feature.QuestionConflictError`).
```json
{ "error": "conflict", "details": "<err.Error()>" }
```
**Wizard behavior (CON-011 / AC-017)**: toast says the question is already answered. In batch submit, a 409 for a question answered by SSE between draft and submit is toasted but does NOT abort the remaining PATCHes (per data-model integrity rule).

### 500 `internal_error`
- Store/lookup failure.
```json
{ "error": "internal_error", "details": "Failed to answer question" }
```
**Wizard behavior (FR-010)**: toast shows the backend message.

## Examples

### Submit one answer (multiple-choice)
```http
PATCH /api/features/f-1/questions/q-1
Content-Type: application/json

{"answer":"B"}
```
→ 200, `status:"answered"`, SSE `question_answered` fires. If this was the last pending question and feature is in autopilot, pipeline auto-resumes.

### Empty answer
```http
PATCH /api/features/f-1/questions/q-1
Content-Type: application/json

{"answer":"   "}
```
→ 400 `validation_error` `details:"answer must be 1-5000 characters"`.

### Re-answer
```http
PATCH /api/features/f-1/questions/q-1
Content-Type: application/json

{"answer":"C"}
```
→ 409 `conflict` (question already answered).