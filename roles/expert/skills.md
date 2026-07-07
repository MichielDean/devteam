# Expert Skills Manifest

The expert agent's skills wrap the devteam CLI-proxy verbs. These are loaded
into the expert's *generated, isolated* opencode.json (never the global config —
C3/NFR-SEC-5). Each skill maps a conversational intent to a proposed `devteam`
verb; the chat backend routes the proposal through the confirm gate.

## Skill: devteam-status

**Intent:** "What's the status of feature X?" / "Where is the pipeline?"
**Verb:** `devteam feature status <feature-id>` (read-only, runs immediately)
**Classification:** read-only

## Skill: devteam-stages

**Intent:** "What stages does feature X have?" / "Show me the stage list."
**Verb:** `devteam stages <feature-id>` (read-only)
**Classification:** read-only

## Skill: devteam-artifacts

**Intent:** "What artifacts exist for feature X?" / "Show me the app-design artifact."
**Verb:** `devteam artifacts <feature-id>` (read-only)
**Classification:** read-only

## Skill: devteam-create-feature

**Intent:** "Create a new feature." / "Start a feature for X."
**Verb:** `devteam feature create --title "..." --description "..."` (safe mutating, requires confirm)
**Classification:** mutating
**Confirm prompt:** "Run `devteam feature create --title "..."`?"

## Skill: devteam-signal-pass

**Intent:** "Approve the gate." / "Signal pass for this stage."
**Verb:** `devteam signal <feature-id> pass` (safe mutating, requires confirm)
**Classification:** mutating
**Confirm prompt:** "Run `devteam signal <id> pass` — mark this stage complete?"

## Skill: devteam-signal-needs-feedback

**Intent:** "I need to ask the human a question." / "Signal needs_feedback."
**Verb:** `devteam signal <feature-id> needs_feedback` (safe mutating)
**Classification:** mutating

## Skill: devteam-run-stage

**Intent:** "Run the next stage." / "Start stage 2.3."
**Verb:** `devteam run-stage <feature-id> <stage-id>` (safe mutating)
**Classification:** mutating

## Skill: devteam-answer-questions

**Intent:** "Answer the pending question." / "Submit the answer."
**Verb:** `devteam questions answer <feature-id> --answer "..."` (safe mutating)
**Classification:** mutating

## Skill: devteam-cancel-feature  (DESTRUCTIVE — use sparingly)

**Intent:** "Cancel this feature."
**Verb:** `devteam feature cancel <feature-id>` (destructive — ALWAYS confirm with consequence)
**Classification:** destructive
**Confirm prompt:** "Run `devteam feature cancel <id>` — this will cancel feature X and cannot be undone. Type the feature id to confirm."