package role

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/MichielDean/devteam/internal/config"
)

type DispatchRequest struct {
	FeatureID string
	Phase     string
	Role      string
	Context   string
	Timeout   time.Duration
	// WorkingDir overrides the Dispatcher's default working directory for
	// this dispatch. When set, the agent process is started with this as
	// its CWD. When empty, the Dispatcher's workingDir is used.
	//
	// This is how Dev Team routes agents to the correct repo: spec-only
	// phases (inception, planning) dispatch with WorkingDir = spec repo;
	// impl phases (construction, review, testing, delivery) dispatch with
	// WorkingDir = the prepared implementation repo worktree so the agent
	// writes code into the right tree and commits land on feature/<id>.
	WorkingDir string
	// Provider is the resolved per-role provider config. When nil, dispatch
	// falls back to opencode's default model (CON-010). When set, the
	// dispatcher writes the provider's base_url + model into the generated
	// opencode.json and injects the API key value into the agent process
	// environment (CON-002, CON-004). The key value is never written to
	// disk or logs.
	Provider *config.ResolvedProvider
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
		timeout:    0,
		tmux:       NewTmuxSessionManager(workingDir),
	}
}

func (d *Dispatcher) WithTimeout(timeout time.Duration) *Dispatcher {
	d.timeout = timeout
	return d
}

// dispatchWorkingDir returns the CWD for an agent process: req.WorkingDir
// if set, otherwise the Dispatcher's default workingDir. This is how the
// pipeline routes agents to the correct repo (spec repo vs impl repo
// worktree) without instantiating a new Dispatcher per dispatch.
func (d *Dispatcher) dispatchWorkingDir(req DispatchRequest) string {
	if req.WorkingDir != "" {
		return req.WorkingDir
	}
	return d.workingDir
}

type OutputLine struct {
	Line     string
	IsStderr bool
}

func (d *Dispatcher) DispatchStreaming(ctx context.Context, req DispatchRequest, lineCh chan<- OutputLine) (*DispatchResult, error) {
	// Use tmux for session management and output capture
	if d.tmux != nil {
		return d.tmux.DispatchStreaming(ctx, req, lineCh)
	}
	return d.dispatchDirect(ctx, req, lineCh)
}

// dispatchDirect is the fallback without tmux (original pipe-based approach)
func (d *Dispatcher) dispatchDirect(ctx context.Context, req DispatchRequest, lineCh chan<- OutputLine) (*DispatchResult, error) {
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
	// Use absolute paths so the agent can find CONTEXT.md and its agent.md
	// regardless of its CWD. When the dispatcher's workingDir was always
	// the spec repo, a bare "CONTEXT.md" worked because the file lived in
	// the CWD. Now that impl phases dispatch with CWD = an impl repo
	// worktree, the bare name would point at the impl repo (which has no
	// CONTEXT.md). Absolute path is robust either way.
	contextMDPath := filepath.Join(contextDir, "CONTEXT.md")
	shortPrompt := "Read " + contextMDPath + " for your task and begin work. Follow the instructions in " + agentMDPath

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
	cmd.Dir = d.dispatchWorkingDir(req)
	cmd.Env = buildAgentEnv(contextDir, req.Role, req.Provider)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting opencode: %w", err)
	}

	var outputBuf strings.Builder

	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})

	go func() {
		defer close(stdoutDone)
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			outputBuf.WriteString(line)
			outputBuf.WriteByte('\n')
			if lineCh != nil {
				lineCh <- OutputLine{Line: line, IsStderr: false}
			}
		}
	}()

	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			outputBuf.WriteString(line)
			outputBuf.WriteByte('\n')
			if lineCh != nil {
				lineCh <- OutputLine{Line: line, IsStderr: true}
			}
		}
	}()

	<-stdoutDone
	<-stderrDone

	err = cmd.Wait()
	result.Duration = time.Since(start)
	result.Output = outputBuf.String()

	if err != nil {
		result.Success = false
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Sprintf("opencode run timed out after %v", result.Duration)
		} else {
			result.Error = fmt.Sprintf("opencode run failed: %v\noutput: %s", err, truncateOutput(result.Output, 500))
		}
		return result, nil
	}

	result.Success = true
	return result, nil
}

func (d *Dispatcher) Dispatch(ctx context.Context, req DispatchRequest) (*DispatchResult, error) {
	return d.DispatchStreaming(ctx, req, nil)
}

// IsSessionAlive checks if a tmux session exists for the given feature ID.
func (d *Dispatcher) IsSessionAlive(featureID string) bool {
	if d.tmux == nil {
		return false
	}
	return d.tmux.IsSessionAlive(d.tmux.SessionName(featureID))
}

// CaptureOutput returns the current terminal output for a feature's tmux session.
func (d *Dispatcher) CaptureOutput(featureID string) (string, error) {
	if d.tmux == nil {
		return "", fmt.Errorf("tmux not available")
	}
	return d.tmux.CaptureOutput(d.tmux.SessionName(featureID))
}

// ListActiveSessions returns feature IDs that have active tmux sessions.
func (d *Dispatcher) ListActiveSessions() map[string]string {
	if d.tmux == nil {
		return map[string]string{}
	}
	return d.tmux.ListActiveSessions()
}

// KillSession kills the tmux session for a feature.
func (d *Dispatcher) KillSession(featureID string) error {
	if d.tmux == nil {
		return nil
	}
	return d.tmux.KillSession(d.tmux.SessionName(featureID))
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

	if err := writeOpencodeJSON(contextDir, req.Provider); err != nil {
		os.RemoveAll(contextDir)
		return "", fmt.Errorf("writing opencode.json: %w", err)
	}

	return contextDir, nil
}

// writeOpencodeJSON writes an opencode.json into the temp OPENCODE_CONFIG_DIR
// declaring the provider's base_url + model so opencode uses the configured
// provider (CON-002). When provider is nil, writes a minimal config with no
// model/provider override (CON-010 — opencode default applies).
//
// opencode's schema: top-level `provider` is a record of provider configs;
// each has `options.baseURL`, `options.apiKey`, and `models` mapping model
// IDs → {name}. The top-level `model` is "provider/model". The api key is
// injected here (not via env) because opencode reads it from config; it is
// never written to devteam.yaml or logs (CON-004).
func writeOpencodeJSON(contextDir string, provider *config.ResolvedProvider) error {
	cfg := map[string]any{
		"$schema": "https://opencode.ai/config.json",
	}
	if provider != nil {
		// Derive a provider id from the resolved config. opencode keys
		// providers by id; the model id is "<provider>/<model>". We use
		// "devteam" as the id so it never collides with built-in providers
		// and so the model id is stable and predictable for tests.
		providerID := "devteam"
		cfg["provider"] = map[string]any{
			providerID: map[string]any{
				"name": "Dev Team Provider",
				"npm":  "@ai-sdk/openai-compatible",
				"options": map[string]any{
					"baseURL": provider.BaseURL,
					"apiKey":  provider.APIKeyValue,
				},
				"models": map[string]any{
					provider.Model: map[string]any{
						"name": provider.Model,
					},
				},
			},
		}
		cfg["model"] = providerID + "/" + provider.Model
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling opencode.json: %w", err)
	}
	return os.WriteFile(filepath.Join(contextDir, "opencode.json"), data, 0600)
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
	b.WriteString("Read CONTEXT.md (provided via OPENCODE_CONFIG_DIR) for the full context including spec artifacts, AIDLC rules, feature state, and implementation repository worktree paths.\n")
	return b.String()
}

func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}

// Ensure all output is read to prevent pipe hangs
var _ = io.EOF

// buildAgentEnv returns the environment for a dispatched opencode process.
// When a provider is configured with an api_key_env, the key's value is
// injected by name so the agent (and any provider SDK that reads env) gets
// it. The value is never logged (CON-004).
func buildAgentEnv(contextDir, role string, provider *config.ResolvedProvider) []string {
	env := append(os.Environ(),
		"OPENCODE_SERVER_USERNAME=",
		"OPENCODE_SERVER_PASSWORD=",
		"OPENCODE_PID=",
		"OPENCODE=",
		"OPENCODE_DISABLE_PROJECT_CONFIG=1",
		"OPENCODE_CONFIG_DIR="+contextDir,
		"GIT_EDITOR=true",
		"GIT_SEQUENCE_EDITOR=true",
		"CT_CATARACTA_NAME="+role,
	)
	if provider != nil && provider.APIKeyEnv != "" && provider.APIKeyValue != "" {
		env = append(env, provider.APIKeyEnv+"="+provider.APIKeyValue)
	}
	return env
}
