# Human Interaction Points

## Context

The Dev Team pipeline currently runs fully autonomously — each phase dispatches an agent, the agent produces artifacts, and the gate evaluator checks them. But the inception and planning phases benefit from human input: requirements clarification, architectural decisions, scope boundaries.

When a human is available through the web UI, the pipeline should pause at decision points and surface questions. When running autonomously (or when the human doesn't respond within a timeout), it should document assumptions and proceed conservatively.

## Key Requirements

1. **Question model**: PM and Architect can surface questions for human input
2. **Feature status "waiting_for_human"**: Pipeline pauses when questions exist
3. **API endpoints**: CRUD for questions, plus pending endpoint
4. **Web UI**: Question cards, badge on feature list, answer input
5. **Timeout fallback**: If no human response within configurable timeout, assume and proceed
6. **Context injection**: Human answers are injected into agent context on re-dispatch

## Existing System

- Feature state machine in `internal/feature/feature.go` supports phases and statuses
- API server in `internal/api/server.go` with CRUD endpoints
- Web UI in `ui/` with React frontend
- Pipeline orchestrator in `internal/pipeline/pipeline.go`
- Rule loader in `internal/rules/loader.go` that builds context for agents
