# Dev Team Pipeline Governance

This is the Dev Team's own process — not borrowed from AIDLC or any other framework.
It's designed for autonomous multi-agent execution with quality baked in at every phase.

## Principles

1. **Quality is built in, not bolted on.** Every phase has quality requirements that must pass before advancing. The tester doesn't catch bugs at the end — every role prevents bugs at their stage.

2. **Proof of work, not claims.** "Tests pass" is not evidence. Name the files, methods, and assertions you verified. "I started the server and hit every endpoint" is evidence. "I tested it" is not.

3. **The pipeline adapts to the work.** Not every feature needs every phase at full depth. A CLI tool doesn't need E2E tests. A UI change does. The test selection matrix tells you what's required.

4. **Agent-generated code has systematic failure modes.** Nil pointer chains, null arrays, phantom methods, over-engineering, missing error paths. Every role must watch for these.

## Phase Map

| Dev Team Phase | Purpose | Rules Loaded |
|---|---|---|
| Inception | Define what and why | `inception/` |
| Planning | Design how, with test strategy | `planning/` |
| Construction | Implement, with self-verification | `construction/` |
| Review | Adversarial review against spec | `review/` |
| Testing | Multi-level verification | `testing/` |
| Delivery | Ship and document | `delivery/` |

## Extension Loading

The pipeline loads phase-appropriate rules for each role during dispatch. Extensions (security, resiliency, testing) are loaded based on feature priority:

- **P1 features**: Security and resiliency extensions are mandatory
- **P2 features**: Security extension is recommended
- **P3 features**: No mandatory extensions

Extensions are in `rules/pipeline/extensions/`.

## Quality at Every Phase

### Inception (PM)
- Acceptance criteria specify test level (smoke, integration, e2e, unit)
- Error paths and empty states explicitly covered
- Gate: spec.md + acceptance.md + repos.yaml exist with verifiable criteria

### Planning (Architect)
- Plan includes test strategy section for each component
- Tasks include done conditions with specific verifiable assertions
- Agent failure mode checks specified for AI-generated code
- Gate: plan.md + tasks.md exist with test strategy and done conditions

### Construction (Developer)
- Self-verification protocol: start service, hit endpoints, verify no panics
- JSON arrays are [] not null (the #1 agent-generated serialization bug)
- Error responses have proper HTTP status codes and structure
- Gate: code compiles, service starts, no stubs, independently buildable

### Review (Reviewer)
- Every acceptance criterion checked with quoted evidence
- Null pointer safety verified
- Error paths verified
- Middleware chain verified end-to-end
- Gate: review-report.md exists with evidence, no critical findings unresolved

### Testing (Tester)
- 4-level testing: smoke (always), integration (API changes), e2e (UI changes), unit (logic)
- Proof of work: name files, methods, assertions verified
- Spec-implementation drift check
- State machine transition verification
- Agent failure mode checklist
- Gate: test-report.md exists, all critical tests pass, smoke + integration tests verify real system

### Delivery (Ops)
- Documentation matches spec terminology
- Cross-repo release order documented
- Changelog references spec number
- Gate: docs exist, terminology matches, release order documented