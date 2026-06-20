# Dev Team

A multi-agent development platform with predefined specialist roles, spec-driven workflow, and cross-repository feature support.

## Status

**v0.2.0** — Full pipeline engine implemented. Agent dispatch, gate evaluation, convergence detection, advance/recirculate commands, init scaffolding. 50+ tests passing. Self-bootstrap working.

## Architecture

Dev Team has 6 specialist roles working through a fixed pipeline:

1. **Product Manager** — Owns the *what* and *why*. Explores and refines ideas into specs.
2. **Architect** — Owns the *how*. Creates technical plans and task breakdowns.
3. **Developer** — Writes code across repos. Follows spec + plan.
4. **Code Reviewer** — Adversarial review against spec acceptance criteria.
5. **Tester** — Writes and runs tests traced to user stories.
6. **Release Engineer** — Owns deployment, docs, and cross-repo coordination.

## Two Intake Paths

- **Loose Ideas**: Submit a rough description. The PM explores, clarifies, and refines it into a structured spec.
- **External Specs**: Bring in a PRD, RFC, or roadmap. The PM decomposes it into N feature specs with dependency edges.

Both produce the same output: `spec.md` + `acceptance.md` + `repos.yaml`.

## Central Spec Repository

Specs live in one place — this repo. Features that span multiple implementation repos have one spec, not fragmented copies across repos. Each implementation repo gets a thin `.devteam/pointer.yaml` back to the central spec.

## Pipeline

```
Inception → Planning → Construction → Review → Testing → Delivery
  (PM+Arch)   (Arch)     (Dev)       (Reviewer) (Tester)   (Ops)
```

Each phase has a gate. You can't skip phases. Failed gates recirculate to the correct earlier phase.

## Quick Start

```bash
# Build
cd ~/source/devteam
go build -o ~/go/bin/devteam ./cmd/devteam/

# Initialize a new project
devteam init

# Check status
devteam status

# Submit a loose idea
devteam intake --type loose --text "We need user authentication" --priority 1

# Run current phase (dispatches agents)
devteam run 001-user-auth

# Evaluate current gate
devteam gate 001-user-auth

# Advance to next phase after gate passes
devteam advance 001-user-auth

# Recirculate back to a previous phase
devteam recirculate 001-user-auth planning

# Self-bootstrap
devteam bootstrap
```

## Commands

| Command | Description |
|---------|-------------|
| `devteam init` | Initialize a new devteam project (scaffolds directory structure) |
| `devteam status` | Show all features and their current phase |
| `devteam intake` | Submit a new feature (loose idea or external spec) |
| `devteam run <id>` | Run the current pipeline phase (dispatches agents) |
| `devteam gate <id>` | Evaluate the current phase gate |
| `devteam advance <id>` | Advance feature to next phase after gate passes |
| `devteam recirculate <id> <phase>` | Send feature back to a previous phase |
| `devteam bootstrap` | Process spec 001 (self-bootstrap) |
| `devteam version` | Print version |

## Pipeline Flow

1. **Intake**: Submit a feature via `devteam intake`
2. **Run phase**: `devteam run <id>` dispatches agents with role instructions + AIDLC rules + spec context
3. **Check gate**: `devteam gate <id>` evaluates whether the phase gate passes
4. **Advance**: `devteam advance <id>` moves to the next phase
5. **Recirculate** (on failure): `devteam recirculate <id> <phase>` goes back to fix issues

## Convergence Detection

Dev Team detects when implementation has drifted from spec:
- Unrefined placeholder text in spec.md or acceptance.md
- Missing sections (user stories, requirements, success criteria)
- Plan sections that don't cover all spec requirements
- Priority-1 features missing security review

## Hybrid Framework

| Aspect | From AIDLC | From Spec Kit | Dev Team Original |
|--------|-----------|---------------|-------------------|
| Phase governance | Adaptive rules per role | — | ✓ |
| Artifact structure | — | Templates (spec.md, plan.md, tasks.md) | ✓ |
| Quality gates | Phase gate reviews | checklist, analyze, converge | ✓ |
| Extensions | Security, resiliency, testing | Community extensions | ✓ |
| Human-in-the-loop | File-based approval gates | — | ✓ |
| Multi-repo support | — | — | ✓ (central spec + repos.yaml) |
| Distinct role agents | — | — | ✓ (6 fixed roles) |
| Self-bootstrap | — | — | ✓ (platform processes its own spec) |
| Intake paths | — | — | ✓ (loose ideas + external specs) |
| Convergence detection | — | — | ✓ (spec drift checking) |

## Project Structure

```
cmd/devteam/main.go           # CLI entrypoint
internal/
├── config/                    # YAML config loading
├── feature/                   # Feature state machine, types, gates
├── init/                       # Project initialization scaffolding
├── intake/                     # Loose idea + external spec intake paths
├── pipeline/                  # Pipeline orchestrator, gate evaluation, convergence
├── role/                       # Role loader, agent dispatcher
├── spec/                       # Spec provider, writer, artifact validation
├── rules/                      # AIDLC phase rule loader
└── repo/                       # Cross-repo git operations, pointer files
specs/                           # Central spec repository
roles/                           # 6 role INSTRUCTIONS.md files
rules/                           # AIDLC governance rules
constitution/                    # 10 governing principles
devteam.yaml                     # Pipeline configuration
repos.yaml                       # Repository registry
```

## License

MIT