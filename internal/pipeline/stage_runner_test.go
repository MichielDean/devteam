package pipeline

import (
	"testing"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/gate"
	"github.com/MichielDean/devteam/internal/stage"
)

func TestShouldSkipStageAlways(t *testing.T) {
	p := &Pipeline{}
	f := &feature.Feature{Scope: stage.ScopeFeature}
	s := db.StageDefinition{Condition: stage.CondAlways, Scopes: []string{stage.ScopeFeature, stage.ScopeEnterprise, stage.ScopeMVP, stage.ScopePOC, stage.ScopeBugfix, stage.ScopeRefactor, stage.ScopeInfra, stage.ScopeSecurityPatch, stage.ScopeWorkshop}}
	if p.ShouldSkipStage(f, s) {
		t.Error("ALWAYS stage should not be skipped")
	}
}

func TestShouldSkipStageConditional(t *testing.T) {
	p := &Pipeline{}
	f := &feature.Feature{Scope: stage.ScopeFeature}
	s := db.StageDefinition{Condition: stage.CondConditional, Scopes: []string{stage.ScopeFeature}}
	if p.ShouldSkipStage(f, s) {
		t.Error("CONDITIONAL stage in scope set should not be skipped")
	}
}

func TestShouldSkipStageNotInScope(t *testing.T) {
	p := &Pipeline{}
	f := &feature.Feature{Scope: stage.ScopeBugfix}
	s := db.StageDefinition{Condition: stage.CondConditional, Scopes: []string{stage.ScopeFeature, stage.ScopeEnterprise}}
	if !p.ShouldSkipStage(f, s) {
		t.Error("Stage not in bugfix scope should be skipped")
	}
}

func TestGateIntegrationWithReject(t *testing.T) {
	g := gate.New("feat-1", "1.1")
	if err := g.Reject("needs work"); err != nil {
		t.Fatal(err)
	}
	if g.RevisionCount != 1 {
		t.Errorf("revision count = %d, want 1", g.RevisionCount)
	}
	g.Reset()
	if !g.IsOpen() {
		t.Error("gate should be open after reset")
	}
}

func TestStageOutcomeInstructions(t *testing.T) {
	stageDef := &db.StageDefinition{ID: "2.3", Name: "Requirements Analysis"}
	result := stageOutcomeInstructions(stageDef)
	if result == "" {
		t.Error("stageOutcomeInstructions returned empty string")
	}
}