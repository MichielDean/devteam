# Changelog — Better Q&A UI (Spec: better-qa-ui)

## [unreleased] - 2026-06-24

### Added — better-qa-ui
- Guided wizard Q&A flow: pending multiple-choice Questions render as selectable option cards (click = select, no immediate submit) (spec better-qa-ui, US-001, CON-001).
- Open-ended Questions (empty `options`) render as a textarea wizard step with the same flow as multiple-choice (spec better-qa-ui, US-004, CON-002).
- Progress indicator showing `X of Y questions answered` across all Questions for a Feature (spec better-qa-ui, US-002, CON-005).
- Auto-scroll to the next pending Question after answering, or to the summary/submit area if none remain (spec better-qa-ui, US-002, CON-006).
- Inline editable answer summary panel listing every Question with its selected/typed answer before submit (spec better-qa-ui, US-003, CON-007).
- Single "Submit Answers & Resume" button sending one PATCH per Question and resuming the pipeline (spec better-qa-ui, US-003, CON-008).
- Phase + role label on every Question card (spec better-qa-ui, US-002, CON-004).
- Toast feedback for 400 `validation_error`, 404 `not_found`, 409 `conflict`, and 500 `internal_error` from the answer endpoint (spec better-qa-ui, US-005, CON-010/011/012, FR-010).
- Restyled answered and assumed history cards consistent with the wizard (checkmark / auto-assumed label) (spec better-qa-ui, US-001, CON-013).
- E2E + integration test suite covering the wizard flow (spec better-qa-ui, US-001..005).

### Changed — better-qa-ui
- Render dispatch for pending Questions is now driven by whether `options` is non-empty, ignoring the `type` field (which remains a display-only badge) (spec better-qa-ui, CON-003).
- Per-question Submit buttons replaced by a single Submit-and-Resume button in the wizard flow (spec better-qa-ui, US-003, CON-008).

### Preserved (unchanged) — better-qa-ui
- `Question` TypeScript interface shape (spec better-qa-ui, CON-014, AC-CON-003).
- Backend answer endpoint `PATCH /api/features/{id}/questions/{questionId}` (spec better-qa-ui, CON-009/010/011/012).
- Backend resume-mode semantics: autopilot auto-resumes; single-phase transitions to `in_progress` awaiting manual advance (spec better-qa-ui, CON-009).
- `question_answered` SSE event + React Query invalidation so answered cards appear in history without a manual refresh (spec better-qa-ui, FR-014).