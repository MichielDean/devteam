# Dev Team Pipeline Governance

This is the Dev Team's own process — not borrowed from AIDLC or any other framework.
It's designed for autonomous multi-agent execution with quality baked in at every phase.
The pipeline supports both fully autonomous execution and interactive execution where
a human can provide input through the UI at decision points.

## Principles

1. **Quality is built in, not bolted on.** Every phase has quality requirements that must pass before advancing. The tester doesn't catch bugs at the end — every role prevents bugs at their stage.

2. **Proof of work, not claims.** "Tests pass" is not evidence. Name the files, methods, and assertions you verified. "I started the server and hit every endpoint" is evidence. "I tested it" is not.

3. **The pipeline adapts to the work.** Not every feature needs every phase at full depth. A CLI tool doesn't need E2E tests. A UI change does. The test selection matrix tells you what's required.

4. **Agent-generated code has systematic failure modes.** Nil pointer chains, null arrays, phantom methods, over-engineering, missing error paths. Every role must watch for these.

5. **Human input improves quality.** The pipeline can run fully autonomous, but inception and planning benefit from human input when available. The PM can ask clarifying questions through the UI. The Architect can surface design decisions for human review. When a human is available, use them. When running autonomously, document assumptions and proceed conservatively.

## Phase Map

| Dev Team Phase | Purpose | Rules Loaded | Human Interaction |
|---|---|---|---|
| Inception | Define what and why | `inception/` | PM asks questions, human clarifies |
| Planning | Design how, with test strategy | `planning/` | Architect surfaces design decisions |
| Construction | Implement, with self-verification | `construction/` | Fully autonomous |
| Review | Adversarial review against spec | `review/` | Fully autonomous |
| Testing | Multi-level verification | `testing/` | Fully autonomous |
| Delivery | Ship and document | `delivery/` | Fully autonomous |

## Human Interaction Points

The pipeline supports two modes:

### Autonomous Mode (default)
The pipeline runs end-to-end without human input. The PM documents assumptions (`[ASSUMPTION: ...]`) for ambiguities. The Architect makes design decisions and documents them. Quality gates are evaluated programmatically.

### Interactive Mode (when a human is available through the UI)
The pipeline can pause at decision points and ask the human:
- **Inception**: PM asks clarifying questions about ambiguous requirements
- **Planning**: Architect surfaces design decisions for human input (architecture choices, NFR tradeoffs, scope boundaries)
- **Between phases**: Gate evaluation results can be surfaced for human review

To enable interactive mode, the human uses the web UI to answer questions that the PM or Architect surfaces. The pipeline pauses at these points and resumes when the human provides input.

When no human response is available within a timeout, the pipeline falls back to autonomous mode — documenting the assumption and proceeding with the conservative choice.

## Extension Loading

The pipeline loads phase-appropriate rules for each role during dispatch. Extensions provide deeper guidance beyond the phase rules:

### Always-On Extensions (loaded for all priorities)

- **error-recovery**: What to do when things go wrong — phase-specific recovery patterns, when to fix vs when to recirculate, conservative defaults for uncertain situations.

- **overconfidence-prevention**: Anti-patterns for the systematic LLM tendency to skip questions, make assumptions, and proceed with incomplete information. Five patterns: skipping exploration, happy-path-only, vague criteria, scope expansion, claiming without evidence.

### Priority-Based Extensions

- **security** (mandatory for P1, recommended for P2): Threat modeling, input validation patterns, authentication architecture, security testing scenarios, OWASP Top 10 coverage.

- **resiliency** (mandatory for P1, recommended for P2): Timeout patterns, retry with backoff, circuit breaker design, graceful degradation, panic recovery. Code-level patterns with Go examples.

## Quality at Every Phase

### Inception (PM)
- Request type and complexity classification
- Structured requirements analysis with completeness check
- Error scenarios and empty states explicitly covered in spec
- Assumptions documented with [ASSUMPTION: ...] markers
- Brownfield workspace analysis (when working on existing code)
- Acceptance criteria specify test level (smoke, integration, e2e, unit)
- Gate: spec.md + acceptance.md + repos.yaml exist with verifiable criteria

### Planning (Architect)
- Application architecture: component identification, interfaces, dependencies
- Data model: entities, relationships, state transitions
- API contracts: endpoints, request/response schemas, error responses
- NFR design: performance, security, scalability, reliability considerations
- Task decomposition with explicit file paths, done conditions, and test levels
- Agent failure mode checks specified for AI-generated code
- Gate: plan.md + tasks.md exist with test strategy and done conditions

### Construction (Developer)
- Context loading: read spec, plan, tasks, and existing code before implementing
- Task-by-task implementation following dependency order
- Brownfield vs greenfield patterns (modify in-place vs create new)
- Self-verification protocol: start service, hit endpoints, verify no panics
- JSON arrays are [] not null (the #1 agent-generated serialization bug)
- Error responses have proper HTTP status codes and structure
- Gate: code compiles, service starts, no stubs, independently buildable

### Review (Reviewer)
- Spec-implementation drift check: does the plan cover every user story?
- Every acceptance criterion checked with quoted evidence
- Over-engineering check: is the implementation the minimum needed?
- Missing error paths check: 400, 404, 409, empty states
- Null pointer safety verified
- Middleware chain verified end-to-end
- Gate: review-report.md exists with evidence, no critical findings unresolved

### Testing (Tester)
- Spec-implementation drift verification before writing tests
- 4-level testing: smoke (always), integration (API changes), e2e (UI changes), unit (logic)
- Proof of work: name files, methods, assertions verified
- State machine transition verification
- Agent failure mode checklist
- Anti-fake-report requirements
- Gate: test-report.md exists, all critical tests pass, smoke + integration tests verify real system

### Delivery (Ops)
- API documentation for every endpoint
- User-facing documentation using spec terminology
- Changelog referencing spec numbers
- Cross-repo release order documented and followed
- Build, start, hit endpoints, verify UI
- Gate: docs exist, terminology matches, release order documented