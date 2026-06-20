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
	baseDir       string
	staticFS      fs.FS
}

func NewServer(addr string, specProvider *spec.SpecProvider, pipeline *pipeline.Pipeline, staticFS fs.FS) *Server {
	s := &Server{
		specProvider: specProvider,
		pipeline:     pipeline,
		baseDir:      specProvider.BaseDir(),
		staticFS:     staticFS,
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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

	resp := FeaturesToSummaryResponse(features)
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

	writeJSON(w, http.StatusCreated, FeatureToDetailResponse(f))
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

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f))
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

	result, err := s.pipeline.RunPhaseWithAgent(r.Context(), f)
	if err != nil {
		log.Printf("error running phase for feature %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("Failed to run phase for feature %s", id))
		return
	}

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

	if f.Current == feature.PhaseDelivery {
		gr, _ := s.pipeline.EvaluateGate(f)
		if gr != nil && gr.Passed {
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

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f))
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

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f))
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

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f))
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

	if _, loaded := s.activeProcess.LoadOrStore(id, true); loaded {
		writeError(w, http.StatusConflict, "already_processing", fmt.Sprintf("Feature %s is already being processed", id))
		return
	}

	if f.IsTerminal() {
		s.activeProcess.Delete(id)
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Feature %s is in a terminal state (%s)", id, f.Status))
		return
	}

	writeJSON(w, http.StatusOK, FeatureToDetailResponse(f))

	go func() {
		defer s.activeProcess.Delete(id)
		ctx := context.Background()
		eventCh := make(chan pipeline.ProcessEvent, 100)

		done := make(chan error, 1)
		go func() {
			done <- s.pipeline.ProcessAsync(ctx, f, eventCh)
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