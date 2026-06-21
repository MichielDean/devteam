# Spec 003: Human Interaction Points

## Priority
P1

## User Stories

### US-001: Human Answers PM Clarification Questions
As a product owner, I want to answer the PM's clarification questions through the web UI so that the spec reflects my actual requirements instead of assumptions.

### US-002: Human Reviews Architect Design Decisions
As a product owner, I want to review and approve key design decisions from the Architect through the web UI so that the plan aligns with my technical preferences.

### US-003: Pipeline Pauses for Human Input
As a product owner, I want the pipeline to pause at decision points when human input is available, so that I can provide direction instead of having the agent make assumptions.

### US-004: Pipeline Falls Back to Autonomous Mode
As a product owner, I want the pipeline to automatically proceed with documented assumptions if I don't respond within a timeout, so that work doesn't stall indefinitely.

## Functional Requirements

### FR-001: Feature State "Waiting for Human"
The feature state machine must support a new status: `waiting_for_human`. When the pipeline reaches a decision point and human input is requested, the feature transitions to this status.

- Valid transitions TO `waiting_for_human`: from any phase status `in_progress`
- Valid transitions FROM `waiting_for_human`: back to `in_progress` (when human responds) or to `in_progress` with assumptions (when timeout expires)

### FR-002: Question Model
A new artifact type `questions_json` stores clarification questions that the PM or Architect surfaces for human input.

Each question has:
- `id`: unique identifier (e.g., "Q-001")
- `phase`: which phase generated this question
- `role`: which role generated this question (pm, architect)
- `question`: the question text
- `type`: "clarification" | "decision" | "priority"
- `options`: optional list of suggested answers
- `answer`: null until human responds
- `assumption`: null until timeout expires, then filled with the agent's assumption
- `status`: "pending" | "answered" | "assumed"
- `created_at`: timestamp
- `answered_at`: null until human responds or timeout

### FR-003: API Endpoints for Questions

```
GET /api/features/{id}/questions
  200: returns list of questions for this feature
  404: feature not found

POST /api/features/{id}/questions
  body: { "phase": "inception", "role": "pm", "question": "...", "type": "clarification", "options": ["A", "B"] }
  201: question created
  400: invalid question data

PATCH /api/features/{id}/questions/{questionId}
  body: { "answer": "I want option A" }
  200: question answered
  404: question not found
  409: question already answered or assumed

GET /api/features/{id}/questions/pending
  200: returns only pending (unanswered) questions
  404: feature not found
```

### FR-004: Web UI Question Display
The feature detail page must show pending questions when the feature is in `waiting_for_human` status.

- Questions are displayed in a card format with the question text, type badge, and suggested options (if any)
- User can type an answer and submit, or select from suggested options
- Once answered, the question shows the answer and the feature can resume

### FR-005: Web UI Question Badge
The feature list page must show a badge on features that have pending questions.

- Badge shows count of pending questions
- Badge color: yellow/orange to indicate "needs attention"
- Clicking the badge navigates to the feature detail page

### FR-006: Pipeline Pauses at Decision Points
The pipeline orchestrator must check for pending questions before advancing phases.

- After PM dispatch: check if questions were generated. If yes, set status to `waiting_for_human`
- After Architect dispatch: check if questions were generated. If yes, set status to `waiting_for_human`
- After human answers all questions: resume the phase, incorporating the answers into the agent context
- After timeout (configurable, default 30 minutes): fill in assumptions, set status back to `in_progress`, proceed

### FR-007: Human Input Incorporated into Agent Context
When a human answers questions, the answers must be injected into the agent's context for the next dispatch.

- Build a "Human Responses" section in CONTEXT.md
- Each answered question is included: "Q-001: [question] → [human answer]"
- Each assumed question is included: "Q-002: [question] → [assumption] (auto-assumed after timeout)"
- The agent receives this context on re-dispatch

### FR-008: Feature Status Transitions for Human Interaction

Current transitions:
```
draft → inception → planning → construction → review → testing → delivery
```

New transitions:
```
draft → inception → planning → construction → review → testing → delivery
                ↕              ↕
        waiting_for_human  waiting_for_human
```

A feature can enter `waiting_for_human` from inception or planning (the two phases that benefit from human input). It can return to `in_progress` when the human responds or when timeout expires.

### FR-009: Timeout Configuration
The timeout for human response is configurable:

- Default: 30 minutes
- Configurable via `devteam.yaml`: `human_interaction_timeout_minutes`
- If set to 0: pipeline never pauses for human input (fully autonomous)
- If set to -1: pipeline waits indefinitely for human input

## Error Scenarios

| User Action | Success | Error Condition | Expected Response |
|---|---|---|---|
| Answer a question | 200 OK | Question already answered | 409 Conflict |
| Answer a question | 200 OK | Question not found | 404 Not Found |
| Get pending questions | 200 OK [] | Feature has no questions | 200 OK [] (empty, not 404) |
| Create a question | 201 Created | Invalid question data | 400 Bad Request |
| Timeout expires | Feature resumes | N/A | Feature status → in_progress, unanswered questions → assumed |

## Empty State Behavior

- GET /api/features/{id}/questions returns `[]` when no questions exist (not `null`, not 404)
- GET /api/features/{id}/questions/pending returns `[]` when all questions are answered (not 404)
- Feature list badge is hidden when no pending questions exist

## Success Criteria

- SC-001: Given a feature in inception phase, when the PM generates questions, then the feature enters `waiting_for_human` status and the questions appear in the API
- SC-002: Given a feature with pending questions, when a human answers all questions, then the feature resumes and the answers are included in the next agent dispatch
- SC-003: Given a feature with pending questions, when the timeout expires, then the feature resumes with documented assumptions and proceeds autonomously
- SC-004: Given a feature with pending questions, when the feature list is viewed, then a badge shows the count of pending questions
- SC-005: Given a feature in `waiting_for_human` status, when the human opens the feature detail page, then pending questions are displayed with input fields