# Dev Team Web UI — User Guide

**Spec**: 002-dev-team-web-ui
**Version**: 1.0.0
**Last Updated**: 2026-06-20

---

## Overview

The Dev Team Web UI provides a browser-based interface for the Dev Team pipeline. From a single dashboard, you can submit feature ideas, watch them progress through pipeline phases, review generated artifacts, and manage features — all without using the CLI.

### Getting Started

Start the web server with the `-http` flag:

```bash
# Build the binary (includes frontend)
go generate ./cmd/devteam
go build ./cmd/devteam

# Start the web server
./devteam -http :8080
```

Open your browser to `http://localhost:8080`. The dashboard loads automatically.

**Prerequisites**: Go 1.23+, Node.js 20+ (for building the frontend).

---

## Submitting a Feature Idea (US-1)

### Loose Idea

1. Click the **"New Feature"** button on the dashboard.
2. In the Intake Form, select **"Loose Idea"** as the type.
3. Enter a **title** (required, max 200 characters).
4. Enter a **description** (required, max 10,000 characters).
5. Select a **priority**: 1 (High), 2 (Medium), or 3 (Low). Defaults to 2 (Medium) if not selected.
6. Click **Submit**.

The feature appears in the dashboard list with status "in_progress" and phase "inception". A `POST /api/features` request is sent with type `loose_idea`.

### External Spec

1. Click the **"New Feature"** button on the dashboard.
2. In the Intake Form, select **"External Spec"** as the type.
3. Enter a **title** and **description** (same rules as loose idea).
4. Upload a file using the file picker. The file content is base64-encoded and sent as `file_content`.
5. Click **Submit**.

The feature is created with `intake_path: external_spec` and the uploaded file content is stored as `input.md`.

### Validation

- **Empty title or description**: The form shows a validation error and does not submit.
- **Title over 200 characters**: The form shows a validation error.
- **Description over 10,000 characters**: The form shows a validation error.
- **Duplicate title**: If the title matches an existing feature (case-insensitive), a warning appears.
- **Priority out of range**: Only values 1, 2, or 3 are accepted.

### Important Note

Creating a feature does **not** automatically start processing. After creating a feature, you must click **"Run Phase"** or **"Process"** to dispatch the PM agent for the inception phase.

---

## Watching Features in Real Time (US-2)

### Dashboard

The dashboard displays all features in a list with the following columns:

| Column | Description |
|--------|-------------|
| **ID** | Feature identifier (e.g., `001-dev-team-platform`) |
| **Title** | Feature title |
| **Phase** | Current pipeline phase (inception, planning, construction, review, testing, delivery) |
| **Priority** | Feature priority (1=High, 2=Medium, 3=Low) |
| **Status** | Current status (in_progress, passed, failed, cancelled, done) |
| **Updated** | Last update time |

Click any column header to sort the feature list by that field.

### Real-Time Updates

The dashboard updates automatically via Server-Sent Events (SSE). When a phase changes, the feature card reflects the new phase within 5 seconds.

If the connection to the server is lost, a **"Connection lost"** banner appears at the top of the page. The connection is restored automatically when the server is available again.

### Empty State

When no features exist, the dashboard shows an empty state with a call-to-action to create the first feature.

---

## Reviewing Artifacts (US-3)

### Feature Detail Page

Click any feature in the dashboard to open the Feature Detail page. This page shows:

- **Feature header**: Title, status, priority, intake path, creation date, last update date.
- **Phase timeline**: Visual indicator of the current pipeline phase.
- **Artifacts tab**: Lists all generated artifacts with their type and the role that generated them.

### Viewing Artifact Content

Click an artifact to view its content rendered as formatted markdown. Code blocks are syntax-highlighted for Go, YAML, and shell languages.

Artifacts that have not yet been generated are listed with a **"Not yet generated"** placeholder state.

### Supported Artifact Types

| Type | API Path | Description |
|------|----------|-------------|
| Input | `input` | The original idea or external spec |
| Specification | `spec` | Generated specification (`spec.md`) |
| Acceptance Criteria | `acceptance` | Acceptance criteria (`acceptance.md`) |
| Repositories | `repos` | Repository scope (`repos.yaml`) |
| Plan | `plan` | Implementation plan (`plan.md`) |
| Tasks | `tasks` | Task breakdown (`tasks.md`) |
| Review Report | `review_report` | Code review report (`review-report.md`) |
| Test Report | `test_report` | Test results (`test-report.md`) |
| Documentation | `docs` | Generated documentation |

---

## Managing Features (US-4)

### Feature Actions

From the Feature Detail page, you can perform the following actions:

| Action | Button | Description |
|--------|--------|-------------|
| **Run Phase** | "Run Phase" | Dispatches the agent for the current phase. Returns the gate result after the phase completes. |
| **Evaluate Gate** | "Evaluate Gate" | Evaluates the current phase's gate and displays the results (pass/fail per check). |
| **Advance** | "Advance" | Moves the feature to the next phase. Only available when the gate has passed. Disabled with a tooltip if the gate has not passed. |
| **Recirculate** | "Recirculate" | Sends the feature back to an earlier phase. You select the target phase from a dropdown of valid earlier phases. A confirmation dialog appears before executing. |
| **Cancel** | "Cancel" | Cancels the feature. A confirmation dialog appears before executing. |
| **Process** | "Process" | Triggers autonomous processing — the pipeline runs all phases automatically. See [Triggering Autonomous Processing](#triggering-autonomous-processing-us-5). |

### Button States

- **Advance**: Disabled when the gate has not passed. Hidden when the feature is at the delivery phase with a passed gate (shows "Mark Done" indicator instead).
- **Cancel and Advance**: Hidden or disabled when the feature is in a terminal state (cancelled or done).
- **Process**: Disabled when the feature is already being processed.

### Confirmation Dialogs

Destructive actions (cancel, recirculate) require confirmation before execution. A dialog appears asking you to confirm before the API request is sent.

---

## Triggering Autonomous Processing (US-5)

### How Processing Works

Clicking **"Process"** starts autonomous processing:

1. The feature begins at its current phase.
2. The PM agent is dispatched for the current phase.
3. When the agent completes, the gate is evaluated.
4. If the gate passes, the feature advances to the next phase.
5. If the gate fails, the feature is recirculated back.
6. Steps 2–5 repeat until the feature reaches delivery (done) or encounters an error.

### Real-Time Progress View

While processing is active, the UI shows a **Process View** with:

- **Current phase** name and status.
- **Agent dispatch** events showing which role is being dispatched.
- **Gate evaluation** results showing pass/fail for each check.
- **Phase transitions** as the feature moves through the pipeline.
- **Elapsed time** after 30 seconds of processing.

All events (`phase_change`, `gate_result`, `agent_dispatch`, `agent_complete`, `processing_complete`) are reflected in the progress view within 5 seconds via SSE.

When processing completes, a summary shows all phases with their durations.

### Gate Failure During Processing

If a gate fails during processing, the UI shows the recirculation event. You can choose to retry or cancel the processing.

---

## Dark Mode and Accessibility (US-6)

### Dark Mode

The dashboard supports dark mode via:

- **System preference**: Automatically follows your operating system's `prefers-color-scheme` setting.
- **Manual toggle**: Click the theme toggle (sun/moon icon) in the header to switch between light and dark modes. Your preference is saved in `localStorage`.

### Mobile Support

The dashboard is responsive and works on viewports as narrow as 375px. All core functions (submit, view, advance) are accessible without horizontal scrolling.

### URL-Based Routing

- `/` — Dashboard (feature list)
- `/features/:id` — Feature detail page

Refreshing the page restores your current view. Navigation between pages takes less than 200ms.

### Notifications

- **Success**: A toast notification confirms successful actions (feature created, advance succeeded, etc.).
- **Error**: A toast notification shows error messages (network error, 409 conflict, etc.).
- **Loading**: Loading spinners or skeleton states appear during data fetches instead of blank content.

---

## CLI Compatibility

The web UI and CLI share the same state files (`.devteam-state.yaml`). Actions performed via the CLI are immediately reflected in the web UI on the next refresh or SSE event.

| CLI Command | Web UI Equivalent |
|-------------|-------------------|
| `devteam status` | Dashboard (feature list) |
| `devteam intake` | Intake Form (new feature) |
| `devteam run` | "Run Phase" button |
| `devteam process` | "Process" button |
| `devteam advance` | "Advance" button |
| `devteam recirculate` | "Recirculate" dropdown |
| `devteam gate` | "Evaluate Gate" button |

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| **"Connection lost" banner** | The server is unreachable. Check that `devteam -http :8080` is running. The UI reconnects automatically. |
| **Feature not updating** | Click the feature in the dashboard to force a refresh. If the SSE connection is lost, the "Connection lost" banner appears. |
| **"Feature is already being processed"** | Wait for the current processing to complete. The "Process" button is disabled during active processing. |
| **"Gate has not passed" tooltip on Advance** | The gate for the current phase has not passed. Click "Evaluate Gate" to see which checks failed, then "Run Phase" to retry. |
| **Artifact shows "Not yet generated"** | The phase that generates this artifact has not completed yet. Run the phase or trigger processing to generate artifacts. |
| **Error toast after action** | Check the error message. Common causes: network error (server down), 409 conflict (duplicate title or already processing), 404 (feature not found). |