# AGENTS.md — Dev Team Repository

This file provides context for AI agents working in this repository.

## Project Overview

Dev Team is an **AI-Driven Development Life Cycle (AI-DLC) platform** that orchestrates multi-agent software development through structured phases: inception → planning → construction → review → testing → delivery.

The platform itself is a Go backend with a React/TypeScript frontend. It dispatches opencode agents to work on features in per-spec git worktrees.

## CRITICAL: This Is a Platform, Not a Project

Phase instructions, gate checks, and role instructions must NEVER reference:
- Specific build commands (go build, npm run build, cargo build)
- Specific test commands (go test, npm test, npx playwright test)
- Specific file names (go.mod, package.json, playwright.config.ts)
- Specific ports (8765, 18765)
- Repository-specific helper scripts
- Language-specific patterns (json tags, omitempty, httptest)

Instructions should tell agents to **discover** the project's build/test infrastructure and use whatever commands the project supports.

## Build & Test Commands

### IMPORTANT: Go binary location

The `go` binary is at `/usr/local/go/bin/go`. It may not be in your default PATH. Always use the full path or prepend to PATH:

```bash
export PATH="$PATH:/usr/local/go/bin"
```

### Backend (Go)

```bash
# Build (use full path if go is not in PATH)
PATH="$PATH:/usr/local/go/bin" go build -o ~/go/bin/devteam ./cmd/devteam/

# Run tests
PATH="$PATH:/usr/local/go/bin" go test ./... -count=1 -timeout 120s

# Run with coverage
PATH="$PATH:/usr/local/go/bin" go test ./... -cover -count=1

# Vet
PATH="$PATH:/usr/local/go/bin" go vet ./...
```

### Frontend (UI)

```bash
cd ui
npm install        # install dependencies
npm run build      # production build
npm run dev        # dev server
npm run lint       # eslint
npm run test:e2e   # Playwright e2e tests (uses port 18765, not 8765)
```

### Playwright E2E Tests

Playwright config is at `ui/playwright.config.ts`. It uses port **18765** (not 8765, which is the production service).

```bash
cd ui
npx playwright install chromium          # install browser if needed
npx playwright test --reporter=line       # run all e2e tests
npx playwright test kanban.spec.ts        # run specific test file
```

The Playwright `webServer` config automatically starts a test server on :18765. The config uses `cwd: repoRoot` so the server runs from the repo root (where `devteam.yaml` lives).

- Set `START_SERVER=1` to force Playwright to start its own server
- If using `SERVER_BINARY`, include `cd` to the repo root: `SERVER_BINARY="cd /path/to/repo && /path/to/binary -http :18765"`
- Do NOT set `SERVER_BINARY` to just a binary path — the binary needs to run from the repo root to find `devteam.yaml`

## Service Architecture

- **devteam-web**: systemd service on `:8765` (production)
- **Binary**: `~/go/bin/devteam`
- **Working directory**: `~/source/devteam` (primary checkout)
- **Config**: `devteam.yaml` in repo root
- **Specs**: `specs/<feature-id>/` (git-tracked runtime data)
- **Spec worktrees**: `~/worktrees/devteam-specs/<feature-id>/` on branch `spec/<feature-id>`

## Important Constraints

### Specs Are Runtime Data
- `specs/` directories contain runtime-generated artifacts that may not be committed to git yet
- NEVER use `git reset --hard` on this repo — use `git pull` instead
- Specs are auto-committed to the spec worktree after each gate passes
- The primary checkout's `specs/` directory has state files for `ListFeatures` to find

### Spec Worktrees
Each feature gets its own git worktree at `~/worktrees/devteam-specs/<feature-id>/`. All agents dispatch with CWD = the worktree. The worktree is on branch `spec/<feature-id>`.

### Testing
- Production service runs on :8765 — do NOT start test servers on this port
- Playwright tests use :18765 (configured in `ui/playwright.config.ts`)
- Go tests should use `httptest` for in-process server testing, not external processes

### Agent Dispatch
- Agents run in tmux sessions named `devteam-<feature-id>`
- Output is captured via `tmux capture-pane` and streamed to the UI via SSE
- The `~/go/bin/devteam` binary must be built from `origin/main` (never from a worktree)

## Project Structure

```
devteam/
├── cmd/devteam/          # Main entry point
├── internal/
│   ├── api/              # HTTP API server + SSE
│   ├── feature/          # Feature state, questions, phases
│   ├── gitops/           # Git operations
│   ├── pipeline/         # Pipeline orchestration, gates, process loop
│   ├── repo/             # Repository management
│   ├── role/             # Agent dispatch (tmux-based)
│   ├── rules/            # AIDLC rule loading
│   └── spec/             # Spec provider, artifact I/O
├── roles/                # Role instructions (pm, architect, developer, reviewer, tester, ops)
├── rules/                # AIDLC rules (inception, planning, construction, etc.)
├── specs/                # Feature spec directories (runtime data)
├── ui/                   # React/TypeScript frontend
│   ├── src/
│   └── e2e/              # Playwright e2e tests
├── devteam.yaml          # Pipeline configuration
└── AGENTS.md             # This file
```

## Deployment

```bash
# Build from main (NEVER from a worktree)
git pull origin main
go build -o ~/go/bin/devteam ./cmd/devteam/
cd ui && npm run build
systemctl --user restart devteam-web
```

Never build or deploy from a worktree. Always wait for PRs to merge, pull main, then build.