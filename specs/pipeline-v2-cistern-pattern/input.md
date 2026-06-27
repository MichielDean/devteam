# Pipeline V2: Cistern Pattern Recirculation

## Problem

The Dev Team pipeline has fundamental architectural issues:
1. Gate evaluator does substring matching with `|| true` — not real verification
2. When testing fails, it re-runs testing 5+ times instead of sending back to construction
3. Agents run in the primary checkout, not a spec worktree — code gets lost
4. No notes mechanism — phases don't communicate structured feedback
5. SQLite database exists but isn't wired into the pipeline
6. Questions loop infinitely because questions.json isn't cleaned up

## Solution

Rebuild the pipeline using Cistern's proven patterns:

### 1. Agent is the evaluator (not a gate)
- Agent reads spec, reviews code, writes an outcome file: `pass`, `recirculate:<target>`, or `pool`
- Pipeline reads the outcome and routes accordingly
- Gate evaluator becomes a fallback safety check, not the primary decision

### 2. Outcome-based routing (Cistern pattern)
- `pass` → advance to next phase (OnPass)
- `recirculate:construction` → send back to construction with notes (OnRecirculate)
- `pool` → mark as blocked, notify user
- Pipeline is a routing engine, not a decision maker

### 3. Notes in SQLite with cycle boundaries
- Each phase writes notes to SQLite after completion
- Notes include: phase, role, type (summary/finding/warning/handoff), content
- Cycle boundary detection: walk notes newest→oldest, break at pass-signals
- Reviewer sees only own notes; implementer sees all reviewer notes

### 4. Worktree always created before first phase
- EnsureSpecWorktree runs before EVERY phase dispatch
- Worktree path injected as CWD for all agents
- Branch `spec/<id>` preserved across recirculations — work is incremental
- Agent commits to the branch; pipeline doesn't need to commit separately

### 5. Recirculation sends back with notes
- Tester finds issue → writes `recirculate:construction` + notes
- Pipeline routes to construction
- Construction agent gets notes in CONTEXT.md: "⚠️ REVISION REQUIRED"
- Construction fixes → writes `pass`
- Pipeline routes back to testing
- Testing agent sees revision notes, verifies fixes first, then does fresh testing

### 6. No re-running on failure
- If agent writes `recirculate`, pipeline routes to target — never re-runs current phase
- If agent writes `pass`, pipeline advances — no gate re-evaluation
- Gate evaluator runs as a safety check AFTER agent says pass — if gate fails, it recirculates

## Phases

### Inception (PM)
- Reads input, asks questions, writes spec
- Outcome: `pass` (spec complete) or `pool` (needs human input beyond questions)

### Planning (Architect)  
- Reads spec, writes plan/tasks/research/data-model/contracts
- Outcome: `pass` or `recirculate:inception` (spec has gaps)

### Construction (Developer)
- Reads plan/tasks, writes code, commits to branch
- Outcome: `pass` (build succeeds, code written) or `pool` (blocked)

### Review (Reviewer)
- Reads spec, reviews code against acceptance criteria
- Outcome: `pass` (all criteria met) or `recirculate:construction` (issues found)
- Writes structured issues to SQLite with evidence

### Testing (Tester)
- Writes and runs tests
- Outcome: `pass` (all tests pass) or `recirculate:construction` (implementation bugs) or `recirculate:review` (test reveals spec gap)
- Writes test results to SQLite

### Delivery (Ops)
- Writes documentation
- Outcome: `pass` (docs complete) or `recirculate:construction` (needs code changes for docs)
- On pass: mark done, create PR
