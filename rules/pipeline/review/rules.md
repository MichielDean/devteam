# Review Phase Rules

## Purpose

Adversarial review against the spec's acceptance criteria. Find what's wrong, not rubber-stamp.

## Reviewer Responsibilities

1. **Verify**: Check implementation against every acceptance criterion in acceptance.md
2. **Quote Evidence**: For every finding, quote the specific code and the specific criterion
3. **Security**: Check for common vulnerabilities
4. **Null Safety**: Verify no nil pointer dereferences, no null arrays in JSON
5. **Error Paths**: Verify 400s, 404s, 409s, empty states
6. **Middleware Chain**: Verify recovery middleware catches panics, CORS is correct

## Step 1: Spec Review — Compare Plan Against Spec

Before reviewing code, verify the plan matches the spec:

1. Does every user story in the spec have corresponding tasks in tasks.md?
2. Does every acceptance criterion have a done condition?
3. Are there tasks in the plan that don't trace to any user story? (Scope creep)
4. Are there user stories with no corresponding tasks? (Missing implementation)

Document any gaps. If the plan doesn't cover a user story, that's a finding.

## Step 2: Code Review — Verify Implementation Against Plan

For each task in tasks.md:

1. **Find the code**: Open the files specified in the task
2. **Check done conditions**: Verify each done condition is met with specific evidence
3. **Check for over-engineering**: Is the implementation the minimum needed, or is there scope creep?
4. **Check for under-engineering**: Is anything in the spec not implemented?

### Review Format

Each finding must include:
- **Criterion**: The acceptance criterion being checked (e.g., "AC-003")
- **Evidence**: Quoted code with file path and line number
- **Status**: MET or NOT MET
- **Explanation**: How the code satisfies (or fails) the criterion

### Key Checks

#### Null Pointer Safety
- Every handler that dereferences a pointer: verify the pointer is initialized
- Every struct field accessed in middleware: verify it's set before middleware wraps it
- Every map access: verify key exists or handle missing key

#### JSON Serialization
- Every slice/map field in API response structs: verify it's [] not null when empty
- Check for `omitempty` on collection fields — this is almost always wrong for API responses

#### Error Path Coverage
- 404 for missing resources
- 400 for invalid input
- 409 for conflicts (e.g., already processing)
- 500 recovery from panics

#### Middleware Chain
- Recovery middleware is outermost (catches panics in all inner handlers)
- CORS middleware is present and correct
- Request body size limits are set

#### Over-Engineering Check
- Is the implementation significantly larger than the plan anticipated?
- Are there features implemented that weren't in the spec?
- Are there abstractions, patterns, or infrastructure that the spec didn't require?
- Line count: if a simple API endpoint is 500+ lines, something's wrong
- If you find over-engineering, flag it as a finding: "Implementation is N lines for task T-XXX, expected ~M lines"

#### Missing Error Paths
- For every endpoint, verify error responses for:
  - Missing required fields → 400
  - Invalid input types → 400
  - Resource not found → 404
  - Conflict (duplicate) → 409
  - Internal errors → 500 (with recovery middleware catching panics)
- Verify empty state returns 200 with [] or {}, not 404

#### State Machine Verification
- If the feature has state transitions, verify:
  - All valid transitions are implemented
  - All invalid transitions are rejected
  - State is persisted correctly
  - Concurrent access doesn't corrupt state

## Step 3: Security Review (Mandatory for P1, Recommended for P2)

For priority-1 features, perform a security review:

- Authentication: Is auth middleware applied to protected endpoints?
- Authorization: Are role checks present? Can user A access user B's resources?
- Input validation: Is every user input validated for type, length, and characters?
- Output filtering: Are internal fields excluded from API responses?
- Error messages: Do errors reveal internal details (stack traces, file paths)?
- CORS: Is it restrictive (specific origins), not `*`?
- Rate limiting: Are sensitive endpoints rate-limited?
- Logging: Are secrets excluded from logs?

## Step 4: Produce Review Report

The review report MUST include:

1. **Per-criterion analysis**: Every acceptance criterion from acceptance.md, with MET or NOT MET status and quoted evidence
2. **Findings**: Any issues discovered, with specific code references and line numbers
3. **Over-engineering findings**: If implementation is significantly larger than expected
4. **Missing implementation**: Any spec requirements not implemented
5. **Security findings** (if P1): Authentication, authorization, input validation, etc.

### Review Report Template

```markdown
# Review Report

## Summary
- Acceptance criteria: X total, Y MET, Z NOT MET
- Findings: A critical, B required, C noted

## Acceptance Criteria Review

### AC-001: [criterion text]
- **Status**: MET
- **Evidence**: `server.go:142` implements the endpoint, `server_test.go:45` verifies 200 response

### AC-002: [criterion text]
- **Status**: NOT MET
- **Evidence**: No implementation found for [specific behavior]
- **Explanation**: The endpoint returns 500 for [scenario] instead of the expected 400

## Findings

### F-001: [finding title]
- **Severity**: [needs fixing / doesn't need fixing]
- **Criterion**: AC-003
- **Code**: `server.go:89-95`
- **Description**: [what's wrong and what needs to change]
```

## Quality Gate

Review is complete when:
1. Every acceptance criterion has been checked with quoted evidence
2. "No issues found" includes evidence of what was verified
3. Security review is complete (if priority-1 feature)
4. Null pointer safety verified
5. Error paths verified
6. Middleware chain verified end-to-end
7. Over-engineering check completed
8. Missing implementation check completed