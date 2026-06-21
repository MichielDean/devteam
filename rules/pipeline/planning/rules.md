# Planning Phase Rules

## Purpose

Design the technical approach with enough specificity that the Developer can implement without making architectural decisions on the fly. Quality starts here — if the plan doesn't specify test strategy and done conditions, the Developer will guess. **Every constraint from the PM's register must have a design decision and a verification checkpoint.**

## Architect Responsibilities

1. **Validate**: Confirm the spec is technically feasible
2. **Constraint Verification**: Map every constraint to a design decision and verification checkpoint
3. **Cross-Component Consistency**: Verify producer/consumer agreement across all components
4. **Plan**: Create plan.md with technical context, constraint map, consistency matrix, and test strategy
5. **Decompose**: Break the spec into implementable tasks in tasks.md with done conditions and constraint references
6. **Scope**: Identify which repos need changes

## Step 1: Validate the Spec — Including Constraints

Before planning, confirm the spec is implementable:

1. **Completeness check**: Are all functional requirements traceable to user stories?
2. **Constraint register check**: Does the constraint register exist? Is every constraint addressable?
3. **Consistency check**: Do any requirements contradict each other?
4. **Feasibility check**: Can this be built with the stated technology stack?
5. **Edge case check**: Are error scenarios, empty states, and malformed input paths defined?
6. **Negative vector check**: Is every negative test vector from the constraint register converted to an acceptance criterion?
7. **Ambiguity check**: Are there any [NEEDS CLARIFICATION] or [ASSUMPTION] markers that need resolution?

If the spec has unresolved ambiguities that affect architecture, resolve them before planning. Document any assumptions you make.

## Step 2: Build the Constraint Verification Map

For every constraint in the PM's register, the architect produces a design decision:

```
| CON-ID | Design Decision | Component(s) | Verification Checkpoint | Test Type |
|--------|-----------------|--------------|------------------------|-----------|
| CON-001 | All parse failures caught and wrapped in Invalid result | Rfc9421Verifier | Negative vector 024 test | Conformance |
| CON-003 | Content-Digest computed for byte[0] in ALL providers | All signing providers | Empty-body test per provider | Integration |
```

**If a constraint applies to multiple components (e.g., "all providers must handle empty bodies"), the design decision must address ALL components, not just one.** The most common multi-component bug is implementing a constraint in one place and forgetting the others.

### Constraint Application Analysis

For each constraint, ask:
- Does this apply to one component or many?
- If many, list ALL components it applies to
- Verify the design decision covers each one explicitly
- The cross-component consistency matrix must confirm this

## Step 3: Build the Cross-Component Consistency Matrix

For features with multiple components, trace every shared value:

1. **List all shared values** — algorithm identifiers, error codes, data formats, signature formats, digest formats
2. **For each, identify the producer(s) and consumer(s)**
3. **Verify they agree** — if the producer emits X, the consumer must accept X
4. **If they don't agree, that's a finding** — the plan must resolve the inconsistency

This catches bugs like: KMS providers emit P-384 signatures but the verifier's allowlist doesn't include P-384. The architect must catch this before the developer writes code.

## Step 4: Design the Application Architecture

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
3. **Traceable to requirements**: Each task references the user stories, acceptance criteria, AND constraints it satisfies
4. **Constraint coverage**: Every constraint from the register is addressed by at least one task
5. **Dependency order**: Tasks that depend on others are clearly marked
6. **Done conditions**: Each task has specific, verifiable completion criteria
7. **Multi-component tasks**: If a constraint applies to multiple components, either one task covers all of them (with explicit per-component done conditions) or separate tasks exist for each component

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
1. **Constraint verification map exists** — every constraint from the register has a design decision and verification checkpoint
2. **Cross-component consistency matrix exists** — every shared value verified across producers and consumers
3. Every task has a specific file path
4. Every task has a done condition with specific verifiable assertions
5. **Every task references the constraints it addresses** (or justifies having none)
6. Test strategy section exists for each component, including conformance tests for negative vectors
7. Cross-repo boundaries are defined with contracts
8. Dependencies between tasks are explicit
9. API contracts specify success and error responses with exact error codes from the standard's taxonomy
10. Data model includes entities, relationships, and state transitions
11. Component design identifies responsibilities, interfaces, and dependencies
12. NFR considerations are addressed (performance, security, scalability, reliability as applicable)
13. **Negative case design exists** for every constraint with a negative test vector
14. **Multi-component constraints verified** — if a constraint applies to N components, all N are addressed