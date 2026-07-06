# Quality Gates — full-crud-and-ui-for-managing-repositories

**Feature ID**: full-crud-and-ui-for-managing-repositories
**Stage**: 3.7 — CI Pipeline
**Phase**: construction
**Author**: pipeline-deploy
**Scope**: feature
**Depth**: minimal
**Created**: 2026-07-06

---

## Purpose

Define the quality gates that block promotion of a commit to release-candidate status. These gates are encoded in `.github/workflows/ci.yml` (the `ci-config` artifact) and run on every push and PR. A commit is a release candidate **only if** every gate below is green (principle #1, team-practices P1).

This artifact is the **gate specification**; `ci.yml` is the **implementation**. Where they differ, `ci.yml` is wrong and must be fixed.

## Gate Catalog

Each gate: **ID | Name | Command | Job in ci.yml | Blocks | Source**.

`Blocks` = what is prevented when the gate fails.

| ID | Gate | Command | ci.yml job | Blocks | Source |
|----|------|---------|------------|--------|--------|
| G1 | Go build | `go build ./...` | `backend`, `backend-full` | release-candidate status | P7, R1 |
| G2 | Go vet | `go vet ./...` | `backend` | release-candidate status | P7 |
| G3 | Go unit (db+api) | `go test ./internal/db/... ./internal/api/... -count=1` | `backend` | release-candidate status; Bolt 1/2 go/no-go | P7, FR12, R1 |
| G4 | Go full suite | `go test ./... -count=1` | `backend-full` | release-candidate status (release gate) | P7 |
| G5 | Dep-tidy (no new deps) | `go mod tidy && git diff --exit-code go.mod go.sum` | `backend` | release-candidate status | R4, C4 |
| G6 | Frontend lint | `npm run lint` | `frontend` | release-candidate status | P7 |
| G7 | Frontend build (strict TS) | `npm run build` (`tsc -b && vite build`) | `frontend` | release-candidate status; Bolt 3 gate | P7, R10 |
| G8 | E2E smoke + non-regression | `npm run test:e2e` | `e2e` | release-candidate status; Bolt 5 gate; deploy smoke (P10) | P7, FR13, R3 |
| G9 | Aggregate promotion gate | (meta — all required jobs green) | `gate` | promotion to release-candidate / merge to main | role brief "promotion gates" |

## Gate Behavior

### Pass condition
The `gate` job succeeds **only if** `backend`, `frontend`, `backend-full`, and `e2e` all return `result == 'success'`. It runs `if: always()` so it executes even when a dependency fails — and fails itself in that case.

### Fail behavior
- Any required job failing → `gate` job fails → the commit is **not** a release candidate.
- On `pull_request`: GitHub branch protection (to be configured by 4.1) treats a failed `gate` check as blocking merge.
- On `push` to `feature/**`: the failed run is the feedback signal to the builder; no auto-merge occurs (the pipeline pushes post-reviewer-gate, not on CI pass).

### No bypass
Per principle #4 ("Bypassing a gate is an incident, not a shortcut"), there is no `continue-on-error` on any required job. The dep-tidy gate (G5) in particular is hard-failing — a new dependency is a Blocking violation of R4 and must be resolved (reverted or explicitly waived with rationale), not skipped.

## Gate ↔ Bolt Mapping (from bolt-plan / quality-report §2)

The bolt-level go/no-go checkpoints map to CI gates as follows. This lets the builder read CI status as bolt status.

| Bolt | Bolt gate (from quality-report) | CI gate(s) |
|------|----------------------------------|------------|
| 1 `db-foundation` | `go test ./internal/db/...` green; `repo_store_test.go` covers CRUD+dup+count+not-found | G3 (db subset) |
| 2 `api-contract` | `go test ./internal/api/...` green; 5 handlers × error paths; `aidlc.spec.ts` green | G3 (api subset) + G8 (aidlc.spec) |
| 3 `client-hooks` | `npm run build` clean; hooks exported + typed | G7 |
| 4 `ui-surface` | `npm run build` clean; `/repos` reachable; existing E2E green | G7 + G8 |
| 5 `e2e-verification` | `repos.spec.ts` green; existing E2E green | G8 |

**Note on the current feature state**: the 3.6 quality-report recorded all bolts as NOT MET because stage 3.5 did not execute (no feature code exists yet). The gates above are defined for when construction lands; they will fail today (no `repos.spec.ts`, no handlers) and that is correct — a gate that passes on unbuilt code is a broken gate.

## Gate ↔ Review-Rule Mapping (from discovered-rules R1–R10)

| Rule | Severity | Enforced by gate |
|------|----------|------------------|
| R1 (store + migration convention) | Blocking | G1, G3 — code that doesn't follow the convention won't compile or pass db tests |
| R2 (handler style + routing) | Blocking | G3 — handler tests assert route + error body shape |
| R3 (additive-only response) | Blocking | G8 — `aidlc.spec.ts` non-regression asserts the existing shape |
| R4 (no new deps) | Blocking | G5 (Go) + `npm ci` strict (frontend) |
| R5 (toolchain pins) | Blocking | ci.yml setup steps pin Go 1.26.1 / Node 22.23 |
| R6 (server-authoritative validation) | Blocking | G3 — handler tests assert each rejection path |
| R7 (delete-guard server-side) | Blocking | G3 — handler test asserts 409 + feature list on referenced delete |
| R8 (react-query invalidation) | Required | G8 — E2E confirms list refreshes after mutation |
| R9 (existing design tokens) | Required | G6 (lint) + G7 — not directly enforced by CI; reviewer gate enforces |
| R10 (TS strict-mode clean) | Blocking | G7 — `tsc -b` fails on strict violations |

**Gates do not enforce everything.** R9 (token usage) and the "no inline hex" rule are stylistic and not machine-checkable without a custom linter; the reviewer gate (3.x review stage) enforces those. CI enforces what is automatable: build, test, lint, type-check, E2E, dep discipline.

## Promotion Flow (input to 4.1 Deployment Pipeline)

```
commit on feature/**
        │
        ▼
   ci.yml runs (G1–G8 in parallel jobs)
        │
        ▼
   gate job (G9) aggregates
        │
   ┌────┴────┐
   ▼         ▼
 pass      fail
   │         │
   ▼         ▼
release-   not a release
candidate  candidate — fix & re-push
   │
   ▼ (post-reviewer-gate, post-4.x)
merge to main → deploy (4.3 re-runs G8 as smoke)
```

4.1 Deployment Pipeline consumes `G9` (the `gate` job status) as its promotion input. 4.3 Deployment Execution re-runs **G8 (E2E)** post-deploy as the smoke test (P10: "deployment is not done until smoke passes").

## Rollback Trigger (input to 4.1/4.3)

Per principle #2 ("Rollback is not optional") and team-practices P9, the rollback trigger for this feature is:

- **Smoke gate (G8) fails post-deploy** → rollback. The rollback procedure is git-based (P9): `git revert` the merge commit on `main`, rebuild, restart the systemd unit. The `repos` migration is forward-only; rollback = drop the `repos` table (re-seeded from `repos.yaml` on next boot via IB-3). The delete-guard (R7) ensures a revert cannot drop a repo referenced by an in-flight feature — rollback is safe by construction.

This is recorded here so 4.1 has the trigger condition and the safe-rollback property without re-deriving them.

## Self-Verification

1. **Every gate maps to a P7 command** — no invented gates. G1–G8 commands are verbatim from the team-practices verification table. ✓
2. **Every gate maps to a ci.yml job** — the implementation file implements each gate; the `gate` job (G9) aggregates. ✓
3. **No `continue-on-error`** — checked: no required job can bypass a failure. ✓
4. **Bolt mapping is consistent** with quality-report §2 (the authoritative bolt list). ✓
5. **Rule mapping covers R1–R10** — every Blocking rule has an enforcing gate; Required rules that are not machine-checkable (R9) are explicitly handed to the reviewer gate. ✓
6. **Rollback trigger is concrete** — G8 failure post-deploy, with the git-revert + drop-table procedure from P9. Not a generic "rollback on error." ✓
7. **Scope discipline** — no coverage gate, no security scan gate, no performance gate. Each is consistent with the minimal depth and the "Practices NOT adopted" list. ✓