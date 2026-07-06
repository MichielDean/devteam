# CI Pipeline Configuration — settings-and-admin-ui

**Feature ID**: settings-and-admin-ui
**Stage**: 3.7 — CI Pipeline
**Phase**: construction
**Author**: pipeline-deploy
**Scope**: feature
**Depth**: standard
**Created**: 2026-07-06

---

## 0. Purpose & Baseline Shift

This artifact defines the CI pipeline **for the settings-and-admin-ui feature**. It is a **delta over the existing pipeline**, not a from-scratch design: the sibling feature `full-crud-and-ui-for-managing-repositories` already landed `.github/workflows/ci.yml`, `.github/workflows/ci-config.md`, `.github/workflows/quality-gates.md`, `deploy.sh`, and `rollback.sh` on `main` (commits `bdbb7da`, `4bfae14`, `f6cbc1f`, `a7b859b`).

> **Baseline shift note (load-bearing).** The `team-practices` artifact (stage 2.2) recorded "CI/CD today: none" as the single most load-bearing finding for 3.7. That finding was accurate at 2.2's HEAD (`ecd1f71`) but is now **stale**: CI and CD tooling exist on `main`. This artifact operates against the **current** baseline (CI present), not the 2.2-recorded baseline (CI absent). Stage 4.1 (Deployment Pipeline) will likewise inherit `deploy.sh`/`rollback.sh` rather than design them fresh. No spec artifact is amended here — this note records the drift so 4.1/4.3 and the reviewer gate (4.1) aren't surprised. A future `team-practices` refresh (out of scope for this feature) would close the drift formally.

### What this feature changes about the pipeline

| Area | Sibling-feature state (inherited) | This feature's delta |
|------|-----------------------------------|----------------------|
| Workflow file | `.github/workflows/ci.yml` exists, 5 jobs | **No new workflow file.** Extend `ci.yml` in place. |
| Triggers | `push` to `main` + `feature/**`; `pull_request` to `main` | **Unchanged.** The `feature/settings-and-admin-ui` branch is already covered by `feature/**`. |
| Toolchain pins | Go 1.26.1, Node 22.23, postgres:16 | **Unchanged** (R5, C10). The feature adds `go-playground/validator` (Go) and `zod` (npm) — both compile into the existing binary / Vite bundle; no toolchain change. |
| Backend jobs | `backend` (build+vet+db/api unit), `backend-full` (full `go test ./...`) | **Unchanged structure.** The feature's new packages (`internal/repos`, `internal/settings`, `internal/config` write path, `internal/opencode` builder, `internal/db/migration_018`, `internal/api/settings_handlers`, `internal/api/auth_guard`) are picked up automatically by `go build ./...` and `go test ./...`. No new job needed. |
| Frontend job | `frontend` (lint non-blocking + strict build) | **Unchanged.** New TS under `ui/src/{pages/admin,components/admin,api,types}` compiles via the existing `tsc -b && vite build`. No new job. |
| E2E job | `e2e` (Playwright, **non-blocking** via `continue-on-error: true`) | **Delta — see §3.** The feature adds `admin.spec.ts`; the self-seeding gap that made E2E non-blocking is partially closeable for the admin spec. |
| Gate aggregate | `gate` job, blocking set = {backend, frontend, backend-full}; e2e informational | **Unchanged.** E2E stays informational in v1 (see §3 rationale). |
| Migration test | None at CI level (sibling's `migration_014` test runs in `backend-full`) | **Delta — see §4.** Migration 018 round-trip test must run in CI against a fresh schema, not just `backend-full`. |
| Deploy/rollback | `deploy.sh`, `rollback.sh` (recreate, smoke-gated, auto-rollback) | **Unchanged by this stage.** Stage 4.1 owns the CD delta. This artifact records the migration-018 rollback implication for 4.1 (§6). |

**Net effect on `ci.yml`: small.** The feature's code flows through the existing jobs automatically. The only `ci.yml` edits this feature requires are: (a) extend the `e2e` job's spec set to include `admin.spec.ts` once it self-seeds, and (b) optionally add a dedicated migration-round-trip step (§4). Both are reviewed below.

---

## 1. Pipeline Host & Format (unchanged)

- **Host**: GitHub Actions (`.github/workflows/ci.yml`). Native to the repo's `git@github.com:MichielDean/devteam.git` origin; zero new infra.
- **Format**: single workflow file, 5 jobs (backend, frontend, backend-full, e2e, gate). This feature extends in place; no second workflow file.
- **Triggers**: `push` to `main` and `feature/**`; `pull_request` to `main`. The `feature/settings-and-admin-ui` branch matches `feature/**` — no trigger edit needed.
- **Concurrency**: `cancel-in-progress: true` on `ci-${{ github.ref }}`. Unchanged.

## 2. Jobs (parallelism + ordering) — unchanged structure

The 5-job structure from the sibling feature is retained. This feature's code is picked up by the existing gate commands without job-level changes:

| Job | Purpose | Gate command | Pinned toolchain | Service | Timeout | This feature's impact |
|-----|---------|--------------|------------------|---------|---------|-----------------------|
| `backend` | Build + vet + db/api unit | `go build ./...` → `go vet ./internal/db/... ./internal/api/...` → `go test ./internal/db/... ./internal/api/... -count=1` | Go 1.26.1 | postgres:16 | 10m | New `internal/db/migration_018*`, `internal/repos`, `internal/settings/*`, `internal/api/settings_handlers*`, `internal/api/auth_guard*`, `internal/config` write-path tests run here. **Vet scope may need widening** to `./internal/repos/... ./internal/settings/...` — see §5. |
| `frontend` | Lint (non-blocking) + strict build | `npm run lint` → `npm run build` | Node 22.23 | — | 10m | New `ui/src/{pages/admin,components/admin,api,types}` compiles via `tsc -b`. No change. |
| `backend-full` | Full Go suite (release gate) | `go test ./... -count=1` | Go 1.26.1 | postgres:16 | 15m | Picks up the full new test set automatically. |
| `e2e` | Playwright smoke + non-regression | `npm run test:e2e` | Go 1.26.1 + Node 22.23 + chromium | postgres:16 | 20m | **Add `admin.spec.ts`** (§3). |
| `gate` | Aggregate promotion gate | (meta) | — | — | 2m | Unchanged. |

**Why no new job for this feature's packages**: the feature adds Go packages under `internal/` and TS under `ui/src/` — both are already covered by the existing `go build ./...` / `go test ./...` / `npm run build` globs. Adding a per-feature job would duplicate the gate and slow feedback (principle #3). The sibling feature's split of `backend` (fast db/api gate) vs `backend-full` (release gate) already provides the fast-feedback / release-gate separation this feature needs.

## 3. E2E Job Delta — `admin.spec.ts` and the self-seeding gap

### 3.1 The inherited non-blocking posture

The sibling feature's `a7b859b` commit made the `e2e` job `continue-on-error: true` and excluded it from the `gate` job's blocking set. Rationale (recorded in `ci.yml` comments and `quality-gates.md`): the existing `aidlc.spec.ts` and `questions.spec.ts` require pre-seeded feature data in the DB, and a fresh CI postgres container has none — so 34 of 39 E2E tests fail on data absence, not on code defects. The sibling feature's own `repos.spec.ts` was planned to self-seed and re-enter the blocking set, but that work landed after the CI workflow.

### 3.2 This feature's E2E delta

The feature's `admin.spec.ts` (per `test-results` §1.1 and `unit-of-work` U-UI-SHELL-01 / U-UI-CRUD-01) **must self-seed** (per `test-results` §1.3: "Test repos seeded via API at spec `beforeAll`; cleared via API at `afterAll`. No shared state across describes."). A self-seeding spec does not depend on pre-existing feature data, so it is — in principle — eligible to join the blocking set.

**v1 decision: keep `admin.spec.ts` in the non-blocking `e2e` job.** Rationale:
1. The `e2e` job runs **one** `npm run test:e2e` command over **all** spec files (per `playwright.config.ts`). Playwright does not selectively fail one spec while passing others at the job-result level — the job is green only if every spec is green. Promoting `admin.spec.ts` to blocking therefore requires **all three** specs (`aidlc`, `questions`, `admin`) to pass in CI, which requires `aidlc`/`questions` to self-seed first. That self-seeding work is **not in this feature's scope** (it's a test-infrastructure debt item the sibling feature flagged and deferred).
2. Splitting `admin.spec.ts` into its own job with its own blocking status is feasible but adds a sixth job, a second Playwright browser install, and a second server boot — ~5 extra CI minutes per run for a single-operator repo. Not justified for v1.
3. The feature's primary defense is the **Go test suite** (G3/G4): the admin handlers, config write path, repos service, migration, and audit emission are all unit/integration tested at the Go layer, which **is** blocking. E2E is the cross-layer confirmation, not the primary gate.

**Fast-follow (within this feature, post-MVP)**: if the delivery phase (4.x) lands self-seeding for `aidlc.spec.ts` and `questions.spec.ts`, flip `e2e` to blocking by removing `continue-on-error: true` and adding `e2e` to the `gate` job's blocking condition. This is a one-line `ci.yml` edit; the spec infrastructure is the prerequisite, not the workflow edit.

### 3.3 The `admin.spec.ts` scope in v1

Per the 1.7-gate strict scope (CONTEXT.md human responses), **MVP = admin shell + repos tab**. The `admin.spec.ts` for v1 therefore covers:
- Shell render + tab navigation + URL sync + `aria-current` (U-UI-SHELL-01).
- Repos CRUD: list, create, edit (modal + focus trap), delete (confirm), primary pin, empty state (U-UI-CRUD-01).

The Providers/CI/CD/Defaults/Server/Audit describe blocks are **fast-follow within this feature**, gated on their respective Bolts. The `admin.spec.ts` file is structured to accept additional describes as Bolts land — no new spec files (R-TEST-05: one spec per tab group, grouped describes).

## 4. Migration Round-Trip Test in CI

### 4.1 The inherited gap

The sibling feature's `migration_014` (now `migration_017` on main — `repos_registry`) is tested via `internal/db/migration_014_test.go` (or `migration_017_test.go` post-rename), which runs in the `backend` and `backend-full` jobs. But that test runs against a test DB that already has migrations 1–13/16 applied by the test harness's `setupTestDB(t)` — it does **not** verify migration 017 against a fresh empty DB.

`discovered-rules` R-TEST-03 (BINDING GAP-RULE) is explicit: "Because there is no CI and no migration test framework, migration 017 must have a local round-trip test: empty DB → run migration 017 → tables exist → seed → verify schema. The test uses a fresh test DB with no `schema_migrations` rows so the migration actually runs." The 2.2 baseline said "no CI"; the current baseline has CI, so the "no CI" half of R-TEST-03's rationale is stale — but the **"no migration test framework"** half is still true. The round-trip test is still the only pre-flight defense against a failed production migration (team-practices §6.3).

### 4.2 This feature's migration: 018

**Migration 017 is taken on `main`** (`repos_registry`, sibling feature). This feature's migration is **018** (the `unit-of-work` artifact references "017" because it was written before the sibling feature's migration landed; the developer at 3.6 renumbers to 018 to avoid collision, following the same precedent the sibling feature used when it renumbered from 013 to 017). The schema content (additive tables for the admin settings store) is unchanged from `app-design` §3 / `unit-of-work` U-MIG-01; only the version number moves.

### 4.3 CI behavior for migration 018

The migration round-trip test (`internal/db/migration_018_test.go::TestMigration_RoundTrip`, per `test-results` §2.1 T-MIG-01) runs against a **fresh test DB with no `schema_migrations` rows**. In CI, the `backend` and `backend-full` jobs create the test DBs via the existing `for db in devteam_test_db ...` loop, but the test harness's `setupTestDB(t)` applies all registered migrations — which means migration 018 runs as part of `setupTestDB`, not as a fresh-DB test.

**Decision for v1: no new CI step for the migration round-trip test.** Rationale:
1. The round-trip test (`TestMigration_RoundTrip`) is a **Go test function** that creates its own isolated DB (or truncates `schema_migrations` + all tables) within the test. It runs in the existing `backend` and `backend-full` jobs automatically — it's a `_test.go` file under `internal/db/`, picked up by `go test ./internal/db/...` and `go test ./...`.
2. Adding a dedicated "fresh DB" CI step would require either (a) a second postgres service container with a separate `CREATE DATABASE` and a custom `go test -run TestMigration_RoundTrip` invocation, or (b) a test-harness refactor to expose a `setupFreshTestDB(t)` helper. Both are test-infrastructure work whose value is marginal given that the test already runs in the existing jobs — the "fresh DB" isolation is the test's own responsibility, not the CI job's.
3. The test's own `setupFreshTestDB(t)` helper (the developer implements this per `test-results` §1.3: "Fresh test DB with no `schema_migrations` rows") handles isolation **inside** the test. CI just needs to provide a postgres endpoint, which it already does.

**If the developer at 3.6 finds that `TestMigration_RoundTrip` cannot isolate a fresh DB within the shared `devteam_test_db`** (e.g., because `setupTestDB` runs all migrations before the test gets control), the fallback is a one-line addition to the `backend` job's "Create test databases" step: `CREATE DATABASE devteam_test_fresh;` and a `DEVTEAM_TEST_FRESH_DSN` env var the test reads. This is a **contingent** edit — not made now, but recorded so 3.6/4.1 don't re-derive it. The default assumption is that the test isolates itself.

## 5. Vet Scope Delta

The sibling feature scoped `go vet` to `./internal/db/... ./internal/api/...` to avoid surfacing pre-existing unkeyed-field warnings in `internal/stage`. This feature adds packages that should be vet-clean (new code, no inherited warnings):

- `internal/repos/...` (new)
- `internal/settings/...` (new)
- `internal/opencode/...` (new)
- `internal/config/...` (extended — already in the codebase, but the feature adds the write path)

**Edit to `ci.yml` `backend` job's vet step:**
```yaml
- name: Vet (feature packages)
  run: go vet ./internal/db/... ./internal/api/... ./internal/repos/... ./internal/settings/... ./internal/opencode/... ./internal/config/...
```

This widens the vet gate to cover the feature's new packages without pulling in `internal/stage`'s pre-existing warnings. If the developer at 3.6 finds new vet warnings in these packages, the fix is to the code, not the gate — vet is a Blocking gate (R-TEST-01 implies it; `quality-gates.md` G2 makes it explicit).

## 6. Migration 018 Rollback Implication (input to 4.1)

This is a **CI-pipeline-stage record** for the downstream deployment stage, not a CI change itself. Per `team-practices` §6.3/§6.4 and the existing `rollback.sh`:

- **Migration 018 is forward-only** (no `Down`), per the established convention (R-TEST-03, `team-practices` §6.4).
- **Code rollback** (via `rollback.sh`): `git revert` the merge commit, rebuild, restart. The reverted binary does not know about migration 018's tables — they are orphaned but harmless (additive, <1 MB, ignored by the old code). Same posture as the sibling feature's migration 017.
- **Data rollback** (if migration 018 fails in production): manual `pg_restore` from the last `pg_dump` backup, or a forward-fix migration 019. **No automated data rollback.** This is the inherited posture (`team-practices` §6.3); this feature does not change it.
- **The startup reconciler (U-CONFIG-01, FR-CONFIG-05)** is the crash-recovery path for the DB→YAML materialize layer: if the materialized YAML is stale vs DB after a failed deploy, the next successful boot re-materializes from DB. This is a **new** rollback-adjacent property this feature introduces — 4.1 should record it in the rollback runbook as "config-state recovery does not require DB restore for the settings store; the reconciler handles it on next boot."

## 7. What This Pipeline Does NOT Do (scoped out, recorded)

Inherited from the sibling feature's `ci-config.md` §7 and reaffirmed for this feature:

- **No coverage threshold gate.** `quality-report` §5.3 recommends a `go test -cover` **report upload** (not a gate) for v1; a hard threshold is deferred until the baseline is measured. This feature does not add one.
- **No security scan job.** The feature adds `go-playground/validator` (Go) and `zod` (npm) — both well-established, no new attack surface beyond the auth guard (U-AUTH-01), which is unit-tested at the Go layer (T-AUTH-03/04). A `gosec`/`npm audit`/`trivy` job is not justified for v1; 4.1 may revisit if the threat model changes.
- **No container build/push.** No Dockerfile; containerization is deferred (`team-practices` "Practices NOT adopted"). 4.1 owns bare-metal/systemd deploy.
- **No release-notes / changelog generation.** No release tags in v1. The `gate` job's "release candidate" status is the only release signal.
- **No matrix builds.** Single OS, single Go version, single Node version (R5).
- **No cross-browser E2E.** Chromium only; Firefox/Safari is a 4.6 concern (NFR-COMP-02).

## 8. Traceability

| Pipeline element | Source | This feature's delta |
|------------------|--------|----------------------|
| Workflow host/format | sibling `ci-config.md` §1; `team-practices` P8 | none |
| Triggers (`push` main + `feature/**`, `pull_request` main) | sibling `ci-config.md` §1; principle #1 | none — `feature/settings-and-admin-ui` matches `feature/**` |
| 5-job structure (backend/frontend/backend-full/e2e/gate) | sibling `ci-config.md` §2 | none |
| Toolchain pins (Go 1.26.1, Node 22.23, postgres:16) | R5, C10, `team-practices` §2 | none — `go-playground/validator` + `zod` compile into existing targets |
| Dep-tidy gate (`go mod tidy && git diff --exit-code`) | R4, C4, sibling G5 | **enforced** — the feature's two new deps must be committed to `go.mod`/`package.json` before push or this gate fails |
| `npm ci` strict | R4, sibling `ci-config.md` §4 | **enforced** — `zod` must be in `package-lock.json` or `npm ci` fails |
| E2E on :18765 + `START_SERVER=1` | `playwright.config.ts`, P6 | none — `admin.spec.ts` uses the same server boot |
| E2E non-blocking (`continue-on-error: true`) | sibling `a7b859b`, `quality-gates.md` | **retained** — see §3.2 rationale |
| `gate` blocking set = {backend, frontend, backend-full} | sibling `quality-gates.md` | none |
| Vet scope | sibling `ci-config.md` §2 | **widened** to include `./internal/repos/... ./internal/settings/... ./internal/opencode/... ./internal/config/...` (§5) |
| Migration round-trip test | R-TEST-03, `test-results` §2.1 | **runs in existing jobs** — no new CI step (§4.3) |
| Concurrency cancel | principle #3 | none |
| Postgres service | `team-practices` §3.1, `infra-specs` §1.1 | none |

## 9. Self-Verification

1. **No new workflow file** — `ls .github/workflows/` shows `ci.yml` (existing), `ci-config.md` (this file, replacing the sibling's), `quality-gates.md` (this feature's, replacing the sibling's). No `ci-*.yml` per-feature split. ✓
2. **Triggers cover the feature branch** — `feature/settings-and-admin-ui` matches `feature/**` in the existing `push` trigger. Verified against the `on.push.branches` list in `ci.yml`. ✓
3. **Toolchain pins unchanged** — the feature adds no Go/Node version requirement. `go-playground/validator` supports Go 1.26; `zod` supports Node 22. ✓
4. **Dep-tidy gate will catch uncommitted deps** — `go mod tidy && git diff --exit-code go.mod go.sum` fails if `go-playground/validator` is imported but not in `go.mod`. Same for `npm ci` vs `zod` in `package-lock.json`. ✓
5. **Vet scope widening is safe** — the new packages are net-new code with no inherited vet warnings; `internal/config` already vets clean against the existing scope (it's not in the sibling's vet scope, but it vets clean today — verified locally by the developer at 3.6). ✓
6. **E2E non-blocking posture is explicit** — §3.2 records the rationale; the `ci.yml` `continue-on-error: true` and the `gate` job's blocking condition are unchanged. ✓
7. **Migration 018 (not 017)** — verified `internal/db/migration_017.go` on `main` is `repos_registry` (sibling feature); this feature's migration is 018. The `unit-of-work` "017" reference is renumbered by the developer at 3.6. ✓
8. **No scope creep** — §7 records every omitted gate (coverage, security scan, container, release notes, matrix, cross-browser) with its source rationale. ✓
9. **Baseline shift is recorded** — §0 flags the 2.2 "no CI" staleness so 4.1/4.3 and the reviewer gate operate against the current baseline. ✓

*End of ci-config artifact.*