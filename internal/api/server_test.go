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
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/spec"
)

func setupTestServer(t *testing.T) (*Server, string) {
	t.Helper()

	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	os.MkdirAll(specsDir, 0755)

	cfg := &config.Config{
		Pipeline: config.PipelineConfig{
			Phases: []config.PhaseConfig{
				{Name: "inception", Roles: []string{"pm"}},
				{Name: "planning", Roles: []string{"architect"}},
				{Name: "construction", Roles: []string{"developer"}},
				{Name: "review", Roles: []string{"reviewer"}},
				{Name: "testing", Roles: []string{"tester"}},
				{Name: "delivery", Roles: []string{"ops"}},
			},
		},
	}

	sp := spec.NewSpecProvider(tmpDir)
	pipe := pipeline.NewPipelineWithDispatcher(cfg, sp, nil)
	questionStore := feature.NewFileQuestionStore(tmpDir)

	s := NewServer(":0", sp, pipe, nil, questionStore)

	return s, tmpDir
}

func TestListFeaturesEmpty(t *testing.T) {
	s, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/features", nil)
	w := httptest.NewRecorder()
	s.listFeatures(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	features, ok := resp["features"].([]interface{})
	if !ok {
		t.Fatal("expected features to be an array")
	}
	if len(features) != 0 {
		t.Errorf("expected empty features list, got %d", len(features))
	}
}

func TestGetFeatureNotFound(t *testing.T) {
	s, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/features/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	s.getFeature(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestEvaluateGateFeatureNotFound(t *testing.T) {
	s, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/features/nonexistent/gate", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	s.evaluateGate(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetArtifactFeatureNotFound(t *testing.T) {
	s, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/features/nonexistent/artifacts/spec", nil)
	req.SetPathValue("id", "nonexistent")
	req.SetPathValue("type", "spec")
	w := httptest.NewRecorder()
	s.getArtifact(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCancelFeatureNotFound(t *testing.T) {
	s, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/features/nonexistent/cancel", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	s.cancelFeature(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	s, _ := setupTestServer(t)

	handler := s.recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestFeatureToDetailResponse(t *testing.T) {
	f := feature.NewFeature("001-test", "Test Feature", 2, feature.IntakeLooseIdea)
	resp := FeatureToDetailResponse(f)

	if resp.ID != "001-test" {
		t.Errorf("expected ID '001-test', got %s", resp.ID)
	}
	if resp.Title != "Test Feature" {
		t.Errorf("expected title 'Test Feature', got %s", resp.Title)
	}
	if resp.CurrentPhase != "inception" {
		t.Errorf("expected current_phase 'inception', got %s", resp.CurrentPhase)
	}
	if len(resp.PhaseStates) != 6 {
		t.Errorf("expected 6 phase states, got %d", len(resp.PhaseStates))
	}
}

func TestCORSHeaders(t *testing.T) {
	s, _ := setupTestServer(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	handler := s.corsMiddleware(inner)

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204 for OPTIONS, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS origin header")
	}
}

func TestSmokeServerStartsAndResponds(t *testing.T) {
	_, tmpDir := setupTestServer(t)

	cfg := &config.Config{
		Pipeline: config.PipelineConfig{
			Phases: []config.PhaseConfig{
				{Name: "inception", Roles: []string{"pm"}},
				{Name: "planning", Roles: []string{"architect"}},
				{Name: "construction", Roles: []string{"developer"}},
				{Name: "review", Roles: []string{"reviewer"}},
				{Name: "testing", Roles: []string{"tester"}},
				{Name: "delivery", Roles: []string{"ops"}},
			},
		},
	}

	sp := spec.NewSpecProvider(tmpDir)
	pipe := pipeline.NewPipelineWithDispatcher(cfg, sp, nil)
	s := NewServer(":0", sp, pipe, nil, feature.NewFileQuestionStore(tmpDir))

	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/features")
	if err != nil {
		t.Fatalf("GET /api/features failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := body["features"]; !ok {
		t.Error("expected 'features' key in response")
	}
}

func TestSmokeRecoveryNoNilPointer(t *testing.T) {
	_, tmpDir := setupTestServer(t)

	cfg := &config.Config{
		Pipeline: config.PipelineConfig{
			Phases: []config.PhaseConfig{
				{Name: "inception", Roles: []string{"pm"}},
			},
		},
	}

	sp := spec.NewSpecProvider(tmpDir)
	pipe := pipeline.NewPipelineWithDispatcher(cfg, sp, nil)
	s := NewServer(":0", sp, pipe, nil, feature.NewFileQuestionStore(tmpDir))

	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	endpoints := []struct {
		method string
		path   string
		want   int
	}{
		{"GET", "/api/features", 200},
		{"GET", "/api/features/nonexistent", 404},
		{"GET", "/api/features/nonexistent/gate", 404},
		{"GET", "/api/features/nonexistent/artifacts/spec", 404},
		{"POST", "/api/features/nonexistent/run", 404},
		{"POST", "/api/features/nonexistent/advance", 404},
		{"POST", "/api/features/nonexistent/cancel", 404},
		{"POST", "/api/features/nonexistent/process", 404},
		{"GET", "/api/features/nonexistent/stream", 404},
	}

	for _, tc := range endpoints {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, ts.URL+tc.path, nil)
			if err != nil {
				t.Fatalf("creating request: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.want {
				t.Errorf("expected %d, got %d for %s %s", tc.want, resp.StatusCode, tc.method, tc.path)
			}
		})
	}
}

func TestSmokeCreateAndGetFeature(t *testing.T) {
	_, tmpDir := setupTestServer(t)

	cfg := &config.Config{
		Pipeline: config.PipelineConfig{
			Phases: []config.PhaseConfig{
				{Name: "inception", Roles: []string{"pm"}},
				{Name: "planning", Roles: []string{"architect"}},
				{Name: "construction", Roles: []string{"developer"}},
				{Name: "review", Roles: []string{"reviewer"}},
				{Name: "testing", Roles: []string{"tester"}},
				{Name: "delivery", Roles: []string{"ops"}},
			},
		},
	}

	sp := spec.NewSpecProvider(tmpDir)
	pipe := pipeline.NewPipelineWithDispatcher(cfg, sp, nil)
	s := NewServer(":0", sp, pipe, nil, feature.NewFileQuestionStore(tmpDir))

	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	createBody := `{"type":"loose_idea","title":"Test Feature","description":"A test feature for smoke testing","priority":2}`
	resp, err := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /api/features failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	var created FeatureDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode created feature: %v", err)
	}

	if created.Title != "Test Feature" {
		t.Errorf("expected title 'Test Feature', got %s", created.Title)
	}
	if created.CurrentPhase != "inception" {
		t.Errorf("expected current_phase 'inception', got %s", created.CurrentPhase)
	}

	getResp, err := http.Get(ts.URL + "/api/features/" + created.ID)
	if err != nil {
		t.Fatalf("GET /api/features/%s failed: %v", created.ID, err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", getResp.StatusCode)
	}

	var detail FeatureDetailResponse
	if err := json.NewDecoder(getResp.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode feature detail: %v", err)
	}

	if len(detail.PhaseStates) != 6 {
		t.Errorf("expected 6 phase states, got %d", len(detail.PhaseStates))
	}

	for phase, state := range detail.PhaseStates {
		if state.Artifacts == nil {
			t.Errorf("phase %s: artifacts should be [], got null", phase)
		}
		if state.GateResult != nil && state.GateResult.Checks == nil {
			t.Errorf("phase %s: gate_result.checks should be [], got null", phase)
		}
	}

	if detail.Dependencies == nil {
		t.Error("dependencies should be [], got null")
	}
	if detail.Repos == nil {
		t.Error("repos should be [], got null")
	}
}

func TestIntegrationJSONArraysNeverNull(t *testing.T) {
	_, tmpDir := setupTestServer(t)

	cfg := &config.Config{
		Pipeline: config.PipelineConfig{
			Phases: []config.PhaseConfig{
				{Name: "inception", Roles: []string{"pm"}},
				{Name: "planning", Roles: []string{"architect"}},
				{Name: "construction", Roles: []string{"developer"}},
				{Name: "review", Roles: []string{"reviewer"}},
				{Name: "testing", Roles: []string{"tester"}},
				{Name: "delivery", Roles: []string{"ops"}},
			},
		},
	}

	sp := spec.NewSpecProvider(tmpDir)
	pipe := pipeline.NewPipelineWithDispatcher(cfg, sp, nil)
	s := NewServer(":0", sp, pipe, nil, feature.NewFileQuestionStore(tmpDir))

	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	createBody := `{"type":"loose_idea","title":"Array Check","description":"Testing arrays are never null","priority":2}`
	resp, err := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /api/features failed: %v", err)
	}
	defer resp.Body.Close()

	var created FeatureDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	raw, err := json.Marshal(created)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	nullChecks := []string{
		`"artifacts":null`,
		`"checks":null`,
		`"missing_arts":null`,
		`"dependencies":null`,
		`"repos":null`,
	}
	for _, bad := range nullChecks {
		if strings.Contains(string(raw), bad) {
			t.Errorf("response contains %s — arrays should be [] not null", bad)
		}
	}
}
