package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/spec"
)

// Helper to read response body as string
func readBody(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(resp.Body)
	return buf.String(), err
}

// === SMOKE TESTS ===
// [T001] [US-001] [AC-088] [SMOKE] Server starts and question endpoints respond without panicking

func TestSmokeQuestionEndpoints(t *testing.T) {
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

	// First create a feature
	createBody := `{"type":"loose_idea","title":"Smoke Test Feature","description":"Testing","priority":2}`
	resp, err := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /api/features failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var created FeatureDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	featureID := created.ID

	endpoints := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"GET questions for existing feature", "GET", "/api/features/" + featureID + "/questions", "", 200},
		{"GET pending questions for existing feature", "GET", "/api/features/" + featureID + "/questions/pending", "", 200},
		{"POST create question", "POST", "/api/features/" + featureID + "/questions", `{"phase":"inception","role":"pm","question":"What is the target?","type":"clarification"}`, 201},
		{"GET questions for nonexistent feature", "GET", "/api/features/nonexistent/questions", "", 404},
		{"GET pending questions for nonexistent feature", "GET", "/api/features/nonexistent/questions/pending", "", 404},
		{"POST question for nonexistent feature", "POST", "/api/features/nonexistent/questions", `{"phase":"inception","role":"pm","question":"test","type":"clarification"}`, 404},
		{"PATCH answer for nonexistent feature", "PATCH", "/api/features/nonexistent/questions/Q-001", `{"answer":"test"}`, 404},
	}

	for _, tc := range endpoints {
		t.Run(tc.name, func(t *testing.T) {
			var body *strings.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			} else {
				body = strings.NewReader("")
			}
			req, err := http.NewRequest(tc.method, ts.URL+tc.path, body)
			if err != nil {
				t.Fatalf("creating request: %v", err)
			}
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
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

// === INTEGRATION TESTS ===
// [T002] [US-001] [AC-051] [INTEGRATION] GET /api/features/{id}/questions returns all questions with correct structure

func TestIntegrationListQuestions(t *testing.T) {
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

	// Create a feature first
	createBody := `{"type":"loose_idea","title":"Test Feature","description":"Testing","priority":2}`
	resp, err := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /api/features failed: %v", err)
	}
	defer resp.Body.Close()
	var created FeatureDetailResponse
	json.NewDecoder(resp.Body).Decode(&created)
	featureID := created.ID

	// Create 3 questions
	questions := []string{
		`{"phase":"inception","role":"pm","question":"What is the target audience?","type":"clarification","options":["Internal","External"]}`,
		`{"phase":"planning","role":"architect","question":"Which database?","type":"decision","options":["PostgreSQL","MongoDB"]}`,
		`{"phase":"inception","role":"pm","question":"What is the priority?","type":"priority"}`,
	}
	for _, q := range questions {
		resp, err := http.Post(ts.URL+"/api/features/"+featureID+"/questions", "application/json", strings.NewReader(q))
		if err != nil {
			t.Fatalf("POST question failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
	}

	// [T003] [US-001] [AC-051] [INTEGRATION] GET /api/features/{id}/questions returns all 3 questions
	resp, err = http.Get(ts.URL + "/api/features/" + featureID + "/questions")
	if err != nil {
		t.Fatalf("GET questions failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var qList []QuestionResponse
	if err := json.NewDecoder(resp.Body).Decode(&qList); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(qList) != 3 {
		t.Errorf("expected 3 questions, got %d", len(qList))
	}

	// Verify all expected fields are present
	for i, q := range qList {
		if q.ID == "" {
			t.Errorf("question %d: ID should not be empty", i)
		}
		if q.FeatureID != featureID {
			t.Errorf("question %d: feature_id should be %s, got %s", i, featureID, q.FeatureID)
		}
		if q.Question == "" {
			t.Errorf("question %d: question text should not be empty", i)
		}
		if q.Status != "pending" {
			t.Errorf("question %d: expected status pending, got %s", i, q.Status)
		}
		if q.CreatedAt == "" {
			t.Errorf("question %d: created_at should not be empty", i)
		}
		if q.Answer != nil {
			t.Errorf("question %d: answer should be nil for pending question, got %v", i, *q.Answer)
		}
		if q.Assumption != nil {
			t.Errorf("question %d: assumption should be nil for pending question, got %v", i, *q.Assumption)
		}
		if q.AnsweredAt != nil {
			t.Errorf("question %d: answered_at should be nil for pending question", i)
		}
	}

	// [T004] [US-001] [AC-052] [INTEGRATION] Verify options field is [] not null
	if qList[0].Options == nil {
		t.Error("options should be [] not null for question with options")
	}
	if len(qList[0].Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(qList[0].Options))
	}
	// Question with no options should have [] not null
	if qList[2].Options == nil {
		t.Error("options should be [] not null for question without options")
	}
	if len(qList[2].Options) != 0 {
		t.Errorf("expected 0 options for question without options, got %d", len(qList[2].Options))
	}
}

// [T005] [US-001] [AC-052] [INTEGRATION] GET /api/features/{id}/questions returns [] for feature with no questions

func TestIntegrationListQuestionsEmpty(t *testing.T) {
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

	// Create a feature (no questions)
	createBody := `{"type":"loose_idea","title":"Empty Feature","description":"No questions","priority":2}`
	resp, err := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /api/features failed: %v", err)
	}
	defer resp.Body.Close()
	var created FeatureDetailResponse
	json.NewDecoder(resp.Body).Decode(&created)
	featureID := created.ID

	resp, err = http.Get(ts.URL + "/api/features/" + featureID + "/questions")
	if err != nil {
		t.Fatalf("GET questions failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// [T006] [AGENT-FAILURE-MODE] Verify the response body is exactly [] not null
	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	bodyStr := strings.TrimSpace(string(body[:n]))

	// The response should be [] not null
	if bodyStr == "null" {
		t.Error("AGENT FAILURE MODE: GET /questions returned null instead of []")
	}
	if !strings.HasPrefix(bodyStr, "[") {
		t.Errorf("expected JSON array, got: %s", bodyStr)
	}

	var qList []QuestionResponse
	// Re-decode from the original response
	resp2, err := http.Get(ts.URL + "/api/features/" + featureID + "/questions")
	if err != nil {
		t.Fatalf("GET questions failed: %v", err)
	}
	defer resp2.Body.Close()
	if err := json.NewDecoder(resp2.Body).Decode(&qList); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(qList) != 0 {
		t.Errorf("expected 0 questions, got %d", len(qList))
	}
}

// [T007] [US-001] [AC-054] [INTEGRATION] POST /api/features/{id}/questions creates a question with auto-generated ID

func TestIntegrationCreateQuestion(t *testing.T) {
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

	createBody := `{"type":"loose_idea","title":"Test Feature","description":"Testing","priority":2}`
	resp, err := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /api/features failed: %v", err)
	}
	var created FeatureDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode feature: %v", err)
	}
	resp.Body.Close()
	featureID := created.ID

	// [T008] [US-001] [AC-044] [INTEGRATION] Create a valid question
	questionBody := `{"phase":"inception","role":"pm","question":"What is the target audience?","type":"clarification","options":["Internal developers","External users"]}`
	resp, err = http.Post(ts.URL+"/api/features/"+featureID+"/questions", "application/json", strings.NewReader(questionBody))
	if err != nil {
		t.Fatalf("POST question failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := readBody(resp)
		t.Fatalf("expected 201, got %d, body: %s", resp.StatusCode, bodyBytes)
	}

	var question QuestionResponse
	if err := json.NewDecoder(resp.Body).Decode(&question); err != nil {
		t.Fatalf("failed to decode question: %v", err)
	}

	// Verify auto-generated ID format
	if !strings.HasPrefix(question.ID, "Q-") {
		t.Errorf("expected ID to start with Q-, got %s", question.ID)
	}
	if question.Status != "pending" {
		t.Errorf("expected status pending, got %s", question.Status)
	}
	if question.FeatureID != featureID {
		t.Errorf("expected feature_id %s, got %s", featureID, question.FeatureID)
	}
	if question.Phase != "inception" {
		t.Errorf("expected phase inception, got %s", question.Phase)
	}
	if question.Role != "pm" {
		t.Errorf("expected role pm, got %s", question.Role)
	}
	if question.Question != "What is the target audience?" {
		t.Errorf("expected question text, got %s", question.Question)
	}
	if question.Type != "clarification" {
		t.Errorf("expected type clarification, got %s", question.Type)
	}
	if question.Answer != nil {
		t.Errorf("expected answer to be nil for new question, got %v", *question.Answer)
	}
	if question.Assumption != nil {
		t.Errorf("expected assumption to be nil for new question, got %v", *question.Assumption)
	}
	if question.AnsweredAt != nil {
		t.Error("expected answered_at to be nil for new question")
	}
	if question.CreatedAt == "" {
		t.Error("expected created_at to be set")
	}
	if len(question.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(question.Options))
	}
}

// [T009] [US-001] [AC-045] [INTEGRATION] POST with empty question returns 400

func TestIntegrationCreateQuestionValidation(t *testing.T) {
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

	createBody := `{"type":"loose_idea","title":"Test Feature","description":"Testing","priority":2}`
	resp, err := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /api/features failed: %v", err)
	}
	var created FeatureDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode feature: %v", err)
	}
	resp.Body.Close()
	featureID := created.ID

	testCases := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "empty question text",
			body:    `{"phase":"inception","role":"pm","question":"","type":"clarification"}`,
			wantErr: "question is required",
		},
		{
			name:    "missing question field",
			body:    `{"phase":"inception","role":"pm","type":"clarification"}`,
			wantErr: "question is required",
		},
		{
			name:    "invalid phase",
			body:    `{"phase":"construction","role":"pm","question":"Test?","type":"clarification"}`,
			wantErr: "phase must be one of: inception, planning",
		},
		{
			name:    "invalid role",
			body:    `{"phase":"inception","role":"developer","question":"Test?","type":"clarification"}`,
			wantErr: "role must be one of: pm, architect",
		},
		{
			name:    "invalid type",
			body:    `{"phase":"inception","role":"pm","question":"Test?","type":"invalid_type"}`,
			wantErr: "type must be one of: clarification, decision, priority",
		},
		{
			name:    "too many options",
			body:    `{"phase":"inception","role":"pm","question":"Test?","type":"clarification","options":["1","2","3","4","5","6","7","8","9","10","11"]}`,
			wantErr: "options must have at most 10 items",
		},
		{
			name:    "option too long",
			body:    `{"phase":"inception","role":"pm","question":"Test?","type":"clarification","options":["` + strings.Repeat("a", 501) + `"]}`,
			wantErr: "each option must be 1-500 characters",
		},
		{
			name:    "question too long",
			body:    `{"phase":"inception","role":"pm","question":"` + strings.Repeat("a", 2001) + `","type":"clarification"}`,
			wantErr: "question must be 1-2000 characters",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Post(ts.URL+"/api/features/"+featureID+"/questions", "application/json", strings.NewReader(tc.body))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", resp.StatusCode)
			}

			var errResp ErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error: %v", err)
			}

			if errResp.Error != "validation_error" {
				t.Errorf("expected error 'validation_error', got %s", errResp.Error)
			}
			if !strings.Contains(errResp.Details, tc.wantErr) {
				t.Errorf("expected details to contain %q, got %q", tc.wantErr, errResp.Details)
			}
		})
	}
}

// [T010] [US-001] [AC-057] [INTEGRATION] PATCH /api/features/{id}/questions/{questionId} answers a question

func TestIntegrationAnswerQuestion(t *testing.T) {
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

	createBody := `{"type":"loose_idea","title":"Test Feature","description":"Testing","priority":2}`
	resp, err := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /api/features failed: %v", err)
	}
	var created FeatureDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode feature: %v", err)
	}
	resp.Body.Close()
	featureID := created.ID

	// Create a question
	questionBody := `{"phase":"inception","role":"pm","question":"What is the target?","type":"clarification"}`
	resp, err = http.Post(ts.URL+"/api/features/"+featureID+"/questions", "application/json", strings.NewReader(questionBody))
	if err != nil {
		t.Fatalf("POST question failed: %v", err)
	}
	var question QuestionResponse
	if err := json.NewDecoder(resp.Body).Decode(&question); err != nil {
		t.Fatalf("failed to decode question: %v", err)
	}
	resp.Body.Close()
	questionID := question.ID

	// [T010] Answer the question
	answerBody := `{"answer":"I want option A"}`
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/features/"+featureID+"/questions/"+questionID, strings.NewReader(answerBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH question failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readBody(resp)
		t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, bodyBytes)
	}

	var answered QuestionResponse
	if err := json.NewDecoder(resp.Body).Decode(&answered); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if answered.Status != "answered" {
		t.Errorf("expected status answered, got %s", answered.Status)
	}
	if answered.Answer == nil || *answered.Answer != "I want option A" {
		t.Errorf("expected answer 'I want option A', got %v", answered.Answer)
	}
	if answered.AnsweredAt == nil {
		t.Error("expected answered_at to be set")
	}
}

// [T011] [US-001] [AC-004] [INTEGRATION] Answering an already-answered question returns 409

func TestIntegrationAnswerConflict(t *testing.T) {
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

	createBody := `{"type":"loose_idea","title":"Test Feature","description":"Testing","priority":2}`
	resp, _ := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	var created FeatureDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode feature: %v", err)
	}
	resp.Body.Close()
	featureID := created.ID

	// Create a question
	questionBody := `{"phase":"inception","role":"pm","question":"What is the target?","type":"clarification"}`
	resp, _ = http.Post(ts.URL+"/api/features/"+featureID+"/questions", "application/json", strings.NewReader(questionBody))
	var question QuestionResponse
	if err := json.NewDecoder(resp.Body).Decode(&question); err != nil {
		t.Fatalf("failed to decode question: %v", err)
	}
	resp.Body.Close()
	questionID := question.ID

	// Answer it once
	answerBody := `{"answer":"First answer"}`
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/features/"+featureID+"/questions/"+questionID, strings.NewReader(answerBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = http.DefaultClient.Do(req)
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("closing first answer response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readBody(resp)
		t.Fatalf("first answer: expected 200, got %d, body: %s", resp.StatusCode, bodyBytes)
	}

	// [T011] Try to answer again — should get 409
	req, _ = http.NewRequest("PATCH", ts.URL+"/api/features/"+featureID+"/questions/"+questionID, strings.NewReader(`{"answer":"Second answer"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = http.DefaultClient.Do(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Error != "conflict" {
		t.Errorf("expected error 'conflict', got %s", errResp.Error)
	}
}

// [T012] [US-001] [AC-005] [INTEGRATION] Answering a nonexistent question returns 404

func TestIntegrationAnswerNotFound(t *testing.T) {
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

	createBody := `{"type":"loose_idea","title":"Test Feature","description":"Testing","priority":2}`
	resp, _ := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	var created FeatureDetailResponse
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	featureID := created.ID

	// Answer a nonexistent question
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/features/"+featureID+"/questions/Q-999", strings.NewReader(`{"answer":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = http.DefaultClient.Do(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// [T013] [US-001] [AC-006] [INTEGRATION] Answer with empty string returns 400

func TestIntegrationAnswerEmptyString(t *testing.T) {
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

	createBody := `{"type":"loose_idea","title":"Test Feature","description":"Testing","priority":2}`
	resp, _ := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	var created FeatureDetailResponse
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	featureID := created.ID

	// Create question
	questionBody := `{"phase":"inception","role":"pm","question":"What?","type":"clarification"}`
	resp, _ = http.Post(ts.URL+"/api/features/"+featureID+"/questions", "application/json", strings.NewReader(questionBody))
	var question QuestionResponse
	json.NewDecoder(resp.Body).Decode(&question)
	resp.Body.Close()
	questionID := question.ID

	// Answer with empty string
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/features/"+featureID+"/questions/"+questionID, strings.NewReader(`{"answer":""}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = http.DefaultClient.Do(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if !strings.Contains(errResp.Details, "answer must be 1-5000 characters") {
		t.Errorf("expected 'answer must be 1-5000 characters', got %q", errResp.Details)
	}
}

// [T014] [US-001] [AC-007] [INTEGRATION] Answer exceeding 5000 characters returns 400

func TestIntegrationAnswerTooLong(t *testing.T) {
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

	createBody := `{"type":"loose_idea","title":"Test Feature","description":"Testing","priority":2}`
	resp, _ := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	var created FeatureDetailResponse
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	featureID := created.ID

	// Create question
	questionBody := `{"phase":"inception","role":"pm","question":"What?","type":"clarification"}`
	resp, _ = http.Post(ts.URL+"/api/features/"+featureID+"/questions", "application/json", strings.NewReader(questionBody))
	var question QuestionResponse
	json.NewDecoder(resp.Body).Decode(&question)
	resp.Body.Close()
	questionID := question.ID

	// Answer with > 5000 chars
	longAnswer := strings.Repeat("a", 5001)
	answerJSON := `{"answer":"` + longAnswer + `"}`
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/features/"+featureID+"/questions/"+questionID, strings.NewReader(answerJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = http.DefaultClient.Do(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// [T015] [US-001] [AC-061] [INTEGRATION] GET /api/features/{id}/questions/pending returns only pending questions

func TestIntegrationListPendingQuestions(t *testing.T) {
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

	createBody := `{"type":"loose_idea","title":"Test Feature","description":"Testing","priority":2}`
	resp, _ := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	var created FeatureDetailResponse
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	featureID := created.ID

	// Create 3 questions
	for i := 0; i < 3; i++ {
		q := `{"phase":"inception","role":"pm","question":"Question ` + string(rune('A'+i)) + `?","type":"clarification"}`
		resp, _ := http.Post(ts.URL+"/api/features/"+featureID+"/questions", "application/json", strings.NewReader(q))
		resp.Body.Close()
	}

	// Get all questions
	resp, _ = http.Get(ts.URL + "/api/features/" + featureID + "/questions")
	var allQuestions []QuestionResponse
	json.NewDecoder(resp.Body).Decode(&allQuestions)
	resp.Body.Close()

	// Answer the first one
	answerBody := `{"answer":"My answer"}`
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/features/"+featureID+"/questions/"+allQuestions[0].ID, strings.NewReader(answerBody))
	req.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(req)

	// [T015] GET pending should return only 2 questions
	resp, _ = http.Get(ts.URL + "/api/features/" + featureID + "/questions/pending")
	defer resp.Body.Close()

	var pending []QuestionResponse
	json.NewDecoder(resp.Body).Decode(&pending)

	if len(pending) != 2 {
		t.Errorf("expected 2 pending questions, got %d", len(pending))
	}

	for _, q := range pending {
		if q.Status != "pending" {
			t.Errorf("expected status pending, got %s", q.Status)
		}
	}

	// [T016] [US-001] [AC-062] [INTEGRATION] After answering all questions, pending returns []
	// Answer the remaining 2
	for _, q := range pending {
		answerBody := `{"answer":"Answer"}`
		req, _ := http.NewRequest("PATCH", ts.URL+"/api/features/"+featureID+"/questions/"+q.ID, strings.NewReader(answerBody))
		req.Header.Set("Content-Type", "application/json")
		http.DefaultClient.Do(req)
	}

	resp, _ = http.Get(ts.URL + "/api/features/" + featureID + "/questions/pending")
	defer resp.Body.Close()

	var emptyPending []QuestionResponse
	json.NewDecoder(resp.Body).Decode(&emptyPending)

	if len(emptyPending) != 0 {
		t.Errorf("expected 0 pending questions, got %d", len(emptyPending))
	}

	// [T017] [AGENT-FAILURE-MODE] Verify it returns [] not null
	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	bodyStr := strings.TrimSpace(string(body[:n]))
	if bodyStr == "null" {
		t.Error("AGENT FAILURE MODE: pending questions returned null instead of []")
	}
}

// [T018] [US-001] [AC-SEC-001] [INTEGRATION] XSS in answer is stored as-is and not executed

func TestIntegrationXSSInAnswer(t *testing.T) {
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

	createBody := `{"type":"loose_idea","title":"Test Feature","description":"Testing","priority":2}`
	resp, _ := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	var created FeatureDetailResponse
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	featureID := created.ID

	// Create a question
	questionBody := `{"phase":"inception","role":"pm","question":"What?","type":"clarification"}`
	resp, _ = http.Post(ts.URL+"/api/features/"+featureID+"/questions", "application/json", strings.NewReader(questionBody))
	var question QuestionResponse
	json.NewDecoder(resp.Body).Decode(&question)
	resp.Body.Close()

	// Answer with XSS payload
	xssPayload := `<script>alert('xss')</script>`
	answerJSON := `{"answer":"` + strings.ReplaceAll(xssPayload, `"`, `\"`) + `"}`
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/features/"+featureID+"/questions/"+question.ID, strings.NewReader(answerJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = http.DefaultClient.Do(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var answered QuestionResponse
	json.NewDecoder(resp.Body).Decode(&answered)
	resp.Body.Close()

	// The XSS payload should be stored as-is, not stripped
	if answered.Answer == nil || *answered.Answer != xssPayload {
		t.Errorf("expected XSS payload stored as-is, got %v", answered.Answer)
	}
}

// [T019] [US-001] [AC-SEC-002] [INTEGRATION] Question text exceeding 2000 characters returns 400

func TestIntegrationQuestionTooLong(t *testing.T) {
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

	createBody := `{"type":"loose_idea","title":"Test Feature","description":"Testing","priority":2}`
	resp, _ := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	var created FeatureDetailResponse
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	featureID := created.ID

	// Create a question with > 2000 char text
	longQuestion := strings.Repeat("a", 2001)
	questionBody := `{"phase":"inception","role":"pm","question":"` + longQuestion + `","type":"clarification"}`
	resp, _ = http.Post(ts.URL+"/api/features/"+featureID+"/questions", "application/json", strings.NewReader(questionBody))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// [T020] [US-003] [AC-019] [INTEGRATION] Advance from waiting_for_human returns 400

func TestIntegrationAdvanceFromWaitingHumanBlocked(t *testing.T) {
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

	// Create a feature and set it to waiting_for_human via the store
	// This requires direct manipulation since we can't easily set status via API
	f := feature.NewFeature("test-wfh-feature", "Test WFH", 2, feature.IntakeLooseIdea)
	f.Status = feature.StatusInProgress
	f.Current = feature.PhaseInception
	sp.SaveFeatureState(f)

	// Create a question (this is valid because the feature is in_progress in inception)
	questionBody := `{"phase":"inception","role":"pm","question":"What?","type":"clarification"}`
	resp, _ := http.Post(ts.URL+"/api/features/"+f.ID+"/questions", "application/json", strings.NewReader(questionBody))
	resp.Body.Close()

	// Now manually set feature to waiting_for_human
	f.Status = feature.StatusWaitingHuman
	sp.SaveFeatureState(f)

	// Try to advance — should get 400
	advanceBody := `{}`
	req, _ := http.NewRequest("POST", ts.URL+"/api/features/"+f.ID+"/advance", strings.NewReader(advanceBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = http.DefaultClient.Do(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for advancing from waiting_for_human, got %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Error != "validation_error" {
		t.Errorf("expected error 'validation_error', got %s", errResp.Error)
	}
}

// [T021] [US-006] [AC-032] [INTEGRATION] Feature list includes pending_questions_count

func TestIntegrationFeatureListIncludesPendingQuestionsCount(t *testing.T) {
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

	// Create a feature
	createBody := `{"type":"loose_idea","title":"Badge Test Feature","description":"Testing badge","priority":2}`
	resp, _ := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	resp.Body.Close()
	var created FeatureDetailResponse
	json.NewDecoder(resp.Body).Decode(&created)
	featureID := created.ID

	// Initially, pending_questions_count should be 0
	resp, _ = http.Get(ts.URL + "/api/features")
	defer resp.Body.Close()
	var listResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&listResp)
	features := listResp["features"].([]interface{})
	for _, f := range features {
		fmap := f.(map[string]interface{})
		if fmap["id"] == featureID {
			count, ok := fmap["pending_questions_count"]
			if !ok {
				t.Error("AGENT FAILURE MODE: pending_questions_count field is missing from feature list response")
			}
			if int(count.(float64)) != 0 {
				t.Errorf("expected pending_questions_count 0, got %v", count)
			}
		}
	}

	// Create 2 questions
	for i := 0; i < 2; i++ {
		q := `{"phase":"inception","role":"pm","question":"Question ` + string(rune('0'+i)) + `?","type":"clarification"}`
		resp, _ := http.Post(ts.URL+"/api/features/"+featureID+"/questions", "application/json", strings.NewReader(q))
		resp.Body.Close()
	}

	// Now pending_questions_count should be 2
	resp, _ = http.Get(ts.URL + "/api/features")
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&listResp)
	features = listResp["features"].([]interface{})
	for _, f := range features {
		fmap := f.(map[string]interface{})
		if fmap["id"] == featureID {
			count := fmap["pending_questions_count"]
			if int(count.(float64)) != 2 {
				t.Errorf("expected pending_questions_count 2, got %v", count)
			}
		}
	}
}

// [T022] [AGENT-FAILURE-MODE] [INTEGRATION] Verify JSON arrays are [] not null for all question endpoints

func TestIntegrationQuestionEndpointsArraysNeverNull(t *testing.T) {
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

	createBody := `{"type":"loose_idea","title":"Array Test Feature","description":"Testing arrays","priority":2}`
	resp, _ := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	var created FeatureDetailResponse
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	featureID := created.ID

	// Test GET /questions with no questions returns []
	resp, _ = http.Get(ts.URL + "/api/features/" + featureID + "/questions")
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	bodyStr := buf.String()

	if bodyStr == "null" {
		t.Error("AGENT FAILURE MODE: GET /questions returned null instead of []")
	}

	// Test GET /questions/pending with no questions returns []
	resp, _ = http.Get(ts.URL + "/api/features/" + featureID + "/questions/pending")
	defer resp.Body.Close()
	buf = new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	bodyStr = buf.String()

	if bodyStr == "null" {
		t.Error("AGENT FAILURE MODE: GET /questions/pending returned null instead of []")
	}

	// Create a question WITHOUT options and verify options is []
	questionBody := `{"phase":"inception","role":"pm","question":"No options question?","type":"clarification"}`
	resp, _ = http.Post(ts.URL+"/api/features/"+featureID+"/questions", "application/json", strings.NewReader(questionBody))
	defer resp.Body.Close()

	var q QuestionResponse
	json.NewDecoder(resp.Body).Decode(&q)

	if q.Options == nil {
		t.Error("AGENT FAILURE MODE: question options is null, should be []")
	}
	if len(q.Options) != 0 {
		t.Errorf("expected 0 options for question without options, got %d", len(q.Options))
	}
}

// [T023] [US-001] [AC-053] [INTEGRATION] GET questions for nonexistent feature returns 404

func TestIntegrationQuestion404s(t *testing.T) {
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

	// GET questions for nonexistent feature → 404
	resp, err := http.Get(ts.URL + "/api/features/nonexistent-feature-id/questions")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent feature, got %d", resp.StatusCode)
	}

	// GET pending questions for nonexistent feature → 404
	resp, err = http.Get(ts.URL + "/api/features/nonexistent-feature-id/questions/pending")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent feature (pending), got %d", resp.StatusCode)
	}

	// POST question for nonexistent feature → 404
	resp, err = http.Post(ts.URL+"/api/features/nonexistent-feature-id/questions", "application/json", strings.NewReader(`{"phase":"inception","role":"pm","question":"test?","type":"clarification"}`))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent feature (POST), got %d", resp.StatusCode)
	}
}

// [T024] [US-001] [AC-059] [INTEGRATION] Answering an assumed question returns 409

func TestIntegrationAnswerAssumedConflict(t *testing.T) {
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

	// Create a feature and question
	createBody := `{"type":"loose_idea","title":"Test Feature","description":"Testing","priority":2}`
	resp, _ := http.Post(ts.URL+"/api/features", "application/json", strings.NewReader(createBody))
	var created FeatureDetailResponse
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	featureID := created.ID

	questionBody := `{"phase":"inception","role":"pm","question":"What?","type":"clarification"}`
	resp, _ = http.Post(ts.URL+"/api/features/"+featureID+"/questions", "application/json", strings.NewReader(questionBody))
	var question QuestionResponse
	json.NewDecoder(resp.Body).Decode(&question)
	resp.Body.Close()

	// Manually mark question as assumed via the store
	qs := feature.NewFileQuestionStore(tmpDir)
	qs.AssumeQuestion(nil, featureID, question.ID, "Auto-assumed")

	// Try to answer the assumed question — should get 409
	answerBody := `{"answer":"My answer"}`
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/features/"+featureID+"/questions/"+question.ID, strings.NewReader(answerBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = http.DefaultClient.Do(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 Conflict for answering assumed question, got %d", resp.StatusCode)
	}
}
