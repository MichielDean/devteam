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

// LoadFeatureData reads the feature_data JSON blob and merges scalar column
// values (status, current_phase, scope, etc.) on top, so column updates made
// directly via SQL (e.g. status='done') are reflected even if the blob is stale.
// The blob is the source of truth for nested fields (repos, dependencies);
// columns are the source of truth for scalars.
func (db *DB) LoadFeatureData(featureID string) ([]byte, error) {
	// Merge columns into the blob using jsonb: start from the blob, overlay
	// scalar columns. This ensures column updates always win.
	var merged string
	err := db.QueryRow(`
		SELECT COALESCE(
		  (COALESCE(NULLIF(feature_data, '')::jsonb, '{}'::jsonb)
		   || jsonb_build_object(
		     'id', id,
		     'title', title,
		     'status', status,
		     'current_phase', current_phase,
		     'scope', scope,
		     'depth', depth,
		     'test_strategy', test_strategy,
		     'execution_mode', execution_mode,
		     'autonomy_mode', autonomy_mode,
		     'priority', priority,
		     'updated_at', to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		   ))::text,
		  ''
		)
		FROM features WHERE id = ?`, featureID).Scan(&merged)
	if err != nil {
		return nil, fmt.Errorf("loading feature data for %s: %w", featureID, err)
	}
	if merged == "" || merged == "{}" {
		return nil, fmt.Errorf("feature %s has no state data", featureID)
	}
	return []byte(merged), nil
}

// ListAllFeatureData returns all features with scalar columns merged into the
// JSON blob, ordered by updated_at desc. This ensures the dashboard sees
// current column values even when the blob is stale.
func (db *DB) ListAllFeatureData() ([]json.RawMessage, error) {
	rows, err := db.Query(`
		SELECT COALESCE(
		  (COALESCE(NULLIF(feature_data, '')::jsonb, '{}'::jsonb)
		   || jsonb_build_object(
		     'id', id,
		     'title', title,
		     'status', status,
		     'current_phase', current_phase,
		     'scope', scope,
		     'depth', depth,
		     'test_strategy', test_strategy,
		     'execution_mode', execution_mode,
		     'autonomy_mode', autonomy_mode,
		     'priority', priority,
		     'updated_at', to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		   ))::text,
		  ''
		)
		FROM features
		WHERE feature_data != '' OR status != 'draft'
		ORDER BY updated_at DESC`)
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
		if data != "" && data != "{}" {
			features = append(features, json.RawMessage(data))
		}
	}
	return features, nil
}
