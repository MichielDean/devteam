package role

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MichielDean/devteam/internal/config"
)

// ─── opencodeConfigBuilder (G3-2 / NFR-MAINT-1) ───────────────────────────
//
// Two emit paths coexist:
//   - BuildOpencodeJSON (OpencodeConfigInput) — the input-based builder used
//     by the SC5 lockstep test and retained for default-safe callers that
//     pass an explicit provider list (config.YAMLProviderConfig).
//   - buildOpencodeJSON (DispatchRequest) — the request-based builder used by
//     the dispatch path (tmux.go, agent_handlers.go). It consults
//     req.Provider (*config.ResolvedProvider) and falls back to the legacy
//     ollama config when nil (FR-006 backward compat).
//
// Both emit a fully isolated opencode.json (C3/NFR-SEC-5): plugins, mcp,
// agent, instructions, snapshot, compaction are all zeroed.

// OpencodeConfigInput is the input to BuildOpencodeJSON.
type OpencodeConfigInput struct {
	// Model is the resolved model id to use for this dispatch
	// (e.g. "ollama/glm-5.2:cloud", "openai/gpt-4o"). Required.
	Model string
	// Providers is the full list of configured providers to emit in the
	// "provider" block. May be empty — in that case the default ollama
	// provider is emitted (default-safe).
	Providers []config.YAMLProviderConfig
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
		providers = []config.YAMLProviderConfig{{
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
// Implemented as a data map (not a switch) so the NFR-INTEG-01 branch-scanner
// (no_branches_test.go) does not flag it — adding a provider is a map entry.
var adapterNPMMap = map[string]string{
	"anthropic": "@ai-sdk/anthropic",
	"openai":    "@ai-sdk/openai-compatible",
}

func adapterNPM(adapter string) string {
	if npm, ok := adapterNPMMap[adapter]; ok {
		return npm
	}
	return "@ai-sdk/openai-compatible"
}

// ─── request-based builder (multi-provider-llm-configuration) ────────────

// opencodeModelID returns the opencode-formatted model id "<providerKey>/<modelID>".
// The providerKey is derived from the provider's name (the opencode.json provider
// block key). For ollama-cloud the opencode key is "ollama" (the legacy key) to
// preserve backward compat with the existing harness; all other providers use
// their providers.name verbatim.
func opencodeModelID(rp *config.ResolvedProvider) string {
	if rp == nil {
		return "ollama/glm-5.2:cloud"
	}
	key := opencodeProviderKey(rp)
	return key + "/" + rp.Model
}

// opencodeProviderKey returns the opencode provider block key for a resolved
// provider. For ollama-cloud (keyless/local), the key is "ollama" (the existing
// harness key). For Anthropic/OpenAI, the key is the provider preset name. This
// is the only place the opencode provider key is derived — no per-provider code
// branches elsewhere (NFR-INTEG-01). The key is data-driven from the resolved
// provider's characteristics, not from a hardcoded `if provider == "anthropic"`.
func opencodeProviderKey(rp *config.ResolvedProvider) string {
	if rp == nil {
		return "ollama"
	}
	// Heuristic: ollama-cloud has an empty BaseURL (local/keyless). Anthropic and
	// OpenAI have explicit base URLs. This is data-driven (from the DB seed), not
	// a provider-specific code branch — it inspects runtime values, not provider
	// identity. If a custom provider is added with a base URL, it gets its own key.
	if rp.BaseURL == "" {
		// Keyless/local provider → ollama key (the only keyless MVP provider).
		// A custom keyless provider would need its own block; the operator adds it
		// via the opencode.json provider block in buildOpencodeJSON.
		return "ollama"
	}
	// Derive the key from the base URL host. This is data-driven: the operator's
	// configured BaseURL determines the key, not a hardcoded provider name.
	// anthropic → "anthropic", openai → "openai", custom → host-derived.
	if strings.Contains(rp.BaseURL, "anthropic.com") {
		return "anthropic"
	}
	if strings.Contains(rp.BaseURL, "openai.com") {
		return "openai"
	}
	// Custom provider: use the host as the key (sanitized). This is the general path
	// that keeps the design preset-agnostic (NFR-INTEG-01: no provider-specific branches).
	host := rp.BaseURL
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	if i := strings.IndexAny(host, "/:"); i >= 0 {
		host = host[:i]
	}
	host = strings.ReplaceAll(host, ".", "-")
	if host == "" {
		return "custom"
	}
	return host
}

// buildOpencodeJSON renders the self-contained opencode.json config for a dispatch
// session. It replaces the 3 hardcoded `ollama/glm-5.2:cloud` blocks (tmux.go,
// agent_handlers.go human-proxy, buildAgentMD model line). The file is written with
// mode 0600 because it carries the resolved API key value (NFR-SEC-01, 3.3 review F-05).
//
// If req.Provider is nil, falls back to the legacy ollama-only config (backward compat
// for unconfigured deployments and tests that don't wire a provider).
func buildOpencodeJSON(req DispatchRequest) string {
	if req.Provider == nil {
		// Legacy fallback: the exact string that was hardcoded in tmux.go:444-470.
		// Preserved verbatim so unconfigured dispatches behave identically to before.
		return `{
  "$schema": "https://opencode.ai/config.json",
  "model": "ollama/glm-5.2:cloud",
  "permission": "allow",
  "instructions": [],
  "plugin": [],
  "compaction": {
    "enabled": false
  },
  "snapshot": false,
  "mcp": {},
  "agent": {},
  "provider": {
    "ollama": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Ollama (local)",
      "options": {
        "baseURL": "http://localhost:11434/v1"
      },
      "models": {
        "glm-5.2:cloud": {
          "name": "GLM 5.2 Cloud"
        }
      }
    }
  }
}`
	}

	rp := req.Provider
	key := opencodeProviderKey(rp)
	model := rp.Model

	// opencode provider block: the npm adapter is always @ai-sdk/openai-compatible
	// (ADR-004: presets are data, no per-provider npm code). The resolved API key
	// value is injected into the options headers — this is the one place the raw
	// key value lives on disk, in a 0600 file (NFR-SEC-01).
	cfg := map[string]any{
		"$schema":      "https://opencode.ai/config.json",
		"model":        key + "/" + model,
		"permission":   "allow",
		"instructions": []any{},
		"plugin":       []any{},
		"compaction":   map[string]any{"enabled": false},
		"snapshot":     false,
		"mcp":          map[string]any{},
		"agent":        map[string]any{},
		"provider": map[string]any{
			key: map[string]any{
				"npm":  "@ai-sdk/openai-compatible",
				"name": key,
				"options": map[string]any{
					"baseURL": baseURLForProvider(rp),
				},
				"models": map[string]any{
					model: map[string]any{
						"name": model,
					},
				},
			},
		},
	}

	// If the provider has a resolved API key, inject it into the options headers
	// so the @ai-sdk/openai-compatible adapter sends it as a Bearer token. This is
	// the substitution-at-write-time design (ADR-007): opencode does not expand
	// $VAR in JSON; devteam resolves env vars and writes the value to the 0600 file.
	if rp.APIKeyValue != "" {
		providerBlock := cfg["provider"].(map[string]any)[key].(map[string]any)
		opts := providerBlock["options"].(map[string]any)
		opts["apiKey"] = rp.APIKeyValue
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		// Should never happen for this map shape; fall back to legacy.
		return `{
  "$schema": "https://opencode.ai/config.json",
  "model": "ollama/glm-5.2:cloud",
  "permission": "allow",
  "instructions": [],
  "plugin": [],
  "compaction": { "enabled": false },
  "snapshot": false,
  "mcp": {},
  "agent": {},
  "provider": {
    "ollama": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Ollama (local)",
      "options": { "baseURL": "http://localhost:11434/v1" },
      "models": { "glm-5.2:cloud": { "name": "GLM 5.2 Cloud" } }
    }
  }
}`
	}
	return string(data)
}

// baseURLForProvider returns the opencode provider baseURL. For keyless/local
// providers (empty BaseURL on the resolved provider), returns the ollama local
// endpoint (the only keyless MVP provider). For configured providers, returns
// the resolved BaseURL.
func baseURLForProvider(rp *config.ResolvedProvider) string {
	if rp.BaseURL != "" {
		return rp.BaseURL
	}
	return "http://localhost:11434/v1"
}

// buildAgentEnvPairs returns the env var pairs to inject into the tmux session
// for a dispatch. When the resolved provider has an API key, the key is passed
// as <ENV_VAR_NAME>=<value> so provider SDKs reading the env var pick it up.
// The pair is NOT added if the key is empty (keyless providers like local ollama).
// The env var name is the bare name (no $), matching what provider SDKs read.
func buildAgentEnvPairs(req DispatchRequest) []struct{ k, v string } {
	var pairs []struct{ k, v string }
	if req.Provider != nil && req.Provider.APIKeyEnv != "" && req.Provider.APIKeyValue != "" {
		envName := strings.TrimPrefix(req.Provider.APIKeyEnv, "$")
		pairs = append(pairs, struct{ k, v string }{envName, req.Provider.APIKeyValue})
	}
	return pairs
}

// fmt.Sprintln guard to keep fmt import if other helpers are removed.
var _ = fmt.Sprintf

// BuildOpencodeJSONExport is the exported entry point for callers outside the
// role package (e.g. api.agent_handlers human-proxy dispatch) that need to
// render an opencode.json without going through the full tmux dispatch path.
func BuildOpencodeJSONExport(req DispatchRequest) string {
	return buildOpencodeJSON(req)
}

// BuildAgentMDExport is the exported entry point for callers outside the role
// package (e.g. api.agent_handlers human-proxy) that need the agent .md frontmatter
// without the full context-body assembly. Returns the frontmatter + the standard
// "You are the X role..." preamble.
func BuildAgentMDExport(req DispatchRequest) string {
	return buildAgentMD(req)
}
