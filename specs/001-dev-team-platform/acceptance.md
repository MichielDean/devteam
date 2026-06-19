# Acceptance Criteria: Dev Team Platform

**Spec**: 001-dev-team-platform
**Created**: 2026-06-19

## US-1: Submit a Loose Idea and Get a Structured Spec

- **AC-001**: Given a loose idea string, the PM intake produces spec.md with at least 1 user story, acceptance.md with at least 1 verifiable criterion per story, and repos.yaml identifying affected repos
- **AC-002**: Given an ambiguous idea, the PM generates structured clarification questions before finalizing the spec
- **AC-003**: Given a spec touching multiple repos, repos.yaml lists all affected repos with their branch names
- **AC-004**: The output artifacts conform to Spec Kit templates (spec-template.md, tasks-template.md)

## US-2: Decompose an External Roadmap into Feature Specs

- **AC-005**: Given a PRD document, the PM decomposes it into N feature specs, each in its own specs/NNN-*/ directory
- **AC-006**: Dependency edges between specs are explicit (spec N depends on spec M) and stored in repos.yaml or a dependency field
- **AC-007**: Gaps in the PRD are identified and flagged, not silently assumed

## US-3: Run a Feature Through the Full Pipeline

- **AC-008**: An approved spec enters Planning and the Architect produces plan.md and tasks.md
- **AC-009**: An approved plan enters Construction and the Developer implements in all repos declared in repos.yaml
- **AC-010**: Completed implementation enters Review and the Reviewer produces a review report with evidence-quoted findings against every acceptance criterion
- **AC-011**: A passing review enters Testing and the Tester produces a test report with test IDs traced to acceptance criteria
- **AC-012**: Passing tests enter Delivery and Ops produces documentation that uses spec terminology (not code-internal names)

## US-4: Cross-Repo Feature Implementation

- **AC-013**: Given a spec with 3 repos in repos.yaml, implementation changes are made in all 3 repos with commit messages referencing the spec number
- **AC-014**: The Reviewer checks all repos against the same acceptance criteria from a single spec
- **AC-015**: Integration tests exercise the full flow across all affected repos

## US-5: Self-Bootstrap the Platform

- **AC-016**: The platform processes spec 001 (itself) through all 6 phases and produces a working Go binary
- **AC-017**: The bootstrapped platform can process spec 002 using the same pipeline it was built with

## US-6: Phase Gate Enforcement

- **AC-018**: Attempting to enter Construction without an approved plan.md is blocked by the orchestrator with a clear error message
- **AC-019**: Attempting to enter Testing without all acceptance criteria passing Review is blocked
- **AC-020**: Attempting to enter Delivery with failing tests is blocked