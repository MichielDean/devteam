# Changelog

All notable changes to the Dev Team project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [1.1.0] — 2026-06-21

### Added — Spec 003: Human Interaction Points

#### Backend (Go)

- Feature status `waiting_for_human` with valid transitions from `in_progress` (only during inception or planning phases) and back to `in_progress` (when all questions are answered or the timeout expires) (spec #003)
- Question model (`internal/feature/question.go`) with `QuestionStore` interface and `FileQuestionStore` implementation storing questions as `specs/{id}/questions.json` (spec #003)
- Question state machine: `pending → answered` (human responds), `pending → assumed` (timeout expires). Both `answered` and `assumed` are terminal states. (spec #003)
- Question ID auto-generation as `Q-{NNN}` (sequential within a feature, human-readable) (spec #003)
- `GET /api/features/{id}/questions` — list all questions for a feature; returns `[]` for empty state (spec #003)
- `POST /api/features/{id}/questions` — create a new question with input validation (phase, role, question, type, options) (spec #003)
- `PATCH /api/features/{id}/questions/{questionId}` — answer a pending question with optimistic concurrency (409 on already-answered/assumed) (spec #003)
- `GET /api/features/{id}/questions/pending` — list only pending questions; returns `[]` for empty state (spec #003)
- `pending_questions_count` field on `FeatureSummaryResponse` for the feature list badge (spec #003)
- Question detection in the pipeline after PM or Architect dispatch (`internal/pipeline/question.go`): reads `questions.json` artifact, validates each question, skips invalid ones with a warning, stores valid ones, and sets the feature to `waiting_for_human` (spec #003)
- Timeout handler goroutine: after the configured timeout, marks pending questions as `assumed` with an auto-generated assumption, returns the feature to `in_progress`, and re-dispatches the agent (spec #003)
- "Human Responses" context injection (`Pipeline.BuildHumanResponsesContext`): builds a section for CONTEXT.md listing each Q&A pair with its source (`[Source: human input]` or `[Source: auto-assumed after timeout of N minutes]`), appended after role instructions and before phase-specific instructions (spec #003)
- `pipeline.human_interaction_timeout_minutes` config field in `devteam.yaml` using `*int` pointer type to distinguish "field absent" (default 30) from "explicitly set to 0" (fully autonomous mode); supports `-1` for indefinite wait (spec #003)
- SSE events `waiting_for_human`, `questions_answered`, `questions_assumed` broadcast through the existing `broadcastSSE` mechanism (spec #003)
- `waiting_for_human` transition rules in the feature state machine: valid only from `in_progress` during inception or planning; `waiting_for_human → cancelled` and `waiting_for_human → recirculated` (with questions cleared) are valid (spec #003)
- `Advance` endpoint rejects features in `waiting_for_human` status with 400 Bad Request `{"error": "validation_error", "details": "Cannot advance feature in waiting_for_human status"}` (spec #003)
- Questions cleared on recirculation: `QuestionStore.DeleteQuestionsForFeature` is called when a feature is recirculated, so the re-run generates fresh questions with new IDs (spec #003)
- Concurrent answer handling: the PATCH endpoint uses optimistic concurrency — if the question status is no longer `pending`, the second request receives 409 Conflict (spec #003)
- Unit tests for question validation, ID generation, state transitions, and concurrent answer handling (spec #003)
- Integration tests for all four question endpoints covering happy path, error paths (400/404/409), and empty state (`[]` not `null`) (spec #003)

#### Frontend (React + TypeScript)

- `QuestionCard` component: displays question text, type badge (color-coded: `clarification` = blue, `decision` = orange, `priority` = purple), phase/role labels, suggested options as clickable buttons, and a text input for answering (spec #003)
- `QuestionCard` read-only states: answered questions show the answer with a green checkmark; assumed questions show the assumption with an "auto-assumed" label (spec #003)
- `QuestionBadge` component: yellow/orange count badge on `FeatureCard`, top-right corner, hidden when `pending_questions_count` is 0, links to the feature detail page (spec #003)
- `FeatureDetail` modification: adds a "Questions" section that lists `QuestionCard` components when the feature has questions; section is completely hidden when the feature has no questions (spec #003)
- `FeatureCard` modification: adds `QuestionBadge` to the top-right corner (spec #003)
- API client functions: `listQuestions`, `createQuestion`, `answerQuestion`, `listPendingQuestions` (spec #003)
- TypeScript `Question`, `CreateQuestionRequest`, `AnswerQuestionRequest` types (spec #003)
- `waiting_for_human` added to `STATUS_LABELS` and the status color map (spec #003)
- `pending_questions_count` added to `FeatureSummary` (spec #003)

### Changed — Spec 003: Human Interaction Points

- `internal/feature/types.go` — Added `StatusWaitingHuman` status constant and `Question` struct (spec #003)
- `internal/feature/state.go` — Added `waiting_for_human` transition validation: valid from `in_progress` only when the current phase is inception or planning (spec #003)
- `internal/api/server.go` — Added question route handlers and `questionStore` dependency; added `PATCH` to CORS allowed methods (spec #003)
- `internal/api/dto.go` — Added `Question` DTOs and `pending_questions_count` to `FeatureSummaryResponse` (spec #003)
- `internal/pipeline/pipeline.go` — Added `questionStore` field, `BuildHumanResponsesContext` method, question detection after agent dispatch, and human responses injection into CONTEXT.md (spec #003)
- `internal/pipeline/process.go` — `ProcessAsync` loop checks for `waiting_for_human` status and breaks the dispatch loop when the feature is waiting for human input (spec #003)
- `internal/config/config.go` — Added `HumanInteractionTimeoutMinutes *int` to `PipelineConfig` (spec #003)
- `devteam.yaml` — Added `pipeline.human_interaction_timeout_minutes: 30` (spec #003)

### Architecture — Spec 003: Human Interaction Points

- Questions are stored as a JSON file (`questions.json`) per feature in the spec directory, consistent with the existing file-based artifact pattern. No database. (spec #003)
- The timeout is per-feature, starting when the feature enters `waiting_for_human` status, and resets when a new question is added while the feature is already waiting. (spec #003)
- Human interaction is supported only during inception and planning. Construction, review, testing, and delivery remain fully autonomous. (spec #003)
- The pipeline orchestrator runs a background goroutine with a timer to check for timeout expiration, rather than requiring an external scheduler. (spec #003)
- On server restart, the timeout timer is recalculated from the original `waiting_for_human` timestamp. (spec #003)
- Fully autonomous mode (`timeout_minutes: 0`) still stores questions but immediately generates assumptions — the pipeline does not pause at all. (spec #003)

### Security — Spec 003: Human Interaction Points

- Input validation at the API boundary for all question fields (phase, role, question, type, options) and answer field (1–5000 chars) (spec #003)
- Questions and answers are immutable once created/answered — no UPDATE or DELETE endpoints (spec #003)
- No authentication for MVP (single-user local tool, consistent with Spec 002 assumptions) (spec #003)
- XSS prevention: question and answer text rendered as text (not HTML) in the UI (spec #003)

### Resilience — Spec 003: Human Interaction Points

- Invalid `questions.json` is skipped with a warning — the pipeline does not crash (spec #003)
- If the timeout goroutine crashes, the feature stays in `waiting_for_human` (safe failure mode requiring manual intervention) (spec #003)
- Question store file operations are atomic (write to temp file, then rename) (spec #003)