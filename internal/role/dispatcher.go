package role

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type DispatchRequest struct {
	FeatureID   string
	Phase       string
	StageID     string
	Role        string
	Context     string
	Timeout     time.Duration
	WorkingDir  string
	SessionName string // tmux session name — if set, reuse existing session; if empty, derive from feature+phase
	ContextDir  string // persistent context dir — if set, use it; if empty, derive from feature+phase
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
	return d.tmux.IsSessionAlive(d.tmux.SessionNameForPhase(featureID, ""))
}

func (d *Dispatcher) CaptureOutput(featureID string) (string, error) {
	return d.tmux.CaptureOutput(d.tmux.SessionNameForPhase(featureID, ""))
}

func (d *Dispatcher) ListActiveSessions() map[string]string {
	return d.tmux.ListActiveSessions()
}

func (d *Dispatcher) KillSession(featureID string) error {
	return d.tmux.KillSession(d.tmux.SessionNameForPhase(featureID, ""))
}

// CapturePaneRaw returns raw ANSI output from the tmux pane for xterm.js rendering.
func (d *Dispatcher) CapturePaneRaw(sessionName string) (string, error) {
	return d.tmux.CapturePaneRaw(sessionName)
}

// IsSessionAliveByName checks if a specific session name is alive.
func (d *Dispatcher) IsSessionAliveByName(sessionName string) bool {
	return d.tmux.IsSessionAlive(sessionName)
}

// KillSessionByName kills a session by its full name.
func (d *Dispatcher) KillSessionByName(sessionName string) error {
	return d.tmux.KillSession(sessionName)
}

// TmuxManager returns the underlying tmux session manager for direct access.
func (d *Dispatcher) TmuxManager() *TmuxSessionManager {
	return d.tmux
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
	b.WriteString("model: ollama/glm-5.2:cloud\n")
	b.WriteString("---\n\n")
	b.WriteString(fmt.Sprintf("You are the %s role in the Dev Team AIDLC v2 pipeline.\n", req.Role))
	b.WriteString(fmt.Sprintf("Feature: %s\n", req.FeatureID))
	b.WriteString(fmt.Sprintf("Phase: %s\n\n", req.Phase))
	b.WriteString("Your task: Execute your role for this stage. Produce the required artifacts.\n\n")
	b.WriteString("## CLI Commands\n\n")
	b.WriteString("### Get prior stage artifacts (DO THIS FIRST)\n")
	b.WriteString(fmt.Sprintf("    devteam artifacts %s              # all artifacts for current stage\n", req.FeatureID))
	b.WriteString(fmt.Sprintf("    devteam artifacts %s --all        # all artifacts from all stages\n", req.FeatureID))
	b.WriteString(fmt.Sprintf("    devteam artifact get %s <type>    # specific artifact by name\n\n", req.FeatureID))
	b.WriteString("### Submit your work\n")
	b.WriteString(fmt.Sprintf("    devteam artifact submit %s <type> --file <filename>\n", req.FeatureID))
	b.WriteString(fmt.Sprintf("    devteam artifact submit %s <type> --content \"inline content\"\n\n", req.FeatureID))
	b.WriteString("### Ask questions (if you need human input)\n")
	b.WriteString(fmt.Sprintf("    devteam questions ask %s --file questions.json\n", req.FeatureID))
	b.WriteString(fmt.Sprintf("    devteam signal %s needs_feedback\n\n", req.FeatureID))
	b.WriteString("### Signal completion\n")
	b.WriteString(fmt.Sprintf("    devteam signal %s pass\n", req.FeatureID))
	b.WriteString(fmt.Sprintf("    devteam signal %s failed --notes \"what went wrong\"\n\n", req.FeatureID))
	b.WriteString("### Query state\n")
	b.WriteString(fmt.Sprintf("    devteam feature status %s\n", req.FeatureID))
	b.WriteString(fmt.Sprintf("    devteam stages %s\n", req.FeatureID))
	b.WriteString(fmt.Sprintf("    devteam audit %s\n\n", req.FeatureID))
	b.WriteString("Read CONTEXT.md for the full context including spec artifacts, AIDLC rules, feature state, and implementation repository worktree paths.\n")
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