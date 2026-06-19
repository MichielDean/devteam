# Dev Team Quick Start

**Version**: 0.1.0-dev | **Spec**: 001-dev-team-platform

## Installation

```bash
# Build from source
cd ~/source/devteam
go build -o ~/go/bin/devteam ./cmd/devteam/

# Or install with Go
go install github.com/MichielDean/devteam/cmd/devteam@latest
```

## Initialize a Project

If you're in a directory with a `devteam.yaml`, you're ready to go. The config file defines the pipeline, roles, and repos.

```bash
# Check current status
devteam status
```

## Submit a Feature

### Loose Idea

```bash
devteam intake --type loose --text "We need user authentication with GitHub OAuth"
```

### External Specification

```bash
devteam intake --type external --file path/to/prd.md
```

### With Priority

```bash
devteam intake --type loose --text "Critical security fix" --priority 1
devteam intake --type loose --text "New feature idea" --priority 3
```

## Run the Pipeline

```bash
# Run the next phase for a feature
devteam run 001-user-auth

# Check gate status
devteam gate 001-user-auth

# View all features
devteam status
```

## Self-Bootstrap

```bash
# Process the platform's own spec through its pipeline
devteam bootstrap
```

## Pipeline Phases

| Phase | Role | Gate | Required Artifacts |
|-------|------|------|--------------------|
| Inception | PM, Architect | spec_approved | spec.md, acceptance.md, repos.yaml |
| Planning | Architect | plan_approved | plan.md, tasks.md |
| Construction | Developer | tasks_complete | implementation across repos |
| Review | Reviewer | criteria_met | review report with evidence |
| Testing | Tester | tests_pass | test report with traced IDs |
| Delivery | Ops | docs_match_spec | documentation matching spec terminology |

## Gate Enforcement

If a gate fails, `devteam gate` exits with a non-zero status and shows:

- Missing artifacts and their expected paths
- Which validation checks passed or failed
- Instructions for what to provide next

## Cross-Repo Features

Features that span multiple repos have a single spec with `repos.yaml` declaring scope:

```yaml
feature: 001-user-auth
repos:
  - name: cistern
    url: git@github.com:MichielDean/cistern.git
    branch: feature/001-user-auth
  - name: LLMem
    url: git@github.com:MichielDean/LLMem.git
    branch: feature/001-user-auth
```

The Developer implements across all declared repos. The Reviewer validates all repos against the same acceptance criteria.

## Architecture

```
Intake → Inception → Planning → Construction → Review → Testing → Delivery
         (PM+Arch)    (Arch)     (Dev)       (Reviewer) (Tester)  (Ops)
```

Each phase loads AIDLC governance rules appropriate to the role and stage. Security and resiliency extensions activate for priority-1 features.

## Configuration

- `devteam.yaml` — Pipeline phases, role definitions, extensions
- `repos.yaml` — Repository registry
- `constitution/constitution.md` — 10 governing principles
- `roles/` — INSTRUCTIONS.md for each of the 6 roles
- `rules/` — AIDLC phase governance rules and extensions