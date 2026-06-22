package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/intake"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/spec"
)

type Server struct {
	httpServer    *http.Server
	pipeline      *pipeline.Pipeline
	specProvider  *spec.SpecProvider
	activeProcess sync.Map
	sseClients    sync.Map
	sseBuffers    sync.Map // featureID -> []*SSEMessage (recent events for late joiners)
	baseDir       string
	staticFS      fs.FS
	questionStore feature.QuestionStore
}

func (s *Server) IsProcessing(id string) bool {
	_, loaded := s.activeProcess.Load(id)
	return loaded
}

func (s *Server) ProcessingMode(id string) string {
	v, loaded := s.activeProcess.Load(id)
	if !loaded {
		return ""
	}
	mode, _ := v.(string)
	return mode
}

// RestoreActiveProcesses scans tmux for active devteam sessions and restores
// the activeProcess map. Also detects orphaned features (status: in_progress
// but no tmux session) and resumes their pipeline. Called on server startup.
func (s *Server) RestoreActiveProcesses() {
	// 1. Restore active tmux sessions
	sessions := s.pipeline.Dispatcher().ListActiveSessions()
	for featureID := range sessions {
		s.activeProcess.Store(featureID, "autopilot")
		log.Printf("restored active process for feature %s from tmux session", featureID)
	}

	// 2. Find and resume orphaned features (in_progress but no tmux session)
	s.resumeOrphanedFeatures()
}

func (s *Server) resumeOrphanedFeatures() {
	features, err := s.pipeline.ListFeatures()
	if err != nil {
		log.Printf("resumeOrphanedFeatures: failed to list features: %v", err)
		return
	}

	for _, f := range features {
		if f.IsTerminal() {
			continue
		}
		if f.Status != "in_progress" {
			continue
		}

		// Skip if tmux session is still alive
		if s.pipeline.Dispatcher().IsSessionAlive(f.ID) {
			continue
		}

		// Skip if already in activeProcess
		if s.IsProcessing(f.ID) {
			continue
		}

		// Don't re-run a phase whose gate already passed — the user just
		// hasn't advanced to the next phase yet. Marking as not processing
		// lets the UI show "Go to Planning" etc.
		currentPhaseState, ok := f.PhaseStates[f.Current]
		if ok && currentPhaseState.GateResult != nil && currentPhaseState.GateResult.Passed {
			log.Printf("resumeOrphanedFeatures: feature %s phase %s gate already passed — not resuming, waiting for user to advance", f.ID, f.Current)
			continue
		}

		log.Printf("resumeOrphanedFeatures: found orphaned feature %s (phase %s, status %s) — resuming current phase", f.ID, f.Current, f.Status)

		// Resume in single-phase mode — run current phase only, then stop.
		// Don't auto-advance; user advances manually through the UI.
		s.activeProcess.Store(f.ID, "single-phase")
		go func(featureID string) {
			defer s.activeProcess.Delete(featureID)

			f, err := s.pipeline.GetFeature(featureID)
			if err != nil {
				log.Printf("resumeOrphanedFeatures: failed to reload feature %s: %v", featureID, err)
				return
			}

			ctx := context.Background()
			currentPhase := f.Current

			s.broadcastSSE(featureID, "phase_change", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","status":"in_progress"}`, featureID, currentPhase))
			s.broadcastSSE(featureID, "agent_dispatch", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","role":"%s","status":"dispatched","message":"Resuming %s"}`, featureID, currentPhase, s.pipeline.PrimaryRole(currentPhase), currentPhase))

			onOutput := func(line string, isStderr bool) {
				escaped, _ := json.Marshal(line)
				s.broadcastSSE(featureID, "agent_output", fmt.Sprintf(`{"feature_id":"%s","line":%s,"stderr":%v}`, featureID, string(escaped), isStderr))
			}

			result, err := s.pipeline.RunPhaseWithAgentStreaming(ctx, f, onOutput)
			if err != nil {
				log.Printf("resumeOrphanedFeatures: error resuming feature %s: %v", featureID, err)
				s.broadcastSSE(featureID, "error", fmt.Sprintf(`{"feature_id":"%s","message":"Resume failed: %s"}`, featureID, err.Error()))
				return
			}

			if result != nil && result.GateResult != nil {
				checks := make([]map[string]interface{}, 0, len(result.GateResult.Checks))
				for _, c := range result.GateResult.Checks {
					checks = append(checks, map[string]interface{}{
						"name":    c.Name,
						"passed":  c.Passed,
						"message": c.Message,
					})
				}
				checksJSON, _ := json.Marshal(checks)
				s.broadcastSSE(featureID, "gate_result", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","passed":%v,"checks":%s}`, featureID, currentPhase, result.GateResult.Passed, string(checksJSON)))
			}

			s.broadcastSSE(featureID, "phase_complete", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","status":"complete"}`, featureID, currentPhase))
			log.Printf("resumeOrphanedFeatures: feature %s phase %s completed", featureID, currentPhase)
		}(f.ID)
	}
}

// NewServer creates a new API server.

func NewServer(addr string, specProvider *spec.SpecProvider, pipeline *pipeline.Pipeline, staticFS fs.FS, questionStore feature.QuestionStore) *Server {
	s := &Server{
		specProvider:  specProvider,
		pipeline:      pipeline,
		baseDir:       specProvider.BaseDir(),
		staticFS:      staticFS,
		questionStore: questionStore,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/features", s.listFeatures)
	mux.HandleFunc("POST /api/features", s.createFeature)
	mux.HandleFunc("GET /api/features/{id}", s.getFeature)
	mux.HandleFunc("POST /api/features/{id}/run", s.runPhase)
	mux.HandleFunc("POST /api/features/{id}/advance", s.advanceFeature)
	mux.HandleFunc("POST /api/features/{id}/recirculate", s.recirculateFeature)
	mux.HandleFunc("POST /api/features/{id}/cancel", s.cancelFeature)
	mux.HandleFunc("POST /api/features/{id}/process", s.processFeature)
	mux.HandleFunc("GET /api/features/{id}/gate", s.evaluateGate)
	mux.HandleFunc("GET /api/features/{id}/artifacts/{type}", s.getArtifact)
	mux.HandleFunc("GET /api/features/{id}/stream", s.streamFeature)
	mux.HandleFunc("GET /api/features/{id}/output", s.getCapturedOutput)

	// Question endpoints
	mux.HandleFunc("GET /api/features/{id}/questions/pending", s.listPendingQuestions)
	mux.HandleFunc("GET /api/features/{id}/questions", s.listQuestions)
	mux.HandleFunc("POST /api/features/{id}/questions", s.createQuestion)
	mux.HandleFunc("PATCH /api/features/{id}/questions/{questionId}", s.answerQuestion)

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

	// Set activeProcess before responding so the UI knows it's processing (only if starting)
	if req.StartImmediately {
		s.activeProcess.Store(f.ID, "single-phase")
	}

	writeJSON(w, http.StatusCreated, FeatureToDetailResponse(f, s.IsProcessing(f.ID), s.ProcessingMode(f.ID)))

	// Only auto-start inception if requested. Otherwise, just create the feature
	// and let the user start it manually from the feature page.
	if !req.StartImmediately {
		return
	}

	// Auto-start inception phase only (not full autopilot).
	// UI-created features should be interactive: run one phase, ask questions,
	// wait for user to review and advance manually.
	go func() {
		defer s.activeProcess.Delete(f.ID)
		ctx := context.Background()
		currentPhase := f.Current

		// Immediate feedback: work has started
		s.broadcastSSE(f.ID, "phase_change", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","status":"in_progress"}`, f.ID, currentPhase))
		s.broadcastSSE(f.ID, "agent_dispatch", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","role":"%s","status":"dispatched","message":"Starting inception — this may take a few minutes"}`, f.ID, currentPhase, s.pipeline.PrimaryRole(currentPhase)))

		onOutput := func(line string, isStderr bool) {
			escaped, _ := json.Marshal(line)
			s.broadcastSSE(f.ID, "agent_output", fmt.Sprintf(`{"feature_id":"%s","line":%s,"stderr":%v}`, f.ID, string(escaped), isStderr))
		}

		result, err := s.pipeline.RunPhaseWithAgentStreaming(ctx, f, onOutput)
		if err != nil {
			log.Printf("error running inception for feature %s: %v", f.ID, err)
			s.broadcastSSE(f.ID, "error", fmt.Sprintf(`{"feature_id":"%s","message":"Inception failed: %s"}`, f.ID, err.Error()))
			return
		}

		// Broadcast gate result
		if result != nil && result.GateResult != nil {
			checks := make([]map[string]interface{}, 0, len(result.GateResult.Checks))
			for _, c := range result.GateResult.Checks {
				checks = append(checks, map[string]interface{}{
					"name":    c.Name,
					"passed":  c.Passed,
					"message": c.Message,
				})
			}
			checksJSON, _ := json.Marshal(checks)
			s.broadcastSSE(f.ID, "gate_result", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","passed":%v,"checks":%s}`, f.ID, currentPhase, result.GateResult.Passed, string(checksJSON)))
		}

		s.broadcastSSE(f.ID, "phase_complete", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","status":"complete"}`, f.ID, currentPhase))
		log.Printf("inception completed for feature %s, gate passed: %v", f.ID, result != nil && result.GateResult != nil && result.GateResult.Passed)
	}()
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

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f, s.IsProcessing(f.ID), s.ProcessingMode(f.ID)))
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

	if _, loaded := s.activeProcess.LoadOrStore(id, "single-phase"); loaded {
		writeError(w, http.StatusConflict, "already_processing", fmt.Sprintf("Feature %s is already being processed", id))
		return
	}

	writeJSON(w, http.StatusAccepted, FeatureToDetailResponse(f, s.IsProcessing(f.ID), s.ProcessingMode(f.ID)))

	go func() {
		defer s.activeProcess.Delete(id)
		ctx := context.Background()
		currentPhase := f.Current
		log.Printf("runPhase goroutine started for feature %s, phase %s", id, currentPhase)

		s.broadcastSSE(id, "phase_change", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","status":"in_progress"}`, id, currentPhase))
		s.broadcastSSE(id, "agent_dispatch", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","role":"%s","status":"dispatched"}`, id, currentPhase, s.pipeline.PrimaryRole(currentPhase)))

		onOutput := func(line string, isStderr bool) {
			escaped, _ := json.Marshal(line)
			s.broadcastSSE(id, "agent_output", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","line":%s,"stderr":%v}`, id, currentPhase, string(escaped), isStderr))
		}

		result, err := s.pipeline.RunPhaseWithAgentStreaming(ctx, f, onOutput)
		if err != nil {
			log.Printf("error running phase for feature %s: %v", id, err)
			s.broadcastSSE(id, "error", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","message":"Phase execution failed: %s"}`, id, currentPhase, err.Error()))
			return
		}
		log.Printf("phase %s completed for feature %s in %v", currentPhase, id, result.Duration)

		s.broadcastSSE(id, "agent_complete", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","role":"%s","status":"success","duration_ms":%d}`, id, currentPhase, s.pipeline.PrimaryRole(currentPhase), result.Duration.Milliseconds()))

		f, err = s.pipeline.GetFeature(id)
		if err != nil {
			log.Printf("error reloading feature %s after phase run: %v", id, err)
			s.broadcastSSE(id, "error", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","message":"Failed to reload feature state"}`, id, currentPhase))
			return
		}

		if result != nil && result.GateResult != nil {
			checks := make([]map[string]interface{}, 0, len(result.GateResult.Checks))
			for _, c := range result.GateResult.Checks {
				checks = append(checks, map[string]interface{}{
					"name":    c.Name,
					"passed":  c.Passed,
					"message": c.Message,
				})
			}
			checksJSON, _ := json.Marshal(checks)
			s.broadcastSSE(id, "gate_result", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","passed":%v,"checks":%s}`, id, currentPhase, result.GateResult.Passed, string(checksJSON)))
		}

		s.broadcastSSE(id, "phase_complete", fmt.Sprintf(`{"feature_id":"%s","phase":"%s","status":"complete"}`, id, currentPhase))
	}()
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

	if f.Status == feature.StatusWaitingHuman {
		writeError(w, http.StatusBadRequest, "validation_error", "Cannot advance feature in waiting_for_human status")
		return
	}

	if f.Current == feature.PhaseDelivery {
		gr, _ := s.pipeline.EvaluateGate(f)
		if gr != nil && gr.Passed {
			f.MarkDone()
			if err := s.pipeline.SaveFeature(f); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "Failed to save feature state")
				return
			}
			writeJSON(w, http.StatusOK, FeatureToDetailResponse(f, s.IsProcessing(f.ID), s.ProcessingMode(f.ID)))
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

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f, s.IsProcessing(f.ID), s.ProcessingMode(f.ID)))
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

	// Clear questions when recirculating
	if s.questionStore != nil {
		if err := s.questionStore.DeleteQuestionsForFeature(r.Context(), id); err != nil {
			log.Printf("warning: failed to delete questions for feature %s on recirculate: %v", id, err)
		}
	}

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f, s.IsProcessing(f.ID), s.ProcessingMode(f.ID)))
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

	f.Cancel()
	if err := s.pipeline.SaveFeature(f); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to save feature state")
		return
	}

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f, s.IsProcessing(f.ID), s.ProcessingMode(f.ID)))
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

func (s *Server) processFeature(w http.ResponseWriter, r *http.Request) {
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

	if _, loaded := s.activeProcess.LoadOrStore(id, "autopilot"); loaded {
		writeError(w, http.StatusConflict, "already_processing", fmt.Sprintf("Feature %s is already being processed", id))
		return
	}

	if f.IsTerminal() {
		s.activeProcess.Delete(id)
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Feature %s is in a terminal state (%s)", id, f.Status))
		return
	}

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f, s.IsProcessing(f.ID), s.ProcessingMode(f.ID)))

	go func() {
		defer s.activeProcess.Delete(id)
		ctx := context.Background()
		eventCh := make(chan pipeline.ProcessEvent, 100)

		onOutput := func(line string, isStderr bool) {
			escaped, _ := json.Marshal(line)
			s.broadcastSSE(id, "agent_output", fmt.Sprintf(`{"feature_id":"%s","line":%s,"stderr":%v}`, id, string(escaped), isStderr))
		}

		done := make(chan error, 1)
		go func() {
			done <- s.pipeline.ProcessAsync(ctx, f, eventCh, onOutput)
			close(eventCh)
		}()

		for evt := range eventCh {
			data, _ := json.Marshal(evt)
			s.broadcastSSE(id, string(evt.Type), string(data))
		}

		if err := <-done; err != nil {
			log.Printf("error processing feature %s: %v", id, err)
			errData, _ := json.Marshal(map[string]string{"message": "Processing failed"})
			s.broadcastSSE(id, "error", string(errData))
		}
	}()
}

func (s *Server) getArtifact(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	artType := r.PathValue("type")
	if id == "" || artType == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID and artifact type are required")
		return
	}

	if _, err := s.pipeline.GetFeature(id); err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	featureType, ok := feature.ArtifactAPIPathToType(artType)
	if !ok {
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Invalid artifact type %q", artType))
		return
	}

	if !s.specProvider.ArtifactExists(id, featureType) {
		writeError(w, http.StatusNotFound, "artifact_not_found", fmt.Sprintf("Artifact %s not found for feature %s", artType, id))
		return
	}

	content, err := s.specProvider.ReadArtifactContent(id, featureType)
	if err != nil {
		writeError(w, http.StatusNotFound, "artifact_not_found", fmt.Sprintf("Artifact %s not found for feature %s", artType, id))
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(content))
}

func (s *Server) getCapturedOutput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	if !s.IsProcessing(id) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"is_processing": false,
			"output":        "",
		})
		return
	}

	output, err := s.pipeline.Dispatcher().CaptureOutput(id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"is_processing": true,
			"output":        "",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"is_processing": true,
		"output":        output,
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

	// Replay buffered events for late joiners (page refresh, etc.)
	if buf, ok := s.sseBuffers.Load(id); ok {
		if buffer, ok := buf.(*[]*SSEMessage); ok && len(*buffer) > 0 {
			for _, msg := range *buffer {
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", msg.EventType, msg.Data)
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}

	// Clear buffer when processing is done
	if !s.IsProcessing(id) {
		s.sseBuffers.Delete(id)
	}

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
	key := featureID
	clients, _ := s.sseClients.LoadOrStore(key, &sync.Mutex{})
	var mu *sync.Mutex
	mu = clients.(*sync.Mutex)
	mu.Lock()
	channels, _ := s.sseClients.Load(featureID + ":channels")
	var channelsList []chan SSEMessage
	if channelsList, ok := channels.([]chan SSEMessage); ok {
		channelsList = append(channelsList, ch)
	} else {
		channelsList = []chan SSEMessage{ch}
	}
	s.sseClients.Store(featureID+":channels", channelsList)
	mu.Unlock()
}

func (s *Server) removeSSEClient(featureID string, ch chan SSEMessage) {
	key := featureID
	clients, _ := s.sseClients.LoadOrStore(key, &sync.Mutex{})
	mu := clients.(*sync.Mutex)
	mu.Lock()
	channels, _ := s.sseClients.Load(featureID + ":channels")
	if channelsList, ok := channels.([]chan SSEMessage); ok {
		for i, c := range channelsList {
			if c == ch {
				channelsList = append(channelsList[:i], channelsList[i+1:]...)
				break
			}
		}
		s.sseClients.Store(featureID+":channels", channelsList)
	}
	mu.Unlock()
}

func (s *Server) broadcastSSE(featureID string, eventType string, data string) {
	// Buffer the event for late joiners
	buf, _ := s.sseBuffers.LoadOrStore(featureID, &[]*SSEMessage{})
	buffer := buf.(*[]*SSEMessage)
	*buffer = append(*buffer, &SSEMessage{EventType: eventType, Data: data})
	// Keep last 200 events
	if len(*buffer) > 200 {
		*buffer = (*buffer)[len(*buffer)-200:]
	}

	channels, _ := s.sseClients.Load(featureID + ":channels")
	if channelsList, ok := channels.([]chan SSEMessage); ok {
		msg := SSEMessage{EventType: eventType, Data: data}
		for _, ch := range channelsList {
			select {
			case ch <- msg:
			default:
			}
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

	// Validate required fields
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

	// Broadcast SSE event for the answer
	answerData, _ := json.Marshal(map[string]string{
		"feature_id": id,
		"question_id": questionId,
		"status":      "answered",
	})
	s.broadcastSSE(id, "question_answered", string(answerData))

	// Check if all questions are answered — if so, auto-resume the pipeline
	go func() {
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

		if f.Status != feature.StatusWaitingHuman {
			return
		}

		if _, loaded := s.activeProcess.LoadOrStore(id, "autopilot"); loaded {
			return
		}

		log.Printf("all questions answered for feature %s, auto-resuming pipeline", id)
		ctx := context.Background()
		eventCh := make(chan pipeline.ProcessEvent, 100)

		onOutput := func(line string, isStderr bool) {
			escaped, _ := json.Marshal(line)
			s.broadcastSSE(id, "agent_output", fmt.Sprintf(`{"feature_id":"%s","line":%s,"stderr":%v}`, id, string(escaped), isStderr))
		}

		done := make(chan error, 1)
		go func() {
			done <- s.pipeline.ProcessAsync(ctx, f, eventCh, onOutput)
			close(eventCh)
		}()

		for evt := range eventCh {
			data, _ := json.Marshal(evt)
			s.broadcastSSE(id, string(evt.Type), string(data))
		}

		if err := <-done; err != nil {
			log.Printf("error resuming feature %s: %v", id, err)
			errData, _ := json.Marshal(map[string]string{"message": "Pipeline resume failed"})
			s.broadcastSSE(id, "error", string(errData))
		}
		s.activeProcess.Delete(id)
	}()
}
