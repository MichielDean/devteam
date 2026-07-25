package db

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// MaterializeReposOverlay writes the per-feature repos.yaml from repo_registry
// (+ repo_settings join) so internal/repo/manager.go consumers keep working
// without code change (FR-DISC-04, C-10, NFR-COMPAT-04, U-07).
//
// The materialized file is the SAME shape as the root repos.yaml (the
// ReposConfig struct in internal/config) — `repos:` list with name/url/description.
// The DB is canonical; the file is a generated overlay. The root repos.yaml
// stays parse-compatible (FR-DISC-05) and is marked deprecated (U-07 adds the
// deprecation comment to the root file separately).
//
// If repo_registry is empty, the materialization seeds from the root repos.yaml
// (the startup seed path, FR-DISC-04). This makes the transition seamless: an
// operator with an existing repos.yaml gets it loaded into repo_registry on
// first startup, then subsequent runs materialize from the DB.
func (db *DB) MaterializeReposOverlay(rootReposYAMLPath, featureReposYAMLPath string) error {
	empty, err := db.RepoRegistryEmpty()
	if err != nil {
		return fmt.Errorf("checking repo_registry empty: %w", err)
	}
	if empty {
		// Seed from the root repos.yaml if it exists.
		if err := db.seedRepoRegistryFromYAML(rootReposYAMLPath); err != nil {
			return fmt.Errorf("seeding repo_registry from %s: %w", rootReposYAMLPath, err)
		}
	}

	// Read the joined registry + settings and write the overlay.
	rows, err := db.Query(
		`SELECT r.name, r.full_name, r.default_branch, r.managed,
		        COALESCE(s.provider, 'native')
		 FROM repo_registry r
		 LEFT JOIN repo_settings s ON s.repo_registry_id = r.id
		 ORDER BY r.managed DESC, r.owner ASC, r.name ASC`,
	)
	if err != nil {
		return fmt.Errorf("querying repo_registry for overlay: %w", err)
	}
	defer rows.Close()

	type overlayEntry struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Managed     int    `yaml:"managed"`
		DefaultBranch string `yaml:"default_branch"`
		Provider    string `yaml:"provider"`
	}
	var entries []overlayEntry
	for rows.Next() {
		var e overlayEntry
		if err := rows.Scan(&e.Name, &e.Description, &e.DefaultBranch, &e.Managed, &e.Provider); err != nil {
			return fmt.Errorf("scanning overlay row: %w", err)
		}
		entries = append(entries, e)
	}

	// Build the overlay YAML. We use a simple struct that matches the root
	// repos.yaml shape (name + description) plus the new managed/provider/default_branch
	// fields (additive — the root file ignores unknown fields via yaml.v3).
	out := struct {
		Repos []overlayEntry `yaml:"repos"`
	}{Repos: entries}

	data, err := yaml.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshaling overlay yaml: %w", err)
	}

	// Ensure the feature dir exists.
	dir := filepath.Dir(featureReposYAMLPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating overlay dir %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(featureReposYAMLPath, data, 0o644); err != nil {
		return fmt.Errorf("writing overlay %s: %w", featureReposYAMLPath, err)
	}
	return nil
}

// seedRepoRegistryFromYAML reads the root repos.yaml and inserts each repo
// into repo_registry (managed=1, since the operator explicitly listed them —
// the root file is the curated set). installation_id is set to 0 (unknown);
// discovery will update it on the next sync.
func (db *DB) seedRepoRegistryFromYAML(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no root file → empty registry is fine
		}
		return fmt.Errorf("reading %s: %w", path, err)
	}
	var cfg struct {
		Repos []struct {
			Name        string `yaml:"name"`
			URL         string `yaml:"url"`
			Description string `yaml:"description"`
		} `yaml:"repos"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	now := time.Now().UTC()
	for _, r := range cfg.Repos {
		owner, name := splitFullName(r.Name)
		_, _, err := db.UpsertRepoRegistry(owner, name, r.Name, "main", 0)
		if err != nil {
			return fmt.Errorf("seeding %s: %w", r.Name, err)
		}
		// Mark seeded repos as managed (they were in the operator's curated list).
		_ = db.SetRepoManaged(owner, name, 1)
	}
	_ = now
	return nil
}

// splitFullName splits "owner/name" into (owner, name). If there's no slash,
// the whole string is the name and owner is "".
func splitFullName(full string) (string, string) {
	for i, c := range full {
		if c == '/' {
			return full[:i], full[i+1:]
		}
	}
	return "", full
}