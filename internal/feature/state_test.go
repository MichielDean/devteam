package feature

import (
	"testing"
)

func TestAllPhases(t *testing.T) {
	phases := AllPhases()
	if len(phases) != 6 {
		t.Fatalf("expected 6 phases, got %d", len(phases))
	}
	if phases[0] != PhaseInception {
		t.Errorf("expected first phase to be inception, got %s", phases[0])
	}
	if phases[5] != PhaseDelivery {
		t.Errorf("expected last phase to be delivery, got %s", phases[5])
	}
}

func TestParsePhase(t *testing.T) {
	tests := []struct {
		input    string
		expected Phase
	}{
		{"inception", PhaseInception},
		{"planning", PhasePlanning},
		{"construction", PhaseConstruction},
		{"review", PhaseReview},
		{"testing", PhaseTesting},
		{"delivery", PhaseDelivery},
		{"unknown", PhaseInception},
	}
	for _, tt := range tests {
		got := ParsePhase(tt.input)
		if got != tt.expected {
			t.Errorf("ParsePhase(%q) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

func TestNewFeature(t *testing.T) {
	f := NewFeature("001-test", "Test Feature", 2, IntakeLooseIdea)
	if f.ID != "001-test" {
		t.Errorf("expected ID '001-test', got %s", f.ID)
	}
	if f.Status != StatusDraft {
		t.Errorf("expected status draft, got %s", f.Status)
	}
	if f.Priority != 2 {
		t.Errorf("expected priority 2, got %d", f.Priority)
	}
	if f.IntakePath != IntakeLooseIdea {
		t.Errorf("expected intake path loose_idea, got %s", f.IntakePath)
	}
	if len(f.PhaseStates) != 6 {
		t.Errorf("expected 6 phase states, got %d", len(f.PhaseStates))
	}
}

func TestAdvanceTo(t *testing.T) {
	f := NewFeature("001-test", "Test Feature", 2, IntakeLooseIdea)
	err := f.AdvanceTo(PhaseInception) // draft -> inception
	if err != nil {
		t.Fatalf("unexpected error advancing to inception: %v", err)
	}
	err = f.AdvanceTo(PhasePlanning) // inception -> planning
	if err != nil {
		t.Fatalf("unexpected error advancing to planning: %v", err)
	}
	if f.PhaseStates[PhaseInception].Status != StatusPassed {
		t.Errorf("expected inception to be passed, got %s", f.PhaseStates[PhaseInception].Status)
	}
	if f.PhaseStates[PhasePlanning].Status != StatusInProgress {
		t.Errorf("expected planning to be in_progress, got %s", f.PhaseStates[PhasePlanning].Status)
	}
}

func TestAdvanceToInvalid(t *testing.T) {
	f := NewFeature("001-test", "Test Feature", 2, IntakeLooseIdea)
	// Can't skip phases: advancing from draft to review should fail
	err := f.AdvanceTo(PhaseReview)
	if err == nil {
		t.Fatal("expected error when skipping phases, got nil")
	}
}

func TestRecirculateTo(t *testing.T) {
	f := NewFeature("001-test", "Test Feature", 2, IntakeLooseIdea)
	f.AdvanceTo(PhaseInception)
	f.AdvanceTo(PhasePlanning)
	f.AdvanceTo(PhaseConstruction)
	f.AdvanceTo(PhaseReview)

	err := f.RecirculateTo(PhaseConstruction)
	if err != nil {
		t.Fatalf("unexpected error recirculating: %v", err)
	}
	if f.PhaseStates[PhaseReview].Status != StatusRecirculated {
		t.Errorf("expected review to be recirculated, got %s", f.PhaseStates[PhaseReview].Status)
	}
	if f.PhaseStates[PhaseConstruction].Status != StatusInProgress {
		t.Errorf("expected construction to be in_progress after recirculation, got %s", f.PhaseStates[PhaseConstruction].Status)
	}
}

func TestRecirculateForwardFails(t *testing.T) {
	f := NewFeature("001-test", "Test Feature", 2, IntakeLooseIdea)
	err := f.RecirculateTo(PhasePlanning)
	if err == nil {
		t.Fatal("expected error when recirculating forward, got nil")
	}
}

func TestCancel(t *testing.T) {
	f := NewFeature("001-test", "Test Feature", 2, IntakeLooseIdea)
	f.Cancel()
	if f.Status != StatusCancelled {
		t.Errorf("expected status cancelled, got %s", f.Status)
	}
}

func TestMarkDone(t *testing.T) {
	f := NewFeature("001-test", "Test Feature", 2, IntakeLooseIdea)
	f.AdvanceTo(PhaseInception)
	f.AdvanceTo(PhasePlanning)
	f.AdvanceTo(PhaseConstruction)
	f.AdvanceTo(PhaseReview)
	f.AdvanceTo(PhaseTesting)
	f.AdvanceTo(PhaseDelivery)
	f.MarkDone()
	if f.Status != StatusDone {
		t.Errorf("expected status done, got %s", f.Status)
	}
}

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		from     Phase
		to       Phase
		expected bool
	}{
		{PhaseInception, PhasePlanning, true},
		{PhasePlanning, PhaseConstruction, true},
		{PhaseConstruction, PhaseReview, true},
		{PhaseInception, PhaseConstruction, false},
		{PhaseReview, PhaseInception, false},
	}
	for _, tt := range tests {
		got := ValidateTransition(tt.from, tt.to)
		if got != tt.expected {
			t.Errorf("ValidateTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.expected)
		}
	}
}

func TestRecirculationTarget(t *testing.T) {
	tests := []struct {
		from     Phase
		reason   string
		expected Phase
	}{
		{PhaseReview, "code has bugs", PhaseConstruction},
		{PhaseReview, "architectural issue with plan", PhasePlanning},
		{PhaseTesting, "tests fail", PhaseConstruction},
		{PhaseDelivery, "docs don't match", PhaseTesting},
	}
	for _, tt := range tests {
		got := RecirculationTarget(tt.from, tt.reason)
		if got != tt.expected {
			t.Errorf("RecirculationTarget(%s, %q) = %s, want %s", tt.from, tt.reason, got, tt.expected)
		}
	}
}

func TestGateDefinitions(t *testing.T) {
	if len(GateDefinitions) != 6 {
		t.Fatalf("expected 6 gate definitions, got %d", len(GateDefinitions))
	}
	gd := GetGateDefinition(PhaseInception)
	if gd == nil {
		t.Fatal("expected gate definition for inception, got nil")
	}
	if gd.GateName != GateSpecApproved {
		t.Errorf("expected gate spec_approved, got %s", gd.GateName)
	}
	if len(gd.RequiredArts) != 3 {
		t.Errorf("expected 3 required arts for inception gate, got %d", len(gd.RequiredArts))
	}
}