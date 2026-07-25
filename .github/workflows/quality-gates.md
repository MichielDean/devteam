# Quality Gates — settings-and-admin-ui (REVISED)

**Feature ID**: settings-and-admin-ui
**Stage**: 3.7 — CI Pipeline
**Phase**: construction
**Author**: pipeline-deploy
**Scope**: feature
**Depth**: standard
**Created**: 2026-07-06
**Revision**: 2 — honors human input Q7/Q8 strictly (Providers cut from v1 → sibling feature; CI/CD cut entirely → separate feature). Aligns the gate set to `bolt-plan` revision 2, `test-results` revision 2, and `quality-report` revision 2. Renumber the Bolt→gate mapping to the revised 5-Bolt v1 sequence (Bolt 2 = Defaults MVP-complete, not Providers; Bolt 5 = Providers gated fast-follow; Bolt 6 removed). Move G14 (Defaults precedence) into the MVP sign-off set. Remove G17 (opencode builder) and G18 (CI/CD vaporware) — those risks are resolved by scope cut, not deferred. Add the `-race` gate (G19) per quality-report QO-14 and the audit-constants structural gate (G20) per T-AUDIT-CONST-01. Align test IDs to `test-results` revision 2.

---

## 0. Purpose & Baseline Shift

This artifact defines the quality gates that block promotion of a `settings-and-admin-ui` commit to release-candidate status. It is a **delta over the existing gate set** from the sibling feature `full-crud-and-ui-for-managing-repositories` (which landed `.github/workflows/quality-gates.md` and the `gate` job in `ci.yml` on `main`). A commit is a release candidate **only if** every blocking gate below is green (principle #1, `team-practices` P1).

> **Baseline shift note (load-bearing).** The `team-practices` artifact (2.2) and `quality-report` (3.6) both recorded "No CI" and "No coverage gate" as inherited gaps. Those findings were accurate at 2.2's HEAD (`ecd1f71`) but are now **partially stale**: a CI workflow with a `gate` aggregate job exists on `main`. The "no coverage gate" finding remains accurate (the sibling feature's gates do not include a coverage threshold). This artifact operates against the current baseline (CI + aggregate gate present, coverage gate absent). See `ci-config.md` §0 for the full drift note.

### v1 scope (binding, per 1.7-gate strict decisions)

Per the CONTEXT.md human responses (Q7/Q8 honored strictly) and `bolt-plan` revision 2, **v1 = admin shell + repos + defaults + server + audit (Bolts 0–4)**. MVP (shippable) = **Bolts 0–2** (shell + repos + defaults). The Providers tab is a **gated fast-follow** integration Bolt (Bolt 5) wired into the sibling feature `multi-provider-llm-configuration` once it reaches 2.6 — it is not "deferred to a sibling feature and tracked here," it is **resolved by scope cut** for this feature's v1 (the sibling feature owns the provider concept end-to-end). The CI/CD tab is a **separate `ci-cd-platform-config` feature** — likewise resolved by scope cut, not tracked here. The gates below are scoped to the v1 sequence first, with the gated Providers integration recorded for when its Bolt activates.

---

## 1. Gate Catalog

Each gate: **ID | Name | Command | Job in ci.yml | Blocks | Source**.

Inherited gates (G1–G9) are the sibling feature's set, unchanged in command and job. This feature's delta is in the **Blocks** column (which Bolt go/no-go each gate drives) and in the new gates G10–G20 for this feature's v1.

### 1.1 Inherited gates (unchanged command/job)

| ID | Gate | Command | ci.yml job | Blocks | Source |
|----|------|---------|------------|--------|--------|
| G1 | Go build | `go build ./...` | `backend`, `backend-full` | release-candidate status | P7, R1 |
| G2 | Go vet | `go vet ./internal/db/... ./internal/api/... ./internal/repos/... ./internal/settings/... ./internal/config/...` | `backend` | release-candidate status | P7 (vet scope widened — see `ci-config.md` §5.1; `internal/opencode` excluded per scope cut) |
| G3 | Go unit (db+api+config) | `go test ./internal/db/... ./internal/api/... ./internal/config/... -count=1` | `backend` | release-candidate status; Bolt 0/1/2 go/no-go | P7, FR-TEST-01, R1 (`internal/config` added — `ci-config.md` §5.2) |
| G4 | Go full suite | `go test ./... -count=1` | `backend-full` | release-candidate status (release gate) | P7, FR-TEST-05 |
| G5 | Dep-tidy (no unexpected deps) | `go mod tidy && git diff --exit-code go.mod go.sum` | `backend` | release-candidate status | R4, C4 |
| G6 | Frontend lint | `npm run lint` | `frontend` | (non-blocking — eslint not installed) | P7 |
| G7 | Frontend build (strict TS) | `npm run build` (`tsc -b && vite build`) | `frontend` | release-candidate status; Bolt 0/1/2 FE gate | P7, R10 |
| G8 | E2E smoke + non-regression | `npm run test:e2e` | `e2e` | (non-blocking — `continue-on-error: true`; pre-existing test-data gap) | P7, FR-TEST-02, R3 |
| G9 | Aggregate promotion gate | (meta — blocking set green) | `gate` | promotion to release-candidate / merge to main | role brief "promotion gates" |

### 1.2 Gates inherited but re-scoped by this feature

| ID | Gate | This feature's re-scope |
|----|------|--------------------------|
| G2 | Go vet | **Vet scope widened** to include `./internal/repos/... ./internal/settings/... ./internal/config/...` (the feature's new + extended packages). `internal/opencode` is **excluded** — the builder package is owned by the sibling feature (scope cut, Q7). See `ci-config.md` §5.1. The widened scope is a strict superset of the sibling's; no regression. |
| G3 | Go unit (db+api+config) | **`internal/config` added to the fast gate** (`ci-config.md` §5.2) so the linchpin (U-CONFIG-01) fails fast. Picks up this feature's new tests in `internal/db` (migration 018 round-trip, audit actor overload, audit constants structural), `internal/api` (settings handlers, auth guard, repos CRUD), and `internal/config` (write path, reconcile, masking helper). The command is widened; the test set grows. |
| G4 | Go full suite | **Picks up `internal/repos`, `internal/settings`, `internal/config` write-path tests.** The command is unchanged; the test set grows. (`internal/opencode` is not built by this feature — scope cut.) |
| G5 | Dep-tidy | **Enforces the two endorsed deps**: `go-playground/validator` (Go) and `zod` (npm, via `npm ci` strict). If either is imported but uncommitted, G5 (Go) or `npm ci` (frontend) fails. |
| G8 | E2E | **Adds `admin.spec.ts`** to the spec set (5 v1 describe groups + 2 placeholder assertions — `ci-config.md` §3.3). Still non-blocking in v1 (`ci-config.md` §3.2). |

### 1.3 New gates for this feature's v1

These are **not new CI jobs** — they are Go test functions that run inside the existing G3/G4 gates, plus the new `-race` step in the `backend` job (G19). They are called out as distinct gates because they map to Bolt go/no-go decisions and to RAID-risk targeted tests (`quality-report` §3).

| ID | Gate | Test function(s) / command | Runs in | Blocks | Source |
|----|------|----------------------------|---------|--------|--------|
| G10 | Migration 018 round-trip (thinned) | `migration_018_test.go::TestMigration_RoundTrip`, `::TestMigration_AdditiveOnly`, `::TestMigration_ActorNullable`, `::TestMigration_NoReposTableAlteration`, `::TestMigration_Atomicity`, `::TestMigration_NumberIs018`, `::TestFeatureDefaults_ValidatorMinimal` | G3 (db subset), G4 | Bolt 0 go/no-go; release-candidate status | FR-MIG-01..04, R-TEST-03, `test-results` rev2 §2.1 T-MIG-01..07 |
| G11 | Config write path (linchpin) | `config_test.go::TestWriteConfig_RoundTrip`, `::TestWriteConfig_CrashInjection`, `::TestWriteConfig_ValidationRejects`, `::TestReconcile_RegeneratesStaleYAML`, `::TestBootstrapFields_YAMLOnly`, `::TestReconcile_BootstrapUntouched`, `::TestWriteConfig_EmitsAuditEvent`, `::TestWriteConfig_AuditFailureNoRevert`, `::TestValidator_EnvVarRefPattern`, `::TestValidationDetails_SecretMasked`, `::TestLoadConfig_Unchanged` | G3 (config subset — added in rev2), G4 | Bolt 0 go/no-go; release-candidate status | FR-CONFIG-01..08, R-CONFIG-MATERIALIZE, R-CONFIG-VALIDATION, `test-results` rev2 §2.2 T-CONFIG-01..11 |
| G12 | Auth guard (fail-closed) | `auth_guard_test.go::TestAdminGuard_LocalhostAllowed`, `::TestAdminGuard_NonLocalhost_TokenMatch`, `::TestAdminGuard_NonLocalhost_TokenMissingOrWrong_401`, `::TestAdminGuard_FailClosed_WhenEnvUnset`, `::TestAdminGuard_RejectsBeforeSideEffect`, `::TestAdminGuard_RejectionNoAudit`, `::TestAdminGuard_PluggableMiddleware`, `server_test.go::TestGetRepos_Unguarded` | G3 (api subset) | Bolt 0 go/no-go; release-candidate status; 4.1 security gate | FR-ROUTE-03, R-AUTH-ABSENT, `test-results` rev2 §2.4 T-AUTH-01..08 |
| G13 | Audit constants + actor overload | `audit_events_test.go::TestFourEventConstants`, `::TestRecordAuditEventWithActor_PopulatesActor`, `::TestRecordAuditEvent_BackwardCompat`, `::TestActor_EmptyIsNULL`, `::TestActor_DefaultOperator`, `::TestActor_FromEnvOnly`, `::TestDetails_NoSecret` | G3 (db subset) | Bolt 0 go/no-go; release-candidate status; scope-creep block (QO-11) | FR-AUDIT-04 (rev2), FR-AUDIT-ACTOR-01..03, BR-AUDIT-01, `test-results` rev2 §2.3 T-AUDIT-CONST-01..07 |
| G14 | Repos service concurrent-write | `service_test.go::TestService_ConcurrentWrites_NoLoss` | G3 (api subset), G4 | Bolt 1 go/no-go; release-candidate status | FR-REPOS-03, R-REPOS-LOCK, `test-results` rev2 §2.6 T-REPOS-02 |
| G15 | Defaults precedence (MVP-completing) | `store_test.go::TestPrecedence_ExplicitWins`, `::TestPrecedence_PerRepoOverGlobal`, `::TestPrecedence_GlobalOverScopeDerived`, `::TestPrecedence_ScopeDerivedFallback`, `::TestStore_EmitsFeatureDefaultsMutated`, `server_test.go::TestCreateFeature_UsesDefaults` | G3 (api subset), G4 | Bolt 2 go/no-go; **MVP sign-off**; release-candidate status | FR-DEF-02, R-DEFAULTS-SEMANTICS, `test-results` rev2 §2.9 T-DEF-01..06 |
| G16 | Server DSN write-only + classification | `store_test.go::TestClassification_Table`, `::TestPut_DSNWriteOnly`, `::TestPut_EmitsConfigUpdated`, `::TestRolesMap_NotExposed` | G3 (api subset), G4 | Bolt 3 go/no-go; release-candidate status | FR-SEC-07, FR-SRV-01..04, R-SERVER-DSN-EXPOSURE, R-SERVER-SELFMUTATION, `test-results` rev2 §2.12 T-SRV-01..04 |
| G17 | Audit query uses index | `audit_handlers_test.go::TestGetAudit_FilterByType`, `::TestGetAudit_Pagination`, `::TestGetAudit_UsesIndex` (EXPLAIN), `::TestGetAudit_UnguardedRead`, `::TestGetAudit_FilterByActorAndTime` | G3 (api subset), G4 | Bolt 4 go/no-go; release-candidate status | FR-AUDIT-01..02, R-AUDIT-VOLUME, `test-results` rev2 §2.14 T-AUDIT-API-01..05 |
| G18 | Scope-creep structural (no cut-scope artifacts) | `audit_events_test.go::TestFourEventConstants` (no `PROVIDER_CONFIG_MUTATED`/`CI_CONFIG_MUTATED`); reviewer grep for `llm_*`/`cicd_platforms`/`internal/opencode/`/`internal/settings/providers/`/`internal/settings/cicd/` → zero hits | G3 (db subset) + 4.1 reviewer grep | release-candidate status; scope-creep block | `bolt-plan` rev2 §6, BR-AUDIT-01, QO-11, `test-results` rev2 §2.18 |
| G19 | Race (concurrency packages) | `go test -race ./internal/repos/... ./internal/config/... ./internal/settings/... -count=1` | `backend` (new step — `ci-config.md` §5.3) | release-candidate status; R-REPOS-LOCK / R-CONFIG-MATERIALIZE enforcement | `quality-report` rev2 QO-14 |

> **Note on renumbering.** Revision 1 had G10–G12 as the MVP gates and G13–G18 as fast-follow/deferred, with G17 (opencode builder) and G18 (CI/CD vaporware) deferred to sibling features. Revision 2 renumbers to match the revised Bolt sequence (Bolt 2 = Defaults, not Providers) and the expanded test set in `test-results` revision 2. The old G17/G18 (builder snapshot / CI/CD vaporware) are **removed** — those risks are **resolved by scope cut**, not deferred; there is no builder or CI/CD config in this feature's v1 to gate. The new G18 is the scope-creep structural gate (verifies the cut artifacts stay absent). The new G19 is the `-race` gate (binding per QO-14).

### 1.4 Gated fast-follow (not in v1, recorded for when Bolt 5 activates)

| ID | Gate | Test function(s) | Bolt | Source |
|----|------|-------------------|------|--------|
| G20 | Providers integration (thin consumer) | `admin.spec.ts::Providers_*` (renders sibling's API data); structural grep (no parallel provider registry in this repo) | Bolt 5 (gated on sibling feature `multi-provider-llm-configuration` reaching 2.6) | `bolt-plan` rev2 §2 Bolt 5, `test-results` rev2 §2.18 |

G20 is **not scheduled in v1**. It activates only when the sibling feature's API contract is frozen. The test design is deferred until the gate activates — do not write these tests now. When activated, the gate verifies the integration is a thin consumer, not a duplicate of the sibling's registry.

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

## 3. Gate ↔ Bolt Mapping (v1)

The bolt-level go/no-go checkpoints map to CI gates as follows. This lets the builder read CI status as bolt status. **MVP = Bolts 0–2** (shell + repos + defaults, per `bolt-plan` revision 2). **v1 = Bolts 0–4** (shell + repos + defaults + server + audit). Bolt numbering follows `bolt-plan` revision 2.

| Bolt | Bolt gate (from `quality-report` rev2 §3 / `bolt-plan` rev2 §2) | CI gate(s) | MVP? | v1? |
|------|------------------------------------------------------------------|------------|------|-----|
| 0 — Walking Skeleton | Migration 018 round-trip green; config write path round-trip + crash-injection green; audit actor overload + 4 constants green; auth guard fail-closed green; POST /api/repos thin slice green; admin shell renders (4 v1 tabs + 2 placeholders) | G10, G11, G12, G13, G3 (api subset), G7 (shell build), G8 (shell E2E, non-blocking) | ✅ MVP | ✅ v1 |
| 1 — Repos | Repos service concurrent-write green; repos CRUD handlers green; `admin.spec.ts` repos cases green; `GET /api/repos` shape unchanged; `repos.yaml.lock` gitignored | G14, G3 (api subset), G8 (admin.spec, non-blocking), G4 (non-regression) | ✅ MVP | ✅ v1 |
| 2 — Defaults (MVP complete) | Defaults precedence 4-branch green; `createFeature` backward-compat green; `FEATURE_DEFAULTS_MUTATED` emitted; defaults handlers guarded; `DefaultsTab` E2E green | G15, G3 (api subset), G8 (admin.spec, non-blocking) | ✅ MVP | ✅ v1 |
| 3 — Server | DSN write-only green; restart classification green; `roles.*` not exposed; server handlers guarded; `ServerTab` E2E green | G16, G3 (api subset), G8 (admin.spec, non-blocking) | ❌ (v1 completeness) | ✅ v1 |
| 4 — Audit (v1 complete) | Audit query uses index (EXPLAIN) green; audit handlers green; `AuditTab` E2E green | G17, G3 (api subset), G8 (admin.spec, non-blocking) | ❌ (v1 completeness) | ✅ v1 |
| 5 — Providers Integration | (gated — not scheduled) | G20 | ❌ gated | ❌ gated |

**Scope-creep structural gate (G18)** runs in Bolt 0 and is re-verified at 4.1: no `PROVIDER_CONFIG_MUTATED`/`CI_CONFIG_MUTATED` constants, no `llm_*`/`cicd_platforms` tables, no `internal/opencode/`/`internal/settings/providers/`/`internal/settings/cicd/` packages. Enforced continuously across all Bolts.

**Race gate (G19)** runs in every CI run (it's a step in the `backend` job, not Bolt-gated): R-REPOS-LOCK and R-CONFIG-MATERIALIZE are enforced by `-race` on the concurrency-touching packages.

**MVP sign-off gate**: G9 (aggregate) green + G10 + G11 + G12 + G13 + G14 + G15 green (the MVP Bolt go/no-go gates) + G19 (race) green + 4.1 review gate verifies R-AUTH-ABSENT (G12), R-REPOS-LOCK (G14 + G19), R-CONFIG-MATERIALIZE (G11 + G19), and R-DEFAULTS-SEMANTICS (G15) mitigated. E2E (G8) is informational.

**v1 sign-off gate**: MVP sign-off + G16 + G17 green + 4.1 review gate verifies R-SERVER-DSN-EXPOSURE (G16), R-SERVER-SELFMUTATION (G16), R-AUDIT-VOLUME (G17) mitigated + G18 (scope-creep structural) verified at 4.1.

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
| R-TEST-05 (Playwright one spec per tab group) | Blocking | G8 — `admin.spec.ts` exists with 5 v1 describe groups + 2 placeholder cases; no per-tab files |
| R-TEST-06 (existing E2E specs still pass) | Blocking | G8 (non-blocking in v1, but regressions surface in the run) + G4 (Go-side non-regression via `TestLoadConfig_Unchanged` — T-CONFIG-04 — and existing `TestRecordAuditEvent` — T-AUDIT-CONST-03) |
| R-TEST-08 (no coverage gate) | (not a gate) | — inherited gap; `quality-report` §5.3 recommends a report upload, not a gate |

**Gates do not enforce everything.** Stylistic rules (no hardcoded hex, design-token usage) are reviewer-gate (4.1) enforced, not CI-enforced. CI enforces what is automatable: build, test, vet, type-check, E2E, dep discipline, race.

---

## 5. Gate ↔ RAID Risk Mapping (from `quality-report` rev2 §3)

Every v1 RAID risk with a mitigation test, mapped to its enforcing gate. Status: **Planned** (tests designed in `test-results` rev2, not yet executed — construction hasn't started on this feature branch).

| Risk | L | I | Score | Mitigation test(s) | Enforcing gate | MVP? | v1? |
|------|---|---|-------|---------------------|----------------|------|-----|
| R-AUTH-ABSENT | 3 | 5 | 15 | T-AUTH-03 (401, no side effect), T-AUTH-04 (fail-closed env unset), T-AUTH-05 (rejects before side effect) | G12 | ✅ MVP (Bolt 0) | ✅ v1 |
| R-REPOS-LOCK | 3 | 4 | 12 | T-REPOS-02 (concurrent, `-race`) | G14 + G19 (race) | ✅ MVP (Bolt 1) | ✅ v1 |
| R-CONFIG-MATERIALIZE | 2 | 5 | 10 | T-CONFIG-02 (crash-injection), T-CONFIG-05 (reconcile), T-CONFIG-07 (bootstrap untouched) | G11 + G19 (race) | ✅ MVP (Bolt 0) | ✅ v1 |
| R-CONFIG-VALIDATION | 3 | 4 | 12 | T-CONFIG-03, T-CONFIG-10 (env-var-ref), T-CONFIG-11 (masking) | G11 | ✅ MVP (Bolt 0) | ✅ v1 |
| R-DEFAULTS-SEMANTICS | 3 | 3 | 9 | T-DEF-01..04 (table-driven, 4 branches) | G15 | ✅ MVP (Bolt 2) | ✅ v1 |
| R-SERVER-SELFMUTATION | 3 | 4 | 12 | T-UI-SRV-01 (badges), T-UI-SRV-03 (banner), T-SRV-01 (classification) | G16 | ❌ (v1 completeness, Bolt 3) | ✅ v1 |
| R-SERVER-DSN-EXPOSURE | 2 | 5 | 10 | T-SRV-02 (BE), T-API-SET-04 (API), T-UI-SRV-02 (FE + network) | G16 | ❌ (v1 completeness, Bolt 3) | ✅ v1 |
| R-AUDIT-VOLUME | 3 | 3 | 9 | T-AUDIT-API-03 (EXPLAIN uses index) | G17 | ❌ (v1 completeness, Bolt 4) | ✅ v1 |
| ~~R-PROVIDER-KEYS~~ | 2 | 5 | 10 | — | — | **Resolved by scope cut** — no provider config in this feature (Q7). Sibling feature `multi-provider-llm-configuration` owns provider-key handling; its 4.1 gate verifies it. |
| ~~R-PROVIDER-COPILOT~~ | 3 | 3 | 9 | — | — | **Resolved by scope cut.** Sibling feature owns it. |
| ~~R-CICD-VAPORWARE~~ | 2 | 2 | 4 | — | — | **Resolved by scope cut** — no CI/CD config in this feature (Q8). Separate `ci-cd-platform-config` feature owns it. |

**Risk burn-down plan** (per `quality-report` rev2 §3): all 8 v1 risks are **Planned** at construction start. Each Bolt burns down the risks for its units:
- Bolt 0 (walking skeleton): R-CONFIG-MATERIALIZE, R-CONFIG-VALIDATION, R-AUTH-ABSENT
- Bolt 1 (repos): R-REPOS-LOCK
- Bolt 2 (defaults — MVP complete): R-DEFAULTS-SEMANTICS
- Bolt 3 (server): R-SERVER-SELFMUTATION, R-SERVER-DSN-EXPOSURE
- Bolt 4 (audit — v1 complete): R-AUDIT-VOLUME

**MVP residual risk**: R-SERVER-DSN-EXPOSURE, R-SERVER-SELFMUTATION, R-AUDIT-VOLUME remain open at MVP sign-off — their gates (G16, G17) are v1-completeness (Bolts 3–4), not MVP. MVP sign-off requires R-AUTH-ABSENT (G12), R-REPOS-LOCK (G14 + G19), R-CONFIG-MATERIALIZE (G11 + G19), R-CONFIG-VALIDATION (G11), and R-DEFAULTS-SEMANTICS (G15) mitigated. This is acceptable: MVP is shippable to the single trusted operator; the remaining risks are v1-completeness risks, not MVP blockers. **There are no provider/cicd residual risks because those scopes are cut** — resolved, not deferred.

---

## 6. Promotion Flow (input to 4.1 Deployment Pipeline)

Inherited from the sibling feature's `quality-gates.md`, unchanged in mechanics:

```
commit on feature/settings-and-admin-ui
        │
        ▼
   ci.yml runs (G1–G8 in parallel jobs; G10–G17, G19 run inside G3/G4/backend)
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

Per `quality-report` rev2 §5 and the sibling feature's `quality-gates.md`:

- **No coverage threshold gate in v1.** `quality-report` §5.3 recommends a `go test -cover` **report upload** (not a gate) so the coverage trend is visible. This feature does not add one. A hard threshold is deferred until the baseline is measured.
- **The requirements-coverage matrix** (`test-results` rev2 §4) is the binding surrogate: every Must FR and every RAID risk maps to at least one named test. This is stronger than a line-coverage number (quality principle 5).
- **`-race` is binding (G19).** `quality-report` rev2 §1.4 / QO-14 elevates `-race` from "recommended" (revision 1) to **binding** on the concurrency-touching packages (`internal/repos`, `internal/config`, `internal/settings`). This is encoded as a CI step in the `backend` job (`ci-config.md` §5.3), not merely a developer-local discipline. R-REPOS-LOCK and R-CONFIG-MATERIALIZE are enforced by `-race` in addition to their functional tests.

---

## 8. Self-Verification

1. **Every inherited gate (G1–G9) maps to a `ci.yml` job** — verified against the existing `ci.yml` on `main` (commits `bdbb7da` + `f6cbc1f` + `a7b859b`) and the feature-branch edits (`ci-config.md` §5). ✓
2. **Every new gate (G10–G19) maps to a Go test function or CI step** in `test-results` rev2 §2 or `ci-config.md` §5.3 — no invented gates. G10 = T-MIG-01..07; G11 = T-CONFIG-01..11; G12 = T-AUTH-01..08; G13 = T-AUDIT-CONST-01..07; G14 = T-REPOS-02; G15 = T-DEF-01..06; G16 = T-SRV-01..04; G17 = T-AUDIT-API-01..05; G18 = T-AUDIT-CONST-01 + reviewer grep; G19 = `go test -race` step. ✓
3. **No `continue-on-error` on any blocking job** — checked: only `e2e` (G8) has `continue-on-error: true`, and it is excluded from the `gate` blocking set. ✓
4. **Bolt mapping is consistent** with `bolt-plan` rev2 §2 and `quality-report` rev2 §3. MVP = Bolts 0–2; their gates (G10–G15) are the MVP sign-off set. v1 = Bolts 0–4; their gates (G10–G17) are the v1 sign-off set. ✓
5. **Rule mapping covers the binding `discovered-rules`** — R-BUILD-*, R-TEST-01..06, R-TEST-08 all have an enforcing gate or an explicit "not a gate" note. ✓
6. **Risk mapping covers the 8 v1 RAID risks** — every v1 risk has an enforcing gate, marked MVP / v1-completeness per the 1.7-gate strict scope. The 3 provider/cicd risks are **resolved by scope cut**, not deferred — no gate is defined for them because there is no code to gate. ✓
7. **Rollback trigger is concrete** — G8 failure post-deploy → `rollback.sh` (git revert + rebuild + restart); migration 018 forward-only; reconciler handles config-state recovery. ✓
8. **Scope discipline** — no coverage gate, no security scan gate, no performance gate, no cross-browser gate, no CI/CD gate, no opencode builder gate. Each omission is recorded with its source rationale (`ci-config.md` §7, `quality-report` rev2 §5). ✓
9. **v1 scope is binding** — G20 (Providers integration) is explicitly gated on the sibling feature reaching 2.6; it is recorded for traceability but not enforced in v1. The CI/CD and opencode builder gates are removed entirely (resolved by scope cut, not deferred). ✓
10. **Baseline shift is recorded** — §0 flags the 2.2/3.6 "no CI" staleness so 4.1/4.3 and the reviewer gate operate against the current baseline. ✓
11. **Race gate is binding** — G19 encodes `quality-report` rev2 QO-14 as a CI step in the `backend` job, not merely a developer-local recommendation (revision 1 had it as "recommended"; revision 2 corrects this). ✓
12. **Linchpin fail-fast** — G3 (revision 2) includes `internal/config` so U-CONFIG-01 failures surface in the fast `backend` gate, not just `backend-full`. ✓
13. **Audit-constants structural gate** — G13 (revision 2) elevates T-AUDIT-CONST-01 (exactly 4 event constants, no `PROVIDER_*`/`CI_*`) to a named gate; this is the scope-creep canary at the Go layer, complementing the 4.1 reviewer grep (G18). ✓

*End of quality-gates artifact (revision 2).*