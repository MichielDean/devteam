---
name: devteam-state
description: Manage Dev Team pipeline state — submit questions for user feedback, signal phase outcomes (pass/recirculate/needs_feedback/failed), add notes for next phase, query feature status. Use this skill when you are an agent dispatched by the Dev Team pipeline and need to communicate with the pipeline orchestrator.
compatibility: Requires the `devteam` CLI binary in PATH
metadata:
  pipeline: devteam
  purpose: state-management
---

# Dev Team State Management

You are an agent dispatched by the Dev Team pipeline. Use the `devteam` CLI to communicate with the pipeline orchestrator. **Never write state files manually** — the CLI handles all database operations.

## Your Feature ID

Your feature ID is provided in your CONTEXT.md file. Look for the line:
```
Feature: <feature-id>
```

Use this ID in all CLI commands below.

## Submit Questions for User Feedback

When you need the user to answer clarifying questions before you can proceed:

### 1. Write questions.json

```json
[
  {
    "phase": "inception",
    "role": "pm",
    "question": "Should the UI support dark mode?",
    "type": "multiple_choice",
    "options": ["Yes", "No", "Other"]
  }
]
```

Rules:
- Every question MUST include `"Other"` as the last option
- `type` must be `multiple_choice` or `open_ended`
- `phase` must match your current phase (`inception` or `planning`)
- `role` must match your role (`pm`, `architect`, etc.)
- Ask 3-8 questions for inception, 1-5 for planning

### 2. Submit the questions

```bash
devteam questions ask <feature-id> --file questions.json
```

This reads the file, stores questions in the database scoped to your feature, and deletes the file.

### 3. Signal that you need feedback

```bash
devteam signal <feature-id> needs_feedback
```

The pipeline will pause and show your questions to the user in the web UI. When the user answers, the pipeline resumes your phase with their answers in your context.

### 4. After receiving answers

Check if you need MORE questions. If so, repeat steps 1-3. If you have enough clarity, proceed with your work and signal `pass` when done.

## Signal Phase Outcome

When your work is complete or you need to route work elsewhere:

```bash
# Your work is complete — advance to the next phase
devteam signal <feature-id> pass

# You found issues — send work back to a previous phase
devteam signal <feature-id> recirculate:construction --notes "Missing error handling in handler.go:42 — returns 500 instead of 400"

# You are blocked and cannot proceed
devteam signal <feature-id> failed --notes "External dependency unavailable, cannot compile"
```

### Recirculate targets by phase

| Your phase | Default recirculate target |
|---|---|
| Review | `recirculate:construction` |
| Testing | `recirculate:construction` |
| Delivery | `recirculate:construction` |
| Planning | `recirculate:inception` |
| Construction | `recirculate:planning` |

## Add Notes for the Next Phase

Leave context for the agent that runs after you:

```bash
devteam notes add <feature-id> --phase inception --content "Spec has 3 user stories. P1 priority on dark mode. Assumed localStorage for persistence."
```

Notes are stored in the database and injected into the next phase agent's CONTEXT.md automatically.

## Query State

Check your current status:

```bash
# Get feature status
devteam feature status <feature-id>

# List pending questions (if any)
devteam questions pending <feature-id>

# List all notes for this feature
devteam notes list <feature-id>
```

## Key Rules

1. **Always use the CLI** — never write `outcome.txt` or `questions.json` and expect the pipeline to find them
2. **The CLI handles all database operations** — you never touch SQLite directly
3. **The CLI finds the database automatically** — you don't need to know where it is
4. **Questions are scoped to your feature** — they won't leak to other features
5. **Signal is mandatory** — if you don't signal, the pipeline assumes `pass`
6. **Notes help the next agent** — leave brief, actionable notes about what you decided or found
7. **"Other" is mandatory** — every multiple_choice question must include "Other" as the last option