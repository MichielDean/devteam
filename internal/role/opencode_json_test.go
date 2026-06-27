package role

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichielDean/devteam/internal/config"
)

// AC-001 [US-1] [CON-002] provider set → opencode.json declares base_url + model.
func TestWriteOpencodeJSON_WithProvider(t *testing.T) {
	dir := t.TempDir()
	rp := &config.ResolvedProvider{
		BaseURL:     "http://x",
		Model:       "m1",
		APIKeyEnv:   "TESTPROV_K",
		APIKeyValue: "sk-test",
	}
	if err := writeOpencodeJSON(dir, rp); err != nil {
		t.Fatalf("writeOpencodeJSON: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "opencode.json"))
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}
	var oc map[string]any
	if err := json.Unmarshal(data, &oc); err != nil {
		t.Fatalf("parse opencode.json: %v\nbody: %s", err, string(data))
	}
	// model field = "devteam/m1"
	if m, _ := oc["model"].(string); m != "devteam/m1" {
		t.Errorf("model = %q, want devteam/m1", m)
	}
	prov, ok := oc["provider"].(map[string]any)
	if !ok {
		t.Fatalf("provider section missing or wrong type, got %T", oc["provider"])
	}
	devteam, ok := prov["devteam"].(map[string]any)
	if !ok {
		t.Fatalf("provider.devteam missing, got %v", prov)
	}
	opts, _ := devteam["options"].(map[string]any)
	if opts == nil {
		t.Fatal("options missing")
	}
	if bu, _ := opts["baseURL"].(string); bu != "http://x" {
		t.Errorf("baseURL = %q, want http://x", bu)
	}
	if ak, _ := opts["apiKey"].(string); ak != "sk-test" {
		t.Errorf("apiKey = %q, want sk-test", ak)
	}
	models, _ := devteam["models"].(map[string]any)
	if _, ok := models["m1"]; !ok {
		t.Errorf("models missing m1, got %v", models)
	}
}

// AC-003 / AC-017 [US-4] [CON-010, FR-006] nil provider → no override.
func TestWriteOpencodeJSON_NilProviderNoOverride(t *testing.T) {
	dir := t.TempDir()
	if err := writeOpencodeJSON(dir, nil); err != nil {
		t.Fatalf("writeOpencodeJSON: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "opencode.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var oc map[string]any
	if err := json.Unmarshal(data, &oc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, string(data))
	}
	if _, present := oc["model"]; present {
		t.Errorf("model override present, want absent (backward compat): %v", oc["model"])
	}
	if _, present := oc["provider"]; present {
		t.Errorf("provider section present, want absent (backward compat): %v", oc["provider"])
	}
	// Should still be valid JSON with the schema key.
	if _, present := oc["$schema"]; !present {
		t.Errorf("$schema missing, got %v", oc)
	}
}

// AC-010 / AC-011 [US-2] [CON-004, FR-004] API key injected by env name in
// buildAgentEnv; config file never holds the key value (writeOpencodeJSON
// does write it into the temp opencode.json — that file lives in /tmp for
// one dispatch and is removed; it is NOT devteam.yaml). The key value must
// not leak into devteam.yaml — verified separately. Here we verify the env
// injection path.
func TestBuildAgentEnv_InjectsAPIKeyByEnvName(t *testing.T) {
	rp := &config.ResolvedProvider{
		BaseURL:     "http://x",
		Model:       "m1",
		APIKeyEnv:   "TESTPROV_AGENT_K",
		APIKeyValue: "sk-agent-test",
	}
	env := buildAgentEnv("/tmp/fake-context", "pm", rp)
	found := false
	for _, e := range env {
		if e == "TESTPROV_AGENT_K=sk-agent-test" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("env missing TESTPROV_AGENT_K=sk-agent-test; got %v", env)
	}
}

// AC-012 [US-2] when provider has no key (nil/empty), env injection is
// skipped (the fail-fast happens earlier in ResolveProvider; buildAgentEnv
// must not inject a bogus entry).
func TestBuildAgentEnv_NoInjectionWhenProviderNil(t *testing.T) {
	env := buildAgentEnv("/tmp/fake", "pm", nil)
	for _, e := range env {
		if strings.HasPrefix(e, "TESTPROV_") {
			t.Errorf("unexpected provider env var with nil provider: %s", e)
		}
	}
}

// AC-010 [US-2] [CON-004] the opencode.json file mode is 0600 (owner-only)
// because it contains the resolved API key value. This is a minimal secrets
// hygiene check.
func TestWriteOpencodeJSON_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	rp := &config.ResolvedProvider{
		BaseURL:     "http://x",
		Model:       "m1",
		APIKeyEnv:   "TESTPROV_K",
		APIKeyValue: "sk-secret",
	}
	if err := writeOpencodeJSON(dir, rp); err != nil {
		t.Fatalf("writeOpencodeJSON: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "opencode.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("opencode.json mode = %o, want 0600 (contains key value)", info.Mode().Perm())
	}
}

// AC-001 [US-1] different providers produce different opencode.json content
// (the provider/model actually written reflects the resolved config).
func TestWriteOpencodeJSON_DifferentProvidersDiffer(t *testing.T) {
	write := func(rp *config.ResolvedProvider) string {
		d := t.TempDir()
		if err := writeOpencodeJSON(d, rp); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(filepath.Join(d, "opencode.json"))
		return string(b)
	}
	a := write(&config.ResolvedProvider{BaseURL: "http://a", Model: "ma", APIKeyEnv: "KA", APIKeyValue: "ka"})
	b := write(&config.ResolvedProvider{BaseURL: "http://b", Model: "mb", APIKeyEnv: "KB", APIKeyValue: "kb"})
	if a == b {
		t.Errorf("two different providers produced identical opencode.json:\n%s", a)
	}
	if !strings.Contains(a, "http://a") || !strings.Contains(a, "ma") {
		t.Errorf("opencode.json A missing expected base_url/model:\n%s", a)
	}
	if !strings.Contains(b, "http://b") || !strings.Contains(b, "mb") {
		t.Errorf("opencode.json B missing expected base_url/model:\n%s", b)
	}
}