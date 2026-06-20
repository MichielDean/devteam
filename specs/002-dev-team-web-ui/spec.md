# Feature Specification: Dev Team Web UI

**Feature Branch**: `002-dev-team-web-ui`

**Created**: 2026-06-20

**Status**: Draft

**Input**: The Dev Team platform needs a web interface so human team members can submit features, monitor pipeline progress, and review artifacts without using the CLI.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Submit a feature idea from the browser (Priority: P1)

A team member visits the Dev Team dashboard, types a loose idea into a text box, and clicks Submit. The feature appears in the pipeline with status "inception" and the PM agent begins processing it.

**Why this priority**: This is the front door. Without intake from the UI, nothing else matters.

**Independent Test**: Submit "We need dark mode" from the web UI and verify it creates a feature in inception phase with input.md generated.

**Acceptance Scenarios**:

1. **Given** the dashboard is open, **When** the user types a description and clicks Submit, **Then** a new feature is created with status "in_progress" and phase "inception"
2. **Given** a submitted idea, **When** the PM agent finishes inception, **Then** spec.md, acceptance.md, and repos.yaml are generated and visible in the UI
3. **Given** a feature in inception, **When** the user navigates to the feature detail page, **Then** they see the input idea, current phase, and all generated artifacts

---

### User Story 2 - Watch features move through the pipeline in real time (Priority: P1)

A team member opens the dashboard and sees all features with their current phase, status, and gate results. When a phase completes, the feature card updates to show the next phase.

**Why this priority**: The pipeline IS the product. Real-time visibility is essential for trust and coordination.

**Independent Test**: Start `devteam process` on a feature and verify the dashboard shows phase transitions as they happen.

**Acceptance Scenarios**:

1. **Given** multiple features exist, **When** the user views the dashboard, **Then** all features are listed with ID, title, phase, priority, and status
2. **Given** a feature is being processed, **When** the phase changes, **Then** the dashboard updates within 5 seconds to reflect the new phase
3. **Given** a gate evaluation completes, **When** the user views the feature, **Then** gate results (pass/fail per check) are displayed

---

### User Story 3 - Review artifacts from each phase in the browser (Priority: P1)

A team member clicks on a feature and sees all artifacts (spec.md, acceptance.md, plan.md, tasks.md, review report, test report, docs) rendered as formatted markdown with syntax highlighting.

**Why this priority**: Artifacts are the output of the pipeline. If users can't read them easily, the pipeline's value is lost.

**Independent Test**: Navigate to a feature detail page and verify all artifacts are rendered as markdown with proper formatting.

**Acceptance Scenarios**:

1. **Given** a feature with generated artifacts, **When** the user clicks the feature, **Then** all artifacts are listed and rendered as markdown
2. **Given** an artifact, **When** the user clicks it, **Then** the full content is displayed with syntax highlighting for code blocks
3. **Given** a feature in the review phase, **When** the user views the review report, **Then** each acceptance criterion shows pass/fail with evidence

---

### User Story 4 - Manage features from the dashboard (Priority: P2)

A team member can advance a feature through the pipeline, recirculate it back to an earlier phase, or cancel it entirely — all from the UI without touching the CLI.

**Why this priority**: Manual control lets humans intervene when the pipeline makes mistakes or priorities change.

**Independent Test**: Click "Advance" on a feature that has passed its gate and verify it moves to the next phase.

**Acceptance Scenarios**:

1. **Given** a feature with a passed gate, **When** the user clicks "Advance", **Then** the feature moves to the next phase
2. **Given** a feature with a failed gate, **When** the user clicks "Recirculate" and selects a target phase, **Then** the feature is sent back to that phase
3. **Given** a feature, **When** the user clicks "Cancel", **Then** the feature is marked as cancelled and removed from the active pipeline view

---

### User Story 5 - Trigger autonomous processing from the UI (Priority: P2)

A team member clicks "Process" on a feature and the UI shows real-time progress as each phase runs — dispatching the agent, waiting for completion, evaluating the gate, and advancing or recirculating.

**Why this priority**: This is the autonomous flow — one click to take a feature from idea to delivery.

**Independent Test**: Click "Process" on a feature in inception and verify it advances through phases automatically until delivery or a gate failure.

**Acceptance Scenarios**:

1. **Given** a feature in any phase, **When** the user clicks "Process", **Then** the UI shows each phase being dispatched with agent role and duration
2. **Given** a processing feature, **When** a gate fails, **Then** the UI shows the recirculation with the option to fix and retry or cancel
3. **Given** a processing feature that reaches delivery, **When** the final gate passes, **Then** the feature is marked as "done" with a summary of all phases and durations

---

### User Story 6 - Modern, responsive UI that works on mobile (Priority: P2)

The dashboard is a single-page application with a clean, modern design that works on desktop and mobile browsers. Dark mode is supported. Navigation is intuitive — features list, feature detail, and settings.

**Why this priority**: A clunky UI undermines trust. The UI should feel like a polished SaaS product, not an internal tool.

**Independent Test**: Open the dashboard on a phone-sized viewport and verify all core functionality is usable.

**Acceptance Scenarios**:

1. **Given** the dashboard, **When** viewed on a viewport of 375px width, **Then** all core functions (submit, view, advance) are accessible without horizontal scrolling
2. **Given** the dashboard in dark mode, **When** the user toggles the theme, **Then** all text is readable and all controls are functional
3. **Given** the dashboard, **When** the user navigates between features, **Then** page transitions take less than 200ms perceived latency

---

## Edge Cases

- What happens when the backend is processing a feature and the user submits the same idea again? The UI should detect potential duplicates and suggest merging.
- What happens when a processing run takes 30+ minutes? The UI should show a progress indicator and allow the user to check back later.
- What happens when the backend is down? The UI should show a clear connection error and queue actions for retry.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The UI MUST provide a form to submit loose ideas that creates a feature and dispatches the PM agent
- **FR-002**: The UI MUST display all features with their current phase, status, priority, and gate results
- **FR-003**: The UI MUST render markdown artifacts (spec.md, acceptance.md, plan.md, etc.) with syntax highlighting
- **FR-004**: The UI MUST provide buttons to advance, recirculate, and cancel features
- **FR-005**: The UI MUST provide a "Process" button that runs the entire pipeline autonomously
- **FR-006**: The UI MUST show real-time progress during processing (phase transitions, agent dispatch results, gate evaluations)
- **FR-007**: The UI MUST be a single-page application that works on mobile viewports
- **FR-008**: The backend MUST expose a REST API that the frontend consumes
- **FR-009**: The backend MUST serve the feature state from the same `.devteam-state.yaml` files used by the CLI
- **FR-010**: The backend MUST stream processing progress via Server-Sent Events (SSE) or WebSocket
- **FR-011**: The UI MUST support dark mode
- **FR-012**: The UI MUST allow creating features via the external spec intake path (file upload)

### Key Entities

- **Dashboard**: The main page showing all features in a card/table layout
- **Feature Card**: Summary of a feature with ID, title, phase, status, priority
- **Feature Detail**: Full view of a feature with all artifacts, gate results, and action buttons
- **Artifact Viewer**: Markdown renderer with syntax highlighting for code blocks
- **Process View**: Real-time progress display showing phase transitions and agent results

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can submit a loose idea from the UI and see it appear in the pipeline within 2 seconds
- **SC-002**: A user can view all features and their current phase on the dashboard within 1 second
- **SC-003**: A user can read any artifact (spec, plan, tasks, etc.) rendered as formatted markdown
- **SC-004**: A user can advance a feature through the pipeline with one click (advance button)
- **SC-005**: A user can trigger autonomous processing and see real-time progress for each phase
- **SC-006**: The UI is usable on a 375px-wide mobile viewport with no horizontal scrolling

## Architecture

### Backend (Go)

- REST API serving feature state, artifacts, and pipeline operations
- SSE endpoint for real-time processing updates
- Reuses all existing `internal/` packages (pipeline, feature, spec, role, config)
- Serves the SPA static files from `/` with API under `/api/`

### Frontend (TypeScript + React)

- Vite + React + TypeScript SPA
- Tailwind CSS for styling, dark mode via `prefers-color-scheme`
- Markdown rendering with `react-markdown` + syntax highlighting
- Real-time updates via EventSource (SSE)

### API Endpoints

```
GET    /api/features                    — List all features with phase/status
POST   /api/features                    — Create feature (loose idea or external spec)
GET    /api/features/:id                — Get feature detail with phase states
POST   /api/features/:id/run             — Run current phase (dispatch agents)
POST   /api/features/:id/advance         — Advance to next phase
POST   /api/features/:id/recirculate     — Recirculate to earlier phase
POST   /api/features/:id/process         — Process entire pipeline autonomously
GET    /api/features/:id/artifacts/:type — Get artifact content (spec.md, etc.)
GET    /api/features/:id/gate            — Evaluate current gate
GET    /api/features/:id/stream           — SSE stream for processing progress
```

## Assumptions

- Single-user mode initially (no auth required for local use)
- The Go binary serves both the API and the static SPA files
- Feature state is stored in the same `.devteam-state.yaml` files used by the CLI
- SSE is used for real-time updates (simpler than WebSocket for this use case)
- The frontend is built during `go generate` and embedded in the binary via `embed.FS`