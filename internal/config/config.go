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
	ExecutionMode                  string        `yaml:"execution_mode" json:"execution_mode"`
}

// GetHumanInteractionTimeoutMinutes returns the configured timeout, defaulting to 30 if not set.
func (pc *PipelineConfig) GetHumanInteractionTimeoutMinutes() int {
	if pc.HumanInteractionTimeoutMinutes == nil {
		return 30
	}
	return *pc.HumanInteractionTimeoutMinutes
}

// GetExecutionMode returns the configured default execution mode, defaulting to "human" if not set.
func (pc *PipelineConfig) GetExecutionMode() string {
	if pc.ExecutionMode == "" {
		return "human"
	}
	return pc.ExecutionMode
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

// NOTE: The slice-based `ReposConfig` / `LoadRepos` parser was removed in the
// settings-and-admin-ui feature (FR-CONFIG-07). It parsed `repos` as a YAML
// sequence, but the on-disk repos.yaml uses a map keyed by repo name, so the
// parser silently produced an empty slice. The DB-backed `repos` table
// (migration 017, repo_store.go) is now the source of truth for the registry,
// and `repos.yaml` is only the seed source (seed.go:SeedReposFromYAML, which
// parses the map-keyed shape). `internal/repo/manager.LoadReposConfig` (the
// sole caller of LoadRepos) was dead code and is also removed.
//
// RepoEntry is retained because internal/repo/manager.CloneForFeature still
// consumes it as the in-memory shape for a repo to clone. It is no longer
// parsed from YAML by this package; callers construct it from the DB store.
type RepoEntry struct {
	Name        string `yaml:"name" json:"name"`
	URL         string `yaml:"url" json:"url"`
	Description string `yaml:"description" json:"description"`
	Primary     bool   `yaml:"primary,omitempty" json:"primary"`
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

func validateConfig(cfg *Config) error {
	// AIDLC v2: phases are defined in the DB (stage_definitions table).
	// Config phases are optional — used only for human interaction timeout and role rules.
	// -1 is a valid timeout meaning "no timeout" (wait indefinitely).
	if cfg.Pipeline.HumanInteractionTimeoutMinutes != nil && *cfg.Pipeline.HumanInteractionTimeoutMinutes < -1 {
		return fmt.Errorf("human_interaction_timeout_minutes must be >= -1 (use -1 for no timeout), got %d", *cfg.Pipeline.HumanInteractionTimeoutMinutes)
	}
	return nil
}
