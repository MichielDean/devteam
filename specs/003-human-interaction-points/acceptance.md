# Acceptance Criteria — Spec 003: Human Interaction Points

## US-001: Human Answers PM Clarification Questions

AC-001: Given a feature in inception phase with pending questions, when the human views the feature detail page, then pending questions are displayed with input fields and optional suggested answers
  Test level: e2e
  Verification: Load feature detail page in browser, verify question cards are visible with input fields

AC-002: Given a feature with pending questions, when the human submits an answer via the API, then the question status changes to "answered" and the answer is stored
  Test level: integration
  Verification: POST answer to /api/features/{id}/questions/{qid}, then GET the question and verify status="answered" and answer is stored

AC-003: Given a feature with pending questions, when the human submits an answer via the web UI, then the question card updates to show the answer and the badge count decreases
  Test level: e2e
  Verification: Answer a question in the UI, verify the card updates and badge count decreases

## US-002: Human Reviews Architect Design Decisions

AC-004: Given a feature in planning phase with pending design decisions, when the human views the feature detail page, then design decisions are displayed with suggested options
  Test level: e2e
  Verification: Load feature detail page, verify design decision cards with options

## US-003: Pipeline Pauses for Human Input

AC-005: Given a feature in inception phase, when the PM agent generates questions, then the feature status transitions to "waiting_for_human"
  Test level: integration
  Verification: Dispatch PM agent, verify feature status becomes "waiting_for_human" when questions exist

AC-006: Given a feature in "waiting_for_human" status, when all questions are answered, then the feature status transitions back to "in_progress" and the phase continues
  Test level: integration
  Verification: Answer all questions, verify feature status becomes "in_progress"

## US-004: Pipeline Falls Back to Autonomous Mode

AC-007: Given a feature in "waiting_for_human" status with pending questions, when the timeout expires, then the feature status transitions to "in_progress" and unanswered questions are marked as "assumed"
  Test level: integration
  Verification: Wait for timeout (or trigger programmatically), verify status becomes "in_progress" and questions get assumptions

AC-008: Given a feature in "waiting_for_human" status, when the timeout is configured to 0, then the pipeline never pauses for human input
  Test level: integration
  Verification: Set timeout to 0, dispatch PM, verify feature never enters "waiting_for_human"

## FR-001: Feature State "Waiting for Human"

AC-009: Given a feature in any phase, when a question is created, then the feature status can transition to "waiting_for_human"
  Test level: unit
  Verification: Create question for a feature, call transition method, verify status is "waiting_for_human"

AC-010: Given a feature in "waiting_for_human" status, when all questions are answered, then the feature can transition back to "in_progress"
  Test level: unit
  Verification: Set feature to "waiting_for_human", answer all questions, call transition, verify status is "in_progress"

## FR-002: Question Model

AC-011: Given a question with all required fields, when the question is created via API, then it is stored with status "pending" and a unique ID
  Test level: integration
  Verification: POST /api/features/{id}/questions with valid data, verify 201 response with question ID

AC-012: Given an invalid question (missing required fields), when created via API, then the response is 400 Bad Request
  Test level: integration
  Verification: POST /api/features/{id}/questions with missing fields, verify 400 response

## FR-003: API Endpoints

AC-013: Given a feature with questions, when GET /api/features/{id}/questions is called, then all questions are returned with correct structure
  Test level: integration
  Verification: Create questions, GET endpoint, verify response contains all questions with id, phase, role, question, type, options, status fields

AC-014: Given a feature with no questions, when GET /api/features/{id}/questions is called, then an empty array is returned (not null, not 404)
  Test level: integration
  Verification: GET /api/features/{id}/questions for feature with no questions, verify response is []

AC-015: Given a feature with some answered and some pending questions, when GET /api/features/{id}/questions/pending is called, then only pending questions are returned
  Test level: integration
  Verification: Create questions, answer some, GET pending endpoint, verify only unanswered questions returned

AC-016: Given a question that is already answered, when PATCH is called with another answer, then the response is 409 Conflict
  Test level: integration
  Verification: Answer a question, then try to answer it again, verify 409

## FR-006: Pipeline Pauses at Decision Points

AC-017: Given a feature in inception phase, when the PM agent produces questions, then the pipeline stores the questions and sets feature status to "waiting_for_human"
  Test level: integration
  Verification: Run pipeline inception phase, verify questions are stored and feature status is "waiting_for_human"

AC-018: Given a feature in planning phase, when the Architect produces design decisions as questions, then the pipeline stores the questions and sets feature status to "waiting_for_human"
  Test level: integration
  Verification: Run pipeline planning phase, verify questions are stored and feature status is "waiting_for_human"

## FR-007: Human Input in Agent Context

AC-019: Given a feature with answered questions, when the pipeline re-dispatches the agent, then CONTEXT.md includes a "Human Responses" section with all answered questions
  Test level: integration
  Verification: Answer questions, re-dispatch, verify CONTEXT.md contains "Human Responses" section with Q&A pairs

AC-020: Given a feature with assumed questions (timeout expired), when the pipeline re-dispatches the agent, then CONTEXT.md includes assumptions labeled "auto-assumed after timeout"
  Test level: integration
  Verification: Let timeout expire, re-dispatch, verify CONTEXT.md contains assumptions with timeout labels