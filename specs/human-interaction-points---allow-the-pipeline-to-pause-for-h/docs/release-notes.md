# Release Notes — Spec 003: Human Interaction Points

**Spec**: 003 — Human Interaction Points — Allow the pipeline to pause for human input at decision points in inception and planning phases
**Version**: 0.3.0
**Date**: 2026-06-20
**Priority**: P1

---

## Release Order

This feature spans a **single repository**: `devteam` (https://github.com/lobsterdog/devteam, branch `main`).

The Dev Team platform is a monorepo: the Go backend (`internal/`, `cmd/`) and the React/TypeScript frontend (`ui/`) ship together from one repository. There are no shared libraries consumed by other repos, and no downstream consumer repos. Cross-repo release ordering is therefore **not applicable** — there is a single release unit.

### Release Order

1. **devteam** — v0.3.0
   - Reason: Single repository; backend and frontend ship together.
   - Breaking changes: none (additive only — see `changelog.md`).
   - Migration required: no.

### Tagging

Tag the repository:

```bash
git tag v0.3.0 -m "Spec 003: Human Interaction Points"
git push origin v0.3.0
```

---

## Build

The binary is built from `cmd/devteam/`:

```bash
go build -o ~/go/bin/devteam ./cmd/devteam/
```

Verified during delivery:
- `go build -o /tmp/devteam-ops ./cmd/devteam/` → **Success** (Go 1.26.1, linux/amd64, 10.1 MB binary)
- Binary reports `devteam 0.3.0` via `devteam version`

The frontend is built from `ui/`:

```bash
cd ui && npm run build
```

Verified during delivery:
- `npm run build` (`tsc -b && vite build`) → **Success** (471 modules transformed, 2.48s, no TypeScript or Vite errors)
- Output: `ui/dist/` (index.html, CSS, vendor-react, vendor-query, index, vendor-markdown chunks)

---

## Deployment Verification

All deployment verification steps were run during the delivery phase and pass:

### Service starts without panicking

```bash
/tmp/devteam-ops -http :18765
# → "Dev Team Web UI starting on :18765" — no panic, no crash
```

### API responds correctly

Hit every endpoint added by Spec 003 against a running server (`/tmp/devteam-ops -http :18768`):

| Endpoint | Scenario | Expected | Observed |
|---|---|---|---|
| `GET /api/features` | List features | 200, `pending_questions_count` present on every feature summary | ✅ 200, field present |
| `GET /api/features/{id}/questions` | Feature with no questions | 200, body `[]` | ✅ 200, `[]` |
| `GET /api/features/{id}/questions/pending` | Feature with no pending questions | 200, body `[]` | ✅ 200, `[]` |
| `POST /api/features/{id}/questions` | Valid payload | 201, auto-generated `Q-001`, `status: "pending"`, `created_at` set, `options` array present | ✅ 201, all fields match |
| `POST /api/features/{id}/questions` | Missing `question` field | 400, `{"error":"validation_error","details":"question is required"}` | ✅ 400, exact message |
| `POST /api/features/{id}/questions` | Invalid `phase` (`construction`) | 400, `{"error":"validation_error","details":"phase must be one of: inception, planning"}` | ✅ 400, exact message |
| `PATCH /api/features/{id}/questions/{qid}` | Valid answer on pending question | 200, `status: "answered"`, `answer` stored, `answered_at` set | ✅ 200, all fields match |
| `PATCH /api/features/{id}/questions/{qid}` | Re-answer an answered question | 409, `{"error":"conflict","details":"Question Q-001 is already answered"}` | ✅ 409, exact message |
| `PATCH /api/features/{id}/questions/Q-999` | Nonexistent question | 404, `{"error":"not_found","details":"Question Q-999 not found"}` | ✅ 404, exact message |
| `GET /api/features/nonexistent-id/questions` | Nonexistent feature | 404, `{"error":"not_found","details":"Feature nonexistent-id not found"}` | ✅ 404, exact message |

All error responses use the existing `{error, details}` envelope. All empty states return `[]` (not `null`, not 404).

### Frontend renders without console errors

Verified with a headless browser against a running server:

| Page | Console errors | Console warnings | Result |
|---|---|---|---|
| Dashboard (`/`) | 0 | 0 | ✅ |
| Feature detail (`/features/001-dev-team-platform`) | 0 | 0 | ✅ |

### UI behavior verified end-to-end

- **Question card** renders with question text, `clarification` type badge, `inception · pm` label, three option buttons, answer text input, and a Submit button (matches FR-004, AC-001).
- **Answering via the UI**: typed an answer, clicked Submit → card switched to read-only state showing the answer with a green ✓ and the message "✓ All questions answered. Pipeline will resume." appeared (matches AC-003, AC-069).
- **Question badge** on the Dashboard: showed "1" on the feature card while a question was pending, then disappeared after the question was answered (matches AC-033, AC-036, FR-005).

### Full test suite passes

```bash
go test ./...
# → 186 passed in 11 packages
```

No skipped, no failed tests.

---

## Configuration

The single new configuration field is `pipeline.human_interaction_timeout_minutes` in `devteam.yaml`. Default: `30`. See `docs/configuration.md` for the full reference (positive integer / `0` / `-1` semantics, pointer-type implementation detail, timeout reset behavior, server-restart recalculation).

No environment variables are required. No database migrations are required (file-based storage).

---

## Documentation Artifacts

| File | Contents |
|---|---|
| `docs/api.md` | Per-endpoint API documentation (method, path, request/response schemas, error responses, status machine, SSE events, concurrency) |
| `docs/user-guide.md` | User-facing documentation for every user story (US-001 through US-006) using spec terminology |
| `docs/configuration.md` | Configuration reference (`devteam.yaml` field, env vars, data files, dependencies) |
| `docs/changelog.md` | Changelog with every entry referencing Spec 003 |
| `docs/release-notes.md` | This file — release order, build, deployment verification, configuration |

---

## Terminology Consistency Check

Documentation terminology was checked against `spec.md`:

| Spec term | Used in docs? | Notes |
|---|---|---|
| `waiting_for_human` (feature status) | ✅ | Used verbatim in api.md, user-guide.md, configuration.md, changelog.md |
| `Question` (artifact) | ✅ | Used verbatim throughout |
| `pending` / `answered` / `assumed` (question status) | ✅ | Used verbatim; state machine documented |
| `clarification` / `decision` / `priority` (question type) | ✅ | Used verbatim; type badge colors documented |
| `inception` / `planning` (phase) | ✅ | Used verbatim; human interaction restricted to these phases documented |
| `pm` / `architect` (role) | ✅ | Used verbatim |
| `waiting_for_human` SSE event | ✅ | Documented in api.md |
| `questions.json` (artifact) | ✅ | Documented in configuration.md and user-guide.md |
| `human_interaction_timeout_minutes` (config) | ✅ | Documented in configuration.md |
| `Human Responses` (context section) | ✅ | Documented in user-guide.md and api.md |
| `Q-{NNN}` (id format) | ✅ | Documented in api.md and user-guide.md |
| `pending_questions_count` (summary field) | ✅ | Documented in api.md |

No terminology mismatches. No code-internal names leak into the documentation.

---

## Quality Gate Checklist

- [x] Documentation exists for every user story (US-001 through US-006) — see `docs/user-guide.md`
- [x] Documentation uses spec terminology (not code-internal names) — see Terminology Consistency Check above
- [x] Cross-repo release order is documented (single repo; N/A for ordering) — see Release Order above
- [x] Release notes reference the spec number (Spec 003) — every changelog entry references it
- [x] Each affected repo builds and deploys successfully — `go build` ✅, `npm run build` ✅
- [x] The service starts and responds to HTTP requests — verified
- [x] The frontend loads without console errors — verified (0 errors, 0 warnings on Dashboard and feature detail)
- [x] All smoke tests from the testing phase still pass — `go test ./...` → 186 passed
- [x] Configuration is documented — see `docs/configuration.md`
- [x] Breaking changes (if any) are documented with migration steps — none; documented as "none" in changelog

The release is ready.