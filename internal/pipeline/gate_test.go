package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/spec"
)

func TestGateEvaluator_InceptionGate_MissingArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	provider := spec.NewSpecProvider(tmpDir)
	writer := spec.NewSpecWriter(tmpDir)

	f := feature.NewFeature("001-test", "Test Feature", 2, feature.IntakeLooseIdea)
	if err := writer.CreateFeatureDir(f.ID); err != nil {
		t.Fatal(err)
	}
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}

	evaluator := NewGateEvaluator(provider)
	result, err := evaluator.Evaluate(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Passed {
		t.Error("expected gate to fail when no artifacts exist")
	}
	if len(result.MissingArts) == 0 {
		t.Error("expected missing artifacts to be reported")
	}
}

func TestGateEvaluator_InceptionGate_AllArtifactsPresent(t *testing.T) {
	tmpDir := t.TempDir()
	provider := spec.NewSpecProvider(tmpDir)
	writer := spec.NewSpecWriter(tmpDir)

	f := feature.NewFeature("001-test", "Test Feature", 2, feature.IntakeLooseIdea)
	if err := writer.CreateFeatureDir(f.ID); err != nil {
		t.Fatal(err)
	}
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}

	for _, art := range feature.RequiredArtifactsForPhase(feature.PhaseInception) {
		content := []byte("# Spec\n\n## User Stories\n\n### User Story 1 - Create features (Priority: P1)\n\n- US-001: User can create features\n\n### User Story 2 - List features (Priority: P2)\n\n- US-002: User can list features\n\n## Functional Requirements\n\n- FR-001: System shall create features\n\n## Error Scenarios\n\n| Action | Success | Error | Response |\n|---|---|---|---|\n| Create | 201 | Missing title | 400 |\n\n## Empty State Behavior\n\n- GET /features returns 200 with []\n\n## Success Criteria\n\n- SC-001: Measurable outcome — 99% of create requests succeed within 200ms\n\n## Edge Cases\n\n- Empty input: returns 400\n- Duplicate id: returns 409\n\n## Assumptions\n\n- [ASSUMPTION: Single user system]\n\n## Constraint Register\n\n| ID | Source | Type | Constraint | Verification |\n|----|--------|------|------------|-------------|\n| CON-001 | RFC 9421 | correctness | Wire-format failures return Invalid | Negative vector test |")
		if art == feature.ArtifactReposYAML {
			content = []byte("feature: 001-test\nrepos:\n  - name: devteam\n    branch: feature/001-test")
		}
		if art == feature.ArtifactAcceptanceMD {
			content = []byte("# Acceptance Criteria\n\n- AC-001: Given a valid request, when creating a feature, then it returns 201\n  Test level: integration\n  Verification: POST /api/features with valid data returns 201\n- AC-CON-001: Given malformed input, when processed, then returns Invalid\n  Source: CON-001 (RFC 9421)")
		}
		if err := writer.WriteArtifact(f.ID, art, content); err != nil {
			t.Fatal(err)
		}
	}

	evaluator := NewGateEvaluator(provider)
	result, err := evaluator.Evaluate(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Passed {
		t.Errorf("expected gate to pass when all artifacts exist; missing=%v", result.MissingArts)
		for _, check := range result.Checks {
			if !check.Passed {
				t.Logf("  Failed check: %s - %s", check.Name, check.Message)
			}
		}
	}
}

func TestGateEvaluator_PlanningGate(t *testing.T) {
	tmpDir := t.TempDir()
	provider := spec.NewSpecProvider(tmpDir)
	writer := spec.NewSpecWriter(tmpDir)

	f := feature.NewFeature("001-test", "Test Feature", 2, feature.IntakeLooseIdea)
	if err := writer.CreateFeatureDir(f.ID); err != nil {
		t.Fatal(err)
	}

	f.AdvanceTo(feature.PhaseInception)
	f.AdvanceTo(feature.PhasePlanning)

	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}

	planContent := `# Plan

## Technical Context

Language: Go. Framework: net/http. Dependencies: standard library.
Storage: in-memory. Testing: go test. Platform: linux.

## Implementation Approach

This plan addresses all acceptance criteria with detailed file paths.

## Component Design

- API Server: Handles HTTP requests, manages feature lifecycle
- Store: Persistence layer for features
- Component interfaces: HTTP handlers, Store interface

## Data Model

- Feature: id, title, priority, current_phase, status, created_at, updated_at
- State transitions: draft → inception → planning → construction → review → testing → delivery

## API Contracts

- GET /api/features → 200 [{feature}]
- POST /api/features → 201 {feature} | 400 {error, details}

## Test Strategy

Each component requires specific testing levels:
- Smoke: Verify service starts and responds to HTTP requests
- Integration: Test full request/response cycles through real endpoints
- Unit: Test business logic in isolation

## Agent Failure Mode Checks

- Nil pointer ordering verified for initialization code
- JSON arrays are [] not null for collection fields
- Recovery middleware is first in chain

## Constraint Verification Map

| CON-ID | Design Decision | Component | Verification |
|--------|-----------------|-----------|-------------|
| CON-001 | All parse failures caught | Verifier | Negative vector test |

## Cross-Component Consistency Matrix

| Shared Value | Producer | Consumer | Consistent |
|--------------|----------|----------|-----------|
| Algorithm IDs | All providers | Verifier | YES |

## Dependencies

Tasks depend on each other as specified.
`
	tasksContent := `# Tasks

## User Story 1 (P1)

## T001 [P] Setup - Create project structure

- [ ] T001 Create project files
- [ ] T002 Implement core logic

Done conditions for T001:
- Verify: service starts without panicking
- Verify: GET /api/features returns 200 with empty list

Constraints: CON-001
Test level: smoke, integration

## Dependencies

T002 depends on T001.
`
	if err := writer.WriteArtifact(f.ID, feature.ArtifactPlanMD, []byte(planContent)); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteArtifact(f.ID, feature.ArtifactTasksMD, []byte(tasksContent)); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteArtifact(f.ID, feature.ArtifactResearchMD, []byte("# Research\n\nExisting code patterns analyzed. Library choices: stdlib only. Alternatives rejected: none needed.")); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteArtifact(f.ID, feature.ArtifactDataModelMD, []byte("# Data Model\n\n## Entities\n\n### Feature\n- Attributes: id, title, priority\n- Relationships: none\n- Constraints: unique id")); err != nil {
		t.Fatal(err)
	}
	// contracts/ is a directory — WriteArtifact writes a file at the path, so
	// create the directory and a contract file inside it directly.
	contractsDir := filepath.Join(tmpDir, "specs", f.ID, "contracts")
	if err := os.MkdirAll(contractsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contractsDir, "GET-api-features.md"), []byte("# GET /api/features\n\nResponse 200: [{feature}]"), 0644); err != nil {
		t.Fatal(err)
	}

	evaluator := NewGateEvaluator(provider)
	result, err := evaluator.Evaluate(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Passed {
		t.Errorf("expected planning gate to pass; missing=%v", result.MissingArts)
		for _, check := range result.Checks {
			if !check.Passed {
				t.Logf("  Failed check: %s - %s", check.Name, check.Message)
			}
		}
	}
}

func TestGateEvaluator_AdvanceFeature(t *testing.T) {
	tmpDir := t.TempDir()
	provider := spec.NewSpecProvider(tmpDir)
	writer := spec.NewSpecWriter(tmpDir)

	f := feature.NewFeature("001-test", "Test Feature", 2, feature.IntakeLooseIdea)
	if err := writer.CreateFeatureDir(f.ID); err != nil {
		t.Fatal(err)
	}

	for _, art := range feature.RequiredArtifactsForPhase(feature.PhaseInception) {
		content := []byte("# Spec\n\n## User Stories\n\n### User Story 1 - Create features (Priority: P1)\n\n- US-001: User can create features\n\n### User Story 2 - List features (Priority: P2)\n\n- US-002: User can list features\n\n## Functional Requirements\n\n- FR-001: System shall create features\n\n## Error Scenarios\n\n| Action | Success | Error | Response |\n|---|---|---|---|\n| Create | 201 | Missing title | 400 |\n\n## Empty State Behavior\n\n- GET /features returns 200 with []\n\n## Success Criteria\n\n- SC-001: Measurable outcome — 99% of create requests succeed within 200ms\n\n## Edge Cases\n\n- Empty input: returns 400\n- Duplicate id: returns 409\n\n## Assumptions\n\n- [ASSUMPTION: Single user system]\n\n## Constraint Register\n\n| ID | Source | Type | Constraint | Verification |\n|----|--------|------|------------|-------------|\n| CON-001 | RFC 9421 | correctness | Wire-format failures return Invalid | Negative vector test |")
		if art == feature.ArtifactReposYAML {
			content = []byte("feature: 001-test\nrepos:\n  - name: devteam\n    branch: feature/001-test")
		}
		if art == feature.ArtifactAcceptanceMD {
			content = []byte("# Acceptance Criteria\n\n- AC-001: Given a valid request, when creating a feature, then it returns 201\n  Test level: integration\n  Verification: POST /api/features with valid data returns 201\n- AC-CON-001: Given malformed input, when processed, then returns Invalid\n  Source: CON-001 (RFC 9421)")
		}
		if err := writer.WriteArtifact(f.ID, art, content); err != nil {
			t.Fatal(err)
		}
	}

	f.AdvanceTo(feature.PhaseInception)
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}

	evaluator := NewGateEvaluator(provider)
	result, err := evaluator.Evaluate(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected inception gate to pass for advancement test; missing=%v", result.MissingArts)
	}

	f.AdvanceTo(feature.PhasePlanning)
	if f.CurrentPhase() != feature.PhasePlanning {
		t.Errorf("expected phase to be planning, got %s", f.CurrentPhase())
	}
}

func TestGateEvaluator_RecirculateFeature(t *testing.T) {
	f := feature.NewFeature("001-test", "Test Feature", 2, feature.IntakeLooseIdea)

	f.AdvanceTo(feature.PhaseInception)
	f.AdvanceTo(feature.PhasePlanning)
	f.AdvanceTo(feature.PhaseConstruction)

	if f.CurrentPhase() != feature.PhaseConstruction {
		t.Fatalf("expected construction, got %s", f.CurrentPhase())
	}

	err := f.RecirculateTo(feature.PhasePlanning)
	if err != nil {
		t.Fatalf("unexpected error recirculating: %v", err)
	}

	if f.CurrentPhase() != feature.PhasePlanning {
		t.Errorf("expected planning after recirculation, got %s", f.CurrentPhase())
	}
	if f.Status != feature.StatusRecirculated {
		t.Errorf("expected recirculated status, got %s", f.Status)
	}
}

func TestPipeline_AdvanceAndRecirculate(t *testing.T) {
	tmpDir := t.TempDir()
	provider := spec.NewSpecProvider(filepath.Join(tmpDir, "devteam"))
	writer := spec.NewSpecWriter(filepath.Join(tmpDir, "devteam"))

	f := feature.NewFeature("001-advance-test", "Advance Test", 2, feature.IntakeLooseIdea)
	if err := writer.CreateFeatureDir(f.ID); err != nil {
		t.Fatal(err)
	}

	f.AdvanceTo(feature.PhaseInception)
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}

	p := NewPipeline(nil, provider)

	advanced, err := p.AdvanceFeature(f)
	if err != nil {
		t.Fatalf("unexpected error advancing: %v", err)
	}
	if advanced.CurrentPhase() != feature.PhasePlanning {
		t.Errorf("expected planning after advance, got %s", advanced.CurrentPhase())
	}

	advanced, err = p.RecirculateFeature(f, feature.PhaseInception, "test recirculation")
	if err != nil {
		t.Fatalf("unexpected error recirculating: %v", err)
	}
	if advanced.CurrentPhase() != feature.PhaseInception {
		t.Errorf("expected inception after recirculation, got %s", advanced.CurrentPhase())
	}
}