package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/spec"
)

// setupTestServerWithServer is like setupTestServer but returns a running
// httptest.Server using the FULL mux + real middleware chain, so route
// wiring, middleware ordering, and CORS headers are exercised.
func setupTestServerWithServer(t *testing.T) (*Server, *httptest.Server, string) {
	t.Helper()
	s, tmpDir := setupTestServer(t)
	ts := httptest.NewServer(s.httpServer.Handler)
	t.Cleanup(ts.Close)
	return s, ts, tmpDir
}

// seedWaitingHumanFeature creates a feature, sets it to in_progress in inception,
// then transitions to waiting_for_human. Returns the feature ID.
func seedWaitingHumanFeature(t *testing.T, tmpDir string) string {
	t.Helper()
	sp := spec.NewSpecProvider(tmpDir)
	sw := spec.NewSpecWriter(tmpDir)
	fid := "waiting-human-feature"
	f := feature.NewFeature(fid, "Waiting Human Feature", 1, feature.IntakeLooseIdea)
	if err := sw.CreateFeatureDir(f.ID); err != nil {
		t.Fatalf("CreateFeatureDir: %v", err)
	}
	f.Start() // in_progress, inception
	if err := f.WaitForHuman(); err != nil {
		t.Fatalf("WaitForHuman: %v", err)
	}
	if err := sp.SaveFeatureState(f); err != nil {
		t.Fatalf("SaveFeatureState: %v", err)
	}
	return fid
}

// [T-AC076] [US-003] [AC-076] [INTEGRATION] Cancel a feature in waiting_for_human
// status transitions it to cancelled.
func TestIntegrationCancelWaitingHumanFeature(t *testing.T) {
	_, ts, tmpDir := setupTestServerWithServer(t)
	fid := seedWaitingHumanFeature(t, tmpDir)

	resp, err := http.Post(ts.URL+"/api/features/"+fid+"/cancel", "application/json", nil)
	if err != nil {
		t.Fatalf("POST cancel: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var detail FeatureDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if detail.Status != string(feature.StatusCancelled) {
		t.Errorf("expected status cancelled, got %s", detail.Status)
	}
}

// [T-AC077] [US-003] [AC-077] [INTEGRATION] Recirculating a feature in
// waiting_for_human status deletes all its questions.
func TestIntegrationRecirculateWaitingHumanClearsQuestions(t *testing.T) {
	s, ts, tmpDir := setupTestServerWithServer(t)
	fid := seedWaitingHumanFeature(t, tmpDir)

	// Seed a question directly via the handler
	createBody := `{"phase":"inception","role":"pm","question":"q?","type":"clarification"}`
	req := httptest.NewRequest(http.MethodPost, "/api/features/"+fid+"/questions", strings.NewReader(createBody))
	req.SetPathValue("id", fid)
	w := httptest.NewRecorder()
	s.createQuestion(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create question: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Recirculate to inception (current is inception, so recirculate target must be < current)
	// Feature is at inception. Recirculation requires target < current. Since inception is
	// the first phase, we need to advance the feature first. Instead, set feature to planning
	// then waiting_for_human, then recirculate back to inception.
	sp := spec.NewSpecProvider(tmpDir)
	f, err := sp.LoadFeatureState(fid)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// Reset to in_progress and advance to planning
	if err := f.ResumeFromWaitingHuman(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	// Mark inception passed, move to planning
	f.PhaseStates[feature.PhaseInception].Status = feature.StatusPassed
	f.Current = feature.PhasePlanning
	f.PhaseStates[feature.PhasePlanning].Status = feature.StatusInProgress
	f.Status = feature.StatusInProgress
	if err := f.WaitForHuman(); err != nil {
		t.Fatalf("WaitForHuman in planning: %v", err)
	}
	if err := sp.SaveFeatureState(f); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Now recirculate back to inception
	recircBody := `{"target_phase":"inception"}`
	resp, err := http.Post(ts.URL+"/api/features/"+fid+"/recirculate", "application/json", strings.NewReader(recircBody))
	if err != nil {
		t.Fatalf("POST recirculate: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, readBody(resp))
	}

	// Verify questions are cleared
	getResp, err := http.Get(ts.URL + "/api/features/" + fid + "/questions")
	if err != nil {
		t.Fatalf("GET questions: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", getResp.StatusCode)
	}
	body := strings.TrimSpace(readBody(getResp))
	if body != "[]" {
		t.Errorf("expected questions to be cleared (body []), got %s", body)
	}
}

// [T-AC086] [US-001] [AC-086] [INTEGRATION] Concurrent answers to the same
// pending question: exactly one wins (200), the other gets 409.
//
// Uses PATCH via http.NewRequest (the route is registered for MethodPatch).
// http.Post would issue POST and get 405 Method Not Allowed.
func TestIntegrationConcurrentAnswerConflict(t *testing.T) {
	s, ts, tmpDir := setupTestServerWithServer(t)
	fid := seedQuestionFeature(t, tmpDir)

	createBody := `{"phase":"inception","role":"pm","question":"concurrent?","type":"clarification"}`
	req := httptest.NewRequest(http.MethodPost, "/api/features/"+fid+"/questions", strings.NewReader(createBody))
	req.SetPathValue("id", fid)
	w := httptest.NewRecorder()
	s.createQuestion(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", w.Code)
	}
	var created QuestionResponse
	json.NewDecoder(w.Body).Decode(&created)
	qid := created.ID

	answerBody := []byte(`{"answer":"concurrent answer"}`)
	var wg sync.WaitGroup
	statusCodes := make([]int, 2)
	const goroutines = 2

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			r, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/features/"+fid+"/questions/"+qid, bytes.NewReader(answerBody))
			r.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(r)
			if err != nil {
				statusCodes[idx] = -1
				return
			}
			defer resp.Body.Close()
			statusCodes[idx] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	has200 := false
	has409 := false
	for _, code := range statusCodes {
		if code == http.StatusOK {
			has200 = true
		}
		if code == http.StatusConflict {
			has409 = true
		}
	}
	if !has200 {
		t.Errorf("expected at least one 200 response, got %v", statusCodes)
	}
	if !has409 {
		t.Errorf("expected at least one 409 response, got %v", statusCodes)
	}
}

// [T-AC086] [US-001] [AC-086] [INTEGRATION] Concurrent PATCH answers to the
// same pending question: exactly one wins (200), the other gets 409.
func TestIntegrationConcurrentPatchAnswerConflict(t *testing.T) {
	s, ts, tmpDir := setupTestServerWithServer(t)
	fid := seedQuestionFeature(t, tmpDir)

	createBody := `{"phase":"inception","role":"pm","question":"concurrent patch?","type":"clarification"}`
	req := httptest.NewRequest(http.MethodPost, "/api/features/"+fid+"/questions", strings.NewReader(createBody))
	req.SetPathValue("id", fid)
	w := httptest.NewRecorder()
	s.createQuestion(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", w.Code)
	}
	var created QuestionResponse
	json.NewDecoder(w.Body).Decode(&created)
	qid := created.ID

	answerBody := []byte(`{"answer":"concurrent patch answer"}`)
	var wg sync.WaitGroup
	statusCodes := make([]int, 2)
	const goroutines = 2

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/features/"+fid+"/questions/"+qid, bytes.NewReader(answerBody))
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				statusCodes[idx] = -1
				return
			}
			defer resp.Body.Close()
			statusCodes[idx] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	has200 := false
	has409 := false
	for _, code := range statusCodes {
		if code == http.StatusOK {
			has200 = true
		}
		if code == http.StatusConflict {
			has409 = true
		}
	}
	// Note: file-based store with mutex serializes writes, so both could
	// theoretically both see pending if the mutex weren't there. With the
	// mutex, the first wins (200) and second sees status != pending (409).
	if !has200 {
		t.Errorf("expected at least one 200 response, got %v", statusCodes)
	}
	if !has409 {
		t.Errorf("expected at least one 409 response, got %v", statusCodes)
	}
}

// [T-AC032] [US-006] [AC-032] [INTEGRATION] Feature list includes
// pending_questions_count so the UI can render a badge.
func TestIntegrationFeatureListPendingQuestionsCount(t *testing.T) {
	s, _, tmpDir := setupTestServerWithServer(t)
	fid := seedQuestionFeature(t, tmpDir)

	// Create 3 pending questions
	for i := 0; i < 3; i++ {
		body := fmt.Sprintf(`{"phase":"inception","role":"pm","question":"q%d?","type":"clarification"}`, i)
		req := httptest.NewRequest(http.MethodPost, "/api/features/"+fid+"/questions", strings.NewReader(body))
		req.SetPathValue("id", fid)
		w := httptest.NewRecorder()
		s.createQuestion(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %d: expected 201, got %d", i, w.Code)
		}
	}

	// List features and check the count
	req := httptest.NewRequest(http.MethodGet, "/api/features", nil)
	w := httptest.NewRecorder()
	s.listFeatures(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Features []FeatureSummaryResponse `json:"features"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var found bool
	for _, f := range resp.Features {
		if f.ID == fid {
			found = true
			if f.PendingQuestionsCount != 3 {
				t.Errorf("expected pending_questions_count=3, got %d", f.PendingQuestionsCount)
			}
		}
	}
	if !found {
		t.Errorf("feature %s not found in list", fid)
	}
}

// [T-AC087] [US-006] [AC-087] [SMOKE] Full httptest.Server smoke test of
// every question endpoint through the real mux + middleware chain.
func TestSmokeQuestionEndpointsViaRealServer(t *testing.T) {
	_, ts, tmpDir := setupTestServerWithServer(t)
	fid := seedQuestionFeature(t, tmpDir)

	// GET empty questions
	resp, err := http.Get(ts.URL + "/api/features/" + fid + "/questions")
	if err != nil {
		t.Fatalf("GET questions: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET questions empty: expected 200, got %d", resp.StatusCode)
	}
	emptyBody := strings.TrimSpace(readBody(resp))
	resp.Body.Close()
	if emptyBody != "[]" {
		t.Errorf("GET questions empty: expected [], got %s", emptyBody)
	}

	// POST create
	createBody := `{"phase":"inception","role":"pm","question":"smoke?","type":"clarification","options":["a","b"]}`
	resp, err = http.Post(ts.URL+"/api/features/"+fid+"/questions", "application/json", strings.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST question: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("POST question: expected 201, got %d: %s", resp.StatusCode, readBody(resp))
	}
	var created QuestionResponse
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.ID != "Q-001" {
		t.Errorf("expected Q-001, got %s", created.ID)
	}

	// GET questions (1 item)
	resp, err = http.Get(ts.URL + "/api/features/" + fid + "/questions")
	if err != nil {
		t.Fatalf("GET questions: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET questions: expected 200, got %d", resp.StatusCode)
	}
	body := readBody(resp)
	resp.Body.Close()
	if strings.Contains(body, `"options":null`) {
		t.Errorf("GET questions: options is null, should be []: %s", body)
	}
	if !strings.Contains(body, `"options":["a","b"]`) {
		t.Errorf("GET questions: options not correctly serialized: %s", body)
	}

	// GET pending
	resp, err = http.Get(ts.URL + "/api/features/" + fid + "/questions/pending")
	if err != nil {
		t.Fatalf("GET pending: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET pending: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// PATCH answer
	patchReq, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/features/"+fid+"/questions/"+created.ID, strings.NewReader(`{"answer":"picked a"}`))
	patchReq.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(patchReq)
	if err != nil {
		t.Fatalf("PATCH answer: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("PATCH answer: expected 200, got %d: %s", resp.StatusCode, readBody(resp))
	}
	resp.Body.Close()

	// CORS preflight on PATCH
	preflightReq, _ := http.NewRequest(http.MethodOptions, ts.URL+"/api/features/"+fid+"/questions/"+created.ID, nil)
	preflightReq.Header.Set("Access-Control-Request-Method", "PATCH")
	preflightReq.Header.Set("Access-Control-Request-Headers", "Content-Type")
	resp, err = http.DefaultClient.Do(preflightReq)
	if err != nil {
		t.Fatalf("OPTIONS preflight: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("OPTIONS preflight: expected 204, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); !strings.Contains(got, "PATCH") {
		t.Errorf("CORS methods missing PATCH: %s", got)
	}
	resp.Body.Close()

	// Malformed JSON POST — expect 400, not panic
	resp, err = http.Post(ts.URL+"/api/features/"+fid+"/questions", "application/json", strings.NewReader(`{bad`))
	if err != nil {
		t.Fatalf("POST malformed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("POST malformed: expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Server still alive
	resp, err = http.Get(ts.URL + "/api/features/" + fid + "/questions")
	if err != nil {
		t.Fatalf("GET after malformed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET after malformed: expected 200 (server alive), got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}