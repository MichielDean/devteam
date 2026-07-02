package role

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type DispatchRequest struct {
	FeatureID  string
	Phase      string
	StageID    string
	Role       string
	Context    string
	Timeout    time.Duration
	WorkingDir string
}

type DispatchResult struct {
	FeatureID string        `yaml:"feature_id" json:"feature_id"`
	Phase     string        `yaml:"phase" json:"phase"`
	Role      string        `yaml:"role" json:"role"`
	Output    string        `yaml:"output" json:"output"`
	Error     string        `yaml:"error,omitempty" json:"error,omitempty"`
	Duration  time.Duration `yaml:"duration" json:"duration"`
	Success   bool          `yaml:"success" json:"success"`
}

type Dispatcher struct {
	workingDir string
	timeout    time.Duration
	tmux       *TmuxSessionManager
}

func NewDispatcher(workingDir string) *Dispatcher {
	return &Dispatcher{
		workingDir: workingDir,
		tmux:       NewTmuxSessionManager(workingDir),
	}
}

func (d *Dispatcher) WithTimeout(timeout time.Duration) *Dispatcher {
	d.timeout = timeout
	return d
}

type OutputLine struct {
	Line     string
	IsStderr bool
}

func (d *Dispatcher) DispatchStreaming(ctx context.Context, req DispatchRequest, lineCh chan<- OutputLine) (*DispatchResult, error) {
	return d.tmux.DispatchStreaming(ctx, req, lineCh)
}

func (d *Dispatcher) Dispatch(ctx context.Context, req DispatchRequest) (*DispatchResult, error) {
	return d.DispatchStreaming(ctx, req, nil)
}

func (d *Dispatcher) IsSessionAlive(featureID string) bool {
	return d.tmux.IsSessionAlive(d.tmux.SessionName(featureID))
}

func (d *Dispatcher) CaptureOutput(featureID string) (string, error) {
	return d.tmux.CaptureOutput(d.tmux.SessionName(featureID))
}

func (d *Dispatcher) ListActiveSessions() map[string]string {
	return d.tmux.ListActiveSessions()
}

func (d *Dispatcher) KillSession(featureID string) error {
	return d.tmux.KillSession(d.tmux.SessionName(featureID))
}

func buildContextMD(req DispatchRequest) string {
	var b strings.Builder
	b.WriteString("# Dev Team Context\n\n")
	b.WriteString(fmt.Sprintf("Feature: %s\n", req.FeatureID))
	if req.StageID != "" {
		b.WriteString(fmt.Sprintf("Stage: %s\n", req.StageID))
	}
	b.WriteString(fmt.Sprintf("Phase: %s\n", req.Phase))
	b.WriteString(fmt.Sprintf("Role: %s\n\n", req.Role))
	b.WriteString("---\n\n")
	b.WriteString(req.Context)
	return b.String()
}

func buildAgentMD(req DispatchRequest) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("description: Dev Team " + req.Role + " role for feature " + req.FeatureID + "\n")
	b.WriteString("mode: primary\n")
	b.WriteString("---\n\n")
	b.WriteString(fmt.Sprintf("You are the %s role in the Dev Team pipeline.\n", req.Role))
	b.WriteString(fmt.Sprintf("Feature: %s\n", req.FeatureID))
	b.WriteString(fmt.Sprintf("Phase: %s\n\n", req.Phase))
	b.WriteString("Your task: Execute your role for this phase. Produce the required artifacts.\n\n")
	b.WriteString("Read CONTEXT.md (provided via OPENCODE_CONFIG_DIR) for the full context including spec artifacts, AIDLC rules, feature state, and implementation repository worktree paths.\n")
	return b.String()
}

// KnowledgeInjector formats team knowledge and learned rules for agent context injection.
// Called by pipeline before dispatch to append team knowledge + rules to req.Context.
type KnowledgeInjector interface {
	GetTeamKnowledge(agentName string) ([]TeamKnowledgeEntry, error)
	GetRulesForAgent(agentName, featureID string) ([]RuleEntry, error)
}

type TeamKnowledgeEntry struct {
	Topic   string
	Content string
}

type RuleEntry struct {
	StageID  string
	RuleText string
}

// FormatKnowledgeAndRules produces a markdown section for agent context.
func FormatKnowledgeAndRules(knowledge []TeamKnowledgeEntry, rules []RuleEntry) string {
	var b strings.Builder
	hasContent := false

	if len(knowledge) > 0 {
		b.WriteString("\n## Team Knowledge\n\n")
		for _, k := range knowledge {
			b.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", k.Topic, k.Content))
		}
		hasContent = true
	}

	if len(rules) > 0 {
		b.WriteString("\n## Learned Rules (from prior gate rejections)\n\n")
		b.WriteString("These rules were generated from corrections in previous stages. Follow them:\n\n")
		for _, r := range rules {
			if r.StageID != "" {
				b.WriteString(fmt.Sprintf("- [%s] %s\n", r.StageID, r.RuleText))
			} else {
				b.WriteString(fmt.Sprintf("- %s\n", r.RuleText))
			}
		}
		hasContent = true
	}

	if !hasContent {
		return ""
	}
	return b.String()
}