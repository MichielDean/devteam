# Acceptance Criteria — Spec 003: Human Interaction Points

## US-001: Human Answers PM Clarification Questions

### Happy Path

AC-001: Given a feature in inception phase with pending questions, when the human views the feature detail page, then pending questions are displayed as cards with question text, type badge (clarification=blue, decision=orange, priority=purple), suggested options (if any), and a text input for answering
  Test level: e2e
  Verification: Create a feature, add questions via API, load the feature detail page in a browser, verify question cards are visible with type badges and input fields

AC-002: Given a feature with pending questions, when the human submits an answer via PATCH /api/features/{id}/questions/{questionId}, then the question status changes to "answered", the answer is stored, and answered_at is set
  Test level: integration
  Verification: Create question, PATCH answer, GET question, verify status="answered", answer field matches submitted value, answered_at is non-null

AC-003: Given a feature with pending questions, when the human submits an answer via the web UI, then the question card updates to show the answer in a read-only state with a green checkmark and the badge count decreases
  Test level: e2e
  Verification: Load feature detail page, type answer in input, click submit, verify card shows answer with checkmark, verify badge count on feature list page decreased by 1

### Error Paths

AC-004: Given a question that has already been answered, when the human submits another answer via PATCH, then the response is 409 Conflict with body {"error": "conflict", "details": "Question Q-001 is already answered"}
  Test level: integration
  Verification: Create question, PATCH answer successfully, PATCH same question again, verify 409 status and error body

AC-005: Given a question ID that does not exist, when the human submits an answer via PATCH, then the response is 404 Not Found with body {"error": "not_found", "details": "Question Q-999 not found"}
  Test level: integration
  Verification: PATCH /api/features/{id}/questions/Q-999 with answer, verify 404 status

AC-006: Given an empty string as an answer, when the human submits via PATCH, then the response is 400 Bad Request with body {"error": "validation_error", "details": "answer must be 1-5000 characters"}
  Test level: integration
  Verification: PATCH with {"answer": ""}, verify 400 status and error body

AC-007: Given an answer exceeding 5000 characters, when the human submits via PATCH, then the response is 400 Bad Request with body {"error": "validation_error", "details": "answer must be 1-5000 characters"}
  Test level: integration
  Verification: PATCH with answer of 5001 characters, verify 400 status

### Empty State

AC-008: Given a feature with no questions, when the feature detail page is viewed, then the question section is completely hidden (no empty state message, no placeholder)
  Test level: e2e
  Verification: Load feature detail page for feature with no questions, verify no question-related UI elements are visible

## US-002: Human Reviews Architect Design Decisions

### Happy Path

AC-009: Given a feature in planning phase with pending design decisions (type="decision"), when the human views the feature detail page, then decision cards are displayed with suggested options as clickable buttons
  Test level: e2e
  Verification: Create questions with type="decision" and options, load feature detail page, verify option buttons are visible and clickable

AC-010: Given a design decision question with suggested options, when the human clicks a suggested option, then the option text is populated into the answer field
  Test level: e2e
  Verification: Click an option button, verify the answer field contains the option text

### Error Paths

AC-011: Given a question with type="decision" that has already been answered, when the human tries to answer it, then the question card shows the answer in read-only state and no input fields are displayed
  Test level: e2e
  Verification: Answer a decision question via API, reload page, verify read-only state with no input fields

### Empty State

AC-012: Given a feature in planning phase with no design decisions, when the feature detail page is viewed, then no decision cards are shown and the pipeline proceeds normally
  Test level: e2e
  Verification: Load feature detail page for feature in planning with no questions, verify no decision cards

## US-003: Pipeline Pauses for Human Input

### Happy Path

AC-013: Given a feature in inception phase, when the PM agent produces a questions.json artifact, then the pipeline stores each question and sets the feature status to "waiting_for_human"
  Test level: integration
  Verification: Create feature, start inception, write questions.json artifact, run pipeline phase, verify questions are stored in API and feature status is "waiting_for_human"

AC-014: Given a feature in planning phase, when the Architect produces design decisions as questions, then the pipeline stores the questions and sets feature status to "waiting_for_human"
  Test level: integration
  Verification: Advance feature to planning, write questions.json artifact, run pipeline phase, verify questions stored and status is "waiting_for_human"

AC-015: Given a feature in "waiting_for_human" status, when all questions are answered via the API, then the feature status transitions to "in_progress" and the pipeline resumes the current phase
  Test level: integration
  Verification: Set feature to waiting_for_human with questions, answer all questions via PATCH, verify status becomes "in_progress" and pipeline re-dispatches agent

AC-016: Given a feature in construction phase, when the developer dispatch completes, then the feature never enters "waiting_for_human" regardless of questions.json presence
  Test level: integration
  Verification: Create feature in construction phase, write questions.json, run phase, verify feature status does not become "waiting_for_human"

### Error Paths

AC-017: Given a feature with status "draft", when a question is created via POST, then the question is stored but the feature does not enter "waiting_for_human"
  Test level: integration
  Verification: Create feature in draft status, POST question, verify question is stored but feature status remains "draft"

AC-018: Given a feature with status "gate_blocked", when a question is created, then the question is stored but the feature does not enter "waiting_for_human"
  Test level: integration
  Verification: Create feature in gate_blocked status, POST question, verify question is stored but feature status remains "gate_blocked"

AC-019: Given a feature in "waiting_for_human" status, when the user tries to advance the feature via POST /api/features/{id}/advance, then the response is 400 Bad Request with body {"error": "validation_error", "details": "Cannot advance feature in waiting_for_human status"}
  Test level: integration
  Verification: Set feature to waiting_for_human, POST advance, verify 400 response

### Empty State

AC-020: Given a feature in inception phase, when the PM agent completes dispatch without producing a questions.json artifact, then the pipeline proceeds normally to gate evaluation without pausing
  Test level: integration
  Verification: Run inception phase without questions.json, verify feature does not enter "waiting_for_human" and proceeds to gate evaluation

## US-004: Pipeline Falls Back to Autonomous Mode

### Happy Path

AC-021: Given a feature in "waiting_for_human" status with pending questions and a timeout of 30 minutes, when 30 minutes elapse without human response, then the feature status transitions to "in_progress" and each unanswered question is marked as "assumed" with an auto-generated assumption
  Test level: integration
  Verification: Set feature to waiting_for_human with pending questions, wait for timeout (or trigger programmatically), verify status becomes "in_progress" and questions have status="assumed" with non-null assumption field

AC-022: Given a feature in "waiting_for_human" status with the timeout configured to 0, when the pipeline processes the feature, then the feature never enters "waiting_for_human" and all questions are immediately marked as "assumed"
  Test level: integration
  Verification: Set timeout to 0 in config, create questions, run pipeline, verify feature never enters "waiting_for_human" and questions are immediately assumed

AC-023: Given a feature in "waiting_for_human" status with the timeout configured to -1, when the pipeline processes the feature, then the pipeline waits indefinitely and does not auto-assume
  Test level: integration
  Verification: Set timeout to -1 in config, create questions, run pipeline, verify feature enters "waiting_for_human" and no assumptions are generated automatically

AC-024: Given a feature with answered and unanswered questions when timeout expires, then only the unanswered questions are marked as "assumed" — already-answered questions remain unchanged
  Test level: integration
  Verification: Create 3 questions, answer 1, wait for timeout, verify answered question remains "answered" and 2 unanswered questions become "assumed"

### Error Paths

AC-025: Given the timeout mechanism fails (e.g., background goroutine crashes), when the timeout should trigger, then the feature remains in "waiting_for_human" status and an error is logged
  Test level: unit
  Verification: Simulate timeout goroutine failure, verify feature remains in "waiting_for_human" and error is logged

### Empty State

AC-026: Given a feature in "waiting_for_human" status where all questions are answered before timeout, when the timeout timer checks, then it finds no pending questions and simply returns the feature to "in_progress" without generating any assumptions
  Test level: integration
  Verification: Create questions, answer all, wait for timeout check, verify no assumptions generated

## US-005: Agent Creates Questions During Dispatch

### Happy Path

AC-027: Given a valid questions.json artifact with all required fields, when the pipeline reads it after agent dispatch, then each question is stored with an auto-generated ID (Q-001, Q-002, ...) and status "pending"
  Test level: integration
  Verification: Write questions.json with valid format to spec directory, trigger question detection, GET /api/features/{id}/questions, verify questions stored with correct IDs and status

AC-028: Given a questions.json artifact with some invalid questions (missing required fields), when the pipeline reads it, then valid questions are stored and invalid questions are skipped with a warning logged
  Test level: integration
  Verification: Write questions.json with mix of valid and invalid questions, trigger detection, verify valid questions stored, invalid skipped, warning in logs

### Error Paths

AC-029: Given a questions.json artifact that is not valid JSON, when the pipeline tries to parse it, then no questions are stored and a warning is logged
  Test level: integration
  Verification: Write invalid JSON to questions.json, trigger detection, verify no questions stored and warning logged

AC-030: Given a questions.json artifact where the `phase` field is "construction" (not inception or planning), when the pipeline reads it, then that question is skipped and a warning is logged
  Test level: integration
  Verification: Write questions.json with phase="construction", trigger detection, verify question not stored and warning logged

### Empty State

AC-031: Given no questions.json artifact in the spec directory, when the pipeline checks after agent dispatch, then the pipeline proceeds normally without pausing
  Test level: integration
  Verification: Run phase without questions.json, verify no questions created and feature does not enter "waiting_for_human"

## US-006: Feature List Shows Question Badge

### Happy Path

AC-032: Given a feature with 3 pending questions, when the feature list is viewed, then a badge showing "3" is displayed on the feature card
  Test level: e2e
  Verification: Create feature with 3 pending questions, load dashboard, verify badge shows "3"

AC-033: Given a feature with 1 pending question, when the feature list is viewed, then a badge showing "1" is displayed on the feature card
  Test level: e2e
  Verification: Create feature with 1 pending question, load dashboard, verify badge shows "1"

AC-034: Given a feature badge showing pending questions, when the user clicks the badge, then they are navigated to the feature detail page
  Test level: e2e
  Verification: Click badge, verify navigation to feature detail page

### Empty State

AC-035: Given a feature with no pending questions, when the feature list is viewed, then no badge is displayed on the feature card
  Test level: e2e
  Verification: Create feature with no questions, load dashboard, verify no badge visible

AC-036: Given a feature where all questions are answered, when the feature list is viewed, then no badge is displayed on the feature card
  Test level: e2e
  Verification: Create feature with questions, answer all, load dashboard, verify badge is hidden

### Error Path

AC-037: Given the questions API returns an error, when the feature list is viewed, then the list still renders and the badge is not shown (graceful degradation)
  Test level: e2e
  Verification: Mock API to return error, load dashboard, verify list renders without badge

## FR-001: Feature Status "Waiting for Human"

### State Transitions

AC-038: Given a feature with status "in_progress" in inception phase, when questions are detected, then the feature status transitions to "waiting_for_human"
  Test level: unit
  Verification: Create feature in inception with in_progress status, trigger question detection, verify status is "waiting_for_human"

AC-039: Given a feature with status "in_progress" in planning phase, when questions are detected, then the feature status transitions to "waiting_for_human"
  Test level: unit
  Verification: Create feature in planning with in_progress status, trigger question detection, verify status is "waiting_for_human"

AC-040: Given a feature in "waiting_for_human" status, when all questions are answered, then the feature status transitions back to "in_progress"
  Test level: unit
  Verification: Set feature to "waiting_for_human", answer all questions, call transition, verify status is "in_progress"

AC-041: Given a feature with status "draft", when question detection is triggered, then the transition to "waiting_for_human" is rejected
  Test level: unit
  Verification: Create feature in draft status, attempt transition to "waiting_for_human", verify error returned and status remains "draft"

AC-042: Given a feature in construction phase with status "in_progress", when question detection is triggered, then the transition to "waiting_for_human" is rejected because only inception and planning support human interaction
  Test level: unit
  Verification: Create feature in construction phase, attempt transition to "waiting_for_human", verify error returned

AC-043: Given a feature in "waiting_for_human" status, when timeout expires, then the feature status transitions to "in_progress"
  Test level: unit
  Verification: Set feature to "waiting_for_human", trigger timeout, verify status transitions to "in_progress"

## FR-002: Question Model

AC-044: Given a question with all required fields (phase, role, question, type), when created via POST /api/features/{id}/questions, then it is stored with auto-generated ID (Q-001), status "pending", and created_at timestamp
  Test level: integration
  Verification: POST valid question, verify 201 response, GET question, verify id format "Q-NNN", status="pending", created_at is set

AC-045: Given a question with an empty question field, when created via POST, then the response is 400 Bad Request with body {"error": "validation_error", "details": "question is required"}
  Test level: integration
  Verification: POST {"phase": "inception", "role": "pm", "question": "", "type": "clarification"}, verify 400 response

AC-046: Given a question with an invalid phase value, when created via POST, then the response is 400 Bad Request with body {"error": "validation_error", "details": "phase must be one of: inception, planning"}
  Test level: integration
  Verification: POST with phase="construction", verify 400 response

AC-047: Given a question with an invalid type value, when created via POST, then the response is 400 Bad Request with body {"error": "validation_error", "details": "type must be one of: clarification, decision, priority"}
  Test level: integration
  Verification: POST with type="invalid_type", verify 400 response

AC-048: Given a question with more than 10 options, when created via POST, then the response is 400 Bad Request with details indicating maximum 10 options allowed
  Test level: integration
  Verification: POST with options array of 11 items, verify 400 response

AC-049: Given a question that is pending, when the timeout expires, then the question status changes to "assumed" and the assumption field is populated
  Test level: integration
  Verification: Create question, trigger timeout, GET question, verify status="assumed" and assumption is non-null

AC-050: Given a question that is "answered", when any attempt is made to change its status, then the question remains unchanged (terminal state)
  Test level: unit
  Verification: Create answered question, attempt to change status, verify no change

## FR-003: API Endpoints

### GET /api/features/{id}/questions

AC-051: Given a feature with 3 questions, when GET /api/features/{id}/questions is called, then all 3 questions are returned with correct structure (id, feature_id, phase, role, question, type, options, answer, assumption, status, created_at, answered_at)
  Test level: integration
  Verification: Create 3 questions, GET endpoint, verify response contains 3 items with all expected fields

AC-052: Given a feature with no questions, when GET /api/features/{id}/questions is called, then an empty array is returned (not null, not 404)
  Test level: integration
  Verification: GET /api/features/{id}/questions for feature with no questions, verify response body is exactly []

AC-053: Given a feature ID that does not exist, when GET /api/features/{id}/questions is called, then the response is 404 Not Found with body {"error": "not_found", "details": "Feature abc not found"}
  Test level: integration
  Verification: GET /api/features/nonexistent-id/questions, verify 404 status and error body

### POST /api/features/{id}/questions

AC-054: Given a valid question payload, when POST /api/features/{id}/questions is called, then the response is 201 Created with the full question object including auto-generated id
  Test level: integration
  Verification: POST valid question, verify 201 status, response body contains id starting with "Q-", and all fields match

AC-055: Given a question payload missing the "question" field, when POST is called, then the response is 400 Bad Request
  Test level: integration
  Verification: POST {"phase": "inception", "role": "pm", "type": "clarification"} (missing "question"), verify 400

AC-056: Given a question payload with a feature ID that does not exist, when POST is called, then the response is 404 Not Found
  Test level: integration
  Verification: POST to /api/features/nonexistent-id/questions, verify 404

### PATCH /api/features/{id}/questions/{questionId}

AC-057: Given a pending question, when PATCH is called with a valid answer, then the response is 200 OK with the updated question (status="answered", answer populated, answered_at set)
  Test level: integration
  Verification: Create question, PATCH with {"answer": "My answer"}, verify 200 status and updated question

AC-058: Given a question that is already "answered", when PATCH is called with another answer, then the response is 409 Conflict
  Test level: integration
  Verification: Answer question, then PATCH again, verify 409

AC-059: Given a question that is "assumed", when PATCH is called with an answer, then the response is 409 Conflict
  Test level: integration
  Verification: Let question timeout to "assumed", then PATCH with answer, verify 409

AC-060: Given a questionId that does not exist, when PATCH is called, then the response is 404 Not Found
  Test level: integration
  Verification: PATCH /api/features/{id}/questions/Q-999 with answer, verify 404

### GET /api/features/{id}/questions/pending

AC-061: Given a feature with 5 questions where 2 are answered and 3 are pending, when GET /api/features/{id}/questions/pending is called, then only the 3 pending questions are returned
  Test level: integration
  Verification: Create 5 questions, answer 2, GET pending endpoint, verify exactly 3 questions returned, all with status="pending"

AC-062: Given a feature where all questions are answered, when GET /api/features/{id}/questions/pending is called, then an empty array is returned (not 404)
  Test level: integration
  Verification: Create questions, answer all, GET pending endpoint, verify response is []

AC-063: Given a feature ID that does not exist, when GET /api/features/{id}/questions/pending is called, then the response is 404 Not Found
  Test level: integration
  Verification: GET /api/features/nonexistent-id/questions/pending, verify 404

## FR-004: Web UI Question Display

AC-064: Given a feature in "waiting_for_human" status with 2 pending questions, when the feature detail page loads, then 2 question cards are displayed with question text, type badges, and input fields
  Test level: e2e
  Verification: Create feature with 2 pending questions, load feature detail page, verify 2 question cards visible with correct type badges

AC-065: Given a question with type "clarification", when the question card is displayed, then a blue badge labeled "clarification" is shown
  Test level: e2e
  Verification: Create clarification question, load page, verify blue badge

AC-066: Given a question with type "decision", when the question card is displayed, then an orange badge labeled "decision" is shown
  Test level: e2e
  Verification: Create decision question, load page, verify orange badge

AC-067: Given a question with type "priority", when the question card is displayed, then a purple badge labeled "priority" is shown
  Test level: e2e
  Verification: Create priority question, load page, verify purple badge

AC-068: Given a question with suggested options, when the question card is displayed, then the options are shown as clickable buttons
  Test level: e2e
  Verification: Create question with options, load page, verify option buttons visible

AC-069: Given a question that has been answered, when the question card is displayed, then it shows the answer in a read-only state with a green checkmark and no input fields
  Test level: e2e
  Verification: Answer a question via API, reload page, verify read-only state with checkmark

## FR-006: Pipeline Pauses at Decision Points

AC-070: Given a feature in inception phase, when the PM dispatch completes and a questions.json artifact exists, then the pipeline stores the questions and sets feature status to "waiting_for_human"
  Test level: integration
  Verification: Create feature, start inception, write questions.json, run pipeline phase, verify questions stored and status is "waiting_for_human"

AC-071: Given a feature in planning phase, when the Architect dispatch completes and a questions.json artifact exists, then the pipeline stores the questions and sets feature status to "waiting_for_human"
  Test level: integration
  Verification: Advance feature to planning, write questions.json, run pipeline phase, verify questions stored and status is "waiting_for_human"

AC-072: Given a feature in inception phase, when the PM dispatch completes and no questions.json artifact exists, then the pipeline proceeds to gate evaluation without pausing
  Test level: integration
  Verification: Run inception phase without questions.json, verify feature does not enter "waiting_for_human" and proceeds normally

## FR-007: Human Input in Agent Context

AC-073: Given a feature with 2 answered questions, when the pipeline re-dispatches the agent, then CONTEXT.md includes a "Human Responses" section listing both Q&A pairs with source "human input"
  Test level: integration
  Verification: Create 2 questions, answer both, re-dispatch, read CONTEXT.md, verify "Human Responses" section contains both Q&A pairs labeled "human input"

AC-074: Given a feature with 1 answered question and 1 assumed question, when the pipeline re-dispatches the agent, then CONTEXT.md includes a "Human Responses" section with one Q&A pair labeled "human input" and one labeled "auto-assumed after timeout"
  Test level: integration
  Verification: Create 2 questions, answer 1, let 1 timeout, re-dispatch, read CONTEXT.md, verify correct labels

AC-075: Given a feature with no questions, when the pipeline dispatches the agent, then CONTEXT.md does not include a "Human Responses" section
  Test level: integration
  Verification: Create feature with no questions, dispatch agent, read CONTEXT.md, verify no "Human Responses" section

## FR-008: Feature Status Transitions

AC-076: Given a feature in "waiting_for_human" status, when the user cancels the feature, then the feature transitions to "cancelled" status
  Test level: integration
  Verification: Set feature to "waiting_for_human", POST cancel, verify status is "cancelled"

AC-077: Given a feature in "waiting_for_human" status, when the user recirculates the feature, then the feature transitions to the target phase and all questions are deleted
  Test level: integration
  Verification: Set feature to "waiting_for_human" with questions, POST recirculate, verify questions deleted and feature is in target phase

## FR-009: Timeout Configuration

AC-078: Given a devteam.yaml with human_interaction_timeout_minutes set to 5, when the pipeline processes a feature with questions, then the feature enters "waiting_for_human" and auto-assumes after 5 minutes
  Test level: integration
  Verification: Set timeout to 5 minutes in config, create questions, verify feature enters "waiting_for_human", wait for timeout, verify questions assumed

AC-079: Given a devteam.yaml with human_interaction_timeout_minutes set to 0, when the pipeline processes a feature, then questions are stored but the feature never enters "waiting_for_human" and all questions are immediately assumed
  Test level: integration
  Verification: Set timeout to 0, create questions, verify feature does not enter "waiting_for_human" and questions are immediately assumed

AC-080: Given a devteam.yaml with human_interaction_timeout_minutes set to -1, when the pipeline processes a feature with questions, then the feature enters "waiting_for_human" and no timeout is applied
  Test level: integration
  Verification: Set timeout to -1, create questions, verify feature enters "waiting_for_human" and remains in that status indefinitely

AC-081: Given a feature in "waiting_for_human" status, when a new question is added while the timeout is counting, then the timeout is reset
  Test level: integration
  Verification: Set timeout to 10 minutes, enter "waiting_for_human", add question at minute 8, verify timeout resets and feature stays in "waiting_for_human" for another 10 minutes

## FR-010: Questions Cleared on Recirculation

AC-082: Given a feature with 5 questions, when the feature is recirculated, then all 5 questions are deleted
  Test level: integration
  Verification: Create 5 questions, recirculate feature, GET /api/features/{id}/questions, verify response is []

## FR-011: Question Detection from Agent Output

AC-083: Given a questions.json file with 3 valid questions, when the pipeline reads it, then 3 questions are stored in the question store
  Test level: integration
  Verification: Write questions.json with 3 questions to spec dir, trigger detection, GET /api/features/{id}/questions, verify 3 questions returned

AC-084: Given a questions.json file that is invalid JSON, when the pipeline tries to parse it, then no questions are stored and a warning is logged
  Test level: integration
  Verification: Write invalid JSON to questions.json, trigger detection, verify no questions stored and warning in logs

AC-085: Given a questions.json file with a question that has phase="construction", when the pipeline reads it, then that question is skipped with a warning
  Test level: integration
  Verification: Write questions.json with phase="construction", trigger detection, verify question not stored and warning in logs

## FR-012: Concurrent Answer Handling

AC-086: Given a pending question, when two PATCH requests arrive simultaneously with answers, then exactly one succeeds with 200 and the other receives 409 Conflict
  Test level: integration
  Verification: Create question, send two concurrent PATCH requests, verify one gets 200 and the other gets 409

## Smoke Tests

AC-087: Given a running server, when the feature list page loads, then it renders without JavaScript console errors
  Test level: smoke
  Verification: Start server, load dashboard in browser, check console for errors

AC-088: Given a running server, when the questions API endpoints are called, then they respond with correct HTTP status codes and JSON structure
  Test level: smoke
  Verification: Start server, call GET /api/features (200), POST /api/features/{id}/questions (201), GET /api/features/{id}/questions (200), PATCH /api/features/{id}/questions/{qid} (200), GET /api/features/{id}/questions/pending (200)

AC-089: Given a running server, when the feature detail page loads for a feature with questions, then it renders without JavaScript console errors
  Test level: smoke
  Verification: Create feature with questions, load feature detail page, check console for errors

## Security Acceptance Criteria

AC-SEC-001: Given a PATCH request with an answer containing a script tag, when the question is answered, then the script tag is stored as-is (not executed) and when displayed in the UI it is properly escaped
  Test level: integration
  Verification: PATCH question with answer `<script>alert('xss')</script>`, GET question, verify answer is stored as plain text, load UI and verify no script execution

AC-SEC-002: Given a POST request with question text exceeding 2000 characters, when the question is created, then the response is 400 Bad Request
  Test level: integration
  Verification: POST with question field of 2001 characters, verify 400 response

AC-SEC-003: Given a PATCH request with answer text exceeding 5000 characters, when the question is answered, then the response is 400 Bad Request
  Test level: integration
  Verification: PATCH with answer field of 5001 characters, verify 400 response

## Resilience Acceptance Criteria

AC-RES-001: Given the question store is temporarily unavailable, when the API receives a GET request for questions, then the response is 503 Service Unavailable with a meaningful error message, not a 500 crash
  Test level: integration
  Verification: Simulate store unavailability, GET /api/features/{id}/questions, verify 503 response with error message

AC-RES-002: Given a feature in "waiting_for_human" status when the server restarts, when the server comes back up, then the timeout timer is restarted based on the original waiting_for_human timestamp
  Test level: integration
  Verification: Set feature to "waiting_for_human", restart server, verify timeout timer is recalculated from the original timestamp