# Inception Phase Rules

## Purpose

Define what to build and why, with enough specificity that the Architect can plan and the Tester can verify. **Every governing standard, RFC, and test vector must be discovered and converted to verifiable constraints before the spec is written.**

## PM Responsibilities

1. **Intake**: Receive loose ideas and external specs
2. **Source Discovery**: Read all governing RFCs, standards, and test vectors (MANDATORY before writing constraints)
3. **Constraint Extraction**: Convert every source requirement and test vector into a traceable constraint
4. **Explore**: Ask structured questions to resolve ambiguity
5. **Clarify**: Fill gaps, resolve contradictions, define edge cases
6. **Specify**: Produce spec.md (with constraint register), acceptance.md, and repos.yaml

## Step 0: Source Discovery — MANDATORY FIRST STEP

Before any analysis, the PM discovers every source of truth that governs the feature's behavior. This is non-negotiable — a spec written without reading the governing standards will produce code that "works" but violates the standard.

### Discovery Checklist

1. **Standards and RFCs**: Does the feature implement a protocol? Find the RFC/standard.
   - Search: HTTP signing → RFC 9421, RFC 9530. OAuth → RFC 6749, 7800, 8252. JWT → RFC 7519. JWK → RFC 7517. JWKS → RFC 7517 §5. Webhooks → Standard Webhooks v1.
   - Read the relevant sections. Do not assume.

2. **Test vectors and conformance suites**: Does the target repo contain compliance test vectors?
   - Search the repo for: `compliance/`, `conformance/`, `test-vectors/`, `negative/`, `positive/`, `fixtures/`
   - Enumerate every negative test vector — each one is a constraint: "Given [this malformed input], the system MUST reject with [this specific response]"
   - Enumerate every positive test vector — each one is a constraint: "Given [this valid input], the system MUST produce [this specific output]"

3. **Error taxonomies**: Does the standard define error codes?
   - Search the standard for: error, taxonomy, code, `*_invalid`, `*_rejected`
   - The spec MUST use these exact codes. Inventing error codes that don't match the standard is a conformance failure.

4. **Security constraints**: Does the standard mandate security behaviors?
   - HTTPS enforcement, private IP rejection, replay protection, key rotation, algorithm allowlists
   - Each becomes a constraint with a security acceptance criterion

5. **Internal conventions**: AGENTS.md, CONTRIBUTING.md, existing patterns
   - The spec must match existing conventions or explicitly justify deviations

### Output: Constraint Register

The constraint register is a section of spec.md. Every constraint is traceable:

```
## Constraint Register

| ID | Source | Section/Vector | Type | Constraint | Verification Method |
|----|--------|----------------|------|------------|---------------------|
| CON-001 | RFC 9421 | §2.5 | correctness | Wire-format failures return Invalid, never throw | Negative vector 024 |
| CON-002 | RFC 9421 | §2.5 | correctness | Signature-Input parsed semantics preserved, not rebuilt | Negative vector 021 |
| CON-003 | RFC 9530 | §2 | correctness | Content-Digest required for all signed bodies including empty | Empty-body test |
| CON-004 | AdCP spec | §D22 | security | JWK alg/kty/crv validated against signature algorithm | Negative vector 025 |
| CON-005 | AdCP spec | taxonomy | consistency | Error codes match expectedUse (request vs webhook) | Error code test |
| CON-006 | AdCP vectors | 024 | conformance | Unquoted keyid param rejected | Conformance test |
| CON-007 | AdCP vectors | 021 | conformance | Duplicate Signature-Input label rejected | Conformance test |
| CON-008 | GCP KMS docs | signing | correctness | P-256/P-384 use Digest, Ed25519 uses setData | Algorithm-specific test |
```

**Every constraint MUST have a corresponding acceptance criterion.** No exceptions. If a constraint has no AC, the spec is not complete.

### Why This Step Exists

PR #32 had 226 passing tests and 11 correctness bugs. The tests passed because they tested the developer's interpretation, not the standard's requirements. The constraint register forces the PM to translate the standard into verifiable criteria before anyone writes code. The architect plans against constraints. The developer implements against constraints. The reviewer verifies against constraints. The tester tests against constraints. The constraint register is the single source of truth that prevents drift from the standard.

## Step 1: Analyze the Request

Before writing anything, analyze the incoming request to determine scope and depth.

### Request Clarity Assessment

Classify the request:
- **Clear**: Specific, well-defined, actionable — minimal clarification needed
- **Vague**: General, ambiguous — needs structured exploration
- **Incomplete**: Missing key information — needs significant clarification

### Request Type Classification

- **New feature**: Adding new functionality
- **Bug fix**: Fixing existing issue
- **Refactoring**: Improving code structure
- **Enhancement**: Improving existing feature
- **Integration**: Connecting systems

### Scope Estimation

- **Single component**: Changes to one component/package
- **Multiple components**: Changes across multiple components
- **System-wide**: Changes affecting entire system
- **Cross-system**: Changes affecting multiple systems

### Complexity Estimation

- **Trivial**: Simple, straightforward change
- **Simple**: Clear implementation path
- **Moderate**: Some complexity, multiple considerations
- **Complex**: Significant complexity, many considerations

This analysis determines how deep to go in subsequent steps. A trivial bug fix needs less exploration than a complex new feature. But always err on the side of more clarity, not less — overconfidence leads to poor specs.

## Step 2: Explore — Requirements Analysis

For anything beyond trivial changes, perform structured requirements analysis.

### Functional Requirements

For each feature, define:
- What the user does (actions)
- What the system does in response (behaviors)
- What data is involved (entities, relationships)
- What the success outcome looks like
- What the failure outcomes look like (error scenarios)

### Non-Functional Requirements

Assess whether the feature has:
- **Performance requirements**: Response time targets, throughput needs
- **Security requirements**: Authentication, authorization, data access controls
- **Scalability requirements**: Concurrent users, data volume growth
- **Reliability requirements**: Uptime, error handling, recovery
- **Usability requirements**: Accessibility, device support

For P1 features, all of these matter. For P3 features, note which ones are relevant.

### Completeness Check

Evaluate ALL of these areas. Mark any that are unclear as [NEEDS CLARIFICATION]:

1. **Functional requirements**: Core features, user interactions, system behaviors — all defined?
2. **Non-functional requirements**: Performance, security, scalability, reliability — addressed?
3. **User scenarios**: Use cases, user journeys, edge cases, error scenarios — covered?
4. **Business context**: Goals, constraints, success criteria — clear?
5. **Technical context**: Integration points, data requirements, system boundaries — defined?
6. **Quality attributes**: Reliability, maintainability, testability, accessibility — considered?

**When in doubt, add a [NEEDS CLARIFICATION] marker.** It's better to flag ambiguity than to assume.

### Resolve Clarifications

For each [NEEDS CLARIFICATION] marker, either:
- Make a reasonable assumption and label it `[ASSUMPTION: ...]` in the spec
- If the ambiguity is fundamental (affects architecture or user-facing behavior), document it and flag it for the Architect to address in planning

Do NOT leave ambiguities unresolved. Every ambiguity either becomes an assumption (documented) or a clarification request (documented).

## Step 3: Clarify — Edge Cases and Error Paths

### Error Scenarios (MANDATORY)

For every user action, define what happens when things go wrong:

| User Action | Success | Error Condition | Expected Response |
|---|---|---|---|
| Create feature | 201 Created | Missing required field | 400 Bad Request |
| Create feature | 201 Created | Duplicate title | 409 Conflict |
| Get feature | 200 OK | Feature not found | 404 Not Found |
| List features | 200 OK [] | No features exist | 200 OK [] (not 404) |
| Update feature | 200 OK | Invalid state transition | 400 Bad Request |

The "200 OK with empty array" vs "404 Not Found" distinction is critical. Empty state is not an error. Missing specific resource is an error.

### Empty State Behavior

For every collection or list in the spec, define what happens when it's empty:
- API returns `200 OK` with `[]` (not `null`, not `404`)
- UI shows "no items" state (not a blank page, not an error)
- Default values are documented

### Boundary Conditions

For every data field, define:
- Minimum and maximum values/lengths
- Required vs optional
- Format constraints (UUID, ISO date, enum values)
- What happens when constraints are violated

## Step 4: Specify — Produce Spec Artifacts

### spec.md must include:

#### User Stories with Priorities

Each user story follows this format:
```
US-001: [Actor] can [action] so that [benefit]
Priority: P1 | P2 | P3
```

Stories are organized by priority. P1 stories are must-have, P2 are should-have, P3 are nice-to-have.

#### Functional Requirements

Each functional requirement is traceable to a user story:
```
FR-001: The system shall [specific behavior]
Source: US-001
```

#### Key Entities and Relationships

Document the data model:
- Entities (what things exist)
- Attributes (what properties each entity has)
- Relationships (how entities relate)
- Lifecycle (how entities change state)

For entities with state transitions, document the valid transitions:
```
Feature states: draft → inception → planning → construction → review → testing → delivery
Invalid transitions: draft → testing (skip phases), delivery → inception (backward)
```

#### Success Criteria

Observable, measurable outcomes that indicate the feature works:
- "User can create a feature and see it in the list"
- "API returns 201 for valid POST, 400 for missing title"
- "Feature list loads in under 2 seconds with 100 items"

NOT: "The feature works well" or "Performance is good"

#### Error Scenarios

The error scenario table from Step 3, with specific HTTP status codes and response bodies.

#### Assumptions and Scope Boundaries

Explicitly document:
- What is IN scope
- What is OUT of scope
- What was assumed (labeled `[ASSUMPTION: ...]`)

### acceptance.md must include:

Verifiable acceptance criteria in this format:
```
AC-001: [Given precondition], when [action], then [expected result]
  Test level: [smoke | integration | e2e | unit]
  Verification: [specific assertion or scenario]
```

Every user story must have at least one acceptance criterion per relevant test level:
- API changes: at least one smoke criterion and one integration criterion
- UI changes: at least one smoke, integration, and E2E criterion
- State machine logic: at least one unit criterion
- Error paths: at least one criterion per error scenario

Error paths and empty states must be explicitly covered. No "should work well" or "should be fast" — only "Given X, When Y, Then Z".

### repos.yaml must include:

- Feature ID
- Affected repositories with name, URL, and branch

## Brownfield Projects — Additional Inception Steps

When working on an existing codebase (brownfield), the PM must also:

### Workspace Analysis

Analyze the existing codebase before writing specs:

1. **Identify existing structure**: What language, framework, build system?
2. **Identify existing patterns**: How is the codebase organized? What conventions exist?
3. **Identify integration points**: What external systems does it connect to?
4. **Identify existing tests**: What test infrastructure exists? What coverage?
5. **Identify existing docs**: Is there API documentation? Architecture docs?

This analysis feeds into the spec's technical context section and ensures the plan respects existing conventions.

### Reverse Engineering Assessment

For brownfield projects, assess:
- **What exists**: Document current architecture, components, data flows
- **What changes**: Identify which existing components are affected
- **What's new**: Identify what needs to be added
- **Impact scope**: Determine the blast radius of changes

Include this assessment in the spec's technical context section.

## Quality Gate

The spec is ready when:
1. **Source discovery is documented** — every governing RFC, standard, and test vector is referenced
2. **Constraint register exists** — every constraint from every source is enumerated with source reference and verification method
3. **Every constraint has an acceptance criterion** — no constraint is unaddressed
4. Every user story has acceptance criteria with test level and verification method
5. Every functional requirement is testable with specific expected outcomes
6. Error paths, empty states, and malformed input paths are explicitly covered
7. Error codes match the standard's taxonomy (not invented)
8. repos.yaml identifies all affected repositories
9. No [NEEDS CLARIFICATION] markers remain (all resolved or converted to [ASSUMPTION])
10. Brownfield projects include workspace analysis in technical context
11. Every entity with state has valid transitions documented
12. **Negative conformance vectors are acceptance criteria** — each negative test vector has an AC that verifies rejection with the exact expected error code