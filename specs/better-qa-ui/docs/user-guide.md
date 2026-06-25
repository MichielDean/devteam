# User Guide — Better Q&A UI (Spec: better-qa-ui)

The Dev Team pipeline's interactive Q&A flow is now a **guided wizard** instead of a flat form. When a Feature reaches `waiting_for_human` status, its Questions appear as a wizard: each pending Question is a step, you can review a summary of all answers, and a single **Submit Answers & Resume** button sends everything and resumes the pipeline.

This guide covers every user story from the spec, using spec terminology (`Feature`, `Question`, `phase`, `role`, `options`, `answer`, `assumption`, `status`, `pending`, `answered`, `assumed`, `autopilot`, `single-phase`).

---

## US-001 — Guided Multiple-Choice Answering

When a Feature is `waiting_for_human`, each pending multiple-choice Question (one whose `options` array is non-empty) renders as a set of **selectable option cards** — one card per option. Clicking an option **selects** it (the card highlights; the other cards deselect) but does **not** submit. No answer is sent to the backend until you click the single Submit button at the end.

**Common workflow**:
1. Open the Feature detail page for a `waiting_for_human` Feature.
2. For a multiple-choice Question, click the option card you want. It becomes highlighted; no network request is sent.
3. Move to the next pending Question.

**Answered and assumed history cards** also render, styled consistently with the wizard:
- An **answered** card shows a checkmark, the asking phase + role, the question text, and the chosen answer.
- An **assumed** card (auto-assumed on timeout) shows an "auto-assumed" label, the phase + role, the question, and the assumption text.

---

## US-002 — Progress, Auto-Scroll, and Phase Context

Each Question card displays a label showing which **phase** is asking (`inception` or `planning`) and which **role** (`pm` or `architect`).

A **progress indicator** shows `X of Y questions answered` across all Questions for the Feature (a single per-Feature total, not per-phase). A draft-filled pending Question counts toward `X` before submit.

After you answer (fill the draft for) a Question, the view **auto-scrolls** to the next pending Question. If no pending Questions remain, it scrolls to the summary/submit area.

**Example**: A Feature with 2 pending and 1 answered Question shows `1 of 3 questions answered` on load. Answer the first pending Question → progress updates and the second Question scrolls into view.

---

## US-003 — Answer Summary and Single Submit

Before the pipeline resumes, an **inline answer summary** panel lists every Question with its selected or typed answer. Answers are editable pre-submit: clicking a row in the summary scrolls back to that Question's step and lets you re-select an option or re-type.

A single **Submit Answers & Resume** button sends all answers. The button is disabled until every pending Question has a non-empty draft. On click, one `PATCH /api/features/{id}/questions/{questionId}` request is sent per Question, sequentially.

Resume behavior is controlled by the backend's existing resume-mode semantics (CON-009), unchanged by this feature:
- **autopilot**: after the final PATCH the pipeline auto-resumes.
- **single-phase**: after the final PATCH the Feature transitions to `in_progress` awaiting manual advance (no agent dispatch begins).

**Pre-submit editing only**. Once Submit is clicked and answers are POSTed, the pipeline resumes and answers are locked — re-answering returns 409 `conflict`.

---

## US-004 — Open-Ended Question Step

Questions with an empty `options` array (open-ended) render as a wizard step with a **textarea** input — no option cards. They use the same flow as multiple-choice Questions: phase + role label, progress indicator, summary, and submit. They get a distinct visual treatment to distinguish them from multiple-choice steps, but remain inside the wizard.

**Workflow**: type your answer into the textarea; it appears in the summary and submits with everything else.

> Render dispatch is driven by whether `options` is non-empty, **not** by the `type` field (CON-003). The `type` field (`clarification` / `decision` / `priority`) is a display-only badge.

---

## US-005 — Error and Empty State Handling

### Error toasts
A failed answer PATCH shows a toast with the backend error message. The wizard stays on the current step (no navigation lost):

| HTTP status | `error` code | Meaning | Toast |
|---|---|---|---|
| 400 | `validation_error` | Empty or >5000-char answer (CON-010) | backend validation message |
| 404 | `not_found` | Feature or Question id not found (CON-012) | "question not found" |
| 409 | `conflict` | Question already answered or assumed (CON-011) | "already answered" |
| 500 | `internal_error` | Store/lookup failure (FR-010) | backend message |

During a batch submit, a 409 for one Question is toasted but does **not** abort the remaining PATCHes.

### Empty states
- **Feature with zero Questions**: the Questions section is not rendered at all — no wizard, no summary, no progress, no submit (FR-013 / AC-019).
- **All Questions already answered on load** + status `waiting_for_human`: the wizard shows the history (answered/assumed cards) plus the summary and submit button (AC-020).
- **Feature not `waiting_for_human`** but has Questions: history cards render, but the summary and submit button do **not** render (AC-021 / CON-009).

### Network failure during submit
A network failure toasts `Failed to answer question: …`; the wizard stays on the summary so you can retry.

---

## Edge cases

- **Mixed multiple-choice and open-ended in one Feature**: each Question renders per its own `options`; progress counts both; summary lists both.
- **Auto-scroll with only one Question**: no scroll happens (nothing to scroll to); the summary appears.
- **Long option text**: option cards wrap; layout does not break.
- **Dark mode**: all new wizard elements have dark-mode variants.
- **SSE `question_answered` while the wizard is open**: an answered card appears in history without a manual page refresh (React Query invalidation preserved — FR-014).