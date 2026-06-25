# Release Order & Configuration — Better Q&A UI (Spec: better-qa-ui)

## Cross-Repo Release Order

**Single repo.** `repos.yaml` declares one repo:

1. `devteam` (path `.`, role `primary`) — UI-only change.

There are no shared libraries, no separate API repo, and no separate frontend repo. The Go backend (`internal/`) and React/TypeScript frontend (`ui/`) live in the same repository. The backend answer endpoint and `Question` DB schema are unchanged (CON-009, CON-014), so there is no producer/consumer ordering to coordinate.

**Release order**: a single release of the `devteam` repo. No cross-repo tagging or coordinated release is required.

### Affected files (UI-only)
- `ui/src/components/QuestionCard.tsx` — pending step (selectable option cards / textarea), answered/assumed restyle.
- `ui/src/pages/FeatureDetail.tsx` — wizard orchestration: `WizardAnswerDraft`, progress indicator, auto-scroll refs, inline summary, single submit button, submit orchestration.
- `ui/e2e/questions.spec.ts` — new e2e + integration suite.

**Not modified** (frozen by CON-014 / AC-CON-003):
- `ui/src/types/index.ts` — `Question` interface shape unchanged.
- `internal/api/server.go` — backend answer endpoint unchanged.

**Breaking changes**: none. UI-only; no API contract, schema, or type changes.

**Migration required**: no. No DB migration, no env var changes, no dependency additions.

---

## Configuration

### Environment variables
No new environment variables. This is a UI-only feature; the backend configuration is unchanged.

The Dev Team server reads its existing configuration (e.g. `-http` flag for the listen address; the e2e stack uses `:18765`). No new flags were added.

### Configuration files
No new configuration files. Existing config files are unchanged.

### Dependencies
**No new dependencies** (spec assumption). The wizard is built entirely on the existing stack:

- **Frontend**: React, React Router v7, React Query (`@tanstack/react-query`), Tailwind v4, Vite. Dark-mode variants use the existing Tailwind `dark:` setup.
- **Testing**: Playwright e2e on port 18765 (`ui/playwright.config.ts`, `reuseExistingServer`). Integration tests use Playwright's API request context against the running server. No unit-test runner is configured (unit-level criteria covered by seeded e2e + a `git diff` check on `ui/src/types/index.ts`).
- **Backend**: Go (unchanged). SQLite storage (unchanged).

### Run / verify (reference only — already executed by Construction and Testing phases)
```bash
# Backend (unchanged)
go build -o ~/go/bin/devteam ./cmd/devteam
~/go/bin/devteam -http :18765 &

# Frontend dev (Vite proxies /api to :18765)
cd ui && npm install && npm run dev   # http://localhost:5173

# E2E (starts/reuses server on :18765)
cd ui && npm run test:e2e

# Frozen-interface check (CON-014 / AC-CON-003)
git diff ui/src/types/index.ts   # must show no Question-field changes
```