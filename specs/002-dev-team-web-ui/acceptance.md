# Acceptance Criteria: Dev Team Web UI

**Spec**: 002-dev-team-web-ui
**Created**: 2026-06-20

---

## US-1: Submit a feature idea from the browser

- **AC-001**: Given the dashboard with the intake form open, When the user types a title, description, and clicks Submit, Then a `POST /api/features` request is made with `{type: "loose_idea", title: "...", description: "...", priority: N}` and the feature appears in the list with status "in_progress" and phase "inception"
- **AC-002**: Given a submitted idea, When the PM agent completes inception (triggered by clicking "Run Phase"), Then the feature detail page shows `spec.md`, `acceptance.md`, and `repos.yaml` as rendered markdown
- **AC-003**: Given the intake form, When the user selects "External Spec" and uploads a file, Then a feature is created with `intake_path: external_spec` and the uploaded file content is stored as `input.md`
- **AC-004**: Given the intake form, When the user submits without entering any text, Then the form shows a validation error "Description is required" and no API request is sent
- **AC-005**: Given the intake form, When the user types a description exceeding 10,000 characters, Then the form shows a validation error about the maximum length
- **AC-006**: Given existing features, When the user types a title matching an existing feature's title, Then the UI shows a warning "A feature with a similar title already exists" with options to proceed or cancel
- **AC-007**: Given the intake form, When the user selects a priority (1, 2, or 3), Then the priority is included in the creation request and defaults to 2 if not selected
- **AC-008**: Given the intake form, When the user submits a title exceeding 200 characters, Then the form shows a validation error about the maximum title length
- **AC-009**: Given the intake form, When the user submits an empty title, Then the form shows a validation error "Title is required" and no API request is sent
- **AC-010**: Given the API, When a `POST /api/features` request is made with a priority value outside 1–3, Then the response is HTTP 400 with an error message "Priority must be 1, 2, or 3"

## US-2: Watch features move through the pipeline in real time

- **AC-011**: Given multiple features exist, When the user views the dashboard, Then all features are displayed in a list/table with ID, title, phase, priority, and status
- **AC-012**: Given a feature being processed, When the phase changes, Then the dashboard updates within 5 seconds via SSE to reflect the new phase and status
- **AC-013**: Given a completed gate evaluation, When the user views the feature, Then each gate check shows pass/fail with a descriptive message
- **AC-014**: Given the dashboard, When the user clicks a sortable column header (phase, priority, status, updated), Then the feature list reorders accordingly
- **AC-015**: Given the dashboard, When the SSE connection drops, Then a "Connection lost" banner appears at the top of the page and disappears when reconnected
- **AC-016**: Given the dashboard with no features, When the user views it, Then an empty state is shown with a call-to-action to create the first feature

## US-3: Review artifacts from each phase in the browser

- **AC-017**: Given a feature with generated artifacts, When the user navigates to the feature detail page, Then all artifacts are listed with their type and `generated_by` role
- **AC-018**: Given an artifact, When the user clicks it, Then the content is rendered as formatted markdown with code syntax highlighting
- **AC-019**: Given code blocks in an artifact, When rendered, Then they display with appropriate syntax highlighting for Go, YAML, and shell languages
- **AC-020**: Given an artifact type that hasn't been generated yet, When the user views the feature detail, Then the artifact is listed but shown as "Not yet generated" with a placeholder state

## US-4: Manage features from the dashboard

- **AC-021**: Given a feature with a passed gate, When the user clicks "Advance", Then a `POST /api/features/:id/advance` is made and the feature moves to the next phase
- **AC-022**: Given a feature whose gate has not passed, When the user views the feature, Then the "Advance" button is disabled with a tooltip explaining "Gate has not passed"
- **AC-023**: Given a feature with a failed gate, When the user clicks "Recirculate" and selects a target phase, Then a `POST /api/features/:id/recirculate` is made with `{target_phase: "..."}` and the feature is sent back to that phase
- **AC-024**: Given a feature, When the user clicks "Cancel", Then a confirmation dialog appears asking "Are you sure you want to cancel this feature?" and only on confirmation is a `POST /api/features/:id/cancel` sent
- **AC-025**: Given a feature already being processed (status `in_progress` with active SSE stream), When the user views the feature, Then the "Process" button is disabled with a tooltip explaining "Feature is already being processed"
- **AC-026**: Given a feature, When the user clicks "Run Phase", Then a `POST /api/features/:id/run` is made and the UI shows the gate result after the phase completes
- **AC-027**: Given a feature, When the user clicks "Evaluate Gate", Then a `GET /api/features/:id/gate` is made and the gate results are displayed in the feature detail
- **AC-028**: Given a feature that is cancelled or done, When the user views it, Then the "Cancel" and "Advance" buttons are hidden or disabled since the feature is in a terminal state
- **AC-029**: Given a feature at the delivery phase with a passed gate, When the user views it, Then the "Advance" button is hidden and a "Mark Done" indicator is shown

## US-5: Trigger autonomous processing from the UI

- **AC-030**: Given a feature in any phase, When the user clicks "Process", Then a `POST /api/features/:id/process` is made and the UI shows a progress view with phase transitions
- **AC-031**: Given a processing feature, When a gate fails, Then the UI shows the recirculation event with the option to retry or cancel
- **AC-032**: Given a processing feature that reaches delivery, When the final gate passes, Then the feature is marked "done" and a summary shows all phases with durations
- **AC-033**: Given a processing feature that has been running for more than 30 seconds, When the user views the progress, Then the UI shows the current phase name and elapsed time
- **AC-034**: Given a processing feature, When SSE events arrive, Then each `phase_change`, `gate_result`, `agent_dispatch`, `agent_complete`, and `processing_complete` event is reflected in the progress view within 5 seconds

## US-6: Modern, responsive UI that works on mobile

- **AC-035**: Given the dashboard, When viewed on a viewport of 375px width, Then all core functions (submit, view, advance, process) are accessible without horizontal scrolling
- **AC-036**: Given the dashboard in dark mode, When the user toggles the theme, Then all text is readable and all controls are functional
- **AC-037**: Given the dashboard, When the user navigates between features, Then page transitions complete in under 200ms perceived latency
- **AC-038**: Given the dashboard, When the user refreshes the page, Then the current view (feature list or feature detail) is restored via URL-based routing
- **AC-039**: Given the dashboard, When an action succeeds (feature created, advance succeeded), Then a toast notification confirms the action
- **AC-040**: Given the dashboard, When an action fails (network error, 409 conflict), Then a toast notification shows the error message
- **AC-041**: Given the dashboard, When data is loading, Then a loading spinner or skeleton state is shown instead of blank content

## API Contract Acceptance Criteria

- **AC-042**: Given a `POST /api/features` with valid loose idea input, When the request is processed, Then the response is HTTP 201 with the full feature detail JSON
- **AC-043**: Given a `POST /api/features` with empty description, When the request is processed, Then the response is HTTP 400 with an error message "Description is required"
- **AC-044**: Given a `POST /api/features` with empty title, When the request is processed, Then the response is HTTP 400 with an error message "Title is required"
- **AC-045**: Given a `POST /api/features` with a title exceeding 200 characters, When the request is processed, Then the response is HTTP 400 with an error message about maximum title length
- **AC-046**: Given a `POST /api/features` with a priority value outside 1–3, When the request is processed, Then the response is HTTP 400 with an error message "Priority must be 1, 2, or 3"
- **AC-047**: Given a `POST /api/features/:id/process` for a feature already being processed, When the request is processed, Then the response is HTTP 409 with an error message indicating the feature is already in progress
- **AC-048**: Given a `GET /api/features/:id` for a non-existent feature ID, When the request is processed, Then the response is HTTP 404
- **AC-049**: Given a `POST /api/features/:id/recirculate` with an invalid target phase, When the request is processed, Then the response is HTTP 400 with an error message listing valid phases
- **AC-050**: Given a `POST /api/features/:id/recirculate` with a target phase that is not earlier than the current phase, When the request is processed, Then the response is HTTP 400 with an error message explaining that recirculation must target an earlier phase
- **AC-051**: Given the API, When any response is returned, Then no secrets, agent prompts, or internal file paths are exposed — only feature state and artifact content
- **AC-052**: Given `GET /api/features/:id/stream`, When a phase transition occurs, Then an SSE event of type `phase_change` is sent within 5 seconds with the new phase and status
- **AC-053**: Given `GET /api/features/:id/stream`, When processing completes, Then an SSE event of type `processing_complete` is sent with the final feature status
- **AC-054**: Given a `POST /api/features/:id/cancel` for a feature that is already cancelled, When the request is processed, Then the response is HTTP 400 with an error message indicating the feature is already cancelled
- **AC-055**: Given a `POST /api/features/:id/cancel` for a feature that is done, When the request is processed, Then the response is HTTP 400 with an error message indicating the feature is already completed
- **AC-056**: Given a `POST /api/features/:id/advance` for a feature at the delivery phase, When the request is processed, Then the response is HTTP 400 with an error message indicating the feature is at the final phase
- **AC-057**: Given a `GET /api/features/:id/artifacts/:type` for an artifact that has not been generated, When the request is processed, Then the response is HTTP 404
- **AC-058**: Given multiple SSE clients connected to the same feature stream, When a state change occurs, Then all connected clients receive the same SSE event