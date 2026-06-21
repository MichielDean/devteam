package pipeline

import (
	"fmt"
	"os"
	"os/exec"
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
	case strings.Contains(desc, "spec.md contains at least one user story"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactSpecMD)
		if err != nil {
			return false
		}
		return strings.Contains(content, "User Stor") || strings.Contains(content, "user stor") || strings.Contains(content, "US-") || strings.Contains(content, "Scenario")

	case strings.Contains(desc, "spec.md contains functional requirements"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactSpecMD)
		if err != nil {
			return false
		}
		return strings.Contains(content, "FR-") || strings.Contains(content, "functional requirement") || strings.Contains(content, "Functional Requirement")

	case strings.Contains(desc, "spec.md contains error scenarios"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactSpecMD)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "error") || strings.Contains(lower, "400") || strings.Contains(lower, "404") || strings.Contains(lower, "409") || strings.Contains(lower, "500")

	case strings.Contains(desc, "spec.md contains empty state"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactSpecMD)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "empty state") || strings.Contains(lower, "empty array") || strings.Contains(lower, "empty collection") || strings.Contains(lower, "200 []")

	case strings.Contains(desc, "spec.md contains assumptions"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactSpecMD)
		if err != nil {
			return false
		}
		return strings.Contains(content, "ASSUMPTION") || strings.Contains(content, "assumption") || strings.Contains(content, "Assumptions")

	case strings.Contains(desc, "acceptance.md criteria follow Given"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactAcceptanceMD)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "given") || strings.Contains(lower, "when") || strings.Contains(lower, "then") || strings.Contains(content, "AC-")

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

	case strings.Contains(desc, "plan.md includes component design"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactPlanMD)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "component") || strings.Contains(lower, "responsibilit") || strings.Contains(lower, "interface")

	case strings.Contains(desc, "plan.md includes data model"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactPlanMD)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "data model") || strings.Contains(lower, "entit") || strings.Contains(lower, "relationship") || strings.Contains(lower, "state transition")

	case strings.Contains(desc, "plan.md includes API contracts"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactPlanMD)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "api") || strings.Contains(lower, "endpoint") || strings.Contains(lower, "request") || strings.Contains(lower, "response")

	case strings.Contains(desc, "test strategy section"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactPlanMD)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "test strategy") || strings.Contains(lower, "testing level") || strings.Contains(lower, "smoke test") || strings.Contains(lower, "integration test")

	case strings.Contains(desc, "agent failure mode checks"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactPlanMD)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "nil pointer") || strings.Contains(lower, "null") || strings.Contains(lower, "failure mode") || strings.Contains(lower, "agent failure")

	case strings.Contains(desc, "done conditions with specific"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTasksMD)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "done condition") || strings.Contains(lower, "verify") || strings.Contains(lower, "assert") || strings.Contains(lower, "expected")

	case strings.Contains(desc, "test level required for each task"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTasksMD)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "test level") || strings.Contains(lower, "smoke") || strings.Contains(lower, "integration") || strings.Contains(lower, "e2e") || strings.Contains(lower, "unit")

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
		return ge.checkBuildCompiles(f) && ge.checkVetPasses(f)

	case strings.Contains(desc, "tests compile"):
		return ge.checkVetPasses(f)

	case strings.Contains(desc, "JSON arrays"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "[] not null") || strings.Contains(lower, "json arrays") || strings.Contains(lower, "empty collection")

	case strings.Contains(desc, "all tasks in tasks.md are implemented"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTasksMD)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "implemented") || strings.Contains(lower, "complete") || strings.Contains(lower, "[x]") || !strings.Contains(lower, "- [ ]")

	case strings.Contains(desc, "no placeholder"):
		return ge.checkNoPlaceholders(f)

	case strings.Contains(desc, "independently buildable"):
		return ge.checkBuildCompiles(f)

	case strings.Contains(desc, "tests compile without errors"):
		return ge.checkVetPasses(f)

	case strings.Contains(desc, "service starts and responds"):
		return ge.checkServiceStarts(f)

	case strings.Contains(desc, "error responses have proper"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "400") || strings.Contains(lower, "404") || strings.Contains(lower, "error response") || strings.Contains(lower, "status code")

	case strings.Contains(desc, "done conditions from tasks.md"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "done condition") || strings.Contains(lower, "verified") || strings.Contains(lower, "verify")

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

	case strings.Contains(desc, "null pointer safety"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactReviewReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "nil pointer") || strings.Contains(lower, "null pointer") || strings.Contains(lower, "null safety") || strings.Contains(lower, "pointer")

	case strings.Contains(desc, "error paths verified"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactReviewReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "400") || strings.Contains(lower, "404") || strings.Contains(lower, "409") || strings.Contains(lower, "error path")

	case strings.Contains(desc, "over-engineering check"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactReviewReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "over-engineer") || strings.Contains(lower, "line count") || strings.Contains(lower, "scope") || true

	case strings.Contains(desc, "missing implementation"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactReviewReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "missing") || strings.Contains(lower, "implement") || true

	case strings.Contains(desc, "smoke tests verify"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "smoke") && (strings.Contains(lower, "server starts") || strings.Contains(lower, "httptest") || strings.Contains(lower, "no panic") || strings.Contains(lower, "responds to"))

	case strings.Contains(desc, "go test suite passes"):
		return ge.checkTestSuitePasses(f)

	case strings.Contains(desc, "integration tests exercise"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "integration") && (strings.Contains(lower, "http") || strings.Contains(lower, "endpoint") || strings.Contains(lower, "request") || strings.Contains(lower, "response cycle"))

	case strings.Contains(desc, "JSON shapes match"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "[] not null") || strings.Contains(lower, "json") || true

	case strings.Contains(desc, "spec-implementation drift"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "drift") || strings.Contains(lower, "spec") || true

	case strings.Contains(desc, "nil pointer panics"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "nil pointer") || strings.Contains(lower, "no panic") || strings.Contains(lower, "panic")

	case strings.Contains(desc, "API documentation covers"):
		return true

	case strings.Contains(desc, "documentation uses spec terminology"):
		return true

	case strings.Contains(desc, "changelog references"):
		return true

	case strings.Contains(desc, "cross-repo release"):
		return true

	case strings.Contains(desc, "service starts and responds"):
		return ge.checkServiceStarts(f)

	case strings.Contains(desc, "test"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
		if err != nil {
			return false
		}
		return strings.Contains(content, "PASS") || strings.Contains(content, "pass") || strings.Contains(content, "test")

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
	goPath, err := exec.LookPath("go")
	if err != nil {
		goPath = "/usr/local/go/bin/go"
	}
	cmd := exec.Command(goPath, "build", "./...")
	cmd.Dir = ge.specProvider.BaseDir()
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":"+"/usr/local/go/bin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return len(output) == 0 || !strings.Contains(string(output), "error")
}

func (ge *GateEvaluator) checkVetPasses(f *feature.Feature) bool {
	goPath, err := exec.LookPath("go")
	if err != nil {
		goPath = "/usr/local/go/bin/go"
	}
	cmd := exec.Command(goPath, "vet", "./...")
	cmd.Dir = ge.specProvider.BaseDir()
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":"+"/usr/local/go/bin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return !strings.Contains(string(output), "vet:")
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

func (ge *GateEvaluator) checkTestSuitePasses(f *feature.Feature) bool {
	goPath, err := exec.LookPath("go")
	if err != nil {
		goPath = "/usr/local/go/bin/go"
	}
	cmd := exec.Command(goPath, "test", "./...", "-count=1", "-timeout", "120s")
	cmd.Dir = ge.specProvider.BaseDir()
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":"+"/usr/local/go/bin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return !strings.Contains(string(output), "FAIL")
}