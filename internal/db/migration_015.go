package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 15,
		Name:    "stage_logs_bolt_number",
		Up:      migration015StageLogsBoltNumber,
	})
}

// migration015StageLogsBoltNumber makes stage_logs per-Bolt for construction
// stages 3.1-3.5, mirroring migration_014's change to feature_stages.
// Non-construction logs stay at bolt_number=0.
func migration015StageLogsBoltNumber(tx *sql.Tx) error {
	// 1. Add the column if missing.
	var colExists int
	row := tx.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`, "stage_logs", "bolt_number")
	if err := row.Scan(&colExists); err != nil {
		return fmt.Errorf("checking bolt_number column: %w", err)
	}
	if colExists == 0 {
		if _, err := tx.Exec(`ALTER TABLE stage_logs ADD COLUMN bolt_number INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("adding bolt_number column: %w", err)
		}
	}

	// 2. Drop the old unique constraint. Find it dynamically by column set.
	var constraintName string
	err := tx.QueryRow(`
		SELECT conname FROM pg_constraint
		WHERE conrelid = 'stage_logs'::regclass
		  AND contype = 'u'
		  AND array_to_string(conkey, ',') = (
		    SELECT array_to_string(array_agg(attnum ORDER BY attnum), ',')
		    FROM pg_attribute
		    WHERE attrelid = 'stage_logs'::regclass
		      AND attname IN ('feature_id', 'stage_id')
		  )`).Scan(&constraintName)
	if err == nil && constraintName != "" {
		if _, err := tx.Exec(fmt.Sprintf(`ALTER TABLE stage_logs DROP CONSTRAINT IF EXISTS %s`, constraintName)); err != nil {
			return fmt.Errorf("dropping old unique constraint %s: %w", constraintName, err)
		}
	} else if err != sql.ErrNoRows {
		_ = err
	}

	// 3. Add the new composite unique constraint.
	if _, err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_stage_logs_unique ON stage_logs(feature_id, stage_id, bolt_number)`); err != nil {
		return fmt.Errorf("creating composite unique index: %w", err)
	}

	return nil
}
