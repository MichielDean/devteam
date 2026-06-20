---

description: "Task list for Dev Team Web UI implementation"
---

# Tasks: Dev Team Web UI

**Input**: Design documents from `/specs/002-dev-team-web-ui/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), acceptance.md (required)

**Organization**: Tasks grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1-US6, or INFRA for infrastructure)
- Include exact file paths in descriptions

## Path Conventions

- **Backend**: `cmd/`, `internal/` at repository root
- **Frontend**: `ui/src/` at repository root
- Paths assume the project structure from plan.md

---

## Phase 1: Setup & Infrastructure (Shared Foundation)

**Purpose**: Project initialization, backend API framework, frontend project scaffold, and core wiring

- [ ] T001 [P] [INFRA] Create Go API server framework in `internal/api/server.go` — HTTP server with `http.ServeMux` routing, `-http` flag parsing in `cmd/devteam/main.go`, `embed.FS` for static assets, SPA catch-all handler
- [ ] T002 [P] [INFRA] Create DTO types and conversion helpers in `internal/api/dto.go` — `CreateFeatureRequest`, `RecirculateRequest`, `FeatureListResponse`, `FeatureDetailResponse`, `FeatureSummary`, `PhaseStateResponse`, `ArtifactResponse`, `GateResultResponse`, `ErrorResponse`, SSE event types. Add `ToResponse()` methods that convert `feature.*` types to DTOs
- [ ] T003 [P] [INFRA] Create middleware in `internal/api/middleware.go` — CORS middleware (allow all origins for local dev), request logging, panic recovery, security headers (CSP, X-Content-Type-Options, X-Frame-Options, Referrer-Policy), request ID
- [ ] T004 [P] [INFRA] Add JSON tags and helper methods to `internal/feature/feature.go` — Add `json:"..."` struct tags to `Feature`, `PhaseState`, `Artifact`, `GateResult`, `CheckResult`, `RepoRef`. Add `IsTerminal() bool` method on `Feature`. Add `String()` methods on `Phase`, `Status`, `ArtifactType` in `internal/feature/types.go`
- [ ] T005 [INFRA] Add helper methods to `internal/spec/provider.go` — `ReadArtifactContent(featureID, artType)` to read artifact file content as string, `ListFeaturesSorted()` to return features sorted by updated_at descending. Add `ArtifactTypeToAPIPath()` mapping function
- [ ] T006 [INFRA] Add `ProcessAsync` method to `internal/pipeline/pipeline.go` — Method that runs autonomous processing in a goroutine, emitting SSE events to a channel. Method signature: `ProcessAsync(ctx context.Context, f *feature.Feature, eventCh chan<- SSEEvent) error`. Tracks active processing goroutines in a sync.Map to prevent duplicate processing
- [ ] T007 [P] [INFRA] Create frontend project scaffold in `ui/` — Initialize Vite + React 19 + TypeScript project with `package.json`, `vite.config.ts` (proxy `/api` to `:8080`), `tsconfig.json`, `tailwind.config.ts`, `postcss.config.js`, `index.html`. Add dependencies: react-router, @tanstack/react-query, react-markdown, rehype-highlight
- [ ] T008 [P] [INFRA] Create TypeScript types in `ui/src/types/index.ts` — Interfaces matching all API response DTOs: `Feature`, `FeatureSummary`, `FeatureDetail`, `PhaseState`, `Artifact`, `GateResult`, `CheckResult`, `RepoRef`, `CreateFeatureRequest`, `RecirculateRequest`, `ErrorResponse`, SSE event types
- [ ] T009 [P] [INFRA] Create API client in `ui/src/api/client.ts` — Fetch wrapper functions for all API endpoints: `listFeatures()`, `getFeature(id)`, `createFeature(req)`, `runPhase(id)`, `advanceFeature(id)`, `recirculateFeature(id, targetPhase)`, `cancelFeature(id)`, `processFeature(id)`, `evaluateGate(id)`, `getArtifact(id, type)`. Include error handling and response typing
- [ ] T010 [P] [INFRA] Create React Query provider and hooks in `ui/src/hooks/useFeatures.ts` and `ui/src/hooks/useFeature.ts` — `useFeatures()` returns cached feature list, `useFeature(id)` returns cached feature detail, both with automatic refetching and cache invalidation
- [ ] T011 [P] [INFRA] Create SSE hook in `ui/src/hooks/useSSE.ts` — `useSSE(featureId)` connects to `/api/features/:id/stream`, parses events, invalidates React Query cache on events, shows connection status, auto-reconnects on disconnect, cleans up on unmount
- [ ] T012 [P] [INFRA] Create theme context and toggle in `ui/src/components/ThemeToggle.tsx` — `ThemeProvider` context with dark/light mode, persisted in `localStorage`, respects `prefers-color-scheme`. `ThemeToggle` component toggles between modes
- [ ] T013 [P] [INFRA] Create toast notification system in `ui/src/components/Toast.tsx` — `ToastProvider` context, `useToast()` hook, success/error toast auto-dismiss after 5s, stack multiple toasts

**Checkpoint**: Backend serves empty API endpoints, frontend renders a blank page with dark mode toggle — foundation ready ✓

---

## Phase 2: US1 — Submit a Feature Idea from the Browser (Priority: P1) 🎯 MVP

**Goal**: Users can submit a loose idea or external spec from the web UI and see it appear in the feature list

**Independent Test**: Open the dashboard, submit "We need dark mode" as a loose idea, verify the feature appears with status "in_progress" and phase "inception"

### Backend for US1

- [ ] T014 [US1] Implement `CreateFeature` handler in `internal/api/handler.go` — Handle `POST /api/features`. Parse `CreateFeatureRequest` JSON body. Validate: title required + max 200 chars, description required for loose_idea + max 10000 chars, priority 1-3 defaulting to 2. For loose_idea: call `intake.NewLooseIdeaIntake().Submit()`. For external_spec: decode base64 file_content, call `intake.NewExternalSpecIntake().Submit()`. Check for duplicate title (case-insensitive) — return 409 Conflict if match found. Return 201 Created with `FeatureDetailResponse`
- [ ] T015 [US1] Implement `ListFeatures` handler in `internal/api/handler.go` — Handle `GET /api/features`. Call `pipeline.ListFeatures()`, convert each to `FeatureSummary`, return `FeatureListResponse`. Sort by `updated_at` descending
- [ ] T016 [US1] Implement `GetFeature` handler in `internal/api/handler.go` — Handle `GET /api/features/:id`. Call `pipeline.GetFeature(id)`, convert to `FeatureDetailResponse`. Return 404 if not found
- [ ] T017 [US1] Wire routes in `internal/api/server.go` — Register all handlers on `http.ServeMux`: `GET /api/features`, `POST /api/features`, `GET /api/features/:id`. Add middleware chain (CORS, logging, recovery, security headers). Add `-http` flag to `cmd/devteam/main.go` that starts the HTTP server

### Frontend for US1

- [ ] T018 [US1] Create `IntakeForm` component in `ui/src/components/IntakeForm.tsx` — Form with: title input (max 200 chars, required), description textarea (max 10000 chars, required for loose_idea), priority selector (1/2/3, default 2), type toggle (loose_idea/external_spec), file upload for external_spec (base64 encode), submit button. Client-side validation before API call. Show duplicate title warning (409 response)
- [ ] T019 [US1] Create `FeatureCard` component in `ui/src/components/FeatureCard.tsx` — Card showing: feature ID (truncated), title, current phase badge, priority indicator, status badge, gate result summary (pass/fail), updated_at relative time
- [ ] T020 [US1] Create `FeatureList` component in `ui/src/components/FeatureList.tsx` — Renders list of `FeatureCard` components from `useFeatures()` data. Sort controls for phase, priority, status, updated_at
- [ ] T021 [US1] Create `EmptyState` component in `ui/src/components/EmptyState.tsx` — Shown when no features exist. CTA button to create the first feature, opens `IntakeForm`

**Checkpoint**: User can submit a feature from the UI and see it in the list ✓

---

## Phase 3: US2 — Watch Features Move Through the Pipeline (Priority: P1)

**Goal**: Real-time dashboard showing all features with their current phase, status, and gate results, updating via SSE within 5 seconds

**Independent Test**: Start `devteam process` on a feature and verify the dashboard shows phase transitions as they happen

### Backend for US2

- [ ] T022 [US2] Implement SSE handler in `internal/api/handler_sse.go` — Handle `GET /api/features/:id/stream`. Set headers for SSE (`Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`). Register client channel in a global channel registry keyed by feature ID. Flush events to client. Send keep-alive comments every 30s. Clean up channel on client disconnect. Support multiple concurrent clients per feature
- [ ] T023 [US2] Implement SSE event broadcasting in `internal/api/handler_sse.go` — When `ProcessAsync` emits events, broadcast to all registered channels for that feature. Event format: `event: <type>\ndata: <json>\n\n`. Include all event types: `phase_change`, `gate_result`, `agent_dispatch`, `agent_complete`, `processing_complete`, `error`

### Frontend for US2

- [ ] T024 [US2] Update `useSSE` hook to process all event types in `ui/src/hooks/useSSE.ts` — On each SSE event, invalidate the relevant React Query cache entry (`useFeature` and `useFeatures`). Show connection status via callback. Auto-reconnect with exponential backoff on disconnect
- [ ] T025 [US2] Create `ConnectionStatus` component in `ui/src/components/ConnectionStatus.tsx` — Banner that shows "Connection lost" when SSE is disconnected, auto-hides when reconnected. Uses `useSSE` connection state
- [ ] T026 [US2] Update `Dashboard` page in `ui/src/pages/Dashboard.tsx` — Compose `FeatureList`, `EmptyState`, `ConnectionStatus`, sort controls. Show loading skeleton while data fetches. Integrate `IntakeForm` as a modal or panel

**Checkpoint**: Dashboard updates in real-time when pipeline phases change ✓

---

## Phase 4: US3 — Review Artifacts in the Browser (Priority: P1)

**Goal**: Users can view any artifact (spec, acceptance, plan, etc.) rendered as formatted markdown with syntax highlighting

**Independent Test**: Navigate to a feature detail page and verify all artifacts render as markdown with proper formatting and code highlighting

### Backend for US3

- [ ] T027 [US3] Implement `GetArtifact` handler in `internal/api/handler_artifact.go` — Handle `GET /api/features/:id/artifacts/:type`. Map `:type` to `ArtifactType` (input→input_md, spec→spec_md, etc.). Call `specProvider.ReadArtifactContent()`. Return content as `text/plain; charset=utf-8`. Return 404 if artifact not yet generated. Handle `docs` type (directory) by returning a listing or 404 if directory doesn't exist

### Frontend for US3

- [ ] T028 [US3] Create `ArtifactViewer` component in `ui/src/components/ArtifactViewer.tsx` — Render markdown content using `react-markdown` with `rehype-highlight` for syntax highlighting. Support Go, YAML, and shell language highlighting. Show "Not yet generated" placeholder for missing artifacts. Lazy-load large artifacts (>5MB) with loading indicator
- [ ] T029 [US3] Create `FeatureDetail` page in `ui/src/pages/FeatureDetail.tsx` — Route: `/features/:id`. Show feature header (title, status, priority, intake_path, created_at, updated_at). Show phase timeline. Tab view: Artifacts tab (list all artifacts with `ArtifactViewer`), Gate Results tab (`GateResult` component), Actions tab (management buttons). Show loading skeleton while data fetches. Handle 404 gracefully with navigation back to dashboard

**Checkpoint**: Users can view any artifact rendered as markdown with syntax highlighting ✓

---

## Phase 5: US4 — Manage Features from the Dashboard (Priority: P2)

**Goal**: Users can advance, recirculate, cancel, run a phase, and evaluate gates from the UI

**Independent Test**: Click "Advance" on a feature that has passed its gate and verify it moves to the next phase

### Backend for US4

- [ ] T030 [US4] Implement `RunPhase` handler in `internal/api/handler_pipeline.go` — Handle `POST /api/features/:id/run`. Load feature, call `pipeline.RunPhaseWithAgent()`. Return updated `FeatureDetailResponse`. Return 409 if feature is already being processed
- [ ] T031 [US4] Implement `AdvanceFeature` handler in `internal/api/handler_pipeline.go` — Handle `POST /api/features/:id/advance`. Load feature, evaluate gate. If gate passes: advance to next phase. If gate fails: return 400 with gate result. If at delivery with passed gate: mark done. If at delivery: return 400. If feature is terminal (cancelled/done): return 400
- [ ] T032 [US4] Implement `RecirculateFeature` handler in `internal/api/handler_pipeline.go` — Handle `POST /api/features/:id/recirculate`. Parse `RecirculateRequest` body. Validate `target_phase` is a valid phase and earlier than current. Call `pipeline.RecirculateFeature()`. Return updated `FeatureDetailResponse`. Return 400 for invalid target phase, forward phase, or terminal feature
- [ ] T033 [US4] Implement `CancelFeature` handler in `internal/api/handler_pipeline.go` — Handle `POST /api/features/:id/cancel`. Load feature, call `feature.Cancel()`, save state. Return updated `FeatureDetailResponse` with status "cancelled". Return 400 if feature is already cancelled or done
- [ ] T034 [US4] Implement `EvaluateGate` handler in `internal/api/handler_pipeline.go` — Handle `GET /api/features/:id/gate`. Load feature, call `pipeline.EvaluateGate()`. Return `GateResultResponse`. Return 404 if feature not found
- [ ] T035 [US4] Add input validation in all handlers in `internal/api/handler_pipeline.go` and `internal/api/handler.go` — Validate all request inputs: title max length, description max length, priority range, phase names for recirculate. Return 400 with descriptive error messages. Sanitize all error responses to not expose internal file paths, secrets, or agent prompts (NFR-005)

### Frontend for US4

- [ ] T036 [US4] Add pipeline action buttons to `FeatureDetail` page in `ui/src/pages/FeatureDetail.tsx` — "Run Phase" button (calls `/api/features/:id/run`), "Evaluate Gate" button (calls `/api/features/:id/gate`), "Advance" button (disabled with tooltip when gate hasn't passed, hidden when at delivery with passed gate — show "Mark Done" instead), "Recirculate" button (dropdown with valid backward phases), "Cancel" button (with confirmation dialog). Hide/disable actions for terminal features (cancelled, done). Show toast on success/error
- [ ] T037 [US4] Create `GateResult` component in `ui/src/components/GateResult.tsx` — Display gate checks: each check name with pass/fail icon, message. Show overall pass/fail badge. Highlight failing checks in red, passing in green

**Checkpoint**: All pipeline management actions work from the UI ✓

---

## Phase 6: US5 — Trigger Autonomous Processing from the UI (Priority: P2)

**Goal**: Users can click "Process" and see real-time progress as each phase runs

**Independent Test**: Click "Process" on a feature in inception and verify it advances through phases automatically until delivery or gate failure

### Backend for US5

- [ ] T038 [US5] Implement `ProcessFeature` handler in `internal/api/handler_pipeline.go` — Handle `POST /api/features/:id/process`. Check if feature is already being processed (use `sync.Map` of active goroutines). Return 409 if already processing. Start `ProcessAsync` goroutine. Return 200 with `FeatureDetailResponse`. Register SSE events channel

### Frontend for US5

- [ ] T039 [US5] Create `ProcessView` component in `ui/src/components/ProcessView.tsx` — Real-time progress display showing: current phase, agent role being dispatched, dispatch status, gate evaluation results, elapsed time (shown after 30s). Uses `useSSE` to receive `phase_change`, `gate_result`, `agent_dispatch`, `agent_complete`, `processing_complete` events. Show progress bar or timeline. Disable "Process" button when feature is already being processed. Show recirculation events with option to retry or cancel
- [ ] T040 [US5] Create `PhaseTimeline` component in `ui/src/components/PhaseTimeline.tsx` — Visual horizontal timeline showing all 6 phases. Highlight current phase. Mark completed phases with green check. Mark failed gate phases with red X. Show in-progress phase with spinner

**Checkpoint**: Users can trigger autonomous processing and see real-time progress ✓

---

## Phase 7: US6 — Modern, Responsive UI (Priority: P2)

**Goal**: Dashboard is a polished SPA that works on mobile with dark mode

**Independent Test**: Open the dashboard on a 375px viewport and verify all core functionality is usable

### Frontend for US6

- [ ] T041 [US6] Implement responsive layout in `ui/src/pages/Dashboard.tsx` and `ui/src/pages/FeatureDetail.tsx` — Use Tailwind responsive classes. Mobile-first design: stack cards vertically on small screens, show compact header. Ensure no horizontal scrolling at 375px width. Use `max-w-7xl mx-auto` for desktop, full width on mobile
- [ ] T042 [US6] Implement dark mode styling across all components — Use Tailwind `dark:` classes. Ensure all text is readable in both modes. Ensure all controls (buttons, inputs, dropdowns) are functional. Apply `ThemeToggle` in the header. Set CSP to allow `unsafe-inline` for Tailwind's dynamic classes
- [ ] T043 [US6] Implement URL-based routing with React Router in `ui/src/App.tsx` — `/` → Dashboard, `/features/:id` → FeatureDetail. On page refresh, restore the current view. Add `<Link>` navigation between pages. Ensure perceived navigation latency <200ms
- [ ] T044 [US6] Implement loading states across all pages — Skeleton/spinner states for: feature list loading, feature detail loading, artifact content loading, action submission in progress. Never show blank content
- [ ] T045 [US6] Implement toast notifications for all actions in `ui/src/components/Toast.tsx` — Success toasts: feature created, phase advanced, feature cancelled, gate evaluated. Error toasts: network error, validation error, 409 conflict, 404 not found. Auto-dismiss after 5s. Stack multiple toasts

**Checkpoint**: UI is responsive, dark-mode-aware, and polished ✓

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories and production readiness

- [ ] T046 [P] [INFRA] Add `go:embed` directive and build integration in `cmd/devteam/main.go` — Add `//go:embed ui/dist/*` directive. Add `go generate` comment that runs `cd ui && npm run build`. Wire `embed.FS` into the server for static file serving. Add SPA catch-all handler that serves `index.html` for non-`/api/` routes
- [ ] T047 [P] [INFRA] Add concurrent access safety in `internal/api/handler_pipeline.go` — Use `sync.Mutex` or `sync.Map` to prevent concurrent processing of the same feature. Track active goroutines for cleanup on server shutdown. Use `context.WithCancel` for graceful goroutine cancellation
- [ ] T048 [P] [INFRA] Add file watching for state changes in `internal/api/handler_sse.go` — Use `fsnotify` to watch `.devteam-state.yaml` files for changes. On file change, parse new state and broadcast SSE events to registered clients. Fallback to polling every 2s if fsnotify fails
- [ ] T049 [P] Write backend tests in `internal/api/handler_test.go` — Test all CRUD endpoints: create feature (valid, invalid, duplicate), list features, get feature (found, not found). Test all pipeline endpoints: run phase, advance (gate pass, gate fail, terminal state), recirculate (valid, invalid phase, forward phase), cancel (valid, already cancelled, already done), evaluate gate, process (valid, already processing)
- [ ] T050 [P] Write artifact handler tests in `internal/api/handler_artifact_test.go` — Test get artifact (found, not found, docs directory). Test all artifact type mappings
- [ ] T051 [P] Write SSE handler tests in `internal/api/handler_sse_test.go` — Test SSE connection, event streaming, multiple concurrent clients, keep-alive, client disconnect cleanup, event format
- [ ] T052 [P] Write DTO conversion tests in `internal/api/dto_test.go` — Test all `ToResponse()` conversions: Feature to FeatureDetailResponse, Feature to FeatureSummary, PhaseState to PhaseStateResponse, Artifact to ArtifactResponse, GateResult to GateResultResponse. Test edge cases: nil fields, empty slices, terminal states
- [ ] T053 [P] Write frontend component tests — Test IntakeForm validation (empty title, empty description, max lengths, priority default). Test FeatureCard rendering. Test GateResult pass/fail display. Test toast notifications. Test SSE hook reconnection
- [ ] T054 [P] Add security hardening — Validate all JSON inputs (reject oversized payloads, sanitize HTML in artifact content). Set security headers on all responses. No secrets or internal paths in error responses. Rate limit the process endpoint to prevent abuse
- [ ] T055 Write quickstart.md in `specs/002-dev-team-web-ui/quickstart.md` — Getting started guide: build the binary with `go generate` + `go build`, run with `-http :8080`, open browser, submit a feature, watch it process

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup & Infrastructure (Phase 1)**: No dependencies — start immediately
- **US1 (Phase 2)**: Depends on Phase 1 completion (API server, DTOs, types, frontend scaffold)
- **US2 (Phase 3)**: Depends on Phase 2 (need ListFeatures, GetFeature endpoints to show data; need SSE backend for real-time)
- **US3 (Phase 4)**: Depends on Phase 2 (need FeatureDetail page to show artifacts; can run in parallel with Phase 3 if backend is done)
- **US4 (Phase 5)**: Depends on Phase 2 (need FeatureDetail page to add action buttons; can run in parallel with Phases 3-4)
- **US5 (Phase 6)**: Depends on Phase 3 (SSE backend) and Phase 4 (FeatureDetail page for ProcessView)
- **US6 (Phase 7)**: Depends on all prior phases (needs all components to polish)
- **Polish (Phase 8)**: Depends on all user stories being complete

### Within Each Phase

- Backend tasks should be completed before frontend tasks that depend on them
- Within backend: DTOs before handlers, handlers before wiring
- Within frontend: types and API client before hooks, hooks before components

### Parallel Opportunities

- T001, T002, T003, T007, T008 can all run in parallel (different files, no dependencies)
- T004, T005, T006 can run in parallel with each other (different packages)
- T012, T013 can run in parallel (different components)
- US1 backend (T014-T017) and US1 frontend (T018-T021) can start in parallel once infrastructure is done
- US3 frontend (T028-T029) and US4 backend (T030-T035) can start in parallel once US1 is done
- US6 (T041-T045) tasks can mostly run in parallel since they're independent styling concerns
- T049-T053 (tests) can all run in parallel

---

## Implementation Strategy

### MVP First (US1 + US2 + US3)

1. Complete Phase 1: Setup & Infrastructure
2. Complete Phase 2: US1 — Feature submission
3. Complete Phase 3: US2 — Real-time dashboard
4. Complete Phase 4: US3 — Artifact viewing
5. **STOP and VALIDATE**: Can submit a feature, see it in the list, and read its artifacts
6. Deploy/demo if ready

### Full Delivery

7. Add Phase 5: US4 — Feature management actions
8. Add Phase 6: US5 — Autonomous processing
9. Add Phase 7: US6 — Responsive UI polish
10. Add Phase 8: Tests, security, documentation
11. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Everyone completes Phase 1 together (foundation)
2. Once foundation is done:
   - Developer A: Backend (US1 → US2 → US4 → US5)
   - Developer B: Frontend (US1 → US3 → US5 → US6)
   - Developer C: Tests and polish (Phase 8, starting after US1 is done)
3. Stories complete and integrate independently

---

## Notes

- **[P] tasks** = different files, no dependencies — can run in parallel
- **[Story] label** maps task to specific user story for traceability
- **[INFRA]** = cross-cutting infrastructure, not tied to a specific user story
- Each user story should be independently completable and testable
- The backend reuses all existing `internal/` packages — no new domain logic, only an HTTP API layer
- The frontend is entirely new code in `ui/` — no modifications to existing CLI code beyond the `-http` flag
- Commit after each task or logical group
- Stop at any checkpoint to validate the story independently
- **Critical design decision**: Feature creation does NOT auto-start processing. The user must explicitly click "Run Phase" or "Process"
- **Critical design decision**: SSE uses file watching on `.devteam-state.yaml`, not instrumentation of pipeline code, so CLI-triggered changes are also visible in the UI
- **Critical design decision**: The Go binary is self-contained — frontend is embedded via `embed.FS` and served from `/`