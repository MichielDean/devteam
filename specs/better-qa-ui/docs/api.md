# API Documentation — Better Q&A UI (Spec: better-qa-ui)

This feature is **UI-only** (CON-014). No backend endpoints were added or modified. The wizard interacts with two existing endpoints, documented below using spec terminology. Contract sources: `specs/better-qa-ui/contracts/`.

Terminology note: terms used here (`Question`, `Feature`, `waiting_for_human`, `phase`, `role`, `options`, `answer`, `assumption`, `status`, `pending`, `answered`, `assumed`, `autopilot`, `single-phase`) are defined in `spec.md` Key Entities and Constraint Register.

---

## [GET] /api/features/{id}/questions

**Purpose**: List all Questions for one Feature. The wizard reads this to render pending steps, answered/assumed history cards, the progress indicator, and the answer summary.

**Path Parameters**:
- `id` (string, required): Feature id (UUID)

**Request**:
- No body. No query parameters. No required headers.

**Response 200**:
- Returns `Question[]`. The backend guarantees a non-nil slice — **`[]` not `null`** for empty (preserved by this feature).
- Each `Question` has the frozen shape (CON-014):
  - `id` (string): question id
  - `feature_id` (string): owning Feature id
  - `phase` (enum `inception` | `planning`): asking phase (CON-004)
  - `role` (enum `pm` | `architect`): asking role (CON-004)
  - `question` (string): question text
  - `type` (enum `clarification` | `decision` | `priority`): display-only badge; **does NOT drive render dispatch** (CON-003)
  - `options` (string[]): non-empty → multiple-choice wizard step; empty → open-ended textarea step (CON-001/002/003)
  - `answer` (string | null): the chosen answer once answered
  - `assumption` (string | null): the auto-assumed answer text once assumed
  - `status` (enum `pending` | `answered` | `assumed`): lifecycle
  - `created_at` (timestamp)
  - `answered_at` (timestamp | null)

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

**Response 400 `validation_error`**:
- `error` (string): `"validation_error"`
- `details` (string): `"Feature ID is required"` — missing `id` path value

**Response 404 `not_found`**:
- `error`: `"not_found"`
- `details`: `"Feature <id> not found"`

**Response 500 `internal_error`**:
- `error`: `"internal_error"`
- `details`: `"Failed to list questions"`

**Wizard behavior**: `options.length === 0` → open-ended textarea step (AC-014); `options.length > 0` → selectable option cards (AC-001). An empty array result (`[]`) hides the Questions section entirely (AC-019 / FR-013).

---

## [PATCH] /api/features/{id}/questions/{questionId}

**Purpose**: Answer one Question. The wizard's single "Submit Answers & Resume" button sends one PATCH per Question in the draft, sequentially (CON-008). The backend's existing resume side-effect fires on the final PATCH.

**Path Parameters**:
- `id` (string, required): Feature id (UUID)
- `questionId` (string, required): Question id (UUID)

**Request**:
- Headers: `Content-Type: application/json`
- Body limit: 1 MiB
- `answer` (string, required): 1–5000 characters after trim (CON-010). The wizard trims and blocks empty drafts client-side (defense-in-depth); the backend remains the authority.

```json
{ "answer": "B" }
```

**Response 200**:
- Returns the updated `Question` with `status: "answered"`, `answer` set, `answered_at` set.
- **Server side effects** (CON-009):
  - Broadcasts SSE `question_answered` `{feature_id, question_id, status:"answered"}`.
  - Goroutine checks `PendingCount`. If 0 and Feature `status == waiting_for_human`:
    - `single-phase` mode → clears active process, sets Feature status to `in_progress` (user advances manually). **No agent dispatch.** (AC-013)
    - `autopilot` mode → sets status to `in_progress` and **auto-resumes** the pipeline (AC-012).

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

**Response 400 `validation_error`** (CON-010 / AC-016 / AC-CON-004):
- Missing `id` or `questionId` path value, invalid JSON, or `answer` empty after trim / longer than 5000 chars.
- `error`: `"validation_error"`
- `details`: `"answer must be 1-5000 characters"`
- **Wizard behavior**: toast shows the `details` message; wizard stays on the current step; no navigation.

**Response 404 `not_found`** (CON-012 / AC-018):
- Feature `id` not found, or Question `questionId` not found / not owned by the Feature.
- `error`: `"not_found"`
- `details`: `"Feature <id> not found"` (or question not found)
- **Wizard behavior**: toast says the question was not found.

**Response 409 `conflict`** (CON-011 / AC-017):
- Question already `answered` or `assumed`.
- `error`: `"conflict"`
- `details`: the backend error message
- **Wizard behavior**: toast says the question is already answered. In a batch submit, a 409 for one Question (e.g. answered by an SSE `question_answered` event between draft and submit) is toasted but does **NOT** abort the remaining PATCHes (data-model integrity rule).

**Response 500 `internal_error`** (FR-010):
- Store/lookup failure.
- `error`: `"internal_error"`
- `details`: `"Failed to answer question"`
- **Wizard behavior**: toast shows the backend message.

**Examples**:

Submit one multiple-choice answer (autopilot, last pending):
```http
PATCH /api/features/f-1/questions/q-1
Content-Type: application/json

{"answer":"B"}
```
→ 200, `status:"answered"`, SSE `question_answered` fires, pipeline auto-resumes.

Empty answer:
```http
PATCH /api/features/f-1/questions/q-1
Content-Type: application/json

{"answer":"   "}
```
→ 400 `validation_error`, `details:"answer must be 1-5000 characters"`.

Re-answer an already-answered Question:
```http
PATCH /api/features/f-1/questions/q-1
Content-Type: application/json

{"answer":"C"}
```
→ 409 `conflict`.