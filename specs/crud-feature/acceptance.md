# Acceptance Criteria — Feature Edit & Delete (crud-feature)

Every criterion is Given/When/Then with a test level and a specific verification assertion. Constraints (CON-###) from spec.md are referenced.

## US-001 — Edit title and priority

AC-001: Given a feature with id "f-001" exists with title "Old" and priority 2, when a PATCH /api/features/f-001 is sent with body `{"title":"New","priority":1}`, then the response is 200 with `title="New"`, `priority=1`, and `id="f-001"` (id unchanged), and `updated_at > previous updated_at`.
  Test level: integration
  Verification: httptest POST to create feature, PATCH with new values, GET /api/features/f-001, assert title/priority/updated_at. (CON-001, CON-004)

AC-002: Given the edit endpoint is available, when a PATCH /api/features/{id} is sent with `{"title":"  "}` (whitespace only), then the response is 400 with `error="empty_title"` and `details` non-empty, and the feature's title is unchanged.
  Test level: integration
  Verification: httptest PATCH with whitespace title, assert 400 and error body, then GET confirms original title. (CON-008)

AC-003: Given a feature exists, when a PATCH is sent with `{"title":"`+strings.Repeat("x",201)+`"}`, then the response is 400 with `error="title_too_long"`.
  Test level: integration
  Verification: httptest PATCH with 201-char title, assert 400 and error code. (CON-008)

AC-004: Given a feature with status `in_progress` exists, when a PATCH /api/features/{id} is sent with any body, then the response is 409 with `error="feature_processing"`.
  Test level: integration
  Verification: httptest — insert/seed a feature with status in_progress (and not in activeProcess), PATCH, assert 409 and error code. (CON-002)

AC-005: Given a feature exists, when a PATCH is sent with `{"priority":4}`, then the response is 400 with `error="invalid_priority"`.
  Test level: integration
  Verification: httptest PATCH with priority 4, assert 400 and error code. (CON-009)

AC-006: Given a feature exists with title "A" and priority 3, when a PATCH is sent with `{"title":"B"}` (priority omitted), then the response is 200 with `title="B"` and `priority=3` (priority unchanged); and vice versa for priority-only PATCH.
  Test level: integration
  Verification: httptest two PATCHes (title-only, then a fresh feature priority-only), assert unchanged field retains prior value. (CON-001)

AC-007: Given a feature with status `waiting_for_feedback` exists, when a PATCH is sent, then the response is 409 `feature_processing` (waiting_for_feedback is a non-editable processing state).
  Test level: integration
  Verification: httptest seed feature with status waiting_for_feedback, PATCH, assert 409. (CON-002)

AC-008: Given no feature with id "ghost" exists, when a PATCH /api/features/ghost is sent, then the response is 404 with `error="feature_not_found"`.
  Test level: integration
  Verification: httptest PATCH to unknown id, assert 404 and error code. (CON-010)

## US-002 — Delete a feature

AC-009: Given a feature with id "f-002" and status `draft` exists, when a DELETE /api/features/f-002 is sent, then the response is 204 with no body, and a subsequent GET /api/features/f-002 returns 404 `feature_not_found`.
  Test level: integration
  Verification: httptest create feature, DELETE, assert 204, then GET and assert 404. (CON-005)

AC-010: Given a feature with id "f-003" and status `in_progress` exists, when a DELETE /api/features/f-003 is sent, then the response is 400 with `error="not_deletable"` and details instructing the user to cancel first.
  Test level: integration
  Verification: httptest seed feature with status in_progress (not in activeProcess), DELETE, assert 400 and error code. (CON-007)

AC-011: Given a feature with id "f-004" and status `waiting_for_feedback` exists, when a DELETE is sent, then the response is 400 `not_deletable`.
  Test level: integration
  Verification: httptest seed, DELETE, assert 400. (CON-007)

AC-012: Given a feature with id "f-005" and status `done` exists, when a DELETE is sent, then the response is 204 and the feature is removed (terminal features are deletable).
  Test level: integration
  Verification: httptest seed done feature, DELETE, assert 204, GET → 404. (CON-007)

AC-013: Given no feature with id "ghost" exists, when a DELETE /api/features/ghost is sent, then the response is 404 `feature_not_found`.
  Test level: integration
  Verification: httptest DELETE unknown id, assert 404. (CON-010)

AC-014: Given a feature with id "f-006" exists and is being processed (`s.IsProcessing("f-006")` returns true, e.g., an active tmux session), when a DELETE /api/features/f-006 is sent, then the response is 409 `feature_processing`.
  Test level: integration
  Verification: httptest with a stub server whose activeProcess is pre-loaded for "f-006" (or mock dispatcher.IsSessionAlive → true), DELETE, assert 409. (CON-012)

AC-015: Given a feature with id "f-007" and status `draft` exists, when a DELETE is sent, then all related rows (phase_states, questions, notes, sessions, recirculations, events, artifacts, gate_results) for f-007 are removed (cascade verified).
  Test level: integration
  Verification: httptest create feature with a related question/note, DELETE, then query each related table for f-007 and assert zero rows. (CON-005)

## US-001 / US-002 — UI behavior

AC-016: Given the feature detail page is rendered for a draft feature, when the user clicks the "Edit" button, then an edit form appears on the detail page (not a separate route) with title and priority pre-filled, and a Save and Cancel button.
  Test level: e2e
  Verification: Playwright — navigate to feature detail, click [data-testid="edit-button"], assert [data-testid="edit-form"] visible with inputs containing current title and priority. (CON-003)

AC-017: Given the edit form is open, when the user changes the title to "Renamed" and clicks Save, then a PATCH request is observed (network), the form closes, the detail header [data-testid="feature-title"] shows "Renamed", and a success toast appears.
  Test level: e2e
  Verification: Playwright — fill title input, click [data-testid="edit-save"], assert [data-testid="feature-title"] text="Renamed" and [data-testid="toast-success"] visible. (CON-014)

AC-018: Given the edit form is open, when the user clicks Cancel, then the form closes, no PATCH request is sent (assert no /api/features PATCH in network log), and the original title remains.
  Test level: e2e
  Verification: Playwright — click [data-testid="edit-cancel"], assert form hidden and title unchanged, assert network has no PATCH. (CON-013)

AC-019: Given the feature detail page is rendered for a draft feature, when the user clicks the "Delete" button, then a confirm dialog appears with text including the feature's title.
  Test level: e2e
  Verification: Playwright — click [data-testid="delete-button"], assert window.confirm dialog text contains the feature title. (CON-006, CON-013)

AC-020: Given the confirm dialog is open, when the user accepts it, then a DELETE request is sent, the response is 204, and the browser navigates to "/" (dashboard).
  Test level: e2e
  Verification: Playwright — accept dialog, assert URL pathname is "/" and the deleted feature id does not appear in the dashboard list. (CON-006)

AC-021: Given the confirm dialog is open, when the user dismisses it, then no DELETE request is sent and the user remains on the detail page.
  Test level: e2e
  Verification: Playwright — dismiss dialog, assert URL unchanged and no DELETE in network log. (CON-013)

AC-022: Given the feature detail page is rendered for an in_progress feature, when the user inspects the Edit button, then it is disabled and has a tooltip explaining edits are blocked while processing.
  Test level: e2e
  Verification: Playwright — navigate to in_progress feature detail, assert [data-testid="edit-button"] is disabled and has a `title` attribute mentioning processing. (CON-002)

AC-023: Given the feature detail page is rendered for an in_progress feature, when the user inspects the Delete button, then it is disabled with a tooltip instructing the user to cancel first.
  Test level: e2e
  Verification: Playwright — assert [data-testid="delete-button"] disabled and `title` mentions cancelling. (CON-007)

AC-024: Given a feature detail page for a done feature, when the user deletes it (confirm), then a success toast appears and the dashboard no longer lists it.
  Test level: e2e
  Verification: Playwright — delete done feature, assert [data-testid="toast-success"], assert dashboard list excludes the id. (CON-014)

## US-003 — Delete from list

AC-025: Given the dashboard lists a deletable (draft) feature card, when the user clicks the delete affordance on that card and confirms, then the card is removed from the list without navigation.
  Test level: e2e
  Verification: Playwright — on dashboard, click [data-testid="feature-card-delete"] for a draft feature, accept confirm, assert the card with that id is gone from the DOM. (CON-006)

## Cross-cutting / conformance

AC-026: Given any error response from PATCH or DELETE, when the response body is parsed, then it matches `{"error": string, "details": string}` (ErrorResponse shape) and never contains a stack trace or internal file path.
  Test level: integration
  Verification: httptest across all error cases (400/404/409), assert JSON shape and that `details`/`error` contain no `/` path separators or stack frames. (CON-010)

AC-027: Given any successful response from PATCH that includes array fields (e.g., dependencies, repos), when serialized, then arrays are `[]` not `null` (verify the existing `FeatureToDetailResponse` nil→[] init is preserved).
  Test level: integration
  Verification: httptest PATCH a feature with no deps/repos, assert response `dependencies` is `[]` and `repos` is `[]`. (CON-011)

AC-028: Given a PATCH or DELETE request with a body larger than 1MB, when received by the server, then the request is rejected (413 or 400 per existing MaxBytesReader behavior) — verify the handler applies `http.MaxBytesReader`.
  Test level: integration
  Verification: httptest send a >1MB PATCH body, assert the request fails (not accepted). (CON-015)

## Smoke (mandatory gate coverage)

AC-029: Given the devteam service is running, when a smoke test hits PATCH /api/features/{existing-draft-id} with a valid body, then the response is 200 and the service does not panic.
  Test level: smoke
  Verification: start service, PATCH a seeded feature, assert 200 and no panic in logs. (CON-010)

AC-030: Given the devteam service is running, when a smoke test hits DELETE /api/features/{existing-deletable-id}, then the response is 204 and the service does not panic.
  Test level: smoke
  Verification: start service, DELETE a seeded feature, assert 204 and no panic in logs. (CON-010)