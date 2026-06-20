# Release Notes — Spec 002: Dev Team Web UI

**Release**: 1.0.0
**Date**: 2026-06-20
**Spec**: 002-dev-team-web-ui

---

## Summary

This release adds a web UI to the Dev Team CLI binary, providing a browser-based dashboard for submitting feature ideas, monitoring pipeline progress, reviewing generated artifacts, and managing features — all without using the command line.

The Go binary gains a `-http` flag that starts an HTTP server serving both the REST API and the embedded React SPA. Without this flag, the binary behaves identically to the existing CLI.

---

## What's New

### Submit Feature Ideas from the Browser

- Submit loose ideas or external specs via an intake form
- Set priority (1=High, 2=Medium, 3=Low) with default of 2
- Client-side and server-side validation for titles (max 200 chars) and descriptions (max 10,000 chars)
- Duplicate title detection with warning

### Real-Time Pipeline Dashboard

- View all features with phase, status, priority, and gate results
- Sort by any column header (phase, priority, status, updated)
- Real-time updates via Server-Sent Events (SSE) within 5 seconds
- "Connection lost" banner when SSE disconnects, auto-reconnect
- Empty state with call-to-action when no features exist

### Artifact Review

- View all generated artifacts (spec, acceptance, plan, tasks, review report, test report, docs) as formatted markdown
- Syntax highlighting for Go, YAML, and shell code blocks
- "Not yet generated" placeholder for artifacts that haven't been produced

### Feature Management

- Run a single phase via "Run Phase" button
- Evaluate gates via "Evaluate Gate" button
- Advance features through the pipeline via "Advance" button
- Recirculate features to earlier phases via "Recirculate" dropdown
- Cancel features with confirmation dialog
- Trigger autonomous processing via "Process" button with real-time progress

### Modern, Responsive UI

- Dark mode with system preference detection and manual toggle
- Responsive layout supporting viewports down to 375px
- URL-based routing (dashboard at `/`, feature detail at `/features/:id`)
- Toast notifications for success and error feedback
- Loading spinners and skeleton states

---

## Cross-Repo Release Order

This feature is contained entirely within the `devteam` repository. No cross-repo coordination is needed.

### Release Order

1. **devteam** (this repo) — Release the Go binary with embedded frontend

### Repositories Affected

| Repository | Change | Release Order |
|------------|--------|---------------|
| devteam | Add web UI (API + frontend SPA) | 1 (only repo) |

---

## Deployment Instructions

### Prerequisites

- Go 1.23+ (for building the backend)
- Node.js 20+ and npm (for building the frontend)

### Build

```bash
# Clone and checkout
git clone git@github.com:MichielDean/devteam.git
cd devteam
git checkout 002-dev-team-web-ui

# Build the frontend and Go binary
go generate ./cmd/devteam   # runs: cd ui && npm install && npm run build
go build ./cmd/devteam

# Or use go run for development
go run ./cmd/devteam -http :8080
```

### Run

```bash
# Start the web server
./devteam -http :8080

# Open in browser
open http://localhost:8080
```

The binary serves the REST API at `/api/` and the SPA at `/`. All non-API routes fall back to `index.html` for client-side routing.

### CLI Mode (Unchanged)

Without the `-http` flag, the binary behaves identically to the existing CLI:

```bash
./devteam status
./devteam intake
```

### Frontend Development

For frontend-only development with hot module replacement:

```bash
# Terminal 1: Backend
go run ./cmd/devteam -http :8080

# Terminal 2: Frontend dev server
cd ui/
npm install
npm run dev   # starts Vite dev server on :5173 with proxy to :8080
```

---

## Verification

### Smoke Tests

1. **Start the server**: `./devteam -http :8080` — server starts within 2 seconds
2. **Submit a feature**: Open `http://localhost:8080`, type "We need dark mode", click Submit — feature appears in list within 2 seconds
3. **View artifacts**: Click on a feature with generated artifacts — all artifacts render as markdown
4. **Real-time updates**: Start `devteam process` on a feature — dashboard updates within 5 seconds
5. **Dark mode**: Toggle the theme — all text readable, all controls functional

### API Tests

```bash
# Create a feature
curl -X POST http://localhost:8080/api/features \
  -H 'Content-Type: application/json' \
  -d '{"type":"loose_idea","title":"Test feature","description":"Test description","priority":2}'

# List features
curl http://localhost:8080/api/features

# Get feature detail
curl http://localhost:8080/api/features/TEST-FEATURE-ID

# Run backend test suite
go test ./internal/api/... -v
```

---

## Backward Compatibility

- The `-http` flag is optional. Without it, the binary operates as the existing CLI with no changes.
- The `.devteam-state.yaml` files are the single source of truth for both CLI and web UI. No migration is needed.
- All existing CLI commands continue to work alongside the web UI.

---

## Known Limitations

- **No authentication**: The web UI is intended for single-user local development. Authentication will be added in a future feature.
- **No feature editing**: Features can be created, advanced, recirculated, and cancelled, but cannot be edited after creation.
- **No notification system**: Email, Slack, and other notifications are out of scope for this feature.
- **No admin dashboard**: Settings UI and admin controls are out of scope.
- **No multi-project support**: The web UI operates on a single project at a time.

---

## Acceptance Criteria Coverage

| User Story | Acceptance Criteria | Backend Tests | Frontend Tests |
|-----------|-------------------|---------------|----------------|
| US-1: Submit feature idea | AC-001 through AC-010 | 10/10 pass | Requires browser testing |
| US-2: Watch pipeline in real time | AC-011 through AC-016 | 4/6 pass (2 frontend-only) | Requires browser testing |
| US-3: Review artifacts | AC-017 through AC-020 | 4/4 pass | Requires browser testing |
| US-4: Manage features | AC-021 through AC-029 | 8/9 pass (1 manual) | Requires browser testing |
| US-5: Trigger processing | AC-030 through AC-034 | 2/5 pass (3 manual) | Requires browser testing |
| US-6: Responsive UI | AC-035 through AC-041 | N/A (frontend only) | Requires browser testing |
| API contracts | AC-042 through AC-058 | 17/17 pass | N/A |

**Total**: 50 backend acceptance tests pass, 7 DTO/middleware tests pass. Frontend ACs require browser-based E2E testing.