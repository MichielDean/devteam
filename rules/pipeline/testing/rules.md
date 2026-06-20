# Testing Phase Rules

## Purpose

Verify that what was built actually works in a running system. Not just that code compiles or unit tests pass.

Your defining question: **"Is this test real enough?"**

## Testing Levels — Mandatory

### Level 1: Smoke Tests (ALWAYS REQUIRED)
Start the service. Hit every endpoint. Verify no panics, no crashes, no nil pointers.

### Level 2: Integration Tests (REQUIRED FOR API CHANGES)
Full request/response cycles through real HTTP endpoints with real middleware.

### Level 3: E2E Tests (REQUIRED FOR UI CHANGES)
Load the web UI in a browser. Click through workflows. Verify no console errors.

### Level 4: Unit Tests (AS APPROPRIATE)
Business logic in isolation. State machine transitions. Serialization.

## Test Selection Matrix

| What changed | Smoke | Integration | E2E | Unit |
|---|---|---|---|---|
| HTTP API handlers | **YES** | **YES** | — | YES |
| Frontend/UI components | **YES** | **YES** | **YES** | YES |
| State machine logic | YES | — | — | **YES** |
| Gate evaluator | YES | — | — | **YES** |
| CLI commands | **YES** | — | — | YES |
| Middleware/auth | **YES** | **YES** | — | YES |

## Agent Failure Mode Checklist

When testing agent-generated code, specifically verify:

1. **Nil pointer chains**: Start the service, hit every endpoint, verify no panics
2. **Null arrays**: Verify every collection field returns [] not null when empty
3. **Phantom methods**: Verify the code compiles AND runs (methods exist, types match)
4. **Over-engineering**: Check line counts. If the API server is 3x the test suite, something's wrong
5. **Missing error paths**: Test 404, 400, 409, empty state, malformed input

## Spec-Implementation Drift Verification

Before writing tests, read the spec (spec.md, acceptance.md) and compare against what was built:

1. Did the spec ask for UI interactions? → Are there E2E tests?
2. Did the spec ask for error handling? → Are there tests for error paths?
3. Did the spec ask for real-time updates? → Are there SSE/WebSocket tests?
4. Frontend-backend contract: Does the frontend handle all error responses the backend can produce?

## Proof of Work

Name specific files, methods, and assertions. "Tests pass" is not evidence.

1. What smoke tests you ran — "I started the server on :8765 and hit every endpoint"
2. What integration test scenarios you covered — "I created a feature, retrieved it, verified all 6 phase states"
3. What E2E scenarios you covered — "I loaded the UI in Playwright, verified no console errors"
4. What null/empty checks you verified — "I verified artifacts, checks, dependencies, repos all return [] not null"
5. What state machine transitions you verified — "I tested start, advance, recirculate, cancel"

## Anti-Fake-Report

An agent can write "all 56 tests pass" in a markdown file without running any tests. Your test report MUST include:
- Exact commands to reproduce each test
- Exact assertions verified
- Exact endpoints hit during smoke testing
- Console output or screenshots from E2E tests

A test report that says "all tests pass" without reproducible commands is not a test report — it's a claim.

## Quality Gate

Testing is complete when:
1. Smoke tests pass: service starts, every endpoint returns expected status codes
2. Integration tests pass: full HTTP cycles work, JSON shapes match contract ([] not null)
3. E2E tests pass (if UI changed): frontend loads, renders data, no console errors
4. State machine verified: all valid transitions work, invalid transitions rejected
5. Spec drift checked: every user story in spec has a corresponding test
6. Every acceptance criterion has at least one test
7. No nil pointer panics, no null-vs-empty-array mismatches, no untested error paths

## Findings Have No Severity Tiers

Every finding is either "needs fixing" (recirculate) or "doesn't need fixing" (don't mention it).

**ANY failing test is an automatic recirculate.** A codebase with red tests is broken, period.
**ANY nil pointer panic is an automatic recirculate.** If the server crashes, it's not ready.
**ANY null-vs-empty-array mismatch is a finding.** Arrays in JSON must be [], not null.