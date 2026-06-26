package pipeline

import (
	"strings"
	"testing"

	"github.com/MichielDean/devteam/internal/feature"
)

func TestConvergenceDetector_UnrefinedSpec(t *testing.T) {
	tmpDir := t.TempDir()
	provider, _ := newTestProvider(t, tmpDir)
	writer := newTestWriter(provider)

	f := feature.NewFeature("001-conv", "Convergence Test", 2, feature.IntakeLooseIdea)
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}

	specContent := `# Feature Specification: Test

**Status**: Draft

## User Scenarios & Testing *(mandatory)*

### User Story 1 - [Title] (Priority: P1)

[To be refined from loose idea]

## Requirements *(mandatory)*

- **FR-001**: [To be refined from loose idea]
`
	acceptanceContent := `# Acceptance Criteria: Test

- **AC-001**: [To be refined during PM exploration]
`
	if err := writer.WriteArtifact(f.ID, feature.ArtifactSpecMD, []byte(specContent)); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteArtifact(f.ID, feature.ArtifactAcceptanceMD, []byte(acceptanceContent)); err != nil {
		t.Fatal(err)
	}

	f.AdvanceTo(feature.PhaseInception)
	f.AdvanceTo(feature.PhasePlanning)
	f.AdvanceTo(feature.PhaseConstruction)
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}

	detector := NewConvergenceDetector(provider)
	result, err := detector.Check(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Drifted {
		t.Error("expected drift to be detected for unrefined spec")
	}

	foundUnrefined := false
	for _, finding := range result.Findings {
		if strings.Contains(finding.Area, "acceptance") && strings.Contains(finding.Actual, "unrefined") {
			foundUnrefined = true
		}
	}
	if !foundUnrefined {
		t.Error("expected finding about unrefined acceptance criteria")
	}
}

func TestConvergenceDetector_RefinedSpec_NoDrift(t *testing.T) {
	tmpDir := t.TempDir()
	provider, _ := newTestProvider(t, tmpDir)
	writer := newTestWriter(provider)

	f := feature.NewFeature("001-refined", "Refined Test", 2, feature.IntakeLooseIdea)
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}

	specContent := `# Feature Specification: Auth System

## User Scenarios & Testing

### User Story 1 - Login (Priority: P1)

Users can log in with email and password.

## Requirements

- **FR-001**: The system MUST authenticate users via email/password

## Success Criteria

- **SC-001**: Users can log in within 3 seconds
`
	acceptanceContent := `# Acceptance Criteria: Auth System

- **AC-001**: Given valid credentials, When user submits login form, Then user is authenticated
`
	if err := writer.WriteArtifact(f.ID, feature.ArtifactSpecMD, []byte(specContent)); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteArtifact(f.ID, feature.ArtifactAcceptanceMD, []byte(acceptanceContent)); err != nil {
		t.Fatal(err)
	}

	f.AdvanceTo(feature.PhaseInception)
	f.AdvanceTo(feature.PhasePlanning)
	f.AdvanceTo(feature.PhaseConstruction)
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}

	detector := NewConvergenceDetector(provider)
	result, err := detector.Check(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Drifted {
		t.Errorf("expected no drift for refined spec, but got %d findings", len(result.Findings))
		for _, finding := range result.Findings {
			t.Logf("  Finding: %s - %s (severity: %s)", finding.Area, finding.Actual, finding.Severity)
		}
	}
}