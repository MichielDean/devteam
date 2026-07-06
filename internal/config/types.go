package config

// ProviderConfig is the aggregate root for a configured LLM provider.
// Persisted in the `providers` table (migration 017); child ProviderModel rows
// live in `provider_models`. The raw API key value is NEVER stored here — only
// the `$VAR` reference in APIKeyEnv. See business-logic-model §1.2, ADR-003.
type ProviderConfig struct {
	Name            string         `json:"name"`
	DisplayName     string         `json:"display_name"`
	Enabled         bool           `json:"enabled"`
	BaseURL         string         `json:"base_url"`
	APIKeyEnv       string         `json:"api_key_env"` // "$VAR" reference or "" (never the raw key)
	DefaultModelID  string         `json:"default_model_id"`
	NPMAdapter      string         `json:"npm_adapter"`
	EnvVarSupported bool           `json:"env_var_supported"`
	PresetID        string         `json:"preset_id"`
	Models          []ProviderModel `json:"models"`

	// KeyState is derived at read time (never persisted): "set" | "not_set" | "not_required".
	// "not_required" when APIKeyEnv == "" (e.g. Copilot, local keyless providers).
	KeyState string `json:"key_state,omitempty"`
}

// ProviderModel is a model offered by a provider. Part of the ProviderConfig
// aggregate (cascade-deleted with the parent). Identity: (ProviderName, ModelID).
type ProviderModel struct {
	ProviderName string `json:"-"` // not serialized in the provider's models list (redundant)
	ModelID      string `json:"model_id"`
	FriendlyName string `json:"friendly_name"`
}

// TierModel maps a logical tier (e.g. "opus", "sonnet") to a concrete model for
// a specific provider. A tier maps to one model PER provider; dispatch picks one
// provider via ResolveTier (first enabled provider with a tier row, alphabetical).
// See business-logic-model §1.2, app-design §5.1.
type TierModel struct {
	Tier         string `json:"tier"`
	ProviderName string `json:"provider_name"`
	ModelID      string `json:"model_id"`
}

// RoleOverride is an explicit per-role provider+model assignment that wins over
// tier resolution. Removing the override (provider="") reverts to tier resolution.
// See business-logic-model §2.3, app-design §4.1.
type RoleOverride struct {
	Role         string `json:"role"`
	ProviderName string `json:"provider_name"`
	ModelID      string `json:"model_id"`
}

// ResolvedProvider is the runtime value object produced by ResolveProvider and
// consumed by the dispatch materialization layer (writeOpencodeJSON, buildAgentEnv).
// It is NEVER persisted: the APIKeyValue lives only in this struct, the per-session
// opencode.json (mode 0600), and the tmux session env. See business-logic-model §1.2.
type ResolvedProvider struct {
	BaseURL    string
	Model      string
	APIKeyEnv  string // the "$VAR" reference (with $), or ""
	APIKeyValue string // the resolved env-var value (lives only here + 0600 opencode.json + tmux env)
}

// KeyState derived values (business-logic-model §1.2 derived field).
const (
	KeyStateSet        = "set"
	KeyStateNotSet     = "not_set"
	KeyStateNotRequired = "not_required"
)

// DeriveKeyState returns the key state for a provider based on its APIKeyEnv.
// "not_required" if APIKeyEnv is empty; "set" if the env var is non-empty;
// "not_set" if the env var is unset/empty. Never returns the raw value.
func DeriveKeyState(apiKeyEnv string) string {
	if apiKeyEnv == "" {
		return KeyStateNotRequired
	}
	envName := stripDollar(apiKeyEnv)
	if envName == "" {
		return KeyStateNotRequired
	}
	if osGetenv(envName) != "" {
		return KeyStateSet
	}
	return KeyStateNotSet
}

// osGetenv is a package-level indirection for os.Getenv, allowing tests to
// inject a stub without touching process env. Set via SetEnvGetter in tests.
var osGetenv = func(name string) string { return "" }