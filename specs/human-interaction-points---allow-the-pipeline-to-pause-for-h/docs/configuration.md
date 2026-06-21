# Configuration — Spec 003: Human Interaction Points

This document describes the configuration introduced by **Spec 003: Human Interaction Points**.

---

## devteam.yaml

Spec 003 adds one new field to the `pipeline` section of `devteam.yaml`:

```yaml
pipeline:
  human_interaction_timeout_minutes: 30
```

### `pipeline.human_interaction_timeout_minutes`

| Property | Value |
|---|---|
| Type | integer (pointer — `*int` in Go) |
| Default | `30` (when field is absent) |
| Scope | Per-feature timeout, starts when the feature enters `waiting_for_human` status |
| Reset behavior | Resets when a new question is added while the feature is already in `waiting_for_human` status |

| Value | Behavior |
|---|---|
| Positive integer (e.g., `30`) | Wait that many minutes for a human response, then auto-assume unanswered questions. |
| `0` | Fully autonomous mode. Questions are still stored but the feature never enters `waiting_for_human`. Assumptions are immediately generated. |
| `-1` | Wait indefinitely. No timeout. The feature remains in `waiting_for_human` until a human answers or the feature is cancelled/recirculated. |
| Field absent | Defaults to 30 minutes. |

### Why a pointer type?

Go's `yaml.v3` unmarshals a missing integer field as `0` by default. Using `*int` (pointer) lets the config loader distinguish between:

- **Field not present** (nil pointer → use default 30)
- **Field explicitly set to 0** (pointer to 0 → fully autonomous mode)

This distinction matters because `0` is a meaningful value (fully autonomous mode), not just "unset".

---

## Environment Variables

Spec 003 does not introduce any new environment variables. The feature reads its configuration exclusively from `devteam.yaml`.

---

## Configuration Files

| File | Purpose | Changed by Spec 003? |
|---|---|---|
| `devteam.yaml` | Pipeline configuration | Yes — adds `pipeline.human_interaction_timeout_minutes` |
| `specs/{id}/.devteam-state.yaml` | Per-feature pipeline state | Yes — may now contain `waiting_for_human` status |
| `specs/{id}/questions.json` | Per-feature question storage | Yes — new file, JSON array of Question objects |

### `specs/{id}/questions.json`

A JSON array of Question objects. Created when an agent produces questions during inception or planning. Each question has the fields described in the [API reference](api-reference.md#data-model).

Example:
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
    "created_at": "2026-06-20T15:30:00Z",
    "answered_at": null
  }
]
```

When the feature has no questions, this file is absent (not an empty array). The API returns `[]` for features with no questions file.

---

## Dependencies

Spec 003 introduces **no new external dependencies**. It reuses:

- Go standard library (`net/http`, `encoding/json`, `os`, `time`, `sync`)
- `gopkg.in/yaml.v3` (existing — for config parsing)
- Existing `internal/feature`, `internal/api`, `internal/pipeline`, `internal/config`, `internal/spec` packages

The frontend reuses the existing React + TypeScript + TanStack Query + Tailwind CSS stack from Spec 002.

---

## Runtime Requirements

- **Go**: 1.26.1+ (unchanged from Spec 002)
- **Node.js**: 20+ (unchanged from Spec 002, only for frontend build)
- **Browser**: Latest Chrome, Firefox, Safari, Edge (unchanged from Spec 002)
- **OS**: Linux, macOS (unchanged)

---

## Verification

To verify the configuration is loaded correctly:

1. Set `human_interaction_timeout_minutes: 5` in `devteam.yaml`.
2. Start the server: `devteam -http :8080`
3. Create a question via the API or by running an inception phase that produces a `questions.json` artifact.
4. Verify the feature enters `waiting_for_human` status.
5. Wait 5 minutes without answering.
6. Verify the feature returns to `in_progress` and the questions have `status: "assumed"` with a non-null `assumption` field.

To verify fully autonomous mode:

1. Set `human_interaction_timeout_minutes: 0` in `devteam.yaml`.
2. Create questions via the API.
3. Verify the feature does **not** enter `waiting_for_human` and the questions are immediately `assumed`.

To verify indefinite wait:

1. Set `human_interaction_timeout_minutes: -1` in `devteam.yaml`.
2. Create questions via the API.
3. Verify the feature enters `waiting_for_human` and remains in that status indefinitely (no auto-assume).