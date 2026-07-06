package db

import (
	"encoding/json"
	"fmt"
	"time"
)

// SaveFeatureData stores the full feature state as JSON in the feature_data column.
// Also updates scalar columns (including scope, depth, execution_mode, etc.)
// for queryability. Creates the row if missing.
func (db *DB) SaveFeatureData(featureID, title, currentPhase, status string, priority int, intakePath, specDir, worktreeDir string, createdAt time.Time, recirculationCount int, jsonData []byte) error {
	now := time.Now().UTC()
	// Extract scalar fields from the JSON blob so they're written to columns too.
	// SaveFeatureData is called with the full Feature struct serialized as JSON;
	// we parse out the fields that have dedicated columns.
	var fields struct {
		Scope         string `json:"scope"`
		Depth         string `json:"depth"`
		TestStrategy  string `json:"test_strategy"`
		ExecutionMode string `json:"execution_mode"`
		AutonomyMode  string `json:"autonomy_mode"`
	}
	json.Unmarshal(jsonData, &fields)

	_, err := db.Exec(
		`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, worktree_dir, created_at, updated_at, recirculation_count, feature_data, scope, depth, test_strategy, execution_mode, autonomy_mode)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		   feature_data = excluded.feature_data,
		   scope = COALESCE(NULLIF(excluded.scope, ''), features.scope, ''),
		   depth = COALESCE(NULLIF(excluded.depth, ''), features.depth, ''),
		   test_strategy = COALESCE(NULLIF(excluded.test_strategy, ''), features.test_strategy, ''),
		   execution_mode = COALESCE(NULLIF(excluded.execution_mode, ''), features.execution_mode, ''),
		   autonomy_mode = COALESCE(NULLIF(excluded.autonomy_mode, ''), features.autonomy_mode, '')`,
		featureID, title, currentPhase, status, priority, intakePath, specDir, worktreeDir, createdAt, now, recirculationCount, string(jsonData),
		fields.Scope, fields.Depth, fields.TestStrategy, fields.ExecutionMode, fields.AutonomyMode,
	)
	if err != nil {
		return fmt.Errorf("saving feature data: %w", err)
	}
	return nil
}

// LoadFeatureData reads the feature_data JSON blob and merges scalar column
// values on top. Column values only override the blob when the column is
// non-empty — empty columns don't wipe blob values. This handles both
// directions: direct SQL column updates (status='done') and blob-only saves
// (execution_mode stored in blob but not yet in column).
func (db *DB) LoadFeatureData(featureID string) ([]byte, error) {
	var merged string
	err := db.QueryRow(`
		SELECT COALESCE(
		  (COALESCE(NULLIF(feature_data, '')::jsonb, '{}'::jsonb)
		   || jsonb_strip_nulls(jsonb_build_object(
		     'id', id,
		     'title', title,
		     'status', status,
		     'current_phase', current_phase,
		     'scope', NULLIF(scope, ''),
		     'depth', NULLIF(depth, ''),
		     'test_strategy', NULLIF(test_strategy, ''),
		     'execution_mode', NULLIF(execution_mode, ''),
		     'autonomy_mode', NULLIF(autonomy_mode, ''),
		     'priority', priority
		   )))::text,
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
// JSON blob, ordered by updated_at desc. Empty columns don't overwrite blob
// values (jsonb_strip_nulls removes NULL entries before the merge).
func (db *DB) ListAllFeatureData() ([]json.RawMessage, error) {
	rows, err := db.Query(`
		SELECT COALESCE(
		  (COALESCE(NULLIF(feature_data, '')::jsonb, '{}'::jsonb)
		   || jsonb_strip_nulls(jsonb_build_object(
		     'id', id,
		     'title', title,
		     'status', status,
		     'current_phase', current_phase,
		     'scope', NULLIF(scope, ''),
		     'depth', NULLIF(depth, ''),
		     'test_strategy', NULLIF(test_strategy, ''),
		     'execution_mode', NULLIF(execution_mode, ''),
		     'autonomy_mode', NULLIF(autonomy_mode, ''),
		     'priority', priority
		   )))::text,
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
