# Plan 003: Human Interaction Points

## Summary

Add the ability for the Dev Team pipeline to pause at decision points during inception and planning phases, surface questions to a human via the web UI, and incorporate their answers back into the agent context. When no human responds within a configurable timeout, the pipeline falls back to autonomous mode with documented assumptions.

## Technical Context

### Language, Framework, Dependencies

- **Backend**: Go 1.26.1, standard library HTTP server, `gopkg.in/yaml.v3` for YAML
- **Frontend**: React + TypeScript, Vite, TanStack Query, TailwindCSS, React Router
- **State**: YAML files on disk (`.devteam-state.yaml` per feature, `questions.json` per feature for question storage)
- **No database**: All state is file-based, consistent with existing patterns
- **Real-time**: SSE (Server-Sent Events) for push notifications, consistent with existing `streamFeature` endpoint
- **No external dependencies** beyond what's already in `go.mod`

### Brownfield Analysis

**Existing architecture**:
- `internal/feature/` — Feature model, state machine, types (Feature, PhaseState, Status, Phase, ArtifactType)
- `internal/api/` — HTTP server with CRUD endpoints, SSE streaming, middleware (recovery, CORS)
- `internal/pipeline/` — Pipeline orchestrator: RunPhaseWithAgent, ProcessAsync, gate evaluation, context building
- `internal/spec/` — SpecProvider reads/writes feature state and artifacts from disk
- `internal/rules/` — RuleLoader builds context for agent dispatch (BuildContext method)
- `internal/config/` — Config loads from `devteam.yaml`
- `ui/src/` — React SPA with Dashboard, FeatureDetail, FeatureCard, FeatureList components

**Existing patterns to follow**:
- Feature state stored as YAML in `specs/{id}/.devteam-state.yaml`
- Artifacts stored as files in `specs/{id}/` directory
- API uses `{error: string, details: string}` error response format
- API uses `writeJSON` / `writeError` helpers
- SSE events broadcast via `broadcastSSE` method
- Feature state machine uses `Status` type constants (`StatusDraft`, `StatusInProgress`, etc.)
- DTOs in `dto.go` convert domain types to API responses
- Frontend uses TanStack Query for data fetching and mutation

**What changes**:
- Add `StatusWaitingHuman` status constant and transition rules
- Add `Question` model and `QuestionStore` for persistence
- Add 4 new API endpoints for questions
- Add question detection logic in pipeline after agent dispatch (between `RunPhaseWithAgent` return and gate evaluation in `ProcessAsync`)
- Add timeout handler goroutine in pipeline for auto-assume
- Add context injection of human responses in `Pipeline.RunPhaseWithAgent` (append "Human Responses" section to context string before writing CONTEXT.md)
- Add UI components for question cards, badges, and answer input
- Add `HumanInteractionTimeoutMinutes` config field to `PipelineConfig` using `*int` pointer type to distinguish zero (explicit autonomous) from missing (default 30)
- Add SSE events for `waiting_for_human`, `questions_answered`, `questions_assumed` status changes
- Add `questionStore` field to `Pipeline` struct and `Server` struct

**What stays the same**:
- File-based storage pattern (no database)
- YAML for feature state (`.devteam-state.yaml`), JSON for questions (`questions.json`)
- SSE for real-time updates via `broadcastSSE` method and `sseClients sync.Map`
- Existing Feature struct fields (only adding status value)
- Existing API endpoints (only adding new ones)
- Existing `BuildContext(phase, roleName, priority)` signature in RuleLoader (Human Responses injected at Pipeline level, not RuleLoader)
- Existing DTO conversion pattern in `dto.go` (nil slices → empty slices)
- Existing middleware chain order: `recoveryMiddleware → corsMiddleware → mux`
- Existing error response format: `ErrorResponse{Error: string, Details: string}`

## Project Structure

### Backend (Go)

```
internal/
├── feature/
│   ├── feature.go          # MODIFY: add WaitForHuman(), ResumeFromWaitingHuman() methods
│   ├── types.go            # MODIFY: add StatusWaitingHuman, Question struct, QuestionStore interface
│   ├── state.go            # MODIFY: add CanTransitionToWaitingHuman() transition validation
│   └── question.go         # NEW: Question model, FileQuestionStore implementation, validation logic
├── api/
│   ├── server.go           # MODIFY: add question route handlers, add questionStore dependency, add PATCH to CORS
│   ├── dto.go              # MODIFY: add Question DTOs, add pending_questions_count to FeatureSummaryResponse
│   └── server_test.go      # MODIFY: add question endpoint tests
├── pipeline/
│   ├── pipeline.go          # MODIFY: add questionStore field, add BuildHumanResponsesContext, add question detection after dispatch, add human responses injection
│   ├── question.go          # NEW: DetectQuestions, HandleTimeout, ShouldPauseForHuman functions
│   ├── process.go           # MODIFY: add waiting_for_human handling in ProcessAsync loop, break loop when waiting
│   └── convergence.go       # NO CHANGE
├── config/
│   ├── config.go            # MODIFY: add HumanInteractionTimeoutMinutes *int to PipelineConfig
│   └── config_test.go       # MODIFY: add config parsing test for timeout values
├── rules/
│   └── loader.go            # NO CHANGE (human responses injected at Pipeline level, not RuleLoader)
└── spec/
    └── provider.go          # MODIFY: add QuestionFile helper methods for read/write

devteam.yaml                # MODIFY: add pipeline.human_interaction_timeout_minutes: 30
```

### Frontend (React/TypeScript)

```
ui/src/
├── api/
│   └── client.ts           # MODIFY: add question API functions
├── types/
│   └── index.ts            # MODIFY: add Question types, add waiting_for_human to STATUS_LABELS
├── components/
│   ├── QuestionCard.tsx     # NEW: question card component
│   ├── QuestionBadge.tsx    # NEW: badge for feature list
│   └── FeatureCard.tsx     # MODIFY: add QuestionBadge
└── pages/
    ├── Dashboard.tsx        # NO CHANGE (QuestionBadge is in FeatureCard)
    └── FeatureDetail.tsx    # MODIFY: add QuestionCard section
```

## Data Model

### Question Entity

```go
type Question struct {
    ID          string    `json:"id" yaml:"id"`                       // Q-001, Q-002, etc.
    FeatureID   string    `json:"feature_id" yaml:"feature_id"`       // Feature this belongs to
    Phase       string    `json:"phase" yaml:"phase"`                 // "inception" or "planning"
    Role        string    `json:"role" yaml:"role"`                   // "pm" or "architect"
    Question    string    `json:"question" yaml:"question"`           // 1-2000 chars
    Type        string    `json:"type" yaml:"type"`                   // "clarification", "decision", "priority"
    Options     []string  `json:"options" yaml:"options"`            // 0-10 suggested answers, each 1-500 chars
    Answer      *string   `json:"answer" yaml:"answer"`               // null until answered, max 5000 chars
    Assumption  *string   `json:"assumption" yaml:"assumption"`        // null until timeout, max 5000 chars
    Status      string    `json:"status" yaml:"status"`               // "pending", "answered", "assumed"
    CreatedAt   time.Time `json:"created_at" yaml:"created_at"`       // auto-set on creation
    AnsweredAt  *time.Time `json:"answered_at" yaml:"answered_at"`      // null until answered/assumed
}
```

**Storage**: Each feature's questions stored as `specs/{id}/questions.json` — a JSON array of Question objects. This follows the existing artifact pattern (files per feature in the spec directory).

**Question ID generation**: Auto-incrementing within a feature. Read existing questions, find max number, increment. Format: `Q-{NNN}` (e.g., Q-001, Q-002).

### Feature Status Extension

Add `StatusWaitingHuman Status = "waiting_for_human"` to existing status constants.

Valid transitions:
- `in_progress` → `waiting_for_human` (only when current phase is inception or planning)
- `waiting_for_human` → `in_progress` (when all questions answered or timeout expires)
- `waiting_for_human` → `cancelled` (user cancels)
- `waiting_for_human` → `recirculated` (user recirculates — questions cleared)

Invalid transitions:
- `waiting_for_human` → `waiting_for_human` (no self-transition)
- `waiting_for_human` → `passed`, `waiting_for_human` → `gate_blocked` (must return to in_progress first)
- `draft` → `waiting_for_human` (feature must be started)
- `done` → `waiting_for_human` (terminal state)
- `cancelled` → `waiting_for_human` (terminal state)

### Question Status State Machine

```
pending → answered   (human provides answer via PATCH)
pending → assumed    (timeout expires, auto-assumed)
answered → (terminal, no further transitions)
assumed → (terminal, no further transitions)
```

### HumanInteractionConfig

```yaml
pipeline:
  human_interaction_timeout_minutes: 30
```

Added to `PipelineConfig` in `config.go`. Default: 30 minutes. 0 = never pause (fully autonomous). -1 = wait indefinitely.

## API Contracts

### GET /api/features/{id}/questions

**Response 200**:
```json
[
  {
    "id": "Q-001",
    "feature_id": "003-human-interaction-points",
    "phase": "inception",
    "role": "pm",
    "question": "What is the target audience for this feature?",
    "type": "clarification",
    "options": ["Internal developers", "External users", "Both"],
    "answer": null,
    "assumption": null,
    "status": "pending",
    "created_at": "2026-06-20T15:30:00Z",
    "answered_at": null
  }
]
```

**Empty state**: Returns `[]` (not `null`, not 404).

**Response 404**: `{"error": "not_found", "details": "Feature abc not found"}`

### POST /api/features/{id}/questions

**Request**:
```json
{
  "phase": "inception",
  "role": "pm",
  "question": "What is the target audience?",
  "type": "clarification",
  "options": ["Internal developers", "External users"]
}
```

**Response 201**:
```json
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
```

**Response 400**: `{"error": "validation_error", "details": "question is required"}`
**Response 404**: `{"error": "not_found", "details": "Feature abc not found"}`

**Validation rules**:
- `phase`: required, one of ["inception", "planning"]
- `role`: required, one of ["pm", "architect"]
- `question`: required, 1-2000 characters
- `type`: required, one of ["clarification", "decision", "priority"]
- `options`: optional, array of 0-10 strings, each 1-500 characters

### PATCH /api/features/{id}/questions/{questionId}

**Request**:
```json
{
  "answer": "I want option A"
}
```

**Response 200**:
```json
{
  "id": "Q-001",
  "feature_id": "003-human-interaction-points",
  "phase": "inception",
  "role": "pm",
  "question": "What is the target audience?",
  "type": "clarification",
  "options": ["Internal developers", "External users"],
  "answer": "I want option A",
  "assumption": null,
  "status": "answered",
  "created_at": "2026-06-20T15:30:00Z",
  "answered_at": "2026-06-20T15:45:00Z"
}
```

**Response 400**: `{"error": "validation_error", "details": "answer must be 1-5000 characters"}`
**Response 404**: `{"error": "not_found", "details": "Question Q-999 not found"}`
**Response 409**: `{"error": "conflict", "details": "Question Q-001 is already answered"}`

### GET /api/features/{id}/questions/pending

**Response 200**: Same shape as GET /api/features/{id}/questions, but filtered to only questions with `status: "pending"`. Returns `[]` when no pending questions.

**Response 404**: `{"error": "not_found", "details": "Feature abc not found"}`

### Feature Summary Response Extension

Add `pending_questions_count` to `FeatureSummaryResponse`:

```json
{
  "id": "003-human-interaction-points",
  "title": "Human Interaction Points",
  "status": "waiting_for_human",
  "priority": 1,
  "current_phase": "inception",
  "updated_at": "2026-06-20T15:30:00Z",
  "pending_questions_count": 3
}
```

When `pending_questions_count` is 0 or the feature has no questions, the field is still present with value 0.

## Component Design

### 1. Question Model & Store (`internal/feature/question.go`)

**Purpose**: Define Question struct and QuestionStore interface for CRUD operations.

**Responsibilities**:
- Question struct with JSON/YAML serialization
- QuestionStore interface: CreateQuestion, GetQuestion, ListQuestions, ListPendingQuestions, AnswerQuestion, AssumeQuestion, DeleteQuestionsForFeature
- FileQuestionStore implementation using `specs/{id}/questions.json`
- Question ID generation (Q-001, Q-002, etc.)
- Question validation

**Interfaces**:
```go
type QuestionStore interface {
    CreateQuestion(ctx context.Context, featureID string, q Question) (*Question, error)
    GetQuestion(ctx context.Context, featureID string, questionID string) (*Question, error)
    ListQuestions(ctx context.Context, featureID string) ([]*Question, error)
    ListPendingQuestions(ctx context.Context, featureID string) ([]*Question, error)
    AnswerQuestion(ctx context.Context, featureID string, questionID string, answer string) (*Question, error)
    AssumeQuestion(ctx context.Context, featureID string, questionID string, assumption string) (*Question, error)
    DeleteQuestionsForFeature(ctx context.Context, featureID string) error
    PendingCount(ctx context.Context, featureID string) (int, error)
}
```

**Dependencies**: `internal/spec` (for file paths), `os`, `encoding/json`

### 2. API Handlers (`internal/api/server.go`)

**Purpose**: Add 4 new HTTP handlers for question CRUD.

**Responsibilities**:
- `handleListQuestions` — GET /api/features/{id}/questions
- `handleCreateQuestion` — POST /api/features/{id}/questions
- `handleAnswerQuestion` — PATCH /api/features/{id}/questions/{questionId}
- `handleListPendingQuestions` — GET /api/features/{id}/questions/pending
- Route registration in `NewServer`
- Input validation at the boundary
- Feature existence check before question operations

**Dependencies**: QuestionStore, existing Pipeline/SpecProvider

### 3. Question Detection (`internal/pipeline/question.go`)

**Purpose**: Detect questions.json artifact after agent dispatch and handle timeout logic.

**Responsibilities**:
- `DetectQuestions(ctx, featureID, specDir) ([]Question, error)` — reads and validates `questions.json`
- `HandleTimeout(ctx, featureID, questionStore, timeout) error` — marks pending questions as assumed
- `ShouldPauseForHuman(feature) bool` — checks if feature is in inception/planning and timeout != 0
- Validate each question: required fields, valid phase, valid role, valid type
- Log warnings for invalid questions, skip them

**Dependencies**: `internal/feature` (Question type), `internal/spec` (SpecProvider)

### 4. Pipeline Integration (`internal/pipeline/pipeline.go`, `internal/pipeline/process.go`)

**Purpose**: Integrate question detection and timeout handling into the pipeline flow.

**Responsibilities**:
- After `RunPhaseWithAgent` for inception/planning, BEFORE gate evaluation, call `DetectQuestions`
- If questions detected and `ShouldPauseForHuman(feature, timeoutMinutes)` returns true: store questions via QuestionStore, set feature status to `waiting_for_human`, save state, broadcast SSE event, start timeout goroutine
- If questions detected and timeout == 0: store questions, immediately call `HandleTimeout` to assume all questions, proceed with normal flow (no pause)
- If no questions detected: proceed with normal gate evaluation (no change to existing flow)
- Timeout goroutine: after configurable timeout, call `HandleTimeout`, set feature status back to `in_progress`, broadcast SSE event, re-dispatch agent with human responses
- When all questions are answered (detected via API endpoint PATCH), resume the pipeline by setting feature status to `in_progress`, building human responses context, and re-dispatching
- On recirculation, call `QuestionStore.DeleteQuestionsForFeature` before proceeding
- Add `waiting_for_human` SSE event broadcasting via `broadcastSSE`
- Add `questionStore` field to Pipeline struct, initialized in `NewPipeline`
- The `ProcessAsync` loop needs to check for `waiting_for_human` status at the start of each iteration and skip the phase dispatch loop if the feature is waiting for human input

**Integration point in ProcessAsync** (`process.go`):
The current loop in `ProcessAsync` is: `for { RunPhaseWithAgent → EvaluateGate → AdvanceOrRecirculate }`. The question detection must happen AFTER `RunPhaseWithAgent` returns and BEFORE `EvaluateGate` is called, and only for inception/planning phases. If questions are detected, the loop should break out (or pause) rather than proceeding to gate evaluation.

**Dependencies**: QuestionStore, config (timeout), existing Pipeline methods

### 5. Context Injection (`internal/pipeline/pipeline.go`)

**Purpose**: Build "Human Responses" section for agent context on re-dispatch.

**Responsibilities**:
- Add a method `BuildHumanResponsesContext(featureID string, questionStore QuestionStore, timeoutMinutes int) (string, error)` to Pipeline (not RuleLoader, since Pipeline has access to QuestionStore)
- After questions are answered or assumed, call this method to build a "Human Responses" section
- The section is appended to the context string AFTER the core context and BEFORE phase-specific instructions (between role instructions and phase instruction in `RunPhaseWithAgent`)
- Format:
  ```
  === Human Responses ===

  Q-001: What is the target audience?
  → Internal developers
  [Source: human input]

  Q-002: Should we use WebSocket or SSE?
  → SSE is sufficient for the MVP
  [Source: auto-assumed after timeout of 30 minutes]
  ```
- If no questions exist (or all are pending with no answers/assumptions yet), return empty string — no section appended
- The injection happens in `RunPhaseWithAgent` when re-dispatching after human interaction

**Why not RuleLoader**: RuleLoader's `BuildContext` doesn't have access to QuestionStore and shouldn't need it. The human responses are feature-specific and only needed during re-dispatch after human interaction, not during every context build. Injecting at the Pipeline level keeps the separation clean.

### 6. Config Extension (`internal/config/config.go`)

**Purpose**: Add `human_interaction_timeout_minutes` to config.

**Responsibilities**:
- Add `HumanInteractionTimeoutMinutes *int` to `PipelineConfig` (pointer type to distinguish zero from missing)
- YAML: `pipeline.human_interaction_timeout_minutes`
- Default to 30 if field is nil (not set in config)
- Value of 0 means "never pause, immediately assume" (fully autonomous)
- Value of -1 means "wait indefinitely" (no timeout)
- Positive values mean "wait that many minutes, then auto-assume"

**Important Go YAML unmarshaling detail**: Go's `yaml.v3` unmarshals a missing integer field as `0` by default. Using `*int` (pointer) allows distinguishing between "field not present" (nil → use default 30) and "field explicitly set to 0" (pointer to 0 → fully autonomous mode).

### 7. Frontend API Client (`ui/src/api/client.ts`)

**Purpose**: Add TypeScript API functions for question endpoints.

**New functions**:
- `listQuestions(featureId: string): Promise<Question[]>`
- `createQuestion(featureId: string, req: CreateQuestionRequest): Promise<Question>`
- `answerQuestion(featureId: string, questionId: string, answer: string): Promise<Question>`
- `listPendingQuestions(featureId: string): Promise<Question[]>`

### 8. Frontend Types (`ui/src/types/index.ts`)

**Purpose**: Add Question types and extend existing types.

**New types**:
```typescript
interface Question {
  id: string;
  feature_id: string;
  phase: 'inception' | 'planning';
  role: 'pm' | 'architect';
  question: string;
  type: 'clarification' | 'decision' | 'priority';
  options: string[];
  answer: string | null;
  assumption: string | null;
  status: 'pending' | 'answered' | 'assumed';
  created_at: string;
  answered_at: string | null;
}

interface CreateQuestionRequest {
  phase: string;
  role: string;
  question: string;
  type: string;
  options?: string[];
}

interface AnswerQuestionRequest {
  answer: string;
}
```

**Modifications**:
- Add `waiting_for_human` to `STATUS_LABELS`
- Add `pending_questions_count` to `FeatureSummary`
- Add `waiting_for_human` to `FeatureSummary.status` color map

### 9. QuestionCard Component (`ui/src/components/QuestionCard.tsx`)

**Purpose**: Display a single question with answer input and status indicators.

**Behavior**:
- Shows question text, type badge (color-coded), and phase/role labels
- If `options` exist, shows clickable buttons that populate the answer input
- If `status === "pending"`, shows text input and submit button
- If `status === "answered"`, shows answer in read-only state with green checkmark
- If `status === "assumed"`, shows assumption in read-only state with "auto-assumed" label
- Submitting an answer calls `answerQuestion` API and refreshes

### 10. QuestionBadge Component (`ui/src/components/QuestionBadge.tsx`)

**Purpose**: Badge overlay on FeatureCard showing pending question count.

**Behavior**:
- Shows count of pending questions (e.g., "3")
- Yellow/orange background to indicate "needs attention"
- Only visible when `pending_questions_count > 0`
- Links to feature detail page

### 11. FeatureDetail Modification (`ui/src/pages/FeatureDetail.tsx`)

**Purpose**: Add question section to feature detail page.

**Behavior**:
- When `feature.status === "waiting_for_human"` or questions exist, show a "Questions" section
- Lists all questions as QuestionCard components
- If all questions are answered, shows a "Pipeline will resume automatically" message
- Polls or uses SSE to detect question answer status changes

### 12. FeatureCard Modification (`ui/src/components/FeatureCard.tsx`)

**Purpose**: Add QuestionBadge to feature card.

**Behavior**:
- Shows QuestionBadge in top-right corner when `pending_questions_count > 0`

## Test Strategy

### Component: Question Model & Store

```
Testing levels required:
  - Unit: Question validation, ID generation, state transitions (pending → answered, pending → assumed), concurrent answer handling
  - Integration: File-based store CRUD, empty state returns []

Quality checkpoints:
  - [ ] Question ID is auto-generated as Q-NNN format
  - [ ] Answering an already-answered question returns an error
  - [ ] Assuming a pending question sets status and assumption field
  - [ ] ListQuestions returns [] not nil for empty feature
  - [ ] ListPendingQuestions filters correctly
  - [ ] DeleteQuestionsForFeature removes all questions
  - [ ] PendingCount returns correct count
```

### Component: API Endpoints

```
Testing levels required:
  - Smoke: Service starts, question endpoints respond with expected status codes
  - Integration: Full request/response cycles for all CRUD operations

Quality checkpoints:
  - [ ] GET /api/features/{id}/questions returns 200 with [] for feature with no questions
  - [ ] GET /api/features/{id}/questions returns 404 for nonexistent feature
  - [ ] POST /api/features/{id}/questions returns 201 for valid question
  - [ ] POST /api/features/{id}/questions returns 400 for missing required fields
  - [ ] POST /api/features/{id}/questions returns 400 for invalid phase/role/type
  - [ ] PATCH /api/features/{id}/questions/{qid} returns 200 for valid answer
  - [ ] PATCH /api/features/{id}/questions/{qid} returns 409 for already-answered question
  - [ ] PATCH /api/features/{id}/questions/{qid} returns 404 for nonexistent question
  - [ ] PATCH /api/features/{id}/questions/{qid} returns 400 for empty answer
  - [ ] GET /api/features/{id}/questions/pending returns only pending questions
  - [ ] All JSON arrays are [] not null for empty collections
  - [ ] Error responses follow {"error": "code", "details": "message"} format
```

### Component: Pipeline Question Detection

```
Testing levels required:
  - Unit: Question validation, invalid JSON handling, invalid phase rejection
  - Integration: Full pipeline flow with question detection

Quality checkpoints:
  - [ ] Valid questions.json is parsed and questions are stored
  - [ ] Invalid JSON in questions.json is skipped with warning
  - [ ] Questions with invalid phase (e.g., "construction") are skipped with warning
  - [ ] Questions with missing required fields are skipped with warning
  - [ ] Feature status transitions to waiting_for_human after detection
  - [ ] Feature status stays in_progress when no questions.json exists
  - [ ] Timeout=0 causes immediate assumption without pausing
  - [ ] Timeout=-1 causes indefinite wait
```

### Component: Frontend Question Components

```
Testing levels required:
  - E2E: Question cards render correctly, answer submission works, badge updates

Quality checkpoints:
  - [ ] QuestionCard renders question text, type badge, and options
  - [ ] QuestionCard shows answer input when status is "pending"
  - [ ] QuestionCard shows read-only answer when status is "answered"
  - [ ] QuestionBadge shows pending count on FeatureCard
  - [ ] QuestionBadge is hidden when pending_questions_count is 0
  - [ ] Answering a question via UI updates the card and badge count
  - [ ] Feature detail page shows question section for features in waiting_for_human status
```

### Component: Context Injection

```
Testing levels required:
  - Integration: CONTEXT.md includes Human Responses section on re-dispatch

Quality checkpoints:
  - [ ] Re-dispatched agent CONTEXT.md contains "Human Responses" section
  - [ ] Answered questions show "[Source: human input]" label
  - [ ] Assumed questions show "[Source: auto-assumed after timeout]" label
  - [ ] Features with no questions don't include Human Responses section
```

### Component: Feature State Machine Extension

```
Testing levels required:
  - Unit: All valid and invalid transitions for waiting_for_human

Quality checkpoints:
  - [ ] in_progress (inception) → waiting_for_human is valid
  - [ ] in_progress (planning) → waiting_for_human is valid
  - [ ] in_progress (construction+) → waiting_for_human is invalid
  - [ ] waiting_for_human → in_progress is valid (when questions answered)
  - [ ] waiting_for_human → in_progress is valid (when timeout expires)
  - [ ] waiting_for_human → cancelled is valid
  - [ ] waiting_for_human → waiting_for_human is invalid
  - [ ] draft → waiting_for_human is invalid
  - [ ] Advance from waiting_for_human returns 400 error
```

## NFR Considerations

### Performance
- Questions stored as a single JSON file per feature — no scalability concerns for MVP (features typically have 0-20 questions)
- Question listing reads from disk — acceptable for single-user local tool
- No pagination needed for MVP (questions per feature are bounded by agent output)

### Security
- Input validation at API boundary for all question fields
- XSS prevention: question text stored as-is but rendered as text (not HTML) in UI
- No authentication for MVP (single-user local tool per spec assumptions)
- Rate limiting not required for MVP (internal tool)

### Resilience
- If questions.json is invalid JSON, skip with warning (don't crash pipeline)
- If timeout goroutine crashes, feature stays in `waiting_for_human` (safe failure mode)
- On server restart, recalculate remaining timeout from `created_at` timestamps
- Question store operations are file-based and atomic (write temp + rename)

### Maintainability
- QuestionStore interface allows swapping file storage for database later
- SSE events for question status changes allow real-time UI updates
- Config-driven timeout allows easy tuning without code changes

## Quickstart Guide for the Developer

1. **Read the spec and acceptance criteria** in `specs/human-interaction-points---allow-the-pipeline-to-pause-for-h/`
2. **Start with the data model** — add Question type and QuestionStore to `internal/feature/question.go`
3. **Then add the status constant** — add `StatusWaitingHuman` to `internal/feature/types.go`
4. **Add state transitions** — update `internal/feature/state.go` with transition rules
5. **Add API endpoints** — create handlers in `internal/api/server.go` and DTOs in `internal/api/dto.go`
6. **Add question detection** — create `internal/pipeline/question.go` for detection and timeout
7. **Integrate into pipeline** — modify `internal/pipeline/pipeline.go` and `internal/pipeline/process.go`
8. **Add context injection** — modify `internal/rules/loader.go`
9. **Add config** — modify `internal/config/config.go`
10. **Build frontend components** — QuestionCard, QuestionBadge, update FeatureCard and FeatureDetail
11. **Write tests** — unit tests for state machine and store, integration tests for API, E2E for UI

**Critical gotchas**:
- JSON arrays must be `[]` not `null` for empty collections — use explicit initialization, not `omitempty`
- `waiting_for_human` can only transition from `in_progress` in inception or planning phases
- Question IDs are sequential within a feature (Q-001, Q-002), not UUIDs
- The timeout resets when a new question is added while in `waiting_for_human` status
- When `timeout_minutes` is 0, don't pause at all — immediately assume all questions
- The `Advance` endpoint must reject features in `waiting_for_human` status with 400 error