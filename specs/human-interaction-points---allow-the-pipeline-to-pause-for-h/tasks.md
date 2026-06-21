# Tasks 003: Human Interaction Points

## P1 Tasks (Must-Have)

### T001: Add Question Model and FileQuestionStore

**Files**: `internal/feature/question.go`, `internal/feature/question_test.go`

**Dependencies**: None (can start immediately)

**Description**: Define the Question struct with JSON/YAML serialization tags, QuestionStore interface, and FileQuestionStore implementation. The FileQuestionStore reads/writes `specs/{id}/questions.json` on disk. Question IDs are auto-generated as Q-NNN (sequential within a feature).

**Done conditions**:
- Question struct is defined with all fields from the spec (id, feature_id, phase, role, question, type, options, answer, assumption, status, created_at, answered_at)
- QuestionStore interface is defined with: CreateQuestion, GetQuestion, ListQuestions, ListPendingQuestions, AnswerQuestion, AssumeQuestion, DeleteQuestionsForFeature, PendingCount
- FileQuestionStore implements QuestionStore using `specs/{id}/questions.json`
- Question ID generation: read existing questions, find max NNN, increment
- Question validation: phase must be "inception" or "planning", role must be "pm" or "architect", type must be "clarification", "decision", or "priority", question must be 1-2000 chars, options must be 0-10 items each 1-500 chars
- AnswerQuestion validates question exists, is in "pending" status, answer is 1-5000 chars, then updates status to "answered" and sets answered_at
- AnswerQuestion returns error if question is already "answered" or "assumed" (409 conflict)
- AssumeQuestion validates question exists and is "pending", then updates status to "assumed", sets assumption field and answered_at
- ListQuestions returns `[]` (empty slice, not nil) when no questions exist
- ListPendingQuestions filters to only status="pending" questions
- DeleteQuestionsForFeature removes the entire questions.json file
- PendingCount returns the count of pending questions for a feature
- All unit tests pass: TestQuestionValidation, TestQuestionIDGeneration, TestAnswerQuestionConflict, TestAssumeQuestion, TestListQuestionsEmpty, TestListPendingQuestions, TestDeleteQuestionsForFeature, TestPendingCount

**Test level**: unit

**Agent failure mode checks**:
- JSON serialization: empty `Options` slice must serialize as `[]` not `null` — initialize as `make([]string, 0)` not `var options []string`
- Nil pointer safety: `Answer` and `Assumption` fields are `*string` — check for nil before dereferencing
- File operations: use temp file + rename pattern for atomic writes

---

### T002: Add WaitingForHuman Status and State Transitions

**Files**: `internal/feature/types.go`, `internal/feature/state.go`, `internal/feature/state_test.go`, `internal/feature/feature.go`

**Dependencies**: None (can start in parallel with T001)

**Description**: Add `StatusWaitingHuman` constant, add `CanTransitionToWaitingHuman()` method on Feature, add `WaitForHuman()` method to transition feature to `waiting_for_human` status, add `ResumeFromWaitingHuman()` method to transition back to `in_progress`.

**Done conditions**:
- `StatusWaitingHuman Status = "waiting_for_human"` constant added to types.go
- `CanTransitionToWaitingHuman()` method returns true only when: current status is `in_progress` AND current phase is `inception` or `planning`
- `WaitForHuman()` method transitions feature from `in_progress` to `waiting_for_human`, updates `UpdatedAt`
- `ResumeFromWaitingHuman()` method transitions feature from `waiting_for_human` back to `in_progress`, updates `UpdatedAt`
- `Cancel()` method works from `waiting_for_human` status (already works since Cancel() sets status directly)
- `RecirculateTo()` method works from `waiting_for_human` status — questions are cleared by the pipeline, not by the state machine
- Attempting to advance a feature in `waiting_for_human` status returns an error
- `STATUS_LABELS` map in frontend types.ts updated to include `waiting_for_human: "Waiting for Human"`
- FeatureCard statusColors map updated with `waiting_for_human: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'`
- All unit tests pass: TestCanTransitionToWaitingHuman_Inception, TestCanTransitionToWaitingHuman_Planning, TestCanTransitionToWaitingHuman_Construction, TestCanTransitionToWaitingHuman_Draft, TestWaitForHuman, TestResumeFromWaitingHuman, TestAdvanceFromWaitingHumanBlocked
- FeatureSummaryResponse DTO includes `pending_questions_count` field (default 0)

**Test level**: unit

**Agent failure mode checks**:
- State machine: verify ALL transitions listed in the spec — both valid and invalid
- Check that `AdvanceTo()` returns an error when status is `waiting_for_human`
- Verify `STATUS_LABELS` in frontend types matches backend status constants exactly

---

### T003: Add Question API Endpoints

**Files**: `internal/api/server.go`, `internal/api/dto.go`

**Dependencies**: T001 (Question model and store), T002 (waiting_for_human status)

**Description**: Add 4 new HTTP handlers and route registration in server.go. Add question-related DTOs in dto.go. Add QuestionStore as a dependency on the Server struct.

**Done conditions**:
- Server struct has a `questionStore` field of type `QuestionStore`
- `NewServer` accepts QuestionStore parameter (or creates default FileQuestionStore)
- Route registration: `GET /api/features/{id}/questions`, `POST /api/features/{id}/questions`, `PATCH /api/features/{id}/questions/{questionId}`, `GET /api/features/{id}/questions/pending`
- Routes are registered BEFORE the SPA catch-all handler
- `PATCH` is added to CORS allowed methods (currently only GET, POST, OPTIONS)
- GET /api/features/{id}/questions returns 200 with `[]` for feature with no questions (NOT null, NOT 404)
- GET /api/features/{id}/questions returns 404 for nonexistent feature
- POST /api/features/{id}/questions returns 201 for valid question with auto-generated ID
- POST /api/features/{id}/questions returns 400 for missing required fields (question, phase, role, type)
- POST /api/features/{id}/questions returns 400 for invalid phase (not "inception" or "planning")
- POST /api/features/{id}/questions returns 400 for invalid type (not "clarification", "decision", "priority")
- POST /api/features/{id}/questions returns 400 for invalid role (not "pm" or "architect")
- POST /api/features/{id}/questions returns 400 for question > 2000 chars
- POST /api/features/{id}/questions returns 400 for options > 10 items
- POST /api/features/{id}/questions returns 404 for nonexistent feature
- PATCH /api/features/{id}/questions/{questionId} returns 200 for valid answer
- PATCH /api/features/{id}/questions/{questionId} returns 400 for empty answer
- PATCH /api/features/{id}/questions/{questionId} returns 400 for answer > 5000 chars
- PATCH /api/features/{id}/questions/{questionId} returns 409 for already-answered question
- PATCH /api/features/{id}/questions/{questionId} returns 409 for already-assumed question
- PATCH /api/features/{id}/questions/{questionId} returns 404 for nonexistent question
- GET /api/features/{id}/questions/pending returns 200 with only pending questions
- GET /api/features/{id}/questions/pending returns 200 with `[]` when all questions answered
- GET /api/features/{id}/questions/pending returns 404 for nonexistent feature
- FeatureSummaryResponse DTO includes `pending_questions_count` field
- listFeatures handler populates `pending_questions_count` from QuestionStore
- All error responses follow `{"error": "code", "details": "message"}` format
- Recovery middleware is outermost in the middleware chain (already the case)
- Service starts without panicking after changes
- Integration tests verify all endpoints

**Test level**: integration (API request/response cycles)

**Agent failure mode checks**:
- JSON arrays: Question `Options` field must initialize as `[]` not null — use `make([]string, 0)` in DTO constructors
- Recovery middleware: verify it's already the outermost middleware in server.go
- CORS: must add "PATCH" to `Access-Control-Allow-Methods` header
- Request body size limit: add `http.MaxBytesReader` for POST/PATCH like existing createFeature handler

---

### T004: Add Question Detection and Timeout Handler

**Files**: `internal/pipeline/question.go`, `internal/pipeline/question_test.go`

**Dependencies**: T001 (Question model and store), T002 (waiting_for_human status)

**Description**: Implement question detection from agent output (reading questions.json artifact) and timeout handling (auto-assuming pending questions after configurable timeout).

**Done conditions**:
- `DetectQuestions(featureID, specDir)` function reads `questions.json` from spec directory
- If questions.json doesn't exist, returns empty slice with no error (not an error condition)
- If questions.json is invalid JSON, logs warning and returns empty slice with no error
- Each question in questions.json is validated: required fields present, valid phase/role/type, non-empty question text
- Invalid questions are skipped and a warning is logged with the reason
- Valid questions are converted to Question structs with auto-generated IDs
- `HandleTimeout(featureID, questionStore)` marks all pending questions as assumed with auto-generated assumptions
- Assumption text format: "No human response received. Assuming: [reasonable assumption based on question type and options]"
- If question has options, assumption includes the first option as the assumed answer
- If question has no options, assumption text is: "No human response received. This question was auto-assumed."
- `ShouldPauseForHuman(feature, timeoutMinutes)` returns false when timeout == 0, true when timeout > 0, true when timeout == -1
- All unit tests pass: TestDetectQuestions_Valid, TestDetectQuestions_InvalidJSON, TestDetectQuestions_MissingFields, TestDetectQuestions_InvalidPhase, TestDetectQuestions_NoFile, TestHandleTimeout_MarksPending, TestHandleTimeout_SkipsAnswered, TestShouldPauseForHuman_Zero, TestShouldPauseForHuman_Positive, TestShouldPauseForHuman_NegativeOne

**Test level**: unit

**Agent failure mode checks**:
- File not found: DetectQuestions returns empty slice, not error — this is the normal case (no questions)
- Invalid JSON: log warning, don't crash — return empty slice
- Nil pointer safety: check that Question fields are not nil before accessing
- Empty file: should not crash, should return empty slice with warning

---

### T005: Integrate Question Detection into Pipeline

**Files**: `internal/pipeline/pipeline.go`, `internal/pipeline/process.go`

**Dependencies**: T004 (question detection and timeout), T003 (API endpoints), T002 (status transitions)

**Description**: After agent dispatch in inception/planning phases, check for questions and transition to `waiting_for_human` if needed. Add timeout goroutine. When questions are all answered, resume pipeline with human responses injected into context.

**Done conditions**:
- After `RunPhaseWithAgent` in ProcessAsync, when current phase is inception or planning, call DetectQuestions
- If questions are detected and ShouldPauseForHuman is true: store questions, set feature status to `waiting_for_human`, broadcast SSE event, start timeout goroutine
- If questions are detected and timeout == 0: store questions, immediately assume all questions, proceed with normal gate evaluation
- If no questions detected: proceed with normal gate evaluation (no change)
- Timeout goroutine: after configurable timeout, call HandleTimeout, set feature status back to `in_progress`, broadcast SSE event
- When timeout goroutine detects all questions answered before timeout: set feature status to `in_progress`, broadcast SSE event, re-dispatch agent with human responses in context
- On ProcessAsync, if feature status is `waiting_for_human`, check if all questions are answered — if yes, resume pipeline
- Recirculation while in `waiting_for_human` deletes all questions before proceeding
- SSE event types: `waiting_for_human`, `questions_answered`, `questions_assumed`
- Timeout resets when a new question is added while in `waiting_for_human` status
- Server starts without panicking after changes
- Integration test: create feature, run inception phase with questions.json, verify status transitions to `waiting_for_human`

**Test level**: integration

**Agent failure mode checks**:
- Goroutine lifecycle: timeout goroutine must be cancellable via context — use `context.WithCancel`
- Race conditions: feature state must be protected when accessed from timeout goroutine — use mutex or channel-based communication
- Panic recovery: if timeout goroutine panics, it should not crash the server — recover and log error
- Nil pointer: check that Pipeline has QuestionStore before accessing it

---

### T006: Add Human Responses to Agent Context

**Files**: `internal/rules/loader.go`

**Dependencies**: T001 (Question model and store)

**Description**: When building context for agent dispatch, if the feature has answered or assumed questions, inject a "Human Responses" section into the CONTEXT.md.

**Done conditions**:
- `BuildContext` method (or a new `BuildContextWithResponses` method) accepts an optional list of Questions
- If questions exist, append a "=== Human Responses ===" section after the core context
- Each answered question is formatted as: `Q-{id}: {question text}\n→ {answer}\n[Source: human input]`
- Each assumed question is formatted as: `Q-{id}: {question text}\n→ {assumption}\n[Source: auto-assumed after timeout of {X} minutes]`
- If no questions exist, no "Human Responses" section is appended
- Unit test: TestBuildContextWithResponses_AnsweredQuestions, TestBuildContextWithResponses_AssumedQuestions, TestBuildContextWithResponses_NoQuestions

**Test level**: unit

**Agent failure mode checks**:
- Nil slice: if questions list is nil or empty, don't append section
- Empty string: if question text is empty, skip that question with a warning

---

### T007: Add Config Field for Timeout

**Files**: `internal/config/config.go`, `devteam.yaml`

**Dependencies**: None (can start in parallel with T001)

**Description**: Add `human_interaction_timeout_minutes` field to `PipelineConfig` in config.go. Default to 30. Update devteam.yaml to include the setting.

**Done conditions**:
- `PipelineConfig` struct has `HumanInteractionTimeoutMinutes int` field with `yaml:"human_interaction_timeout_minutes"`
- Default value of 30 is applied if field is zero or missing in config
- Value of 0 means "never pause, immediately assume" (fully autonomous)
- Value of -1 means "wait indefinitely" (no timeout)
- devteam.yaml includes `human_interaction_timeout_minutes: 30` under `pipeline:`
- Config validation accepts 0, -1, and positive integers
- Unit test: TestConfig_DefaultTimeout, TestConfig_ZeroTimeout, TestConfig_NegativeOneTimeout, TestConfig_CustomTimeout

**Test level**: unit

**Agent failure mode checks**:
- Zero value vs missing: Go's YAML unmarshaling sets int to 0 by default — must distinguish between "not set" (use default 30) and "explicitly set to 0" (fully autonomous). Use a pointer `*int` or add a separate `TimeoutSet bool` field.

---

### T008: Frontend — API Client Functions

**Files**: `ui/src/api/client.ts`, `ui/src/types/index.ts`

**Dependencies**: None (can start in parallel with backend, uses TypeScript types)

**Description**: Add TypeScript types for Question and API functions for question CRUD. Update FeatureSummary type to include `pending_questions_count`. Add `waiting_for_human` to STATUS_LABELS.

**Done conditions**:
- Question interface defined in types/index.ts with all fields from the spec
- CreateQuestionRequest interface defined
- AnswerQuestionRequest interface defined
- `listQuestions(featureId: string)` function added to client.ts
- `createQuestion(featureId: string, req: CreateQuestionRequest)` function added
- `answerQuestion(featureId: string, questionId: string, answer: string)` function added
- `listPendingQuestions(featureId: string)` function added
- FeatureSummary type updated with `pending_questions_count: number` field
- STATUS_LABELS updated with `waiting_for_human: "Waiting for Human"`
- FeatureCard statusColors updated with `waiting_for_human` entry
- TypeScript compiles without errors

**Test level**: unit (TypeScript compilation)

**Agent failure mode checks**:
- Type safety: ensure all API response types match the backend DTOs exactly
- Null handling: Question.answer and Question.assumption are `string | null`, not `string`

---

### T009: Frontend — QuestionCard Component

**Files**: `ui/src/components/QuestionCard.tsx`

**Dependencies**: T008 (frontend API types)

**Description**: Create QuestionCard React component that displays a question with answer input and status indicators.

**Done conditions**:
- QuestionCard renders question text with type badge
- Type badges are color-coded: clarification=blue, decision=orange, priority=purple
- When status is "pending": shows text input and submit button
- When options exist: shows clickable option buttons that populate the answer input
- Clicking an option button fills the answer field with the option text (doesn't auto-submit)
- Submit button calls `answerQuestion` API, then triggers refetch
- When status is "answered": shows answer in read-only state with green checkmark
- When status is "assumed": shows assumption in read-only state with "auto-assumed" label
- Error handling: shows error toast if answer submission fails (409 conflict, network error)
- Empty state: if question has no options, only text input is shown
- Component accepts Question as prop, calls onAnswered callback on success
- E2E test: create question via API, render QuestionCard, answer via UI, verify card updates

**Test level**: e2e

**Agent failure mode checks**:
- XSS prevention: question text and answer text are rendered as text, not dangerouslySetInnerHTML
- Loading state: submit button is disabled while API call is in progress
- Error state: 409 conflict shows "Question already answered" message, not a crash

---

### T010: Frontend — QuestionBadge Component and FeatureCard Integration

**Files**: `ui/src/components/QuestionBadge.tsx`, `ui/src/components/FeatureCard.tsx`

**Dependencies**: T008 (frontend API types), T009 (QuestionCard for consistency)

**Description**: Create QuestionBadge component and integrate it into FeatureCard to show pending question count.

**Done conditions**:
- QuestionBadge component renders a small badge with the pending question count
- Badge color: yellow/orange (bg-yellow-500 text-white or similar)
- Badge is positioned top-right of the FeatureCard
- Badge only visible when `pending_questions_count > 0`
- Badge count matches `pending_questions_count` from FeatureSummaryResponse
- Clicking the badge navigates to the feature detail page (same behavior as clicking the card)
- Badge is hidden when no pending questions exist (not shown with "0")
- FeatureCard imports and renders QuestionBadge when feature has pending questions
- E2E test: create feature with questions, load dashboard, verify badge shows correct count

**Test level**: e2e

**Agent failure mode checks**:
- Conditional rendering: badge must not render when `pending_questions_count === 0` or is undefined
- Z-index: badge must be visible above the card content

---

### T011: Frontend — FeatureDetail Question Section

**Files**: `ui/src/pages/FeatureDetail.tsx`

**Dependencies**: T009 (QuestionCard), T010 (QuestionBadge), T008 (API functions)

**Description**: Add a "Questions" section to the FeatureDetail page that shows all questions for the feature, with the ability to answer them.

**Done conditions**:
- FeatureDetail page fetches questions for the current feature using `listQuestions`
- Questions section is visible when feature has questions (any status)
- Questions section is hidden when feature has no questions
- Each question is rendered as a QuestionCard component
- When `feature.status === "waiting_for_human"`, a prominent banner says "This feature is waiting for your input"
- When all questions are answered, a message says "All questions answered. Pipeline will resume."
- Answering a question refetches the question list and updates the badge on Dashboard
- Questions section appears below the Action Buttons section
- SSE updates: when a question is answered, the question list refreshes
- Empty state: if questions section is hidden when no questions exist, no empty state message shown
- E2E test: create feature with questions, navigate to detail page, verify questions shown, answer one, verify card updates

**Test level**: e2e

**Agent failure mode checks**:
- Loading state: show loading spinner while fetching questions
- Error state: show error message if question fetch fails, don't crash
- Refetch after answer: use TanStack Query invalidation to refresh question list

---

### T012: Integration — Full Pipeline Flow with Questions

**Files**: `internal/pipeline/pipeline_test.go`

**Dependencies**: T001-T007 (all backend tasks)

**Description**: End-to-end integration tests verifying the full pipeline flow with question detection, waiting, answering, and resumption.

**Done conditions**:
- Test: feature enters waiting_for_human when questions.json is present after inception phase dispatch
- Test: feature stays in_progress when no questions.json after inception dispatch
- Test: feature enters waiting_for_human when questions.json is present after planning phase dispatch
- Test: feature in construction phase does NOT enter waiting_for_human even with questions.json
- Test: when timeout expires, pending questions are assumed and feature returns to in_progress
- Test: when timeout is 0, feature never enters waiting_for_human and questions are immediately assumed
- Test: when timeout is -1, feature enters waiting_for_human and stays indefinitely (no auto-assume)
- Test: when all questions are answered, feature returns to in_progress
- Test: CONTEXT.md includes Human Responses section on re-dispatch after questions answered
- Test: CONTEXT.md includes assumptions labeled "auto-assumed after timeout" when timeout expires
- Test: on recirculation, all questions are deleted
- Test: feature in waiting_for_human cannot be advanced (returns 400)
- Test: feature in waiting_for_human can be cancelled
- Test: SSE events broadcast waiting_for_human status change
- All tests pass with `go test ./internal/pipeline/...`

**Test level**: integration

**Agent failure mode checks**:
- Goroutine cleanup: verify timeout goroutines are cleaned up after test
- Race conditions: run tests with `-race` flag
- File system: use temp directories for test data, clean up after tests

---

## P2 Tasks (Should-Have)

### T013: Frontend — SSE Event Handling for Questions

**Files**: `ui/src/hooks/useSSE.ts`, `ui/src/pages/FeatureDetail.tsx`

**Dependencies**: T011 (FeatureDetail question section), T005 (pipeline SSE events)

**Description**: Handle SSE events for question status changes so the UI updates in real-time when questions are answered or assumed.

**Done conditions**:
- SSE event `waiting_for_human` triggers feature state refresh and question list refresh
- SSE event `questions_answered` triggers question list refresh
- SSE event `questions_assumed` triggers question list refresh
- When all questions are answered via the UI, the question list updates without manual page refresh
- When questions are answered by another source (e.g., timeout), SSE updates the question list
- No console errors when SSE events arrive for question changes

**Test level**: e2e

**Agent failure mode checks**:
- SSE reconnection: verify useSSE hook handles reconnection
- Concurrent updates: verify no race conditions when multiple SSE events arrive quickly

---

### T014: Frontend — Question Section for Features Not in waiting_for_human

**Files**: `ui/src/pages/FeatureDetail.tsx`

**Dependencies**: T011 (FeatureDetail question section)

**Description**: Show answered/assumed questions on the FeatureDetail page even when the feature is no longer in `waiting_for_human` status, so users can see what questions were asked and how they were resolved.

**Done conditions**:
- Question section is visible for features that have questions in any status (not just `waiting_for_human`)
- Answered questions show the answer with a green checkmark
- Assumed questions show the assumption with a "auto-assumed" label
- No answer input is shown for questions that are already answered or assumed
- Empty state: section hidden when no questions exist (no empty state message)

**Test level**: e2e

---

## Task Dependency Graph

```
T001 ──┐
       ├── T003 ──┐
T002 ──┤          ├── T005 ── T012
       ├── T004 ──┘
T007 ──┘

T008 ──┐
       ├── T009 ── T011
       ├── T010 ──┘
       └──────────┘

T006 ── (depends on T001 for Question types)
```

**Parallel opportunities**:
- T001 + T002 + T007 can run in parallel (no dependencies)
- T008 can start in parallel with backend tasks (TypeScript types)
- T004 can start once T001 is complete
- T003 can start once T001 and T002 are complete
- T009, T010 can start once T008 is complete

**Critical path**: T001 → T004 → T005 → T012 (backend) | T008 → T009 → T011 (frontend)

## Quality Checkpoints

### After T001: Question model and store
- [ ] `go test ./internal/feature/...` passes
- [ ] Empty questions.json returns `[]` not `null`
- [ ] Question ID generation is sequential within a feature

### After T002: Status transitions
- [ ] `go test ./internal/feature/...` passes
- [ ] All valid and invalid transitions are tested
- [ ] `AdvanceTo` blocks `waiting_for_human` status

### After T003: API endpoints
- [ ] `go test ./internal/api/...` passes
- [ ] All 4 endpoints respond with correct status codes
- [ ] Error responses use `{"error": "code", "details": "message"}` format
- [ ] CORS includes PATCH method

### After T004: Question detection
- [ ] `go test ./internal/pipeline/...` passes
- [ ] Invalid JSON in questions.json doesn't crash the pipeline
- [ ] Missing questions.json is handled gracefully

### After T005: Pipeline integration
- [ ] `go test ./internal/pipeline/...` passes
- [ ] Feature enters `waiting_for_human` when questions exist in inception/planning
- [ ] Feature does NOT enter `waiting_for_human` in construction/review/testing/delivery
- [ ] Timeout goroutine is cancellable and doesn't leak

### After T006: Context injection
- [ ] `go test ./internal/rules/...` passes
- [ ] CONTEXT.md includes "Human Responses" section with correct format
- [ ] Features with no questions don't have empty "Human Responses" section

### After T007: Config
- [ ] `go test ./internal/config/...` passes
- [ ] Default timeout is 30 minutes
- [ ] Zero timeout means fully autonomous

### After T011: Frontend question section
- [ ] QuestionCard renders correctly for all three statuses
- [ ] QuestionBadge shows correct count
- [ ] Answering a question updates the UI
- [ ] No JavaScript console errors

### After T012: Full integration
- [ ] Full pipeline flow works: create feature → dispatch → questions → waiting_for_human → answer → resume
- [ ] Timeout flow works: create feature → dispatch → questions → wait → assume → resume
- [ ] `go test -race ./...` passes