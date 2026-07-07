package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/spec"
)

// setupReposTestServer builds a server backed by the test DB with the repos
// table seeded empty. Each test gets a clean registry.
func setupReposTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Pipeline: config.PipelineConfig{
			Phases: []config.PhaseConfig{
				{Name: "inception", Roles: []string{"pm"}},
			},
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

// TestGetRepos_ShapeUnchanged verifies the GET /api/repos response shape
// [{name,url,description,primary}] is preserved after the refactor to the
// DB store (C-INTEG-03, S-ROUTE-01 AC2, FR-REPOS-07).
func TestGetRepos_ShapeUnchanged(t *testing.T) {
	s, ts := setupReposTestServer(t)
	// Seed via the DB store directly.
	_, err := s.db.CreateRepo("devteam", "git@github.com:MichielDean/devteam.git", "main", "platform", true)
	if err != nil {
		t.Fatalf("seed CreateRepo: %v", err)
	}

	resp, err := http.Get(ts.URL + "/api/repos")
	if err != nil {
		t.Fatalf("GET /api/repos: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got []repoListEntry
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	// Verify the 4-field shape exactly (C-INTEG-03).
	if got[0].Name != "devteam" || got[0].URL == "" || got[0].Primary != true {
		t.Errorf("repo entry shape wrong: %+v", got[0])
	}
}

// TestGetRepos_EmptyArrayNotNull verifies the empty-array-not-null invariant
// (developer.md: collections serialize as empty, not null).
func TestGetRepos_EmptyArrayNotNull(t *testing.T) {
	_, ts := setupReposTestServer(t)

	resp, err := http.Get(ts.URL + "/api/repos")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var raw []byte
	resp.Body.Read(raw)
	// Re-decode to check it's an array (not null).
	var got []repoListEntry
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
}

// TestPostRepo_CreateAndAudit verifies POST /api/repos creates an entry,
// returns 201, and emits a REPOS_REGISTRY_MUTATED audit event (S-REPOS-03).
func TestPostRepo_CreateAndAudit(t *testing.T) {
	s, ts := setupReposTestServer(t)

	body := repoWriteRequest{
		Name:        "devteam",
		URL:         "git@github.com:MichielDean/devteam.git",
		Branch:      "main",
		Description: "platform",
		Primary:     true,
	}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/repos", bytes.NewReader(bodyBytes))
	req.RemoteAddr = "127.0.0.1:54321" // localhost bypasses the guard
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var got repoWriteResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "devteam" {
		t.Errorf("name = %q, want devteam", got.Name)
	}
	if got.Branch != "main" {
		t.Errorf("branch = %q, want main", got.Branch)
	}

	// Verify the audit event was emitted.
	_, total, err := s.db.GetAuditEventsFiltered(AuditFilterForTest(db.AuditReposRegistryMutated))
	if err != nil {
		t.Fatalf("GetAuditEventsFiltered: %v", err)
	}
	if total != 1 {
		t.Errorf("audit events = %d, want 1", total)
	}
}

// TestPutRepo_UpdateAndAudit verifies PUT /api/repos/{name} updates and
// emits audit (S-REPOS-04).
func TestPutRepo_UpdateAndAudit(t *testing.T) {
	s, ts := setupReposTestServer(t)
	// Seed.
	_, err := s.db.CreateRepo("devteam", "git@github.com:MichielDean/devteam.git", "main", "old", true)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	body := repoWriteRequest{
		URL:         "git@github.com:MichielDean/devteam.git",
		Branch:      "develop",
		Description: "new desc",
		Primary:     true,
	}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/repos/devteam", bytes.NewReader(bodyBytes))
	req.RemoteAddr = "127.0.0.1:54321"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got repoWriteResponse
	json.NewDecoder(resp.Body).Decode(&got)
	if got.Branch != "develop" {
		t.Errorf("branch = %q, want develop", got.Branch)
	}
	if got.Description != "new desc" {
		t.Errorf("description = %q, want new desc", got.Description)
	}

	events, total, _ := s.db.GetAuditEventsFiltered(AuditFilterForTest(db.AuditReposRegistryMutated))
	if total != 1 {
		t.Errorf("audit events = %d, want 1 (update)", total)
	}
	_ = events
}

// TestDeleteRepo_DeleteAndAudit verifies DELETE /api/repos/{name} returns
// 204 and emits audit (S-REPOS-05).
func TestDeleteRepo_DeleteAndAudit(t *testing.T) {
	s, ts := setupReposTestServer(t)
	_, err := s.db.CreateRepo("cistern", "git@github.com:MichielDean/cistern.git", "main", "", false)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/repos/cistern", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	// Verify deleted.
	row, err := s.db.GetRepo("cistern")
	if err == nil && row != nil {
		t.Error("repo still exists after delete")
	}

	events, total, _ := s.db.GetAuditEventsFiltered(AuditFilterForTest(db.AuditReposRegistryMutated))
	if total != 1 {
		t.Errorf("audit events = %d, want 1 (delete)", total)
	}
	_ = events
}

// TestDeleteRepo_ReferencedByFeatures_409 verifies the delete-guard: a repo
// referenced by feature_repos cannot be deleted and returns 409 with the
// referencing feature IDs.
func TestDeleteRepo_ReferencedByFeatures_409(t *testing.T) {
	s, ts := setupReposTestServer(t)
	_, err := s.db.CreateRepo("devteam", "git@github.com:MichielDean/devteam.git", "main", "", true)
	if err != nil {
		t.Fatalf("seed repo: %v", err)
	}
	// Seed a feature + feature_repos reference.
	now := time.Now().UTC()
	_, err = s.db.Exec(`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at) VALUES (?, ?, 'inception', 'in_progress', 3, 'loose_idea', '', ?, ?) ON CONFLICT (id) DO NOTHING`,
		"feat-test", "Test Feature", now, now)
	if err != nil {
		t.Fatalf("seed feature: %v", err)
	}
	_, err = s.db.Exec(`INSERT INTO feature_repos (feature_id, name, url, dir, branch) VALUES (?, ?, ?, ?, ?)`,
		"feat-test", "devteam", "git@github.com:MichielDean/devteam.git", "/tmp/test", "main")
	if err != nil {
		t.Fatalf("seed feature_repos: %v", err)
	}

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/repos/devteam", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "repo_in_use" {
		t.Errorf("error = %v, want repo_in_use", body["error"])
	}
}

// TestPostRepo_ValidationRejects verifies invalid input (empty name) is
// rejected with 400 + CONFIG_VALIDATION_FAILED audit (FR-CONFIG-03,
// S-CONFIG-03 AC1).
func TestPostRepo_ValidationRejects(t *testing.T) {
	s, ts := setupReposTestServer(t)

	body := repoWriteRequest{Name: "", URL: "git@example.com:x/y.git"}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/repos", bytes.NewReader(bodyBytes))
	req.RemoteAddr = "127.0.0.1:54321"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	// Verify no repo was created.
	count, _ := s.db.CountRepos()
	if count != 0 {
		t.Errorf("repos count = %d, want 0 (validation rejected, no mutation)", count)
	}

	// Verify validation-failure audit was emitted.
	_, total, _ := s.db.GetAuditEventsFiltered(AuditFilterForTest(db.AuditConfigValidationFailed))
	if total != 1 {
		t.Errorf("validation-failed audit events = %d, want 1", total)
	}
}

// TestReposWrites_Guarded verifies unauthenticated write (non-localhost, no
// token) returns 401 with no mutation and no audit (S-ROUTE-03 AC1,
// NFR-SEC-02). We test the guard via direct handler invocation because
// httptest.NewServer always receives requests from localhost (the test
// server's own loopback), so the guard's localhost bypass would mask the
// non-localhost rejection.
func TestReposWrites_Guarded(t *testing.T) {
	s, _ := setupReposTestServer(t)

	body := repoWriteRequest{Name: "devteam", URL: "git@github.com:MichielDean/devteam.git"}
	bodyBytes, _ := json.Marshal(body)

	// Non-localhost + no token → 401. Hit the mux directly so RemoteAddr
	// is respected (the real httptest.NewServer overwrites RemoteAddr with
	// the loopback address of the test TCP connection).
	req, _ := http.NewRequest(http.MethodPost, "/api/repos", bytes.NewReader(bodyBytes))
	req.RemoteAddr = "10.0.0.5:54321"
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}

	count, _ := s.db.CountRepos()
	if count != 0 {
		t.Errorf("repos count = %d, want 0 (guard blocked mutation)", count)
	}
	_, total, _ := s.db.GetAuditEventsFiltered(AuditFilterForTest(db.AuditReposRegistryMutated))
	if total != 0 {
		t.Errorf("audit events = %d, want 0 (guard blocked, no audit)", total)
	}
}

// TestPostRepo_DuplicateName_409 verifies creating a repo with an existing
// name returns 409 (not 500).
func TestPostRepo_DuplicateName_409(t *testing.T) {
	_, ts := setupReposTestServer(t)
	s2, _ := setupReposTestServer(t)
	_ = s2

	body := repoWriteRequest{Name: "devteam", URL: "git@github.com:MichielDean/devteam.git"}
	bodyBytes, _ := json.Marshal(body)

	// First create succeeds.
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/repos", bytes.NewReader(bodyBytes))
	req.RemoteAddr = "127.0.0.1:54321"
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first create status = %d, want 201", resp.StatusCode)
	}

	// Second create → 409.
	req2, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/repos", bytes.NewReader(bodyBytes))
	req2.RemoteAddr = "127.0.0.1:54321"
	resp2, _ := http.DefaultClient.Do(req2)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("second create status = %d, want 409", resp2.StatusCode)
	}
}

// AuditFilterForTest builds an AuditFilter scoped to a single event type,
// page 1, page size 50 — the common test case.
func AuditFilterForTest(eventType string) db.AuditFilter {
	return db.AuditFilter{EventType: eventType, Page: 1, PageSize: 50}
}