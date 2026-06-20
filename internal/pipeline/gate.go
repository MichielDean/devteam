package pipeline

import (
	"fmt"
	"strings"

	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/spec"
)

type GateEvaluator struct {
	specProvider *spec.SpecProvider
}

func NewGateEvaluator(specProvider *spec.SpecProvider) *GateEvaluator {
	return &GateEvaluator{
		specProvider: specProvider,
	}
}

func (ge *GateEvaluator) Evaluate(f *feature.Feature) (*feature.GateResult, error) {
	return ge.EvaluateForPhase(f, f.CurrentPhase())
}

func (ge *GateEvaluator) EvaluateForPhase(f *feature.Feature, phase feature.Phase) (*feature.GateResult, error) {
	gateDef := feature.GetGateDefinition(phase)
	if gateDef == nil {
		return nil, fmt.Errorf("no gate definition for phase %s", phase)
	}

	result := ge.specProvider.ValidateArtifacts(f.ID, gateDef.RequiredArts)
	result.Phase = phase

	for _, desc := range gateDef.ValidationDescs {
		passed := ge.evaluateDesc(f, desc)
		result.Checks = append(result.Checks, feature.CheckResult{
			Name:    desc,
			Passed:  passed,
			Message: ge.checkMessage(desc, passed, f),
		})
	}

	if result.Passed {
		allChecksPass := true
		for _, check := range result.Checks {
			if !check.Passed {
				allChecksPass = false
				break
			}
		}
		result.Passed = allChecksPass
	}

	return &result, nil
}

func (ge *GateEvaluator) evaluateDesc(f *feature.Feature, desc string) bool {
	switch {
	case strings.Contains(desc, "spec.md contains"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactSpecMD)
		if err != nil {
			return false
		}
		return strings.Contains(content, "User Stor") || strings.Contains(content, "user stor") || strings.Contains(content, "US-") || strings.Contains(content, "Scenario")

	case strings.Contains(desc, "acceptance.md contains"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactAcceptanceMD)
		if err != nil {
			return false
		}
		return strings.Contains(content, "AC-") || strings.Contains(content, "Acceptance Criteria")

	case strings.Contains(desc, "repos.yaml identifies"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactReposYAML)
		if err != nil {
			return false
		}
		return strings.Contains(content, "repos:") && (strings.Contains(content, "name:") || strings.Contains(content, "url:") || strings.Contains(content, "branch:"))

	case strings.Contains(desc, "plan.md addresses"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactPlanMD)
		if err != nil {
			return false
		}
		return strings.Contains(content, "##") && len(content) > 100

	case strings.Contains(desc, "tasks.md contains"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTasksMD)
		if err != nil {
			return false
		}
		return strings.Contains(content, "T0") || strings.Contains(content, "- [ ]") || strings.Contains(content, "- [x]")

	case strings.Contains(desc, "dependencies between tasks"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTasksMD)
		if err != nil {
			return false
		}
		return strings.Contains(content, "depend") || strings.Contains(content, "Depend") || strings.Contains(content, "Prerequisite")

	case strings.Contains(desc, "code compiles"):
		return ge.checkBuildCompiles(f)

	case strings.Contains(desc, "no placeholder"):
		return ge.checkNoPlaceholders(f)

	case strings.Contains(desc, "independently buildable"):
		return ge.checkBuildCompiles(f)

	case strings.Contains(desc, "service starts and responds"):
		return ge.checkServiceStarts(f)

	case strings.Contains(desc, "acceptance criterion"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactReviewReport)
		if err != nil {
			return false
		}
		return strings.Contains(content, "AC-") || strings.Contains(content, "criterion")

	case strings.Contains(desc, "critical findings"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactReviewReport)
		if err != nil {
			return false
		}
		return !strings.Contains(content, "CRITICAL") || strings.Contains(content, "resolved")

	case strings.Contains(desc, "security review"):
		if f.Priority == 1 {
			content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactReviewReport)
			if err != nil {
				return false
			}
			return strings.Contains(content, "security") || strings.Contains(content, "Security")
		}
		return true

	case strings.Contains(desc, "smoke tests verify"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "smoke") && (strings.Contains(lower, "server starts") || strings.Contains(lower, "httptest") || strings.Contains(lower, "no panic") || strings.Contains(lower, "responds to"))

	case strings.Contains(desc, "integration tests exercise"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "integration") && (strings.Contains(lower, "http") || strings.Contains(lower, "endpoint") || strings.Contains(lower, "request") || strings.Contains(lower, "response cycle"))

	case strings.Contains(desc, "test"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
		if err != nil {
			return false
		}
		return strings.Contains(content, "PASS") || strings.Contains(content, "pass") || strings.Contains(content, "test")

	case strings.Contains(desc, "documentation uses spec"):
		return true

	case strings.Contains(desc, "changelog references"):
		return true

	case strings.Contains(desc, "cross-repo release"):
		return true

	default:
		return true
	}
}

func (ge *GateEvaluator) checkMessage(desc string, passed bool, f *feature.Feature) string {
	if passed {
		return fmt.Sprintf("✓ %s", desc)
	}
	return fmt.Sprintf("✗ %s (phase: %s, feature: %s)", desc, f.CurrentPhase(), f.ID)
}

func (ge *GateEvaluator) checkBuildCompiles(f *feature.Feature) bool {
	content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
	if err != nil {
		return false
	}
	lower := strings.ToLower(content)
	return strings.Contains(lower, "compiles") || strings.Contains(lower, "build") || strings.Contains(lower, "go build") || strings.Contains(lower, "make") || strings.Contains(lower, "npm run build")
}

func (ge *GateEvaluator) checkNoPlaceholders(f *feature.Feature) bool {
	content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
	if err != nil {
		return false
	}
	lower := strings.ToLower(content)
	return !strings.Contains(lower, "placeholder") && !strings.Contains(lower, "stub") && !strings.Contains(lower, "todo")
}

func (ge *GateEvaluator) checkServiceStarts(f *feature.Feature) bool {
	content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
	if err != nil {
		return false
	}
	lower := strings.ToLower(content)
	return (strings.Contains(lower, "smoke") || strings.Contains(lower, "server starts") || strings.Contains(lower, "httptest") || strings.Contains(lower, "playwright")) && strings.Contains(lower, "no panic")
}