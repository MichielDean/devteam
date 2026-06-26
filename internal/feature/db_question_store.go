package feature

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/MichielDean/devteam/internal/db"
)

// DBQuestionStore implements QuestionStore using SQLite.
// Questions are stored in the questions table with full history.
type DBQuestionStore struct {
	db *db.DB
}

// NewDBQuestionStore creates a new DBQuestionStore.
func NewDBQuestionStore(database *db.DB) *DBQuestionStore {
	return &DBQuestionStore{db: database}
}

func (s *DBQuestionStore) CreateQuestion(ctx context.Context, featureID string, q Question) (*Question, error) {
	now := time.Now().UTC()
	q.FeatureID = featureID
	if q.ID == "" {
		q.ID = fmt.Sprintf("%s-%s-%d", featureID, q.Phase, now.UnixNano())
	}
	if q.CreatedAt.IsZero() {
		q.CreatedAt = now
	}
	if q.Status == "" {
		q.Status = QuestionStatusPending
	}

	optionsJSON, _ := json.Marshal(q.Options)

	var answerStr string
	if q.Answer != nil {
		answerStr = *q.Answer
	}
	assumedInt := 0
	if q.Status == QuestionStatusAssumed {
		assumedInt = 1
	}

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO questions (id, feature_id, phase, role, question, question_type, options, answer, status, assumed, created_at, answered_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		q.ID, featureID, string(q.Phase), q.Role, q.Question, q.Type, string(optionsJSON), answerStr, q.Status, assumedInt, q.CreatedAt, q.AnsweredAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating question: %w", err)
	}

	return &q, nil
}

func (s *DBQuestionStore) GetQuestion(ctx context.Context, featureID string, questionID string) (*Question, error) {
	row := s.db.QueryRow(
		`SELECT id, feature_id, phase, role, question, question_type, options, answer, status, assumed, created_at, answered_at
		 FROM questions WHERE id = ?`, questionID,
	)
	return scanQuestion(row)
}

func (s *DBQuestionStore) ListQuestions(ctx context.Context, featureID string) ([]*Question, error) {
	rows, err := s.db.Query(
		`SELECT id, feature_id, phase, role, question, question_type, options, answer, status, assumed, created_at, answered_at
		 FROM questions WHERE feature_id = ? ORDER BY created_at ASC`, featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing questions: %w", err)
	}
	defer rows.Close()

	var questions []*Question
	for rows.Next() {
		q, err := scanQuestionRow(rows)
		if err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, nil
}

func (s *DBQuestionStore) ListPendingQuestions(ctx context.Context, featureID string) ([]*Question, error) {
	rows, err := s.db.Query(
		`SELECT id, feature_id, phase, role, question, question_type, options, answer, status, assumed, created_at, answered_at
		 FROM questions WHERE feature_id = ? AND status = 'pending' ORDER BY created_at ASC`, featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing pending questions: %w", err)
	}
	defer rows.Close()

	var questions []*Question
	for rows.Next() {
		q, err := scanQuestionRow(rows)
		if err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, nil
}

func (s *DBQuestionStore) AnswerQuestion(ctx context.Context, featureID string, questionID string, answer string) (*Question, error) {
	// Atomic check-and-update to prevent race: only update if status is pending
	result, err := s.db.Exec(
		`UPDATE questions SET answer = ?, status = 'answered', answered_at = ? WHERE id = ? AND status = 'pending'`,
		answer, time.Now().UTC(), questionID,
	)
	if err != nil {
		return nil, fmt.Errorf("answering question: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Either not found or already answered/assumed
		existing, err := s.GetQuestion(ctx, featureID, questionID)
		if err != nil {
			return nil, fmt.Errorf("question %s not found: %w", questionID, err)
		}
		return nil, &QuestionConflictError{QuestionID: questionID, Status: existing.Status}
	}
	return s.GetQuestion(ctx, featureID, questionID)
}

func (s *DBQuestionStore) AssumeQuestion(ctx context.Context, featureID string, questionID string, assumption string) (*Question, error) {
	_, err := s.db.Exec(
		`UPDATE questions SET answer = ?, status = 'assumed', assumed = 1, answered_at = ? WHERE id = ?`,
		assumption, time.Now().UTC(), questionID,
	)
	if err != nil {
		return nil, fmt.Errorf("assuming question: %w", err)
	}
	return s.GetQuestion(ctx, featureID, questionID)
}

func (s *DBQuestionStore) DeleteQuestionsForFeature(ctx context.Context, featureID string) error {
	_, err := s.db.Exec(`DELETE FROM questions WHERE feature_id = ?`, featureID)
	if err != nil {
		return fmt.Errorf("deleting questions: %w", err)
	}
	return nil
}

func (s *DBQuestionStore) PendingCount(ctx context.Context, featureID string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM questions WHERE feature_id = ? AND status = 'pending'`, featureID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting pending questions: %w", err)
	}
	return count, nil
}

// Helpers

type questionScanner interface {
	Scan(dest ...interface{}) error
}

func scanQuestion(row *sql.Row) (*Question, error) {
	var q Question
	var optionsStr string
	var answerStr string
	var assumedInt int
	var answeredAt sql.NullTime
	err := row.Scan(&q.ID, &q.FeatureID, &q.Phase, &q.Role, &q.Question, &q.Type, &optionsStr, &answerStr, &q.Status, &assumedInt, &q.CreatedAt, &answeredAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("question not found")
		}
		return nil, fmt.Errorf("scanning question: %w", err)
	}
	if answerStr != "" {
		q.Answer = &answerStr
	}
	if assumedInt == 1 && answerStr != "" {
		q.Assumption = &answerStr
	}
	if answeredAt.Valid {
		q.AnsweredAt = &answeredAt.Time
	}
	json.Unmarshal([]byte(optionsStr), &q.Options)
	return &q, nil
}

func scanQuestionRow(rows *sql.Rows) (*Question, error) {
	var q Question
	var optionsStr string
	var answerStr string
	var assumedInt int
	var answeredAt sql.NullTime
	if err := rows.Scan(&q.ID, &q.FeatureID, &q.Phase, &q.Role, &q.Question, &q.Type, &optionsStr, &answerStr, &q.Status, &assumedInt, &q.CreatedAt, &answeredAt); err != nil {
		return nil, fmt.Errorf("scanning question: %w", err)
	}
	if answerStr != "" {
		q.Answer = &answerStr
	}
	if assumedInt == 1 && answerStr != "" {
		q.Assumption = &answerStr
	}
	if answeredAt.Valid {
		q.AnsweredAt = &answeredAt.Time
	}
	json.Unmarshal([]byte(optionsStr), &q.Options)
	return &q, nil
}