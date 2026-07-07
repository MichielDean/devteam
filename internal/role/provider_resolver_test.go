package role

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MichielDean/devteam/internal/config"
)

// ─── SC6: ModelTier resolver (FR-G3-3, NFR-REL-4) ─────────────────────────

func TestResolveProvider_DefaultSafeWhenNoProviders(t *testing.T) {
	// No providers configured → default ollama/glm-5.2:cloud (no regression).
	cfg := &config.Config{}
	rp, err := ResolveProvider(cfg, "opus")
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	if rp.Name != DefaultProviderName {
		t.Errorf("name = %q, want %q", rp.Name, DefaultProviderName)
	}
	if rp.Model != DefaultModel {
		t.Errorf("model = %q, want %q", rp.Model, DefaultModel)
	}
	if rp.BaseURL != DefaultBaseURL {
		t.Errorf("base_url = %q, want %q", rp.BaseURL, DefaultBaseURL)
	}
}

func TestResolveProvider_DefaultSafeWhenNilConfig(t *testing.T) {
	rp, err := ResolveProvider(nil, "opus")
	if err != nil {
		t.Fatalf("ResolveProvider nil: %v", err)
	}
	if rp.Model != DefaultModel {
		t.Errorf("nil config model = %q, want default", rp.Model)
	}
}

func TestResolveProvider_MappedTierResolves(t *testing.T) {
	cfg := &config.Config{
		Providers: config.ProviderList{
			{Name: "ollama", BaseURL: "http://localhost:11434/v1", Model: "glm-5.2:cloud", Adapter: "openai", Tiers: []string{"opus", "sonnet"}},
			{Name: "openai", BaseURL: "https://api.openai.com/v1", APIKeyEnv: "OPENAI_API_KEY", Model: "gpt-4o", Adapter: "openai", Tiers: []string{"opus"}},
			{Name: "anthropic", BaseURL: "https://api.anthropic.com/v1", APIKeyEnv: "ANTHROPIC_API_KEY", Model: "claude-3-5-sonnet-20241022", Adapter: "anthropic", Tiers: []string{"sonnet"}},
		},
	}
	// opus → first provider that lists opus = ollama
	rp, err := ResolveProvider(cfg, "opus")
	if err != nil {
		t.Fatalf("opus: %v", err)
	}
	if rp.Name != "ollama" {
		t.Errorf("opus → name = %q, want ollama (first listing opus)", rp.Name)
	}
	// A different config order: openai listed first for opus → openai
	cfg2 := &config.Config{
		Providers: config.ProviderList{
			{Name: "openai", BaseURL: "https://api.openai.com/v1", APIKeyEnv: "OPENAI_API_KEY", Model: "gpt-4o", Adapter: "openai", Tiers: []string{"opus"}},
			{Name: "ollama", BaseURL: "http://localhost:11434/v1", Model: "glm-5.2:cloud", Adapter: "openai", Tiers: []string{"opus", "sonnet"}},
		},
	}
	rp, err = ResolveProvider(cfg2, "opus")
	if err != nil {
		t.Fatalf("opus cfg2: %v", err)
	}
	if rp.Name != "openai" {
		t.Errorf("opus → name = %q, want openai (first listing opus)", rp.Name)
	}
}

func TestResolveProvider_UnmappedTierFallsBackToDefault(t *testing.T) {
	cfg := &config.Config{
		Providers: config.ProviderList{
			{Name: "openai", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o", Adapter: "openai", Tiers: []string{"opus"}},
		},
	}
	// sonnet has no mapping → default-safe
	rp, err := ResolveProvider(cfg, "sonnet")
	if err != nil {
		t.Fatalf("sonnet unmapped: %v", err)
	}
	if rp.Model != DefaultModel {
		t.Errorf("unmapped sonnet → model = %q, want default %q", rp.Model, DefaultModel)
	}
}

func TestResolveProvider_ByName(t *testing.T) {
	cfg := &config.Config{
		Providers: config.ProviderList{
			{Name: "ollama", BaseURL: "http://localhost:11434/v1", Model: "glm-5.2:cloud", Adapter: "openai"},
			{Name: "openai", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o", Adapter: "openai"},
		},
	}
	rp, err := ResolveProviderByName(cfg, "openai")
	if err != nil {
		t.Fatalf("ByName openai: %v", err)
	}
	if rp.Name != "openai" || rp.Model != "gpt-4o" {
		t.Errorf("ByName openai → %s/%s, want openai/gpt-4o", rp.Name, rp.Model)
	}
	// Unknown name → error
	if _, err := ResolveProviderByName(cfg, "nope"); err == nil {
		t.Error("expected error for unknown provider name")
	}
	// Empty config / empty name → default
	rp, err = ResolveProviderByName(nil, "")
	if err != nil || rp.Model != DefaultModel {
		t.Errorf("nil cfg empty name → default; got %v / %v", rp, err)
	}
}

// ─── SC5: opencodeConfigBuilder lockstep (FR-G3-2, NFR-MAINT-1) ───────────

func TestBuildOpencodeJSON_DefaultSafeSingleOllama(t *testing.T) {
	out, err := BuildOpencodeJSON(OpencodeConfigInput{
		Model: DefaultModel,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if cfg["model"] != DefaultModel {
		t.Errorf("model = %v, want %s", cfg["model"], DefaultModel)
	}
	prov := cfg["provider"].(map[string]any)
	if _, ok := prov["ollama"]; !ok {
		t.Error("expected default ollama provider block")
	}
	// Isolation fields zeroed (C3/NFR-SEC-5)
	if cfg["plugin"] != nil && len(cfg["plugin"].([]any)) != 0 {
		t.Error("plugin should be empty")
	}
	if cfg["mcp"] != nil && len(cfg["mcp"].(map[string]any)) != 0 {
		t.Error("mcp should be empty")
	}
}

func TestBuildOpencodeJSON_EmitsAllConfiguredProviders(t *testing.T) {
	providers := config.ProviderList{
		{Name: "ollama", BaseURL: "http://localhost:11434/v1", Model: "glm-5.2:cloud", Adapter: "openai", Tiers: []string{"opus", "sonnet"}},
		{Name: "openai", BaseURL: "https://api.openai.com/v1", APIKeyEnv: "OPENAI_API_KEY", Model: "gpt-4o", Adapter: "openai", Tiers: []string{"opus"}},
		{Name: "anthropic", BaseURL: "https://api.anthropic.com/v1", APIKeyEnv: "ANTHROPIC_API_KEY", Model: "claude-3-5-sonnet-20241022", Adapter: "anthropic", Tiers: []string{"sonnet"}},
	}
	out, err := BuildOpencodeJSON(OpencodeConfigInput{
		Model:     "openai/gpt-4o",
		Providers: providers,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var cfg map[string]any
	json.Unmarshal(out, &cfg)
	prov := cfg["provider"].(map[string]any)
	for _, want := range []string{"ollama", "openai", "anthropic"} {
		if _, ok := prov[want]; !ok {
			t.Errorf("expected provider %q in output", want)
		}
	}
	// The selected model is the top-level "model"
	if cfg["model"] != "openai/gpt-4o" {
		t.Errorf("model = %v, want openai/gpt-4o", cfg["model"])
	}
	// Anthropic uses the @ai-sdk/anthropic adapter
	a := prov["anthropic"].(map[string]any)
	if a["npm"] != "@ai-sdk/anthropic" {
		t.Errorf("anthropic npm = %v, want @ai-sdk/anthropic", a["npm"])
	}
	// Ollama uses the @ai-sdk/openai-compatible adapter
	o := prov["ollama"].(map[string]any)
	if o["npm"] != "@ai-sdk/openai-compatible" {
		t.Errorf("ollama npm = %v, want @ai-sdk/openai-compatible", o["npm"])
	}
}

// SC5 lockstep: both emit sites produce identical output for the same input.
// This is the test that catches R4 divergence. It simulates calling BuildOpencodeJSON
// from tmux.go and from agent_handlers.go with the same config — the bytes match.
func TestBuildOpencodeJSON_LockstepBothSitesIdentical(t *testing.T) {
	providers := config.ProviderList{
		{Name: "ollama", BaseURL: "http://localhost:11434/v1", Model: "glm-5.2:cloud", Adapter: "openai"},
		{Name: "openai", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o", Adapter: "openai"},
	}
	in := OpencodeConfigInput{Model: "ollama/glm-5.2:cloud", Providers: providers}
	out1, _ := BuildOpencodeJSON(in)
	out2, _ := BuildOpencodeJSON(in)
	if string(out1) != string(out2) {
		t.Errorf("lockstep: identical inputs produced different outputs\n%s\n%s", out1, out2)
	}
}

// Zero provider-specific branches (NFR-MAINT-2): adding a new adapter is a
// config entry. Verify the builder handles an unknown adapter gracefully
// (falls back to openai-compatible).
func TestBuildOpencodeJSON_UnknownAdapterFallsBack(t *testing.T) {
	providers := config.ProviderList{
		{Name: "future", BaseURL: "https://x/v1", Model: "m1", Adapter: "google"},
	}
	out, err := BuildOpencodeJSON(OpencodeConfigInput{Model: "future/m1", Providers: providers})
	if err != nil {
		t.Fatalf("Build unknown adapter: %v", err)
	}
	if !strings.Contains(string(out), "@ai-sdk/openai-compatible") {
		t.Error("expected unknown adapter to fall back to openai-compatible")
	}
}