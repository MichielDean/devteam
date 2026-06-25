# Data Model: Better Q&A UI

This feature is UI-only. No backend entity or DB schema changes (CON-014, repos.yaml). The data model documents the existing entities the wizard reads/writes plus the new client-only draft.

## Entities

### Question (existing, unchanged shape — CON-014)

- **Attributes** (TypeScript `Question` interface in `ui/src/types/index.ts:232`):
  - `id`: string, required, UUID
  - `feature_id`: string, required, FK→Feature.id
  - `phase`: `'inception' | 'planning'`, required
  - `role`: `'pm' | 'architect'`, required
  - `question`: string, required, 1–2000 chars (enforced backend-side on creation)
  - `type`: `'clarification' | 'decision' | 'priority'`, required — **display-only** (CON-003: render dispatch ignores `type`)
  - `options`: `string[]`, required — `[]` (never null; backend sets `[]` when nil). Empty/non-empty drives render dispatch (CON-001/002/003).
  - `answer`: `string | null`, nullable, default null — set when status→answered
  - `assumption`: `string | null`, nullable, default null — set when status→assumed
  - `status`: `'pending' | 'answered' | 'assumed'`, required
  - `created_at`: string (ISO datetime), required
  - `answered_at`: `string | null`, nullable
- **Relationships**: belongs-to Feature (many Questions : one Feature)
- **Constraints** (existing, backend-enforced): unique `id`; `answer` length 1–5000 when set (CON-010); status transitions enforced server-side (CON-011 conflict on re-answer).
- **Integrity rule**: this feature MUST NOT add/remove/rename any field on the `Question` interface. `git diff ui/src/types/index.ts` must show no Question-field changes (AC-CON-003 / CON-014).

### Feature (existing, unchanged)

- Owns Questions. `status: 'waiting_for_human'` triggers the wizard UI; `'in_progress'` shows history-only (AC-021); terminal states show nothing interactive.
- No field changes.

### WizardAnswerDraft (NEW — client-only, non-persisted)

- **Attributes**:
  - `Record<questionId: string, answer: string>` — the user's selections/typed answers before submit.
  - Lives in `useState` inside `FeatureDetail.tsx`.
  - Default: `{}` (empty object).
  - Cleared on successful submit (all PATCHes resolved) or component unmount.
- **Relationships**: belongs-to one Feature's current question set (transient, in-memory).
- **Constraints**:
  - Keys are question ids with `status === 'pending'` only (answered/assumed questions are read-only history).
  - Values are non-empty trimmed strings at submit time (empty draft for a pending question blocks submit, matching backend CON-010).
  - NOT serialized, NOT sent to backend, NOT stored in localStorage. UI state only (spec Key Entities section).
- **Derived "answered-in-draft" semantics** for the progress indicator (CON-005):
  - A question counts as "answered" for the progress display if `question.status !== 'pending'` OR `draft[question.id]` is a non-empty trimmed string.
  - Progress text: `${answeredCount} of ${total} questions answered` where `total = questions.length`.

## State Transitions

### Question (server-authoritative; wizard observes via React Query)

- `pending → answered`: trigger = user submits via PATCH (wizard's final batch PATCH or single answer).
- `pending → assumed`: trigger = backend auto-assume on timeout (wizard cannot initiate; reads via SSE/listQuestions).
- Invalid: `answered → pending`, `assumed → pending`, `answered → answered` (returns 409 `conflict`, CON-011).

### Wizard flow (client state machine, implicit in FeatureDetail)

1. **load** — `listQuestions` resolves; if `questions.length === 0` → section hidden (FR-013). If feature.status ≠ `waiting_for_human` → history-only view (AC-021). Else render pending steps + history.
2. **drafting** — user selects options / types text; `WizardAnswerDraft` mutates; progress updates live; auto-scroll fires on each draft fill (target = next pending question without a draft, else summary).
3. **submittable** — all pending questions have a non-empty draft → summary panel visible + `submit-answers` enabled.
4. **submitting** — sequential PATCHes; `submit-answers` disabled + spinner; on any error → toast, remaining PATCHes aborted, wizard stays on summary (retryable). On all-success → draft cleared; backend resume side-effect fires on the final PATCH (autopilot) or status flips to in_progress (single-phase).
5. **done** — React Query invalidation (SSE `question_answered` / `processing_complete`) re-fetches feature; section re-renders as history-only or hides (status left waiting_for_human).

Invalid transitions: editing a draft after submit commits (backend 409 on re-answer — spec assumption "pre-submit editing only").

## Data Integrity Rules

- **No schema migration.** No DB column added/removed.
- **Frontend validation (before PATCH)**: draft values trimmed; empty draft not submitted (matches backend CON-010 1–5000). No client-side truncation of >5000 chars — let the backend reject (AC-CON-004 boundary test).
- **Referential integrity (client)**: draft keys must reference pending questions currently in the `listQuestions` result; stale keys (question answered via SSE between draft and submit) are skipped at submit, and the 409 from the backend is toasted (CON-011) without aborting the rest of the batch.
- **SSE consistency**: `question_answered` invalidates `['questions', featureId]` (already in `useSSE`); the wizard re-renders from fresh `listQuestions` data — answered cards appear in history without manual refresh (AC-CON-005).