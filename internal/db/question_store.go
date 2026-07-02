package db

import (
	"database/sql"
	"fmt"
	"time"
)

// QuestionRow is the database representation of a question.
type QuestionRow struct {
	ID          string     `json:"id"`
	FeatureID   string     `json:"feature_id"`
	Phase       string     `json:"phase"`
	Role        string     `json:"role"`
	Question    string     `json:"question"`
	Type        string     `json:"type"`
	Options     string     `json:"options"`
	Answer      string     `json:"answer"`
	Status      string     `json:"status"`
	Assumed     bool       `json:"assumed"`
	CreatedAt   time.Time  `json:"created_at"`
	AnsweredAt  *time.Time `json:"answered_at,omitempty"`
}

// CreateQuestion inserts a new question.
func (db *DB) CreateQuestion(q QuestionRow) error {
	_, err := db.Exec(
		`INSERT INTO questions (id, feature_id, phase, role, question, question_type, options, answer, status, assumed, created_at, answered_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET
		   feature_id = excluded.feature_id, phase = excluded.phase, role = excluded.role,
		   question = excluded.question, question_type = excluded.question_type, options = excluded.options,
		   answer = excluded.answer, status = excluded.status, assumed = excluded.assumed,
		   created_at = excluded.created_at, answered_at = excluded.answered_at`,
		q.ID, q.FeatureID, q.Phase, q.Role, q.Question, q.Type, q.Options, q.Answer, q.Status, boolToInt(q.Assumed), q.CreatedAt, q.AnsweredAt,
	)
	if err != nil {
		return fmt.Errorf("creating question: %w", err)
	}
	return nil
}

// GetQuestion retrieves a question by ID.
func (db *DB) GetQuestion(id string) (*QuestionRow, error) {
	row := db.QueryRow(
		`SELECT id, feature_id, phase, role, question, question_type, options, answer, status, assumed, created_at, answered_at
		 FROM questions WHERE id = ?`, id,
	)

	var q QuestionRow
	var assumedInt int
	var answeredAt sql.NullTime
	err := row.Scan(&q.ID, &q.FeatureID, &q.Phase, &q.Role, &q.Question, &q.Type, &q.Options, &q.Answer, &q.Status, &assumedInt, &q.CreatedAt, &answeredAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting question %s: %w", id, err)
	}
	q.Assumed = assumedInt == 1
	if answeredAt.Valid {
		q.AnsweredAt = &answeredAt.Time
	}
	return &q, nil
}

// ListQuestions retrieves all questions for a feature.
func (db *DB) ListQuestions(featureID string) ([]QuestionRow, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, phase, role, question, question_type, options, answer, status, assumed, created_at, answered_at
		 FROM questions WHERE feature_id = ? ORDER BY created_at ASC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing questions: %w", err)
	}
	defer rows.Close()

	var questions []QuestionRow
	for rows.Next() {
		var q QuestionRow
		var assumedInt int
		var answeredAt sql.NullTime
		if err := rows.Scan(&q.ID, &q.FeatureID, &q.Phase, &q.Role, &q.Question, &q.Type, &q.Options, &q.Answer, &q.Status, &assumedInt, &q.CreatedAt, &answeredAt); err != nil {
			return nil, fmt.Errorf("scanning question: %w", err)
		}
		q.Assumed = assumedInt == 1
		if answeredAt.Valid {
			q.AnsweredAt = &answeredAt.Time
		}
		questions = append(questions, q)
	}
	return questions, nil
}

// ListPendingQuestions retrieves all pending questions for a feature.
func (db *DB) ListPendingQuestions(featureID string) ([]QuestionRow, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, phase, role, question, question_type, options, answer, status, assumed, created_at, answered_at
		 FROM questions WHERE feature_id = ? AND status = 'pending' ORDER BY created_at ASC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing pending questions: %w", err)
	}
	defer rows.Close()

	var questions []QuestionRow
	for rows.Next() {
		var q QuestionRow
		var assumedInt int
		var answeredAt sql.NullTime
		if err := rows.Scan(&q.ID, &q.FeatureID, &q.Phase, &q.Role, &q.Question, &q.Type, &q.Options, &q.Answer, &q.Status, &assumedInt, &q.CreatedAt, &answeredAt); err != nil {
			return nil, fmt.Errorf("scanning pending question: %w", err)
		}
		q.Assumed = assumedInt == 1
		if answeredAt.Valid {
			q.AnsweredAt = &answeredAt.Time
		}
		questions = append(questions, q)
	}
	return questions, nil
}

// PendingQuestionCount returns the number of pending questions for a feature.
func (db *DB) PendingQuestionCount(featureID string) (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM questions WHERE feature_id = ? AND status = 'pending'`, featureID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting pending questions: %w", err)
	}
	return count, nil
}

// AnswerQuestion updates a question with the user's answer.
func (db *DB) AnswerQuestion(id, answer string) error {
	_, err := db.Exec(
		`UPDATE questions SET answer = ?, status = 'answered', answered_at = ? WHERE id = ?`,
		answer, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("answering question %s: %w", id, err)
	}
	return nil
}

// AssumeQuestion marks a question as auto-assumed (timeout).
func (db *DB) AssumeQuestion(id, assumedAnswer string) error {
	_, err := db.Exec(
		`UPDATE questions SET answer = ?, status = 'assumed', assumed = 1, answered_at = ? WHERE id = ?`,
		assumedAnswer, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("assuming question %s: %w", id, err)
	}
	return nil
}