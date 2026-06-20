# Planning Phase Rules

## Purpose

Design the technical approach with enough specificity that the Developer can implement without making architectural decisions on the fly. Quality starts here — if the plan doesn't specify test strategy and done conditions, the Developer will guess.

## Architect Responsibilities

1. **Validate**: Confirm the spec is technically feasible
2. **Plan**: Create plan.md with technical context and test strategy
3. **Decompose**: Break the spec into implementable tasks in tasks.md with done conditions
4. **Scope**: Identify which repos need changes

## Plan Requirements

### plan.md must include:
- Technical context (language, framework, dependencies)
- Project structure (where files go)
- Data model (entities, relationships)
- API contracts (endpoints, request/response schemas)
- **Test strategy** — what testing levels are required for each component:
  ```
  Component: [name]
  Testing levels required:
    - Smoke: [what to verify on startup]
    - Integration: [what request/response cycles to test]
    - E2E: [what user workflows to test, if UI changes]
    - Unit: [what logic to test in isolation]
  Quality checkpoints:
    - [ ] Service starts without panicking
    - [ ] All API endpoints return expected status codes
    - [ ] JSON arrays are [] not null for empty collections
  ```
- **Agent failure mode checks** — for tasks that AI agents will implement:
  - Does the task produce initialization code? → Check nil pointer ordering
  - Does the task produce JSON serialization? → Check null vs empty arrays
  - Does the task produce HTTP middleware? → Check recovery middleware is first in chain
  - Does the task produce state machine logic? → Check all transitions and invalid transitions

### tasks.md must include:
- Tasks grouped by user story priority
- Exact file paths in each repo
- Dependencies between tasks
- **Done conditions** — not "implement the API" but "implement the API and verify:
  service starts, GET /api/features returns 200, POST /api/features with missing title returns 400,
  JSON arrays are [] not null for empty collections"
- Test level required for each task

## Quality Gate

The plan is ready when:
1. Every task has a specific file path
2. Every task has a done condition with specific verifiable assertions
3. Test strategy section exists for each component
4. Cross-repo boundaries are defined with contracts
5. Dependencies between tasks are explicit