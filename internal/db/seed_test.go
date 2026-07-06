package db

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleReposYAML = `# Repository Registry
repos:
  devteam:
    url: git@github.com:MichielDean/devteam.git
    description: Dev Team platform — orchestrator, roles, specs, and configuration
    primary: true

  cistern:
    url: git@github.com:MichielDean/cistern.git
    description: Agentic workflow orchestrator

  LLMem:
    url: git@github.com:MichielDean/LLMem.git
    description: Structured agent memory system
`

func writeReposYAML(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "repos.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing repos.yaml: %v", err)
	}
	return path
}

func TestSeedReposFromYAMLFreshDB(t *testing.T) {
	d := setupTestDB(t)
	path := writeReposYAML(t, t.TempDir(), sampleReposYAML)

	if err := d.SeedReposFromYAML(path); err != nil {
		t.Fatalf("SeedReposFromYAML: %v", err)
	}

	repos, err := d.ListRepos()
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(repos) != 3 {
		t.Fatalf("len = %d, want 3", len(repos))
	}

	// Verify devteam seeded with primary=true and branch defaulted to 'main'.
	devteam, err := d.GetRepo("devteam")
	if err != nil {
		t.Fatalf("GetRepo devteam: %v", err)
	}
	if !devteam.Primary {
		t.Error("devteam.primary = false, want true")
	}
	if devteam.Branch != "main" {
		t.Errorf("devteam.branch = %q, want main (default)", devteam.Branch)
	}
	if devteam.URL != "git@github.com:MichielDean/devteam.git" {
		t.Errorf("devteam.url = %q", devteam.URL)
	}

	// LLMem should be seeded with primary=false (not set in yaml).
	llmem, err := d.GetRepo("LLMem")
	if err != nil {
		t.Fatalf("GetRepo LLMem: %v", err)
	}
	if llmem.Primary {
		t.Error("LLMem.primary = true, want false (not set in yaml)")
	}
}

func TestSeedReposFromYAMLIdempotentSecondBootNoReseed(t *testing.T) {
	d := setupTestDB(t)
	path := writeReposYAML(t, t.TempDir(), sampleReposYAML)

	// First boot: seeds 3 repos.
	if err := d.SeedReposFromYAML(path); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	before, err := d.ListRepos()
	if err != nil {
		t.Fatalf("ListRepos after first seed: %v", err)
	}
	if len(before) != 3 {
		t.Fatalf("after first seed: len = %d, want 3", len(before))
	}

	// Second boot: must NOT re-seed (count > 0 → skip). Row count and
	// timestamps should be unchanged — a re-seed would either error on
	// duplicate names or, if we'd used ON CONFLICT DO UPDATE, bump
	// updated_at. Neither should happen.
	if err := d.SeedReposFromYAML(path); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	after, err := d.ListRepos()
	if err != nil {
		t.Fatalf("ListRepos after second seed: %v", err)
	}
	if len(after) != 3 {
		t.Fatalf("after second seed: len = %d, want 3 (no re-seed)", len(after))
	}

	// Verify updated_at did not change for devteam between boots.
	devteamBefore, _ := d.GetRepo("devteam")
	_ = devteamBefore // already fetched via before list; re-fetch is identical
	devteamAfter, err := d.GetRepo("devteam")
	if err != nil {
		t.Fatalf("GetRepo after second seed: %v", err)
	}
	// Find devteam in the `before` slice to compare timestamps.
	var devteamBeforeRow *RepoRow
	for i := range before {
		if before[i].Name == "devteam" {
			devteamBeforeRow = &before[i]
			break
		}
	}
	if devteamBeforeRow == nil {
		t.Fatal("devteam not found in before-seed list")
	}
	if !devteamAfter.UpdatedAt.Equal(devteamBeforeRow.UpdatedAt) {
		t.Errorf("updated_at changed after second boot: %s → %s (re-seed should not bump it)",
			devteamBeforeRow.UpdatedAt, devteamAfter.UpdatedAt)
	}
}

func TestSeedReposFromYAMLMissingFileNoError(t *testing.T) {
	d := setupTestDB(t)

	// Point at a path that does not exist. Table is empty. Hook must not
	// error — the empty registry is a valid starting state.
	missingPath := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	if err := d.SeedReposFromYAML(missingPath); err != nil {
		t.Fatalf("SeedReposFromYAML on missing file: %v (expected nil)", err)
	}

	count, err := d.CountRepos()
	if err != nil {
		t.Fatalf("CountRepos: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (missing file → empty registry)", count)
	}
}

func TestSeedReposFromYAMLEmptyMapNoError(t *testing.T) {
	d := setupTestDB(t)
	path := writeReposYAML(t, t.TempDir(), `repos:
`)

	if err := d.SeedReposFromYAML(path); err != nil {
		t.Fatalf("SeedReposFromYAML on empty map: %v", err)
	}

	count, err := d.CountRepos()
	if err != nil {
		t.Fatalf("CountRepos: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (empty yaml map)", count)
	}
}

func TestSeedReposFromYAMLDoesNotOverwriteOperatorEdits(t *testing.T) {
	d := setupTestDB(t)
	path := writeReposYAML(t, t.TempDir(), sampleReposYAML)

	// First boot seeds the registry.
	if err := d.SeedReposFromYAML(path); err != nil {
		t.Fatalf("first seed: %v", err)
	}

	// Operator edits devteam via the CRUD API — changes branch to 'develop'.
	if _, err := d.UpdateRepo("devteam", "git@github.com:MichielDean/devteam.git", "develop", "edited", true); err != nil {
		t.Fatalf("UpdateRepo: %v", err)
	}

	// Second boot: must NOT re-seed and must NOT overwrite the operator's edit.
	if err := d.SeedReposFromYAML(path); err != nil {
		t.Fatalf("second seed: %v", err)
	}

	devteam, err := d.GetRepo("devteam")
	if err != nil {
		t.Fatalf("GetRepo: %v", err)
	}
	if devteam.Branch != "develop" {
		t.Errorf("branch = %q, want develop (operator edit must survive re-boot)", devteam.Branch)
	}
	if devteam.Description != "edited" {
		t.Errorf("description = %q, want edited", devteam.Description)
	}
}