package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 5,
		Name:    "aidlc_v2_stages_audit_bolts",
		Up:      migration005AIDLCv2,
	})
}

// migration005AIDLCv2 creates the AIDLC v2 stage-based workflow tables:
//   - stage_definitions: the 32 static stages (seeded by the application, not here)
//   - feature_stages: per-feature per-stage state (replaces phase_states)
//   - audit_events: 68-event audit trail (replaces events table)
//   - bolts: construction Bolt records
func migration005AIDLCv2(tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS stage_definitions (
			id TEXT PRIMARY KEY,
			phase TEXT NOT NULL,
			name TEXT NOT NULL,
			lead_agent TEXT NOT NULL,
			supporting_agents TEXT DEFAULT '[]',
			key_artifacts TEXT DEFAULT '[]',
			condition TEXT NOT NULL DEFAULT 'ALWAYS',
			scopes TEXT DEFAULT '[]',
			reviewer TEXT DEFAULT '',
			sort_order INTEGER NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS feature_stages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			stage_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'not_started',
			revision_count INTEGER NOT NULL DEFAULT 0,
			started_at TIMESTAMP,
			completed_at TIMESTAMP,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE,
			UNIQUE(feature_id, stage_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_feature_stages_feature ON feature_stages(feature_id)`,

		`CREATE TABLE IF NOT EXISTS audit_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			stage_id TEXT DEFAULT '',
			phase TEXT DEFAULT '',
			details TEXT DEFAULT '',
			created_at TIMESTAMP NOT NULL,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_events_feature ON audit_events(feature_id, created_at)`,

		`CREATE TABLE IF NOT EXISTS bolts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			bolt_number INTEGER NOT NULL,
			unit_ids TEXT DEFAULT '[]',
			status TEXT NOT NULL DEFAULT 'pending',
			is_walking_skeleton INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_bolts_feature ON bolts(feature_id)`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("executing statement: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}