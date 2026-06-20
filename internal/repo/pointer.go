package repo

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type DevTeamPointer struct {
	SpecRepo   string `yaml:"spec_repo"`
	FeatureID  string `yaml:"feature_id"`
	SpecPath   string `yaml:"spec_path"`
	Branch     string `yaml:"branch"`
}

func WritePointer(repoDir string, pointer *DevTeamPointer) error {
	dir := filepath.Join(repoDir, ".devteam")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating .devteam directory: %w", err)
	}
	data, err := yaml.Marshal(pointer)
	if err != nil {
		return fmt.Errorf("marshaling pointer: %w", err)
	}
	pointerPath := filepath.Join(dir, "pointer.yaml")
	if err := os.WriteFile(pointerPath, data, 0644); err != nil {
		return fmt.Errorf("writing pointer: %w", err)
	}
	return nil
}

func ReadPointer(repoDir string) (*DevTeamPointer, error) {
	pointerPath := filepath.Join(repoDir, ".devteam", "pointer.yaml")
	data, err := os.ReadFile(pointerPath)
	if err != nil {
		return nil, fmt.Errorf("reading pointer: %w", err)
	}
	var pointer DevTeamPointer
	if err := yaml.Unmarshal(data, &pointer); err != nil {
		return nil, fmt.Errorf("parsing pointer: %w", err)
	}
	return &pointer, nil
}

func (m *Manager) WritePointersForFeature(featureID string, workDirs []*RepoWorkDir) error {
	for _, wd := range workDirs {
		pointer := &DevTeamPointer{
			SpecRepo:  "devteam",
			FeatureID: featureID,
			SpecPath:  filepath.Join("specs", featureID),
			Branch:    fmt.Sprintf("feature/%s", featureID),
		}
		if err := WritePointer(wd.Dir, pointer); err != nil {
			return fmt.Errorf("writing pointer for %s: %w", wd.Name, err)
		}
	}
	return nil
}