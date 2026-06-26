# Dev Team State Management

You are an agent in the Dev Team pipeline. Use the `devteam` CLI to manage your state — do NOT write state files manually.

## CLI Commands

### Submit Questions for User Feedback

When you need to ask the user clarifying questions:

1. Write a `questions.json` file with your questions:
```json
[
  {"phase":"inception","role":"pm","question":"Should the UI support dark mode?","type":"multiple_choice","options":["Yes","No","Other"]},
  {"phase":"inception","role":"pm","question":"What should the default page size be?","type":"multiple_choice","options":["10","25","50","Other"]}
]
```
Every question MUST include "Other" as the last option.

2. Submit the questions:
```bash
devteam questions ask <feature-id> --file questions.json
```

3. Signal that you need feedback:
```bash
devteam signal <feature-id> needs_feedback
```

The pipeline will pause and show your questions to the user in the web UI. Their answers will be provided to you on the next run.

### Signal Phase Outcome

When your work is complete or you need to send work back:

```bash
# Work is complete — advance to next phase
devteam signal <feature-id> pass

# Found issues — send back to a previous phase with notes
devteam signal <feature-id> recirculate:construction --notes "Missing error handling in handler.go:42"

# Blocked — cannot proceed
devteam signal <feature-id> failed --notes "External dependency unavailable"
```

### Add Notes for the Next Phase

Leave context for the next phase agent:

```bash
devteam notes add <feature-id> --phase inception --content "Spec has 3 user stories, P1 priority on dark mode"
```

### Query State

Check your current status or pending questions:

```bash
devteam feature status <feature-id>
devteam questions pending <feature-id>
devteam notes list <feature-id>
```

## Key Rules

1. **Always use the CLI** — never write `outcome.txt` or `questions.json` manually and expect the pipeline to find them
2. **The CLI handles all database operations** — you don't touch SQLite directly
3. **The CLI finds the database automatically** — you don't need to know where it is
4. **Questions are scoped to your feature** — they won't leak to other features
5. **Signal is mandatory** — if you don't signal, the pipeline assumes `pass`
6. **Notes help the next agent** — leave brief, actionable notes about what you decided or found