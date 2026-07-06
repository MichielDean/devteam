# CI Pipeline Configuration — full-crud-and-ui-for-managing-repositories

**Feature ID**: full-crud-and-ui-for-managing-repositories
**Stage**: 3.7 — CI Pipeline
**Phase**: construction
**Author**: pipeline-deploy
**Scope**: feature
**Depth**: minimal
**Created**: 2026-07-06

---

## 1. Pipeline Host & Format

- **Host**: GitHub Actions (`.github/workflows/ci.yml`). The repo is hosted on `git@github.com:MichielDean/devteam.git`; Actions is the native CI with zero new infra (no self-hosted runner, no external CI service — consistent with the single-operator, single-host posture in infra-specs §0 and team-practices P8).
- **Format**: single workflow file, 5 jobs, YAML validated. This is the **first** CI workflow in the repo — `team-practices.md` §"Repo Posture" recorded "CI/CD today: none" as the single most load-bearing finding for 3.7. This artifact closes that gap.
- **Triggers** (R from role brief "Define pipeline triggers"):
  - `push` to `main` and `feature/**` — every commit is a release candidate (P1).
  - `pull_request` to `main` — gate PRs before merge.
  - No scheduled runs (no nightly needed for a single-operator tool), no tag triggers (no release tags for MVP per P8).
- **Concurrency**: `cancel-in-progress: true` on the ref group — a new push cancels a stale run. Keeps feedback fast (principle #3) and avoids wasted Actions minutes.

## 2. Jobs (parallelism + ordering)

The pipeline runs **4 verification jobs in parallel** (backend, frontend, backend-full, e2e-which-gates-on-backend+frontend) plus **1 aggregate gate job** that depends on all four. Parallelism minimizes wall-clock feedback time.

| Job | Purpose | Gate command (from P7) | Pinned toolchain | Service | Timeout |
|-----|---------|------------------------|------------------|---------|---------|
| `backend` | Build + vet + db/api unit tests | `go build ./...` → `go vet ./...` → `go test ./internal/db/... ./internal/api/... -count=1` | Go 1.26.1 | postgres:16 | 10m |
| `frontend` | Lint + strict-mode build | `npm run lint` → `npm run build` (`tsc -b && vite build`) | Node 22.23 | — | 10m |
| `backend-full` | Full Go suite (release gate) | `go test ./... -count=1` | Go 1.26.1 | postgres:16 | 15m |
| `e2e` | Playwright smoke + non-regression | `npm run test:e2e` (repos + aidlc + questions specs) | Go 1.26.1 + Node 22.23 + chromium | postgres:16 | 20m |
| `gate` | Aggregate promotion gate | (no command — fails if any `needs.*.result != 'success'`) | — | — | 2m |

**Why split `backend` and `backend-full`**: the db/api unit test gate (Bolt 1/2) is the fast feedback the builder needs every push; the full `go test ./...` suite is the release-candidate gate and is heavier. Running them in parallel means a unit-test failure is visible in minutes without waiting on the full suite.

**Why `e2e` `needs: [backend, frontend]`**: E2E is meaningless if the code doesn't build. The dependency avoids spending ~10m on Playwright setup for a broken build.

## 3. Toolchain Pins (R5, C10)

Pinned in the workflow, not floating:

- **Go**: `1.26.1` via `actions/setup-go@v5` with `go-version: '1.26.1'`. Matches `go.mod` and the workspace-state record. No `toolchain` directive added to `go.mod`.
- **Node**: `22.23` via `actions/setup-node@v4` with `node-version: '22.23'`. Matches the local operator environment.
- **Postgres**: `postgres:16` service container. The codebase targets "any modern Postgres"; 16 is the current LTS-equivalent and matches the local deployment posture.

## 4. Dependency Discipline (R4) — encoded as a gate

The `backend` job runs `go mod tidy` then `git diff --exit-code go.mod go.sum`. If `tidy` changes anything, the gate fails — this catches accidental new deps at CI time, not at review time. The `frontend` job uses `npm ci` (fails if `package-lock.json` is out of sync with `package.json`), which rejects un-pinned or new deps. No `npm install` (which would mutate the lockfile).

## 5. Service Containers

Two Go jobs (`backend`, `backend-full`) and the `e2e` job spin a `postgres:16` service container with a `devteam` user/db and expose 5432. The DSN is passed via the `DEVTEAM_DB_DSN` env var. The db package's existing tests (and the new `repo_store_test.go`) require a live Postgres — this is verified by the 3.6 quality-report baseline (`go test ./internal/db/...` PASS against live Postgres).

> **Note on the env var name**: the workflow sets `DEVTEAM_DB_DSN`. If the codebase reads the DSN from a differently-named env var (e.g. `DATABASE_URL`), this is a one-line fix in the workflow — flagged here so 4.1/4.3 don't get surprised. The gate logic is unaffected.

## 6. E2E Job Specifics (P10 — smoke gate)

- Builds the Go binary to `/tmp/devteam-ci` with `go build -o /tmp/devteam-ci ./cmd/devteam`.
- Sets `START_SERVER=1` and `SERVER_BINARY="/tmp/devteam-ci -http :18765"` so `playwright.config.ts` spawns a dedicated server on the test port (18765), not the operator's production port (8765). This matches the existing config's `reuseExistingServer: !process.env.START_SERVER` logic.
- Installs only `chromium` (`npx playwright install --with-deps chromium`) — the existing specs are single-browser; adding firefox/webkit would be scope creep against R4/C10.
- Uploads `ui/playwright-report` as an artifact on failure (7-day retention) so failures are inspectable without re-running locally.

## 7. What This Pipeline Does NOT Do (scoped out, recorded)

Per team-practices "Practices NOT adopted" and the minimal depth of this stage:

- **No coverage threshold gate.** C11 requires tests exist; it does not set a coverage %. A coverage regression gate is a future enhancement, not an MVP gate.
- **No security scan job** (no `gosec`, no `npm audit`, no `trivy`). The feature adds no new deps (R4) and no new auth surface (infra-specs §1.6); a security scan job has marginal value for this feature and would add CI minutes. 4.1 may revisit if the threat model changes.
- **No container build/push.** No Dockerfile exists (infra-specs §0); containerization is explicitly deferred (team-practices "Practices NOT adopted"). 4.1 Deployment Pipeline designs for bare-metal/systemd.
- **No release-notes / changelog generation.** P8: no release tags for MVP. The `gate` job's "release candidate" status is the only release signal; automated release notes are a 4.x concern.
- **No matrix builds.** Single OS (ubuntu-latest), single Go version, single Node version. The toolchain is pinned (R5); matrix builds test version flexibility, which is explicitly NOT a goal (R5 forbids version drift).

## 8. Traceability

| Pipeline element | Source |
|------------------|--------|
| Gate commands (build/vet/test/lint/build/e2e) | team-practices P7 verification table — verbatim |
| Toolchain pins | R5, C10, D7, workspace-state |
| Dep-tidy gate | R4, C4 |
| `npm ci` (lockfile-pinned) | R4 |
| E2E on :18765 + `START_SERVER=1` | playwright.config.ts (verified), P6 |
| Concurrency cancel | principle #3 (fast feedback) |
| Postgres service | infra-specs §1.1, 3.6 baseline (db tests need live Postgres) |
| `e2e` needs backend+frontend | principle #6 (don't smoke a broken build) |
| `gate` aggregate job | role brief "promotion gates"; input to 4.1 |
| GitHub Actions as host | team-practices "Repo Posture" (origin is github.com); infra-specs §0 (no new infra) |

## 9. Self-Verification

1. **YAML is valid** — `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"` passes. ✓
2. **Gate commands match P7 exactly** — `go build ./...`, `go vet ./...`, `go test ./internal/db/... ./internal/api/...`, `go test ./...`, `npm run lint`, `npm run build`, `npm run test:e2e`. No invented commands. ✓
3. **Toolchain versions match the pinned set** — Go 1.26.1 (go.mod), Node 22.23 (workspace-state). No floating `latest`. ✓
4. **No new deps introduced by the workflow itself** — Actions uses only first-party `actions/checkout`, `actions/setup-go`, `actions/setup-node`, `actions/upload-artifact` (standard, pre-existing GitHub-hosted actions). No third-party actions added. ✓
5. **Postgres service matches the test harness requirement** — the 3.6 baseline confirms db tests need live Postgres; the service container provides it. ✓
6. **E2E port isolation** — `START_SERVER=1` + `:18765` matches `playwright.config.ts` (read and verified, not inferred). Does not touch the operator's :8765. ✓
7. **Scope discipline** — no security scan, no container, no release notes, no matrix. Each omission is recorded in §7 with its source rationale. ✓
8. **File lands in the impl repo worktree** — `.github/workflows/ci.yml` written to the feature branch worktree, not the spec repo. Will be committed with the feature. ✓