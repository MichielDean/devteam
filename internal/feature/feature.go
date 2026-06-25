package feature

import (
	"fmt"
	"strings"
	"time"
)

type Feature struct {
	ID           string     `yaml:"id" json:"id"`
	Title        string     `yaml:"title" json:"title"`
	Current      Phase      `yaml:"current_phase" json:"current_phase"`
	Status       Status     `yaml:"status" json:"status"`
	Priority     int        `yaml:"priority" json:"priority"`
	IntakePath   IntakePath `yaml:"intake_path" json:"intake_path"`
	SpecDir      string     `yaml:"spec_dir" json:"spec_dir"`
	WorktreeDir  string     `yaml:"worktree_dir,omitempty" json:"worktree_dir,omitempty"`
	CreatedAt    time.Time  `yaml:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `yaml:"updated_at" json:"updated_at"`
	Dependencies []string   `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Repos        []RepoRef  `yaml:"repos,omitempty" json:"repos,omitempty"`
	// PreparedRepos records working dirs for impl repos prepared by
	// Pipeline.PrepareImplRepos. Persisted so later phases (review,
	// testing, delivery) can dispatch agents to the same worktrees without
	// re-cloning. Cleared by CleanupImplRepos on feature completion.
	PreparedRepos []PreparedRepo        `yaml:"prepared_repos,omitempty" json:"prepared_repos,omitempty"`
	PhaseStates   map[Phase]*PhaseState `yaml:"phase_states" json:"phase_states"`
}

// PreparedRepo is a persisted record of a prepared implementation repo
// worktree. It survives across pipeline phases so the same clone is reused
// for construction, review, testing, and delivery. The Branch is always
// feature/<featureID> (see repo.FeatureBranchName).
type PreparedRepo struct {
	Name   string `yaml:"name" json:"name"`
	URL    string `yaml:"url" json:"url"`
	Dir    string `yaml:"dir" json:"dir"`
	Branch string `yaml:"branch" json:"branch"`
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
		Current:     PhaseInception,
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
	return f.Current
}

func (f *Feature) Start() {
	now := time.Now()
	f.Current = PhaseInception
	f.Status = StatusInProgress
	f.PhaseStates[PhaseInception].Status = StatusInProgress
	f.PhaseStates[PhaseInception].StartedAt = &now
	f.UpdatedAt = now
}

func (f *Feature) AdvanceTo(phase Phase) error {
	if f.Status == StatusWaitingFeedback {
		return fmt.Errorf("cannot advance feature in waiting_for_human status")
	}
	phases := AllPhases()
	currentIdx := -1
	targetIdx := -1
	for i, p := range phases {
		if p == f.Current {
			currentIdx = i
		}
		if p == phase {
			targetIdx = i
		}
	}
	if currentIdx < 0 {
		return fmt.Errorf("current phase %s not found in phase list", f.Current)
	}
	if targetIdx < 0 {
		return fmt.Errorf("target phase %s not found in phase list", phase)
	}
	if targetIdx != currentIdx+1 {
		return fmt.Errorf("can only advance one phase at a time: current=%s target=%s", f.Current, phase)
	}
	now := time.Now()
	if ps, ok := f.PhaseStates[f.Current]; ok {
		ps.Status = StatusPassed
		ps.CompletedAt = &now
	}
	f.PhaseStates[phase].Status = StatusInProgress
	f.PhaseStates[phase].StartedAt = &now
	f.Current = phase
	f.Status = StatusInProgress
	f.UpdatedAt = now
	return nil
}

func (f *Feature) RecirculateTo(phase Phase) error {
	phases := AllPhases()
	currentIdx := -1
	targetIdx := -1
	for i, p := range phases {
		if p == f.Current {
			currentIdx = i
		}
		if p == phase {
			targetIdx = i
		}
	}
	if currentIdx < 0 || targetIdx < 0 {
		return fmt.Errorf("invalid phase in recirculation: current=%s target=%s", f.Current, phase)
	}
	if targetIdx >= currentIdx {
		return fmt.Errorf("cannot recirculate forward: current=%s target=%s", f.Current, phase)
	}
	now := time.Now()
	if ps, ok := f.PhaseStates[f.Current]; ok {
		ps.Status = StatusRecirculated
		ps.CompletedAt = &now
	}
	for i := targetIdx + 1; i <= currentIdx; i++ {
		p := phases[i]
		if ps, ok := f.PhaseStates[p]; ok {
			if p != f.Current {
				ps.Status = StatusDraft
				ps.GateResult = nil
				ps.Artifacts = nil
				ps.StartedAt = nil
				ps.CompletedAt = nil
			}
		}
	}
	f.PhaseStates[phase].Status = StatusInProgress
	f.PhaseStates[phase].StartedAt = &now
	f.Current = phase
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
	if ps, ok := f.PhaseStates[PhaseDelivery]; ok {
		ps.Status = StatusPassed
		ps.CompletedAt = &now
	}
	f.Current = PhaseDelivery
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
