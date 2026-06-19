package role

import (
	"context"
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
	FeatureID  string
	Phase      string
	Role       string
	Output     string
	Error      string
	Duration   time.Duration
	Success    bool
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
		Phase:    req.Phase,
		Role:     req.Role,
	}

	cmd := exec.CommandContext(ctx, "opencode", "run",
		"--format", "json",
		"--dangerously-skip-permissions",
		"--message", buildPrompt(req),
	)
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
		result.Error = fmt.Sprintf("opencode run failed: %v\noutput: %s", err, string(output))
		return result, nil
	}

	result.Success = true
	return result, nil
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