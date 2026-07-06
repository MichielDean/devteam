package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
)

// ensureFeatureInDB inserts a minimal feature row if it doesn't exist (for FK constraints)
func ensureFeatureInDB(database *db.DB, featureID string) {
	database.Exec(`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count) VALUES (?, ?, 'inception', 'in_progress', 3, 'loose_idea', '', ?, ?, 0) ON CONFLICT (id) DO NOTHING`,
		featureID, featureID, time.Now().UTC(), time.Now().UTC())
}

// SignalRequest is the body for POST /api/features/{id}/signal
type SignalRequest struct {
	Outcome string `json:"outcome"`
	Target  string `json:"target,omitempty"`
	Notes   string `json:"notes,omitempty"`
}

// handleSignal handles POST /api/features/{id}/signal
func (s *Server) handleSignal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	var req SignalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}

	if req.Outcome == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "outcome is required")
		return
	}

	if s.db != nil {
		ensureFeatureInDB(s.db, id)

		// Load feature to get current phase for the outcome record
		var phase string
		if f, err := s.pipeline.GetFeature(id); err == nil {
			phase = string(f.CurrentPhase())
		}

		// Parse outcome: "pass", "recirculate:construction", "needs_feedback", "failed"
		outcome := req.Outcome
		target := req.Target
		if strings.HasPrefix(outcome, "recirculate:") {
			parts := strings.SplitN(outcome, ":", 2)
			outcome = parts[0]
			if target == "" && len(parts) > 1 {
				target = parts[1]
			}
		}

		// Save outcome to DB — pipeline reads this after dispatch completes
		s.db.SaveOutcome(id, phase, outcome, target, req.Notes)

		eventType := "phase_complete"
		if outcome == "recirculate" {
			eventType = "recirculate"
		} else if outcome == "needs_feedback" {
			eventType = "needs_feedback"
		} else if outcome == "failed" {
			eventType = "failed"
		}
		s.db.RecordEvent(id, eventType, phase, req.Notes)
	}

	s.broadcastSSE(id, "outcome_signal", fmt.Sprintf(`{"feature_id":"%s","outcome":"%s","target":"%s","notes":"%s"}`,
		id, req.Outcome, req.Target, escapeJSON(req.Notes)))

	// In autonomous/guided mode, auto-answer questions via human-proxy agent
	if req.Outcome == "needs_feedback" {
		f, err := s.pipeline.GetFeature(id)
		if err == nil && (f.ExecutionMode == "autonomous" || f.ExecutionMode == "guided") {
			log.Printf("handleSignal: needs_feedback in %s mode — dispatching human-proxy agent for %s", f.ExecutionMode, id)
			// Use current phase (not stage) for session naming — stage may be empty
			phase := string(f.CurrentPhase())
			if phase == "" {
				phase = "ideation"
			}
			go s.dispatchHumanProxy(id, phase)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"feature_id": id,
		"outcome":    req.Outcome,
		"status":     "recorded",
	})
}

// dispatchHumanProxy runs the human-proxy agent to auto-answer pending questions.
// The agent reads questions via CLI, reads artifacts for context, answers each
// question, and signals pass to resume the stage.
func (s *Server) dispatchHumanProxy(featureID, phase string) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("dispatchHumanProxy panic for feature %s: %v", featureID, rec)
		}
	}()

	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		log.Printf("dispatchHumanProxy: failed to get feature %s: %v", featureID, err)
		return
	}

	// Build context for the human-proxy agent
	stageDef, _ := s.db.GetStageDefinition(f.CurrentStage)
	stageName := ""
	if stageDef != nil {
		stageName = stageDef.Name
	}

	// Dispatch the human-proxy agent in a tmux session
	tmuxMgr := s.pipeline.Dispatcher().TmuxManager()
	sessionName := tmuxMgr.SessionNameForPhase(featureID, phase) + "-proxy"
	contextDir := tmuxMgr.ContextDirForPhase(featureID, phase) + "-proxy"

	// Prepare context dir
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		log.Printf("dispatchHumanProxy: failed to create context dir: %v", err)
		return
	}

	// Write minimal context
	contextContent := fmt.Sprintf("# Human Proxy Task\n\nFeature: %s\nTitle: %s\nScope: %s\nDepth: %s\nCurrent Stage: %s (%s)\n\nA stage agent has asked questions that need answers. You are acting as the human-in-the-loop proxy. Read the questions, review the artifacts and code, and answer them.\n",
		f.ID, f.Title, f.Scope, f.Depth, f.CurrentStage, stageName)
	os.WriteFile(filepath.Join(contextDir, "CONTEXT.md"), []byte(contextContent), 0644)

	// Write opencode config (same isolated config as stage agents)
	opencodeConfig := `{
  "$schema": "https://opencode.ai/config.json",
  "model": "ollama/glm-5.2:cloud",
  "permission": "allow",
  "instructions": [],
  "plugin": [],
  "compaction": { "enabled": false },
  "snapshot": false,
  "mcp": {},
  "agent": {},
  "provider": {
    "ollama": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Ollama (local)",
      "options": { "baseURL": "http://localhost:11434/v1" },
      "models": { "glm-5.2:cloud": { "name": "GLM 5.2 Cloud" } }
    }
  }
}`
	os.WriteFile(filepath.Join(contextDir, "opencode.json"), []byte(opencodeConfig), 0644)

	// Write AGENTS.md
	agentsMD := "# Human Proxy Agent\n\nYou are answering questions on behalf of the human in an autonomous pipeline. Use the devteam CLI to read and answer questions.\n"
	os.WriteFile(filepath.Join(contextDir, "AGENTS.md"), []byte(agentsMD), 0644)

	// Write .bashrc for PATH
	devteamPath, _ := exec.LookPath("devteam")
	if devteamPath == "" {
		devteamPath = filepath.Join(os.Getenv("HOME"), "go", "bin", "devteam")
	}
	if devteamPath != "" {
		os.WriteFile(filepath.Join(contextDir, ".bashrc"), []byte(fmt.Sprintf("export PATH=\"%s:$PATH\"\n", filepath.Dir(devteamPath))), 0644)
	}

	// Write agent role file
	agentsDir := filepath.Join(contextDir, "agents")
	os.MkdirAll(agentsDir, 0755)
	agentMD := "---\ndescription: Answers questions on behalf of the human\nmode: primary\nmodel: ollama/glm-5.2:cloud\n---\n\nYou are the human-proxy agent. Read the pending questions and answer them.\n"
	os.WriteFile(filepath.Join(agentsDir, "human-proxy.md"), []byte(agentMD), 0644)

	// Build and run the opencode command
	cmdPath := "opencode"
	if p, err := exec.LookPath("opencode"); err == nil {
		cmdPath = p
	}
	prompt := fmt.Sprintf("Read CONTEXT.md. Then: devteam questions list %s. Answer each pending question using devteam questions answer. Then signal pass.", featureID)
	agentCmd := fmt.Sprintf("%s run --pure --dangerously-skip-permissions --agent human-proxy '%s' 2>&1 | tee %s; echo ${PIPESTATUS[0]} > %s",
		cmdPath, prompt,
		filepath.Join(contextDir, "proxy.log"),
		filepath.Join(contextDir, "exit_code"))

	logPath := filepath.Join(contextDir, "logs")
	os.MkdirAll(logPath, 0755)

	// Create tmux session
	args := []string{"new-session", "-d", "-s", sessionName, "-c", contextDir}
	home := os.Getenv("HOME")
	tmuxPath := os.Getenv("PATH")
	if home != "" {
		tmuxPath = filepath.Join(home, "go/bin") + ":" + filepath.Join(home, ".opencode/bin") + ":" + tmuxPath
	}
	args = append(args, "-e", "PATH="+tmuxPath, "-e", "HOME="+home, "-e", "OPENCODE_CONFIG_DIR="+contextDir, "-e", "OPENCODE_DISABLE_PROJECT_CONFIG=1")
	args = append(args, agentCmd)

	createCmd := exec.Command("tmux", args...)
	createCmd.Env = minimalTmuxEnvForProxy()
	if out, err := createCmd.CombinedOutput(); err != nil {
		log.Printf("dispatchHumanProxy: failed to create tmux session: %v: %s", err, string(out))
		return
	}

	log.Printf("dispatchHumanProxy: created session %s for feature %s", sessionName, featureID)

	// Poll for tmux session exit
	for i := 0; i < 600; i++ { // 10 minute timeout
		if !tmuxMgr.IsSessionAlive(sessionName) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Kill if still alive
	tmuxMgr.KillSession(sessionName)

	// Read exit code to check if the proxy actually ran
	exitCodeBytes, _ := os.ReadFile(filepath.Join(contextDir, "exit_code"))
	exitCode := strings.TrimSpace(string(exitCodeBytes))
	if exitCode != "0" {
		log.Printf("dispatchHumanProxy: proxy exited with code %s (not 0) — agent may not have answered questions", exitCode)
		// Read the proxy log for debugging
		proxyLog, _ := os.ReadFile(filepath.Join(contextDir, "proxy.log"))
		log.Printf("dispatchHumanProxy: proxy log tail: %s", string(proxyLog))
	}

	// Check if questions were answered
	questions, _ := s.db.GetPendingQuestions(featureID)
	if len(questions) > 0 {
		log.Printf("dispatchHumanProxy: %d questions still pending after proxy (exit=%s) — leaving for human", len(questions), exitCode)
		return
	}

	// All questions answered — resume the existing tmux session.
	// The stage agent was waiting for needs_feedback. Send "continue" to
	// the same tmux session so it reads the answers and finishes the artifacts.
	// Do NOT re-dispatch from scratch — the agent already has context.
	log.Printf("dispatchHumanProxy: all questions answered — resuming existing session for %s", featureID)

	// Find the stage's tmux session
	stageDef2, _ := s.db.GetStageDefinition(f.CurrentStage)
	if stageDef2 == nil {
		log.Printf("dispatchHumanProxy: could not find stage def for %s", f.CurrentStage)
		return
	}

	tmuxMgr2 := s.pipeline.Dispatcher().TmuxManager()
	boltNumber2 := 0
	if stageDef2.Phase == "construction" && f.CurrentBolt > 0 {
		boltNumber2 = f.CurrentBolt
	}
	var resumeSessionName string
	if boltNumber2 > 0 {
		resumeSessionName = tmuxMgr2.SessionNameForBolt(featureID, boltNumber2)
	} else {
		resumeSessionName = tmuxMgr2.SessionNameForPhase(featureID, stageDef2.Phase)
	}

	if !tmuxMgr2.IsSessionAlive(resumeSessionName) {
		// Session died — fall back to fresh dispatch
		log.Printf("dispatchHumanProxy: session %s is dead — falling back to fresh dispatch", resumeSessionName)
		if !s.isFeatureActive(featureID) {
			s.markFeatureActive(featureID)
			go s.runStageAsync(context.Background(), featureID, f.CurrentStage)
		}
		return
	}

	// Session is alive — send "continue" and poll for exit
	if !s.isFeatureActive(featureID) {
		s.markFeatureActive(featureID)
	}
	go func() {
		defer s.unmarkFeatureActive(featureID)
		log.Printf("dispatchHumanProxy: sending continue to session %s", resumeSessionName)
		exec.Command("tmux", "send-keys", "-t", resumeSessionName,
			"The user has answered your questions. Run 'devteam questions list "+featureID+"' to see the answers, then continue with your task and signal pass when done.", "Enter").Run()

		// Poll for session exit
		for i := 0; i < 600; i++ {
			if !tmuxMgr2.IsSessionAlive(resumeSessionName) {
				break
			}
			time.Sleep(5 * time.Second)
		}
		if tmuxMgr2.IsSessionAlive(resumeSessionName) {
			tmuxMgr2.KillSession(resumeSessionName)
		}

		// Process the outcome via the state machine
		s.recoverStage(featureID, f.CurrentStage)
	}()
}

func minimalTmuxEnvForProxy() []string {
	home, _ := os.UserHomeDir()
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"USER=" + os.Getenv("USER"),
		"SHELL=" + os.Getenv("SHELL"),
		"TERM=" + os.Getenv("TERM"),
	}
	return env
}

// NotesRequest is the body for POST /api/features/{id}/notes
type NotesRequest struct {
	Phase   string `json:"phase"`
	Content string `json:"content"`
}

// handleAddNote handles POST /api/features/{id}/notes
func (s *Server) handleAddNote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	var req NotesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "content is required")
		return
	}

	if s.db != nil {
		ensureFeatureInDB(s.db, id)
		s.db.AddNote(id, req.Phase, "agent", "summary", req.Content)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"feature_id": id,
		"phase":      req.Phase,
		"status":     "recorded",
	})
}

// ArtifactRequest is the body for POST /api/features/{id}/artifacts/{type}
type ArtifactRequest struct {
	Content string `json:"content"`
}

// handleSubmitArtifact handles POST /api/features/{id}/artifacts/{type}
func (s *Server) handleSubmitArtifact(w http.ResponseWriter, r *http.Request) {
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

	var req ArtifactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "content is required")
		return
	}

	if s.db != nil {
		ensureFeatureInDB(s.db, id)
		// Look up the feature's current stage to tag the artifact
		stageID := ""
		if f, err := s.pipeline.GetFeature(id); err == nil && f != nil {
			stageID = f.CurrentStage
		}
		// Check if artifact already exists (create vs update)
		existing, _ := s.db.GetArtifact(id, dbKey)
		if err := s.db.SaveArtifactWithStage(id, dbKey, req.Content, stageID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("Failed to save artifact: %v", err))
			return
		}
		// Record audit event
		eventType := db.AuditArtifactCreated
		if existing != nil {
			eventType = db.AuditArtifactUpdated
		}
		s.db.RecordAuditEvent(id, eventType, stageID, "", fmt.Sprintf("artifact=%s size=%d", dbKey, len(req.Content)))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"feature_id":    id,
		"artifact_type": artType,
		"size":          len(req.Content),
		"status":        "saved",
	})
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}
