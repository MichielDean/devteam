package spec

import (
	"fmt"
	"time"

	"github.com/MichielDean/devteam/internal/feature"
)

type SpecWriter struct {
	baseDir   string
	provider  *SpecProvider
}

func NewSpecWriter(baseDir string) *SpecWriter {
	return &SpecWriter{
		baseDir:  baseDir,
		provider: NewSpecProvider(baseDir),
	}
}

// SetDatabase wires the database for artifact writes.
func (sw *SpecWriter) SetDatabase(database interface{}) {
	if db, ok := database.(interface {
		SaveArtifact(string, string, string) error
	}); ok {
		_ = db // provider handles DB via its own SetDatabase
	}
}

// SetProvider links the writer to a SpecProvider that already has a DB.
// This avoids creating a separate provider without DB access.
func (sw *SpecWriter) SetProvider(provider *SpecProvider) {
	sw.provider = provider
}

// CreateFeatureDir is a no-op when DB-backed. Features live in the DB.
func (sw *SpecWriter) CreateFeatureDir(featureID string) error {
	return nil
}

// WriteArtifact writes an artifact to the DB (via the provider).
func (sw *SpecWriter) WriteArtifact(featureID string, artType feature.ArtifactType, content []byte) error {
	return sw.provider.SaveArtifactBytes(featureID, artType, content)
}

// RecordArtifact records that an artifact was generated in the feature's phase state.
func (sw *SpecWriter) RecordArtifact(featureID string, artType feature.ArtifactType, role feature.RoleName) error {
	f, err := sw.provider.LoadFeatureState(featureID)
	if err != nil {
		return fmt.Errorf("loading feature for RecordArtifact: %w", err)
	}
	phase := f.CurrentPhase()
	ps, ok := f.PhaseStates[phase]
	if !ok {
		ps = &feature.PhaseState{Phase: phase, Status: feature.StatusInProgress}
		f.PhaseStates[phase] = ps
	}
	ps.Artifacts = append(ps.Artifacts, feature.Artifact{
		Type:        artType,
		Path:        "", // no disk path — DB-backed
		GeneratedBy: role,
		GeneratedAt: time.Now(),
	})
	return sw.provider.SaveFeatureState(f)
}