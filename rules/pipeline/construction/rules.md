# Construction Phase Rules

## Purpose

Implement the plan, following the task breakdown, writing code that matches the spec's acceptance criteria. Verify before marking complete.

## Developer Responsibilities

1. **Implement**: Write code following tasks.md
2. **Self-verify**: Before marking a task complete, verify locally
3. **Cross-repo**: Implement coherently across repos
4. **Constitution**: Follow project coding standards

## Step 1: Load Context

Before writing any code, read the full context:

1. **Spec**: Read spec.md and acceptance.md — understand what you're building and why
2. **Plan**: Read plan.md — understand the technical approach and test strategy
3. **Tasks**: Read tasks.md — understand what you need to implement and in what order
4. **Existing code** (brownfield): Read the existing codebase — understand conventions, patterns, and what already exists

Do NOT start implementing until you've read all four. Implementing without context leads to code that doesn't match the spec or breaks existing conventions.

## Step 2: Implement Task by Task

### Task Execution Order

1. Start with tasks that have no dependencies (foundational types, data model)
2. Then tasks that depend on those (API handlers, routes)
3. Then integration tasks (connecting components)
4. Write tests alongside the code, not after

### Implementation Approach

For each task:

1. **Read the task**: Understand the done conditions, file paths, dependencies
2. **Check existing code** (brownfield): If modifying an existing file, understand its current structure before changing it
3. **Implement**: Write the minimum code needed to satisfy the done conditions
4. **Self-verify**: Run the done conditions locally before marking complete
5. **Move to next task**: Follow the dependency order

### Brownfield vs Greenfield

**Greenfield** (new codebase):
- Follow the project structure from the plan
- Create files in the paths specified by the tasks
- Establish conventions early (naming, error handling, testing patterns)

**Brownfield** (existing codebase):
- Read the existing code before modifying it
- Follow existing conventions (naming, error handling, testing patterns)
- Modify existing files in-place — do NOT create `ClassName_modified.go`, `ClassName_new.go`, etc.
- Check for existing tests that might be affected by your changes
- Verify no duplicate files are created alongside existing ones

### File Location Rules

- **Application code**: In the repository, at the paths specified by the plan (NEVER in documentation directories)
- **Documentation**: Only in designated docs directories
- **Tests**: Alongside the code they test (Go: `_test.go` files, TypeScript: `.spec.ts` or `.test.ts` files)

### Project Structure by Type

- **Greenfield single service**: `cmd/`, `internal/`, `pkg/`, `ui/`, `specs/`
- **Greenfield multi-service**: `[service-name]/cmd/`, `[service-name]/internal/`, etc.
- **Brownfield**: Use existing structure — don't introduce a new layout

## Step 3: Self-Verification Protocol

Before marking any task as complete, verify:

1. **The service starts** — build succeeds, binary runs without panicking
2. **The endpoints respond** — hit each endpoint, verify no nil pointer panics, proper error codes
3. **The done conditions pass** — the Architect specified specific assertions for each task
4. **No stubs remain** — search for TODO, FIXME, HACK, placeholder implementations
5. **JSON arrays are [] not null** — marshal the zero-value struct, verify empty collections
6. **Error paths work** — test 400, 404, 409, and other error responses
7. **Existing tests still pass** — if brownfield, run the existing test suite

## Step 4: Agent Failure Mode Checklist

When implementing code as an AI agent, specifically check these systematic bugs:

### 1. Nil Pointer Chains
Initialize struct fields in the correct order. If a handler uses `s.Field`, make sure `s.Field` is set before the handler is registered.

```go
// WRONG — middleware uses s.mux before it's set
handler := corsMiddleware(s.mux)  // nil
s.mux = http.NewServeMux()

// CORRECT — set fields before using them
mux := http.NewServeMux()
s.mux = mux
handler := corsMiddleware(s.mux)
```

### 2. Null vs Empty Arrays
Use `json:"fieldname"` NOT `json:"fieldname,omitempty"` for slice/map fields. Initialize slices to empty (not nil).

```go
Artifacts []Artifact `json:"artifacts"`  // correct: [] when empty
Artifacts []Artifact `json:"artifacts,omitempty"`  // wrong: null when empty
```

### 3. Recovery Middleware First
Recovery middleware must be the outermost middleware:
```go
handler := s.recoveryMiddleware(s.corsMiddleware(s.loggingMiddleware(mux)))
```

### 4. Error Response Structure
All error responses: `{"error": "error_code", "details": "Human-readable message"}`

### 5. No Over-Engineering
Write the minimum code needed. If the task says "add an API endpoint," don't add file watchers, SSE registries, and acceptance test generators. 500 lines is suspicious. 5000 lines is almost certainly wrong.

### 6. Don't Create Phantom Methods
Every method you call must actually exist. Every type you reference must be defined. If you write `s.processFeature(ctx, feature)`, make sure `processFeature` is actually implemented on `s`, not just referenced in a comment or docstring.

### 7. Follow Existing Conventions
In brownfield projects, match the existing code style:
- Same error handling pattern
- Same logging pattern
- Same test naming pattern
- Same project structure

## Step 5: Build and Test Integration

### Build Verification

After implementing a task (or group of related tasks):

1. **Build the project**: `go build ./...` or equivalent
2. **Verify build succeeds**: No compilation errors, no warnings that weren't there before
3. **If build fails**: Read the error message carefully. Fix the reported error, not what you think the error might be. Do NOT rewrite large sections of code to fix a compile error.

### Test Execution

Run relevant tests after implementing:

1. **Unit tests**: `go test ./internal/...` or equivalent
2. **Integration tests**: Start the service and hit the endpoints
3. **If tests fail**: Read the test output and the test code. Determine if the test is correct — if it tests a real contract, fix your code. If the test tests an assumption that's no longer valid, document why and update the test.
4. **Do NOT skip or delete failing tests** without understanding what they verify.

### Smoke Test Protocol

After all tasks are complete:

1. Build the binary: `go build -o ~/go/bin/devteam ./cmd/devteam/`
2. Start the service: verify it starts without panicking
3. Hit every endpoint: verify expected status codes
4. Test error paths: verify 400, 404, 409 responses
5. Verify empty state: `GET /api/features` returns `200 []` (not `null`)

## Quality Gate

Implementation is ready for review when:
1. Every task in tasks.md is complete
2. Code compiles in every affected repo
3. Service starts and responds to HTTP requests without panicking
4. JSON arrays are [] not null in all API responses
5. Error responses have proper HTTP status codes and structure
6. No placeholder/stub code remains
7. Each repo's changes are independently buildable
8. All done conditions from tasks.md are verified
9. Existing tests (brownfield) still pass
10. No phantom methods (every method referenced actually exists)