# Dev Team

A multi-agent development platform aligned with the AIDLC v2 methodology. 10 specialist agents + 2 reviewers work through a 32-stage workflow with per-stage approval gates. All specs, artifacts, audit events, and state in SQLite.

## Status

**v0.3.0** — AIDLC v2 alignment complete. 5 phases, 32 stages, 10 agents + 2 reviewers, 9 scopes with auto-detection, 3 depth levels, 3 test strategy levels, per-stage approval gates, 68-event audit trail, Bolt-by-Bolt construction, team knowledge in DB, learning loop from gate rejections. 283 tests passing.

## Architecture

### 5 Phases / 32 Stages

| Phase | Stages | Purpose |
|-------|--------|---------|
| **0. Initialization** | 0.1-0.3 (3) | Workspace scaffold, detection, state init. Auto-proceed, no gates. |
| **1. Ideation** | 1.1-1.7 (7) | Intent capture, market research, feasibility, scope, team, mockups, approval. |
| **2. Inception** | 2.1-2.8 (8) | Reverse engineering, practices, requirements, stories, mockups, app design, units, delivery planning. |
| **3. Construction** | 3.1-3.7 (7) | Functional design, NFR reqs, NFR design, infra design, code gen, build+test, CI. Per-Bolt (3.1-3.5), once (3.6-3.7). |
| **4. Operation** | 4.1-4.7 (7) | Deploy pipeline, env provisioning, deploy execution, observability, incident response, perf validation, feedback. |

### 10 Agents + 2 Reviewers

| Agent | Domain | Model Tier |
|-------|--------|-----------|
| product | Requirements, stories, scope, market research | opus |
| design | UX/UI, wireframes, interaction design | opus |
| delivery | Team formation, Bolt sequencing, delivery planning | sonnet |
| architect | App design, domain modeling, NFRs, decomposition | opus |
| platform | Infrastructure, provisioning (cloud-agnostic: Linux/systemd/Docker) | opus |
| devsecops | Threat modeling, security scanning, DevSecOps | opus |
| developer | Code implementation, reverse engineering | opus |
| quality | Test strategy, test generation, perf validation | opus |
| pipeline-deploy | CI/CD pipelines, deployment strategy | sonnet |
| operations | Observability, incident response, SLOs, feedback | sonnet |

**Reviewers:**
- product-lead (reviews requirements/stories/UX) — sonnet
- architecture-reviewer (reviews technical design) — sonnet

### 9 Scopes (auto-detected from intent)

| Scope | Stages | Depth | Test Strategy | Use Case |
|-------|--------|-------|---------------|----------|
| enterprise | 32 | Comprehensive | Comprehensive | Regulated enterprise feature |
| feature | 32 | Standard | Standard | Default for new features |
| mvp | 22 | Standard | Standard | Greenfield, skip late operations |
| poc | 8 | Minimal | Minimal | Prove feasibility fast |
| bugfix | 7 | Minimal | Minimal | Fix a specific bug |
| refactor | 8 | Minimal | Minimal | Clean up existing code |
| infra | 13 | Standard | Standard | Infrastructure change |
| security-patch | 9 | Minimal | Minimal | CVE response |
| workshop | 25 | Standard | Minimal | AI-DLC workshop or training |

## Two Intake Paths

- **Loose Ideas**: Submit a rough description. Scope auto-detected. Product agent explores, clarifies, refines into structured specs.
- **External Specs**: Bring in a PRD, RFC, or roadmap. Product agent decomposes into feature specs.

## Quick Start

```bash
# Build
cd ~/source/devteam
go build -o ~/go/bin/devteam ./cmd/devteam/

# Start the web server
devteam -http :8765

# Create a feature (scope auto-detected)
curl -X POST http://localhost:8765/api/features \
  -H "Content-Type: application/json" \
  -d '{"type":"loose_idea","title":"Fix login crash","description":"Fix the nil pointer crash in login handler","priority":1}'

# Run a stage
curl -X POST http://localhost:8765/api/features/{id}/run-stage \
  -H "Content-Type: application/json" \
  -d '{"stage_id":"1.1"}'

# Approve a stage gate
curl -X POST http://localhost:8765/api/features/{id}/stages/1.1/approve

# Reject a stage (saves rule for learning loop)
curl -X POST http://localhost:8765/api/features/{id}/stages/1.1/reject \
  -H "Content-Type: application/json" \
  -d '{"notes":"Missing error case for duplicate users"}'

# Jump to a stage or phase
curl -X POST http://localhost:8765/api/features/{id}/jump \
  -H "Content-Type: application/json" \
  -d '{"phase":"construction"}'

# View audit trail
curl http://localhost:8765/api/features/{id}/audit

# View all stages with status
curl http://localhost:8765/api/features/{id}/stages
```

## API Endpoints

### Stage Workflow
| Endpoint | Description |
|----------|-------------|
| `POST /api/features/{id}/run-stage` | Dispatch one stage's lead agent |
| `POST /api/features/{id}/stages/{stageId}/approve` | Approve gate, advance |
| `POST /api/features/{id}/stages/{stageId}/reject` | Reject with notes (saves rule) |
| `POST /api/features/{id}/stages/{stageId}/accept-as-is` | 3-strike escape hatch |
| `POST /api/features/{id}/jump` | Jump to stage_id or phase |
| `GET /api/features/{id}/stages` | All stages with status |
| `GET /api/features/{id}/audit` | Full 68-event audit trail |

### Scope/Depth/Test Strategy
| Endpoint | Description |
|----------|-------------|
| `POST /api/features/{id}/scope` | Change scope mid-workflow |
| `POST /api/features/{id}/depth` | Change depth |
| `POST /api/features/{id}/test-strategy` | Change test strategy |
| `POST /api/features/{id}/ladder` | Set construction autonomy (gated/autonomous) |

### Construction Bolts
| Endpoint | Description |
|----------|-------------|
| `POST /api/features/{id}/prepare-bolts` | Create Bolts from inception output |
| `GET /api/features/{id}/bolts` | List all Bolts |
| `POST /api/features/{id}/run-bolt/{n}` | Run one Bolt through stages 3.1-3.5 |

### Team Knowledge + Learning Loop
| Endpoint | Description |
|----------|-------------|
| `GET /api/knowledge` | All team knowledge |
| `GET /api/knowledge/{agent}` | Agent's knowledge |
| `POST /api/knowledge/{agent}` | Save knowledge (topic + content) |
| `DELETE /api/knowledge/{agent}/{topic}` | Delete knowledge |
| `GET /api/features/{id}/rules` | Learned rules for feature |
| `DELETE /api/features/{id}/rules/{ruleId}` | Delete rule |

## Key Features

- **Per-stage approval gates** — 32 gates per enterprise workflow, 3-strike escape hatch
- **68-event audit trail** — full traceability in SQLite
- **Bolt-by-Bolt construction** — walking skeleton first, then ladder prompt for autonomy mode
- **Learning loop** — gate rejections become rules injected into future agent context
- **Team knowledge in DB** — per-agent knowledge entries loaded at dispatch time
- **Scope auto-detection** — intent text analyzed, scope selected, stage count determined
- **Reviewer dispatch** — product-lead and architecture-reviewer fire as independent runs
- **DB-only** — all specs, artifacts, events, state in SQLite. Nothing on disk except DB + log files

## Project Structure

```
cmd/devteam/main.go              # CLI + web server entrypoint
internal/
├── api/                          # HTTP handlers, SSE, stage endpoints
├── config/                       # YAML config loading
├── db/                           # SQLite store, migrations, audit events, bolts
├── feature/                      # Feature state machine, types
├── gate/                         # Approval gate state machine
├── init/                         # Project initialization scaffolding
├── intake/                       # Loose idea + external spec intake paths
├── pipeline/                     # RunStage, RunBolt, AdvanceStage, reviewer dispatch
├── role/                         # 12 role loader, tmux dispatcher, knowledge injection
├── spec/                         # Spec provider, writer, artifact validation
├── stage/                        # 32 stage definitions, 9 scopes, auto-detection
├── rules/                        # AIDLC phase rule loader
└── repo/                         # Cross-repo git operations
roles/                            # 12 agent INSTRUCTIONS.md files
rules/                            # AIDLC governance rules
```

## License

MIT