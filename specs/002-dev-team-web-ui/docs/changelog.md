# Changelog

All notable changes to the Dev Team project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [1.0.0] — 2026-06-20

### Added — Spec 002: Dev Team Web UI

#### Backend (Go)

- REST API under `/api/` serving feature state, artifacts, and pipeline operations
- `GET /api/features` — List all features with phase/status summary
- `POST /api/features` — Create feature (loose idea or external spec)
- `GET /api/features/:id` — Get feature detail with full phase states
- `POST /api/features/:id/run` — Run current phase (dispatch agents)
- `POST /api/features/:id/advance` — Advance to next phase
- `POST /api/features/:id/recirculate` — Recirculate to earlier phase
- `POST /api/features/:id/cancel` — Cancel feature
- `POST /api/features/:id/process` — Process entire pipeline autonomously
- `GET /api/features/:id/artifacts/:type` — Get artifact content as markdown
- `GET /api/features/:id/gate` — Evaluate current gate
- `GET /api/features/:id/stream` — SSE stream for processing progress
- Security headers on all API responses (CSP, X-Content-Type-Options, X-Frame-Options, Referrer-Policy)
- Input validation on all endpoints (title, description, priority, phase names)
- Request body size limiting (1MB)
- Panic recovery middleware
- CORS middleware for local development
- SSE file watching via `fsnotify` for CLI-triggered state changes
- `-http` flag on `devteam` binary to start the web server
- `embed.FS` for serving the SPA frontend from the Go binary
- `go generate` directive to build the frontend before Go compilation
- Active processing registry with `sync.Map` to prevent duplicate processing (409 Conflict)
- DTO types and conversion helpers for all request/response shapes
- Integration tests covering 58 acceptance criteria

#### Frontend (React + TypeScript)

- Single-page application with React 19, Vite, and Tailwind CSS v4
- Dashboard page showing all features with sort controls
- Feature detail page with phase timeline, artifacts, and action buttons
- Intake form for submitting loose ideas and external specs
- Artifact viewer with markdown rendering and syntax highlighting (Go, YAML, shell)
- Process view showing real-time pipeline progress
- Gate result display with pass/fail per check
- Server-Sent Events (SSE) hook with auto-reconnect
- Connection status banner ("Connection lost" indicator)
- Dark mode with system preference detection and manual toggle
- Toast notification system for success and error feedback
- Empty state with call-to-action when no features exist
- URL-based routing (React Router): `/` for dashboard, `/features/:id` for detail
- Responsive layout supporting viewports down to 375px
- Loading states (spinners, skeletons) for all data fetches
- Confirmation dialogs for destructive actions (cancel, recirculate)

### Changed — Spec 002: Dev Team Web UI

- `cmd/devteam/main.go` — Added `-http` flag for web server mode
- `internal/feature/types.go` — Added helper methods for API validation (`IsValidPriority`, `ArtifactAPIPathToType`, phase ordering)
- `internal/pipeline/pipeline.go` — Added `ProcessAsync` method for goroutine-based processing with event streaming
- `go.mod` — Added `github.com/fsnotify/fsnotify` dependency

### Architecture — Spec 002: Dev Team Web UI

- The Go binary is self-contained: the frontend is embedded via `embed.FS` and served from `/`
- Feature state is stored in `.devteam-state.yaml` files (same as CLI) — no external database
- SSE (Server-Sent Events) for real-time updates, not WebSocket
- No authentication (local-only, single-user mode)
- Pipeline execution model is unchanged — the API calls the same `internal/` packages the CLI uses