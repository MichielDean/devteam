package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
)

type SpecProvider struct {
	baseDir   string
	database  *db.DB
}

func NewSpecProvider(baseDir string) *SpecProvider {
	return &SpecProvider{baseDir: baseDir}
}

// SetDatabase wires the database for state and artifact storage.
// Once set, all reads/writes go through the DB — no disk for spec artifacts or state.
func (sp *SpecProvider) SetDatabase(database *db.DB) {
	sp.database = database
}

func (sp *SpecProvider) BaseDir() string {
	return sp.baseDir
}

// FeatureDir returns the worktree spec directory for a feature.
// This is used for ephemeral files (CONTEXT.md, outcome.txt, etc.) that
// agents read/write during a run. Spec artifacts themselves live in the DB.
func (sp *SpecProvider) FeatureDir(featureID string) string {
	if sp.database != nil {
		f, err := sp.LoadFeatureState(featureID)
		if err == nil && f.WorktreeDir != "" {
			return filepath.Join(f.WorktreeDir, "specs", featureID)
		}
	}
	return filepath.Join(sp.baseDir, "specs", featureID)
}

// FeatureDirFromFeature returns the spec directory using the feature's WorktreeDir.
func (sp *SpecProvider) FeatureDirFromFeature(f *feature.Feature) string {
	if f.WorktreeDir != "" {
		return filepath.Join(f.WorktreeDir, "specs", f.ID)
	}
	return filepath.Join(sp.baseDir, "specs", f.ID)
}

// artifactTypeToDBKey maps ArtifactType to the string key used in spec_artifacts table.
func artifactTypeToDBKey(artType feature.ArtifactType) string {
	return string(artType)
}

// SaveArtifact writes an artifact to the DB.
func (sp *SpecProvider) SaveArtifact(featureID string, artType feature.ArtifactType, content string) error {
	if sp.database == nil {
		return fmt.Errorf("database not configured")
	}
	return sp.database.SaveArtifact(featureID, artifactTypeToDBKey(artType), content)
}

// SaveArtifactBytes writes artifact bytes to the DB.
func (sp *SpecProvider) SaveArtifactBytes(featureID string, artType feature.ArtifactType, content []byte) error {
	return sp.SaveArtifact(featureID, artType, string(content))
}

func (sp *SpecProvider) LoadFeatureState(featureID string) (*feature.Feature, error) {
	if sp.database != nil {
		data, err := sp.database.LoadFeatureData(featureID)
		if err != nil {
			return nil, err
		}
		var f feature.Feature
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("parsing feature state for %s: %w", featureID, err)
		}
		return &f, nil
	}
	return sp.loadFeatureStateFromDisk(featureID)
}

func (sp *SpecProvider) loadFeatureStateFromDisk(featureID string) (*feature.Feature, error) {
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
	if sp.database != nil {
		data, err := json.Marshal(f)
		if err != nil {
			return fmt.Errorf("marshaling feature state: %w", err)
		}
		return sp.database.SaveFeatureData(f.ID, f.Title, string(f.Current), string(f.Status), f.Priority, string(f.IntakePath), f.SpecDir, f.WorktreeDir, f.CreatedAt, 0, data)
	}
	return sp.saveFeatureStateToDisk(f)
}

func (sp *SpecProvider) saveFeatureStateToDisk(f *feature.Feature) error {
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshaling feature state: %w", err)
	}
	dir := sp.FeatureDirFromFeature(f)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating feature dir: %w", err)
	}
	statePath := filepath.Join(dir, ".devteam-state.yaml")
	return os.WriteFile(statePath, data, 0644)
}

func (sp *SpecProvider) ListFeatures() ([]*feature.Feature, error) {
	if sp.database != nil {
		rawFeatures, err := sp.database.ListAllFeatureData()
		if err != nil {
			return nil, err
		}
		var features []*feature.Feature
		for _, raw := range rawFeatures {
			var f feature.Feature
			if err := json.Unmarshal(raw, &f); err != nil {
				continue
			}
			features = append(features, &f)
		}
		return features, nil
	}
	return sp.listFeaturesFromDisk()
}

func (sp *SpecProvider) listFeaturesFromDisk() ([]*feature.Feature, error) {
	var features []*feature.Feature
	seen := make(map[string]bool)

	worktreeBase := filepath.Join(os.Getenv("HOME"), "worktrees", "devteam-specs")
	if wtEntries, err := os.ReadDir(worktreeBase); err == nil {
		for _, entry := range wtEntries {
			if !entry.IsDir() {
				continue
			}
			specDir := filepath.Join(worktreeBase, entry.Name(), "specs", entry.Name())
			statePath := filepath.Join(specDir, ".devteam-state.yaml")
			if data, err := os.ReadFile(statePath); err == nil {
				var f feature.Feature
				if err := yaml.Unmarshal(data, &f); err == nil {
					if strings.Contains(f.SpecDir, sp.baseDir) || f.SpecDir == "" {
						features = append(features, &f)
						seen[f.ID] = true
					}
				}
			}
		}
	}

	specsDir := filepath.Join(sp.baseDir, "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		if len(features) == 0 {
			return nil, fmt.Errorf("reading specs directory: %w", err)
		}
		return features, nil
	}
	for _, entry := range entries {
		if !entry.IsDir() || seen[entry.Name()] {
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

// ArtifactPath returns the on-disk path for backward compatibility (tests, aux files).
// Spec artifacts live in the DB — this is only for ephemeral files.
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
	if sp.database != nil {
		_, err := sp.database.GetArtifact(featureID, artifactTypeToDBKey(artType))
		return err == nil
	}
	_, err := os.Stat(sp.ArtifactPath(featureID, artType))
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
					return fmt.Sprintf("artifact %s present", art)
				}
				return fmt.Sprintf("artifact %s missing", art)
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
	if sp.database != nil {
		a, err := sp.database.GetArtifact(featureID, artifactTypeToDBKey(artType))
		if err == nil && a != nil {
			return a.Content, nil
		}
		return "", fmt.Errorf("artifact %s not found for feature %s", artType, featureID)
	}
	return sp.readArtifactFromDisk(featureID, artType)
}

func (sp *SpecProvider) readArtifactFromDisk(featureID string, artType feature.ArtifactType) (string, error) {
	path := sp.ArtifactPath(featureID, artType)
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

// LoadFeatureRepos reads repos.yaml from the DB and returns the declared RepoRefs.
func (sp *SpecProvider) LoadFeatureRepos(featureID string) ([]feature.RepoRef, error) {
	content, err := sp.ReadArtifact(featureID, feature.ArtifactReposYAML)
	if err != nil {
		return nil, nil // no repos.yaml = no external repos, that's fine
	}
	var parsed struct {
		Feature string            `yaml:"feature"`
		Repos   []feature.RepoRef `yaml:"repos"`
	}
	if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("parsing repos.yaml for %s: %w", featureID, err)
	}
	return parsed.Repos, nil
}