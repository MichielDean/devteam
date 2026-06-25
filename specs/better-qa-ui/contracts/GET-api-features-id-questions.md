# API Contract: GET /api/features/{id}/questions

Backend unchanged (UI-only feature). Documents the read the wizard performs.

## Method & Path
`GET /api/features/{id}/questions`

## Path Parameters
- `id` (string, required): feature id

## Request
- No body. No query params.
- Headers: none required.

## Responses

### 200 OK
Returns `Question[]` (Go `QuestionsToResponse` — always a non-nil slice; **`[]` not `null`** for empty).
```json
[
  {
    "id": "q-1",
    "feature_id": "f-1",
    "phase": "inception",
    "role": "pm",
    "question": "Which option?",
    "type": "decision",
    "options": ["A","B","Other"],
    "answer": null,
    "assumption": null,
    "status": "pending",
    "created_at": "2026-06-24T12:00:00Z",
    "answered_at": null
  }
]
```
**Wizard use**: drives the entire wizard render — pending steps (options non-empty → option cards; options empty → textarea), answered/assumed history cards, progress count (`answered/total`), summary panel.

### 400 `validation_error`
- Missing `id` path value.
```json
{ "error": "validation_error", "details": "Feature ID is required" }
```

### 404 `not_found`
- Feature `id` not found.
```json
{ "error": "not_found", "details": "Feature <id> not found" }
```

### 500 `internal_error`
- Store failure.
```json
{ "error": "internal_error", "details": "Failed to list questions" }
```

## Examples

### Empty result (`[]` not null — FR-013 empty-state)
```json
[]
```
**Wizard behavior**: `questions.length === 0` → Questions section not rendered (AC-019). No `question-progress`, no `answer-summary`, no `submit-answers`.

### Mixed pending + answered + assumed
Returns array with all three statuses; the wizard splits them: pending → steps, answered/assumed → history cards (AC-004/005/020).