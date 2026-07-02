package gate

import (
	"fmt"
	"time"
)

// State values for an approval gate.
const (
	StateOpen        = "open"         // awaiting user decision
	StateApproved    = "approved"     // user approved, advance
	StateRejected    = "rejected"     // user requested changes, revision cycle
	StateAcceptAsIs  = "accept_as_is" // 3-strike escape hatch
)

// MaxRevisions is the number of rejections before accept-as-is becomes available.
const MaxRevisions = 3

// Gate represents an approval gate on a stage.
type Gate struct {
	FeatureID     string
	StageID       string
	State         string
	RevisionCount int
	RevisionNotes string    // notes from last rejection
	DecidedAt     *time.Time // when user approved/rejected/accepted
}

// CanAcceptAsIs returns true if the 3-strike escape hatch is available.
func (g *Gate) CanAcceptAsIs() bool {
	return g.RevisionCount >= MaxRevisions
}

// IsOpen returns true if the gate is awaiting a user decision.
func (g *Gate) IsOpen() bool {
	return g.State == StateOpen
}

// Approve marks the gate as approved.
func (g *Gate) Approve() error {
	if !g.IsOpen() {
		return fmt.Errorf("gate %s/%s is not open (state=%s)", g.FeatureID, g.StageID, g.State)
	}
	now := time.Now().UTC()
	g.State = StateApproved
	g.DecidedAt = &now
	return nil
}

// Reject marks the gate as rejected, increments revision count, stores notes.
func (g *Gate) Reject(notes string) error {
	if !g.IsOpen() {
		return fmt.Errorf("gate %s/%s is not open (state=%s)", g.FeatureID, g.StageID, g.State)
	}
	g.State = StateRejected
	g.RevisionCount++
	g.RevisionNotes = notes
	return nil
}

// AcceptAsIs marks the gate as accepted despite issues (escape hatch).
func (g *Gate) AcceptAsIs() error {
	if !g.CanAcceptAsIs() {
		return fmt.Errorf("accept-as-is not available until %d revisions (current: %d)", MaxRevisions, g.RevisionCount)
	}
	if !g.IsOpen() {
		return fmt.Errorf("gate %s/%s is not open (state=%s)", g.FeatureID, g.StageID, g.State)
	}
	now := time.Now().UTC()
	g.State = StateAcceptAsIs
	g.DecidedAt = &now
	return nil
}

// Reset reopens the gate for a new revision cycle.
func (g *Gate) Reset() {
	g.State = StateOpen
	g.RevisionNotes = ""
	g.DecidedAt = nil
}

// New creates a new open gate.
func New(featureID, stageID string) *Gate {
	return &Gate{
		FeatureID: featureID,
		StageID:   stageID,
		State:     StateOpen,
	}
}