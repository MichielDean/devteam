package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ─── Multi-provider config (G3) ───────────────────────────────────────────
//
// ProviderConfig describes one LLM provider in a provider-agnostic schema
// (FR-G3-1). A provider is fully described by: name, base_url, api_key_env
// (env var name, never the key value — NFR-SEC-4), model, adapter (the opencode
// npm adapter package), and the tiers this provider serves (opus/sonnet).
//
// The schema has ZERO provider-specific code branches (NFR-MAINT-2): adding
// a new provider (e.g. Google) is a config entry, not a code change. The
// adapter field is the one place a provider-kind identifier is legitimate —
// it selects the opencode npm package (@ai-sdk/openai-compatible vs
// @ai-sdk/anthropic), not branching logic.
type ProviderConfig struct {
	Name      string   `yaml:"name" json:"name"`
	BaseURL   string   `yaml:"base_url" json:"base_url"`
	APIKeyEnv string   `yaml:"api_key_env" json:"api_key_env"`
	Model     string   `yaml:"model" json:"model"`
	Adapter   string   `yaml:"adapter" json:"adapter"` // "openai" | "anthropic"
	Tiers     []string `yaml:"tiers" json:"tiers"`       // ["opus","sonnet"]
}

// ProviderList unmarshals from YAML in three forms (FR-G3-5):
//   - absent / empty → zero providers (default-safe, NFR-REL-4)
//   - a sequence of maps: providers: [{name: ...}, {name: ...}]
//   - an empty map: providers: {} (treated as zero providers)
//
// This mirrors the prior-art ProviderList.UnmarshalYAML (re §7.1, CON-010).
type ProviderList []ProviderConfig

func (p *ProviderList) UnmarshalYAML(value *yaml.Node) error {
	// Empty map or null → zero providers (default-safe).
	if value.Kind == yaml.MappingNode && len(value.Content) == 0 {
		*p = nil
		return nil
	}
	if value.Kind == yaml.ScalarNode && (value.Value == "" || value.Value == "null") {
		*p = nil
		return nil
	}
	// Otherwise treat as a sequence (the canonical form) or a single map.
	var list []ProviderConfig
	if err := value.Decode(&list); err != nil {
		return fmt.Errorf("providers: must be a sequence of provider entries: %w", err)
	}
	*p = list
	return nil
}

// ExpertConfig carries the expert scope toggle (FR-CL-6). Default false =
// hard refusal for off-topic questions (NG7). The field ships in MVS so the
// toggle is operator-controllable from day one; the *behavior* (best-effort
// with prefix) is the Should-after-MVS.
type ExpertConfig struct {
	AllowOffTopic bool `yaml:"allow_off_topic" json:"allow_off_topic"`
}

// ChatConfig carries chat-surface settings. The trust_mode toggle (FR-CL-5)
// is a Should-after-MVS; the field ships now for forward-compat but defaults
// to false (confirm every safe mutating op — the MVS behavior).
type ChatConfig struct {
	TrustMode bool `yaml:"trust_mode" json:"trust_mode"`
}