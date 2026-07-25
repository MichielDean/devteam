package db

import (
	"database/sql"
	"fmt"
	"time"
)

// RepoRegistryRow is one entry in the canonical repo_registry table
// (feature github-authorization-integration, U-04, FR-DISC-02).
type RepoRegistryRow struct {
	ID             int64     `json:"id"`
	Owner          string    `json:"owner"`
	Name           string    `json:"name"`
	FullName       string    `json:"full_name"`
	DefaultBranch  string    `json:"default_branch"`
	InstallationID int64     `json:"installation_id"`
	Managed        int       `json:"managed"` // 0 = discovered/unmanaged, 1 = managed
	DiscoveredAt   time.Time `json:"discovered_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// UpsertRepoRegistry inserts or updates a repo_registry row for the given
// (owner, name). On conflict it updates default_branch + installation_id +
// updated_at (the discovered fields). `managed` is preserved across re-discovery
// (the operator's curation is not reset by a sync — interaction-spec §3.4).
// Returns the row's ID and whether the row was newly inserted.
func (db *DB) UpsertRepoRegistry(owner, name, fullName, defaultBranch string, installationID int64) (id int64, inserted bool, err error) {
	now := time.Now().UTC()
	// Try insert first.
	res, err := db.Exec(
		`INSERT INTO repo_registry (owner, name, full_name, default_branch, installation_id, managed, discovered_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 0, ?, ?)
		 ON CONFLICT (owner, name) DO UPDATE SET
		   default_branch = EXCLUDED.default_branch,
		   installation_id = EXCLUDED.installation_id,
		   full_name = EXCLUDED.full_name,
		   updated_at = EXCLUDED.updated_at`,
		owner, name, fullName, defaultBranch, installationID, now, now,
	)
	if err != nil {
		return 0, false, fmt.Errorf("upserting repo_registry (%s/%s): %w", owner, name, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		// ON CONFLICT path — fetch existing id.
		err = db.QueryRow(`SELECT id FROM repo_registry WHERE owner = ? AND name = ?`, owner, name).Scan(&id)
		if err != nil {
			return 0, false, fmt.Errorf("fetching repo_registry id after upsert (%s/%s): %w", owner, name, err)
		}
		return id, false, nil
	}
	// Insert path — fetch the new id.
	err = db.QueryRow(`SELECT id FROM repo_registry WHERE owner = ? AND name = ?`, owner, name).Scan(&id)
	if err != nil {
		return 0, false, fmt.Errorf("fetching repo_registry id after insert (%s/%s): %w", owner, name, err)
	}
	return id, true, nil
}

// SetRepoManaged flips the managed flag on a repo_registry row (FR-DISC-06,
// `devteam repo manage`, interaction-spec §3.4). Returns sql.ErrNoRows if the
// repo is not in the registry (the caller surfaces the F-3 error branch).
func (db *DB) SetRepoManaged(owner, name string, managed int) error {
	res, err := db.Exec(
		`UPDATE repo_registry SET managed = ?, updated_at = ? WHERE owner = ? AND name = ?`,
		managed, time.Now().UTC(), owner, name,
	)
	if err != nil {
		return fmt.Errorf("setting managed=%d on (%s/%s): %w", managed, owner, name, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("repo %s/%s not found in registry", owner, name)
	}
	return nil
}

// ListRepoRegistry returns all registry rows, optionally filtered by managed.
func (db *DB) ListRepoRegistry(managedFilter *int) ([]RepoRegistryRow, error) {
	var rows *sql.Rows
	var err error
	if managedFilter != nil {
		rows, err = db.Query(
			`SELECT id, owner, name, full_name, default_branch, installation_id, managed, discovered_at, updated_at
			 FROM repo_registry WHERE managed = ? ORDER BY managed DESC, owner ASC, name ASC`,
			*managedFilter,
		)
	} else {
		rows, err = db.Query(
			`SELECT id, owner, name, full_name, default_branch, installation_id, managed, discovered_at, updated_at
			 FROM repo_registry ORDER BY managed DESC, owner ASC, name ASC`,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("listing repo_registry: %w", err)
	}
	defer rows.Close()

	var out []RepoRegistryRow
	for rows.Next() {
		var r RepoRegistryRow
		if err := rows.Scan(&r.ID, &r.Owner, &r.Name, &r.FullName, &r.DefaultBranch, &r.InstallationID, &r.Managed, &r.DiscoveredAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning repo_registry row: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}

// GetRepoRegistry returns a single registry row by owner/name.
func (db *DB) GetRepoRegistry(owner, name string) (*RepoRegistryRow, error) {
	var r RepoRegistryRow
	err := db.QueryRow(
		`SELECT id, owner, name, full_name, default_branch, installation_id, managed, discovered_at, updated_at
		 FROM repo_registry WHERE owner = ? AND name = ?`,
		owner, name,
	).Scan(&r.ID, &r.Owner, &r.Name, &r.FullName, &r.DefaultBranch, &r.InstallationID, &r.Managed, &r.DiscoveredAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting repo_registry (%s/%s): %w", owner, name, err)
	}
	return &r, nil
}

// RepoRegistryEmpty reports whether the repo_registry table has zero rows.
// Used by the startup seed path (U-07): if empty, seed from repos.yaml.
func (db *DB) RepoRegistryEmpty() (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM repo_registry`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("counting repo_registry: %w", err)
	}
	return count == 0, nil
}