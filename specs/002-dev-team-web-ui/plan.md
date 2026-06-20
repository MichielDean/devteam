# Implementation Plan: Dev Team Web UI

**Branch**: `002-dev-team-web-ui` | **Date**: 2026-06-20 | **Spec**: [spec.md](../specs/002-dev-team-web-ui/spec.md)

**Input**: Feature specification from `specs/002-dev-team-web-ui/spec.md`

## Summary

Add a web UI to the existing Dev Team CLI binary, exposing a REST API under `/api/` and serving an embedded React SPA from `/`. The backend reuses all existing `internal/` packages (pipeline, feature, spec, role, intake, config) — no new domain logic, only an HTTP layer. The frontend is a TypeScript + React 19 SPA with Vite, Tailwind CSS v4, React Router, React Query, and SSE for real-time updates. The Go binary gains a `-http` flag; without it, behavior is unchanged.

## Technical Context

**Language/Version**: Go 1.23+ (backend), TypeScript 5+ with React 19 (frontend)

**Primary Dependencies**:
- Backend: Go standard library `net/http`, `encoding/json`, `embed`, existing `internal/` packages (pipeline, feature, spec, role, intake, config, repo, rules)
- Frontend: Vite 6+, React 19, React Router 7, React Query (TanStack Query v5), Tailwind CSS v4, react-markdown, rehype-highlight

**Storage**: `.devteam-state.yaml` files on disk (same as CLI). No database. Single source of truth.

**Testing**: Go standard `testing` + `net/http/httptest` for API handlers. Vitest + React Testing Library for frontend. Manual integration testing via browser.

**Target Platform**: Linux/macOS (Go binary). Modern browsers (Chrome, Firefox, Safari, Edge latest).

**Project Type**: Web application (Go backend + React SPA frontend)

**Performance Goals**: API responses <500ms for 100 features. SSE events within 5s of state change. Frontend bundle <500KB gzipped. Server startup <2s.

**Constraints**: Single Go binary with embedded frontend. No external database. No auth (local-only, single-user). SSE (not WebSocket). Must work alongside CLI (same state files).

**Scale/Scope**: 1 repo (devteam). ~10 API endpoints. ~10 frontend components. ~6 pages/views.

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Spec-Driven, Always | PASS | Feature starts from spec.md + acceptance.md |
| II. Six Roles, Fixed Pipeline | PASS | Web UI exposes the same 6-phase pipeline, no new phases |
| III. Central Spec, Distributed Implementation | PASS | Web UI reads/writes same `.devteam-state.yaml` as CLI |
| IV. Two Intake Paths, One Output | PASS | UI supports both loose_idea and external_spec |
| V. Proof-of-Work Gates | PASS | Gate evaluation exposed via API, UI shows results |
| VI. Cross-Repo Coherence | PASS | UI displays repos.yaml; no multi-repo changes needed |
| VII. Self-Bootstrap | PASS | Feature 002 is the platform's own web UI |
| VIII. Go, Minimal Dependencies | PASS | Backend uses only stdlib; frontend bundled into binary |
| IX. AIDLC Phase Governance | PASS | Same rules, same gates, same orchestrator |
| X. Learn From Cistern | PASS | Structured context, real-time progress, mechanical gates |

## Data Model

### Existing Entities (No Schema Changes)

The web UI reads and writes the same `Feature`, `PhaseState`, `Artifact`, `GateResult`, and `RepoRef` types already defined in `internal/feature/`. No database schema changes are needed.

### New API DTOs (internal/api/dto.go)

```
// Request DTOs

CreateFeatureRequest
├── Type          string    // "loose_idea" or "external_spec"
├── Title         string    // Required, max 200 chars
├── Description   string    // Required for loose_idea, max 10000 chars
├── Priority      int       // 1, 2, or 3 (default 2)
└── FileContent   string    // base64-encoded file content for external_spec

RecirculateRequest
└── TargetPhase   string    // Must be a valid phase earlier than current

// Response DTOs

FeatureListResponse
└── Features      []FeatureSummary

FeatureSummary
├── ID            string
├── Title         string
├── Status        string
├── Priority      int
├── CurrentPhase  string
├── UpdatedAt     time.Time
└── GateResult    *GateResultResponse (nullable)

FeatureDetailResponse
├── ID            string
├── Title         string
├── Status        string
├── Priority      int
├── IntakePath    string
├── CreatedAt     time.Time
├── UpdatedAt     time.Time
├── PhaseStates   map[string]PhaseStateResponse
├── Dependencies  []string
└── Repos         []RepoRefResponse

PhaseStateResponse
├── Phase         string
├── Status        string
├── StartedAt     *time.Time
├── CompletedAt   *time.Time
├── Artifacts     []ArtifactResponse
└── GateResult    *GateResultResponse (nullable)

ArtifactResponse
├── Type          string
├── Path          string
├── GeneratedBy   string
└── GeneratedAt   time.Time

GateResultResponse
├── Phase         string
├── Passed        bool
└── Checks        []CheckResultResponse

CheckResultResponse
├── Name          string
├── Passed        bool
└── Message       string

RepoRefResponse
├── Name          string
├── URL           string
└── Branch         string

// SSE Events

SSEEvent
├── Type          string    // "phase_change", "gate_result", "agent_dispatch", "agent_complete", "processing_complete", "error"
└── Data          json.RawMessage

PhaseChangeEvent
├── FeatureID     string
├── Phase         string
├── Status        string
└── Timestamp     time.Time

GateResultEvent
├── FeatureID     string
├── Phase         string
├── Passed        bool
└── Checks        []CheckResultResponse

AgentDispatchEvent
├── FeatureID     string
├── Phase         string
├── Role          string
├── Status        string
└── Timestamp     time.Time

AgentCompleteEvent
├── FeatureID     string
├── Phase         string
├── Role          string
├── Status        string
└── DurationMs    int64

ProcessingCompleteEvent
├── FeatureID     string
├── Status        string
└── Timestamp     time.Time

ErrorEvent
├── FeatureID     string
├── Message       string
└── Timestamp     time.Time

// Error Response

ErrorResponse
├── Error         string
└── Details       string (optional)
```

### Artifact Type to API Path Mapping

| ArtifactType | API `:type` parameter | File on disk |
|---|---|---|
| `input_md` | `input` | `specs/<id>/input.md` |
| `spec_md` | `spec` | `specs/<id>/spec.md` |
| `acceptance_md` | `acceptance` | `specs/<id>/acceptance.md` |
| `repos_yaml` | `repos` | `specs/<id>/repos.yaml` |
| `plan_md` | `plan` | `specs/<id>/plan.md` |
| `tasks_md` | `tasks` | `specs/<id>/tasks.md` |
| `review_report` | `review_report` | `specs/<id>/review-report.md` |
| `test_report` | `test_report` | `specs/<id>/test-report.md` |
| `docs` | `docs` | `specs/<id>/docs` (directory) |

## Project Structure

### Documentation (this feature)

```text
specs/002-dev-team-web-ui/
├── spec.md              # Feature specification
├── acceptance.md        # Acceptance criteria
├── repos.yaml           # Repository scope
├── plan.md              # This file
├── tasks.md             # Task breakdown
└── quickstart.md        # Getting started guide
```

### Source Code (repository root)

```text
cmd/
└── devteam/
    └── main.go                  # MODIFIED — add -http flag, wire up server

internal/
├── api/                         # NEW — HTTP API layer
│   ├── server.go                # Server setup, ServeMux routing, embed.FS serving
│   ├── handler.go               # Feature CRUD: list, get, create
│   ├── handler_artifact.go      # Artifact serving: get by type
│   ├── handler_pipeline.go      # Pipeline actions: run, advance, recirculate, cancel, process, gate
│   ├── handler_sse.go           # SSE stream handler with fsnotify file watching
│   ├── middleware.go             # CORS, logging, recovery, request-id middleware
│   ├── dto.go                   # Request/response types, conversion helpers
│   ├── server_test.go           # Integration tests for routing and middleware
│   ├── handler_test.go          # Handler unit tests
│   ├── handler_artifact_test.go  # Artifact handler tests
│   ├── handler_pipeline_test.go  # Pipeline handler tests
│   ├── handler_sse_test.go      # SSE handler tests
│   └── dto_test.go              # DTO conversion tests
├── config/                      # EXISTING — no changes expected
├── feature/                     # EXISTING — may add JSON tags or helper methods
│   ├── feature.go               # MODIFIED — add JSON tags to exported types, add IsTerminal() helper
│   ├── types.go                 # MODIFIED — add String() methods on Phase/Status/ArtifactType, add ValidPhases for API validation
│   ├── state.go                 # EXISTING — no changes expected
│   └── ...
├── intake/                      # EXISTING — no changes expected (already programmatic)
├── pipeline/                    # EXISTING — add ProcessAsync for goroutine-based processing
│   ├── pipeline.go              # MODIFIED — add ProcessAsync() method for SSE-streamed processing
│   └── ...
├── repo/                        # EXISTING — no changes expected
├── role/                        # EXISTING — no changes expected
├── rules/                       # EXISTING — no changes expected
└── spec/                        # EXISTING — may add helper methods
    ├── provider.go              # MODIFIED — add ListFeaturesSorted(), ReadArtifactContent() helpers
    └── ...

ui/                              # NEW — Frontend SPA
├── package.json
├── vite.config.ts
├── tsconfig.json
├── tailwind.config.ts
├── postcss.config.js
├── index.html
└── src/
    ├── main.tsx                 # Entry point, React root, providers
    ├── App.tsx                  # Router setup, layout, theme context
    ├── api/
    │   └── client.ts            # API client: fetch wrappers for all endpoints
    ├── hooks/
    │   ├── useFeatures.ts       # React Query hook for feature list
    │   ├── useFeature.ts         # React Query hook for feature detail
    │   └── useSSE.ts             # SSE connection hook with reconnect
    ├── pages/
    │   ├── Dashboard.tsx         # Feature list with sort/filter
    │   └── FeatureDetail.tsx     # Single feature view with tabs
    ├── components/
    │   ├── FeatureCard.tsx       # Card for feature list view
    │   ├── FeatureList.tsx       # List/table with sorting
    │   ├── IntakeForm.tsx        # Create feature form with validation
    │   ├── ArtifactViewer.tsx    # Markdown renderer with syntax highlighting
    │   ├── ProcessView.tsx       # Real-time processing progress display
    │   ├── GateResult.tsx        # Gate checks display (pass/fail per check)
    │   ├── PhaseTimeline.tsx     # Visual pipeline phase indicator
    │   ├── Toast.tsx             # Toast notification system
    │   ├── ThemeToggle.tsx       # Dark/light mode toggle
    │   ├── ConnectionStatus.tsx  # SSE connection status banner
    │   └── EmptyState.tsx        # Empty list placeholder with CTA
    └── types/
        └── index.ts              # TypeScript interfaces matching API responses

go.mod                           # MODIFIED — no new Go deps (stdlib only)
```

**Structure Decision**: Web application structure — `internal/api/` for backend HTTP handlers, `ui/` for frontend SPA. The Go binary serves both via `embed.FS` for static assets and `http.ServeMux` for API routing.

## API Contracts

### REST Endpoints

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/features` | `handler.ListFeatures` | List all features with phase/status summary |
| POST | `/api/features` | `handler.CreateFeature` | Create feature (loose idea or external spec) |
| GET | `/api/features/:id` | `handler.GetFeature` | Get feature detail with full phase states |
| POST | `/api/features/:id/run` | `handler_pipeline.RunPhase` | Run current phase (dispatch agents) |
| POST | `/api/features/:id/advance` | `handler_pipeline.AdvanceFeature` | Advance to next phase |
| POST | `/api/features/:id/recirculate` | `handler_pipeline.RecirculateFeature` | Recirculate to earlier phase |
| POST | `/api/features/:id/cancel` | `handler_pipeline.CancelFeature` | Cancel feature |
| POST | `/api/features/:id/process` | `handler_pipeline.ProcessFeature` | Process entire pipeline autonomously |
| GET | `/api/features/:id/artifacts/:type` | `handler_artifact.GetArtifact` | Get artifact content as text/markdown |
| GET | `/api/features/:id/gate` | `handler_pipeline.EvaluateGate` | Evaluate current gate |
| GET | `/api/features/:id/stream` | `handler_sse.StreamFeature` | SSE stream for processing progress |
| GET | `/` | `server.ServeSPA` | Serve embedded SPA (catch-all) |

### Create Feature — `POST /api/features`

**Request** (loose idea):
```json
{
  "type": "loose_idea",
  "title": "We need dark mode",
  "description": "Add dark mode support to the dashboard for better UX in low-light environments",
  "priority": 1
}
```

**Request** (external spec):
```json
{
  "type": "external_spec",
  "title": "External PRD",
  "description": "PRD from product team",
  "priority": 2,
  "file_content": "base64-encoded-file-content"
}
```

**Response**: `201 Created` with `FeatureDetailResponse` body

**Validation rules**:
- `title`: required, max 200 chars
- `description`: required for loose_idea, max 10000 chars
- `priority`: optional, defaults to 2, must be 1-3
- `file_content`: required for external_spec, base64-encoded
- Duplicate title warning: if title matches existing feature (case-insensitive), return `409 Conflict` with `{ "error": "duplicate_title", "details": "..." }` — client can choose to proceed

### Feature List — `GET /api/features`

**Response**: `200 OK` with `FeatureListResponse`

### Feature Detail — `GET /api/features/:id`

**Response**: `200 OK` with `FeatureDetailResponse`
**Error**: `404 Not Found` if feature doesn't exist

### Run Phase — `POST /api/features/:id/run`

**Response**: `200 OK` with `FeatureDetailResponse` (updated with phase run results)
**Error**: `409 Conflict` if feature is already being processed

### Advance — `POST /api/features/:id/advance`

**Response**: `200 OK` with `FeatureDetailResponse`
**Error**: `400 Bad Request` if gate hasn't passed, feature is at delivery, or feature is terminal

### Recirculate — `POST /api/features/:id/recirculate`

**Request**: `{ "target_phase": "planning" }`
**Response**: `200 OK` with `FeatureDetailResponse`
**Error**: `400 Bad Request` if target phase is invalid, not earlier than current, or feature is terminal

### Cancel — `POST /api/features/:id/cancel`

**Response**: `200 OK` with `FeatureDetailResponse` (status: "cancelled")
**Error**: `400 Bad Request` if feature is already cancelled or done

### Process — `POST /api/features/:id/process`

**Response**: `200 OK` with `FeatureDetailResponse`. Processing runs in a goroutine; progress streamed via SSE.
**Error**: `409 Conflict` if feature is already being processed

### Evaluate Gate — `GET /api/features/:id/gate`

**Response**: `200 OK` with `GateResultResponse`

### Get Artifact — `GET /api/features/:id/artifacts/:type`

**Response**: `200 OK` with content as `text/plain; charset=utf-8`
**Error**: `404 Not Found` if artifact hasn't been generated yet
**Supported types**: `input`, `spec`, `acceptance`, `repos`, `plan`, `tasks`, `review_report`, `test_report`, `docs`

### SSE Stream — `GET /api/features/:id/stream`

**Response**: `text/event-stream` with events:
- `phase_change`: Feature moved to a new phase
- `gate_result`: Gate evaluation completed
- `agent_dispatch`: Agent dispatched for a role
- `agent_complete`: Agent finished execution
- `processing_complete`: Autonomous processing finished
- `error`: An error occurred during processing

Each event is a JSON object with a `type` field. Connection stays open until processing completes or client disconnects. Multiple concurrent clients for the same feature are supported.

### Error Response Format

```json
{
  "error": "error_code",
  "details": "Human-readable message"
}
```

**Error Codes**:
- `400` — `validation_error`, `invalid_phase`, `invalid_priority`, `empty_title`, `empty_description`, `title_too_long`, `description_too_long`
- `404` — `feature_not_found`, `artifact_not_found`
- `409` — `duplicate_title`, `already_processing`
- `500` — `internal_error`

### Security Headers

All API responses include:
- `Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'` (relaxed for SPA inline styles)
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: strict-origin-when-cross-origin`

No `Strict-Transport-Security` header since the server is local-only (no TLS by default).

## SSE Architecture

### Event Flow

```
POST /api/features/:id/process
         │
         ▼
  Goroutine started
  ┌─────────────────────────────────────────┐
  │  Pipeline.ProcessAsync(ctx, feature)    │
  │                                         │
  │  for each phase:                        │
  │    → emit agent_dispatch event           │
  │    → run phase via pipeline              │
  │    → emit agent_complete event           │
  │    → evaluate gate                       │
  │    → emit gate_result event              │
  │    → if gate passed: advance             │
  │    → emit phase_change event             │
  │    → if gate failed: recirculate         │
  │    → emit phase_change event             │
  │                                         │
  │  → emit processing_complete event        │
  └─────────────────────────────────────────┘
         │
         ▼
  SSE clients receive events
  via registered channels
```

### Implementation

- **File watching**: Use `fsnotify` to watch `.devteam-state.yaml` for changes. When the file changes, parse the new state and broadcast events to all registered SSE clients.
- **Channel registry**: A map of feature IDs to slices of `chan SSEEvent`. When a client connects, register a channel; when processing emits an event, broadcast to all channels for that feature.
- **Reconnection**: Clients auto-reconnect via `EventSource` API. Server sends periodic keep-alive comments (every 30s) to prevent proxy timeouts.
- **Cleanup**: When a client disconnects, remove the channel from the registry. Use `context.Context` cancellation for goroutine cleanup.

### ProcessAsync Method

```go
// internal/pipeline/pipeline.go — new method
func (p *Pipeline) ProcessAsync(ctx context.Context, f *feature.Feature, eventCh chan<- SSEEvent) error
```

This method runs the autonomous processing loop in a goroutine:
1. Set feature status to `in_progress` if not already
2. Loop through phases until delivery or max recirculations
3. For each phase: emit `agent_dispatch` → run phase → emit `agent_complete` → evaluate gate → emit `gate_result`
4. On gate pass: advance → emit `phase_change`
5. On gate fail: recirculate → emit `phase_change`
6. On completion: emit `processing_complete`
7. On error: emit `error` event

## Frontend Architecture

### Component Tree

```
App.tsx
├── ThemeProvider (context)
├── ConnectionStatus (banner)
├── ToastProvider (context)
├── Routes
│   ├── "/" → Dashboard
│   │   ├── FeatureList
│   │   │   └── FeatureCard (×N)
│   │   ├── IntakeForm (modal/panel)
│   │   └── EmptyState (when no features)
│   └── "/features/:id" → FeatureDetail
│       ├── PhaseTimeline
│       ├── ArtifactViewer (tab)
│       ├── GateResult (tab)
│       └── ProcessView (when processing)
└── ThemeToggle (header)
```

### State Management

- **Server state**: React Query (TanStack Query v5) for all API data
  - `useFeatures()` — fetches and caches feature list
  - `useFeature(id)` — fetches and caches feature detail
  - Mutations for create, advance, recirculate, cancel, run, process
- **Real-time state**: SSE via `useSSE()` hook
  - Invalidates React Query cache on events
  - Shows connection status banner on disconnect
- **UI state**: React context
  - `ThemeContext` — dark/light mode, persisted in localStorage
  - `ToastContext` — success/error notifications

### Routing

- `/` — Dashboard (feature list)
- `/features/:id` — Feature detail

Client-side routing via React Router v7. No server-side routing needed — the Go server serves the SPA for all non-`/api/` routes.

### Dark Mode

Tailwind CSS v4 dark mode via `prefers-color-scheme` media query + manual toggle persisted in `localStorage`. The `ThemeProvider` reads the stored preference on mount, falls back to system preference.

## Key Design Decisions

### 1. embed.FS for Frontend Assets

The Go binary embeds the built frontend via `//go:embed ui/dist/*`. This means:
- `go generate` runs `npm run build` in `ui/` before compilation
- The binary is self-contained — no external file serving needed
- The SPA is served at `/` with fallback to `index.html` for client-side routes
- API routes at `/api/` take precedence over the SPA catch-all

### 2. SSE Over WebSocket

SSE is simpler for server-to-client push and is sufficient for pipeline progress events:
- Unidirectional (server → client) — matches our use case
- Auto-reconnect built into `EventSource` API
- No need for WebSocket library on either side
- Works with HTTP/2 and proxies

### 3. File Watching for State Changes

Instead of instrumenting the pipeline code to emit events, the API server watches `.devteam-state.yaml` files for changes:
- Decouples the API layer from pipeline internals
- Works for CLI-triggered state changes too (since CLI writes to the same files)
- Uses `fsnotify` for cross-platform file change notifications
- Fallback polling every 2s if fsnotify fails

### 4. No Auth (Local-Only Mode)

The server listens on `localhost` by default:
- No authentication middleware
- No session management
- Suitable for single-user local development
- Auth can be added later as a separate feature

### 5. No External Database

The `.devteam-state.yaml` files are the single source of truth, shared with the CLI:
- No migration needed between CLI and UI
- CLI actions are immediately visible in the UI (on next refresh or SSE event)
- Concurrent CLI and UI access is safe (file-based locking or compare-and-swap)

## Quickstart Guide for the Developer

### Prerequisites

- Go 1.23+
- Node.js 20+ and npm
- `opencode` CLI (for agent dispatch)

### Backend Setup

```bash
# Clone the repo and checkout the feature branch
git clone git@github.com:MichielDean/devteam.git
cd devteam
git checkout 002-dev-team-web-ui

# Build and run in CLI mode (unchanged behavior)
go build ./cmd/devteam
./devteam status

# Build and run with web UI
go generate ./cmd/devteam  # builds the frontend
go build ./cmd/devteam
./devteam -http :8080

# Or use go run
go run ./cmd/devteam -http :8080
```

### Frontend Development

```bash
cd ui/
npm install
npm run dev    # starts Vite dev server on :5173 with proxy to :8080

# In another terminal, run the backend
cd ..
go run ./cmd/devteam -http :8080
```

### Testing

```bash
# Backend tests
go test ./internal/api/... -v

# Frontend tests
cd ui/
npm test

# Integration test: create a feature via API, verify in UI
curl -X POST http://localhost:8080/api/features \
  -H 'Content-Type: application/json' \
  -d '{"type":"loose_idea","title":"Test feature","description":"Test description","priority":2}'
```

### Key Files to Start With

1. `internal/api/server.go` — HTTP server, routing, static file serving
2. `internal/api/handler.go` — Feature CRUD handlers (list, get, create)
3. `internal/api/handler_pipeline.go` — Pipeline action handlers
4. `internal/api/handler_sse.go` — SSE streaming handler
5. `internal/api/dto.go` — All request/response types
6. `ui/src/api/client.ts` — Frontend API client
7. `ui/src/hooks/useSSE.ts` — SSE hook with reconnect
8. `ui/src/pages/Dashboard.tsx` — Main dashboard page

## Feasibility Assessment

### Spec Items That Are Well-Defined

- All API endpoints, request/response shapes, and error codes are specified
- Data model is fully defined (reuses existing `Feature` types)
- Frontend component tree and state management approach are clear
- SSE event types and flow are documented
- Project structure is explicit with file paths
- Acceptance criteria are testable and unambiguous

### Items Flagged for Clarification

1. **Concurrent file access**: When both CLI and API write to `.devteam-state.yaml`, there's a race condition. The API should use file locking (e.g., `flock` on Unix) or compare-and-swap to prevent data loss. This needs explicit handling in `handler_pipeline.go`.

2. **Processing goroutine lifecycle**: When `POST /api/features/:id/process` starts a goroutine, how is it tracked? The server needs a registry of active processing goroutines so it can:
   - Return 409 Conflict if processing is already in progress
   - Cancel goroutines on server shutdown
   - Clean up goroutines that outlive their feature

3. **SSE channel cleanup**: When does a feature's SSE channel registry get cleaned up? When all clients disconnect? After processing completes? Both? The spec says "multiple concurrent SSE connections for the same feature" are supported — need a channel-per-feature model with subscriber management.

4. **Artifact type `docs`**: The `docs` artifact type maps to a directory, not a file. The API response for artifact content should handle this — either return a 404, return a listing, or return a zipped response. The spec says "404 if not yet generated" but `docs` is a directory. Decision: return the directory listing as markdown, or return 404 if the directory doesn't exist.

5. **Feature creation does NOT auto-start processing**: The spec is clear on this — creating a feature sets it to `in_progress` in `inception` phase, but the user must explicitly click "Run Phase" or "Process". This means the `POST /api/features` handler should call `intake.Submit()` but NOT call `pipeline.RunPhaseWithAgent()`.

6. **Priority validation range**: The spec says 1-3. The existing `Feature` struct uses `Priority int` without validation. The API handler must enforce the 1-3 range and default to 2.