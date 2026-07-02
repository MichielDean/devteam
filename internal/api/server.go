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

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/intake"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/ratelimit"
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
	// Rate limiting (rate-limiting-middleware feature). All three are nil when
	// the limiter is disabled (absent rate_limit block or enabled=false), in
	// which case rateLimitMiddleware is pure pass-through (D7/R12, BR-33).
	rateLimiter *ratelimit.Limiter
	rlExtractor ratelimit.KeyExtractor
	rlCfg       *config.RateLimitConfig
	// mux is the root ServeMux; retained so ConfigureRateLimiting can register
	// the status route only when the limiter is armed (BR-47).
	mux *http.ServeMux
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

	// Chain order (D6/ADR-004): recovery(cors(rateLimit(mux))). Recovery is
	// outermost so it catches panics in all inner handlers (including the
	// limiter's own malfunction path — two-layer recovery, NDP-02). CORS is
	// middle so OPTIONS preflight short-circuits to 204 BEFORE the limiter runs
	// (browsers break if preflight is throttled, BR-34). Rate limit is innermost
	// (before the mux) so only real requests are counted.
	handler := s.recoveryMiddleware(s.corsMiddleware(s.rateLimitMiddleware(mux)))

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Retain the mux so ConfigureRateLimiting (U-W) can register the status
	// route ONLY when the limiter is armed (BR-47 — when disabled, the route
	// 404s, byte-identical to today, R12).
	s.mux = mux

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

// rateLimitMiddleware is the rate-limit hot-path closure (U-D/US3). When
// s.rateLimiter is nil (limiter disabled — absent rate_limit block or
// enabled=false), it is pure pass-through, byte-identical to the pre-feature
// chain (D7/R12/BR-33). When armed, it enacts the LOCKED request lifecycle
// (business-logic-model §2.1):
//
//  1. Exempt-route short-circuit BEFORE Allow (BR-09) — exempt overrides and
//     GET /health/rate-limit pass through with NO header mutation, NO count,
//     NO log. Exemption is structural (route-match-before-Allow), so the
//     status endpoint cannot be locked out by the limiter it observes (D3).
//  2. Compute composite key (BR-10) via rlExtractor.
//  3. Sliding-window verdict via Limiter.Allow.
//  4. Branch on verdict + dry_run:
//     - allow → set advisory headers (BR-22/O-9), call next, return.
//     - deny + dry_run → set advisory headers + Retry-After (M5), log
//       "dry_run would_reject" (M4.3), call next, return (NO 429).
//     - deny + enforce → set 429 headers (BR-20), writeError 429 body (BR-21),
//       log "rejected" (BR-31), rejections_total++, return (next NOT called).
//  5. Malfunction catch (BR-50/NDP-01): if Allow returns err != nil, log
//     "internal_error" (BR-32), failures_total++, NO RateLimit-* headers
//     (invariant 4), call next (traffic flows — fail-open), return.
//
// The closure has its own defer/recover (NDP-02) so a panic in key extraction or
// header-setting is caught by the limiter's malfunction path first; if THAT
// path panics, the outer recoveryMiddleware catches it → 500 (two layers, D6).
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// NDP-08: nil limiter = pure pass-through, byte-identical to today.
		if s.rateLimiter == nil {
			next.ServeHTTP(w, r)
			return
		}

		// Two-layer recovery (NDP-02). The inner defer/recover converts a panic
		// in the limiter closure to a fail-open outcome (allow + log + next). If
		// this recovery itself panics, the outer recoveryMiddleware catches it.
		defer func() {
			if rec := recover(); rec != nil {
				s.rateLimiter.RecordFailure()
				log.Printf("rate_limit: internal_error err=%q key=<unknown> route=%s decision=allow fail_mode=%s", rec, r.Method+" "+r.URL.Path, s.failMode())
				// Fail-open: no RateLimit-* headers, traffic flows (invariant 4).
				next.ServeHTTP(w, r)
			}
		}()

		route := r.Method + " " + r.URL.Path

		// BR-09: exempt-route short-circuit BEFORE Allow. The status endpoint
		// (GET /health/rate-limit) is exempt by construction here, AND registered
		// only when armed (BR-47) — belt and suspenders.
		if route == "GET /health/rate-limit" || s.isExemptRoute(route) {
			next.ServeHTTP(w, r)
			return
		}

		key, _ := s.rlExtractor.Key(r)

		v, err := s.rateLimiter.Allow(key)
		if err != nil {
			// Malfunction (BR-50/NDP-01): fail-open. Allow returned allow=true + err.
			s.rateLimiter.RecordFailure()
			log.Printf("rate_limit: internal_error err=%q key=%s route=%s decision=allow fail_mode=%s", err, key, route, s.failMode())
			// NO RateLimit-* headers (invariant 4) — response looks like normal handler.
			next.ServeHTTP(w, r)
			return
		}

		if v.Allow {
			// Allow path: advisory headers ALWAYS (O-9/BR-22 — reversed C-2). Set on
			// w.Header() BEFORE next.ServeHTTP calls WriteHeader (invariant 1/BR-18).
			s.setAdvisoryHeaders(w, v)
			next.ServeHTTP(w, r)
			return
		}

		// Deny path. Counter already incremented in Allow (BR-11 — count==limit+1).
		dryRun := s.rlCfg != nil && s.rlCfg.GetDryRun()
		if dryRun {
			// M5/M4.3: dry-run never rejects. Advisory headers + Retry-After, log
			// "dry_run would_reject", call next, return. rejections_total NOT incremented.
			s.setAdvisoryHeaders(w, v)
			w.Header().Set("Retry-After", secondsStr(v.ResetIn))
			log.Printf("rate_limit: dry_run would_reject key=%s route=%s count=%d limit=%d window=%ds retry_after=%ds",
				key, route, v.Count, v.Limit, int(v.Window.Seconds()), int(v.ResetIn.Seconds()))
			next.ServeHTTP(w, r)
			return
		}

		// Clean deny (BR-19): 429. Set 429 headers + body BEFORE WriteHeader.
		s.writeRateLimitRejection(w, v, route)
		s.rateLimiter.RecordRejection()
		log.Printf("rate_limit: rejected key=%s route=%s count=%d limit=%d window=%ds retry_after=%ds",
			key, route, v.Count, v.Limit, int(v.Window.Seconds()), int(v.ResetIn.Seconds()))
		// next is NOT called (invariant 2 — the handler never sees the rejected request).
	})
}

// isExemptRoute reports whether the route matches an exempt override. MVP
// supports overrides via config (U-C parses them; U-I does the lookup — U-I is
// post-MVP, but the lookup helper is here so the MVP middleware is
// override-aware and the post-MVP unit is additive). With no overrides this
// returns false for every route.
func (s *Server) isExemptRoute(route string) bool {
	if s.rlCfg == nil || len(s.rlCfg.EndpointOverrides) == 0 {
		return false
	}
	ov, ok := s.rlCfg.EndpointOverrides[route]
	return ok && ov.Exempt
}

// failMode returns the configured fail mode string ("fail_open" in v1) or the
// default if unset. Used in log lines so the operator can audit the decision
// without a YAML cross-ref.
func (s *Server) failMode() string {
	if s.rlCfg != nil && s.rlCfg.FailMode != "" {
		return s.rlCfg.FailMode
	}
	return "fail_open"
}

// setAdvisoryHeaders sets the 4 advisory headers on the allow path (O-9/BR-22).
// Must be called BEFORE next.ServeHTTP (which calls WriteHeader) — stdlib locks
// the header map at WriteHeader time; setting after is a silent no-op (invariant 1).
func (s *Server) setAdvisoryHeaders(w http.ResponseWriter, v ratelimit.Verdict) {
	w.Header().Set("RateLimit-Limit", fmt.Sprintf("%d", v.Limit))
	remaining := v.Limit - int(v.Count)
	if remaining < 0 {
		remaining = 0
	}
	w.Header().Set("RateLimit-Remaining", fmt.Sprintf("%d", remaining))
	w.Header().Set("RateLimit-Reset", secondsStr(v.ResetIn))
	w.Header().Set("X-RateLimit-Policy", fmt.Sprintf("%d;w=%d", v.Limit, int(v.Window.Seconds())))
}

// secondsStr formats a duration as an integer delta-seconds string (O-3 —
// delta-seconds, NOT epoch/HTTP-date). Rounded down.
func secondsStr(d time.Duration) string {
	s := int(d.Seconds())
	if s < 0 {
		s = 0
	}
	return fmt.Sprintf("%d", s)
}

// ConfigureRateLimiting arms (or skips) the rate limiter from config (U-W/D10).
// Setter-based wiring, NOT cfg-threaded NewServer (ADR-007 — 86 existing call
// sites stay valid; the regression guard is TestNewServerSignatureUnchanged).
//
// Behavior (business-logic-model §4, §3.6):
//   - cfg == nil || !cfg.Enabled → return (passthrough, D7 — s.rateLimiter stays nil).
//   - Validate (BR-01..BR-07) → on error: log + return (ADR-008 fail-open startup;
//     server starts WITHOUT the limiter; main.go does NOT exit). This is the
//     NDP-07 critical path — validation MUST live here, NOT in the fatal
//     config.validateConfig path (see config.go note on validateConfig).
//   - Build limiter; on error → log + return (fail-open).
//   - On success: set s.rateLimiter, s.rlExtractor, s.rlCfg; register the status
//     route ONLY when armed (BR-47 — when disabled, the route 404s).
func (s *Server) ConfigureRateLimiting(cfg *config.RateLimitConfig) {
	if cfg == nil || !cfg.Enabled {
		return // passthrough (D7/R12)
	}
	if err := cfg.Validate(); err != nil {
		// NDP-07/O-5/ADR-008: log + run WITHOUT the limiter (no crash).
		log.Printf("rate_limit: config invalid: %v", err)
		return
	}
	limit := cfg.GetDefaultLimit()
	window := time.Duration(cfg.GetDefaultWindowSeconds()) * time.Second
	limiter, err := ratelimit.New(
		ratelimit.Policy{Limit: limit, Window: window},
		ratelimit.WithMaxTrackedKeys(cfg.GetMaxTrackedKeys()),
	)
	if err != nil {
		log.Printf("rate_limit: limiter build failed, running without limiter: %v", err)
		return
	}
	s.rateLimiter = limiter
	s.rlCfg = cfg
	s.rlExtractor = ratelimit.KeyExtractor{TrustProxyHeaders: cfg.GetTrustProxyHeaders()}
	// BR-47: register the status route ONLY when armed. When disabled, the route
	// 404s (byte-identical to today, R12).
	if s.mux != nil {
		s.mux.HandleFunc("GET /health/rate-limit", s.handleRateLimitStatus)
	}
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
		s.db.Exec(`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0) ON CONFLICT (id) DO NOTHING`,
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