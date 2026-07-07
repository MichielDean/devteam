package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/MichielDean/devteam/internal/db"
)

// ─── Audit read endpoint (Bolt 4 / v1 tab) ───
//
// GET /api/audit returns filtered, paginated config-mutation audit events
// (FR-AUDIT-01). Filters: type (single or comma-list), feature_id, actor,
// from/to (ISO 8601). Pagination: page (1-based), page_size (default 50,
// max 200). Read-only, unguarded (FR-ROUTE-02). Backed by
// idx_audit_events_type_time (FR-AUDIT-02).

func (s *Server) listAuditHandler(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"events":    []db.AuditEvent{},
			"total":     0,
			"page":      1,
			"page_size": 50,
		})
		return
	}

	q := r.URL.Query()
	filter := db.AuditFilter{
		EventType: q.Get("type"),
		FeatureID: q.Get("feature_id"),
		Actor:     q.Get("actor"),
	}
	if fromStr := q.Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			filter.From = t
		}
	}
	if toStr := q.Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			filter.To = t
		}
	}
	if pageStr := q.Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil {
			filter.Page = p
		}
	}
	if psStr := q.Get("page_size"); psStr != "" {
		if ps, err := strconv.Atoi(psStr); err == nil {
			filter.PageSize = ps
		}
	}

	events, total, err := s.db.GetAuditEventsFiltered(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to query audit events")
		return
	}
	if events == nil {
		events = []db.AuditEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"events":    events,
		"total":     total,
		"page":      filter.Page,
		"page_size": filter.PageSize,
	})
}