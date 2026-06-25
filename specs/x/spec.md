# Feature Specification: x

**Feature Branch**: `x`

**Created**: 2026-06-24

**Status**: Draft

**Input**: User description: "x"

## Workspace Summary

This is a brownfield change to the **devteam-specs** repository (the Dev Team AI-DLC platform itself).

- **Repo**: `devteam-specs` at worktree `/home/lobsterdog/worktrees/devteam-specs/x` (branch `spec/x`)
- **Languages**: Go (backend, `cmd/`, `internal/`), TypeScript/React (frontend, `ui/`)
- **Build**: `PATH="$PATH:/usr/local/go/bin" go build -o ~/go/bin/devteam ./cmd/devteam/`; `cd ui && npm run build`
- **Tests**: `PATH="$PATH:/usr/local/go/bin" go test ./... -count=1 -timeout 120s`; `cd ui && npx playwright test`
- **Config**: `devteam.yaml` (repo root), SQLite at `.devteam.db`
- **Conventions**: Per `AGENTS.md` — platform-agnostic phase/role instructions; build/deploy only from main, never from a worktree; specs are runtime data under `specs/`; service on `:8765`, Playwright on `:18765`. Agents must discover the project's actual build/test commands, not hardcode them.

No `constitution.md` exists at repo root or `.specify/constitution.md`. Constitution compliance check: N/A (no constitution).

## Source Discovery

**No external RFCs or standards govern this feature.** The input ("x") provides no protocol, no standard reference, no test vectors. Source discovery found:

- `AGENTS.md` — internal conventions (read, applied above)
- `.specify/templates/spec-template.md` — SpecKit template (followed)
- No `compliance/`, `conformance/`, `test-vectors/`, `fixtures/` directories in repo
- No external standards applicable

**Constraint register**: N/A — no external sources, so no traceable external constraints. Internal conventions captured as assumptions below.

## Request Analysis

- **Clarity**: Vague (input is the single letter "x")
- **Type**: New feature (placeholder/test feature per intake)
- **Scope**: Single component (devteam-specs repo, this spec dir)
- **Complexity**: Trivial — feature exists to exercise the pipeline

Per the overconfidence-prevention conservative default and error-recovery guidance for ambiguous requirements: treat feature `x` as a **pipeline exercise feature** — its purpose is to run the Dev Team pipeline end-to-end (inception → … → delivery) and verify each phase produces its required artifacts. The "product" is a successful pipeline run that gates pass.

Clarifying questions were written to `specs/x/questions.json`. In autonomous mode (no human answers received within the interaction window), the conservative interpretation below is used and all ambiguities are marked `[ASSUMPTION: ...]`. If answers arrive later, the spec will be revised.

## User Scenarios & Testing

### User Story 1 - Pipeline operator runs feature x through all phases (Priority: P1)

A pipeline operator (the human or autonomous runner invoking `devteam run x`) starts feature x and observes each Dev Team phase (inception → planning → construction → review → testing → delivery) execute, gate-check, and advance with no errors.

**Why this priority**: Without an end-to-end pipeline run, no other value is possible. This is the minimal viable behavior of feature x.

**Independent Test**: Run `devteam run x`; assert `.devteam-state.yaml` shows each phase `status: complete` in order and `outcome.txt` = `pass` at each phase.

**Acceptance Scenarios**:
1. **Given** feature x is intaked, **When** `devteam run x` executes inception, **Then** `specs/x/spec.md`, `specs/x/acceptance.md`, `specs/x/repos.yaml`, and `specs/x/outcome.txt` (=pass) exist and the inception gate passes.
2. **Given** inception is complete, **When** planning runs, **Then** `specs/x/plan.md` and `specs/x/tasks.md` exist and the planning gate passes.
3. **Given** planning is complete, **When** construction runs, **Then** the build/test command per AGENTS.md succeeds and the construction gate passes.
4. **Given** construction is complete, **When** review runs, **Then** a review report exists with no unresolved critical findings and the review gate passes.
5. **Given** review is complete, **When** testing runs, **Then** a test report exists, all critical tests pass, and the testing gate passes.
6. **Given** testing is complete, **When** delivery runs, **Then** delivery docs exist and the delivery gate passes; `.devteam-state.yaml` shows feature x `status: complete`.

### User Story 2 - Operator inspects feature x artifacts (Priority: P2)

An operator can list/read the artifacts produced for feature x to confirm the pipeline ran correctly.

**Why this priority**: Observability of a completed run; depends on US-001 succeeding first.

**Independent Test**: After a full run, `ls specs/x/` shows the full artifact set and each file is non-empty and well-formed.

**Acceptance Scenarios**:
1. **Given** feature x has completed all phases, **When** the operator lists `specs/x/`, **Then** all phase artifacts exist (spec.md, acceptance.md, repos.yaml, plan.md, tasks.md, review report, test report, delivery docs).
2. **Given** any artifact file, **When** the operator reads it, **Then** the file contains real content (no empty/placeholder sections).

### User Story 3 - Operator retries a failed phase (Priority: P3)

If a gate fails for feature x, the operator can re-run the phase after a fix and the pipeline advances without re-doing completed phases.

**Why this priority**: Resilience/ergonomics; only needed once a failure occurs.

**Independent Test**: Force a gate failure (e.g., delete `outcome.txt`), re-run, and assert the phase recovers and advances.

**Acceptance Scenarios**:
1. **Given** a phase gate failed for feature x, **When** the operator fixes the blocker and re-runs `devteam run x`, **Then** only the failed-and-later phases re-execute and the feature eventually completes.

### Edge Cases

- **No human answers questions.json**: Pipeline falls back to autonomous mode using documented `[ASSUMPTION: ...]` markers (this spec's default). Verified by running inception with no `answers.json`.
- **Empty spec dir at intake**: `specs/x/` starts with only `input.md` and `.devteam-state.yaml`; inception must create the required artifacts, not error on missing files.
- **Re-run with artifacts already present**: Re-running inception should not corrupt or duplicate artifacts; idempotent writes.
- **State file out of sync with artifacts**: If `.devteam-state.yaml` says a phase is complete but artifacts are missing, the pipeline documents the gap and proceeds with best-available information (per error-recovery extension).

## Requirements

### Functional Requirements

- **FR-001**: The system MUST allow `devteam run x` to start the inception phase for feature x. Source: US-001
- **FR-002**: The inception phase for feature x MUST produce `specs/x/spec.md`, `specs/x/acceptance.md`, `specs/x/repos.yaml`, and `specs/x/outcome.txt` (=pass). Source: US-001
- **FR-003**: Each phase gate for feature x MUST evaluate the required artifacts per the Dev Team pipeline rules and advance only when the gate passes. Source: US-001
- **FR-004**: The pipeline MUST run phases in order: inception → planning → construction → review → testing → delivery, with no backward or skipped transitions for feature x. Source: US-001
- **FR-005**: The operator MUST be able to list and read feature x artifacts under `specs/x/`. Source: US-002
- **FR-006**: Every artifact produced for feature x MUST contain real, non-placeholder content matching its template. Source: US-002
- **FR-007**: When a phase gate fails for feature x, re-running `devteam run x` MUST re-execute the failed phase and later phases without re-doing already-complete phases. Source: US-003
- **FR-008**: When no human answers `questions.json` within the interaction timeout, the PM MUST fall back to autonomous mode using documented `[ASSUMPTION: ...]` markers. Source: US-001 (edge case)

### Key Entities

- **Feature x**: id=`x`, title=`x`, priority=P3, intake_path=`loose_idea`. State machine: `draft → inception → planning → construction → review → testing → delivery`. Valid transitions: forward only. Invalid: skip (draft→testing), backward (delivery→inception).
- **Phase state**: per-phase `status` field in `.devteam-state.yaml` (`draft` → `in_progress` → `complete`).
- **Artifact**: a file under `specs/x/` produced by a phase (spec.md, plan.md, tasks.md, etc.).

## Success Criteria

- **SC-001**: Running `devteam run x` from intake results in `.devteam-state.yaml` showing feature x `status: complete` with all six phases `status: complete`.
- **SC-002**: Every phase artifact listed in the Dev Team gate criteria exists under `specs/x/` and is non-empty after delivery.
- **SC-003**: No phase gate for feature x reports an unresolved critical finding at the end of the run.
- **SC-004**: `specs/x/outcome.txt` contains `pass` as its first line at the end of inception (this phase).
- **SC-005**: If a gate fails, re-running `devteam run x` advances the feature to completion within 2 re-runs.

## Error Scenarios

| User Action | Success | Error Condition | Expected Response |
|---|---|---|---|
| `devteam run x` (inception) | outcome.txt=pass, artifacts present | Artifact write fails (disk full, permissions) | Phase marked failed; error captured in audit.md and logs/; pipeline does not advance |
| `devteam run x` (inception) | outcome.txt=pass | questions.json answers missing after timeout | Fall back to autonomous mode; spec written with [ASSUMPTION:] markers; outcome.txt=pass |
| `devteam run x` (planning) | plan.md present | Architect cannot determine scope from spec | Recirculate to inception with specific gap; planning gate fails |
| `devteam run x` (construction) | build succeeds | `go build` fails | Construction gate fails; error captured; recirculate to planning if architectural |
| `devteam run x` (testing) | test report present, tests pass | Service panics on start during smoke | Auto-recirculate to construction with panic message |
| List `specs/x/` after complete | full artifact set | An artifact is missing despite state=complete | Operator sees gap; spec notes best-available fallback |

## Assumptions

- [ASSUMPTION: feature x is a placeholder/test feature whose product purpose is to exercise the Dev Team pipeline end-to-end. Input "x" carried no real product description.]
- [ASSUMPTION: target surface is the devteam-specs repo itself (self-hosting exercise), not an external product.]
- [ASSUMPTION: priority remains P3 as intaked unless a human answer changes it.]
- [ASSUMPTION: no external RFC/standard applies; no constraint register beyond internal conventions.]
- [ASSUMPTION: autonomous mode (no human answers within timeout) is acceptable; this spec is written under that fallback.]
- [ASSUMPTION: build/test commands are those in AGENTS.md (`go build`/`go test`, `npm run build`/Playwright) — agents must discover and use the project's actual commands, not hardcode them.]

## Scope Boundaries

**In scope**: A successful end-to-end Dev Team pipeline run for feature x producing all required artifacts; the spec defines what "done" means for the pipeline run itself.

**Out of scope**: Any real product capability beyond pipeline exercise (no new user-facing product features are specified, because the input provided none). If the human answers clarify a real product intent, this spec will be rewritten.