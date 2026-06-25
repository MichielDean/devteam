# Implementation Plan: Better Q&A UI

**Branch**: `better-qa-ui` | **Date**: 2026-06-24 | **Spec**: `specs/better-qa-ui/spec.md`

**Input**: Feature specification from `specs/better-qa-ui/spec.md`

## Summary

Replace the Dev Team pipeline's flat Q&A form with a guided wizard: multiple-choice questions render as selectable option cards (click = select, no immediate submit), open-ended questions render as textarea steps, each step shows the asking phase/role, a progress indicator ("X of Y questions answered") updates live, answering auto-scrolls to the next pending question or the summary, an inline editable answer summary lists all Q+A, and a single "Submit Answers & Resume" button sends all answers via the existing `PATCH /api/features/{id}/questions/{qid}` endpoint (one PATCH per question) and lets the backend's existing resume side-effect fire on the final PATCH. UI-only: the `Question` TS interface and the backend answer endpoint are unchanged.

## Technical Context

- **Language/Version**: TypeScript (frontend), Go (backend — unchanged).
- **Primary Dependencies**: React, React Router v7, React Query (`@tanstack/react-query`), Tailwind v4, Vite. **No new dependencies** (spec assumption).
- **Storage**: None added. Backend SQLite (questions) unchanged.
- **Testing**: Playwright e2e on port 18765 (`ui/playwright.config.ts`, `reuseExistingServer`). No unit-test runner configured — unit-level AC (AC-CON-001/002 render dispatch, AC-CON-003 interface diff) are covered by e2e + diff check (see Test Strategy). Integration tests use Playwright's API request context against the running server.
- **Target Platform**: Browser (desktop); mobile out of scope (spec assumption).
- **Project Type**: Brownfield web app — UI-only change to an existing Go+React monorepo.
- **Performance Goals**: None stated; wizard is single-page, few questions per feature, no heavy compute.
- **Constraints**: UI-only (CON-014, repos.yaml); `Question` interface unchanged; backend endpoint unchanged; preserve SSE+React Query invalidation (FR-014); preserve backend resume-mode semantics (CON-009).
- **Scale/Scope**: Single feature page; questions per feature typically < 10.

## Constitution Check

No `constitution.md` in repo root or `.specify/`. **PASS** — no constitution check required (spec.md Constitution Compliance section).

## Project Structure

```text
ui/src/
├── components/
│   └── QuestionCard.tsx      [MODIFY] pending step: selectable option cards / textarea; answered/assumed restyled consistently
├── pages/
│   └── FeatureDetail.tsx     [MODIFY] owns WizardAnswerDraft, progress indicator, auto-scroll refs, inline summary, single submit button, submit orchestration
└── types/
    └── index.ts              [NO CHANGE] Question interface frozen (CON-014)
ui/e2e/
└── questions.spec.ts         [CREATE] wizard flow e2e + integration (API request context) tests
```

**Structure decision**: modify the two existing files that already own the Questions section; add one e2e spec file. Lift wizard orchestration state into `FeatureDetail.tsx` (the section owner) so the single React Query mutation, SSE invalidation wiring, and scroll-target refs live in one place; `QuestionCard.tsx` becomes presentational for the pending step (props: `draft`, `onSelect`, `onType`) and keeps its read-only answered/assumed render (testids preserved). No new components — YAGNI; a `Wizard` abstraction with one consumer is speculative.

## Component Design

### Component: FeatureDetail (Questions section owner) — [MODIFY]
- **Purpose**: render the feature page; owns the Questions section wizard orchestration.
- **Responsibilities**:
  - Hold `WizardAnswerDraft` (`useState<Record<string,string>>`), cleared on successful submit / unmount.
  - Compute `answeredCount` and `total` for progress (draft-filled counts as answered, CON-005).
  - Render progress indicator `question-progress` (only when `questions.length > 0`).
  - Render pending questions as wizard steps (via `QuestionCard` with `draft`/`onSelect`/`onType` props) and answered/assumed as history cards (via `QuestionCard` read-only branch).
  - Maintain a ref map `questionCardRefs: Record<questionId, HTMLElement>` + `summaryRef` for auto-scroll (CON-006). On draft fill, `scrollIntoView({block:'center'})` to next pending question without a draft, else `summaryRef`.
  - Render inline `answer-summary` panel (one row per question: question text + draft/answer). Editable: clicking a row scrolls back to that step and focuses it (CON-007 / AC-011).
  - Render single `submit-answers` button ("Submit Answers & Resume"); disabled until all pending have non-empty draft; on click run sequential `answerQuestion` PATCHes.
  - Toast on 400/404/409/500 per PATCH using `ApiError.code`/`.details` (FR-010). 409 mid-batch is toasted but does not abort the remaining PATCHes (data-model integrity rule).
  - Hide the Questions section when `questions.length === 0` (FR-013, already true — preserve).
  - Show summary+submit only when `feature.status === 'waiting_for_human'` (AC-021: history-only otherwise).
- **Interfaces**:
  - Reads `questions` (React Query `['questions', id]`) and `feature` (`['feature', id]`).
  - Calls `answerQuestion(featureId, questionId, answer)` per draft entry, sequentially.
- **Dependencies**: `useSSE` (unchanged — preserves `question_answered` invalidation, FR-014), `useToast`, React Query.

### Component: QuestionCard — [MODIFY]
- **Purpose**: render one question in any state (pending step / answered history / assumed history).
- **Responsibilities**:
  - **Pending, options non-empty**: render each option as a selectable card (`<button>` with `data-testid="question-option-{idx}"`, `aria-pressed`/`data-selected` reflecting `draft[question.id] === option`). Clicking calls `onSelect(option)` — does NOT submit (CON-001). No text input shown (AC-001: not a bare input).
  - **Pending, options empty**: render a textarea `question-answer-input` (no option cards, CON-002/003). Typing calls `onType(text)`.
  - **Answered**: preserve existing testids (`question-card-{id}`, `question-type-badge`, `question-checkmark`, `question-text`, `question-answer`) + phase/role text (CON-013 / AC-004). Restyle for visual consistency with wizard.
  - **Assumed**: preserve `question-auto-assumed-label`, `question-assumption`, `question-text`, `question-type-badge`, phase/role (AC-005).
  - Always render phase + role label (CON-004 / AC-009) — already present as `{phase} · {role}` text; keep.
  - Forward ref to parent for auto-scroll (attach to outer `div`).
- **Interfaces** (props):
  - `question: Question`, `featureId: string` (existing)
  - `draft?: string` (the draft answer for this question, pending only)
  - `onSelect?: (option: string) => void` (pending, options non-empty)
  - `onType?: (text: string) => void` (pending, options empty)
  - `ref?: React.Ref<HTMLDivElement>` (for auto-scroll)
- **Dependencies**: `answerQuestion` removed from this component (submit moves to parent). No mutation here.

## API Contracts

See `specs/better-qa-ui/contracts/`:
- `PATCH-api-features-id-questions-questionId.md` — the answer endpoint (unchanged; wizard's submit target).
- `GET-api-features-id-questions.md` — list questions (unchanged; wizard's read).

No new endpoints. No backend changes.

## Constraint Verification Map

| CON-ID | Design Decision | Component(s) | Verification Checkpoint | Test Type |
|--------|-----------------|--------------|------------------------|-----------|
| CON-001 | Pending options-non-empty render selectable cards; click→`onSelect` updates draft, no PATCH | QuestionCard (pending branch) | AC-001/002 e2e: option cards visible not `<input>`; click highlights, no PATCH intercepted | e2e |
| CON-002 | Pending options-empty render `question-answer-input` textarea, no option cards | QuestionCard (pending branch) | AC-014 e2e: `question-option-*` count 0; `question-answer-input` visible | e2e |
| CON-003 | Render dispatch keyed on `question.options.length > 0`, not `question.type` | QuestionCard (pending branch) | AC-CON-001/002 e2e: seeded type=clarification+options shows cards; type=decision+[] shows textarea | e2e (unit-level criteria covered by seeded e2e — no runner configured) |
| CON-004 | Phase + role label rendered on every card (kept from existing `{phase} · {role}`) | QuestionCard (all branches) | AC-009 e2e: phase/role text matches question | e2e |
| CON-005 | Progress `question-progress` shows `${answeredCount} of ${total}` where answeredCount = non-pending OR draft-filled | FeatureDetail | AC-003/006/007 e2e: "1 of 3", updates on draft fill | e2e |
| CON-006 | On draft fill, `scrollIntoView({block:'center'})` to next pending-without-draft card, else `summaryRef` | FeatureDetail | AC-007/008 e2e: next card / `answer-summary` in viewport after answer | e2e |
| CON-007 | Inline `answer-summary` panel lists Q+A; clicking a row scrolls back to the step and allows re-select/re-type | FeatureDetail | AC-010/011/015 e2e: summary rows; edit updates draft | e2e |
| CON-008 | Single `submit-answers` button sends one PATCH per draft entry; final PATCH triggers backend resume | FeatureDetail | AC-012 e2e: intercept PATCHes, one per question; feature leaves `waiting_for_human` | e2e |
| CON-009 | No backend change; single-phase final PATCH → `in_progress` (server-side), autopilot → auto-resume (server-side). Wizard just PATCHes. | FeatureDetail (uses existing endpoint) | AC-013 integration: single-phase submit → `GET /api/features/{id}` `status: in_progress`, no `agent_dispatch` SSE | integration |
| CON-010 | Draft values trimmed before PATCH; empty draft blocks submit (client); backend still enforces 1–5000 (unchanged) | FeatureDetail + backend (unchanged) | AC-016 + AC-CON-004 integration: empty submit → 400 toast; 5001-char → 400 `validation_error` | integration |
| CON-011 | 409 from backend toasted "already answered"; mid-batch 409 does not abort remaining PATCHes | FeatureDetail | AC-017 integration: re-answer → 409 toast | integration |
| CON-012 | 404 toasted "question not found" (invalid qid path) | FeatureDetail | AC-018 integration: PATCH bad qid → 404 toast | integration |
| CON-013 | Answered + assumed branches restyled consistently with wizard; existing testids preserved | QuestionCard (answered/assumed branches) | AC-004/005 e2e: checkmark, auto-assumed label, phase/role, answer/assumption present | e2e |
| CON-014 | `Question` interface in `ui/src/types/index.ts` unchanged | — (no code change to types) | AC-CON-003 diff: `git diff ui/src/types/index.ts` shows no Question-field changes | diff (unit-equivalent) |

## Cross-Component Consistency Matrix

| Shared Value | Producer | Consumer | Consistent? | Verification |
|---|---|---|---|---|
| Question shape (fields/types) | Backend `QuestionToResponse` (Go) | `ui/src/types/index.ts` `Question` (frozen, CON-014) | YES — feature changes neither | e2e: existing list+answer round-trips; diff: types file unchanged |
| options emptiness → render branch | `Question.options` (backend, `[]` never null) | QuestionCard pending dispatch (CON-003) | YES — both treat `options.length>0` as multiple-choice | AC-CON-001/002 e2e |
| Error code strings | Backend `writeError` (`validation_error`/`not_found`/`conflict`/`internal_error`) | `ApiError.code` (client) → toast branch (FeatureDetail) | YES — client already surfaces `code`/`details` | AC-016/17/18 integration toasts per code |
| Resume trigger | Backend final-PATCH goroutine (server.go:1082) | FeatureDetail submit orchestration (just PATCHes; relies on server side-effect) | YES — wizard sends N PATCHes; server resumes on last | AC-012 e2e + AC-013 integration |
| SSE `question_answered` invalidation | `useSSE` (unchanged) | FeatureDetail React Query `['questions', id]` | YES — preserved (FR-014) | AC-CON-005 integration: second-client answer → card flips in open page |
| Progress "answered" definition | FeatureDetail draft + `question.status` | (display only) | YES — non-pending OR draft-filled counts | AC-003/006/007 e2e |

**Multi-component note**: CON-010 (validation) and CON-011/012 (conflict/not-found) apply to BOTH the client submit path AND the backend. The client pre-trims and blocks empty drafts (defense-in-depth), but the backend remains the authority — verified by integration tests hitting the real endpoint. No "apply to all providers" pattern here (single endpoint), but the producer/consumer pair (client PATCH ↔ server handler) is traced.

## Test Strategy

No unit-test runner is configured and the spec forbids new dependencies; unit-level criteria are satisfied by seeded e2e + diff checks. Playwright covers e2e (browser) and integration (API request context against the running server on :18765).

### Component: FeatureDetail (Questions section / wizard)
Testing levels required:
- **Smoke**: page loads without console errors for a `waiting_for_human` feature with questions; Questions section renders.
- **Integration** (Playwright API request context, real server):
  - Submit empty answer → 400 `validation_error` + toast (AC-016 / CON-010).
  - Re-answer answered question → 409 `conflict` + toast (AC-017 / CON-011).
  - PATCH invalid qid → 404 `not_found` + toast (AC-018 / CON-012).
  - 5001-char answer → 400 `validation_error` (AC-CON-004 / CON-010 boundary).
  - Single-phase mode: submit all → `GET /api/features/{id}` `status: in_progress`, no `agent_dispatch` SSE (AC-013 / CON-009).
  - SSE `question_answered` from a second API client → open wizard card flips to answered without reload (AC-CON-005 / FR-014).
- **E2E** (browser):
  - One pending multiple-choice question → 3 option cards, not `<input>` (AC-001).
  - Click option → `data-selected` on it, off on others, no PATCH intercepted (AC-002 / CON-001).
  - 3 pending → answer 1 → progress "1 of 3" (AC-003).
  - 2 pending + 1 answered → progress "1 of 3" on load (AC-006 / CON-005).
  - Answer 1 of 2 → progress updates + next card in viewport (AC-007 / CON-006).
  - Answer last → `answer-summary` in viewport (AC-008 / CON-006).
  - Phase/role label on card (AC-009 / CON-004).
  - All answered → summary lists Q+A (AC-010 / CON-007).
  - Click summary row → edit updates draft (AC-011 / CON-007).
  - Submit → one PATCH per question intercepted + status leaves `waiting_for_human` (AC-012 / CON-008/009).
  - Open-ended question → textarea, no option cards (AC-014 / CON-002/003).
  - Typed open-ended answer in summary (AC-015 / CON-007).
  - Answered card: checkmark, phase/role, question, answer (AC-004 / CON-013).
  - Assumed card: auto-assumed label, phase/role, question, assumption (AC-005 / CON-013).
  - Zero questions → section hidden, no `answer-summary`/`question-progress` (AC-019 / FR-013).
  - All answered on load + `waiting_for_human` → history + summary + submit (AC-020).
  - Not `waiting_for_human` → history only, no submit/summary (AC-021 / CON-009).
  - Render dispatch: type=clarification+options → cards; type=decision+[] → textarea (AC-CON-001/002 / CON-003).
- **Unit-equivalent (diff)**: `git diff ui/src/types/index.ts` shows no `Question` field changes (AC-CON-003 / CON-014).

Quality checkpoints:
- [ ] Service starts without panicking (smoke) — `~/go/bin/devteam -http :18765` boots, `GET /api/features` 200.
- [ ] No console errors on feature detail page (smoke).
- [ ] `GET /api/features/{id}/questions` returns `[]` not `null` for empty (integration) — already true; assert preserved.
- [ ] Error responses have correct `error` code + `details` (integration) — per AC-016/17/18/CON-004.
- [ ] One PATCH per question on submit, correct answer body (e2e) — AC-012.
- [ ] `Question` interface diff-only unchanged (diff) — AC-CON-003.

### Component: QuestionCard
Testing levels required:
- **E2E**: all card-state assertions above are driven through FeatureDetail rendering QuestionCard.
- **Unit-equivalent (render dispatch)**: AC-CON-001/002 covered by seeded e2e (type+options combinations) since no unit runner.
Quality checkpoints:
- [ ] Existing testids preserved on answered/assumed branches (AC-004/005).
- [ ] Pending options-non-empty renders no `<input>` text element (AC-001).
- [ ] Pending options-empty renders no `question-option-*` (AC-014).

## Agent Failure Mode Checks (per task — see tasks.md for per-task checklist)

- **JSON/serialization**: N/A — no new serialization; `Question` shape frozen.
- **Nil pointer / init ordering**: FeatureDetail `WizardAnswerDraft` initializes to `{}` before any `onSelect`/`onType` callback — verify no read of `draft[qid]` before initialization. Ref map populated on render via callback refs (not during render body) to avoid stale refs.
- **Recovery middleware**: N/A — backend unchanged.
- **State machine**: Wizard client flow has implicit states (load/drafting/submittable/submitting/done). Verify: submit disabled until all pending drafted; draft cleared on success; 409 mid-batch doesn't corrupt remaining drafts; SSE during submit doesn't double-submit.
- **Parsing code**: N/A — no new parsing; `ApiError` already parsed by `request()`.
- **Multi-component consistency**: CON-010/011/012 apply to client submit + backend — verify client pre-validation AND backend rejection both tested (integration hits real backend).
- **Language footguns (TS)**: `options.length === 0` (not `!options` — backend guarantees `[]` but defense). `draft[question.id] ?? ''` to avoid undefined. `scrollIntoView` guarded by `if (el)`.

## Quality Checkpoints (task boundaries)

- After T001 (QuestionCard pending step): seed a feature with one multiple-choice + one open-ended question → both render correctly, no PATCH on option click.
- After T002 (FeatureDetail wizard orchestration): progress, auto-scroll, summary, submit all work end-to-end on a seeded `waiting_for_human` feature.
- After T003 (Answered/assumed restyle): history cards still pass AC-004/005.
- After T004 (e2e + integration suite): all AC-001..021 + AC-CON-001..005 green; diff check passes.

## Quickstart (for the Developer)

```bash
# 1. Build + run the backend (unchanged)
cd /home/lobsterdog/worktrees/devteam-specs/better-qa-ui
go build -o ~/go/bin/devteam ./cmd/devteam
~/go/bin/devteam -http :18765 &

# 2. Frontend dev (Vite proxies /api to :18765)
cd ui && npm install && npm run dev   # http://localhost:5173

# 3. Seed a waiting_for_human feature with questions for manual verification:
#    POST /api/features (loose_idea) → run inception → questions created → status waiting_for_human
#    Or POST /api/features/{id}/questions directly with options.

# 4. Run e2e (starts/reuses server on :18765)
cd ui && npm run test:e2e

# 5. Verify the frozen interface (CON-014 / AC-CON-003)
git diff ui/src/types/index.ts   # must show no Question-field changes
```

## Open Questions

None. The PM's `questions.json` was answered via documented assumptions (spec.md Assumptions). The architecture follows those assumptions: select-then-submit, per-feature progress total, auto-scroll to next-pending-or-summary, inline summary, open-ended as textarea step, restyle all three states, options-driven render dispatch, pre-submit editing only. No architect-level ambiguity remains.