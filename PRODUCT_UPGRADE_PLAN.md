# Product Upgrade Plan — Second Pass

## Goal

Take Dev Team from hobby project to real open-source product. Two pillars:

1. **UI that feels purpose-built** — not a dashboard bolted onto an API, but a workflow cockpit designed around the back-and-forth rhythm of human-agent collaboration.
2. **Persistent tmux sessions** — agents resume existing sessions when questions are answered, preserving context across stage boundaries. No more cold-starting every dispatch.

---

## What's Wrong Now (honest audit)

### UI Problems

| Problem | Evidence |
|---------|----------|
| **Monolithic FeatureDetail** | 590 lines in one file. 8 useQuery calls, 11 mutations, 6 conditional banners, inline jump/scope/depth/cancel controls. Unreadable, unmaintainable. |
| **No live agent interaction** | AgentOutput polls `/output` every 2s via HTTP. SSE `agent_output` events are broadcast but **not consumed by AgentOutput** — they go to useSSE which only invalidates queries. The live stream is wasted. User can't see what the agent is doing in real time. |
| **Gate modal is disconnected** | GateModal receives `smokeFailures` and `reviewerVerdict` as props, but FeatureDetail.tsx never passes them. The modal only shows approve/reject/accept-as-is — no artifact preview, no reviewer notes, no diff, no context about what was produced. User approves blind. |
| **Artifacts are an afterthought** | ArtifactViewer is a separate panel at the bottom, passes `phaseStates={{}}` (always empty). No connection between a stage completing and its artifacts appearing. User has to manually look up what was produced. |
| **No stage detail view** | StageProgress shows 32 checkboxes with stage IDs ("1.1", "2.3"). No stage names, no descriptions, no artifact list, no click-to-expand. User has to memorize what each number means. |
| **Questions are a wall** | All questions render as cards stacked vertically. No grouping by phase/role. Answer summary is a separate section at the bottom. Hard to scan when there are 5+ questions. |
| **Bolt view is static** | Bolts show status only. No unit breakdown, no per-Bolt stage progress, no way to run a specific Bolt from the UI (the API exists, the button doesn't). |
| **No global state visibility** | No view of active tmux sessions, active agents, system health. KnowledgeEditor is crammed at the bottom of the Dashboard. Rules only show on FeatureDetail. |
| **No keyboard shortcuts** | Every action is a click. For a tool that involves 32 approval gates, this is exhausting. |
| **No responsive layout** | Grid layouts assume desktop. Mobile/tablet unusable. |
| **No loading states for mutations** | Buttons show "Starting..." but gate approve/reject/jump have no pending state — user clicks twice, double-fires. |
| **SSE event types are wrong** | useSSE listens for `stage_change`, `gate_result`, `phase_change` — but the server broadcasts `agent_output`, `processing_complete`, `interrupted`, `question_answered`. The event type list in useSSE doesn't match what the server sends. Events are partially handled. |

### Tmux Session Problems

| Problem | Evidence |
|---------|----------|
| **Kill-and-recreate every dispatch** | `tmux.go:79` — `m.KillSession(sessionName)` unconditionally before every dispatch. Every stage = new session = cold context. Agent loses all memory of prior conversation. |
| **One session per feature, not per phase** | Session name is `devteam-{featureID}`. No phase or stage in the name. Can't have ideation and inception sessions alive simultaneously. |
| **No resume capability** | No API to attach to an existing session, send input, or resume after a question is answered. The only operations are kill, check-alive, capture-output. |
| **Context dir is ephemeral** | `prepareContextDir` creates a temp dir, writes CONTEXT.md + agent.md, deletes it after dispatch (`defer os.RemoveAll`). Next dispatch rebuilds from scratch. No continuity. |
| **No session metadata** | No tracking of which session is for which stage, what its state is (running, waiting, idle), or what the last outcome was. |
| **Log files clobber each other** | `logPath = phase-role.log` — reviewer and lead agent for the same phase overwrite each other's logs. No stage ID in the filename. |
| **No inter-agent communication** | Agents can't hand off context to the next agent in the same phase. Each starts fresh, re-derives what the prior agent already figured out. |

---

## Design Principles (product-grade)

1. **The UI is a workflow cockpit, not a CRUD dashboard.** The primary view is the active stage — what's happening right now, what the agent is producing, what the user needs to do next. Everything else is secondary navigation.

2. **Live first, polling second.** SSE drives real-time updates. HTTP polling is only for reconnection/missed events. No 2-second poll loops for data that's already streamed.

3. **Context is king.** When a gate opens, the user sees the artifacts, the reviewer's notes, the smoke check results, and the diff — all in one view. No clicking through tabs to find what the agent did.

4. **Sessions are persistent boundaries.** A tmux session lives for an entire phase (or stage cluster). Questions pause the session; answers resume it. Agents within the same phase share a session and see each other's context.

5. **Keyboard-driven.** Approve with `A`, reject with `R`, run next stage with `Enter`. Power users shouldn't need a mouse for the 32-gate workflow.

6. **Progressive disclosure.** The stage progress bar is always visible but compact. Click a stage to expand its full detail (artifacts, audit history, agent output). Overwhelming detail is opt-in.

7. **Null-safe by construction.** Every API response is normalized at the client boundary. No `undefined` propagates to render. No crash on missing data.

---

## Pillar 1: Persistent Tmux Sessions

### Session Model

**Current:** One session per feature, killed and recreated every dispatch.

**New:** Sessions are **phase-scoped, persistent, and resumable.**

```
Session naming: devteam-{featureID}-{scope}
  e.g. devteam-abc123-ideation
       devteam-abc123-inception
       devteam-abc123-construction-bolt1    ← per-Bolt sessions
       devteam-abc123-construction-bolt2
       devteam-abc123-operation
```

- **Phase sessions** (ideation, inception, operation): one session per phase. All stages within the phase run in the same session.
- **Construction sessions**: one session per Bolt. Stages 3.1-3.5 for a Bolt share a session. The walking skeleton Bolt has its own session. This enables future parallel Bolt execution without session conflicts.

### Session Lifecycle

```
┌─────────────┐     ┌──────────┐     ┌──────────────┐     ┌──────────┐
│  CREATED    │────▶│ RUNNING  │────▶│ AWAITING     │────▶│ DONE     │
│ (new tmux)  │     │ (agent   │     │ GATE/QUESTION│     │ (keep    │
│             │     │  active) │     │ (session     │     │  alive)  │
│             │     │          │     │  paused)     │     │          │
└─────────────┘     └──────────┘     └──────────────┘     └──────────┘
      │                   │                   │
      │                   ▼                   ▼
      │             ┌──────────┐       ┌──────────────┐
      │             │ FAILED   │       │ RESUMING     │
      │             │ (error,  │       │ (send answer │
      │             │  keep    │       │  to session, │
      │             │  alive)  │       │  continue)   │
      │             └──────────┘       └──────────────┘
      ▼
┌─────────────┐
│ EXPIRED     │  (session killed after feature done or phase advanced past)
└─────────────┘
```

**Key change: sessions are NOT killed when a gate opens or a question is asked.** They stay alive in a paused state. When the user approves or answers, the session resumes — either by sending input to the existing tmux pane, or by dispatching the next stage's agent into the same session with accumulated context.

### Session State in DB

New table `tmux_sessions`:

```sql
CREATE TABLE tmux_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feature_id TEXT NOT NULL,
    phase TEXT NOT NULL,              -- ideation, inception, construction, operation
    bolt_number INTEGER DEFAULT 0,   -- 0 for non-construction, 1+ for construction Bolts
    stage_id TEXT DEFAULT '',         -- current stage running in this session
    session_name TEXT NOT NULL UNIQUE, -- devteam-{featureID}-{phase} or devteam-{featureID}-construction-bolt{N}
    state TEXT NOT NULL DEFAULT 'created', -- created, running, awaiting_gate, awaiting_question, done, failed, expired
    context_dir TEXT NOT NULL,        -- persistent context dir path (NOT temp)
    last_agent TEXT DEFAULT '',       -- last agent role that ran
    last_output_at TIMESTAMP,         -- last time output was seen
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
);
```

### Context Directory (persistent)

**Current:** `os.MkdirTemp("", "devteam-"+req.Role+"-*")` — temp dir, deleted after dispatch.

**New:** `~/.local/share/devteam/sessions/{featureID}/{phase}/` — persistent, survives across dispatches.

Structure:
```
~/.local/share/devteam/sessions/{featureID}/{phase}/
├── CONTEXT.md          # Generated fresh before each stage dispatch from DB state
├── agents/
│   ├── product.md      # Role file for current agent
│   ├── architect.md
│   └── ...
└── logs/
    ├── 1.1-product.log    # Stage ID + agent in filename (no clobbering)
    ├── 1.3-architect.log
    ├── 1.6-design.log
    └── 2.3-product-lead-review.log
```

### Context is DB-Driven, Not File-Driven

**No `conversation.log`.** All context that an agent needs is retrieved from the database at dispatch time. The pipeline's `buildStageContext` already does this:

- **Artifacts** → `spec_artifacts` table (produced by prior stages)
- **Audit events** → `audit_events` table (what happened so far)
- **Notes** → `notes` table (revision notes, summaries)
- **Rules** → `rules` table (learned rules for this agent)
- **Team knowledge** → `team_knowledge` table (per-agent knowledge)
- **Questions + answers** → `questions` table (human responses)
- **Feature state** → `features` + `feature_stages` tables

The CONTEXT.md is **generated fresh** from DB state before each dispatch. It includes:
1. Feature metadata (scope, depth, current stage)
2. Key artifacts from prior stages (from `spec_artifacts`)
3. Questions and human responses (from `questions`)
4. Revision notes if this stage was rejected before (from `notes`)
5. Learned rules for this agent (from `rules`)
6. Team knowledge for this agent (from `team_knowledge`)
7. Audit events summary (from `audit_events` — last N events)

The persistent session directory holds only:
- `CONTEXT.md` (regenerated each dispatch)
- `agents/` (role files)
- `logs/` (per-stage output logs, never clobbered)

The tmux session itself provides continuity — the agent's prior output is visible in the pane history. But the structured context comes from the DB, not from a file we accumulate.

**Why this is better:** No file accumulation, no truncation logic, no context bloat. DB queries are bounded and fast. Agents get exactly the context they need, structured and queryable. If the DB says it, the agent knows it. Single source of truth.

### Resume After Question

**Flow:**
1. Agent asks a question → signals `needs_feedback` → session stays alive, state = `awaiting_question`
2. User answers questions in UI → `POST /api/features/{id}/questions/{qid}` (existing)
3. Server updates question answers in DB → session state = `resuming`
4. Server re-dispatches the same stage into the existing tmux session
5. `buildStageContext` runs again — now includes the answered questions from DB
6. Agent starts fresh in the same session, but CONTEXT.md now has the answers

**Decision: re-dispatch approach.** The agent process from the prior dispatch has exited (it signaled `needs_feedback`). We start a new agent in the same tmux session with a fresh CONTEXT.md generated from DB state (now including answers). No race conditions, no send-keys fragility. The tmux session provides visual continuity (prior output visible in pane history) while the DB provides structured context.

### API Changes (tmux sessions)

New endpoints:
- `GET /api/features/{id}/sessions` — list all tmux sessions for a feature with state
- `POST /api/features/{id}/sessions/{phase}/resume` — resume a paused session (re-dispatch current stage with fresh DB context)
- `POST /api/features/{id}/sessions/{phase}/kill` — kill a session (cleanup)
- `GET /api/features/{id}/sessions/{phase}/output` — get per-stage output from log files
- `GET /api/features/{id}/sessions/{phase}/pane` — raw tmux capture-pane output (for the terminal viewer)

Modified:
- `runStage` — creates or reuses session for the feature+phase. Does NOT kill existing session. Regenerates CONTEXT.md from DB.
- `answerQuestion` — triggers session resume for the relevant phase

### Implementation Files

| File | Change |
|------|--------|
| `internal/role/tmux.go` | Session naming with phase. No unconditional kill. Persistent context dirs. Log filenames with stage ID. `ResumeSession` method. `SendInput` method. |
| `internal/role/dispatcher.go` | `DispatchRequest` gains `SessionName` and `ContextDir` fields. Dispatcher reuses existing sessions. |
| `internal/db/session_store.go` | New `tmux_sessions` table CRUD. |
| `internal/db/migration_009.go` | Create `tmux_sessions` table. |
| `internal/pipeline/stage_runner.go` | `RunStage` resolves or creates a session for the feature+phase. Passes session name to dispatcher. Writes conversation.log. |
| `internal/pipeline/session_manager.go` | New file: session lifecycle management (create, resume, expire, cleanup). |
| `internal/api/session_handlers.go` | New endpoints: list sessions, resume, kill, get output. |

---

## Pillar 2: UI Product Upgrade

### Architecture: Component Decomposition

**Current:** FeatureDetail.tsx is a 590-line monolith.

**New:** FeatureDetail is a layout shell with tabs/panels. Each panel is a focused component.

```
FeatureDetail (layout shell, ~100 lines)
├── FeatureHeader                    (status, scope, depth, priority — compact)
├── StageRail (left sidebar)         (32 stages, expandable, click to select)
├── StageDetail (main panel)         (selected stage: artifacts, output, gate)
│   ├── StageSummary                 (name, agent, status, artifacts produced)
│   ├── ArtifactPanel                (rendered artifacts for this stage)
│   ├── AgentOutputLive              (SSE-driven live output, not polling)
│   ├── GatePanel                    (approve/reject/accept-as-is with context)
│   └── RevisionHistory              (prior revision notes for this stage)
├── QuestionPanel (right panel)      (questions for current stage, grouped)
├── BoltPanel (construction only)    (Bolt list, per-Bolt progress, run buttons)
├── AuditDrawer (collapsible)        (audit timeline, slide-in from right)
└── ControlBar (bottom)              (jump, scope, depth, cancel — compact)
```

### New Components

#### 1. StageRail (left sidebar)

Replaces StageProgress. A vertical rail showing all 32 stages grouped by phase. Each stage is a clickable row:

```
 Ideation
  ✓ 1.1 Intent Capture & Framing     [product]
  ✓ 1.2 Market Research               [product]
  ✓ 1.3 Feasibility & Constraints     [architect]
  ▶ 1.4 Scope Definition              [product]  ← current
  [ ] 1.5 Team Formation              [delivery]
  [ ] 1.6 Rough Mockups               [design]
  [S] 1.7 Approval & Handoff          [delivery]

 Inception
  [ ] 2.1 Reverse Engineering         [developer]
  ...
```

- Status icons: `✓` completed, `▶` in progress, `[?]` awaiting approval, `[R]` revising, `[ ]` not started, `[S]` skipped
- Click any stage → loads StageDetail for that stage
- Current stage is highlighted
- Revision count badge if >0
- Reviewer badge if stage has a reviewer
- Collapsible phases (click phase header to collapse)

#### 2. StageDetail (main panel)

The core of the product. When you click a stage in the rail, this panel shows everything about that stage:

**Sections (tabbed or stacked):**
- **Overview**: Stage name, lead agent, supporting agents, key artifacts expected, reviewer
- **Artifacts**: Every artifact produced by this stage, rendered as markdown. If artifact not yet produced, shows "Expected: {artifact name}" placeholder.
- **Agent Output**: Live SSE-driven output for this specific stage. Not polling — real-time stream. Scrollable, searchable, with timestamp. Shows only this stage's output (filtered from the session log).
- **Gate**: If stage is awaiting approval, shows the gate panel inline (not a modal). Includes: reviewer verdict + notes, smoke check results, artifact preview, and approve/reject/accept-as-is buttons.
- **Revisions**: If the stage was rejected, shows the revision notes and prior attempts. Each revision is a collapsible entry.
- **Audit**: Audit events filtered to this stage only.

#### 3. AgentOutputLive (replaces AgentOutput)

**Current:** Polls `/output` every 2s. Ignores SSE.

**New:** Consumes SSE `agent_output` events directly. Falls back to HTTP `/output` only on reconnect or initial load.

```typescript
// Uses useSSE's onEvent callback to append lines directly
const { connected } = useSSE(featureId, (event) => {
  if (event.type === 'agent_output') {
    appendLine(event.data.line);
  }
});
```

Features:
- Real-time line streaming (no 2s delay)
- Search/filter within output
- Timestamp per line
- Auto-scroll with "jump to bottom" button
- Pause stream button (for reading while agent runs)
- Copy output button
- Per-stage filtering (show only current stage's output)
- Connection indicator (green = live, yellow = reconnecting, red = disconnected)

#### 4. GatePanel (replaces GateModal)

**Current:** Modal with approve/reject/accept-as-is. No context. Smoke failures and reviewer notes are passed as props but never populated.

**New:** Inline panel (not modal) that shows full context before asking for a decision:

```
┌─────────────────────────────────────────────────────────┐
│ Stage 2.3: Requirements Analysis                         │
│ Agent: product · Reviewer: product-lead · 0 revisions   │
├─────────────────────────────────────────────────────────┤
│ ✓ Reviewer: READY                                        │
│   "Requirements are complete and well-structured."       │
│                                                          │
│ Artifacts produced:                                      │
│   • requirements.md (2.4 KB)  [view]                     │
│                                                          │
│ Quality checks: passed                                   │
│                                                          │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ Approve  ·  Request Changes  ·  Accept as-is        │ │
│ └─────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

When "Request Changes" is clicked, an inline textarea appears (not a separate view) with a placeholder explaining the learning loop.

#### 5. QuestionPanel

**Current:** Vertical card stack. No grouping.

**New:** Grouped by role, collapsible. Shows which stage asked the question. Inline answer input.

```
┌─────────────────────────────────────────────┐
│ Questions (2 pending, 1 answered)            │
├─────────────────────────────────────────────┤
│ ▼ Product (stage 1.1)                        │
│   Q: What's the target deployment?           │
│   [ Cloud ] [ On-prem ] [ Hybrid ] [ Other ] │
│   ◇ Your answer: Cloud                       │
│                                              │
│   Q: What's the expected user count?         │
│   [ <1K ] [ 1K-10K ] [ 10K+ ] [ Other ]      │
│   [ Type answer...                ]          │
│                                              │
│ ▶ Architect (stage 1.3) — 1 answered         │
└─────────────────────────────────────────────┘
```

- Grouped by role (product, architect, design, etc.)
- Stage badge showing which stage asked
- Collapsible groups
- Submit all answers for a group at once
- Answered questions show as collapsed with the answer

#### 6. BoltPanel (construction only)

**Current:** Static list of bolts with status. No run button. No unit detail.

**New:** Full Bolt management:

```
┌──────────────────────────────────────────────────────┐
│ Construction Bolts                                    │
│ [Prepare Bolts]  [Run All]                            │
├──────────────────────────────────────────────────────┤
│ Bolt 1  🔨 Walking Skeleton     [in_progress]         │
│   Units: auth-login, session-setup                    │
│   Stages: 3.1 ✓  3.2 ✓  3.3 ▶  3.4 [ ]  3.5 [ ]     │
│   [Run Bolt 1]  [View Output]                         │
│                                                       │
│ Bolt 2  [pending]                                     │
│   Units: profile-page                                 │
│   Stages: 3.1 [ ]  3.2 [ ]  3.3 [ ]  3.4 [ ]  3.5 [ ]│
│   [Run Bolt 2]                                        │
│                                                       │
│ 🪜 Ladder: Walking skeleton complete.                 │
│   ( ) Gated (approve each Bolt)                       │
│   (•) Autonomous (skip per-Bolt gates)               │
└──────────────────────────────────────────────────────┘
```

- Per-Bolt stage progress (mini StageRail)
- Run individual Bolt buttons
- Unit IDs listed
- Ladder prompt inline (not separate)
- "Run All" for autonomous mode

#### 7. AuditDrawer

**Current:** AuditTimeline is a full panel taking vertical space.

**New:** Slide-in drawer from the right edge. Collapsible. Shows audit events with filtering by event type and stage.

#### 8. ControlBar (bottom, compact)

**Current:** Jump controls, scope/depth/test-strategy, and cancel are all in `<details>` elements in the main panel.

**New:** A compact bottom bar with icon buttons:
- `Jump` → opens a dropdown with stage/phase jump options
- `Settings` → opens a modal with scope/depth/test-strategy selectors
- `Cancel` → red icon, confirms
- `Audit` → toggles audit drawer
- Keyboard shortcut hints on hover

#### 9. SessionIndicator

New component showing live tmux session status:

```
┌──────────────────────────┐
│ Sessions: 2 active        │
│ ● ideation (running)      │
│ ● inception (paused)      │
└──────────────────────────┘
```

Shows in the FeatureHeader. Click to see session details (output, resume, kill).

#### 10. TmuxPaneViewer (raw terminal)

Live tmux capture-pane viewer using xterm.js. Shows exactly what the agent sees in its tmux session — including ANSI colors, cursor position, and full terminal output.

```
┌──────────────────────────────────────────────────┐
│ tmux: devteam-abc123-ideation    [Detach] [Kill]   │
├──────────────────────────────────────────────────┤
│ $ opencode run --agent product ...                │
│ Reading CONTEXT.md for your task and begin work.  │
│ ...                                                │
│ ▌ (cursor)                                         │
└──────────────────────────────────────────────────┘
```

- xterm.js renders the raw ANSI output from `tmux capture-pane`
- Polls `GET /api/features/{id}/sessions/{phase}/pane` every 500ms
- Secondary view — SSE-driven AgentOutputLive is the primary output view
- "Detach" button hides the terminal (session keeps running)
- "Kill" button terminates the session
- Available from SessionIndicator click and from StageDetail "View Raw Terminal" link
- Full terminal emulation — colors, cursor, line wrapping all preserved

### Dashboard Upgrade

**Current:** List/Kanban toggle + IntakeForm + KnowledgeEditor crammed at bottom.

**New:**
- **Feature grid** (card view, not just list/kanban) — cards show scope badge, current stage, progress bar (X/32 stages), pending questions indicator, agent activity indicator
- **Active sessions panel** — shows all currently running tmux sessions across all features. Click to jump to that feature's live output.
- **Knowledge editor** — moved to a dedicated `/knowledge` route, not crammed on dashboard
- **System health** — DB size, active sessions count, recent errors

### Routing

**Current:** `/` (dashboard) and `/features/:id` (detail). That's it.

**New:**
- `/` — Dashboard (feature grid + active sessions)
- `/features/:id` — Feature workspace (StageRail + StageDetail)
- `/features/:id/stages/:stageId` — Deep link to a specific stage
- `/features/:id/bolts` — Bolt management view (construction)
- `/features/:id/audit` — Full audit trail view
- `/features/:id/sessions` — Session management view (list, resume, kill)
- `/features/:id/sessions/:phase/pane` — Raw tmux terminal viewer (xterm.js)
- `/knowledge` — Team knowledge editor (global, not per-feature)
- `/settings` — System settings (if needed later)

### SSE Fix

**Current problems:**
1. `useSSE` lists event types that don't match what the server sends
2. `agent_output` events are broadcast but not consumed by AgentOutput
3. No event for gate state changes (server broadcasts `gate_result` but pipeline doesn't call broadcastSSE for gate transitions)
4. No event for stage state changes (server broadcasts `stage_change` but pipeline doesn't call it)

**Fix:**
1. Align event types between server and client
2. Wire `broadcastSSE` calls into `stage_runner.go` for every state transition:
   - `stage_started` — when RunStage begins
   - `stage_awaiting_approval` — when gate opens
   - `stage_revising` — when revision starts
   - `stage_completed` — when gate approved
   - `gate_result` — when reviewer returns verdict
   - `bolt_started` / `bolt_completed` — construction
   - `session_state_change` — when tmux session state changes
3. AgentOutputLive consumes `agent_output` SSE events directly
4. useSSE provides a `subscribe(type, handler)` API for component-level event handling

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `A` | Approve current gate |
| `R` | Reject current gate (focuses notes textarea) |
| `Enter` | Run next stage |
| `J` | Open jump menu |
| `?` | Show all shortcuts |
| `Esc` | Close any drawer/modal |
| `[` / `]` | Navigate to previous/next stage |
| `O` | Toggle agent output |
| `G` | Toggle audit drawer |

Implemented via a `useKeyboardShortcuts` hook that checks context (only active when no input/textarea is focused).

### State Management

**Current:** React Query for all data. SSE for real-time. `isProcessing` is local state synced from feature data.

**New:** Same React Query foundation, but:
- **Normalized cache**: Use React Query's `normalize` or a lightweight store (zustand) for cross-component state (selected stage, active tab, drawer open state)
- **Optimistic updates**: Approve/reject/jump mutations use `onMutate` to update UI immediately, rollback on error
- **SSE → query invalidation map**: Each SSE event type maps to specific query keys to invalidate (not blanket-invalidate everything)

### Visual Design

**Current:** Tailwind utility classes inline. Functional but generic.

**New:**
- **Design tokens**: Centralized color palette, spacing, typography in `tailwind.config.ts` — consistent dark/light theme
- **Component library**: Extract reusable primitives (Button, Badge, Card, Drawer, Modal, Tabs) into `ui/primitives/`
- **Status colors**: Semantic and consistent — `success` (green), `warning` (yellow), `error` (red), `info` (blue), `neutral` (gray)
- **Loading states**: Skeletons for initial load, spinners for mutations, progress bars for long-running operations
- **Empty states**: Every panel has a purpose-designed empty state (not just "No data")
- **Animations**: Subtle transitions for state changes (gate opening, stage advancing). `framer-motion` for drawer/modal animations.
- **Responsive**: Mobile-first breakpoints. StageRail collapses to a horizontal scroll on mobile. Panels stack vertically.

---

## Execution Plan

### Phase 1: Tmux Session Boundaries (backend, no UI changes yet)

**Goal:** Persistent sessions, resume after questions, DB-driven context.

1. Migration 009: `tmux_sessions` table (with `bolt_number` for per-Bolt construction sessions)
2. `internal/pipeline/session_manager.go` — session lifecycle (create, resume, expire, cleanup). Per-Bolt session naming for construction.
3. `internal/role/tmux.go` rewrite:
   - Session naming: `devteam-{featureID}-{phase}` or `devteam-{featureID}-construction-bolt{N}`
   - No unconditional kill — check if session exists, reuse if alive
   - Persistent context dir: `~/.local/share/devteam/sessions/{featureID}/{phase}/` (or `/{featureID}/construction-bolt{N}/`)
   - Log filenames: `{stageID}-{agent}.log` (no clobbering)
   - No `conversation.log` — context is DB-driven via `buildStageContext`
   - `ResumeSession` method (re-dispatch into existing session with fresh DB context)
   - `CapturePane` method (raw ANSI output for xterm.js viewer)
4. `internal/role/dispatcher.go` — `DispatchRequest` gains `SessionName`, `ContextDir` fields. Dispatcher reuses existing sessions.
5. `internal/pipeline/stage_runner.go` — resolve or create session per phase (or per Bolt for construction). Regenerate CONTEXT.md from DB before each dispatch. Pass session name to dispatcher.
6. `internal/api/session_handlers.go` — new endpoints: list sessions, resume, kill, get output, capture-pane
7. Question answer flow: after all questions answered, trigger session resume for the phase (re-dispatch with fresh DB context including answers)
8. Tests: session creation, reuse, resume after question, per-Bolt sessions, log non-clobbering, session expiry, capture-pane output

**Files touched:** tmux.go, dispatcher.go, stage_runner.go, session_manager.go (new), session_handlers.go (new), session_store.go (new), migration_009.go (new)

### Phase 2: SSE Fix + Live Output (backend + minimal UI)

**Goal:** Real-time agent output via SSE. Aligned event types.

1. Wire `broadcastSSE` into `stage_runner.go` for all state transitions:
   - `stage_started`, `stage_awaiting_approval`, `stage_revising`, `stage_completed`
   - `gate_result` (reviewer verdict), `bolt_started`, `bolt_completed`
   - `session_state_change`
2. Fix event type constants in `useSSE.ts` to match server
3. Rewrite `AgentOutput` → `AgentOutputLive`:
   - Consume SSE `agent_output` events directly via `useSSE` callback
   - HTTP `/output` fallback only for initial load and reconnect
   - Per-stage output filtering (read from `logs/{stageID}-{agent}.log`)
4. Add `useSSE` `subscribe(type, handler)` API for component-level event handling
5. New endpoint: `GET /api/features/{id}/sessions/{phase}/output` — per-stage output from log files
6. Tests: SSE event emission on state transitions, client event handling

**Files touched:** stage_runner.go, server.go, useSSE.ts, AgentOutputLive.tsx (renamed), session_handlers.go

### Phase 3: Component Decomposition (UI refactor, no new features)

**Goal:** Break FeatureDetail monolith into focused components.

1. Extract `FeatureHeader` — compact status/scope/depth/priority
2. Extract `StageRail` — left sidebar, replaces StageProgress
3. Extract `StageDetail` — main panel shell
4. Extract `StageSummary`, `ArtifactPanel`, `RevisionHistory` — stage detail sub-components
5. Extract `QuestionPanel` — grouped questions
6. Extract `ControlBar` — compact bottom bar
7. Extract `AuditDrawer` — slide-in drawer
8. FeatureDetail becomes a layout shell (~100 lines) that composes these
9. Add routing: `/features/:id/stages/:stageId` deep links
10. All existing tests still pass (same data, different layout)

**Files touched:** FeatureDetail.tsx (rewrite), 8 new components, App.tsx (routing)

### Phase 4: Gate Panel + Artifact Integration (UI)

**Goal:** Gate decisions with full context. Artifacts visible per-stage.

1. `GatePanel` — inline (not modal), shows reviewer notes, smoke results, artifact links
2. Wire `smokeFailures` and `reviewerVerdict` from `StageRunResult` through to GatePanel
3. `ArtifactPanel` — fetches and renders artifacts for the selected stage
4. Artifact type → stage mapping (which stage produces which artifacts)
5. New endpoint: `GET /api/features/{id}/stages/{stageId}/artifacts` — artifacts for a specific stage
6. Click artifact → inline markdown render with syntax highlighting
7. Tests: gate panel shows context, artifact panel renders, stage-artifact mapping

**Files touched:** GatePanel.tsx (new), ArtifactPanel.tsx (new), stage_handlers.go (artifacts endpoint), client.ts

### Phase 5: Bolt Panel + Construction UX (UI)

**Goal:** Full Bolt management from the UI.

1. `BoltPanel` — per-Bolt stage progress, unit IDs, run buttons
2. Wire `runBolt` API to UI buttons
3. Ladder prompt inline in BoltPanel
4. "Run All Bolts" button for autonomous mode
5. Per-Bolt output view (filter AgentOutputLive by Bolt)
6. Tests: bolt panel renders, run bolt button works, ladder prompt

**Files touched:** BoltPanel.tsx (new), FeatureDetail.tsx, client.ts

### Phase 6: Keyboard Shortcuts + UX Polish (UI)

**Goal:** Keyboard-driven workflow. Visual polish.

1. `useKeyboardShortcuts` hook
2. Optimistic updates for approve/reject/jump mutations
3. Loading states: skeletons, spinners, progress bars
4. Empty states for every panel
5. `framer-motion` for drawer/modal animations
6. Design tokens in tailwind.config.ts
7. Extract UI primitives (Button, Badge, Card, Drawer, Modal, Tabs)
8. Responsive breakpoints (mobile-first)
9. SessionIndicator in FeatureHeader
10. Tests: keyboard shortcuts work, optimistic updates rollback on error

**Files touched:** useKeyboardShortcuts.ts (new), tailwind.config.ts, ui/primitives/* (new), all components (polish)

### Phase 7: Dashboard + Routing + Knowledge (UI)

**Goal:** Dashboard upgrade, proper routing, knowledge editor move.

1. Feature grid with rich cards (scope badge, progress bar, activity indicator)
2. Active sessions panel on dashboard
3. System health indicators
4. `/knowledge` route — dedicated knowledge editor page
5. `/features/:id/bolts`, `/features/:id/audit`, `/features/:id/sessions` routes
6. Playwright e2e tests for new UI flows
7. Tests: dashboard renders, routing works, knowledge CRUD

**Files touched:** Dashboard.tsx (rewrite), FeatureCard.tsx (rewrite), App.tsx (routing), KnowledgePage.tsx (new), e2e tests

### Phase 8: Session UI + Tmux Pane Viewer + Cleanup (UI + backend)

**Goal:** Session management UI, raw terminal viewer, final cleanup.

1. Session view: `/features/:id/sessions` — list all sessions with state, output, resume/kill buttons
2. TmuxPaneViewer: `/features/:id/sessions/:phase/pane` — xterm.js terminal rendering live `tmux capture-pane` output
   - Polls `GET /api/features/{id}/sessions/{phase}/pane` every 500ms
   - xterm.js renders ANSI escape sequences (colors, cursor, line wrapping)
   - "Detach" button hides terminal (session keeps running)
   - "Kill" button terminates session
   - Available from SessionIndicator click and StageDetail "View Raw Terminal" link
3. SessionIndicator in FeatureHeader showing live session count + click to pane viewer
4. Session resume UI (button to resume paused session after questions answered)
5. Cleanup: remove old `AgentOutput.tsx`, `GateResult.tsx`, `PhaseTimeline.tsx`, `ProcessView.tsx` (already deleted but verify no references)
6. Remove old `getCapturedOutput` polling endpoint (replaced by SSE + session output + capture-pane)
7. Full e2e test: create feature → run stages → answer questions → resume session → approve gates → construction → done
8. README update with new UI screenshots and features

**Files touched:** SessionView.tsx (new), TmuxPaneViewer.tsx (new), FeatureHeader.tsx, server.go (remove old output endpoint), e2e tests, README.md

---

## What Gets Deleted

- `ui/src/components/AgentOutput.tsx` — replaced by `AgentOutputLive.tsx`
- `ui/src/components/GateResult.tsx` — replaced by `GatePanel.tsx`
- `ui/src/components/PhaseTimeline.tsx` — replaced by `StageRail.tsx`
- `ui/src/components/ProcessView.tsx` — already deleted, verify
- `ui/src/components/KnowledgeEditor.tsx` — moved to `/knowledge` route
- `GET /api/features/{id}/output` — replaced by SSE + session output endpoints
- Polling logic in `AgentOutput` — replaced by SSE consumption

## What Stays

- React Query for data fetching
- Tailwind CSS for styling
- React Router for routing
- SSE infrastructure (server-side)
- All backend API endpoints (except old output endpoint)
- All DB schema (expanded with tmux_sessions)
- All role files, stage definitions, gate system, audit system

---

## Risks & Mitigations

1. **Persistent sessions use more memory.** tmux sessions stay alive across stages. Mitigation: sessions expire when the phase advances or feature completes. Cleanup on feature done. Max one session per phase per feature (plus one per Bolt for construction).

2. **DB-driven context queries on every dispatch.** `buildStageContext` queries multiple DB tables each dispatch. Mitigation: these are indexed queries on SQLite, fast for the data volumes involved (hundreds of rows, not millions). The context is structured and queryable — far better than parsing a log file.

3. **Component decomposition could break tests.** Large refactor of FeatureDetail. Mitigation: Phase 3 is decomposition only — no data changes, same API. Tests assert on data-testid which can be preserved across component boundaries.

4. **SSE reliability.** Long-running SSE connections drop. Mitigation: exponential backoff reconnection (already in useSSE). HTTP fallback for initial load. Buffer lifecycle events for late joiners (already in server).

5. **Keyboard shortcuts conflict with browser.** Some keys are reserved. Mitigation: only activate when no input/textarea focused. Use `?` meta key pattern (not single letters that conflict with browser shortcuts like `/` for quick find).

6. **Session resume could race with agent state.** Mitigation: re-dispatch approach. The agent process from the prior dispatch has exited (it signaled completion/needs_feedback). We start a new agent in the same session with fresh DB context. No race.

7. **Parallel Bolts need separate sessions.** Mitigation: per-Bolt session naming (`construction-bolt1`, `construction-bolt2`) already in the design. The session manager handles this natively.

8. **xterm.js bundle size.** xterm.js is ~130KB gzipped. Mitigation: lazy-load the TmuxPaneViewer route (`React.lazy`) so it only loads when a user navigates to the pane viewer. Primary output view (AgentOutputLive via SSE) doesn't need xterm.js.

9. **capture-pane polling for raw viewer.** 500ms polling for tmux capture-pane is less efficient than SSE. Mitigation: this is a secondary view for power users. Primary output is SSE-driven. The pane viewer is opt-in and only polls while visible.

---

## Estimated Effort

| Phase | Effort | Why |
|-------|--------|-----|
| 1: Tmux session boundaries | 3-4 days | Session manager, tmux rewrite, persistent context dirs, DB-driven context, resume flow, migration, tests |
| 2: SSE fix + live output | 2 days | Wire broadcastSSE into transitions, rewrite AgentOutput, event type alignment |
| 3: Component decomposition | 3-4 days | Break 590-line monolith into 8+ components, routing, preserve tests |
| 4: Gate panel + artifacts | 2-3 days | GatePanel with context, ArtifactPanel, stage-artifact mapping, endpoint |
| 5: Bolt panel + construction UX | 2 days | BoltPanel, run buttons, ladder inline, per-Bolt output |
| 6: Keyboard shortcuts + polish | 2-3 days | Shortcuts, optimistic updates, loading/empty states, design tokens, animations |
| 7: Dashboard + routing + knowledge | 2-3 days | Feature grid, active sessions, knowledge page, routing, e2e tests |
| 8: Session UI + pane viewer + cleanup | 2-3 days | Session view, xterm.js pane viewer, indicator, cleanup, full e2e, README |
| **Total** | **18-24 days** | |

---

## Decisions (Locked)

1. **Session resume approach: re-dispatch.** Agent process exited after signaling completion. New agent starts in same tmux session with fresh CONTEXT.md generated from DB. No race conditions. tmux pane provides visual continuity; DB provides structured context.

2. **UI component library: build primitives.** Button, Badge, Card, Drawer, Modal, Tabs — ~200 lines, no dependency, full styling control. Purpose-built, not shadcn-clone.

3. **State management: React Query + zustand.** React Query for server state. zustand for UI state (selected stage, drawer open, active tab). ~1KB dependency.

4. **Animations: CSS + framer-motion for drawers/modals.** CSS transitions for hover/focus/state. framer-motion (30KB) for drawer slide-in, modal scale-in/fade-out, exit animations.

5. **Old /output endpoint: remove after SSE works.** SSE + session output endpoints fully replace it. No dual code paths.

6. **Raw tmux pane viewer: YES.** Add a terminal viewer using xterm.js that shows live `tmux capture-pane` output. Power-user feature, full transparency into what the agent sees. New `GET /api/features/{id}/sessions/{phase}/pane` endpoint streams capture-pane output. xterm.js renders the ANSI escape sequences. Polling-based (500ms) since tmux doesn't have a stream API — but this is a secondary view, not the primary output (SSE is primary).

7. **Context: DB-driven, not file-driven.** No `conversation.log`. All context retrieved from DB at dispatch time via `buildStageContext`. Artifacts, audit events, notes, rules, knowledge, questions — all in DB. CONTEXT.md regenerated fresh each dispatch. Single source of truth.

8. **Construction sessions: per-Bolt.** Each Bolt gets its own tmux session: `devteam-{featureID}-construction-bolt{N}`. Stages 3.1-3.5 share within a Bolt. Walking skeleton isolated. Future parallel Bolts work naturally.