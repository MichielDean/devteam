# Feature Specification: Dev Team Web UI

**Feature Branch**: `002-dev-team-web-ui`

**Created**: 2026-06-20

**Status**: Draft

**Input**: The Dev Team platform needs a web interface so human team members can submit features, monitor pipeline progress, and review artifacts without using the CLI.

---

## Problem Statement

The Dev Team pipeline currently requires the CLI for all interactions. Team members who want to submit ideas, check on feature progress, or review artifacts must use `devteam` commands in a terminal. This creates friction for non-technical stakeholders and makes the pipeline's value invisible until someone opens a shell. A web UI provides a real-time window into the pipeline and lowers the barrier to participation.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Submit a feature idea from the browser (Priority: P1)

A team member visits the Dev Team dashboard, types a loose idea into a text box, and clicks Submit. The feature appears in the pipeline with status "in_progress" and phase "inception". The intake form also accepts external spec files. The user can set a priority (1, 2, or 3) before submitting.

**Why this priority**: This is the front door. Without intake from the UI, nothing else matters.

**Independent Test**: Submit "We need dark mode" from the web UI and verify it creates a feature in inception phase with `input.md` generated.

**Acceptance Scenarios**:

1. **Given** the dashboard is open, **When** the user types a description and clicks Submit, **Then** a `POST /api/features` request is sent with type `loose_idea` and the feature appears in the list with status "in_progress" and phase "inception"
2. **Given** a submitted idea, **When** the PM agent finishes inception, **Then** spec.md, acceptance.md, and repos.yaml are generated and visible in the UI
3. **Given** a feature in inception, **When** the user navigates to the feature detail page, **Then** they see the input idea, current phase, and all generated artifacts
4. **Given** the intake form, **When** the user submits without entering any text, **Then** the form shows a validation error "Description is required" and no request is sent
5. **Given** the intake form, **When** the user types a title that matches an existing feature's title, **Then** the UI warns about the potential duplicate and offers to proceed or cancel
6. **Given** the intake form, **When** the user selects "External Spec" and uploads a file, **Then** a feature is created with `intake_path: external_spec` and the file content is stored as `input.md`
7. **Given** the intake form, **When** the user selects priority 1, 2, or 3, **Then** the priority is included in the creation request; if no priority is selected, it defaults to 2

---

### User Story 2 — Watch features move through the pipeline in real time (Priority: P1)

A team member opens the dashboard and sees all features with their current phase, status, and gate results. When a phase completes, the feature card updates to show the next phase within 5 seconds.

**Why this priority**: The pipeline IS the product. Real-time visibility is essential for trust and coordination.

**Independent Test**: Start `devteam process` on a feature and verify the dashboard shows phase transitions as they happen.

**Acceptance Scenarios**:

1. **Given** multiple features exist, **When** the user views the dashboard, **Then** all features are listed with ID, title, phase, priority, and status
2. **Given** a feature is being processed, **When** the phase changes, **Then** the dashboard updates within 5 seconds to reflect the new phase
3. **Given** a gate evaluation completes, **When** the user views the feature, **Then** gate results (pass/fail per check) are displayed
4. **Given** the dashboard, **When** the user clicks a sortable column header (phase, priority, status, updated), **Then** the feature list reorders accordingly
5. **Given** the dashboard, **When** the backend connection is lost, **Then** the UI shows a clear "Connection lost" banner and reconnects automatically when available
6. **Given** the dashboard with no features, **When** the user views it, **Then** an empty state is shown with a call-to-action to create the first feature

---

### User Story 3 — Review artifacts from each phase in the browser (Priority: P1)

A team member clicks on a feature and sees all artifacts (spec.md, acceptance.md, plan.md, tasks.md, review report, test report, docs) rendered as formatted markdown with syntax highlighting.

**Why this priority**: Artifacts are the output of the pipeline. If users can't read them easily, the pipeline's value is lost.

**Independent Test**: Navigate to a feature detail page and verify all artifacts are rendered as markdown with proper formatting.

**Acceptance Scenarios**:

1. **Given** a feature with generated artifacts, **When** the user navigates to the feature detail page, **Then** all artifacts are listed with their type and `generated_by` role
2. **Given** an artifact, **When** the user clicks it, **Then** the full content is displayed with syntax highlighting for code blocks
3. **Given** a feature in the review phase, **When** the user views the review report, **Then** each acceptance criterion shows pass/fail with evidence
4. **Given** an artifact type that hasn't been generated yet, **When** the user views the feature detail, **Then** the artifact is listed but shown as "Not yet generated" with a placeholder state
5. **Given** code blocks in an artifact, **When** rendered, **Then** they display with syntax highlighting for Go, YAML, and shell languages

---

### User Story 4 — Manage features from the dashboard (Priority: P2)

A team member can advance a feature through the pipeline, recirculate it back to an earlier phase, or cancel it entirely — all from the UI without touching the CLI. The UI also supports running a single phase and evaluating gates manually.

**Why this priority**: Manual control lets humans intervene when the pipeline makes mistakes or priorities change.

**Independent Test**: Click "Advance" on a feature that has passed its gate and verify it moves to the next phase.

**Acceptance Scenarios**:

1. **Given** a feature with a passed gate, **When** the user clicks "Advance", **Then** a `POST /api/features/:id/advance` is made and the feature moves to the next phase
2. **Given** a feature whose gate has not passed, **When** the user views the feature, **Then** the "Advance" button is disabled with a tooltip explaining "Gate has not passed"
3. **Given** a feature with a failed gate, **When** the user clicks "Recirculate" and selects a target phase, **Then** a `POST /api/features/:id/recirculate` is made with the selected target phase and the feature is sent back
4. **Given** a feature, **When** the user clicks "Cancel", **Then** a confirmation dialog appears; only on confirmation is `POST /api/features/:id/cancel` sent
5. **Given** a feature in any phase, **When** the user clicks "Run Phase", **Then** a `POST /api/features/:id/run` is made to dispatch the agent for the current phase
6. **Given** a feature, **When** the user clicks "Evaluate Gate", **Then** a `GET /api/features/:id/gate` is made and the gate results are displayed
7. **Given** a feature that is already cancelled or done, **When** the user views it, **Then** the "Cancel" and "Advance" buttons are hidden or disabled since the feature is terminal

---

### User Story 5 — Trigger autonomous processing from the UI (Priority: P2)

A team member clicks "Process" on a feature and the UI shows real-time progress as each phase runs — dispatching the agent, waiting for completion, evaluating the gate, and advancing or recirculating.

**Why this priority**: This is the autonomous flow — one click to take a feature from idea to delivery.

**Independent Test**: Click "Process" on a feature in inception and verify it advances through phases automatically until delivery or a gate failure.

**Acceptance Scenarios**:

1. **Given** a feature in any phase, **When** the user clicks "Process", **Then** a `POST /api/features/:id/process` is made and the UI shows a progress view with phase transitions
2. **Given** a processing feature, **When** a gate fails, **Then** the UI shows the recirculation event with the option to retry or cancel
3. **Given** a processing feature that reaches delivery, **When** the final gate passes, **Then** the feature is marked as "done" with a summary of all phases and durations
4. **Given** a processing feature, **When** processing takes more than 30 seconds, **Then** the UI shows the current phase name and elapsed time
5. **Given** a processing feature, **When** SSE events arrive, **Then** each `phase_change`, `gate_result`, `agent_dispatch`, and `agent_complete` event is reflected in the progress view within 5 seconds
6. **Given** a feature already being processed (status `in_progress` and an active SSE stream), **When** the user views the feature, **Then** the "Process" button is disabled with a tooltip explaining why

---

### User Story 6 — Modern, responsive UI that works on mobile (Priority: P2)

The dashboard is a single-page application with a clean, modern design that works on desktop and mobile browsers. Dark mode is supported. Navigation is intuitive — features list, feature detail, and settings.

**Why this priority**: A clunky UI undermines trust. The UI should feel like a polished SaaS product, not an internal tool.

**Independent Test**: Open the dashboard on a phone-sized viewport and verify all core functionality is usable.

**Acceptance Scenarios**:

1. **Given** the dashboard, **When** viewed on a viewport of 375px width, **Then** all core functions (submit, view, advance) are accessible without horizontal scrolling
2. **Given** the dashboard in dark mode, **When** the user toggles the theme, **Then** all text is readable and all controls are functional
3. **Given** the dashboard, **When** the user navigates between features, **Then** page transitions take less than 200ms perceived latency
4. **Given** the dashboard, **When** the user refreshes the page, **Then** the current view is restored via URL-based routing
5. **Given** the dashboard, **When** an action succeeds, **Then** a toast notification confirms the action
6. **Given** the dashboard, **When** an action fails (network error, 409 conflict), **Then** a toast notification shows the error message
7. **Given** the dashboard, **When** data is loading, **Then** a loading spinner or skeleton state is shown instead of blank content

---

## Edge Cases

| # | Edge Case | Expected Behavior |
|---|---|---|
| 1 | Duplicate idea submission | The UI warns about potential duplicates by matching the submitted title against existing feature titles. User can proceed or cancel. |
| 2 | Processing run takes 30+ minutes | The UI shows a progress indicator with the current phase name and elapsed time. The SSE connection keeps the UI updated. If the connection drops, it reconnects automatically. |
| 3 | Backend is down | The UI shows a clear "Connection lost" banner. Pending actions are NOT queued locally — the user is told to retry after the connection is restored. |
| 4 | Feature already being processed | The "Process" button is disabled if a feature's status is `in_progress` and an active SSE stream exists. The UI shows the current processing status. |
| 5 | Empty feature list | The dashboard shows an empty state with a call-to-action to create the first feature. |
| 6 | Large artifact files | Artifacts larger than 5MB are rendered with a "loading" state and may paginate or lazy-load rather than rendering all at once. |
| 7 | Concurrent CLI and UI actions | The `.devteam-state.yaml` file is the single source of truth. The API reads from it on every request, so CLI actions are reflected immediately on the next UI refresh or SSE event. |
| 8 | Invalid feature ID in URL | `GET /api/features/:id` returns 404. The UI shows a "Feature not found" message and offers navigation back to the dashboard. |
| 9 | Cancel on a cancelled or done feature | The API returns 400 with an error message. The UI hides or disables the Cancel button for terminal features. |
| 10 | Recirculate to an invalid or forward phase | The API returns 400 with an error message listing valid phases. The UI only offers backward phases in the recirculate dropdown. |
| 11 | Priority out of range (0, 4, -1) | The API returns 400. The UI restricts the priority selector to values 1, 2, or 3. |
| 12 | Empty title on feature creation | The UI validates that the title is not empty before submission. The API rejects empty titles with 400. |
| 13 | Title exceeds 200 characters | The UI shows a validation error. The API rejects titles over 200 characters with 400. |
| 14 | Advance on a feature at the delivery phase | The API returns 400 (already at final phase). The UI hides the Advance button when the feature is in delivery with a passed gate, and shows "Mark Done" instead. |
| 15 | Multiple SSE connections to the same feature | The backend supports multiple concurrent SSE connections for the same feature. Each client receives the same events. |
| 16 | Special characters in feature titles | Feature titles are displayed as-is (HTML-escaped). The slug-based feature ID generation strips or replaces special characters. |

---

## Requirements *(mandatory)*

### Functional Requirements

**Feature Intake**

- **FR-001**: The UI MUST provide a form to submit loose ideas that creates a feature with `intake_path: loose_idea` via `POST /api/features`
- **FR-002**: The UI MUST provide a file upload path for external specs that creates a feature with `intake_path: external_spec` and passes the uploaded file content (base64-encoded) as `file_content` in the request body
- **FR-003**: The UI MUST validate the intake form: reject empty titles (max 200 characters), reject empty descriptions, enforce a maximum description length of 10,000 characters, and warn about potential duplicates by matching against existing feature titles
- **FR-004**: The UI MUST allow the user to set the feature priority (1, 2, or 3) at intake time, defaulting to 2 (medium); the API MUST reject values outside 1–3
- **FR-005**: Feature creation via the API MUST set the initial phase to "inception" and status to "in_progress"; the feature does NOT automatically run the PM agent — the user must click "Run Phase" or "Process" to start processing

**Dashboard and Feature Listing**

- **FR-006**: The UI MUST display all features with their current phase, status, priority, and gate results on a single dashboard page
- **FR-007**: The UI MUST support sorting the feature list by phase, priority, status, and last-updated date
- **FR-008**: The UI MUST update feature state in real time via Server-Sent Events, reflecting phase transitions within 5 seconds
- **FR-009**: The UI MUST display a clear "Connection lost" indicator when the SSE connection drops and automatically reconnect when the backend is available
- **FR-010**: The UI MUST display an empty state with a call-to-action when no features exist

**Artifact Viewing**

- **FR-011**: The UI MUST render markdown artifacts (spec.md, acceptance.md, plan.md, etc.) with syntax highlighting for code blocks
- **FR-012**: The UI MUST display artifacts that have not yet been generated with a "Not yet generated" placeholder state
- **FR-013**: The UI MUST render code blocks in artifacts with syntax highlighting for at least Go, YAML, and shell languages

**Feature Management**

- **FR-014**: The UI MUST provide buttons to advance, recirculate, and cancel features via the corresponding API endpoints
- **FR-015**: The UI MUST disable the "Advance" button when the feature's gate has not passed and show a tooltip explaining why
- **FR-016**: The UI MUST show a confirmation dialog before executing destructive actions (cancel, recirculate)
- **FR-017**: The UI MUST provide a "Process" button that triggers the autonomous pipeline via `POST /api/features/:id/process`
- **FR-018**: The UI MUST provide a "Run Phase" button that triggers a single phase via `POST /api/features/:id/run`
- **FR-019**: The UI MUST provide an "Evaluate Gate" button that triggers gate evaluation via `GET /api/features/:id/gate`
- **FR-020**: The UI MUST hide or disable action buttons for terminal feature states (cancelled, done)

**Processing Progress**

- **FR-021**: The UI MUST show real-time progress during processing: current phase, agent role, dispatch status, and gate evaluation results
- **FR-022**: The UI MUST display elapsed time during processing phases longer than 30 seconds
- **FR-023**: The UI MUST disable the "Process" button when a feature is already being processed (status `in_progress` with an active SSE stream)
- **FR-024**: The UI MUST show the phase name, agent dispatch results, and gate outcomes as SSE events arrive during processing

**Backend API**

- **FR-025**: The backend MUST expose a REST API under `/api/` that the frontend SPA consumes
- **FR-026**: The backend MUST read and write feature state from the same `.devteam-state.yaml` files used by the CLI — the YAML files are the single source of truth
- **FR-027**: The backend MUST stream processing progress via Server-Sent Events on `GET /api/features/:id/stream`
- **FR-028**: The backend MUST serve the SPA static files from `/` with the API under `/api/`
- **FR-029**: The backend MUST embed the built frontend assets via `embed.FS` so the Go binary is self-contained
- **FR-030**: The backend MUST return appropriate HTTP status codes: 201 for feature creation, 200 for reads, 400 for validation errors (including invalid phase names, out-of-range priorities, empty titles, descriptions exceeding max length), 404 for missing features, 409 for conflicts (duplicate title warning, already processing), 500 for internal errors
- **FR-031**: The backend MUST validate all API inputs: reject empty titles and descriptions, enforce max lengths (title: 200 chars, description: 10,000 chars), validate phase names for recirculate (must be a valid phase and earlier than current), reject invalid priority values (only 1, 2, 3 allowed)
- **FR-032**: The backend MUST handle concurrent requests safely — if a feature is already being processed, return 409 Conflict rather than starting a second process
- **FR-033**: The backend MUST return 400 with a descriptive error when attempting to cancel an already-cancelled or done feature, advance a feature at the delivery phase, or recirculate to an invalid target phase
- **FR-034**: The backend MUST support multiple concurrent SSE connections for the same feature, broadcasting state changes to all connected clients

**Frontend**

- **FR-035**: The UI MUST be a single-page application using client-side routing (features list, feature detail, with URL-based routes)
- **FR-036**: The UI MUST support dark mode via `prefers-color-scheme` media query and a manual toggle that persists the preference in `localStorage`
- **FR-037**: The UI MUST be usable on viewports as narrow as 375px without horizontal scrolling
- **FR-038**: The UI MUST show loading spinners or skeleton states during data fetches and action submissions
- **FR-039**: The UI MUST show toast notifications for success and error outcomes of user actions (feature created, advance succeeded, cancel failed, etc.)

### Key Entities

- **Dashboard**: The main page showing all features in a card/table layout with sort controls
- **Feature Card**: Summary of a feature with ID, title, phase, status, priority, and gate badge
- **Feature Detail**: Full view of a feature with all artifacts, gate results, and action buttons (Run Phase, Evaluate Gate, Advance, Recirculate, Cancel, Process)
- **Artifact Viewer**: Markdown renderer with syntax highlighting for code blocks
- **Process View**: Real-time progress display showing phase transitions, agent dispatch results, and gate evaluations
- **Intake Form**: Submission form for loose ideas or external spec file uploads, with title, description, priority selector, and duplicate detection

### Non-Functional Requirements

- **NFR-001**: API responses for feature listing and detail MUST complete within 500ms for up to 100 features
- **NFR-002**: The frontend bundle MUST be under 500KB gzipped for initial load
- **NFR-003**: SSE events MUST be delivered within 5 seconds of a state change
- **NFR-004**: The Go binary with embedded frontend MUST start serving requests within 2 seconds
- **NFR-005**: The API MUST not expose secrets, agent prompts, or internal file paths — only feature state and artifact content
- **NFR-006**: The SPA MUST work in the latest versions of Chrome, Firefox, Safari, and Edge

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can submit a loose idea from the UI and see it appear in the pipeline within 2 seconds
- **SC-002**: A user can view all features and their current phase on the dashboard within 1 second
- **SC-003**: A user can read any artifact (spec, plan, tasks, etc.) rendered as formatted markdown
- **SC-004**: A user can advance a feature through the pipeline with one click (advance button)
- **SC-005**: A user can trigger autonomous processing and see real-time progress for each phase
- **SC-006**: The UI is usable on a 375px-wide mobile viewport with no horizontal scrolling
- **SC-007**: All CLI operations (`status`, `intake`, `run`, `process`, `advance`, `recirculate`, `gate`) are available through the web UI with equivalent behavior

## Architecture

### Backend (Go)

- Standard library `net/http` with `http.ServeMux` for routing (no external framework dependency)
- REST API serving feature state, artifacts, and pipeline operations under `/api/`
- SSE endpoint for real-time processing updates on `GET /api/features/:id/stream`
- Reuses all existing `internal/` packages: `pipeline`, `feature`, `spec`, `role`, `config`, `intake`
- Serves the SPA static files from `/` with API under `/api/`
- Frontend assets embedded via `embed.FS` and served from `//go:embed ui/dist/*`
- Built via `go generate` which runs `npm run build` in the `ui/` directory

### Frontend (TypeScript + React)

- Vite + React 19 + TypeScript SPA
- Tailwind CSS v4 for styling, dark mode via `prefers-color-scheme` + manual toggle
- React Router for client-side routing (feature list, feature detail)
- Markdown rendering with `react-markdown` + `rehype-highlight` for syntax highlighting
- Real-time updates via `EventSource` (SSE)
- State management via React Query (server state) + React context (theme, connection status)

### API Endpoints

```
GET    /api/features                    — List all features with phase/status
POST   /api/features                    — Create feature (loose idea or external spec)
GET    /api/features/:id                — Get feature detail with phase states
POST   /api/features/:id/run            — Run current phase (dispatch agents)
POST   /api/features/:id/advance        — Advance to next phase
POST   /api/features/:id/recirculate    — Recirculate to earlier phase (body: {"target_phase": "planning"})
POST   /api/features/:id/cancel         — Cancel feature
POST   /api/features/:id/process        — Process entire pipeline autonomously
GET    /api/features/:id/artifacts/:type — Get artifact content (spec, acceptance, plan, tasks, review_report, test_report, docs)
GET    /api/features/:id/gate            — Evaluate current gate
GET    /api/features/:id/stream          — SSE stream for processing progress
```

### API Response Shapes

```json
// GET /api/features — list
{
  "features": [
    {
      "id": "001-dev-team-platform",
      "title": "Dev Team Platform",
      "status": "in_progress",
      "priority": 1,
      "current_phase": "planning",
      "updated_at": "2026-06-20T10:30:00Z",
      "gate_result": null
    }
  ]
}

// GET /api/features/:id — detail
{
  "id": "001-dev-team-platform",
  "title": "Dev Team Platform",
  "status": "in_progress",
  "priority": 1,
  "intake_path": "loose_idea",
  "created_at": "2026-06-19T00:00:00Z",
  "updated_at": "2026-06-20T10:30:00Z",
  "phase_states": {
    "inception": {
      "phase": "inception",
      "status": "passed",
      "started_at": "2026-06-19T00:00:00Z",
      "completed_at": "2026-06-19T01:00:00Z",
      "artifacts": [
        {"type": "spec_md", "path": "specs/001-dev-team-platform/spec.md", "generated_by": "pm", "generated_at": "2026-06-19T01:00:00Z"}
      ],
      "gate_result": {
        "phase": "inception",
        "passed": true,
        "checks": [
          {"name": "spec.md exists", "passed": true, "message": "Found spec.md"},
          {"name": "acceptance.md exists", "passed": true, "message": "Found acceptance.md"}
        ]
      }
    },
    "planning": {
      "phase": "planning",
      "status": "in_progress",
      "started_at": "2026-06-20T10:00:00Z",
      "artifacts": [],
      "gate_result": null
    }
  },
  "dependencies": [],
  "repos": [
    {"name": "devteam", "url": "git@github.com:MichielDean/devteam.git", "branch": "main"}
  ]
}

// POST /api/features — create
// Request:
{
  "type": "loose_idea",          // or "external_spec"
  "title": "We need dark mode",
  "description": "Add dark mode support to the dashboard...",
  "priority": 1,
  "file_content": null            // base64-encoded file content for external_spec
}
// Response: 201 Created with full feature detail (same shape as GET /api/features/:id)

// POST /api/features/:id/advance — advance
// Response: 200 OK with updated feature detail

// POST /api/features/:id/recirculate — recirculate
// Request:
{
  "target_phase": "planning"
}
// Response: 200 OK with updated feature detail

// POST /api/features/:id/cancel — cancel
// Response: 200 OK with updated feature detail (status: "cancelled")

// POST /api/features/:id/run — run single phase
// Response: 200 OK with feature detail and gate result

// POST /api/features/:id/process — process entire pipeline
// Response: 200 OK with feature detail; real-time updates via SSE stream

// GET /api/features/:id/gate — evaluate gate
// Response: 200 OK with gate result
{
  "phase": "inception",
  "passed": true,
  "checks": [
    {"name": "spec.md exists", "passed": true, "message": "Found spec.md"},
    {"name": "acceptance.md exists", "passed": true, "message": "Found acceptance.md"}
  ]
}

// GET /api/features/:id/artifacts/:type — artifact content
// Response: 200 OK with artifact content as plain text (markdown)
// 404 if artifact not yet generated

// SSE event format (GET /api/features/:id/stream)
// Each event is a JSON object with a type field:
event: phase_change
data: {"feature_id": "001-dev-team-platform", "phase": "planning", "status": "in_progress", "timestamp": "2026-06-20T10:00:00Z"}

event: gate_result
data: {"feature_id": "001-dev-team-platform", "phase": "inception", "passed": true, "checks": [...]}

event: agent_dispatch
data: {"feature_id": "001-dev-team-platform", "phase": "inception", "role": "pm", "status": "dispatched", "timestamp": "2026-06-19T00:05:00Z"}

event: agent_complete
data: {"feature_id": "001-dev-team-platform", "phase": "inception", "role": "pm", "status": "success", "duration_ms": 120000}

event: processing_complete
data: {"feature_id": "001-dev-team-platform", "status": "done", "timestamp": "2026-06-20T12:00:00Z"}

event: error
data: {"feature_id": "001-dev-team-platform", "message": "Agent dispatch failed: timeout", "timestamp": "2026-06-20T10:00:00Z"}
```

### Project Structure

```
devteam/
├── cmd/
│   └── devteam/
│       └── main.go          # CLI + server mode (flag: -http :8080)
├── internal/
│   ├── config/              # Existing — YAML config loading
│   ├── feature/             # Existing — domain types, state machine
│   ├── intake/              # Existing — loose idea & external spec intake
│   ├── pipeline/            # Existing — phase execution, gate evaluation
│   ├── repo/                # Existing — cross-repo git operations
│   ├── role/                # Existing — role loader, agent dispatcher
│   ├── rules/               # Existing — AIDLC rule loader
│   ├── spec/                # Existing — spec provider, state persistence
│   └── api/                 # NEW — HTTP handlers, SSE, routing
│       ├── handler.go        # Feature CRUD handlers
│       ├── handler_artifact.go # Artifact serving handlers
│       ├── handler_pipeline.go # Pipeline action handlers (run, advance, etc.)
│       ├── handler_sse.go     # SSE stream handler
│       ├── server.go         # HTTP server setup, routing, middleware
│       ├── middleware.go     # CORS, logging, recovery middleware
│       └── dto.go            # Request/response data transfer objects
├── ui/                       # NEW — frontend SPA
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── tailwind.config.ts
│   ├── index.html
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── api/
│       │   └── client.ts     # API client functions
│       ├── hooks/
│       │   ├── useFeatures.ts
│       │   ├── useFeature.ts
│       │   └── useSSE.ts
│       ├── pages/
│       │   ├── Dashboard.tsx
│       │   └── FeatureDetail.tsx
│       ├── components/
│       │   ├── FeatureCard.tsx
│       │   ├── FeatureList.tsx
│       │   ├── IntakeForm.tsx
│       │   ├── ArtifactViewer.tsx
│       │   ├── ProcessView.tsx
│       │   ├── GateResult.tsx
│       │   └── ThemeToggle.tsx
│       └── types/
│           └── index.ts      # TypeScript types matching API responses
└── go.mod
```

## Assumptions

- **Single-user mode initially** — no auth required for local use. The API listens on `localhost` by default.
- **The Go binary serves both the API and the static SPA files.** The frontend is built during `go generate` and embedded via `embed.FS`.
- **Feature state is stored in the same `.devteam-state.yaml` files used by the CLI.** The API reads and writes these files directly. No separate database.
- **SSE is used for real-time updates** — simpler than WebSocket for server-to-client push, and sufficient for pipeline progress events.
- **The `devteam` binary gains a `-http` flag** (e.g., `devteam -http :8080`) to start the web server. Without this flag, it behaves as the existing CLI.
- **The pipeline execution model is unchanged.** The API calls the same `pipeline`, `feature`, `spec`, `role`, and `intake` packages the CLI uses. No new execution path.
- **Feature creation does NOT auto-start processing.** Creating a feature via the API sets it to `in_progress` in the `inception` phase, but the user must explicitly click "Run Phase" or "Process" to dispatch agents.
- **Agent dispatching is synchronous per request** but processing runs in a goroutine. SSE events are generated by watching the `.devteam-state.yaml` file for changes (file system notification or polling).
- **Valid phases for recirculation** are `inception`, `planning`, `construction`, `review`, `testing`, and `delivery`. The target phase must be earlier than the current phase.
- **Feature titles** have a maximum length of 200 characters. Descriptions have a maximum length of 10,000 characters.

## Scope Boundaries

### In Scope

- Web UI for all existing pipeline operations (intake, status, run, process, advance, recirculate, gate)
- Real-time pipeline progress via SSE
- Artifact viewing with markdown rendering
- Responsive SPA with dark mode
- REST API backing the SPA

### Out of Scope

- Authentication and authorization (deferred to a future feature)
- Multi-user session management
- Feature editing or modification after creation (beyond pipeline actions)
- Notification system (email, Slack, etc.)
- Admin dashboard or settings UI
- Custom pipeline configuration via the UI
- Multiple project/workspace support