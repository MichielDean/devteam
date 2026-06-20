package api

import (
	"encoding/json"
	"time"

	"github.com/MichielDean/devteam/internal/feature"
)

// Request DTOs

// CreateFeatureRequest represents the JSON body for POST /api/features
type CreateFeatureRequest struct {
	Type        string `json:"type"`         // "loose_idea" or "external_spec"
	Title       string `json:"title"`        // Required, max 200 chars
	Description string `json:"description"`  // Required for loose_idea, max 10000 chars
	Priority    int    `json:"priority"`      // 1, 2, or 3 (default 2)
	FileContent string `json:"file_content"` // base64-encoded file content for external_spec
}

// RecirculateRequest represents the JSON body for POST /api/features/:id/recirculate
type RecirculateRequest struct {
	TargetPhase string `json:"target_phase"` // Must be a valid phase earlier than current
}

// Response DTOs

// FeatureListResponse is the response for GET /api/features
type FeatureListResponse struct {
	Features []FeatureSummary `json:"features"`
}

// FeatureSummary is a compact feature representation for list views
type FeatureSummary struct {
	ID           string             `json:"id"`
	Title        string             `json:"title"`
	Status       string             `json:"status"`
	Priority     int                `json:"priority"`
	CurrentPhase string             `json:"current_phase"`
	UpdatedAt    time.Time          `json:"updated_at"`
	GateResult   *GateResultResponse `json:"gate_result,omitempty"`
}

// FeatureDetailResponse is the full feature representation for detail views
type FeatureDetailResponse struct {
	ID            string                        `json:"id"`
	Title         string                        `json:"title"`
	Status        string                        `json:"status"`
	Priority      int                           `json:"priority"`
	IntakePath    string                        `json:"intake_path"`
	CreatedAt     time.Time                     `json:"created_at"`
	UpdatedAt     time.Time                     `json:"updated_at"`
	PhaseStates   map[string]PhaseStateResponse `json:"phase_states"`
	Dependencies  []string                      `json:"dependencies,omitempty"`
	Repos         []RepoRefResponse             `json:"repos,omitempty"`
}

// PhaseStateResponse represents a single phase state in the feature detail
type PhaseStateResponse struct {
	Phase       string              `json:"phase"`
	Status      string              `json:"status"`
	StartedAt   *time.Time          `json:"started_at,omitempty"`
	CompletedAt *time.Time          `json:"completed_at,omitempty"`
	Artifacts   []ArtifactResponse  `json:"artifacts,omitempty"`
	GateResult  *GateResultResponse `json:"gate_result,omitempty"`
}

// ArtifactResponse represents an artifact in the API response
type ArtifactResponse struct {
	Type        string    `json:"type"`
	Path        string    `json:"path"`
	GeneratedBy string    `json:"generated_by"`
	GeneratedAt time.Time `json:"generated_at"`
}

// GateResultResponse represents a gate evaluation result
type GateResultResponse struct {
	Phase   string               `json:"phase"`
	Passed  bool                 `json:"passed"`
	Checks  []CheckResultResponse `json:"checks,omitempty"`
}

// CheckResultResponse represents a single gate check result
type CheckResultResponse struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// RepoRefResponse represents a repository reference
type RepoRefResponse struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Branch string `json:"branch"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// SSE Event DTOs

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// PhaseChangeEvent represents a phase transition SSE event
type PhaseChangeEvent struct {
	FeatureID string    `json:"feature_id"`
	Phase     string    `json:"phase"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// GateResultEvent represents a gate evaluation SSE event
type GateResultEvent struct {
	FeatureID string               `json:"feature_id"`
	Phase     string               `json:"phase"`
	Passed    bool                 `json:"passed"`
	Checks    []CheckResultResponse `json:"checks,omitempty"`
}

// AgentDispatchEvent represents an agent dispatch SSE event
type AgentDispatchEvent struct {
	FeatureID string    `json:"feature_id"`
	Phase     string    `json:"phase"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// AgentCompleteEvent represents an agent completion SSE event
type AgentCompleteEvent struct {
	FeatureID  string    `json:"feature_id"`
	Phase      string    `json:"phase"`
	Role       string    `json:"role"`
	Status     string    `json:"status"`
	DurationMs int64     `json:"duration_ms"`
}

// ProcessingCompleteEvent represents a processing completion SSE event
type ProcessingCompleteEvent struct {
	FeatureID string    `json:"feature_id"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// ErrorEvent represents an error SSE event
type ErrorEvent struct {
	FeatureID string    `json:"feature_id"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Conversion functions

// FeatureToDetailResponse converts a feature.Feature to FeatureDetailResponse
func FeatureToDetailResponse(f *feature.Feature) FeatureDetailResponse {
	phaseStates := make(map[string]PhaseStateResponse)
	for phase, ps := range f.PhaseStates {
		phaseStates[string(phase)] = PhaseStateResponse{
			Phase:       string(ps.Phase),
			Status:      string(ps.Status),
			StartedAt:   ps.StartedAt,
			CompletedAt: ps.CompletedAt,
			Artifacts:   artifactsToResponse(ps.Artifacts),
			GateResult:  gateResultToResponse(ps.GateResult),
		}
	}

	deps := f.Dependencies
	if deps == nil {
		deps = []string{}
	}

	repos := reposToResponse(f.Repos)
	if repos == nil {
		repos = []RepoRefResponse{}
	}

	return FeatureDetailResponse{
		ID:           f.ID,
		Title:        f.Title,
		Status:       string(f.Status),
		Priority:     f.Priority,
		IntakePath:   string(f.IntakePath),
		CreatedAt:    f.CreatedAt,
		UpdatedAt:    f.UpdatedAt,
		PhaseStates:  phaseStates,
		Dependencies: deps,
		Repos:        repos,
	}
}

// FeatureToSummaryResponse converts a feature.Feature to FeatureSummary
func FeatureToSummaryResponse(f *feature.Feature) FeatureSummary {
	// Find the gate result for the current phase
	var gateResult *GateResultResponse
	if ps, ok := f.PhaseStates[f.Current]; ok && ps.GateResult != nil {
		gateResult = gateResultToResponse(ps.GateResult)
	}

	return FeatureSummary{
		ID:           f.ID,
		Title:        f.Title,
		Status:       string(f.Status),
		Priority:     f.Priority,
		CurrentPhase: string(f.Current),
		UpdatedAt:    f.UpdatedAt,
		GateResult:   gateResult,
	}
}

// FeaturesToSummaryResponse converts a slice of features to FeatureListResponse
func FeaturesToSummaryResponse(features []*feature.Feature) FeatureListResponse {
	summaries := make([]FeatureSummary, 0, len(features))
	for _, f := range features {
		summaries = append(summaries, FeatureToSummaryResponse(f))
	}
	return FeatureListResponse{Features: summaries}
}

// GateResultToResponse converts a feature.GateResult to GateResultResponse
func GateResultToResponse(gr *feature.GateResult) GateResultResponse {
	if gr == nil {
		return GateResultResponse{}
	}
	return *gateResultToResponse(gr)
}

func artifactsToResponse(artifacts []feature.Artifact) []ArtifactResponse {
	if artifacts == nil {
		return []ArtifactResponse{}
	}
	result := make([]ArtifactResponse, 0, len(artifacts))
	for _, a := range artifacts {
		result = append(result, ArtifactResponse{
			Type:        string(a.Type),
			Path:        a.Path,
			GeneratedBy: string(a.GeneratedBy),
			GeneratedAt: a.GeneratedAt,
		})
	}
	return result
}

func gateResultToResponse(gr *feature.GateResult) *GateResultResponse {
	if gr == nil {
		return nil
	}
	checks := make([]CheckResultResponse, 0, len(gr.Checks))
	for _, c := range gr.Checks {
		checks = append(checks, CheckResultResponse{
			Name:    c.Name,
			Passed:  c.Passed,
			Message: c.Message,
		})
	}
	return &GateResultResponse{
		Phase:  string(gr.Phase),
		Passed: gr.Passed,
		Checks: checks,
	}
}

func reposToResponse(repos []feature.RepoRef) []RepoRefResponse {
	if repos == nil {
		return []RepoRefResponse{}
	}
	result := make([]RepoRefResponse, 0, len(repos))
	for _, r := range repos {
		result = append(result, RepoRefResponse{
			Name:   r.Name,
			URL:    r.URL,
			Branch: r.Branch,
		})
	}
	return result
}