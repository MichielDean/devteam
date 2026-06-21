package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/role"
	"github.com/MichielDean/devteam/internal/rules"
	"github.com/MichielDean/devteam/internal/spec"
)

type Pipeline struct {
	config        *config.Config
	specProvider  *spec.SpecProvider
	specWriter    *spec.SpecWriter
	ruleLoader    *rules.RuleLoader
	roleLoader    *role.RoleLoader
	dispatcher    *role.Dispatcher
	questionStore feature.QuestionStore
}

func NewPipeline(cfg *config.Config, specProvider *spec.SpecProvider) *Pipeline {
	baseDir := specProvider.BaseDir()
	return &Pipeline{
		config:        cfg,
		specProvider:  specProvider,
		specWriter:    spec.NewSpecWriter(baseDir),
		ruleLoader:    rules.NewRuleLoaderWithConfig(baseDir, cfg),
		roleLoader:    role.NewRoleLoader(baseDir),
		dispatcher:    role.NewDispatcher(baseDir),
		questionStore: feature.NewFileQuestionStore(baseDir),
	}
}

func NewPipelineWithDispatcher(cfg *config.Config, specProvider *spec.SpecProvider, dispatcher *role.Dispatcher) *Pipeline {
	baseDir := specProvider.BaseDir()
	return &Pipeline{
		config:        cfg,
		specProvider:  specProvider,
		specWriter:    spec.NewSpecWriter(baseDir),
		ruleLoader:    rules.NewRuleLoaderWithConfig(baseDir, cfg),
		roleLoader:    role.NewRoleLoader(baseDir),
		dispatcher:    dispatcher,
		questionStore: feature.NewFileQuestionStore(baseDir),
	}
}

func NewPipelineWithQuestionStore(cfg *config.Config, specProvider *spec.SpecProvider, questionStore feature.QuestionStore) *Pipeline {
	baseDir := specProvider.BaseDir()
	dispatcher := role.NewDispatcher(baseDir)
	return &Pipeline{
		config:        cfg,
		specProvider:  specProvider,
		specWriter:    spec.NewSpecWriter(baseDir),
		ruleLoader:    rules.NewRuleLoaderWithConfig(baseDir, cfg),
		roleLoader:    role.NewRoleLoader(baseDir),
		dispatcher:    dispatcher,
		questionStore: questionStore,
	}
}

func (p *Pipeline) RunPhase(f *feature.Feature) (*feature.PhaseState, error) {
	currentPhase := f.CurrentPhase()
	phaseConfig, err := p.getPhaseConfig(currentPhase)
	if err != nil {
		return nil, err
	}

	roles := phaseConfig.Roles
	if len(roles) == 0 {
		return nil, fmt.Errorf("no roles configured for phase %s", currentPhase)
	}

	now := time.Now()
	ps, ok := f.PhaseStates[currentPhase]
	if !ok {
		ps = &feature.PhaseState{
			Phase: currentPhase,
		}
		f.PhaseStates[currentPhase] = ps
	}
	ps.Status = feature.StatusInProgress
	ps.StartedAt = &now

	return ps, nil
}

type RunResult struct {
	Phase       feature.Phase
	RoleResults []*role.DispatchResult
	GateResult  *feature.GateResult
	Advanced    bool
	Message     string
}

func (p *Pipeline) RunPhaseWithAgent(ctx context.Context, f *feature.Feature) (*RunResult, error) {
	currentPhase := f.CurrentPhase()
	phaseConfig, err := p.getPhaseConfig(currentPhase)
	if err != nil {
		return nil, err
	}

	roles := phaseConfig.Roles
	if len(roles) == 0 {
		return nil, fmt.Errorf("no roles configured for phase %s", currentPhase)
	}

	now := time.Now()
	ps, ok := f.PhaseStates[currentPhase]
	if !ok {
		ps = &feature.PhaseState{
			Phase: currentPhase,
		}
		f.PhaseStates[currentPhase] = ps
	}
	ps.Status = feature.StatusInProgress
	ps.StartedAt = &now

	contextStr, err := p.ruleLoader.BuildContext(string(currentPhase), roles[0], f.Priority)
	if err != nil {
		return nil, fmt.Errorf("building context for phase %s role %s: %w", currentPhase, roles[0], err)
	}

	if currentPhase == feature.PhaseInception {
		inputContent, err := p.specProvider.ReadArtifact(f.ID, feature.ArtifactInputMD)
		if err == nil && inputContent != "" {
			contextStr = contextStr + "\n\n---\n\n=== Feature Input ===\n" + inputContent
		}
	}

	// Inject human responses if there are answered/assumed questions
	if p.questionStore != nil {
		questions, qErr := p.questionStore.ListQuestions(ctx, f.ID)
		if qErr == nil && len(questions) > 0 {
			timeoutMinutes := p.config.Pipeline.GetHumanInteractionTimeoutMinutes()
			humanResponses := feature.BuildHumanResponsesContext(questions, timeoutMinutes)
			if humanResponses != "" {
				contextStr = contextStr + humanResponses
			}
		}
	}

	specContext, err := p.specProvider.BuildCrossRepoContext(f.ID, nil)
	if err == nil && specContext != "" {
		contextStr = contextStr + "\n\n---\n\n" + specContext
	}

	// Include gate failure details if present (for recirculation context)
	gateFailurePath := filepath.Join(p.specProvider.FeatureDir(f.ID), "GATE_FAILURE.md")
	if gateFailureContent, err := os.ReadFile(gateFailurePath); err == nil {
		contextStr = contextStr + "\n\n---\n\n# Gate Failure (Previous Attempt)\n\n" + string(gateFailureContent)
	}

	var roleResults []*role.DispatchResult
	for _, roleName := range roles {
		roleDef, err := p.roleLoader.Load(roleName)
		if err != nil {
			return nil, fmt.Errorf("loading role %s: %w", roleName, err)
		}

		promptContext := roleDef.Instructions + "\n\n---\n\n" + contextStr

		phaseInstruction := p.phaseInstruction(currentPhase, f.ID)
		if phaseInstruction != "" {
			promptContext = promptContext + "\n\n---\n\n" + phaseInstruction
		}

		contextMD := buildContextMD(f.ID, string(currentPhase), roleName, promptContext)
		contextPath := filepath.Join(p.specProvider.FeatureDir(f.ID), "CONTEXT.md")
		if err := os.WriteFile(contextPath, []byte(contextMD), 0644); err != nil {
			return nil, fmt.Errorf("writing CONTEXT.md: %w", err)
		}

		req := role.DispatchRequest{
			FeatureID: f.ID,
			Phase:     string(currentPhase),
			Role:      roleName,
			Context:   promptContext,
		}

		result, err := p.dispatcher.Dispatch(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("dispatching role %s for phase %s: %w", roleName, currentPhase, err)
		}
		roleResults = append(roleResults, result)
	}

	gateResult, err := NewGateEvaluator(p.specProvider).EvaluateForPhase(f, currentPhase)
	if err != nil {
		return nil, fmt.Errorf("evaluating gate for phase %s: %w", currentPhase, err)
	}

	ps.GateResult = gateResult
	if gateResult.Passed {
		ps.Status = feature.StatusPassed
		ps.CompletedAt = &now
	} else {
		ps.Status = feature.StatusGateBlocked
	}

	result := &RunResult{
		Phase:       currentPhase,
		RoleResults: roleResults,
		GateResult:  gateResult,
		Message:     fmt.Sprintf("Phase %s completed. Gate passed: %v", currentPhase, gateResult.Passed),
	}

	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}

	return result, nil
}

func (p *Pipeline) AdvanceFeature(f *feature.Feature) (*feature.Feature, error) {
	fromPhase := f.CurrentPhase()
	phases := feature.AllPhases()
	fromIdx := -1
	for i, phase := range phases {
		if phase == fromPhase {
			fromIdx = i
			break
		}
	}
	if fromIdx < 0 {
		return nil, fmt.Errorf("current phase %s not found", fromPhase)
	}
	if fromIdx >= len(phases)-1 {
		return nil, fmt.Errorf("already at final phase %s, use MarkDone to complete", fromPhase)
	}
	nextPhase := phases[fromIdx+1]
	if err := f.AdvanceTo(nextPhase); err != nil {
		return nil, fmt.Errorf("advancing from %s to %s: %w", fromPhase, nextPhase, err)
	}
	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}
	return f, nil
}

func (p *Pipeline) AdvanceFeatureFrom(f *feature.Feature, fromPhase feature.Phase) (*feature.Feature, error) {
	phases := feature.AllPhases()
	fromIdx := -1
	for i, phase := range phases {
		if phase == fromPhase {
			fromIdx = i
			break
		}
	}
	if fromIdx < 0 {
		return nil, fmt.Errorf("phase %s not found", fromPhase)
	}
	if fromIdx >= len(phases)-1 {
		return nil, fmt.Errorf("already at final phase %s, use MarkDone to complete", fromPhase)
	}
	nextPhase := phases[fromIdx+1]
	if err := f.AdvanceTo(nextPhase); err != nil {
		return nil, fmt.Errorf("advancing from %s to %s: %w", fromPhase, nextPhase, err)
	}
	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}
	return f, nil
}

func (p *Pipeline) RecirculateFeature(f *feature.Feature, targetPhase feature.Phase, reason string) (*feature.Feature, error) {
	if err := f.RecirculateTo(targetPhase); err != nil {
		return nil, fmt.Errorf("recirculating from %s to %s: %w", f.CurrentPhase(), targetPhase, err)
	}
	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}
	return f, nil
}

func (p *Pipeline) EvaluateGate(f *feature.Feature) (*feature.GateResult, error) {
	return NewGateEvaluator(p.specProvider).Evaluate(f)
}

func (p *Pipeline) EvaluateGateForPhase(f *feature.Feature, phase feature.Phase) (*feature.GateResult, error) {
	return NewGateEvaluator(p.specProvider).EvaluateForPhase(f, phase)
}

func (p *Pipeline) ListFeatures() ([]*feature.Feature, error) {
	return p.specProvider.ListFeatures()
}

func (p *Pipeline) GetFeature(featureID string) (*feature.Feature, error) {
	return p.specProvider.LoadFeatureState(featureID)
}

func (p *Pipeline) SaveFeature(f *feature.Feature) error {
	return p.specProvider.SaveFeatureState(f)
}

func (p *Pipeline) getPhaseConfig(phase feature.Phase) (*config.PhaseConfig, error) {
	for i := range p.config.Pipeline.Phases {
		if p.config.Pipeline.Phases[i].Name == string(phase) {
			return &p.config.Pipeline.Phases[i], nil
		}
	}
	return nil, fmt.Errorf("phase %s not found in config", phase)
}

func (p *Pipeline) phaseInstruction(phase feature.Phase, featureID string) string {
	switch phase {
	case feature.PhaseInception:
		return fmt.Sprintf(`You are in the INCEPTION phase for feature %s.

Your task: Explore, clarify, and refine the idea into a structured specification.

Follow the Inception Phase Rules for detailed procedures (request type classification, completeness analysis, error scenario tables, empty state behavior, brownfield analysis). The rules are loaded in your context — use them.

You MUST produce the following artifacts in the spec directory:

1. **spec.md** — Write this file at specs/%s/spec.md with:
   - Feature title and description
   - User stories with priority (P1, P2, P3) — each with independent test
   - Functional requirements (FR-NNN format) — each traced to a user story
   - Key entities and relationships (data model overview)
   - State transitions for entities with lifecycle (valid transitions and invalid transitions)
   - Success criteria (SC-NNN format, measurable — "Given X, When Y, Then Z")
   - Error scenarios table: for each user action, what happens on success AND on each error condition (400, 404, 409, 500)
   - Empty state behavior: what the API/UI returns when collections are empty (200 with [], not 404)
   - Assumptions and scope boundaries — flag every assumption with [ASSUMPTION: ...]
   - No [NEEDS CLARIFICATION] markers may remain — resolve them or convert to assumptions

2. **acceptance.md** — Write this file at specs/%s/acceptance.md with:
   - Acceptance criteria traced to each user story (AC-NNN format)
   - Each criterion in format: AC-NNN: Given [precondition], when [action], then [expected result]
     Test level: [smoke | integration | e2e | unit]
     Verification: [specific assertion or scenario]
   - Every user story has at least one criterion per relevant test level
   - Error paths and empty states explicitly covered
   - No "should work well" or "should be fast" — only "Given X, When Y, Then Z"

3. **repos.yaml** — Write this file at specs/%s/repos.yaml with:
   - Feature ID
   - List of affected repositories with name, URL, and branch
   - At minimum, the devteam repo itself

Do NOT write placeholder content. Every section must contain real, specific content derived from the feature input. If information is missing, make reasonable assumptions and flag them with [ASSUMPTION: ...].`, featureID, featureID, featureID, featureID)

	case feature.PhasePlanning:
		return fmt.Sprintf(`You are in the PLANNING phase for feature %s.

Your task: Design the technical approach with enough specificity that the Developer can implement without making architectural decisions on the fly.

Follow the Planning Phase Rules for detailed procedures (component identification, data modeling, API contracts, NFR design, task decomposition). The rules are loaded in your context — use them.

You MUST produce the following artifacts:

1. **plan.md** — Write this file at specs/%s/plan.md with:
   - Summary of what is being built
   - Technical context (language, framework, dependencies)
   - Project structure (where files go)
   - Component design: for each component, its purpose, responsibilities, interfaces, and dependencies
   - Data model: entities, attributes, relationships, state transitions, data integrity rules
   - API contracts: for each endpoint, method, path, request schema, response schema (including error responses)
   - Test strategy per component: what testing levels are required (smoke, integration, e2e, unit)
   - Agent failure mode checks: which checks apply to which tasks
   - NFR considerations: performance, security, scalability, reliability (as applicable)

2. **tasks.md** — Write this file at specs/%s/tasks.md with:
   - Tasks grouped by user story priority (P1 first, then P2, then P3)
   - Each task has: ID (T001, T002...), description with exact file paths, [P] for parallelizable
   - Done conditions: specific verifiable assertions (not "implement the API" but "implement the API and verify: service starts, GET /api/features returns 200, POST with missing title returns 400")
   - Dependencies between tasks explicitly stated
   - Test level required for each task (smoke, integration, e2e, unit)
   - Agent failure mode checks per task

The plan MUST address all acceptance criteria from acceptance.md. Every task must reference specific files.`, featureID, featureID, featureID)

	case feature.PhaseConstruction:
		return fmt.Sprintf(`You are in the CONSTRUCTION phase for feature %s.

Your task: Implement the code according to the plan and tasks, following the Construction Phase Rules for self-verification, brownfield patterns, and agent failure mode checks.

Before writing any code:
1. Read spec.md and acceptance.md — understand what you're building and why
2. Read plan.md — understand the technical approach and test strategy
3. Read tasks.md — understand what to implement and in what order
4. If brownfield: read existing code to understand conventions

Implementation approach:
- Follow the task list in tasks.md, respecting dependency order
- Write the minimum code needed to satisfy each task's done conditions
- If brownfield: modify existing files in-place, follow existing conventions, do NOT create ClassName_modified.go
- Write tests alongside the code, not after

Self-verification before marking any task complete:
- Build succeeds, binary runs without panicking
- Hit each endpoint, verify no nil pointer panics, proper error codes
- Done conditions from tasks.md are verified
- No TODO, FIXME, HACK, or placeholder implementations remain
- JSON arrays are [] not null (marshal zero-value struct to check)
- Error paths work: 400 for invalid input, 404 for missing resources, 409 for conflicts

Agent failure mode checks:
- Nil pointer chains: initialize struct fields in correct order
- Null vs empty arrays: use json:"fieldname" NOT json:"fieldname,omitempty"
- Recovery middleware first: must be outermost middleware
- Error response structure: {"error": "code", "details": "message"}
- No over-engineering: 500 lines is suspicious, 5000 lines is almost certainly wrong
- No phantom methods: every method called must actually exist

After all tasks are complete:
- go build ./... must succeed
- go test ./... must pass
- Service starts and responds without panicking`, featureID)

	case feature.PhaseReview:
		return fmt.Sprintf(`You are in the REVIEW phase for feature %s.

Your task: Perform adversarial review against the spec acceptance criteria. Follow the Review Phase Rules for the structured review process.

Review process:
1. Spec review: Compare plan against spec — does every user story have corresponding tasks?
2. Code review: For each task, verify done conditions with specific evidence
3. Over-engineering check: Is implementation the minimum needed?
4. Missing implementation check: Any spec requirements not implemented?

Write your findings to specs/%s/review-report.md with:
- Per-criterion analysis: every AC-NNN from acceptance.md with MET or NOT MET status
- Quoted evidence: specific code with file path and line number
- Over-engineering findings: line count vs expected
- Missing implementation: user stories with no corresponding code

Format for each criterion:
  AC-NNN: [criterion text]
  Status: MET or NOT MET
  Evidence: [file:line] [quoted code or spec text]
  Explanation: [how the code satisfies or fails the criterion]

Key checks:
- Null pointer safety: every handler dereferencing pointers, every middleware chain
- JSON serialization: every slice/map field returns [] not null
- Error path coverage: 400, 404, 409, empty state, 500 recovery
- Middleware chain: recovery middleware is outermost, CORS is correct
- Security (P1): authentication, authorization, input validation, no secrets in logs

No critical findings may remain unresolved.`, featureID, featureID)

	case feature.PhaseTesting:
		return fmt.Sprintf(`You are in the TESTING phase for feature %s.

Your task: Verify that what was built actually works. Follow the Testing Phase Rules for the structured testing process.

Testing process:
1. Spec-implementation drift: Compare spec against what was built before writing tests
2. Determine testing levels needed (smoke always, integration for API, E2E for UI, unit for logic)
3. Write and execute smoke tests: start service, hit every endpoint, verify no panics
4. Write and execute integration tests: full HTTP request/response cycles
5. Write and execute E2E tests (if UI changed): load in browser, verify no console errors
6. Write and execute unit tests: business logic, state machine transitions, serialization
7. Agent failure mode verification: nil pointers, null arrays, phantom methods, over-engineering

Write your test report to specs/%s/test-report.md with:
- Spec-implementation drift findings
- Smoke test results: which endpoints were hit, what status codes returned
- Integration test results: which request/response cycles were verified
- E2E test results (if applicable): which pages were loaded, any console errors
- Unit test results: which logic was tested in isolation
- Null/empty checks: which fields verified to return [] not null
- State machine transitions: which transitions were verified
- Exact commands to reproduce each test
- Exact assertions verified
- Anti-fake-report: specific evidence, not "all tests pass"

Quality gate:
- Every acceptance criterion has at least one test
- No nil pointer panics, no null-vs-empty-array mismatches
- All smoke and integration tests pass
- ANY failing test is an automatic recirculate`, featureID, featureID)

	case feature.PhaseDelivery:
		return fmt.Sprintf(`You are in the DELIVERY phase for feature %s.

Your task: Ship and document. Follow the Delivery Phase Rules for documentation, release coordination, and deployment verification.

Documentation:
1. API documentation: for every endpoint in the plan, document method, path, request/response schemas, error responses
2. User-facing documentation: for every user story in the spec, document using spec terminology
3. Changelog: reference the spec number in every entry

Cross-repo release:
- If the feature spans repos, document release order (shared libraries first, consumers second, frontend last)
- Tag all repos with consistent version references

Deployment verification (ALL must pass before marking delivery complete):
- Build the binary: go build -o ~/go/bin/devteam ./cmd/devteam/
- Start the service: verify it starts without panicking
- Hit the endpoints: verify the API responds correctly
- Load the UI: verify the frontend renders without console errors
- Run the test suite: verify all tests pass

Write documentation to specs/%s/docs/ with:
- API documentation per endpoint (method, path, request, response, errors)
- User-facing documentation using spec terminology
- Changelog referencing the spec number
- Cross-repo release order (if applicable)
- Configuration documentation (env vars, config files, dependencies)

Terminology consistency check: documentation must use the same terms as spec.md, not code-internal names.`, featureID, featureID)

	default:
		return ""
	}
}

func buildContextMD(featureID, phase, role, promptContext string) string {
	var b strings.Builder
	b.WriteString("# Dev Team Context\n\n")
	b.WriteString(fmt.Sprintf("Feature: %s\n", featureID))
	b.WriteString(fmt.Sprintf("Phase: %s\n", phase))
	b.WriteString(fmt.Sprintf("Role: %s\n\n", role))
	b.WriteString("---\n\n")
	b.WriteString(promptContext)
	return b.String()
}
