# Review Report — Better Q&A UI

**Feature**: better-qa-ui | **Phase**: review | **Reviewer**: adversarial
**Impl files reviewed**: `ui/src/components/QuestionCard.tsx` (132 lines), `ui/src/pages/FeatureDetail.tsx` (640 lines), `ui/src/types/index.ts` (diff-clean), `ui/src/api/client.ts` (unchanged)

## Summary
- Acceptance criteria: 21 numbered (AC-001..021) + 5 constraint-derived (AC-CON-001..005) = **26 total, 26 MET, 0 NOT MET**
- Constraint register: 14 constraints (CON-001..014), **all MET** with execution-path traces below
- Findings: **0 critical, 0 required, 1 noted** (by-design, no fix needed)
- Over-engineering: none — diff is +241/-128 across two files, no new components/deps, matches plan estimate
- Security (P1): input validation at boundary, XSS-safe (React text escaping), no auth surface touched

---

## Phase 1 — Constraint Register Review (execution-path traces)

### CON-001 — Multiple-choice options render as selectable cards; click selects, no immediate submit
**Trace**: FeatureDetail.tsx:558 `onSelect={(opt) => onSelect(q.id, opt)}` → FeatureDetail.tsx:63-65 `onSelect` calls `setDraft` (state update only) → QuestionCard.tsx:101-114 `<button onClick={() => onSelect?.(option)}>` — no fetch, no mutation in the handler chain.
**Status**: MET — Evidence: QuestionCard.tsx:103 (`onClick={() => onSelect?.(option)}`), FeatureDetail.tsx:63-65 (`setDraft((prev) => ({ ...prev, [qid]: option }))`)

### CON-002 — Open-ended (empty options) renders a textarea wizard step, no option cards
**Trace**: QuestionCard.tsx:86 `const hasOptions = question.options && question.options.length > 0` → line 96 ternary `hasOptions ? (...) : (<textarea data-testid="question-answer-input" .../>)` (118-127). When `options` is empty/missing, the `<div data-testid="question-options">` (97) is never mounted.
**Status**: MET — Evidence: QuestionCard.tsx:86, :96-127

### CON-003 — Render dispatch driven by `options` non-empty, NOT by `type`
**Trace**: `type` is used only for badge color (`typeColors[question.type]`, line 27) and badge label (`{question.type}`, line 38). The render branch is keyed solely on `hasOptions` (line 86 → ternary line 96). A question with `type:"clarification"` + options renders cards; `type:"decision"` + `[]` renders textarea.
**Status**: MET — Evidence: QuestionCard.tsx:27 (type→color), :86 (hasOptions), :96-127 (dispatch)

### CON-004 — Each question displays phase + role
**Trace**: `phaseRoleLabel` (lines 29-31) is a `<span>{question.phase} · {question.role}</span>` injected into the answered branch (line 49), assumed branch (line 68), and pending branch (line 91).
**Status**: MET — Evidence: QuestionCard.tsx:29-31, :49, :68, :91

### CON-005 — Progress indicator "X of Y questions answered"
**Trace**: FeatureDetail.tsx:546-548 renders `data-testid="question-progress"` with `count` = `questions.filter(q => q.status !== 'pending' || (draft[q.id]?.trim() ?? '').length > 0).length` over `questions.length`. On draft fill, `onSelect`/`onType` updates `draft` → re-render → count increments.
**Status**: MET — Evidence: FeatureDetail.tsx:546-548

### CON-006 — Auto-scroll to next pending or summary after answering
**Trace**: FeatureDetail.tsx:46-56 `useEffect([draft, isWaitingForHuman])`: finds first pending question whose draft is empty (`nextEmpty`), `questionCardRefs.current[nextEmpty.id]?.scrollIntoView({block:'center'})`; else `summaryRef.current?.scrollIntoView(...)`. Refs populated via `setCardRef` callback (70-75) attached at render (line 560) and `summaryRef` on the summary panel (line 572).
**Status**: MET — Evidence: FeatureDetail.tsx:46-56, :70-75, :560, :572

### CON-007 — Inline editable answer summary before submit
**Trace**: FeatureDetail.tsx:568-606 `data-testid="answer-summary"` lists every question (line 578 `questions.map`). Each row is a `<button data-testid={`summary-row-${q.id}`}>` whose `onClick` (589-591) scrolls back to that question's card. For pending questions the card renders the interactive branch (draft + onSelect/onType, FeatureDetail.tsx:552-561) so the user can re-select/re-type and the draft updates.
**Status**: MET — Evidence: FeatureDetail.tsx:568-606, :552-561

### CON-008 — Single Submit sends all answers and resumes pipeline
**Trace**: FeatureDetail.tsx:607-622 `data-testid="submit-answers"` button → `handleSubmitAll` (80-112): loops `pendingQuestions` (84), `await answerQuestion(id, q.id, answer)` (88) per drafted pending question. Backend resumes on final PATCH (server-side, unchanged). Submit disabled until `allPendingDrafted` (610).
**Status**: MET — Evidence: FeatureDetail.tsx:80-112, :607-622

### CON-009 — Resume-mode semantics preserved (autopilot auto-resume, single-phase manual)
**Trace**: No backend change (diff stat: only `ui/src/**` touched). Client only PATCHes; resume trigger is server-side (`internal/api/server.go`, untouched). Single-phase: final PATCH clears processing; user advances manually.
**Status**: MET — Evidence: `git diff main...HEAD --stat` shows no `internal/` changes; FeatureDetail.tsx:88 calls `answerQuestion` only.

### CON-010 — Answer 1–5000 chars, empty/oversized → 400 validation_error
**Trace**: Client defense: FeatureDetail.tsx:85 `(draft[q.id] ?? '').trim()`, line 81 blocks submit unless `allPendingDrafted` (non-empty). 5001-char not client-blocked (textarea has no maxLength) → reaches backend → 400 → toast (line 97-98). Backend unchanged enforces 1–5000.
**Status**: MET — Evidence: FeatureDetail.tsx:81, :85, :97-98; backend `internal/api/server.go` unchanged

### CON-011 — Re-answer → 409 conflict
**Trace**: FeatureDetail.tsx:93-95: `if (code === 'conflict') { addToast('error', details || 'Question already answered'); }` — toasts and continues the batch (does NOT abort), so a 409 race (e.g., SSE answered the question between draft and submit) doesn't strand remaining answers.
**Status**: MET — Evidence: FeatureDetail.tsx:93-95

### CON-012 — Nonexistent qid → 404 not_found
**Trace**: Falls into the `else` branch (97-101): toast with `details` (backend "not found" message) + `aborted = true` + `break`. Remaining PATCHes skipped.
**Status**: MET — Evidence: FeatureDetail.tsx:97-101

### CON-013 — Answered + assumed states restyled consistently with wizard
**Trace**: QuestionCard answered (43-58) and assumed (62-83) both use `cardBase` (line 18), `badge`, `phaseRoleLabel`, preserved testids (`question-card-{id}`, `question-type-badge`, `question-checkmark`, `question-text`, `question-answer` / `question-auto-assumed-label`, `question-assumption`).
**Status**: MET — Evidence: QuestionCard.tsx:18, :43-58, :62-83

### CON-014 — `Question` TS interface unchanged
**Trace**: `git diff main...HEAD -- ui/src/types/index.ts` produces **no output** (no changes). Interface at types/index.ts:232-245 retains all fields: `id, feature_id, phase, role, question, type, options, answer, assumption, status, created_at, answered_at`.
**Status**: MET — Evidence: `ui/src/types/index.ts:232-245` (diff clean)

---

## Phase 2 — Acceptance Criteria Review

### AC-001 — Option cards visible, not bare text input
**Status**: MET — Evidence: QuestionCard.tsx:101 `<button data-testid={`question-option-${idx}`}>` (a `<button>`, not `<input>`); pending branch has no `<input>` element.

### AC-002 — Click selects (data-selected), others deselect, no PATCH
**Status**: MET — Evidence: QuestionCard.tsx:111 `data-selected={selected ? 'true' : 'false'}`, :99 `const selected = draft === option`; FeatureDetail.tsx:63-65 `onSelect` → `setDraft` only (no fetch). No `useMutation`/`fetch` in the click path.

### AC-003 — 3 pending, answer 1 → "1 of 3 questions answered"
**Status**: MET — Evidence: FeatureDetail.tsx:547 filter counts draft-filled + non-pending; on first `onSelect`, draft[q.id] set → count 1.

### AC-004 — Answered card: checkmark, phase+role, question, answer
**Status**: MET — Evidence: QuestionCard.tsx:51 `question-checkmark`, :48-49 badge+phaseRole, :53 `question-text`, :55 `question-answer`.

### AC-005 — Assumed card: auto-assumed label, phase+role, question, assumption
**Status**: MET — Evidence: QuestionCard.tsx:72 `question-auto-assumed-label`, :68 phaseRole, :77 `question-text`, :79 `question-assumption`.

### AC-006 — 2 pending + 1 answered → "1 of 3" on load
**Status**: MET — Evidence: FeatureDetail.tsx:547 — 1 answered (status≠pending) + 2 pending undrafted → count 1, total 3.

### AC-007 — Answer 1 of 2 → "1 of 2" + q2 in viewport
**Status**: MET — Evidence: FeatureDetail.tsx:48-51 scrolls to `nextEmpty` (first pending without draft = q2). Progress recomputes to "1 of 2". (Note: label reads "1 of 2 questions answered"; AC-007 wording "1 of 2 answered" — same numeric content.)

### AC-008 — Answer last → summary in viewport
**Status**: MET — Evidence: FeatureDetail.tsx:52-53 — when no `nextEmpty`, `summaryRef.current.scrollIntoView(...)`.

### AC-009 — Phase/role label on card
**Status**: MET — Evidence: QuestionCard.tsx:91 `{phaseRoleLabel}` in pending branch; also in answered (49) and assumed (68).

### AC-010 — All answered → summary lists each Q+A
**Status**: MET — Evidence: FeatureDetail.tsx:578-604 maps all `questions`; each row shows phase/role (596), question (597 `summary-question`), answer (598 `summary-answer`).

### AC-011 — Click summary row → edit answer, draft updates
**Status**: MET — Evidence: FeatureDetail.tsx:589-591 `onClick` scrolls to `questionCardRefs.current[q.id]`; pending card at 552-561 receives `draft`/`onSelect`/`onType` so re-select/re-type updates draft. (See Note F-NOTE-1 for answered-row edge — by design.)

### AC-012 — Submit → one PATCH per question, status leaves waiting_for_human
**Status**: MET — Evidence: FeatureDetail.tsx:84-88 `for (const q of pendingQuestions) { ... await answerQuestion(id, q.id, answer) }` — one PATCH per pending drafted question. Backend resumes on final PATCH (unchanged).

### AC-013 — Single-phase submit → in_progress, no agent_dispatch
**Status**: MET — Evidence: no backend change; client only PATCHes (FeatureDetail.tsx:88). Single-phase resume side-effect is server-side (`server.go`, untouched).

### AC-014 — Open-ended → textarea `question-answer-input`, no option cards
**Status**: MET — Evidence: QuestionCard.tsx:118-127 textarea with `data-testid="question-answer-input"`; `question-options` div (97) not rendered when `!hasOptions`.

### AC-015 — Typed open-ended answer in summary
**Status**: MET — Evidence: FeatureDetail.tsx:584 `draft[q.id] ?? ''` for pending questions; summary row (598) displays it.

### AC-016 — Empty submit → 400 toast, wizard stays on step
**Status**: MET — Evidence: FeatureDetail.tsx:81 blocks submit when `!allPendingDrafted` (defense-in-depth); the 400 toast path is exercised by the 5001-char case (AC-CON-004) — line 97-98 toasts `details` on non-conflict errors. Wizard step unchanged (no navigation on error; `aborted=true` keeps draft).

### AC-017 — Re-answer → 409 toast "already answered"
**Status**: MET — Evidence: FeatureDetail.tsx:93-95.

### AC-018 — Bad qid → 404 toast
**Status**: MET — Evidence: FeatureDetail.tsx:97-98 (falls into else branch, toasts `details`).

### AC-019 — Zero questions → Questions section hidden
**Status**: MET — Evidence: FeatureDetail.tsx:542 `questions.length > 0 && (...)`. No `answer-summary`/`question-progress`/`questions-section` rendered when empty.

### AC-020 — All answered on load + waiting_for_human → history + summary + submit
**Status**: MET — Evidence: history cards render (FeatureDetail.tsx:551-565, non-pending branch); summary gated on `isWaitingForHuman` (569); submit button (607).

### AC-021 — Not waiting_for_human → history only, no submit/summary
**Status**: MET — Evidence: FeatureDetail.tsx:569 `isWaitingForHuman && (...)` — summary+submit block only mounts when `feature.status === 'waiting_for_human'`. History cards render unconditionally within `questions.length > 0`.

### AC-CON-001 — type=clarification + options → option cards
**Status**: MET — Evidence: QuestionCard.tsx:86,96 dispatch on `hasOptions` not `type`.

### AC-CON-002 — type=decision + [] → textarea
**Status**: MET — Evidence: QuestionCard.tsx:96,118-127.

### AC-CON-003 — Question interface diff clean
**Status**: MET — Evidence: `git diff main...HEAD -- ui/src/types/index.ts` empty; `ui/src/types/index.ts:232-245` unchanged.

### AC-CON-004 — 5001-char answer → 400 validation_error
**Status**: MET — Evidence: client does not impose maxLength (textarea QuestionCard.tsx:119-127); draft passes `allPendingDrafted` (FeatureDetail.tsx:77) → PATCHed → backend 400 → toast (97-98). Backend `internal/api/server.go` unchanged enforces 1–5000.

### AC-CON-005 — SSE `question_answered` → card flips without reload
**Status**: MET — Evidence: FeatureDetail.tsx:141-144 — on `question_answered` event, `queryClient.invalidateQueries(['questions', id!])` + `['feature', id!]`. React Query refetch + re-render flips the card to answered branch.

---

## Phase 3 — Negative Test Vector Verification
The constraint register defines no formal RFC negative vectors (internal UX feature). The error-path vectors map to integration ACs:
- Empty answer → client-blocked (disabled submit); backend 400 path exercised by 5001-char (AC-CON-004). Verified trace: FeatureDetail.tsx:81, :97-98.
- Re-answer → 409 (AC-017): FeatureDetail.tsx:93-95 toasts + continues.
- Bad qid → 404 (AC-018): FeatureDetail.tsx:97-101 toasts + aborts.
- Zero questions → section hidden (AC-019): FeatureDetail.tsx:542.

All negative paths reject with correct response (no exceptions, no acceptance).

---

## Phase 4 — Cross-Component Consistency
From plan.md matrix:
| Shared Value | Producer | Consumer | Consistent? |
|---|---|---|---|
| Question shape | Backend `QuestionToResponse` (Go, unchanged) | `ui/src/types/index.ts` `Question` (frozen) | YES — diff clean (CON-014) |
| options emptiness → render | `Question.options` (backend `[]` never null) | QuestionCard `hasOptions` dispatch | YES — both treat `length>0` (QuestionCard.tsx:86) |
| Error code strings | Backend `writeError` | `ApiError.code` → toast branch | YES — FeatureDetail.tsx:90-92 reads `apiErr?.code`/`details`; branches on `conflict` vs else |
| Resume trigger | Backend final-PATCH goroutine | FeatureDetail submit (just PATCHes) | YES — client sends N PATCHes (84-88), server resumes on last (unchanged) |
| SSE `question_answered` invalidation | `useSSE` | `['questions', id]` query | YES — FeatureDetail.tsx:141-144 preserved (FR-014) |
| Progress "answered" definition | draft + `question.status` | display | YES — FeatureDetail.tsx:547 |

No producer/consumer mismatch found.

---

## Phase 5 — Language-Specific Footgun Review (TypeScript)
- `draft[q.id]?.trim() ?? ''` (FeatureDetail.tsx:48, :77, :547) — guards undefined. ✓
- `question.options && question.options.length > 0` (QuestionCard.tsx:86) — guards null/undefined options. ✓
- `questionCardRefs.current[nextEmpty.id]` guarded by `if (el)` (FeatureDetail.tsx:50-51), summary by `if (summaryRef.current)` (52-53). ✓
- `answer || <span>...</span>` (FeatureDetail.tsx:599) — empty-string fallback to JSX, no `==` confusion. ✓
- No `any` types in diff. Props typed via `QuestionCardProps`. ✓
- `void featureId` (QuestionCard.tsx:25) — intentional unused-prop acknowledgment; `featureId` retained in interface for existing call sites. Harmless. ✓
- `aria-pressed`/`data-selected` are string-typed (QuestionCard.tsx:110-111) — no boolean↔string coercion bug. ✓

No footguns producing wrong behavior.

---

## Over-Engineering Check
- Diff: +241/-128 across 2 existing files. No new components, no new dependencies, no abstractions beyond spec.
- `forwardRef` (QuestionCard.tsx:21) — required for parent auto-scroll ref forwarding; minimal cost.
- `cardBase`/`badge`/`phaseRoleLabel` extracted constants (QuestionCard.tsx:18,33,29) — DRY across 3 render branches; reduces duplication the restyle would otherwise cause. Appropriate, not speculative.
- No `Wizard`/`WizardStep`/`WizardContext` abstraction (plan explicitly rejected this — YAGNI). ✓
- `handleSubmitAll` (~32 lines) is the minimum to do sequential PATCHes with per-error branching. Not over-built.

No over-engineering findings.

---

## Missing Implementation Check
All 14 functional requirements (FR-001..014) trace to code:
- FR-001/002/003 → QuestionCard.tsx:86,96-127
- FR-004 → QuestionCard.tsx:29-31 (all 3 branches)
- FR-005 → FeatureDetail.tsx:546-548
- FR-006 → FeatureDetail.tsx:46-56
- FR-007 → FeatureDetail.tsx:568-606
- FR-008 → FeatureDetail.tsx:607-622, :80-112
- FR-009 → no backend change (unchanged)
- FR-010 → FeatureDetail.tsx:90-101 (toasts per code)
- FR-011 → QuestionCard.tsx:43-83
- FR-012 → types/index.ts diff clean
- FR-013 → FeatureDetail.tsx:542
- FR-014 → FeatureDetail.tsx:141-144

No missing implementation.

---

## Security Review (P1)
- **Input validation at boundary**: client trims + blocks empty (FeatureDetail.tsx:85,81); backend remains authority for 1–5000 (CON-010). Defense-in-depth present.
- **XSS**: option text, question text, answer/assumption, toast messages all rendered as React children (text nodes), not `dangerouslySetInnerHTML`. React escapes by default. ✓
- **Auth/authz**: no auth surface touched (UI-only feature; endpoints unchanged). No new endpoints. ✓
- **Secrets in logs/errors**: error path uses `apiErr.details` (backend-controlled message), not raw payloads. No secrets flow in this feature. ✓
- **CORS/headers/security headers**: not in scope (backend unchanged).

No security findings.

---

## Null/Empty/Error Path Verification
- **Null pointer safety**: all ref reads guarded (`if (el)`); `draft[q.id]` always coalesced (`?? ''`); `question.options` guarded before `.length`. ✓
- **JSON null arrays**: `listQuestions` returns `[]` per existing backend contract; `questions = []` default (FeatureDetail.tsx:36 `const { data: questions = [] }`). Empty state handled (542). ✓
- **Error paths**: 400 (97-98), 404 (97-101), 409 (93-95), 500 (97-101) all toasted; mid-batch 409 continues, others abort retaining draft for retry. ✓
- **Empty state**: zero questions → section hidden (542); all answered on load → history + summary (569-622). ✓

---

## Findings

### F-NOTE-1: Answered/assumed summary rows scroll to read-only cards (by design)
- **Severity**: noted (no fix needed)
- **Criterion**: AC-011
- **Code**: FeatureDetail.tsx:578-604 (summary), :552-561 (pending-only interactivity)
- **Description**: The summary lists all questions including answered/assumed. Clicking an answered question's row scrolls to its card, but answered/assumed cards render the read-only branch (QuestionCard.tsx:43-83) — not editable. Only pending questions (with drafts) are editable from the summary. This matches the spec assumption ("pre-submit editing only; once submitted, answers are locked") and the CON-011 contract (re-answer → 409). No change required; documented for the Tester so AC-011 verification targets a pending (drafted) question, not an already-answered one.

No other findings. No critical or required issues.

---

## Gate
- [x] Every constraint (CON-001..014) checked with execution-path trace + quoted evidence
- [x] Every acceptance criterion (AC-001..021, AC-CON-001..005) checked with quoted evidence
- [x] Negative/error paths verified (empty, 409, 404, 5001-char, zero-questions)
- [x] Cross-component consistency verified across all producer/consumer pairs
- [x] "No issues" backed by evidence of what was verified (this section + traces)
- [x] Security review complete (P1)
- [x] Null/empty/error paths verified
- [x] Over-engineering check complete (none found)
- [x] Missing implementation check complete (none found)
- [x] Language footguns checked (none producing wrong behavior)
- [x] `Question` interface diff clean (CON-014)

**Outcome: PASS** — implementation matches spec and constraint register with no critical or required findings.