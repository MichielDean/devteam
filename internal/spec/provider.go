package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/MichielDean/devteam/internal/feature"
)

type SpecProvider struct {
	baseDir string
}

func NewSpecProvider(baseDir string) *SpecProvider {
	return &SpecProvider{baseDir: baseDir}
}

func (sp *SpecProvider) BaseDir() string {
	return sp.baseDir
}

func (sp *SpecProvider) FeatureDir(featureID string) string {
	return filepath.Join(sp.baseDir, "specs", featureID)
}

func (sp *SpecProvider) LoadFeatureState(featureID string) (*feature.Feature, error) {
	statePath := filepath.Join(sp.FeatureDir(featureID), ".devteam-state.yaml")
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("feature %s not found: no state file at %s", featureID, statePath)
		}
		return nil, fmt.Errorf("reading feature state: %w", err)
	}
	var f feature.Feature
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing feature state: %w", err)
	}
	return &f, nil
}

func (sp *SpecProvider) SaveFeatureState(f *feature.Feature) error {
	dir := sp.FeatureDir(f.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating feature directory: %w", err)
	}
	statePath := filepath.Join(dir, ".devteam-state.yaml")
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshaling feature state: %w", err)
	}
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("writing feature state: %w", err)
	}
	return nil
}

func (sp *SpecProvider) ListFeatures() ([]*feature.Feature, error) {
	specsDir := filepath.Join(sp.baseDir, "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading specs directory: %w", err)
	}
	var features []*feature.Feature
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		f, err := sp.LoadFeatureState(entry.Name())
		if err != nil {
			continue
		}
		features = append(features, f)
	}
	return features, nil
}

func (sp *SpecProvider) ArtifactPath(featureID string, artType feature.ArtifactType) string {
	dir := sp.FeatureDir(featureID)
	switch artType {
	case feature.ArtifactInputMD:
		return filepath.Join(dir, "input.md")
	case feature.ArtifactSpecMD:
		return filepath.Join(dir, "spec.md")
	case feature.ArtifactAcceptanceMD:
		return filepath.Join(dir, "acceptance.md")
	case feature.ArtifactReposYAML:
		return filepath.Join(dir, "repos.yaml")
	case feature.ArtifactPlanMD:
		return filepath.Join(dir, "plan.md")
	case feature.ArtifactTasksMD:
		return filepath.Join(dir, "tasks.md")
	case feature.ArtifactReviewReport:
		return filepath.Join(dir, "review-report.md")
	case feature.ArtifactTestReport:
		return filepath.Join(dir, "test-report.md")
	case feature.ArtifactDocs:
		return filepath.Join(dir, "docs")
	case feature.ArtifactDataModelMD:
		return filepath.Join(dir, "data-model.md")
	case feature.ArtifactQuickstartMD:
		return filepath.Join(dir, "quickstart.md")
	case feature.ArtifactContractsDir:
		return filepath.Join(dir, "contracts")
	default:
		return filepath.Join(dir, string(artType))
	}
}

func (sp *SpecProvider) ArtifactExists(featureID string, artType feature.ArtifactType) bool {
	path := sp.ArtifactPath(featureID, artType)
	_, err := os.Stat(path)
	return err == nil
}

func (sp *SpecProvider) ValidateArtifacts(featureID string, requiredArts []feature.ArtifactType) feature.GateResult {
	result := feature.GateResult{
		Phase:       sp.currentPhase(featureID),
		Passed:      true,
		EvaluatedAt: time.Now(),
	}
	for _, art := range requiredArts {
		exists := sp.ArtifactExists(featureID, art)
		if !exists {
			result.Passed = false
			result.MissingArts = append(result.MissingArts, string(art))
		}
		result.Checks = append(result.Checks, feature.CheckResult{
			Name:   fmt.Sprintf("artifact_%s_exists", art),
			Passed: exists,
			Message: func() string {
				if exists {
					return fmt.Sprintf("artifact %s present at %s", art, sp.ArtifactPath(featureID, art))
				}
				return fmt.Sprintf("artifact %s missing (expected at %s)", art, sp.ArtifactPath(featureID, art))
			}(),
		})
	}
	return result
}

func (sp *SpecProvider) BuildCrossRepoContext(featureID string, repoNames []string) (string, error) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("=== Feature: %s ===\n\n", featureID))

	specContent, err := sp.ReadArtifact(featureID, feature.ArtifactSpecMD)
	if err == nil {
		b.WriteString("=== spec.md ===\n")
		b.WriteString(specContent)
		b.WriteString("\n\n")
	}

	acceptanceContent, err := sp.ReadArtifact(featureID, feature.ArtifactAcceptanceMD)
	if err == nil {
		b.WriteString("=== acceptance.md ===\n")
		b.WriteString(acceptanceContent)
		b.WriteString("\n\n")
	}

	planContent, err := sp.ReadArtifact(featureID, feature.ArtifactPlanMD)
	if err == nil {
		b.WriteString("=== plan.md ===\n")
		b.WriteString(planContent)
		b.WriteString("\n\n")
	}

	if len(repoNames) > 0 {
		b.WriteString("=== Affected Repositories ===\n")
		for _, name := range repoNames {
			b.WriteString(fmt.Sprintf("- %s\n", name))
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func (sp *SpecProvider) ReadArtifact(featureID string, artType feature.ArtifactType) (string, error) {
	path := sp.ArtifactPath(featureID, artType)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (sp *SpecProvider) currentPhase(featureID string) feature.Phase {
	f, err := sp.LoadFeatureState(featureID)
	if err != nil {
		return feature.PhaseInception
	}
	return f.CurrentPhase()
}
