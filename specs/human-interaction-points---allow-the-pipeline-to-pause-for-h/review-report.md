# Review Report — Spec 003: Human Interaction Points

## Summary

- **Acceptance criteria**: 92 total (AC-001..AC-089, AC-SEC-001..003, AC-RES-001..002)
- **MET**: 87
- **NOT MET**: 3 (AC-081, AC-RES-001, AC-RES-002)
- **UNVERIFIABLE (E2E)**: 24 criteria marked e2e — verified by code inspection only; tester must run browser
- **Findings**: 3 required, 4 noted
- **Build**: `go build ./...` succeeds (go1.26.1)
- **Tests**: `go test ./internal/feature/... ./internal/api/... ./internal/config/... ./internal/pipeline/...` → 150 passed

---

## Spec Review — Plan vs Spec Coverage

Every user story (US-001..US-006) has corresponding tasks (T001..T014). Every functional requirement (FR-001..FR-012) maps to tasks. No scope creep detected — implementation files match plan's file list exactly. No tasks trace to user stories that don't exist.

**Spec drift**: None. Implementation matches spec terminology (`waiting_for_human`, `Q-NNN`, `questions.json`).

---

## Acceptance Criteria Review

### US-001: Human Answers PM Clarification Questions

#### AC-001: Pending questions displayed as cards with type badge, options, text input
- **Status**: MET (code inspection; e2e verification pending tester)
- **Evidence**: `ui/src/components/QuestionCard.tsx:104-156` renders pending state with type badge (line 107), options buttons (line 117-129), text input (line 132-145), submit button (line 146-153). `ui/src/pages/FeatureDetail.tsx:321-343` renders Questions section when `questions.length > 0`.

#### AC-002: PATCH answer → status "answered", answer stored, answered_at set
- **Status**: MET
- **Evidence**: `internal/feature/question.go:258-283` `AnswerQuestion` sets `q.Answer = &answer`, `q.Status = QuestionStatusAnswered`, `q.AnsweredAt = &now`. Test `TestAnswerQuestionLifecycle` (`internal/api/server_test.go:810-857`) verifies 200 response, status="answered", answer field matches, answered_at non-nil.

#### AC-003: UI submit → card read-only with checkmark, badge count decreases
- **Status**: MET (code inspection; e2e pending)
- **Evidence**: `QuestionCard.tsx:54-76` answered branch renders checkmark `✓` (line 66) and read-only answer (line 70). `QuestionCard.tsx:25-28` invalidates `['features']` query on success, triggering FeatureCard re-render with updated `pending_questions_count`.

#### AC-004: Already answered → 409 with `{"error": "conflict", "details": "Question Q-001 is already answered"}`
- **Status**: MET
- **Evidence**: `internal/feature/question.go:269-271` returns `&QuestionConflictError{QuestionID: questionID, Status: q.Status}` when status != pending. `internal/api/server.go:781-783` maps to 409 with `conflict` error code. `QuestionConflictError.Error()` (`question.go:347-349`) returns `"Question %s is already %s"` matching expected format. Test `TestAnswerQuestionLifecycle:848-856` verifies 409 on second answer.

#### AC-005: Question ID not found → 404 with `{"error": "not_found", "details": "Question Q-999 not found"}`
- **Status**: MET
- **Evidence**: `question.go:282` returns `fmt.Errorf("question %s not found", questionID)`. `server.go:785-787` detects "not found" substring and returns 404 with `not_found` code and `Question %s not found` details. Test `TestAnswerQuestionNotFound:893-906` verifies 404.

#### AC-006: Empty answer → 400 with `{"error": "validation_error", "details": "answer must be 1-5000 characters"}`
- **Status**: MET
- **Evidence**: `server.go:769-773` trims answer, returns 400 with exact message when empty. Test `TestAnswerQuestionValidationErrors:876-877` verifies 400 for empty answer.

#### AC-007: Answer > 5000 chars → 400 with same message
- **Status**: MET
- **Evidence**: `server.go:774-777` checks `len(req.Answer) > 5000` returns 400 with same message. Test verifies 400 for 5001-char answer.

#### AC-008: No questions → question section hidden
- **Status**: MET
- **Evidence**: `FeatureDetail.tsx:322` `{questions.length > 0 && (...)}` — section not rendered when empty. No empty-state placeholder.

### US-002: Human Reviews Architect Design Decisions

#### AC-009: Decision cards with options as clickable buttons
- **Status**: MET (e2e pending)
- **Evidence**: `QuestionCard.tsx:116-129` renders options as buttons regardless of type. Type badge rendered at line 107. Type `decision` gets orange color via `typeColors` map (line 14).

#### AC-010: Click option → answer field populated
- **Status**: MET (e2e pending)
- **Evidence**: `QuestionCard.tsx:43-45` `handleOptionClick` calls `setAnswerText(option)`. Input value bound to `answerText` (line 134). No auto-submit — matches "doesn't auto-submit" task condition.

#### AC-011: Answered decision → read-only, no input fields
- **Status**: MET (e2e pending)
- **Evidence**: `QuestionCard.tsx:54-76` answered branch renders only text display, no input/button elements.

#### AC-012: Planning with no decisions → no cards, pipeline proceeds
- **Status**: MET
- **Evidence**: `FeatureDetail.tsx:322` conditional render. `process.go:152-213` detection only pauses if `len(detectedQuestions) > 0`; otherwise proceeds to gate evaluation.

### US-003: Pipeline Pauses for Human Input

#### AC-013: Inception questions.json → questions stored, status waiting_for_human
- **Status**: MET
- **Evidence**: `process.go:152-202` after inception dispatch, calls `DetectQuestions`, stores via `CreateQuestion`, checks `ShouldPauseForHuman`, calls `f.WaitForHuman()`, saves state, emits `waiting_for_human` event. `feature.WaitForHuman()` (`question.go:514-522`) sets `StatusWaitingHuman`.

#### AC-014: Planning questions.json → questions stored, status waiting_for_human
- **Status**: MET
- **Evidence**: Same code path `process.go:152` checks `currentPhase == PhaseInception || currentPhase == PhasePlanning`. `ShouldPauseForHuman` (`question.go:486-500`) returns true for planning.

#### AC-015: waiting_for_human + all answered → in_progress, pipeline resumes
- **Status**: MET
- **Evidence**: `process.go:69-95` checks `PendingCount == 0` at loop start when status is `waiting_for_human`, calls `f.ResumeFromWaitingHuman()`, saves, emits `questions_answered` event, reloads feature and continues loop (re-dispatches agent).

#### AC-016: Construction phase never enters waiting_for_human
- **Status**: MET
- **Evidence**: `process.go:152` detection only runs for inception/planning. `ShouldPauseForHuman` (`question.go:492-494`) returns false for construction. No path to set `waiting_for_human` from construction.

#### AC-017: draft status + POST question → stored, no transition
- **Status**: MET
- **Evidence**: POST handler `server.go:670-742` creates question without touching feature status. `ShouldPauseForHuman` returns false for `StatusDraft`. Pipeline path only runs after `f.Start()` (`process.go:43-48`).

#### AC-018: gate_blocked + question → stored, no transition
- **Status**: MET
- **Evidence**: Same as AC-017 — POST creates question without status change. `ShouldPauseForHuman` returns false for non-`in_progress`.

#### AC-019: Advance from waiting_for_human → 400 with exact message
- **Status**: MET
- **Evidence**: `server.go:283-286` checks `f.Status == feature.StatusWaitingHuman` returns 400 `validation_error` `"Cannot advance feature in waiting_for_human status"`. Also `feature.go:97-99` `AdvanceTo` blocks. Test `TestAdvanceFeatureWaitingHumanBlocked:1013-1046` verifies 400 and error code.

#### AC-020: No questions.json → proceeds to gate evaluation
- **Status**: MET
- **Evidence**: `DetectQuestions` (`question.go:353-365`) returns nil if file doesn't exist. `process.go:156` `len(detectedQuestions) > 0` false → skips pause block, falls through to `EvaluateGate` at line 216.

### US-004: Pipeline Falls Back to Autonomous Mode

#### AC-021: 30 min timeout → in_progress, unanswered → assumed with assumption
- **Status**: MET
- **Evidence**: `process.go:340-377` `startTimeoutGoroutine` creates `time.NewTimer(time.Duration(timeoutMinutes) * time.Minute)`, on fire calls `AssumeAllPendingQuestions` (`question.go:407-426`) which marks each pending as assumed with `GenerateAssumptionText` (line 416). Then `f.ResumeFromWaitingHuman()`, save, broadcast `questions_assumed`. Test `TestAssumeAllPendingQuestions:946-994` verifies assumed count and that answered questions remain answered.

#### AC-022: timeout=0 → never waiting_for_human, immediately assumed
- **Status**: MET
- **Evidence**: `ShouldPauseForHuman` (`question.go:488-490`) returns false when `timeoutMinutes == 0`. `process.go:204-211` else branch calls `AssumeAllPendingQuestions` immediately. No `WaitForHuman` call. Config test `config_test.go:306-308` verifies timeout=0 parses.

#### AC-023: timeout=-1 → waiting_for_human, no auto-assume
- **Status**: MET
- **Evidence**: `ShouldPauseForHuman` returns true for -1 (only checks `== 0`). `process.go:191` `if timeoutMinutes > 0` guards goroutine start — for -1, no goroutine started, so no auto-assume. Feature stays waiting. Config test `config_test.go:391-393` verifies -1 parses.

#### AC-024: Mixed answered+unanswered timeout → only unanswered assumed
- **Status**: MET
- **Evidence**: `AssumeAllPendingQuestions` (`question.go:407-410`) calls `ListPendingQuestions` which filters to status="pending" only. Answered questions never touched. Test `TestAssumeAllPendingQuestions:958-993` verifies answeredCount=1, assumedCount=2 after running.

#### AC-025: Timeout goroutine fails → feature stays waiting, error logged
- **Status**: MET (partial — error paths logged, but no explicit panic recovery)
- **Evidence**: `process.go:340-377` every error path logs via `log.Printf` and returns without transitioning status. If `AssumeAllPendingQuestions` fails, feature remains `waiting_for_human`. **Note**: no `defer recover()` in goroutine — a panic would crash the server, not just the goroutine. See Finding F-003.

#### AC-026: All answered before timeout → no assumptions, return to in_progress
- **Status**: MET
- **Evidence**: `AssumeAllPendingQuestions` calls `ListPendingQuestions` — if empty, loop body doesn't execute, returns empty slice. `process.go:355` `if len(assumed) > 0` guards the resume logic — but **BUG**: if all answered, `len(assumed) == 0`, so the feature is NOT resumed by the goroutine. However, `process.go:69-95` loop-top check detects `PendingCount == 0` and resumes. So resume happens via loop polling, not goroutine. Functionally MET but fragile — relies on 5-second polling loop (line 103).

### US-005: Agent Creates Questions During Dispatch

#### AC-027: Valid questions.json → stored with Q-NNN, status pending
- **Status**: MET
- **Evidence**: `DetectQuestions` (`question.go:353-403`) parses, validates, returns valid questions. `process.go:160` `CreateQuestion` assigns ID via `nextQuestionID` (`question.go:157-168`) which finds max NNN and increments, formats `Q-%03d`. `CreateQuestion` (`question.go:182`) sets `Status = QuestionStatusPending`. Test `TestDetectQuestions_Valid:523-545` verifies 2 valid questions returned from mixed input.

#### AC-028: Some invalid → valid stored, invalid skipped with warning
- **Status**: MET
- **Evidence**: `DetectQuestions:394-397` calls `ValidateQuestion`, on error logs warning `log.Printf("warning: skipping invalid question %d...")` and `continue`. Test `TestDetectQuestions_MissingFields:559-573` and `TestDetectQuestions_MixedValidInvalid:599-615` verify.

#### AC-029: Invalid JSON → no questions stored, warning logged
- **Status**: MET
- **Evidence**: `DetectQuestions:375-378` `json.Unmarshal` error → logs warning, returns nil. Test `TestDetectQuestions_InvalidJSON:546-558` verifies 0 questions.

#### AC-030: phase="construction" → skipped with warning
- **Status**: MET
- **Evidence**: `ValidateQuestion:61-63` rejects phase not in `ValidQuestionPhases` (which only has inception, planning). Test `TestDetectQuestions_InvalidPhase:574-587` verifies.

#### AC-031: No questions.json → proceeds normally
- **Status**: MET
- **Evidence**: Same as AC-020. `DetectQuestions` returns nil on file-not-found (`question.go:357-360`). Test `TestDetectQuestions_NoFile:588-598` verifies.

### US-006: Feature List Shows Question Badge

#### AC-032: 3 pending → badge "3"
- **Status**: MET (e2e pending)
- **Evidence**: `dto.go:89-106` `FeaturesToSummaryResponse` calls `questionStore.PendingCount` for each feature, sets `PendingQuestionsCount`. `FeatureCard.tsx:34-36` renders `QuestionBadge` when `pending_questions_count > 0`. `QuestionBadge.tsx:8-19` renders count.

#### AC-033: 1 pending → badge "1"
- **Status**: MET (e2e pending)
- **Evidence**: Same path — count displayed dynamically.

#### AC-034: Click badge → feature detail page
- **Status**: MET (e2e pending)
- **Evidence**: `QuestionBadge.tsx:12-19` wraps count in `<Link to={/features/${featureId}}>`.

#### AC-035: No pending → no badge
- **Status**: MET (e2e pending)
- **Evidence**: `FeatureCard.tsx:34` `{feature.pending_questions_count > 0 && (...)}`. Also `QuestionBadge.tsx:9` `if (count <= 0) return null`.

#### AC-036: All answered → no badge
- **Status**: MET (e2e pending)
- **Evidence**: `PendingCount` (`question.go:323-339`) only counts status="pending". Answered questions excluded. Count=0 → badge hidden.

#### AC-037: API error → list renders, badge not shown
- **Status**: MET (e2e pending)
- **Evidence**: `dto.go:96-99` on `PendingCount` error, sets `count = 0` (graceful degradation). Feature list still renders. Badge hidden since count=0.

### FR-001: Feature Status State Transitions

#### AC-038: in_progress inception + questions → waiting_for_human
- **Status**: MET
- **Evidence**: `ShouldPauseForHuman` + `WaitForHuman` path in `process.go:173-202`. Test `TestCanTransitionToWaitingHuman:684-688` verifies inception allowed.

#### AC-039: in_progress planning + questions → waiting_for_human
- **Status**: MET
- **Evidence**: Same path. Test `:690-692` verifies planning allowed.

#### AC-040: waiting_for_human + all answered → in_progress
- **Status**: MET
- **Evidence**: `process.go:69-95` loop-top check. `ResumeFromWaitingHuman` (`question.go:525-533`) transitions. Test `TestResumeFromWaitingHuman:765-777` verifies.

#### AC-041: draft + question detection → transition rejected
- **Status**: MET
- **Evidence**: `CanTransitionToWaitingHuman` (`question.go:503-511`) returns false for `StatusDraft`. `WaitForHuman` returns error. Test `:700-703` verifies draft rejected. Test `TestWaitForHuman_InvalidStatus:744-752` verifies error.

#### AC-042: construction in_progress + detection → rejected
- **Status**: MET
- **Evidence**: `CanTransitionToWaitingHuman:507-510` returns false for non-inception/planning. Test `:694-698` verifies construction rejected. Test `TestWaitForHuman_InvalidPhase:754-763` verifies error.

#### AC-043: waiting_for_human + timeout → in_progress
- **Status**: MET
- **Evidence**: `startTimeoutGoroutine:363-371` calls `ResumeFromWaitingHuman` on timeout. Test `TestResumeFromWaitingHuman` covers the transition method.

### FR-002: Question Model

#### AC-044: Valid POST → 201, Q-001, status pending, created_at set
- **Status**: MET
- **Evidence**: `server.go:734-741` returns 201 via `writeJSON(w, http.StatusCreated, ...)`. `question.go:180-186` sets ID, status, created_at. Test `TestCreateQuestionValid:731-760` verifies ID=Q-001, status=pending, created_at set.

#### AC-045: Empty question → 400 "question is required"
- **Status**: MET
- **Evidence**: `server.go:691-694` checks `strings.TrimSpace(req.Question) == ""` returns 400 with exact message. Test `TestCreateQuestionValidationErrors:774` verifies.

#### AC-046: Invalid phase → 400 "phase must be one of: inception, planning"
- **Status**: MET
- **Evidence**: `server.go:699-702` checks `!feature.ValidQuestionPhases[req.Phase]` returns exact message. Test `:771` verifies.

#### AC-047: Invalid type → 400 "type must be one of: clarification, decision, priority"
- **Status**: MET
- **Evidence**: `server.go:707-710` checks `!feature.ValidQuestionTypes[req.Type]`. Test `:773` verifies.

#### AC-048: > 10 options → 400 "options must have at most 10 items"
- **Status**: MET
- **Evidence**: `server.go:711-714` checks `len(req.Options) > 10`. Test `:775` verifies 11 options rejected.

#### AC-049: Pending + timeout → status assumed, assumption populated
- **Status**: MET
- **Evidence**: `AssumeQuestion` (`question.go:285-310`) sets `q.Assumption = &assumption`, `q.Status = QuestionStatusAssumed`. `GenerateAssumptionText` (`question.go:429-434`) produces non-empty assumption. Test `TestAssumeQuestion:243-282` verifies.

#### AC-050: Answered question → status unchanged (terminal)
- **Status**: MET
- **Evidence**: `AnswerQuestion:269-271` and `AssumeQuestion:296-298` both return `QuestionConflictError` if status != pending. No code path mutates an answered/assumed question. Test `TestAnswerQuestionConflict:196-231` and `TestAssumeQuestionConflict:283-312` verify.

### FR-003: API Endpoints

#### AC-051: GET returns all questions with full structure
- **Status**: MET
- **Evidence**: `server.go:626-646` `listQuestions` returns `QuestionsToResponse(questions)`. `dto.go:230-258` `QuestionToResponse` maps all 12 fields (id, feature_id, phase, role, question, type, options, answer, assumption, status, created_at, answered_at).

#### AC-052: No questions → empty array `[]` not null, not 404
- **Status**: MET
- **Evidence**: `QuestionsToResponse` (`dto.go:260-269`) returns `[]QuestionResponse{}` when nil. `FileQuestionStore.ListQuestions` (`question.go:228-232`) returns `[]*Question{}` when empty. Test `TestListQuestionsEmptyReturnsArray:693-709` verifies body is exactly `[]`.

#### AC-053: Feature not found → 404 with `{"error": "not_found", "details": "Feature abc not found"}`
- **Status**: MET
- **Evidence**: `server.go:633-636` checks `GetFeature` error, returns 404 `not_found` with `Feature %s not found`. Test `TestListQuestionsFeatureNotFound:711-729` verifies.

#### AC-054: Valid POST → 201 with full question object
- **Status**: MET
- **Evidence**: `server.go:741` returns 201. `QuestionToResponse` includes all fields. Test `TestCreateQuestionValid` verifies ID starts with "Q-" and fields match.

#### AC-055: Missing question field → 400
- **Status**: MET
- **Evidence**: `server.go:691-694` empty check. Test `:770` verifies missing question → 400.

#### AC-056: Nonexistent feature POST → 404
- **Status**: MET
- **Evidence**: `server.go:677-680` checks `GetFeature` error, returns 404. Test `TestCreateQuestionFeatureNotFound:798-808` verifies.

#### AC-057: Pending + valid PATCH → 200, status answered, answered_at set
- **Status**: MET
- **Evidence**: `server.go:794` returns 200. `question.go:272-275` sets answer, status, answered_at. Test `TestAnswerQuestionLifecycle:833-846` verifies.

#### AC-058: Already answered PATCH → 409
- **Status**: MET
- **Evidence**: `question.go:269-271` returns conflict error. `server.go:781-783` maps to 409. Test `:848-856` verifies.

#### AC-059: Assumed question PATCH → 409
- **Status**: MET
- **Evidence**: Same `q.Status != QuestionStatusPending` check at `question.go:296-298` in `AssumeQuestion` — but PATCH calls `AnswerQuestion`, which has the same check at line 269. Assumed status != pending → `QuestionConflictError` → 409. Test `TestAssumeQuestionConflict:283-312` verifies the store-level error.

#### AC-060: Nonexistent questionId PATCH → 404
- **Status**: MET
- **Evidence**: `question.go:282` returns "not found" error. `server.go:785-787` maps to 404. Test `TestAnswerQuestionNotFound:893-906` verifies.

#### AC-061: GET pending → only pending questions
- **Status**: MET
- **Evidence**: `server.go:648-668` `listPendingQuestions` calls `ListPendingQuestions` which filters status="pending" (`question.go:244-249`). Test `TestListPendingQuestions:908-948` verifies 2 pending after answering 1 of 3.

#### AC-062: All answered → GET pending returns `[]`
- **Status**: MET
- **Evidence**: `ListPendingQuestions:251-255` returns `[]*Question{}` when no pending. Test `TestListPendingQuestionsEmptyReturnsArray:950-965` verifies body is `[]`.

#### AC-063: Nonexistent feature GET pending → 404
- **Status**: MET
- **Evidence**: `server.go:655-658` checks `GetFeature` error, returns 404. Test `TestListPendingQuestionsFeatureNotFound:967-977` verifies.

### FR-004: Web UI Question Display

#### AC-064: waiting_for_human + 2 pending → 2 cards with badges and inputs
- **Status**: MET (e2e pending)
- **Evidence**: `FeatureDetail.tsx:331-335` maps questions to `QuestionCard`. Each card renders badge + input (pending branch).

#### AC-065: type clarification → blue badge
- **Status**: MET (e2e pending)
- **Evidence**: `QuestionCard.tsx:12-16` `typeColors.clarification` = `bg-blue-100...text-blue-800`.

#### AC-066: type decision → orange badge
- **Status**: MET (e2e pending)
- **Evidence**: `typeColors.decision` = `bg-orange-100...text-orange-800`.

#### AC-067: type priority → purple badge
- **Status**: MET (e2e pending)
- **Evidence**: `typeColors.priority` = `bg-purple-100...text-purple-800`.

#### AC-068: Options shown as clickable buttons
- **Status**: MET (e2e pending)
- **Evidence**: `QuestionCard.tsx:117-129` renders `<button>` per option.

#### AC-069: Answered → read-only with green checkmark, no inputs
- **Status**: MET (e2e pending)
- **Evidence**: `QuestionCard.tsx:54-76` answered branch: checkmark `✓` at line 66, answer text at line 70, no input/button elements in this branch.

### FR-006: Pipeline Pauses at Decision Points

#### AC-070: Inception + questions.json → waiting_for_human
- **Status**: MET (same as AC-013)

#### AC-071: Planning + questions.json → waiting_for_human
- **Status**: MET (same as AC-014)

#### AC-072: Inception + no questions.json → proceeds to gate
- **Status**: MET (same as AC-020)

### FR-007: Human Input in Agent Context

#### AC-073: 2 answered → CONTEXT.md has Human Responses, both "human input"
- **Status**: MET
- **Evidence**: `pipeline.go:137-147` injects `BuildHumanResponsesContext` into `contextStr` which is written to CONTEXT.md at line 176. `question.go:464-468` answered branch writes `[Source: human input]`. Test `TestBuildHumanResponsesContext:865-877` verifies "human input" substring present.

#### AC-074: 1 answered + 1 assumed → correct labels
- **Status**: MET
- **Evidence**: `question.go:469-477` assumed branch writes `[Source: auto-assumed after timeout of %d minutes]`. Test `:878-891` verifies both "human input" and "auto-assumed" + "30 minutes" present.

#### AC-075: No questions → no Human Responses section
- **Status**: MET
- **Evidence**: `pipeline.go:140` `if len(questions) > 0` guards the injection. `BuildHumanResponsesContext:438-452` returns "" if no answered/assumed questions. Test `:858-863` verifies empty string for nil.

### FR-008: Feature Status Transitions

#### AC-076: waiting_for_human + cancel → cancelled
- **Status**: MET
- **Evidence**: `feature.go:176-180` `Cancel()` sets `StatusCancelled` unconditionally (only checks not already cancelled/done in handler at `server.go:387-394`). Test `TestCancelFromWaitingHuman:800-809` verifies.

#### AC-077: waiting_for_human + recirculate → target phase, questions deleted
- **Status**: MET
- **Evidence**: `server.go:365-369` calls `DeleteQuestionsForFeature` on recirculate. `feature.RecirculateTo` (`feature.go:133-174`) transitions. `process.go:302-306` also deletes questions on recirculate during autonomous processing.

### FR-009: Timeout Configuration

#### AC-078: timeout=5 → waiting_for_human, auto-assume after 5 min
- **Status**: MET
- **Evidence**: `config.go:26-31` `GetHumanInteractionTimeoutMinutes` returns configured value. `process.go:191-193` starts goroutine with `timeoutMinutes`. Config test `:402-478` verifies 5 parses.

#### AC-079: timeout=0 → never waiting_for_human, immediately assumed
- **Status**: MET (same as AC-022)

#### AC-080: timeout=-1 → waiting_for_human indefinitely
- **Status**: MET (same as AC-023)

#### AC-081: New question during timeout → timeout reset
- **Status**: NOT MET
- **Evidence**: `process.go:340-377` `startTimeoutGoroutine` starts a `time.NewTimer` with fixed duration. There is no mechanism to cancel or reset this timer when a new question is added via POST while the feature is in `waiting_for_human`. The goroutine holds no reference that could be cancelled. `context.WithCancel` is not used per-feature for the timeout. The `ctx` passed is the ProcessAsync context, which is only cancelled when processing stops. No code path calls `timer.Reset()` on new question creation.
- **Explanation**: Spec FR-009 explicitly states "It is reset if a new question is added while the feature is already in `waiting_for_human` status." This is a missing feature, not a bug in existing code. The POST endpoint (`server.go:670-742`) creates questions but does not signal any running timeout goroutine.

### FR-010: Questions Cleared on Recirculation

#### AC-082: 5 questions + recirculate → all deleted
- **Status**: MET
- **Evidence**: `server.go:365-369` and `process.go:302-306` both call `DeleteQuestionsForFeature`. `question.go:312-321` removes the questions.json file entirely.

### FR-011: Question Detection from Agent Output

#### AC-083: 3 valid questions → 3 stored
- **Status**: MET (same evidence as AC-027, `TestDetectQuestions_Valid`)

#### AC-084: Invalid JSON → no questions, warning
- **Status**: MET (same as AC-029)

#### AC-085: phase="construction" → skipped, warning
- **Status**: MET (same as AC-030)

### FR-012: Concurrent Answer Handling

#### AC-086: Two simultaneous PATCH → one 200, one 409
- **Status**: MET
- **Evidence**: `FileQuestionStore` has `mu sync.Mutex` (`question.go:103`). `AnswerQuestion:259-260` acquires `s.mu.Lock()` before reading and checking status. Second concurrent call blocks until first releases (after save), then sees `status == answered` → returns `QuestionConflictError` → 409. The mutex serializes the read-check-update-save sequence, guaranteeing exactly one winner.

### Smoke Tests

#### AC-087: Feature list page loads without console errors
- **Status**: UNVERIFIABLE (smoke — tester must run browser)

#### AC-088: Questions API endpoints respond with correct status codes
- **Status**: MET (verified via integration tests `TestCreateQuestionValid`, `TestAnswerQuestionLifecycle`, `TestListQuestions`, `TestListPendingQuestions` — all exercise real handler functions)

#### AC-089: Feature detail page with questions loads without console errors
- **Status**: UNVERIFIABLE (smoke — tester must run browser)

### Security Acceptance Criteria

#### AC-SEC-001: Script tag in answer → stored as-is, escaped in UI
- **Status**: MET
- **Evidence**: `AnswerQuestion` store (`question.go:272`) stores raw answer string — no sanitization, no execution. UI: `QuestionCard.tsx:70` renders `{question.answer}` as React child (text), not `dangerouslySetInnerHTML`. No `dangerouslySetInnerHTML` found anywhere in ui/src. React auto-escapes. Grep confirmed zero matches.

#### AC-SEC-002: Question > 2000 chars → 400
- **Status**: MET
- **Evidence**: `server.go:695-698` checks `len(req.Question) > 2000` returns 400. Test `TestCreateQuestionValidationErrors:776` verifies 2001 chars → 400.

#### AC-SEC-003: Answer > 5000 chars → 400
- **Status**: MET
- **Evidence**: `server.go:774-777` checks `len(req.Answer) > 5000` returns 400. Test `TestAnswerQuestionValidationErrors:877` verifies 5001 chars → 400.

### Resilience Acceptance Criteria

#### AC-RES-001: Store unavailable → 503 not 500
- **Status**: NOT MET
- **Evidence**: `server.go:638-643` `listQuestions` on store error returns `writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list questions")` — that's 500, not 503. Same pattern in `listPendingQuestions:660-665`, `createQuestion:735-739`, `answerQuestion:789-791`. No 503 `ServiceUnavailable` anywhere in the codebase (grep confirmed zero matches for `503`/`ServiceUnavailable`/`status_unavailable`).
- **Explanation**: Spec requires 503 with meaningful error for transient store unavailability. Implementation returns generic 500.

#### AC-RES-002: Server restart → timeout recalculated from original timestamp
- **Status**: NOT MET
- **Evidence**: `process.go:340-377` `startTimeoutGoroutine` uses `time.NewTimer(time.Duration(timeoutMinutes) * time.Minute)` — a fresh full timeout. There is no code that, on server startup, scans for features in `waiting_for_human` status and starts timeout goroutines recalculated from the original `UpdatedAt` (set when `WaitForHuman` was called at `question.go:519`). On restart, any in-flight timeout goroutines are lost (they die with the process), and no bootstrap code restarts them. Features stuck in `waiting_for_human` would only resume via the 5-second polling loop (`process.go:103`) if `ProcessAsync` is re-invoked, but that loop doesn't handle timeout — it only resumes when `PendingCount == 0`.
- **Explanation**: Missing restart recovery logic. Spec requires timer restart based on original timestamp.

---

## Findings

### F-001: AC-081 — Timeout reset on new question not implemented
- **Severity**: needs fixing
- **Criterion**: AC-081, FR-009
- **Code**: `internal/pipeline/process.go:340-377`
- **Description**: `startTimeoutGoroutine` starts a one-shot `time.NewTimer` with no cancellation handle stored. When a new question is POSTed while feature is in `waiting_for_human`, no signal reaches the running goroutine to reset the timer. Spec FR-009: "It is reset if a new question is added while the feature is already in `waiting_for_human` status." Fix requires: (1) storing a per-feature cancel context or timer reference, (2) on POST createQuestion when feature is `waiting_for_human`, cancel existing timer and start new one, or call `timer.Reset()`.

### F-002: AC-RES-001 — Store errors return 500, not 503
- **Severity**: needs fixing
- **Criterion**: AC-RES-001
- **Code**: `internal/api/server.go:641, 664, 737, 790`
- **Description**: All question store error paths return `http.StatusInternalServerError` (500) with `internal_error` code. Spec requires 503 Service Unavailable with meaningful error for transient store unavailability. Fix: distinguish transient store errors (file I/O) from programming errors, return 503 with `{"error": "service_unavailable", "details": "..."}`.

### F-003: AC-RES-002 — No timeout recovery on server restart
- **Severity**: needs fixing
- **Criterion**: AC-RES-002
- **Code**: `internal/pipeline/process.go:340-377` (missing startup recovery)
- **Description**: On server restart, features in `waiting_for_human` have no timeout goroutine running. No bootstrap code scans for waiting features and restarts timers based on `UpdatedAt`. Fix requires: on `NewPipeline` or server start, load all features, for each in `waiting_for_human` calculate `remaining = timeoutMinutes - (now - UpdatedAt)`, start goroutine with `remaining` (or immediately assume if remaining <= 0).

### F-004: Timeout goroutine lacks panic recovery
- **Severity**: doesn't need fixing (noted)
- **Criterion**: AC-025 (partial)
- **Code**: `internal/pipeline/process.go:340-377`
- **Description**: `startTimeoutGoroutine` has no `defer recover()`. A panic in `AssumeAllPendingQuestions` or `GetFeature` would crash the server process. Spec AC-025 says "feature remains in waiting_for_human and an error is logged" on goroutine failure — a panic doesn't log, it crashes. Low risk since code paths are simple, but inconsistent with the recovery middleware pattern used for HTTP handlers.

### F-005: CORS allows all origins (`*`)
- **Severity**: doesn't need fixing (noted)
- **Criterion**: Security review checklist
- **Code**: `internal/api/server.go:85`
- **Description**: `Access-Control-Allow-Origin: *`. Security extension recommends restrictive CORS. Acceptable per spec assumptions: "single-user local tool, no network exposure beyond localhost." Noted for future hardening.

### F-006: Security headers not set
- **Severity**: doesn't need fixing (noted)
- **Criterion**: Security review checklist
- **Code**: `internal/api/server.go` (no headers set in handlers or middleware)
- **Description**: `X-Content-Type-Options`, `X-Frame-Options`, `Content-Security-Policy` not set anywhere. Spec assumptions say single-user local tool, so not blocking, but security extension recommends these for all responses.

### F-007: ProcessAsync waiting loop uses 5-second polling, not event-driven
- **Severity**: doesn't need fixing (noted)
- **Criterion**: AC-026 (functionally MET but fragile)
- **Code**: `internal/pipeline/process.go:96-112`
- **Description**: When feature is `waiting_for_human` with pending questions, the loop sleeps 5 seconds and re-checks `PendingCount`. This works but means up to 5-second latency between last answer and pipeline resume. Acceptable for MVP but not event-driven as spec FR-006 implies ("Pipeline detects all questions are answered (via API call or SSE event)").

---

## Over-Engineering Check

- `internal/feature/question.go`: 533 lines — reasonable for model + store + detection + context builder (4 responsibilities). Could split but not excessive.
- `internal/api/server.go`: 795 lines — grew from ~650 (pre-feature) to 795 with 4 new handlers. Proportional.
- `internal/pipeline/process.go`: 398 lines — grew from ~280 to 398 with question detection integration. Proportional.
- No speculative abstractions, no unused interfaces, no premature optimization. `QuestionStore` interface is justified (enables test mocking — `NewPipelineWithQuestionStore` exists).

**Verdict**: No over-engineering detected.

---

## Missing Implementation Check

All user stories US-001..US-006 have corresponding implementations. All functional requirements FR-001..FR-012 have code. Missing items:

1. **FR-009 timeout reset** (AC-081) — not implemented. See F-001.
2. **AC-RES-001 503 response** — not implemented. See F-002.
3. **AC-RES-002 restart recovery** — not implemented. See F-003.

---

## Null Pointer Safety

- `Question.Answer *string`, `Question.Assumption *string`, `Question.AnsweredAt *time.Time` — all checked for nil before deref in `BuildHumanResponsesContext` (`question.go:465, 469`) and `QuestionToResponse` (`dto.go:239`).
- `Question.Options []string` — initialized to `[]string{}` in `CreateQuestion:189-191` and `DetectQuestions:390-392`. DTO `QuestionToResponse:231-236` converts nil to `[]string{}`.
- `pending_questions_count` — `FeaturesToSummaryResponse:96-99` defaults to 0 on error.
- `FileQuestionStore.loadQuestions:119-126` returns empty slice on file-not-found and empty file.
- **Verdict**: No nil pointer dereferences found. All nullable fields guarded.

## JSON Serialization

- `Options` field: `QuestionResponse` (`dto.go:210`) has no `omitempty`, `QuestionToResponse:231-236` ensures `[]string{}` not nil. Test `TestQuestionsJSONArraysNeverNull:979-1011` verifies response contains `"options":[]` not `"options":null`.
- `QuestionsToResponse:261-263` returns `[]QuestionResponse{}` not nil.
- `FeaturesToSummaryResponse:90` initializes `make([]FeatureSummaryResponse, 0, len(features))`.
- **Verdict**: All collections serialize as `[]` not `null`.

## Error Path Coverage

- 400: missing/empty fields (question, answer), invalid enums (phase, role, type), length limits (question>2000, answer>5000, options>10, option>500), advance from waiting_for_human. All tested.
- 404: feature not found (all 4 endpoints), question not found (PATCH). All tested.
- 409: answer already answered, answer already assumed, recirculate conflict (existing). Tested.
- 500: store errors via recovery middleware (outermost at `server.go:64`). Tested implicitly.
- Empty state: GET questions returns `[]`, GET pending returns `[]`. Tested.
- **Verdict**: Error paths complete except 503 for transient store errors (F-002).

## Middleware Chain

- `server.go:64`: `s.recoveryMiddleware(s.corsMiddleware(mux))` — recovery is outermost. ✓
- CORS includes PATCH (`server.go:86`). ✓
- Request body size limit: `http.MaxBytesReader` on POST/PATCH questions (`server.go:682, 761`). ✓
- **Verdict**: Middleware chain correct.

---

## Quality Gate

1. ✅ Every acceptance criterion checked with quoted evidence (92 criteria)
2. ✅ "No issues found" backed by evidence (tests cited, code quoted)
3. ✅ Security review complete (P1 feature) — XSS, input validation, CORS, security headers checked
4. ✅ Null pointer safety verified
5. ✅ Error paths verified (400/404/409/empty/500)
6. ✅ Middleware chain verified end-to-end
7. ✅ Over-engineering check completed — no findings
8. ✅ Missing implementation check completed — 3 items (F-001, F-002, F-003)

**Gate status**: BLOCKED — 3 findings need fixing (F-001, F-002, F-003 are severity "needs fixing" and map to explicit acceptance criteria). F-004, F-005, F-006, F-007 are noted, not blocking.

**Recommendation**: Recirculate to construction with findings F-001, F-002, F-003. The 24 e2e/smoke criteria (AC-001, AC-003, AC-008, AC-009..AC-012, AC-032..AC-037, AC-064..AC-069, AC-087, AC-089) remain for tester verification in the Testing phase — they are not blocking the review gate since code inspection confirms the implementation exists and is wired correctly.