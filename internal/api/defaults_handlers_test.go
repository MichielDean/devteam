package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/settings/defaults"
	"github.com/MichielDean/devteam/internal/spec"
)

func setupDefaultsTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Pipeline: config.PipelineConfig{
			Phases: []config.PhaseConfig{{Name: "inception", Roles: []string{"pm"}}},
		},
	}
	sp := spec.NewSpecProvider(tmpDir)
	pipe := pipeline.NewPipelineWithDispatcher(cfg, sp, nil)
	database := setupTestDB(t, tmpDir)
	sp.SetDatabase(database)
	pipe.SetDatabase(database)
	s := NewServer(":0", sp, pipe, nil, feature.NewDBQuestionStore(database), database)
	ts := httptest.NewServer(s.httpServer.Handler)
	t.Cleanup(func() { ts.Close() })
	return s, ts
}

// TestGetDefaults_Empty verifies the GET response shape and the
// empty-array-not-null invariant for per_repo.
func TestGetDefaults_Empty(t *testing.T) {
	_, ts := setupDefaultsTestServer(t)

	resp, err := http.Get(ts.URL + "/api/settings/defaults")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got defaultsResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.PerRepo == nil {
		t.Error("per_repo is nil, want []")
	}
	if len(got.PerRepo) != 0 {
		t.Errorf("per_repo len = %d, want 0", len(got.PerRepo))
	}
}

// TestPutGlobalDefaults_RoundTrip verifies PUT then GET round-trips the
// global defaults and emits a FEATURE_DEFAULTS_MUTATED audit event.
func TestPutGlobalDefaults_RoundTrip(t *testing.T) {
	s, ts := setupDefaultsTestServer(t)

	body := defaultsPutRequest{Scope: "feature", Depth: "standard", TestStrategy: "unit", ExecutionMode: "human"}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/settings/defaults/global", bytes.NewReader(bodyBytes))
	req.RemoteAddr = "127.0.0.1:54321"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Verify GET returns the saved values.
	resp2, _ := http.Get(ts.URL + "/api/settings/defaults")
	defer resp2.Body.Close()
	var got defaultsResponse
	json.NewDecoder(resp2.Body).Decode(&got)
	if got.Global.Scope != "feature" {
		t.Errorf("global scope = %q, want feature", got.Global.Scope)
	}

	// Verify audit.
	_, total, _ := s.db.GetAuditEventsFiltered(AuditFilterForTest(db.AuditFeatureDefaultsMutated))
	if total != 1 {
		t.Errorf("audit events = %d, want 1", total)
	}
}

// TestPutRepoDefaults_RoundTrip verifies per-repo override round-trip.
func TestPutRepoDefaults_RoundTrip(t *testing.T) {
	_, ts := setupDefaultsTestServer(t)

	body := defaultsPutRequest{Scope: "greenfield"}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/settings/defaults/devteam", bytes.NewReader(bodyBytes))
	req.RemoteAddr = "127.0.0.1:54321"
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// GET should list the per-repo override.
	resp2, _ := http.Get(ts.URL + "/api/settings/defaults")
	defer resp2.Body.Close()
	var got defaultsResponse
	json.NewDecoder(resp2.Body).Decode(&got)
	if len(got.PerRepo) != 1 {
		t.Fatalf("per_repo len = %d, want 1", len(got.PerRepo))
	}
	if got.PerRepo[0].Repo != "devteam" || got.PerRepo[0].Scope != "greenfield" {
		t.Errorf("per_repo[0] = %+v, want {repo:devteam, scope:greenfield}", got.PerRepo[0])
	}
}

// TestDeleteRepoDefaults verifies deletion returns 204 and the override is gone.
func TestDeleteRepoDefaults(t *testing.T) {
	s, ts := setupDefaultsTestServer(t)
	// Seed a per-repo override.
	_, err := s.defaultsStore.PutForRepo(context.Background(), "devteam", defaults.Defaults{Scope: "feature"}, "operator")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/settings/defaults/devteam", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	// GET should show no per-repo overrides.
	resp2, _ := http.Get(ts.URL + "/api/settings/defaults")
	defer resp2.Body.Close()
	var got defaultsResponse
	json.NewDecoder(resp2.Body).Decode(&got)
	if len(got.PerRepo) != 0 {
		t.Errorf("per_repo len = %d, want 0 after delete", len(got.PerRepo))
	}
}

// TestDefaultsWrites_Guarded verifies unauthenticated writes are rejected.
func TestDefaultsWrites_Guarded(t *testing.T) {
	s, _ := setupDefaultsTestServer(t)

	body := defaultsPutRequest{Scope: "feature"}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPut, "/api/settings/defaults/global", bytes.NewReader(bodyBytes))
	req.RemoteAddr = "10.0.0.5:54321"
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}

	// Verify no global default was written.
	g, _ := s.defaultsStore.GetGlobal(context.Background())
	if g.Scope != "" {
		t.Errorf("global scope = %q, want empty (guard blocked)", g.Scope)
	}
}

// TestDefaultsReads_Unguarded verifies GET works without auth (FR-ROUTE-02).
func TestDefaultsReads_Unguarded(t *testing.T) {
	_, ts := setupDefaultsTestServer(t)

	resp, err := http.Get(ts.URL + "/api/settings/defaults")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (reads unguarded)", resp.StatusCode)
	}
}

// TestCreateFeature_UsesDefaults_PerRepoOverGlobal verifies the precedence
// wiring in createFeature: per-repo default overrides global default when
// no explicit value is supplied (S-DEF-01, FR-DEF-02).
func TestCreateFeature_UsesDefaults_PerRepoOverGlobal(t *testing.T) {
	s, ts := setupDefaultsTestServer(t)

	// Seed: global scope = enterprise, per-repo devteam scope = greenfield.
	_, err := s.defaultsStore.PutGlobal(context.Background(), defaults.Defaults{Scope: "enterprise", Depth: "comprehensive"}, "operator")
	if err != nil {
		t.Fatalf("PutGlobal: %v", err)
	}
	_, err = s.defaultsStore.PutForRepo(context.Background(), "devteam", defaults.Defaults{Scope: "greenfield"}, "operator")
	if err != nil {
		t.Fatalf("PutForRepo: %v", err)
	}

	// Create a feature with a repo but no explicit scope — per-repo default
	// (greenfield) should win over global (enterprise).
	body := map[string]interface{}{
		"type":        "loose_idea",
		"title":       "Test feature for defaults precedence",
		"description": "Testing per-repo over global precedence",
		"repos":       []map[string]string{{"name": "devteam", "url": "git@github.com:MichielDean/devteam.git"}},
	}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/features", bytes.NewReader(bodyBytes))
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("createFeature status = %d, want 201", resp.StatusCode)
	}
	var respBody FeatureDetailResponse
	json.NewDecoder(resp.Body).Decode(&respBody)
	if respBody.Scope != "greenfield" {
		t.Errorf("scope = %q, want greenfield (per-repo over global)", respBody.Scope)
	}
	if respBody.Depth != "comprehensive" {
		t.Errorf("depth = %q, want comprehensive (global, no per-repo override)", respBody.Depth)
	}
}

// TestCreateFeature_UsesDefaults_GlobalOverScopeDerived verifies global
// default overrides the scope-derived fallback when no explicit/per-repo
// value is supplied (S-DEF-01, FR-DEF-02). Uses a non-repo-requiring scope
// (greenfield) so the needsRepos guard doesn't reject the no-repos request.
func TestCreateFeature_UsesDefaults_GlobalOverScopeDerived(t *testing.T) {
	s, ts := setupDefaultsTestServer(t)

	_, err := s.defaultsStore.PutGlobal(context.Background(), defaults.Defaults{Scope: "greenfield"}, "operator")
	if err != nil {
		t.Fatalf("PutGlobal: %v", err)
	}

	// Create a feature with no repos and no explicit scope — global default
	// (greenfield) should win over the scope-derived detection.
	body := map[string]interface{}{
		"type":        "loose_idea",
		"title":       "Some feature",
		"description": "Some description",
	}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/features", bytes.NewReader(bodyBytes))
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var respBody FeatureDetailResponse
	json.NewDecoder(resp.Body).Decode(&respBody)
	if respBody.Scope != "greenfield" {
		t.Errorf("scope = %q, want greenfield (global over scope-derived)", respBody.Scope)
	}
}

// TestCreateFeature_ExplicitWins verifies the explicit request value wins
// over all defaults (S-DEF-01, FR-DEF-02).
func TestCreateFeature_ExplicitWins(t *testing.T) {
	s, ts := setupDefaultsTestServer(t)

	_, err := s.defaultsStore.PutGlobal(context.Background(), defaults.Defaults{Scope: "enterprise"}, "operator")
	if err != nil {
		t.Fatalf("PutGlobal: %v", err)
	}

	body := map[string]interface{}{
		"type":        "loose_idea",
		"title":       "Explicit scope test",
		"description": "desc",
		"scope":       "greenfield",
	}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/features", bytes.NewReader(bodyBytes))
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	var respBody FeatureDetailResponse
	json.NewDecoder(resp.Body).Decode(&respBody)
	if respBody.Scope != "greenfield" {
		t.Errorf("scope = %q, want greenfield (explicit wins)", respBody.Scope)
	}
}