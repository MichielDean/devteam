# Review Phase Rules

## Purpose

Adversarial review against the spec's acceptance criteria AND the constraint register. Find what's wrong, not rubber-stamp. **Every constraint from every standard must be verified by tracing execution paths through the code.**

## Reviewer Responsibilities

1. **Constraint Compliance**: Check implementation against EVERY constraint in the constraint register
2. **Execution Path Tracing**: For each constraint, trace the data path from input to output
3. **Cross-Component Consistency**: Verify producer/consumer agreement across all components
4. **Negative Case Verification**: For every negative test vector, verify the implementation rejects it
5. **Verify**: Check implementation against every acceptance criterion in acceptance.md
6. **Quote Evidence**: For every finding, quote the specific code and the specific criterion/constraint
7. **Security**: Check for common vulnerabilities
8. **Null Safety**: Verify no nil pointer dereferences, no null arrays in JSON
9. **Error Paths**: Verify 400s, 404s, 409s, empty states, malformed input
10. **Middleware Chain**: Verify recovery middleware catches panics, CORS is correct

## Step 0: Constraint Register Review — MANDATORY FIRST STEP

Before reviewing acceptance criteria, read the constraint register from spec.md. Every constraint is a review item with a source (RFC section, test vector, security requirement).

For each constraint, trace the execution path:

```
Constraint: CON-001 — Wire-format failures return Invalid, never throw
Source: RFC 9421 §2.5

Trace:
1. Input: malformed Signature-Input header (e.g., unquoted keyid)
2. Entry point: Rfc9421Verifier.verify() line 95
3. parseSignatureInput() line 100
4. Long.parseLong("created" value) line 105
   - Path A: valid number → continues
   - Path B: "abc" → NumberFormatException → caught? line 108: returns Invalid ✓
   - Path C: null → NPE → caught? NOT CAUGHT → finding! ✗
5. Base64.decode(signature bytes) line 364
   - Path D: valid bytes → continues
   - Path E: malformed → IllegalArgumentException → caught? line 368: returns Invalid ✓

Status: NOT MET — Path C (null "created") throws NPE instead of returning Invalid
Evidence: Rfc9421Verifier.java:105 — no null check before Long.parseLong
```

**Execution path tracing is mandatory.** Reading code and thinking "looks right" is not tracing. You must follow the data through every branch and verify the constraint holds on every path that can reach the constrained behavior.

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

## Step 4: Cross-Component Consistency Review

For features with multiple components (e.g., multiple providers, signer + verifier):

1. Read the architect's cross-component consistency matrix
2. For each shared value, verify the producer and consumer agree
3. **Check ALL producers, not just the first** — if 4 providers emit algorithm identifiers, verify all 4
4. If a constraint applies to "all providers," verify it in ALL of them

Common findings:
- Provider A handles empty bodies, provider B doesn't (same constraint, inconsistent implementation)
- Provider A emits algorithm X, verifier only accepts Y
- Error path in component A uses code X, same error path in component B uses code Y

## Step 5: Negative Test Vector Verification

For every negative test vector in the constraint register:

1. Read the vector's input (e.g., "unquoted keyid param")
2. Trace what the implementation does with that input
3. Verify it rejects with the expected response (not an exception, not acceptance)
4. If the implementation accepts the malformed input or throws, that's a finding

## Step 6: Language-Specific Footgun Review

Check for language-specific pitfalls in the implementation:

- **Java**: `(-x) % 4` returns negative; `String.repeat(n)` throws if n < 0; integer overflow on `int` arithmetic
- **Go**: writing to nil map panics; nil channel blocks forever; interface containing nil isn't nil
- **TypeScript**: `any` type; `==` vs `===`; optional chaining hiding null
- **Python**: mutable default args; `is` vs `==`; `//` vs `/`

If any of these could produce wrong behavior, that's a finding with the specific line and the footgun explanation.

## Step 7: Produce Review Report

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
1. **Every constraint in the register has been checked with execution path trace and quoted evidence**
2. Every acceptance criterion has been checked with quoted evidence
3. **Every negative test vector has been verified** — implementation rejects each with correct response
4. **Cross-component consistency verified** — all shared values agree across ALL producers and consumers
5. "No issues found" includes evidence of what was verified
6. Security review is complete (if priority-1 feature)
7. Null pointer safety verified
8. Error paths verified — including malformed input, empty state, and all error codes from the standard's taxonomy
9. Middleware chain verified end-to-end
10. Over-engineering check completed
11. Missing implementation check completed
12. **Language-specific footguns checked** — modulo, nil maps, negative repeat, overflow
13. **Execution paths traced** — review includes input-to-output traces for each constraint
14. **Multi-component constraints verified across ALL components** — not just the first