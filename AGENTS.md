# AGENTS.md — Dev Team Repository (AIDLC v2)

This file provides context for AI agents working in this repository.

## AIDLC v2 Workflow

Dev Team uses the AIDLC v2 methodology: 5 phases, 32 stages, 10 agents + 2 reviewers, 9 scopes with auto-detection, 3 depth levels, 3 test strategy levels, per-stage approval gates, 68-event audit trail, Bolt-by-Bolt construction.

### 5 Phases / 32 Stages

| Phase | Stages | Purpose |
|-------|--------|---------|
| 0. Initialization | 0.1-0.3 (3) | Workspace scaffold, detection, state init. Auto-proceed, no gates. |
| 1. Ideation | 1.1-1.7 (7) | Intent capture, market research, feasibility, scope, team, mockups, approval. |
| 2. Inception | 2.1-2.8 (8) | Reverse engineering, practices, requirements, stories, mockups, app design, units, delivery planning. |
| 3. Construction | 3.1-3.7 (7) | Functional design, NFR reqs, NFR design, infra design, code gen, build+test, CI. Per-Bolt (3.1-3.5), once (3.6-3.7). |
| 4. Operation | 4.1-4.7 (7) | Deploy pipeline, env provisioning, deploy execution, observability, incident response, perf validation, feedback. |

### 10 Agents + 2 Reviewers

- product (opus) — requirements, stories, scope
- design (opus) — UX/UI, wireframes
- delivery (sonnet) — team formation, Bolt sequencing
- architect (opus) — app design, domain modeling, NFRs
- platform (opus) — infrastructure (cloud-agnostic: Linux/systemd/Docker)
- devsecops (opus) — threat modeling, security scanning
- developer (opus) — code implementation, reverse engineering
- quality (opus) — test strategy, test generation
- pipeline-deploy (sonnet) — CI/CD pipelines, deployment
- operations (sonnet) — observability, incident response, SLOs

Reviewers:
- product-lead (sonnet) — reviews requirements/stories/UX
- architecture-reviewer (sonnet) — reviews technical design

### 9 Scopes (auto-detected from intent)

enterprise (32 stages), feature (32), mvp (22), poc (8), bugfix (7), refactor (8), infra (13), security-patch (9), workshop (25)

### Per-Stage Approval Gates

Every stage (except 0.1-0.3) ends with a gate:
- **Approve** → advance to next stage
- **Request Changes** → revision cycle (up to 3, then "Accept as-is" escape hatch)
- **Accept as-is** → after 3 rejections, accept despite issues

### Learning Loop

Gate rejections are saved as rules in the `rules` DB table. Future stages load relevant rules into agent context. Prevents repeating mistakes.

### Team Knowledge

Per-agent knowledge entries in `team_knowledge` DB table. Loaded into context at dispatch time. Managed via API (`/api/knowledge/{agent}`).

## State Management — USE THE CLI

You are an agent in the Dev Team pipeline. Use the `devteam` CLI to manage your state — do NOT write state files manually.

### Submit Questions
```bash
devteam questions ask <feature-id> --file questions.json
devteam signal <feature-id> needs_feedback
```

### Signal Outcome
```bash
devteam signal <feature-id> pass                                    # stage complete
devteam signal <feature-id> recirculate:<stage-id> --notes "fix"    # send back for revision
devteam signal <feature-id> failed --notes "why"                    # blocked
```

### Submit Artifacts
```bash
devteam artifact submit <feature-id> <type> --file <filename>
devteam artifact submit <feature-id> <type> --content "inline content"
```

Artifact types: spec, acceptance, repos, plan, tasks, research, data_model, review_report, test_report, docs

### Query State
```bash
devteam feature status <feature-id>
devteam stages <feature-id>        # show all 32 stages with status
devteam audit <feature-id>         # show audit trail
```

### Run a Stage
```bash
devteam run-stage <feature-id> <stage-id>    # e.g. devteam run-stage feat-001 1.1
```

### Approve/Reject a Stage Gate
```bash
devteam approve <feature-id> <stage-id>
devteam reject <feature-id> <stage-id> "what needs fixing"
```

### Jump to a Stage or Phase
```bash
devteam jump <feature-id> 2.3                    # jump to stage 2.3
devteam jump <feature-id> phase:construction     # jump to first construction stage
```

## DB-Only

All specs, artifacts, audit events, state, team knowledge, and rules are in SQLite. Nothing on disk except the DB file and agent log files.