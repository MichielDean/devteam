package db

import (
	"database/sql"
	"fmt"
	"time"
)

func init() {
	RegisterMigration(Migration{
		Version: 16,
		Name:    "migrate_bolt0_to_bolt1",
		Up:      migration016MigrateBolt0ToBolt1,
	})
}

// migration016MigrateBolt0ToBolt1 cleans up pre-PR-70 data: features that had
// per-Bolt construction stages (3.1-3.5) created at bolt_number=0 by the old
// InitFeatureStages. For each such feature:
//
//  1. Create a bolt record (bolt 1, walking skeleton) if none exists.
//  2. Move the bolt=0 feature_stages rows for 3.1-3.5 to bolt_number=1.
//  3. Move the bolt=0 stage_logs rows for 3.1-3.5 to bolt_number=1.
//  4. Set the feature's current_bolt to 1 (so RunStage resolves the right row).
//
// After this migration, NO per-Bolt stage rows exist at bolt_number=0. The
// per-Bolt model is clean and consistent — no legacy mode needed.
func migration016MigrateBolt0ToBolt1(tx *sql.Tx) error {
	// 1. Find features that have per-Bolt stages at bolt_number=0.
	rows, err := tx.Query(`
		SELECT DISTINCT feature_id
		FROM feature_stages
		WHERE stage_id IN ('3.1','3.2','3.3','3.4','3.5')
		  AND bolt_number = 0`)
	if err != nil {
		return fmt.Errorf("finding legacy bolt=0 features: %w", err)
	}
	var featureIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scanning feature id: %w", err)
		}
		featureIDs = append(featureIDs, id)
	}
	rows.Close()

	for _, fid := range featureIDs {
		// 2. Create bolt 1 record (walking skeleton) if no bolts exist for this feature.
		var boltCount int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM bolts WHERE feature_id = $1`, fid).Scan(&boltCount); err != nil {
			return fmt.Errorf("counting bolts for %s: %w", fid, err)
		}
		if boltCount == 0 {
			now := time.Now().UTC()
			units := `["unit-1"]`
			if _, err := tx.Exec(
				`INSERT INTO bolts (feature_id, bolt_number, unit_ids, depends_on, status, is_walking_skeleton, created_at)
				 VALUES ($1, 1, $2, '[]', 'completed', 1, $3)`,
				fid, units, now,
			); err != nil {
				return fmt.Errorf("creating bolt 1 for %s: %w", fid, err)
			}
		}

		// 3. Move feature_stages bolt=0 rows for 3.1-3.5 to bolt_number=1.
		//    Delete any existing bolt=1 rows first (shouldn't exist, but be safe).
		if _, err := tx.Exec(
			`DELETE FROM feature_stages
			 WHERE feature_id = $1 AND stage_id IN ('3.1','3.2','3.3','3.4','3.5') AND bolt_number = 1`,
			fid,
		); err != nil {
			return fmt.Errorf("deleting stale bolt=1 rows for %s: %w", fid, err)
		}
		if _, err := tx.Exec(
			`UPDATE feature_stages SET bolt_number = 1
			 WHERE feature_id = $1 AND stage_id IN ('3.1','3.2','3.3','3.4','3.5') AND bolt_number = 0`,
			fid,
		); err != nil {
			return fmt.Errorf("migrating feature_stages bolt=0→1 for %s: %w", fid, err)
		}

		// 4. Move stage_logs bolt=0 rows for 3.1-3.5 to bolt_number=1.
		if _, err := tx.Exec(
			`DELETE FROM stage_logs
			 WHERE feature_id = $1 AND stage_id IN ('3.1','3.2','3.3','3.4','3.5') AND bolt_number = 1`,
			fid,
		); err != nil {
			return fmt.Errorf("deleting stale stage_logs bolt=1 for %s: %w", fid, err)
		}
		if _, err := tx.Exec(
			`UPDATE stage_logs SET bolt_number = 1
			 WHERE feature_id = $1 AND stage_id IN ('3.1','3.2','3.3','3.4','3.5') AND bolt_number = 0`,
			fid,
		); err != nil {
			return fmt.Errorf("migrating stage_logs bolt=0→1 for %s: %w", fid, err)
		}

		// 5. Set the feature's current_bolt to 1. The features table stores state
		//    in a TEXT column (feature_data) holding JSON. Parse to jsonb, patch,
		//    cast back to text.
		var featureData string
		err := tx.QueryRow(`SELECT feature_data FROM features WHERE id = $1`, fid).Scan(&featureData)
		if err == nil && featureData != "" {
			if _, err := tx.Exec(
				`UPDATE features SET feature_data = jsonb_set(
				   COALESCE($2::jsonb, '{}'::jsonb),
				   '{current_bolt}',
				   '1',
				   true
				 )::text WHERE id = $1`,
				fid, featureData,
			); err != nil {
				return fmt.Errorf("setting current_bolt for %s: %w", fid, err)
			}
		}
	}

	return nil
}
