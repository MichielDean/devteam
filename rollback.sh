#!/usr/bin/env bash
# rollback.sh — rollback procedure for the devteam repo.
# Feature: full-crud-and-ui-for-managing-repositories — stage 4.1 (minimal).
#
# Invoked by deploy.sh on smoke failure (T1, automated) or by the operator
# manually (T3). Reverts the latest merge to main, rebuilds, restarts, and
# re-smokes. See rollback-runbook artifact.
#
# Data rollback (DROP TABLE repos + re-seed) is a separate procedure —
# see rollback-runbook §4. This script is CODE rollback only.
#
# Usage:  ./rollback.sh
# Exit:   0 = rollback complete and smoke-green; 1 = rollback failed.

set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$REPO_DIR"

log()  { printf '[rollback] %s\n' "$*"; }
fail() { printf '[rollback] FATAL: %s\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# 1. Identify the deploy commit (the current HEAD of main).
# ---------------------------------------------------------------------------
DEPLOY_SHA=$(git rev-parse --short HEAD)
log "reverting deploy $DEPLOY_SHA"

# ---------------------------------------------------------------------------
# 2. Find the merge commit to revert (latest merge on main). If no merge,
# revert the HEAD commit directly.
# ---------------------------------------------------------------------------
MERGE_SHA=$(git log --merges -1 --format='%H' 2>/dev/null || true)
if [ -z "$MERGE_SHA" ]; then
  MERGE_SHA=$DEPLOY_SHA
fi
log "reverting merge $MERGE_SHA"

# ---------------------------------------------------------------------------
# 3. git revert (code rollback — preserves history, auditable).
# ---------------------------------------------------------------------------
if ! git revert --no-edit "$MERGE_SHA"; then
  fail "git revert failed (conflicts?). Abort: 'git revert --abort' and investigate. See rollback-runbook §3."
fi
REVERT_SHA=$(git rev-parse --short HEAD)
log "reverted to $REVERT_SHA"

# ---------------------------------------------------------------------------
# 4. Rebuild from the reverted code.
# ---------------------------------------------------------------------------
log "rebuilding frontend"
( cd ui && npm ci && npm run build ) || fail "frontend rebuild failed."
log "rebuilding backend"
go build -o devteam-server ./cmd/devteam || fail "backend rebuild failed."

# ---------------------------------------------------------------------------
# 5. Restart the service (recreate back to the pre-feature code).
# The reverted binary does not know about the repos table — it serves the
# registry from repos.yaml (pre-feature behavior). The repos table is
# orphaned but harmless (<1 MB, ignored by the old code).
# ---------------------------------------------------------------------------
log "restarting devteam-server"
systemctl --user restart devteam-server || fail "systemctl restart failed — check: systemctl --user status devteam-server"

# ---------------------------------------------------------------------------
# 6. Health check.
# ---------------------------------------------------------------------------
healthy=0
for i in 1 2 3 4 5; do
  if curl -sf http://localhost:8765/api/repos >/dev/null 2>&1; then
    healthy=1
    break
  fi
  sleep 2
done
if [ "$healthy" != "1" ]; then
  fail "rollback server did not come up on :8765. Manual intervention required (rollback-runbook §3). Check: journalctl --user -u devteam-server -n 50"
fi
log "server healthy."

# ---------------------------------------------------------------------------
# 7. Re-smoke (confirm the revert is healthy).
# ---------------------------------------------------------------------------
log "rollback smoke (E2E on :8765)"
if ! ( cd ui && npm run test:e2e ); then
  fail "rollback smoke failed — the revert itself is broken. Manual intervention required (rollback-runbook §3, nuclear: git reset --hard <known-good-sha>)."
fi

# ---------------------------------------------------------------------------
# 8. Report.
# ---------------------------------------------------------------------------
timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)
log "rollback complete: reverted $MERGE_SHA (deploy $DEPLOY_SHA) -> $REVERT_SHA at=$timestamp"
log "the registry is now served from repos.yaml (pre-feature behavior)."
log "note: the repos table still exists but is ignored by the reverted binary."
log "      to fully clean up (optional): see rollback-runbook §4 (DROP TABLE repos)."