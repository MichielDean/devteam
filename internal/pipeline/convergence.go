package pipeline

import (
	"fmt"
	"strings"

	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/spec"
)

type ConvergenceCheck struct {
	FeatureID  string
	Phase       feature.Phase
	Drifted     bool
	Findings    []ConvergenceFinding
}

type ConvergenceFinding struct {
	Area       string
	Expected   string
	Actual     string
	Severity   string
	Suggestion string
}

type ConvergenceDetector struct {
	specProvider *spec.SpecProvider
}

func NewConvergenceDetector(specProvider *spec.SpecProvider) *ConvergenceDetector {
	return &ConvergenceDetector{
		specProvider: specProvider,
	}
}

func (cd *ConvergenceDetector) Check(f *feature.Feature) (*ConvergenceCheck, error) {
	result := &ConvergenceCheck{
		FeatureID: f.ID,
		Phase:     f.CurrentPhase(),
		Drifted:   false,
	}

	switch f.CurrentPhase() {
	case feature.PhaseConstruction, feature.PhaseReview, feature.PhaseTesting:
		if err := cd.checkSpecAlignment(f, result); err != nil {
			return nil, fmt.Errorf("checking spec alignment: %w", err)
		}
	case feature.PhasePlanning:
		if err := cd.checkInceptionArtifacts(f, result); err != nil {
			return nil, fmt.Errorf("checking inception artifacts: %w", err)
		}
	default:
	}

	if len(result.Findings) > 0 {
		result.Drifted = true
	}

	return result, nil
}

func (cd *ConvergenceDetector) checkSpecAlignment(f *feature.Feature, result *ConvergenceCheck) error {
	specContent, err := cd.specProvider.ReadArtifact(f.ID, feature.ArtifactSpecMD)
	if err != nil {
		result.Findings = append(result.Findings, ConvergenceFinding{
			Area:     "spec.md",
			Expected: "spec file exists and is non-empty",
			Actual:   fmt.Sprintf("error reading spec: %v", err),
			Severity: "critical",
			Suggestion: "Re-run inception phase to regenerate spec.md",
		})
		return nil
	}

	if len(strings.TrimSpace(specContent)) < 50 {
		result.Findings = append(result.Findings, ConvergenceFinding{
			Area:       "spec.md",
			Expected:  "spec with meaningful content",
			Actual:    "spec appears to be a stub with minimal content",
			Severity:  "warning",
			Suggestion: "PM should refine the spec during inception",
		})
	}

	acceptanceContent, err := cd.specProvider.ReadArtifact(f.ID, feature.ArtifactAcceptanceMD)
	if err != nil {
		result.Findings = append(result.Findings, ConvergenceFinding{
			Area:     "acceptance.md",
			Expected: "acceptance criteria file exists",
			Actual:   fmt.Sprintf("error reading acceptance: %v", err),
			Severity: "critical",
			Suggestion: "Re-run inception phase to regenerate acceptance.md",
		})
		return nil
	}

	if strings.Contains(acceptanceContent, "To be refined") || strings.Contains(acceptanceContent, "[To be refined") {
		result.Findings = append(result.Findings, ConvergenceFinding{
			Area:       "acceptance.md",
			Expected:   "refined acceptance criteria",
			Actual:     "acceptance criteria contain unrefined placeholders",
			Severity:   "warning",
			Suggestion: "PM should refine acceptance criteria before proceeding",
		})
	}

	if strings.Contains(specContent, "To be refined") || strings.Contains(specContent, "[To be refined") {
		result.Findings = append(result.Findings, ConvergenceFinding{
			Area:       "spec.md",
			Expected:   "refined user stories",
			Actual:     "spec contains unrefined placeholder stories",
			Severity:   "warning",
			Suggestion: "PM should refine user stories during inception",
		})
	}

	planContent, _ := cd.specProvider.ReadArtifact(f.ID, feature.ArtifactPlanMD)
	if planContent != "" && specContent != "" {
		specSections := countSections(specContent)
		planSections := countSections(planContent)
		if planSections < specSections/2 && specSections > 2 {
			result.Findings = append(result.Findings, ConvergenceFinding{
				Area:       "plan.md",
				Expected:   "plan addressing all spec sections",
				Actual:     fmt.Sprintf("plan has %d sections vs spec's %d", planSections, specSections),
				Severity:   "warning",
				Suggestion: "Architect should ensure plan covers all spec requirements",
			})
		}
	}

	return nil
}

func (cd *ConvergenceDetector) checkInceptionArtifacts(f *feature.Feature, result *ConvergenceCheck) error {
	specContent, err := cd.specProvider.ReadArtifact(f.ID, feature.ArtifactSpecMD)
	if err != nil {
		return nil
	}

	requiredSections := []string{"User Story", "Requirements", "Success Criteria"}
	for _, section := range requiredSections {
		if !strings.Contains(specContent, section) {
			result.Findings = append(result.Findings, ConvergenceFinding{
				Area:       "spec.md",
				Expected:   fmt.Sprintf("section '%s'", section),
				Actual:     "section missing from spec",
				Severity:   "warning",
				Suggestion: fmt.Sprintf("Add %s section to spec.md", section),
			})
		}
	}

	return nil
}

func countSections(content string) int {
	count := 0
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "##") {
			count++
		}
	}
	return count
}