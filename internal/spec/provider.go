package spec

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
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

// FeatureDir returns the spec directory for a feature. If the feature has
// a WorktreeDir set, it returns the spec dir inside the worktree. Otherwise
// it falls back to the base dir (primary checkout).
func (sp *SpecProvider) FeatureDir(featureID string) string {
	// Check if this feature has a worktree by loading its state
	statePath := filepath.Join(sp.baseDir, "specs", featureID, ".devteam-state.yaml")
	if data, err := os.ReadFile(statePath); err == nil {
		var f feature.Feature
		if err := yaml.Unmarshal(data, &f); err == nil && f.WorktreeDir != "" {
			return filepath.Join(f.WorktreeDir, "specs", featureID)
		}
	}
	return filepath.Join(sp.baseDir, "specs", featureID)
}

// FeatureDirFromFeature returns the spec directory using the feature's
// WorktreeDir if set. More efficient than FeatureDir when the feature
// is already loaded.
func (sp *SpecProvider) FeatureDirFromFeature(f *feature.Feature) string {
	if f.WorktreeDir != "" {
		return filepath.Join(f.WorktreeDir, "specs", f.ID)
	}
	return filepath.Join(sp.baseDir, "specs", f.ID)
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
	// Always save to the primary checkout so ListFeatures can find it
	primaryDir := filepath.Join(sp.baseDir, "specs", f.ID)
	if err := os.MkdirAll(primaryDir, 0755); err != nil {
		return fmt.Errorf("creating feature directory in primary checkout: %w", err)
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshaling feature state: %w", err)
	}
	primaryPath := filepath.Join(primaryDir, ".devteam-state.yaml")
	if err := os.WriteFile(primaryPath, data, 0644); err != nil {
		return fmt.Errorf("writing feature state to primary checkout: %w", err)
	}

	// Also save to the worktree if set
	if f.WorktreeDir != "" {
		wtDir := filepath.Join(f.WorktreeDir, "specs", f.ID)
		if err := os.MkdirAll(wtDir, 0755); err != nil {
			log.Printf("warning: could not create feature dir in worktree: %v", err)
			return nil // primary save succeeded
		}
		wtPath := filepath.Join(wtDir, ".devteam-state.yaml")
		if err := os.WriteFile(wtPath, data, 0644); err != nil {
			log.Printf("warning: could not write feature state to worktree: %v", err)
		}
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

func (sp *SpecProvider) ListFeaturesSorted() ([]*feature.Feature, error) {
	features, err := sp.ListFeatures()
	if err != nil {
		return nil, err
	}
	sort.Slice(features, func(i, j int) bool {
		return features[i].UpdatedAt.After(features[j].UpdatedAt)
	})
	return features, nil
}

func (sp *SpecProvider) ReadArtifactContent(featureID string, artType feature.ArtifactType) (string, error) {
	return sp.ReadArtifact(featureID, artType)
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
	case feature.ArtifactResearchMD:
		return filepath.Join(dir, "research.md")
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

	// If it's a directory (docs/, contracts/), read all files and concatenate
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return readDirectoryContents(path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// readDirectoryContents reads all .md files in a directory and concatenates them.
func readDirectoryContents(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		b.WriteString(fmt.Sprintf("\n## %s\n\n", name))
		b.Write(data)
		b.WriteString("\n")
	}

	if b.Len() == 0 {
		return "", fmt.Errorf("no readable files in directory %s", dir)
	}
	return b.String(), nil
}

func (sp *SpecProvider) currentPhase(featureID string) feature.Phase {
	f, err := sp.LoadFeatureState(featureID)
	if err != nil {
		return feature.PhaseInception
	}
	return f.CurrentPhase()
}

// LoadFeatureRepos reads the feature's repos.yaml and returns the declared
// RepoRefs. Returns an empty slice (not an error) if repos.yaml is absent
// — features that only touch the spec repo legitimately have no repos.yaml.
func (sp *SpecProvider) LoadFeatureRepos(featureID string) ([]feature.RepoRef, error) {
	path := sp.ArtifactPath(featureID, feature.ArtifactReposYAML)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading repos.yaml for %s: %w", featureID, err)
	}
	// repos.yaml uses the same shape as the global repos config but with
	// feature-specific fields (name, url, branch, scope). We only need
	// name+url+branch here.
	var parsed struct {
		Feature string            `yaml:"feature"`
		Repos   []feature.RepoRef `yaml:"repos"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parsing repos.yaml for %s: %w", featureID, err)
	}
	return parsed.Repos, nil
}
