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
	// Providers lists OpenAI-compatible LLM providers. Each entry has a
	// unique name. Optional: when absent or empty, dispatch falls back to
	// opencode's default model. API keys are NEVER stored here — only the
	// env var name (api_key_env).
	Providers     []ProviderConfig              `yaml:"providers"`
	// RoleProviders maps a Dev Team role → provider + model override.
	// A role absent from this map falls back to opencode's default.
	RoleProviders map[string]RoleProviderMapping `yaml:"role_providers"`
}

// ProviderConfig describes one OpenAI-compatible LLM provider. A provider is
// fully described by base_url + api_key_env + model — no provider-specific
// code branches exist (CON-003, FR-008). The API key value is never stored;
// only the name of the env var holding it.
type ProviderConfig struct {
	Name      string `yaml:"name" json:"name"`
	BaseURL   string `yaml:"base_url" json:"base_url"`
	APIKeyEnv string `yaml:"api_key_env" json:"api_key_env"`
	Model     string `yaml:"model" json:"model"` // optional default model
}

// RoleProviderMapping maps a Dev Team role to a provider + optional model
// override. When Model is empty, the provider's default Model is used.
type RoleProviderMapping struct {
	Provider string `yaml:"provider" json:"provider"`
	Model    string `yaml:"model" json:"model"` // optional; falls back to provider default
}

// ResolvedProvider is the dispatch-time resolution of a role's provider config.
// It lives only for the duration of one dispatch; the API key value is read
// from the environment at resolution time and never persisted.
type ResolvedProvider struct {
	BaseURL    string
	Model      string
	APIKeyEnv  string // env var name
	APIKeyValue string // value read from env at resolve time
}

// DatabaseConfig configures the database connection.
// Defaults to SQLite at .devteam.db if not specified.
// Set driver to "postgres" and provide a DSN for shared/multi-user deployments.
type DatabaseConfig struct {
	Driver string `yaml:"driver" json:"driver"` // "sqlite3" (default) or "postgres"
	DSN    string `yaml:"dsn" json:"dsn"`       // connection string
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
	return validateProviders(cfg)
}

// validateProviders validates the providers: + role_providers: config.
// Empty/absent providers: is the backward-compatible fallback (not an error).
func validateProviders(cfg *Config) error {
	seen := map[string]struct{}{}
	for _, p := range cfg.Providers {
		if _, dup := seen[p.Name]; dup {
			return fmt.Errorf("duplicate provider name: %s", p.Name)
		}
		seen[p.Name] = struct{}{}
		if p.BaseURL == "" {
			return fmt.Errorf("provider '%s' has empty base_url", p.Name)
		}
	}
	for role, mapping := range cfg.RoleProviders {
		if mapping.Provider == "" {
			continue // empty provider = no mapping, fallback
		}
		if _, ok := seen[mapping.Provider]; !ok {
			return fmt.Errorf("role '%s' references unknown provider '%s'", role, mapping.Provider)
		}
		// Find the provider to check model default.
		for _, p := range cfg.Providers {
			if p.Name == mapping.Provider {
				if mapping.Model == "" && p.Model == "" {
					return fmt.Errorf("role '%s' has no model and provider '%s' has no default model", role, mapping.Provider)
				}
				break
			}
		}
	}
	return nil
}

// ResolveProvider resolves the dispatch-time provider config for a role.
// Returns nil (no error) when the role has no mapping or providers: is absent
// — signaling the caller to use opencode's default (CON-010, FR-006).
// Returns an error when the mapped provider's api_key_env is unset or empty
// in the environment (CON-005, FR-005 — fail fast, do not spawn opencode).
func (cfg *Config) ResolveProvider(role string) (*ResolvedProvider, error) {
	if len(cfg.Providers) == 0 {
		return nil, nil
	}
	mapping, ok := cfg.RoleProviders[role]
	if !ok || mapping.Provider == "" {
		return nil, nil
	}
	var p ProviderConfig
	found := false
	for _, candidate := range cfg.Providers {
		if candidate.Name == mapping.Provider {
			p = candidate
			found = true
			break
		}
	}
	if !found {
		// validateProviders should have caught this at load; defensive.
		return nil, fmt.Errorf("role '%s' references unknown provider '%s'", role, mapping.Provider)
	}
	model := mapping.Model
	if model == "" {
		model = p.Model
	}
	rp := &ResolvedProvider{
		BaseURL:   p.BaseURL,
		Model:     model,
		APIKeyEnv: p.APIKeyEnv,
	}
	if p.APIKeyEnv != "" {
		val := os.Getenv(p.APIKeyEnv)
		if val == "" {
			return nil, fmt.Errorf("provider '%s': api key env var '%s' is not set", mapping.Provider, p.APIKeyEnv)
		}
		rp.APIKeyValue = val
	}
	return rp, nil
}
