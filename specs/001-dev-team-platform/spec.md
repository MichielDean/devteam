# Feature Specification: Dev Team Platform

**Feature Branch**: `001-dev-team-platform`

**Created**: 2026-06-19

**Status**: Draft

**Input**: User description: "A multi-agent development platform with 6 predefined specialist roles (PM, Architect, Developer, Reviewer, Tester, Ops) working through a structured pipeline adapted from AIDLC and Spec Kit. Features a central spec repository for cross-repo feature support, two intake paths (loose ideas and external specs), and self-bootstrapping capability."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Submit a Loose Idea and Get a Structured Spec (Priority: P1)

A developer has a vague idea for a feature. They submit a rough description to Dev Team and the PM explores, clarifies, and refines it into a full spec with acceptance criteria and cross-repo scope.

**Why this priority**: The intake path is the front door of the platform. Without it, nothing else works.

**Independent Test**: Submit "We need user auth" and verify the output includes spec.md with user stories, acceptance.md with verifiable criteria, and repos.yaml identifying affected repositories.

**Acceptance Scenarios**:

1. **Given** a loose idea description, **When** submitted through the PM intake path, **Then** the PM produces spec.md, acceptance.md, and repos.yaml
2. **Given** an ambiguous idea, **When** the PM explores it, **Then** structured clarification questions are generated and answered before the spec is finalized
3. **Given** a spec that touches multiple repos, **When** repos.yaml is generated, **Then** all affected repos are identified with their branches

---

### User Story 2 - Decompose an External Roadmap into Feature Specs (Priority: P1)

A team brings in a PRD or roadmap document. The PM decomposes it into N feature specs with dependency edges, each with its own repos.yaml scope.

**Why this priority**: Real teams work from existing requirements docs, not just loose ideas. This is the second intake path and equally critical.

**Independent Test**: Submit a PRD for a multi-repo feature and verify it decomposes into multiple specs with dependency relationships (spec 002 depends on 001).

**Acceptance Scenarios**:

1. **Given** a PRD document, **When** submitted through the external spec intake path, **Then** the PM produces N feature specs, each with its own spec.md, acceptance.md, and repos.yaml
2. **Given** a PRD with cross-repo scope, **When** decomposed, **Then** dependency edges between specs are explicit (spec N depends on spec M)
3. **Given** a PRD with gaps, **When** decomposed, **Then** the PM identifies and flags gaps rather than assuming missing details

---

### User Story 3 - Run a Feature Through the Full Pipeline (Priority: P1)

Once a spec exists, it flows through all 6 pipeline phases: Inception → Planning → Construction → Review → Testing → Delivery. Each phase produces artifacts, and each gate must pass before proceeding.

**Why this priority**: The pipeline IS the product. Without the full flow working, the platform doesn't exist.

**Independent Test**: Create a spec for a simple feature (single repo, no cross-repo dependencies) and verify it passes through all 6 phases producing the expected artifacts at each gate.

**Acceptance Scenarios**:

1. **Given** an approved spec, **When** entering the Planning phase, **Then** the Architect produces plan.md and tasks.md
2. **Given** an approved plan, **When** entering Construction, **Then** the Developer implements across all repos declared in repos.yaml
3. **Given** completed implementation, **When** entering Review, **Then** the Reviewer produces a review report with evidence-quoted findings against acceptance criteria
4. **Given** a passing review, **When** entering Testing, **Then** the Tester produces a test report with traceable test IDs mapping to acceptance criteria
5. **Given** passing tests, **When** entering Delivery, **Then** the Ops role produces documentation matching spec terminology and coordinates release

---

### User Story 4 - Cross-Repo Feature Implementation (Priority: P2)

A feature that touches 3 repos (e.g., cistern, LLMem, lobsterdog) has a single spec with repos.yaml declaring all three. The Developer works across all repos, the Reviewer validates all repos against the same acceptance criteria, and QA tests the full cross-repo flow.

**Why this priority**: Cross-repo support is a key differentiator from single-repo tools. It's what makes Dev Team a team platform, not just a project tool.

**Independent Test**: Create a spec that declares 3 repos in scope and verify that implementation, review, and testing all operate across the declared repos coherently.

**Acceptance Scenarios**:

1. **Given** a spec with repos.yaml declaring 3 repos, **When** the Developer implements, **Then** changes are made in all 3 repos with consistent commit messages referencing the spec
2. **Given** cross-repo implementation, **When** the Reviewer validates, **Then** all repos are checked against the same acceptance criteria from a single spec
3. **Given** cross-repo changes, **When** QA tests, **Then** integration tests exercise the full flow across all affected repos

---

### User Story 5 - Self-Bootstrap the Platform (Priority: P2)

The Dev Team platform uses itself to build itself. Spec 001 is "build the Dev Team platform." The platform processes its own spec through its own pipeline.

**Why this priority**: Self-bootstrapping proves the platform works for real work. It's the strongest possible integration test.

**Independent Test**: The platform processes spec 001 (itself) through all 6 phases and produces a working Go binary.

**Acceptance Scenarios**:

1. **Given** the Dev Team repo with spec 001, **When** the pipeline runs, **Then** it produces a working Go binary that can process other specs
2. **Given** the bootstrapped platform, **When** spec 002 is submitted, **Then** the platform processes it correctly using the same pipeline it was built with

---

### User Story 6 - Phase Gate Enforcement (Priority: P2)

The orchestrator enforces phase gates. A spec cannot skip from Inception directly to Construction. Review cannot proceed without completed implementation. Each gate must pass before the next phase starts.

**Why this priority**: Mechanical gate enforcement is what separates Dev Team from ad-hoc agent orchestration. Without it, agents skip steps and quality degrades.

**Independent Test**: Attempt to advance a spec past a phase gate without the required artifacts and verify the orchestrator blocks the advancement.

**Acceptance Scenarios**:

1. **Given** a spec without an approved plan.md, **When** attempting to enter Construction, **Then** the orchestrator blocks the advancement and reports missing artifacts
2. **Given** a review that hasn't passed all acceptance criteria, **When** attempting to enter Testing, **Then** the orchestrator blocks the advancement
3. **Given** failing tests, **When** attempting to enter Delivery, **Then** the orchestrator blocks the advancement

---

### Edge Cases

- What happens when a spec touches 0 repos? The PM must identify at least one repo before the spec can proceed.
- What happens when two specs have a circular dependency? The PM must break the cycle by splitting or reordering.
- What happens when a review finds critical issues? The spec is recirculated to Construction, not back to Inception.
- What happens when an external spec has contradictory requirements? The PM flags contradictions and resolves them before decomposition.
- What happens when the same loose idea is submitted twice? The PM detects duplicates and proposes merging.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Dev Team MUST accept loose idea input and produce spec.md, acceptance.md, and repos.yaml through PM exploration
- **FR-002**: Dev Team MUST accept external spec/roadmap input and decompose it into N feature specs with dependency edges
- **FR-003**: Dev Team MUST enforce a 6-phase pipeline: Inception → Planning → Construction → Review → Testing → Delivery
- **FR-004**: Each phase MUST have a defined gate with required artifacts that must pass before proceeding
- **FR-005**: Dev Team MUST support features that span multiple implementation repositories via repos.yaml
- **FR-006**: The orchestrator MUST be a Go binary with no Python runtime dependency for core pipeline execution
- **FR-007**: Each role MUST have a dedicated INSTRUCTIONS.md defining its identity, responsibilities, and quality gate
- **FR-008**: Dev Team MUST load AIDLC phase-appropriate rules for each role during its active phase
- **FR-009**: Dev Team MUST use Spec Kit's artifact templates (spec.md, plan.md, tasks.md, acceptance.md) for structured context
- **FR-010**: The central spec repo MUST be the single source of truth for what's being built
- **FR-011**: Implementation repos MUST contain a thin .devteam/ pointer back to the central spec
- **FR-012**: Dev Team MUST support self-bootstrapping: the platform can process its own spec through its own pipeline
- **FR-013**: Dev Team MUST support the Constitution as the governing document for coding principles and quality standards
- **FR-014**: Dev Team MUST support opt-in security and resiliency extensions for priority-1 features
- **FR-015**: Dev Team MUST detect spec convergence drift (implementation diverges from spec)

### Key Entities

- **Spec**: The central artifact — what is being built. Contains spec.md, acceptance.md, plan.md, tasks.md, repos.yaml
- **Role**: One of 6 specialist agents with defined identity and responsibilities
- **Phase**: A pipeline stage with entry gate, exit gate, and role assignments
- **Gate**: A checkpoint requiring specific artifacts to pass before proceeding
- **Feature**: A unit of work defined by a spec, potentially spanning multiple repos
- **Intake Path**: How a feature enters the pipeline (loose idea or external spec)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A loose idea can be submitted and produces a complete spec.md with user stories, acceptance criteria, and repos.yaml in under 3 PM exploration rounds
- **SC-002**: An external PRD can be decomposed into independent feature specs with dependency edges
- **SC-003**: A feature can be processed through all 6 pipeline phases producing artifacts at each gate
- **SC-004**: A feature declaring 3 repos in repos.yaml results in coordinated changes across all 3 repos
- **SC-005**: The platform can process its own spec (spec 001) and produce a working binary
- **SC-006**: Phase gates block advancement when required artifacts are missing or failing
- **SC-007**: No Python runtime is required to execute the core pipeline (Go binary only)

## Assumptions

- The platform uses the opencode agent provider (same as Cistern) for LLM interactions
- Spec Kit's `specify` CLI is available as a build-time tool for scaffolding specs, not a runtime dependency
- AIDLC rules are markdown files injected into agent context, not code
- The initial implementation is a single Go binary that orchestrates the 6 roles
- Each role runs as a separate agent invocation with its INSTRUCTIONS.md loaded
- The platform stores spec state in the central git repository (no external database required)
- Cross-repo features use git worktrees for isolated implementation