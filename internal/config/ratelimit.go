package config

import (
	"fmt"
	"regexp"
	"time"
)

// RateLimitConfig holds the parsed `rate_limit:` block (BLM §3.5). Absent
// section or enabled=false = passthrough (D7/R12, F-11 — byte-identical to
// pre-feature). Pointer fields implement the R5 pointer+default pattern:
// nil means "use the default", a non-nil value is validated.
//
// Validation lives in Validate() (BR-01..BR-07) and is invoked by
// ConfigureRateLimiting — NOT by the fatal validateConfig path (F-10,
// BR-08). An invalid block logs + disables the limiter (ADR-008 fail-open
// startup), so the server starts and traffic flows uncontrolled rather
// than crashing on a P2-feature config typo (O-5).
type RateLimitConfig struct {
	Enabled           bool                          `yaml:"enabled"`
	Defaults          RateLimitDefaults             `yaml:"defaults"`
	TrustProxyHeaders *bool                         `yaml:"trust_proxy_headers"` // default false (D2)
	FailMode          string                        `yaml:"fail_mode"`           // v2: "fail_open"
	DryRun            *bool                         `yaml:"dry_run"`             // default false (U10)
	EndpointOverrides map[string]RateLimitOverride  `yaml:"endpoint_overrides"`
	MaxTrackedKeys    *int                          `yaml:"max_tracked_keys"`    // default 10000 (§3.4)

	// configPath is the YAML file path the block was loaded from, set by
	// ConfigureRateLimiting so the status endpoint can echo config_source
	// (BR-46). Not parsed from YAML.
	configPath string
}

// RateLimitDefaults are the default policy values applied when no override
// matches a route (BLM §3.5). Defaults: limit=100, window_seconds=60 (O-1).
type RateLimitDefaults struct {
	Limit         *int `yaml:"limit"`          // default 100 (O-1)
	WindowSeconds *int `yaml:"window_seconds"`  // default 60 (O-1)
}

// RateLimitOverride is a per-route policy entry keyed by "METHOD /path"
// (BR-04). An override may set a limit+window OR mark the route exempt, but
// not both (BR-07).
type RateLimitOverride struct {
	Limit         *int  `yaml:"limit,omitempty"`
	WindowSeconds *int  `yaml:"window_seconds,omitempty"`
	Exempt        bool  `yaml:"exempt,omitempty"`
}

// GetTrustProxyHeaders returns the configured value, defaulting to false
// when nil (D2 — XFF opt-in, default OFF).
func (c *RateLimitConfig) GetTrustProxyHeaders() bool {
	if c == nil || c.TrustProxyHeaders == nil {
		return false
	}
	return *c.TrustProxyHeaders
}

// GetDryRun returns the configured value, defaulting to false (U10).
func (c *RateLimitConfig) GetDryRun() bool {
	if c == nil || c.DryRun == nil {
		return false
	}
	return *c.DryRun
}

// GetMaxTrackedKeys returns the configured value, defaulting to 10000
// (BLM §3.4 — ~640KB-1.28MB at ~80B/key).
func (c *RateLimitConfig) GetMaxTrackedKeys() int {
	if c == nil || c.MaxTrackedKeys == nil {
		return 10000
	}
	return *c.MaxTrackedKeys
}

// GetDefaultLimit returns the configured default limit, defaulting to 100
// (O-1).
func (c *RateLimitConfig) GetDefaultLimit() int {
	if c == nil || c.Defaults.Limit == nil {
		return 100
	}
	return *c.Defaults.Limit
}

// GetDefaultWindowSeconds returns the configured default window, defaulting
// to 60 seconds (O-1).
func (c *RateLimitConfig) GetDefaultWindowSeconds() int {
	if c == nil || c.Defaults.WindowSeconds == nil {
		return 60
	}
	return *c.Defaults.WindowSeconds
}

// GetDefaultWindow returns the default window as a Duration.
func (c *RateLimitConfig) GetDefaultWindow() time.Duration {
	return time.Duration(c.GetDefaultWindowSeconds()) * time.Second
}

// SetConfigPath records the YAML file path the block was loaded from, so the
// status endpoint can echo config_source (BR-46). Called by
// ConfigureRateLimiting after LoadConfig.
func (c *RateLimitConfig) SetConfigPath(path string) {
	if c == nil {
		return
	}
	c.configPath = path
}

// ConfigPath returns the recorded YAML file path (BR-46).
func (c *RateLimitConfig) ConfigPath() string {
	if c == nil {
		return ""
	}
	return c.configPath
}

// overrideKeyRe is the shape check for endpoint_overrides keys (BR-04):
// uppercase METHOD, single space, leading-slash path.
var overrideKeyRe = regexp.MustCompile(`^[A-Z]+ /`)

// Validate runs the 7 LOCKED validation rules (BR-01..BR-07). On any failure
// it returns an error naming the bad field; the caller (ConfigureRateLimiting)
// logs the error and leaves the limiter nil (BR-08, ADR-008 — fail-open
// startup, NOT a crash). Validate does NOT touch the fatal validateConfig
// path (F-10).
func (c *RateLimitConfig) Validate() error {
	if c == nil {
		return nil // nil config = passthrough; nothing to validate
	}
	if !c.Enabled {
		return nil // disabled = passthrough; nothing to validate
	}

	// BR-01 — fail_mode must be "fail_open" (v2). Any other value is a typo
	// that must surface now, not silently default.
	if c.FailMode != "" && c.FailMode != "fail_open" {
		return fmt.Errorf("rate_limit.fail_mode: only fail_open is supported in v2, got %q", c.FailMode)
	}

	// BR-02 — defaults.limit if non-nil must be > 0.
	if c.Defaults.Limit != nil && *c.Defaults.Limit <= 0 {
		return fmt.Errorf("rate_limit.defaults.limit: must be > 0, got %d", *c.Defaults.Limit)
	}

	// BR-03 — defaults.window_seconds if non-nil must be > 0.
	if c.Defaults.WindowSeconds != nil && *c.Defaults.WindowSeconds <= 0 {
		return fmt.Errorf("rate_limit.defaults.window_seconds: must be > 0, got %d", *c.Defaults.WindowSeconds)
	}

	// BR-04 — endpoint_overrides keys must match "METHOD /path".
	for key, ov := range c.EndpointOverrides {
		if !overrideKeyRe.MatchString(key) {
			return fmt.Errorf("rate_limit.endpoint_overrides: key %q must match \"METHOD /path\" (uppercase method, single space, leading slash)", key)
		}
		// BR-05 — each override must set at least one of limit, window_seconds, or exempt.
		hasLimit := ov.Limit != nil && *ov.Limit > 0
		hasWindow := ov.WindowSeconds != nil && *ov.WindowSeconds > 0
		if !hasLimit && !hasWindow && !ov.Exempt {
			return fmt.Errorf("rate_limit.endpoint_overrides: key %q is empty (set limit, window_seconds, or exempt)", key)
		}
		// BR-07 — exempt:true and non-zero limit/window_seconds on the same
		// override is contradictory (does it limit or not?).
		if ov.Exempt && (hasLimit || hasWindow) {
			return fmt.Errorf("rate_limit.endpoint_overrides: key %q has exempt:true with limit/window_seconds (mutually exclusive — BR-07)", key)
		}
		// BR-02/BR-03 apply to overrides too: a non-nil limit/window must be > 0.
		if ov.Limit != nil && *ov.Limit <= 0 {
			return fmt.Errorf("rate_limit.endpoint_overrides: key %q limit must be > 0, got %d", key, *ov.Limit)
		}
		if ov.WindowSeconds != nil && *ov.WindowSeconds <= 0 {
			return fmt.Errorf("rate_limit.endpoint_overrides: key %q window_seconds must be > 0, got %d", key, *ov.WindowSeconds)
		}
	}

	// BR-06 — max_tracked_keys if non-nil must be > 0 (hard floor 100).
	if c.MaxTrackedKeys != nil {
		if *c.MaxTrackedKeys <= 0 {
			return fmt.Errorf("rate_limit.max_tracked_keys: must be > 0, got %d", *c.MaxTrackedKeys)
		}
		if *c.MaxTrackedKeys < 100 {
			return fmt.Errorf("rate_limit.max_tracked_keys: must be >= 100 (hard floor to prevent thrash), got %d", *c.MaxTrackedKeys)
		}
	}

	return nil
}