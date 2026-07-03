package api

import (
	"context"
	"time"

	"github.com/MichielDean/devteam/internal/feature"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

type CreateFeatureRequest struct {
	Type             string            `json:"type"`
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	Priority         int               `json:"priority"`
	FileContent      string            `json:"file_content,omitempty"`
	StartImmediately bool              `json:"start_immediately,omitempty"`
	Scope            string            `json:"scope,omitempty"`
	Depth            string            `json:"depth,omitempty"`
	TestStrategy     string            `json:"test_strategy,omitempty"`
	Repos            []feature.RepoRef `json:"repos,omitempty"`
}

type RecirculateRequest struct {
	TargetPhase string `json:"target_phase"`
}

type FeatureSummaryResponse struct {
	ID                    string `json:"id"`
	Title                 string `json:"title"`
	Status                string `json:"status"`
	Priority              int    `json:"priority"`
	CurrentPhase          string `json:"current_phase"`
	Scope                 string `json:"scope,omitempty"`
	CurrentStage          string `json:"current_stage,omitempty"`
	UpdatedAt             time.Time `json:"updated_at"`
	PendingQuestionsCount int    `json:"pending_questions_count"`
}

type FeatureDetailResponse struct {
	ID             string            `json:"id"`
	Title          string            `json:"title"`
	Status         string            `json:"status"`
	Priority       int               `json:"priority"`
	IntakePath     string            `json:"intake_path"`
	CurrentPhase   string            `json:"current_phase"`
	Scope          string            `json:"scope,omitempty"`
	Depth          string            `json:"depth,omitempty"`
	TestStrategy   string            `json:"test_strategy,omitempty"`
	AutonomyMode   string            `json:"autonomy_mode,omitempty"`
	CurrentStage   string            `json:"current_stage,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	Dependencies   []string          `json:"dependencies"`
	Repos          []RepoRefResponse `json:"repos"`
	IsProcessing   bool              `json:"is_processing"`
	ProcessingMode string            `json:"processing_mode,omitempty"`
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
	return FeatureSummaryResponse{
		ID:           f.ID,
		Title:        f.Title,
		Status:       string(f.Status),
		Priority:     f.Priority,
		CurrentPhase: f.CurrentPhase(),
		Scope:        f.Scope,
		CurrentStage: f.CurrentStage,
		UpdatedAt:    f.UpdatedAt,
	}
}

func FeatureToDetailResponse(f *feature.Feature, isProcessing bool, processingMode string) FeatureDetailResponse {
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
		ID:             f.ID,
		Title:          f.Title,
		Status:         string(f.Status),
		Priority:       f.Priority,
		IntakePath:     string(f.IntakePath),
		CurrentPhase:   f.CurrentPhase(),
		Scope:          f.Scope,
		Depth:          f.Depth,
		TestStrategy:   f.TestStrategy,
		AutonomyMode:   f.AutonomyMode,
		CurrentStage:   f.CurrentStage,
		CreatedAt:      f.CreatedAt,
		UpdatedAt:      f.UpdatedAt,
		Dependencies:   deps,
		Repos:          repos,
		IsProcessing:   isProcessing,
		ProcessingMode: processingMode,
	}
}

type QuestionResponse struct {
	ID         string   `json:"id"`
	FeatureID  string   `json:"feature_id"`
	Phase      string   `json:"phase"`
	Role       string   `json:"role"`
	StageID    string   `json:"stage_id"`
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
		StageID:    q.StageID,
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


