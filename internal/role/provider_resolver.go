package role

import (
	"fmt"

	"github.com/MichielDean/devteam/internal/config"
)

// ─── ModelTier resolver (G3-3) ────────────────────────────────────────────
//
// ResolveProvider translates a ModelTier ("opus"/"sonnet") + the providers
// config into a concrete (provider, model) tuple. Default-safe: if the tier
// has no mapping in config (or providers is empty), returns the current
// ollama/glm-5.2:cloud behavior — no regression for existing agents (C15,
// NFR-REL-4, R7).
//
// This function is PURE (no I/O) and has ZERO provider-specific branches
// (NFR-MAINT-2): adding a provider is a config entry, not a code change here.

// DefaultProviderName and DefaultModel are the pre-feature behavior — the
// hardcoded values tmux.go and agent_handlers.go used before this feature.
// The resolver returns these when no config mapping exists.
const (
	DefaultProviderName = "ollama"
	DefaultModel         = "ollama/glm-5.2:cloud"
	DefaultBaseURL       = "http://localhost:11434/v1"
	DefaultAdapter       = "openai"
)

// ResolvedProvider is the concrete provider+model a dispatch should use.
type ResolvedProvider struct {
	Name      string // provider name (e.g. "ollama", "openai", "anthropic")
	BaseURL   string // provider API base URL
	APIKeyEnv string // env var name holding the API key (empty = no key)
	Model     string // model id (e.g. "glm-5.2:cloud", "gpt-4o")
	Adapter   string // "openai" | "anthropic" — selects the opencode npm adapter
}

// APIKeyValue returns the API key from the env var named by APIKeyEnv, or "".
// Read at dispatch time (NFR-SEC-4 — never stored in config).
func (rp *ResolvedProvider) APIKeyValue() string {
	if rp == nil || rp.APIKeyEnv == "" {
		return ""
	}
	return osGetenv(rp.APIKeyEnv)
}

// ResolveProviderEnvPresent reports whether an env var is set (non-empty).
// Used by the provider picker to mark a provider "available" — a provider
// that needs a key is only available if the key env var is set. This is a
// presence check, not a value check (the value is never read here — NFR-SEC-4).
func ResolveProviderEnvPresent(envVar string) bool {
	if envVar == "" {
		return true // no key needed → always available
	}
	return osGetenv(envVar) != ""
}

// ResolveProvider returns the concrete provider for a role's ModelTier.
// Default-safe: if the tier has no mapping in config, returns the current
// ollama/glm-5.2:cloud behavior (NFR-REL-4, C15). Never panics.
func ResolveProvider(cfg *config.Config, tier string) (*ResolvedProvider, error) {
	if cfg == nil || len(cfg.Providers) == 0 {
		return defaultProvider(), nil
	}
	// Find the first provider that lists this tier.
	for _, p := range cfg.Providers {
		for _, t := range p.Tiers {
			if t == tier {
				return &ResolvedProvider{
					Name:      p.Name,
					BaseURL:    p.BaseURL,
					APIKeyEnv:  p.APIKeyEnv,
					Model:      p.Model,
					Adapter:    p.Adapter,
				}, nil
			}
		}
	}
	// No mapping for this tier → default-safe.
	return defaultProvider(), nil
}

// ResolveProviderByName returns the provider config with the given name, or
// the default if no providers configured / name not found.
func ResolveProviderByName(cfg *config.Config, name string) (*ResolvedProvider, error) {
	if cfg == nil || len(cfg.Providers) == 0 || name == "" {
		return defaultProvider(), nil
	}
	for _, p := range cfg.Providers {
		if p.Name == name {
			return &ResolvedProvider{
				Name:      p.Name,
				BaseURL:   p.BaseURL,
				APIKeyEnv: p.APIKeyEnv,
				Model:     p.Model,
				Adapter:   p.Adapter,
			}, nil
		}
	}
	return nil, fmt.Errorf("provider %q not found in config", name)
}

func defaultProvider() *ResolvedProvider {
	return &ResolvedProvider{
		Name:    DefaultProviderName,
		BaseURL: DefaultBaseURL,
		Model:   DefaultModel,
		Adapter: DefaultAdapter,
	}
}