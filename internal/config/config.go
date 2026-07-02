package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

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
	RateLimit  RateLimitConfig            `yaml:"rate_limit"`
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
	// AIDLC v2: phases are defined in the DB (stage_definitions table).
	// Config phases are optional — used only for human interaction timeout and role rules.
	// -1 is a valid timeout meaning "no timeout" (wait indefinitely).
	if cfg.Pipeline.HumanInteractionTimeoutMinutes != nil && *cfg.Pipeline.HumanInteractionTimeoutMinutes < -1 {
		return fmt.Errorf("human_interaction_timeout_minutes must be >= -1 (use -1 for no timeout), got %d", *cfg.Pipeline.HumanInteractionTimeoutMinutes)
	}
	// NOTE (NDP-07, O-5/ADR-008): rate-limit validation does NOT run here. The
	// existing validateConfig return path is FATAL (LoadConfig wraps its error,
	// main.go exits on any LoadConfig error). Routing rate-limit rules through
	// it would crash startup on a rate-limit config typo, violating the fail-open
	// startup contract (O-5). Rate-limit validation lives in
	// RateLimitConfig.Validate(), called by api.Server.ConfigureRateLimiting
	// (U-W), which on failure logs + leaves the limiter nil (passthrough). This
	// keeps the existing fatal-validation contract untouched for other sections
	// and isolates rate-limit fail-open behavior in the wiring unit.
	return nil
}

// --- Rate limiting config (rate_limiting-middleware feature) ---
//
// YAML key is `rate_limit` (NOT `rate_limiting` — that was a 2.6 reversal
// corrected by the LOCKED 2.5 M3.1 decision; see business-logic-model §0 C-6).
// Absent or enabled=false = passthrough (byte-identical, D7/R12). Optional
// fields use pointer+getter defaults (R5): nil → documented default.
//
// Validation rules (BR-01..BR-07, 7 rules) live in RateLimitConfig.Validate(),
// NOT in the fatal validateConfig path (see the note above). The Validate
// method is the single entry point the wiring unit (U-W) calls before arming
// the limiter.

// RateLimitConfig is the YAML config section for the rate limiter. The zero
// value (absent block) is passthrough: Enabled=false, no limiter armed.
type RateLimitConfig struct {
	// Enabled is non-pointer bool: zero value = false = passthrough (D7/R12).
	// The absent-section case is byte-identical passthrough without a getter.
	Enabled           bool                         `yaml:"enabled"`
	Defaults          RateLimitDefaults            `yaml:"defaults"`
	TrustProxyHeaders *bool                        `yaml:"trust_proxy_headers"`   // default false (D2)
	FailMode          string                       `yaml:"fail_mode"`             // v1: "fail_open"
	DryRun            *bool                        `yaml:"dry_run"`               // default false (U10)
	EndpointOverrides map[string]RateLimitOverride  `yaml:"endpoint_overrides"`
	MaxTrackedKeys    *int                         `yaml:"max_tracked_keys"`      // default 10000 (§3.4)
	// ConfigSource records the path LoadConfig was called with, surfaced by the
	// status endpoint as config_source (M2.1). Set by the wiring unit (U-W), not
	// parsed from YAML.
	ConfigSource string `yaml:"-"`
}

// RateLimitDefaults holds the default policy. Pointers allow "absent → default"
// semantics (R5): nil → documented default (100 / 60).
type RateLimitDefaults struct {
	Limit         *int `yaml:"limit"`          // default 100 (O-1)
	WindowSeconds *int `yaml:"window_seconds"` // default 60 (O-1)
}

// RateLimitOverride is one per-endpoint override. An exempt override sets
// Exempt=true and leaves Limit/WindowSeconds nil (BR-07 rejects mixing).
type RateLimitOverride struct {
	Limit         *int  `yaml:"limit,omitempty"`
	WindowSeconds *int  `yaml:"window_seconds,omitempty"`
	Exempt        bool  `yaml:"exempt,omitempty"`
}

// GetTrustProxyHeaders returns the configured value or false if unset (D2).
func (c *RateLimitConfig) GetTrustProxyHeaders() bool {
	if c == nil || c.TrustProxyHeaders == nil {
		return false
	}
	return *c.TrustProxyHeaders
}

// GetDryRun returns the configured value or false if unset (U10).
func (c *RateLimitConfig) GetDryRun() bool {
	if c == nil || c.DryRun == nil {
		return false
	}
	return *c.DryRun
}

// GetMaxTrackedKeys returns the configured value or 10000 if unset (O-7).
func (c *RateLimitConfig) GetMaxTrackedKeys() int {
	if c == nil || c.MaxTrackedKeys == nil {
		return 10000
	}
	return *c.MaxTrackedKeys
}

// GetDefaultLimit returns the configured default limit or 100 if unset (O-1).
func (c *RateLimitConfig) GetDefaultLimit() int {
	if c == nil || c.Defaults.Limit == nil {
		return 100
	}
	return *c.Defaults.Limit
}

// GetDefaultWindowSeconds returns the configured default window or 60 if unset (O-1).
func (c *RateLimitConfig) GetDefaultWindowSeconds() int {
	if c == nil || c.Defaults.WindowSeconds == nil {
		return 60
	}
	return *c.Defaults.WindowSeconds
}

// overrideKeyRe is the BR-04 shape check: uppercase METHOD, single space,
// leading-slash path. Validated per override key at config time so a malformed
// key (which would never match at runtime) is surfaced as a typo.
var overrideKeyRe = regexp.MustCompile(`^[A-Z]+ /`)

// Validate runs the 7 LOCKED validation rules (BR-01..BR-07). On any failure it
// returns an error naming the bad field/value. This method is called by
// api.Server.ConfigureRateLimiting (U-W), NOT by the fatal validateConfig path
// (NDP-07/O-5/ADR-008 — see the note on validateConfig above).
func (c *RateLimitConfig) Validate() error {
	if c == nil {
		return nil // nil config = passthrough, not an error
	}
	// BR-01: fail_mode must be fail_open (v1 only).
	if c.FailMode != "" && c.FailMode != "fail_open" {
		return fmt.Errorf("rate_limit.fail_mode: only fail_open is supported in v1, got %q", c.FailMode)
	}
	// BR-02: defaults.limit if non-nil must be > 0.
	if c.Defaults.Limit != nil {
		if *c.Defaults.Limit <= 0 {
			return fmt.Errorf("rate_limit.defaults.limit: must be > 0, got %d", *c.Defaults.Limit)
		}
	}
	// BR-03: defaults.window_seconds if non-nil must be > 0.
	if c.Defaults.WindowSeconds != nil {
		if *c.Defaults.WindowSeconds <= 0 {
			return fmt.Errorf("rate_limit.defaults.window_seconds: must be > 0, got %d", *c.Defaults.WindowSeconds)
		}
	}
	// BR-04: endpoint_overrides keys must match "METHOD /path".
	for k, ov := range c.EndpointOverrides {
		if !overrideKeyRe.MatchString(k) {
			return fmt.Errorf("rate_limit.endpoint_overrides: key %q must match \"METHOD /path\" (uppercase method, leading slash)", k)
		}
		// BR-05: each override must set at least one of limit/window_seconds/exempt.
		hasLimit := ov.Limit != nil
		hasWindow := ov.WindowSeconds != nil
		if !ov.Exempt && !hasLimit && !hasWindow {
			return fmt.Errorf("rate_limit.endpoint_overrides: key %q is empty (set limit, window_seconds, or exempt)", k)
		}
		// BR-07: exempt + limit/window on the same override → error (mutually exclusive).
		if ov.Exempt {
			if (hasLimit && *ov.Limit > 0) || (hasWindow && *ov.WindowSeconds > 0) {
				return fmt.Errorf("rate_limit.endpoint_overrides: key %q: exempt is mutually exclusive with limit/window_seconds", k)
			}
		}
		// BR-02/03 applied per-override when set.
		if ov.Limit != nil && *ov.Limit <= 0 {
			return fmt.Errorf("rate_limit.endpoint_overrides: key %q: limit must be > 0, got %d", k, *ov.Limit)
		}
		if ov.WindowSeconds != nil && *ov.WindowSeconds <= 0 {
			return fmt.Errorf("rate_limit.endpoint_overrides: key %q: window_seconds must be > 0, got %d", k, *ov.WindowSeconds)
		}
	}
	// BR-06: max_tracked_keys if non-nil must be > 0. Hard floor is 100 (2.7 U-C
	// tightening accepted by 3.1 — prevents pathological configs that thrash).
	if c.MaxTrackedKeys != nil {
		if *c.MaxTrackedKeys < 100 {
			return fmt.Errorf("rate_limit.max_tracked_keys: must be >= 100, got %d", *c.MaxTrackedKeys)
		}
	}
	return nil
}

// EndpointOverridesList returns the overrides as a stable, sorted slice of
// {route, limit, window_seconds, exempt} for the status endpoint's
// endpoint_overrides array (M2.1 requires an array, not a map, with stable
// order). Sorted by route for deterministic output.
type EndpointOverrideView struct {
	Route         string
	Limit         *int
	WindowSeconds *int
	Exempt        bool
}

func (c *RateLimitConfig) EndpointOverridesList() []EndpointOverrideView {
	if c == nil || len(c.EndpointOverrides) == 0 {
		return []EndpointOverrideView{}
	}
	routes := make([]string, 0, len(c.EndpointOverrides))
	for k := range c.EndpointOverrides {
		routes = append(routes, k)
	}
	// Sort for stable output (sort.Strings via strings package to avoid a new dep).
	for i := 1; i < len(routes); i++ {
		for j := i; j > 0 && strings.Compare(routes[j-1], routes[j]) > 0; j-- {
			routes[j-1], routes[j] = routes[j], routes[j-1]
		}
	}
	out := make([]EndpointOverrideView, 0, len(routes))
	for _, r := range routes {
		ov := c.EndpointOverrides[r]
		out = append(out, EndpointOverrideView{
			Route:         r,
			Limit:         ov.Limit,
			WindowSeconds: ov.WindowSeconds,
			Exempt:        ov.Exempt,
		})
	}
	return out
}
