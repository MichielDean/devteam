package api

import (
	"net/http"
)

// getGateHistory returns all gate evaluations for a feature.
func (s *Server) getGateHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	history, err := s.db.GetGateHistory(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get gate history")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"feature_id":   id,
		"gate_results": history,
	})
}

// getRecirculations returns all recirculation events for a feature.
func (s *Server) getRecirculations(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	recirculations, err := s.db.GetRecirculations(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get recirculations")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"feature_id":     id,
		"recirculations": recirculations,
	})
}

// getEvents returns the full audit trail for a feature.
func (s *Server) getEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	events, err := s.db.GetEvents(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get events")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"feature_id": id,
		"events":     events,
	})
}

// getNotes returns all inter-phase notes for a feature.
func (s *Server) getNotes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	notes, err := s.db.GetNotes(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get notes")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"feature_id": id,
		"notes":      notes,
	})
}

// getChurnMetrics returns churn metrics for a feature.
func (s *Server) getChurnMetrics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Feature ID is required")
		return
	}

	metrics, err := s.db.GetChurnMetrics(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get churn metrics")
		return
	}

	writeJSON(w, http.StatusOK, metrics)
}