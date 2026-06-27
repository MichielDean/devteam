# SpecKit + AIDLC Integration

## Problem

The Dev Team pipeline reinvents what SpecKit already does well. PM agents write specs from scratch instead of using SpecKit's `/speckit.specify` command. Architects write plans from scratch instead of `/speckit.plan`. The quality is inconsistent because we're not using SpecKit's proven template-driven approach.

## Solution

Integrate SpecKit commands into the pipeline:

### Inception (PM)
1. PM asks clarifying questions (AIDLC pattern) — writes questions.json
2. Pipeline pauses for user feedback — UI shows questions
3. User answers through UI — Go code stores answers, resumes inception
4. PM reads answers, checks if more questions needed — repeat if so
5. When enough clarity, PM uses SpecKit's spec template to write spec.md, acceptance.md, repos.yaml
6. PM writes `pass` — inception done
7. Inception should almost NEVER fail — it's just a question-answer loop

### Planning (Architect)  
1. Architect reads spec.md
2. Uses SpecKit's plan template to generate plan.md, research.md, data-model.md, contracts/
3. Uses SpecKit's tasks template to generate tasks.md
4. Architect writes `pass` — planning done

### Construction (Developer)
1. Developer reads plan.md, tasks.md, data-model.md, contracts/
2. Implements per tasks.md
3. Commits and pushes
4. Writes `pass` — construction done

### Review, Testing, Delivery — unchanged

## Key Principles
- Agents NEVER touch SQLite — Go code handles all state
- Agents only write files — Go code reads, parses, manages state
- SpecKit templates constrain LLM output for quality (per spec-driven.md)
- Inception is iterative — ask, answer, ask more, write spec
- Worktree is the only place agents work — primary checkout untouched
