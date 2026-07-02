package stage

import (
	"strings"
)

// Phase names (AIDLC v2 5 phases).
const (
	PhaseInitialization = "initialization"
	PhaseIdeation       = "ideation"
	PhaseInception      = "inception"
	PhaseConstruction   = "construction"
	PhaseOperation      = "operation"
)

// StageStatus values (6-state checkbox notation from AIDLC v2).
const (
	StatusNotStarted        = "not_started"         // [ ]
	StatusInProgress        = "in_progress"         // [-]
	StatusAwaitingApproval  = "awaiting_approval"   // [?]
	StatusRevising          = "revising"             // [R]
	StatusCompleted         = "completed"           // [x]
	StatusSkipped           = "skipped"             // [S]
)

// Depth levels.
const (
	DepthMinimal       = "minimal"
	DepthStandard      = "standard"
	DepthComprehensive = "comprehensive"
)

// Test strategy levels.
const (
	TestStrategyMinimal       = "minimal"
	TestStrategyStandard      = "standard"
	TestStrategyComprehensive = "comprehensive"
)

// Condition values.
const (
	CondAlways       = "ALWAYS"
	CondConditional  = "CONDITIONAL"
	CondBrownfield   = "BROWNFIELD"
	CondUserFacing   = "USER_FACING"
	CondUIProject    = "UI_PROJECT"
	CondPerBolt      = "PER_BOLT"
	CondOnceAtEnd    = "ONCE_AT_END"
)

// AllPhases returns the 5 phases in order.
func AllPhases() []string {
	return []string{PhaseInitialization, PhaseIdeation, PhaseInception, PhaseConstruction, PhaseOperation}
}

// PhaseDisplay returns a human-friendly phase name.
func PhaseDisplay(phase string) string {
	switch phase {
	case PhaseInitialization:
		return "Initialization"
	case PhaseIdeation:
		return "Ideation"
	case PhaseInception:
		return "Inception"
	case PhaseConstruction:
		return "Construction"
	case PhaseOperation:
		return "Operation"
	}
	return phase
}

// StageCheckbox returns the 6-state checkbox symbol for a status.
func StageCheckbox(status string) string {
	switch status {
	case StatusNotStarted:
		return "[ ]"
	case StatusInProgress:
		return "[-]"
	case StatusAwaitingApproval:
		return "[?]"
	case StatusRevising:
		return "[R]"
	case StatusCompleted:
		return "[x]"
	case StatusSkipped:
		return "[S]"
	}
	return "[ ]"
}

// IsValidStageID checks if a string looks like a valid stage ID (e.g. "1.1", "3.5").
func IsValidStageID(id string) bool {
	if len(id) < 3 {
		return false
	}
	parts := strings.Split(id, ".")
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if len(p) == 0 {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}