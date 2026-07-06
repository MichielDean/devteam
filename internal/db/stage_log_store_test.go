package db

import (
	"database/sql"
	"strings"
	"testing"
)

// TestAppendStageLogForBoltConcatenates verifies that repeated appends produce
// concatenated content — the primitive the batcher depends on (U-BK-02 / FR-3).
func TestAppendStageLogForBoltConcatenates(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-append")
	featureID := "feat-append"
	stageID := "3.5"
	bolt := 1

	chunks := []string{"first chunk\n", "second chunk\n", "third chunk\n"}
	for _, c := range chunks {
		if err := d.AppendStageLogForBolt(featureID, stageID, bolt, "developer", c); err != nil {
			t.Fatalf("AppendStageLogForBolt: %v", err)
		}
	}

	got, err := d.GetStageLogForBolt(featureID, stageID, bolt)
	if err != nil {
		t.Fatalf("GetStageLogForBolt: %v", err)
	}
	want := strings.Join(chunks, "")
	if got != want {
		t.Errorf("concatenated content = %q, want %q", got, want)
	}
}

// TestAppendStageLogForBoltIdempotentIndex verifies the unique-index upsert
// behavior: appending to an existing row updates it, doesn't duplicate (U-BK-02).
func TestAppendStageLogForBoltIdempotentIndex(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-idem")
	featureID := "feat-idem"
	stageID := "1.1"
	bolt := 0

	if err := d.AppendStageLogForBolt(featureID, stageID, bolt, "product", "a"); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := d.AppendStageLogForBolt(featureID, stageID, bolt, "product", "b"); err != nil {
		t.Fatalf("second append: %v", err)
	}

	// Should be one row, content = "ab"
	got, err := d.GetStageLogForBolt(featureID, stageID, bolt)
	if err != nil {
		t.Fatalf("GetStageLogForBolt: %v", err)
	}
	if got != "ab" {
		t.Errorf("content = %q, want %q", got, "ab")
	}

	// Verify only one row exists
	var n int
	err = d.QueryRow(
		`SELECT COUNT(*) FROM stage_logs WHERE feature_id = ? AND stage_id = ? AND bolt_number = ?`,
		featureID, stageID, bolt,
	).Scan(&n)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("row count = %d, want 1 (unique index should prevent duplicates)", n)
	}
}

// TestAppendStageLogForBoltEmptyInput verifies empty-input appends are tolerated
// (the batcher may flush a zero-length tail on cancellation; it must not error) (U-BK-02).
func TestAppendStageLogForBoltEmptyInput(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-empty")
	featureID := "feat-empty"
	stageID := "2.1"
	bolt := 0

	if err := d.AppendStageLogForBolt(featureID, stageID, bolt, "developer", ""); err != nil {
		t.Fatalf("empty append: %v", err)
	}
	if err := d.AppendStageLogForBolt(featureID, stageID, bolt, "developer", "real content"); err != nil {
		t.Fatalf("non-empty append: %v", err)
	}

	got, err := d.GetStageLogForBolt(featureID, stageID, bolt)
	if err != nil {
		t.Fatalf("GetStageLogForBolt: %v", err)
	}
	if got != "real content" {
		t.Errorf("content after empty+non-empty = %q, want %q", got, "real content")
	}
}

// TestAppendStageLogForBoltLargeInput verifies a single large append (≥8KB, one
// flush) is stored correctly — the batcher's flush_bytes threshold boundary (U-BK-02).
func TestAppendStageLogForBoltLargeInput(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-large")
	featureID := "feat-large"
	stageID := "3.5"
	bolt := 1

	// 16KB — twice the default flush_bytes threshold
	large := strings.Repeat("x", 16*1024)
	if err := d.AppendStageLogForBolt(featureID, stageID, bolt, "developer", large); err != nil {
		t.Fatalf("large append: %v", err)
	}

	got, err := d.GetStageLogForBolt(featureID, stageID, bolt)
	if err != nil {
		t.Fatalf("GetStageLogForBolt: %v", err)
	}
	if len(got) != len(large) {
		t.Errorf("content length = %d, want %d", len(got), len(large))
	}
	if got != large {
		t.Errorf("content mismatch (first/last 32 chars): got %q...%q", got[:32], got[len(got)-32:])
	}
}

// TestAppendStageLogAfterSave verifies the R-1 hazard scenario at the store level:
// a full-replace (SaveStageLogForBolt) followed by an append. ADR-2 removes this
// sequence at the orchestration level, but the store-level behavior is tested here
// as defense-in-depth (U-BK-02).
func TestAppendStageLogAfterSave(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-hazard")
	featureID := "feat-hazard"
	stageID := "3.5"
	bolt := 1

	// Save (full-replace) — simulates a "completion" save
	if err := d.SaveStageLogForBolt(featureID, stageID, bolt, "developer", "canonical final content"); err != nil {
		t.Fatalf("SaveStageLogForBolt: %v", err)
	}
	got, err := d.GetStageLogForBolt(featureID, stageID, bolt)
	if err != nil {
		t.Fatalf("Get after save: %v", err)
	}
	if got != "canonical final content" {
		t.Errorf("after save = %q, want %q", got, "canonical final content")
	}

	// Append (concatenates) — simulates a late batcher flush landing after the save
	if err := d.AppendStageLogForBolt(featureID, stageID, bolt, "developer", " LATE FLUSH"); err != nil {
		t.Fatalf("late append: %v", err)
	}
	got, err = d.GetStageLogForBolt(featureID, stageID, bolt)
	if err != nil {
		t.Fatalf("Get after late append: %v", err)
	}
	// The append concatenates onto the saved content — this is the R-1 hazard at
	// the store level. ADR-2 removes the dual-write at the orchestration level
	// (no SaveStageLogForBolt on the completion path), so this scenario does not
	// occur in production. This test documents the store-level semantics.
	if got != "canonical final content LATE FLUSH" {
		t.Errorf("after late append = %q, want %q (concatenation is the documented store behavior)", got, "canonical final content LATE FLUSH")
	}
}

// TestSaveStageLogForBoltResetsRow verifies the re-dispatch reset (ADR-2 / O-11):
// a SaveStageLogForBolt with empty content resets the row so a re-dispatch does
// not concatenate stale + new output (U-BK-02 / R-6).
func TestSaveStageLogForBoltResetsRow(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-redispatch")
	featureID := "feat-redispatch"
	stageID := "3.5"
	bolt := 1

	// First dispatch: append some content
	if err := d.AppendStageLogForBolt(featureID, stageID, bolt, "developer", "first run output\n"); err != nil {
		t.Fatalf("first run append: %v", err)
	}
	got, _ := d.GetStageLogForBolt(featureID, stageID, bolt)
	if got != "first run output\n" {
		t.Fatalf("first run = %q, want %q", got, "first run output\n")
	}

	// Re-dispatch reset: SaveStageLogForBolt with empty content full-replaces
	if err := d.SaveStageLogForBolt(featureID, stageID, bolt, "developer", ""); err != nil {
		t.Fatalf("reset save: %v", err)
	}
	got, _ = d.GetStageLogForBolt(featureID, stageID, bolt)
	if got != "" {
		t.Errorf("after reset = %q, want empty (row reset for re-dispatch)", got)
	}

	// Second dispatch: append fresh content — no stale concatenation
	if err := d.AppendStageLogForBolt(featureID, stageID, bolt, "developer", "second run output\n"); err != nil {
		t.Fatalf("second run append: %v", err)
	}
	got, _ = d.GetStageLogForBolt(featureID, stageID, bolt)
	if got != "second run output\n" {
		t.Errorf("second run = %q, want %q (no stale concatenation)", got, "second run output\n")
	}
}

// TestGetStageLogForBoltMissingRow verifies a missing row returns empty with no
// error (the read path treats empty as "no content yet") (U-BK-02).
func TestGetStageLogForBoltMissingRow(t *testing.T) {
	d := setupTestDB(t)
	seedFeature(t, d, "feat-missing")
	featureID := "feat-missing"
	stageID := "9.9"
	bolt := 0

	got, err := d.GetStageLogForBolt(featureID, stageID, bolt)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("GetStageLogForBolt on missing row: %v", err)
	}
	if got != "" {
		t.Errorf("missing row = %q, want empty", got)
	}
}