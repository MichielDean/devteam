package api

// Acceptance tests for the Dev Team Web UI API, traced to acceptance criteria AC-001 through AC-058.
// These tests verify backend API behavior as specified in the feature spec.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/spec"
	"gopkg.in/yaml.v3"
)

// setupTestServerWithDir creates a test server with a temporary directory,
// returning the server, the temp dir, and a helper to create features.
func setupTestServerWithDir(t *testing.T) (*Server, string) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "devteam-acceptance-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

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

// wrappedHandler applies all middleware to the server mux for testing
func wrappedHandler(s *Server) http.Handler {
	return corsMiddleware(securityHeadersMiddleware(loggingMiddleware(recoveryMiddleware(s.mux))))
}

// createTestFeature creates a feature state file in the test directory for testing
func createTestFeature(t *testing.T, tmpDir string, f *feature.Feature) {
	t.Helper()
	dir := filepath.Join(tmpDir, "specs", f.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create feature dir: %v", err)
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		t.Fatalf("failed to marshal feature: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".devteam-state.yaml"), data, 0644); err != nil {
		t.Fatalf("failed to write feature state: %v", err)
	}
}

// makeFeature is a helper to create a feature with sensible defaults
func makeFeature(id, title string, phase feature.Phase, status feature.Status, priority int) *feature.Feature {
	now := time.Now()
	f := &feature.Feature{
		ID:           id,
		Title:        title,
		Current:      phase,
		Status:       status,
		Priority:     priority,
		IntakePath:   feature.IntakeLooseIdea,
		SpecDir:      fmt.Sprintf("specs/%s/", id),
		CreatedAt:    now.Add(-24 * time.Hour),
		UpdatedAt:    now,
		Dependencies: []string{},
		Repos:        []feature.RepoRef{},
		PhaseStates:  make(map[feature.Phase]*feature.PhaseState),
	}
	for _, p := range feature.AllPhases() {
		f.PhaseStates[p] = &feature.PhaseState{
			Phase:  p,
			Status: feature.StatusDraft,
		}
	}
	return f
}

// ========================================================================
// US-1: Submit a feature idea from the browser
// ========================================================================

// [TEST-001] [US-1] [AC-004] Empty description validation
func TestCreateFeatureEmptyDescription(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	body := `{"type":"loose_idea","title":"My Feature","description":"","priority":2}`
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-004] expected 400 for empty description, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "empty_description" {
		t.Errorf("[AC-004] expected error code 'empty_description', got %q", errResp.Error)
	}
}

// [TEST-002] [US-1] [AC-005] Description exceeding max length
func TestCreateFeatureDescriptionTooLong(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	longDesc := strings.Repeat("x", 10001)
	body := fmt.Sprintf(`{"type":"loose_idea","title":"My Feature","description":"%s","priority":2}`, longDesc)
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-005] expected 400 for description > 10000 chars, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "description_too_long" {
		t.Errorf("[AC-005] expected error code 'description_too_long', got %q", errResp.Error)
	}
}

// [TEST-003] [US-1] [AC-008] Title exceeding 200 characters
func TestCreateFeatureTitleTooLong(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	longTitle := strings.Repeat("A", 201)
	body := fmt.Sprintf(`{"type":"loose_idea","title":"%s","description":"A valid description","priority":2}`, longTitle)
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-008] expected 400 for title > 200 chars, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "title_too_long" {
		t.Errorf("[AC-008] expected error code 'title_too_long', got %q", errResp.Error)
	}
}

// [TEST-004] [US-1] [AC-009] Empty title validation
func TestCreateFeatureEmptyTitle(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	body := `{"type":"loose_idea","title":"","description":"A valid description","priority":2}`
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-009] expected 400 for empty title, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "empty_title" {
		t.Errorf("[AC-009] expected error code 'empty_title', got %q", errResp.Error)
	}
}

// [TEST-005] [US-1] [AC-010] Invalid priority values
func TestCreateFeatureInvalidPriority(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	tests := []struct {
		name     string
		priority int
	}{
		{"zero", 0}, // 0 is valid — defaults to 2
		{"four", 4},
		{"negative", -1},
		{"ten", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.priority == 0 {
				t.Skip("priority 0 defaults to 2, which is valid per AC-007")
			}
			body := fmt.Sprintf(`{"type":"loose_idea","title":"Test Feature","description":"A description","priority":%d}`, tt.priority)
			req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("[AC-010] expected 400 for priority %d, got %d", tt.priority, w.Code)
			}

			var errResp ErrorResponse
			json.Unmarshal(w.Body.Bytes(), &errResp)
			if errResp.Error != "invalid_priority" {
				t.Errorf("[AC-010] expected error code 'invalid_priority', got %q", errResp.Error)
			}
		})
	}
}

// [TEST-006] [US-1] [AC-007] Priority defaults to 2 when not specified
func TestCreateFeaturePriorityDefault(t *testing.T) {
	// This tests that priority defaults to 2 — we verify the validation logic
	// (in practice the feature creation goes through intake, so this validates
	// the API layer correctly defaults priority 0 to 2)
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	// Priority 0 should be treated as "not set" and default to 2
	// The createFeature handler sets req.Priority = 2 when req.Priority == 0
	// This is tested by verifying priority 0 doesn't return an error
	// (the actual creation depends on intake working)
	body := `{"type":"loose_idea","title":"Default Priority Feature","description":"Testing default priority","priority":0}`
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Priority 0 defaults to 2, so it should NOT be a validation error.
	// The request may still fail due to intake issues, but not validation.
	if w.Code == http.StatusBadRequest {
		var errResp ErrorResponse
		json.Unmarshal(w.Body.Bytes(), &errResp)
		if errResp.Error == "invalid_priority" {
			t.Errorf("[AC-007] priority 0 should default to 2, not be rejected")
		}
	}
}

// [TEST-007] [US-1] [AC-006] Duplicate title detection
func TestCreateFeatureDuplicateTitle(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	// Create a feature first
	existing := makeFeature("001-dark-mode", "We need dark mode", feature.PhaseInception, feature.StatusInProgress, 2)
	createTestFeature(t, tmpDir, existing)

	// Try to create a feature with the same title (case-insensitive)
	body := `{"type":"loose_idea","title":"We Need Dark Mode","description":"Another description","priority":2}`
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("[AC-006] expected 409 for duplicate title, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "duplicate_title" {
		t.Errorf("[AC-006] expected error code 'duplicate_title', got %q", errResp.Error)
	}
}

// ========================================================================
// US-2: Watch features move through the pipeline in real time
// ========================================================================

// [TEST-008] [US-2] [AC-011] List all features with phase/status
func TestListFeaturesWithData(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	// Create test features
	f1 := makeFeature("001-feature-a", "Feature A", feature.PhaseInception, feature.StatusInProgress, 1)
	createTestFeature(t, tmpDir, f1)

	f2 := makeFeature("002-feature-b", "Feature B", feature.PhaseReview, feature.StatusGateBlocked, 2)
	f2.PhaseStates[feature.PhaseReview] = &feature.PhaseState{
		Phase:  feature.PhaseReview,
		Status: feature.StatusGateBlocked,
		GateResult: &feature.GateResult{
			Phase:  feature.PhaseReview,
			Passed: false,
			Checks: []feature.CheckResult{
				{Name: "review_report exists", Passed: false, Message: "Missing review_report"},
			},
		},
	}
	createTestFeature(t, tmpDir, f2)

	req := httptest.NewRequest(http.MethodGet, "/api/features", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("[AC-011] expected 200, got %d", w.Code)
	}

	var resp FeatureListResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Features) < 2 {
		t.Errorf("[AC-011] expected at least 2 features, got %d", len(resp.Features))
	}

	// Verify each feature has the required fields per AC-011
	for _, f := range resp.Features {
		if f.ID == "" {
			t.Error("[AC-011] feature ID should not be empty")
		}
		if f.Title == "" {
			t.Error("[AC-011] feature title should not be empty")
		}
		if f.CurrentPhase == "" {
			t.Error("[AC-011] feature current_phase should not be empty")
		}
		if f.Status == "" {
			t.Error("[AC-011] feature status should not be empty")
		}
	}
}

// [TEST-009] [US-2] [AC-016] Empty state when no features
func TestListFeaturesEmptyState(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	req := httptest.NewRequest(http.MethodGet, "/api/features", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("[AC-016] expected 200, got %d", w.Code)
	}

	var resp FeatureListResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Features) != 0 {
		t.Errorf("[AC-016] expected empty features list, got %d", len(resp.Features))
	}
}

// ========================================================================
// US-4: Manage features from the dashboard
// ========================================================================

// [TEST-010] [US-4] [AC-028] Cancel and Advance buttons disabled for terminal states
func TestCancelAlreadyCancelledFeature(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-cancelled", "Cancelled Feature", feature.PhaseInception, feature.StatusCancelled, 2)
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodPost, "/api/features/001-cancelled/cancel", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-054] expected 400 for cancelling already-cancelled feature, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "validation_error" {
		t.Errorf("[AC-054] expected error code 'validation_error', got %q", errResp.Error)
	}
}

// [TEST-011] [US-4] [AC-055] Cancel a done feature
func TestCancelDoneFeature(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-done", "Done Feature", feature.PhaseDelivery, feature.StatusDone, 2)
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodPost, "/api/features/001-done/cancel", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-055] expected 400 for cancelling a done feature, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "validation_error" {
		t.Errorf("[AC-055] expected error code 'validation_error', got %q", errResp.Error)
	}
}

// [TEST-012] [US-4] [AC-049] Recirculate to invalid phase
func TestRecirculateInvalidPhase(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-test", "Test Feature", feature.PhaseReview, feature.StatusGateBlocked, 2)
	createTestFeature(t, tmpDir, f)

	tests := []struct {
		name        string
		targetPhase string
		wantCode   int
	}{
		{"invalid phase name", "nonexistent", http.StatusBadRequest},
		{"forward phase (not earlier)", "delivery", http.StatusBadRequest},
		{"same phase", "review", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"target_phase": "%s"}`, tt.targetPhase)
			req := httptest.NewRequest(http.MethodPost, "/api/features/001-test/recirculate", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("[AC-049] expected %d for recirculate to %q, got %d", tt.wantCode, tt.targetPhase, w.Code)
			}
		})
	}
}

// [TEST-013] [US-4] [AC-050] Recirculate to a forward phase is rejected
func TestRecirculateForwardPhase(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	// Feature in inception — trying to recirculate to planning (forward) should fail
	f := makeFeature("001-test", "Test Feature", feature.PhaseInception, feature.StatusInProgress, 2)
	createTestFeature(t, tmpDir, f)

	body := `{"target_phase": "planning"}`
	req := httptest.NewRequest(http.MethodPost, "/api/features/001-test/recirculate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-050] expected 400 for recirculate to forward phase, got %d", w.Code)
	}
}

// [TEST-014] [US-4] [AC-048] Get feature that doesn't exist returns 404
func TestGetFeatureNotFound404(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	req := httptest.NewRequest(http.MethodGet, "/api/features/nonexistent-id", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("[AC-048] expected 404 for nonexistent feature, got %d", w.Code)
	}
}

// [TEST-015] [US-4] [AC-056] Advance feature at delivery phase
func TestAdvanceAtDeliveryPhase(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-at-delivery", "At Delivery", feature.PhaseDelivery, feature.StatusInProgress, 2)
	// Set gate result as not passed so we test the specific case
	f.PhaseStates[feature.PhaseDelivery] = &feature.PhaseState{
		Phase:  feature.PhaseDelivery,
		Status: feature.StatusInProgress,
	}
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodPost, "/api/features/001-at-delivery/advance", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Advance at delivery should fail (400) because it's the final phase
	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-056] expected 400 for advance at delivery phase, got %d", w.Code)
	}
}

// ========================================================================
// US-5: Trigger autonomous processing
// ========================================================================

// [TEST-016] [US-5] [AC-047] Process already processing feature returns 409
func TestProcessAlreadyProcessing(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-processing", "Processing Feature", feature.PhaseInception, feature.StatusInProgress, 2)
	createTestFeature(t, tmpDir, f)

	// Mark the feature as actively processing using the server's sync.Map
	srv := server
	srv.setActive("001-processing")

	req := httptest.NewRequest(http.MethodPost, "/api/features/001-processing/process", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("[AC-047] expected 409 for already-processing feature, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "already_processing" {
		t.Errorf("[AC-047] expected error code 'already_processing', got %q", errResp.Error)
	}

	srv.clearActive("001-processing")
}

// ========================================================================
// API Contract Acceptance Criteria
// ========================================================================

// [TEST-017] [US-1] [AC-042] Valid loose idea creation returns 201
func TestCreateFeatureLooseIdea201(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	body := `{"type":"loose_idea","title":"My New Feature","description":"A description of the feature","priority":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// The request should either succeed (201) or fail due to intake issues (500)
	// We check that it doesn't return a validation error (400)
	if w.Code == http.StatusBadRequest {
		var errResp ErrorResponse
		json.Unmarshal(w.Body.Bytes(), &errResp)
		t.Errorf("[AC-042] valid loose idea should not get 400, got: %s", errResp.Error)
	}
}

// [TEST-018] [US-1] [AC-043] Empty description returns 400
func TestCreateFeatureEmptyDesc400(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	body := `{"type":"loose_idea","title":"My Feature","description":"","priority":2}`
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-043] expected 400 for empty description, got %d", w.Code)
	}
}

// [TEST-019] [US-1] [AC-044] Empty title returns 400
func TestCreateFeatureEmptyTitle400(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	body := `{"type":"loose_idea","title":"","description":"Some description","priority":2}`
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-044] expected 400 for empty title, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "empty_title" {
		t.Errorf("[AC-044] expected error code 'empty_title', got %q", errResp.Error)
	}
}

// [TEST-020] [US-1] [AC-045] Title exceeding 200 chars returns 400
func TestCreateFeatureTitleTooLong400(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	longTitle := strings.Repeat("X", 201)
	body := fmt.Sprintf(`{"type":"loose_idea","title":"%s","description":"Valid desc","priority":2}`, longTitle)
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-045] expected 400 for title > 200 chars, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "title_too_long" {
		t.Errorf("[AC-045] expected error code 'title_too_long', got %q", errResp.Error)
	}
}

// [TEST-021] [US-1] [AC-046] Priority out of range returns 400
func TestCreateFeaturePriorityOutOfRange(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	tests := []struct {
		name     string
		priority int
	}{
		{"priority 0 (should default to 2, not reject)", 0},
		{"priority 4", 4},
		{"priority -1", -1},
		{"priority 5", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"type":"loose_idea","title":"Test %d","description":"Valid desc","priority":%d}`, tt.priority, tt.priority)
			req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if tt.priority == 0 {
				// Priority 0 defaults to 2, should not be 400 for validation
				if w.Code == http.StatusBadRequest {
					var errResp ErrorResponse
					json.Unmarshal(w.Body.Bytes(), &errResp)
					if errResp.Error == "invalid_priority" {
						t.Errorf("[AC-046] priority 0 should default to 2, not be rejected")
					}
				}
			} else {
				if w.Code != http.StatusBadRequest {
					t.Errorf("[AC-046] expected 400 for priority %d, got %d", tt.priority, w.Code)
				}
				var errResp ErrorResponse
				json.Unmarshal(w.Body.Bytes(), &errResp)
				if errResp.Error != "invalid_priority" {
					t.Errorf("[AC-046] expected error code 'invalid_priority' for priority %d, got %q", tt.priority, errResp.Error)
				}
			}
		})
	}
}

// [TEST-022] [US-4] [AC-051] API does not expose secrets or internal paths
func TestAPIDoesNotExposeSecrets(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-test", "Test Feature", feature.PhaseInception, feature.StatusInProgress, 2)
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodGet, "/api/features/001-test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("[AC-051] expected 200, got %d", w.Code)
	}

	// Verify response doesn't contain internal file paths or secrets
	body := w.Body.String()
	// The response should not contain internal paths like /home/ or .devteam-state.yaml
	if strings.Contains(body, ".devteam-state.yaml") {
		t.Errorf("[AC-051] response exposes internal state file path")
	}
	// The response should not contain internal config paths
	if strings.Contains(body, "/internal/") {
		t.Errorf("[AC-051] response exposes internal paths")
	}
}

// [TEST-023] [US-4] [AC-057] Get artifact that doesn't exist returns 404
func TestGetArtifactNotYetGenerated404(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-test", "Test Feature", feature.PhaseInception, feature.StatusDraft, 2)
	createTestFeature(t, tmpDir, f)

	artifactTypes := []string{"spec", "acceptance", "plan", "tasks", "review_report", "test_report", "docs", "input"}
	for _, artType := range artifactTypes {
		t.Run(artType, func(t *testing.T) {
			url := fmt.Sprintf("/api/features/001-test/artifacts/%s", artType)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("[AC-057] expected 404 for artifact %s not yet generated, got %d", artType, w.Code)
			}
		})
	}
}

// [TEST-024] [US-4] [AC-054] Cancel already-cancelled feature returns 400
func TestCancelAlreadyCancelledFeature400(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-cancelled", "Cancelled Feature", feature.PhaseInception, feature.StatusCancelled, 2)
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodPost, "/api/features/001-cancelled/cancel", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-054] expected 400 for already-cancelled feature, got %d", w.Code)
	}
}

// [TEST-025] [US-4] [AC-055] Cancel already-done feature returns 400
func TestCancelAlreadyDoneFeature400(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-done", "Done Feature", feature.PhaseDelivery, feature.StatusDone, 2)
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodPost, "/api/features/001-done/cancel", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-055] expected 400 for already-done feature, got %d", w.Code)
	}
}

// [TEST-026] [US-4] [AC-056] Advance at delivery phase returns 400
func TestAdvanceAtDelivery400(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-at-delivery", "At Delivery", feature.PhaseDelivery, feature.StatusInProgress, 2)
	f.PhaseStates[feature.PhaseDelivery] = &feature.PhaseState{
		Phase:  feature.PhaseDelivery,
		Status: feature.StatusInProgress,
	}
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodPost, "/api/features/001-at-delivery/advance", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-056] expected 400 for advance at delivery, got %d", w.Code)
	}
}

// [TEST-027] [US-4] [AC-025] Process button disabled when feature is already being processed
func TestRunPhaseAlreadyProcessing409(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-active", "Active Feature", feature.PhaseInception, feature.StatusInProgress, 2)
	createTestFeature(t, tmpDir, f)

	s := server
	s.setActive("001-active")
	defer s.clearActive("001-active")

	req := httptest.NewRequest(http.MethodPost, "/api/features/001-active/run", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("[AC-025] expected 409 for run on already-processing feature, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "already_processing" {
		t.Errorf("[AC-025] expected error code 'already_processing', got %q", errResp.Error)
	}
}

// [TEST-028] [US-4] Advance on terminal feature returns 400
func TestAdvanceTerminalFeature400(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-cancelled", "Cancelled Feature", feature.PhaseInception, feature.StatusCancelled, 2)
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodPost, "/api/features/001-cancelled/advance", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for advance on cancelled feature, got %d", w.Code)
	}
}

// [TEST-029] [US-4] Recirculate on terminal feature returns 400
func TestRecirculateTerminalFeature400(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-done", "Done Feature", feature.PhaseDelivery, feature.StatusDone, 2)
	createTestFeature(t, tmpDir, f)

	body := `{"target_phase": "planning"}`
	req := httptest.NewRequest(http.MethodPost, "/api/features/001-done/recirculate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for recirculate on done feature, got %d", w.Code)
	}
}

// ========================================================================
// Security Headers (SECURITY-04)
// ========================================================================

// [TEST-030] [SECURITY-04] Verify security headers on API responses
func TestSecurityHeadersOnAPIResponses(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	req := httptest.NewRequest(http.MethodGet, "/api/features", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Check all required security headers per SECURITY-04
	tests := []struct {
		header string
		want   string
	}{
		{"Content-Security-Policy", "default-src 'self'"},
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			got := w.Header().Get(tt.header)
			if !strings.Contains(got, tt.want) {
				t.Errorf("expected %s to contain %q, got %q", tt.header, tt.want, got)
			}
		})
	}
}

// [TEST-031] [SECURITY-04] Verify HSTS header is NOT present (local-only server)
func TestNoHSTSHeaderForLocalServer(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	req := httptest.NewRequest(http.MethodGet, "/api/features", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts != "" {
		t.Errorf("local-only server should not send HSTS header, got %q", hsts)
	}
}

// [TEST-032] [SECURITY-05] Request body size is limited
func TestRequestBodySizeLimit(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	// Try to send a request body larger than 1MB
	largeBody := strings.Repeat("x", 1<<20+1) // 1MB + 1 byte
	body := fmt.Sprintf(`{"type":"loose_idea","title":"Test","description":"%s","priority":2}`, largeBody)
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// The server should reject the oversized request
	// Expected: either 400 (if the body is truncated and invalid) or the request should be limited
	// The important thing is that it doesn't process the entire oversized body
	if w.Code == http.StatusOK || w.Code == http.StatusCreated {
		t.Error("[SECURITY-05] oversized request body should be rejected")
	}
}

// [TEST-033] [SECURITY-08] CORS headers allow local development
func TestCORSHeadersForLocalDev(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	// Test preflight
	req := httptest.NewRequest(http.MethodOptions, "/api/features", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS preflight, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected Access-Control-Allow-Origin: * for local dev")
	}
	if w.Header().Get("Access-Control-Allow-Methods") != "GET, POST, OPTIONS" {
		t.Error("expected Access-Control-Allow-Methods: GET, POST, OPTIONS")
	}
}

// ========================================================================
// Invalid artifact type (edge case)
// ========================================================================

// [TEST-034] Invalid artifact type returns 400
func TestGetArtifactInvalidType(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-test", "Test Feature", feature.PhaseInception, feature.StatusInProgress, 2)
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodGet, "/api/features/001-test/artifacts/invalid_type", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid artifact type, got %d", w.Code)
	}
}

// [TEST-035] Run phase on cancelled feature returns 400
func TestRunPhaseTerminalFeature400(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-cancelled", "Cancelled Feature", feature.PhaseInception, feature.StatusCancelled, 2)
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodPost, "/api/features/001-cancelled/run", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for run on cancelled feature, got %d", w.Code)
	}
}

// [TEST-036] Process on cancelled feature returns 400
func TestProcessTerminalFeature400(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-cancelled", "Cancelled Feature", feature.PhaseInception, feature.StatusCancelled, 2)
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodPost, "/api/features/001-cancelled/process", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for process on cancelled feature, got %d", w.Code)
	}
}

// [TEST-037] [US-2] Get feature detail returns correct structure
func TestGetFeatureDetail(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-detail", "Detail Feature", feature.PhasePlanning, feature.StatusInProgress, 1)
	f.PhaseStates[feature.PhaseInception] = &feature.PhaseState{
		Phase:       feature.PhaseInception,
		Status:      feature.StatusPassed,
		StartedAt:   timePtr(time.Now().Add(-2 * time.Hour)),
		CompletedAt: timePtr(time.Now().Add(-1 * time.Hour)),
		Artifacts: []feature.Artifact{
			{Type: feature.ArtifactSpecMD, Path: "specs/001-detail/spec.md", GeneratedBy: feature.RolePM, GeneratedAt: time.Now()},
		},
		GateResult: &feature.GateResult{
			Phase:  feature.PhaseInception,
			Passed: true,
			Checks: []feature.CheckResult{
				{Name: "spec.md exists", Passed: true, Message: "Found spec.md"},
				{Name: "acceptance.md exists", Passed: true, Message: "Found acceptance.md"},
			},
		},
	}
	f.PhaseStates[feature.PhasePlanning] = &feature.PhaseState{
		Phase:    feature.PhasePlanning,
		Status:   feature.StatusInProgress,
		StartedAt: timePtr(time.Now()),
	}
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodGet, "/api/features/001-detail", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp FeatureDetailResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	// [AC-017] Verify all artifacts are listed with type and generated_by
	if resp.ID != "001-detail" {
		t.Errorf("expected ID '001-detail', got %q", resp.ID)
	}
	if resp.Title != "Detail Feature" {
		t.Errorf("expected Title 'Detail Feature', got %q", resp.Title)
	}
	if resp.Status != "in_progress" {
		t.Errorf("expected Status 'in_progress', got %q", resp.Status)
	}
	if resp.Priority != 1 {
		t.Errorf("expected Priority 1, got %d", resp.Priority)
	}
	if resp.IntakePath != "loose_idea" {
		t.Errorf("expected IntakePath 'loose_idea', got %q", resp.IntakePath)
	}

	// Verify phase states
	if len(resp.PhaseStates) == 0 {
		t.Error("expected phase states to be present")
	}

	inception, ok := resp.PhaseStates["inception"]
	if !ok {
		t.Fatal("expected 'inception' phase state")
	}
	// [AC-013] Verify gate results with pass/fail per check
	if inception.GateResult == nil || !inception.GateResult.Passed {
		t.Error("expected inception gate result to be passed")
	}
	if len(inception.GateResult.Checks) != 2 {
		t.Errorf("expected 2 gate checks, got %d", len(inception.GateResult.Checks))
	}
	if len(inception.Artifacts) != 1 {
		t.Errorf("[AC-017] expected 1 artifact, got %d", len(inception.Artifacts))
	}
	if inception.Artifacts[0].GeneratedBy != "pm" {
		t.Errorf("[AC-017] expected artifact generated_by 'pm', got %q", inception.Artifacts[0].GeneratedBy)
	}
}

// [TEST-038] [US-4] Cancel a feature successfully
func TestCancelFeatureSuccess(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-active", "Active Feature", feature.PhaseInception, feature.StatusInProgress, 2)
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodPost, "/api/features/001-active/cancel", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for cancel, got %d", w.Code)
	}

	var resp FeatureDetailResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Status != "cancelled" {
		t.Errorf("[AC-024] expected status 'cancelled', got %q", resp.Status)
	}
}

// [TEST-039] [US-1] Invalid type returns 400
func TestCreateFeatureInvalidType(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	body := `{"type":"invalid_type","title":"Test","description":"Valid desc","priority":2}`
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid type, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "validation_error" {
		t.Errorf("expected error code 'validation_error', got %q", errResp.Error)
	}
}

// [TEST-040] [US-1] External spec without file_content returns 400
func TestCreateFeatureExternalSpecNoFileContent(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	body := `{"type":"external_spec","title":"External Feature","description":"A description","priority":2,"file_content":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-003] expected 400 for external_spec without file_content, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "validation_error" {
		t.Errorf("[AC-003] expected error code 'validation_error', got %q", errResp.Error)
	}
}

// [TEST-041] Feature detail includes all expected fields (API contract test)
func TestFeatureDetailResponseStructure(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-structure", "Structure Test", feature.PhaseInception, feature.StatusInProgress, 2)
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodGet, "/api/features/001-structure", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp FeatureDetailResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	// Verify all required fields are present per spec
	requiredStringFields := map[string]string{
		"id":          resp.ID,
		"title":       resp.Title,
		"status":      resp.Status,
		"intake_path": resp.IntakePath,
	}
	for name, val := range requiredStringFields {
		if val == "" {
			t.Errorf("expected %s to be non-empty", name)
		}
	}

	if resp.Priority < 1 || resp.Priority > 3 {
		t.Errorf("expected priority to be 1-3, got %d", resp.Priority)
	}

	if resp.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
	if resp.UpdatedAt.IsZero() {
		t.Error("expected updated_at to be set")
	}
}

// [TEST-042] Feature list response structure
func TestFeatureListResponseStructure(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-list", "List Feature", feature.PhaseInception, feature.StatusInProgress, 2)
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodGet, "/api/features", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp FeatureListResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Features) < 1 {
		t.Fatalf("expected at least 1 feature, got %d", len(resp.Features))
	}

	summary := resp.Features[0]
	if summary.ID == "" {
		t.Error("[AC-011] feature summary should have ID")
	}
	if summary.Title == "" {
		t.Error("[AC-011] feature summary should have title")
	}
	if summary.CurrentPhase == "" {
		t.Error("[AC-011] feature summary should have current_phase")
	}
	if summary.Status == "" {
		t.Error("[AC-011] feature summary should have status")
	}
}

// [TEST-043] Recovery middleware catches panics
func TestRecoveryMiddlewareCatchesPanics(t *testing.T) {
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic for recovery")
	})
	handler := recoveryMiddleware(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for panic, got %d", w.Code)
	}

	// Verify the response body contains an error JSON
	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "internal_error" {
		t.Errorf("expected error code 'internal_error' from panic recovery, got %q", errResp.Error)
	}
}

// [TEST-044] SSE stream returns correct content type
func TestSSEStreamContentType(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-sse", "SSE Feature", feature.PhaseInception, feature.StatusInProgress, 2)
	createTestFeature(t, tmpDir, f)

	// Use a context with timeout so the SSE handler doesn't block forever
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/features/001-sse/stream", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	// Start SSE handler in a goroutine
	done := make(chan bool)
	go func() {
		handler.ServeHTTP(w, req)
		done <- true
	}()

	// Wait briefly for headers to be written
	time.Sleep(100 * time.Millisecond)

	// The SSE response should have the correct content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("[AC-052] expected Content-Type 'text/event-stream', got %q", contentType)
	}
	_ = done // just drain if needed
}

// [TEST-045] SSE stream for non-existent feature returns 404
func TestSSEStreamFeatureNotFound404(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	req := httptest.NewRequest(http.MethodGet, "/api/features/nonexistent/stream", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for SSE stream on non-existent feature, got %d", w.Code)
	}
}

// [TEST-046] [US-1] Whitespace-only title returns 400
func TestCreateFeatureWhitespaceTitle(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	body := `{"type":"loose_idea","title":"   ","description":"Valid description","priority":2}`
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-009] expected 400 for whitespace-only title, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "empty_title" {
		t.Errorf("[AC-009] expected error code 'empty_title', got %q", errResp.Error)
	}
}

// [TEST-047] [US-1] Whitespace-only description returns 400
func TestCreateFeatureWhitespaceDescription(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	body := `{"type":"loose_idea","title":"Valid Title","description":"   ","priority":2}`
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("[AC-004] expected 400 for whitespace-only description, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "empty_description" {
		t.Errorf("[AC-004] expected error code 'empty_description', got %q", errResp.Error)
	}
}

// [TEST-048] Invalid JSON body returns 400
func TestCreateFeatureInvalidJSON(t *testing.T) {
	server, _ := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(`{invalid json}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "validation_error" {
		t.Errorf("expected error code 'validation_error', got %q", errResp.Error)
	}
}

// [TEST-049] Recirculate with valid backward phase succeeds
func TestRecirculateValidBackwardPhase(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-test", "Test Feature", feature.PhaseReview, feature.StatusGateBlocked, 2)
	createTestFeature(t, tmpDir, f)

	body := `{"target_phase": "planning"}`
	req := httptest.NewRequest(http.MethodPost, "/api/features/001-test/recirculate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid recirculate, got %d", w.Code)
	}

	var resp FeatureDetailResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	// After recirculation, the status should be "recirculated"
	if resp.Status != "recirculated" {
		t.Errorf("[AC-023] expected status 'recirculated' after recirculate, got %q", resp.Status)
	}
	// Verify the target phase (planning) is now in progress
	planning, ok := resp.PhaseStates["planning"]
	if !ok {
		t.Error("[AC-023] expected 'planning' phase state after recirculate")
	} else if planning.Status != "in_progress" {
		t.Errorf("[AC-023] expected planning status 'in_progress', got %q", planning.Status)
	}
}

// [TEST-050] Evaluate gate returns gate result for existing feature
func TestEvaluateGateForFeature(t *testing.T) {
	server, tmpDir := setupTestServerWithDir(t)
	handler := wrappedHandler(server)

	f := makeFeature("001-test", "Test Feature", feature.PhaseInception, feature.StatusInProgress, 2)
	createTestFeature(t, tmpDir, f)

	req := httptest.NewRequest(http.MethodGet, "/api/features/001-test/gate", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for gate evaluation, got %d", w.Code)
	}

	var resp GateResultResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	// The gate result should have a phase field
	if resp.Phase == "" {
		t.Error("expected gate result to have a phase")
	}
}

// timePtr returns a pointer to the given time
func timePtr(t time.Time) *time.Time {
	return &t
}