# Spec Provider API Contract

**Feature**: 001-dev-team-platform

## Core Operations

### Load Feature State

```go
func (sp *SpecProvider) LoadFeatureState(featureID string) (*feature.Feature, error)
```

Loads the feature's state from `specs/<id>/.devteam-state.yaml`. Returns the full feature struct with phase states, artifacts, and gate results.

### Save Feature State

```go
func (sp *SpecProvider) SaveFeatureState(f *feature.Feature) error
```

Persists the feature's state to `specs/<id>/.devteam-state.yaml`. Creates the directory if it doesn't exist.

### List Features

```go
func (sp *SpecProvider) ListFeatures() ([]*feature.Feature, error)
```

Scans the `specs/` directory for all features with state files. Returns all features in any phase.

### Artifact Resolution

```go
func (sp *SpecProvider) ArtifactPath(featureID string, artType feature.ArtifactType) string
```

Returns the expected file path for a given artifact type within a feature's spec directory.

```go
func (sp *SpecProvider) ArtifactExists(featureID string, artType feature.ArtifactType) bool
```

Checks whether an artifact file exists on disk.

```go
func (sp *SpecProvider) ReadArtifact(featureID string, artType feature.ArtifactType) (string, error)
```

Reads the content of an artifact file. Returns the markdown/yaml content as a string.

### Gate Evaluation

```go
func (sp *SpecProvider) ValidateArtifacts(featureID string, requiredArts []feature.ArtifactType) feature.GateResult
```

Validates that all required artifacts for a phase gate exist. Returns a GateResult with pass/fail status, missing artifacts list, and per-artifact checks.

### Cross-Repo Context

```go
func (sp *SpecProvider) BuildCrossRepoContext(featureID string, repoNames []string) (string, error)
```

Builds a context string for cross-repo features that includes:
- The spec.md content
- The acceptance.md content
- The plan.md content
- A list of affected repositories

This context is injected into the agent invocation when working across multiple repos.