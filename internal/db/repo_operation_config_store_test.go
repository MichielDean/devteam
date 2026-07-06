package db

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// sampleRow builds a RepoOperationConfigRow with representative JSONB payloads
// for round-trip and upsert tests.
func sampleRow(name string) RepoOperationConfigRow {
	return RepoOperationConfigRow{
		RepoName:         name,
		CiPlatform:       "github-actions",
		CdPlatform:       "argocd",
		Environments:     json.RawMessage(`{"staging":{"cd_platform":"argocd"},"prod":{"cd_platform":"argocd","observability":{"prometheus":true}}}`),
		Observability:    json.RawMessage(`{"prometheus":true,"grafana":true,"loki":false}`),
		IncidentResponse: json.RawMessage(`{"oncall_rotation":"primary","pagerduty":true}`),
	}
}

func TestSaveAndGetOperationConfig(t *testing.T) {
	d := setupTestDB(t)
	row := sampleRow("repo-a")
	if err := d.SaveOperationConfig(row); err != nil {
		t.Fatalf("SaveOperationConfig: %v", err)
	}
	got, err := d.GetOperationConfig("repo-a")
	if err != nil {
		t.Fatalf("GetOperationConfig: %v", err)
	}
	if got.RepoName != "repo-a" {
		t.Errorf("RepoName = %q, want repo-a", got.RepoName)
	}
	if got.CiPlatform != "github-actions" {
		t.Errorf("CiPlatform = %q, want github-actions", got.CiPlatform)
	}
	if got.CdPlatform != "argocd" {
		t.Errorf("CdPlatform = %q, want argocd", got.CdPlatform)
	}
	// JSONB semantic round-trip (C-A5 opacity): Postgres JSONB normalizes
	// whitespace and sorts keys, so we compare semantic content, not bytes.
	assertJSONBSemanticEqual(t, "Environments", got.Environments, row.Environments)
	assertJSONBSemanticEqual(t, "Observability", got.Observability, row.Observability)
	assertJSONBSemanticEqual(t, "IncidentResponse", got.IncidentResponse, row.IncidentResponse)
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set by the store on insert")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set by the store on insert")
	}
}

// assertJSONBSemanticEqual compares two json.RawMessage by semantic content
// (unmarshal to map[string]any and compare). Postgres JSONB normalizes
// whitespace and key order on storage, so byte-exact comparison would fail
// even when the content is identical. The opacity contract (C-A5) requires
// that every key and value the operator wrote is preserved — semantic
// equality is the correct assertion.
func assertJSONBSemanticEqual(t *testing.T, field string, got, want json.RawMessage) {
	t.Helper()
	var gotMap, wantMap map[string]any
	if err := json.Unmarshal(got, &gotMap); err != nil {
		t.Fatalf("%s: unmarshal got: %v (raw=%s)", field, err, got)
	}
	if err := json.Unmarshal(want, &wantMap); err != nil {
		t.Fatalf("%s: unmarshal want: %v (raw=%s)", field, err, want)
	}
	if len(gotMap) != len(wantMap) {
		t.Errorf("%s: key count changed (opacity — got %d, want %d)", field, len(gotMap), len(wantMap))
		return
	}
	for k, wantVal := range wantMap {
		gotVal, ok := gotMap[k]
		if !ok {
			t.Errorf("%s: key %q lost in round-trip (opacity violated — C-A5)", field, k)
			continue
		}
		if !reflect.DeepEqual(gotVal, wantVal) {
			t.Errorf("%s: key %q value changed: got %v, want %v", field, k, gotVal, wantVal)
		}
	}
}

func TestGetOperationConfig_Missing(t *testing.T) {
	d := setupTestDB(t)
	got, err := d.GetOperationConfig("nonexistent-repo")
	if err != nil {
		t.Fatalf("GetOperationConfig missing row should not error, got: %v", err)
	}
	if got.RepoName != "nonexistent-repo" {
		t.Errorf("RepoName = %q, want nonexistent-repo (caller's name echoed)", got.RepoName)
	}
	if got.CiPlatform != "" {
		t.Errorf("CiPlatform = %q, want empty (zero-value)", got.CiPlatform)
	}
	if got.CdPlatform != "" {
		t.Errorf("CdPlatform = %q, want empty (zero-value)", got.CdPlatform)
	}
	// Empty-not-null (DR-R6): JSONB fields must be {} not null.
	if string(got.Environments) != "{}" {
		t.Errorf("Environments = %s, want {} (DR-R6 empty-not-null)", got.Environments)
	}
	if string(got.Observability) != "{}" {
		t.Errorf("Observability = %s, want {} (DR-R6 empty-not-null)", got.Observability)
	}
	if string(got.IncidentResponse) != "{}" {
		t.Errorf("IncidentResponse = %s, want {} (DR-R6 empty-not-null)", got.IncidentResponse)
	}
	if !got.CreatedAt.IsZero() {
		t.Errorf("CreatedAt = %v, want zero for a missing row", got.CreatedAt)
	}
}

func TestGetAllOperationConfigs_Empty(t *testing.T) {
	d := setupTestDB(t)
	got, err := d.GetAllOperationConfigs()
	if err != nil {
		t.Fatalf("GetAllOperationConfigs: %v", err)
	}
	if got == nil {
		t.Fatal("got nil slice, want non-nil empty slice (DR-R6 / FR-STORE-03)")
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0 on empty table", len(got))
	}
	// JSON marshal must yield [], not null (FR-TEST-06).
	b, _ := json.Marshal(got)
	if string(b) != "[]" {
		t.Errorf("JSON marshal of empty slice = %s, want [] (FR-TEST-06)", b)
	}
}

func TestSaveOperationConfig_Idempotent(t *testing.T) {
	d := setupTestDB(t)
	row := sampleRow("repo-idem")
	if err := d.SaveOperationConfig(row); err != nil {
		t.Fatalf("first save: %v", err)
	}
	first, err := d.GetOperationConfig("repo-idem")
	if err != nil {
		t.Fatalf("first get: %v", err)
	}
	// Save again — must not duplicate, must not error.
	if err := d.SaveOperationConfig(row); err != nil {
		t.Fatalf("second save: %v", err)
	}
	second, err := d.GetOperationConfig("repo-idem")
	if err != nil {
		t.Fatalf("second get: %v", err)
	}
	// Still exactly one row (FR-STORE-02).
	all, _ := d.GetAllOperationConfigs()
	if len(all) != 1 {
		t.Errorf("after double-save, row count = %d, want 1 (FR-STORE-02 idempotent upsert)", len(all))
	}
	// updated_at should advance (or at least not regress).
	if second.UpdatedAt.Before(first.UpdatedAt) {
		t.Errorf("UpdatedAt went backwards: first=%v second=%v", first.UpdatedAt, second.UpdatedAt)
	}
	// JSONB bytes preserved across upsert.
	if string(second.Observability) != string(first.Observability) {
		t.Errorf("Observability bytes changed across idempotent upsert (ADR-03)")
	}
}

func TestSaveOperationConfig_JSONBRoundTrip(t *testing.T) {
	d := setupTestDB(t)
	// Operator's payload includes unknown keys, booleans, nested structure,
	// and a notes string. The platform must round-trip the *semantic*
	// content (all keys and values present) — that is the opacity contract
	// (C-A5): the platform does not interpret the shape, so every key the
	// operator wrote must come back.
	//
	// NOTE: Postgres JSONB normalizes whitespace and sorts keys on storage,
	// so byte-exact round-trip is NOT guaranteed by JSONB (the ADR-03
	// rationale point about "exact bytes for diffs" over-claimed — that
	// would require a TEXT column, not JSONB). The binding constraints
	// (C-D4 JSONB, C-A5 opacity) are satisfied by semantic round-trip:
	// every key and value the operator wrote is preserved.
	originalObs := json.RawMessage(`{"loki":true,"prometheus":false,"grafana":true,"notes":"order matters","datadog":false}`)
	row := RepoOperationConfigRow{
		RepoName:      "repo-jsonb",
		CiPlatform:    "gitlab-ci",
		Observability: originalObs,
	}
	if err := d.SaveOperationConfig(row); err != nil {
		t.Fatalf("SaveOperationConfig: %v", err)
	}
	got, err := d.GetOperationConfig("repo-jsonb")
	if err != nil {
		t.Fatalf("GetOperationConfig: %v", err)
	}
	// Semantic equality: unmarshal both to map[string]any and compare.
	var wantMap, gotMap map[string]any
	if err := json.Unmarshal(originalObs, &wantMap); err != nil {
		t.Fatalf("unmarshal original: %v", err)
	}
	if err := json.Unmarshal(got.Observability, &gotMap); err != nil {
		t.Fatalf("unmarshal got: %v", err)
	}
	if len(gotMap) != len(wantMap) {
		t.Fatalf("key count changed: got %d, want %d (opacity — unknown keys must be preserved)", len(gotMap), len(wantMap))
	}
	for k, wantVal := range wantMap {
		gotVal, ok := gotMap[k]
		if !ok {
			t.Errorf("key %q lost in round-trip (opacity violated — C-A5)", k)
			continue
		}
		if !reflect.DeepEqual(gotVal, wantVal) {
			t.Errorf("key %q value changed: got %v, want %v", k, gotVal, wantVal)
		}
	}
	// The result must be valid JSON (FR-API-07 relies on this).
	if !json.Valid(got.Observability) {
		t.Errorf("returned JSONB is not valid JSON: %s", got.Observability)
	}
}

func TestGetOperationConfigsByRepoNames(t *testing.T) {
	d := setupTestDB(t)
	// Seed 3 repos.
	for _, n := range []string{"repo-1", "repo-2", "repo-3"} {
		if err := d.SaveOperationConfig(sampleRow(n)); err != nil {
			t.Fatalf("SaveOperationConfig %s: %v", n, err)
		}
	}
	// Fetch 2 by name — repo-2 is omitted from the query to verify it's
	// not returned.
	got, err := d.GetOperationConfigsByRepoNames([]string{"repo-1", "repo-3"})
	if err != nil {
		t.Fatalf("GetOperationConfigsByRepoNames: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (missing repos omitted — FR-BRIDGE-04)", len(got))
	}
	names := map[string]bool{}
	for _, r := range got {
		names[r.RepoName] = true
	}
	if !names["repo-1"] || !names["repo-3"] {
		t.Errorf("expected repo-1 and repo-3, got names=%v", names)
	}
	if names["repo-2"] {
		t.Errorf("repo-2 was not requested but appeared in result")
	}
	// Ordered by repo_name ASC.
	if got[0].RepoName != "repo-1" || got[1].RepoName != "repo-3" {
		t.Errorf("result not ordered by repo_name ASC: %s, %s", got[0].RepoName, got[1].RepoName)
	}
}

func TestGetOperationConfigsByRepoNames_EmptyInput(t *testing.T) {
	d := setupTestDB(t)
	got, err := d.GetOperationConfigsByRepoNames([]string{})
	if err != nil {
		t.Fatalf("empty input: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("empty input should return non-nil empty slice, got len=%d nil=%v", len(got), got == nil)
	}
}

func TestDeleteOperationConfig(t *testing.T) {
	d := setupTestDB(t)
	if err := d.SaveOperationConfig(sampleRow("repo-del")); err != nil {
		t.Fatalf("SaveOperationConfig: %v", err)
	}
	if err := d.DeleteOperationConfig("repo-del"); err != nil {
		t.Fatalf("DeleteOperationConfig: %v", err)
	}
	got, err := d.GetOperationConfig("repo-del")
	if err != nil {
		t.Fatalf("GetOperationConfig after delete: %v", err)
	}
	// After delete, Get returns zero-value (no row) — not an error.
	if got.CiPlatform != "" {
		t.Errorf("after delete, CiPlatform = %q, want empty (zero-value)", got.CiPlatform)
	}
	if string(got.Observability) != "{}" {
		t.Errorf("after delete, Observability = %s, want {} (zero-value DR-R6)", got.Observability)
	}
}

func TestDeleteOperationConfig_Missing(t *testing.T) {
	d := setupTestDB(t)
	// Deleting a non-existent row is a no-op, not an error.
	if err := d.DeleteOperationConfig("never-existed"); err != nil {
		t.Errorf("deleting non-existent row should be a no-op, got error: %v", err)
	}
}

func TestErrorWrapping(t *testing.T) {
	d := setupTestDB(t)
	// Trigger a DB error by closing the connection then querying.
	d.Close()
	_, err := d.GetOperationConfig("repo-wrap")
	if err == nil {
		t.Fatal("expected error after DB close, got nil")
	}
	// FR-STORE-04: the error message must contain the repo name so the
	// operator can identify which repo's config failed.
	if !strings.Contains(err.Error(), "repo-wrap") {
		t.Errorf("error message %q does not contain the repo name (FR-STORE-04)", err.Error())
	}
}
