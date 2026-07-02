package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/intake"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/spec"
	"github.com/MichielDean/devteam/internal/stage"
)

type Server struct {
	httpServer   *http.Server
	pipeline     *pipeline.Pipeline
	specProvider *spec.SpecProvider
	db           *db.DB
	active       sync.Map // featureID -> struct{} (set of currently running features)
	sseMu        sync.Mutex
	sseClients   map[string][]chan SSEMessage
	sseBuffers   map[string][]*SSEMessage
	baseDir      string
	staticFS     fs.FS
	questionStore feature.QuestionStore
}

func NewServer(addr string, specProvider *spec.SpecProvider, pipeline *pipeline.Pipeline, staticFS fs.FS, questionStore feature.QuestionStore, database *db.DB) *Server {
	s := &Server{
		specProvider:  specProvider,
		pipeline:      pipeline,
		baseDir:       specProvider.BaseDir(),
		staticFS:      staticFS,
		questionStore: questionStore,
		db:            database,
		sseClients:    make(map[string][]chan SSEMessage),
		sseBuffers:    make(map[string][]*SSEMessage),
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/features", s.listFeatures)
	mux.HandleFunc("POST /api/features", s.createFeature)
	mux.HandleFunc("GET /api/features/{id}", s.getFeature)
	mux.HandleFunc("POST /api/features/{id}/cancel", s.cancelFeature)
	mux.HandleFunc("GET /api/features/{id}/artifacts/{type}", s.getArtifact)
	mux.HandleFunc("POST /api/features/{id}/artifacts/{type}", s.handleSubmitArtifact)
	mux.HandleFunc("GET /api/features/{id}/stream", s.streamFeature)
	mux.HandleFunc("GET /api/features/{id}/output", s.getCapturedOutput)

	// Agent CLI endpoints (called by devteam CLI from agents)
	mux.HandleFunc("POST /api/features/{id}/signal", s.handleSignal)
	mux.HandleFunc("POST /api/features/{id}/notes", s.handleAddNote)

	// Question endpoints
	mux.HandleFunc("GET /api/features/{id}/questions/pending", s.listPendingQuestions)
	mux.HandleFunc("GET /api/features/{id}/questions", s.listQuestions)
	mux.HandleFunc("POST /api/features/{id}/questions", s.createQuestion)
	mux.HandleFunc("PATCH /api/features/{id}/questions/{questionId}", s.answerQuestion)

	// Database-backed history endpoints
	mux.HandleFunc("GET /api/features/{id}/notes", s.getNotes)

	// AIDLC v2 stage-based endpoints
	s.registerStageRoutes(mux)

	if staticFS != nil {
		mux.Handle("/", s.spaHandler(staticFS))
	}

	handler := s.recoveryMiddleware(s.corsMiddleware(mux))

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	return s
}

func (s *Server) Start() error {
	log.Printf("Dev Team Web UI starting on %s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) IsProcessing(id string) bool {
	_, loaded := s.active.Load(id)
	return loaded
}

// RestoreActiveProcesses restores active state from tmux sessions on startup.
// Features with status=in_progress but no tmux session are marked failed —
// user re-runs manually. No auto-resume, no credit burn.
func (s *Server) RestoreActiveProcesses() {
	sessions := s.pipeline.Dispatcher().ListActiveSessions()
	for featureID := range sessions {
		s.active.Store(featureID, struct{}{})
		log.Printf("restored active state for feature %s from tmux session", featureID)
	}

	// Mark orphaned features as failed
	features, err := s.pipeline.ListFeatures()
	if err != nil {
		log.Printf("RestoreActiveProcesses: failed to list features: %v", err)
		return
	}
	for _, f := range features {
		if f.IsTerminal() {
			continue
		}
		if f.Status != feature.StatusInProgress {
			continue
		}
		if s.IsProcessing(f.ID) {
			continue
		}
		log.Printf("RestoreActiveProcesses: feature %s was in_progress but no tmux session — marking interrupted", f.ID)
		f.Status = feature.StatusFailed
		s.pipeline.SaveFeature(f)
		s.broadcastSSE(f.ID, "interrupted", fmt.Sprintf(`{"feature_id":"%s","message":"Interrupted by server restart. Re-run manually."}`, f.ID))
	}
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) spaHandler(staticFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(staticFS))
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/") {
			http.NotFound(w, r)
			return
		}
		f, err := staticFS.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	}
}

// runPhaseGoroutine is the single entry point for dispatching a phase.
// Replaces the 6 duplicated inline goroutines. Dispatches one phase, broadcasts
// SSE events, removes the feature from active set on completion.
func (s *Server) runPhaseGoroutine(id string, currentPhase feature.Phase) {
	defer s.active.Delete(id)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in runPhase goroutine for %s: %v", id, r)
			s.broadcastSSE(id, "error", fmt.Sprintf(`{"feature_id":"%s","message":"Internal error (recovered). Check logs."}`, id))
		}
	}()

	ctx := context.Background()

	s.broadcastSSE(id, "phase_change", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","status":"in_progress"}`, id, currentPhase))
	s.broadcastSSE(id, "agent_dispatch", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","role":"%s","status":"dispatched"}`, id, currentPhase, s.pipeline.PrimaryRole(currentPhase)))

	onOutput := func(line string, isStderr bool) {
		escaped, _ := json.Marshal(line)
		s.broadcastSSE(id, "agent_output", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","line":%s,"stderr":%v}`, id, currentPhase, string(escaped), isStderr))
	}

	f, err := s.pipeline.GetFeature(id)
	if err != nil {
		s.broadcastSSE(id, "error", fmt.Sprintf(`{"feature_id":"%s","message":"Failed to load feature: %s"}`, id, err.Error()))
		return
	}

	result, err := s.pipeline.RunPhase(ctx, f, onOutput)
	if err != nil {
		log.Printf("error running phase for feature %s: %v", id, err)
		s.broadcastSSE(id, "error", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","message":"Phase execution failed: %s"}`, id, currentPhase, err.Error()))
		return
	}
	log.Printf("phase %s completed for feature %s in %v", currentPhase, id, result.Duration)

	s.broadcastSSE(id, "agent_complete", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","role":"%s","status":"success","duration_ms":%d}`, id, currentPhase, s.pipeline.PrimaryRole(currentPhase), result.Duration.Milliseconds()))

	// Broadcast gate result (smoke check)
	passed := len(result.SmokeFailures) == 0
	checks := make([]map[string]interface{}, 0, len(result.SmokeFailures)+1)
	if passed {
		checks = append(checks, map[string]interface{}{"name": "smoke_check", "passed": true})
	} else {
		for _, fail := range result.SmokeFailures {
			checks = append(checks, map[string]interface{}{"name": "smoke_check", "passed": false, "message": fail})
		}
	}
	checksJSON, _ := json.Marshal(checks)
	s.broadcastSSE(id, "gate_result", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","passed":%v,"checks":%s}`, id, currentPhase, passed, string(checksJSON)))

	s.broadcastSSE(id, "phase_complete", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","status":"complete"}`, id, currentPhase))
}

func (s *Server) listFeatures(w http.ResponseWriter, r *http.Request) {
	features, err := s.pipeline.ListFeatures()
	if err != nil {
		log.Printf("error listing features: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list features")
		return
	}

	resp := FeaturesToSummaryResponse(features, s.questionStore)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) createFeature(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CreateFeatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "empty_title", "Title is required")
		return
	}
	if len(req.Title) > 200 {
		writeError(w, http.StatusBadRequest, "title_too_long", "Title must be 200 characters or less")
		return
	}
	if strings.TrimSpace(req.Description) == "" {
		writeError(w, http.StatusBadRequest, "empty_description", "Description is required")
		return
	}
	if req.Priority == 0 {
		req.Priority = 2
	}
	if req.Priority < 1 || req.Priority > 3 {
		writeError(w, http.StatusBadRequest, "invalid_priority", "Priority must be 1, 2, or 3")
		return
	}

	var f *feature.Feature

	switch req.Type {
	case "loose_idea":
		looseIntake := intake.NewLooseIdeaIntake(s.specProvider.BaseDir())
		if s.db != nil {
			looseIntake.SetDatabase(s.db)
		}
		var err error
		f, err = looseIntake.Submit(req.Title, req.Description, req.Priority, nil)
		if err != nil {
			log.Printf("error creating feature: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create feature")
			return
		}
	case "external_spec":
		if req.FileContent == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "file_content is required for external_spec type")
			return
		}
		extIntake := intake.NewExternalSpecIntake(s.specProvider.BaseDir())
		if s.db != nil {
			extIntake.SetDatabase(s.db)
		}
		result, err := extIntake.Submit(req.Title, req.FileContent, req.Priority, nil)
		if err != nil {
			log.Printf("error creating feature: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create feature")
			return
		}
		if len(result.Features) == 0 {
			writeError(w, http.StatusInternalServerError, "internal_error", "External spec intake produced no features")
			return
		}
		f = result.Features[0]
	default:
		writeError(w, http.StatusBadRequest, "validation_error", "Type must be 'loose_idea' or 'external_spec'")
		return
	}

	if s.db != nil {
		s.db.Exec(`INSERT OR IGNORE INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
			f.ID, f.Title, string(f.Current), string(f.Status), f.Priority, string(f.IntakePath), f.SpecDir, f.CreatedAt, f.UpdatedAt)
	}

	// Set AIDLC v2 scope/depth/test_strategy
	if req.Scope != "" {
		f.Scope = req.Scope
	} else {
		// Auto-detect scope from title + description
		detectedScope, _ := stage.DetectScope(req.Title + " " + req.Description)
		f.Scope = detectedScope
	}
	if req.Depth != "" {
		f.Depth = req.Depth
	} else {
		// Default depth from scope
		scopeInfo := stage.GetScopeInfo(f.Scope)
		if scopeInfo != nil {
			f.Depth = scopeInfo.DefaultDepth
		} else {
			f.Depth = stage.DepthStandard
		}
	}
	if req.TestStrategy != "" {
		f.TestStrategy = req.TestStrategy
	} else {
		scopeInfo := stage.GetScopeInfo(f.Scope)
		if scopeInfo != nil {
			f.TestStrategy = scopeInfo.DefaultTestStr
		} else {
			f.TestStrategy = stage.TestStrategyStandard
		}
	}
	s.pipeline.SaveFeature(f)
	if s.db != nil {
		s.db.InitFeatureStages(f.ID, f.Scope)
		s.db.RecordAuditEvent(f.ID, db.AuditWorkflowStart, "", "", fmt.Sprintf("scope=%s depth=%s test_strategy=%s", f.Scope, f.Depth, f.TestStrategy))
	}

	if req.StartImmediately {
		s.active.Store(f.ID, struct{}{})
	}

	writeJSON(w, http.StatusCreated, FeatureToDetailResponse(f, s.IsProcessing(f.ID), ""))

	if !req.StartImmediately {
		return
	}

	go s.runPhaseGoroutine(f.ID, f.Current)
}

func (s *Server) getFeature(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	f, err := s.pipeline.GetFeature(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f, s.IsProcessing(id), ""))
}

func (s *Server) runPhase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	f, err := s.pipeline.GetFeature(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	if f.IsTerminal() {
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Feature %s is in a terminal state (%s)", id, f.Status))
		return
	}

	if f.Status == feature.StatusWaitingFeedback {
		pending, _ := s.questionStore.PendingCount(r.Context(), id)
		if pending > 0 {
			writeError(w, http.StatusBadRequest, "validation_error", "Cannot run phase with pending questions")
			return
		}
		f.Status = feature.StatusInProgress
		s.pipeline.UpdateFeatureStatus(f)
	}

	if _, loaded := s.active.LoadOrStore(id, struct{}{}); loaded {
		writeError(w, http.StatusConflict, "already_processing", fmt.Sprintf("Feature %s is already being processed", id))
		return
	}

	// Prepare impl repos when entering construction
	if f.Current == feature.PhaseConstruction {
		if err := s.pipeline.PrepareImplRepos(f); err != nil {
			log.Printf("warning: could not prepare impl repos for %s: %v", f.ID, err)
		}
	}

	currentPhase := f.Current
	writeJSON(w, http.StatusAccepted, FeatureToDetailResponse(f, s.IsProcessing(id), ""))

	go s.runPhaseGoroutine(id, currentPhase)
}

func (s *Server) advanceFeature(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	f, err := s.pipeline.GetFeature(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	if f.IsTerminal() {
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Feature %s is in a terminal state (%s)", id, f.Status))
		return
	}

	if f.Status == feature.StatusWaitingFeedback {
		pending, _ := s.questionStore.PendingCount(r.Context(), id)
		if pending > 0 {
			writeError(w, http.StatusBadRequest, "validation_error", "Cannot advance feature with pending questions")
			return
		}
		f.Status = feature.StatusInProgress
	}

	if f.Current == feature.PhaseDelivery {
		gr, _ := s.pipeline.EvaluateGate(f)
		if gr != nil && gr.Passed {
			f.MarkDone()
			if err := s.pipeline.SaveFeature(f); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "Failed to save feature state")
				return
			}
			writeJSON(w, http.StatusOK, FeatureToDetailResponse(f, s.IsProcessing(id), ""))
			return
		}
		writeError(w, http.StatusBadRequest, "validation_error", "Feature is at the final phase (delivery) and the gate has not passed")
		return
	}

	gr, err := s.pipeline.EvaluateGate(f)
	if err != nil {
		log.Printf("error evaluating gate: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to evaluate gate")
		return
	}

	if gr == nil || !gr.Passed {
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Gate has not passed for phase %s", f.Current))
		return
	}

	f, err = s.pipeline.AdvanceFeature(f)
	if err != nil {
		log.Printf("error advancing feature: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to advance feature")
		return
	}

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f, s.IsProcessing(id), ""))
}

func (s *Server) recirculateFeature(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req RecirculateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}

	f, err := s.pipeline.GetFeature(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	if f.IsTerminal() {
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Feature %s is in a terminal state (%s)", id, f.Status))
		return
	}

	targetPhase := feature.ParsePhase(req.TargetPhase)
	if !feature.IsValidPhase(req.TargetPhase) {
		writeError(w, http.StatusBadRequest, "invalid_phase", fmt.Sprintf("Invalid phase %q. Valid phases: %v", req.TargetPhase, feature.ValidPhaseNames()))
		return
	}

	f, err = s.pipeline.RecirculateFeature(f, targetPhase, "recirculated via web UI")
	if err != nil {
		log.Printf("error recirculating feature: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to recirculate feature")
		return
	}

	if s.questionStore != nil {
		if err := s.questionStore.DeleteQuestionsForFeature(r.Context(), id); err != nil {
			log.Printf("warning: failed to delete questions for feature %s on recirculate: %v", id, err)
		}
	}

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f, s.IsProcessing(id), ""))
}

func (s *Server) cancelFeature(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	f, err := s.pipeline.GetFeature(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	if f.Status == feature.StatusCancelled {
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Feature %s is already cancelled", id))
		return
	}
	if f.Status == feature.StatusDone {
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Feature %s is already completed", id))
		return
	}

	// Kill any active tmux session
	if s.IsProcessing(id) {
		s.pipeline.Dispatcher().KillSession(id)
		s.active.Delete(id)
	}

	f.Cancel()
	if err := s.pipeline.SaveFeature(f); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to save feature state")
		return
	}

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f, s.IsProcessing(id), ""))
}

func (s *Server) evaluateGate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	f, err := s.pipeline.GetFeature(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	gr, err := s.pipeline.EvaluateGate(f)
	if err != nil {
		log.Printf("error evaluating gate: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to evaluate gate")
		return
	}

	writeJSON(w, http.StatusOK, GateResultToResponse(gr))
}

func (s *Server) getArtifact(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	artType := r.PathValue("type")
	if id == "" || artType == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID and artifact type are required")
		return
	}
	parsedType, ok := feature.ArtifactAPIPathToType(artType)
	if !ok {
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Unknown artifact type: %s", artType))
		return
	}
	dbKey := parsedType.String()

	if s.db == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Database not configured")
		return
	}

	artifact, err := s.db.GetArtifact(id, dbKey)
	if err != nil || artifact == nil {
		writeError(w, http.StatusNotFound, "artifact_not_found", fmt.Sprintf("Artifact %s not found for feature %s", artType, id))
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(artifact.Content))
}

// getCapturedOutput reads the agent log file for a feature.
// Returns the last N lines (default 200, configurable via ?lines= query param).
func (s *Server) getCapturedOutput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	f, err := s.pipeline.GetFeature(id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"is_processing": false, "output": ""})
		return
	}

	// Find the log file for the current phase
	logPath := filepath.Join(s.pipeline.WorktreeDir(f), "logs", string(f.Current)+"-"+s.pipeline.PrimaryRole(f.Current)+".log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"is_processing": s.IsProcessing(id),
			"output":        "",
		})
		return
	}

	lines := strings.Split(string(data), "\n")
	maxLines := 200
	if l := r.URL.Query().Get("lines"); l != "" {
		if n, err := strconvAtoi(l); err == nil && n > 0 {
			maxLines = n
		}
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"is_processing": s.IsProcessing(id),
		"output":        strings.Join(lines, "\n"),
	})
}

func (s *Server) streamFeature(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	if _, err := s.pipeline.GetFeature(id); err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan SSEMessage, 50)
	s.addSSEClient(id, ch)
	defer s.removeSSEClient(id, ch)

	// Replay buffered lifecycle events for late joiners
	s.sseMu.Lock()
	if buffer, ok := s.sseBuffers[id]; ok && len(buffer) > 0 {
		for _, msg := range buffer {
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", msg.EventType, msg.Data)
		}
	}
	s.sseMu.Unlock()

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	keepAlive := time.NewTicker(30 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", msg.EventType, msg.Data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-keepAlive.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}

type SSEMessage struct {
	EventType string
	Data      string
}

func (s *Server) addSSEClient(featureID string, ch chan SSEMessage) {
	s.sseMu.Lock()
	defer s.sseMu.Unlock()
	s.sseClients[featureID] = append(s.sseClients[featureID], ch)
}

func (s *Server) removeSSEClient(featureID string, ch chan SSEMessage) {
	s.sseMu.Lock()
	defer s.sseMu.Unlock()
	clients := s.sseClients[featureID]
	for i, c := range clients {
		if c == ch {
			s.sseClients[featureID] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
}

// broadcastSSE sends an event to all SSE clients for a feature.
// Lifecycle events (phase_change, gate_result, agent_dispatch, agent_complete,
// phase_complete, error) are buffered for late joiners. agent_output is NOT
// buffered — it's ephemeral and would bloat memory.
func (s *Server) broadcastSSE(featureID string, eventType string, data string) {
	// Buffer lifecycle events only
	switch eventType {
	case "phase_change", "gate_result", "agent_dispatch", "agent_complete", "phase_complete", "error", "interrupted":
		s.sseMu.Lock()
		buffer := s.sseBuffers[featureID]
		buffer = append(buffer, &SSEMessage{EventType: eventType, Data: data})
		if len(buffer) > 200 {
			buffer = buffer[len(buffer)-200:]
		}
		s.sseBuffers[featureID] = buffer
		s.sseMu.Unlock()
	}

	s.sseMu.Lock()
	clients := s.sseClients[featureID]
	s.sseMu.Unlock()

	msg := SSEMessage{EventType: eventType, Data: data}
	for _, ch := range clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, code int, errorCode string, details string) {
	writeJSON(w, code, ErrorResponse{Error: errorCode, Details: details})
}

// Question API handlers

func (s *Server) listQuestions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	if _, err := s.pipeline.GetFeature(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	questions, err := s.questionStore.ListQuestions(r.Context(), id)
	if err != nil {
		log.Printf("error listing questions for feature %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list questions")
		return
	}

	writeJSON(w, http.StatusOK, QuestionsToResponse(questions))
}

func (s *Server) listPendingQuestions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	if _, err := s.pipeline.GetFeature(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	questions, err := s.questionStore.ListPendingQuestions(r.Context(), id)
	if err != nil {
		log.Printf("error listing pending questions for feature %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list pending questions")
		return
	}

	writeJSON(w, http.StatusOK, QuestionsToResponse(questions))
}

func (s *Server) createQuestion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	if _, err := s.pipeline.GetFeature(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CreateQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Question) == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "question is required")
		return
	}
	if len(req.Question) > 2000 {
		writeError(w, http.StatusBadRequest, "validation_error", "question must be 1-2000 characters")
		return
	}
	if !feature.ValidQuestionPhases[req.Phase] {
		writeError(w, http.StatusBadRequest, "validation_error", "phase must be one of: inception, planning")
		return
	}
	if !feature.ValidQuestionRoles[req.Role] {
		writeError(w, http.StatusBadRequest, "validation_error", "role must be one of: pm, architect")
		return
	}
	if !feature.ValidQuestionTypes[req.Type] {
		writeError(w, http.StatusBadRequest, "validation_error", "type must be one of: clarification, decision, priority")
		return
	}
	if len(req.Options) > 10 {
		writeError(w, http.StatusBadRequest, "validation_error", "options must have at most 10 items")
		return
	}
	if req.Type == "multiple_choice" {
		hasOther := false
		for _, opt := range req.Options {
			if opt == "Other" {
				hasOther = true
			}
		}
		if !hasOther {
			writeError(w, http.StatusBadRequest, "validation_error", "multiple_choice questions must include 'Other' as the last option")
			return
		}
		if req.Options[len(req.Options)-1] != "Other" {
			writeError(w, http.StatusBadRequest, "validation_error", "'Other' must be the last option")
			return
		}
	}
	for _, opt := range req.Options {
		if len(opt) > 500 {
			writeError(w, http.StatusBadRequest, "validation_error", "each option must be 1-500 characters")
			return
		}
	}

	q := feature.Question{
		FeatureID: id,
		Phase:     req.Phase,
		Role:      req.Role,
		Question:  req.Question,
		Type:      req.Type,
		Options:   req.Options,
	}
	if q.Options == nil {
		q.Options = []string{}
	}

	created, err := s.questionStore.CreateQuestion(r.Context(), id, q)
	if err != nil {
		log.Printf("error creating question for feature %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create question")
		return
	}

	writeJSON(w, http.StatusCreated, QuestionToResponse(created))
}

func (s *Server) answerQuestion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	questionId := r.PathValue("questionId")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}
	if questionId == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Question ID is required")
		return
	}

	if _, err := s.pipeline.GetFeature(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req AnswerQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}

	answer := strings.TrimSpace(req.Answer)
	if answer == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "answer must be 1-5000 characters")
		return
	}
	if len(req.Answer) > 5000 {
		writeError(w, http.StatusBadRequest, "validation_error", "answer must be 1-5000 characters")
		return
	}

	updated, err := s.questionStore.AnswerQuestion(r.Context(), id, questionId, answer)
	if err != nil {
		if _, ok := err.(*feature.QuestionConflictError); ok {
			writeError(w, http.StatusConflict, "conflict", err.Error())
			return
		}
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Question %s not found", questionId))
			return
		}
		log.Printf("error answering question %s for feature %s: %v", questionId, id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to answer question")
		return
	}

	writeJSON(w, http.StatusOK, QuestionToResponse(updated))

	answerData, _ := json.Marshal(map[string]string{
		"feature_id":  id,
		"question_id": questionId,
		"status":      "answered",
	})
	s.broadcastSSE(id, "question_answered", string(answerData))

	// Check if all questions answered — if so, resume the feature
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC in auto-resume goroutine for %s: %v", id, r)
			}
		}()
		pending, err := s.questionStore.PendingCount(context.Background(), id)
		if err != nil {
			log.Printf("error checking pending count for feature %s: %v", id, err)
			return
		}
		if pending > 0 {
			return
		}

		f, err := s.pipeline.GetFeature(id)
		if err != nil {
			log.Printf("error reloading feature %s after question answered: %v", id, err)
			return
		}

		if f.Status != feature.StatusWaitingFeedback {
			return
		}

		// Transition back to in_progress so user can advance
		f.Status = feature.StatusInProgress
		if err := s.pipeline.UpdateFeatureStatus(f); err != nil {
			log.Printf("warning: could not transition feature %s back to in_progress: %v", id, err)
		}
		log.Printf("all questions answered for feature %s — waiting for user to advance", id)
	}()
}

// strconvAtoi is a helper to avoid importing strconv at the top level.
func strconvAtoi(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid integer")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}