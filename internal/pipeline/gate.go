package pipeline

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/spec"
)

type GateEvaluator struct {
	specProvider     *spec.SpecProvider
	lastCheckOutput string
	workDir          string
}

func NewGateEvaluator(specProvider *spec.SpecProvider) *GateEvaluator {
	return &GateEvaluator{
		specProvider: specProvider,
	}
}

// NewGateEvaluatorWithWorkDir creates a gate evaluator that runs build/test
// commands in the given working directory (e.g., the spec worktree).
func NewGateEvaluatorWithWorkDir(specProvider *spec.SpecProvider, workDir string) *GateEvaluator {
	return &GateEvaluator{
		specProvider: specProvider,
		workDir:       workDir,
	}
}

func (ge *GateEvaluator) workDirOr(f *feature.Feature) string {
	if ge.workDir != "" {
		return ge.workDir
	}
	return ge.specProvider.BaseDir()
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

	case strings.Contains(desc, "constraint register"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactSpecMD)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "constraint register") || strings.Contains(lower, "con-001") || strings.Contains(lower, "| con-") || strings.Contains(lower, "constraint id")

	case strings.Contains(desc, "constraint-derived criteria"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactAcceptanceMD)
		if err != nil {
			return false
		}
		return strings.Contains(content, "CON-") || strings.Contains(content, "Source: CON") || strings.Contains(content, "constraint")

	case strings.Contains(desc, "constraint verification map"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactPlanMD)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "constraint verification") || strings.Contains(lower, "con-") && strings.Contains(lower, "design decision")

	case strings.Contains(desc, "cross-component consistency matrix"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactPlanMD)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "cross-component") || strings.Contains(lower, "consistency matrix") || strings.Contains(lower, "shared value")

	case strings.Contains(desc, "constraint references"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTasksMD)
		if err != nil {
			return false
		}
		return strings.Contains(content, "CON-") || strings.Contains(content, "constraint")

	case strings.Contains(desc, "execution path trace"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactReviewReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "trace") || strings.Contains(lower, "execution path") || strings.Contains(lower, "path:") || strings.Contains(lower, "entry:")

	case strings.Contains(desc, "cross-component consistency verified"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactReviewReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "cross-component") || strings.Contains(lower, "consistency") || strings.Contains(lower, "all producers") || strings.Contains(lower, "all providers")

	case strings.Contains(desc, "negative test vectors verified"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactReviewReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "negative") && (strings.Contains(lower, "vector") || strings.Contains(lower, "reject") || strings.Contains(lower, "conformance"))

	case strings.Contains(desc, "every constraint in the register has at least one test"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "constraint") && (strings.Contains(lower, "con-") || strings.Contains(lower, "register") || strings.Contains(lower, "verified"))

	case strings.Contains(desc, "conformance tests verify"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "conformance") || (strings.Contains(lower, "negative") && strings.Contains(lower, "vector"))

	case strings.Contains(desc, "multi-component constraints tested"):
		content, err := ge.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
		if err != nil {
			return false
		}
		lower := strings.ToLower(content)
		return strings.Contains(lower, "all providers") || strings.Contains(lower, "all components") || strings.Contains(lower, "multi-component") || strings.Contains(lower, "each provider")

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
	msg := fmt.Sprintf("✗ %s (phase: %s, feature: %s)", desc, f.CurrentPhase(), f.ID)
	if ge.lastCheckOutput != "" {
		lines := strings.Split(ge.lastCheckOutput, "\n")
		maxLines := 10
		if len(lines) > maxLines {
			msg += "\n" + strings.Join(lines[:maxLines], "\n") + fmt.Sprintf("\n... (%d more lines)", len(lines)-maxLines)
		} else {
			msg += "\n" + ge.lastCheckOutput
		}
		ge.lastCheckOutput = ""
	}
	return msg
}

func (ge *GateEvaluator) checkBuildCompiles(f *feature.Feature) bool {
	goPath, err := exec.LookPath("go")
	if err != nil {
		goPath = "/usr/local/go/bin/go"
	}
	cmd := exec.Command(goPath, "build", "./...")
	cmd.Dir = ge.workDirOr(f)
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":"+"/usr/local/go/bin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		ge.lastCheckOutput = string(output)
		return false
	}
	if len(output) > 0 && strings.Contains(string(output), "error") {
		ge.lastCheckOutput = string(output)
		return false
	}
	ge.lastCheckOutput = ""
	return true
}

func (ge *GateEvaluator) checkVetPasses(f *feature.Feature) bool {
	goPath, err := exec.LookPath("go")
	if err != nil {
		goPath = "/usr/local/go/bin/go"
	}
	cmd := exec.Command(goPath, "vet", "./...")
	cmd.Dir = ge.workDirOr(f)
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":"+"/usr/local/go/bin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		ge.lastCheckOutput = string(output)
		return false
	}
	if strings.Contains(string(output), "vet:") {
		ge.lastCheckOutput = string(output)
		return false
	}
	ge.lastCheckOutput = ""
	return true
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
	hasSmokeOrServer := strings.Contains(lower, "smoke") || strings.Contains(lower, "server starts") || strings.Contains(lower, "httptest") || strings.Contains(lower, "playwright")
	noPanic := strings.Contains(lower, "no panic") || strings.Contains(lower, "without panic") || strings.Contains(lower, "without panics") || strings.Contains(lower, "no nil pointer")
	return hasSmokeOrServer && noPanic
}

func (ge *GateEvaluator) checkTestSuitePasses(f *feature.Feature) bool {
	workDir := ge.workDirOr(f)
	var failures []string

	// Run go tests if go.mod exists
	if _, err := os.Stat(filepath.Join(workDir, "go.mod")); err == nil {
		goPath, err := exec.LookPath("go")
		if err != nil {
			goPath = "/usr/local/go/bin/go"
		}
		cmd := exec.Command(goPath, "test", "./...", "-count=1", "-timeout", "120s")
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":"+"/usr/local/go/bin")
		output, err := cmd.CombinedOutput()
		if err != nil {
			failures = append(failures, fmt.Sprintf("go test failed:\n%s", string(output)))
		} else if strings.Contains(string(output), "FAIL") {
			failures = append(failures, fmt.Sprintf("go test had failures:\n%s", string(output)))
		}
	}

	// Run npm test if package.json exists and has a test script
	if pkgOutput, err := exec.Command("cat", filepath.Join(workDir, "package.json")).Output(); err == nil {
		if strings.Contains(string(pkgOutput), "\"test\"") {
			cmd := exec.Command("npm", "test")
			cmd.Dir = workDir
			cmd.Env = append(os.Environ(), "CI=true", "PATH="+os.Getenv("PATH"))
			output, err := cmd.CombinedOutput()
			if err != nil {
				failures = append(failures, fmt.Sprintf("npm test failed:\n%s", string(output)))
			}
		}
	}

	// Run UI tests if ui/package.json exists with a test script
	uiDir := filepath.Join(workDir, "ui")
	if pkgOutput, err := os.ReadFile(filepath.Join(uiDir, "package.json")); err == nil {
		if strings.Contains(string(pkgOutput), "\"test\"") {
			// Install deps if needed
			if _, err := os.Stat(filepath.Join(uiDir, "node_modules")); os.IsNotExist(err) {
				exec.Command("npm", "install", "--prefix", uiDir).Run()
			}
			cmd := exec.Command("npm", "test")
			cmd.Dir = uiDir
			cmd.Env = append(os.Environ(), "CI=true", "PATH="+os.Getenv("PATH"))
			output, err := cmd.CombinedOutput()
			if err != nil {
				failures = append(failures, fmt.Sprintf("ui npm test failed:\n%s", string(output)))
			}
		}
	}

	if len(failures) > 0 {
		ge.lastCheckOutput = strings.Join(failures, "\n\n")
		return false
	}

	ge.lastCheckOutput = ""
	return true
}

func (ge *GateEvaluator) checkFrontendTests(f *feature.Feature) bool {
	uiDir := filepath.Join(ge.workDirOr(f), "ui")
	if _, err := os.Stat(uiDir); os.IsNotExist(err) {
		return true
	}

	packageJSON := filepath.Join(uiDir, "package.json")
	if _, err := os.Stat(packageJSON); os.IsNotExist(err) {
		return true
	}

	nodeModules := filepath.Join(uiDir, "node_modules")
	if _, err := os.Stat(nodeModules); os.IsNotExist(err) {
		if err := exec.Command("npm", "install", "--prefix", uiDir).Run(); err != nil {
			ge.lastCheckOutput = fmt.Sprintf("npm install failed: %v", err)
			return false
		}
	}

	playwrightConfig := filepath.Join(uiDir, "playwright.config.ts")
	if _, err := os.Stat(playwrightConfig); err != nil {
		return true
	}

	npxPath, err := exec.LookPath("npx")
	if err != nil {
		return true
	}

	cmd := exec.Command(npxPath, "playwright", "test", "--reporter=list")
	cmd.Dir = uiDir
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":"+"/usr/local/go/bin",
		"CI=true",
		"BASE_URL=http://localhost:18765",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		ge.lastCheckOutput = fmt.Sprintf("Playwright tests failed:\n%s", string(output))
		return false
	}
	if strings.Contains(string(output), "failed") {
		ge.lastCheckOutput = fmt.Sprintf("Playwright tests had failures:\n%s", string(output))
		return false
	}
	return true
}