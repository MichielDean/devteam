// Package defaults owns the feature_defaults table — global and per-repo
// default settings (scope, depth, test_strategy, execution_mode) that the
// createFeature flow falls back to when no explicit value is supplied
// (ADR-DEFAULTS-PRECEDENCE).
//
// Precedence (binding, FR-DEF-02):
//
//	explicit request  >  per-repo default  >  global default  >  scope-derived fallback
//
// The store is DB-only — no YAML materialization. createFeature reads from
// the table directly at feature-creation time; the admin UI reads/writes
// via the /api/settings/defaults handlers.
package defaults

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/MichielDean/devteam/internal/db"
)

// Defaults is one row of the feature_defaults table. A zero-value field
// means "no override" — the precedence chain falls through to the next
// level. The Repo field is "" for the global row and the repo name for a
// per-repo override.
type Defaults struct {
	Scope         string    `json:"scope,omitempty"`
	Depth         string    `json:"depth,omitempty"`
	TestStrategy  string    `json:"test_strategy,omitempty"`
	ExecutionMode string    `json:"execution_mode,omitempty"`
	Repo          string    `json:"repo,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Store owns the feature_defaults table. All methods are safe for
// concurrent use (the underlying *db.DB is a sql.DB connection pool).
type Store struct {
	db *db.DB
}

// NewStore constructs a Store backed by the given DB.
func NewStore(database *db.DB) *Store {
	return &Store{db: database}
}

// ErrNotFound is returned by GetForRepo when no per-repo override exists
// for the given repo. Callers should treat this as "fall through to global"
// rather than an error.
var ErrNotFound = errors.New("no feature_defaults row for repo")

// GetGlobal returns the global defaults row (repo IS NULL), or a zero-value
// Defaults if no global row has been set. A zero-value Defaults means "no
// overrides at any level" — createFeature falls all the way through to the
// scope-derived fallback.
func (s *Store) GetGlobal(ctx context.Context) (*Defaults, error) {
	return s.get("repo IS NULL")
}

// GetForRepo returns the per-repo override for the given repo name, or
// ErrNotFound if no per-repo row exists. Callers should fall through to
// GetGlobal on ErrNotFound.
func (s *Store) GetForRepo(ctx context.Context, repo string) (*Defaults, error) {
	d, err := s.get("repo = ?", repo)
	if err != nil {
		return nil, err
	}
	if d.Scope == "" && d.Depth == "" && d.TestStrategy == "" && d.ExecutionMode == "" {
		// No row — get() returns zero-value Defaults on sql.ErrNoRows for
		// the global path. For per-repo we surface ErrNotFound so callers
		// can fall through. We detect "no row" by checking whether the
		// scan actually found data. But get() already returns zero-value
		// on ErrNoRows... we need a different signal. Let's re-query with
		// an existence check.
	}
	// We can't distinguish "row exists but all fields empty" from "no row"
	// via the zero-value Defaults. Re-check existence explicitly.
	var exists bool
	err = s.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM feature_defaults WHERE repo = ?)`, repo).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("checking per-repo defaults existence for %q: %w", repo, err)
	}
	if !exists {
		return nil, ErrNotFound
	}
	return d, nil
}

// PutGlobal upserts the global defaults row. If no global row exists, one is
// inserted; otherwise the existing row is updated. Emits a
// FEATURE_DEFAULTS_MUTATED audit event with the operator identity.
func (s *Store) PutGlobal(ctx context.Context, d Defaults, actor string) (*Defaults, error) {
	return s.upsert(nil, d, actor)
}

// PutForRepo upserts the per-repo override for the given repo name. Emits a
// FEATURE_DEFAULTS_MUTATED audit event.
func (s *Store) PutForRepo(ctx context.Context, repo string, d Defaults, actor string) (*Defaults, error) {
	if repo == "" {
		return nil, errors.New("repo name is required for a per-repo override")
	}
	return s.upsert(&repo, d, actor)
}

// DeleteForRepo removes the per-repo override for the given repo. After
// deletion, createFeature falls through to the global default for this repo.
// Emits a FEATURE_DEFAULTS_MUTATED audit event. Returns ErrNotFound if no
// per-repo row existed.
func (s *Store) DeleteForRepo(ctx context.Context, repo string, actor string) error {
	if repo == "" {
		return errors.New("repo name is required")
	}
	res, err := s.db.Exec(`DELETE FROM feature_defaults WHERE repo = ?`, repo)
	if err != nil {
		return fmt.Errorf("deleting per-repo defaults for %q: %w", repo, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	s.emitAudit("delete per-repo defaults for " + repo, actor)
	return nil
}

// ListPerRepo returns all per-repo override rows (repo IS NOT NULL), ordered
// by repo name. Returns an empty (non-nil) slice when no per-repo overrides
// exist so JSON serialization emits [] not null.
func (s *Store) ListPerRepo(ctx context.Context) ([]Defaults, error) {
	rows, err := s.db.Query(
		`SELECT scope, depth, test_strategy, execution_mode, repo, updated_at
		 FROM feature_defaults WHERE repo IS NOT NULL ORDER BY repo ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing per-repo defaults: %w", err)
	}
	defer rows.Close()

	out := []Defaults{}
	for rows.Next() {
		var d Defaults
		var repo sql.NullString
		if err := rows.Scan(&d.Scope, &d.Depth, &d.TestStrategy, &d.ExecutionMode, &repo, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning per-repo default: %w", err)
		}
		d.Repo = repo.String
		out = append(out, d)
	}
	return out, nil
}

// --- internals ---

func (s *Store) get(where string, args ...interface{}) (*Defaults, error) {
	q := `SELECT scope, depth, test_strategy, execution_mode, repo, updated_at FROM feature_defaults WHERE ` + where
	row := s.db.QueryRow(q, args...)
	var d Defaults
	var repo sql.NullString
	err := row.Scan(&d.Scope, &d.Depth, &d.TestStrategy, &d.ExecutionMode, &repo, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return &Defaults{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying feature_defaults: %w", err)
	}
	d.Repo = repo.String
	return &d, nil
}

func (s *Store) upsert(repo *string, d Defaults, actor string) (*Defaults, error) {
	scope := d.Scope
	depth := d.Depth
	testStrategy := d.TestStrategy
	execMode := d.ExecutionMode

	if repo == nil {
		// Global row: UPDATE WHERE repo IS NULL; if 0 rows affected, INSERT.
		// Postgres treats multiple NULLs as distinct under UNIQUE(repo), so
		// we can't use ON CONFLICT for the global row — the two-step avoids
		// inserting a second NULL row.
		res, err := s.db.Exec(
			`UPDATE feature_defaults SET scope = ?, depth = ?, test_strategy = ?, execution_mode = ?, updated_at = now() WHERE repo IS NULL`,
			scope, depth, testStrategy, execMode)
		if err != nil {
			return nil, fmt.Errorf("updating global defaults: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			_, err = s.db.Exec(
				`INSERT INTO feature_defaults (scope, depth, test_strategy, execution_mode, repo) VALUES (?, ?, ?, ?, NULL)`,
				scope, depth, testStrategy, execMode)
			if err != nil {
				return nil, fmt.Errorf("inserting global defaults: %w", err)
			}
		}
		got, err := s.GetGlobal(context.Background())
		if err != nil {
			return nil, err
		}
		s.emitAudit("update global defaults", actor)
		return got, nil
	}

	// Per-repo: INSERT ON CONFLICT (repo) DO UPDATE.
	_, err := s.db.Exec(
		`INSERT INTO feature_defaults (scope, depth, test_strategy, execution_mode, repo) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (repo) DO UPDATE SET scope = EXCLUDED.scope, depth = EXCLUDED.depth, test_strategy = EXCLUDED.test_strategy, execution_mode = EXCLUDED.execution_mode, updated_at = now()`,
		scope, depth, testStrategy, execMode, *repo)
	if err != nil {
		return nil, fmt.Errorf("upserting per-repo defaults for %q: %w", *repo, err)
	}
	got, err := s.get("repo = ?", *repo)
	if err != nil {
		return nil, err
	}
	s.emitAudit("update per-repo defaults for " + *repo, actor)
	return got, nil
}

func (s *Store) emitAudit(details, actor string) {
	if s.db == nil {
		return
	}
	_ = s.db.RecordAuditEventWithActor("platform", db.AuditFeatureDefaultsMutated, "", "construction", details, actor)
}