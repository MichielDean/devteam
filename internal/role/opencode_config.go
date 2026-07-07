package role

import (
	"encoding/json"
	"fmt"

	"github.com/MichielDean/devteam/internal/config"
)

// ─── opencodeConfigBuilder (G3-2 / NFR-MAINT-1) ───────────────────────────
//
// BuildOpencodeJSON emits the isolated opencode.json config bytes for a
// dispatched agent. It is the SHARED emit function called from BOTH
// internal/role/tmux.go:prepareContextDir AND internal/api/agent_handlers.go:
// dispatchHumanProxy — eliminating the R4 divergence (FR-G3-2, NFR-MAINT-1).
//
// The output is structurally identical regardless of caller (SC5). Both sites
// emit ALL configured providers (FR-G3-2); the provider the dispatch actually
// uses is selected by ResolveProvider(tier) and named in the top-level
// "model" field, but every configured provider is present in the "provider"
// block so the opencode process can fall back if needed.
//
// The config is fully isolated from the global harness (C3/NFR-SEC-5):
// plugins, mcp, agent, instructions, snapshot, compaction are all zeroed.

// OpencodeConfigInput is the input to BuildOpencodeJSON.
type OpencodeConfigInput struct {
	// Model is the resolved model id to use for this dispatch
	// (e.g. "ollama/glm-5.2:cloud", "openai/gpt-4o"). Required.
	Model string
	// Providers is the full list of configured providers to emit in the
	// "provider" block. May be empty — in that case the default ollama
	// provider is emitted (default-safe).
	Providers []config.ProviderConfig
}

// BuildOpencodeJSON returns the opencode.json bytes. The output is stable
// for a given input (same input → same bytes), so SC5 (both-sites lockstep)
// is testable with a golden file or a direct equality assertion.
func BuildOpencodeJSON(in OpencodeConfigInput) ([]byte, error) {
	if in.Model == "" {
		in.Model = DefaultModel
	}
	providers := in.Providers
	if len(providers) == 0 {
		// Default-safe: emit the pre-feature ollama provider only.
		providers = []config.ProviderConfig{{
			Name:      DefaultProviderName,
			BaseURL:   DefaultBaseURL,
			APIKeyEnv: "",
			Model:     "glm-5.2:cloud",
			Adapter:   DefaultAdapter,
		}}
	}

	// Build the provider block. Each provider becomes one key in "provider".
	// The npm adapter package is selected by the provider's Adapter field:
	//   "openai"    → @ai-sdk/openai-compatible
	//   "anthropic" → @ai-sdk/anthropic
	providerBlock := map[string]any{}
	for _, p := range providers {
		npm := adapterNPM(p.Adapter)
		providerBlock[p.Name] = map[string]any{
			"npm":  npm,
			"name": p.Name,
			"options": map[string]any{
				"baseURL": p.BaseURL,
			},
			"models": map[string]any{
				p.Model: map[string]any{
					"name": p.Model,
				},
			},
		}
		// If the provider needs an API key, pass it via options.apiKey.
		// The value is read from the env var at *emit* time (NFR-SEC-4) and
		// embedded in the per-session isolated config (never the global one).
		if p.APIKeyEnv != "" {
			key := osGetenv(p.APIKeyEnv)
			if key != "" {
				providerBlock[p.Name].(map[string]any)["options"].(map[string]any)["apiKey"] = key
			}
		}
	}

	cfg := map[string]any{
		"$schema":      "https://opencode.ai/config.json",
		"model":        in.Model,
		"permission":   "allow",
		"instructions": []any{},
		"plugin":       []any{},
		"compaction":   map[string]any{"enabled": false},
		"snapshot":     false,
		"mcp":          map[string]any{},
		"agent":        map[string]any{},
		"provider":     providerBlock,
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshalling opencode.json: %w", err)
	}
	return out, nil
}

// adapterNPM maps an adapter identifier to its opencode npm package.
// This is the ONE place a provider-kind identifier is legitimate (re §7.1):
// it selects the npm adapter package, not branching logic elsewhere.
func adapterNPM(adapter string) string {
	switch adapter {
	case "anthropic":
		return "@ai-sdk/anthropic"
	default: // "openai" or empty
		return "@ai-sdk/openai-compatible"
	}
}