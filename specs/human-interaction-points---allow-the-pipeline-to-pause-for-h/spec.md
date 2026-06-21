# Spec 003: Human Interaction Points

## Priority
P1

## Feature Description

The Dev Team pipeline currently runs fully autonomously — each phase dispatches an agent, the agent produces artifacts, and the gate evaluator checks them. But the inception and planning phases benefit from human input: requirements clarification, architectural decisions, scope boundaries.

This feature adds the ability for the pipeline to pause at decision points during inception and planning, surface questions to a human through the web UI, and incorporate their answers back into the agent context. When no human responds within a configurable timeout, the pipeline falls back to autonomous mode with documented assumptions.

## User Stories

### US-001: Human Answers PM Clarification Questions
Priority: P1

As a product owner, I want to answer the PM's clarification questions through the web UI so that the spec reflects my actual requirements instead of assumptions.

Happy path: PM agent surfaces questions during inception, feature enters `waiting_for_human` status, human sees questions on the feature detail page, answers them, pipeline resumes with answers in context.

Error path: Question already answered (409), question not found (404), invalid answer data (400).

Empty state: Feature has no questions — question section is hidden.

### US-002: Human Reviews Architect Design Decisions
Priority: P1

As a product owner, I want to review and approve key design decisions from the Architect through the web UI so that the plan aligns with my technical preferences.

Happy path: Architect surfaces design decisions as questions during planning, human reviews and responds, pipeline incorporates answers.

Error path: Same error paths as US-001.

Empty state: No design decisions surfaced — question section is hidden.

### US-003: Pipeline Pauses for Human Input
Priority: P1

As a product owner, I want the pipeline to pause at decision points when human input is available, so that I can provide direction instead of having the agent make assumptions.

Happy path: After PM or Architect dispatch, if questions were generated, pipeline sets status to `waiting_for_human` and waits for responses.

Error path: Feature not in a valid phase for human interaction (inception or planning) — questions are still stored but feature does not enter `waiting_for_human`.

Empty state: No questions generated — pipeline proceeds normally without pausing.

### US-004: Pipeline Falls Back to Autonomous Mode
Priority: P1

As a product owner, I want the pipeline to automatically proceed with documented assumptions if I don't respond within a timeout, so that work doesn't stall indefinitely.

Happy path: Timeout expires, unanswered questions get assumptions, feature returns to `in_progress`, pipeline proceeds.

Error path: [ASSUMPTION: timeout mechanism is reliable — if it fails, pipeline logs error and keeps feature in `waiting_for_human` requiring manual intervention]

Empty state: All questions answered before timeout — timeout mechanism never triggers.

### US-005: Agent Creates Questions During Dispatch
Priority: P2

As a pipeline operator, I want agents (PM and Architect) to create questions during their dispatch so that they can surface ambiguities and decisions that need human input.

Happy path: Agent writes questions as part of its output, pipeline detects them and stores them as question artifacts.

Error path: Agent output has invalid question format — questions are ignored, pipeline logs a warning.

Empty state: Agent produces no questions — pipeline proceeds normally.

### US-006: Feature List Shows Question Badge
Priority: P2

As a product owner, I want to see at a glance which features have pending questions so that I can prioritize my attention.

Happy path: Feature list shows badge with count of pending questions on features in `waiting_for_human` status.

Error path: Feature list API returns error — list still renders, badge not shown.

Empty state: No features have pending questions — no badges shown, list renders normally.

## Functional Requirements

### FR-001: Feature Status "Waiting for Human"
Source: US-003

The feature state machine must support a new status: `waiting_for_human`. When the pipeline reaches a decision point and human input is requested, the feature transitions to this status.

Valid transitions:
- TO `waiting_for_human`: from `in_progress` (only when the feature is in inception or planning phase)
- FROM `waiting_for_human`: back to `in_progress` (when all questions are answered or when timeout expires)

Invalid transitions:
- `waiting_for_human` from construction, review, testing, or delivery phases
- `waiting_for_human` from `draft`, `gate_blocked`, `passed`, `failed`, `done`, `cancelled`, or `recirculated` statuses
- `waiting_for_human` to `waiting_for_human` (no self-transition)

### FR-002: Question Model
Source: US-001, US-002

A new artifact type `questions_json` stores clarification questions that the PM or Architect surfaces for human input.

Each question has:
- `id`: string, unique identifier within the feature, format `Q-{sequence}` (e.g., "Q-001", "Q-002"), auto-generated, immutable
- `feature_id`: string, the feature this question belongs to
- `phase`: enum, which phase generated this question — one of: "inception", "planning"
- `role`: enum, which role generated this question — one of: "pm", "architect"
- `question`: string, required, the question text, 1-2000 characters
- `type`: enum, required — one of: "clarification", "decision", "priority"
- `options`: array of strings, optional, 0-10 suggested answers, each option 1-500 characters
- `answer`: string or null, null until human responds, max 5000 characters
- `assumption`: string or null, null until timeout expires, then filled with the agent's assumption, max 5000 characters
- `status`: enum, required — one of: "pending", "answered", "assumed"
- `created_at`: timestamp, ISO 8601, auto-set on creation, immutable
- `answered_at`: timestamp or null, ISO 8601, set when human responds or timeout expires

State transitions for questions:
- `pending` → `answered`: human provides an answer
- `pending` → `assumed`: timeout expires without human response
- `answered` → (terminal): cannot be changed
- `assumed` → (terminal): cannot be changed

### FR-003: API Endpoints for Questions
Source: US-001, US-002

#### GET /api/features/{id}/questions
Returns all questions for a feature.
- 200: returns array of question objects (may be empty `[]`)
- 404: feature not found

#### POST /api/features/{id}/questions
Creates a new question for a feature.
- Request body: `{ "phase": "inception"|"planning", "role": "pm"|"architect", "question": "...", "type": "clarification"|"decision"|"priority", "options": ["A", "B"] }`
- `phase`, `role`, `question`, and `type` are required
- `options` is optional
- 201: question created with auto-generated `id`, `status: "pending"`, `created_at` set
- 400: invalid request body (missing required fields, field validation failures)
- 404: feature not found

#### PATCH /api/features/{id}/questions/{questionId}
Answers a pending question.
- Request body: `{ "answer": "I want option A" }`
- `answer` is required, must be 1-5000 characters
- 200: question answered, `status` → `"answered"`, `answer` stored, `answered_at` set
- 400: invalid request body (missing answer, answer too long, answer empty string)
- 404: feature not found or question not found
- 409: question already answered or assumed (status is not "pending")

#### GET /api/features/{id}/questions/pending
Returns only pending (unanswered) questions for a feature.
- 200: returns array of question objects with `status: "pending"` (may be empty `[]`)
- 404: feature not found

### FR-004: Web UI Question Display
Source: US-001, US-002

The feature detail page must show pending questions when the feature is in `waiting_for_human` status.

- Questions are displayed in a card format with the question text, type badge (color-coded: clarification=blue, decision=orange, priority=purple), and suggested options (if any)
- User can type an answer in a text input and submit, or click a suggested option to fill the answer
- Once answered, the question card shows the answer in a read-only state with a green checkmark
- When all questions are answered, a "Resume Pipeline" button appears (or the pipeline auto-resumes)
- Questions section is hidden when the feature has no questions

### FR-005: Web UI Question Badge
Source: US-006

The feature list page (Dashboard) must show a badge on features that have pending questions.

- Badge displays the count of pending questions (e.g., "2" for 2 pending questions)
- Badge color: yellow/orange to indicate "needs attention"
- Badge is positioned on the feature card, top-right corner
- Clicking the badge navigates to the feature detail page
- Badge is hidden when no pending questions exist (not shown with "0")

### FR-006: Pipeline Pauses at Decision Points
Source: US-003

The pipeline orchestrator must check for pending questions after agent dispatch.

Flow:
1. PM or Architect agent completes dispatch
2. Pipeline checks if the agent produced a `questions_json` artifact
3. If questions exist:
   a. Store each question in the question store
   b. Set feature status to `waiting_for_human`
   c. Broadcast `waiting_for_human` SSE event
4. If no questions exist:
   a. Proceed with normal gate evaluation

When human answers all questions:
1. Pipeline detects all questions are answered (via API call or SSE event)
2. Set feature status back to `in_progress`
3. Build "Human Responses" section for agent context
4. Re-dispatch the agent with updated context including answers

When timeout expires:
1. For each pending question, generate an assumption
2. Set question `status` to `assumed`, `assumption` to the generated assumption, `answered_at` to current timestamp
3. Set feature status back to `in_progress`
4. Build "Human Responses" section with assumptions marked as auto-assumed
5. Re-dispatch the agent with updated context including assumptions

### FR-007: Human Input Incorporated into Agent Context
Source: US-001, US-002, US-004

When a human answers questions (or questions are auto-assumed after timeout), the answers must be injected into the agent's context for the next dispatch.

The pipeline builds a "Human Responses" section in CONTEXT.md:
```
=== Human Responses ===

Q-001: [question text]
→ [human answer]
[Source: human input]

Q-002: [question text]
→ [assumption text]
[Source: auto-assumed after timeout of 30 minutes]
```

Each answered question includes the question, the answer, and the source (human or auto-assumed). This section is appended after the role instructions and before the phase-specific instructions in the CONTEXT.md file.

### FR-008: Feature Status Transitions for Human Interaction
Source: US-001, US-003

Current status flow:
```
draft → in_progress → gate_blocked → passed → ... → done
                                        ↘ failed ↗
```

New status with human interaction:
```
draft → in_progress ↔ waiting_for_human
                ↑         (only during inception or planning phase)
```

A feature can enter `waiting_for_human` from `in_progress` only when the feature is in inception or planning phase. It returns to `in_progress` when:
- All questions are answered by a human
- Timeout expires and assumptions are generated

Other status interactions:
- If a feature is in `waiting_for_human` and the user cancels it, it transitions to `cancelled`
- If a feature is in `waiting_for_human` and the user recirculates it, it transitions to `recirculated` and questions are cleared
- A feature in `gate_blocked` or `failed` status cannot enter `waiting_for_human`

### FR-009: Timeout Configuration
Source: US-004

The timeout for human response is configurable via `devteam.yaml`:

```yaml
pipeline:
  human_interaction_timeout_minutes: 30
```

Behavior:
- Default: 30 minutes
- If set to 0: pipeline never pauses for human input (fully autonomous mode) — questions are still stored but feature never enters `waiting_for_human`, and assumptions are immediately generated
- If set to -1: pipeline waits indefinitely for human input (no timeout)
- If set to a positive number: pipeline waits that many minutes, then auto-assumes

The timeout starts when the feature enters `waiting_for_human` status. It is reset if a new question is added while the feature is already in `waiting_for_human` status. [ASSUMPTION: The timeout resets on new question addition to avoid premature assumption generation while the human is actively engaging.]

### FR-010: Questions Cleared on Recirculation
Source: US-003

When a feature is recirculated (sent back to an earlier phase), all existing questions for that feature are deleted. The re-run of the phase may generate new questions, which will be new Question objects with new IDs.

This ensures stale questions from a previous run don't interfere with the new phase execution.

### FR-011: Question Detection from Agent Output
Source: US-005

When an agent dispatch completes during inception or planning, the pipeline must check for a `questions_json` artifact in the feature's spec directory.

Detection logic:
1. After agent dispatch completes, check if `{spec_dir}/questions.json` exists
2. If it exists, parse it as a JSON array of question objects
3. Each question object must have: `phase`, `role`, `question`, `type` (required); `options` (optional)
4. Validate each question object: required fields present, `phase` is "inception" or "planning", `role` is "pm" or "architect", `type` is valid enum, `question` is non-empty
5. Invalid questions are skipped and a warning is logged
6. Valid questions are stored with auto-generated IDs
7. If any valid questions were stored, set feature status to `waiting_for_human`

### FR-012: Concurrent Answer Handling
Source: US-001

When two users attempt to answer the same question simultaneously, only the first answer wins.

- The PATCH endpoint uses optimistic concurrency: if the question status is no longer "pending" when the update is attempted, return 409 Conflict
- The second user receives a 409 response with message: `{"error": "conflict", "details": "Question {id} is already answered"}`
- The winning answer is stored and no subsequent modifications are allowed

## Key Entities

### Question
- `id`: string (Q-001, Q-002, ...)
- `feature_id`: string (UUID of the feature)
- `phase`: enum (inception, planning)
- `role`: enum (pm, architect)
- `question`: string (1-2000 chars)
- `type`: enum (clarification, decision, priority)
- `options`: array of strings (0-10 items, each 1-500 chars)
- `answer`: string or null (max 5000 chars)
- `assumption`: string or null (max 5000 chars)
- `status`: enum (pending, answered, assumed)
- `created_at`: timestamp (ISO 8601)
- `answered_at`: timestamp or null (ISO 8601)

### Feature (extended)
- Existing Feature struct gains a new status value: `waiting_for_human`
- Existing PhaseState may reference questions via artifacts of type `ArtifactQuestionsJSON`
- No changes to the core Feature struct — questions are stored separately and linked by `feature_id`

### HumanInteractionConfig
- `timeout_minutes`: integer (default 30, 0=never pause, -1=wait forever)
- Sourced from `devteam.yaml` under `pipeline.human_interaction_timeout_minutes`

## Entity Relationships

```
Feature 1──* Question
  (a feature can have many questions)
  (a question belongs to exactly one feature)

Feature Status Machine:
  draft → in_progress ↔ waiting_for_human
  in_progress → gate_blocked → in_progress (after fix)
  in_progress → passed → ... → done
  in_progress → failed (after max recirculations)
  any → cancelled
```

## State Transitions

### Feature Status Transitions (Extended)

Valid transitions:
| From | To | Condition |
|---|---|---|
| draft | in_progress | Feature.Start() called |
| in_progress | waiting_for_human | Questions exist for feature in inception or planning |
| waiting_for_human | in_progress | All questions answered or timeout expired |
| waiting_for_human | cancelled | User cancels feature |
| waiting_for_human | recirculated | User recirculates feature |
| in_progress | gate_blocked | Gate evaluation failed |
| gate_blocked | in_progress | After fixes, re-evaluate |
| in_progress | passed | Gate evaluation passed |
| passed | in_progress (next phase) | Feature.AdvanceTo() called |
| passed | done | Feature at delivery phase |
| any non-terminal | cancelled | User cancels |

Invalid transitions:
| From | To | Reason |
|---|---|---|
| waiting_for_human | waiting_for_human | No self-transition |
| waiting_for_human | passed | Must return to in_progress first |
| waiting_for_human | gate_blocked | Must return to in_progress first |
| in_progress (construction+) | waiting_for_human | Only inception and planning support human interaction |
| draft | waiting_for_human | Feature must be started first |
| done | waiting_for_human | Feature is terminal |
| cancelled | waiting_for_human | Feature is terminal |

### Question Status Transitions

Valid transitions:
| From | To | Condition |
|---|---|---|
| pending | answered | Human provides an answer |
| pending | assumed | Timeout expires |

Invalid transitions:
| From | To | Reason |
|---|---|---|
| answered | answered | Already answered |
| answered | assumed | Already answered |
| assumed | answered | Already assumed |
| assumed | assumed | Already assumed |

## Success Criteria

- SC-001: Given a feature in inception phase, when the PM agent generates questions and produces a `questions.json` artifact, then the feature enters `waiting_for_human` status and the questions appear in the API
- SC-002: Given a feature with pending questions, when a human answers all questions via the API, then the feature resumes (`in_progress`) and the answers are included in the next agent dispatch context
- SC-003: Given a feature with pending questions, when the timeout expires, then the feature resumes with documented assumptions and proceeds autonomously
- SC-004: Given a feature with pending questions, when the feature list is viewed, then a badge shows the count of pending questions
- SC-005: Given a feature in `waiting_for_human` status, when the human opens the feature detail page, then pending questions are displayed with input fields
- SC-006: Given a feature with no questions, when the PM agent completes dispatch without generating questions, then the pipeline proceeds normally without pausing
- SC-007: Given the timeout is set to 0, when the pipeline processes a feature, then questions are stored but the feature never enters `waiting_for_human` and assumptions are immediately generated
- SC-008: Given a feature in `waiting_for_human` status, when the user cancels the feature, then it transitions to `cancelled` and questions are cleared
- SC-009: Given a feature in `waiting_for_human` status, when the user recirculates the feature, then it transitions to the target phase and questions are cleared
- SC-010: Given two users answering the same question simultaneously, when both PATCH requests arrive, then the first one wins and the second receives 409 Conflict

## Error Scenarios

| User Action | Success | Error Condition | Expected Response |
|---|---|---|---|
| Answer a question | 200 OK | Question already answered | 409 Conflict `{"error": "conflict", "details": "Question Q-001 is already answered"}` |
| Answer a question | 200 OK | Question not found | 404 Not Found `{"error": "not_found", "details": "Question Q-999 not found"}` |
| Answer a question | 200 OK | Feature not found | 404 Not Found `{"error": "not_found", "details": "Feature abc not found"}` |
| Answer a question | 200 OK | Answer is empty string | 400 Bad Request `{"error": "validation_error", "details": "answer must be 1-5000 characters"}` |
| Answer a question | 200 OK | Answer exceeds 5000 chars | 400 Bad Request `{"error": "validation_error", "details": "answer must be 1-5000 characters"}` |
| Get pending questions | 200 OK [] | Feature has no pending questions | 200 OK [] (empty array, not 404) |
| Get pending questions | 200 OK | Feature not found | 404 Not Found `{"error": "not_found", "details": "Feature abc not found"}` |
| Create a question | 201 Created | Missing required field (question) | 400 Bad Request `{"error": "validation_error", "details": "question is required"}` |
| Create a question | 201 Created | Invalid phase value | 400 Bad Request `{"error": "validation_error", "details": "phase must be one of: inception, planning"}` |
| Create a question | 201 Created | Invalid type value | 400 Bad Request `{"error": "validation_error", "details": "type must be one of: clarification, decision, priority"}` |
| Create a question | 201 Created | Feature not found | 404 Not Found `{"error": "not_found", "details": "Feature abc not found"}` |
| Get all questions | 200 OK [] | Feature has no questions | 200 OK [] (empty array, not 404) |
| Get all questions | 200 OK | Feature not found | 404 Not Found |
| Advance feature in waiting_for_human | — | Feature is in waiting_for_human | 400 Bad Request `{"error": "validation_error", "details": "Cannot advance feature in waiting_for_human status"}` |
| Timeout expires | Feature resumes | N/A | Feature status → in_progress, unanswered questions → assumed |

## Empty State Behavior

- GET /api/features/{id}/questions returns `[]` when no questions exist for the feature (not `null`, not 404)
- GET /api/features/{id}/questions/pending returns `[]` when no pending questions exist (not `null`, not 404)
- Feature list badge is hidden (not shown with "0") when no pending questions exist
- Feature detail page question section is hidden when the feature has no questions
- Question `options` field is `[]` when no suggested options exist (not `null`)
- Question `answer` field is `null` when not yet answered (not `""`)
- Question `assumption` field is `null` when not yet assumed (not `""`)

## Assumptions and Scope Boundaries

### In Scope
- Question model and storage (as YAML artifact in feature spec directory)
- API endpoints for question CRUD
- Feature status `waiting_for_human` with valid transitions
- Pipeline orchestrator changes to detect questions and pause
- Pipeline orchestrator changes to handle timeout and auto-assume
- Web UI question cards and badge
- Context injection of human responses into agent context
- Timeout configuration in `devteam.yaml`

### Out of Scope
- Real-time notification (push notifications, email) — [ASSUMPTION: SSE events are sufficient for real-time updates]
- Question editing after creation (questions are immutable once created)
- Answer modification after submission (answers are immutable)
- Bulk answer submission (answer one question at a time) — [ASSUMPTION: answering one at a time is sufficient for MVP]
- Rich text in questions or answers (plain text only) — [ASSUMPTION: plain text is sufficient for MVP]
- Question prioritization or ordering — [ASSUMPTION: questions are displayed in creation order]
- Human interaction during construction, review, testing, or delivery phases — only inception and planning
- WebSocket support — SSE is the existing real-time mechanism and is sufficient

### Assumptions
- [ASSUMPTION: The timeout resets when a new question is added while the feature is in `waiting_for_human` status, to avoid premature assumption generation while the human is actively engaging]
- [ASSUMPTION: Questions are stored as a YAML artifact (`questions.json`) in the feature spec directory, consistent with existing artifact patterns]
- [ASSUMPTION: The `ArtifactQuestionsJSON` artifact type uses the filename `questions.json` in the spec directory]
- [ASSUMPTION: Question IDs are auto-generated with format `Q-{NNN}` (sequential within a feature), not UUIDs, to be human-readable]
- [ASSUMPTION: The pipeline orchestrator runs a background goroutine with a timer to check for timeout expiration, rather than requiring an external scheduler]
- [ASSUMPTION: When a feature is recirculated, all existing questions are deleted and the re-run may generate new questions]
- [ASSUMPTION: The timeout is per-feature, starting from when the feature enters `waiting_for_human` status, and resets when a new question is added]
- [ASSUMPTION: In fully autonomous mode (timeout=0), questions are still stored but assumptions are immediately generated — the pipeline does not pause at all]
- [ASSUMPTION: The web UI polls or uses SSE to detect when all questions are answered and auto-resumes the pipeline without requiring manual "Resume" button click, but also provides a "Resume Pipeline" button as a fallback]

## Security Considerations (P1 Feature)

### Threat Modeling

1. **Spoofing**: Could someone submit answers on behalf of another user? [ASSUMPTION: The current system has no authentication — this is acceptable for the MVP since it's a single-user local tool. Authentication will be added in a future feature.]

2. **Tampering**: Could someone modify a question or answer after submission? Questions and answers are immutable once created/answered. The API only supports creation (POST) and answering (PATCH with status check). No UPDATE or DELETE endpoints for questions or answers.

3. **Repudiation**: Could someone deny submitting an answer? The `answered_at` timestamp records when an answer was submitted. [ASSUMPTION: No user identity tracking in MVP — single-user system.]

4. **Information disclosure**: Questions may contain sensitive project details. [ASSUMPTION: Local development tool, no network exposure beyond localhost.]

5. **Denial of service**: Could someone flood the API with questions? Rate limiting is not in scope for MVP but the API validates question data (max 5000 char answers, max 10 options).

6. **Elevation of privilege**: Could someone create questions for a role they shouldn't? [ASSUMPTION: Only the pipeline (PM/architect agent) creates questions via the questions.json artifact. The POST endpoint is for internal use but accessible via API. In MVP, no role-based access control.]

### Data Classification
- **Public**: Question text, type, phase, role, options — visible to all
- **Internal**: Answers, assumptions — visible to the pipeline and users
- **Restricted**: None identified for this feature

### Input Validation Rules

POST /api/features/{id}/questions:
- `phase`: enum(inception, planning), required
- `role`: enum(pm, architect), required
- `question`: string, required, 1-2000 characters
- `type`: enum(clarification, decision, priority), required
- `options`: array of strings, optional, 0-10 items, each 1-500 characters

PATCH /api/features/{id}/questions/{questionId}:
- `answer`: string, required, 1-5000 characters