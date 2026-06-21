# User Guide — Spec 003: Human Interaction Points

This guide explains how a product owner uses the Dev Team web UI to answer questions surfaced by the PM (during inception) and the Architect (during planning). This feature lets the pipeline pause at decision points and incorporate your answers instead of making assumptions.

The terminology in this guide matches **Spec 003: Human Interaction Points**.

---

## Overview

The Dev Team pipeline runs autonomously through construction, review, testing, and delivery. But during **inception** and **planning**, the PM and Architect agents may surface questions that benefit from human input — clarifications about requirements, decisions about architecture, and priorities about scope.

When an agent produces questions, the pipeline pauses: the feature enters `waiting_for_human` status, the questions appear on the feature detail page, and you can answer them through the UI. Once you answer all questions (or the timeout expires), the pipeline resumes with your answers injected into the agent's context.

If you don't respond within the configured timeout, the pipeline falls back to autonomous mode: it documents an assumption for each unanswered question and proceeds with the conservative choice.

---

## US-001: Answer PM Clarification Questions

During **inception**, the PM agent may surface clarification questions about ambiguous requirements.

### How to answer

1. Open the **Dashboard** (`/`).
2. If a feature has pending questions, a yellow/orange **question badge** appears on its feature card showing the count of pending questions.
3. Click the feature card (or the badge) to open the **feature detail page**.
4. The **Questions** section appears with one card per pending question.
5. Each card shows:
   - The **question text**.
   - A **type badge** color-coded by question type:
     - `clarification` — blue
     - `decision` — orange
     - `priority` — purple
   - The phase and role that generated the question (e.g., `inception · pm`).
   - **Suggested options** as clickable buttons (if the agent provided any).
   - A **text input** labeled "Type your answer..." and a **Submit** button.
6. Either click a suggested option (the option text fills the input) or type your own answer.
7. Click **Submit**.
8. The card updates to a **read-only state** with a green checkmark and the answer text. The badge count on the dashboard decreases by one.

### Error scenarios

| Action | Error | What it means |
|---|---|---|
| Submit an answer | 409 Conflict `{"error": "conflict", "details": "Question Q-001 is already answered"}` | Someone else already answered this question. Only the first answer wins; refresh the page to see the winning answer. |
| Submit an answer | 404 Not Found `{"error": "not_found", "details": "Question Q-999 not found"}` | The question ID does not exist. |
| Submit an empty answer | 400 Bad Request `{"error": "validation_error", "details": "answer must be 1-5000 characters"}` | The answer field cannot be empty. |
| Submit an answer over 5000 characters | 400 Bad Request (same message as above) | The answer exceeds the 5000-character limit. |

### Empty state

When a feature has no questions, the **Questions** section is completely hidden — no empty state message, no placeholder.

---

## US-002: Review Architect Design Decisions

During **planning**, the Architect agent may surface design decisions as questions with `type: "decision"`.

### How to review

1. Open the feature detail page for a feature in `waiting_for_human` status during planning.
2. Decision cards appear with the same layout as clarification cards, but the type badge is **orange** (`decision`).
3. Suggested options are shown as clickable buttons. Clicking an option populates the answer input with the option text.
4. Review the suggested options or type your own answer.
5. Click **Submit**.
6. The card shows your answer in a read-only state with a green checkmark.

### Error scenarios

Same error paths as US-001 (409 Conflict, 404 Not Found, 400 validation errors).

### Empty state

When the Architect surfaces no design decisions, no decision cards are shown and the pipeline proceeds normally without pausing.

---

## US-003: Pipeline Pauses for Human Input

The pipeline automatically pauses when:

1. The PM or Architect agent completes dispatch during **inception** or **planning**.
2. The agent produced a `questions.json` artifact in the feature's spec directory.
3. The pipeline detected and stored valid questions.
4. The feature's `timeout_minutes` configuration is not `0` (which would mean fully autonomous mode).

When these conditions are met, the feature transitions to `waiting_for_human` status, a `waiting_for_human` SSE event is broadcast, and the pipeline waits. The pipeline will not advance, recirculate, or evaluate the gate while the feature is in this status.

### How to resume

- **Answer all questions**: The pipeline detects that all questions are answered, transitions the feature back to `in_progress`, builds a "Human Responses" section for the agent context, and re-dispatches the agent with your answers.
- **Let the timeout expire**: See US-004.
- **Cancel the feature**: The feature transitions to `cancelled`.
- **Recirculate the feature**: The feature transitions to `recirculated`, all questions are deleted, and the re-run may generate new questions with new IDs.

### Attempting to advance

If you click **Advance** on a feature in `waiting_for_human` status, the API returns:
```json
{"error": "validation_error", "details": "Cannot advance feature in waiting_for_human status"}
```
You must answer the questions (or wait for the timeout) before the pipeline can advance.

### Phases that do not pause

The pipeline **never** pauses for human input during construction, review, testing, or delivery. If an agent in one of those phases happens to produce a `questions.json` artifact, the questions are still stored but the feature does not enter `waiting_for_human`.

---

## US-004: Pipeline Falls Back to Autonomous Mode

If you don't respond within the configured timeout, the pipeline proceeds autonomously with documented assumptions.

### Timeout behavior

1. The timeout starts when the feature enters `waiting_for_human` status.
2. The timeout resets if a new question is added while the feature is already in `waiting_for_human` status (to avoid premature assumption generation while you are actively engaging).
3. When the timeout expires, for each pending question:
   - An assumption is generated.
   - The question's `status` transitions `pending → assumed`.
   - The `assumption` field is populated.
   - The `answered_at` field is set.
4. The feature transitions back to `in_progress`.
5. A "Human Responses" section is built with assumptions marked as auto-assumed.
6. The agent is re-dispatched with the assumptions in context.

### Timeout configuration

The timeout is configured in `devteam.yaml`:

```yaml
pipeline:
  human_interaction_timeout_minutes: 30
```

| Value | Behavior |
|---|---|
| `30` (default) | Wait 30 minutes, then auto-assume. |
| Positive integer | Wait that many minutes, then auto-assume. |
| `0` | Never pause. Questions are still stored but assumptions are immediately generated. The feature never enters `waiting_for_human`. |
| `-1` | Wait indefinitely. No timeout. The feature remains in `waiting_for_human` until a human answers or the feature is cancelled/recirculated. |
| Field absent | Defaults to 30 minutes. |

### What you see after timeout

On the feature detail page, assumed questions show the assumption text in a read-only state with an "auto-assumed" label (instead of the green checkmark used for human-answered questions). The source is labeled `[Source: auto-assumed after timeout of 30 minutes]` in the agent context.

---

## US-005: How Agents Create Questions

Agents (PM and Architect) create questions by writing a `questions.json` artifact in the feature's spec directory as part of their dispatch output.

### Detection logic

After an agent dispatch completes during inception or planning, the pipeline:

1. Checks if `specs/{id}/questions.json` exists.
2. Parses it as a JSON array of question objects.
3. Validates each question:
   - Required fields: `phase`, `role`, `question`, `type`.
   - `phase` must be `"inception"` or `"planning"`.
   - `role` must be `"pm"` or `"architect"`.
   - `type` must be `"clarification"`, `"decision"`, or `"priority"`.
   - `question` must be non-empty.
4. Invalid questions are skipped and a warning is logged.
5. Valid questions are stored with auto-generated IDs (`Q-001`, `Q-002`, ...).
6. If any valid questions were stored and the timeout is not `0`, the feature enters `waiting_for_human`.

### What this means for you

You don't need to create questions manually. The PM and Architect agents do it as part of their work. You only need to answer them through the UI.

The `POST /api/features/{id}/questions` endpoint is also available for programmatic use (e.g., testing or tooling), but in normal operation questions come from agent output.

---

## US-006: Feature List Shows Question Badge

The Dashboard shows a **question badge** on feature cards that have pending questions.

### Badge behavior

- The badge displays the **count of pending questions** (e.g., "2" for 2 pending questions).
- Badge color: **yellow/orange** to indicate "needs attention".
- Badge position: **top-right corner** of the feature card.
- Clicking the badge navigates to the feature detail page.
- The badge is **hidden** when:
  - The feature has no pending questions.
  - All questions are answered.
  - The questions API returns an error (graceful degradation — the list still renders, badge is not shown).

### How to use the badge

Scan the Dashboard for yellow/orange badges. Each badge tells you a feature is waiting for your input. Click the badge to jump straight to the questions.

---

## Common Workflows

### Workflow: Answer all questions and let the pipeline resume

1. Open the Dashboard.
2. See a yellow/orange badge with count "3" on a feature.
3. Click the badge.
4. Answer all 3 questions on the feature detail page.
5. After the last answer, the pipeline detects all questions are answered and auto-resumes. The feature status returns to `in_progress`.
6. The badge disappears from the Dashboard.

### Workflow: Ignore questions and let the timeout handle it

1. Open the Dashboard.
2. See a badge on a feature but don't have time to answer.
3. Do nothing.
4. After the configured timeout (default 30 minutes), the pipeline auto-assumes all pending questions and resumes.
5. The badge disappears. The feature detail page shows the assumptions in read-only state with "auto-assumed" labels.

### Workflow: Recirculate a feature that is waiting for human input

1. Open the feature detail page for a feature in `waiting_for_human` status.
2. Click **Recirculate** and select a target phase.
3. The feature transitions to `recirculated`, all questions are deleted, and the re-run begins.
4. The re-run may generate new questions with new IDs.

---

## Terminology Reference

| Term | Meaning |
|---|---|
| **Question** | A clarification or decision surfaced by the PM or Architect for human input. |
| **Clarification question** | A question with `type: "clarification"`, shown with a blue badge. |
| **Decision question** | A question with `type: "decision"`, shown with an orange badge. |
| **Priority question** | A question with `type: "priority"`, shown with a purple badge. |
| **Pending question** | A question with `status: "pending"` — not yet answered or assumed. |
| **Answered question** | A question with `status: "answered"` — a human provided an answer. Terminal state. |
| **Assumed question** | A question with `status: "assumed"` — the timeout expired and the pipeline generated an assumption. Terminal state. |
| **Waiting for human** | Feature status `waiting_for_human`. The pipeline is paused and waiting for human input. |
| **Timeout** | The configurable duration (in minutes) the pipeline waits before auto-assuming unanswered questions. |
| **Question badge** | The yellow/orange count badge shown on feature cards with pending questions. |
| **Human Responses** | The section injected into the agent's CONTEXT.md when re-dispatching after human interaction. |

---

## Reference

- **Spec**: `specs/human-interaction-points---allow-the-pipeline-to-pause-for-h/spec.md`
- **Acceptance criteria**: `specs/human-interaction-points---allow-the-pipeline-to-pause-for-h/acceptance.md`
- **API reference**: `specs/human-interaction-points---allow-the-pipeline-to-pause-for-h/docs/api-reference.md`
- **Changelog**: `specs/human-interaction-points---allow-the-pipeline-to-pause-for-h/docs/changelog.md`
- **Configuration**: `specs/human-interaction-points---allow-the-pipeline-to-pause-for-h/docs/configuration.md`