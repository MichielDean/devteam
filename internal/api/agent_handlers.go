package api

import (
	"encoding/json"
	"fmt"
	"net/http"
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"feature_id": id,
		"outcome":    req.Outcome,
		"status":     "recorded",
	})
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
		// Check if artifact already exists (create vs update)
		existing, _ := s.db.GetArtifact(id, dbKey)
		if err := s.db.SaveArtifact(id, dbKey, req.Content); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("Failed to save artifact: %v", err))
			return
		}
		// Record audit event
		eventType := db.AuditArtifactCreated
		if existing != nil {
			eventType = db.AuditArtifactUpdated
		}
		s.db.RecordAuditEvent(id, eventType, "", "", fmt.Sprintf("artifact=%s size=%d", dbKey, len(req.Content)))
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