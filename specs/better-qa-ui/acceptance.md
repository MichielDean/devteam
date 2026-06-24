# Acceptance Criteria — Better Q&A UI

Every criterion is testable at a specific level. Constraint-derived criteria reference their CON- ID. Test levels: `smoke` (service up + page loads), `integration` (API contract via real or mocked backend), `e2e` (browser against running stack on :18765), `unit` (pure component/logic).

## US-001 — Guided Multiple-Choice Answering

AC-001: Given a feature with status `waiting_for_human` and one pending question with options `["A","B","Other"]`, when the user opens the feature detail page, then the question renders three selectable option cards (data-testid `question-option-*`) and NOT a bare text input with buttons.
  Test level: e2e
  Verification: Playwright — assert `[data-testid="question-option-0"]`, `[data-testid="question-option-1"]`, `[data-testid="question-option-2"]` are visible and are not `<input>` elements.

AC-002: Given the question card from AC-001, when the user clicks option "B", then option "B" becomes selected (has a selected indicator class/attribute, e.g. `aria-selected="true"` or `data-selected="true"`), options "A" and "Other" are not selected, and no `PATCH /api/features/{id}/questions/{qid}` request is sent.
  Test level: e2e
  Verification: Playwright — click option 1; assert selected attribute on it, absence on others; assert no network PATCH via `page.route` interception or `requests` collector.

AC-003: Given the feature has 3 pending questions, when the user answers 1 (selects an option and advances), then a progress indicator (data-testid `question-progress`) shows "1 of 3 questions answered".
  Test level: e2e
  Verification: Playwright — assert `[data-testid="question-progress"]` contains "1 of 3".

AC-004: Given an answered question in the history, when the user views the feature, then the answered card (data-testid `question-card-{id}`) shows a checkmark (`question-checkmark`), the phase + role label, the question text (`question-text`), and the answer (`question-answer`).
  Test level: e2e
  Verification: Playwright — assert all four testids present and non-empty; matches existing testids in QuestionCard.tsx.

AC-005: Given an auto-assumed question in the history, when the user views the feature, then the assumed card shows an `auto-assumed` label (`question-auto-assumed-label`), the phase + role, the question, and the assumption (`question-assumption`).
  Test level: e2e
  Verification: Playwright — assert `question-auto-assumed-label` and `question-assumption` visible and non-empty.
  Constraint refs: CON-013.

## US-002 — Progress, Auto-Scroll, and Phase Context

AC-006: Given a feature with 2 pending and 1 answered question, when the page loads, then the progress indicator shows "1 of 3 questions answered".
  Test level: e2e
  Verification: Playwright — seed feature state; assert `[data-testid="question-progress"]` text "1 of 3".
  Constraint refs: CON-005.

AC-007: Given the user just answered question 1 of 2 pending, when the answer is recorded, then the progress indicator updates to "1 of 2 answered" (answered+pending total) and the view auto-scrolls so question 2 (data-testid `question-card-{id2}`) is within the viewport.
  Test level: e2e
  Verification: Playwright — after answer, assert progress text; assert `question-card-{id2}` bounding box intersects viewport via `isVisible()` / `BoundingBox`.
  Constraint refs: CON-005, CON-006.

AC-008: Given the user answered the last pending question, when there are no more pending questions, then the view auto-scrolls so the summary/submit area (data-testid `answer-summary`) is within the viewport.
  Test level: e2e
  Verification: Playwright — answer last; assert `[data-testid="answer-summary"]` visible in viewport.
  Constraint refs: CON-006.

AC-009: Given any question card, when rendered, then it displays a label showing the asking phase (e.g. "Inception") and role (e.g. "PM"). Existing testid `question-type-badge` plus phase/role text.
  Test level: e2e
  Verification: Playwright — assert the phase/role text node next to the badge matches the question's phase/role.
  Constraint refs: CON-004.

## US-003 — Answer Summary and Single Submit

AC-010: Given all questions have been answered (none pending), when the user views the feature, then an inline summary panel (data-testid `answer-summary`) lists each question with its selected/typed answer.
  Test level: e2e
  Verification: Playwright — assert `[data-testid="answer-summary"]` visible and contains one row per question with Q + A text.
  Constraint refs: CON-007.

AC-011: Given the summary is visible, when the user clicks an answer row in the summary, then they can edit that answer (re-select an option or re-type in the textarea) and the draft updates.
  Test level: e2e
  Verification: Playwright — click summary row for a multiple-choice question; re-select a different option; assert summary row updates after returning.
  Constraint refs: CON-007.

AC-012: Given all questions answered and the summary shown, when the user clicks the "Submit Answers & Resume" button (data-testid `submit-answers`), then every answered question is sent via `PATCH /api/features/{id}/questions/{qid}` (one request per question) and the pipeline resumes (feature status leaves `waiting_for_human`).
  Test level: e2e
  Verification: Playwright — intercept PATCH requests; assert one PATCH per question with correct answer body; assert feature status transitions (poll feature or assert SSE `processing_complete`/`phase_change`).
  Constraint refs: CON-008, CON-009.

AC-013: Given the feature is in single-phase mode and all questions answered, when the user clicks Submit, then answers are POSTed and the feature transitions to `in_progress` awaiting manual advance (no agent dispatch begins).
  Test level: integration
  Verification: Backend contract test (or e2e with single-phase mode) — after submit, `GET /api/features/{id}` returns `status: in_progress`, no `agent_dispatch` SSE fires.
  Constraint refs: CON-009.

## US-004 — Open-Ended Question Step

AC-014: Given a pending question with empty `options`, when rendered, then it shows a textarea/input (data-testid `question-answer-input`) and NO option cards (`question-option-*` absent), styled as a wizard step with phase/role label and progress.
  Test level: e2e
  Verification: Playwright — assert `[data-testid="question-answer-input"]` visible; assert `[data-testid^="question-option-"]` count is 0; assert phase/role label and progress present.
  Constraint refs: CON-002, CON-003.

AC-015: Given an open-ended question with a typed answer, when the user navigates to the summary, then the summary shows the question and the typed text.
  Test level: e2e
  Verification: Playwright — type into textarea; open summary; assert summary row contains the typed text.
  Constraint refs: CON-007.

## US-005 — Error and Empty State Handling

AC-016: Given the user submits an empty answer, when the backend returns 400 `validation_error`, then a toast displays the validation message and the wizard remains on the current step (no navigation).
  Test level: integration
  Verification: Force a 400 (empty body / clear draft); assert toast (data-testid `toast-error`) contains the backend message; assert wizard step unchanged.
  Constraint refs: CON-010.

AC-017: Given the user re-answers an already-answered question, when the backend returns 409 `conflict`, then a toast says the question is already answered.
  Test level: integration
  Verification: Submit a valid answer twice; assert second response 409 and toast text matches "already answered".
  Constraint refs: CON-011.

AC-018: Given the user answers a nonexistent question id, when the backend returns 404 `not_found`, then a toast says the question was not found.
  Test level: integration
  Verification: PATCH with an invalid qid; assert 404 and toast text.
  Constraint refs: CON-012.

AC-019: Given a feature with zero questions, when the detail page loads, then the Questions section is not rendered (no wizard, no `answer-summary`, no `question-progress`).
  Test level: e2e
  Verification: Playwright — feature with no questions; assert `[data-testid="answer-summary"]` and `[data-testid="question-progress"]` absent; Questions section header absent.
  Constraint refs: CON-013 (negative: no empty wizard).

AC-020: Given a feature where all questions are already answered on page load and status is `waiting_for_human`, when the page loads, then the wizard shows the history (answered/assumed cards) and the summary with submit.
  Test level: e2e
  Verification: Playwright — seed all-answered + waiting_for_human; assert answered cards + summary + submit button visible.

AC-021: Given a feature not in `waiting_for_human` but with answered/assumed questions, when the page loads, then the history cards render but the submit button and summary are NOT rendered.
  Test level: e2e
  Verification: Playwright — feature `in_progress` with answered questions; assert `answer-summary` and `submit-answers` absent; answered cards present.
  Constraint refs: CON-009.

## Constraint-Derived Criteria (cross-story)

AC-CON-001: Given a question with `type: "clarification"` and non-empty `options`, when rendered, then option cards are shown (render dispatch is options-based, not type-based).
  Test level: unit
  Verification: Render QuestionCard with type=clarification + options=["A","B"]; assert option buttons present.
  Constraint refs: CON-001, CON-003.

AC-CON-002: Given a question with `type: "decision"` and empty `options`, when rendered, then a textarea is shown (no option cards), proving options drives rendering not type.
  Test level: unit
  Verification: Render QuestionCard with type=decision + options=[]; assert textarea present, option buttons absent.
  Constraint refs: CON-002, CON-003.

AC-CON-003: Given the `Question` interface in `ui/src/types/index.ts`, when the feature is implemented, then no field on `Question` is added/removed/renamed (diff-only check).
  Test level: unit
  Verification: `git diff ui/src/types/index.ts` shows no changes to the `Question` interface fields.
  Constraint refs: CON-014.

AC-CON-004: Given the backend answer endpoint, when an answer of 5001 chars is submitted, then the response is 400 `validation_error` (boundary preserved).
  Test level: integration
  Verification: POST answer with 5001 chars; assert 400 and error code `validation_error`.
  Constraint refs: CON-010.

AC-CON-005: Given a `question_answered` SSE event arrives while the wizard is open, when the event is received, then the answered card appears in history without a manual page refresh (React Query invalidation preserved).
  Test level: integration
  Verification: Open wizard; answer via a second client/API; assert the card transitions to answered state in the open page without reload.
  Constraint refs: CON-013, FR-014.