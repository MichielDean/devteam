package pipeline

import (
	"testing"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/stage"
)

func TestGroupUnitsIntoBolts(t *testing.T) {
	p := &Pipeline{}
	units := []string{"unit-1", "unit-2", "unit-3"}
	bolts := p.groupUnitsIntoBolts(units)
	if len(bolts) != 3 {
		t.Fatalf("expected 3 bolts, got %d", len(bolts))
	}
	if len(bolts[0]) != 1 || bolts[0][0] != "unit-1" {
		t.Errorf("bolt 1 = %v, want [unit-1]", bolts[0])
	}
}

func TestAutonomyModeConstants(t *testing.T) {
	if AutonomyGated != "gated" {
		t.Errorf("AutonomyGated = %s, want 'gated'", AutonomyGated)
	}
	if AutonomyAutonomous != "autonomous" {
		t.Errorf("AutonomyAutonomous = %s, want 'autonomous'", AutonomyAutonomous)
	}
}

func TestLadderPromptInvalidMode(t *testing.T) {
	p := &Pipeline{}
	f := &feature.Feature{ID: "feat-1", Scope: stage.ScopeFeature}
	// Invalid mode should error before touching specProvider
	if err := p.LadderPrompt(f, "invalid"); err == nil {
		t.Error("expected error for invalid autonomy mode")
	}
}

func TestFilterBolts(t *testing.T) {
	in := []db.BoltRow{
		{BoltNumber: 1},
		{BoltNumber: 2},
		{BoltNumber: 3},
		{BoltNumber: 4},
	}
	remove := []db.BoltRow{
		{BoltNumber: 2},
		{BoltNumber: 4},
	}
	out := filterBolts(in, remove)
	if len(out) != 2 {
		t.Fatalf("expected 2 remaining, got %d", len(out))
	}
	if out[0].BoltNumber != 1 || out[1].BoltNumber != 3 {
		t.Errorf("remaining = %v, want [1 3]", boltNumbers(out))
	}
}

func TestBoltNumbers(t *testing.T) {
	bolts := []db.BoltRow{
		{BoltNumber: 5},
		{BoltNumber: 7},
		{BoltNumber: 9},
	}
	nums := boltNumbers(bolts)
	if len(nums) != 3 || nums[0] != 5 || nums[1] != 7 || nums[2] != 9 {
		t.Errorf("boltNumbers = %v, want [5 7 9]", nums)
	}
}