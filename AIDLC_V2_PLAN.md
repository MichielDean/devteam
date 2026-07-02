# AIDLC v2 Alignment Plan

## Goal

Adopt the full AIDLC v2 methodology: 5 phases, 32 stages, 11 agents, 9 scopes, 3 depth levels, 3 test strategy levels, per-stage approval gates, 68-event audit trail, Bolt-by-Bolt construction. All specs, artifacts, audit events, and state in SQLite — nothing on disk except the DB file and agent log files.

---

## Current State (post-simplification refactor)

We just simplified dispatch and status. What we have:
- 6 phases (inception, planning, construction, review, testing, delivery)
- 6 agents (pm, architect, developer, reviewer, tester, ops)
- Single smoke check per phase (no per-stage gates)
- SQLite for state + artifacts + outcomes + notes + events
- One `RunPhase` call per phase, no autopilot
- Log file output capture via tmux

What we need to become:
- 5 phases, 32 stages, 11 agents, 9 scopes, 3 depth levels, 3 test strategy levels
- Approval gate after every stage (32 gates)
- 68-event audit trail in DB
- Bolt-by-Bolt construction with autonomy modes
- All artifacts in DB, nothing on disk

---

## AIDLC v2 Structure (from awslabs/aidlc-workflows)

### 5 Phases / 32 Stages

| Phase | Stages | Purpose |
|-------|--------|---------|
| **0. Initialization** | 0.1-0.3 (3) | Workspace scaffold, detection, state init. Auto-proceed, no gates. |
| **1. Ideation** | 1.1-1.7 (7) | Intent capture, market research, feasibility, scope, team, mockups, approval. Gates at 1.1, 1.4, 1.7 (ALWAYS); 1.2, 1.3, 1.5, 1.6 (CONDITIONAL). |
| **2. Inception** | 2.1-2.8 (8) | Reverse engineering, practices, requirements, stories, mockups, app design, units, delivery planning. |
| **3. Construction** | 3.1-3.7 (7) | Functional design, NFR reqs, NFR design, infra design, code gen, build+test, CI. Per-Bolt (3.1-3.5), once (3.6-3.7). |
| **4. Operation** | 4.1-4.7 (7) | Deploy pipeline, env provisioning, deploy execution, observability, incident response, perf validation, feedback. All CONDITIONAL. |

### 10 Agents (+ 2 reviewers)

| Agent | Domain | Model tier |
|-------|--------|-----------|
| product-agent | Requirements, stories, scope, market research | opus |
| design-agent | UX/UI, wireframes, interaction design | opus |
| delivery-agent | Team formation, capacity, delivery sequencing | sonnet |
| architect-agent | App design, domain modeling, NFRs, decomposition | opus |
| platform-agent | Infrastructure, provisioning, cost (cloud-agnostic: Linux/systemd/Docker) | opus |
| devsecops-agent | Threat modeling, security scanning, DevSecOps | opus |
| developer-agent | Code implementation, code analysis | opus |
| quality-agent | Test strategy, test generation, perf validation | opus |
| pipeline-deploy-agent | CI/CD pipelines, deployment strategy | sonnet |
| operations-agent | Observability, incident response, SLOs, feedback | sonnet |

Plus 2 reviewer agents:
- product-lead-agent (reviews requirements/stories/UX)
- architecture-reviewer-agent (reviews technical design)

**Note:** `aidlc-compliance-agent` dropped (no regulatory context for our use case). `aidlc-aws-platform-agent` generalized to `platform-agent` (cloud-agnostic).

### 9 Scopes

| Scope | Stages | Depth | Test Strategy |
|-------|--------|-------|---------------|
| enterprise | 32 | Comprehensive | Comprehensive |
| feature | 32 | Standard | Standard |
| mvp | 22 | Standard | Standard |
| poc | 8 | Minimal | Minimal |
| bugfix | 7 | Minimal | Minimal |
| refactor | 8 | Minimal | Minimal |
| infra | 13 | Standard | Standard |
| security-patch | 9 | Minimal | Minimal |
| workshop | 25 | Standard | Minimal |

Auto-detection from keywords: "fix/bug" → bugfix, "refactor/clean up" → refactor, "infrastructure/deploy" → infra, "security/CVE" → security-patch, "proof of concept/prototype" → poc, "mvp/minimum viable" → mvp, "workshop/lab/training" → workshop, else → feature.

### 3 Depth Levels
- **Minimal**: Core essentials, 1-2 page artifacts, key decisions only
- **Standard**: Complete artifacts, all required sections, concise rationale
- **Comprehensive**: Full enterprise detail, compliance matrices, exhaustive NFRs

### 3 Test Strategy Levels
- **Minimal (Nyquist)**: 1 test per requirement, unit only, ~5-15 tests
- **Standard**: 5-8 tests per component, unit + integration, 75/20/5 pyramid
- **Comprehensive**: 10-15 tests per component, all test types

### Per-Stage Approval Gates

Every stage (except 0.1-0.3) ends with a gate:
- **Approve** → advance to next stage
- **Request Changes** → revision cycle (up to 3, then "Accept as-is" escape hatch)
- **Add Skipped Stage** (Ideation/Inception only) → insert a skipped stage back

### 68-Event Audit Trail

18 categories: Workflow Lifecycle (4), Phase Lifecycle (4), Stage Lifecycle (6), Session (4), Initialization (3), Navigation (4), Interaction (4), Artifact (3), Subagent (1), Utility (1), Error/Recovery (2), Construction Bolt (4), Worktree (7), Practices (4), Merge Dispatch (3), Sensors (5), Learning Loop (3), Swarm (6).

### Construction: Bolt-by-Bolt

- Bolt 1 = walking skeleton (stages 3.1-3.5 for first unit), gated
- Ladder prompt fires once: "continue autonomously, or gate every Bolt?"
- Remaining Bolts run per the chosen mode
- Stages 3.6 (build+test) and 3.7 (CI pipeline) run once at end
- Parallel Bolt batches when dependencies allow

---

## Database Schema Changes

### New tables

```sql
-- Stages (replaces our phase concept; 32 rows, static)
CREATE TABLE stage_definitions (
    id TEXT PRIMARY KEY,           -- "1.1", "2.3", etc.
    phase TEXT NOT NULL,           -- "ideation", "inception", etc.
    name TEXT NOT NULL,            -- "Intent Capture & Framing"
    lead_agent TEXT NOT NULL,      -- "aidlc-product-agent"
    supporting_agents TEXT DEFAULT '', -- JSON array
    key_artifacts TEXT DEFAULT '',     -- JSON array of artifact names
    condition TEXT NOT NULL DEFAULT 'ALWAYS', -- ALWAYS, CONDITIONAL, BROWNFIELD, etc.
    scopes TEXT DEFAULT '[]',      -- JSON array of scope names that execute this stage
    reviewer TEXT DEFAULT '',      -- reviewer agent slug if this stage has a reviewer
    sort_order INTEGER NOT NULL    -- ordering within phase
);

-- Feature workflow state (replaces phase_states)
CREATE TABLE feature_stages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feature_id TEXT NOT NULL,
    stage_id TEXT NOT NULL,        -- "1.1", "2.3"
    status TEXT NOT NULL DEFAULT 'not_started', -- not_started, in_progress, awaiting_approval, revising, completed, skipped
    revision_count INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE,
    UNIQUE(feature_id, stage_id)
);

-- Scope configuration per feature
-- (add columns to features table or separate table)
-- scope TEXT DEFAULT 'feature'
-- depth TEXT DEFAULT 'standard'
-- test_strategy TEXT DEFAULT 'standard'
-- autonomy_mode TEXT DEFAULT 'gated' -- for construction: 'gated' or 'autonomous'

-- Audit events (replaces events table, expanded to 68 types)
CREATE TABLE audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feature_id TEXT NOT NULL,
    event_type TEXT NOT NULL,      -- STAGE_STARTED, GATE_APPROVED, ARTIFACT_CREATED, etc.
    stage_id TEXT DEFAULT '',
    phase TEXT DEFAULT '',
    details TEXT DEFAULT '',       -- JSON blob with event-specific data
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
);

-- Bolts (construction units)
CREATE TABLE bolts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feature_id TEXT NOT NULL,
    bolt_number INTEGER NOT NULL,
    unit_ids TEXT DEFAULT '[]',    -- JSON array of unit-of-work IDs in this Bolt
    status TEXT NOT NULL DEFAULT 'pending', -- pending, in_progress, completed, failed
    is_walking_skeleton INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
);

-- Questions per stage (expand existing questions table)
-- Add: stage_id TEXT DEFAULT '' (in addition to existing phase column)

-- Team knowledge (two-tier: methodology in role files, team knowledge in DB)
CREATE TABLE team_knowledge (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_name TEXT NOT NULL,        -- "product-agent", "architect-agent", etc.
    topic TEXT NOT NULL,             -- "coding-standards", "api-conventions", etc.
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    UNIQUE(agent_name, topic)
);

-- Learning loop: behavioral rules from gate rejections
CREATE TABLE rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feature_id TEXT DEFAULT '',      -- empty = global rule, feature_id = feature-specific
    agent_name TEXT NOT NULL,        -- which agent this rule applies to
    stage_id TEXT DEFAULT '',        -- which stage triggered this rule (optional)
    rule_text TEXT NOT NULL,         -- the behavioral rule
    source_rejection TEXT DEFAULT '',-- the rejection notes that generated this rule
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
);
```

### Migrations

- Migration 005: `stage_definitions`, `feature_stages`, `audit_events`, `bolts` tables
- Migration 006: Add `scope`, `depth`, `test_strategy`, `autonomy_mode` columns to `features`
- Migration 007: Add `stage_id` column to `questions`, `spec_artifacts` tables
- Migration 008: `team_knowledge`, `rules` tables
- Seed `stage_definitions` with all 32 stages (adapted for 10 agents — compliance stages folded into architect/devsecops)

---

## Code Changes

### 1. Stage definitions (new package: `internal/stage/`)

- `stage_definitions.go` — the 32 stages as Go structs, seeded into DB
- `stage.go` — Stage type, phase mapping, condition evaluation
- `scope.go` — 9 scopes, auto-detection from intent text, stage set per scope

### 2. Agent definitions (expand `internal/role/`)

- 11 agent role files (replace current 6)
- 2 reviewer agent files
- Each agent has: name, domain description, model tier, special tools, knowledge loading

### 3. Pipeline rewrite (`internal/pipeline/`)

- `RunStage` replaces `RunPhase` — dispatches one stage's lead agent, waits for outcome, runs smoke check, opens approval gate
- `AdvanceStage` — moves to next stage in the scope's stage set, respecting conditions
- `EvaluateStageCondition` — determines if a CONDITIONAL stage should run (based on scope + project context)
- `PrepareBolts` — reads units-of-work from inception output, creates Bolt records
- `RunBolt` — dispatches stages 3.1-3.5 for one Bolt's units
- `LadderPrompt` — fires once after walking skeleton, sets `autonomy_mode` on the feature

### 4. Gate system (new: `internal/gate/`)

- `ApprovalGate` — represents a pending gate on a stage
- Gate states: open (awaiting user), approved, rejected (revision), accept_as_is (3-strike)
- API: `POST /api/features/{id}/stages/{stageId}/approve`
- API: `POST /api/features/{id}/stages/{stageId}/reject` with revision notes
- API: `POST /api/features/{id}/stages/{stageId}/accept-as-is`
- Pipeline pauses after `RunStage` if gate is open; resumes when user approves

### 5. Audit system (expand `internal/db/`)

- `RecordAuditEvent(featureID, eventType, stageID, phase, details)` — replaces `RecordEvent`
- 68 event types as constants
- Query API: `GET /api/features/{id}/audit` — returns full chronological event log
- Audit events drive the UI's activity feed and traceability view

### 6. API changes (`internal/api/`)

New endpoints:
- `POST /api/features/{id}/run-stage` — dispatch one stage, returns when gate opens
- `POST /api/features/{id}/stages/{stageId}/approve` — approve gate, advance
- `POST /api/features/{id}/stages/{stageId}/reject` — request changes
- `POST /api/features/{id}/stages/{stageId}/accept-as-is` — 3-strike escape
- `GET /api/features/{id}/stages` — list all stages with status
- `GET /api/features/{id}/audit` — audit trail
- `POST /api/features/{id}/scope` — change scope mid-workflow
- `POST /api/features/{id}/depth` — change depth
- `POST /api/features/{id}/test-strategy` — change test strategy
- `POST /api/features/{id}/ladder` — answer the construction autonomy prompt

Modified endpoints:
- `POST /api/features` — accepts `scope`, `depth`, `test_strategy` in request body
- `POST /api/features/{id}/run` — becomes `run-stage` (runs current stage only)

Removed endpoints:
- `POST /api/features/{id}/advance` — replaced by stage approval
- `POST /api/features/{id}/recirculate` — replaced by stage reject + revision
- `GET /api/features/{id}/gate` — replaced by per-stage gate status

### 7. UI changes (`ui/`)

- Feature detail page: stage-by-stage progress view (32 checkboxes: `[ ]`, `[-]`, `[?]`, `[R]`, `[x]`, `[S]`)
- Stage gate modal: Approve / Request Changes / Accept as-is (after 3 revisions)
- Scope selector on feature creation (9 scopes + auto-detect)
- Depth + test strategy selectors
- Construction Bolt view: shows Bolts, walking skeleton, ladder prompt
- Audit timeline: chronological event feed
- Agent output: per-stage log file viewer

---

## Execution Order

Each phase is a separate PR, tests pass between phases.

### Phase A: Database schema + stage definitions (foundation)
1. Migration 005: `stage_definitions`, `feature_stages`, `audit_events`, `bolts` tables
2. Migration 006: `scope`, `depth`, `test_strategy`, `autonomy_mode` columns on `features`
3. Migration 007: `stage_id` column on `questions`, `spec_artifacts`
4. Migration 008: `team_knowledge`, `rules` tables
5. Seed `stage_definitions` with all 32 stages (adapted: compliance stages folded into architect/devsecops, platform agent generalized)
6. `internal/stage/` package: Stage type, scope definitions (9 scopes), auto-detection from intent text
7. Tests: stage definition loading, scope routing, auto-detection

### Phase B: Agent definitions + knowledge system (expand roster)
1. Write 10 agent role files (replace current 6): product, design, delivery, architect, platform, devsecops, developer, quality, pipeline-deploy, operations
2. Write 2 reviewer agent files: product-lead, architecture-reviewer
3. Update `internal/role/` to load all 12 agents
4. Team knowledge system: `team_knowledge` DB table, API for CRUD (`GET/POST/PATCH/DELETE /api/knowledge/{agent}`), inject into agent context during dispatch
5. Tests: agent loading, model tier assignment, team knowledge injection

### Phase C: Pipeline rewrite (RunStage + gates + learning loop)
1. `RunStage` — dispatch one stage's lead agent, wait for outcome, smoke check, open approval gate
2. `AdvanceStage` — next stage in scope's stage set, respecting conditions
3. `EvaluateStageCondition` — CONDITIONAL stage logic (based on scope + project context)
4. Gate system: `internal/gate/` package — open, approved, rejected (revision), accept-as-is (3-strike)
5. Reviewer dispatch: after stage produces artifacts, if stage declares a reviewer, dispatch reviewer agent as separate run. Reviewer appends READY/NOT-READY verdict. Up to `reviewer_max_iterations` (default 2) revision cycles.
6. Learning loop: on gate rejection, save rejection notes as a rule in `rules` table. On next dispatch, load relevant rules into agent context.
7. Jump commands: `POST /api/features/{id}/jump` with `stage_id` or `phase`. Intervening stages marked `[S]`.
8. Delete `RunPhase`, `AdvanceFeature`, `RecirculateFeature` (replaced by stage flow)
9. Tests: stage dispatch, gate open/approve/reject, revision cycle, accept-as-is, reviewer dispatch, rule generation, jump

### Phase D: Audit system
1. `RecordAuditEvent` with 68 event types (adapted: drop AWS-specific events, keep generic)
2. Expand audit event recording in pipeline: STAGE_STARTED, STAGE_AWAITING_APPROVAL, STAGE_REVISING, STAGE_COMPLETED, STAGE_SKIPPED, STAGE_JUMPED, GATE_APPROVED, GATE_REJECTED, ARTIFACT_CREATED, ARTIFACT_UPDATED, SUBAGENT_COMPLETED (reviewer), RULE_LEARNED, BOLT_STARTED, BOLT_COMPLETED, etc.
3. `GET /api/features/{id}/audit` endpoint — chronological event log
4. Tests: event recording, query, chronological ordering

### Phase E: Construction Bolts (serialized)
1. `PrepareBolts` — read units-of-work from inception output (stage 2.7), create Bolt records in `bolts` table
2. `RunBolt` — dispatch stages 3.1-3.5 for one Bolt (serialized, one tmux session)
3. Ladder prompt: after walking skeleton (Bolt 1), `POST /api/features/{id}/ladder` sets `autonomy_mode` (gated or autonomous)
4. In autonomous mode, skip per-Bolt gates. Failures always halt (retry/skip/abort).
5. Stages 3.6 (build+test) and 3.7 (CI pipeline) run once at end across all Bolts
6. Halt-and-ask on failure: retry (re-run Bolt), skip (mark `[S]`), abort (stop construction)
7. Parallel Bolt batches: **serialized only for now** (ponytail debt — add parallel tmux sessions later if needed)
8. Tests: Bolt creation, walking skeleton, ladder, autonomous vs gated mode, halt-and-ask

### Phase F: API + UI
1. New API endpoints:
   - `POST /api/features/{id}/run-stage` — dispatch current stage
   - `POST /api/features/{id}/stages/{stageId}/approve` — approve gate
   - `POST /api/features/{id}/stages/{stageId}/reject` — request changes (body: revision notes)
   - `POST /api/features/{id}/stages/{stageId}/accept-as-is` — 3-strike escape
   - `POST /api/features/{id}/jump` — jump to stage/phase (body: stage_id or phase)
   - `GET /api/features/{id}/stages` — all stages with status
   - `GET /api/features/{id}/audit` — audit trail
   - `POST /api/features/{id}/scope` — change scope
   - `POST /api/features/{id}/depth` — change depth
   - `POST /api/features/{id}/test-strategy` — change test strategy
   - `POST /api/features/{id}/ladder` — answer construction autonomy prompt
   - `GET/POST/PATCH/DELETE /api/knowledge/{agent}` — team knowledge CRUD
   - `GET /api/features/{id}/rules` — learned rules for a feature
2. Remove old endpoints: `advance`, `recirculate`, `gate`, `process`, `run` (replaced by `run-stage`)
3. UI: stage-by-stage progress view (32 checkboxes: `[ ]`, `[-]`, `[?]`, `[R]`, `[x]`, `[S]`), gate modal (Approve/Request Changes/Accept-as-is), scope selector, depth/test-strategy selectors, Bolt view, audit timeline, team knowledge editor, agent output per-stage
4. Playwright e2e tests: full workflow with per-stage gates, scope selection, Bolt flow, jump commands

### Phase G: Cleanup + docs
1. Delete old phase-based code: `phase_states` table usage, `GateDefinitions`, `Phase` type, old 6 agent files, `devteam.yaml` phase definitions
2. Scope auto-detection on feature creation with confirmation prompt
3. Update AGENTS.md: 10 agents, 32 stages, 9 scopes, per-stage gates, DB-only, team knowledge, learning loop
4. Update README.md: v2 alignment status
5. Full e2e test: create feature → auto-detect scope → run all stages with gates → delivery

---

## What Gets Deleted

- `internal/feature/state.go` — GateDefinitions, RecirculationTarget (replaced by stage flow)
- `internal/pipeline/smoke.go` — phase-based smoke checks (replaced by per-stage smoke + reviewer agents)
- `internal/pipeline/instructions.go` — phase instructions (replaced by per-stage instructions)
- `internal/feature/types.go` — Phase type (replaced by Stage type), 6 phase constants
- `internal/api/server.go` — advanceFeature, recirculateFeature, evaluateGate, runPhase handlers
- Old 6 agent role files (pm, architect, developer, reviewer, tester, ops)
- `devteam.yaml` phase definitions (replaced by stage definitions in DB)
- `internal/pipeline/pipeline.go` — RunPhase, AdvanceFeature, RecirculateFeature (replaced by RunStage, AdvanceStage)
- `phase_states` table usage (replaced by `feature_stages`)

## What Stays

- `internal/role/dispatcher.go` + `tmux.go` — dispatch mechanics (unchanged from simplification refactor)
- `internal/db/` — SQLite infrastructure, artifact store, outcome store, feature repo store (expanded with new tables)
- `internal/repo/` — repo management (unchanged)
- `internal/gitops/` — git operations (unchanged)
- `internal/config/` — config loading (expanded for scopes/depth)
- Log file output capture (unchanged)
- SSE streaming (unchanged)
- `devteam signal` CLI (unchanged — agent signals stage completion)

---

## Risks & Mitigations

1. **32 stages × 1 agent dispatch each = 32 LLM calls per feature.** Cost concern. Mitigation: scopes reduce stage count (bugfix = 7, poc = 8). User sees stage count before confirming. Autonomy mode in construction batches Bolts. Reviewer agents add ~7 more dispatches for enterprise scope.

2. **Per-stage gates = 32 pauses per feature.** UX concern. Mitigation: UI makes approval one click. Construction autonomy mode skips per-Bolt gates. Initialization (0.1-0.3) has no gates. Conditional stages skipped automatically. Jump commands let users skip ahead.

3. **12 agent role files to write.** Mitigation: port from AIDLC v2's `core/agents/` (MIT-0 licensed), adapt for our dispatch model and cloud-agnostic platform. Each is ~200 lines of markdown.

4. **Schema migration from 6-phase to 32-stage.** Mitigation: migration creates `feature_stages` from scratch. Old `phase_states` not migrated (features start fresh — already wiped). `stage_definitions` seeded, not migrated.

5. **Learning loop could generate bad rules.** A rejected gate produces a rule that over-constrains future agents. Mitigation: rules are feature-scoped by default (empty `feature_id` = global, set = feature-specific). UI shows learned rules, user can delete. Rules are injected as context, not hard constraints.

6. **Team knowledge in DB needs a management UI.** Mitigation: simple CRUD API + UI editor. Teams edit knowledge per-agent. Loaded into context at dispatch time. No file management.

7. **Serialized Bolts slower for large features.** Mitigation: acceptable for now (ponytail debt). Most features have 3-8 Bolts. Parallel tmux sessions can be added later without changing the stage/gate model.

---

## Estimated effort

| Phase | Effort | Why |
|-------|--------|-----|
| A: Schema + stages + scopes | 2-3 days | 4 migrations + 32 stage definitions (adapted) + 9 scopes + auto-detection |
| B: Agents + knowledge | 2-3 days | 12 agent files (port+adapt from AIDLC v2) + team knowledge DB + API |
| C: Pipeline + gates + learning loop | 4-5 days | RunStage, gate system, reviewer dispatch, revision cycle, accept-as-is, rules, jumps |
| D: Audit | 1-2 days | 68 event types (adapted), recording across pipeline, query endpoint |
| E: Bolts (serialized) | 2-3 days | Bolt planning, walking skeleton, ladder, halt-and-ask, no parallelism yet |
| F: API + UI | 4-5 days | 15+ new endpoints, stage progress view, gate modal, Bolt view, audit timeline, knowledge editor, e2e tests |
| G: Cleanup + docs | 1-2 days | Delete old code, scope auto-detection, AGENTS.md, README.md, full e2e |
| **Total** | **16-23 days** | |

---

## Decisions (confirmed)

1. **Full v2 alignment** — 5 phases, 32 stages, 11 agents, 9 scopes, 3 depth levels, 3 test strategy levels, per-stage approval gates, 68-event audit trail, Bolt construction.
2. **DB only** — all specs, artifacts, audit events, state, team knowledge, and rules in SQLite. Nothing on disk except DB file and agent log files.
3. **Per-stage gates** — pause after every stage. User approves/rejects/accepts-as-is. UI drives progression.
4. **Generalize platform, drop compliance** — rename `aidlc-aws-platform-agent` → `platform-agent` (cloud-agnostic: Linux/systemd/Docker/Vagrant). Drop `aidlc-compliance-agent` entirely. **10 agents + 2 reviewers = 12 total.**
5. **Serialize Bolts** — run Bolts one at a time, one tmux session per feature. Parallelism is a future optimization (ponytail debt).
6. **Separate reviewer dispatches** — product-lead and architecture-reviewer fire as separate agent runs after stages that declare a reviewer. Independent review.
7. **Two-tier knowledge, team knowledge in DB** — methodology knowledge in agent role files (read-only). Team knowledge stored in SQLite `team_knowledge` table, loaded per-agent into context. Consistent with DB-only decision.
8. **Adopt learning loop** — gate rejections saved as rules in DB `rules` table. Future stages load relevant rules into agent context. Prevents repeating mistakes.
9. **Both --stage and --phase jumps** — `POST /api/features/{id}/jump` with `stage_id` or `phase`. Intervening stages marked `[S]` skipped.