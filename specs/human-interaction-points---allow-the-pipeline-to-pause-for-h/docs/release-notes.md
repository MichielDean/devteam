# Release Notes — Spec 003: Human Interaction Points

**Spec**: 003 — Human Interaction Points
**Released**: 2026-06-21
**Priority**: P1

---

## Summary

The Dev Team pipeline can now pause at decision points during inception and planning to ask a human for input. The PM (inception) and Architect (planning) agents surface questions through the web UI; a product owner answers them; the pipeline resumes with the answers injected into the agent's context. When no human responds within a configurable timeout, the pipeline falls back to autonomous mode with documented assumptions.

This feature does not change the pipeline's behavior during construction, review, testing, or delivery — those phases remain fully autonomous.

---

## What's New

### Human Interaction at Decision Points

- The PM agent can surface **clarification questions** during inception.
- The Architect agent can surface **design decisions** (as questions) during planning.
- When questions exist, the feature enters `waiting_for_human` status and the pipeline pauses.
- When all questions are answered (or the timeout expires), the pipeline resumes and re-dispatches the agent with the answers in context.

### Question API

Four new endpoints under `/api/features/{id}/questions`:

- `GET /api/features/{id}/questions` — list all questions
- `POST /api/features/{id}/questions` — create a question
- `PATCH /api/features/{id}/questions/{questionId}` — answer a pending question
- `GET /api/features/{id}/questions/pending` — list pending questions

Full contracts are in [api-reference.md](api-reference.md).

### Web UI

- **Question cards** on the feature detail page, with color-coded type badges (`clarification` = blue, `decision` = orange, `priority` = purple), suggested options as clickable buttons, and a text input for answering.
- **Question badge** on feature cards in the Dashboard, showing the count of pending questions. Yellow/orange, top-right corner, hidden when there are no pending questions.
- Answered questions show a green checkmark; assumed questions show an "auto-assumed" label.

### Configurable Timeout

```yaml
pipeline:
  human_interaction_timeout_minutes: 30
```

- Default: 30 minutes
- `0`: fully autonomous mode (questions stored, immediately assumed, no pause)
- `-1`: wait indefinitely
- Positive integer: wait that many minutes, then auto-assume

See [configuration.md](configuration.md) for details.

---

## Upgrade Guide

### 1. Rebuild the binary

```bash
go build -o ~/go/bin/devteam ./cmd/devteam/
```

The frontend is embedded via `embed.FS`, so `go generate` is required if you're building from a clean checkout:

```bash
go generate ./cmd/devteam
go build -o ~/go/bin/devteam ./cmd/devteam/
```

### 2. Update `devteam.yaml` (optional)

The default timeout is 30 minutes. To change it, add or update:

```yaml
pipeline:
  human_interaction_timeout_minutes: 30
```

If the field is absent, the default (30 minutes) is used. No migration is required.

### 3. Restart the server

```bash
devteam -http :8080
```

Existing features continue to work. Features that were already in `in_progress` are unaffected. Only features that enter inception or planning after the upgrade can produce questions.

---

## Breaking Changes

None.

- Existing API endpoints from Spec 002 are unchanged.
- The `GET /api/features` response gains a new `pending_questions_count` field, which is always present (0 when no pending questions). This is additive and does not break existing clients.
- The feature state machine gains a new status (`waiting_for_human`) but no existing transitions are removed or altered.

---

## Migration Steps

None required. The upgrade is backward-compatible.

If you were previously running with a custom `devteam.yaml`, you can optionally add `pipeline.human_interaction_timeout_minutes` to control the timeout behavior. Without it, the default (30 minutes) applies.

---

## Verification

Run the test suite:

```bash
go test ./...
```

All 189 tests across 12 packages pass.

Start the server and verify the new endpoints:

```bash
devteam -http :8080
curl http://localhost:8080/api/features/human-interaction-points---allow-the-pipeline-to-pause-for-h/questions
# []

curl -X POST http://localhost:8080/api/features/human-interaction-points---allow-the-pipeline-to-pause-for-h/questions \
  -H "Content-Type: application/json" \
  -d '{"phase":"inception","role":"pm","question":"What is the target audience?","type":"clarification","options":["Internal","External"]}'
# 201 Created

curl http://localhost:8080/api/features/human-interaction-points---allow-the-pipeline-to-pause-for-h/questions/pending
# [question with status "pending"]
```

Open the Dashboard in a browser (`http://localhost:8080/`) and verify:
- The feature card shows a yellow/orange badge with the pending question count.
- Clicking the badge navigates to the feature detail page.
- The question card renders with a blue `clarification` badge, the suggested option buttons, and a text input.
- Submitting an answer updates the card to a read-only state with a green checkmark.
- The badge count decreases on the Dashboard.

No browser console errors should appear on the Dashboard or the feature detail page.

---

## Cross-Repo Release Order

This feature is contained entirely in the `devteam` repository. It does not span repos and there is no cross-repo release order to follow.

| Repo | Version | Reason |
|---|---|---|
| `devteam` | 1.1.0 | Single repo — backend and frontend ship together via `embed.FS` |

---

## References

- **Spec**: `specs/human-interaction-points---allow-the-pipeline-to-pause-for-h/spec.md`
- **Acceptance criteria**: `specs/human-interaction-points---allow-the-pipeline-to-pause-for-h/acceptance.md`
- **API reference**: `specs/human-interaction-points---allow-the-pipeline-to-pause-for-h/docs/api-reference.md`
- **User guide**: `specs/human-interaction-points---allow-the-pipeline-to-pause-for-h/docs/user-guide.md`
- **Configuration**: `specs/human-interaction-points---allow-the-pipeline-to-pause-for-h/docs/configuration.md`
- **Changelog**: `specs/human-interaction-points---allow-the-pipeline-to-pause-for-h/docs/changelog.md`