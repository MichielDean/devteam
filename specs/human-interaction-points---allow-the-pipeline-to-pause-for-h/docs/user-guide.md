# User Guide — Spec 003: Human Interaction Points

The Dev Team pipeline can run in two modes: **autonomous** (the default, where it runs end-to-end without human input) and **interactive** (where it pauses at decision points so a human can provide input through the web UI).

This feature adds the interactive mode. When a human is available, the pipeline pauses during the **inception** and **planning** phases, surfaces questions through the web UI, and incorporates the human's answers back into the agent context. When no human responds within a configurable timeout, the pipeline falls back to autonomous mode with documented assumptions.

This guide covers every user story in Spec 003 using the terminology defined in the spec.

---

## Concepts

### Question

A **Question** is a clarification, decision, or priority prompt that an agent (the **PM** during inception, or the **Architect** during planning) surfaces for human input. Each question has:

- a **type** — `clarification`, `decision`, or `priority`
- a **phase** — `inception` or `planning`
- a **role** — `pm` or `architect`
- optional **options** — suggested answers the human can click
- a **status** — `pending`, `answered`, or `assumed`

A question is `pending` until either the human answers it (→ `answered`) or the timeout expires (→ `assumed`). Both terminal states are immutable.

### `waiting_for_human` feature status

When the pipeline detects questions for a feature in the `inception` or `planning` phase, the feature transitions from `in_progress` to `waiting_for_human`. The pipeline pauses. The feature returns to `in_progress` (and the pipeline resumes) when:

- all questions are answered by a human, or
- the timeout expires and unanswered questions are auto-assumed.

### Timeout

The timeout is configured in `devteam.yaml` under `pipeline.human_interaction_timeout_minutes`:

| Value | Behavior |
|---|---|
| `30` (default) | Wait 30 minutes, then auto-assume unanswered questions |
| positive integer | Wait that many minutes, then auto-assume |
| `0` | Never pause — fully autonomous. Questions are still stored but the feature never enters `waiting_for_human`; assumptions are generated immediately. |
| `-1` | Wait indefinitely — no timeout, no auto-assume |

The timeout is per-feature, starting when the feature enters `waiting_for_human`. It resets if a new question is added while the feature is already waiting.

---

## Answering PM Clarification Questions (US-001)

**When**: a feature is in the **inception** phase and the PM agent has surfaced clarification questions.

1. Open the Dev Team web UI (Dashboard).
2. Features with pending questions show a **badge** in the top-right corner of their card showing the pending question count (e.g., "2"). Click the badge (or the feature card) to open the feature detail page.
3. The **Questions** section shows each pending question as a card with:
   - the question text
   - a **type badge** — blue for `clarification`, orange for `decision`, purple for `priority`
   - the phase and role that generated it (e.g., "inception · pm")
   - suggested **option buttons** (if the agent provided options) — clicking an option fills the answer field with that option's text
   - a text input labeled "Type your answer..." and a **Submit** button
4. Type your answer (or click an option button), then click **Submit**.
5. The question card updates to a **read-only** state showing your answer with a green checkmark (✓).
6. When all questions are answered, the section shows "✓ All questions answered. Pipeline will resume." The feature returns to `in_progress` and the pipeline re-dispatches the PM agent with your answers in its context.

**Empty state**: if a feature has no questions, the Questions section is hidden entirely — no placeholder, no empty state message.

### Error scenarios

| What you did | What happens |
|---|---|
| Try to answer a question that's already answered | The API returns 409 Conflict: `Question Q-001 is already answered`. The first answer wins. |
| Try to answer a question that doesn't exist | 404 Not Found: `Question Q-999 not found`. |
| Submit an empty answer | 400 Bad Request: `answer must be 1-5000 characters`. |
| Submit an answer over 5000 characters | 400 Bad Request: `answer must be 1-5000 characters`. |

---

## Reviewing Architect Design Decisions (US-002)

**When**: a feature is in the **planning** phase and the Architect has surfaced design decisions as questions of type `decision`.

The flow is identical to answering PM clarification questions, with one emphasis: decision questions typically include suggested **option buttons** representing architecture choices, NFR tradeoffs, or scope boundaries. Click an option to populate the answer field, or type your own answer.

Once answered, the decision card shows the chosen answer in read-only state with a green checkmark.

**Empty state**: if no design decisions were surfaced, no decision cards appear and the pipeline proceeds normally.

---

## The Pipeline Pausing for Human Input (US-003)

The pipeline orchestrator checks for a `questions.json` artifact in the feature's spec directory after the PM or Architect agent completes its dispatch, before gate evaluation.

- **If questions exist** and the feature is in `inception` or `planning`: the questions are stored, the feature transitions to `waiting_for_human`, an SSE `waiting_for_human` event is broadcast, and the pipeline pauses.
- **If questions exist** but the feature is in `construction`, `review`, `testing`, or `delivery`: the questions are stored but the feature does **not** enter `waiting_for_human`. (Only inception and planning support human interaction.)
- **If no questions exist**: the pipeline proceeds normally to gate evaluation without pausing.

When all questions are answered (or the timeout expires), the feature returns to `in_progress` and the pipeline re-dispatches the current phase's agent with a **Human Responses** section injected into its context.

### Cancelling or recirculating a waiting feature

- **Cancel** a feature in `waiting_for_human`: the feature transitions to `cancelled`.
- **Recirculate** a feature in `waiting_for_human`: the feature transitions to the target phase and **all existing questions are deleted**. The re-run may generate new questions with new IDs.

---

## Falling Back to Autonomous Mode (US-004)

If no human responds within the configured timeout, the pipeline does not stall — it falls back to autonomous mode:

1. For each still-`pending` question, the pipeline generates an **assumption**.
2. Each such question transitions to `assumed` with the `assumption` field populated and `answered_at` set.
3. The feature returns to `in_progress`.
4. The pipeline re-dispatches the agent with a Human Responses section where assumed questions are labeled `[Source: auto-assumed after timeout of N minutes]`.

Already-`answered` questions are left untouched — only `pending` questions are assumed.

**If the timeout mechanism itself fails** (e.g., the background timer goroutine crashes), the feature stays in `waiting_for_human` and an error is logged. This is a safe failure mode: the pipeline will not silently proceed with wrong assumptions; it requires manual intervention.

---

## How Agents Create Questions (US-005)

Agents (PM and Architect) create questions during their dispatch by writing a `questions.json` artifact to the feature's spec directory. The file is a JSON array of question objects, each with:

- `phase` (`inception` or `planning`)
- `role` (`pm` or `architect`)
- `question` (1–2000 characters)
- `type` (`clarification`, `decision`, or `priority`)
- `options` (optional, 0–10 strings)

After dispatch completes, the pipeline:

1. Reads `questions.json` if it exists.
2. Validates each question object. **Invalid questions are skipped and a warning is logged** — they do not halt the pipeline.
3. Stores each valid question with an auto-generated ID (`Q-001`, `Q-002`, ...).
4. If any valid questions were stored, transitions the feature to `waiting_for_human`.

Invalid-question cases that are skipped with a warning:

- the file is not valid JSON
- a question is missing a required field
- a question has `phase: "construction"` (only `inception` and `planning` are valid)
- a question has an invalid `type` or `role`

---

## The Question Badge on the Dashboard (US-006)

The Dashboard (feature list page) shows a **badge** on any feature card that has pending questions:

- the badge displays the **count** of pending questions (e.g., "3")
- the badge is yellow/orange, indicating "needs attention"
- the badge sits in the **top-right corner** of the feature card
- clicking the badge navigates to the feature detail page
- the badge is **hidden** when the feature has zero pending questions (it is never shown with "0")
- when all of a feature's questions are answered, the badge disappears

**Graceful degradation**: if the questions API returns an error, the feature list still renders — the badge is simply not shown.

---

## Human Responses in Agent Context (FR-007)

When the pipeline re-dispatches an agent after human interaction, it injects a **Human Responses** section into the agent's `CONTEXT.md`. The section lists each question with its answer and the source:

```
=== Human Responses ===

Q-001: What is the target audience for this feature?
→ Internal developers
[Source: human input]

Q-002: Should we use WebSocket or SSE?
→ SSE is sufficient for the MVP
[Source: auto-assumed after timeout of 30 minutes]
```

- Answered questions are labeled `[Source: human input]`.
- Assumed questions are labeled `[Source: auto-assumed after timeout of N minutes]` (with the configured timeout value).
- Features with no questions do not include a Human Responses section.

This section appears after the role instructions and before the phase-specific instructions, so the agent sees the human's direction before doing its work.

---

## Security Notes

This is a single-user local development tool. Per the spec's security assumptions:

- **No authentication** is enforced in MVP. Authentication will be added in a future feature.
- **Questions and answers are immutable** once terminal: there is no UPDATE or DELETE endpoint for questions or answers. The only mutation is PATCH to answer a `pending` question.
- **XSS**: question and answer text is stored as-is and rendered as text (not HTML) in the UI, so `<script>` tags in answers are displayed, not executed.
- **Input validation** runs on every question endpoint (length limits, enum validation, option count limits).

---

## Configuration

See `docs/configuration.md` for the full configuration reference, including the `human_interaction_timeout_minutes` setting and all environment/configuration dependencies.