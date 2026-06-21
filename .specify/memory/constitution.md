# Dev Team Constitution

## I. Spec-Driven, Always

Every feature starts with a spec. No implementation begins without spec.md, plan.md, and acceptance.md in the central spec repo. The spec IS the contract — code that doesn't match the spec is wrong code, not wrong spec.

## II. Six Roles, Fixed Pipeline

The pipeline is the product. Product Manager → Architect → Developer → Code Reviewer → Tester → Release Engineer. Roles cannot be skipped, reordered, or combined. Phase gates are enforced by the orchestrator, not by agent goodwill.

## III. Central Spec, Distributed Implementation

Specs live in one place: the devteam repository. Features that span multiple repos have one spec with a repos.yaml declaring scope. Implementation repos hold a thin .devteam/ pointer back to the central spec. Never duplicate specs across repos.

## IV. Two Intake Paths, One Output Format

Loose ideas and formal roadmaps both enter through the PM role and produce the same artifact shape: spec.md + acceptance.md + repos.yaml. The downstream pipeline is format-agnostic.

## V. Proof-of-Work Gates

Every role must demonstrate it did its job, not just claim it. The Reviewer quotes specific code against acceptance criteria. The Tester names specific test cases traced to user stories. The Architect references specific files and lines. "No issues found" is not acceptable without evidence.

## VI. Cross-Repo Coherence

When a feature spans repos, the Developer works across all of them in one coherent task. The Reviewer validates all repos against the same spec. QA traces user stories to implementations across repos. One spec, one set of acceptance criteria, coherent output.

## VII. Self-Bootstrap

The platform builds itself. Spec 001 is "build the Dev Team platform." Every bug found during real work becomes a spec processed through the platform itself.

## VIII. Go, Minimal Dependencies

The orchestrator is a Go binary. No Python runtime dependency for the core pipeline. Spec Kit's specify CLI is used as a build-time tool for spec scaffolding, not as a runtime dependency.

## IX. Pipeline Governance

Phase-appropriate rules govern each role's behavior. Inception rules guide the PM. Planning rules guide the Architect (including test strategy). Construction rules guide the Developer (including self-verification). Review rules guide the Reviewer. Testing rules guide the Tester (including 4-level testing). Delivery rules guide the Ops. Extensions provide deeper guidance:
- Security and resiliency extensions load for priority-1 features (security also for priority-2)
- Error-recovery and overconfidence-prevention extensions load for all features (always-on)
- The rules are markdown files injected into agent context, not code.

Quality is baked into every phase, not bolted on at the end:
- PM writes acceptance criteria with test levels
- Architect includes test strategy and done conditions
- Developer self-verifies before marking complete
- Reviewer checks null safety, error paths, and middleware chains
- Tester verifies at 4 levels (smoke, integration, e2e, unit)

## X. Learn From Cistern

Structured context beats freeform descriptions. Role identity must be clear and distinct. Phase gates must be mechanically enforced, not procedurally remembered. Specs must have convergence detection. We carry these lessons forward without carrying the runtime.

---

**Version**: 1.1 | **Ratified**: 2026-06-19 | **Last Amended**: 2026-06-20