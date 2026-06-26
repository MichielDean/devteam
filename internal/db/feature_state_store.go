package db

import (
	"encoding/json"
	"fmt"
	"time"
)

// SaveFeatureData stores the full feature state as JSON in the feature_data column.
// The caller serializes the Feature struct to JSON and passes it here.
// Also updates scalar columns for queryability. Creates the row if missing.
func (db *DB) SaveFeatureData(featureID, title, currentPhase, status string, priority int, intakePath, specDir, worktreeDir string, createdAt time.Time, recirculationCount int, jsonData []byte) error {
	now := time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, worktree_dir, created_at, updated_at, recirculation_count, feature_data)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   title = excluded.title,
		   current_phase = excluded.current_phase,
		   status = excluded.status,
		   priority = excluded.priority,
		   intake_path = excluded.intake_path,
		   spec_dir = excluded.spec_dir,
		   worktree_dir = excluded.worktree_dir,
		   updated_at = excluded.updated_at,
		   recirculation_count = excluded.recirculation_count,
		   feature_data = excluded.feature_data`,
		featureID, title, currentPhase, status, priority, intakePath, specDir, worktreeDir, createdAt, now, recirculationCount, string(jsonData),
	)
	if err != nil {
		return fmt.Errorf("saving feature data: %w", err)
	}
	return nil
}

// LoadFeatureData reads the raw JSON feature_data column.
// The caller deserializes it into a Feature struct.
func (db *DB) LoadFeatureData(featureID string) ([]byte, error) {
	var data string
	err := db.QueryRow(`SELECT feature_data FROM features WHERE id = ?`, featureID).Scan(&data)
	if err != nil {
		return nil, fmt.Errorf("loading feature data for %s: %w", featureID, err)
	}
	if data == "" {
		return nil, fmt.Errorf("feature %s has no state data", featureID)
	}
	return []byte(data), nil
}

// ListAllFeatureData returns all feature JSON blobs ordered by updated_at desc.
func (db *DB) ListAllFeatureData() ([]json.RawMessage, error) {
	rows, err := db.Query(`SELECT feature_data FROM features WHERE feature_data != '' ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing feature data: %w", err)
	}
	defer rows.Close()

	var features []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scanning feature: %w", err)
		}
		features = append(features, json.RawMessage(data))
	}
	return features, nil
}