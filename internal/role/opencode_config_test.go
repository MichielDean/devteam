package role

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MichielDean/devteam/internal/config"
)

// TestBuildOpencodeJSON_LegacyFallback verifies that a nil Provider produces the
// exact legacy ollama config (backward compat for unconfigured deployments).
func TestBuildOpencodeJSON_LegacyFallback(t *testing.T) {
	out := buildOpencodeJSON(DispatchRequest{Role: "developer"})
	if !strings.Contains(out, `"model": "ollama/glm-5.2:cloud"`) {
		t.Errorf("legacy fallback missing ollama model line:\n%s", out)
	}
	if !strings.Contains(out, `"baseURL": "http://localhost:11434/v1"`) {
		t.Errorf("legacy fallback missing ollama baseURL:\n%s", out)
	}
	// Must be valid JSON.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("legacy fallback is not valid JSON: %v\n%s", err, out)
	}
}

// TestBuildOpencodeJSON_AnthropicWithKey verifies a resolved Anthropic provider
// produces a config with the anthropic provider block, the resolved model, and
// the API key injected into options (the one place the raw key lives on disk, 0600).
func TestBuildOpencodeJSON_AnthropicWithKey(t *testing.T) {
	rp := &config.ResolvedProvider{
		BaseURL:     "https://api.anthropic.com/v1",
		Model:       "claude-opus-4",
		APIKeyEnv:   "$ANTHROPIC_API_KEY",
		APIKeyValue: "sk-ant-secret-test",
	}
	out := buildOpencodeJSON(DispatchRequest{Role: "architect", Provider: rp})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("anthropic config not valid JSON: %v\n%s", err, out)
	}
	if parsed["model"] != "anthropic/claude-opus-4" {
		t.Errorf("model = %v, want anthropic/claude-opus-4", parsed["model"])
	}
	provider, ok := parsed["provider"].(map[string]any)
	if !ok {
		t.Fatalf("provider block missing or wrong type")
	}
	anthropic, ok := provider["anthropic"].(map[string]any)
	if !ok {
		t.Fatalf("anthropic provider block missing")
	}
	opts, ok := anthropic["options"].(map[string]any)
	if !ok {
		t.Fatalf("options missing")
	}
	if opts["baseURL"] != "https://api.anthropic.com/v1" {
		t.Errorf("baseURL = %v, want https://api.anthropic.com/v1", opts["baseURL"])
	}
	// The raw key MUST be in the options (this is the 0600-file design, ADR-007).
	if opts["apiKey"] != "sk-ant-secret-test" {
		t.Errorf("apiKey = %v, want sk-ant-secret-test (resolved key injected at write time)", opts["apiKey"])
	}
}

// TestBuildOpencodeJSON_KeylessProvider verifies a keyless provider (empty APIKeyEnv)
// produces a config WITHOUT an apiKey field (no key to inject).
func TestBuildOpencodeJSON_KeylessProvider(t *testing.T) {
	rp := &config.ResolvedProvider{
		BaseURL:     "", // local ollama
		Model:       "glm-5.2:cloud",
		APIKeyEnv:   "",
		APIKeyValue: "",
	}
	out := buildOpencodeJSON(DispatchRequest{Role: "developer", Provider: rp})
	var parsed map[string]any
	json.Unmarshal([]byte(out), &parsed)
	provider := parsed["provider"].(map[string]any)
	ollama := provider["ollama"].(map[string]any)
	opts := ollama["options"].(map[string]any)
	if _, hasKey := opts["apiKey"]; hasKey {
		t.Error("keyless provider should NOT have apiKey in options")
	}
	if parsed["model"] != "ollama/glm-5.2:cloud" {
		t.Errorf("model = %v, want ollama/glm-5.2:cloud", parsed["model"])
	}
}

// TestBuildAgentMD_ModelLine verifies the agent .md model line follows the
// resolved provider (or the legacy fallback when Provider is nil).
func TestBuildAgentMD_ModelLine(t *testing.T) {
	// Legacy fallback.
	md := buildAgentMD(DispatchRequest{Role: "developer", FeatureID: "f1", Phase: "construction"})
	if !strings.Contains(md, "model: ollama/glm-5.2:cloud") {
		t.Errorf("legacy agent MD missing ollama model line:\n%s", md)
	}
	// Resolved anthropic.
	rp := &config.ResolvedProvider{
		BaseURL: "https://api.anthropic.com/v1",
		Model:   "claude-opus-4",
	}
	md = buildAgentMD(DispatchRequest{Role: "architect", FeatureID: "f1", Phase: "construction", Provider: rp})
	if !strings.Contains(md, "model: anthropic/claude-opus-4") {
		t.Errorf("agent MD missing anthropic model line:\n%s", md)
	}
}

// TestBuildAgentEnvPairs verifies the API key env pair is injected when the
// provider has a resolved key, and omitted when keyless.
func TestBuildAgentEnvPairs(t *testing.T) {
	// Nil provider → no pairs.
	pairs := buildAgentEnvPairs(DispatchRequest{Role: "developer"})
	if len(pairs) != 0 {
		t.Errorf("nil provider: got %d pairs, want 0", len(pairs))
	}
	// Keyless provider → no pairs.
	pairs = buildAgentEnvPairs(DispatchRequest{Role: "developer", Provider: &config.ResolvedProvider{APIKeyEnv: ""}})
	if len(pairs) != 0 {
		t.Errorf("keyless provider: got %d pairs, want 0", len(pairs))
	}
	// Provider with key → one pair.
	pairs = buildAgentEnvPairs(DispatchRequest{Role: "architect", Provider: &config.ResolvedProvider{
		APIKeyEnv:   "$ANTHROPIC_API_KEY",
		APIKeyValue: "sk-ant-test",
	}})
	if len(pairs) != 1 {
		t.Fatalf("provider with key: got %d pairs, want 1", len(pairs))
	}
	if pairs[0].k != "ANTHROPIC_API_KEY" {
		t.Errorf("pair key = %s, want ANTHROPIC_API_KEY (bare name, no $)", pairs[0].k)
	}
	if pairs[0].v != "sk-ant-test" {
		t.Errorf("pair value = %s, want sk-ant-test", pairs[0].v)
	}
}