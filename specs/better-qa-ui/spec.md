# Feature Specification: Better Q&A UI

**Feature Branch**: `better-qa-ui`
**Created**: 2026-06-24
**Status**: Draft
**Input**: Loose idea — "Improve the interactive question and answer UI for the Dev Team pipeline. Currently questions appear as plain cards with a text input for answers. Make it a richer experience: show multiple-choice options as clickable buttons/radio cards, show which phase is asking, show a progress indicator (X of Y questions answered), auto-scroll to next question after answering, show a summary of all answers before submitting, and add a submit button that resumes the pipeline. Make questions feel like a guided wizard, not a form."

## Workspace Summary (Brownfield)

This feature modifies an existing codebase — the Dev Team platform itself.

**Stack**: Go backend (`internal/`) + React/TypeScript frontend (`ui/`). Frontend: Vite, Tailwind v4, React Query, React Router v7, Playwright e2e (port 18765). No new dependencies expected — uses existing stack.

**Affected files (current state)**:
- `ui/src/components/QuestionCard.tsx` — renders each question card; pending state shows option buttons that only set the text-input value, plus a per-question Submit button. Answered/assumed states are read-only display.
- `ui/src/pages/FeatureDetail.tsx` — renders the Questions section: header, pending count badge, waiting banner, maps `QuestionCard`s, and an "all answered" banner.
- `ui/src/types/index.ts` — `Question` interface: `{id, feature_id, phase: 'inception'|'planning', role: 'pm'|'architect', question, type: 'clarification'|'decision'|'priority', options: string[], answer, assumption, status: 'pending'|'answered'|'assumed', created_at, answered_at}`.
- `ui/src/api/client.ts` — `answerQuestion(featureId, questionId, answer)` → `PATCH /api/features/{id}/questions/{qid}`; `listQuestions(featureId)` → `GET /api/features/{id}/questions`.
- `ui/e2e/app.spec.ts` — current e2e suite; no question-flow coverage yet.

**Backend behavior (unchanged by this feature)**: `answerQuestion` handler (`internal/api/server.go:1022`) validates answer (1–5000 chars), stores via `questionStore.AnswerQuestion`, broadcasts `question_answered` SSE, and auto-resumes the pipeline when pending count reaches 0 in autopilot mode. In single-phase mode it clears processing state and waits for the user to advance. Error codes: 400 `validation_error`, 404 `not_found`, 409 `conflict` (already answered/assumed), 500 `internal_error`.

**Conventions to follow**: Tailwind utility classes, `data-testid` attributes on interactive elements, dark-mode variants (`dark:`), React Query mutations with `onSuccess`/`onError`, toast feedback via `useToast`. No external RFC/standard governs this UX feature.

## Constraint Register

No external RFC/standard/test-vector governs this feature — it is internal product UX. Constraints below are sourced from the existing codebase (the governing implementation contract) and the feature input. Each is traceable and verifiable.

| ID | Source | Section/Vector | Type | Constraint | Verification Method |
|----|--------|----------------|------|------------|---------------------|
| CON-001 | QuestionCard.tsx | pending render | correctness | Multiple-choice questions (options non-empty) MUST render each option as a clickable selectable card; clicking selects, does not immediately submit | E2E: click option → option highlighted, no network POST until submit |
| CON-002 | QuestionCard.tsx | open-ended | correctness | Questions with empty/no options MUST render a textarea/input wizard step (no option buttons shown) | E2E: open-ended question shows input, no option buttons |
| CON-003 | QuestionCard.tsx | render dispatch | correctness | Render dispatch is driven by whether `options` is non-empty, NOT the `type` field (type is display-only) | Unit: question with type=clarification + options renders option buttons |
| CON-004 | FeatureDetail.tsx | phase label | correctness | Each question MUST display which phase is asking (inception/planning) and which role (pm/architect) | E2E: question card shows phase + role |
| CON-005 | Feature input | progress | correctness | A progress indicator MUST show "X of Y questions answered" across all pending+answered questions for the feature | E2E: answer one → count increments |
| CON-006 | Feature input | auto-scroll | correctness | After answering a question, view auto-scrolls to the next pending question; if none remain, scroll to summary/submit area | E2E: answer → scroll target visible in viewport |
| CON-007 | Feature input | summary | correctness | Before final submit, an inline summary lists every question with its selected/typed answer, editable | E2E: summary shows Q+A, user can change an answer |
| CON-008 | Feature input | submit | correctness | A single Submit button sends all answers and resumes the pipeline (replaces per-question Submit for the wizard flow) | E2E: one submit button, click → all answers POSTed |
| CON-009 | server.go:1103 | resume mode | consistency | Pipeline auto-resumes only in autopilot mode; in single-phase mode the user advances manually after submit | Integration: single-phase + submit → status in_progress, no auto-run |
| CON-010 | server.go:1047 | validation | security | Answer MUST be 1–5000 chars (trim required); empty/oversized → 400 `validation_error` | Integration: empty submit → 400 |
| CON-011 | server.go:1059 | conflict | consistency | Answering an already-answered/assumed question → 409 `conflict` | Integration: re-answer → 409 |
| CON-012 | server.go:1063 | not found | consistency | Answering a nonexistent question → 404 `not_found` | Integration: bad qid → 404 |
| CON-013 | QuestionCard.tsx | answered/assumed | consistency | Answered and assumed states MUST still render (history), styled consistently with the wizard; this feature restyles all three states | E2E: answered card renders with checkmark, assumed with auto-assumed label |
| CON-014 | types/index.ts | Question model | consistency | The Question model fields (id, phase, role, type, options, answer, assumption, status) MUST NOT change shape; this feature is UI-only | Unit/diff: types/index.ts Question interface unchanged |

## User Scenarios & Testing

### User Story 1 - Guided Multiple-Choice Answering (Priority: P1)

A user reaches a `waiting_for_human` feature. Instead of a flat form, they see a guided wizard: each pending question is a step showing which phase/role is asking, the question text, and — for multiple-choice questions — clickable radio cards for each option. Clicking an option selects it (highlighted), it does not submit. A progress indicator shows "X of Y questions answered." After selecting, the user can continue to the next question. This story delivers the core interaction upgrade and is independently testable: a feature with one multiple-choice question can be answered via the new option-card UI.

**Why this priority**: This is the headline ask — replace plain text-input option buttons with a real selectable card UI. Without this, the feature is just a restyle.

**Independent Test**: Start a feature with a single pending multiple-choice question; click an option card; verify it highlights and no POST fires until submit.

**Acceptance Scenarios**:

1. **Given** a feature with status `waiting_for_human` and one pending question with options `["A","B","Other"]`, **When** the user opens the feature detail page, **Then** the question renders with three selectable option cards (not a text input with buttons that only fill the input).
2. **Given** the question card from scenario 1, **When** the user clicks option "B", **Then** option "B" becomes highlighted/selected and options "A" and "Other" are not selected, and no `PATCH /questions/{id}` request is sent.
3. **Given** the feature has 3 pending questions, **When** the user answers 1, **Then** a progress indicator shows "1 of 3 questions answered."
4. **Given** an answered question in the history, **When** the user views the feature, **Then** the answered card shows a checkmark, the phase + role label, the question, and the chosen answer, styled consistently with the wizard.
5. **Given** an auto-assumed question in the history, **When** the user views the feature, **Then** the assumed card shows an "auto-assumed" label, the phase + role, the question, and the assumption text.

---

### User Story 2 - Progress, Auto-Scroll, and Phase Context (Priority: P1)

The wizard shows a progress indicator ("X of Y questions answered") across all questions for the feature, labels each step with the asking phase (inception/planning) and role (pm/architect), and auto-scrolls to the next pending question after the user answers one. If no pending questions remain, it scrolls to the summary/submit area. This story is independently testable: a feature with 2+ questions can verify the progress count and scroll behavior.

**Why this priority**: Progress visibility and phase context are the "richer experience" core to the idea; auto-scroll is the wizard feel. Together they make the difference between "form" and "wizard."

**Independent Test**: Feature with 2 pending questions; answer the first; verify progress reads "1 of 2" and the second question scrolls into view.

**Acceptance Scenarios**:

1. **Given** a feature with 2 pending and 1 answered question, **When** the page loads, **Then** the progress indicator shows "1 of 3 questions answered."
2. **Given** the user just answered question 1 of 2 pending, **When** the answer is recorded, **Then** the progress indicator updates to "1 of 2 answered" and the view auto-scrolls so question 2 is visible in the viewport.
3. **Given** the user answered the last pending question, **When** there are no more pending questions, **Then** the view auto-scrolls to the summary/submit area.
4. **Given** any question card, **When** rendered, **Then** it displays a label showing the asking phase (e.g. "Inception") and role (e.g. "PM").

---

### User Story 3 - Answer Summary and Single Submit (Priority: P1)

Before the pipeline resumes, the user sees an inline summary panel listing every question with its selected/typed answer. Answers are editable inline (clicking one jumps back to that question or lets the user re-select). A single "Submit Answers & Resume" button sends all answers and resumes the pipeline. This replaces per-question Submit buttons in the wizard flow. This story is independently testable: a feature with all questions answered shows the summary and a working submit button.

**Why this priority**: The explicit review-and-submit step is what makes the flow trustworthy (the user confirms before the pipeline takes off). It is the single biggest behavioral change and must be right.

**Independent Test**: Answer all questions; verify the summary lists each Q+A; click Submit; verify all answers are POSTed and the pipeline resumes.

**Acceptance Scenarios**:

1. **Given** all questions have been answered (none pending), **When** the user views the feature, **Then** an inline summary panel lists each question and its answer.
2. **Given** the summary is visible, **When** the user clicks an answer in the summary, **Then** they can edit that answer (re-select an option or re-type).
3. **Given** all questions answered and the summary shown, **When** the user clicks "Submit Answers & Resume", **Then** every answered question is sent via `PATCH /api/features/{id}/questions/{qid}` and the pipeline resumes.
4. **Given** the feature is in single-phase mode and all questions answered, **When** the user clicks Submit, **Then** answers are sent and the feature transitions to `in_progress` awaiting manual advance (no auto-run).

---

### User Story 4 - Open-Ended Question Step (Priority: P2)

Questions without options (open-ended) render as a wizard step with a textarea input, using the same flow (phase label, progress, summary, submit) as multiple-choice questions. They get a distinct visual treatment to distinguish them from multiple-choice steps but remain inside the wizard — not relegated to a separate form. Independently testable: a feature with one open-ended question can be answered via the textarea.

**Why this priority**: Open-ended questions already work today via the text input; this is a consistency upgrade so they don't feel bolted on. Important but not blocking.

**Independent Test**: Feature with one open-ended question; type an answer; verify it appears in the summary and submits.

**Acceptance Scenarios**:

1. **Given** a pending question with empty `options`, **When** rendered, **Then** it shows a textarea input (no option cards) styled as a wizard step with phase/role label and progress.
2. **Given** an open-ended question with a typed answer, **When** the user navigates to the summary, **Then** the summary shows the question and the typed text.

---

### User Story 5 - Error and Empty State Handling (Priority: P2)

The wizard handles errors and empty states explicitly: a failed answer POST shows a toast with the backend error message (400 validation, 409 conflict, 404 not found, 500 internal); a feature with zero questions shows no wizard (the Questions section is hidden as today); a feature with all questions already answered on load shows only the history (answered/assumed cards) plus, if applicable, the summary. Independently testable: trigger each error code and verify the toast text.

**Why this priority**: Resilience and clarity on failure. Not headline but required for a P1 feature per the resiliency extension.

**Independent Test**: Submit an empty answer; verify a 400 toast; submit a second answer to an already-answered question; verify a 409 toast.

**Acceptance Scenarios**:

1. **Given** the user submits an empty answer, **When** the backend returns 400 `validation_error`, **Then** a toast displays the validation message and the wizard remains on the current step.
2. **Given** the user re-answers an already-answered question, **When** the backend returns 409 `conflict`, **Then** a toast says the question is already answered.
3. **Given** a feature with zero questions, **When** the detail page loads, **Then** the Questions section is not rendered (no empty wizard).
4. **Given** a feature where all questions are already answered on page load, **When** the page loads, **Then** the wizard shows the history (answered/assumed cards) and, if status allows resume, the summary.

### Edge Cases

- **All questions answered on page load**: show history + summary; submit resumes if status is `waiting_for_human`.
- **Mixed multiple-choice and open-ended in one feature**: each renders per its own type; progress counts both; summary lists both.
- **User edits an answer in the summary then re-submits**: the edited answer is sent; the old answer is overwritten (backend supports re-answer until status flips — 409 once answered, so edit must happen before submit commits).
  - [ASSUMPTION: editing after submit is not supported — once submitted, the pipeline resumes and answers are locked. Pre-submit editing only.]
- **Network failure during submit**: toast shows "Failed to answer question: …"; the wizard stays on the summary; the user can retry.
- **Auto-scroll with only one question**: no scroll happens (nothing to scroll to); summary appears.
- **Very long option text**: option cards wrap; layout does not break.
- **Dark mode**: all new elements have `dark:` variants.
- **Feature not in `waiting_for_human` but has questions**: history-only view (answered/assumed cards), no submit button.
- **Empty `options` array vs missing field**: both treated as open-ended (`options.length === 0`).
- **SSE `question_answered` event arrives while wizard open**: the answered card should appear in history without a manual refresh (React Query invalidation already handles this — must be preserved).

## Requirements

### Functional Requirements

- **FR-001**: The system MUST render each pending multiple-choice question (options non-empty) as a set of selectable option cards where clicking selects (highlights) without submitting. Source: US-001, CON-001.
- **FR-002**: The system MUST render each pending open-ended question (options empty) as a textarea wizard step. Source: US-004, CON-002.
- **FR-003**: The system MUST drive render dispatch by whether `options` is non-empty, not by the `type` field. Source: CON-003.
- **FR-004**: Each question card MUST display the asking phase (inception/planning) and role (pm/architect). Source: US-002, CON-004.
- **FR-005**: The system MUST show a progress indicator "X of Y questions answered" across all questions for the feature. Source: US-002, CON-005.
- **FR-006**: After a question is answered, the system MUST auto-scroll to the next pending question, or to the summary/submit area if none remain. Source: US-002, CON-006.
- **FR-007**: The system MUST show an inline answer summary listing each question with its answer, editable pre-submit. Source: US-003, CON-007.
- **FR-008**: A single "Submit Answers & Resume" button MUST send all answers and resume the pipeline. Source: US-003, CON-008.
- **FR-009**: The system MUST preserve backend resume-mode semantics: auto-resume in autopilot, manual advance in single-phase. Source: US-003, CON-009.
- **FR-010**: The system MUST display a toast with the backend error message on 400/404/409/500 from the answer endpoint. Source: US-005, CON-010/11/12.
- **FR-011**: The system MUST render answered and assumed states consistently with the wizard (checkmark / auto-assumed label, phase+role, question, answer/assumption). Source: US-001, CON-013.
- **FR-012**: The system MUST NOT change the `Question` TypeScript interface shape. Source: CON-014.
- **FR-013**: The system MUST hide the Questions section when a feature has zero questions. Source: US-005.
- **FR-014**: The system MUST preserve React Query invalidation on the `question_answered` SSE event so answered cards appear in history without manual refresh. Source: Edge case.

### Key Entities

- **Question** (existing, unchanged shape): `{id, feature_id, phase, role, question, type, options[], answer, assumption, status, created_at, answered_at}`. Relationships: belongs to one Feature. Lifecycle: `pending → answered` (user) or `pending → assumed` (auto-assume on timeout). The wizard is a view over a list of Questions for one Feature.
- **Feature** (existing, unchanged): owns Questions; status `waiting_for_human` triggers the wizard.
- **WizardAnswerDraft** (new, client-only, non-persisted): per-feature in-memory map of `{questionId → selectedAnswer}` collecting selections before submit. Cleared on successful submit or unmount. This is UI state only — not a backend entity and not a DB row.

## Success Criteria

- **SC-001**: A user can answer a multiple-choice question by clicking an option card (not typing), with the selection visible before submit. Measurable: e2e test clicks an option and asserts it is selected.
- **SC-002**: The progress indicator accurately reflects answered/total for the feature at all times. Measurable: e2e test asserts "1 of 2" then "2 of 2".
- **SC-003**: Answering a question auto-scrolls the next pending question (or summary) into the viewport. Measurable: e2e test asserts the target element is in the viewport after answer.
- **SC-004**: A single Submit button sends every answer and resumes the pipeline (autopilot) or transitions to in_progress (single-phase). Measurable: e2e test asserts one PATCH per question and status transition.
- **SC-005**: All error codes (400/404/409/500) produce a visible toast with the backend message. Measurable: e2e test asserts toast text per code.
- **SC-006**: Open-ended questions render as textarea wizard steps with phase/role label and progress. Measurable: e2e test asserts textarea present, no option buttons.
- **SC-007**: Answered and assumed cards render with phase+role, checkmark / auto-assumed label, and the answer/assumption text. Measurable: e2e test asserts each element.
- **SC-008**: The `Question` TypeScript interface is unchanged (diff-only verification). Measurable: `git diff ui/src/types/index.ts` shows no Question field changes.

## Assumptions

- [ASSUMPTION: No human answered the clarifying questions; the PM chose conservative defaults per the error-recovery extension. The defaults below are documented for reviewer/architect challenge.]
- [ASSUMPTION: Option selection is "select then submit" — clicking an option highlights it; a single Submit at the end sends all answers. This is the most reviewable, least-surprising model and matches the feature input's "summary before submitting" ask.]
- [ASSUMPTION: Progress is a per-feature total ("X of Y questions answered") across all questions for the feature, not per-phase. The feature input says "X of Y questions answered" without qualification; a single total is simplest and matches the wording.]
- [ASSUMPTION: Auto-scroll target is the next pending question; if none remain, scroll to the summary/submit area. Smooth scroll within the questions section.]
- [ASSUMPTION: The answer summary is an inline review panel at the bottom of the questions section (Q + selected answer list, editable), not a modal or separate page. Inline keeps context visible and is the least disruptive.]
- [ASSUMPTION: Open-ended questions stay inside the wizard as a textarea step with a distinct visual treatment, not relegated to a separate flow.]
- [ASSUMPTION: All three states (pending, answered, assumed) are restyled for visual consistency in the wizard history.]
- [ASSUMPTION: Render dispatch is driven by whether `options` is non-empty, ignoring the `type` field. `type` remains a display badge. No DB migration or new `type` value.]
- [ASSUMPTION: Pre-submit answer editing only. Once Submit is clicked and answers POSTed, the pipeline resumes and answers are locked (backend 409 on re-answer).]
- [ASSUMPTION: No new frontend dependencies — Tailwind + React Query + existing primitives suffice.]
- [ASSUMPTION: The backend answer endpoint and Question DB schema are unchanged; this is a UI-only feature.]
- [ASSUMPTION: Target users are developers/operators running the Dev Team pipeline in a browser; mobile is out of scope.]
- [ASSUMPTION: The existing `question_answered` SSE event + React Query invalidation pattern is preserved so answered cards appear in history without manual refresh.]

## Constitution Compliance

No `constitution.md` exists in the repo root or `.specify/`. No constitution check required.