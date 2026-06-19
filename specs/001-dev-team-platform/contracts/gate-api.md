# Gate API Contract

**Feature**: 001-dev-team-platform

## Gate Definitions

Each of the 6 pipeline phases has a gate that must pass before the feature can advance to the next phase.

### Gate: spec_approved (Inception → Planning)

**Required artifacts**: `spec.md`, `acceptance.md`, `repos.yaml`

**Validation checks**:
- spec.md contains at least one user story with priority
- acceptance.md contains at least one verifiable criterion per story
- repos.yaml identifies at least one affected repository

**Failure message**: Lists missing artifacts and failed checks with file paths.

---

### Gate: plan_approved (Planning → Construction)

**Required artifacts**: `plan.md`, `tasks.md`

**Validation checks**:
- plan.md addresses all acceptance criteria from acceptance.md
- tasks.md contains specific file paths for implementation
- dependencies between tasks are explicit

---

### Gate: tasks_complete (Construction → Review)

**Required artifacts**: none (implementation is in repos)

**Validation checks**:
- code compiles in every affected repository
- no placeholder or stub code remains (no TODO, FIXME, HACK)
- each repository's changes are independently buildable

---

### Gate: criteria_met (Review → Testing)

**Required artifacts**: `review-report.md`

**Validation checks**:
- every acceptance criterion has been reviewed with quoted evidence
- no critical findings remain unresolved
- security review complete for priority-1 features

---

### Gate: tests_pass (Testing → Delivery)

**Required artifacts**: `test-report.md`

**Validation checks**:
- every acceptance criterion has at least one test traced to it
- all critical-path tests pass
- failed tests have reproduction steps

---

### Gate: docs_match_spec (Delivery → Done)

**Required artifacts**: documentation directory

**Validation checks**:
- documentation uses spec terminology (not code-internal names)
- changelog references the spec number
- cross-repo release order is documented

---

## Recirculation

When a gate fails, the feature is recirculated to an earlier phase:

| Failed gate | Recirculation target |
|-------------|---------------------|
| criteria_met (review) with code issues | construction |
| criteria_met (review) with architectural issues | planning |
| tests_pass (testing) | construction |
| docs_match_spec (delivery) | testing |

The recirculation target depends on the type of failure, not a fixed rule.