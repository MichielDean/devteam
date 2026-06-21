# Changelog — Spec 003: Human Interaction Points

All entries reference Spec 003.

## [0.3.0] - 2026-06-20

### Added
- `waiting_for_human` feature status, allowing the pipeline to pause at decision points during the `inception` and `planning` phases for human input (Spec 003, FR-001, US-003)
- `Question` artifact model with `pending` → `answered` / `pending` → `assumed` state machine, stored per-feature as `specs/{id}/questions.json` (Spec 003, FR-002)
- `GET /api/features/{id}/questions` — returns all questions for a feature; empty state returns `[]` (Spec 003, FR-003)
- `GET /api/features/{id}/questions/pending` — returns only `pending` questions; empty state returns `[]` (Spec 003, FR-003)
- `POST /api/features/{id}/questions` — creates a question with auto-generated `Q-{NNN}` id, `status: "pending"`, and `created_at` set (Spec 003, FR-003, US-005)
- `PATCH /api/features/{id}/questions/{questionId}` — answers a `pending` question with optimistic concurrency (409 on conflict) (Spec 003, FR-003, FR-012, US-001, US-002)
- `pending_questions_count` field on feature summary responses (Spec 003, FR-005, US-006)
- Question detection from agent output: the pipeline reads a `questions.json` artifact after PM/Architect dispatch, validates each entry, skips invalid entries with a warning, and stores valid questions (Spec 003, FR-011, US-005)
- Pipeline pause: after question detection, if the feature is in `inception` or `planning`, the feature transitions to `waiting_for_human` and the pipeline waits (Spec 003, FR-006, US-003)
- Timeout handler: after a configurable timeout, unanswered `pending` questions are auto-assumed and the feature resumes to `in_progress` (Spec 003, FR-009, US-004)
- Human Responses context injection: on re-dispatch after human interaction, a "Human Responses" section is appended to the agent's `CONTEXT.md` with each Q&A pair labeled by source (`human input` or `auto-assumed after timeout of N minutes`) (Spec 003, FR-007, US-001, US-002, US-004)
- `human_interaction_timeout_minutes` config field under `pipeline` in `devteam.yaml`; supports positive integers (wait N minutes), `0` (never pause, fully autonomous), and `-1` (wait indefinitely) (Spec 003, FR-009, US-004)
- SSE events `waiting_for_human`, `questions_answered`, and `questions_assumed` broadcast on the existing feature stream endpoint (Spec 003, FR-006)
- Web UI `QuestionCard` component: displays question text, color-coded type badge (clarification=blue, decision=orange, priority=purple), phase/role label, clickable option buttons, and an answer input; shows answered questions in read-only state with a green checkmark (Spec 003, FR-004, US-001, US-002)
- Web UI `QuestionBadge` component: shows the pending-question count on feature cards in the Dashboard; hidden when the count is zero (Spec 003, FR-005, US-006)
- Web UI Questions section on the feature detail page; hidden entirely when the feature has no questions (Spec 003, FR-004)
- Rejection of `POST /api/features/{id}/advance` for features in `waiting_for_human` status with 400 `Cannot advance feature in waiting_for_human status` (Spec 003, FR-008, AC-019)
- Questions cleared on recirculation: recirculating a feature in `waiting_for_human` deletes all its questions (Spec 003, FR-010, US-003)

### Changed
- Feature status machine extended to support `in_progress` ↔ `waiting_for_human` transitions, restricted to `inception` and `planning` phases (Spec 003, FR-001, FR-008)
- `devteam.yaml` `pipeline` section now accepts `human_interaction_timeout_minutes` (defaults to 30 when absent) (Spec 003, FR-009)
- CORS middleware now allows the `PATCH` method, required by the answer-question endpoint (Spec 003, FR-003)

### Fixed
- None — this is a net-new feature with no prior bugs being fixed.

### Breaking Changes
- None. The new `waiting_for_human` status, question endpoints, and config field are additive. Existing features without `questions.json` behave exactly as before. The `pending_questions_count` field is additive to the feature summary response.