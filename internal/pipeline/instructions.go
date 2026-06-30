package pipeline

import (
	"fmt"
	"strings"

	"github.com/MichielDean/devteam/internal/feature"
)

// phaseInstruction returns phase-specific instructions for the agent.
// This is the large text block telling the agent what to do for this phase.
func (p *Pipeline) phaseInstruction(phase feature.Phase, f *feature.Feature) string {
	featureID := f.ID
	prefix := fmt.Sprintf(`## IMPORTANT: Submit Artifacts via CLI

Spec artifacts (spec.md, plan.md, tasks.md, etc.) are stored in the database, NOT on disk.
Submit them using the devteam CLI:

  devteam artifact submit %s <type> --file <filename>
  devteam artifact submit %s <type> --content "inline content"

Artifact types: spec, acceptance, repos, plan, tasks, research, data_model, review_report, test_report, docs, contracts

Do NOT write spec artifacts to disk. Use the CLI.
`, featureID, featureID)

	switch phase {
	case feature.PhaseInception:
		return prefix + fmt.Sprintf(`You are in the INCEPTION phase for feature %s.

Your task: Gather requirements through interactive questions, then generate the spec using SpecKit.

## Step 1: Ask Clarifying Questions (AIDLC pattern)

If this is a loose idea (not an external spec), write a questions.json file with 3-8 clarifying questions:
[
  {"phase":"inception","role":"pm","question":"Your question here","type":"multiple_choice","options":["Option A","Option B","Other"]},
]
Every question MUST include "Other" as the last option.

Then submit the questions using the devteam CLI:
  devteam questions ask %s --file questions.json

The pipeline will pause and show these questions to the user. Their answers will be provided to you on the next run.
If you can resolve something by reading existing code, do that instead of asking.

After submitting questions, signal that you need feedback:
  devteam signal %s needs_feedback

When you receive answers, check if you need MORE questions. If so, repeat. If you have enough clarity, proceed to Step 2.

## Step 2: Generate the Spec

When you have enough clarity, use the SpecKit spec template at .specify/templates/spec-template.md to write:
- spec.md — user stories with priorities, acceptance scenarios, functional requirements, success criteria, assumptions
- acceptance.md — acceptance criteria in Given/When/Then format with test levels
- repos.yaml — affected repositories

Submit each artifact via CLI:
  devteam artifact submit %s spec --file spec.md
  devteam artifact submit %s acceptance --file acceptance.md
  devteam artifact submit %s repos --file repos.yaml

If a constitution.md exists, verify compliance.

When the spec is complete, signal pass:
  devteam signal %s pass

Inception should almost never fail — it's just a question-answer loop that ends with a spec.`, featureID, featureID, featureID, featureID, featureID, featureID, featureID)

	case feature.PhasePlanning:
		return prefix + fmt.Sprintf(`You are in the PLANNING phase for feature %s.

Your task: Generate the implementation plan and task list using SpecKit templates.

## Step 1: Ask Clarifying Questions (optional)

If the spec leaves architectural decisions open, write a questions.json file:
[
  {"phase":"planning","role":"architect","question":"...","type":"multiple_choice","options":["A","B","Other"]},
]
Submit via: devteam questions ask %s --file questions.json
Signal: devteam signal %s needs_feedback
If the spec is clear, skip this step.

## Step 2: Generate the Plan

Use the SpecKit plan template at .specify/templates/plan-template.md to write:
- plan.md — technical context, project structure, component design, API contracts, test strategy
- research.md — existing code patterns, library choices, alternatives considered
- data-model.md — entity definitions, attributes, relationships, validation
- contracts/ — one file per API endpoint with request/response schemas

Submit each artifact via CLI:
  devteam artifact submit %s plan --file plan.md
  devteam artifact submit %s research --file research.md
  devteam artifact submit %s data_model --file data-model.md
  devteam artifact submit %s contracts --file contracts/index.md

If a constitution.md exists, perform a constitution check.

## Step 3: Generate the Task List

Use the SpecKit tasks template at .specify/templates/tasks-template.md to write:
- tasks.md — tasks grouped by user story priority, each with file paths, done conditions, dependencies, test levels

Submit via CLI:
  devteam artifact submit %s tasks --file tasks.md

The plan MUST address all acceptance criteria from acceptance.md. Every task must reference specific files.

When done, signal pass: devteam signal %s pass`, featureID, featureID, featureID, featureID, featureID, featureID, featureID, featureID, featureID)

	case feature.PhaseConstruction:
		return fmt.Sprintf(`You are in the CONSTRUCTION phase for feature %s.

Your task: Build the spec. Read the spec, plan, and tasks. Write the code. Commit and push.

1. Read spec.md, acceptance.md, plan.md, tasks.md, data-model.md, contracts/ — understand what to build
2. Read existing code to understand conventions
3. Write the code — implement every task in tasks.md
4. Verify the build succeeds (discover and run the project's build command)
5. Commit all changes: git add -A && git commit -m "feat: implement %s"
6. Push to the current branch: git push origin HEAD
7. Signal pass: devteam signal %s pass

That's it. Build to spec. Commit. Push. Signal.

DO NOT write tests, review code, or write documentation — other phases handle those.`, featureID, featureID, featureID)

	case feature.PhaseReview:
		return fmt.Sprintf(`You are in the REVIEW phase for feature %s.

Your task: Read the code and verify it matches the spec. You are a code reviewer, NOT a tester. Do NOT run tests, start servers, or hit endpoints — that's the Tester's job.

Review process:
1. For each acceptance criterion (AC-NNN) in acceptance.md, find the code that implements it and verify it's correct
2. Check for over-engineering: is the implementation the minimum needed?
3. Check for missing implementations: any spec requirements with no corresponding code?
4. Security review for P1 features: authentication, authorization, input validation

Write your findings to review-report.md and submit via CLI:
  devteam artifact submit %s review_report --file review-report.md

With:
- Per-criterion analysis: every AC-NNN from acceptance.md with MET or NOT MET status
- Quoted evidence: specific code with file path and line number
- Over-engineering findings: line count vs expected
- Missing implementation: user stories with no corresponding code

Format for each criterion:
  AC-NNN: [criterion text]
  Status: MET or NOT MET
  Evidence: [file:line] [quoted code or spec text]
  Explanation: [how the code satisfies or fails the criterion]

DO NOT:
- Run tests — that's the Testing phase's job
- Start the service or hit endpoints — that's the Testing phase's job
- Write test files — that's the Testing phase's job
- Write documentation — that's the Delivery phase's job
- Run build commands — that's the Construction phase's job

No critical findings may remain unresolved.`, featureID, featureID)

	case feature.PhaseTesting:
		return fmt.Sprintf(`You are in the TESTING phase for feature %s.

Your task: Write and run tests. You own testing — no other phase runs tests.

Testing process:
1. Spec-implementation drift: Compare spec against what was built before writing tests
2. Discover the project's test infrastructure: read package.json scripts, Makefile, go.mod, Cargo.toml, etc.
3. Write tests at the appropriate levels for what changed:
   - Smoke tests: verify the service/app starts and responds without panicking
   - Integration tests: full request/response cycles or API interactions
   - E2E tests: if the repo has browser test infrastructure, write and run them
   - Unit tests: business logic, state machine transitions, serialization
4. Run ALL tests that the project supports — discover and use the project's test commands
5. Agent failure mode verification: null pointers, empty collections vs null, phantom methods

Key principles:
- Discover what test commands exist and run them — don't invent new commands
- If the project has browser test infrastructure (Playwright, Cypress, etc.), use it
- If tests need a running server, check if the test framework handles server lifecycle automatically
- If you need to start a server for tests, use a port that is NOT already in use
- If tests fail, fix the TEST if the test is wrong, or report the BUG in test-report.md if the implementation is wrong
- Write real tests with real assertions — not "all tests pass" without evidence

Do NOT manage server processes manually:
- Do NOT run ps, grep for processes, start/stop/kill servers by hand
- Let the test framework handle server lifecycle
- Do NOT run commands in a loop waiting for something to happen — run once, read output, act on it

DO NOT:
- Write implementation code — that's the Construction phase's job
- Review code against acceptance criteria — that's the Review phase's job
- Write documentation — that's the Delivery phase's job
- Run build commands (beyond what's needed to compile tests)

Write your test report to test-report.md and submit via CLI:
  devteam artifact submit %s test_report --file test-report.md

With:
- Spec-implementation drift findings
- Test commands discovered and run (exact commands with output)
- Smoke test results: what was started, what was hit, what status codes returned
- Integration test results: which request/response cycles were verified
- E2E test results (if applicable): which scenarios were tested in a browser
- Unit test results: which logic was tested in isolation
- Null/empty checks: which fields verified to return empty collections not null
- Exact assertions verified
- Anti-fake-report: specific evidence, not "all tests pass"

Quality gate:
- Every acceptance criterion has at least one test
- No null pointer panics, no null-vs-empty-collection mismatches
- All tests pass
- ANY failing test is an automatic recirculate`, featureID, featureID)

	case feature.PhaseDelivery:
		return fmt.Sprintf(`You are in the DELIVERY phase for feature %s.

Your task: Write documentation ONLY. The previous phases already built, reviewed, and tested everything. You do NOT verify, build, test, or deploy anything.

The Testing phase ran the full test suite. The Review phase verified acceptance criteria. The Construction phase built the code. Your job is documentation.

Write documentation to a local docs/ directory, then submit via CLI:
  devteam artifact submit %s docs --file docs/index.md

With:
1. **API documentation** — for every endpoint in the plan: method, path, request/response schemas, error responses
2. **User-facing documentation** — for every user story in the spec, using spec terminology
3. **Changelog** — reference the spec number in every entry
4. **Cross-repo release order** (if applicable) — shared libraries first, consumers second, frontend last
5. **Configuration documentation** — env vars, config files, dependencies

Terminology consistency check: documentation must use the same terms as spec.md, not code-internal names.

DO NOT:
- Run build commands (go build, npm run build, etc.) — Construction already did this
- Run test commands (go test, npm test, npx playwright test, etc.) — Testing already did this
- Start the service or hit endpoints — Testing already did this
- Review code against acceptance criteria — Review already did this
- Write implementation code — Construction already did this
- Commit or push code — the pipeline handles commits and pushes automatically
- Check running processes, verify dependencies, or re-prove anything

Write the docs. That's all.`, featureID, featureID)

	default:
		return ""
	}
}

// outcomeInstructions tells the agent HOW to signal pass/recirculate.
func outcomeInstructions(phase feature.Phase) string {
	var recirculateTarget string
	switch phase {
	case feature.PhaseReview, feature.PhaseTesting, feature.PhaseDelivery:
		recirculateTarget = "construction"
	case feature.PhasePlanning:
		recirculateTarget = "inception"
	case feature.PhaseConstruction:
		recirculateTarget = "planning"
	}

	var b strings.Builder
	b.WriteString("\n\n---\n\n## Outcome Signal (MANDATORY)\n\n")
	b.WriteString("After completing your work, signal your outcome using the devteam CLI:\n\n")
	b.WriteString("- `devteam signal <feature-id> pass` — your work is complete and verified\n")
	if recirculateTarget != "" {
		b.WriteString(fmt.Sprintf("- `devteam signal <feature-id> recirculate:%s --notes \"what needs fixing\"` — send work back to %s\n", recirculateTarget, recirculateTarget))
	}
	b.WriteString("- `devteam signal <feature-id> needs_feedback` — you submitted questions and need user answers\n")
	b.WriteString("- `devteam signal <feature-id> failed --notes \"why\"` — you are blocked\n\n")

	if recirculateTarget != "" {
		b.WriteString("Example recirculate command:\n```\n")
		b.WriteString(fmt.Sprintf("devteam signal <feature-id> recirculate:%s --notes \"Missing error handling in handler.go:42\"\n", recirculateTarget))
		b.WriteString("```\n\n")
		b.WriteString(fmt.Sprintf("These notes will be passed to the %s agent so they know exactly what to fix.\n", recirculateTarget))
	} else {
		b.WriteString("Write `devteam signal <feature-id> pass` when your work is complete.\n")
	}

	b.WriteString("\nThe pipeline reads the signal to decide what to do next. If you don't signal, the pipeline will assume `pass`.\n")
	return b.String()
}