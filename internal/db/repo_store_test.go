package db

import (
	"errors"
	"testing"
)

// seedFeatureRepo inserts a feature_repos row so the delete-guard primitives
// (CountRepoReferences / ListReferencingFeatures) have data to count.
func seedFeatureRepo(t *testing.T, d *DB, featureID, repoName string) {
	t.Helper()
	seedFeature(t, d, featureID)
	_, err := d.Exec(
		`INSERT INTO feature_repos (feature_id, name, url, dir, branch) VALUES (?, ?, ?, ?, ?)`,
		featureID, repoName, "git@host:org/repo.git", "/tmp/"+featureID, "main",
	)
	if err != nil {
		t.Fatalf("seedFeatureRepo %s/%s: %v", featureID, repoName, err)
	}
}

func TestRepoCreateAndGet(t *testing.T) {
	d := setupTestDB(t)

	created, err := d.CreateRepo("devteam", "git@github.com:MichielDean/devteam.git", "main", "platform", true)
	if err != nil {
		t.Fatalf("CreateRepo: %v", err)
	}
	if created.Name != "devteam" {
		t.Errorf("name = %q, want devteam", created.Name)
	}
	if created.URL != "git@github.com:MichielDean/devteam.git" {
		t.Errorf("url = %q", created.URL)
	}
	if created.Branch != "main" {
		t.Errorf("branch = %q, want main", created.Branch)
	}
	if created.Description != "platform" {
		t.Errorf("description = %q", created.Description)
	}
	if !created.Primary {
		t.Error("primary = false, want true")
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Error("timestamps not set by DB")
	}

	fetched, err := d.GetRepo("devteam")
	if err != nil {
		t.Fatalf("GetRepo: %v", err)
	}
	if fetched.Name != "devteam" {
		t.Errorf("fetched name = %q", fetched.Name)
	}
}

func TestRepoCreateBranchDefault(t *testing.T) {
	d := setupTestDB(t)

	// CreateRepo with empty branch should fall back to 'main'.
	created, err := d.CreateRepo("cistern", "git@github.com:org/cistern.git", "", "", false)
	if err != nil {
		t.Fatalf("CreateRepo: %v", err)
	}
	if created.Branch != "main" {
		t.Errorf("branch = %q, want main (default)", created.Branch)
	}
	if created.Description != "" {
		t.Errorf("description = %q, want empty", created.Description)
	}
}

func TestRepoCreateDuplicateReturnsErrRepoExists(t *testing.T) {
	d := setupTestDB(t)

	if _, err := d.CreateRepo("LLMem", "git@github.com:org/LLMem.git", "main", "memory", false); err != nil {
		t.Fatalf("first CreateRepo: %v", err)
	}
	_, err := d.CreateRepo("LLMem", "git@github.com:org/LLMem.git", "main", "memory v2", false)
	if !errors.Is(err, ErrRepoExists) {
		t.Fatalf("second CreateRepo err = %v, want ErrRepoExists", err)
	}
}

func TestRepoGetNotFound(t *testing.T) {
	d := setupTestDB(t)

	_, err := d.GetRepo("nonexistent")
	if !errors.Is(err, ErrRepoNotFound) {
		t.Errorf("GetRepo err = %v, want ErrRepoNotFound", err)
	}
}

func TestRepoUpdate(t *testing.T) {
	d := setupTestDB(t)

	if _, err := d.CreateRepo("devteam", "git@github.com:org/devteam.git", "main", "v1", false); err != nil {
		t.Fatalf("CreateRepo: %v", err)
	}
	updated, err := d.UpdateRepo("devteam", "git@github.com:org/devteam.git", "develop", "v2", true)
	if err != nil {
		t.Fatalf("UpdateRepo: %v", err)
	}
	if updated.Branch != "develop" {
		t.Errorf("branch = %q, want develop", updated.Branch)
	}
	if updated.Description != "v2" {
		t.Errorf("description = %q, want v2", updated.Description)
	}
	if !updated.Primary {
		t.Error("primary = false, want true")
	}
	// name is immutable — must not change
	if updated.Name != "devteam" {
		t.Errorf("name = %q, want devteam (immutable)", updated.Name)
	}
}

func TestRepoUpdatesUpdatedAt(t *testing.T) {
	d := setupTestDB(t)

	created, err := d.CreateRepo("devteam", "url", "main", "v1", false)
	if err != nil {
		t.Fatalf("CreateRepo: %v", err)
	}
	originalUpdatedAt := created.UpdatedAt

	updated, err := d.UpdateRepo("devteam", "url2", "main", "v1", false)
	if err != nil {
		t.Fatalf("UpdateRepo: %v", err)
	}
	if !updated.UpdatedAt.After(originalUpdatedAt) && !updated.UpdatedAt.Equal(originalUpdatedAt) {
		// Allow equal in case of fast DB clock resolution, but it should never go backwards.
		t.Errorf("updated_at went backwards: %s → %s", originalUpdatedAt, updated.UpdatedAt)
	}
}

func TestRepoUpdateNotFound(t *testing.T) {
	d := setupTestDB(t)

	_, err := d.UpdateRepo("ghost", "url", "main", "desc", false)
	if !errors.Is(err, ErrRepoNotFound) {
		t.Errorf("UpdateRepo err = %v, want ErrRepoNotFound", err)
	}
}

func TestRepoDelete(t *testing.T) {
	d := setupTestDB(t)

	if _, err := d.CreateRepo("devteam", "url", "main", "desc", false); err != nil {
		t.Fatalf("CreateRepo: %v", err)
	}
	if err := d.DeleteRepo("devteam"); err != nil {
		t.Fatalf("DeleteRepo: %v", err)
	}
	if _, err := d.GetRepo("devteam"); !errors.Is(err, ErrRepoNotFound) {
		t.Errorf("GetRepo after delete err = %v, want ErrRepoNotFound", err)
	}
}

func TestRepoDeleteNotFound(t *testing.T) {
	d := setupTestDB(t)

	err := d.DeleteRepo("ghost")
	if !errors.Is(err, ErrRepoNotFound) {
		t.Errorf("DeleteRepo err = %v, want ErrRepoNotFound", err)
	}
}

func TestRepoListEmptyReturnsEmptySlice(t *testing.T) {
	d := setupTestDB(t)

	repos, err := d.ListRepos()
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if repos == nil {
		t.Fatal("ListRepos returned nil slice — JSON would serialize as null, not []")
	}
	if len(repos) != 0 {
		t.Errorf("len = %d, want 0", len(repos))
	}
}

func TestRepoListOrdersByNameAndComputesReferenceCount(t *testing.T) {
	d := setupTestDB(t)

	// Create three repos out of order; ListRepos must return them sorted by name.
	if _, err := d.CreateRepo("ScaledTest", "url-s", "main", "", false); err != nil {
		t.Fatalf("CreateRepo ScaledTest: %v", err)
	}
	if _, err := d.CreateRepo("LLMem", "url-l", "main", "", false); err != nil {
		t.Fatalf("CreateRepo LLMem: %v", err)
	}
	if _, err := d.CreateRepo("devteam", "url-d", "main", "", true); err != nil {
		t.Fatalf("CreateRepo devteam: %v", err)
	}

	// Attach a feature to devteam so its reference_count is 1.
	seedFeatureRepo(t, d, "feat-1", "devteam")

	repos, err := d.ListRepos()
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(repos) != 3 {
		t.Fatalf("len = %d, want 3", len(repos))
	}
	// Sorted by name (Postgres default collation — case-insensitive-ish
	// alphabetical: 'devteam' < 'LLMem' < 'ScaledTest').
	if repos[0].Name != "devteam" {
		t.Errorf("repos[0].Name = %q, want devteam", repos[0].Name)
	}
	if repos[1].Name != "LLMem" {
		t.Errorf("repos[1].Name = %q, want LLMem", repos[1].Name)
	}
	if repos[2].Name != "ScaledTest" {
		t.Errorf("repos[2].Name = %q, want ScaledTest", repos[2].Name)
	}

	// reference_count: devteam=1, LLMem=0, ScaledTest=0
	if repos[0].ReferenceCount != 1 {
		t.Errorf("devteam reference_count = %d, want 1", repos[0].ReferenceCount)
	}
	if repos[1].ReferenceCount != 0 {
		t.Errorf("LLMem reference_count = %d, want 0", repos[1].ReferenceCount)
	}
	if repos[2].ReferenceCount != 0 {
		t.Errorf("ScaledTest reference_count = %d, want 0", repos[2].ReferenceCount)
	}
}

func TestRepoCountReferencesZero(t *testing.T) {
	d := setupTestDB(t)

	if _, err := d.CreateRepo("devteam", "url", "main", "", false); err != nil {
		t.Fatalf("CreateRepo: %v", err)
	}
	count, err := d.CountRepoReferences("devteam")
	if err != nil {
		t.Fatalf("CountRepoReferences: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestRepoCountReferencesNonZero(t *testing.T) {
	d := setupTestDB(t)

	if _, err := d.CreateRepo("devteam", "url", "main", "", false); err != nil {
		t.Fatalf("CreateRepo: %v", err)
	}
	// Two features reference the same repo → count = 2.
	seedFeatureRepo(t, d, "feat-1", "devteam")
	seedFeatureRepo(t, d, "feat-2", "devteam")

	count, err := d.CountRepoReferences("devteam")
	if err != nil {
		t.Fatalf("CountRepoReferences: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestRepoListReferencingFeaturesEmpty(t *testing.T) {
	d := setupTestDB(t)

	features, err := d.ListReferencingFeatures("unreferenced")
	if err != nil {
		t.Fatalf("ListReferencingFeatures: %v", err)
	}
	if features == nil {
		t.Fatal("returned nil slice — JSON would serialize as null, not []")
	}
	if len(features) != 0 {
		t.Errorf("len = %d, want 0", len(features))
	}
}

func TestRepoListReferencingFeaturesDistinct(t *testing.T) {
	d := setupTestDB(t)

	// feat-1 and feat-2 both attach devteam → ListReferencingFeatures returns
	// both feature_ids (DISTINCT collapses any within-feature dupes, but the
	// UNIQUE(feature_id, name) constraint already prevents those).
	seedFeatureRepo(t, d, "feat-1", "devteam")
	seedFeatureRepo(t, d, "feat-2", "devteam")

	features, err := d.ListReferencingFeatures("devteam")
	if err != nil {
		t.Fatalf("ListReferencingFeatures: %v", err)
	}
	if len(features) != 2 {
		t.Errorf("len = %d, want 2 (distinct feature_ids)", len(features))
	}
}

func TestRepoCountRepos(t *testing.T) {
	d := setupTestDB(t)

	n, err := d.CountRepos()
	if err != nil {
		t.Fatalf("CountRepos: %v", err)
	}
	if n != 0 {
		t.Errorf("empty count = %d, want 0", n)
	}

	if _, err := d.CreateRepo("a", "url", "main", "", false); err != nil {
		t.Fatalf("CreateRepo a: %v", err)
	}
	if _, err := d.CreateRepo("b", "url", "main", "", false); err != nil {
		t.Fatalf("CreateRepo b: %v", err)
	}
	n, err = d.CountRepos()
	if err != nil {
		t.Fatalf("CountRepos: %v", err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
}