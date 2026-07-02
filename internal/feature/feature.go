package feature

import (
	"strings"
	"time"
)

// Feature is the core entity for a development initiative.
// AIDLC v2: stage-based workflow. Phase is derived from CurrentStage.
type Feature struct {
	ID            string     `yaml:"id" json:"id"`
	Title         string     `yaml:"title" json:"title"`
	Status        Status     `yaml:"status" json:"status"`
	Priority      int        `yaml:"priority" json:"priority"`
	IntakePath    IntakePath `yaml:"intake_path" json:"intake_path"`
	SpecDir       string     `yaml:"spec_dir" json:"spec_dir"`
	WorktreeDir   string     `yaml:"worktree_dir,omitempty" json:"worktree_dir,omitempty"`
	CreatedAt     time.Time  `yaml:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `yaml:"updated_at" json:"updated_at"`
	Dependencies  []string   `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Repos         []RepoRef  `yaml:"repos,omitempty" json:"repos,omitempty"`
	PreparedRepos []PreparedRepo `yaml:"prepared_repos,omitempty" json:"prepared_repos,omitempty"`
	Scope         string     `yaml:"scope,omitempty" json:"scope,omitempty"`
	Depth         string     `yaml:"depth,omitempty" json:"depth,omitempty"`
	TestStrategy  string     `yaml:"test_strategy,omitempty" json:"test_strategy,omitempty"`
	AutonomyMode  string     `yaml:"autonomy_mode,omitempty" json:"autonomy_mode,omitempty"`
	CurrentStage  string     `yaml:"current_stage,omitempty" json:"current_stage,omitempty"`
	CurrentBolt   int        `yaml:"current_bolt,omitempty" json:"current_bolt,omitempty"`
}

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

func NewFeature(id, title string, priority int, intakePath IntakePath) *Feature {
	now := time.Now()
	return &Feature{
		ID:         id,
		Title:      title,
		Status:     StatusDraft,
		Priority:   priority,
		IntakePath: intakePath,
		SpecDir:    "specs/" + id + "/",
		CreatedAt:  now,
		UpdatedAt:  now,
		Scope:      "feature",
		Depth:      "standard",
		TestStrategy: "standard",
	}
}

// CurrentPhase derives the AIDLC v2 phase from CurrentStage.
// Stage IDs are "0.1", "1.3", "3.5" etc. Phase is the first digit.
func (f *Feature) CurrentPhase() string {
	if f.CurrentStage == "" {
		return "ideation"
	}
	parts := strings.SplitN(f.CurrentStage, ".", 2)
	if len(parts) < 1 {
		return "ideation"
	}
	switch parts[0] {
	case "0":
		return "initialization"
	case "1":
		return "ideation"
	case "2":
		return "inception"
	case "3":
		return "construction"
	case "4":
		return "operation"
	default:
		return "ideation"
	}
}

// CurrentPhaseLegacy returns the old phase name for backward compat with DB.
// Maps AIDLC v2 phases to old phase names used in the features table.
func (f *Feature) CurrentPhaseLegacy() string {
	phase := f.CurrentPhase()
	switch phase {
	case "initialization", "ideation":
		return "inception"
	case "inception":
		return "planning"
	case "construction":
		return "construction"
	case "operation":
		return "delivery"
	default:
		return "inception"
	}
}

func (f *Feature) Cancel() {
	f.Status = StatusCancelled
	f.UpdatedAt = time.Now()
}

func (f *Feature) MarkDone() {
	f.Status = StatusDone
	f.UpdatedAt = time.Now()
}

func (f *Feature) IsTerminal() bool {
	return f.Status == StatusDone || f.Status == StatusCancelled
}

func (f *Feature) WaitForHuman() error {
	f.Status = StatusWaitingFeedback
	f.UpdatedAt = time.Now()
	return nil
}

func (f *Feature) Slug() string {
	s := strings.ToLower(f.Title)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}