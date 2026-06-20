# Acceptance Criteria: Dev Team Web UI

**Spec**: 002-dev-team-web-ui
**Created**: 2026-06-20

## US-1: Submit a feature idea from the browser

- **AC-001**: Given the dashboard, When the user types a description in the intake form and clicks Submit, Then a POST /api/features request is made and the feature appears in the list with status "in_progress" and phase "inception"
- **AC-002**: Given a submitted idea, When the PM agent completes inception, Then the feature detail page shows spec.md, acceptance.md, and repos.yaml as rendered markdown
- **AC-003**: Given the intake form, When the user uploads a file (external spec), Then a feature is created with intake_path "external_spec"

## US-2: Watch features move through the pipeline in real time

- **AC-004**: Given multiple features exist, When the user views the dashboard, Then all features are displayed in a list/table with ID, title, phase, priority, and status
- **AC-005**: Given a feature being processed, When the phase changes, Then the dashboard updates within 5 seconds via SSE
- **AC-006**: Given a completed gate evaluation, When the user views the feature, Then each gate check shows pass/fail with a descriptive message

## US-3: Review artifacts from each phase in the browser

- **AC-007**: Given a feature with generated artifacts, When the user navigates to the feature detail page, Then all artifacts are listed with their type and generated_by role
- **AC-008**: Given an artifact, When the user clicks it, Then the content is rendered as formatted markdown with code syntax highlighting
- **AC-009**: Given code blocks in an artifact, When rendered, Then they display with appropriate syntax highlighting for Go, YAML, and shell

## US-4: Manage features from the dashboard

- **AC-010**: Given a feature with a passed gate, When the user clicks "Advance", Then a POST /api/features/:id/advance is made and the feature moves to the next phase
- **AC-011**: Given a feature with a failed gate, When the user clicks "Recirculate" and selects a target phase, Then a POST /api/features/:id/recirculate is made with the target phase
- **AC-012**: Given a feature, When the user clicks "Cancel", Then the feature is marked as cancelled and a confirmation dialog appears before the action

## US-5: Trigger autonomous processing from the UI

- **AC-013**: Given a feature, When the user clicks "Process", Then a POST /api/features/:id/process is made and the UI shows a progress view with phase transitions
- **AC-014**: Given a processing feature, When a gate fails, Then the UI shows the recirculation with the option to retry or cancel
- **AC-015**: Given a processing feature that reaches delivery, When the final gate passes, Then the feature is marked "done" and a summary shows all phases with durations

## US-6: Modern, responsive UI that works on mobile

- **AC-016**: Given the dashboard, When viewed on a 375px-wide viewport, Then all core functions are accessible without horizontal scrolling
- **AC-017**: Given the dashboard in dark mode, When the user toggles the theme, Then all text is readable and controls are functional
- **AC-018**: Given the dashboard, When navigating between features, Then page transitions complete in under 200ms perceived latency