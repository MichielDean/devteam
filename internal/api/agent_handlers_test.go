package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/spec"
)

func timeNowUTC() time.Time { return time.Now().UTC() }

func setupTestServerWithDB(t *testing.T) (*Server, *db.DB, string) {
	t.Helper()

	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

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

	database, err := db.Open(db.Config{Driver: "sqlite3", DSN: filepath.Join(tmpDir, "test.db")}, filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	s := NewServer(":0", sp, pipe, nil, questionStore, database)
	return s, database, tmpDir
}

// --- Signal endpoint tests ---

func TestSignalPass(t *testing.T) {
	s, database, tmpDir := setupTestServerWithDB(t)
	_ = tmpDir

	// Ensure feature exists in DB (FK constraint)
	database.Exec(`INSERT OR IGNORE INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count) VALUES (?, ?, 'inception', 'in_progress', 3, 'loose_idea', '', ?, ?, 0)`,
		"feat-sig-1", "feat-sig-1", timeNowUTC(), timeNowUTC())

	body := `{"outcome":"pass"}`
	req := httptest.NewRequest(http.MethodPost, "/api/features/feat-sig-1/signal", strings.NewReader(body))
	req.SetPathValue("id", "feat-sig-1")
	w := httptest.NewRecorder()
	s.handleSignal(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "recorded" {
		t.Errorf("status = %v, want recorded", resp["status"])
	}
	if resp["outcome"] != "pass" {
		t.Errorf("outcome = %v, want pass", resp["outcome"])
	}

	// Verify event was recorded
	var eventCount int
	database.QueryRow(`SELECT COUNT(*) FROM events WHERE feature_id = ? AND event_type = ?`, "feat-sig-1", "phase_complete").Scan(&eventCount)
	if eventCount != 1 {
		t.Errorf("expected 1 phase_complete event, got %d", eventCount)
	}
}

func TestSignalRecirculateWithNotes(t *testing.T) {
	s, database, _ := setupTestServerWithDB(t)

	database.Exec(`INSERT OR IGNORE INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count) VALUES (?, ?, 'inception', 'in_progress', 3, 'loose_idea', '', ?, ?, 0)`,
		"feat-recirc-1", "feat-recirc-1", timeNowUTC(), timeNowUTC())

	body := `{"outcome":"recirculate:construction","target":"construction","notes":"fix the bug in line 42"}`
	req := httptest.NewRequest(http.MethodPost, "/api/features/feat-recirc-1/signal", strings.NewReader(body))
	req.SetPathValue("id", "feat-recirc-1")
	w := httptest.NewRecorder()
	s.handleSignal(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify recirculate event
	var eventCount int
	database.QueryRow(`SELECT COUNT(*) FROM events WHERE feature_id = ? AND event_type = ?`, "feat-recirc-1", "recirculate").Scan(&eventCount)
	if eventCount != 1 {
		t.Errorf("expected 1 recirculate event, got %d", eventCount)
	}

	// Verify outcome was recorded in outcomes table
	var outcomeCount int
	database.QueryRow(`SELECT COUNT(*) FROM outcomes WHERE feature_id = ? AND outcome = ?`, "feat-recirc-1", "recirculate").Scan(&outcomeCount)
	if outcomeCount != 1 {
		t.Errorf("expected 1 recirculate outcome, got %d", outcomeCount)
	}
}

func TestSignalNeedsFeedback(t *testing.T) {
	s, database, _ := setupTestServerWithDB(t)

	database.Exec(`INSERT OR IGNORE INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count) VALUES (?, ?, 'inception', 'in_progress', 3, 'loose_idea', '', ?, ?, 0)`,
		"feat-feedback-1", "feat-feedback-1", timeNowUTC(), timeNowUTC())

	body := `{"outcome":"needs_feedback"}`
	req := httptest.NewRequest(http.MethodPost, "/api/features/feat-feedback-1/signal", strings.NewReader(body))
	req.SetPathValue("id", "feat-feedback-1")
	w := httptest.NewRecorder()
	s.handleSignal(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var eventCount int
	database.QueryRow(`SELECT COUNT(*) FROM events WHERE feature_id = ? AND event_type = ?`, "feat-feedback-1", "needs_feedback").Scan(&eventCount)
	if eventCount != 1 {
		t.Errorf("expected 1 needs_feedback event, got %d", eventCount)
	}
}

func TestSignalValidationErrors(t *testing.T) {
	s, _, _ := setupTestServerWithDB(t)

	cases := []struct {
		name string
		body string
	}{
		{"missing outcome", `{}`},
		{"invalid json", `{bad`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/features/feat-1/signal", strings.NewReader(tc.body))
			req.SetPathValue("id", "feat-1")
			w := httptest.NewRecorder()
			s.handleSignal(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for %s, got %d", tc.name, w.Code)
			}
		})
	}
}

// --- Notes endpoint tests ---

func TestAddNote(t *testing.T) {
	s, database, _ := setupTestServerWithDB(t)

	database.Exec(`INSERT OR IGNORE INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count) VALUES (?, ?, 'inception', 'in_progress', 3, 'loose_idea', '', ?, ?, 0)`,
		"feat-note-1", "feat-note-1", timeNowUTC(), timeNowUTC())

	body := `{"phase":"construction","content":"agent found issue in auth module"}`
	req := httptest.NewRequest(http.MethodPost, "/api/features/feat-note-1/notes", strings.NewReader(body))
	req.SetPathValue("id", "feat-note-1")
	w := httptest.NewRecorder()
	s.handleAddNote(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var noteCount int
	database.QueryRow(`SELECT COUNT(*) FROM notes WHERE feature_id = ? AND phase = ?`, "feat-note-1", "construction").Scan(&noteCount)
	if noteCount != 1 {
		t.Errorf("expected 1 note, got %d", noteCount)
	}
}

func TestAddNoteValidationErrors(t *testing.T) {
	s, _, _ := setupTestServerWithDB(t)

	cases := []struct {
		name string
		body string
	}{
		{"empty content", `{"phase":"inception","content":""}`},
		{"missing content", `{"phase":"inception"}`},
		{"invalid json", `{bad`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/features/feat-1/notes", strings.NewReader(tc.body))
			req.SetPathValue("id", "feat-1")
			w := httptest.NewRecorder()
			s.handleAddNote(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for %s, got %d", tc.name, w.Code)
			}
		})
	}
}

// --- Artifact submit/get endpoint tests ---

func TestSubmitAndGetArtifact(t *testing.T) {
	s, database, _ := setupTestServerWithDB(t)

	// ensureFeatureInDB will create the feature row
	database.Exec(`INSERT OR IGNORE INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count) VALUES (?, ?, 'inception', 'in_progress', 3, 'loose_idea', '', ?, ?, 0)`,
		"feat-art-1", "feat-art-1", timeNowUTC(), timeNowUTC())

	// Submit artifact
	body := `{"content":"# My Spec\n\nThis is a spec document."}`
	req := httptest.NewRequest(http.MethodPost, "/api/features/feat-art-1/artifacts/spec", strings.NewReader(body))
	req.SetPathValue("id", "feat-art-1")
	req.SetPathValue("type", "spec")
	w := httptest.NewRecorder()
	s.handleSubmitArtifact(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("submit: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "saved" {
		t.Errorf("status = %v, want saved", resp["status"])
	}
	if resp["size"] != float64(len("# My Spec\n\nThis is a spec document.")) {
		t.Errorf("size = %v, want %d", resp["size"], len("# My Spec\n\nThis is a spec document."))
	}

	// Get artifact back via getArtifact handler
	req = httptest.NewRequest(http.MethodGet, "/api/features/feat-art-1/artifacts/spec", nil)
	req.SetPathValue("id", "feat-art-1")
	req.SetPathValue("type", "spec")
	w = httptest.NewRecorder()
	s.getArtifact(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "# My Spec\n\nThis is a spec document." {
		t.Errorf("content = %q, want spec content", w.Body.String())
	}
}

func TestSubmitArtifactValidationErrors(t *testing.T) {
	s, _, _ := setupTestServerWithDB(t)

	cases := []struct {
		name string
		body string
	}{
		{"empty content", `{"content":""}`},
		{"missing content", `{}`},
		{"invalid json", `{bad`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/features/feat-1/artifacts/spec", strings.NewReader(tc.body))
			req.SetPathValue("id", "feat-1")
			req.SetPathValue("type", "spec")
			w := httptest.NewRecorder()
			s.handleSubmitArtifact(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for %s, got %d", tc.name, w.Code)
			}
		})
	}
}

func TestGetArtifactFromDBOnly(t *testing.T) {
	// Artifact submitted via DB should be retrievable even if not on disk
	s, database, _ := setupTestServerWithDB(t)

	database.Exec(`INSERT OR IGNORE INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count) VALUES (?, ?, 'inception', 'in_progress', 3, 'loose_idea', '', ?, ?, 0)`,
		"feat-db-only", "feat-db-only", timeNowUTC(), timeNowUTC())

	database.SaveArtifact("feat-db-only", "plan_md", "# Plan from DB")

	req := httptest.NewRequest(http.MethodGet, "/api/features/feat-db-only/artifacts/plan", nil)
	req.SetPathValue("id", "feat-db-only")
	req.SetPathValue("type", "plan")
	w := httptest.NewRecorder()
	s.getArtifact(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "# Plan from DB" {
		t.Errorf("content = %q, want # Plan from DB", w.Body.String())
	}
}

func TestGetArtifactMissingReturns404(t *testing.T) {
	s, _, tmpDir := setupTestServerWithDB(t)

	// Create a feature on disk so getFeature passes, but no artifact
	sp := spec.NewSpecProvider(tmpDir)
	sw := spec.NewSpecWriter(tmpDir)
	f := feature.NewFeature("feat-missing-art", "Missing Art", 1, feature.IntakeLooseIdea)
	sw.CreateFeatureDir(f.ID)
	sp.SaveFeatureState(f)

	req := httptest.NewRequest(http.MethodGet, "/api/features/feat-missing-art/artifacts/spec", nil)
	req.SetPathValue("id", "feat-missing-art")
	req.SetPathValue("type", "spec")
	w := httptest.NewRecorder()
	s.getArtifact(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}