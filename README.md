# Dev Team

A multi-agent development platform with predefined specialist roles, spec-driven workflow, and cross-repository feature support.

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

Specs live in one place — this repo. Features that span multiple implementation repos have one spec, not fragmented copies across repos. Each implementation repo gets a thin `.devteam/` pointer back to the central spec.

## Pipeline

```
Inception → Planning → Construction → Review → Testing → Delivery
  (PM+Arch)   (Arch)     (Dev)       (Reviewer) (Tester)   (Ops)
```

Each phase has a gate. You can't skip phases.

## Quick Start

```bash
# Install specify CLI (Spec Kit)
uv tool install specify-cli --from git+https://github.com/github/spec-kit.git@v0.11.2

# Create a new feature spec
cd ~/source/devteam
specify create-new-feature --name "feature-name"

# Or submit a loose idea for the PM to refine
# (handled by the orchestrator once built)
```

## Repository Structure

```
devteam/
├── specs/                    # Central spec repository
│   └── 001-dev-team-platform/
│       ├── spec.md
│       ├── acceptance.md
│       └── repos.yaml
├── constitution/             # Project governing principles
│   └── constitution.md
├── roles/                    # Role definitions with INSTRUCTIONS.md
│   ├── pm/
│   ├── architect/
│   ├── developer/
│   ├── reviewer/
│   ├── tester/
│   └── ops/
├── rules/                    # AIDLC phase governance rules
│   ├── aidlc/
│   └── aidlc-rule-details/
├── .specify/                 # Spec Kit configuration
├── devteam.yaml              # Team configuration
└── repos.yaml                # Repository registry
```

## Hybrid Framework

Dev Team takes the best from two open-source frameworks:

| Aspect | From AIDLC | From Spec Kit |
|--------|-----------|---------------|
| Phase governance | Adaptive rules per role | — |
| Artifact structure | — | Templates (spec.md, plan.md, tasks.md) |
| Quality gates | Phase gate reviews | checklist, analyze, converge |
| Extensions | Security, resiliency, testing | Community extensions |
| Human-in-the-loop | File-based approval gates | — |
| Multi-repo support | — | — (original contribution) |
| Distinct role agents | — | — (original contribution) |

## License

MIT