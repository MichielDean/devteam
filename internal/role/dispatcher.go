package role

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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

	prompt := buildPrompt(req)

	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	} else if d.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.timeout)
		defer cancel()
	}

	args := []string{
		"run",
		"--format", "json",
		"--dangerously-skip-permissions",
		prompt,
	}

	cmd := exec.CommandContext(ctx, "opencode", args...)
	cmd.Dir = d.workingDir
	cmd.Env = append(os.Environ(),
		"OPENCODE_SERVER_USERNAME=",
		"OPENCODE_SERVER_PASSWORD=",
		"OPENCODE_PID=",
		"OPENCODE=",
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

func buildPrompt(req DispatchRequest) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("You are the %s role in the Dev Team pipeline.\n", req.Role))
	b.WriteString(fmt.Sprintf("Feature: %s\n", req.FeatureID))
	b.WriteString(fmt.Sprintf("Phase: %s\n\n", req.Phase))
	b.WriteString(req.Context)
	b.WriteString("\n\nExecute your role for this phase. Produce the required artifacts.")
	return b.String()
}

type OpenCodeEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}