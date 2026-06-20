package api

import (
	"testing"
	"time"

	"github.com/MichielDean/devteam/internal/feature"
)

func TestFeatureToDetailResponse(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-1 * time.Hour)

	f := &feature.Feature{
		ID:         "001-test-feature",
		Title:      "Test Feature",
		Current:    feature.PhasePlanning,
		Status:     feature.StatusInProgress,
		Priority:   1,
		IntakePath: feature.IntakeLooseIdea,
		CreatedAt:  now.Add(-24 * time.Hour),
		UpdatedAt:  now,
		Dependencies: []string{"dep1", "dep2"},
		Repos: []feature.RepoRef{
			{Name: "test-repo", URL: "https://github.com/test/repo", Branch: "main"},
		},
		PhaseStates: map[feature.Phase]*feature.PhaseState{
			feature.PhaseInception: {
				Phase:       feature.PhaseInception,
				Status:      feature.StatusPassed,
				StartedAt:   &startedAt,
				CompletedAt: &now,
				Artifacts: []feature.Artifact{
					{Type: feature.ArtifactSpecMD, Path: "specs/001-test-feature/spec.md", GeneratedBy: feature.RolePM, GeneratedAt: now},
				},
				GateResult: &feature.GateResult{
					Phase:  feature.PhaseInception,
					Passed: true,
					Checks: []feature.CheckResult{
						{Name: "spec.md exists", Passed: true, Message: "Found spec.md"},
					},
				},
			},
			feature.PhasePlanning: {
				Phase:    feature.PhasePlanning,
				Status:   feature.StatusInProgress,
				StartedAt: &now,
			},
		},
	}

	resp := FeatureToDetailResponse(f)

	if resp.ID != "001-test-feature" {
		t.Errorf("expected ID '001-test-feature', got %q", resp.ID)
	}
	if resp.Title != "Test Feature" {
		t.Errorf("expected Title 'Test Feature', got %q", resp.Title)
	}
	if resp.Status != "in_progress" {
		t.Errorf("expected Status 'in_progress', got %q", resp.Status)
	}
	if resp.Priority != 1 {
		t.Errorf("expected Priority 1, got %d", resp.Priority)
	}
	if resp.IntakePath != "loose_idea" {
		t.Errorf("expected IntakePath 'loose_idea', got %q", resp.IntakePath)
	}

	// Check phase states
	if len(resp.PhaseStates) != 2 {
		t.Errorf("expected 2 phase states, got %d", len(resp.PhaseStates))
	}

	inception, ok := resp.PhaseStates["inception"]
	if !ok {
		t.Fatal("expected 'inception' phase state")
	}
	if inception.Status != "passed" {
		t.Errorf("expected inception status 'passed', got %q", inception.Status)
	}
	if inception.GateResult == nil || !inception.GateResult.Passed {
		t.Error("expected inception gate result to be passed")
	}
	if len(inception.Artifacts) != 1 {
		t.Errorf("expected 1 inception artifact, got %d", len(inception.Artifacts))
	}
	if inception.Artifacts[0].Type != "spec_md" {
		t.Errorf("expected artifact type 'spec_md', got %q", inception.Artifacts[0].Type)
	}

	planning, ok := resp.PhaseStates["planning"]
	if !ok {
		t.Fatal("expected 'planning' phase state")
	}
	if planning.Status != "in_progress" {
		t.Errorf("expected planning status 'in_progress', got %q", planning.Status)
	}
	if planning.GateResult != nil {
		t.Error("expected planning gate result to be nil")
	}

	// Check repos
	if len(resp.Repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(resp.Repos))
	}
	if resp.Repos[0].Name != "test-repo" {
		t.Errorf("expected repo name 'test-repo', got %q", resp.Repos[0].Name)
	}

	// Check dependencies
	if len(resp.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(resp.Dependencies))
	}
}

func TestFeatureToSummaryResponse(t *testing.T) {
	now := time.Now()

	f := &feature.Feature{
		ID:         "002-another",
		Title:      "Another Feature",
		Current:    feature.PhaseReview,
		Status:     feature.StatusGateBlocked,
		Priority:   3,
		UpdatedAt:  now,
		PhaseStates: map[feature.Phase]*feature.PhaseState{
			feature.PhaseReview: {
				Phase:  feature.PhaseReview,
				Status: feature.StatusGateBlocked,
				GateResult: &feature.GateResult{
					Phase:  feature.PhaseReview,
					Passed: false,
					Checks: []feature.CheckResult{
						{Name: "review_report exists", Passed: false, Message: "Missing review_report"},
					},
				},
			},
		},
	}

	summary := FeatureToSummaryResponse(f)

	if summary.ID != "002-another" {
		t.Errorf("expected ID '002-another', got %q", summary.ID)
	}
	if summary.Title != "Another Feature" {
		t.Errorf("expected Title 'Another Feature', got %q", summary.Title)
	}
	if summary.Status != "gate_blocked" {
		t.Errorf("expected Status 'gate_blocked', got %q", summary.Status)
	}
	if summary.Priority != 3 {
		t.Errorf("expected Priority 3, got %d", summary.Priority)
	}
	if summary.CurrentPhase != "review" {
		t.Errorf("expected CurrentPhase 'review', got %q", summary.CurrentPhase)
	}
	if summary.GateResult == nil {
		t.Error("expected GateResult to be non-nil")
	}
	if summary.GateResult.Passed {
		t.Error("expected GateResult.Passed to be false")
	}
}

func TestFeaturesToSummaryResponse(t *testing.T) {
	features := []*feature.Feature{
		{ID: "001-a", Title: "A", Current: feature.PhaseInception, Status: feature.StatusInProgress, Priority: 1, UpdatedAt: time.Now()},
		{ID: "002-b", Title: "B", Current: feature.PhaseDelivery, Status: feature.StatusDone, Priority: 2, UpdatedAt: time.Now()},
	}

	resp := FeaturesToSummaryResponse(features)

	if len(resp.Features) != 2 {
		t.Errorf("expected 2 features, got %d", len(resp.Features))
	}
	if resp.Features[0].ID != "001-a" {
		t.Errorf("expected first feature ID '001-a', got %q", resp.Features[0].ID)
	}
}

func TestGateResultToResponse(t *testing.T) {
	gr := &feature.GateResult{
		Phase:  feature.PhaseInception,
		Passed: true,
		Checks: []feature.CheckResult{
			{Name: "spec.md exists", Passed: true, Message: "Found spec.md"},
			{Name: "acceptance.md exists", Passed: true, Message: "Found acceptance.md"},
		},
	}

	resp := GateResultToResponse(gr)

	if resp.Phase != "inception" {
		t.Errorf("expected Phase 'inception', got %q", resp.Phase)
	}
	if !resp.Passed {
		t.Error("expected Passed to be true")
	}
	if len(resp.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(resp.Checks))
	}
	if resp.Checks[0].Name != "spec.md exists" {
		t.Errorf("expected check name 'spec.md exists', got %q", resp.Checks[0].Name)
	}
}

func TestGateResultToResponseNil(t *testing.T) {
	resp := GateResultToResponse(nil)
	if resp.Passed {
		t.Error("expected Passed to be false for nil gate result")
	}
}

func TestFeatureToDetailResponseWithNilFields(t *testing.T) {
	f := &feature.Feature{
		ID:         "003-minimal",
		Title:      "Minimal",
		Current:    feature.PhaseInception,
		Status:     feature.StatusDraft,
		Priority:   2,
		IntakePath: feature.IntakeLooseIdea,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	resp := FeatureToDetailResponse(f)

	// Nil slices should be empty slices, not null
	if resp.Dependencies == nil {
		t.Error("expected Dependencies to be non-nil empty slice")
	}
	if len(resp.Dependencies) != 0 {
		t.Errorf("expected 0 dependencies, got %d", len(resp.Dependencies))
	}
	if resp.Repos == nil {
		t.Error("expected Repos to be non-nil empty slice")
	}
	if len(resp.Repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(resp.Repos))
	}
}

func TestFeatureToSummaryResponseNoGateResult(t *testing.T) {
	f := &feature.Feature{
		ID:        "004-no-gate",
		Title:     "No Gate",
		Current:   feature.PhaseInception,
		Status:    feature.StatusDraft,
		Priority:  2,
		UpdatedAt: time.Now(),
		PhaseStates: map[feature.Phase]*feature.PhaseState{
			feature.PhaseInception: {
				Phase:  feature.PhaseInception,
				Status: feature.StatusDraft,
			},
		},
	}

	summary := FeatureToSummaryResponse(f)
	if summary.GateResult != nil {
		t.Error("expected GateResult to be nil when no gate result exists for current phase")
	}
}