# Configuration — Spec 003: Human Interaction Points

This documents all configuration added or relevant to Spec 003.

---

## `devteam.yaml`

The feature adds one new field under the existing `pipeline` section.

### `pipeline.human_interaction_timeout_minutes`

| Property | Value |
|---|---|
| Type | integer (YAML) / `*int` (Go — pointer to distinguish "not set" from "explicitly 0") |
| Default | `30` (used when the field is absent) |
| Section | `pipeline` |

Controls how long the pipeline waits for human input before falling back to autonomous mode.

```yaml
pipeline:
  human_interaction_timeout_minutes: 30
```

#### Behavior by value

| Value | Mode | Behavior |
|---|---|---|
| positive integer (e.g., `30`) | Interactive with timeout | Feature enters `waiting_for_human`; after N minutes with no human response, all `pending` questions are auto-assumed and the feature resumes. |
| `0` | Fully autonomous | Questions are still stored, but the feature **never** enters `waiting_for_human`. Assumptions are generated **immediately**. The pipeline does not pause. |
| `-1` | Interactive, no timeout | Feature enters `waiting_for_human` and waits **indefinitely**. No auto-assume. A human must answer or the feature must be cancelled/recirculated. |
| absent | Interactive with timeout (default 30) | Treated as if `30` was set. |

#### Timeout semantics

- The timeout is **per-feature**, starting when the feature enters `waiting_for_human`.
- The timeout **resets** if a new question is added while the feature is already in `waiting_for_human`. This avoids premature assumption generation while the human is actively engaging. (Spec assumption.)
- On server restart, the timeout is recalculated from the feature's `waiting_for_human` timestamp — the timer does not silently restart from zero.
- Only `pending` questions are assumed on timeout. Already-`answered` questions are left unchanged.

#### Pointer-type note (Go implementation detail)

Go's `yaml.v3` unmarshals a missing integer field as the zero value (`0`). To distinguish "field not present in the YAML" (→ use default 30) from "field explicitly set to 0" (→ fully autonomous mode), the Go config struct uses `*int`. A nil pointer means "not set"; a pointer to 0 means "explicitly autonomous". Consumers call `PipelineConfig.GetHumanInteractionTimeoutMinutes()` which returns the resolved integer (defaulting to 30 when nil).

---

## Environment Variables

No new environment variables are introduced by this feature. The Dev Team server is started with the existing `-http` flag:

```bash
devteam -http :8080
```

No environment variables are required for human interaction points to function.

---

## Configuration Files

| File | Purpose | Modified by Spec 003? |
|---|---|---|
| `devteam.yaml` | Pipeline, roles, phases, extensions configuration | Yes — added `pipeline.human_interaction_timeout_minutes` |
| `specs/{feature-id}/.devteam-state.yaml` | Per-feature pipeline state | Yes — features can now have `status: waiting_for_human` |
| `specs/{feature-id}/questions.json` | Per-feature question store (new) | Yes — new artifact, JSON array of Question objects |
| `specs/{feature-id}/CONTEXT.md` | Per-feature agent context (regenerated each dispatch) | Yes — may now contain a "Human Responses" section |

---

## Dependencies

No new external dependencies. The feature uses:

- **Backend**: Go standard library, `gopkg.in/yaml.v3` (already in `go.mod`), `encoding/json` (stdlib)
- **Frontend**: existing React + TypeScript + TanStack Query + TailwindCSS stack (no new npm dependencies)
- **Storage**: file-based (YAML for feature state, JSON for questions) — no database, consistent with existing patterns
- **Real-time**: existing SSE mechanism (`broadcastSSE` / `GET /api/features/{id}/stream`)

---

## Data Files

### `specs/{feature-id}/questions.json`

A JSON array of Question objects. Created on first question creation; deleted when the feature is recirculated. Each Question object's shape:

```json
[
  {
    "id": "Q-001",
    "feature_id": "003-human-interaction-points",
    "phase": "inception",
    "role": "pm",
    "question": "What is the target audience?",
    "type": "clarification",
    "options": ["Internal developers", "External users"],
    "answer": null,
    "assumption": null,
    "status": "pending",
    "created_at": "2026-06-20T15:30:00-06:00",
    "answered_at": null
  }
]
```

When a feature has no questions, the file does not exist; the API returns `[]` (not `null`).

### `questions.json` artifact (agent output)

Agents (PM, Architect) produce a `questions.json` artifact in the feature spec directory during their dispatch. This is the **detection input** the pipeline reads after dispatch. It is a JSON array of question *creation requests* (no `id`, `status`, `created_at`, or `answered_at` — those are server-generated):

```json
[
  {
    "phase": "inception",
    "role": "pm",
    "question": "What is the target audience?",
    "type": "clarification",
    "options": ["Internal developers", "External users"]
  }
]
```

Invalid entries (missing required fields, wrong enum values, `phase` outside `inception`/`planning`) are skipped with a logged warning; valid entries are stored as `pending` questions.

---

## Database Migrations

Not applicable. The Dev Team platform is file-based — there is no database. No migrations are required.