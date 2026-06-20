package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/spec"
)

// setupTestServer creates a test server with a temporary directory
func setupTestServer(t *testing.T) (*Server, string) {
	t.Helper()

	// Create a temporary directory for test data
	tmpDir, err := os.MkdirTemp("", "devteam-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create minimal config
	configPath := filepath.Join(tmpDir, "devteam.yaml")
	configContent := `version: "1"
pipeline:
  phases:
    - name: inception
      roles: [pm]
      gate: spec_approved
      artifacts: [spec_md, acceptance_md, repos_yaml]
    - name: planning
      roles: [architect]
      gate: plan_approved
      artifacts: [plan_md, tasks_md]
    - name: construction
      roles: [developer]
      gate: tasks_complete
      artifacts: []
    - name: review
      roles: [reviewer]
      gate: criteria_met
      artifacts: [review_report]
    - name: testing
      roles: [tester]
      gate: tests_pass
      artifacts: [test_report]
    - name: delivery
      roles: [ops]
      gate: docs_match_spec
      artifacts: [docs]
roles:
  pm:
    name: pm
    description: Product Manager
  architect:
    name: architect
    description: Architect
  developer:
    name: developer
    description: Developer
  reviewer:
    name: reviewer
    description: Reviewer
  tester:
    name: tester
    description: Tester
  ops:
    name: ops
    description: Operations
intake:
  loose_idea:
    description: Loose idea intake
    output: [spec_md, acceptance_md, repos_yaml]
  external_spec:
    description: External spec intake
    output: [spec_md, acceptance_md, repos_yaml]
spec_repo:
  path: specs
  specs_dir: specs
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to load config: %v", err)
	}

	specProvider := spec.NewSpecProvider(tmpDir)
	p := pipeline.NewPipeline(cfg, specProvider)

	// Create specs directory
	if err := os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create specs dir: %v", err)
	}

	server := NewServer(":0", specProvider, p, nil)

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	return server, tmpDir
}

func TestListFeaturesEmpty(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/features", nil)
	w := httptest.NewRecorder()

	// Wrap handler with middleware for proper response
	handler := server.mux
	// Apply middleware in reverse order (same as server setup)
	wrappedHandler := corsMiddleware(securityHeadersMiddleware(loggingMiddleware(recoveryMiddleware(handler))))

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp FeatureListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Features) != 0 {
		t.Errorf("expected empty features list, got %d", len(resp.Features))
	}
}

func TestGetFeatureNotFound(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/features/nonexistent", nil)
	w := httptest.NewRecorder()

	handler := corsMiddleware(securityHeadersMiddleware(loggingMiddleware(recoveryMiddleware(server.mux))))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCreateFeatureValidation(t *testing.T) {
	server, _ := setupTestServer(t)
	handler := corsMiddleware(securityHeadersMiddleware(loggingMiddleware(recoveryMiddleware(server.mux))))

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "empty title",
			body:       `{"type":"loose_idea","title":"","description":"Test desc","priority":2}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "empty_title",
		},
		{
			name:       "empty description",
			body:       `{"type":"loose_idea","title":"Test","description":"","priority":2}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "empty_description",
		},
		{
			name:       "invalid priority",
			body:       `{"type":"loose_idea","title":"Test","description":"Test desc","priority":5}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_priority",
		},
		{
			name:       "invalid type",
			body:       `{"type":"invalid","title":"Test","description":"Test desc","priority":2}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			var errResp ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("failed to parse error response: %v", err)
			}
			if errResp.Error != tt.wantCode {
				t.Errorf("expected error code %q, got %q", tt.wantCode, errResp.Error)
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	// Verify the server can start and the mux is configured
	server, _ := setupTestServer(t)
	if server == nil {
		t.Fatal("server should not be nil")
	}
}

func TestGetArtifactNotFound(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/features/nonexistent/artifacts/spec", nil)
	w := httptest.NewRecorder()

	handler := corsMiddleware(securityHeadersMiddleware(loggingMiddleware(recoveryMiddleware(server.mux))))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestRecirculateFeatureNotFound(t *testing.T) {
	server, _ := setupTestServer(t)

	// Test with a feature that doesn't exist - the handler validates body first
	body := `{"target_phase": "inception"}`
	req := httptest.NewRequest(http.MethodPost, "/api/features/nonexistent/recirculate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler := corsMiddleware(securityHeadersMiddleware(loggingMiddleware(recoveryMiddleware(server.mux))))
	handler.ServeHTTP(w, req)

	// Feature not found returns 404
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for nonexistent feature, got %d", w.Code)
	}
}

func TestCancelFeatureNotFound(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/features/nonexistent/cancel", nil)
	w := httptest.NewRecorder()

	handler := corsMiddleware(securityHeadersMiddleware(loggingMiddleware(recoveryMiddleware(server.mux))))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestEvaluateGateNotFound(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/features/nonexistent/gate", nil)
	w := httptest.NewRecorder()

	handler := corsMiddleware(securityHeadersMiddleware(loggingMiddleware(recoveryMiddleware(server.mux))))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestMiddlewareSecurityHeaders(t *testing.T) {
	// Test that security headers are applied
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Security-Policy") == "" {
		t.Error("expected Content-Security-Policy header")
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected X-Content-Type-Options: nosniff")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("expected X-Frame-Options: DENY")
	}
	if w.Header().Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Error("expected Referrer-Policy: strict-origin-when-cross-origin")
	}
}

func TestMiddlewareCORSPreflight(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204 for OPTIONS, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected Access-Control-Allow-Origin: *")
	}
}

func TestMiddlewareRecovery(t *testing.T) {
	handler := recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for panic, got %d", w.Code)
	}
}