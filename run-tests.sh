#!/bin/bash
# Helper script for running tests in the devteam repo.
# Usage: ./run-tests.sh [go|ui|all]
# The agent should use this instead of running test commands manually.
set -e

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
WHAT="${1:-all}"

run_go_tests() {
  echo "=== Running Go tests ==="
  cd "$REPO_DIR"
  PATH="$PATH:/usr/local/go/bin" go test ./... -count=1 -timeout 120s 2>&1
  echo "=== Go tests: exit $? ==="
}

run_ui_tests() {
  echo "=== Running UI tests ==="
  UI_DIR="$REPO_DIR/ui"
  if [ ! -f "$UI_DIR/package.json" ]; then
    echo "No ui/package.json found, skipping UI tests"
    return
  fi
  
  # Install deps if needed
  if [ ! -d "$UI_DIR/node_modules" ]; then
    echo "Installing UI dependencies..."
    npm install --prefix "$UI_DIR" 2>&1 | tail -3
  fi
  
  # Run npm test if it has a test script
  if /usr/bin/grep -q '"test"' "$UI_DIR/package.json"; then
    echo "Running npm test..."
    cd "$UI_DIR"
    CI=true npm test 2>&1 || true
    echo "=== npm test: exit $? ==="
  fi
  
  # Run Playwright if configured
  if [ -f "$UI_DIR/playwright.config.ts" ]; then
    echo "Running Playwright tests on port 18765..."
    cd "$UI_DIR"
    # Install browsers if needed
    npx playwright install chromium 2>&1 | tail -3 || true
    # Run with START_SERVER=1 so Playwright starts its own server on :18765
    # Do NOT use port 8765 — that's the production devteam-web service
    PATH="$PATH:/usr/local/go/bin" START_SERVER=1 SERVER_PORT=18765 BASE_URL=http://localhost:18765 \
      npx playwright test --reporter=line 2>&1 || true
    echo "=== Playwright: exit $? ==="
  fi
}

case "$WHAT" in
  go) run_go_tests ;;
  ui) run_ui_tests ;;
  all) run_go_tests; run_ui_tests ;;
  *) echo "Usage: $0 [go|ui|all]"; exit 1 ;;
esac