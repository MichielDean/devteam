package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/stage"
)

// registerStageRoutes adds the AIDLC v2 stage-based API endpoints.
func (s *Server) registerStageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/features/{id}/run-stage", s.runStage)
	mux.HandleFunc("POST /api/features/{id}/stages/{stageId}/approve", s.approveStage)
	mux.HandleFunc("POST /api/features/{id}/stages/{stageId}/reject", s.rejectStage)
	mux.HandleFunc("POST /api/features/{id}/stages/{stageId}/accept-as-is", s.acceptStageAsIs)
	mux.HandleFunc("POST /api/features/{id}/stages/{stageId}/add-skipped", s.addSkippedStage)
	mux.HandleFunc("POST /api/features/{id}/jump", s.jumpToStage)
	mux.HandleFunc("GET /api/features/{id}/stages", s.getFeatureStages)
	mux.HandleFunc("GET /api/features/{id}/audit", s.getAuditTrail)
	mux.HandleFunc("POST /api/features/{id}/scope", s.setScope)
	mux.HandleFunc("POST /api/features/{id}/depth", s.setDepth)
	mux.HandleFunc("POST /api/features/{id}/test-strategy", s.setTestStrategy)
	mux.HandleFunc("POST /api/features/{id}/ladder", s.setLadderMode)
	mux.HandleFunc("GET /api/features/{id}/bolts", s.getBolts)
	mux.HandleFunc("POST /api/features/{id}/prepare-bolts", s.prepareBolts)
	mux.HandleFunc("POST /api/features/{id}/run-bolt/{boltNumber}", s.runBolt)
	mux.HandleFunc("GET /api/features/{id}/rules", s.getRules)
	mux.HandleFunc("DELETE /api/features/{id}/rules/{ruleId}", s.deleteRule)

	// Team knowledge CRUD
	mux.HandleFunc("GET /api/knowledge", s.listAllKnowledge)
	mux.HandleFunc("GET /api/knowledge/{agent}", s.getKnowledge)
	mux.HandleFunc("POST /api/knowledge/{agent}", s.saveKnowledge)
	mux.HandleFunc("PATCH /api/knowledge/{agent}/{topic}", s.updateKnowledge)
	mux.HandleFunc("DELETE /api/knowledge/{agent}/{topic}", s.deleteKnowledge)
}

// runStage dispatches the lead agent for one stage.
// Returns immediately with 202 Accepted — agent output streams via SSE.
// The final result is broadcast as a "processing_complete" SSE event.
func (s *Server) runStage(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		http.Error(w, `{"error":"feature_not_found","details":"`+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	var req struct {
		StageID string `json:"stage_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad_request","details":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if req.StageID == "" {
		http.Error(w, `{"error":"bad_request","details":"stage_id required"}`, http.StatusBadRequest)
		return
	}

	if !stage.IsValidStageID(req.StageID) {
		http.Error(w, `{"error":"bad_request","details":"invalid stage_id format"}`, http.StatusBadRequest)
		return
	}

	if s.isFeatureActive(featureID) {
		http.Error(w, `{"error":"conflict","details":"feature already running"}`, http.StatusConflict)
		return
	}

	// Initialize feature stages if not done
	scope := f.Scope
	if scope == "" {
		scope = stage.ScopeFeature
	}
	if s.db != nil {
		fstages, _ := s.db.GetFeatureStages(featureID)
		if len(fstages) == 0 {
			s.db.InitFeatureStages(featureID, scope)
		}
	}

	s.markFeatureActive(featureID)

	// Run the stage asynchronously — output streams via SSE
	go func() {
		defer s.unmarkFeatureActive(featureID)
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("runStage goroutine panic for feature %s stage %s: %v", featureID, req.StageID, rec)
				s.broadcastSSE(featureID, "error", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"message":"internal panic"}`, jsonString(featureID), jsonString(req.StageID)))
			}
		}()

		result, err := s.pipeline.RunStage(context.Background(), f, req.StageID, func(line string, isStderr bool) {
			s.broadcastSSE(featureID, "agent_output", fmt.Sprintf(`{"line":%s,"stderr":%v}`, jsonString(line), isStderr))
		})
		if err != nil {
			log.Printf("runStage: dispatch failed for feature %s stage %s: %v", featureID, req.StageID, err)
			s.broadcastSSE(featureID, "error", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"message":%s}`, jsonString(featureID), jsonString(req.StageID), jsonString(err.Error())))
			return
		}

		// Broadcast the final result via SSE
		resultJSON, _ := json.Marshal(result)
		s.broadcastSSE(featureID, "processing_complete", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"result":%s}`, jsonString(featureID), jsonString(req.StageID), string(resultJSON)))
	}()

	// Return immediately — client watches SSE for updates
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"dispatched","stage_id":"` + req.StageID + `"}`))
}

// approveStage approves a stage gate and advances.
func (s *Server) approveStage(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	stageID := r.PathValue("stageId")
	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		http.Error(w, `{"error":"feature_not_found"}`, http.StatusNotFound)
		return
	}

	if err := s.pipeline.ApproveStage(f, stageID); err != nil {
		http.Error(w, `{"error":"approve_failed","details":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"approved"}`))
}

// rejectStage rejects a stage gate, saves rejection notes as a rule.
func (s *Server) rejectStage(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	stageID := r.PathValue("stageId")
	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		http.Error(w, `{"error":"feature_not_found"}`, http.StatusNotFound)
		return
	}

	var req struct {
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad_request","details":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if req.Notes == "" {
		http.Error(w, `{"error":"bad_request","details":"notes required"}`, http.StatusBadRequest)
		return
	}

	if err := s.pipeline.RejectStage(f, stageID, req.Notes); err != nil {
		http.Error(w, `{"error":"reject_failed","details":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"rejected"}`))
}

// acceptStageAsIs uses the 3-strike escape hatch.
func (s *Server) acceptStageAsIs(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	stageID := r.PathValue("stageId")
	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		http.Error(w, `{"error":"feature_not_found"}`, http.StatusNotFound)
		return
	}

	if err := s.pipeline.AcceptStageAsIs(f, stageID); err != nil {
		http.Error(w, `{"error":"accept_failed","details":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"accepted_as_is"}`))
}

// jumpToStage jumps to a specific stage or phase.
func (s *Server) jumpToStage(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		http.Error(w, `{"error":"feature_not_found"}`, http.StatusNotFound)
		return
	}

	var req struct {
		StageID string `json:"stage_id"`
		Phase   string `json:"phase"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad_request","details":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if req.StageID != "" {
		if err := s.pipeline.JumpToStage(f, req.StageID); err != nil {
			http.Error(w, `{"error":"jump_failed","details":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
	} else if req.Phase != "" {
		if err := s.pipeline.JumpToPhase(f, req.Phase); err != nil {
			http.Error(w, `{"error":"jump_failed","details":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
	} else {
		http.Error(w, `{"error":"bad_request","details":"stage_id or phase required"}`, http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"jumped"}`))
}

// addSkippedStage inserts a previously skipped stage back into the workflow.
// Only available for Ideation (1.x) and Inception (2.x) stages.
func (s *Server) addSkippedStage(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	stageID := r.PathValue("stageId")
	if s.db == nil {
		http.Error(w, `{"error":"no_database"}`, http.StatusInternalServerError)
		return
	}

	// Only ideation (1.x) and inception (2.x) stages can be re-added
	if len(stageID) < 3 || (stageID[0] != '1' && stageID[0] != '2') {
		http.Error(w, `{"error":"bad_request","details":"add-skipped only available for ideation (1.x) and inception (2.x) stages"}`, http.StatusBadRequest)
		return
	}

	fs, err := s.db.GetFeatureStage(featureID, stageID)
	if err != nil || fs == nil {
		http.Error(w, `{"error":"not_found","details":"stage not found for feature"}`, http.StatusNotFound)
		return
	}

	if fs.Status != stage.StatusSkipped {
		http.Error(w, `{"error":"bad_request","details":"stage is not skipped — only skipped stages can be re-added"}`, http.StatusBadRequest)
		return
	}

	// Reset the stage to not_started
	if err := s.db.UpdateFeatureStage(featureID, stageID, stage.StatusNotStarted, 0, nil, nil); err != nil {
		http.Error(w, `{"error":"update_failed","details":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	s.db.RecordAuditEvent(featureID, "STAGE_RE_ADDED", stageID, "", "")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"stage_re_added"}`))
}

// getFeatureStages returns all stages with their status for a feature.
func (s *Server) getFeatureStages(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	if s.db == nil {
		http.Error(w, `{"error":"no_database"}`, http.StatusInternalServerError)
		return
	}

	stages, err := s.db.GetFeatureStages(featureID)
	if err != nil {
		http.Error(w, `{"error":"query_failed","details":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stages)
}

// getAuditTrail returns the full audit trail for a feature.
func (s *Server) getAuditTrail(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	if s.db == nil {
		http.Error(w, `{"error":"no_database"}`, http.StatusInternalServerError)
		return
	}

	events, err := s.db.GetAuditEvents(featureID)
	if err != nil {
		http.Error(w, `{"error":"query_failed","details":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// setScope changes the scope of a feature.
func (s *Server) setScope(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	var req struct {
		Scope string `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad_request"}`, http.StatusBadRequest)
		return
	}
	if !stage.IsValidScope(req.Scope) {
		http.Error(w, `{"error":"bad_request","details":"invalid scope"}`, http.StatusBadRequest)
		return
	}

	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		http.Error(w, `{"error":"feature_not_found"}`, http.StatusNotFound)
		return
	}
	f.Scope = req.Scope
	if err := s.pipeline.SaveFeature(f); err != nil {
		http.Error(w, `{"error":"save_failed"}`, http.StatusInternalServerError)
		return
	}

	if s.db != nil {
		s.db.RecordAuditEvent(featureID, db.AuditScopeChange, "", "", req.Scope)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"scope_set"}`))
}

// setDepth changes the depth level of a feature.
func (s *Server) setDepth(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	var req struct {
		Depth string `json:"depth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad_request"}`, http.StatusBadRequest)
		return
	}

	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		http.Error(w, `{"error":"feature_not_found"}`, http.StatusNotFound)
		return
	}
	f.Depth = req.Depth
	if err := s.pipeline.SaveFeature(f); err != nil {
		http.Error(w, `{"error":"save_failed"}`, http.StatusInternalServerError)
		return
	}

	if s.db != nil {
		s.db.RecordAuditEvent(featureID, db.AuditDepthChange, "", "", req.Depth)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"depth_set"}`))
}

// setTestStrategy changes the test strategy of a feature.
func (s *Server) setTestStrategy(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	var req struct {
		TestStrategy string `json:"test_strategy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad_request"}`, http.StatusBadRequest)
		return
	}

	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		http.Error(w, `{"error":"feature_not_found"}`, http.StatusNotFound)
		return
	}
	f.TestStrategy = req.TestStrategy
	if err := s.pipeline.SaveFeature(f); err != nil {
		http.Error(w, `{"error":"save_failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"test_strategy_set"}`))
}

// setLadderMode sets the construction autonomy mode.
func (s *Server) setLadderMode(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	var req struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad_request"}`, http.StatusBadRequest)
		return
	}

	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		http.Error(w, `{"error":"feature_not_found"}`, http.StatusNotFound)
		return
	}
	if err := s.pipeline.LadderPrompt(f, req.Mode); err != nil {
		http.Error(w, `{"error":"ladder_failed","details":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ladder_set"}`))
}

// getBolts returns all Bolts for a feature.
func (s *Server) getBolts(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	if s.db == nil {
		http.Error(w, `{"error":"no_database"}`, http.StatusInternalServerError)
		return
	}

	bolts, err := s.db.GetBolts(featureID)
	if err != nil {
		http.Error(w, `{"error":"query_failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bolts)
}

// prepareBolts creates Bolt records from inception output.
func (s *Server) prepareBolts(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		http.Error(w, `{"error":"feature_not_found"}`, http.StatusNotFound)
		return
	}

	if err := s.pipeline.PrepareBolts(f); err != nil {
		http.Error(w, `{"error":"prepare_bolts_failed","details":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"bolts_prepared"}`))
}

// runBolt runs one Bolt through construction stages.
// Returns immediately with 202 Accepted — output streams via SSE.
func (s *Server) runBolt(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	boltStr := r.PathValue("boltNumber")
	f, err := s.pipeline.GetFeature(featureID)
	if err != nil {
		http.Error(w, `{"error":"feature_not_found"}`, http.StatusNotFound)
		return
	}

	var boltNumber int
	if _, err := fmt.Sscanf(boltStr, "%d", &boltNumber); err != nil || boltNumber < 1 {
		http.Error(w, `{"error":"bad_request","details":"invalid bolt number"}`, http.StatusBadRequest)
		return
	}

	if s.isFeatureActive(featureID) {
		http.Error(w, `{"error":"conflict","details":"feature already running"}`, http.StatusConflict)
		return
	}

	s.markFeatureActive(featureID)

	go func() {
		defer s.unmarkFeatureActive(featureID)
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("runBolt goroutine panic for feature %s bolt %d: %v", featureID, boltNumber, rec)
				s.broadcastSSE(featureID, "error", fmt.Sprintf(`{"feature_id":%s,"message":"bolt panic"}`, jsonString(featureID)))
			}
		}()

		result, err := s.pipeline.RunBolt(context.Background(), f, boltNumber, func(line string, isStderr bool) {
			s.broadcastSSE(featureID, "agent_output", fmt.Sprintf(`{"line":%s,"stderr":%v}`, jsonString(line), isStderr))
		})
		if err != nil {
			log.Printf("runBolt: failed for feature %s bolt %d: %v", featureID, boltNumber, err)
			s.broadcastSSE(featureID, "error", fmt.Sprintf(`{"feature_id":%s,"message":%s}`, jsonString(featureID), jsonString(err.Error())))
			return
		}
		resultJSON, _ := json.Marshal(result)
		s.broadcastSSE(featureID, "processing_complete", fmt.Sprintf(`{"feature_id":%s,"bolt":%d,"result":%s}`, jsonString(featureID), boltNumber, string(resultJSON)))
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"dispatched","bolt":` + boltStr + `}`))
}

// getRules returns learned rules for a feature.
func (s *Server) getRules(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	if s.db == nil {
		http.Error(w, `{"error":"no_database"}`, http.StatusInternalServerError)
		return
	}

	rules, err := s.db.GetRulesForFeature(featureID)
	if err != nil {
		http.Error(w, `{"error":"query_failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}

// deleteRule removes a learned rule.
func (s *Server) deleteRule(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, `{"error":"no_database"}`, http.StatusInternalServerError)
		return
	}

	ruleStr := r.PathValue("ruleId")
	var ruleID int64
	if _, err := fmt.Sscanf(ruleStr, "%d", &ruleID); err != nil {
		http.Error(w, `{"error":"bad_request","details":"invalid rule id"}`, http.StatusBadRequest)
		return
	}

	if err := s.db.DeleteRule(ruleID); err != nil {
		http.Error(w, `{"error":"delete_failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"deleted"}`))
}

// listAllKnowledge returns all team knowledge entries.
func (s *Server) listAllKnowledge(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, `{"error":"no_database"}`, http.StatusInternalServerError)
		return
	}

	// Get all agents and return their knowledge
	result := make(map[string][]db.TeamKnowledgeRow)
	for _, agent := range []string{"product", "design", "delivery", "architect", "platform", "devsecops", "developer", "quality", "pipeline-deploy", "operations", "product-lead", "architecture-reviewer"} {
		entries, _ := s.db.GetTeamKnowledge(agent)
		if len(entries) > 0 {
			result[agent] = entries
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// getKnowledge returns team knowledge for an agent.
func (s *Server) getKnowledge(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, `{"error":"no_database"}`, http.StatusInternalServerError)
		return
	}

	agent := r.PathValue("agent")
	entries, err := s.db.GetTeamKnowledge(agent)
	if err != nil {
		http.Error(w, `{"error":"query_failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// saveKnowledge saves team knowledge for an agent+topic.
func (s *Server) saveKnowledge(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, `{"error":"no_database"}`, http.StatusInternalServerError)
		return
	}

	agent := r.PathValue("agent")
	var req struct {
		Topic   string `json:"topic"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad_request"}`, http.StatusBadRequest)
		return
	}
	if req.Topic == "" || req.Content == "" {
		http.Error(w, `{"error":"bad_request","details":"topic and content required"}`, http.StatusBadRequest)
		return
	}

	if err := s.db.SaveTeamKnowledge(agent, req.Topic, req.Content); err != nil {
		http.Error(w, `{"error":"save_failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"saved"}`))
}

// deleteKnowledge removes a team knowledge entry.
func (s *Server) deleteKnowledge(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, `{"error":"no_database"}`, http.StatusInternalServerError)
		return
	}

	agent := r.PathValue("agent")
	topic := r.PathValue("topic")
	if err := s.db.DeleteTeamKnowledge(agent, topic); err != nil {
		http.Error(w, `{"error":"delete_failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"deleted"}`))
}

// updateKnowledge updates the content of an existing team knowledge entry.
func (s *Server) updateKnowledge(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, `{"error":"no_database"}`, http.StatusInternalServerError)
		return
	}

	agent := r.PathValue("agent")
	topic := r.PathValue("topic")
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad_request"}`, http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		http.Error(w, `{"error":"bad_request","details":"content required"}`, http.StatusBadRequest)
		return
	}

	if err := s.db.SaveTeamKnowledge(agent, topic, req.Content); err != nil {
		http.Error(w, `{"error":"update_failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"updated"}`))
}

// isFeatureActive checks if a feature is currently being processed.
func (s *Server) isFeatureActive(featureID string) bool {
	_, ok := s.active.Load(featureID)
	return ok
}

// markFeatureActive marks a feature as being processed.
func (s *Server) markFeatureActive(featureID string) {
	s.active.Store(featureID, struct{}{})
}

// unmarkFeatureActive unmarks a feature as being processed.
func (s *Server) unmarkFeatureActive(featureID string) {
	s.active.Delete(featureID)
}

// jsonString safely quotes a string for JSON embedding.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}