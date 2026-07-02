package pipeline

import (
	"testing"

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