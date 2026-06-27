# Feature Specification: Feature Edit & Delete (CRUD Complete)

**Feature Branch**: `spec/crud-feature`

**Created**: 2026-06-26

**Status**: Draft

**Input**: User description: "Add edit and delete operations for features. Users should be able to rename/edit features and delete features from the UI."

## Workspace Summary (Brownfield)

**Repo**: `devteam` (git@github.com:MichielDean/devteam.git) — primary implementation repo.

**Stack**: Go backend (Go 1.x, `net/http` with Go 1.22 `http.ServeMux` method-pattern routing) + React/TypeScript frontend (Vite, `@tanstack/react-query`, Tailwind CSS). SQLite (default) / PostgreSQL operational DB. E2E via Playwright on port 18765. Go tests use `httptest`.

**Existing feature surface** (current state — no edit or delete endpoints exist):
- `GET  /api/features` — list (returns `[]` when empty, never `null`/`404`)
- `POST /api/features` — create (`CreateFeatureRequest`: type, title, description, priority, file_content?, start_immediately?)
- `GET  /api/features/{id}` — detail
- `POST /api/features/{id}/run` — run current phase
- `POST /api/features/{id}/advance` — advance phase
- `POST /api/features/{id}/recirculate` — back to earlier phase
- `POST /api/features/{id}/cancel` — set status `cancelled`
- `POST /api/features/{id}/process` — autopilot full pipeline
- `GET  /api/features/{id}/gate` — evaluate gate
- `GET|POST /api/features/{id}/artifacts/{type}` — artifact I/O
- `GET /api/features/{id}/stream` — SSE
- Question endpoints (`/questions`, `/questions/pending`, `PATCH /questions/{questionId}`)

**DB layer** (`internal/db/feature_store.go`): `UpdateFeature(FeatureRow)` updates title/current_phase/status/priority/worktree_dir/updated_at/recirculation_count — **does NOT update intake_path or spec_dir** (immutable). `DeleteFeature(id)` hard-deletes with cascade (FK `ON DELETE CASCADE` on phase_states, questions, notes, sessions, recirculations, events, artifacts, gate_results).

**Feature entity** (`internal/feature/feature.go`): fields `ID, Title, Current, Status, Priority, IntakePath, SpecDir, WorktreeDir, CreatedAt, UpdatedAt, Dependencies, Repos, PreparedRepos, PhaseStates`.

**Valid statuses**: `draft, in_progress, gate_blocked, passed, failed, done, recirculated, cancelled, waiting_for_feedback`. **Valid priorities**: `1|2|3`.

**Conventions**: error responses use `ErrorResponse{Error string, Details string}` via `writeError(w, code, errorCode, details)`. JSON arrays initialized `[]` not `null`. CORS allows `GET, POST, PATCH, OPTIONS`. `MaxBytesReader` 1MB on request bodies. Existing validation pattern: trim check, length check, enum check, 400 `validation_error` on failure.

**UI** (`ui/src/`): `FeatureDetail.tsx` is the feature page (header card shows title/id/status/priority + metadata grid). API client in `api/client.ts`. `@tanstack/react-query` for server state, `useToast` for feedback, `useSSE` for live updates. `Link` from `react-router`. Tailwind utility classes, dark-mode variants. Buttons carry `data-testid` selectors.

## User Scenarios & Testing

### User Story 1 - Edit a feature's title and priority (Priority: P1)

A user viewing a feature's detail page opens an edit affordance (edit button → edit form rendered on the detail page itself), changes the title and/or priority, and saves. The feature row is updated server-side and the UI reflects the new values without a full page reload. The feature ID is immutable.

**Why this priority**: Editing is the core "U" in CRUD. Users misname features or mis-prioritize them constantly; without edit they must cancel + recreate, losing pipeline state.

**Independent Test**: Create a feature, open its detail page, click edit, change title and priority, save — verify `GET /api/features/{id}` returns the new title and priority and the list page shows the updated values. No delete needed for this story.

**Acceptance Scenarios**:

1. **Given** a feature in `draft` status exists, **When** the user opens its detail page and clicks "Edit", **Then** an edit form appears on the detail page with the current title and priority pre-filled.
2. **Given** the edit form is open, **When** the user changes the title to "New Name" and priority to 1 and clicks Save, **Then** `PATCH /api/features/{id}` returns 200 with the updated feature and the detail header shows "New Name" / "P1".
3. **Given** the edit form is open, **When** the user clicks Cancel, **Then** the form closes with no server request and the original values remain.
4. **Given** a feature with status `in_progress` or `waiting_for_feedback`, **When** the user opens the detail page, **Then** the edit affordance is disabled/hidden with a tooltip explaining edits are blocked while processing.

### User Story 2 - Delete a feature (Priority: P1)

A user viewing a feature's detail page clicks a "Delete" button behind a confirm dialog. On confirmation, the feature row and all related data (phase states, questions, notes, sessions, recirculations, events, artifacts, gate results, spec worktree) are removed. The user is redirected to the dashboard.

**Why this priority**: Delete is the core "D" in CRUD. Stale/abandoned features clutter the dashboard and confuse the pipeline. Hard delete (per human answer) removes the row and cascades.

**Independent Test**: Create a feature, open its detail page, click Delete, confirm — verify `GET /api/features/{id}` returns 404 and the feature no longer appears in `GET /api/features`. No edit needed for this story.

**Acceptance Scenarios**:

1. **Given** a feature in `draft` or terminal (`done`/`cancelled`) status exists, **When** the user opens its detail page, **Then** a "Delete" button is visible.
2. **Given** the Delete button is visible, **When** the user clicks it, **Then** a confirm dialog appears asking to confirm deletion of "[title]".
3. **Given** the confirm dialog is open, **When** the user confirms, **Then** `DELETE /api/features/{id}` is called, returns 204, and the user is navigated to the dashboard.
4. **Given** the confirm dialog is open, **When** the user cancels, **Then** the dialog closes and no request is sent.
5. **Given** a feature in `in_progress`/`waiting_for_feedback`/`gate_blocked` status, **When** the user opens the detail page, **Then** the Delete button is disabled with a tooltip: feature must be cancelled first.

### User Story 3 - Delete from list context (Priority: P3)

A user can delete multiple stale draft/terminal features quickly without drilling into each detail page.

**Why this priority**: Convenience for cleanup, not core. P1 stories deliver full CRUD already.

**Independent Test**: On the dashboard, each deletable feature card has a delete affordance; clicking it shows the confirm dialog; confirming removes the feature from the list without navigation.

**Acceptance Scenarios**:

1. **Given** the dashboard lists several draft/terminal features, **When** the user clicks delete on a feature card, **Then** a confirm dialog appears and confirming removes that card from the list.

### Edge Cases

- **Delete a feature currently being processed by an agent (tmux session alive)**: DELETE must be rejected with 409 `feature_processing` — deleting mid-run corrupts pipeline state and orphans tmux sessions. [ASSUMPTION: the API checks `s.IsProcessing(id)` / dispatcher `IsSessionAlive` before deleting and returns 409.]
- **Edit a feature mid-processing**: per human answer, no edits while processing. PATCH rejected with 409 `feature_processing`.
- **Edit ID field**: ID is immutable; the request DTO must not include `id`, and any `id` in the body is ignored. The URL `{id}` is the source of truth.
- **Edit title to empty/whitespace**: 400 `empty_title`.
- **Edit title > 200 chars**: 400 `title_too_long` (matches existing create constraint).
- **Edit priority out of range (not 1-3)**: 400 `invalid_priority`.
- **Edit a non-existent feature**: 404 `feature_not_found`.
- **Delete a non-existent feature**: 404 `feature_not_found` (idempotent consideration: 204 vs 404 — see [ASSUMPTION] below; we choose 404 to distinguish "never existed" from "already gone").
- **Delete leaves the spec worktree behind**: hard delete of the DB row must also clean up `~/worktrees/devteam-specs/<id>/` and the `spec/<id>` branch if present. [ASSUMPTION: pipeline exposes a cleanup hook; if not, the delete endpoint removes the DB row and logs a warning for orphan worktree cleanup tracked separately. Minimal MVP: DB row + cascade only; worktree cleanup is a P2 follow-up.]
- **Concurrent edit + delete**: two clients, one PATCHing and one DELETEing the same feature. DB transaction ordering decides; whichever runs last wins. Acceptable for single-user platform. No optimistic concurrency control (YAGNI).
- **DELETE called twice (race)**: first returns 204, second returns 404. Acceptable.

## Requirements

### Functional Requirements

- **FR-001**: System MUST expose `PATCH /api/features/{id}` accepting `{title?, priority?}` and updating only those fields. Source: US-001.
- **FR-002**: System MUST validate the title (1-200 chars, non-empty after trim) and priority (1-3) on PATCH, returning 400 with `empty_title` / `title_too_long` / `invalid_priority` error codes consistent with the existing create endpoint. Source: US-001.
- **FR-003**: System MUST reject PATCH on a feature whose status is `in_progress`, `waiting_for_feedback`, or `gate_blocked` with 409 `feature_processing`. Source: US-001, human answer (no edits while processing).
- **FR-004**: System MUST keep the feature ID immutable; `id`, `intake_path`, `spec_dir`, `current_phase`, `created_at` are never modified by PATCH. Source: US-001, human answer.
- **FR-005**: System MUST expose `DELETE /api/features/{id}` that hard-deletes the feature row and cascade-deletes all related rows (phase_states, questions, notes, sessions, recirculations, events, artifacts, gate_results) via the existing `ON DELETE CASCADE` schema. Source: US-002, human answer.
- **FR-006**: System MUST reject DELETE on a feature currently being processed (`IsProcessing(id)` true or tmux session alive) with 409 `feature_processing`. Source: US-002, edge case.
- **FR-007**: System MUST reject DELETE on a feature whose status is `in_progress`, `waiting_for_feedback`, or `gate_blocked` with 400 `not_deletable` and message instructing the user to cancel first. Source: US-002, human answer (only draft or terminal features deletable; in-progress must be cancelled first).
- **FR-008**: System MUST return 404 `feature_not_found` for PATCH/DELETE on a non-existent feature ID. Source: US-001, US-002.
- **FR-009**: System MUST return 204 No Content on successful DELETE (no response body). Source: US-002.
- **FR-010**: System MUST set `updated_at` to `now()` on successful PATCH. Source: US-001.
- **FR-011**: PATCH MUST allow updating title and priority independently — a request with only `{priority: 2}` leaves title unchanged, and vice versa. Omitted fields are not zeroed. Source: US-001, human answer (title and priority are the editable fields).
- **FR-012**: UI MUST render an edit form on the feature detail page (not the list) behind an "Edit" affordance, pre-filled with current title and priority. Source: US-001, human answer.
- **FR-013**: UI MUST disable/hide the Edit affordance when the feature is in a non-editable status, with a tooltip explaining why. Source: US-001, FR-003.
- **FR-014**: UI MUST render a "Delete" button on the feature detail page behind a confirm dialog. Source: US-002, human answer.
- **FR-015**: UI MUST disable the Delete button when the feature is in a non-deletable status (`in_progress`, `waiting_for_feedback`, `gate_blocked`), with a tooltip instructing the user to cancel first. Source: US-002, FR-007.
- **FR-016**: UI MUST navigate to the dashboard after a successful delete. Source: US-002.
- **FR-017**: UI MUST show a success toast after edit save and after delete. Source: US-001, US-002 (consistent with existing toast pattern).
- **FR-018**: UI MUST invalidate the `['features']` and `['feature', id]` react-query caches after edit and delete so the dashboard list refreshes. Source: US-001, US-002.
- **FR-019**: UI MUST render a delete affordance on each deletable feature card on the dashboard behind a confirm dialog. Source: US-003.

### Key Entities

- **Feature** (existing): attributes edited by this feature are `Title` (string, 1-200 chars) and `Priority` (int, 1-3). Immutable fields: `ID`, `IntakePath`, `SpecDir`, `CreatedAt`. Lifecycle: a feature is editable when `Status ∈ {draft, passed, failed, done, recirculated}` (i.e., not actively processing or waiting for feedback); deletable when `Status ∈ {draft, done, cancelled, failed, recirculated, passed}` (not in_progress/waiting_for_feedback/gate_blocked). Editable and deletable state sets are identical per the human answers.

**State-editability matrix**:

| Status | Editable | Deletable | Notes |
|---|---|---|---|
| draft | yes | yes | clean state |
| in_progress | no (409) | no (400) | processing |
| gate_blocked | no (409) | no (400) | must cancel first |
| waiting_for_feedback | no (409) | no (400) | processing/paused |
| passed | yes | yes | between phases |
| failed | yes | yes | |
| recirculated | yes | yes | |
| done | yes | yes | terminal |
| cancelled | yes | yes | terminal |

Invalid edit/delete transitions: any → (edit/delete) while `in_progress`/`waiting_for_feedback`/`gate_blocked` → rejected.

## Success Criteria

- **SC-001**: A user can rename a draft feature and the new title is returned by `GET /api/features/{id}` within one request/response cycle (latency < 200ms on local SQLite).
- **SC-002**: A user can delete a draft feature and it no longer appears in `GET /api/features` (verified by a subsequent list request returning no matching ID).
- **SC-003**: 100% of error paths return the documented HTTP status code and `error` code (400/404/409 with the specific code from FR-002/006/007/008).
- **SC-004**: No JSON response in the new endpoints returns `null` for an array field — empty arrays are `[]` (matches existing convention).
- **SC-005**: Editing or deleting a feature that is mid-processing always fails fast with 409 (never corrupts pipeline state, never orphans a tmux session).

## Assumptions

- [ASSUMPTION: Single-user platform — no optimistic concurrency control on PATCH/DELETE. Last write wins. Acceptable because the pipeline serializes agent dispatch via `activeProcess` sync.Map.]
- [ASSUMPTION: Spec worktree (`~/worktrees/devteam-specs/<id>/`) and `spec/<id>` git branch cleanup on delete is a P2 follow-up, not in this feature's MVP. The DELETE endpoint removes the DB row and cascade-related rows only. A tracked TODO is left for worktree/branch cleanup.]
- [ASSUMPTION: DELETE on a non-existent ID returns 404 (not 204). This distinguishes "never existed" from "already deleted" and aids client error handling.]
- [ASSUMPTION: The editable field set is exactly `{title, priority}` per the human answer. Description is NOT editable (description lives on the intake input artifact, not the feature row — and the feature.go struct has no Description field).]
- [ASSUMPTION: The delete-while-processing guard uses the existing `s.IsProcessing(id)` check (in-memory `activeProcess` sync.Map) AND the dispatcher's `IsSessionAlive(id)` check to catch tmux sessions started by a previous server instance. If neither is available in a path, the status check (FR-007) is the backstop.]
- [ASSUMPTION: The DB layer's existing `UpdateFeature(FeatureRow)` function writes `current_phase`, `status`, `recirculation_count` from the passed row — the PATCH handler must reload the current feature first and only overwrite title/priority to avoid clobbering these. Alternatively, a targeted `UpdateFeatureMetadata(id, title, priority)` DB method is added. Architect decides; the constraint is that PATCH must not change phase/status.]
- [ASSUMPTION: PATCH method is used (not PUT) for partial update semantics consistent with the existing `PATCH /questions/{questionId}` route and the CORS allow-list already includes PATCH.]
- [ASSUMPTION: The confirm dialog is a native `window.confirm` (matches the existing cancel/recirculate confirm pattern in `FeatureDetail.tsx`). A custom modal is P3 polish, out of scope.]

## Constraint Register

This feature does not implement an external protocol/RFC. Constraints are internal-convention-derived (traceable to existing code patterns and human answers).

| ID | Source | Section/Vector | Type | Constraint | Verification Method |
|----|--------|----------------|------|------------|---------------------|
| CON-001 | Human answer | q:editable fields | behavior | PATCH may change only `title` and `priority`; `id` is immutable | AC-001 (PATCH with title+priority updates only those fields; id unchanged) |
| CON-002 | Human answer | q:edit restrictions | behavior | No edits while status `in_progress`/`waiting_for_feedback` | AC-004 (PATCH on in_progress → 409) |
| CON-003 | Human answer | q:edit location | ui | Edit form on feature detail page only, not inline on list | AC-013 (edit form rendered on detail page) |
| CON-004 | Human answer | q:id mutability | behavior | Feature ID never changes | AC-001 (id in response equals URL id) |
| CON-005 | Human answer | q:delete semantics | behavior | Hard delete — row + cascade-related data removed | AC-007 (DELETE → 204; subsequent GET → 404; related rows gone) |
| CON-006 | Human answer | q:delete trigger | ui | Delete button on detail page behind confirm dialog | AC-009 (confirm dialog shown before DELETE) |
| CON-007 | Human answer | q:deletable states | behavior | Only draft or terminal features deletable; in-progress must cancel first | AC-011 (DELETE on in_progress → 400) |
| CON-008 | Existing code | `server.go` createFeature validation | consistency | Title validation: non-empty trim, ≤200 chars; error codes `empty_title`, `title_too_long` | AC-002, AC-003 |
| CON-009 | Existing code | `server.go` createFeature validation | consistency | Priority validation: 1-3; error code `invalid_priority` | AC-005 |
| CON-010 | Existing code | `dto.go` `ErrorResponse`, `writeError` | consistency | Error response shape `{error, details}` via `writeError(w, code, errorCode, details)` | AC-016 (every error response matches shape) |
| CON-011 | Existing code | `dto.go` array init pattern | consistency | JSON arrays never `null` — initialize `[]` | AC-017 (no null arrays in responses) |
| CON-012 | Existing code | `server.go` `IsProcessing` / dispatcher `IsSessionAlive` | safety | DELETE/PATCH on a feature with an active tmux session → 409 to prevent orphaning | AC-006 (DELETE while processing → 409) |
| CON-013 | Existing code | `FeatureDetail.tsx` cancel/recirculate confirm pattern | consistency | Native `window.confirm` for destructive actions | AC-010 |
| CON-014 | Existing code | `api/client.ts` `ApiError` | consistency | UI errors surface via `ApiError` with `status`/`code`/`details`; toasts on failure | AC-015 |
| CON-015 | Existing code | `MaxBytesReader` 1MB on request bodies | consistency | PATCH/DELETE request bodies capped at 1MB | AC-018 |

## Constitution Compliance

No `constitution.md` or `.specify/constitution.md` found in the repo root. No constitution principles to check against. N/A.