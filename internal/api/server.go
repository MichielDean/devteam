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

// Server is the HTTP server for the Dev Team Web UI
type Server struct {
	httpServer     *http.Server
	specProvider   *spec.SpecProvider
	pipeline       *pipeline.Pipeline
	mux            *http.ServeMux
	activeProcess  sync.Map // featureID -> bool: tracks features being processed
	sseRegistry    *SSERegistry
	fileWatcher    *FileWatcher
	baseDir        string
}

// SSERegistry manages SSE client channels per feature
type SSERegistry struct {
	mu      sync.RWMutex
	clients map[string][]chan SSEMessage
}

// SSEMessage represents an SSE message to send to clients
type SSEMessage struct {
	EventType string // event: field
	Data      string // data: field (JSON string)
}

// NewSSERegistry creates a new SSE registry
func NewSSERegistry() *SSERegistry {
	return &SSERegistry{
		clients: make(map[string][]chan SSEMessage),
	}
}

// Register adds a new SSE client channel for a feature
func (r *SSERegistry) Register(featureID string, ch chan SSEMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[featureID] = append(r.clients[featureID], ch)
}

// Unregister removes an SSE client channel for a feature
func (r *SSERegistry) Unregister(featureID string, ch chan SSEMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	clients := r.clients[featureID]
	for i, c := range clients {
		if c == ch {
			r.clients[featureID] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
	if len(r.clients[featureID]) == 0 {
		delete(r.clients, featureID)
	}
}

// Broadcast sends an SSE message to all clients for a feature
func (r *SSERegistry) Broadcast(featureID string, msg SSEMessage) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	clients := r.clients[featureID]
	for _, ch := range clients {
		select {
		case ch <- msg:
		default:
			// Channel is full, skip this client (they'll get the next update)
			log.Printf("SSE client channel full for feature %s, skipping message", featureID)
		}
	}
}

// NewServer creates a new Server with the given configuration
func NewServer(addr string, specProvider *spec.SpecProvider, pipeline *pipeline.Pipeline, staticFS fs.FS) *Server {
	s := &Server{
		specProvider: specProvider,
		pipeline:    pipeline,
		sseRegistry: NewSSERegistry(),
		baseDir:     specProvider.BaseDir(),
	}

	// Start file watcher for CLI-triggered state changes
	s.fileWatcher = NewFileWatcher(specProvider.BaseDir(), s.sseRegistry)
	go s.fileWatcher.Start()

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/features", s.listFeatures)
	mux.HandleFunc("POST /api/features", s.createFeature)
	mux.HandleFunc("GET /api/features/{id}", s.getFeature)
	mux.HandleFunc("POST /api/features/{id}/run", s.runPhase)
	mux.HandleFunc("POST /api/features/{id}/advance", s.advanceFeature)
	mux.HandleFunc("POST /api/features/{id}/recirculate", s.recirculateFeature)
	mux.HandleFunc("POST /api/features/{id}/cancel", s.cancelFeature)
	mux.HandleFunc("POST /api/features/{id}/process", s.processFeature)
	mux.HandleFunc("GET /api/features/{id}/artifacts/{type}", s.getArtifact)
	mux.HandleFunc("GET /api/features/{id}/gate", s.evaluateGate)
	mux.HandleFunc("GET /api/features/{id}/stream", s.streamFeature)

	// Serve static files (SPA)
	if staticFS != nil {
		spaHandler := s.spaHandler(staticFS)
		mux.Handle("/", spaHandler)
	}

	// Wrap with middleware
	handler := corsMiddleware(s.mux)
	handler = securityHeadersMiddleware(handler)
	handler = loggingMiddleware(handler)
	handler = recoveryMiddleware(handler)

	s.mux = mux
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("Starting Dev Team Web UI server on %s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// spaHandler serves the SPA static files, falling back to index.html for client-side routes
func (s *Server) spaHandler(staticFS fs.FS) http.HandlerFunc {
	// Create a sub-filesystem if needed
	fileServer := http.FileServer(http.FS(staticFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Don't serve index.html for API routes
		if strings.HasPrefix(path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Try to serve the static file
		f, err := staticFS.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fall back to index.html for client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

// writeJSON writes a JSON response with the given status code
func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("error encoding JSON response: %v", err)
	}
}

// writeError writes an error JSON response
func writeError(w http.ResponseWriter, code int, errorCode string, details string) {
	writeJSON(w, code, ErrorResponse{
		Error:   errorCode,
		Details: details,
	})
}

// getFeatureID extracts the feature ID from the URL path
func getFeatureID(r *http.Request) string {
	return r.PathValue("id")
}

// setActive marks a feature as being actively processed
func (s *Server) setActive(featureID string) {
	s.activeProcess.Store(featureID, true)
}

// clearActive removes a feature from active processing
func (s *Server) clearActive(featureID string) {
	s.activeProcess.Delete(featureID)
}

// isActive checks if a feature is currently being processed
func (s *Server) isActive(featureID string) bool {
	_, ok := s.activeProcess.Load(featureID)
	return ok
}

// broadcastEvent sends an SSE event to all clients watching a feature
func (s *Server) broadcastEvent(featureID string, eventType string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("error marshaling SSE event data: %v", err)
		return
	}
	s.sseRegistry.Broadcast(featureID, SSEMessage{
		EventType: eventType,
		Data:      string(jsonData),
	})
}

// listFeatures handles GET /api/features
func (s *Server) listFeatures(w http.ResponseWriter, r *http.Request) {
	features, err := s.pipeline.ListFeatures()
	if err != nil {
		log.Printf("error listing features: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list features")
		return
	}

	writeJSON(w, http.StatusOK, FeaturesToSummaryResponse(features))
}

// getFeature handles GET /api/features/:id
func (s *Server) getFeature(w http.ResponseWriter, r *http.Request) {
	id := getFeatureID(r)
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	f, err := s.pipeline.GetFeature(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f))
}

// maxRequestBodyBytes limits request body size to 1MB to prevent abuse
const maxRequestBodyBytes = 1 << 20 // 1MB

// createFeature handles POST /api/features
func (s *Server) createFeature(w http.ResponseWriter, r *http.Request) {
	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req CreateFeatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}

	// Validate title
	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "empty_title", "Title is required")
		return
	}
	if len(req.Title) > 200 {
		writeError(w, http.StatusBadRequest, "title_too_long", "Title must be 200 characters or less")
		return
	}

	// Validate description
	if strings.TrimSpace(req.Description) == "" {
		writeError(w, http.StatusBadRequest, "empty_description", "Description is required")
		return
	}
	if len(req.Description) > 10000 {
		writeError(w, http.StatusBadRequest, "description_too_long", "Description must be 10000 characters or less")
		return
	}

	// Validate priority (default to 2)
	if req.Priority == 0 {
		req.Priority = 2
	}
	if req.Priority < 1 || req.Priority > 3 {
		writeError(w, http.StatusBadRequest, "invalid_priority", "Priority must be 1, 2, or 3")
		return
	}

	// Validate type
	if req.Type != "loose_idea" && req.Type != "external_spec" {
		writeError(w, http.StatusBadRequest, "validation_error", "Type must be 'loose_idea' or 'external_spec'")
		return
	}

	// Check for duplicate title
	existing, _ := s.pipeline.ListFeatures()
	for _, f := range existing {
		if strings.EqualFold(f.Title, req.Title) {
			writeError(w, http.StatusConflict, "duplicate_title", fmt.Sprintf("A feature with title %q already exists", req.Title))
			return
		}
	}

	var f *feature.Feature

	switch req.Type {
	case "loose_idea":
		looseIntake := intake.NewLooseIdeaIntake(s.specProvider.BaseDir())
		var err error
		f, err = looseIntake.Submit(req.Title, req.Description, req.Priority, nil)
		if err != nil {
			log.Printf("error creating feature via loose idea intake: %v", err)
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
			log.Printf("error creating feature via external spec intake: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create feature")
			return
		}
		if len(result.Features) == 0 {
			writeError(w, http.StatusInternalServerError, "internal_error", "External spec intake produced no features")
			return
		}
		f = result.Features[0]
	}

	writeJSON(w, http.StatusCreated, FeatureToDetailResponse(f))
}

// runPhase handles POST /api/features/:id/run
func (s *Server) runPhase(w http.ResponseWriter, r *http.Request) {
	id := getFeatureID(r)
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	f, err := s.pipeline.GetFeature(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	if s.isActive(id) {
		writeError(w, http.StatusConflict, "already_processing", fmt.Sprintf("Feature %s is already being processed", id))
		return
	}

	if f.IsTerminal() {
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Feature %s is in a terminal state (%s)", id, f.Status))
		return
	}

	ctx := r.Context()
	result, err := s.pipeline.RunPhaseWithAgent(ctx, f)
	if err != nil {
		log.Printf("error running phase for feature %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("Failed to run phase for feature %s", id))
		return
	}

	// Reload feature state
	f, err = s.pipeline.GetFeature(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to reload feature state")
		return
	}

	resp := FeatureToDetailResponse(f)
	if result != nil && result.GateResult != nil {
		phaseKey := string(f.Current)
		if ps, ok := resp.PhaseStates[phaseKey]; ok {
			gr := GateResultToResponse(result.GateResult)
			ps.GateResult = &gr
			resp.PhaseStates[phaseKey] = ps
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// advanceFeature handles POST /api/features/:id/advance
func (s *Server) advanceFeature(w http.ResponseWriter, r *http.Request) {
	id := getFeatureID(r)
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

	// At delivery phase with passed gate means done
	if f.Current == feature.PhaseDelivery {
		// Check if gate passed
		gr, _ := s.pipeline.EvaluateGate(f)
		if gr != nil && gr.Passed {
			// Mark as done
			f.MarkDone()
			if err := s.pipeline.SaveFeature(f); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "Failed to save feature state")
				return
			}
			writeJSON(w, http.StatusOK, FeatureToDetailResponse(f))
			return
		}
		writeError(w, http.StatusBadRequest, "validation_error", "Feature is at the final phase (delivery) and the gate has not passed")
		return
	}

	// Evaluate gate before advancing
	gr, err := s.pipeline.EvaluateGate(f)
	if err != nil {
		log.Printf("error evaluating gate for feature %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to evaluate gate")
		return
	}

	if gr == nil || !gr.Passed {
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Gate has not passed for phase %s", f.Current))
		return
	}

	f, err = s.pipeline.AdvanceFeature(f)
	if err != nil {
		log.Printf("error advancing feature %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to advance feature")
		return
	}

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f))
}

// recirculateFeature handles POST /api/features/:id/recirculate
func (s *Server) recirculateFeature(w http.ResponseWriter, r *http.Request) {
	id := getFeatureID(r)
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

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

	// Validate target phase
	targetPhase := feature.ParsePhase(req.TargetPhase)
	if !feature.IsValidPhase(req.TargetPhase) {
		writeError(w, http.StatusBadRequest, "invalid_phase", fmt.Sprintf("Invalid phase %q. Valid phases: %v", req.TargetPhase, feature.ValidPhaseNames()))
		return
	}

	// Target phase must be earlier than current phase
	currentIdx := -1
	targetIdx := -1
	for i, p := range feature.AllPhases() {
		if p == f.Current {
			currentIdx = i
		}
		if p == targetPhase {
			targetIdx = i
		}
	}
	if targetIdx >= currentIdx {
		writeError(w, http.StatusBadRequest, "invalid_phase", fmt.Sprintf("Target phase %q must be earlier than current phase %q", req.TargetPhase, f.Current))
		return
	}

	f, err = s.pipeline.RecirculateFeature(f, targetPhase, "recirculated via web UI")
	if err != nil {
		log.Printf("error recirculating feature %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to recirculate feature")
		return
	}

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f))
}

// cancelFeature handles POST /api/features/:id/cancel
func (s *Server) cancelFeature(w http.ResponseWriter, r *http.Request) {
	id := getFeatureID(r)
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

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f))
}

// evaluateGate handles GET /api/features/:id/gate
func (s *Server) evaluateGate(w http.ResponseWriter, r *http.Request) {
	id := getFeatureID(r)
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
		log.Printf("error evaluating gate for feature %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to evaluate gate")
		return
	}

	writeJSON(w, http.StatusOK, GateResultToResponse(gr))
}

// processFeature handles POST /api/features/:id/process
func (s *Server) processFeature(w http.ResponseWriter, r *http.Request) {
	id := getFeatureID(r)
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	f, err := s.pipeline.GetFeature(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	if s.isActive(id) {
		writeError(w, http.StatusConflict, "already_processing", fmt.Sprintf("Feature %s is already being processed", id))
		return
	}

	if f.IsTerminal() {
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Feature %s is in a terminal state (%s)", id, f.Status))
		return
	}

	// Mark as active
	s.setActive(id)

	// Start processing in a goroutine
	go func() {
		defer s.clearActive(id)
		ctx := context.Background()
		eventCh := make(chan pipeline.ProcessEvent, 100)

		// Start ProcessAsync, feeding events to the channel
		done := make(chan error, 1)
		go func() {
			err := s.pipeline.ProcessAsync(ctx, f, eventCh)
			done <- err
			close(eventCh)
		}()

		// Forward events to SSE clients
		for evt := range eventCh {
			switch evt.Type {
			case "phase_change":
				s.broadcastEvent(id, "phase_change", PhaseChangeEvent{
					FeatureID: id,
					Phase:     string(evt.Phase),
					Status:    evt.Status,
					Timestamp: evt.Timestamp,
				})
			case "gate_result":
				s.broadcastEvent(id, "gate_result", GateResultEvent{
					FeatureID: id,
					Phase:     string(evt.Phase),
					Passed:    evt.Passed,
					Checks:    pipelineChecksToAPI(evt.Checks),
				})
			case "agent_dispatch":
				s.broadcastEvent(id, "agent_dispatch", AgentDispatchEvent{
					FeatureID: id,
					Phase:     string(evt.Phase),
					Role:       evt.Role,
					Status:    evt.Status,
					Timestamp: evt.Timestamp,
				})
			case "agent_complete":
				s.broadcastEvent(id, "agent_complete", AgentCompleteEvent{
					FeatureID:  id,
					Phase:      string(evt.Phase),
					Role:        evt.Role,
					Status:     evt.Status,
					DurationMs: evt.DurationMs,
				})
			case "processing_complete":
				s.broadcastEvent(id, "processing_complete", ProcessingCompleteEvent{
					FeatureID: id,
					Status:    evt.Status,
					Timestamp: evt.Timestamp,
				})
			case "error":
				s.broadcastEvent(id, "error", ErrorEvent{
					FeatureID: id,
					Message:   evt.Message,
					Timestamp: evt.Timestamp,
				})
			}
		}

		if err := <-done; err != nil {
			log.Printf("error processing feature %s: %v", id, err)
			s.broadcastEvent(id, "error", ErrorEvent{
				FeatureID: id,
				Message:   "Processing failed. Check server logs for details.",
				Timestamp: time.Now(),
			})
		}
	}()

	// Return current feature state immediately
	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f))
}

// getArtifact handles GET /api/features/:id/artifacts/:type
func (s *Server) getArtifact(w http.ResponseWriter, r *http.Request) {
	id := getFeatureID(r)
	artType := r.PathValue("type")
	if id == "" || artType == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID and artifact type are required")
		return
	}

	// Check feature exists
	_, err := s.pipeline.GetFeature(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	// Map API path to ArtifactType
	featureType, ok := feature.ArtifactAPIPathToType(artType)
	if !ok {
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Invalid artifact type %q", artType))
		return
	}

	// Check if artifact exists
	if !s.specProvider.ArtifactExists(id, featureType) {
		writeError(w, http.StatusNotFound, "artifact_not_found", fmt.Sprintf("Artifact %s not yet generated for feature %s", artType, id))
		return
	}

	// Handle docs directory type
	if featureType == feature.ArtifactDocs {
		content, err := s.specProvider.ReadArtifactContent(id, featureType)
		if err != nil {
			writeError(w, http.StatusNotFound, "artifact_not_found", fmt.Sprintf("Docs directory not found for feature %s", id))
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(content))
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

// streamFeature handles GET /api/features/:id/stream (SSE)
func (s *Server) streamFeature(w http.ResponseWriter, r *http.Request) {
	id := getFeatureID(r)
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	// Check feature exists
	_, err := s.pipeline.GetFeature(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "feature_not_found", fmt.Sprintf("Feature %s not found", id))
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Register client channel
	ch := make(chan SSEMessage, 50)
	s.sseRegistry.Register(id, ch)
	defer s.sseRegistry.Unregister(id, ch)

	// Flush headers
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Send keep-alive ticker
	keepAlive := time.NewTicker(30 * time.Second)
	defer keepAlive.Stop()

	// Listen for client disconnect or events
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

// pipelineChecksToAPI converts pipeline.CheckResult slice to API CheckResultResponse slice
func pipelineChecksToAPI(checks []pipeline.CheckResult) []CheckResultResponse {
	if checks == nil {
		return []CheckResultResponse{}
	}
	result := make([]CheckResultResponse, 0, len(checks))
	for _, c := range checks {
		result = append(result, CheckResultResponse{
			Name:    c.Name,
			Passed:  c.Passed,
			Message: c.Message,
		})
	}
	return result
}