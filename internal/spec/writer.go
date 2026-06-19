package spec

import (
	"os"
	"path/filepath"
	"time"

	"github.com/MichielDean/devteam/internal/feature"
)

type SpecWriter struct {
	baseDir string
}

func NewSpecWriter(baseDir string) *SpecWriter {
	return &SpecWriter{baseDir: baseDir}
}

func (sw *SpecWriter) CreateFeatureDir(featureID string) error {
	dir := filepath.Join(sw.baseDir, "specs", featureID)
	return os.MkdirAll(dir, 0755)
}

func (sw *SpecWriter) WriteArtifact(featureID string, artType feature.ArtifactType, content []byte) error {
	provider := NewSpecProvider(sw.baseDir)
	path := provider.ArtifactPath(featureID, artType)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}

func (sw *SpecWriter) RecordArtifact(featureID string, artType feature.ArtifactType, role feature.RoleName) error {
	provider := NewSpecProvider(sw.baseDir)
	f, err := provider.LoadFeatureState(featureID)
	if err != nil {
		return err
	}
	phase := f.CurrentPhase()
	ps, ok := f.PhaseStates[phase]
	if !ok {
		ps = &feature.PhaseState{Phase: phase, Status: feature.StatusInProgress}
		f.PhaseStates[phase] = ps
	}
	ps.Artifacts = append(ps.Artifacts, feature.Artifact{
		Type:        artType,
		Path:        provider.ArtifactPath(featureID, artType),
		GeneratedBy: role,
		GeneratedAt: time.Now(),
	})
	return provider.SaveFeatureState(f)
}