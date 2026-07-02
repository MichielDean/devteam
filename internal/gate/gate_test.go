package gate

import (
	"testing"
	"time"
)

func TestGateApprove(t *testing.T) {
	g := New("feat-1", "1.1")
	if !g.IsOpen() {
		t.Error("new gate should be open")
	}
	if err := g.Approve(); err != nil {
		t.Fatalf("Approve() error: %v", err)
	}
	if g.State != StateApproved {
		t.Errorf("State = %s, want approved", g.State)
	}
	if g.DecidedAt == nil {
		t.Error("DecidedAt should be set after approve")
	}
}

func TestGateReject(t *testing.T) {
	g := New("feat-1", "1.1")
	if err := g.Reject("needs more detail"); err != nil {
		t.Fatalf("Reject() error: %v", err)
	}
	if g.State != StateRejected {
		t.Errorf("State = %s, want rejected", g.State)
	}
	if g.RevisionCount != 1 {
		t.Errorf("RevisionCount = %d, want 1", g.RevisionCount)
	}
	if g.RevisionNotes != "needs more detail" {
		t.Errorf("RevisionNotes = %s, want 'needs more detail'", g.RevisionNotes)
	}
}

func TestGateRejectNotOpen(t *testing.T) {
	g := New("feat-1", "1.1")
	g.Approve()
	if err := g.Reject("test"); err == nil {
		t.Error("expected error rejecting non-open gate")
	}
}

func TestGateAcceptAsIs(t *testing.T) {
	g := New("feat-1", "1.1")
	if g.CanAcceptAsIs() {
		t.Error("should not be able to accept-as-is with 0 revisions")
	}
	for i := 0; i < MaxRevisions; i++ {
		g.Reject("rev")
		g.Reset()
	}
	if !g.CanAcceptAsIs() {
		t.Error("should be able to accept-as-is after 3 revisions")
	}
	if err := g.AcceptAsIs(); err != nil {
		t.Fatalf("AcceptAsIs() error: %v", err)
	}
	if g.State != StateAcceptAsIs {
		t.Errorf("State = %s, want accept_as_is", g.State)
	}
}

func TestGateAcceptAsIsNotEnoughRevisions(t *testing.T) {
	g := New("feat-1", "1.1")
	g.Reject("rev 1")
	g.Reset()
	g.Reject("rev 2")
	g.Reset()
	if err := g.AcceptAsIs(); err == nil {
		t.Error("expected error accepting with only 2 revisions")
	}
}

func TestGateReset(t *testing.T) {
	g := New("feat-1", "1.1")
	g.Reject("test")
	g.Reset()
	if !g.IsOpen() {
		t.Error("gate should be open after reset")
	}
	if g.RevisionNotes != "" {
		t.Error("RevisionNotes should be cleared after reset")
	}
	if g.DecidedAt != nil {
		t.Error("DecidedAt should be nil after reset")
	}
}

func TestGateDecidedAtUTC(t *testing.T) {
	g := New("feat-1", "1.1")
	g.Approve()
	if g.DecidedAt != nil {
		_ = g.DecidedAt.UTC()
		_ = time.Now()
	}
}