package pipeline

import (
	"path/filepath"
	"testing"

	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/spec"
)

func TestEvaluateGate_MissingArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	provider := spec.NewSpecProvider(tmpDir)
	writer := spec.NewSpecWriter(tmpDir)

	f := feature.NewFeature("001-test", "Test", 2, feature.IntakeLooseIdea)
	if err := writer.CreateFeatureDir(f.ID); err != nil {
		t.Fatal(err)
	}
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}

	gateResult := provider.ValidateArtifacts(f.ID, feature.RequiredArtifactsForPhase(feature.PhaseInception))
	if gateResult.Passed {
		t.Error("expected gate to fail when artifacts are missing")
	}
	if len(gateResult.MissingArts) != 3 {
		t.Errorf("expected 3 missing artifacts, got %d", len(gateResult.MissingArts))
	}
}

func TestEvaluateGate_WithArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	provider := spec.NewSpecProvider(tmpDir)
	writer := spec.NewSpecWriter(tmpDir)

	f := feature.NewFeature("001-test", "Test", 2, feature.IntakeLooseIdea)
	if err := writer.CreateFeatureDir(f.ID); err != nil {
		t.Fatal(err)
	}
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}

	requiredArts := feature.RequiredArtifactsForPhase(feature.PhaseInception)
	for _, art := range requiredArts {
		if err := writer.WriteArtifact(f.ID, art, []byte("# Test content")); err != nil {
			t.Fatal(err)
		}
	}

	gateResult := provider.ValidateArtifacts(f.ID, requiredArts)
	if !gateResult.Passed {
		t.Errorf("expected gate to pass when all artifacts exist, but it failed: missing=%v", gateResult.MissingArts)
	}
}

func TestGateDefinitions(t *testing.T) {
	gd := feature.GetGateDefinition(feature.PhaseInception)
	if gd == nil {
		t.Fatal("expected gate definition for inception")
	}
	if gd.GateName != feature.GateSpecApproved {
		t.Errorf("expected gate spec_approved, got %s", gd.GateName)
	}
	if len(gd.RequiredArts) != 3 {
		t.Errorf("expected 3 required arts, got %d", len(gd.RequiredArts))
	}

	gd = feature.GetGateDefinition(feature.PhasePlanning)
	if gd == nil {
		t.Fatal("expected gate definition for planning")
	}
	if gd.GateName != feature.GatePlanApproved {
		t.Errorf("expected gate plan_approved, got %s", gd.GateName)
	}
}

func TestPipelineRunPhase(t *testing.T) {
	tmpDir := t.TempDir()
	provider := spec.NewSpecProvider(filepath.Join(tmpDir, "devteam"))
	writer := spec.NewSpecWriter(filepath.Join(tmpDir, "devteam"))

	f := feature.NewFeature("001-pipeline-test", "Pipeline Test", 2, feature.IntakeLooseIdea)
	if err := writer.CreateFeatureDir(f.ID); err != nil {
		t.Fatal(err)
	}

	// Advance to inception first
	f.AdvanceTo(feature.PhaseInception)
	if err := provider.SaveFeatureState(f); err != nil {
		t.Fatal(err)
	}

	// Verify the feature is in inception
	loaded, err := provider.LoadFeatureState(f.ID)
	if err != nil {
		t.Fatalf("LoadFeatureState() error: %v", err)
	}
	if loaded.PhaseStates[feature.PhaseInception].Status != feature.StatusInProgress {
		t.Errorf("expected inception to be in_progress, got %s", loaded.PhaseStates[feature.PhaseInception].Status)
	}
}