# Data Model: Dev Team Platform

**Feature**: 001-dev-team-platform
**Date**: 2026-06-19

## Entity Definitions

### Feature

The central entity. A unit of work flowing through the pipeline.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Unique identifier (e.g., "001-dev-team-platform") |
| Title | string | Human-readable title |
| Status | enum | draft, inception, planning, construction, review, testing, delivery, done, recirculated, cancelled |
| Priority | int | 1=critical, 2=standard, 3=low |
| IntakePath | enum | loose_idea, external_spec |
| SpecDir | string | Relative path to spec directory (e.g., "specs/001-dev-team-platform/") |
| CreatedAt | timestamp | ISO 8601 |
| UpdatedAt | timestamp | ISO 8601 |
| Dependencies | []string | Feature IDs this feature depends on |
| Repos | []RepoRef | Repositories in scope |

### RepoRef

A reference to an implementation repository affected by this feature.

| Field | Type | Description |
|-------|------|-------------|
| Name | string | Repository name (e.g., "cistern") |
| URL | string | Git remote URL |
| Branch | string | Feature branch name |

### PhaseState

Tracks which phase a feature is in and whether the gate has passed.

| Field | Type | Description |
|-------|------|-------------|
| FeatureID | string | Parent feature ID |
| Phase | enum | inception, planning, construction, review, testing, delivery |
| Status | enum | pending, in_progress, gate_blocked, passed, failed |
| Artifacts | []Artifact | Artifacts produced by this phase |
| GateResult | *GateResult | Gate evaluation result (nil if not yet evaluated) |
| StartedAt | timestamp | When the phase started |
| CompletedAt | timestamp | When the phase completed (or was recirculated) |

### Artifact

A file produced by a role during a phase.

| Field | Type | Description |
|-------|------|-------------|
| Type | enum | spec_md, acceptance_md, repos_yaml, plan_md, tasks_md, review_report, test_report, docs |
| Path | string | Relative path within spec directory |
| GeneratedBy | string | Role that produced this artifact (pm, architect, developer, reviewer, tester, ops) |
| GeneratedAt | timestamp | When the artifact was created |

### GateResult

The result of evaluating whether a feature can proceed to the next phase.

| Field | Type | Description |
|-------|------|-------------|
| Phase | enum | The phase this gate is for |
| Passed | bool | Whether all required artifacts are present and valid |
| RequiredArts | []ArtifactCheck | Status of each required artifact |
| Checks | []CheckResult | Phase-specific validation checks |
| EvaluatedAt | timestamp | When the gate was evaluated |

### ArtifactCheck

Status of a single required artifact.

| Field | Type | Description |
|-------|------|-------------|
| Type | ArtifactType | The artifact type |
| Path | string | Expected path |
| Present | bool | Whether the file exists |
| Valid | bool | Whether the file passes basic validation |
| Errors | []string | Validation errors if invalid |

### CheckResult

A phase-specific validation check.

| Field | Type | Description |
|-------|------|-------------|
| Name | string | Check name (e.g., "acceptance_criteria_traceable") |
| Passed | bool | Whether the check passed |
| Message | string | Human-readable result or error description |

## State Machine

```
States: draft → inception → planning → construction → review → testing → delivery → done
Transitions:
  draft → inception:       intake submitted
  inception → planning:     spec_approved gate passes
  planning → construction:  plan_approved gate passes
  construction → review:    tasks_complete gate passes
  review → testing:         criteria_met gate passes
  testing → delivery:       tests_pass gate passes
  delivery → done:          docs_match_spec gate passes

Recirculation:
  review → construction:  review fails (fix implementation)
  review → planning:       review finds architectural issues (re-plan)
  testing → construction:  tests fail (fix implementation)
  testing → review:        tests find issues that need re-review
  delivery → testing:       docs don't match spec (re-test doc alignment)

Terminal states:
  done:      all gates passed
  cancelled: explicitly cancelled by user
```

## Gate Definitions

Each gate requires specific artifacts and validation checks:

| Gate | Required Artifacts | Validation Checks |
|------|--------------------|--------------------|
| spec_approved | spec.md, acceptance.md, repos.yaml | User stories have priorities, acceptance criteria are testable, repos identified |
| plan_approved | plan.md, tasks.md | Plan addresses all acceptance criteria, tasks have file paths, dependencies explicit |
| tasks_complete | (implementation in repos) | Code compiles, no placeholder/stub code, basic linting passes |
| criteria_met | review_report | Every acceptance criterion reviewed with evidence, no critical findings |
| tests_pass | test_report | Every acceptance criterion has at least one test, all critical tests pass |
| docs_match_spec | documentation | Docs use spec terminology, changelog references spec number |

## Storage

All state is stored in the git repository as files. No external database.

- Feature state: `specs/001-dev-team-platform/.devteam-state.yaml`
- Phase state: tracked within the feature state file
- Artifacts: files within the spec directory

This ensures full auditability via git history and offline operation.