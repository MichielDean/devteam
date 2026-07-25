package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MichielDean/devteam/internal/db"
)

// configTestServer builds a Server wired to a fresh test DB with the provider
// config seeded. Returns the server and a cleanup func.
func configTestServer(t *testing.T) *Server {
	t.Helper()
	database, err := openTestDB()
	if err != nil {
		t.Fatalf("openTestDB: %v", err)
	}
	truncateForAPITest(database)
	seedProviderConfigForAPITest(t, database)
	srv := &Server{db: database}
	t.Cleanup(func() { database.Close() })
	return srv
}

func openTestDB() (d *db.DB, err error) {
	return db.Open(db.Config{DSN: "host=localhost port=5432 user=devteam password=devteam dbname=devteam_test_db sslmode=disable"},
		"host=localhost port=5432 user=devteam password=devteam dbname=devteam_test_db sslmode=disable")
}

func truncateForAPITest(d *db.DB) {
	tables := []string{
		"role_overrides", "tier_models", "provider_models", "providers", "features",
		"audit_events", "tmux_sessions", "bolts", "feature_stages",
		"spec_artifacts", "outcomes", "notes", "events", "questions",
		"rules", "team_knowledge", "feature_repos", "sessions",
		"phase_states", "gate_results", "recirculations",
	}
	for _, table := range tables {
		d.Conn().Exec("TRUNCATE TABLE " + table + " CASCADE")
	}
}

func seedProviderConfigForAPITest(t *testing.T, d *db.DB) {
	t.Helper()
	_, err := d.Exec(`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, worktree_dir, created_at, updated_at, recirculation_count)
		VALUES ('__platform__', 'Platform Configuration', 'construction', 'draft', 0, 'platform', 'platform', '', NOW(), NOW(), 0)
		ON CONFLICT (id) DO NOTHING`)
	if err != nil {
		t.Fatalf("seed sentinel: %v", err)
	}
	providers := []struct {
		name, display, baseURL, apiKeyEnv, defaultModel, npm, preset string
		enabled, envVar                                              int
	}{
		{"anthropic", "Anthropic", "https://api.anthropic.com/v1", "$ANTHROPIC_API_KEY", "claude-opus-4", "@ai-sdk/openai-compatible", "anthropic", 1, 1},
		{"ollama-cloud", "Ollama Cloud", "", "$OLLAMA_API_KEY", "glm-5.2:cloud", "@ai-sdk/openai-compatible", "ollama-cloud", 1, 1},
		{"openai", "OpenAI", "https://api.openai.com/v1", "$OPENAI_API_KEY", "gpt-4o", "@ai-sdk/openai-compatible", "openai", 0, 1},
		{"copilot", "GitHub Copilot", "", "", "", "@ai-sdk/openai-compatible", "copilot", 0, 0},
	}
	for _, p := range providers {
		d.Exec(`INSERT INTO providers (name, display_name, enabled, base_url, api_key_env, default_model_id, npm_adapter, env_var_supported, preset_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (name) DO NOTHING`,
			p.name, p.display, p.enabled, p.baseURL, p.apiKeyEnv, p.defaultModel, p.npm, p.envVar, p.preset)
	}
	for _, ti := range []struct{ tier, provider, model string }{
		{"opus", "anthropic", "claude-opus-4"},
		{"sonnet", "ollama-cloud", "glm-5.2:cloud"},
	} {
		d.Exec(`INSERT INTO tier_models (tier, provider_name, model_id) VALUES (?, ?, ?) ON CONFLICT (tier, provider_name) DO NOTHING`,
			ti.tier, ti.provider, ti.model)
	}
	for _, m := range []struct{ provider, model, friendly string }{
		{"anthropic", "claude-opus-4", "Claude Opus 4"},
		{"ollama-cloud", "glm-5.2:cloud", "GLM 5.2 Cloud"},
		{"openai", "gpt-4o", "GPT-4o"},
	} {
		d.Exec(`INSERT INTO provider_models (provider_name, model_id, friendly_name) VALUES (?, ?, ?) ON CONFLICT (provider_name, model_id) DO NOTHING`,
			m.provider, m.model, m.friendly)
	}
}

func doJSON(t *testing.T, srv *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	// Set RemoteAddr to localhost so adminGuard allows the PUT.
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	srv.httpServer = nil // not used; handlers call s.db directly
	serveMuxForConfig(srv, rec, req)
	return rec
}

// serveMuxForConfig dispatches to the config handlers via a fresh mux (avoids
// constructing the full Server with pipeline/specProvider dependencies).
func serveMuxForConfig(srv *Server, w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/config/providers" && r.Method == "GET":
		srv.handleGetProviders(w, r)
	case r.URL.Path == "/api/config/providers" && r.Method == "PUT":
		srv.handlePutProviders(w, r)
	case r.URL.Path == "/api/config/tiers" && r.Method == "GET":
		srv.handleGetTiers(w, r)
	case r.URL.Path == "/api/config/tiers" && r.Method == "PUT":
		srv.handlePutTiers(w, r)
	case r.URL.Path == "/api/config/role-overrides" && r.Method == "GET":
		srv.handleGetRoleOverrides(w, r)
	case r.URL.Path == "/api/config/role-overrides" && r.Method == "PUT":
		srv.handlePutRoleOverrides(w, r)
	default:
		http.NotFound(w, r)
	}
}

// TestAC009_AdminAPIProviderList verifies GET /api/config/providers returns all
// seeded providers with derived key_state (never the raw key value). Traces U-API-02.
func TestAC009_AdminAPIProviderList(t *testing.T) {
	srv := configTestServer(t)
	rec := doJSON(t, srv, "GET", "/api/config/providers", nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Providers []map[string]any `json:"providers"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Providers) != 4 {
		t.Fatalf("providers = %d, want 4", len(resp.Providers))
	}
	// Verify no raw key value field is present (NFR-SEC-01).
	for _, p := range resp.Providers {
		if _, has := p["api_key_value"]; has {
			t.Errorf("provider %v returned api_key_value (must never leak)", p["name"])
		}
		if _, has := p["key_state"]; !has {
			t.Errorf("provider %v missing key_state", p["name"])
		}
	}
}

// TestAC002_APIRejectsRawKey verifies PUT /api/config/providers rejects a raw
// key string with 400. Traces U-API-02, FR-002 acceptance b, R-09, ADR-003.
func TestAC002_APIRejectsRawKey(t *testing.T) {
	srv := configTestServer(t)
	rec := doJSON(t, srv, "PUT", "/api/config/providers", map[string]any{
		"name":          "anthropic",
		"display_name":  "Anthropic",
		"api_key_env":   "sk-ant-raw-key-value", // raw key, not a $VAR reference
	})
	if rec.Code != 400 {
		t.Fatalf("raw key: status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

// TestAC002_GETMasksKey verifies GET never returns the resolved key value, only
// the $VAR reference + derived key_state. Traces U-API-02, NFR-SEC-01.
func TestAC002_GETMasksKey(t *testing.T) {
	srv := configTestServer(t)
	rec := doJSON(t, srv, "GET", "/api/config/providers", nil)
	var resp struct {
		Providers []map[string]any `json:"providers"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	for _, p := range resp.Providers {
		// The api_key_env field IS present (it's a $VAR reference, display-safe).
		// But no field should contain the resolved value.
		for k := range p {
			if k == "api_key_value" {
				t.Errorf("GET returned api_key_value (must never leak the key)")
			}
		}
	}
}

// TestAC009_AdminAPIProviderCRUD verifies PUT then GET round-trips a provider.
func TestAC009_AdminAPIProviderCRUD(t *testing.T) {
	srv := configTestServer(t)
	// PUT a new provider.
	rec := doJSON(t, srv, "PUT", "/api/config/providers", map[string]any{
		"name":             "new-prov",
		"display_name":     "New Provider",
		"enabled":          true,
		"base_url":         "https://new.example/v1",
		"api_key_env":      "$NEW_API_KEY",
		"default_model_id": "new-model",
		"models":           []map[string]any{{"model_id": "new-model", "friendly_name": "New Model"}},
	})
	if rec.Code != 200 {
		t.Fatalf("PUT: status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	// GET and verify it's present.
	rec = doJSON(t, srv, "GET", "/api/config/providers", nil)
	var resp struct {
		Providers []map[string]any `json:"providers"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	found := false
	for _, p := range resp.Providers {
		if p["name"] == "new-prov" {
			found = true
		}
	}
	if !found {
		t.Error("new-prov not found after PUT")
	}
}

// TestAC011_AuditEventOnProviderMutation verifies a PUT emits an audit event
// for the __platform__ sentinel feature. Traces U-AUDIT-01, FR-011.
func TestAC011_AuditEventOnProviderMutation(t *testing.T) {
	srv := configTestServer(t)
	doJSON(t, srv, "PUT", "/api/config/providers", map[string]any{
		"name":          "anthropic",
		"display_name":  "Anthropic Updated",
		"enabled":       true,
		"api_key_env":   "$ANTHROPIC_API_KEY",
	})
	// Read audit_events for __platform__.
	var eventType, details string
	err := srv.db.QueryRow(
		"SELECT event_type, details FROM audit_events WHERE feature_id = '__platform__' ORDER BY created_at DESC LIMIT 1",
	).Scan(&eventType, &details)
	if err != nil {
		t.Fatalf("no audit event found: %v", err)
	}
	if eventType != "provider_config_mutated" {
		t.Errorf("event_type = %s, want provider_config_mutated", eventType)
	}
}

// TestAC010_AdminAPITierCRUD verifies GET/PUT /api/config/tiers.
func TestAC010_AdminAPITierCRUD(t *testing.T) {
	srv := configTestServer(t)
	// GET initial tiers.
	rec := doJSON(t, srv, "GET", "/api/config/tiers", nil)
	if rec.Code != 200 {
		t.Fatalf("GET tiers: status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Tiers []map[string]any `json:"tiers"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Tiers) != 2 {
		t.Fatalf("initial tiers = %d, want 2", len(resp.Tiers))
	}
	// PUT a new tier assignment (sonnet → anthropic/claude-opus-4, which is valid since
	// anthropic is enabled and claude-opus-4 is in its models).
	rec = doJSON(t, srv, "PUT", "/api/config/tiers", map[string]any{
		"tier":     "sonnet",
		"provider": "anthropic",
		"model_id": "claude-opus-4",
	})
	if rec.Code != 200 {
		t.Fatalf("PUT tiers: status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	// PUT to a disabled provider → 400.
	rec = doJSON(t, srv, "PUT", "/api/config/tiers", map[string]any{
		"tier":     "opus",
		"provider": "openai",
		"model_id": "gpt-4o",
	})
	if rec.Code != 400 {
		t.Errorf("PUT tiers to disabled provider: status = %d, want 400", rec.Code)
	}

	// PUT with unknown model → 400.
	rec = doJSON(t, srv, "PUT", "/api/config/tiers", map[string]any{
		"tier":     "opus",
		"provider": "anthropic",
		"model_id": "nonexistent-model",
	})
	if rec.Code != 400 {
		t.Errorf("PUT tiers unknown model: status = %d, want 400", rec.Code)
	}
}

// TestAC013_AdminAPIRoleOverride verifies PUT/GET /api/config/role-overrides and
// override removal (provider=""). Traces U-API-03, FR-007.
func TestAC013_AdminAPIRoleOverride(t *testing.T) {
	srv := configTestServer(t)
	// PUT an override.
	rec := doJSON(t, srv, "PUT", "/api/config/role-overrides", map[string]any{
		"role":      "architect",
		"provider":  "ollama-cloud",
		"model_id":  "glm-5.2:cloud",
	})
	if rec.Code != 200 {
		t.Fatalf("PUT override: status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	// GET and verify.
	rec = doJSON(t, srv, "GET", "/api/config/role-overrides", nil)
	var resp struct {
		RoleOverrides []map[string]any `json:"role_overrides"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	found := false
	for _, ro := range resp.RoleOverrides {
		if ro["role"] == "architect" {
			found = true
		}
	}
	if !found {
		t.Error("architect override not found after PUT")
	}
	// Remove via provider="".
	rec = doJSON(t, srv, "PUT", "/api/config/role-overrides", map[string]any{
		"role":     "architect",
		"provider": "",
	})
	if rec.Code != 200 {
		t.Fatalf("PUT remove override: status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	// GET and verify it's gone.
	rec = doJSON(t, srv, "GET", "/api/config/role-overrides", nil)
	json.Unmarshal(rec.Body.Bytes(), &resp)
	for _, ro := range resp.RoleOverrides {
		if ro["role"] == "architect" {
			t.Error("architect override should be removed")
		}
	}
}

// TestAC_AdminGuardRejectsNonLocalhost verifies the admin guard rejects non-localhost
// requests without the shared secret. Traces U-API-04, NFR-SEC-02.
func TestAC_AdminGuardRejectsNonLocalhost(t *testing.T) {
	// Build a request with a non-localhost RemoteAddr.
	body := bytes.NewBufferString(`{"name":"x"}`)
	req := httptest.NewRequest("PUT", "/api/config/providers", body)
	req.RemoteAddr = "192.168.1.5:12345" // non-localhost
	rec := httptest.NewRecorder()
	// Directly test the guard wrapping a handler.
	guarded := adminGuard(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	guarded(rec, req)
	if rec.Code != 401 {
		t.Errorf("non-localhost guard: status = %d, want 401", rec.Code)
	}
}

// TestAC_AdminGuardAllowsLocalhost verifies localhost passes the guard.
func TestAC_AdminGuardAllowsLocalhost(t *testing.T) {
	guarded := adminGuard(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
	req := httptest.NewRequest("PUT", "/api/config/providers", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	guarded(rec, req)
	if rec.Code != 200 {
		t.Errorf("localhost guard: status = %d, want 200", rec.Code)
	}
}

// TestAC_AdminGuardAcceptsSharedSecret verifies the X-Admin-Secret header passes.
func TestAC_AdminGuardAcceptsSharedSecret(t *testing.T) {
	t.Setenv("ADMIN_SECRET", "test-secret-123")
	guarded := adminGuard(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	req := httptest.NewRequest("PUT", "/api/config/providers", nil)
	req.RemoteAddr = "10.0.0.5:12345" // non-localhost
	req.Header.Set("X-Admin-Secret", "test-secret-123")
	rec := httptest.NewRecorder()
	guarded(rec, req)
	if rec.Code != 200 {
		t.Errorf("shared secret guard: status = %d, want 200", rec.Code)
	}
}