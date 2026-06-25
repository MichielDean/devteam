# Technical Research: Better Q&A UI

## Existing Code Patterns (brownfield)

### Backend answer endpoint (unchanged — UI-only feature)
- `internal/api/server.go:1022` `answerQuestion`: validates `answer` (TrimSpace, 1–5000 chars), stores via `questionStore.AnswerQuestion`, broadcasts `question_answered` SSE, then in a goroutine checks `PendingCount`. If 0 and `status == waiting_for_human`:
  - `single-phase` mode → clears `activeProcess`, sets status to `in_progress`, **does not auto-dispatch** (user advances manually).
  - otherwise (`autopilot`) → sets status to `in_progress`, `LoadOrStore` the active process, and auto-resumes the pipeline.
- Error codes (verbatim from handler):
  - 400 `validation_error` — missing feature/question id, invalid JSON, empty/oversized answer (>5000).
  - 404 `not_found` — feature or question not found.
  - 409 `conflict` — `feature.QuestionConflictError` (already answered/assumed).
  - 500 `internal_error` — store/lookup failure.
- **Implication for the wizard**: each answer is its own PATCH. The "Submit all" button issues N sequential PATCHes. The backend resume-side-effect fires only when the *last* pending question is answered — i.e. on the final PATCH of the batch. This means the frontend does NOT call a separate "resume" endpoint; submitting the final answer triggers resume. For single-phase, the same final PATCH transitions to `in_progress`.

### SSE + React Query invalidation (must be preserved)
- `ui/src/hooks/useSSE.ts` registers a `question_answered` listener (line 90) and, in `handleEvent`, invalidates `['questions', feature_id]` and `['feature', feature_id]` for every event with a `feature_id` (lines 37–42). So when a second client answers a question, an open wizard re-fetches `listQuestions` and the card flips to answered state without a manual refresh. FR-014 / AC-CON-005 require this to keep working — the wizard must not replace the query-key scheme.

### QuestionCard.tsx (current — to be rewritten, pending branch)
- Pending state renders option buttons that only call `setAnswerText(option)` (fill a text input) plus a per-question Submit (`answerMutation.mutate`). The mutation fires immediately on submit.
- Answered/assumed states render read-only display with existing testids: `question-card-{id}`, `question-type-badge`, `question-checkmark`, `question-text`, `question-answer`, `question-auto-assumed-label`, `question-assumption`, plus phase/role text (`{question.phase} · {question.role}`).
- **Design decision**: preserve all existing testids so answered/assumed e2e assertions (AC-004, AC-005) need no change. Add new testids for wizard-only elements (`question-option-*` already exists — repurpose as selectable cards with `data-selected`; `question-progress`, `answer-summary`, `submit-answers` are new).

### FeatureDetail.tsx (current — Questions section)
- Renders Questions section only when `questions.length > 0` (FR-013 already satisfied — AC-019).
- "All answered" banner (`all-questions-answered`) shows when none pending.
- **Design decision**: lift answer-draft state and submit orchestration into FeatureDetail (the section owner), pass `draft`, `onSelect`, `onType`, `onSubmitAll` down to QuestionCard. QuestionCard becomes presentational for the pending step. This keeps the single React Query mutation + SSE invalidation wiring in one place and lets auto-scroll target refs live in the parent.

### API client (unchanged)
- `answerQuestion(featureId, questionId, answer)` → `PATCH /api/features/{id}/questions/{qid}` returns the updated `Question`. `listQuestions(featureId)` → `GET /api/features/{id}/questions` returns `Question[]` (`[]` never null — Go `QuestionsToResponse` returns non-nil slice).
- `ApiError` exposes `.status`, `.code`, `.details`. The wizard's toast logic can branch on `err.code` (`validation_error` / `not_found` / `conflict` / `internal_error`) for accurate messaging (AC-016/17/18).

## Library / Framework Choices

| Choice | Selected | Rationale |
|---|---|---|
| Option card UI | Native `<button>` + Tailwind + `aria-pressed`/`data-selected` | No new dep. Tailwind already in repo. `question-option-*` testid already exists. |
| Auto-scroll | `scrollIntoView({ behavior: 'smooth', block: 'center' })` via ref map | Stdlib DOM API; no animation lib. `block: 'center'` keeps target in viewport (AC-007/08). |
| Submit orchestration | Sequential `Promise.allSettled` over `answerQuestion` per draft entry, or sequential `for...of` to guarantee last-answer resume side-effect ordering | Reuses existing `answerQuestion`. Sequential preferred so the final PATCH reliably triggers backend resume (see Backend section). |
| Draft state | `useState<Record<string,string>>` in FeatureDetail | Client-only `WizardAnswerDraft` per spec — not persisted, cleared on successful submit/unmount. |
| Toast | Existing `useToast` / `toast-error` testid | AC-016/17/18 assert `toast-error`. Reuse, do not add a new toast system. |
| Test runner | Playwright (e2e + integration via API request context) | Already configured (`ui/playwright.config.ts`, port 18765, `reuseExistingServer`). No unit-test runner is configured — adding vitest/jsdom would be a new dependency the spec explicitly forbids ("No new frontend dependencies"). Unit-level AC-CON-001/002 are instead covered by e2e with seeded questions (option-driven render dispatch observable in the browser). |

## Alternative Approaches Considered and Rejected

1. **Batch submit endpoint** (`POST /api/features/{id}/answers` accepting `{questionId:answer}` map). Rejected: backend is unchanged per CON-014 / repos.yaml; adding an endpoint violates the UI-only scope and would require a backend task. The sequential-PATCH approach reuses the existing endpoint and its resume side-effect.
2. **Per-question immediate submit (current behavior, just restyle)**. Rejected by spec: US-003 / CON-007/008 require a single Submit + summary.
3. **vitest + @testing-library for unit tests** to satisfy AC-CON-001/002 at the unit level. Rejected: no unit runner configured; adding one is a new dependency the spec forbids. e2e with a seeded feature (type=clarification + options, type=decision + empty options) exercises the same render-dispatch branch. AC-CON-003 (interface unchanged) is a diff check, not a unit test.
4. **React Router wizard step route** (`/features/:id/wizard/:step`). Rejected: the spec assumption is an inline review panel, not separate pages. Keeps FeatureDetail single-page; auto-scroll replaces routing.
5. **Global draft store (Zustand/context)**. Rejected: YAGNI — single feature page owns the draft; `useState` in FeatureDetail suffices.

## Spikes / Prototypes

None needed. All patterns (React Query mutation, SSE invalidation, Toast, Tailwind cards, Playwright API request context) are already used in the codebase. No unfamiliar technology.