package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version    string                     `yaml:"version"`
	Pipeline   PipelineConfig             `yaml:"pipeline"`
	Roles      map[string]RoleConfig      `yaml:"roles"`
	Extensions map[string]ExtensionConfig `yaml:"extensions"`
	Plugins    map[string]PluginConfig     `yaml:"plugins"`
	Intake     IntakeConfig               `yaml:"intake"`
	SpecRepo   SpecRepoConfig             `yaml:"spec_repo"`
	Database   DatabaseConfig             `yaml:"database"`
}

// DatabaseConfig configures the PostgreSQL database connection.
type DatabaseConfig struct {
	DSN string `yaml:"dsn" json:"dsn"` // PostgreSQL connection string
}

type PipelineConfig struct {
	Phases                         []PhaseConfig `yaml:"phases"`
	HumanInteractionTimeoutMinutes *int          `yaml:"human_interaction_timeout_minutes"`
}

// GetHumanInteractionTimeoutMinutes returns the configured timeout, defaulting to 30 if not set.
func (pc *PipelineConfig) GetHumanInteractionTimeoutMinutes() int {
	if pc.HumanInteractionTimeoutMinutes == nil {
		return 30
	}
	return *pc.HumanInteractionTimeoutMinutes
}

type PhaseConfig struct {
	Name      string   `yaml:"name"`
	Roles     []string `yaml:"roles"`
	Gate      string   `yaml:"gate"`
	Artifacts []string `yaml:"artifacts"`
	Rules     string   `yaml:"rules"`
}

type RoleConfig struct {
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	Instructions string `yaml:"instructions"`
	PhaseRules   string `yaml:"phase_rules"`
}

type ExtensionConfig struct {
	OptIn           bool   `yaml:"opt_in"`
	LoadForPriority []int  `yaml:"load_for_priority"`
	Rules           string `yaml:"rules"`
}

type PluginConfig struct {
	Source string   `yaml:"source"`
	Phases []string `yaml:"phases"`
	Roles  []string `yaml:"roles"`
	Mode   string   `yaml:"mode"`
}

type IntakeConfig struct {
	LooseIdea    IntakePathConfig `yaml:"loose_idea"`
	ExternalSpec IntakePathConfig `yaml:"external_spec"`
}

type IntakePathConfig struct {
	Description string   `yaml:"description"`
	Output      []string `yaml:"output"`
}

type SpecRepoConfig struct {
	Path            string `yaml:"path"`
	SpecsDir        string `yaml:"specs_dir"`
	ConstitutionDir string `yaml:"constitution_dir"`
}

type ReposConfig struct {
	Repos []RepoEntry `yaml:"repos"`
}

type RepoEntry struct {
	Name        string `yaml:"name"`
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
	Primary     bool   `yaml:"primary,omitempty"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}
	return &cfg, nil
}

func LoadRepos(path string) (*ReposConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading repos file %s: %w", path, err)
	}
	var repos ReposConfig
	if err := yaml.Unmarshal(data, &repos); err != nil {
		return nil, fmt.Errorf("parsing repos file %s: %w", path, err)
	}
	return &repos, nil
}

func validateConfig(cfg *Config) error {
	if len(cfg.Pipeline.Phases) != 6 {
		return fmt.Errorf("expected 6 pipeline phases, got %d", len(cfg.Pipeline.Phases))
	}
	expectedPhases := []string{"inception", "planning", "construction", "review", "testing", "delivery"}
	for i, ep := range expectedPhases {
		if cfg.Pipeline.Phases[i].Name != ep {
			return fmt.Errorf("expected phase %d to be %s, got %s", i, ep, cfg.Pipeline.Phases[i].Name)
		}
	}
	expectedRoles := []string{"pm", "architect", "developer", "reviewer", "tester", "ops"}
	for _, er := range expectedRoles {
		if _, ok := cfg.Roles[er]; !ok {
			return fmt.Errorf("missing required role: %s", er)
		}
	}
	return nil
}
