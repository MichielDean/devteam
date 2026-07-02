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

func NewServer(addr string, specProvider *spec.SpecProvider, pipe *pipeline.Pipeline, staticFS fs.FS, questionStore feature.QuestionStore, database *db.DB) *Server {
	s := &Server{
		specProvider:  specProvider,
		pipeline:     pipe,
		baseDir:       specProvider.BaseDir(),
		staticFS:      staticFS,
		questionStore: questionStore,
		db:            database,
		sseClients:    make(map[string][]chan SSEMessage),
		sseBuffers:    make(map[string][]*SSEMessage),
	}

	// Register the server as the SSE broadcaster for pipeline events
	pipeline.SetSSEBroadcaster(s)

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

	// Tmux session management endpoints
	s.registerSessionRoutes(mux)

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
			f.ID, f.Title, f.CurrentPhaseLegacy(), string(f.Status), f.Priority, string(f.IntakePath), f.SpecDir, f.CreatedAt, f.UpdatedAt)
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
	logPath := filepath.Join(s.pipeline.WorktreeDir(f), "logs", f.CurrentPhaseLegacy()+".log")
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

// BroadcastSSE sends an event to all SSE clients for a feature.
// Implements the pipeline.SSEBroadcaster interface.
func (s *Server) BroadcastSSE(featureID string, eventType string, data string) {
	s.broadcastSSE(featureID, eventType, data)
}

// broadcastSSE sends an event to all SSE clients for a feature.
// Lifecycle events are buffered for late joiners. agent_output is NOT
// buffered — it's ephemeral and would bloat memory.
func (s *Server) broadcastSSE(featureID string, eventType string, data string) {
	// Buffer lifecycle events only
	switch eventType {
	case "phase_change", "gate_result", "agent_dispatch", "agent_complete", "phase_complete", "error", "interrupted",
		"stage_started", "stage_awaiting_approval", "stage_revising", "stage_completed", "gate_approved", "gate_rejected",
		"processing_complete", "session_state_change":
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
		writeError(w, http.StatusBadRequest, "validation_error", "phase must be one of: ideation, inception")
		return
	}
	if !feature.ValidQuestionRoles[req.Role] {
		writeError(w, http.StatusBadRequest, "validation_error", "role must be one of: product, architect, design, delivery, developer, platform, devsecops, quality, pipeline-deploy, operations")
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