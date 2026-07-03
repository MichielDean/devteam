package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ptrInt returns a pointer to n (helper for the pointer+default pattern R5).
func ptrInt(n int) *int { return &n }

// ptrBool returns a pointer to b.
func ptrBool(b bool) *bool { return &b }

func validEnabled() RateLimitConfig {
	return RateLimitConfig{
		Enabled:  true,
		FailMode: "fail_open",
		Defaults: RateLimitDefaults{
			Limit:         ptrInt(100),
			WindowSeconds: ptrInt(60),
		},
		MaxTrackedKeys: ptrInt(10000),
	}
}

// TestValidateRateLimitRejectsBadFailMode (BR-01) — fail_mode must be
// "fail_open"; anything else is a typo.
func TestValidateRateLimitRejectsBadFailMode(t *testing.T) {
	c := validEnabled()
	c.FailMode = "fail_closed"
	if err := c.Validate(); err == nil {
		t.Errorf("expected error for fail_closed (BR-01), got nil")
	}
}

// TestValidateRateLimitRejectsNegativeLimit (BR-02) — defaults.limit <= 0.
func TestValidateRateLimitRejectsNegativeLimit(t *testing.T) {
	c := validEnabled()
	c.Defaults.Limit = ptrInt(-5)
	if err := c.Validate(); err == nil {
		t.Errorf("expected error for negative limit (BR-02), got nil")
	}
}

// TestValidateRateLimitRejectsZeroLimit (BR-02) — defaults.limit == 0.
func TestValidateRateLimitRejectsZeroLimit(t *testing.T) {
	c := validEnabled()
	c.Defaults.Limit = ptrInt(0)
	if err := c.Validate(); err == nil {
		t.Errorf("expected error for zero limit (BR-02), got nil")
	}
}

// TestValidateRateLimitRejectsZeroWindow (BR-03) — defaults.window_seconds == 0.
func TestValidateRateLimitRejectsZeroWindow(t *testing.T) {
	c := validEnabled()
	c.Defaults.WindowSeconds = ptrInt(0)
	if err := c.Validate(); err == nil {
		t.Errorf("expected error for zero window (BR-03), got nil")
	}
}

// TestValidateRateLimitRejectsNegativeWindow (BR-03).
func TestValidateRateLimitRejectsNegativeWindow(t *testing.T) {
	c := validEnabled()
	c.Defaults.WindowSeconds = ptrInt(-1)
	if err := c.Validate(); err == nil {
		t.Errorf("expected error for negative window (BR-03), got nil")
	}
}

// TestValidateRateLimitRejectsBadOverrideKey (BR-04) — keys must match
// "METHOD /path" (uppercase method, single space, leading slash).
func TestValidateRateLimitRejectsBadOverrideKey(t *testing.T) {
	cases := []string{
		"post /v1/run", // lowercase method
		"/v1/run",      // no method
		"GET v1/run",   // no leading slash
		"GET  /v1/run", // double space
	}
	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			c := validEnabled()
			c.EndpointOverrides = map[string]RateLimitOverride{
				key: {Limit: ptrInt(300)},
			}
			if err := c.Validate(); err == nil {
				t.Errorf("expected error for bad override key %q (BR-04), got nil", key)
			}
		})
	}
}

// TestValidateRateLimitAcceptsGoodOverrideKey (BR-04) — "GET /v1/run" is OK.
func TestValidateRateLimitAcceptsGoodOverrideKey(t *testing.T) {
	c := validEnabled()
	c.EndpointOverrides = map[string]RateLimitOverride{
		"POST /v1/run": {Limit: ptrInt(300), WindowSeconds: ptrInt(60)},
	}
	if err := c.Validate(); err != nil {
		t.Errorf("expected OK for good override key, got %v (BR-04)", err)
	}
}

// TestValidateRateLimitRejectsEmptyOverride (BR-05) — an override with no
// limit, window_seconds, or exempt is a no-op typo.
func TestValidateRateLimitRejectsEmptyOverride(t *testing.T) {
	c := validEnabled()
	c.EndpointOverrides = map[string]RateLimitOverride{
		"POST /v1/run": {},
	}
	if err := c.Validate(); err == nil {
		t.Errorf("expected error for empty override (BR-05), got nil")
	}
}

// TestValidateRateLimitRejectsZeroMaxTrackedKeys (BR-06) — must be > 0.
func TestValidateRateLimitRejectsZeroMaxTrackedKeys(t *testing.T) {
	c := validEnabled()
	c.MaxTrackedKeys = ptrInt(0)
	if err := c.Validate(); err == nil {
		t.Errorf("expected error for zero max_tracked_keys (BR-06), got nil")
	}
}

// TestValidateRateLimitRejectsLowMaxTrackedKeys (BR-06) — hard floor 100.
func TestValidateRateLimitRejectsLowMaxTrackedKeys(t *testing.T) {
	c := validEnabled()
	c.MaxTrackedKeys = ptrInt(10)
	if err := c.Validate(); err == nil {
		t.Errorf("expected error for max_tracked_keys=10 < 100 floor (BR-06), got nil")
	}
}

// TestValidateRateLimitAccepts100MaxTrackedKeys (BR-06) — 100 is the floor.
func TestValidateRateLimitAccepts100MaxTrackedKeys(t *testing.T) {
	c := validEnabled()
	c.MaxTrackedKeys = ptrInt(100)
	if err := c.Validate(); err != nil {
		t.Errorf("expected OK for max_tracked_keys=100 (BR-06 floor), got %v", err)
	}
}

// TestValidateRateLimitRejectsExemptWithLimit (BR-07) — exempt:true and a
// non-zero limit on the same override is contradictory.
func TestValidateRateLimitRejectsExemptWithLimit(t *testing.T) {
	c := validEnabled()
	c.EndpointOverrides = map[string]RateLimitOverride{
		"GET /health": {Exempt: true, Limit: ptrInt(100)},
	}
	if err := c.Validate(); err == nil {
		t.Errorf("expected error for exempt+limit (BR-07), got nil")
	}
}

// TestValidateRateLimitAcceptsExemptAlone (BR-07) — exempt:true alone is OK.
func TestValidateRateLimitAcceptsExemptAlone(t *testing.T) {
	c := validEnabled()
	c.EndpointOverrides = map[string]RateLimitOverride{
		"GET /health": {Exempt: true},
	}
	if err := c.Validate(); err != nil {
		t.Errorf("expected OK for exempt alone, got %v", err)
	}
}

// TestValidateRateLimitAcceptsValidConfig — a full valid config passes.
func TestValidateRateLimitAcceptsValidConfig(t *testing.T) {
	c := validEnabled()
	if err := c.Validate(); err != nil {
		t.Errorf("valid config should pass, got %v", err)
	}
}

// TestValidateRateLimitDisabledIsNoOp (BR-33) — enabled=false skips
// validation (passthrough; nothing to validate).
func TestValidateRateLimitDisabledIsNoOp(t *testing.T) {
	c := RateLimitConfig{Enabled: false, FailMode: "fail_closed"} // would fail if enabled
	if err := c.Validate(); err != nil {
		t.Errorf("disabled config should skip validation, got %v", err)
	}
}

// TestValidateRateLimitNilIsNoOp — nil config is passthrough.
func TestValidateRateLimitNilIsNoOp(t *testing.T) {
	var c *RateLimitConfig
	if err := c.Validate(); err != nil {
		t.Errorf("nil config should skip validation, got %v", err)
	}
}

// TestLoadConfigAbsentRateLimitSection (BR-33, F-11) — a YAML with no
// rate_limit: block loads to a zero-value RateLimitConfig (Enabled:false).
// The server's behavior is byte-identical to pre-feature.
func TestLoadConfigAbsentRateLimitSection(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `version: "2.0"
`
	cfgPath := filepath.Join(tmpDir, "devteam.yaml")
	writeFile(t, cfgPath, cfgContent)

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.RateLimit.Enabled {
		t.Errorf("absent rate_limit section should yield Enabled=false (BR-33/F-11), got Enabled=true")
	}
	// Getters return defaults when nil.
	if cfg.RateLimit.GetDefaultLimit() != 100 {
		t.Errorf("default limit should be 100, got %d", cfg.RateLimit.GetDefaultLimit())
	}
	if cfg.RateLimit.GetDefaultWindowSeconds() != 60 {
		t.Errorf("default window should be 60, got %d", cfg.RateLimit.GetDefaultWindowSeconds())
	}
	if cfg.RateLimit.GetMaxTrackedKeys() != 10000 {
		t.Errorf("default max_tracked_keys should be 10000, got %d", cfg.RateLimit.GetMaxTrackedKeys())
	}
	if cfg.RateLimit.GetTrustProxyHeaders() {
		t.Errorf("default trust_proxy_headers should be false (D2)")
	}
	if cfg.RateLimit.GetDryRun() {
		t.Errorf("default dry_run should be false (U10)")
	}
}

// TestLoadConfigInvalidRateLimitDoesNotCrash (BR-08, F-10) — a malformed
// rate_limit block loads structurally (no crash); the semantic validation
// is deferred to ConfigureRateLimiting. LoadConfig must NOT route rate-limit
// validation through the fatal validateConfig path.
func TestLoadConfigInvalidRateLimitDoesNotCrash(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `version: "2.0"
rate_limit:
  enabled: true
  fail_mode: fail_closed
  defaults:
    limit: -5
`
	cfgPath := filepath.Join(tmpDir, "devteam.yaml")
	writeFile(t, cfgPath, cfgContent)

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig must not fail on invalid rate_limit block (BR-08/F-10); structural parse only, validation deferred to ConfigureRateLimiting: %v", err)
	}
	// Structural parse succeeded; the block is present but invalid.
	if !cfg.RateLimit.Enabled {
		t.Errorf("expected Enabled=true after structural parse")
	}
	// Validation SHOULD fail now (deferred to Validate()).
	if err := cfg.RateLimit.Validate(); err == nil {
		t.Errorf("Validate() should reject fail_closed + negative limit (BR-01/BR-02), got nil")
	}
}

// TestLoadConfigValidRateLimitParses — a valid block parses and validates.
func TestLoadConfigValidRateLimitParses(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `version: "2.0"
rate_limit:
  enabled: true
  fail_mode: fail_open
  defaults:
    limit: 100
    window_seconds: 60
  endpoint_overrides:
    "POST /v1/run":
      limit: 300
      window_seconds: 60
  max_tracked_keys: 5000
  trust_proxy_headers: false
  dry_run: false
`
	cfgPath := filepath.Join(tmpDir, "devteam.yaml")
	writeFile(t, cfgPath, cfgContent)

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.RateLimit.Enabled {
		t.Errorf("expected Enabled=true")
	}
	if err := cfg.RateLimit.Validate(); err != nil {
		t.Errorf("valid config should pass Validate(), got %v", err)
	}
	if cfg.RateLimit.GetMaxTrackedKeys() != 5000 {
		t.Errorf("max_tracked_keys should be 5000, got %d", cfg.RateLimit.GetMaxTrackedKeys())
	}
	ov := cfg.RateLimit.EndpointOverrides["POST /v1/run"]
	if ov.Limit == nil || *ov.Limit != 300 {
		t.Errorf("override limit should be 300, got %v", ov.Limit)
	}
}

// TestLoadConfigDisabledRateLimit — enabled:false parses and skips validation.
func TestLoadConfigDisabledRateLimit(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `version: "2.0"
rate_limit:
  enabled: false
`
	cfgPath := filepath.Join(tmpDir, "devteam.yaml")
	writeFile(t, cfgPath, cfgContent)

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.RateLimit.Enabled {
		t.Errorf("expected Enabled=false")
	}
	// Disabled skips validation — even an invalid fail_mode would not error.
	cfg.RateLimit.FailMode = "fail_closed"
	if err := cfg.RateLimit.Validate(); err != nil {
		t.Errorf("disabled config should skip validation, got %v (BR-33)", err)
	}
}

// TestRateLimitConfigPathRoundTrip (BR-46) — SetConfigPath/ConfigPath echo.
func TestRateLimitConfigPathRoundTrip(t *testing.T) {
	c := validEnabled()
	c.SetConfigPath("devteam.yaml")
	if got := c.ConfigPath(); got != "devteam.yaml" {
		t.Errorf("ConfigPath = %q, want devteam.yaml (BR-46)", got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}