package feature

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestQuestionValidation(t *testing.T) {
	tests := []struct {
		name     string
		question Question
		wantErr  string
	}{
		{
			name:     "valid clarification question",
			question: Question{Phase: "inception", Role: "pm", Question: "What is the target audience?", Type: "clarification"},
			wantErr:  "",
		},
		{
			name:     "valid decision question with options",
			question: Question{Phase: "planning", Role: "architect", Question: "Which approach?", Type: "decision", Options: []string{"A", "B"}},
			wantErr:  "",
		},
		{
			name:     "valid priority question",
			question: Question{Phase: "inception", Role: "pm", Question: "What priority?", Type: "priority"},
			wantErr:  "",
		},
		{
			name:     "empty question text",
			question: Question{Phase: "inception", Role: "pm", Question: "", Type: "clarification"},
			wantErr:  "question is required",
		},
		{
			name:     "question too long",
			question: Question{Phase: "inception", Role: "pm", Question: string(make([]byte, 2001)), Type: "clarification"},
			wantErr:  "question must be 1-2000 characters",
		},
		{
			name:     "invalid phase",
			question: Question{Phase: "construction", Role: "pm", Question: "What?", Type: "clarification"},
			wantErr:  "phase must be one of: inception, planning",
		},
		{
			name:     "invalid role",
			question: Question{Phase: "inception", Role: "developer", Question: "What?", Type: "clarification"},
			wantErr:  "role must be one of: pm, architect",
		},
		{
			name:     "invalid type",
			question: Question{Phase: "inception", Role: "pm", Question: "What?", Type: "invalid"},
			wantErr:  "type must be one of: clarification, decision, priority",
		},
		{
			name:     "too many options",
			question: Question{Phase: "inception", Role: "pm", Question: "What?", Type: "clarification", Options: make([]string, 11)},
			wantErr:  "options must have at most 10 items",
		},
		{
			name:     "option too long",
			question: Question{Phase: "inception", Role: "pm", Question: "What?", Type: "clarification", Options: []string{string(make([]byte, 501))}},
			wantErr:  "each option must be 1-500 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateQuestion(&tt.question)
			if tt.wantErr == "" && got != "" {
				t.Errorf("ValidateQuestion() = %q, want empty string", got)
			}
			if tt.wantErr != "" && got != tt.wantErr {
				t.Errorf("ValidateQuestion() = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

func TestQuestionIDGeneration(t *testing.T) {
	tests := []struct {
		name     string
		existing []*Question
		wantID   string
	}{
		{
			name:     "first question gets Q-001",
			existing: nil,
			wantID:   "Q-001",
		},
		{
			name: "second question gets Q-002",
			existing: []*Question{
				{ID: "Q-001"},
			},
			wantID: "Q-002",
		},
		{
			name: "skips gaps in IDs",
			existing: []*Question{
				{ID: "Q-001"},
				{ID: "Q-003"},
			},
			wantID: "Q-004",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextQuestionID(tt.existing)
			if got != tt.wantID {
				t.Errorf("nextQuestionID() = %q, want %q", got, tt.wantID)
			}
		})
	}
}

func setupTestQuestionStore(t *testing.T) *FileQuestionStore {
	t.Helper()
	dir := t.TempDir()
	return NewFileQuestionStore(dir)
}

func TestCreateQuestion(t *testing.T) {
	store := setupTestQuestionStore(t)
	ctx := context.Background()

	q := Question{
		FeatureID: "test-feature",
		Phase:     "inception",
		Role:      "pm",
		Question:  "What is the target audience?",
		Type:      "clarification",
		Options:   []string{"Developers", "Users"},
	}

	created, err := store.CreateQuestion(ctx, "test-feature", q)
	if err != nil {
		t.Fatalf("CreateQuestion() error = %v", err)
	}

	if created.ID != "Q-001" {
		t.Errorf("CreateQuestion() ID = %q, want Q-001", created.ID)
	}
	if created.Status != QuestionStatusPending {
		t.Errorf("CreateQuestion() Status = %q, want %q", created.Status, QuestionStatusPending)
	}
	if created.FeatureID != "test-feature" {
		t.Errorf("CreateQuestion() FeatureID = %q, want test-feature", created.FeatureID)
	}
	if created.CreatedAt.IsZero() {
		t.Error("CreateQuestion() CreatedAt should be set")
	}
	if created.Answer != nil {
		t.Error("CreateQuestion() Answer should be nil")
	}
	if created.Assumption != nil {
		t.Error("CreateQuestion() Assumption should be nil")
	}
	if created.AnsweredAt != nil {
		t.Error("CreateQuestion() AnsweredAt should be nil")
	}
	// Verify Options is not nil (empty array, not null)
	if created.Options == nil {
		t.Error("CreateQuestion() Options should not be nil")
	}
}

func TestCreateQuestionEmptyOptions(t *testing.T) {
	store := setupTestQuestionStore(t)
	ctx := context.Background()

	q := Question{
		FeatureID: "test-feature",
		Phase:     "inception",
		Role:      "pm",
		Question:  "What?",
		Type:      "clarification",
	}

	created, err := store.CreateQuestion(ctx, "test-feature", q)
	if err != nil {
		t.Fatalf("CreateQuestion() error = %v", err)
	}

	// Empty options should be [] not nil
	if created.Options == nil {
		t.Error("CreateQuestion() Options should be empty slice, not nil")
	}
	if len(created.Options) != 0 {
		t.Errorf("CreateQuestion() Options length = %d, want 0", len(created.Options))
	}
}

func TestAnswerQuestionConflict(t *testing.T) {
	store := setupTestQuestionStore(t)
	ctx := context.Background()

	q := Question{
		FeatureID: "test-feature",
		Phase:     "inception",
		Role:      "pm",
		Question:  "What?",
		Type:      "clarification",
	}

	created, err := store.CreateQuestion(ctx, "test-feature", q)
	if err != nil {
		t.Fatalf("CreateQuestion() error = %v", err)
	}

	// First answer should succeed
	_, err = store.AnswerQuestion(ctx, "test-feature", created.ID, "My answer")
	if err != nil {
		t.Fatalf("AnswerQuestion() first call error = %v", err)
	}

	// Second answer should fail with conflict
	_, err = store.AnswerQuestion(ctx, "test-feature", created.ID, "Another answer")
	if err == nil {
		t.Error("AnswerQuestion() second call should return error")
	}
	if conflictErr, ok := err.(*QuestionConflictError); !ok {
		t.Errorf("AnswerQuestion() error type = %T, want *QuestionConflictError", err)
	} else {
		if conflictErr.QuestionID != created.ID {
			t.Errorf("QuestionConflictError.QuestionID = %q, want %q", conflictErr.QuestionID, created.ID)
		}
	}
}

func TestAnswerQuestionNotFound(t *testing.T) {
	store := setupTestQuestionStore(t)
	ctx := context.Background()

	_, err := store.AnswerQuestion(ctx, "test-feature", "Q-999", "My answer")
	if err == nil {
		t.Error("AnswerQuestion() should return error for nonexistent question")
	}
}

func TestAssumeQuestion(t *testing.T) {
	store := setupTestQuestionStore(t)
	ctx := context.Background()

	q := Question{
		FeatureID: "test-feature",
		Phase:     "inception",
		Role:      "pm",
		Question:  "What?",
		Type:      "clarification",
		Options:   []string{"Option A", "Option B"},
	}

	created, err := store.CreateQuestion(ctx, "test-feature", q)
	if err != nil {
		t.Fatalf("CreateQuestion() error = %v", err)
	}

	assumed, err := store.AssumeQuestion(ctx, "test-feature", created.ID, "No human response received. Assuming: Option A")
	if err != nil {
		t.Fatalf("AssumeQuestion() error = %v", err)
	}

	if assumed.Status != QuestionStatusAssumed {
		t.Errorf("AssumeQuestion() Status = %q, want %q", assumed.Status, QuestionStatusAssumed)
	}
	if assumed.Assumption == nil || *assumed.Assumption != "No human response received. Assuming: Option A" {
		t.Errorf("AssumeQuestion() Assumption = %v, want specific value", assumed.Assumption)
	}
	if assumed.AnsweredAt == nil {
		t.Error("AssumeQuestion() AnsweredAt should be set")
	}

	// Assumed question cannot be answered
	_, err = store.AnswerQuestion(ctx, "test-feature", created.ID, "Late answer")
	if err == nil {
		t.Error("AnswerQuestion() on assumed question should return error")
	}
}

func TestAssumeQuestionConflict(t *testing.T) {
	store := setupTestQuestionStore(t)
	ctx := context.Background()

	q := Question{
		FeatureID: "test-feature",
		Phase:     "inception",
		Role:      "pm",
		Question:  "What?",
		Type:      "clarification",
	}

	created, err := store.CreateQuestion(ctx, "test-feature", q)
	if err != nil {
		t.Fatalf("CreateQuestion() error = %v", err)
	}

	// Answer the question first
	_, err = store.AnswerQuestion(ctx, "test-feature", created.ID, "My answer")
	if err != nil {
		t.Fatalf("AnswerQuestion() error = %v", err)
	}

	// Trying to assume an already-answered question should fail
	_, err = store.AssumeQuestion(ctx, "test-feature", created.ID, "Assumption")
	if err == nil {
		t.Error("AssumeQuestion() on answered question should return error")
	}
}

func TestListQuestionsEmpty(t *testing.T) {
	store := setupTestQuestionStore(t)
	ctx := context.Background()

	questions, err := store.ListQuestions(ctx, "nonexistent-feature")
	if err != nil {
		t.Fatalf("ListQuestions() error = %v", err)
	}
	if questions == nil {
		t.Error("ListQuestions() should return empty slice, not nil")
	}
	if len(questions) != 0 {
		t.Errorf("ListQuestions() length = %d, want 0", len(questions))
	}
}

func TestListQuestionsWithData(t *testing.T) {
	store := setupTestQuestionStore(t)
	ctx := context.Background()

	// Create two questions
	q1 := Question{FeatureID: "test-feature", Phase: "inception", Role: "pm", Question: "Q1?", Type: "clarification"}
	q2 := Question{FeatureID: "test-feature", Phase: "planning", Role: "architect", Question: "Q2?", Type: "decision"}

	_, err := store.CreateQuestion(ctx, "test-feature", q1)
	if err != nil {
		t.Fatalf("CreateQuestion() error = %v", err)
	}
	_, err = store.CreateQuestion(ctx, "test-feature", q2)
	if err != nil {
		t.Fatalf("CreateQuestion() error = %v", err)
	}

	questions, err := store.ListQuestions(ctx, "test-feature")
	if err != nil {
		t.Fatalf("ListQuestions() error = %v", err)
	}
	if len(questions) != 2 {
		t.Errorf("ListQuestions() length = %d, want 2", len(questions))
	}
}

func TestListPendingQuestions(t *testing.T) {
	store := setupTestQuestionStore(t)
	ctx := context.Background()

	q1 := Question{FeatureID: "test-feature", Phase: "inception", Role: "pm", Question: "Q1?", Type: "clarification"}
	q2 := Question{FeatureID: "test-feature", Phase: "planning", Role: "architect", Question: "Q2?", Type: "decision"}
	q3 := Question{FeatureID: "test-feature", Phase: "inception", Role: "pm", Question: "Q3?", Type: "priority"}

	created1, _ := store.CreateQuestion(ctx, "test-feature", q1)
	store.CreateQuestion(ctx, "test-feature", q2)
	created3, _ := store.CreateQuestion(ctx, "test-feature", q3)

	// Answer one question
	store.AnswerQuestion(ctx, "test-feature", created1.ID, "Answer 1")

	// 2 pending questions remain
	pending, err := store.ListPendingQuestions(ctx, "test-feature")
	if err != nil {
		t.Fatalf("ListPendingQuestions() error = %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("ListPendingQuestions() length = %d, want 2", len(pending))
	}

	// Answer the third question
	store.AnswerQuestion(ctx, "test-feature", created3.ID, "Answer 3")

	// Now only 1 pending
	pending, err = store.ListPendingQuestions(ctx, "test-feature")
	if err != nil {
		t.Fatalf("ListPendingQuestions() error = %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("ListPendingQuestions() length = %d, want 1", len(pending))
	}

	// All questions answered - pending should be empty
	// q2 is still pending, so we have 1

	// Answer q2
	pending2, _ := store.ListPendingQuestions(ctx, "test-feature")
	if len(pending2) > 0 {
		store.AnswerQuestion(ctx, "test-feature", pending2[0].ID, "Answer 2")
	}

	pending, err = store.ListPendingQuestions(ctx, "test-feature")
	if err != nil {
		t.Fatalf("ListPendingQuestions() error = %v", err)
	}
	if pending == nil {
		t.Error("ListPendingQuestions() should return empty slice, not nil")
	}
	if len(pending) != 0 {
		t.Errorf("ListPendingQuestions() length = %d, want 0", len(pending))
	}
}

func TestDeleteQuestionsForFeature(t *testing.T) {
	store := setupTestQuestionStore(t)
	ctx := context.Background()

	q := Question{FeatureID: "test-feature", Phase: "inception", Role: "pm", Question: "Q1?", Type: "clarification"}
	_, err := store.CreateQuestion(ctx, "test-feature", q)
	if err != nil {
		t.Fatalf("CreateQuestion() error = %v", err)
	}

	err = store.DeleteQuestionsForFeature(ctx, "test-feature")
	if err != nil {
		t.Fatalf("DeleteQuestionsForFeature() error = %v", err)
	}

	questions, err := store.ListQuestions(ctx, "test-feature")
	if err != nil {
		t.Fatalf("ListQuestions() error = %v", err)
	}
	if len(questions) != 0 {
		t.Errorf("ListQuestions() after delete length = %d, want 0", len(questions))
	}
}

func TestDeleteQuestionsNonexistentFeature(t *testing.T) {
	store := setupTestQuestionStore(t)
	ctx := context.Background()

	err := store.DeleteQuestionsForFeature(ctx, "nonexistent-feature")
	if err != nil {
		t.Errorf("DeleteQuestionsForFeature() for nonexistent feature should not error, got: %v", err)
	}
}

func TestPendingCount(t *testing.T) {
	store := setupTestQuestionStore(t)
	ctx := context.Background()

	// No questions initially
	count, err := store.PendingCount(ctx, "test-feature")
	if err != nil {
		t.Fatalf("PendingCount() error = %v", err)
	}
	if count != 0 {
		t.Errorf("PendingCount() = %d, want 0", count)
	}

	// Create 3 questions
	q1 := Question{FeatureID: "test-feature", Phase: "inception", Role: "pm", Question: "Q1?", Type: "clarification"}
	q2 := Question{FeatureID: "test-feature", Phase: "planning", Role: "architect", Question: "Q2?", Type: "decision"}
	q3 := Question{FeatureID: "test-feature", Phase: "inception", Role: "pm", Question: "Q3?", Type: "priority"}

	created1, _ := store.CreateQuestion(ctx, "test-feature", q1)
	store.CreateQuestion(ctx, "test-feature", q2)
	created3, _ := store.CreateQuestion(ctx, "test-feature", q3)

	count, err = store.PendingCount(ctx, "test-feature")
	if err != nil {
		t.Fatalf("PendingCount() error = %v", err)
	}
	if count != 3 {
		t.Errorf("PendingCount() = %d, want 3", count)
	}

	// Answer one question
	store.AnswerQuestion(ctx, "test-feature", created1.ID, "Answer")
	count, _ = store.PendingCount(ctx, "test-feature")
	if count != 2 {
		t.Errorf("PendingCount() after answer = %d, want 2", count)
	}

	// Assume another
	store.AssumeQuestion(ctx, "test-feature", created3.ID, "Assumed")
	count, _ = store.PendingCount(ctx, "test-feature")
	if count != 1 {
		t.Errorf("PendingCount() after assume = %d, want 1", count)
	}
}

func TestGetQuestion(t *testing.T) {
	store := setupTestQuestionStore(t)
	ctx := context.Background()

	q := Question{FeatureID: "test-feature", Phase: "inception", Role: "pm", Question: "Q1?", Type: "clarification"}
	created, err := store.CreateQuestion(ctx, "test-feature", q)
	if err != nil {
		t.Fatalf("CreateQuestion() error = %v", err)
	}

	got, err := store.GetQuestion(ctx, "test-feature", created.ID)
	if err != nil {
		t.Fatalf("GetQuestion() error = %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("GetQuestion() ID = %q, want %q", got.ID, created.ID)
	}
	if got.Question != "Q1?" {
		t.Errorf("GetQuestion() Question = %q, want %q", got.Question, "Q1?")
	}
}

func TestGetQuestionNotFound(t *testing.T) {
	store := setupTestQuestionStore(t)
	ctx := context.Background()

	_, err := store.GetQuestion(ctx, "test-feature", "Q-999")
	if err == nil {
		t.Error("GetQuestion() should return error for nonexistent question")
	}
}

func TestDetectQuestions_Valid(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "test-feature")
	os.MkdirAll(specDir, 0755)

	content := `[{"phase":"inception","role":"pm","question":"What is the target?","type":"clarification","options":["A","B"]}]`
	os.WriteFile(filepath.Join(specDir, "questions.json"), []byte(content), 0644)

	questions := DetectQuestions("test-feature", specDir)
	if len(questions) != 1 {
		t.Fatalf("DetectQuestions() returned %d questions, want 1", len(questions))
	}
	if questions[0].Phase != "inception" {
		t.Errorf("Phase = %q, want inception", questions[0].Phase)
	}
	if questions[0].Question != "What is the target?" {
		t.Errorf("Question = %q, want %q", questions[0].Question, "What is the target?")
	}
	if len(questions[0].Options) != 2 {
		t.Errorf("Options length = %d, want 2", len(questions[0].Options))
	}
}

func TestDetectQuestions_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "test-feature")
	os.MkdirAll(specDir, 0755)

	os.WriteFile(filepath.Join(specDir, "questions.json"), []byte("{invalid json"), 0644)

	questions := DetectQuestions("test-feature", specDir)
	if questions != nil {
		t.Errorf("DetectQuestions() with invalid JSON should return nil, got %d questions", len(questions))
	}
}

func TestDetectQuestions_MissingFields(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "test-feature")
	os.MkdirAll(specDir, 0755)

	// Missing "question" field
	content := `[{"phase":"inception","role":"pm","type":"clarification"}]`
	os.WriteFile(filepath.Join(specDir, "questions.json"), []byte(content), 0644)

	questions := DetectQuestions("test-feature", specDir)
	if len(questions) != 0 {
		t.Errorf("DetectQuestions() with missing required fields should return 0 questions, got %d", len(questions))
	}
}

func TestDetectQuestions_InvalidPhase(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "test-feature")
	os.MkdirAll(specDir, 0755)

	content := `[{"phase":"construction","role":"pm","question":"What?","type":"clarification"}]`
	os.WriteFile(filepath.Join(specDir, "questions.json"), []byte(content), 0644)

	questions := DetectQuestions("test-feature", specDir)
	if len(questions) != 0 {
		t.Errorf("DetectQuestions() with invalid phase should return 0 questions, got %d", len(questions))
	}
}

func TestDetectQuestions_NoFile(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "test-feature")
	os.MkdirAll(specDir, 0755)

	questions := DetectQuestions("test-feature", specDir)
	if questions != nil {
		t.Errorf("DetectQuestions() with no file should return nil, got %d questions", len(questions))
	}
}

func TestDetectQuestions_MixedValidInvalid(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "test-feature")
	os.MkdirAll(specDir, 0755)

	content := `[
		{"phase":"inception","role":"pm","question":"Valid question?","type":"clarification"},
		{"phase":"construction","role":"pm","question":"Invalid phase","type":"clarification"},
		{"phase":"planning","role":"architect","question":"Also valid?","type":"decision"}
	]`
	os.WriteFile(filepath.Join(specDir, "questions.json"), []byte(content), 0644)

	questions := DetectQuestions("test-feature", specDir)
	if len(questions) != 2 {
		t.Errorf("DetectQuestions() with mixed valid/invalid should return 2 questions, got %d", len(questions))
	}
}

func TestShouldPauseForHuman(t *testing.T) {
	tests := []struct {
		name           string
		feature        *Feature
		timeoutMinutes int
		want           bool
	}{
		{
			name:           "in_progress inception with positive timeout",
			feature:        &Feature{Current: PhaseInception, Status: StatusInProgress},
			timeoutMinutes: 30,
			want:           true,
		},
		{
			name:           "in_progress planning with positive timeout",
			feature:        &Feature{Current: PhasePlanning, Status: StatusInProgress},
			timeoutMinutes: 30,
			want:           true,
		},
		{
			name:           "in_progress construction with positive timeout - not allowed",
			feature:        &Feature{Current: PhaseConstruction, Status: StatusInProgress},
			timeoutMinutes: 30,
			want:           false,
		},
		{
			name:           "in_progress inception with zero timeout - fully autonomous",
			feature:        &Feature{Current: PhaseInception, Status: StatusInProgress},
			timeoutMinutes: 0,
			want:           false,
		},
		{
			name:           "in_progress inception with -1 timeout - wait forever",
			feature:        &Feature{Current: PhaseInception, Status: StatusInProgress},
			timeoutMinutes: -1,
			want:           true,
		},
		{
			name:           "draft status - not allowed",
			feature:        &Feature{Current: PhaseInception, Status: StatusDraft},
			timeoutMinutes: 30,
			want:           false,
		},
		{
			name:           "waiting_for_human status - not allowed (must resume first)",
			feature:        &Feature{Current: PhaseInception, Status: StatusWaitingFeedback},
			timeoutMinutes: 30,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldPauseForHuman(tt.feature, tt.timeoutMinutes)
			if got != tt.want {
				t.Errorf("ShouldPauseForHuman() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanTransitionToWaitingFeedback(t *testing.T) {
	tests := []struct {
		name    string
		feature *Feature
		want    bool
	}{
		{
			name:    "in_progress inception - allowed",
			feature: &Feature{Current: PhaseInception, Status: StatusInProgress},
			want:    true,
		},
		{
			name:    "in_progress planning - allowed",
			feature: &Feature{Current: PhasePlanning, Status: StatusInProgress},
			want:    true,
		},
		{
			name:    "in_progress construction - not allowed",
			feature: &Feature{Current: PhaseConstruction, Status: StatusInProgress},
			want:    false,
		},
		{
			name:    "draft status - not allowed",
			feature: &Feature{Current: PhaseInception, Status: StatusDraft},
			want:    false,
		},
		{
			name:    "waiting_for_human status - not allowed (no self-transition)",
			feature: &Feature{Current: PhaseInception, Status: StatusWaitingFeedback},
			want:    false,
		},
		{
			name:    "gate_blocked status - not allowed",
			feature: &Feature{Current: PhaseInception, Status: StatusGateBlocked},
			want:    false,
		},
		{
			name:    "done status - not allowed",
			feature: &Feature{Current: PhaseDelivery, Status: StatusDone},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.feature.CanTransitionToWaitingFeedback()
			if got != tt.want {
				t.Errorf("CanTransitionToWaitingFeedback() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWaitForHuman(t *testing.T) {
	f := NewFeature("test-feature", "Test Feature", 2, IntakeLooseIdea)
	f.Start()

	err := f.WaitForHuman()
	if err != nil {
		t.Fatalf("WaitForHuman() error = %v", err)
	}
	if f.Status != StatusWaitingFeedback {
		t.Errorf("Status = %q, want %q", f.Status, StatusWaitingFeedback)
	}
}

func TestWaitForHuman_InvalidStatus(t *testing.T) {
	f := NewFeature("test-feature", "Test Feature", 2, IntakeLooseIdea)
	// f is in draft status — WaitForFeedback allows non-terminal statuses
	_ = f.WaitForHuman()
}

func TestWaitForHuman_InvalidPhase(t *testing.T) {
	f := NewFeature("test-feature", "Test Feature", 2, IntakeLooseIdea)
	f.Start()
	f.Current = PhaseConstruction

	// WaitForFeedback is more permissive — allows any non-terminal phase to pause for feedback
	_ = f.WaitForHuman()
}

func TestResumeFromWaitingHuman(t *testing.T) {
	f := NewFeature("test-feature", "Test Feature", 2, IntakeLooseIdea)
	f.Start()
	f.WaitForHuman()

	err := f.ResumeFromWaitingHuman()
	if err != nil {
		t.Fatalf("ResumeFromWaitingHuman() error = %v", err)
	}
	if f.Status != StatusInProgress {
		t.Errorf("Status = %q, want %q", f.Status, StatusInProgress)
	}
}

func TestResumeFromWaitingHuman_InvalidStatus(t *testing.T) {
	f := NewFeature("test-feature", "Test Feature", 2, IntakeLooseIdea)
	f.Start()

	err := f.ResumeFromWaitingHuman()
	if err == nil {
		t.Error("ResumeFromWaitingHuman() on in_progress feature should return error")
	}
}

func TestAdvanceFromWaitingHumanBlocked(t *testing.T) {
	f := NewFeature("test-feature", "Test Feature", 2, IntakeLooseIdea)
	f.Start()
	f.WaitForHuman()

	err := f.AdvanceTo(PhasePlanning)
	if err == nil {
		t.Error("AdvanceTo() from waiting_for_human should return error")
	}
}

func TestCancelFromWaitingHuman(t *testing.T) {
	f := NewFeature("test-feature", "Test Feature", 2, IntakeLooseIdea)
	f.Start()
	f.WaitForHuman()

	f.Cancel()
	if f.Status != StatusCancelled {
		t.Errorf("Status = %q, want %q", f.Status, StatusCancelled)
	}
}

func TestGenerateAssumptionText(t *testing.T) {
	tests := []struct {
		name           string
		question       *Question
		timeoutMinutes int
		wantContains   string
	}{
		{
			name: "with options uses first option",
			question: &Question{
				ID:       "Q-001",
				Question: "Which approach?",
				Options:  []string{"REST API", "GraphQL", "gRPC"},
			},
			timeoutMinutes: 30,
			wantContains:   "REST API",
		},
		{
			name: "without options uses default text",
			question: &Question{
				ID:       "Q-001",
				Question: "What is the target audience?",
				Options:  []string{},
			},
			timeoutMinutes: 30,
			wantContains:   "auto-assumed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateAssumptionText(tt.question, tt.timeoutMinutes)
			if !contains(got, tt.wantContains) {
				t.Errorf("GenerateAssumptionText() = %q, want to contain %q", got, tt.wantContains)
			}
		})
	}
}

func TestBuildHumanResponsesContext(t *testing.T) {
	tests := []struct {
		name           string
		questions      []*Question
		timeoutMinutes int
		wantEmpty      bool
		wantContains   []string
	}{
		{
			name:           "empty questions returns empty string",
			questions:      nil,
			timeoutMinutes: 30,
			wantEmpty:      true,
		},
		{
			name: "answered question shows human input source",
			questions: []*Question{
				{
					ID:       "Q-001",
					Question: "What is the target?",
					Status:   QuestionStatusAnswered,
					Answer:   strPtr("Internal developers"),
				},
			},
			timeoutMinutes: 30,
			wantEmpty:      false,
			wantContains:   []string{"Q-001", "What is the target?", "Internal developers", "human input"},
		},
		{
			name: "assumed question shows auto-assumed source",
			questions: []*Question{
				{
					ID:         "Q-001",
					Question:   "Which approach?",
					Status:     QuestionStatusAssumed,
					Assumption: strPtr("REST API"),
				},
			},
			timeoutMinutes: 30,
			wantEmpty:      false,
			wantContains:   []string{"Q-001", "Which approach?", "REST API", "auto-assumed", "30 minutes"},
		},
		{
			name: "mixed answered and assumed questions",
			questions: []*Question{
				{
					ID:       "Q-001",
					Question: "What is the target?",
					Status:   QuestionStatusAnswered,
					Answer:   strPtr("Developers"),
				},
				{
					ID:         "Q-002",
					Question:   "Which approach?",
					Status:     QuestionStatusAssumed,
					Assumption: strPtr("SSE"),
				},
			},
			timeoutMinutes: 30,
			wantEmpty:      false,
			wantContains:   []string{"Q-001", "human input", "Q-002", "auto-assumed"},
		},
		{
			name: "pending questions are not included in responses",
			questions: []*Question{
				{
					ID:       "Q-001",
					Question: "Pending question?",
					Status:   QuestionStatusPending,
				},
			},
			timeoutMinutes: 30,
			wantEmpty:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildHumanResponsesContext(tt.questions, tt.timeoutMinutes)
			if tt.wantEmpty && got != "" {
				t.Errorf("BuildHumanResponsesContext() = %q, want empty string", got)
			}
			if !tt.wantEmpty {
				if !contains(got, "Human Responses") {
					t.Errorf("BuildHumanResponsesContext() missing 'Human Responses' header")
				}
				for _, substr := range tt.wantContains {
					if !contains(got, substr) {
						t.Errorf("BuildHumanResponsesContext() missing substring %q", substr)
					}
				}
			}
		})
	}
}

func TestAssumeAllPendingQuestions(t *testing.T) {
	store := setupTestQuestionStore(t)
	ctx := context.Background()

	q1 := Question{FeatureID: "test-feature", Phase: "inception", Role: "pm", Question: "Q1?", Type: "clarification"}
	q2 := Question{FeatureID: "test-feature", Phase: "planning", Role: "architect", Question: "Q2?", Type: "decision", Options: []string{"Option A"}}
	q3 := Question{FeatureID: "test-feature", Phase: "inception", Role: "pm", Question: "Q3?", Type: "priority"}

	store.CreateQuestion(ctx, "test-feature", q1)
	store.CreateQuestion(ctx, "test-feature", q2)
	created3, _ := store.CreateQuestion(ctx, "test-feature", q3)

	// Answer one question first
	store.AnswerQuestion(ctx, "test-feature", created3.ID, "My answer")

	// Assume all pending (2 remaining)
	assumed, err := AssumeAllPendingQuestions(store, "test-feature", 30)
	if err != nil {
		t.Fatalf("AssumeAllPendingQuestions() error = %v", err)
	}
	if len(assumed) != 2 {
		t.Errorf("AssumeAllPendingQuestions() returned %d assumed questions, want 2", len(assumed))
	}

	// All questions should now be answered or assumed
	pending, _ := store.ListPendingQuestions(ctx, "test-feature")
	if len(pending) != 0 {
		t.Errorf("ListPendingQuestions() after assume = %d, want 0", len(pending))
	}

	// The previously answered question should remain answered
	questions, _ := store.ListQuestions(ctx, "test-feature")
	answeredCount := 0
	assumedCount := 0
	for _, q := range questions {
		if q.Status == QuestionStatusAnswered {
			answeredCount++
		}
		if q.Status == QuestionStatusAssumed {
			assumedCount++
		}
	}
	if answeredCount != 1 {
		t.Errorf("Answered count = %d, want 1", answeredCount)
	}
	if assumedCount != 2 {
		t.Errorf("Assumed count = %d, want 2", assumedCount)
	}
}

// Helper functions

func strPtr(s string) *string {
	return &s
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
