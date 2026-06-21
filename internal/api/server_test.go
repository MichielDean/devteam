package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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

	if resp["total_count"] != float64(0) {
		t.Errorf("expected total_count 0, got %v", resp["total_count"])
	}
}

func TestListFeaturesTotalCountPopulated(t *testing.T) {
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

	for i := 0; i < 3; i++ {
		createBody := `{"type":"loose_idea","title":"Populated ` + string(rune('A'+i)) + `","description":"desc","priority":2}`
		resp, err := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
		if err != nil {
			t.Fatalf("POST /api/features failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
	}

	listResp, err := http.Get(ts.URL + "/api/features")
	if err != nil {
		t.Fatalf("GET /api/features failed: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.StatusCode)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(listResp.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["total_count"] != float64(3) {
		t.Errorf("expected total_count 3, got %v", resp["total_count"])
	}

	features, ok := resp["features"].([]interface{})
	if !ok {
		t.Fatal("expected features to be an array")
	}
	if len(features) != 3 {
		t.Errorf("expected 3 features, got %d", len(features))
	}

	if resp["total_count"] != float64(len(features)) {
		t.Errorf("total_count %v != len(features) %d", resp["total_count"], len(features))
	}
}

func TestErrorResponseShape(t *testing.T) {
	// FR-003: error responses must NOT contain total_count. ErrorResponse encodes
	// only `error` and `details`, so a 500 body never carries total_count.
	raw, err := json.Marshal(ErrorResponse{Error: "internal_error", Details: "Failed to list features"})
	if err != nil {
		t.Fatalf("failed to marshal ErrorResponse: %v", err)
	}

	if strings.Contains(string(raw), "total_count") {
		t.Errorf("error response must not contain total_count, got: %s", raw)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if _, exists := decoded["total_count"]; exists {
		t.Error("error response must not have total_count key")
	}
	if _, exists := decoded["error"]; !exists {
		t.Error("error response must have error key")
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

	// Verify total_count on the features list after creating one feature
	listResp, err := http.Get(ts.URL + "/api/features")
	if err != nil {
		t.Fatalf("GET /api/features failed: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(listResp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}

	if body["total_count"] != float64(1) {
		t.Errorf("expected total_count 1, got %v", body["total_count"])
	}
	features, ok := body["features"].([]interface{})
	if !ok {
		t.Fatal("expected features to be an array")
	}
	if len(features) != 1 {
		t.Errorf("expected 1 feature, got %d", len(features))
	}
}

func TestListFeaturesTotalCountConsistency(t *testing.T) {
	// AC-012: total_count == len(features) for N in {0, 1, 5, 50}
	for _, n := range []int{0, 1, 5, 50} {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
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
			s := NewServer(":0", sp, pipe, nil, feature.NewFileQuestionStore(tmpDir))
			ts := httptest.NewServer(s.httpServer.Handler)
			defer ts.Close()

			for i := 0; i < n; i++ {
				body := `{"type":"loose_idea","title":"F` + strconv.Itoa(i) + `","description":"d","priority":2}`
				resp, err := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(body))
				if err != nil {
					t.Fatalf("POST failed: %v", err)
				}
				if resp.StatusCode != http.StatusCreated {
					t.Fatalf("expected 201, got %d", resp.StatusCode)
				}
				resp.Body.Close()
			}

			listResp, err := http.Get(ts.URL + "/api/features")
			if err != nil {
				t.Fatalf("GET failed: %v", err)
			}
			defer listResp.Body.Close()
			if listResp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", listResp.StatusCode)
			}

			raw, err := io.ReadAll(listResp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}

			// Guard against null array regression (FR-005)
			if bytes.Contains(raw, []byte(`"features":null`)) {
				t.Errorf("features must be [] not null, body: %s", raw)
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(raw, &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}

			tc, ok := resp["total_count"].(float64)
			if !ok {
				t.Fatalf("total_count missing or not a number, got %v", resp["total_count"])
			}
			feats := resp["features"].([]interface{})
			if int(tc) != n {
				t.Errorf("total_count=%v want %d", tc, n)
			}
			if len(feats) != n {
				t.Errorf("features len=%d want %d", len(feats), n)
			}
			if int(tc) != len(feats) {
				t.Errorf("total_count %v != len(features) %d (FR-004 invariant)", tc, len(feats))
			}
		})
	}
}

func TestListFeaturesErrorResponseHasNoTotalCount(t *testing.T) {
	// AC-011: 500 error response must NOT contain total_count. Force a failure
	// by pointing the SpecProvider at an unreadable directory (chmod 000).
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	os.MkdirAll(specsDir, 0755)

	// Make specs dir unreadable to force ReadDir error
	if err := os.Chmod(specsDir, 0000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(specsDir, 0755)

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
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if bytes.Contains(body, []byte("total_count")) {
		t.Errorf("error response must not contain total_count, got: %s", body)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, exists := decoded["total_count"]; exists {
		t.Error("error response must not have total_count key")
	}
	if _, exists := decoded["error"]; !exists {
		t.Error("error response must have error key")
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
