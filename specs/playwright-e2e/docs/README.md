# Documentation — playwright-e2e

**Spec**: playwright-e2e — *Health Check Endpoint (GET /api/health)*
**Phase**: Delivery (Ops)

This directory contains the release documentation for the `playwright-e2e`
feature. All terminology matches `specs/playwright-e2e/spec.md`.

## Contents

| File | Audience | Contents |
|---|---|---|
| [`api.md`](./api.md) | API consumers, operators | Endpoint reference for `GET /api/health`: method, path, request/response schemas, error responses (200/405/404/500), examples, constraint trace. |
| [`user-guide.md`](./user-guide.md) | Operators, developers | User-facing guide for every user story in the spec (US-001 Health Check Probe, US-002 Method Restriction, US-003 End-to-End Playwright Coverage), with workflows and error messages. |
| [`CHANGELOG.md`](./CHANGELOG.md) | Release reviewers | Changelog for `v1.0` — every entry references spec `playwright-e2e`. |
| [`release.md`](./release.md) | Release engineers | Cross-repo release order (single-repo feature), deployment verification summary, `.devteam/` pointer note. |
| [`configuration.md`](./configuration.md) | Operators | Configuration documentation: `devteam.yaml` `version` field, env vars (none new), dependencies, ports, DB, auth, caching. |

## Quality gate coverage

| Gate criterion | Where |
|---|---|
| Documentation exists for every user story | `user-guide.md` — US-001, US-002, US-003 each have a section. |
| Documentation uses spec terminology | All files use `HealthStatus`, `Config.Version`, `recoveryMiddleware`, `Health Check Probe`, `Method Restriction`, `End-to-End Playwright Coverage` — terms from `spec.md`. No code-internal names. |
| Cross-repo release order documented | `release.md` — single-repo feature, order documented. |
| Release notes reference the spec number | `CHANGELOG.md` — every entry cites `spec playwright-e2e`. |
| Configuration documented | `configuration.md` — `devteam.yaml` `version`, env vars, deps, ports, DB, auth, caching. |

## Out of scope for this phase

This phase produces documentation only. It does not build, test, start the
service, hit endpoints, or commit code. Proof of work for the running
system is in `specs/playwright-e2e/test-report.md` (Testing phase) and
`specs/playwright-e2e/review-report.md` (Review phase).