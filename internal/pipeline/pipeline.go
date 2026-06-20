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
	config       *config.Config
	specProvider *spec.SpecProvider
	specWriter   *spec.SpecWriter
	ruleLoader   *rules.RuleLoader
	roleLoader   *role.RoleLoader
	dispatcher   *role.Dispatcher
}

func NewPipeline(cfg *config.Config, specProvider *spec.SpecProvider) *Pipeline {
	baseDir := specProvider.BaseDir()
	return &Pipeline{
		config:       cfg,
		specProvider: specProvider,
		specWriter:   spec.NewSpecWriter(baseDir),
		ruleLoader:   rules.NewRuleLoader(baseDir),
		roleLoader:   role.NewRoleLoader(baseDir),
		dispatcher:   role.NewDispatcher(baseDir),
	}
}

func NewPipelineWithDispatcher(cfg *config.Config, specProvider *spec.SpecProvider, dispatcher *role.Dispatcher) *Pipeline {
	baseDir := specProvider.BaseDir()
	return &Pipeline{
		config:       cfg,
		specProvider: specProvider,
		specWriter:   spec.NewSpecWriter(baseDir),
		ruleLoader:   rules.NewRuleLoader(baseDir),
		roleLoader:   role.NewRoleLoader(baseDir),
		dispatcher:   dispatcher,
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

	specContext, err := p.specProvider.BuildCrossRepoContext(f.ID, nil)
	if err == nil && specContext != "" {
		contextStr = contextStr + "\n\n---\n\n" + specContext
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
	currentPhase := f.CurrentPhase()
	phases := feature.AllPhases()
	currentIdx := -1
	for i, phase := range phases {
		if phase == currentPhase {
			currentIdx = i
			break
		}
	}
	if currentIdx < 0 {
		return nil, fmt.Errorf("current phase %s not found", currentPhase)
	}
	if currentIdx >= len(phases)-1 {
		return nil, fmt.Errorf("already at final phase %s, use MarkDone to complete", currentPhase)
	}
	nextPhase := phases[currentIdx+1]
	if err := f.AdvanceTo(nextPhase); err != nil {
		return nil, fmt.Errorf("advancing from %s to %s: %w", currentPhase, nextPhase, err)
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

You MUST produce the following artifacts in the spec directory:

1. **spec.md** — Write this file at specs/%s/spec.md with:
   - Feature title and description
   - User stories with priority (P1, P2, P3) and independent tests
   - Functional requirements (FR-NNN format)
   - Success criteria (SC-NNN format, measurable)
   - Edge cases
   - Assumptions

2. **acceptance.md** — Write this file at specs/%s/acceptance.md with:
   - Acceptance criteria traced to each user story (AC-NNN format)
   - Each criterion MUST be verifiable (testable, not vague)
   - Group criteria by user story

3. **repos.yaml** — Write this file at specs/%s/repos.yaml with:
   - Feature ID
   - List of affected repositories with name, URL, and branch
   - At minimum, the devteam repo itself

Do NOT write placeholder content. Every section must contain real, specific content derived from the feature input. If information is missing, make reasonable assumptions and flag them explicitly.`, featureID, featureID, featureID, featureID)

	case feature.PhasePlanning:
		return fmt.Sprintf(`You are in the PLANNING phase for feature %s.

Your task: Create a technical implementation plan and task breakdown from the approved spec.

You MUST produce the following artifacts:

1. **plan.md** — Write this file at specs/%s/plan.md with:
   - Summary of what is being built
   - Technical context (language, dependencies, testing)
   - Component design for each major component
   - API contracts
   - Data model
   - Complexity tracking

2. **tasks.md** — Write this file at specs/%s/tasks.md with:
   - Tasks grouped by phase (not by user story)
   - Each task has: ID (T001, T002...), [P] for parallelizable, description with exact file paths
   - Dependencies between tasks explicitly stated
   - Checkpoints between phases

The plan MUST address all acceptance criteria from acceptance.md. Every task must reference specific files.`, featureID, featureID, featureID)

	case feature.PhaseConstruction:
		return fmt.Sprintf(`You are in the CONSTRUCTION phase for feature %s.

Your task: Implement the code according to the plan and tasks.

Follow the task list in tasks.md. Implement tasks in order, respecting dependencies. Every function must have a real implementation — no stubs or placeholders.

After implementation, verify:
- Code compiles in every affected repository
- No placeholder or stub code remains
- Each repository's changes are independently buildable`, featureID)

	case feature.PhaseReview:
		return fmt.Sprintf(`You are in the REVIEW phase for feature %s.

Your task: Perform adversarial review against the spec acceptance criteria.

Write your findings to specs/%s/review-report.md with:
- Each acceptance criterion reviewed with evidence
- Security findings (especially for P1 features)
- No critical findings may remain unresolved
- Quote specific code or spec text as evidence

Format: For each AC-NNN, state whether it PASSES or FAILS with evidence.`, featureID, featureID)

	case feature.PhaseTesting:
		return fmt.Sprintf(`You are in the TESTING phase for feature %s.

Your task: Write and run tests traced to spec requirements.

Write your test report to specs/%s/test-report.md with:
- Every acceptance criterion has at least one test
- All critical-path tests pass
- Failed tests have reproduction steps
- Test IDs trace to acceptance criteria IDs (AC-NNN)`, featureID, featureID)

	case feature.PhaseDelivery:
		return fmt.Sprintf(`You are in the DELIVERY phase for feature %s.

Your task: Produce documentation matching spec terminology and coordinate release.

Write documentation to specs/%s/docs/ with:
- Documentation using spec terminology
- Changelog referencing the spec number
- Cross-repo release order documented`, featureID, featureID)

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