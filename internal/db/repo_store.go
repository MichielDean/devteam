package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Sentinel errors for the repo store. These are matched by the API layer to
// produce the correct HTTP status (404 / 409). Internal callers should use
// errors.Is to distinguish not-found from conflict.
var (
	// ErrRepoNotFound is returned by GetRepo/UpdateRepo/DeleteRepo when no row
	// matches the given name.
	ErrRepoNotFound = errors.New("repo not found")
	// ErrRepoExists is returned by CreateRepo when a repo with the given name
	// already exists (natural-key conflict).
	ErrRepoExists = errors.New("repo already exists")
)

// RepoRow is a row of the `repos` registry table.
//
// The eight fields map 1:1 to the additive Registry DTO (app-design §2.3,
// C3′): name, url, branch, description, primary, created_at, updated_at, and
// the computed reference_count (populated only by ListRepos via a LEFT JOIN
// COUNT — single-row reads leave it as the zero value).
type RepoRow struct {
	Name           string    `json:"name"`
	URL            string    `json:"url"`
	Branch         string    `json:"branch"`
	Description    string    `json:"description"`
	Primary        bool      `json:"primary"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	ReferenceCount int       `json:"reference_count"`
}

// ListRepos returns every registry row ordered by name, with each row's
// reference_count computed as the number of feature_repos rows that share the
// repo's name. Returns an empty (non-nil) slice when the registry is empty so
// JSON serialization emits `[]` not `null` (developer.md failure-mode guard).
func (db *DB) ListRepos() ([]RepoRow, error) {
	rows, err := db.Query(
		`SELECT r.name, r.url, r.branch, r.description, r."primary",
		        r.created_at, r.updated_at,
		        COUNT(fr.feature_id) AS reference_count
		 FROM repos r
		 LEFT JOIN feature_repos fr ON fr.name = r.name
		 GROUP BY r.name, r.url, r.branch, r.description, r."primary",
		          r.created_at, r.updated_at
		 ORDER BY r.name`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing repos: %w", err)
	}
	defer rows.Close()

	repos := []RepoRow{}
	for rows.Next() {
		var r RepoRow
		if err := rows.Scan(
			&r.Name, &r.URL, &r.Branch, &r.Description, &r.Primary,
			&r.CreatedAt, &r.UpdatedAt, &r.ReferenceCount,
		); err != nil {
			return nil, fmt.Errorf("scanning repo row: %w", err)
		}
		repos = append(repos, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating repo rows: %w", err)
	}
	return repos, nil
}

// GetRepo returns a single registry row by name. The reference_count is not
// populated here (single-row reads do not need it; the API layer uses
// CountRepoReferences when a count is required).
func (db *DB) GetRepo(name string) (*RepoRow, error) {
	var r RepoRow
	err := db.QueryRow(
		`SELECT name, url, branch, description, "primary", created_at, updated_at
		 FROM repos WHERE name = ?`,
		name,
	).Scan(&r.Name, &r.URL, &r.Branch, &r.Description, &r.Primary, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRepoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting repo %q: %w", name, err)
	}
	return &r, nil
}

// CreateRepo inserts a new registry row. Returns ErrRepoExists if a repo with
// the given name is already present (ON CONFLICT DO NOTHING + rows-affected
// check, so the natural-key collision is reported as a typed error rather than
// a raw Postgres unique-violation).
func (db *DB) CreateRepo(name, url, branch, description string, primary bool) (*RepoRow, error) {
	if branch == "" {
		branch = "main"
	}
	if description == "" {
		description = ""
	}
	res, err := db.Exec(
		`INSERT INTO repos (name, url, branch, description, "primary", created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, now(), now())
		 ON CONFLICT (name) DO NOTHING`,
		name, url, branch, description, primary,
	)
	if err != nil {
		return nil, fmt.Errorf("creating repo %q: %w", name, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("checking rows affected for create repo %q: %w", name, err)
	}
	if affected == 0 {
		return nil, ErrRepoExists
	}
	return db.GetRepo(name)
}

// UpdateRepo mutates the editable fields of an existing repo. The name is
// immutable (natural PK, ADR-001) and is used only as the WHERE key. Bumps
// updated_at server-side. Returns ErrRepoNotFound when no row matches.
func (db *DB) UpdateRepo(name, url, branch, description string, primary bool) (*RepoRow, error) {
	res, err := db.Exec(
		`UPDATE repos
		 SET url = ?, branch = ?, description = ?, "primary" = ?, updated_at = now()
		 WHERE name = ?`,
		url, branch, description, primary, name,
	)
	if err != nil {
		return nil, fmt.Errorf("updating repo %q: %w", name, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("checking rows affected for update repo %q: %w", name, err)
	}
	if affected == 0 {
		return nil, ErrRepoNotFound
	}
	return db.GetRepo(name)
}

// DeleteRepo removes a registry row by name. It does NOT check references —
// the caller (the API delete-guard handler) must call CountRepoReferences
// first and decide. Returns ErrRepoNotFound when no row matches. This keeps
// the guard logic in the API layer and the store a thin data primitive.
func (db *DB) DeleteRepo(name string) error {
	res, err := db.Exec(`DELETE FROM repos WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("deleting repo %q: %w", name, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected for delete repo %q: %w", name, err)
	}
	if affected == 0 {
		return ErrRepoNotFound
	}
	return nil
}

// CountRepoReferences returns the number of feature_repos rows that reference
// the given repo name. This is the delete-guard primitive: the API layer
// blocks deletion when this returns > 0. The query hits idx_feature_repos_name
// (created in migration_013).
func (db *DB) CountRepoReferences(name string) (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM feature_repos WHERE name = ?`,
		name,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting references for repo %q: %w", name, err)
	}
	return count, nil
}

// ListReferencingFeatures returns the feature_ids of feature_repos rows that
// reference the given repo name. Used by the delete-guard to populate the 409
// `{"error":"repo_in_use","features":[…]}` body (O3′ ratified). Returns an
// empty (non-nil) slice when no feature references the repo.
func (db *DB) ListReferencingFeatures(name string) ([]string, error) {
	rows, err := db.Query(
		`SELECT DISTINCT feature_id FROM feature_repos WHERE name = ?`,
		name,
	)
	if err != nil {
		return nil, fmt.Errorf("listing referencing features for repo %q: %w", name, err)
	}
	defer rows.Close()

	features := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning referencing feature: %w", err)
		}
		features = append(features, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating referencing features: %w", err)
	}
	return features, nil
}

// CountRepos returns the total number of registry rows. Used by the seed hook
// to decide whether to seed (count=0 → seed, count>0 → skip).
func (db *DB) CountRepos() (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM repos`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting repos: %w", err)
	}
	return count, nil
}