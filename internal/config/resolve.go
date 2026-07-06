package config

import (
	"fmt"
	"strings"
)

// ResolveProvider resolves the provider+model for a single dispatch, given the
// merged provider list, the role's tier, and any per-role override. Returns:
//   - (rp, nil) on a successful resolution (rp may have empty APIKeyValue if keyless)
//   - (nil, nil) when no provider is configured / resolvable → opencode default
//   - (nil, error) when a provider was resolved but its API key env var is unset
//     (fail-fast per R-12; no fallback to a secondary provider per NFR-OP-02)
//
// Algorithm (business-logic-model §2.4, app-design §5):
//  1. providers empty → nil, nil (FR-006 backward compat)
//  2. RoleOverride for this role? → use override's provider + model (or default)
//  3. RoleDefinition.ModelTier non-empty? → TierStore.ResolveTier(tier, enabled)
//  4. No tier, no override: exactly one enabled provider → its default model
//  5. Nothing resolved → nil, nil (opencode default)
//  6. Env resolution: APIKeyEnv empty → keyless ResolvedProvider; else os.Getenv,
//     empty → error (fail-fast)
//
// The roleLoader function is called only if needed (step 3) to avoid loading
// role files when an override short-circuits. It returns the ModelTier string.
type RoleTierLoader func(role string) (tier string, err error)

// ResolveProvider is the load-bearing resolution function. tierStore may be nil
// (YAML-only config); roleLoader may be nil (no tier resolution possible).
func ResolveProvider(role string, providers []ProviderConfig, overrides map[string]RoleOverride, tierStore *TierStore, roleLoader RoleTierLoader) (*ResolvedProvider, error) {
	// Step 1: no providers → opencode default (FR-006).
	if len(providers) == 0 {
		return nil, nil
	}

	var provider *ProviderConfig
	var model string

	// Step 2: per-role override wins.
	if ro, ok := overrides[role]; ok && ro.ProviderName != "" {
		p := findProvider(providers, ro.ProviderName)
		if p == nil {
			return nil, fmt.Errorf("role %s references unknown provider %s", role, ro.ProviderName)
		}
		provider = p
		model = ro.ModelID
		if model == "" {
			model = p.DefaultModelID
		}
	}

	// Step 3: tier resolution.
	if provider == nil && roleLoader != nil {
		tier, err := roleLoader(role)
		if err != nil {
			return nil, fmt.Errorf("loading tier for role %s: %w", role, err)
		}
		if tier != "" && tierStore != nil {
			p, m := tierStore.ResolveTier(tier, providers)
			if p != nil {
				provider = p
				model = m
				if model == "" {
					model = p.DefaultModelID
				}
			}
		}
	}

	// Step 4: single enabled provider fallback (defensive; all roster roles have tiers).
	if provider == nil {
		var enabled []ProviderConfig
		for _, p := range providers {
			if p.Enabled {
				enabled = append(enabled, p)
			}
		}
		if len(enabled) == 1 {
			cp := enabled[0]
			provider = &cp
			model = cp.DefaultModelID
		}
	}

	// Step 5: nothing resolved → opencode default.
	if provider == nil {
		return nil, nil
	}

	// Step 6: env resolution (fail-fast, R-12).
	rp := &ResolvedProvider{
		BaseURL:   provider.BaseURL,
		Model:     model,
		APIKeyEnv: provider.APIKeyEnv,
	}
	if provider.APIKeyEnv != "" {
		envName := strings.TrimPrefix(provider.APIKeyEnv, "$")
		val := osGetenv(envName)
		if val == "" {
			return nil, fmt.Errorf("provider %s: api key env var %s is not set", provider.Name, envName)
		}
		rp.APIKeyValue = val
	}
	return rp, nil
}

func findProvider(providers []ProviderConfig, name string) *ProviderConfig {
	for i := range providers {
		if providers[i].Name == name {
			cp := providers[i]
			return &cp
		}
	}
	return nil
}