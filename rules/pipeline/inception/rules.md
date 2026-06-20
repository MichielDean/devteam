# Inception Phase Rules

## Purpose

Define what to build and why, with enough specificity that the Architect can plan and the Tester can verify.

## PM Responsibilities

1. **Intake**: Receive loose ideas and external specs
2. **Explore**: Ask structured questions to resolve ambiguity
3. **Clarify**: Fill gaps, resolve contradictions, define edge cases
4. **Specify**: Produce spec.md, acceptance.md, and repos.yaml

## Spec Requirements

### spec.md must include:
- User scenarios with priorities (P1, P2, P3)
- Functional requirements (FR-001, FR-002, etc.)
- Key entities and relationships
- Success criteria
- Error scenarios (404, 400, 409, 500 responses)
- Empty state behavior
- Assumptions and scope boundaries

### acceptance.md must include:
- Verifiable acceptance criteria in this format:
  ```
  AC-001: [Given precondition], when [action], then [expected result]
    Test level: [smoke | integration | e2e | unit]
    Verification: [specific assertion or scenario]
  ```
- Every user story must have at least one criterion per test level appropriate to the change type
- Error paths and empty states explicitly covered
- No "should work well" or "should be fast" — only "Given X, When Y, Then Z"

### repos.yaml must include:
- Feature ID
- Affected repositories with name, URL, and branch

## Quality Gate

The spec is ready when:
1. Every user story has acceptance criteria with test level and verification method
2. Every functional requirement is testable with specific expected outcomes
3. Error paths and empty states are explicitly covered
4. repos.yaml identifies all affected repositories
5. No [NEEDS CLARIFICATION] markers remain