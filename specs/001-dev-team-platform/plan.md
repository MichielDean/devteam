# Implementation Plan: Dev Team Platform

**Branch**: `001-dev-team-platform` | **Date**: 2026-06-19 | **Spec**: [specs/001-dev-team-platform/spec.md](../specs/001-dev-team-platform/spec.md)

**Input**: Feature specification from `specs/001-dev-team-platform/spec.md`

## Summary

Build a Go binary that orchestrates 6 specialist agent roles through a fixed 6-phase pipeline (Inception → Planning → Construction → Review → Testing → Delivery). The platform manages specs in a central git repository, dispatches agents with role-specific instructions and AIDLC phase rules, enforces phase gates, and supports two intake paths (loose ideas and external specs). It uses Spec Kit's artifact templates for structured output and AIDLC's adaptive workflow rules for phase governance. No Python runtime dependency for the core pipeline.

## Technical Context

**Language/Version**: Go 1.23+

**Primary Dependencies**:
- `cobra` — CLI framework
- `go-git` — Git operations for spec repo management
- `yaml` — Configuration parsing (devteam.yaml, repos.yaml)
- Spec Kit `specify` CLI — Build-time tool for spec scaffolding (not runtime dependency)

**Storage**: Git repository (central spec repo). No external database. Spec state stored as files in `specs/` directory.

**Testing**: Go standard testing + testify assertions. Integration tests via test directories with fixture specs.

**Target Platform**: Linux (primary), macOS (secondary). Single binary distribution.

**Project Type**: CLI tool (orchestrator) + library (pipeline engine)

**Performance Goals**: Pipeline dispatch <5s per role invocation. Spec resolution <1s. Gate evaluation <500ms.

**Constraints**: No Python runtime dependency. Single Go binary. Must work offline (spec operations are local git).

**Scale/Scope**: 6 roles, 6 phases, 2 intake paths. Support for 1-20 implementation repos per feature spec.

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Spec-Driven, Always | PASS | All features start with spec.md + acceptance.md + repos.yaml |
| II. Six Roles, Fixed Pipeline | PASS | 6 roles, 6 phases, enforced by orchestrator |
| III. Central Spec, Distributed Implementation | PASS | devteam/ is the single source of truth |
| IV. Two Intake Paths, One Output | PASS | Both produce spec.md + acceptance.md + repos.yaml |
| V. Proof-of-Work Gates | PASS | Each gate requires specific artifacts to proceed |
| VI. Cross-Repo Coherence | PASS | repos.yaml declares scope, single spec for multi-repo features |
| VII. Self-Bootstrap | PASS | Spec 001 is the platform itself |
| VIII. Go, Minimal Dependencies | PASS | Go binary, cobra + go-git + yaml |
| IX. AIDLC Phase Governance | PASS | Rules loaded per role per phase from markdown files |
| X. Learn From Cistern | PASS | Structured context, fixed roles, mechanical gates, convergence detection |

## AIDLC Phase Governance Mapping

The AIDLC three-phase model maps to Dev Team's six-phase pipeline:

| AIDLC Phase | Dev Team Phase(s) | Rules Loaded |
|-------------|-------------------|---------------|
| Inception | Inception (PM + Architect) | `rules/aidlc-rule-details/inception/` |
| Construction | Planning, Construction, Review | `rules/aidlc-rule-details/construction/` |
| Operations | Testing, Delivery | `rules/aidlc-rule-details/operations/` |

Extensions loaded per feature priority:
- Priority 1: Security baseline + Resiliency baseline + Property-based testing
- Priority 2: Resiliency baseline
- Priority 3: None (minimal governance)

## Data Model

### Core Entities

```
Feature
├── ID          string    (e.g., "001-dev-team-platform")
├── Title       string
├── Status      enum      (draft, inception, planning, construction, review, testing, delivery, done, recirculated)
├── Priority    int       (1=critical, 2=standard, 3=low)
├── IntakePath  enum      (loose_idea, external_spec)
├── SpecDir     string    (path to specs/001-dev-team-platform/)
├── CreatedAt   timestamp
├── UpdatedAt   timestamp
├── Dependencies []string (feature IDs this depends on)
└── Repos       []RepoRef

RepoRef
├── Name        string    (e.g., "cistern")
├── URL         string    (git URL)
└── Branch      string    (e.g., "feature/001-user-auth")

PhaseState
├── FeatureID   string
├── Phase        enum     (inception, planning, construction, review, testing, delivery)
├── Status       enum     (pending, in_progress, gate_blocked, passed, failed)
├── Artifacts    []Artifact
├── GateResult   *GateResult
├── StartedAt    timestamp
└── CompletedAt  timestamp

Artifact
├── Type        enum      (spec_md, acceptance_md, repos_yaml, plan_md, tasks_md, review_report, test_report, docs)
├── Path        string    (relative path within spec dir)
├── GeneratedBy string    (role that produced it)
└── GeneratedAt timestamp

GateResult
├── Phase        enum
├── Passed       bool
├── MissingArts  []string  (list of required artifacts not present)
├── FailedChecks []string  (list of check descriptions that failed)
└── EvaluatedAt  timestamp
```

### State Machine

```
draft → inception → planning → construction → review → testing → delivery → done
  ↑         │                      │           │          │          │
  │         │                      │           │          │          └──→ recirculated (back to construction)
  │         │                      │           │          └──→ recirculated (back to construction)
  │         │                      │           └──→ recirculated (back to construction or planning)
  │         └──→ recirculated (back to inception)
  └──→ cancelled
```

Each transition requires the gate to pass. Recirculation sends the feature back to fix issues, not to a random earlier phase.

## Project Structure

### Documentation (this feature)

```text
specs/001-dev-team-platform/
├── spec.md              # Feature specification
├── acceptance.md        # Acceptance criteria
├── repos.yaml           # Repository scope
├── plan.md              # This file
├── tasks.md             # Task breakdown (created by /speckit.tasks)
├── data-model.md        # Phase state and entity definitions
├── quickstart.md        # Getting started guide
└── contracts/
    ├── intake-api.md     # Intake path API contracts
    ├── pipeline-api.md   # Pipeline orchestration API contracts
    ├── gate-api.md       # Phase gate evaluation contracts
    └── spec-provider-api.md  # Spec resolution API contracts
```

### Source Code (repository root)

```text
cmd/
└── devteam/
    └── main.go              # CLI entrypoint

internal/
├── config/
│   ├── config.go            # Load devteam.yaml, repos.yaml
│   └── config_test.go
├── feature/
│   ├── feature.go           # Feature entity, state machine
│   ├── feature_test.go
│   ├── state.go             # Phase state, transitions, gate evaluation
│   └── state_test.go
├── intake/
│   ├── loose.go             # Loose idea intake path
│   ├── loose_test.go
│   ├── external.go          # External spec/roadmap intake path
│   ├── external_test.go
│   └── intake_test.go       # Integration tests for both paths
├── pipeline/
│   ├── pipeline.go          # Orchestrator: phase dispatch, gate enforcement
│   ├── pipeline_test.go
│   ├── gate.go              # Gate evaluation logic
│   └── gate_test.go
├── role/
│   ├── role.go              # Role definition, INSTRUCTIONS.md loader
│   ├── role_test.go
│   └── dispatcher.go        # Agent dispatch (opencode provider)
├── spec/
│   ├── provider.go          # Spec resolution: find spec, load artifacts
│   ├── provider_test.go
│   ├── writer.go             # Write spec artifacts to disk
│   └── writer_test.go
├── rules/
│   ├── loader.go             # Load AIDLC phase rules per role
│   └── loader_test.go
└── repo/
    ├── manager.go            # Cross-repo operations: clone, checkout, commit
    └── manager_test.go

roles/                            # Role INSTRUCTIONS.md (already exists)
├── pm/INSTRUCTIONS.md
├── architect/INSTRUCTIONS.md
├── developer/INSTRUCTIONS.md
├── reviewer/INSTRUCTIONS.md
├── tester/INSTRUCTIONS.md
└── ops/INSTRUCTIONS.md

rules/                           # AIDLC rules (already exists)
├── aidlc/core-workflow.md
└── aidlc-rule-details/
    ├── inception/
    ├── construction/
    ├── operations/
    └── extensions/

constitution/
└── constitution.md              # Already exists

devteam.yaml                     # Already exists
repos.yaml                       # Already exists
```

**Structure Decision**: Go CLI project with internal packages. The `cmd/devteam/main.go` is the entrypoint. `internal/` contains the engine: config, feature state machine, intake paths, pipeline orchestrator, role dispatch, spec provider, rule loader, and cross-repo management. Tests mirror the source structure.

## Component Design

### 1. CLI (cmd/devteam/main.go)

Cobra-based CLI with subcommands:

```
devteam init          # Initialize a new devteam project (scaffold directory structure)
devteam status        # Show current pipeline status for all features
devteam intake        # Submit a new feature (loose idea or external spec)
devteam run <feature> # Run the next phase for a feature
devteam gate <feature># Evaluate the current gate for a feature
devteam version       # Print version
```

### 2. Config (internal/config/)

Loads and validates `devteam.yaml` and `repos.yaml`. Provides typed access to pipeline phases, role definitions, and extension configuration.

### 3. Feature State Machine (internal/feature/)

Manages feature lifecycle through the 6-phase pipeline. Each phase has:
- Entry gate: required artifacts from the previous phase
- Active state: agent dispatched with role INSTRUCTIONS.md + phase rules
- Exit gate: required artifacts for the current phase must pass validation

Recirculation sends features back to a specific earlier phase based on gate failure type.

### 4. Intake Paths (internal/intake/)

**Loose idea path**: Receives a text description, invokes the PM role to explore and refine. PM uses AIDLC inception rules and Spec Kit spec template. Output: `spec.md`, `acceptance.md`, `repos.yaml`.

**External spec path**: Receives a document (file path or URL), invokes the PM role to decompose. PM identifies gaps, maps requirements to repos, breaks into N features with dependency edges. Output: N × `spec.md`, `acceptance.md`, `repos.yaml`.

### 5. Pipeline Orchestrator (internal/pipeline/)

Dispatches roles through phases. For each phase:
1. Load the role's INSTRUCTIONS.md
2. Load the phase's AIDLC rules
3. Load any active extensions (security, resiliency) based on feature priority
4. Inject spec artifacts as context
5. Invoke the agent (opencode provider)
6. Collect output artifacts
7. Evaluate the exit gate

### 6. Role Dispatcher (internal/role/)

Maps role names to INSTRUCTIONS.md paths. Prepares the agent invocation payload: role instructions + phase rules + spec context. Uses the opencode CLI provider (same agent interface as Cistern).

### 7. Spec Provider (internal/spec/)

Resolves spec artifacts from the central repo. Given a feature ID, finds and loads `spec.md`, `acceptance.md`, `repos.yaml`, `plan.md`, `tasks.md`. Validates artifact presence for gate evaluation.

### 8. Rule Loader (internal/rules/)

Loads AIDLC markdown rules for the current phase and role. Scans the rules directory based on the phase mapping:
- Inception: `rules/aidlc-rule-details/inception/`
- Planning/Construction/Review: `rules/aidlc-rule-details/construction/`
- Testing/Delivery: `rules/aidlc-rule-details/operations/`

Plus extensions based on feature priority.

### 9. Cross-Repo Manager (internal/repo/)

For features spanning multiple repos:
- Clone/checkout repos declared in repos.yaml
- Create feature branches
- Coordinate commits across repos with consistent messages
- Verify each repo is independently buildable at checkpoints

## API Contracts

### Intake API

```go
type IntakeRequest struct {
    Type       IntakeType  // LooseIdea or ExternalSpec
    Content    string       // Idea text or path to spec document
    Priority   int          // 1, 2, or 3
    Repos      []string     // Initial repo list (PM may expand)
}

type IntakeResponse struct {
    FeatureID   string
    SpecPath    string       // Path to specs/NNN-*/ directory
    Artifacts   []string     // List of generated artifact paths
}
```

### Pipeline API

```go
type RunRequest struct {
    FeatureID  string
    Phase       Phase       // If empty, run next phase
    DryRun      bool        // Evaluate gate without dispatching agent
}

type RunResponse struct {
    FeatureID   string
    Phase        Phase
    GateResult   GateResult
    Artifacts    []string    // Artifacts produced by this phase
}
```

### Gate API

```go
type GateResult struct {
    Phase         Phase
    Passed        bool
    RequiredArts  []ArtifactCheck  // Each required artifact and its status
    Checks        []CheckResult    // Phase-specific validation checks
    EvaluatedAt   time.Time
}

type ArtifactCheck struct {
    Type      ArtifactType
    Path      string
    Present   bool
    Valid     bool   // If present, does it pass basic validation?
    Errors    []string
}
```

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Git-only storage | Constitution: central spec repo, no external database | Could use SQLite for state tracking, but adds deployment complexity and contradicts "specs in git" principle. Git-only keeps things simple and auditable. |
| Single binary | Constitution VIII: Go, minimal dependencies | Could split into microservices, but that adds orchestration complexity for a tool that runs locally. |

## Research Notes

### Agent Provider Model

Dev Team uses the opencode CLI as its agent provider (same model as Cistern). Each role invocation is an `opencode run` subprocess with:
- The role's INSTRUCTIONS.md as system context
- The relevant AIDLC rules injected into the agent context
- The spec artifacts as task context
- A timeout to prevent runaway agents

This model is proven (Cistern uses it) and avoids building an LLM integration layer.

### Spec Kit Integration

The `specify` CLI is used as a **build-time tool** for scaffolding specs, not as a runtime dependency. When `devteam intake` creates a new feature, it calls `specify init` to scaffold the spec directory structure and templates. The runtime engine works directly with the markdown files — it doesn't invoke `specify` during pipeline execution.

### Self-Bootstrap Path

Spec 001 defines the platform itself. The first time `devteam run 001-dev-team-platform` executes:
1. The PM reads spec.md and produces refined acceptance criteria
2. The Architect produces plan.md (this file) and tasks.md
3. The Developer implements the Go binary
4. The Reviewer checks implementation against acceptance.md
5. The Tester runs tests
6. The Ops role documents and tags the release

From spec 002 onward, the platform processes features for other repos using the binary built from spec 001.