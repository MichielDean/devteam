package db

import (
	"database/sql"
	"fmt"
	"time"
)

// FeatureRow is the database representation of a feature.
type FeatureRow struct {
	ID                string    `json:"id"`
	Title             string    `json:"title"`
	CurrentPhase      string    `json:"current_phase"`
	Status            string    `json:"status"`
	Priority          int       `json:"priority"`
	IntakePath        string    `json:"intake_path"`
	SpecDir           string    `json:"spec_dir"`
	WorktreeDir       string    `json:"worktree_dir"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	RecirculationCount int      `json:"recirculation_count"`
}

// PhaseStateRow is the database representation of a phase state.
type PhaseStateRow struct {
	Phase       string     `json:"phase"`
	Status      string     `json:"status"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// CreateFeature inserts a new feature.
func (db *DB) CreateFeature(f FeatureRow) error {
	_, err := db.Exec(
		`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, worktree_dir, created_at, updated_at, recirculation_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.Title, f.CurrentPhase, f.Status, f.Priority, f.IntakePath, f.SpecDir, f.WorktreeDir, f.CreatedAt, f.UpdatedAt, f.RecirculationCount,
	)
	if err != nil {
		return fmt.Errorf("creating feature: %w", err)
	}

	// Create phase states for all phases
	for _, phase := range []string{"inception", "planning", "construction", "review", "testing", "delivery"} {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO phase_states (feature_id, phase, status) VALUES (?, ?, 'draft')`,
			f.ID, phase,
		); err != nil {
			return fmt.Errorf("creating phase state for %s: %w", phase, err)
		}
	}

	return nil
}

// GetFeature retrieves a feature by ID.
func (db *DB) GetFeature(id string) (*FeatureRow, error) {
	row := db.QueryRow(
		`SELECT id, title, current_phase, status, priority, intake_path, spec_dir, worktree_dir, created_at, updated_at, recirculation_count
		 FROM features WHERE id = ?`, id,
	)

	var f FeatureRow
	var worktreeDir sql.NullString
	err := row.Scan(&f.ID, &f.Title, &f.CurrentPhase, &f.Status, &f.Priority, &f.IntakePath, &f.SpecDir, &worktreeDir, &f.CreatedAt, &f.UpdatedAt, &f.RecirculationCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting feature %s: %w", id, err)
	}
	f.WorktreeDir = worktreeDir.String
	return &f, nil
}

// ListFeatures retrieves all features ordered by updated_at desc.
func (db *DB) ListFeatures() ([]FeatureRow, error) {
	rows, err := db.Query(
		`SELECT id, title, current_phase, status, priority, intake_path, spec_dir, worktree_dir, created_at, updated_at, recirculation_count
		 FROM features ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing features: %w", err)
	}
	defer rows.Close()

	var features []FeatureRow
	for rows.Next() {
		var f FeatureRow
		var worktreeDir sql.NullString
		if err := rows.Scan(&f.ID, &f.Title, &f.CurrentPhase, &f.Status, &f.Priority, &f.IntakePath, &f.SpecDir, &worktreeDir, &f.CreatedAt, &f.UpdatedAt, &f.RecirculationCount); err != nil {
			return nil, fmt.Errorf("scanning feature: %w", err)
		}
		f.WorktreeDir = worktreeDir.String
		features = append(features, f)
	}
	return features, nil
}

// UpdateFeature updates a feature's mutable fields.
func (db *DB) UpdateFeature(f FeatureRow) error {
	_, err := db.Exec(
		`UPDATE features SET title = ?, current_phase = ?, status = ?, priority = ?, worktree_dir = ?, updated_at = ?, recirculation_count = ?
		 WHERE id = ?`,
		f.Title, f.CurrentPhase, f.Status, f.Priority, f.WorktreeDir, time.Now().UTC(), f.RecirculationCount, f.ID,
	)
	if err != nil {
		return fmt.Errorf("updating feature %s: %w", f.ID, err)
	}
	return nil
}

// UpdateFeatureStatus updates just the status and current phase.
func (db *DB) UpdateFeatureStatus(id, status, currentPhase string) error {
	_, err := db.Exec(
		`UPDATE features SET status = ?, current_phase = ?, updated_at = ? WHERE id = ?`,
		status, currentPhase, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("updating feature status %s: %w", id, err)
	}
	return nil
}

// UpdateWorktreeDir sets the worktree directory for a feature.
func (db *DB) UpdateWorktreeDir(id, worktreeDir string) error {
	_, err := db.Exec(
		`UPDATE features SET worktree_dir = ?, updated_at = ? WHERE id = ?`,
		worktreeDir, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("updating worktree dir %s: %w", id, err)
	}
	return nil
}

// GetPhaseStates retrieves all phase states for a feature.
func (db *DB) GetPhaseStates(featureID string) (map[string]PhaseStateRow, error) {
	rows, err := db.Query(
		`SELECT phase, status, started_at, completed_at FROM phase_states WHERE feature_id = ?`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting phase states: %w", err)
	}
	defer rows.Close()

	states := make(map[string]PhaseStateRow)
	for rows.Next() {
		var ps PhaseStateRow
		var startedAt, completedAt sql.NullTime
		if err := rows.Scan(&ps.Phase, &ps.Status, &startedAt, &completedAt); err != nil {
			return nil, fmt.Errorf("scanning phase state: %w", err)
		}
		if startedAt.Valid {
			ps.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			ps.CompletedAt = &completedAt.Time
		}
		states[ps.Phase] = ps
	}
	return states, nil
}

// UpdatePhaseState updates a phase state for a feature.
func (db *DB) UpdatePhaseState(featureID, phase, status string, startedAt, completedAt *time.Time) error {
	_, err := db.Exec(
		`UPDATE phase_states SET status = ?, started_at = ?, completed_at = ? WHERE feature_id = ? AND phase = ?`,
		status, startedAt, completedAt, featureID, phase,
	)
	if err != nil {
		return fmt.Errorf("updating phase state %s/%s: %w", featureID, phase, err)
	}
	return nil
}

// DeleteFeature removes a feature and all related data (cascade).
func (db *DB) DeleteFeature(id string) error {
	_, err := db.Exec(`DELETE FROM features WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting feature %s: %w", id, err)
	}
	return nil
}

// GetRecirculationCount returns the number of recirculations for a feature.
func (db *DB) GetRecirculationCount(featureID string) (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM recirculations WHERE feature_id = ?`, featureID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("getting recirculation count: %w", err)
	}
	return count, nil
}