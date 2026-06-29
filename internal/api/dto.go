package api

import (
	"context"
	"time"

	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/pipeline"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

type CreateFeatureRequest struct {
	Type            string `json:"type"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Priority        int    `json:"priority"`
	FileContent     string `json:"file_content,omitempty"`
	StartImmediately bool  `json:"start_immediately,omitempty"`
}

type RecirculateRequest struct {
	TargetPhase string `json:"target_phase"`
}

// UpdateFeatureRequest is the PATCH /api/features/{id} body. Only title and
// priority are editable; id/current_phase/status/intake_path/spec_dir are
// immutable and never accepted here (mass-assignment prevention). Omitted
// fields are not zeroed — pointers distinguish "absent" from "set to zero".
type UpdateFeatureRequest struct {
	Title    *string `json:"title,omitempty"`
	Priority *int    `json:"priority,omitempty"`
}

type FeatureSummaryResponse struct {
	ID                    string              `json:"id"`
	Title                 string              `json:"title"`
	Status                string              `json:"status"`
	Priority              int                 `json:"priority"`
	CurrentPhase          string              `json:"current_phase"`
	UpdatedAt             time.Time           `json:"updated_at"`
	GateResult            *GateResultResponse `json:"gate_result,omitempty"`
	PendingQuestionsCount int                 `json:"pending_questions_count"`
}

type FeatureDetailResponse struct {
	ID           string                        `json:"id"`
	Title        string                        `json:"title"`
	Status       string                        `json:"status"`
	Priority     int                           `json:"priority"`
	IntakePath   string                        `json:"intake_path"`
	CurrentPhase string                        `json:"current_phase"`
	CreatedAt    time.Time                     `json:"created_at"`
	UpdatedAt    time.Time                     `json:"updated_at"`
	PhaseStates  map[string]PhaseStateResponse `json:"phase_states"`
	Dependencies []string                      `json:"dependencies"`
	Repos        []RepoRefResponse             `json:"repos"`
	IsProcessing bool                          `json:"is_processing"`
	ProcessingMode string                      `json:"processing_mode,omitempty"`
}

type PhaseStateResponse struct {
	Phase       string              `json:"phase"`
	Status      string              `json:"status"`
	StartedAt   *time.Time          `json:"started_at,omitempty"`
	CompletedAt *time.Time          `json:"completed_at,omitempty"`
	Artifacts   []ArtifactResponse  `json:"artifacts"`
	GateResult  *GateResultResponse `json:"gate_result,omitempty"`
}

type ArtifactResponse struct {
	Type        string    `json:"type"`
	Path        string    `json:"path"`
	GeneratedBy string    `json:"generated_by"`
	GeneratedAt time.Time `json:"generated_at"`
}

type GateResultResponse struct {
	Phase       string                `json:"phase"`
	Passed      bool                  `json:"passed"`
	MissingArts []string              `json:"missing_arts"`
	Checks      []CheckResultResponse `json:"checks"`
	EvaluatedAt time.Time             `json:"evaluated_at"`
}

type CheckResultResponse struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

type RepoRefResponse struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Branch string `json:"branch"`
}

func FeaturesToSummaryResponse(features []*feature.Feature, questionStore feature.QuestionStore) map[string]interface{} {
	summaries := make([]FeatureSummaryResponse, 0, len(features))
	for _, f := range features {
		summary := FeatureToSummaryResponse(f)

		// Populate pending questions count
		if questionStore != nil {
			count, err := questionStore.PendingCount(context.Background(), f.ID)
			if err != nil {
				count = 0
			}
			summary.PendingQuestionsCount = count
		}

		summaries = append(summaries, summary)
	}
	return map[string]interface{}{"features": summaries, "total_count": len(summaries)}
}

func FeatureToSummaryResponse(f *feature.Feature) FeatureSummaryResponse {
	resp := FeatureSummaryResponse{
		ID:           f.ID,
		Title:        f.Title,
		Status:       string(f.Status),
		Priority:     f.Priority,
		CurrentPhase: string(f.Current),
		UpdatedAt:    f.UpdatedAt,
	}
	if ps, ok := f.PhaseStates[f.Current]; ok && ps != nil && ps.GateResult != nil {
		gr := GateResultToResponse(ps.GateResult)
		resp.GateResult = &gr
	}
	return resp
}

func FeatureToDetailResponse(f *feature.Feature, isProcessing bool, processingMode string) FeatureDetailResponse {
	phaseStates := make(map[string]PhaseStateResponse)
	for phase, ps := range f.PhaseStates {
		resp := PhaseStateResponse{
			Phase:       string(ps.Phase),
			Status:      string(ps.Status),
			StartedAt:   ps.StartedAt,
			CompletedAt: ps.CompletedAt,
			Artifacts:   []ArtifactResponse{},
		}
		for _, a := range ps.Artifacts {
			resp.Artifacts = append(resp.Artifacts, ArtifactResponse{
				Type:        string(a.Type),
				Path:        a.Path,
				GeneratedBy: string(a.GeneratedBy),
				GeneratedAt: a.GeneratedAt,
			})
		}
		if ps.GateResult != nil {
			gr := GateResultToResponse(ps.GateResult)
			resp.GateResult = &gr
		}
		phaseStates[string(phase)] = resp
	}

	deps := f.Dependencies
	if deps == nil {
		deps = []string{}
	}

	repos := make([]RepoRefResponse, 0, len(f.Repos))
	for _, r := range f.Repos {
		repos = append(repos, RepoRefResponse{
			Name:   r.Name,
			URL:    r.URL,
			Branch: r.Branch,
		})
	}

	return FeatureDetailResponse{
		ID:           f.ID,
		Title:        f.Title,
		Status:       string(f.Status),
		Priority:     f.Priority,
		IntakePath:   string(f.IntakePath),
		CurrentPhase: string(f.Current),
		CreatedAt:    f.CreatedAt,
		UpdatedAt:    f.UpdatedAt,
		PhaseStates:  phaseStates,
		Dependencies: deps,
		Repos:        repos,
		IsProcessing: isProcessing,
		ProcessingMode: processingMode,
	}
}

func GateResultToResponse(gr *feature.GateResult) GateResultResponse {
	if gr == nil {
		return GateResultResponse{Checks: []CheckResultResponse{}}
	}
	checks := make([]CheckResultResponse, 0, len(gr.Checks))
	for _, c := range gr.Checks {
		checks = append(checks, CheckResultResponse{
			Name:    c.Name,
			Passed:  c.Passed,
			Message: c.Message,
		})
	}
	missingArts := gr.MissingArts
	if missingArts == nil {
		missingArts = []string{}
	}
	return GateResultResponse{
		Phase:       string(gr.Phase),
		Passed:      gr.Passed,
		MissingArts: missingArts,
		Checks:      checks,
		EvaluatedAt: gr.EvaluatedAt,
	}
}

type QuestionResponse struct {
	ID         string   `json:"id"`
	FeatureID  string   `json:"feature_id"`
	Phase      string   `json:"phase"`
	Role       string   `json:"role"`
	Question   string   `json:"question"`
	Type       string   `json:"type"`
	Options    []string `json:"options"`
	Answer     *string  `json:"answer"`
	Assumption *string  `json:"assumption"`
	Status     string   `json:"status"`
	CreatedAt  string   `json:"created_at"`
	AnsweredAt *string  `json:"answered_at"`
}

type CreateQuestionRequest struct {
	Phase    string   `json:"phase"`
	Role     string   `json:"role"`
	Question string   `json:"question"`
	Type     string   `json:"type"`
	Options  []string `json:"options"`
}

type AnswerQuestionRequest struct {
	Answer string `json:"answer"`
}

func QuestionToResponse(q *feature.Question) QuestionResponse {
	var options []string
	if q.Options == nil {
		options = []string{}
	} else {
		options = q.Options
	}

	var answeredAt *string
	if q.AnsweredAt != nil {
		s := q.AnsweredAt.Format(time.RFC3339)
		answeredAt = &s
	}

	return QuestionResponse{
		ID:         q.ID,
		FeatureID:  q.FeatureID,
		Phase:      q.Phase,
		Role:       q.Role,
		Question:   q.Question,
		Type:       q.Type,
		Options:    options,
		Answer:     q.Answer,
		Assumption: q.Assumption,
		Status:     q.Status,
		CreatedAt:  q.CreatedAt.Format(time.RFC3339),
		AnsweredAt: answeredAt,
	}
}

func QuestionsToResponse(questions []*feature.Question) []QuestionResponse {
	if questions == nil {
		return []QuestionResponse{}
	}
	result := make([]QuestionResponse, 0, len(questions))
	for _, q := range questions {
		result = append(result, QuestionToResponse(q))
	}
	return result
}

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
