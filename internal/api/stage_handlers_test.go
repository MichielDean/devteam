package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/role"
	"github.com/MichielDean/devteam/internal/spec"
	"github.com/MichielDean/devteam/internal/stage"
)

// setupStageTestServer creates a server with AIDLC v2 stage support.
func setupStageTestServer(t *testing.T) (*Server, string, *db.DB) {
	t.Helper()

	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Pipeline: config.PipelineConfig{
			Phases: []config.PhaseConfig{
				{Name: "inception", Roles: []string{"product"}},
				{Name: "construction", Roles: []string{"developer"}},
			},
		},
	}

	sp := spec.NewSpecProvider(tmpDir)
	dispatcher := role.NewDispatcher(tmpDir)
	pipe := pipeline.NewPipelineWithDispatcher(cfg, sp, dispatcher)
	database := setupStageTestDB(t, tmpDir)
	sp.SetDatabase(database)
	pipe.SetDatabase(database)
	questionStore := feature.NewDBQuestionStore(database)

	// Seed stage definitions
	if err := stage.SeedStages(database); err != nil {
		t.Fatalf("SeedStages: %v", err)
	}

	s := NewServer(":0", sp, pipe, nil, questionStore, database)
	return s, tmpDir, database
}

func setupStageTestDB(t *testing.T, tmpDir string) *db.DB {
	t.Helper()
	dsn := "host=localhost port=5432 user=devteam password=devteam dbname=devteam_test_api sslmode=disable"
	database, err := db.Open(db.Config{DSN: dsn}, dsn)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	// Truncate all data tables for clean test state
	truncateTables(database)
	t.Cleanup(func() { database.Close() })
	return database
}

// insertTestFeature inserts a feature into the DB and spec provider, initializes its stages.
func insertTestFeature(t *testing.T, database *db.DB, scope string, s *Server) string {
	t.Helper()
	fid := "test-feat-" + scope
	now := time.Now().UTC()
	_, err := database.Exec(
		`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0) ON CONFLICT (id) DO UPDATE SET title = excluded.title, current_phase = excluded.current_phase, status = excluded.status, priority = excluded.priority, intake_path = excluded.intake_path, spec_dir = excluded.spec_dir, created_at = excluded.created_at, updated_at = excluded.updated_at, recirculation_count = excluded.recirculation_count`,
		fid, "Test Feature "+scope, "ideation", "draft", 2, "loose_idea", "specs/"+fid, now, now,
	)
	if err != nil {
		t.Fatalf("inserting test feature: %v", err)
	}
	// Also save to spec provider so GetFeature works
	f := feature.NewFeature(fid, "Test Feature "+scope, 2, feature.IntakeLooseIdea)
	f.Scope = scope
	if err := s.specProvider.SaveFeatureState(f); err != nil {
		t.Fatalf("SaveFeatureState: %v", err)
	}
	if err := database.InitFeatureStages(fid, scope); err != nil {
		t.Fatalf("InitFeatureStages: %v", err)
	}
	return fid
}

func doRequest(t *testing.T, s *Server, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshaling body: %v", err)
		}
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)
	return w
}

// ─── Tests ───

func TestGetFeatureStages(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	w := doRequest(t, s, "GET", "/api/features/"+fid+"/stages", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var stages []db.FeatureStage
	if err := json.Unmarshal(w.Body.Bytes(), &stages); err != nil {
		t.Fatalf("unmarshaling stages: %v", err)
	}
	if len(stages) == 0 {
		t.Fatal("expected stages, got 0")
	}
}

func TestGetFeatureStagesEmpty(t *testing.T) {
	s, _, _ := setupStageTestServer(t)
	w := doRequest(t, s, "GET", "/api/features/nonexistent/stages", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetAuditTrail(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	database.RecordAuditEvent(fid, db.AuditStageStart, "1.1", "ideation", "test")

	w := doRequest(t, s, "GET", "/api/features/"+fid+"/audit", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var events []db.AuditEvent
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Fatalf("unmarshaling events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != db.AuditStageStart {
		t.Errorf("event type = %s, want %s", events[0].EventType, db.AuditStageStart)
	}
}

func TestSetScope(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	w := doRequest(t, s, "POST", "/api/features/"+fid+"/scope", map[string]string{"scope": "bugfix"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetScopeInvalid(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	w := doRequest(t, s, "POST", "/api/features/"+fid+"/scope", map[string]string{"scope": "invalid"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid scope, got %d", w.Code)
	}
}

func TestSetDepth(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	w := doRequest(t, s, "POST", "/api/features/"+fid+"/depth", map[string]string{"depth": "comprehensive"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetTestStrategy(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	w := doRequest(t, s, "POST", "/api/features/"+fid+"/test-strategy", map[string]string{"test_strategy": "minimal"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetLadderMode(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	w := doRequest(t, s, "POST", "/api/features/"+fid+"/ladder", map[string]string{"mode": "gated"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetLadderModeInvalid(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	w := doRequest(t, s, "POST", "/api/features/"+fid+"/ladder", map[string]string{"mode": "invalid"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid mode, got %d", w.Code)
	}
}

func TestGetBolts(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	database.CreateBolt(fid, 1, []string{"unit-1"}, true)
	database.CreateBolt(fid, 2, []string{"unit-2"}, false)

	w := doRequest(t, s, "GET", "/api/features/"+fid+"/bolts", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var bolts []db.BoltRow
	if err := json.Unmarshal(w.Body.Bytes(), &bolts); err != nil {
		t.Fatalf("unmarshaling bolts: %v", err)
	}
	if len(bolts) != 2 {
		t.Fatalf("expected 2 bolts, got %d", len(bolts))
	}
	if bolts[0].BoltNumber != 1 {
		t.Errorf("bolt number = %d, want 1", bolts[0].BoltNumber)
	}
	if !bolts[0].IsWalkingSkeleton {
		t.Error("bolt 1 should be walking skeleton")
	}
}

func TestGetRules(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	database.SaveRule(fid, "product", "1.1", "Always include error case", "Missing error case")

	w := doRequest(t, s, "GET", "/api/features/"+fid+"/rules", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var rules []db.RuleRow
	if err := json.Unmarshal(w.Body.Bytes(), &rules); err != nil {
		t.Fatalf("unmarshaling rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].AgentName != "product" {
		t.Errorf("agent = %s, want product", rules[0].AgentName)
	}
}

func TestDeleteRule(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	database.SaveRule(fid, "product", "1.1", "test rule", "test")
	rules, _ := database.GetRulesForFeature(fid)
	if len(rules) != 1 {
		t.Fatal("expected 1 rule before delete")
	}

	w := doRequest(t, s, "DELETE", fmt.Sprintf("/api/features/%s/rules/%d", fid, rules[0].ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	rules, _ = database.GetRulesForFeature(fid)
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules after delete, got %d", len(rules))
	}
}

func TestSaveKnowledge(t *testing.T) {
	s, _, _ := setupStageTestServer(t)

	w := doRequest(t, s, "POST", "/api/knowledge/product", map[string]string{
		"topic":   "coding-standards",
		"content": "Use tabs not spaces",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSaveKnowledgeMissingFields(t *testing.T) {
	s, _, _ := setupStageTestServer(t)

	w := doRequest(t, s, "POST", "/api/knowledge/product", map[string]string{
		"topic": "coding-standards",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing content, got %d", w.Code)
	}
}

func TestGetKnowledge(t *testing.T) {
	s, _, database := setupStageTestServer(t)

	database.SaveTeamKnowledge("product", "api-conventions", "REST endpoints plural")

	w := doRequest(t, s, "GET", "/api/knowledge/product", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var entries []db.TeamKnowledgeRow
	if err := json.Unmarshal(w.Body.Bytes(), &entries); err != nil {
		t.Fatalf("unmarshaling knowledge: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Topic != "api-conventions" {
		t.Errorf("topic = %s, want api-conventions", entries[0].Topic)
	}
}

func TestListAllKnowledge(t *testing.T) {
	s, _, database := setupStageTestServer(t)

	database.SaveTeamKnowledge("product", "topic1", "content1")
	database.SaveTeamKnowledge("architect", "topic2", "content2")

	w := doRequest(t, s, "GET", "/api/knowledge", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result map[string][]db.TeamKnowledgeRow
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshaling knowledge: %v", err)
	}
	if len(result) < 2 {
		t.Fatalf("expected at least 2 agents with knowledge, got %d", len(result))
	}
}

func TestDeleteKnowledge(t *testing.T) {
	s, _, database := setupStageTestServer(t)

	database.SaveTeamKnowledge("product", "to-delete", "content")

	w := doRequest(t, s, "DELETE", "/api/knowledge/product/to-delete", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	entries, _ := database.GetTeamKnowledge("product")
	for _, e := range entries {
		if e.Topic == "to-delete" {
			t.Fatal("knowledge entry should be deleted")
		}
	}
}

func TestJumpToStageMissingBoth(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	w := doRequest(t, s, "POST", "/api/features/"+fid+"/jump", map[string]string{})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when no stage_id or phase, got %d", w.Code)
	}
}

func TestApproveStageNotFound(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	w := doRequest(t, s, "POST", "/api/features/"+fid+"/stages/9.9/approve", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-existent stage, got %d", w.Code)
	}
}

func TestRejectStageMissingNotes(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	w := doRequest(t, s, "POST", "/api/features/"+fid+"/stages/1.1/reject", map[string]string{})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing notes, got %d", w.Code)
	}
}

func TestAcceptAsIsNotEnoughRevisions(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	w := doRequest(t, s, "POST", "/api/features/"+fid+"/stages/1.1/accept-as-is", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for accept-as-is with 0 revisions, got %d", w.Code)
	}
}

func TestRunStageInvalidStageID(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	w := doRequest(t, s, "POST", "/api/features/"+fid+"/run-stage", map[string]string{"stage_id": "invalid"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid stage_id, got %d", w.Code)
	}
}

func TestRunStageMissingStageID(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	fid := insertTestFeature(t, database, "feature", s)

	w := doRequest(t, s, "POST", "/api/features/"+fid+"/run-stage", map[string]string{})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing stage_id, got %d", w.Code)
	}
}