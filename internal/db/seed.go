package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SeedReposFromYAML loads the implementation-repo registry from repos.yaml
// into the `repos` table on first boot, then never writes back to the file
// (ADR-002: the YAML is the seed source, not a live config the app mutates).
//
// Idempotency: seeds ONLY when the `repos` table is empty (COUNT = 0). On
// every subsequent boot the table is non-empty, so the file is not re-read
// and no rows are inserted. This makes the hook safe to call on every boot
// without risk of clobbering operator edits made via the CRUD API.
//
// Missing file: if repos.yaml does not exist AND the table is empty, the
// hook is a no-op — the registry starts empty and the operator populates it
// via the UI. No error is returned (the file is optional; the empty registry
// is a valid state).
//
// The YAML is parsed as a map keyed by repo name (matching the actual
// repos.yaml format). Each entry's `branch` defaults to 'main' because the
// seed file does not carry a branch field (app-design §2.1, §3.5). `primary`
// and `description` are read from the file; `url` is required.
//
// This function never writes back to repos.yaml. The file on disk is a
// read-only seed source preserved for reference and disaster recovery
// (rollback-runbook §4: DROP TABLE + restart re-seeds from this file).
func (db *DB) SeedReposFromYAML(yamlPath string) error {
	count, err := db.CountRepos()
	if err != nil {
		return fmt.Errorf("seed: counting repos: %w", err)
	}
	if count > 0 {
		log.Printf("seed: repos table already has %d rows — skipping seed", count)
		return nil
	}

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("seed: %s not found and repos table empty — starting with empty registry", filepath.Base(yamlPath))
			return nil
		}
		return fmt.Errorf("seed: reading %s: %w", yamlPath, err)
	}

	var parsed struct {
		Repos map[string]struct {
			URL         string `yaml:"url"`
			Description string `yaml:"description"`
			Primary     bool   `yaml:"primary"`
		} `yaml:"repos"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("seed: parsing %s: %w", yamlPath, err)
	}

	if len(parsed.Repos) == 0 {
		log.Printf("seed: %s contained no repos — starting with empty registry", filepath.Base(yamlPath))
		return nil
	}

	seeded := 0
	for name, r := range parsed.Repos {
		// branch defaults to 'main' — repos.yaml has no branch field.
		if _, err := db.CreateRepo(name, r.URL, "main", r.Description, r.Primary); err != nil {
			// A duplicate-name conflict during seed is unexpected (we just
			// confirmed COUNT=0) but is logged rather than fatal so a
			// partially-broken yaml doesn't wedge the boot. The operator can
			// fix the file and restart.
			log.Printf("seed: skipping repo %q: %v", name, err)
			continue
		}
		log.Printf("seed: seeded repo %q (%s)", name, r.URL)
		seeded++
	}

	log.Printf("seed: seeded %d repo(s) from %s", seeded, filepath.Base(yamlPath))
	return nil
}