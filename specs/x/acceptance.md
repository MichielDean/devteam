# Acceptance Criteria — Feature x

Every criterion follows Given/When/Then with a test level and verification method.

## US-001 — Pipeline operator runs feature x through all phases

AC-001: Given feature x is intaked and `.devteam-state.yaml` shows `inception: in_progress`, when `devteam run x` completes inception, then `specs/x/spec.md`, `specs/x/acceptance.md`, `specs/x/repos.yaml`, and `specs/x/outcome.txt` (first line `pass`) all exist.
  Test level: smoke
  Verification: `test -f specs/x/spec.md && test -f specs/x/acceptance.md && test -f specs/x/repos.yaml && head -n1 specs/x/outcome.txt | grep -qx pass`

AC-002: Given inception is complete, when planning runs, then `specs/x/plan.md` and `specs/x/tasks.md` exist and are non-empty.
  Test level: smoke
  Verification: `[ -s specs/x/plan.md ] && [ -s specs/x/tasks.md ]`

AC-003: Given planning is complete, when construction runs, then the project's build command (per AGENTS.md: `PATH="$PATH:/usr/local/go/bin" go build ./cmd/devteam/`) exits 0.
  Test level: integration
  Verification: `PATH="$PATH:/usr/local/go/bin" go build ./cmd/devteam/ && echo BUILD_OK`

AC-004: Given construction is complete, when review runs, then a review report exists under `specs/x/` with no unresolved critical findings recorded.
  Test level: smoke
  Verification: review report file exists; `grep -c "critical" <report>` for unresolved critical findings == 0.

AC-005: Given review is complete, when testing runs, then a test report exists under `specs/x/` and all critical tests pass per the report.
  Test level: integration
  Verification: `PATH="$PATH:/usr/local/go/bin" go test ./... -count=1 -timeout 120s` exits 0; test report references the run.

AC-006: Given testing is complete, when delivery runs, then delivery docs exist under `specs/x/` and `.devteam-state.yaml` shows feature x `status: complete`.
  Test level: smoke
  Verification: delivery docs file present; `grep -E '^\s*status:\s*complete' specs/x/.devteam-state.yaml` matches the feature-level status.

AC-007: Given the full run, when inspecting `.devteam-state.yaml`, then phases appear in order inception → planning → construction → review → testing → delivery, each `status: complete`, with no skipped or backward transitions.
  Test level: unit
  Verification: parse `.devteam-state.yaml`, assert each phase's `status` is `complete` and ordering matches the Dev Team phase map.

## US-002 — Operator inspects feature x artifacts

AC-008: Given feature x has completed all phases, when the operator runs `ls specs/x/`, then the full artifact set is present: spec.md, acceptance.md, repos.yaml, plan.md, tasks.md, review report, test report, delivery docs.
  Test level: smoke
  Verification: enumerate expected files and assert each exists.

AC-009: Given any artifact file under `specs/x/`, when the operator reads it, then the file contains non-placeholder content (no unchanged template placeholders like `[FEATURE NAME]` or `[Brief Title]`).
  Test level: unit
  Verification: `grep -R "\[FEATURE NAME\]\|\[Brief Title\]\|\[Describe this" specs/x/` returns no matches.

## US-003 — Operator retries a failed phase

AC-010: Given a phase gate for feature x has failed (e.g., `outcome.txt` missing or `pool`), when the operator fixes the blocker and re-runs `devteam run x`, then only the failed phase and subsequent phases re-execute and the feature reaches `complete`.
  Test level: integration
  Verification: force `outcome.txt=pool`, re-run, assert `.devteam-state.yaml` eventually reaches `status: complete` and earlier-complete phases are not re-marked `in_progress`.

## Edge cases

AC-011: Given no human answers `specs/x/questions.json` within the interaction timeout, when inception runs in autonomous mode, then the spec is written with `[ASSUMPTION: ...]` markers and `outcome.txt` = `pass`.
  Test level: smoke
  Verification: `grep -c "\[ASSUMPTION:" specs/x/spec.md` >= 1; `head -n1 specs/x/outcome.txt | grep -qx pass`.

AC-012: Given `specs/x/` contains only `input.md` and `.devteam-state.yaml` at intake, when inception runs, then the required artifacts are created (not an error).
  Test level: smoke
  Verification: run inception from a clean `specs/x/` and assert AC-001.

AC-013: Given inception has already produced artifacts, when inception is re-run, then artifacts are overwritten idempotently (not duplicated/corrupted).
  Test level: unit
  Verification: run inception twice; assert file set unchanged and each file still parses.

AC-014: Given `.devteam-state.yaml` marks a phase `complete` but its artifact is missing, when the next phase runs, then the gap is documented in that phase's report and the pipeline proceeds with best-available information (per error-recovery extension).
  Test level: integration
  Verification: delete `plan.md`, run testing phase, assert test report documents the missing plan.md.

## Constraint coverage

No external constraints (no RFC/standard applies). Internal conventions from `AGENTS.md` are covered as assumptions in `spec.md`. Every functional requirement (FR-001..FR-008) maps to at least one AC: FR-001→AC-001, FR-002→AC-001, FR-003→AC-002/AC-004/AC-005/AC-006, FR-004→AC-007, FR-005→AC-008, FR-006→AC-009, FR-007→AC-010, FR-008→AC-011.