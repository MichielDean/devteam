package feature

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Question statuses
const (
	QuestionStatusPending  = "pending"
	QuestionStatusAnswered = "answered"
	QuestionStatusAssumed  = "assumed"
)

// Valid question phases (AIDLC v2: ideation and inception stages can ask questions)
var ValidQuestionPhases = map[string]bool{
	"ideation":   true,
	"inception":  true,
}

// Valid question roles (AIDLC v2: any agent can ask questions)
var ValidQuestionRoles = map[string]bool{
	"product":    true,
	"architect":  true,
	"design":     true,
	"delivery":   true,
	"developer":  true,
	"platform":   true,
	"devsecops":  true,
	"quality":    true,
	"pipeline-deploy": true,
	"operations": true,
}

// Valid question types
var ValidQuestionTypes = map[string]bool{
	"clarification":    true,
	"decision":         true,
	"priority":         true,
	"multiple_choice":  true,
	"open_ended":       true,
}

// Question represents a clarification or decision question surfaced by an agent
// during inception or planning phases for human input.
type Question struct {
	ID         string     `json:"id" yaml:"id"`
	FeatureID  string     `json:"feature_id" yaml:"feature_id"`
	Phase      string     `json:"phase" yaml:"phase"`
	Role       string     `json:"role" yaml:"role"`
	Question   string     `json:"question" yaml:"question"`
	Type       string     `json:"type" yaml:"type"`
	Options    []string   `json:"options" yaml:"options"`
	Answer     *string    `json:"answer" yaml:"answer"`
	Assumption *string    `json:"assumption" yaml:"assumption"`
	Status     string     `json:"status" yaml:"status"`
	CreatedAt  time.Time  `json:"created_at" yaml:"created_at"`
	AnsweredAt *time.Time `json:"answered_at" yaml:"answered_at"`
}

// ValidateQuestion validates a question's fields and returns an error description if invalid.
func ValidateQuestion(q *Question) string {
	if q.Phase == "" || !ValidQuestionPhases[q.Phase] {
		return "phase must be one of: ideation, inception"
	}
	if q.Role == "" || !ValidQuestionRoles[q.Role] {
		return "role must be one of: product, architect, design, delivery, developer, platform, devsecops, quality, pipeline-deploy, operations"
	}
	qText := strings.TrimSpace(q.Question)
	if qText == "" {
		return "question is required"
	}
	if len(qText) > 2000 {
		return "question must be 1-2000 characters"
	}
	if q.Type == "" || !ValidQuestionTypes[q.Type] {
		return "type must be one of: clarification, decision, priority"
	}
	if len(q.Options) > 10 {
		return "options must have at most 10 items"
	}
	for _, opt := range q.Options {
		if len(opt) > 500 {
			return "each option must be 1-500 characters"
		}
	}
	return ""
}

// QuestionStore defines the interface for question persistence.
type QuestionStore interface {
	CreateQuestion(ctx context.Context, featureID string, q Question) (*Question, error)
	GetQuestion(ctx context.Context, featureID string, questionID string) (*Question, error)
	ListQuestions(ctx context.Context, featureID string) ([]*Question, error)
	ListPendingQuestions(ctx context.Context, featureID string) ([]*Question, error)
	AnswerQuestion(ctx context.Context, featureID string, questionID string, answer string) (*Question, error)
	AssumeQuestion(ctx context.Context, featureID string, questionID string, assumption string) (*Question, error)
	DeleteQuestionsForFeature(ctx context.Context, featureID string) error
	PendingCount(ctx context.Context, featureID string) (int, error)
}

// FileQuestionStore implements QuestionStore using JSON files on disk.
type FileQuestionStore struct {
	baseDir string
	mu      sync.Mutex
}

// NewFileQuestionStore creates a new FileQuestionStore rooted at baseDir.
func NewFileQuestionStore(baseDir string) *FileQuestionStore {
	return &FileQuestionStore{baseDir: baseDir}
}

func (s *FileQuestionStore) questionsPath(featureID string) string {
	// Check if this feature has a worktree by loading its state file
	statePath := filepath.Join(s.baseDir, "specs", featureID, ".devteam-state.yaml")
	if data, err := os.ReadFile(statePath); err == nil {
		var f Feature
		if err := yaml.Unmarshal(data, &f); err == nil && f.WorktreeDir != "" {
			return filepath.Join(f.WorktreeDir, "specs", featureID, "questions.json")
		}
	}
	return filepath.Join(s.baseDir, "specs", featureID, "questions.json")
}

func (s *FileQuestionStore) loadQuestions(featureID string) ([]*Question, error) {
	path := s.questionsPath(featureID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Question{}, nil
		}
		return nil, fmt.Errorf("reading questions file: %w", err)
	}
	if len(data) == 0 {
		return []*Question{}, nil
	}
	var questions []*Question
	if err := json.Unmarshal(data, &questions); err != nil {
		return nil, fmt.Errorf("parsing questions file: %w", err)
	}
	return questions, nil
}

func (s *FileQuestionStore) saveQuestions(featureID string, questions []*Question) error {
	dir := filepath.Dir(s.questionsPath(featureID))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating questions directory: %w", err)
	}

	data, err := json.MarshalIndent(questions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling questions: %w", err)
	}

	// Write to temp file then rename for atomicity
	tmpPath := s.questionsPath(featureID) + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing questions temp file: %w", err)
	}
	if err := os.Rename(tmpPath, s.questionsPath(featureID)); err != nil {
		return fmt.Errorf("renaming questions file: %w", err)
	}
	return nil
}

// nextQuestionID generates the next Q-NNN ID for a feature.
func nextQuestionID(questions []*Question) string {
	maxNum := 0
	for _, q := range questions {
		var num int
		if _, err := fmt.Sscanf(q.ID, "Q-%d", &num); err == nil {
			if num > maxNum {
				maxNum = num
			}
		}
	}
	return fmt.Sprintf("Q-%03d", maxNum+1)
}

func (s *FileQuestionStore) CreateQuestion(ctx context.Context, featureID string, q Question) (*Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	questions, err := s.loadQuestions(featureID)
	if err != nil {
		return nil, fmt.Errorf("loading questions: %w", err)
	}

	// Set auto-generated fields
	q.ID = nextQuestionID(questions)
	q.FeatureID = featureID
	q.Status = QuestionStatusPending
	q.CreatedAt = time.Now()
	q.Answer = nil
	q.Assumption = nil
	q.AnsweredAt = nil

	// Ensure Options is never nil (empty slice, not null)
	if q.Options == nil {
		q.Options = []string{}
	}

	questions = append(questions, &q)

	if err := s.saveQuestions(featureID, questions); err != nil {
		return nil, fmt.Errorf("saving questions: %w", err)
	}

	return &q, nil
}

func (s *FileQuestionStore) GetQuestion(ctx context.Context, featureID string, questionID string) (*Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	questions, err := s.loadQuestions(featureID)
	if err != nil {
		return nil, fmt.Errorf("loading questions: %w", err)
	}

	for _, q := range questions {
		if q.ID == questionID {
			return q, nil
		}
	}
	return nil, fmt.Errorf("question %s not found", questionID)
}

func (s *FileQuestionStore) ListQuestions(ctx context.Context, featureID string) ([]*Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	questions, err := s.loadQuestions(featureID)
	if err != nil {
		return nil, fmt.Errorf("loading questions: %w", err)
	}

	// Ensure we never return nil — always an empty slice
	if questions == nil {
		questions = []*Question{}
	}
	return questions, nil
}

func (s *FileQuestionStore) ListPendingQuestions(ctx context.Context, featureID string) ([]*Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	questions, err := s.loadQuestions(featureID)
	if err != nil {
		return nil, fmt.Errorf("loading questions: %w", err)
	}

	var pending []*Question
	for _, q := range questions {
		if q.Status == QuestionStatusPending {
			pending = append(pending, q)
		}
	}

	// Ensure we never return nil — always an empty slice
	if pending == nil {
		pending = []*Question{}
	}
	return pending, nil
}

func (s *FileQuestionStore) AnswerQuestion(ctx context.Context, featureID string, questionID string, answer string) (*Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	questions, err := s.loadQuestions(featureID)
	if err != nil {
		return nil, fmt.Errorf("loading questions: %w", err)
	}

	for _, q := range questions {
		if q.ID == questionID {
			if q.Status != QuestionStatusPending {
				return nil, &QuestionConflictError{QuestionID: questionID, Status: q.Status}
			}
			q.Answer = &answer
			q.Status = QuestionStatusAnswered
			now := time.Now()
			q.AnsweredAt = &now
			if err := s.saveQuestions(featureID, questions); err != nil {
				return nil, fmt.Errorf("saving questions: %w", err)
			}
			return q, nil
		}
	}
	return nil, fmt.Errorf("question %s not found", questionID)
}

func (s *FileQuestionStore) AssumeQuestion(ctx context.Context, featureID string, questionID string, assumption string) (*Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	questions, err := s.loadQuestions(featureID)
	if err != nil {
		return nil, fmt.Errorf("loading questions: %w", err)
	}

	for _, q := range questions {
		if q.ID == questionID {
			if q.Status != QuestionStatusPending {
				return nil, &QuestionConflictError{QuestionID: questionID, Status: q.Status}
			}
			q.Assumption = &assumption
			q.Status = QuestionStatusAssumed
			now := time.Now()
			q.AnsweredAt = &now
			if err := s.saveQuestions(featureID, questions); err != nil {
				return nil, fmt.Errorf("saving questions: %w", err)
			}
			return q, nil
		}
	}
	return nil, fmt.Errorf("question %s not found", questionID)
}

func (s *FileQuestionStore) DeleteQuestionsForFeature(ctx context.Context, featureID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.questionsPath(featureID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting questions file: %w", err)
	}
	return nil
}

func (s *FileQuestionStore) PendingCount(ctx context.Context, featureID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	questions, err := s.loadQuestions(featureID)
	if err != nil {
		return 0, fmt.Errorf("loading questions: %w", err)
	}

	count := 0
	for _, q := range questions {
		if q.Status == QuestionStatusPending {
			count++
		}
	}
	return count, nil
}

// QuestionConflictError is returned when trying to answer/assume a question that is already answered or assumed.
type QuestionConflictError struct {
	QuestionID string
	Status     string
}

func (e *QuestionConflictError) Error() string {
	return fmt.Sprintf("Question %s is already %s", e.QuestionID, e.Status)
}

// DetectQuestions reads a questions.json file from the spec directory and returns valid questions.
// Invalid questions are skipped with a warning logged.
func DetectQuestions(featureID, specDir string) []Question {
	path := filepath.Join(specDir, "questions.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("warning: error reading questions.json for feature %s: %v", featureID, err)
		}
		return nil
	}

	if len(data) == 0 {
		return nil
	}

	var rawQuestions []struct {
		Phase    string   `json:"phase"`
		Role     string   `json:"role"`
		Question string   `json:"question"`
		Type     string   `json:"type"`
		Options  []string `json:"options"`
	}

	if err := json.Unmarshal(data, &rawQuestions); err != nil {
		log.Printf("warning: invalid JSON in questions.json for feature %s: %v", featureID, err)
		return nil
	}

	var validQuestions []Question
	for i, rq := range rawQuestions {
		q := Question{
			FeatureID: featureID,
			Phase:     rq.Phase,
			Role:      rq.Role,
			Question:  rq.Question,
			Type:      rq.Type,
			Options:   rq.Options,
		}
		if q.Options == nil {
			q.Options = []string{}
		}

		if errMsg := ValidateQuestion(&q); errMsg != "" {
			log.Printf("warning: skipping invalid question %d in questions.json for feature %s: %s", i+1, featureID, errMsg)
			continue
		}

		validQuestions = append(validQuestions, q)
	}

	return validQuestions
}

// AssumeAllPendingQuestions marks all pending questions as assumed for a feature.
// It returns the list of assumed questions.
func AssumeAllPendingQuestions(store QuestionStore, featureID string, timeoutMinutes int) ([]*Question, error) {
	ctx := context.Background()
	questions, err := store.ListPendingQuestions(ctx, featureID)
	if err != nil {
		return nil, fmt.Errorf("listing pending questions: %w", err)
	}

	var assumed []*Question
	for _, q := range questions {
		assumptionText := GenerateAssumptionText(q, timeoutMinutes)
		updated, err := store.AssumeQuestion(ctx, featureID, q.ID, assumptionText)
		if err != nil {
			log.Printf("warning: failed to assume question %s: %v", q.ID, err)
			continue
		}
		assumed = append(assumed, updated)
	}

	return assumed, nil
}

// GenerateAssumptionText creates an assumption text for a question that timed out.
func GenerateAssumptionText(q *Question, timeoutMinutes int) string {
	if len(q.Options) > 0 {
		return fmt.Sprintf("No human response received. Assuming: %s", q.Options[0])
	}
	return "No human response received. This question was auto-assumed."
}

// BuildHumanResponsesContext builds the "Human Responses" section for agent context.
func BuildHumanResponsesContext(questions []*Question, timeoutMinutes int) string {
	if len(questions) == 0 {
		return ""
	}

	// Check if there are any answered or assumed questions
	hasResponses := false
	for _, q := range questions {
		if q.Status == QuestionStatusAnswered || q.Status == QuestionStatusAssumed {
			hasResponses = true
			break
		}
	}
	if !hasResponses {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n=== Human Responses ===\n")

	// Sort by ID for consistent output
	sorted := make([]*Question, len(questions))
	copy(sorted, questions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	for _, q := range sorted {
		if q.Status == QuestionStatusAnswered && q.Answer != nil {
			fmt.Fprintf(&b, "\n%s: %s\n", q.ID, q.Question)
			fmt.Fprintf(&b, "→ %s\n", *q.Answer)
			fmt.Fprintf(&b, "[Source: human input]\n")
		} else if q.Status == QuestionStatusAssumed && q.Assumption != nil {
			fmt.Fprintf(&b, "\n%s: %s\n", q.ID, q.Question)
			fmt.Fprintf(&b, "→ %s\n", *q.Assumption)
			if timeoutMinutes > 0 {
				fmt.Fprintf(&b, "[Source: auto-assumed after timeout of %d minutes]\n", timeoutMinutes)
			} else {
				fmt.Fprintf(&b, "[Source: auto-assumed]\n")
			}
		}
	}

	b.WriteString("\n")
	return b.String()
}

// ShouldPauseForHuman returns whether the feature should enter waiting_for_human status.
// It checks if the feature is in inception or planning phase and if the timeout is not 0.
func ShouldPauseForHuman(f *Feature, timeoutMinutes int) bool {
	// timeoutMinutes == 0 means fully autonomous, never pause
	if timeoutMinutes == 0 {
		return false
	}
	// Can only pause for human during ideation or inception phases
	phase := f.CurrentPhase()
	if phase != "ideation" && phase != "inception" {
		return false
	}
	// Must be in_progress status
	if f.Status != StatusInProgress {
		return false
	}
	return true
}

// WaitForFeedback transitions the feature to waiting_for_feedback status.
// Used when the PM asks questions that need user answers before proceeding.
func (f *Feature) WaitForFeedback() error {
	if f.Status == StatusDone || f.Status == StatusCancelled {
		return fmt.Errorf("cannot wait for feedback from terminal status %q", f.Status)
	}
	f.Status = StatusWaitingFeedback
	f.UpdatedAt = time.Now()
	return nil
}

// ResumeFromFeedback transitions the feature from waiting_for_feedback back to in_progress.
func (f *Feature) ResumeFromFeedback() error {
	if f.Status != StatusWaitingFeedback {
		return fmt.Errorf("feature is not in waiting_for_feedback status (current: %s)", f.Status)
	}
	f.Status = StatusInProgress
	f.UpdatedAt = time.Now()
	return nil
}

// ResumeFromWaitingHuman is kept for backward compatibility
func (f *Feature) ResumeFromWaitingHuman() error {
	return f.ResumeFromFeedback()
}

// CanTransitionToWaitingFeedback checks if a feature can transition to waiting_for_feedback.
func (f *Feature) CanTransitionToWaitingFeedback() bool {
	if f.Status != StatusInProgress {
		return false
	}
	phase := f.CurrentPhase()
	if phase != "ideation" && phase != "inception" {
		return false
	}
	return true
}
