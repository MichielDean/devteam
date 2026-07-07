package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProviderList_UnmarshalYAML_Sequence(t *testing.T) {
	yml := `
providers:
  - name: ollama
    base_url: "http://localhost:11434/v1"
    model: "glm-5.2:cloud"
    adapter: "openai"
    tiers: [opus, sonnet]
  - name: openai
    base_url: "https://api.openai.com/v1"
    api_key_env: "OPENAI_API_KEY"
    model: "gpt-4o"
    adapter: "openai"
    tiers: [opus]
`
	var cfg Config
	if err := yamlUnmarshal(t, yml, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cfg.Providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(cfg.Providers))
	}
	if cfg.Providers[0].Name != "ollama" {
		t.Errorf("first provider name = %q, want ollama", cfg.Providers[0].Name)
	}
	if cfg.Providers[1].APIKeyEnv != "OPENAI_API_KEY" {
		t.Errorf("second provider api_key_env = %q, want OPENAI_API_KEY", cfg.Providers[1].APIKeyEnv)
	}
}

func TestProviderList_UnmarshalYAML_EmptyMap(t *testing.T) {
	yml := `
providers: {}
`
	var cfg Config
	if err := yamlUnmarshal(t, yml, &cfg); err != nil {
		t.Fatalf("unmarshal empty map: %v", err)
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("empty map → expected 0 providers, got %d", len(cfg.Providers))
	}
}

func TestProviderList_UnmarshalYAML_Absent(t *testing.T) {
	yml := `
version: "2.0"
`
	var cfg Config
	if err := yamlUnmarshal(t, yml, &cfg); err != nil {
		t.Fatalf("unmarshal absent: %v", err)
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("absent → expected 0 providers, got %d", len(cfg.Providers))
	}
}

func TestProviderList_UnmarshalYAML_EmptyScalar(t *testing.T) {
	// `providers:` with nothing under it parses as an empty scalar.
	yml := `
version: "2.0"
providers:
`
	var cfg Config
	if err := yamlUnmarshal(t, yml, &cfg); err != nil {
		t.Fatalf("unmarshal empty scalar: %v", err)
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("empty scalar → expected 0 providers, got %d", len(cfg.Providers))
	}
}

func TestValidateConfig_ProvidersRejectsDuplicate(t *testing.T) {
	cfg := &Config{
		Providers: ProviderList{
			{Name: "x", BaseURL: "u", Model: "m", Adapter: "openai"},
			{Name: "x", BaseURL: "u2", Model: "m2", Adapter: "openai"},
		},
	}
	if err := validateConfig(cfg); err == nil {
		t.Error("expected error for duplicate provider name")
	}
}

func TestValidateConfig_ProvidersRejectsMissingFields(t *testing.T) {
	cases := []struct {
		name string
		p    ProviderConfig
	}{
		{"empty name", ProviderConfig{Name: "", BaseURL: "u", Model: "m", Adapter: "openai"}},
		{"empty base_url", ProviderConfig{Name: "x", BaseURL: "", Model: "m", Adapter: "openai"}},
		{"empty model", ProviderConfig{Name: "x", BaseURL: "u", Model: "", Adapter: "openai"}},
		{"empty adapter", ProviderConfig{Name: "x", BaseURL: "u", Model: "m", Adapter: ""}},
		{"bad adapter", ProviderConfig{Name: "x", BaseURL: "u", Model: "m", Adapter: "google"}},
	}
	for _, c := range cases {
		cfg := &Config{Providers: ProviderList{c.p}}
		if err := validateConfig(cfg); err == nil {
			t.Errorf("case %s: expected error, got nil", c.name)
		}
	}
}

func TestExpertConfig_DefaultsFalse(t *testing.T) {
	yml := `
version: "2.0"
`
	var cfg Config
	if err := yamlUnmarshal(t, yml, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Expert.AllowOffTopic {
		t.Error("default allow_off_topic should be false (hard refusal)")
	}
	if cfg.Chat.TrustMode {
		t.Error("default trust_mode should be false (confirm every mutating op)")
	}
}

func TestExpertConfig_AllowOffTopicTrue(t *testing.T) {
	yml := `
version: "2.0"
expert:
  allow_off_topic: true
chat:
  trust_mode: true
`
	var cfg Config
	if err := yamlUnmarshal(t, yml, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !cfg.Expert.AllowOffTopic {
		t.Error("expected allow_off_topic true")
	}
	if !cfg.Chat.TrustMode {
		t.Error("expected trust_mode true")
	}
}

// yamlUnmarshal is a tiny helper to reduce boilerplate.
func yamlUnmarshal(t *testing.T, yml string, cfg *Config) error {
	t.Helper()
	return yaml.NewDecoder(strings.NewReader(yml)).Decode(cfg)
}