package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 4,
		Name:    "outcomes_and_feature_repos",
		Up:      migration004OutcomesAndFeatureRepos,
	})
}

// migration004OutcomesAndFeatureRepos adds:
//  1. outcomes table — agent signals (pass/recirculate/needs_feedback/failed)
//     written by `devteam signal` CLI, read by the pipeline after dispatch.
//     Replaces outcome.txt on disk.
//  2. feature_repos table — prepared impl repo worktrees per feature.
//     Replaces the PreparedRepos slice on the Feature struct.
func migration004OutcomesAndFeatureRepos(tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS outcomes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			phase TEXT NOT NULL,
			outcome TEXT NOT NULL,
			target TEXT DEFAULT '',
			notes TEXT DEFAULT '',
			created_at TIMESTAMP NOT NULL,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_outcomes_feature ON outcomes(feature_id, phase)`,

		`CREATE TABLE IF NOT EXISTS feature_repos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			dir TEXT NOT NULL,
			branch TEXT NOT NULL,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE,
			UNIQUE(feature_id, name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_feature_repos_feature ON feature_repos(feature_id)`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("executing statement: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}