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

## Review Format

Each finding must include:
- **Criterion**: The acceptance criterion being checked (e.g., "AC-003")
- **Evidence**: Quoted code with file path and line number
- **Status**: MET or NOT MET
- **Explanation**: How the code satisfies (or fails) the criterion

## Key Checks

### Null Pointer Safety
- Every handler that dereferences a pointer: verify the pointer is initialized
- Every struct field accessed in middleware: verify it's set before middleware wraps it
- Every map access: verify key exists or handle missing key

### JSON Serialization
- Every slice/map field in API response structs: verify it's [] not null when empty
- Check for `omitempty` on collection fields — this is almost always wrong for API responses

### Error Path Coverage
- 404 for missing resources
- 400 for invalid input
- 409 for conflicts (e.g., already processing)
- 500 recovery from panics

### Middleware Chain
- Recovery middleware is outermost (catches panics in all inner handlers)
- CORS middleware is present and correct
- Request body size limits are set

## Quality Gate

Review is complete when:
1. Every acceptance criterion has been checked with quoted evidence
2. "No issues found" includes evidence of what was verified
3. Security review is complete (if priority-1 feature)
4. Null pointer safety verified
5. Error paths verified
6. Middleware chain verified end-to-end