package role

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TmuxSessionManager manages tmux sessions for agent dispatch.
// Based on Cistern's proven session.go patterns.
type TmuxSessionManager struct {
	workingDir string
}

func NewTmuxSessionManager(workingDir string) *TmuxSessionManager {
	return &TmuxSessionManager{workingDir: workingDir}
}

// SessionName returns the tmux session name for a feature.
func (m *TmuxSessionManager) SessionName(featureID string) string {
	safe := strings.NewReplacer(" ", "-", ":", "-", ".", "-", "/", "-", "\\", "-").Replace(featureID)
	return "devteam-" + safe
}

// shellQuote wraps s in single quotes, escaping any single quotes within s.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// DispatchStreaming runs the agent in a tmux session, capturing output via pipe-pane.
func (m *TmuxSessionManager) DispatchStreaming(ctx context.Context, req DispatchRequest, lineCh chan<- OutputLine) (*DispatchResult, error) {
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
	}

	contextDir, err := m.prepareContextDir(req)
	if err != nil {
		return nil, fmt.Errorf("preparing context directory: %w", err)
	}
	defer os.RemoveAll(contextDir)

	agentMDPath := filepath.Join(contextDir, "agents", req.Role+".md")
	// Absolute paths so the agent finds CONTEXT.md and agent.md regardless
	// of CWD. Impl phases dispatch with CWD = impl repo worktree, where a
	// bare "CONTEXT.md" would not exist.
	contextMDPath := filepath.Join(contextDir, "CONTEXT.md")
	shortPrompt := "Read " + contextMDPath + " for your task and begin work. Follow the instructions in " + agentMDPath

	cmdPath, err := exec.LookPath("opencode")
	if err != nil {
		// Check common locations
		for _, p := range []string{
			os.ExpandEnv("$HOME/.opencode/bin/opencode"),
			"/usr/local/bin/opencode",
			"/usr/bin/opencode",
		} {
			if _, statErr := os.Stat(p); statErr == nil {
				cmdPath = p
				break
			}
		}
		if cmdPath == "" {
			cmdPath = "opencode"
		}
	}

	sessionName := m.SessionName(req.FeatureID)

	// Kill any existing session for this feature
	m.KillSession(sessionName)

	// CWD for the agent: per-dispatch override if set, else the manager default.
	workingDir := m.workingDir
	if req.WorkingDir != "" {
		workingDir = req.WorkingDir
	}

	// Build tmux args using Cistern's pattern: -e flags + exec prefix
	args := []string{"new-session", "-d", "-s", sessionName, "-c", workingDir}

	// Env vars via -e flags (Cistern pattern)
	envPairs := []struct{ k, v string }{
		{"OPENCODE_DISABLE_PROJECT_CONFIG", "1"},
		{"OPENCODE_CONFIG_DIR", contextDir},
		{"CT_CATARACTA_NAME", req.Role},
		{"GIT_EDITOR", "true"},
		{"GIT_SEQUENCE_EDITOR", "true"},
		{"OPENCODE_SERVER_USERNAME", ""},
		{"OPENCODE_SERVER_PASSWORD", ""},
		{"OPENCODE_PID", ""},
		{"OPENCODE", ""},
	}
	for _, e := range envPairs {
		args = append(args, "-e", e.k+"="+e.v)
	}
	// Inject the provider's API key value by env var name (CON-004). The
	// value is read at dispatch time and never written to disk or logs.
	if req.Provider != nil && req.Provider.APIKeyEnv != "" && req.Provider.APIKeyValue != "" {
		args = append(args, "-e", req.Provider.APIKeyEnv+"="+req.Provider.APIKeyValue)
	}
	// Always pass PATH so the agent can find binaries
	tmuxPath := os.Getenv("PATH")
	if home := os.Getenv("HOME"); home != "" {
		// Ensure .opencode/bin and go/bin are in PATH for agent sessions
		for _, binDir := range []string{home + "/.opencode/bin", home + "/go/bin"} {
			if !strings.Contains(tmuxPath, binDir) {
				tmuxPath = binDir + ":" + tmuxPath
			}
		}
	}
	if tmuxPath != "" {
		args = append(args, "-e", "PATH="+tmuxPath)
	}
	if home := os.Getenv("HOME"); home != "" {
		args = append(args, "-e", "HOME="+home)
	}

	// Build the agent command — tmux runs this via sh -c, so the string is a shell command
	// Use exec so the shell replaces itself with opencode (Cistern pattern)
	agentCmd := fmt.Sprintf(
		"exec %s run --dangerously-skip-permissions --agent %s %s",
		cmdPath, req.Role, shellQuote(shortPrompt),
	)
	args = append(args, agentCmd)

	log.Printf("tmux: creating session %s, args: %v", sessionName, args)

	// Create session with minimal env (Cistern pattern)
	createCmd := exec.Command("tmux", args...)
	createCmd.Env = minimalTmuxEnv()
	out, err := createCmd.CombinedOutput()
	if err != nil {
		log.Printf("tmux: failed to create session %s: %v, output: %s", sessionName, err, string(out))
		return nil, fmt.Errorf("creating tmux session: %w: %s", err, string(out))
	}
	log.Printf("tmux: created session %s for feature %s", sessionName, req.FeatureID)

	// Set remain-on-exit off so session dies when process exits (Cistern pattern)
	exec.Command("tmux", "set-window-option", "-t", sessionName, "remain-on-exit", "off").Run()

	// Use pipe-pane to capture ALL output to a log file in the spec worktree
	// (not /tmp — this preserves the log for post-hoc debugging)
	logDir := filepath.Join(m.workingDir, "logs")
	os.MkdirAll(logDir, 0755)
	sessionLogPath := filepath.Join(logDir, req.Phase+"-"+req.Role+".log")
	exec.Command("tmux", "pipe-pane", "-o", "-t", sessionName, "cat >> "+shellQuote(sessionLogPath)).Run()
	log.Printf("tmux: pipe-pane logging to %s", sessionLogPath)
	// Do NOT delete the log file — it's the audit trail

	// Poll for output and completion
	var outputBuf strings.Builder
	lastCaptureLen := 0
	lastOutputTime := time.Now()  // Track when output last changed (liveness)
	staleTimeout := 5 * time.Minute // Kill session if no new output for 5 min

	// Give the session a moment to start
	time.Sleep(2 * time.Second)

	// Capture initial output before first liveness check
	if captureOut, err := exec.Command("tmux", "capture-pane", "-p", "-t", sessionName, "-S", "-500").Output(); err == nil {
		captured := string(captureOut)
		for _, line := range strings.Split(captured, "\n") {
			if line == "" {
				continue
			}
			outputBuf.WriteString(line)
			outputBuf.WriteByte('\n')
			if lineCh != nil {
				lineCh <- OutputLine{Line: line, IsStderr: false}
			}
		}
		lastCaptureLen = len(captured)
		lastOutputTime = time.Now()
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			result.Duration = time.Since(start)
			result.Output = outputBuf.String()
			result.Success = false
			result.Error = fmt.Sprintf("opencode run timed out after %v", result.Duration)
			return result, nil
		case <-ticker.C:
			// Check if session still exists
			if !m.IsSessionAlive(sessionName) {
				result.Duration = time.Since(start)
				result.Output = outputBuf.String()
				log.Printf("tmux: session %s ended, output len=%d", sessionName, outputBuf.Len())
				if outputBuf.Len() == 0 {
					result.Success = false
					result.Error = "session ended immediately — opencode may have failed to start"
				} else {
					result.Success = true
				}
				return result, nil
			}

			// Liveness check — if no new output for staleTimeout, kill the session
			if time.Since(lastOutputTime) > staleTimeout {
				log.Printf("tmux: session %s stale for %v — killing (liveness check)", sessionName, time.Since(lastOutputTime))
				m.KillSession(sessionName)
				result.Duration = time.Since(start)
				result.Output = outputBuf.String()
				result.Success = false
				result.Error = fmt.Sprintf("agent session killed — no output for %v (likely hung or stuck)", staleTimeout)
				return result, nil
			}

			// Capture new output via capture-pane
			captureCmd := exec.Command("tmux", "capture-pane", "-p", "-t", sessionName, "-S", "-500")
			captureOut, err := captureCmd.Output()
			if err != nil {
				continue
			}

			captured := string(captureOut)
			if len(captured) > lastCaptureLen {
				lastOutputTime = time.Now() // Output changed — reset stale timer
				newContent := captured[lastCaptureLen:]
				for _, line := range strings.Split(newContent, "\n") {
					if line == "" {
						continue
					}
					outputBuf.WriteString(line)
					outputBuf.WriteByte('\n')
					if lineCh != nil {
						select {
						case lineCh <- OutputLine{Line: line, IsStderr: false}:
						default:
						}
					}
				}
			}
			lastCaptureLen = len(captured)
		}
	}
}

// minimalTmuxEnv returns a minimal environment for the tmux process (Cistern pattern).
func minimalTmuxEnv() []string {
	home, _ := os.UserHomeDir()
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"USER=" + os.Getenv("USER"),
		"SHELL=" + os.Getenv("SHELL"),
		"TERM=" + os.Getenv("TERM"),
	}
	if tmp := os.Getenv("TMPDIR"); tmp != "" {
		env = append(env, "TMPDIR="+tmp)
	}
	if xrd := os.Getenv("XDG_RUNTIME_DIR"); xrd != "" {
		env = append(env, "XDG_RUNTIME_DIR="+xrd)
	}
	return env
}

// KillSession kills a tmux session by name.
func (m *TmuxSessionManager) KillSession(sessionName string) error {
	exec.Command("tmux", "kill-session", "-t", sessionName).Run()
	return nil
}

// IsSessionAlive checks if a tmux session exists.
func (m *TmuxSessionManager) IsSessionAlive(sessionName string) bool {
	return exec.Command("tmux", "has-session", "-t", sessionName).Run() == nil
}

// CaptureOutput returns the current pane content for a session.
func (m *TmuxSessionManager) CaptureOutput(sessionName string) (string, error) {
	cmd := exec.Command("tmux", "capture-pane", "-p", "-t", sessionName, "-S", "-500")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("capturing tmux output: %w", err)
	}
	return string(out), nil
}

// ListActiveSessions returns feature IDs that have active tmux sessions.
func (m *TmuxSessionManager) ListActiveSessions() map[string]string {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	out, err := cmd.Output()
	if err != nil {
		return map[string]string{}
	}

	sessions := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		name := strings.TrimSpace(line)
		if strings.HasPrefix(name, "devteam-") {
			featureID := strings.TrimPrefix(name, "devteam-")
			sessions[featureID] = name
		}
	}
	return sessions
}

func (m *TmuxSessionManager) prepareContextDir(req DispatchRequest) (string, error) {
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
