#!/usr/bin/env bash
# deploy.sh — CD pipeline executable for the devteam repo.
# Feature: full-crud-and-ui-for-managing-repositories — stage 4.1 (minimal).
#
# Deploys the latest main to the single-host devteam-server systemd unit.
# Strategy: recreate (stop old, build new, restart, smoke). See cd-config
# and deploy-strategy artifacts.
#
# Promotion gate: the latest CI run on main MUST be green. This script is
# the branch-protection backstop for a private repo on a free GitHub plan
# (branch protection is unavailable — see cd-config §3.2).
#
# Usage:  ./deploy.sh
# Exit:   0 = deployed and smoke-green; 1 = aborted or rolled back.

set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$REPO_DIR"

log()  { printf '[deploy] %s\n' "$*"; }
fail() { printf '[deploy] FATAL: %s\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# Step 0 — Pre-flight: working tree clean
# ---------------------------------------------------------------------------
if ! git diff-index --quiet HEAD --; then
  fail "working tree dirty — commit or stash first."
fi

# ---------------------------------------------------------------------------
# Step 1 — Pre-flight: on main
# ---------------------------------------------------------------------------
branch=$(git rev-parse --abbrev-ref HEAD)
if [ "$branch" != "main" ]; then
  fail "deploy only runs from main (currently on '$branch')."
fi

# ---------------------------------------------------------------------------
# Step 2 — Pre-flight: CI green on main (branch-protection backstop, §3.2)
# ---------------------------------------------------------------------------
conclusion=$(gh run list --branch main --limit 1 --json conclusion --jq '.[0].conclusion' 2>/dev/null || true)
if [ "$conclusion" != "success" ]; then
  fail "latest CI run on main is '$conclusion' (expected 'success'). \
Branch protection is unavailable on this private repo (cd-config §3.2). \
Do not deploy a red main. Revert the merge or fix the build."
fi
log "CI green on main (conclusion=$conclusion)."

# ---------------------------------------------------------------------------
# Step 3 — Pull latest (fast-forward only)
# ---------------------------------------------------------------------------
if ! git pull --ff-only origin main; then
  fail "git pull --ff-only failed — main diverged. Investigate before deploying."
fi
DEPLOY_SHA=$(git rev-parse --short HEAD)
log "deploying $DEPLOY_SHA"

# ---------------------------------------------------------------------------
# Step 4 — Build frontend (ui/dist, filesystem-served by the Go binary)
# ---------------------------------------------------------------------------
log "building frontend (ui/dist)"
( cd ui && npm ci && npm run build ) || fail "frontend build failed."

# ---------------------------------------------------------------------------
# Step 5 — Build backend (overwrite the on-disk binary; old process keeps
# running from the old inode until step 6 restarts it).
# ---------------------------------------------------------------------------
log "building backend (devteam-server)"
go build -o devteam-server ./cmd/devteam || fail "backend build failed."

# ---------------------------------------------------------------------------
# Step 6 — Recreate: restart the systemd user unit.
# systemd sends SIGTERM to the old PID, starts the new binary, which runs
# RunMigrations (applies migration_014 idempotently) + seed hook at boot.
# ---------------------------------------------------------------------------
log "restarting devteam-server (recreate)"
systemctl --user restart devteam-server || fail "systemctl restart failed — check: systemctl --user status devteam-server"

# ---------------------------------------------------------------------------
# Step 7 — Health check (server came up + DB + migrations).
# ---------------------------------------------------------------------------
log "health check (:8765)"
healthy=0
for i in 1 2 3 4 5; do
  if curl -sf http://localhost:8765/api/repos >/dev/null 2>&1; then
    healthy=1
    break
  fi
  sleep 2
done
if [ "$healthy" != "1" ]; then
  fail "server did not come up on :8765. Check: journalctl --user -u devteam-server -n 50"
fi
log "server healthy."

# ---------------------------------------------------------------------------
# Step 8 — Smoke (E2E against the production port :8765, not CI :18765).
# Principle #6: deployment is not done until smoke passes.
# On failure, invoke rollback.sh automatically (rollback-runbook §2).
# ---------------------------------------------------------------------------
log "smoke (E2E on :8765)"
# Smoke runs against the restarted production server — do NOT set START_SERVER
# (that would spawn a test server on :18765). We want to verify the actual
# production process.
if ! ( cd ui && npm run test:e2e ); then
  log "SMOKE FAILED — initiating rollback (rollback-runbook §2)"
  ./rollback.sh || fail "rollback failed — manual intervention required (rollback-runbook §3)"
  exit 1
fi

# ---------------------------------------------------------------------------
# Step 9 — Report
# ---------------------------------------------------------------------------
timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)
operator=$(whoami)
log "deployed sha=$DEPLOY_SHA at=$timestamp by=$operator"
log "feature is live."