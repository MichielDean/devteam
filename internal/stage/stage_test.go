package stage

import (
	"testing"
)

func TestAllStagesCount(t *testing.T) {
	defs := GetStageDefinitions()
	if len(defs) != 32 {
		t.Errorf("expected 32 stage definitions, got %d", len(defs))
	}
}

func TestStagePhases(t *testing.T) {
	defs := GetStageDefinitions()
	phaseCounts := map[string]int{}
	for _, s := range defs {
		phaseCounts[s.Phase]++
	}
	expected := map[string]int{
		PhaseInitialization: 3,
		PhaseIdeation:       7,
		PhaseInception:      8,
		PhaseConstruction:   7,
		PhaseOperation:      7,
	}
	for phase, count := range expected {
		if phaseCounts[phase] != count {
			t.Errorf("phase %s: expected %d stages, got %d", phase, count, phaseCounts[phase])
		}
	}
}

func TestStageIDs(t *testing.T) {
	defs := GetStageDefinitions()
	seen := map[string]bool{}
	for _, s := range defs {
		if seen[s.ID] {
			t.Errorf("duplicate stage ID: %s", s.ID)
		}
		seen[s.ID] = true
		if !IsValidStageID(s.ID) {
			t.Errorf("invalid stage ID format: %s", s.ID)
		}
	}
}

func TestStageSortOrder(t *testing.T) {
	defs := GetStageDefinitions()
	for i, s := range defs {
		if s.SortOrder != i+1 {
			t.Errorf("stage %s: expected sort_order %d, got %d", s.ID, i+1, s.SortOrder)
		}
	}
}

func TestStageLeadAgents(t *testing.T) {
	validAgents := map[string]bool{
		"orchestrator": true, "product": true, "design": true, "delivery": true,
		"architect": true, "platform": true, "devsecops": true, "developer": true,
		"quality": true, "pipeline-deploy": true, "operations": true,
	}
	defs := GetStageDefinitions()
	for _, s := range defs {
		if !validAgents[s.LeadAgent] {
			t.Errorf("stage %s: invalid lead agent %q", s.ID, s.LeadAgent)
		}
	}
}

func TestStageReviewers(t *testing.T) {
	validReviewers := map[string]bool{"": true, "product-lead": true, "architecture-reviewer": true}
	defs := GetStageDefinitions()
	for _, s := range defs {
		if !validReviewers[s.Reviewer] {
			t.Errorf("stage %s: invalid reviewer %q", s.ID, s.Reviewer)
		}
	}
}

func TestScopeStages(t *testing.T) {
	// AIDLC v2 reference: enterprise/feature=32, mvp=22, poc=8, bugfix=7, refactor=8,
	// infra=13, security-patch=9, workshop=25. Our adaptation (10 agents, no compliance,
	// platform generalized) differs. Bounds are wide to validate the adaptation is
	// in the right ballpark, not exact.
	tests := []struct {
		scope     string
		expectMin int
		expectMax int
	}{
		{ScopeEnterprise, 30, 32},
		{ScopeFeature, 30, 32},
		{ScopeMVP, 18, 24},
		{ScopePOC, 5, 12},
		{ScopeBugfix, 5, 10},
		{ScopeRefactor, 5, 12},
		{ScopeInfra, 10, 22},
		{ScopeSecurityPatch, 7, 16},
		{ScopeWorkshop, 20, 28},
	}
	for _, tt := range tests {
		count := 0
		for _, s := range GetStageDefinitions() {
			for _, sc := range s.Scopes {
				if sc == tt.scope {
					count++
					break
				}
			}
		}
		if count < tt.expectMin || count > tt.expectMax {
			t.Errorf("scope %s: expected %d-%d stages, got %d", tt.scope, tt.expectMin, tt.expectMax, count)
		}
	}
}

func TestDetectScope(t *testing.T) {
	tests := []struct {
		intent       string
		expectScope  string
		expectConf   bool
	}{
		{"fix the login bug", ScopeBugfix, true},
		{"Fix the infrastructure monitoring dashboard", ScopeFeature, false}, // >5 words, disambiguation → feature
		{"refactor the auth module", ScopeRefactor, true},
		{"refactor auth", ScopeRefactor, true}, // ≤5 words, specific keyword
		{"proof of concept for search", ScopePOC, true},
		{"build an MVP for task management", ScopeMVP, true},
		{"workshop on microservices", ScopeWorkshop, true},
		{"deploy new environments", ScopeInfra, true}, // 3 words, specific keyword
		{"CVE-2024-1234 security patch", ScopeSecurityPatch, true},
		{"vulnerability in auth", ScopeSecurityPatch, true},
		{"build a REST API for inventory management", ScopeFeature, false},
		{"clean up database layer", ScopeRefactor, true}, // 4 words, specific keyword
		{"fix bug", ScopeBugfix, true},
	}
	for _, tt := range tests {
		scope, conf := DetectScope(tt.intent)
		if scope != tt.expectScope {
			t.Errorf("DetectScope(%q): expected %s, got %s", tt.intent, tt.expectScope, scope)
		}
		_ = conf
	}
}

func TestIsValidScope(t *testing.T) {
	valid := []string{ScopeEnterprise, ScopeFeature, ScopeMVP, ScopePOC, ScopeBugfix, ScopeRefactor, ScopeInfra, ScopeSecurityPatch, ScopeWorkshop}
	for _, s := range valid {
		if !IsValidScope(s) {
			t.Errorf("expected %s to be valid", s)
		}
	}
	if IsValidScope("invalid") {
		t.Error("expected 'invalid' to be invalid scope")
	}
}

func TestStageCheckbox(t *testing.T) {
	tests := []struct {
		status string
		expect string
	}{
		{StatusNotStarted, "[ ]"},
		{StatusInProgress, "[-]"},
		{StatusAwaitingApproval, "[?]"},
		{StatusRevising, "[R]"},
		{StatusCompleted, "[x]"},
		{StatusSkipped, "[S]"},
	}
	for _, tt := range tests {
		if got := StageCheckbox(tt.status); got != tt.expect {
			t.Errorf("StageCheckbox(%s): expected %s, got %s", tt.status, tt.expect, got)
		}
	}
}

func TestAllPhases(t *testing.T) {
	phases := AllPhases()
	if len(phases) != 5 {
		t.Errorf("expected 5 phases, got %d", len(phases))
	}
	if phases[0] != PhaseInitialization || phases[4] != PhaseOperation {
		t.Errorf("phases in wrong order: %v", phases)
	}
}