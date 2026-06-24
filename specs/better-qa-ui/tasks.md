# Tasks: Better Q&A UI

**Input**: Design documents from `specs/better-qa-ui/` (plan.md, spec.md, research.md, data-model.md, contracts/)

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: e2e + integration tests ARE required (acceptance criteria specify e2e/integration levels). No unit-test runner is configured (spec forbids new deps); unit-level criteria are covered by seeded e2e + diff checks.

**Organization**: Tasks grouped by user story priority (P1 first, then P2), then a cross-cutting test+verify phase.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story (US-001..US-005, or CROSS for cross-cutting)
- Exact file paths in descriptions

## Path Conventions

Web app: frontend `ui/src/`, e2e `ui/e2e/`. Backend `internal/api/server.go` is NOT modified (UI-only, CON-014).

---

## Phase 1: Foundational — Wizard pending step (blocks all user stories)

**Purpose**: Convert QuestionCard's pending branch from "option buttons that fill a text input + per-question Submit" into a presentational selectable-card / textarea step. Lift submit out of QuestionCard.

**⚠️ CRITICAL**: All user-story work depends on this — the wizard interaction model starts here.

- [ ] T001 [US-001] Rework `QuestionCard.tsx` pending branch into a presentational wizard step
  - File: `ui/src/components/QuestionCard.tsx` — [MODIFY]
  - Changes:
    - Remove `useMutation`/`answerQuestion`/`answerText` local state + `handleSubmit` + the text `<input>` + per-question `question-answer-submit` button from the pending branch.
    - Add props: `draft?: string`, `onSelect?: (option: string) => void`, `onType?: (text: string) => void`, and accept a `ref` (forward to outer `<div>` for auto-scroll).
    - Pending, `options.length > 0`: render each option as a `<button>` with `data-testid="question-option-{idx}"` and `aria-pressed={String(draft === option)}` / `data-selected={draft === option ? 'true' : 'false'}`; `onClick={() => onSelect?.(option)}`. **No text input rendered** (AC-001). Wrap long option text (`break-words`/`whitespace-normal`) so layout doesn't break (edge case).
    - Pending, `options.length === 0`: render a `<textarea>` with `data-testid="question-answer-input"`, value=`draft ?? ''`, `onChange={e => onType?.(e.target.value)}`. **No `question-option-*` elements** (AC-014 / CON-002).
    - Keep phase + role label (`{question.phase} · {question.role}`) on the pending branch (CON-004 / AC-009).
    - Keep `data-testid="question-card-{question.id}"` on the outer div (all branches).
  - Constraint refs: CON-001, CON-002, CON-003, CON-004, CON-014 (no type change).
  - Done conditions:
    - A pending multiple-choice question renders option `<button>`s (not `<input>`) with `question-option-*` testids; clicking one sets `aria-pressed="true"` on it and `false` on others; no PATCH is sent (verified by Playwright route interception on a seeded feature).
    - A pending open-ended question renders `question-answer-input` (textarea) and zero `question-option-*` elements.
    - `question-card-{id}`, phase/role label present on the pending branch.
    - `git diff ui/src/types/index.ts` shows no `Question` field changes (CON-014 / AC-CON-003).
  - Test level: e2e (render assertions via Playwright on a seeded feature) + smoke (page loads, no console errors).
  - Agent failure mode checks:
    - [ ] No `answerText` local state left over (stale submit path) — grep `QuestionCard.tsx` for `answerMutation`/`handleSubmit`/`answerText` in the pending branch; must be absent.
    - [ ] `draft === option` comparison uses the exact option string (not index) — multi-component consistency between `onSelect(option)` and the highlight predicate.
    - [ ] `options.length === 0` guard (not `!options`) — backend guarantees `[]`, but defense.
    - [ ] Ref forwarding does not read the ref during render body (attach via callback ref or `forwardRef`).

**Checkpoint**: Pending questions render as wizard steps with no immediate submit. Answered/assumed branches still render with existing testids (unchanged this task).

---

## Phase 2: User Story 1 — Guided Multiple-Choice Answering (Priority: P1) 🎯 MVP

**Goal**: Selectable option cards + progress indicator + answered/assumed history restyle.

**Independent Test**: One pending multiple-choice question; click option → highlighted, no POST.

- [ ] T002 [US-001] Add `WizardAnswerDraft` + progress indicator + option selection wiring in `FeatureDetail.tsx`
  - File: `ui/src/pages/FeatureDetail.tsx` — [MODIFY]
  - Changes:
    - Add `const [draft, setDraft] = useState<Record<string,string>>({})`.
    - `onSelect = (qid, option) => setDraft(prev => ({...prev, [qid]: option}))`.
    - `onType = (qid, text) => setDraft(prev => ({...prev, [qid]: text}))`.
    - Compute `total = questions.length`; `answeredCount = questions.filter(q => q.status !== 'pending' || (draft[q.id]?.trim() ?? '').length > 0).length`.
    - Render `<div data-testid="question-progress">{answeredCount} of {total} questions answered</div>` inside the Questions section (only when `questions.length > 0`).
    - Pass `draft={draft[q.id]}`, `onSelect`, `onType` props to each pending `QuestionCard`.
  - Constraint refs: CON-001, CON-003, CON-005.
  - Done conditions:
    - One pending MC question + click option → `question-progress` shows "1 of 1"; option card has `data-selected="true"`; no PATCH intercepted.
    - 3 pending questions + answer 1 (select option) → progress "1 of 3" (AC-003).
    - 2 pending + 1 answered on load → progress "1 of 3" (AC-006 / CON-005).
  - Test level: e2e + smoke.
  - Agent failure mode checks:
    - [ ] `draft[q.id]` accessed with `?.trim() ?? ''` (no undefined leak).
    - [ ] Progress text format exactly `${n} of ${total} questions answered` (AC-003/006 assert substring "1 of 3").
    - [ ] No PATCH fired on `onSelect` (AC-002) — verify no `answerQuestion` import in the selection path.

- [ ] T003 [US-001] Restyle answered + assumed branches in `QuestionCard.tsx` for wizard visual consistency
  - File: `ui/src/components/QuestionCard.tsx` — [MODIFY]
  - Changes: restyle the answered and assumed branches (Tailwind classes, dark variants) so history cards look consistent with the pending wizard step. **Preserve all existing testids**: `question-card-{id}`, `question-type-badge`, `question-checkmark`, `question-text`, `question-answer`, `question-auto-assumed-label`, `question-assumption`, and the `{phase} · {role}` text (AC-004/005). Do NOT change the `Question` interface.
  - Constraint refs: CON-013, CON-014.
  - Done conditions:
    - Answered card renders `question-checkmark`, `question-type-badge`, phase/role text, `question-text`, `question-answer` all non-empty (AC-004).
    - Assumed card renders `question-auto-assumed-label`, `question-type-badge`, phase/role, `question-text`, `question-assumption` non-empty (AC-005).
    - Dark-mode variants present on new classes (edge case).
  - Test level: e2e.
  - Agent failure mode checks:
    - [ ] No testid renamed/removed (AC-004/005 depend on exact testids) — grep the file for each testid.
    - [ ] `git diff ui/src/types/index.ts` still shows no Question field changes.

**Checkpoint**: US-001 independently testable — a feature with one MC question can be answered via option cards with visible selection and progress.

---

## Phase 3: User Story 2 — Progress, Auto-Scroll, Phase Context (Priority: P1)

**Goal**: Auto-scroll on answer + phase/role labels (labels already present from T001/T003).

**Independent Test**: 2 pending; answer first → progress "1 of 2" + second card in viewport.

- [ ] T004 [US-002] Implement auto-scroll to next pending question or summary in `FeatureDetail.tsx`
  - File: `ui/src/pages/FeatureDetail.tsx` — [MODIFY]
  - Changes:
    - Add `const questionCardRefs = useRef<Record<string, HTMLDivElement | null>>({})` and `const summaryRef = useRef<HTMLDivElement | null>(null)`.
    - Pass a callback ref to each `QuestionCard` (pending) that stores the node in `questionCardRefs.current[q.id]`.
    - `useEffect` on `draft` change: find the next pending question whose draft is empty; if found, `questionCardRefs.current[nextId]?.scrollIntoView({behavior:'smooth', block:'center'})`; else `summaryRef.current?.scrollIntoView({behavior:'smooth', block:'center'})`.
    - Guard: if only one question, no scroll target exists — summary appears (edge case).
  - Constraint refs: CON-006.
  - Done conditions:
    - Answer 1 of 2 pending → progress updates + `question-card-{id2}` bounding box intersects viewport (AC-007).
    - Answer last pending → `answer-summary` in viewport (AC-008).
    - Single-question feature → no scroll, summary visible (edge case).
  - Test level: e2e.
  - Agent failure mode checks:
    - [ ] `scrollIntoView` called only when target element exists (`if (el)` guard) — TS optional-chaining.
    - [ ] Effect dependency is `draft` (and `questions` ids) — not a value that changes every render (avoid scroll loop).
    - [ ] Refs populated via callback refs (`ref={el => questionCardRefs.current[q.id] = el}`), not read during render body.

**Checkpoint**: US-002 independently testable — 2-question feature shows progress + scroll behavior.

---

## Phase 4: User Story 3 — Answer Summary and Single Submit (Priority: P1)

**Goal**: Inline editable summary + single Submit-and-Resume.

**Independent Test**: Answer all; summary lists Q+A; submit → one PATCH per question + pipeline resumes.

- [ ] T005 [US-003] Add inline answer summary panel in `FeatureDetail.tsx`
  - File: `ui/src/pages/FeatureDetail.tsx` — [MODIFY]
  - Changes:
    - Render `<div data-testid="answer-summary" ref={summaryRef}>` with one row per question showing `question.question` + (`draft[q.id]` for pending-drafted, `q.answer` for answered, `q.assumption` for assumed).
    - Each row clickable → `scrollIntoView` to that question's card and focus (re-select/re-type updates draft, AC-011).
    - Show summary only when `feature.status === 'waiting_for_human'` (AC-021: history-only otherwise).
  - Constraint refs: CON-007.
  - Done conditions:
    - All pending drafted → `answer-summary` visible, one row per question with Q+A (AC-010).
    - Click a summary row for an MC question → re-select a different option → draft updates (AC-011).
    - Open-ended typed answer appears in its summary row (AC-015 / CON-007).
    - Feature not `waiting_for_human` → no `answer-summary` (AC-021 / CON-009).
  - Test level: e2e.
  - Agent failure mode checks:
    - [ ] Summary row count matches `questions.length` (no off-by-one).
    - [ ] `answer-summary` hidden when `questions.length === 0` (FR-013 / AC-019) — guard the whole section.

- [ ] T006 [US-003] Add single "Submit Answers & Resume" button + sequential PATCH orchestration in `FeatureDetail.tsx`
  - File: `ui/src/pages/FeatureDetail.tsx` — [MODIFY]
  - Changes:
    - Render `<button data-testid="submit-answers">Submit Answers & Resume</button>`. Disabled unless every pending question has a non-empty trimmed draft. Show spinner + disable while submitting.
    - `handleSubmitAll`: for each pending question with a draft, sequentially `await answerQuestion(featureId, qid, draft.trim())`. Trim each value; skip empty (CON-010 client defense).
    - On 400/404/409/500 (`ApiError`): `addToast('error', <message from code/details>)` (FR-010). **409 mid-batch toasts but does NOT abort remaining PATCHes** (data-model integrity rule — question may have been answered via SSE between draft and submit). 400/500 abort the batch (genuine error).
    - On all-success: clear `draft` (`setDraft({})`); React Query `onSuccess` invalidation + SSE `question_answered` drives re-fetch; backend resume side-effect fires on the final PATCH (autopilot auto-resume, single-phase → in_progress).
    - Remove the old "All questions answered. Pipeline will resume automatically." banner behavior replaced by the submit flow (the `all-questions-answered` banner may stay as a fallback but the submit button is now the resume trigger).
  - Constraint refs: CON-008, CON-009, CON-010, CON-011, CON-012.
  - Done conditions:
    - Submit with all drafted → exactly one PATCH per question intercepted with correct answer body; feature status leaves `waiting_for_human` (AC-012).
    - Single-phase mode submit → `GET /api/features/{id}` returns `status: in_progress`, no `agent_dispatch` SSE (AC-013 / CON-009).
    - Empty answer submit → 400 `validation_error` toast, wizard stays on summary (AC-016 / CON-010).
    - Re-answer (submit twice / already answered) → 409 `conflict` toast "already answered" (AC-017 / CON-011).
    - PATCH invalid qid → 404 `not_found` toast (AC-018 / CON-012).
    - 5001-char answer → 400 `validation_error` (AC-CON-004 / CON-010 boundary).
  - Test level: e2e (AC-012/016) + integration (AC-013/017/018/AC-CON-004 via Playwright API request context).
  - Agent failure mode checks:
    - [ ] Submit button disabled until all pending drafted (no partial submit).
    - [ ] Sequential PATCHes (not `Promise.all`) so the final PATCH reliably triggers backend resume ordering.
    - [ ] 409 mid-batch does NOT abort remaining PATCHes (verify with a test that pre-answers one question via API, then submits the batch).
    - [ ] `draft` cleared on success; not cleared on error (user can retry).
    - [ ] `ApiError.code` used for toast branching, not string-matching `err.message` (current QuestionCard did string-match — improve).

**Checkpoint**: US-003 independently testable — all-answered feature shows summary + working submit.

---

## Phase 5: User Story 4 — Open-Ended Question Step (Priority: P2)

**Goal**: Open-ended questions render as textarea wizard steps (already delivered structurally by T001).

**Independent Test**: One open-ended question; type answer; appears in summary; submits.

- [ ] T007 [US-004] Verify + style-distinct open-ended textarea step
  - File: `ui/src/components/QuestionCard.tsx` — [MODIFY] (visual distinctness only — structural render from T001)
  - Changes: give the open-ended textarea branch a distinct visual treatment (e.g. different border/badge) so it's distinguishable from MC steps, while staying inside the wizard (spec assumption). Reuse `question-answer-input` testid (AC-014).
  - Constraint refs: CON-002, CON-003, CON-007.
  - Done conditions:
    - Pending open-ended question → `question-answer-input` visible, `question-option-*` count 0, phase/role + progress present (AC-014).
    - Typed answer appears in summary (AC-015).
  - Test level: e2e.
  - Agent failure mode checks:
    - [ ] Distinct treatment doesn't change the testid (`question-answer-input`).
    - [ ] Render dispatch still keyed on `options.length === 0` (CON-003) — not on `type`.

**Checkpoint**: US-004 independently testable.

---

## Phase 6: User Story 5 — Error and Empty State Handling (Priority: P2)

**Goal**: Toasts per error code + empty/zero-question + all-answered-on-load + not-waiting-for-human states.

**Independent Test**: empty submit → 400 toast; re-answer → 409 toast.

- [ ] T008 [US-005] Error/empty-state coverage in `FeatureDetail.tsx` (mostly verify T006's toast logic + existing zero-question guard)
  - File: `ui/src/pages/FeatureDetail.tsx` — [MODIFY]
  - Changes:
    - Ensure Questions section hidden when `questions.length === 0` (already true — preserve; AC-019). No `question-progress`, no `answer-summary`, no `submit-answers` in that case.
    - Ensure all-answered-on-load + `waiting_for_human` → history cards + summary + submit (AC-020).
    - Ensure not-`waiting_for_human` → history cards only, no `submit-answers`/`answer-summary` (AC-021 / CON-009).
    - Confirm toast branch uses `ApiError.code` for `validation_error`/`not_found`/`conflict`/`internal_error` (FR-010). Reuse `toast-error` testid (existing).
  - Constraint refs: CON-009, CON-010, CON-011, CON-012, CON-013.
  - Done conditions:
    - Zero-question feature → Questions section absent, `answer-summary`/`question-progress` absent (AC-019).
    - All-answered + `waiting_for_human` on load → history + summary + submit visible (AC-020).
    - Not-`waiting_for_human` with answered questions → history only, no submit/summary (AC-021).
    - 400/404/409/500 each produce a `toast-error` with the backend message (AC-016/17/18).
  - Test level: e2e (AC-019/020/021) + integration (AC-016/17/18/CON-004).
  - Agent failure mode checks:
    - [ ] Zero-question guard uses `questions.length === 0` not falsy-coercion.
    - [ ] Submit/summary gated on `feature.status === 'waiting_for_human'` exactly (AC-021).

**Checkpoint**: US-005 independently testable.

---

## Phase 7: Cross-Cutting — E2E + Integration Test Suite + Diff Verification

**Purpose**: The acceptance criteria are e2e/integration-level. This phase writes the Playwright suite that proves every AC + constraint.

- [ ] T009 [CROSS] [P] Write e2e + integration suite for the wizard flow
  - File: `ui/e2e/questions.spec.ts` — [CREATE]
  - Contents (one Playwright test per AC where practical):
    - **Seed helper**: create a feature via `POST /api/features`, drive it to `waiting_for_human` with questions (or `POST /api/features/{id}/questions` directly with chosen `options`/`type`/`phase`/`role`). Provide fixtures for: one MC pending; 3 MC pending; 2 pending + 1 answered; all-answered + waiting_for_human; not-waiting_for_human with answered; zero questions; one open-ended; mixed MC+open-ended; single-phase mode.
    - AC-001: MC pending → 3 `question-option-*` visible, not `<input>` elements.
    - AC-002: click option 1 → `data-selected="true"` on it, false on others; `page.route`/requests collector asserts no PATCH.
    - AC-003: 3 pending, select one → `question-progress` contains "1 of 3".
    - AC-004: answered card → `question-checkmark`, `question-type-badge`, phase/role, `question-text`, `question-answer` non-empty.
    - AC-005: assumed card → `question-auto-assumed-label`, `question-assumption` non-empty.
    - AC-006: 2 pending + 1 answered on load → "1 of 3".
    - AC-007: answer 1 of 2 → progress updates + `question-card-{id2}` in viewport (`boundingBox` intersects viewport).
    - AC-008: answer last → `answer-summary` in viewport.
    - AC-009: phase/role text node present on a card.
    - AC-010: all drafted → `answer-summary` visible, one row per question.
    - AC-011: click summary row → re-select different option → draft/summary updates.
    - AC-012: submit → intercept PATCHes, one per question with correct body; feature status leaves `waiting_for_human`.
    - AC-013 (integration, API context): single-phase mode submit → `GET /api/features/{id}` `status: in_progress`, no `agent_dispatch` SSE.
    - AC-014: open-ended → `question-answer-input` visible, `question-option-*` count 0, phase/role + progress present.
    - AC-015: type into textarea → summary row contains typed text.
    - AC-016 (integration): empty submit → 400 `validation_error`, `toast-error` contains backend message, wizard step unchanged.
    - AC-017 (integration): re-answer → 409 `conflict`, toast "already answered".
    - AC-018 (integration): PATCH bad qid → 404 `not_found`, toast.
    - AC-019: zero questions → no Questions section, no `answer-summary`/`question-progress`.
    - AC-020: all-answered + `waiting_for_human` on load → history + summary + submit.
    - AC-021: not-`waiting_for_human` → history only, no `submit-answers`/`answer-summary`.
    - AC-CON-001: seeded type=clarification + options=["A","B"] → option cards present.
    - AC-CON-002: seeded type=decision + options=[] → textarea present, option buttons absent.
    - AC-CON-003 (diff): run `git diff --exit-code ui/src/types/index.ts` against the base — assert no `Question` field changes (or assert the Question interface lines are unchanged).
    - AC-CON-004 (integration): PATCH answer 5001 chars → 400 `validation_error`.
    - AC-CON-005 (integration): open wizard for feature A; answer a question via a second Playwright API request context; assert the open page's card flips to answered without reload (React Query invalidation preserved).
  - Constraint refs: all CON-001..CON-014.
  - Done conditions:
    - `cd ui && npm run test:e2e` passes all tests above.
    - No console errors captured during e2e (mirror existing `app.spec.ts` pattern).
    - `git diff ui/src/types/index.ts` shows no `Question` field changes.
  - Test level: e2e + integration (this IS the test suite).
  - Agent failure mode checks:
    - [ ] Seed helper produces deterministic state (feature id reused or re-created per test).
    - [ ] Each test cleans up or is idempotent (avoid cross-test state bleed).
    - [ ] `page.route` interception for "no PATCH on select" (AC-002) asserts the request was NOT made, not just that the UI didn't navigate.
    - [ ] Viewport-intersection assertion (AC-007/008) uses `boundingBox()` + viewport dimensions, not just `isVisible()`.

**Checkpoint**: All acceptance criteria green; diff check passes; ready for Review phase.

---

## Dependencies & Execution Order

### Task Dependencies
- T001 (QuestionCard pending step) → T002, T004, T005, T007 (all need the presentational step + props).
- T002 (draft + progress) → T004 (auto-scroll reads draft), T005 (summary reads draft), T006 (submit reads draft).
- T003 (answered/assumed restyle) — independent of T001/T002; can run in parallel with T002 ([P]).
- T004 (auto-scroll) → depends on T002 (draft state) and T001 (refs on QuestionCard).
- T005 (summary) → depends on T001, T002.
- T006 (submit) → depends on T002 (draft), T005 (summary visible to enable submit). T006 also delivers the toast logic used by T008.
- T007 (open-ended styling) → depends on T001 (textarea branch exists).
- T008 (error/empty states) → depends on T006 (toast logic) and T002/T005 (section guards).
- T009 (e2e suite) → depends on T001–T008 all complete.

### Parallel Opportunities
- T003 [P] can run in parallel with T002 (different concerns in the same file — coordinate to avoid merge conflicts, or sequence T002 then T003).
- T007 [P] can run in parallel with T004/T005 once T001 is done (it only touches the QuestionCard textarea branch styling).

### Execution Order (recommended)
1. T001 (foundational — blocks all)
2. T002 (draft + progress)
3. T003 [P] (answered/assumed restyle) — parallel with T004
4. T004 (auto-scroll)
5. T005 (summary)
6. T006 (submit + toasts)
7. T007 [P] (open-ended styling) — can slot in after T001
8. T008 (empty/error states — verify + guard)
9. T009 (e2e + integration suite + diff)

## Notes

- [P] = different files/concerns, no dependency conflict.
- Backend (`internal/api/server.go`) is NOT modified. If any task seems to require a backend change, STOP — that violates CON-014/repos.yaml; recirculate to planning.
- Preserve all existing testids on answered/assumed branches (AC-004/005 depend on them).
- The `Question` interface in `ui/src/types/index.ts` is frozen — T009's diff check enforces CON-014.