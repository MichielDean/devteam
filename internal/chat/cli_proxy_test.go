package chat

import (
	"testing"
)

func TestFindVerb_ReadOnly(t *testing.T) {
	v := FindVerb("feature status my-feature")
	if v == nil {
		t.Fatal("expected match for feature status")
	}
	if v.class != ClassReadOnly {
		t.Errorf("class = %v, want read-only", v.class)
	}
}

func TestFindVerb_Mutating(t *testing.T) {
	v := FindVerb("feature create --title X")
	if v == nil {
		t.Fatal("expected match for feature create")
	}
	if v.class != ClassMutating {
		t.Errorf("class = %v, want mutating", v.class)
	}
}

func TestFindVerb_Destructive(t *testing.T) {
	v := FindVerb("feature cancel my-feature")
	if v == nil {
		t.Fatal("expected match for feature cancel")
	}
	if v.class != ClassDestructive {
		t.Errorf("class = %v, want destructive", v.class)
	}
	if v.consequence == "" {
		t.Error("destructive verb should have a consequence string")
	}
}

func TestFindVerb_NotAllowed(t *testing.T) {
	if v := FindVerb("rm -rf /"); v != nil {
		t.Error("rm should not be on the allowlist")
	}
	if v := FindVerb("feature destroy"); v != nil {
		t.Error("feature destroy should not be on the allowlist")
	}
}

func TestFindVerb_LongestPrefixWins(t *testing.T) {
	// "feature status" should match before "feature" would (if it existed alone)
	v := FindVerb("feature status abc")
	if v == nil || v.verb != "feature status" {
		t.Errorf("expected longest-prefix match on 'feature status', got %+v", v)
	}
}

func TestParseArgs_SimpleSplit(t *testing.T) {
	args := parseArgs(`--title "My Feature" --priority 2`)
	want := []string{"--title", "My Feature", "--priority", "2"}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i := range args {
		if args[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestParseArgs_Empty(t *testing.T) {
	if args := parseArgs(""); args != nil {
		t.Errorf("expected nil for empty, got %v", args)
	}
}

func TestProposalStore_CreateReadOnlyNoConfirm(t *testing.T) {
	s := NewProposalStore(0)
	p, err := s.Create("sess-1", "feature status abc", false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.NeedsConfirm() {
		t.Error("read-only should not need confirm")
	}
}

func TestProposalStore_CreateMutatingNeedsConfirm(t *testing.T) {
	s := NewProposalStore(0)
	p, err := s.Create("sess-1", "signal abc pass", false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !p.NeedsConfirm() {
		t.Error("mutating (no trust) should need confirm")
	}
}

func TestProposalStore_CreateMutatingTrustModeSkipsConfirm(t *testing.T) {
	s := NewProposalStore(0)
	p, err := s.Create("sess-1", "signal abc pass", true)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.NeedsConfirm() {
		t.Error("mutating + trust_mode should NOT need confirm")
	}
}

func TestProposalStore_CreateDestructiveAlwaysNeedsConfirm(t *testing.T) {
	s := NewProposalStore(0)
	// Even with trust_mode on, destructive requires confirm.
	p, err := s.Create("sess-1", "feature cancel abc", true)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !p.NeedsConfirm() {
		t.Error("destructive should ALWAYS need confirm (even with trust_mode)")
	}
	if p.Consequence == "" {
		t.Error("destructive proposal should carry the consequence string")
	}
}

func TestProposalStore_CreateRejectsNonAllowlist(t *testing.T) {
	s := NewProposalStore(0)
	if _, err := s.Create("sess-1", "rm -rf /", false); err == nil {
		t.Error("expected verb_not_allowed error for rm")
	}
	if _, err := s.Create("sess-1", "feature destroy", false); err == nil {
		t.Error("expected verb_not_allowed error for feature destroy")
	}
}

func TestProposalStore_GetAndResolve(t *testing.T) {
	s := NewProposalStore(0)
	p, _ := s.Create("sess-1", "signal abc pass", false)
	got := s.Get(p.ID)
	if got == nil || got.ID != p.ID {
		t.Error("Get should return the pending proposal")
	}
	resolved := s.Resolve(p.ID)
	if resolved == nil || resolved.ID != p.ID {
		t.Error("Resolve should return the proposal")
	}
	if s.Get(p.ID) != nil {
		t.Error("Get after Resolve should return nil")
	}
}