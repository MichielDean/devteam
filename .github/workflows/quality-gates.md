# Quality Gates — settings-and-admin-ui

**Feature ID**: settings-and-admin-ui
**Stage**: 3.7 — CI Pipeline
**Phase**: construction
**Author**: pipeline-deploy
**Scope**: feature
**Depth**: standard
**Created**: 2026-07-06

---

## 0. Purpose & Baseline Shift

This artifact defines the quality gates that block promotion of a `settings-and-admin-ui` commit to release-candidate status. It is a **delta over the existing gate set** from the sibling feature `full-crud-and-ui-for-managing-repositories` (which landed `.github/workflows/quality-gates.md` and the `gate` job in `ci.yml` on `main`). A commit is a release candidate **only if** every blocking gate below is green (principle #1, `team-practices` P1).

> **Baseline shift note (load-bearing).** The `team-practices` artifact (2.2) and `quality-report` (3.6) both recorded "No CI" and "No coverage gate" as inherited gaps. Those findings were accurate at 2.2's HEAD (`ecd1f71`) but are now **partially stale**: a CI workflow with a `gate` aggregate job exists on `main`. The "no coverage gate" finding remains accurate (the sibling feature's gates do not include a coverage threshold). This artifact operates against the current baseline (CI + aggregate gate present, coverage gate absent). See `ci-config.md` §0 for the full drift note.

### MVP scope (binding, per 1.7-gate strict decisions)

Per the CONTEXT.md human responses (Q1/Q2 honored strictly), **MVP = admin shell + repos tab**. The Providers tab and CI/CD tab are **cut from v1** — deferred to the `multi-provider-llm-configuration` and a future `ci-cd-platform-config` sibling feature respectively. Defaults/Server/Audit are fast-follow within this feature. The gates below are scoped to the MVP first, with fast-follow gates recorded for when their Bolts land.

---

## 1. Gate Catalog

Each gate: **ID | Name | Command | Job in ci.yml | Blocks | Source**.

Inherited gates (G1–G9) are the sibling feature's set, unchanged in command and job. This feature's delta is in the **Blocks** column (which Bolt go/no-go each gate drives) and in the new gates G10–G12 for this feature's MVP.

### 1.1 Inherited gates (unchanged command/job)

| ID | Gate | Command | ci.yml job | Blocks | Source |
|----|------|---------|------------|--------|--------|
| G1 | Go build | `go build ./...` | `backend`, `backend-full` | release-candidate status | P7, R1 |
| G2 | Go vet | `go vet ./internal/db/... ./internal/api/... ./internal/repos/... ./internal/settings/... ./internal/opencode/... ./internal/config/...` | `backend` | release-candidate status | P7 (vet scope widened — see `ci-config.md` §5) |
| G3 | Go unit (db+api) | `go test ./internal/db/... ./internal/api/... -count=1` | `backend` | release-candidate status; Bolt 0/1 go/no-go | P7, FR-TEST-01, R1 |
| G4 | Go full suite | `go test ./... -count=1` | `backend-full` | release-candidate status (release gate) | P7, FR-TEST-05 |
| G5 | Dep-tidy (no unexpected deps) | `go mod tidy && git diff --exit-code go.mod go.sum` | `backend` | release-candidate status | R4, C4 |
| G6 | Frontend lint | `npm run lint` | `frontend` | (non-blocking — eslint not installed) | P7 |
| G7 | Frontend build (strict TS) | `npm run build` (`tsc -b && vite build`) | `frontend` | release-candidate status; Bolt 0/1 FE gate | P7, R10 |
| G8 | E2E smoke + non-regression | `npm run test:e2e` | `e2e` | (non-blocking — `continue-on-error: true`; pre-existing test-data gap) | P7, FR-TEST-02, R3 |
| G9 | Aggregate promotion gate | (meta — blocking set green) | `gate` | promotion to release-candidate / merge to main | role brief "promotion gates" |

### 1.2 Gates inherited but re-scoped by this feature

| ID | Gate | This feature's re-scope |
|----|------|--------------------------|
| G2 | Go vet | **Vet scope widened** to include `./internal/repos/... ./internal/settings/... ./internal/opencode/... ./internal/config/...` (the feature's new + extended packages). See `ci-config.md` §5. The widened scope is a strict superset of the sibling's; no regression. |
| G3 | Go unit (db+api) | **Picks up this feature's new tests** in `internal/db` (migration 018 round-trip, audit actor overload) and `internal/api` (settings handlers, auth guard, repos CRUD). The command is unchanged; the test set grows. |
| G4 | Go full suite | **Picks up `internal/repos`, `internal/settings`, `internal/opencode`, `internal/config` write-path tests.** The command is unchanged; the test set grows. |
| G5 | Dep-tidy | **Enforces the two endorsed deps**: `go-playground/validator` (Go) and `zod` (npm, via `npm ci` strict). If either is imported but uncommitted, G5 (Go) or `npm ci` (frontend) fails. |
| G8 | E2E | **Adds `admin.spec.ts`** to the spec set. Still non-blocking in v1 (`ci-config.md` §3.2). |

### 1.3 New gates for this feature's MVP

These are **not new CI jobs** — they are Go test functions that run inside the existing G3/G4 gates. They are called out as distinct gates because they map to MVP Bolt go/no-go decisions and to RAID-risk targeted tests (`quality-report` §3).

| ID | Gate | Test function(s) | Runs in | Blocks | Source |
|----|------|-------------------|---------|--------|--------|
| G10 | Migration 018 round-trip | `migration_018_test.go::TestMigration_RoundTrip`, `::TestMigration_AdditiveOnly`, `::TestMigration_ActorNullable` | G3 (db subset), G4 | Bolt 0 go/no-go; release-candidate status | FR-MIG-01..04, R-TEST-03, `test-results` §2.1 T-MIG-01..03 |
| G11 | Config write path (linchpin) | `config_test.go::TestWriteConfig_RoundTrip`, `::TestWriteConfig_CrashInjection`, `::TestWriteConfig_ValidationRejects`, `::TestReconcile_RegeneratesStaleYAML`, `::TestBootstrapFields_YAMLOnly` | G3 (config is under `internal/config`, currently in the db/api gate's package set? — **see note**) | Bolt 0 go/no-go; release-candidate status | FR-CONFIG-01..06, R-CONFIG-MATERIALIZE, R-CONFIG-VALIDATION, `test-results` §2.2 T-CONFIG-01..06 |
| G12 | Auth guard (fail-closed) | `auth_guard_test.go::TestAdminGuard_LocalhostAllowed`, `::TestAdminGuard_ValidTokenAllowed`, `::TestAdminGuard_NonLocalhostNoTokenRejected`, `::TestAdminGuard_FailClosedWhenEnvUnset` | G3 (api subset) | Bolt 0 go/no-go; release-candidate status; 4.1 security gate | FR-ROUTE-03, R-AUTH-ABSENT, `test-results` §2.5 T-AUTH-01..05 |

> **Note on G11's job placement.** `internal/config` is not in the sibling feature's G3 command (`go test ./internal/db/... ./internal/api/...`). The feature's config write-path tests therefore run in **G4** (`go test ./...`), not G3. This is acceptable for v1 — G4 is the release gate and is blocking. If the builder wants fast feedback on the linchpin (U-CONFIG-01) before the full suite, the `backend` job's G3 command can be widened to `go test ./internal/db/... ./internal/api/... ./internal/config/...`. This is a **contingent** `ci.yml` edit, recorded here so 3.6/4.1 don't re-derive it; the default is no edit (G4 covers it).

### 1.4 Fast-follow gates (not in MVP, recorded for when their Bolts land)

These gates are **defined now** but **not enforced in v1** because their Bolts are fast-follow within this feature (Defaults/Server/Audit) or deferred to sibling features (Providers/CI/CD). They are recorded so the delivery phase (4.x) and the reviewer gate (4.1) know what to enforce when each Bolt ships.

| ID | Gate | Test function(s) | Bolt | Source |
|----|------|-------------------|------|--------|
| G13 | Repos service concurrent-write | `service_test.go::TestService_ConcurrentWrites_NoLoss` | Bolt 1 (MVP) | FR-REPOS-03, R-REPOS-LOCK, T-REPOS-02 |
| G14 | Defaults precedence | `defaults_test.go::TestPrecedence_*` (4 branches) | Bolt 3 (fast-follow) | FR-DEF-02, R-DEFAULTS-SEMANTICS, T-DEF-01..04 |
| G15 | Server DSN write-only | `server_test.go::TestDSN_WriteOnly`, `::TestDSN_AuditMasked` | Bolt 4 (fast-follow) | FR-SEC-07, R-SERVER-DSN-EXPOSURE, T-SRV-02 |
| G16 | Audit query uses index | `audit_test.go::TestAuditQuery_UsesIndex` (EXPLAIN) | Bolt 5 (fast-follow) | FR-AUDIT-02, R-AUDIT-VOLUME, T-AUDIT-API-03 |
| G17 | opencode builder snapshot | `builder_test.go::TestBuilder_*` (4 providers, golden) | Bolt 2 (Providers — **deferred to sibling feature**) | FR-TEST-03, A-OPENCODE-CONTRACT, T-OPCODE-01..04 |
| G18 | CI/CD vaporware structural | `cicd_test.go::TestNoRuntimeConsumer` (grep) | Bolt 6 (CI/CD — **deferred to sibling feature**) | FR-CICD-04, R-CICD-VAPORWARE, T-CICD-03 |

**G13 is MVP** (Bolt 1 = repos, which is in MVP). It is listed under "fast-follow" only because it is a Bolt-1 gate, not a Bolt-0 gate — it enforces as soon as Bolt 1 lands, which is within MVP. G14–G16 are fast-follow within this feature. G17/G18 are deferred to sibling features per the 1.7-gate strict scope.

---

## 2. Gate Behavior

### 2.1 Pass condition (unchanged from sibling)

The `gate` job (G9) succeeds **only if** `backend`, `frontend`, and `backend-full` all return `result == 'success'`. It runs `if: always()` so it executes even when a dependency fails — and fails itself in that case. E2E (G8) is **informational only** in v1 (non-blocking, per `ci-config.md` §3.2).

### 2.2 Fail behavior

- Any blocking job failing → `gate` job fails → the commit is **not** a release candidate.
- On `pull_request`: the failed `gate` check is the feedback signal to the builder. Branch protection is not configured (private repo, free plan — see sibling `cd-config` §3.2); `deploy.sh`'s pre-flight `gh run list --branch main --limit 1` check is the branch-protection backstop.
- On `push` to `feature/**`: the failed run is the feedback signal; no auto-merge occurs (the pipeline pushes post-reviewer-gate, not on CI pass).

### 2.3 No bypass

Per principle #4 ("Bypassing a gate is an incident, not a shortcut"), there is no `continue-on-error` on any blocking job. The dep-tidy gate (G5) in particular is hard-failing — a new dependency is a Blocking violation of R4 unless explicitly waived with rationale. The two endorsed deps (`go-playground/validator`, `zod`) are committed to `go.mod`/`package.json` before push, so G5 passes; any **unendorsed** new dep fails G5.

**E2E (G8) is the sole `continue-on-error` job** — and it is informational, not blocking. This is the inherited posture from the sibling feature's `a7b859b` commit, retained for v1 per `ci-config.md` §3.2.

---

## 3. Gate ↔ Bolt Mapping (MVP)

The bolt-level go/no-go checkpoints map to CI gates as follows. This lets the builder read CI status as bolt status. **MVP = Bolts 0–1** (shell + repos, per 1.7-gate strict scope).

| Bolt | Bolt gate (from `quality-report` §6 / `bolt-plan`) | CI gate(s) | MVP? |
|------|-----------------------------------------------------|------------|------|
| 0 — Walking Skeleton | Migration 018 round-trip green; config write path round-trip + crash-injection green; auth guard fail-closed green; admin shell renders | G10, G11, G12, G7 (shell build), G8 (shell E2E, non-blocking) | ✅ MVP |
| 1 — Repos | Repos service concurrent-write green; repos CRUD handlers green; `admin.spec.ts` repos cases green; `GET /api/repos` shape unchanged | G13, G3 (api subset), G8 (admin.spec, non-blocking), G4 (non-regression) | ✅ MVP |
| 2 — Providers | opencode builder snapshot green; provider CRUD + masking green; dispatch sites call builder | G17, G3, G8 | ❌ Deferred (sibling feature) |
| 3 — Defaults | Defaults precedence 4-branch green; defaults handlers green | G14, G3 | Fast-follow |
| 4 — Server | DSN write-only green; restart classification green | G15, G3 | Fast-follow |
| 5 — Audit | Audit query uses index green; audit handlers green | G16, G3 | Fast-follow |
| 6 — CI/CD | CI/CD vaporware structural grep green; CI/CD handlers green | G18, G3 | ❌ Deferred (sibling feature) |

**MVP sign-off gate**: G9 (aggregate) green + G10 + G11 + G12 + G13 green (the MVP Bolt go/no-go gates) + 4.1 review gate verifies R-AUTH-ABSENT (G12) and R-REPOS-LOCK (G13) mitigated. E2E (G8) is informational.

---

## 4. Gate ↔ Review-Rule Mapping (from `discovered-rules`)

The `discovered-rules` artifact (2.2) defines R-BUILD-*, R-TEST-*, R-REL-* rules. The binding rules for this feature and their enforcing gates:

| Rule | Severity | Enforced by gate |
|------|----------|------------------|
| R-BUILD-01 (Go build command) | Blocking | G1 |
| R-BUILD-02 (new Go dep compiles into single binary) | Blocking | G1 + G5 (dep-tidy catches uncommitted) |
| R-BUILD-03 (frontend build command) | Blocking | G7 |
| R-BUILD-04 (new npm dep compiles into Vite bundle) | Blocking | G7 + `npm ci` strict |
| R-BUILD-06 (binary deployed from main only) | Blocking | G9 (gate must be green on main before `deploy.sh` runs) — enforced by `deploy.sh` pre-flight, not CI itself |
| R-TEST-01 (all Go tests pass before merge) | Blocking | G3, G4 |
| R-TEST-02 (plain `testing`, no framework) | Blocking | G3, G4 — a testify import fails to compile against the existing test pattern (reviewer-enforced; CI catches via build) |
| R-TEST-03 (migration round-trip test) | Blocking | G10 |
| R-TEST-05 (Playwright one spec per tab group) | Blocking | G8 — `admin.spec.ts` exists with grouped describes; no per-tab files |
| R-TEST-06 (existing E2E specs still pass) | Blocking | G8 (non-blocking in v1, but regressions surface in the run) + G4 (Go-side non-regression via `TestLoadConfig_Unchanged` etc.) |
| R-TEST-08 (no coverage gate) | (not a gate) | — inherited gap; `quality-report` §5.3 recommends a report upload, not a gate |

**Gates do not enforce everything.** Stylistic rules (no hardcoded hex, design-token usage) are reviewer-gate (4.1) enforced, not CI-enforced. CI enforces what is automatable: build, test, vet, type-check, E2E, dep discipline.

---

## 5. Gate ↔ RAID Risk Mapping (from `quality-report` §3)

Every RAID risk with a mitigation test, mapped to its enforcing gate. Status: **Planned** (tests designed in `test-results`, not yet executed — construction hasn't started on this feature branch).

| Risk | Mitigation test(s) | Enforcing gate | MVP? |
|------|---------------------|----------------|------|
| R-AUTH-ABSENT | T-AUTH-03 (401, no side effect), T-AUTH-04 (fail-closed env unset) | G12 | ✅ MVP (Bolt 0) |
| R-REPOS-LOCK | T-REPOS-02 (concurrent, `-race`) | G13 | ✅ MVP (Bolt 1) |
| R-CONFIG-MATERIALIZE | T-CONFIG-02 (crash-injection), T-CONFIG-05 (reconcile) | G11 | ✅ MVP (Bolt 0) |
| R-CONFIG-VALIDATION | T-CONFIG-03, T-PROV-01, T-CICD-01 | G11 (config), G3 (provider/cicd handler tests) | ✅ MVP (config side) / ❌ Deferred (provider/cicd side) |
| R-PROVIDER-KEYS | T-OPCODE-03, T-PROV-04, T-UI-PROV-01 | G17 (deferred) | ❌ Deferred (sibling feature) |
| R-SERVER-DSN-EXPOSURE | T-SRV-02, T-API-SET-04, T-UI-SRV-02 | G15 (fast-follow) | Fast-follow |
| R-SERVER-SELFMUTATION | T-UI-SRV-01, T-UI-SRV-03, T-SRV-01 | G15 (fast-follow) | Fast-follow |
| R-AUDIT-VOLUME | T-AUDIT-API-03 (EXPLAIN uses index) | G16 (fast-follow) | Fast-follow |
| R-CICD-VAPORWARE | T-CICD-03 (structural grep), T-UI-CICD-01 (banner) | G18 (deferred) | ❌ Deferred (sibling feature) |
| R-DEFAULTS-SEMANTICS | T-DEF-01..04 (table-driven) | G14 (fast-follow) | Fast-follow |
| R-PROVIDER-COPILOT | T-PROV-07, T-UI-PROV-04 | G17 (deferred) | ❌ Deferred (sibling feature) |

**MVP residual risk**: R-PROVIDER-KEYS, R-SERVER-DSN-EXPOSURE, R-SERVER-SELFMUTATION, R-AUDIT-VOLUME, R-CICD-VAPORWARE, R-DEFAULTS-SEMANTICS, R-PROVIDER-COPILOT remain open at MVP sign-off — their gates (G14–G18) are fast-follow or deferred. MVP sign-off requires only R-AUTH-ABSENT (G12) and R-REPOS-LOCK (G13) mitigated, plus the linchpin R-CONFIG-MATERIALIZE (G11). This is acceptable: MVP is shippable to the single trusted operator; the remaining risks are v1-completeness or sibling-feature risks, not MVP blockers.

---

## 6. Promotion Flow (input to 4.1 Deployment Pipeline)

Inherited from the sibling feature's `quality-gates.md`, unchanged in mechanics:

```
commit on feature/settings-and-admin-ui
        │
        ▼
   ci.yml runs (G1–G8 in parallel jobs; G10–G13 run inside G3/G4)
        │
        ▼
   gate job (G9) aggregates the blocking set {backend, frontend, backend-full}
        │
   ┌────┴────┐
   ▼         ▼
 pass      fail
   │         │
   ▼         ▼
release-   not a release
candidate  candidate — fix & re-push
   │
   ▼ (post-reviewer-gate 4.1, post-4.x)
merge to main → deploy.sh (4.3 re-runs G8 as smoke)
```

4.1 Deployment Pipeline consumes G9 (the `gate` job status) as its promotion input. 4.3 Deployment Execution re-runs **G8 (E2E)** post-deploy as the smoke test (P10: "deployment is not done until smoke passes"). `deploy.sh`'s pre-flight `gh run list --branch main` check is the branch-protection backstop for the private repo.

### 6.1 Migration 018 rollback trigger (input to 4.1)

Per principle #2 ("Rollback is not optional") and the existing `rollback.sh`:

- **Smoke gate (G8) fails post-deploy** → `deploy.sh` auto-invokes `rollback.sh`. The rollback procedure is git-based: `git revert` the merge commit on `main`, rebuild, restart the systemd unit.
- **Migration 018 is forward-only** (no `Down`). Code rollback (via `rollback.sh`) leaves migration 018's tables orphaned but harmless (additive, <1 MB, ignored by the reverted binary). Same posture as the sibling feature's migration 017.
- **Data rollback** (if migration 018 fails in production): manual `pg_restore` or forward-fix migration 019. No automated data rollback — inherited posture.
- **Config-state recovery (new this feature)**: the startup reconciler (U-CONFIG-01, FR-CONFIG-05) re-materializes stale YAML from DB on next successful boot. 4.1 should record this in the rollback runbook: "settings-store config-state recovery does not require DB restore; the reconciler handles it on next boot."

---

## 7. Coverage Posture (honest, inherited)

Per `quality-report` §5 and the sibling feature's `quality-gates.md`:

- **No coverage threshold gate in v1.** `quality-report` §5.3 recommends a `go test -cover` **report upload** (not a gate) so the coverage trend is visible. This feature does not add one. A hard threshold is deferred until the baseline is measured.
- **The requirements-coverage matrix** (`test-results` §4) is the binding surrogate: every Must FR and every RAID risk maps to at least one named test. This is stronger than a line-coverage number (quality principle 5).
- **`-race` is recommended, not binding.** `quality-report` §1.4 / QO-13 recommends `go test -race ./internal/repos/ ./internal/config/ ./internal/settings/...` for the concurrency-touching tests. This is **not** encoded as a CI gate in v1 — it's a developer-local discipline. If 4.1 finds concurrency issues, a `-race` CI job can be added as a fast-follow.

---

## 8. Self-Verification

1. **Every inherited gate (G1–G9) maps to a `ci.yml` job** — verified against the existing `ci.yml` on `main` (commits `bdbb7da` + `f6cbc1f` + `a7b859b`). ✓
2. **Every new gate (G10–G12) maps to a Go test function** in `test-results` §2 — no invented gates. G10 = T-MIG-01..03; G11 = T-CONFIG-01..06; G12 = T-AUTH-01..05. ✓
3. **No `continue-on-error` on any blocking job** — checked: only `e2e` (G8) has `continue-on-error: true`, and it is excluded from the `gate` blocking set. ✓
4. **Bolt mapping is consistent** with `bolt-plan` §2 and `quality-report` §6. MVP = Bolts 0–1; their gates (G10–G13) are the MVP sign-off set. ✓
5. **Rule mapping covers the binding `discovered-rules`** — R-BUILD-*, R-TEST-01..06, R-TEST-08 all have an enforcing gate or an explicit "not a gate" note. ✓
6. **Risk mapping covers the 11 RAID risks with mitigations** — every risk has an enforcing gate, marked MVP / fast-follow / deferred per the 1.7-gate strict scope. ✓
7. **Rollback trigger is concrete** — G8 failure post-deploy → `rollback.sh` (git revert + rebuild + restart); migration 018 forward-only; reconciler handles config-state recovery. ✓
8. **Scope discipline** — no coverage gate, no security scan gate, no performance gate, no cross-browser gate. Each omission is recorded with its source rationale (`ci-config.md` §7, `quality-report` §5). ✓
9. **MVP scope is binding** — G17 (Providers) and G18 (CI/CD) are explicitly deferred to sibling features per the 1.7-gate strict decisions; they are recorded for traceability but not enforced in v1. ✓
10. **Baseline shift is recorded** — §0 flags the 2.2/3.6 "no CI" staleness so 4.1/4.3 and the reviewer gate operate against the current baseline. ✓

*End of quality-gates artifact.*