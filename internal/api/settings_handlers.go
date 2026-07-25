package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/settings/defaults"
)

// ─── Repos write endpoints (Bolt 1) ───
//
// The sibling feature full-crud-and-ui-for-managing-repositories shipped the
// `repos` table (migration 017) and the DB store (repo_store.go) with full
// CRUD. This feature wires the HTTP write endpoints on top of that store,
// behind AdminGuard, with audit emission (FR-REPOS-04..07).
//
// GET /api/repos is refactored to read from the DB store (FR-REPOS-01) while
// preserving the existing response shape [{name,url,description,primary}]
// (C-INTEG-03). The intake form's consumption of this endpoint is unchanged.

// repoListEntry is the GET /api/repos response shape — preserved exactly
// (C-INTEG-03). The DB row carries more fields (branch, timestamps,
// reference_count) but the public GET contract exposes only these four.
type repoListEntry struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Primary     bool   `json:"primary"`
}

// repoWriteRequest is the POST/PUT /api/repos body. Branch defaults to "main"
// in the store layer when empty (consistent with the seed hook).
type repoWriteRequest struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Branch      string `json:"branch,omitempty"`
	Description string `json:"description,omitempty"`
	Primary     bool   `json:"primary"`
}

// repoWriteResponse is the POST/PUT/DELETE response — the full DB row
// (branch, timestamps, reference_count) so the admin UI can display them.
// This is additive over the GET shape; the GET endpoint keeps its 4-field
// contract for backward compat.
type repoWriteResponse struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	Branch         string `json:"branch"`
	Description    string `json:"description"`
	Primary        bool   `json:"primary"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	ReferenceCount int    `json:"reference_count"`
}

func repoRowToResponse(r db.RepoRow) repoWriteResponse {
	return repoWriteResponse{
		Name:           r.Name,
		URL:            r.URL,
		Branch:         r.Branch,
		Description:    r.Description,
		Primary:        r.Primary,
		CreatedAt:      r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      r.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		ReferenceCount: r.ReferenceCount,
	}
}

// createRepoHandler handles POST /api/repos. Validates, creates via the DB
// store, emits REPOS_REGISTRY_MUTATED, returns 201 with the full row.
// Guarded by AdminGuard (FR-ROUTE-03).
func (s *Server) createRepoHandler(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Database not configured")
		return
	}

	var req repoWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}
	if err := validateRepoWrite(req); err != nil {
		s.emitValidationFailure(r, "repos", err.Error())
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	row, err := s.db.CreateRepo(req.Name, req.URL, req.Branch, req.Description, req.Primary)
	if err != nil {
		if errors.Is(err, db.ErrRepoExists) {
			writeError(w, http.StatusConflict, "repo_exists", "A repo with this name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create repo")
		return
	}

	s.emitReposMutation(r, "create", row.Name, row.URL)
	writeJSON(w, http.StatusCreated, repoRowToResponse(*row))
}

// updateRepoHandler handles PUT /api/repos/{name}. The name is the URL path
// key (immutable natural PK); the body carries the new field values.
// Guarded by AdminGuard.
func (s *Server) updateRepoHandler(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Database not configured")
		return
	}
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Repo name is required in the path")
		return
	}

	var req repoWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}
	// The body's name field is ignored — the path key is authoritative.
	req.Name = name
	if err := validateRepoWrite(req); err != nil {
		s.emitValidationFailure(r, "repos", err.Error())
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	row, err := s.db.UpdateRepo(name, req.URL, req.Branch, req.Description, req.Primary)
	if err != nil {
		if errors.Is(err, db.ErrRepoNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Repo not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update repo")
		return
	}

	s.emitReposMutation(r, "update", row.Name, row.URL)
	writeJSON(w, http.StatusOK, repoRowToResponse(*row))
}

// deleteRepoHandler handles DELETE /api/repos/{name}. Guards against
// deleting a repo referenced by features (returns 409 with the referencing
// feature IDs). Guarded by AdminGuard.
func (s *Server) deleteRepoHandler(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Database not configured")
		return
	}
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Repo name is required in the path")
		return
	}

	// Delete-guard: block if features reference this repo.
	refs, err := s.db.CountRepoReferences(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to check repo references")
		return
	}
	if refs > 0 {
		featureIDs, _ := s.db.ListReferencingFeatures(name)
		if featureIDs == nil {
			featureIDs = []string{}
		}
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"error":    "repo_in_use",
			"details":  "Repo is referenced by existing features and cannot be deleted.",
			"features": featureIDs,
		})
		return
	}

	if err := s.db.DeleteRepo(name); err != nil {
		if errors.Is(err, db.ErrRepoNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Repo not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete repo")
		return
	}

	s.emitReposMutation(r, "delete", name, "")
	w.WriteHeader(http.StatusNoContent)
}

// validateRepoWrite enforces FR-REPOS-05: name required, URL format valid.
// Branch is optional (defaults to "main" in the store). Description is
// optional. Primary is a bool (no validation needed).
func validateRepoWrite(req repoWriteRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(req.URL) == "" {
		return errors.New("url is required")
	}
	return nil
}

// emitReposMutation records a REPOS_REGISTRY_MUTATED audit event with the
// operator identity (ADR-AUDIT-ACTOR-IDENTITY). The actor is derived from
// DEVTEAM_OPERATOR_NAME (default "operator").
func (s *Server) emitReposMutation(r *http.Request, action, name, url string) {
	if s.db == nil {
		return
	}
	details := action + " repo " + name
	if url != "" {
		details += " (" + url + ")"
	}
	_ = s.db.RecordAuditEventWithActor(
		"platform", db.AuditReposRegistryMutated, "", "construction",
		details, currentActor(),
	)
}

// emitValidationFailure records a CONFIG_VALIDATION_FAILED audit event
// (FR-CONFIG-03). The details carry the category and error message — no
// secret values (repos has no secrets).
func (s *Server) emitValidationFailure(r *http.Request, category, message string) {
	if s.db == nil {
		return
	}
	_ = s.db.RecordAuditEventWithActor(
		"platform", db.AuditConfigValidationFailed, "", "construction",
		category+": "+message, currentActor(),
	)
}

// currentActor returns the operator identity for audit events. In
// single-operator v1 this is the DEVTEAM_OPERATOR_NAME env var, defaulting
// to "operator" (ADR-AUDIT-ACTOR-IDENTITY).
func currentActor() string {
	actor := os.Getenv("DEVTEAM_OPERATOR_NAME")
	if actor == "" {
		actor = "operator"
	}
	return actor
}

// ─── Settings/defaults endpoints (Bolt 2) ───
//
// GET /api/settings/defaults returns {global: {...}, per_repo: [{repo, ...}]}.
// PUT /api/settings/defaults/global upserts the global defaults.
// PUT /api/settings/defaults/{repo} upserts a per-repo override.
// DELETE /api/settings/defaults/{repo} removes a per-repo override.
//
// Reads are unguarded (FR-ROUTE-02); writes are guarded by AdminGuard
// (FR-ROUTE-03). Every mutation emits FEATURE_DEFAULTS_MUTATED (FR-DEF-05).

// defaultsResponse is the GET /api/settings/defaults response shape.
// per_repo is always a non-nil slice (empty-array-not-null invariant).
type defaultsResponse struct {
	Global  defaultsDefaultsDTO   `json:"global"`
	PerRepo []defaultsDefaultsDTO `json:"per_repo"`
}

// defaultsDefaultsDTO is the JSON shape for a single defaults row.
type defaultsDefaultsDTO struct {
	Scope         string `json:"scope,omitempty"`
	Depth         string `json:"depth,omitempty"`
	TestStrategy  string `json:"test_strategy,omitempty"`
	ExecutionMode string `json:"execution_mode,omitempty"`
	Repo          string `json:"repo,omitempty"`
}

// defaultsPutRequest is the PUT body for global/per-repo defaults.
type defaultsPutRequest struct {
	Scope         string `json:"scope,omitempty"`
	Depth         string `json:"depth,omitempty"`
	TestStrategy  string `json:"test_strategy,omitempty"`
	ExecutionMode string `json:"execution_mode,omitempty"`
}

func (s *Server) getDefaultsHandler(w http.ResponseWriter, r *http.Request) {
	if s.db == nil || s.defaultsStore == nil {
		writeJSON(w, http.StatusOK, defaultsResponse{PerRepo: []defaultsDefaultsDTO{}})
		return
	}
	g, err := s.defaultsStore.GetGlobal(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, defaultsResponse{PerRepo: []defaultsDefaultsDTO{}})
		return
	}
	perRepo, err := s.defaultsStore.ListPerRepo(r.Context())
	if err != nil {
		perRepo = nil
	}
	resp := defaultsResponse{
		Global: defaultsDefaultsDTO{
			Scope:         g.Scope,
			Depth:         g.Depth,
			TestStrategy:  g.TestStrategy,
			ExecutionMode: g.ExecutionMode,
		},
		PerRepo: []defaultsDefaultsDTO{},
	}
	for _, d := range perRepo {
		resp.PerRepo = append(resp.PerRepo, defaultsDefaultsDTO{
			Scope:         d.Scope,
			Depth:         d.Depth,
			TestStrategy:  d.TestStrategy,
			ExecutionMode: d.ExecutionMode,
			Repo:          d.Repo,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) putGlobalDefaultsHandler(w http.ResponseWriter, r *http.Request) {
	if s.db == nil || s.defaultsStore == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Database not configured")
		return
	}
	var req defaultsPutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.emitValidationFailure(r, "defaults", "Invalid JSON body")
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}
	got, err := s.defaultsStore.PutGlobal(r.Context(), defaults.Defaults{
		Scope:         req.Scope,
		Depth:         req.Depth,
		TestStrategy:  req.TestStrategy,
		ExecutionMode: req.ExecutionMode,
	}, currentActor())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to save global defaults")
		return
	}
	writeJSON(w, http.StatusOK, defaultsDefaultsDTO{
		Scope:         got.Scope,
		Depth:         got.Depth,
		TestStrategy:  got.TestStrategy,
		ExecutionMode: got.ExecutionMode,
	})
}

func (s *Server) putRepoDefaultsHandler(w http.ResponseWriter, r *http.Request) {
	if s.db == nil || s.defaultsStore == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Database not configured")
		return
	}
	repo := r.PathValue("repo")
	if repo == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Repo name is required in the path")
		return
	}
	var req defaultsPutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.emitValidationFailure(r, "defaults", "Invalid JSON body")
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}
	got, err := s.defaultsStore.PutForRepo(r.Context(), repo, defaults.Defaults{
		Scope:         req.Scope,
		Depth:         req.Depth,
		TestStrategy:  req.TestStrategy,
		ExecutionMode: req.ExecutionMode,
	}, currentActor())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to save per-repo defaults")
		return
	}
	writeJSON(w, http.StatusOK, defaultsDefaultsDTO{
		Scope:         got.Scope,
		Depth:         got.Depth,
		TestStrategy:  got.TestStrategy,
		ExecutionMode: got.ExecutionMode,
		Repo:          got.Repo,
	})
}

func (s *Server) deleteRepoDefaultsHandler(w http.ResponseWriter, r *http.Request) {
	if s.db == nil || s.defaultsStore == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Database not configured")
		return
	}
	repo := r.PathValue("repo")
	if repo == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Repo name is required in the path")
		return
	}
	err := s.defaultsStore.DeleteForRepo(r.Context(), repo, currentActor())
	if err != nil {
		if errors.Is(err, defaults.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "No per-repo defaults for this repo")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete per-repo defaults")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}