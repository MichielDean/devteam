package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version    string                     `yaml:"version"`
	Pipeline   PipelineConfig             `yaml:"pipeline"`
	Roles      map[string]RoleConfig      `yaml:"roles"`
	Extensions map[string]ExtensionConfig `yaml:"extensions"`
	Plugins    map[string]PluginConfig    `yaml:"plugins"`
	Intake     IntakeConfig               `yaml:"intake"`
	SpecRepo   SpecRepoConfig             `yaml:"spec_repo"`
	Database   DatabaseConfig             `yaml:"database"`
	GitHub     GitHubConfig               `yaml:"github"`
}

// GitHubConfig configures native GitHub App authorization (feature
// github-authorization-integration). Empty/zero value = feature not configured;
// construction of a GitHubClient from this block is gated by the caller (the
// block is optional and ignored if absent, preserving pre-feature behavior).
type GitHubConfig struct {
	Provider                 string            `yaml:"provider"`                   // "native" (default) | "gh" (fallback adapter). Empty → "native".
	AppID                    int64             `yaml:"app_id"`                     // GitHub App ID (from App settings page)
	InstallationID            int64             `yaml:"installation_id"`            // App installation ID (from installation URL/API)
	PrivateKeyPath            string            `yaml:"private_key_path"`          // env/config-pointed App private key PEM path (NOT the master key)
	TokenCacheTTL             time.Duration     `yaml:"token_cache_ttl"`            // < GitHub's 60m installation-token lifetime; default 9m
	PATFallback               PATFallbackConfig `yaml:"pat_fallback"`
	ConflictPollMaxRetries    int               `yaml:"conflict_poll_max_retries"`  // NFR-PERF-02; default 5, hard ceiling 10
	ConflictPollMaxDuration   time.Duration     `yaml:"conflict_poll_max_duration"`  // NFR-PERF-02; default 60s, hard ceiling 300s
}

// PATFallbackConfig configures the PAT fallback auth path (FR-AUTH-02, ADR-09).
type PATFallbackConfig struct {
	Enabled bool `yaml:"enabled"` // default false; PAT stored via `devteam auth store-pat` (stdin)
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
	// AIDLC v2: phases are defined in the DB (stage_definitions table).
	// Config phases are optional — used only for human interaction timeout and role rules.
	// -1 is a valid timeout meaning "no timeout" (wait indefinitely).
	if cfg.Pipeline.HumanInteractionTimeoutMinutes != nil && *cfg.Pipeline.HumanInteractionTimeoutMinutes < -1 {
		return fmt.Errorf("human_interaction_timeout_minutes must be >= -1 (use -1 for no timeout), got %d", *cfg.Pipeline.HumanInteractionTimeoutMinutes)
	}

	// GitHub authorization block validation (feature github-authorization-integration).
	// The block is OPTIONAL: a zero-value GitHubConfig (AppID == 0, Provider == "")
	// means the feature is not configured and construction of a GitHubClient is
	// the caller's responsibility (the pipeline skips auth-health when the block
	// is absent, preserving pre-feature behavior — NFR-COMPAT-03).
	if cfg.GitHub.AppID != 0 || cfg.GitHub.Provider != "" || cfg.GitHub.PrivateKeyPath != "" {
		if err := validateGitHubConfig(&cfg.GitHub); err != nil {
			return err
		}
	}
	return nil
}

// validateGitHubConfig enforces the load-time rules from interaction-spec §4.1
// and nfr-design-specs §4.5. It returns a non-nil error for any bound violation;
// the caller (LoadConfig) wraps it with the file path and the recovery pointer.
func validateGitHubConfig(g *GitHubConfig) error {
	// Provider normalization: empty → "native" (FR-AUTH-05).
	if g.Provider == "" {
		g.Provider = "native"
	}
	if g.Provider != "native" && g.Provider != "gh" {
		return fmt.Errorf("github.provider: must be 'native' or 'gh'; got %q", g.Provider)
	}

	// app_id + installation_id required when the feature is configured.
	if g.AppID == 0 {
		return fmt.Errorf("github.app_id: required (integer, from GitHub App settings page); see docs/github-app-setup.md §2")
	}
	if g.InstallationID == 0 {
		return fmt.Errorf("github.installation_id: required (integer, from App installation URL/API); see docs/github-app-setup.md §4")
	}
	if g.PrivateKeyPath == "" {
		return fmt.Errorf("github.private_key_path: required (path to App private key PEM); see docs/github-app-setup.md §3")
	}

	// token_cache_ttl: default 9m, must be > 0 and < 60m (GitHub's 60-minute
	// installation-token lifetime). interaction-spec §4.1 / infra-specs §7.2
	// (B2 resolution: the correct bound is 60m, not the 10m stated in the
	// stale nfr-design-specs §4.2 line — GitHub App installation tokens expire
	// in 1 hour; see https://docs.github.com/en/apps/creating-github-apps/).
	if g.TokenCacheTTL == 0 {
		g.TokenCacheTTL = 9 * time.Minute
	}
	if g.TokenCacheTTL <= 0 {
		return fmt.Errorf("github.token_cache_ttl: must be > 0; got %s", g.TokenCacheTTL)
	}
	if g.TokenCacheTTL >= 60*time.Minute {
		return fmt.Errorf("github.token_cache_ttl: must be < 60m (GitHub limit); got %s", g.TokenCacheTTL)
	}

	// conflict_poll bounds: defaults 5 / 60s, hard ceilings 10 / 300s.
	// nfr-design-specs §4.5, infra-specs §7.2.
	if g.ConflictPollMaxRetries == 0 {
		g.ConflictPollMaxRetries = 5
	}
	if g.ConflictPollMaxDuration == 0 {
		g.ConflictPollMaxDuration = 60 * time.Second
	}
	if g.ConflictPollMaxRetries < 1 || g.ConflictPollMaxRetries > 10 {
		return fmt.Errorf("github.conflict_poll_max_retries: must be 1..10; got %d", g.ConflictPollMaxRetries)
	}
	if g.ConflictPollMaxDuration < 0 || g.ConflictPollMaxDuration > 300*time.Second {
		return fmt.Errorf("github.conflict_poll_max_duration: must be 0..300s; got %s", g.ConflictPollMaxDuration)
	}
	return nil
}
