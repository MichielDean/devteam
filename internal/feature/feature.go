package feature

import (
	"fmt"
	"strings"
	"time"
)

type Feature struct {
	ID           string                `yaml:"id" json:"id"`
	Title        string                `yaml:"title" json:"title"`
	Status       Status                `yaml:"status" json:"status"`
	Priority     int                   `yaml:"priority" json:"priority"`
	IntakePath   IntakePath            `yaml:"intake_path" json:"intake_path"`
	SpecDir      string                `yaml:"spec_dir" json:"spec_dir"`
	CreatedAt    time.Time             `yaml:"created_at" json:"created_at"`
	UpdatedAt    time.Time             `yaml:"updated_at" json:"updated_at"`
	Dependencies []string              `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Repos        []RepoRef             `yaml:"repos,omitempty" json:"repos,omitempty"`
	PhaseStates  map[Phase]*PhaseState `yaml:"phase_states" json:"phase_states"`
}

type RepoRef struct {
	Name   string `yaml:"name" json:"name"`
	URL    string `yaml:"url" json:"url"`
	Branch string `yaml:"branch" json:"branch"`
}

type PhaseState struct {
	Phase       Phase       `yaml:"phase" json:"phase"`
	Status      Status      `yaml:"status" json:"status"`
	Artifacts   []Artifact  `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`
	GateResult  *GateResult `yaml:"gate_result,omitempty" json:"gate_result,omitempty"`
	StartedAt   *time.Time  `yaml:"started_at,omitempty" json:"started_at,omitempty"`
	CompletedAt *time.Time  `yaml:"completed_at,omitempty" json:"completed_at,omitempty"`
}

type Artifact struct {
	Type        ArtifactType `yaml:"type" json:"type"`
	Path        string       `yaml:"path" json:"path"`
	GeneratedBy RoleName     `yaml:"generated_by" json:"generated_by"`
	GeneratedAt time.Time    `yaml:"generated_at" json:"generated_at"`
}

type GateResult struct {
	Phase       Phase         `yaml:"phase" json:"phase"`
	Passed      bool          `yaml:"passed" json:"passed"`
	MissingArts []string      `yaml:"missing_arts,omitempty" json:"missing_arts,omitempty"`
	Checks      []CheckResult `yaml:"checks,omitempty" json:"checks,omitempty"`
	EvaluatedAt time.Time     `yaml:"evaluated_at" json:"evaluated_at"`
}

type CheckResult struct {
	Name    string `yaml:"name" json:"name"`
	Passed  bool   `yaml:"passed" json:"passed"`
	Message string `yaml:"message,omitempty" json:"message,omitempty"`
}

func NewFeature(id, title string, priority int, intakePath IntakePath) *Feature {
	now := time.Now()
	f := &Feature{
		ID:          id,
		Title:       title,
		Status:      StatusDraft,
		Priority:    priority,
		IntakePath:  intakePath,
		SpecDir:     fmt.Sprintf("specs/%s/", id),
		CreatedAt:   now,
		UpdatedAt:   now,
		PhaseStates: make(map[Phase]*PhaseState),
	}
	for _, phase := range AllPhases() {
		f.PhaseStates[phase] = &PhaseState{
			Phase:  phase,
			Status: StatusDraft,
		}
	}
	return f
}

func (f *Feature) CurrentPhase() Phase {
	phases := AllPhases()
	// Find the first phase that is in_progress or gate_blocked
	for _, phase := range phases {
		ps, ok := f.PhaseStates[phase]
		if !ok {
			continue
		}
		if ps.Status == StatusInProgress || ps.Status == StatusGateBlocked {
			return phase
		}
	}
	// Find the last passed phase and return the next one
	lastPassedIdx := -1
	for i, phase := range phases {
		ps, ok := f.PhaseStates[phase]
		if !ok {
			continue
		}
		if ps.Status == StatusPassed {
			lastPassedIdx = i
		}
	}
	// If phases have passed, the current phase is the one after the last passed
	if lastPassedIdx >= 0 && lastPassedIdx < len(phases)-1 {
		return phases[lastPassedIdx+1]
	}
	// If all phases passed, feature is done
	if lastPassedIdx == len(phases)-1 {
		return phases[lastPassedIdx]
	}
	// Nothing has started yet — feature is in draft
	return PhaseInception
}

func (f *Feature) AdvanceTo(phase Phase) error {
	current := f.CurrentPhase()
	// If inception phase has passed and we're advancing to planning, allow it
	// regardless of top-level status
	if f.Status == StatusDraft {
		if phase == PhaseInception {
			now := time.Now()
			if ps, ok := f.PhaseStates[PhaseInception]; ok && ps.Status == StatusDraft {
				ps.Status = StatusInProgress
				ps.StartedAt = &now
			}
			f.Status = StatusInProgress
			f.UpdatedAt = now
			return nil
		}
		// If inception already passed (gate evaluated) but top-level status wasn't updated
		if ps, ok := f.PhaseStates[PhaseInception]; ok && ps.Status == StatusPassed {
			f.Status = StatusInProgress
			if !ValidateTransition(PhaseInception, phase) {
				return fmt.Errorf("cannot advance from inception to %s: invalid transition", phase)
			}
			now := time.Now()
			ps.Status = StatusPassed
			ps.CompletedAt = &now
			f.PhaseStates[phase].Status = StatusInProgress
			f.PhaseStates[phase].StartedAt = &now
			f.UpdatedAt = now
			return nil
		}
		return fmt.Errorf("first advance must be to inception, not %s", phase)
	}
	// For subsequent advances, must go to the next phase
	if !ValidateTransition(current, phase) {
		return fmt.Errorf("cannot advance from %s to %s: invalid transition", current, phase)
	}
	now := time.Now()
	if ps, ok := f.PhaseStates[current]; ok {
		ps.Status = StatusPassed
		ps.CompletedAt = &now
	}
	f.PhaseStates[phase].Status = StatusInProgress
	f.PhaseStates[phase].StartedAt = &now
	f.Status = StatusInProgress
	f.UpdatedAt = now
	return nil
}

func (f *Feature) RecirculateTo(phase Phase) error {
	current := f.CurrentPhase()
	phases := AllPhases()
	currentIdx := -1
	targetIdx := -1
	for i, p := range phases {
		if p == current {
			currentIdx = i
		}
		if p == phase {
			targetIdx = i
		}
	}
	if currentIdx < 0 || targetIdx < 0 {
		return fmt.Errorf("invalid phase in recirculation: current=%s target=%s", current, phase)
	}
	if targetIdx >= currentIdx {
		return fmt.Errorf("cannot recirculate forward: current=%s target=%s", current, phase)
	}
	now := time.Now()
	// Mark the current phase as recirculated
	if ps, ok := f.PhaseStates[current]; ok {
		ps.Status = StatusRecirculated
		ps.CompletedAt = &now
	}
	// Reset all phases between target+1 and current (inclusive of current) to draft
	// The current phase itself was already marked as Recirculated above,
	// so we only reset the intermediate phases
	for i := targetIdx + 1; i < currentIdx; i++ {
		p := phases[i]
		if ps, ok := f.PhaseStates[p]; ok {
			ps.Status = StatusDraft
			ps.GateResult = nil
			ps.Artifacts = nil
			ps.StartedAt = nil
			ps.CompletedAt = nil
		}
	}
	f.PhaseStates[phase].Status = StatusInProgress
	f.PhaseStates[phase].StartedAt = &now
	f.Status = StatusRecirculated
	f.UpdatedAt = now
	return nil
}

func (f *Feature) Cancel() {
	now := time.Now()
	f.Status = StatusCancelled
	f.UpdatedAt = now
}

func (f *Feature) MarkDone() {
	now := time.Now()
	lastPhase := PhaseDelivery
	if ps, ok := f.PhaseStates[lastPhase]; ok {
		ps.Status = StatusPassed
		ps.CompletedAt = &now
	}
	f.Status = StatusDone
	f.UpdatedAt = now
}

func (f *Feature) IsTerminal() bool {
	return f.Status == StatusDone || f.Status == StatusCancelled
}

func (f *Feature) Slug() string {
	s := strings.ToLower(f.Title)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}
