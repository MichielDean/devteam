# Construction Phase Rules

## Purpose

Implement the plan, following the task breakdown, writing code that matches the spec's acceptance criteria. Verify before marking complete.

## Developer Responsibilities

1. **Implement**: Write code following tasks.md
2. **Self-verify**: Before marking a task complete, verify locally
3. **Cross-repo**: Implement coherently across repos
4. **Constitution**: Follow project coding standards

## Self-Verification Protocol

Before marking any task as complete, verify:

1. **The service starts** — build succeeds, binary runs without panicking
2. **The endpoints respond** — hit each endpoint, verify no nil pointer panics, proper error codes
3. **The done conditions pass** — the Architect specified specific assertions for each task
4. **No stubs remain** — search for TODO, FIXME, HACK, placeholder implementations
5. **JSON arrays are [] not null** — marshal the zero-value struct, verify empty collections

## Agent Failure Mode Checklist

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