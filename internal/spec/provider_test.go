package spec

import (
	"path/filepath"
	"testing"

	"github.com/MichielDean/devteam/internal/feature"
)

func TestSpecProviderFeatureDir(t *testing.T) {
	sp := NewSpecProvider("/tmp/test-devteam")
	got := sp.FeatureDir("001-test-feature")
	expected := filepath.Join("/tmp/test-devteam", "specs", "001-test-feature")
	if got != expected {
		t.Errorf("FeatureDir() = %s, want %s", got, expected)
	}
}

func TestSpecProviderArtifactPath(t *testing.T) {
	sp := NewSpecProvider("/tmp/test-devteam")
	tests := []struct {
		artType feature.ArtifactType
		suffix  string
	}{
		{feature.ArtifactSpecMD, "spec.md"},
		{feature.ArtifactAcceptanceMD, "acceptance.md"},
		{feature.ArtifactReposYAML, "repos.yaml"},
		{feature.ArtifactPlanMD, "plan.md"},
		{feature.ArtifactTasksMD, "tasks.md"},
	}
	for _, tt := range tests {
		got := sp.ArtifactPath("001-test", tt.artType)
		expected := filepath.Join("/tmp/test-devteam", "specs", "001-test", tt.suffix)
		if got != expected {
			t.Errorf("ArtifactPath(%s) = %s, want %s", tt.artType, got, expected)
		}
	}
}

func TestSpecProviderSaveAndLoad(t *testing.T) {
	sp, _ := newTestProvider(t)

	f := feature.NewFeature("001-test-feature", "Test Feature", 2, feature.IntakeLooseIdea)
	if err := sp.SaveFeatureState(f); err != nil {
		t.Fatalf("SaveFeatureState() error: %v", err)
	}

	loaded, err := sp.LoadFeatureState(f.ID)
	if err != nil {
		t.Fatalf("LoadFeatureState() error: %v", err)
	}
	if loaded.ID != f.ID {
		t.Errorf("loaded ID = %s, want %s", loaded.ID, f.ID)
	}
	if loaded.Title != f.Title {
		t.Errorf("loaded Title = %s, want %s", loaded.Title, f.Title)
	}
	if loaded.Status != f.Status {
		t.Errorf("loaded Status = %s, want %s", loaded.Status, f.Status)
	}
}

func TestSpecProviderArtifactExists(t *testing.T) {
	sw, sp, _ := newTestWriter(t)

	fid := "001-artifact-test"
	f := feature.NewFeature(fid, "Artifact Test", 2, feature.IntakeLooseIdea)
	if err := sp.SaveFeatureState(f); err != nil {
		t.Fatalf("SaveFeatureState: %v", err)
	}

	if sp.ArtifactExists(fid, feature.ArtifactSpecMD) {
		t.Error("ArtifactExists() = true for nonexistent artifact")
	}

	if err := sw.WriteArtifact(fid, feature.ArtifactSpecMD, []byte("# Test Spec")); err != nil {
		t.Fatalf("WriteArtifact() error: %v", err)
	}

	if !sp.ArtifactExists(fid, feature.ArtifactSpecMD) {
		t.Error("ArtifactExists() = false for existing artifact")
	}
}

func TestSpecWriterRecordArtifact(t *testing.T) {
	sw, sp, _ := newTestWriter(t)

	f := feature.NewFeature("001-record-test", "Record Test", 2, feature.IntakeLooseIdea)
	if err := sp.SaveFeatureState(f); err != nil {
		t.Fatalf("SaveFeatureState: %v", err)
	}

	if err := sw.WriteArtifact(f.ID, feature.ArtifactSpecMD, []byte("# Spec")); err != nil {
		t.Fatalf("WriteArtifact() error: %v", err)
	}
	if err := sw.RecordArtifact(f.ID, feature.ArtifactSpecMD, feature.RolePM); err != nil {
		t.Fatalf("RecordArtifact() error: %v", err)
	}

	loaded, err := sp.LoadFeatureState(f.ID)
	if err != nil {
		t.Fatalf("LoadFeatureState() error: %v", err)
	}
	phase := loaded.CurrentPhase()
	ps, ok := loaded.PhaseStates[phase]
	if !ok {
		t.Fatalf("no phase state for %s", phase)
	}
	if len(ps.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(ps.Artifacts))
	}
	if ps.Artifacts[0].Type != feature.ArtifactSpecMD {
		t.Errorf("artifact type = %s, want %s", ps.Artifacts[0].Type, feature.ArtifactSpecMD)
	}
	if ps.Artifacts[0].GeneratedBy != feature.RolePM {
		t.Errorf("artifact generated_by = %s, want %s", ps.Artifacts[0].GeneratedBy, feature.RolePM)
	}
}