package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 17,
		Name:    "repos_registry",
		Up:      migration017ReposRegistry,
	})
}

// migration017ReposRegistry creates the `repos` table — the DB-backed
// implementation-repo registry. This replaces the embryonic repos.yaml-only
// read in `listRepos` with a first-class CRUD surface.
//
// NOTE on version number: upstream artifacts (app-design §2.1, business-logic-
// model INV-6/INV-7, unit-of-work U1) all reference `migration_013`. That
// number was taken by `bolt_depends_on` on the main branch (and 14/15/16 by
// follow-on migrations). This migration is numbered 17 to avoid colliding
// with already-applied migrations regardless of merge order. The schema,
// index strategy, and store are unchanged from upstream — only the version
// number moves. See architecture-review (3.3) finding N1.
//
// Schema notes (tracing app-design §2.1, business-logic-model INV-6/INV-7):
//   - `name` is the natural PRIMARY KEY (ADR-001). No surrogate id.
//   - `branch` defaults to 'main' — repos.yaml (the seed source) lacks a
//     branch field, so the seed hook fills this default.
//   - `description` defaults to '' (kept — D4 reversed; IntakeForm consumes it).
//   - `"primary"` is quoted because `primary` is a Postgres reserved word.
//     Defaults to false; operator-settable from ReposPage (N4).
//   - `created_at` / `updated_at` are server-managed (C9). updated_at is
//     bumped on every UPDATE by the store, not by a DB trigger — keeps the
//     migration forward-only and the logic in one place (repo_store.go).
//   - No FK from feature_repos.name → repos.name (ADR-003). The delete-guard
//     is a runtime check in repo_store, not a DB constraint. This keeps the
//     registry decoupled from the feature-attachment lifecycle.
//   - No reference_count column (INV-7). It is computed at read time via a
//     LEFT JOIN COUNT in ListRepos.
//
// Index on feature_repos.name: the delete-guard query
// `SELECT COUNT(*) FROM feature_repos WHERE name = ?` and
// `ListReferencingFeatures` hit this column. migration_004 only indexes
// feature_id. Adding an index here authorizes the perf path flagged by the
// 3.2/3.3 architecture review (N4 — Option A: add the index in this migration).
func migration017ReposRegistry(tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS repos (
			name        TEXT PRIMARY KEY,
			url         TEXT NOT NULL,
			branch      TEXT NOT NULL DEFAULT 'main',
			description TEXT NOT NULL DEFAULT '',
			"primary"   BOOLEAN NOT NULL DEFAULT false,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		// The delete-guard queries feature_repos.name, which migration_004
		// does not index (it indexes feature_id only). Add it here so
		// CountRepoReferences / ListReferencingFeatures hit an index.
		`CREATE INDEX IF NOT EXISTS idx_feature_repos_name ON feature_repos(name)`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("executing statement: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}