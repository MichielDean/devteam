package role

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type DispatchRequest struct {
	FeatureID string
	Phase     string
	Role      string
	Context   string
	Timeout   time.Duration
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
}

func NewDispatcher(workingDir string) *Dispatcher {
	return &Dispatcher{
		workingDir: workingDir,
		timeout:    10 * time.Minute,
	}
}

func (d *Dispatcher) WithTimeout(timeout time.Duration) *Dispatcher {
	d.timeout = timeout
	return d
}

func (d *Dispatcher) Dispatch(ctx context.Context, req DispatchRequest) (*DispatchResult, error) {
	start := time.Now()
	result := &DispatchResult{
		FeatureID: req.FeatureID,
		Phase:     req.Phase,
		Role:      req.Role,
	}

	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	} else if d.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.timeout)
		defer cancel()
	}

	contextDir, err := d.prepareContextDir(req)
	if err != nil {
		return nil, fmt.Errorf("preparing context directory: %w", err)
	}
	defer os.RemoveAll(contextDir)

	agentMDPath := filepath.Join(contextDir, "agents", req.Role+".md")
	shortPrompt := "Read CONTEXT.md for your task and begin work. Follow the instructions in " + filepath.Base(agentMDPath)

	args := []string{
		"run",
		"--dangerously-skip-permissions",
		"--agent", req.Role,
		shortPrompt,
	}

	cmdPath, err := exec.LookPath("opencode")
	if err != nil {
		cmdPath = "opencode"
	}

	cmd := exec.CommandContext(ctx, cmdPath, args...)
	cmd.Dir = d.workingDir
	cmd.Env = append(os.Environ(),
		"OPENCODE_SERVER_USERNAME=",
		"OPENCODE_SERVER_PASSWORD=",
		"OPENCODE_PID=",
		"OPENCODE=",
		"OPENCODE_DISABLE_PROJECT_CONFIG=1",
		"OPENCODE_CONFIG_DIR="+contextDir,
		"GIT_EDITOR=true",
		"GIT_SEQUENCE_EDITOR=true",
		"CT_CATARACTA_NAME="+req.Role,
	)

	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)
	result.Output = string(output)

	if err != nil {
		result.Success = false
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Sprintf("opencode run timed out after %v", result.Duration)
		} else {
			result.Error = fmt.Sprintf("opencode run failed: %v\noutput: %s", err, truncateOutput(string(output), 500))
		}
		return result, nil
	}

	result.Success = true
	return result, nil
}

func (d *Dispatcher) DispatchCrossRepo(ctx context.Context, req DispatchRequest, repoNames []string) (*DispatchResult, error) {
	req.Context = fmt.Sprintf("%s\n\n=== Cross-Repo Context ===\nThis feature spans the following repositories: %s\nReview ALL repositories against the SAME spec acceptance criteria.", req.Context, strings.Join(repoNames, ", "))
	return d.Dispatch(ctx, req)
}

func (d *Dispatcher) prepareContextDir(req DispatchRequest) (string, error) {
	contextDir, err := os.MkdirTemp("", "devteam-"+req.Role+"-*")
	if err != nil {
		return "", fmt.Errorf("creating temp context dir: %w", err)
	}

	contextContent := buildContextMD(req)
	contextPath := filepath.Join(contextDir, "CONTEXT.md")
	if err := os.WriteFile(contextPath, []byte(contextContent), 0644); err != nil {
		os.RemoveAll(contextDir)
		return "", fmt.Errorf("writing CONTEXT.md: %w", err)
	}

	agentsDir := filepath.Join(contextDir, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		os.RemoveAll(contextDir)
		return "", fmt.Errorf("creating agents dir: %w", err)
	}

	agentContent := buildAgentMD(req)
	agentPath := filepath.Join(agentsDir, req.Role+".md")
	if err := os.WriteFile(agentPath, []byte(agentContent), 0644); err != nil {
		os.RemoveAll(contextDir)
		return "", fmt.Errorf("writing agent markdown: %w", err)
	}

	return contextDir, nil
}

func buildContextMD(req DispatchRequest) string {
	var b strings.Builder
	b.WriteString("# Dev Team Context\n\n")
	b.WriteString(fmt.Sprintf("Feature: %s\n", req.FeatureID))
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
	b.WriteString("Read CONTEXT.md in the working directory for the full context including spec artifacts, AIDLC rules, and feature state.\n")
	return b.String()
}

func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}