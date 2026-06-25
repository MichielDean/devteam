# Better Q&A UI — Delivery Documentation

Spec: `better-qa-ui` | Phase: delivery | Role: ops

This directory contains the delivery-phase documentation for the better-qa-ui feature. The feature is UI-only (CON-014): it replaces the Dev Team pipeline's flat Q&A form with a guided wizard. No backend endpoints, DB schema, or `Question` TypeScript interface fields were changed.

## Contents

| Document | Covers |
|---|---|
| [`api.md`](api.md) | API documentation for every endpoint the wizard touches (`GET /api/features/{id}/questions`, `PATCH /api/features/{id}/questions/{questionId}`), with spec terminology and error responses. |
| [`user-guide.md`](user-guide.md) | User-facing documentation for every user story (US-001..US-005), using spec terminology, with common workflows and error/empty-state behavior. |
| [`changelog.md`](changelog.md) | Changelog with every entry referencing the spec `better-qa-ui` and the relevant constraint/user-story. |
| [`release-and-config.md`](release-and-config.md) | Cross-repo release order (single repo — no ordering needed), breaking changes, migration, environment variables, config files, and dependencies. |

## Terminology

All docs use the terminology defined in `specs/better-qa-ui/spec.md`: `Feature`, `Question`, `phase` (`inception`/`planning`), `role` (`pm`/`architect`), `options`, `answer`, `assumption`, `status` (`pending`/`answered`/`assumed`), `waiting_for_human`, `autopilot`, `single-phase`. No code-internal names are introduced.

## Quality gate

- [x] Documentation exists for every user story (US-001..US-005) — `user-guide.md`.
- [x] Documentation uses spec terminology (not code-internal names) — terminology check above.
- [x] API documentation matches the contracts in `plan.md` / `contracts/` — `api.md`.
- [x] Changelog entries reference the spec `better-qa-ui` — `changelog.md`.
- [x] Cross-repo release order documented (single repo, no ordering) — `release-and-config.md`.
- [x] Configuration documented (env vars: none new; config files: none new; dependencies: none new) — `release-and-config.md`.