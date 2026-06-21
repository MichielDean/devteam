# Planning Phase Rules

## Purpose

Design the technical approach with enough specificity that the Developer can implement without making architectural decisions on the fly. Quality starts here — if the plan doesn't specify test strategy and done conditions, the Developer will guess.

## Architect Responsibilities

1. **Validate**: Confirm the spec is technically feasible
2. **Plan**: Create plan.md with technical context and test strategy
3. **Decompose**: Break the spec into implementable tasks in tasks.md with done conditions
4. **Scope**: Identify which repos need changes

## Step 1: Validate the Spec

Before planning, confirm the spec is implementable:

1. **Completeness check**: Are all functional requirements traceable to user stories?
2. **Consistency check**: Do any requirements contradict each other?
3. **Feasibility check**: Can this be built with the stated technology stack?
4. **Edge case check**: Are error scenarios and empty states defined?
5. **Ambiguity check**: Are there any [NEEDS CLARIFICATION] or [ASSUMPTION] markers that need resolution?

If the spec has unresolved ambiguities that affect architecture, resolve them before planning. Document any assumptions you make.

## Step 2: Design the Application Architecture

### Component Identification

Identify the main functional components:
- What are the major components and their responsibilities?
- What are the component interfaces (APIs, events, data contracts)?
- What are the component dependencies (which component depends on which)?
- What is the service layer design (how do components orchestrate)?

### Component Design Template

For each component, document:
```
Component: [name]
Purpose: [what it does]
Responsibilities:
  - [responsibility 1]
  - [responsibility 2]
Interfaces:
  - [interface 1]: [input] → [output]
  - [interface 2]: [input] → [output]
Dependencies:
  - depends on [component] for [reason]
```

### Component Dependency Map

Document which components depend on which:
- Direct dependencies (A calls B)
- Shared dependencies (A and B both use C)
- Circular dependencies (identify and flag — must be resolved before implementation)

### Service Layer Design

For multi-component systems:
- Which services orchestrate which workflows?
- What are the service boundaries?
- How do services communicate (REST, events, shared data)?

## Step 3: Design the Data Model

### Entity Definitions

For each entity, document:
```
Entity: [name]
Attributes:
  - [attribute]: [type], [required/optional], [constraints]
Relationships:
  - [relationship]: [cardinality] with [other entity]
State Transitions:
  - [state1] → [state2]: [trigger]
  - [state2] → [state3]: [trigger]
  - Invalid: [state1] → [state3] (skip phases)
```

### Data Integrity Rules

- Which fields are required vs optional?
- What are the unique constraints?
- What are the referential integrity rules?
- What happens on delete (cascade, restrict, set null)?

### API Contracts

For each endpoint:
```
[METHOD] [path]
Request:
  [field]: [type], [required/optional], [constraints]
Response 200:
  [field]: [type], [description]
Response 400:
  { "error": "[code]", "details": "[message]" }
Response 404:
  { "error": "not_found", "details": "[resource] not found" }
```

## Step 4: Design for Non-Functional Requirements

### Performance

If the spec has performance requirements:
- Response time targets per endpoint
- Throughput requirements (requests per second)
- Data volume considerations (how many records, how large)
- Caching strategy (what to cache, invalidation approach)

### Security

If the spec has security requirements (mandatory for P1):
- Authentication approach (who verifies identity?)
- Authorization approach (who can do what?)
- Data classification (public, internal, confidential, restricted)
- Input validation rules per endpoint
- Security headers required

### Scalability

If the spec has scalability requirements:
- Horizontal scaling approach
- Database scaling considerations
- State management (stateless vs stateful)
- Connection pooling and resource limits

### Reliability

If the spec has reliability requirements:
- Error handling strategy per component
- Recovery patterns (retry, circuit breaker, fallback)
- Graceful degradation behavior
- Monitoring and alerting approach

## Step 5: Unit Decomposition — Break into Tasks

### Task Breakdown Methodology

Break the spec into implementable tasks following these principles:

1. **One task, one purpose**: Each task should do one thing well
2. **Explicit file paths**: Every task names the exact files it will create or modify
3. **Traceable to requirements**: Each task references the user stories and acceptance criteria it satisfies
4. **Dependency order**: Tasks that depend on others are clearly marked
5. **Done conditions**: Each task has specific, verifiable completion criteria

### Task Template

```
Task: [T-001] [verb] [what]
Priority: P1 | P2 | P3
User stories: [US-001, US-002]
Files:
  - [repo]/[path/to/file.go] — [create/modify]
  - [repo]/[path/to/other_file.go] — [create/modify]
Dependencies: [T-000] must complete first
Done conditions:
  - [specific verifiable assertion]
  - [specific verifiable assertion]
Test level: [smoke | integration | e2e | unit]
Agent failure mode checks:
  - [ ] Nil pointer ordering verified (if producing initialization code)
  - [ ] JSON arrays are [] not null (if producing serialization)
  - [ ] Recovery middleware is first (if producing HTTP handlers)
  - [ ] State transitions tested (if producing state machine logic)
```

### Dependency Management

Tasks must be ordered so dependencies are built first:
- Shared types and interfaces before consumers
- Data model before API handlers
- Middleware before routes
- Tests alongside (not after) the code they test

For cross-repo tasks:
- Shared libraries/APIs before consumers
- API contracts before implementations
- Document the release order

### Brownfield Task Considerations

For brownfield projects:
- Identify which existing files need modification (not just new files)
- Mark tasks as [MODIFY] or [CREATE] to distinguish
- Document existing conventions to follow (naming, patterns, error handling)
- Flag any breaking changes to existing APIs

## Step 6: Test Strategy

### Per-Component Test Strategy

For each component, document:
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
  - [ ] Error paths return correct status codes and response bodies
```

### Test Level Selection Matrix

| What changed | Smoke | Integration | E2E | Unit |
|---|---|---|---|---|
| HTTP API handlers | **YES** | **YES** | — | YES |
| Frontend/UI components | **YES** | **YES** | **YES** | YES |
| State machine logic | YES | — | — | **YES** |
| Gate evaluator | YES | — | — | **YES** |
| CLI commands | **YES** | — | — | YES |
| Middleware/auth | **YES** | **YES** | — | YES |
| Database operations | **YES** | **YES** | — | YES |

## Quality Gate

The plan is ready when:
1. Every task has a specific file path
2. Every task has a done condition with specific verifiable assertions
3. Test strategy section exists for each component
4. Cross-repo boundaries are defined with contracts
5. Dependencies between tasks are explicit
6. API contracts specify success and error responses
7. Data model includes entities, relationships, and state transitions
8. Component design identifies responsibilities, interfaces, and dependencies
9. NFR considerations are addressed (performance, security, scalability, reliability as applicable)