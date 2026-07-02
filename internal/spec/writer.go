package spec

import (
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