---

description: "Task list for Dev Team Platform implementation"
---

# Tasks: Dev Team Platform

**Input**: Design documents from `/specs/001-dev-team-platform/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), data-model.md (required)

**Organization**: Tasks grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4, US5, US6)
- Include exact file paths in descriptions

## Path Conventions

- **Single project**: `cmd/`, `internal/` at repository root
- Paths assume the project structure from plan.md

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization, Go module setup, core types, and configuration loading

- [ ] T001 [P] Create Go module and project structure per plan.md — `go.mod`, `cmd/devteam/main.go`, `internal/` package directories
- [ ] T002 [P] Define core types and enums in `internal/feature/feature.go` — Feature, Phase, Status, IntakePath, ArtifactType, RoleName constants and types
- [ ] T003 [P] Define RepoRef and artifact types in `internal/feature/types.go` — RepoRef, Artifact, GateResult, ArtifactCheck, CheckResult structs
- [ ] T004 [P] Create YAML config structs in `internal/config/config.go` — Config, PipelineConfig, RoleConfig, PhaseConfig, ExtensionConfig structs matching devteam.yaml schema
- [ ] T005 Create config loader in `internal/config/config.go` — Load devteam.yaml and repos.yaml, validate pipeline phases and role definitions, return typed Config struct
- [ ] T006 Write config loader tests in `internal/config/config_test.go` — Test loading devteam.yaml, test loading repos.yaml, test validation errors for missing phases/roles

**Checkpoint**: Project structure exists, config loads, types compile

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T007 [P] Implement feature state machine in `internal/feature/state.go` — PhaseState transitions, gate evaluation entry/exit logic, recirculation paths. State transitions: draft→inception→planning→construction→review→testing→delivery→done with recirculation back to construction/planning/inception
- [ ] T008 [P] Implement Feature CRUD in `internal/feature/feature.go` — Create feature from intake, load feature from spec directory, save feature state to `.devteam-state.yaml`, list features, get feature by ID
- [ ] T009 Implement spec provider in `internal/spec/provider.go` — Load spec.md, acceptance.md, repos.yaml, plan.md, tasks.md from a feature's spec directory. Validate artifact presence and basic structure. Return typed structs.
- [ ] T010 Implement spec writer in `internal/spec/writer.go` — Write spec artifacts to disk in the correct directory structure. Create feature directory if it doesn't exist. Write `.devteam-state.yaml` for feature state.
- [ ] T011 [P] Implement rule loader in `internal/rules/loader.go` — Load AIDLC markdown rules for a given phase and role. Map Dev Team phases to AIDLC rule directories (inception→inception/, construction→construction/, operations→operations/). Load extension rules based on feature priority.
- [ ] T012 [P] Implement role loader in `internal/role/role.go` — Load role INSTRUCTIONS.md from roles/{role}/INSTRUCTIONS.md. Return role definition with name, description, phase rules path. Validate all 6 roles exist in config.
- [ ] T013 Implement role dispatcher in `internal/role/dispatcher.go` — Dispatch an agent invocation for a role. Build the context payload: role INSTRUCTIONS.md + phase AIDLC rules + spec artifacts + feature state. Invoke opencode CLI as subprocess with timeout.
- [ ] T014 Write state machine tests in `internal/feature/state_test.go` — Test all valid transitions, test invalid transitions are blocked, test recirculation paths, test gate evaluation with missing artifacts
- [ ] T015 Write feature CRUD tests in `internal/feature/feature_test.go` — Test create, load, save, list, get operations. Test loading from fixture spec directories.

**Checkpoint**: Foundation ready — state machine, config, spec I/O, rule loading, role dispatch all work

---

## Phase 3: User Story 1 - Loose Idea Intake (Priority: P1) 🎯 MVP

**Goal**: Submit a loose idea and get a structured spec with acceptance criteria and repos.yaml

**Independent Test**: Submit "We need user auth" and verify the output includes spec.md with user stories, acceptance.md with verifiable criteria, and repos.yaml identifying affected repositories

### Implementation for User Story 1

- [ ] T016 Implement loose idea intake in `internal/intake/loose.go` — Accept a text description, create a Feature with IntakePath=loose_idea and Status=draft. Generate feature ID from title. Create spec directory structure.
- [ ] T017 Implement PM exploration prompt in `internal/intake/loose.go` — Build the PM agent context: load PM INSTRUCTIONS.md + AIDLC inception rules + Spec Kit spec template. Format the loose idea as the user input for the PM to explore and clarify.
- [ ] T018 Implement PM spec generation in `internal/intake/loose.go` — After PM exploration, generate spec.md (following Spec Kit template), acceptance.md (with verifiable criteria traced to user stories), and repos.yaml (identifying affected repos).
- [ ] T019 Wire intake into CLI in `cmd/devteam/main.go` — Add `devteam intake --type loose --text "idea text"` subcommand. Call loose idea intake path. Print resulting feature ID and spec directory path.
- [ ] T020 Write loose idea intake tests in `internal/intake/loose_test.go` — Test: submit "We need user auth" and verify spec.md contains user stories, acceptance.md contains testable criteria, repos.yaml lists repos. Test: submit ambiguous idea and verify PM asks clarifying questions. Test: submit multi-repo idea and verify repos.yaml identifies all repos.

**Checkpoint**: Loose idea intake produces complete spec artifacts

---

## Phase 4: User Story 2 - External Spec Decomposition (Priority: P1)

**Goal**: Submit a PRD or roadmap and get N feature specs with dependency edges

**Independent Test**: Submit a PRD for a multi-repo feature and verify it decomposes into multiple specs with dependency relationships

### Implementation for User Story 2

- [ ] T021 Implement external spec intake in `internal/intake/external.go` — Accept a file path or URL, create a Feature with IntakePath=external_spec and Status=draft. Load the document content.
- [ ] T022 Implement PM decomposition prompt in `internal/intake/external.go` — Build the PM agent context: load PM INSTRUCTIONS.md + AIDLC inception rules + Spec Kit spec template. Format the external spec as input for the PM to decompose into N features.
- [ ] T023 Implement dependency edge generation in `internal/intake/external.go` — After PM decomposition, create N feature specs each with their own spec.md, acceptance.md, repos.yaml. Record dependency edges between features (spec N depends on spec M).
- [ ] T024 Wire external intake into CLI in `cmd/devteam/main.go` — Add `devteam intake --type external --file path/to/prd.md` subcommand. Call external spec intake path. Print resulting feature IDs and dependency graph.
- [ ] T025 Write external spec intake tests in `internal/intake/external_test.go` — Test: submit a PRD and verify it decomposes into N specs with dependency edges. Test: submit a PRD with gaps and verify the PM flags them. Test: submit a single-repo PRD and verify it creates 1 spec.

**Checkpoint**: Both intake paths produce structured specs from different inputs

---

## Phase 5: User Story 3 - Full Pipeline Execution (Priority: P1)

**Goal**: A spec flows through all 6 pipeline phases producing artifacts at each gate

**Independent Test**: Create a spec for a simple feature and verify it passes through all 6 phases producing expected artifacts

### Implementation for User Story 3

- [ ] T026 Implement pipeline orchestrator in `internal/pipeline/pipeline.go` — Given a feature ID and current phase, dispatch the appropriate role with the right context. Transition phase state after successful gate evaluation. Handle recirculation on gate failure.
- [ ] T027 Implement gate evaluator in `internal/pipeline/gate.go` — For each of the 6 gates, check required artifacts exist and pass validation. spec_approved: spec.md + acceptance.md + repos.yaml. plan_approved: plan.md + tasks.md. tasks_complete: implementation exists in repos. criteria_met: review_report with evidence. tests_pass: test_report with traced IDs. docs_match_spec: documentation with spec terminology.
- [ ] T028 Implement pipeline CLI in `cmd/devteam/main.go` — Add `devteam run <feature-id>` subcommand to run the next phase. Add `devteam gate <feature-id>` to evaluate the current gate. Add `devteam status` to show all features and their current phase.
- [ ] T029 Write pipeline orchestrator tests in `internal/pipeline/pipeline_test.go` — Test: run a feature from inception through delivery with all gates passing. Test: run a feature where a gate fails and verify recirculation to the correct earlier phase. Test: run a feature with missing artifacts and verify gate blocks advancement.
- [ ] T030 Write gate evaluator tests in `internal/pipeline/gate_test.go` — Test each of the 6 gates individually. Test: spec_approved gate with all required artifacts passes. Test: spec_approved gate with missing acceptance.md fails with clear error. Test: criteria_met gate with review report missing evidence fails.

**Checkpoint**: Full pipeline flows from intake to delivery with gate enforcement

---

## Phase 6: User Story 4 - Cross-Repo Feature Implementation (Priority: P2)

**Goal**: A feature spanning multiple repos is implemented coherently across all of them

**Independent Test**: Create a spec declaring 3 repos and verify implementation, review, and testing all operate across them

### Implementation for User Story 4

- [ ] T031 Implement repo manager in `internal/repo/manager.go` — Clone/checkout repos declared in repos.yaml. Create feature branches with consistent naming (feature/NNN-description). Coordinate commits across repos with consistent messages referencing the spec number.
- [ ] T032 Implement cross-repo spec resolution in `internal/spec/provider.go` — Extend spec provider to resolve spec artifacts across multiple repos. When a feature declares N repos in repos.yaml, the spec provider makes the central spec available to agent invocations for each repo.
- [ ] T033 Implement cross-repo review context in `internal/role/dispatcher.go` — When dispatching the Reviewer for a multi-repo feature, include the diff for all repos in the context. The reviewer validates all repos against the same acceptance criteria.
- [ ] T034 Write repo manager tests in `internal/repo/manager_test.go` — Test: clone 3 repos and create feature branches. Test: commit across 3 repos with consistent messages. Test: verify each repo is independently buildable at a checkpoint.
- [ ] T035 Write cross-repo integration test in `internal/intake/intake_test.go` — Test: create a spec with 3 repos in repos.yaml. Run through intake and verify repos.yaml identifies all 3. Test: pipeline dispatches developer across all 3 repos. Test: reviewer context includes all 3 repo diffs.

**Checkpoint**: Cross-repo features work end-to-end

---

## Phase 7: User Story 5 - Self-Bootstrap (Priority: P2)

**Goal**: The platform processes its own spec (spec 001) through all 6 phases and produces a working binary

**Independent Test**: `devteam run 001-dev-team-platform` processes the platform's own spec through all phases

### Implementation for User Story 5

- [ ] T036 Implement self-referential spec handling in `internal/feature/feature.go` — When feature ID matches the platform's own spec, the devteam repo itself is the implementation target. Ensure the pipeline handles this without circular dependency (the binary being built is the one running the pipeline).
- [ ] T037 Implement bootstrap mode in `cmd/devteam/main.go` — Add `devteam bootstrap` subcommand that runs spec 001 through the pipeline. This is the self-referential entry point: the platform building itself.
- [ ] T038 Write bootstrap integration test — Test: `devteam bootstrap` processes spec 001 and produces artifacts at each gate. Test: the resulting binary can process a second spec.

**Checkpoint**: Platform builds itself — the strongest possible integration test

---

## Phase 8: User Story 6 - Phase Gate Enforcement (Priority: P2)

**Goal**: The orchestrator blocks advancement when required artifacts are missing or failing

**Independent Test**: Attempt to advance past a gate without required artifacts and verify blocking behavior

### Implementation for User Story 6

- [ ] T039 [P] Implement gate enforcement errors in `internal/pipeline/gate.go` — When a gate evaluation fails, produce a clear error message listing missing artifacts and failed checks. Include the specific artifact paths expected and which ones are missing or invalid.
- [ ] T040 [P] Implement recirculation logic in `internal/feature/state.go` — When a gate fails, transition the feature to the correct earlier phase based on failure type. Review failure → construction. Architecture failure → planning. Test failure → construction.
- [ ] T041 Implement gate enforcement CLI in `cmd/devteam/main.go` — When `devteam run` is called for a feature whose current gate hasn't passed, print the gate failure report and exit with non-zero status. Include instructions for what artifacts are needed.
- [ ] T042 Write gate enforcement tests in `internal/pipeline/gate_test.go` — Test: attempt to enter construction without plan.md → blocked with clear error. Test: attempt to enter testing without all acceptance criteria passing review → blocked. Test: attempt to enter delivery with failing tests → blocked. Test: recirculation sends to correct earlier phase.

**Checkpoint**: Gate enforcement blocks invalid transitions and provides clear feedback

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T043 [P] Write quickstart.md in `specs/001-dev-team-platform/quickstart.md` — Getting started guide: install, initialize, submit a loose idea, run through pipeline
- [ ] T044 [P] Write API contracts in `specs/001-dev-team-platform/contracts/` — intake-api.md, pipeline-api.md, gate-api.md, spec-provider-api.md
- [ ] T045 Code cleanup and go vet across all packages
- [ ] T046 Performance validation: pipeline dispatch <5s per role, spec resolution <1s, gate evaluation <500ms
- [ ] T047 Security hardening: validate all YAML inputs, sanitize agent context construction, no command injection in opencode invocation
- [ ] T048 Update README.md with usage examples, architecture diagram, and quick start

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Foundational — loose idea intake
- **US2 (Phase 4)**: Depends on Foundational — external spec intake
- **US3 (Phase 5)**: Depends on US1 + US2 (pipeline needs both intake paths)
- **US4 (Phase 6)**: Depends on US3 (cross-repo needs pipeline working)
- **US5 (Phase 7)**: Depends on US3 (bootstrap needs pipeline working)
- **US6 (Phase 8)**: Depends on US3 (gate enforcement needs pipeline working)
- **Polish (Phase 9)**: Depends on all desired user stories being complete

### User Story Dependencies

- **US1 (Loose Idea Intake)**: Can start after Foundational (Phase 2)
- **US2 (External Spec Decomposition)**: Can start after Foundational (Phase 2) — independent of US1
- **US3 (Full Pipeline)**: Depends on US1 + US2 (both intake paths must work)
- **US4 (Cross-Repo)**: Depends on US3 (pipeline must work for single-repo first)
- **US5 (Self-Bootstrap)**: Depends on US3 (pipeline must work first)
- **US6 (Gate Enforcement)**: Can start in parallel with US4/US5 after US3

### Parallel Opportunities

- T001, T002, T003, T004 can all run in parallel (different files, no dependencies)
- T007, T008 can run in parallel (state machine and CRUD are separate)
- T011, T012 can run in parallel (rules and roles are separate)
- T014, T015 can run in parallel (separate test files)
- US1 and US2 can start in parallel after Foundational completes
- T039, T040 can run in parallel (different files, no dependencies)
- T043, T044 can run in parallel (documentation tasks)

---

## Implementation Strategy

### MVP First (US1 + US2 + US3)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: US1 (Loose Idea Intake)
4. Complete Phase 4: US2 (External Spec Intake)
5. Complete Phase 5: US3 (Full Pipeline)
6. **STOP and VALIDATE**: Test full pipeline end-to-end with a real spec
7. Deploy MVP — the platform can accept ideas, decompose roadmaps, and run them through 6 phases

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add US1 (Loose Idea) → Test independently → MVP intake works
3. Add US2 (External Spec) → Test independently → Both intake paths work
4. Add US3 (Full Pipeline) → Test end-to-end → Core product works
5. Add US4 (Cross-Repo) → Test with multi-repo spec → Cross-repo works
6. Add US5 (Self-Bootstrap) → Test with spec 001 → Platform builds itself
7. Add US6 (Gate Enforcement) → Test blocked transitions → Quality gates solid

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- The `specify` CLI is a build-time tool for scaffolding specs, not a runtime dependency
- Agent dispatch uses the opencode CLI subprocess model (same approach as Cistern)
- All state is stored in git — no external database required