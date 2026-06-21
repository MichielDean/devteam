# Implementation Plan: Feature Spec Count Badge

**Branch**: `feature-spec-count-badge---show-total-count-of-feature-specs` | **Date**: 2026-06-20 | **Spec**: [specs/feature-spec-count-badge---show-total-count-of-feature-specs/spec.md](../specs/feature-spec-count-badge---show-total-count-of-feature-specs/spec.md)

**Input**: Feature specification from `specs/feature-spec-count-badge---show-total-count-of-feature-specs/spec.md`

## Summary

Add a `total_count` integer field to the existing `GET /api/features` response (computed as `len(features)` inside the existing `FeaturesToSummaryResponse` helper) and render a display-only count badge next to the "Features" heading on the Dashboard. No new endpoints, no new persistence, no new state machine, no new queries. Single repo (devteam). Two layers: Go backend DTO + React/TypeScript frontend.

This is a trivial additive change. The plan is deliberately minimal — it matches the spec's scope boundaries. Any addition beyond what is listed here is over-engineering and must be rejected at review.

## Technical Context

**Language/Version**: Go 1.23+ (backend), TypeScript 5.8 + React 19 (frontend)

**Primary Dependencies (existing, unchanged)**:
- Backend: `encoding/json`, `net/http` (stdlib only). No new Go dependencies.
- Frontend: `@tanstack/react-query` (existing list query), `react`, `tailwindcss` (existing badge styling language).
- Testing: Go stdlib `testing` + `net/http/httptest`; Playwright for E2E.

**Storage**: None new. `total_count` is derived per-request from the in-memory `features` slice returned by `Pipeline.ListFeatures()`. No persistence, no migration, no cache.

**Testing**: Go `go test ./internal/api/...` for backend; `npm run test:e2e` (Playwright) for frontend E2E; `tsc --noEmit` for type-level checks.

**Target Platform**: Linux (primary). No platform-specific behavior in this change.

**Project Type**: Brownfield enhancement to an existing Go server + Vite SPA.

**Performance Goals**: None. `len()` of an already-computed slice adds no measurable latency. NFR-001 in spec confirms this.

**Constraints**: Backward-compatible at the API contract level — existing clients that ignore unknown fields are unaffected. Frontend degrades gracefully when `total_count` is absent (NFR-005). No new endpoints. No new auth surface.

**Scale/Scope**: Single repo. 4 files modified, 0 created (tests extend existing files). Touched files: `internal/api/dto.go`, `internal/api/server_test.go`, `ui/src/types/index.ts`, `ui/src/pages/Dashboard.tsx`, plus one new E2E block in `ui/e2e/app.spec.ts`.

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Spec-Driven, Always | PASS | spec.md + acceptance.md + repos.yaml exist and are the input to this plan |
| II. Six Roles, Fixed Pipeline | N/A | This feature does not change the pipeline |
| III. Central Spec, Distributed Implementation | PASS | Single repo (devteam); repos.yaml confirms |
| IV. Two Intake Paths, One Output | N/A | No intake change |
| V. Proof-of-Work Gates | N/A | No pipeline gate change |
| VI. Cross-Repo Coherence | PASS | No cross-repo coordination — single repo declared in repos.yaml |
| VII. Self-Bootstrap | N/A | Enhancement to existing platform |
| VIII. Go, Minimal Dependencies | PASS | No new Go dependencies; stdlib only |
| IX. AIDLC Phase Governance | PASS | Plan follows planning rules (test strategy + done conditions) |
| X. Learn From Cistern | PASS | Conservative scope, mechanical verification |

## Spec Validation

**Completeness check**: All 10 functional requirements (FR-001..FR-010) trace to US-1 or US-2. All acceptance criteria (AC-001..AC-015) trace to a user story and specify a test level. PASS.

**Consistency check**: No contradictions found. FR-004 (`total_count == len(features)`) and edge case #5 ("features empty but total_count non-zero must never happen") are consistent — both assert the count is derived from the same slice. PASS.

**Feasibility check**: `FeaturesToSummaryResponse` (`internal/api/dto.go:89`) already iterates `features` and builds `summaries`. Adding `"total_count": len(summaries)` to the returned map is a one-line change with no architectural risk. Frontend `Dashboard.tsx:36` already reads `data?.features`; adding `data?.total_count` follows the same pattern. PASS.

**Edge case check**: Spec covers empty state (edge #1), single feature (#2), large lists (#3), backend error (#4), defensive missing-field (#6), network error (#7), concurrent mutation (#8), cancelled features (#9). All have corresponding acceptance criteria. PASS.

**Ambiguity check**: All assumptions in spec are marked `[ASSUMPTION: ...]`. No `[NEEDS CLARIFICATION]` markers remain. PASS.

## Architecture

This feature does not introduce new components. It modifies two existing components. There is no new service layer, no new component boundary, no new dependency.

### Component: Backend DTO builder (`FeaturesToSummaryResponse`)

**Purpose**: Transform a `[]*feature.Feature` slice into the JSON response map for `GET /api/features`.

**Responsibilities (after change)**:
- Build the `features` array of `FeatureSummaryResponse` (unchanged).
- Add a top-level `total_count: int` key equal to `len(summaries)` (NEW).

**Interfaces**:
- `FeaturesToSummaryResponse(features []*feature.Feature, questionStore feature.QuestionStore) map[string]interface{}` — signature unchanged; only the returned map gains a key.

**Dependencies**:
- Depends on `feature.Feature` and `feature.QuestionStore` (unchanged).

**Design decision**: Use `len(summaries)`, NOT `len(features)`. `summaries` is the slice actually serialized into the response, so the count must reflect what the client sees. Since every input feature produces exactly one summary, the two lengths are always equal — but using `len(summaries)` makes the invariant self-evident and robust to future filtering. This directly satisfies FR-004 and prevents edge case #5 from ever occurring by construction.

### Component: Frontend Dashboard (`Dashboard.tsx`)

**Purpose**: Render the features list page header with a count badge.

**Responsibilities (after change)**:
- Render the "Features" heading (unchanged).
- Render a display-only badge adjacent to the heading showing `total_count` (NEW).
- Degrade safely when `total_count` is missing (render "0" or hide the badge).

**Interfaces**:
- Consumes `FeatureListResponse` from `listFeatures()` (modified type — gains `total_count: number`).

**Dependencies**:
- Depends on `react-query` useQuery result `data` (unchanged).
- Depends on `FeatureListResponse` type (modified).

**Design decision**: The badge is a non-interactive `<span>` (FR-010: no click handler, no link). It uses `aria-label="Total features: N"` for accessibility (NFR-003). It uses `data-testid="feature-count-badge"` for E2E targeting (matches AC-001 verification). When `data?.total_count` is `undefined` (older backend), it defaults to `0` via `?? 0` (FR-009, AC-005). It is rendered inside the existing header `<div>` so it does not cause layout shift (NFR-002 — min-width accommodates 3 digits via `min-w-[2.5rem]` or equivalent inline-block styling).

### Component Dependency Map

```
Dashboard.tsx  ──reads──▶  FeatureListResponse (TS type)
        │                          ▲
        └──calls──▶ listFeatures() ┘
                            │
                            ▼
                  GET /api/features  ──served by──▶  listFeatures handler
                                                            │
                                                            ▼
                                                FeaturesToSummaryResponse (Go)
                                                            │
                                                returns map with "features" + "total_count"
```

No circular dependencies. No new dependencies. The only dependency direction is frontend → API → DTO builder.

## Data Model

No new entities. No new relationships. No new state transitions. One existing response shape gains a field.

### Modified Entity: `GET /api/features` response

```
FeatureListResponse (HTTP response, untyped map in Go)
├── features: []FeatureSummaryResponse  (unchanged)
└── total_count: int                   (NEW — equals len(features))
```

**Integrity rules**:
- `total_count` is REQUIRED on every 200 response (FR-002), including empty state (`total_count: 0`).
- `total_count` MUST NOT appear on error responses (FR-003). Error responses use the existing `{"error": "...", "details": "..."}` shape.
- `total_count === len(features)` always, by construction (FR-004). There is no code path that can violate this because both values come from the same slice.
- `features` serializes as `[]` not `null` on empty state (FR-005 — existing behavior, preserved).

### State Transitions

None. `total_count` is derived per-request. No lifecycle, no persistence, no migration.

## API Contracts

### `GET /api/features` (modified)

**Request**: No request body. No query parameters added.

**Response 200** (modified — one new field):
```json
{
  "features": [
    {
      "id": "string",
      "title": "string",
      "status": "string",
      "priority": 1,
      "current_phase": "string",
      "updated_at": "2026-06-20T12:00:00Z",
      "gate_result": null,
      "pending_questions_count": 0
    }
  ],
  "total_count": 0
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `features` | array | YES (always present, `[]` when empty) | Per-feature summaries. Unchanged. |
| `total_count` | int | YES (always present on 200) | `len(features)`. NEW. |

**Response 500** (unchanged — no `total_count` field):
```json
{ "error": "internal_error", "details": "Failed to list features" }
```

**Other responses**: No other status codes are possible from this endpoint (it takes no input and reads local state). The 500 path is the only error path and is unchanged by this feature.

**Backward compatibility**: Adding a field to a JSON object is backward-compatible for clients that ignore unknown fields (the common case). The only risk is a client that does strict schema validation rejecting unknown keys — this is not how the existing frontend (`listFeatures` decodes into a typed interface that ignores extra keys) or any known consumer behaves. NFR-005 documents this.

## Non-Functional Requirements Design

### Performance (NFR-001, NFR-004)
No design needed. `len(summaries)` is O(1). Response size grows by ~15 bytes. No caching, no batching, no pagination introduced.

### Accessibility (NFR-003)
Badge must have an accessible name. Design: `aria-label="Total features: {count}"` on the badge `<span>`. This is readable by screen readers and satisfies AC-007. The badge is not focusable (it's a `<span>`, not a `<button>` or `<a>`), consistent with FR-010 (non-interactive).

### Layout stability (NFR-002)
Badge styling must not cause layout shift on first paint. Design: use `inline-flex` with a minimum width that accommodates 3 digits (e.g., `min-w-[2.5rem]`). The badge renders only after the query resolves (during loading, the header shows just the heading + button — same as today), so there is no shift from 0 → N because the badge appears with the rest of the loaded content. The min-width prevents reflow as the count grows from 1 to 3+ digits.

### Security
No new attack surface. `total_count` is output-only, derived from existing data already in the response. The spec's Security section confirms: no new inputs, no new endpoints, no new auth surface, no new information disclosed (count is inferable from `len(features)` today). No security acceptance criteria required (per spec assumption). No input validation needed (no input).

### Resiliency
No new failure modes. The badge degrades safely when `total_count` is missing (defaults to 0, FR-009). When the API errors, the existing error path applies (badge not rendered, AC-006). No retry, no circuit breaker needed — the list query already has React Query's existing retry/invalidation behavior.

## Test Strategy

### Component: Backend DTO builder (`FeaturesToSummaryResponse`)

Testing levels required:
- **Smoke**: Service starts and `GET /api/features` returns 200 with a parseable body.
- **Integration**: `GET /api/features` returns `total_count` equal to the `features` array length for N in {0, 1, 5, 50}. Empty state returns `total_count: 0` and `features: []` (not null). Error path (500) returns no `total_count` field.
- **Unit**: Direct call to `FeaturesToSummaryResponse` with a known input slice asserts the returned map has `total_count == len(summaries)`.

Quality checkpoints:
- [ ] `go build ./...` succeeds (smoke)
- [ ] `go test ./internal/api/ -run TestListFeatures -v` passes (integration)
- [ ] `TestListFeaturesEmpty` asserts `resp["total_count"] == 0` (integration — existing test extended)
- [ ] A populated-list test asserts `resp["total_count"] == N` for N > 0 (integration — new or existing test extended)
- [ ] Response body on 200 contains the substring `"total_count"` (integration)
- [ ] Response body on 500 does NOT contain `"total_count"` (integration — verify error path isolation)
- [ ] `features` serializes as `[]` not `null` when empty (integration — regression guard, existing test extended)

### Component: Frontend types (`FeatureListResponse`)

Testing levels required:
- **Unit (type-level)**: `tsc --noEmit` passes with `total_count: number` declared on `FeatureListResponse`.

Quality checkpoints:
- [ ] `ui/src/types/index.ts` declares `total_count: number` on `FeatureListResponse`
- [ ] `npm run build` (which runs `tsc -b && vite build`) succeeds
- [ ] No TypeScript errors reference `total_count`

### Component: Frontend Dashboard badge (`Dashboard.tsx`)

Testing levels required:
- **Smoke**: Dashboard page loads without console errors; badge element exists in the DOM.
- **E2E**: Badge text matches `total_count` for N in {0, 5}. Badge has `aria-label` matching `/Total features: \d+/`. Defensive default when `total_count` missing (intercepted response) — no crash, no console error. Error path (500 intercepted) — badge not rendered, error element visible. Loading state — no `NaN`/`undefined` badge.

Quality checkpoints:
- [ ] Dashboard renders a `[data-testid="feature-count-badge"]` element
- [ ] Badge text equals `String(total_count)` from the API response
- [ ] Badge has `aria-label` matching `/Total features: \d+/`
- [ ] When `total_count` is absent (intercepted), page does not crash and no console error
- [ ] When API returns 500 (intercepted), badge is absent and `features-error` is visible
- [ ] No console errors on any path (happy, empty, missing-field, error)
- [ ] `npm run test:e2e` passes the new badge tests

### Test Level Selection Matrix (applied)

| What changed | Smoke | Integration | E2E | Unit |
|---|---|---|---|---|
| `FeaturesToSummaryResponse` (HTTP DTO) | YES | YES | — | YES |
| `Dashboard.tsx` (UI component) | YES | YES | YES | YES (type-level) |

## Agent Failure Mode Checks

These are the systematic LLM-generated-code failure modes the Developer and Reviewer must watch for, specific to this feature:

1. **Null vs empty array** — The #1 agent serialization bug. `FeaturesToSummaryResponse` currently returns `map[string]interface{}{"features": summaries}` where `summaries` is a `[]FeatureSummaryResponse` initialized with `make([]FeatureSummaryResponse, 0, len(features))`. This already serializes as `[]` not `null`. The Developer MUST NOT change this to a nil slice or add `omitempty`. The new `total_count` key is an int (zero value `0`, no `omitempty`), so it always serializes. Check: `grep -n "omitempty" internal/api/dto.go` must NOT show `total_count` with `omitempty`.

2. **Phantom methods** — Agent might invent a `s.pipeline.CountFeatures()` or `s.pipeline.TotalCount()` method that does not exist. The count MUST come from `len(summaries)` inside `FeaturesToSummaryResponse`, not from a new pipeline method. Check: no new methods added to `*pipeline.Pipeline`.

3. **Over-engineering** — Agent might add pagination, filtering, a separate `/api/features/count` endpoint, a React context for the count, a custom hook, or a memoized selector. NONE of these are in scope. Check: diff is small (target <30 lines of production code across both layers). If the diff exceeds ~50 lines of production code, the Reviewer must flag it.

4. **Initialization ordering / nil pointer** — Not applicable. No new struct fields, no new initialization. The existing `FeaturesToSummaryResponse` is called after `ListFeatures` succeeds; the error path returns before the DTO builder runs.

5. **Middleware chain** — Not applicable. No new middleware. Existing chain (`recoveryMiddleware(corsMiddleware(mux))`) is unchanged.

6. **State machine logic** — Not applicable. No state machine touched.

7. **Frontend defensive default** — Agent might render `NaN` or `undefined` if it reads `data.total_count` without a fallback. The Developer MUST use `data?.total_count ?? 0` (or equivalent) so a missing field renders `0`, never `NaN`. Check: the badge renders `String(data?.total_count ?? 0)` or equivalent.

## Quality Gate (Plan Readiness)

| # | Criterion | Status |
|---|---|---|
| 1 | Every task has a specific file path | PASS — see tasks.md |
| 2 | Every task has a done condition with specific verifiable assertions | PASS — see tasks.md |
| 3 | Every task specifies the required test level | PASS — see tasks.md |
| 4 | Cross-repo boundaries are defined with contracts | PASS — single repo, no cross-repo |
| 5 | Dependencies between tasks are explicit | PASS — T-002 depends on T-001 |
| 6 | The Developer can start without asking "where does this go?" | PASS — exact file paths and line anchors given |
| 7 | Test strategy section exists with testing levels per component | PASS — see above |
| 8 | Quality checkpoints exist at task boundaries | PASS — see tasks.md checkpoints |
| 9 | Agent failure mode checks specified for AI-implemented tasks | PASS — see above |
| 10 | Constitution principles honored | PASS — see Constitution Check |

## Quickstart Guide for the Developer

1. **Read first**: `specs/feature-spec-count-badge---show-total-count-of-feature-specs/spec.md` (the what/why) and `acceptance.md` (the verification criteria). This plan is the how.

2. **Order of work**:
   - **T-001 (backend, ~5 lines)**: Modify `internal/api/dto.go` `FeaturesToSummaryResponse` to add `"total_count": len(summaries)` to the returned map. Extend `internal/api/server_test.go` `TestListFeaturesEmpty` to assert `resp["total_count"] == 0`. Add a populated-list assertion (extend `TestSmokeCreateAndGetFeature` or add a new test) that `total_count == N` after creating N features.
   - **T-002 (frontend, ~15 lines)**: Modify `ui/src/types/index.ts` `FeatureListResponse` to add `total_count: number`. Modify `ui/src/pages/Dashboard.tsx` to render the badge `<span data-testid="feature-count-badge" aria-label={...}>` next to the "Features" heading, reading `data?.total_count ?? 0`. Add E2E tests in `ui/e2e/app.spec.ts` for badge rendering (happy, empty, missing-field, error, aria-label).

3. **Build / test commands**:
   - Backend: `go build ./...` then `go test ./internal/api/ -v`
   - Frontend types: `npm run build` (runs `tsc -b`)
   - Frontend E2E: `npm run test:e2e` (requires the dev server running — see `playwright.config.ts` for baseURL)

4. **Self-verification before declaring done**:
   - Start the server (`go run ./cmd/devteam` or the configured run command) and `curl http://localhost:<port>/api/features | jq '.total_count, (.features | length)'` — the two values must be equal.
   - Load the dashboard in a browser; verify the badge shows the count and has an `aria-label`.
   - Check the browser console for errors on load, on empty state, and on API error (use DevTools network throttling / blocking to simulate).

5. **Do NOT**:
   - Add pagination, filtering, or sorting (out of scope — spec is explicit).
   - Add a new endpoint (spec says "No new endpoints needed").
   - Add a new pipeline method (use `len(summaries)` inside the existing DTO builder).
   - Add `omitempty` to `total_count` (it must always serialize, even when 0).
   - Add a click handler or link to the badge (FR-010: display-only).
   - Add a separate React component file for the badge (it's 1 element — inline it in the header `<div>`).
   - Add real-time SSE updates for the count (out of scope — spec is explicit).

## Open Questions

None. The spec resolved all ambiguities via documented assumptions. No design decisions required human input — all choices are conservative and follow existing conventions.