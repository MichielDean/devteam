package role

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TmuxSessionManager manages persistent tmux sessions for agent dispatch.
// Sessions are scoped per feature+phase (or feature+construction-boltN).
// Sessions are NOT killed when a gate opens or question is asked — they stay
// alive for resume. Context is DB-driven; CONTEXT.md is regenerated fresh
// from DB state before each dispatch.
type TmuxSessionManager struct {
	workingDir string
}

func NewTmuxSessionManager(workingDir string) *TmuxSessionManager {
	return &TmuxSessionManager{workingDir: workingDir}
}

// SessionNameForPhase returns the tmux session name for a feature+phase.
// For construction Bolts, use SessionNameForBolt.
func (m *TmuxSessionManager) SessionNameForPhase(featureID, phase string) string {
	safe := sanitizeSessionID(featureID)
	return "devteam-" + safe + "-" + phase
}

// SessionNameForBolt returns the tmux session name for a construction Bolt.
func (m *TmuxSessionManager) SessionNameForBolt(featureID string, boltNumber int) string {
	safe := sanitizeSessionID(featureID)
	return fmt.Sprintf("devteam-%s-construction-bolt%d", safe, boltNumber)
}

func sanitizeSessionID(s string) string {
	return strings.NewReplacer(" ", "-", ":", "-", ".", "-", "/", "-", "\\", "-").Replace(s)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ContextDirForPhase returns the persistent context directory path for a feature+phase.
func (m *TmuxSessionManager) ContextDirForPhase(featureID, phase string) string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	return filepath.Join(base, "devteam", "sessions", featureID, phase)
}

// ContextDirForBolt returns the persistent context directory path for a construction Bolt.
func (m *TmuxSessionManager) ContextDirForBolt(featureID string, boltNumber int) string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	return filepath.Join(base, "devteam", "sessions", featureID, fmt.Sprintf("construction-bolt%d", boltNumber))
}

// DispatchStreaming runs the agent in a tmux session, capturing output to a log
// file. If the session already exists and is alive, it reuses it (does NOT kill).
// If the session does not exist, it creates a new one.
func (m *TmuxSessionManager) DispatchStreaming(ctx context.Context, req DispatchRequest, lineCh chan<- OutputLine) (*DispatchResult, error) {
	start := time.Now()
	result := &DispatchResult{
		FeatureID: req.FeatureID,
		Phase:      req.Phase,
		Role:       req.Role,
	}

	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	// Use the session name from the request, or derive from feature+phase
	sessionName := req.SessionName
	if sessionName == "" {
		sessionName = m.SessionNameForPhase(req.FeatureID, req.Phase)
	}

	// Use the context dir from the request, or derive
	contextDir := req.ContextDir
	if contextDir == "" {
		contextDir = m.ContextDirForPhase(req.FeatureID, req.Phase)
	}

	// Prepare persistent context dir (does NOT delete after dispatch)
	if err := m.prepareContextDir(req, contextDir); err != nil {
		return nil, fmt.Errorf("preparing context directory: %w", err)
	}

	agentMDPath := filepath.Join(contextDir, "agents", req.Role+".md")
	contextMDPath := filepath.Join(contextDir, "CONTEXT.md")
	shortPrompt := "Read " + contextMDPath + " for your task and begin work. Follow the instructions in " + agentMDPath

	cmdPath, err := exec.LookPath("opencode")
	if err != nil {
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

	workingDir := m.workingDir
	if req.WorkingDir != "" {
		workingDir = req.WorkingDir
	}

	// Log file: per-stage, per-agent. Stage ID in filename prevents clobbering.
	logDir := filepath.Join(contextDir, "logs")
	os.MkdirAll(logDir, 0755)
	logFileName := req.Phase + "-" + req.Role + ".log"
	if req.StageID != "" {
		logFileName = req.StageID + "-" + req.Role + ".log"
	}
	logPath := filepath.Join(logDir, logFileName)
	os.Truncate(logPath, 0)

	// Exit code marker
	exitCodePath := filepath.Join(contextDir, "exit_code")

	// Build the shell command
	agentCmd := fmt.Sprintf(
		"%s run --dangerously-skip-permissions --agent %s %s 2>&1 | tee %s; echo ${PIPESTATUS[0]} > %s",
		cmdPath, req.Role, shellQuote(shortPrompt), shellQuote(logPath), shellQuote(exitCodePath),
	)

	// Check if session already exists — if so, reuse it (don't kill)
	sessionExists := m.IsSessionAlive(sessionName)
	if !sessionExists {
		// Create new session
		args := []string{"new-session", "-d", "-s", sessionName, "-c", workingDir}

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
			// Clear lobsterdog harness env vars so they don't leak into devteam agents
			{"LOBSTERDOG_HOME", ""},
			{"AGENTS_MD_PATH", ""},
		}
		for _, e := range envPairs {
			args = append(args, "-e", e.k+"="+e.v)
		}
		tmuxPath := os.Getenv("PATH")
		if home := os.Getenv("HOME"); home != "" {
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
		args = append(args, agentCmd)

		log.Printf("tmux: creating session %s for feature %s phase %s", sessionName, req.FeatureID, req.Phase)

		createCmd := exec.Command("tmux", args...)
		createCmd.Env = minimalTmuxEnv()
		out, err := createCmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("creating tmux session: %w: %s", err, string(out))
		}
		log.Printf("tmux: created session %s", sessionName)
	} else {
		// Session exists — send the agent command to it
		log.Printf("tmux: reusing existing session %s for feature %s phase %s", sessionName, req.FeatureID, req.Phase)
		// Clear the pane and send the new command
		exec.Command("tmux", "send-keys", "-t", sessionName, "C-c", "").Run()
		time.Sleep(100 * time.Millisecond)
		exec.Command("tmux", "send-keys", "-t", sessionName, agentCmd, "Enter").Run()
	}

	exec.Command("tmux", "set-window-option", "-t", sessionName, "remain-on-exit", "off").Run()

	// Stream output by tailing the log file
	// Use a cancellable context so tailLog stops when the tmux session exits
	streamCtx, streamCancel := context.WithCancel(ctx)
	defer streamCancel()
	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		m.tailLog(streamCtx, logPath, lineCh)
	}()

	// Wait for session command to finish
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			streamCancel()
			m.KillSession(sessionName)
			<-streamDone
			result.Duration = time.Since(start)
			result.Output = readLogFile(logPath)
			result.Success = false
			result.Error = fmt.Sprintf("dispatch cancelled after %v", result.Duration)
			return result, nil

		case <-ticker.C:
			if !m.IsSessionAlive(sessionName) {
				// Cancel the tailLog goroutine — it's following the file forever
				streamCancel()
				<-streamDone
				result.Duration = time.Since(start)
				result.Output = readLogFile(logPath)

				exitCode := readExitCode(exitCodePath)
				if exitCode == "0" {
					result.Success = true
					log.Printf("tmux: session %s exited cleanly", sessionName)
				} else {
					result.Success = false
					result.Error = fmt.Sprintf("agent exited with code %s", exitCode)
					log.Printf("tmux: session %s exited with code %s", sessionName, exitCode)
				}
				return result, nil
			}
		}
	}
}

// tailLog reads the log file line by line and follows it (like tail -f),
// sending each line to lineCh. It keeps reading as the file grows until
// the context is cancelled or the channel is closed.
func (m *TmuxSessionManager) tailLog(ctx context.Context, logPath string, lineCh chan<- OutputLine) {
	// Wait for log file to exist
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(logPath); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}

	f, err := os.Open(logPath)
	if err != nil {
		log.Printf("tailLog: could not open %s: %v", logPath, err)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Read available lines
		for scanner.Scan() {
			line := scanner.Text()
			if lineCh != nil {
				select {
				case lineCh <- OutputLine{Line: line, IsStderr: false}:
				default:
				}
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("tailLog: scanner error: %v", err)
			return
		}

		// EOF reached — check if context is done before waiting for more data
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
			// Wait for more data, then continue scanning
			// Seek to current position to continue reading new data
			pos, _ := f.Seek(0, 1) // get current position
			f.Close()
			f, err = os.Open(logPath)
			if err != nil {
				log.Printf("tailLog: could not reopen %s: %v", logPath, err)
				return
			}
			f.Seek(pos, 0) // seek to where we left off
			scanner = bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
		}
	}
}

func readLogFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	s := string(data)
	if len(s) > 256*1024 {
		return s[:256*1024] + "\n... (truncated)"
	}
	return s
}

func readExitCode(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}

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

func (m *TmuxSessionManager) KillSession(sessionName string) error {
	exec.Command("tmux", "kill-session", "-t", sessionName).Run()
	return nil
}

func (m *TmuxSessionManager) IsSessionAlive(sessionName string) bool {
	return exec.Command("tmux", "has-session", "-t", sessionName).Run() == nil
}

// CapturePane returns the raw tmux capture-pane output for a session.
// This includes ANSI escape sequences — used by the xterm.js pane viewer.
func (m *TmuxSessionManager) CapturePane(sessionName string) (string, error) {
	cmd := exec.Command("tmux", "capture-pane", "-p", "-t", sessionName, "-S", "-500")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("capturing tmux pane: %w", err)
	}
	return string(out), nil
}

// CapturePaneRaw returns the raw tmux capture-pane output including ANSI codes.
// Uses -e flag to preserve escape sequences for xterm.js rendering.
func (m *TmuxSessionManager) CapturePaneRaw(sessionName string) (string, error) {
	cmd := exec.Command("tmux", "capture-pane", "-p", "-e", "-t", sessionName, "-S", "-1000")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("capturing tmux pane (raw): %w", err)
	}
	return string(out), nil
}

func (m *TmuxSessionManager) CaptureOutput(sessionName string) (string, error) {
	return m.CapturePane(sessionName)
}

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
			sessions[name] = name
		}
	}
	return sessions
}

// prepareContextDir creates the persistent context directory and writes
// CONTEXT.md, agent role files, and a self-contained opencode config that
// isolates the agent from the global lobsterdog harness.
func (m *TmuxSessionManager) prepareContextDir(req DispatchRequest, contextDir string) error {
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		return fmt.Errorf("creating context dir: %w", err)
	}

	contextContent := buildContextMD(req)
	contextPath := filepath.Join(contextDir, "CONTEXT.md")
	if err := os.WriteFile(contextPath, []byte(contextContent), 0644); err != nil {
		return fmt.Errorf("writing CONTEXT.md: %w", err)
	}

	agentsDir := filepath.Join(contextDir, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("creating agents dir: %w", err)
	}

	agentContent := buildAgentMD(req)
	agentPath := filepath.Join(agentsDir, req.Role+".md")
	if err := os.WriteFile(agentPath, []byte(agentContent), 0644); err != nil {
		return fmt.Errorf("writing agent markdown: %w", err)
	}

	// Write self-contained opencode config — isolates from global harness
	// Must include provider config so the agent can reach the LLM
	opencodeConfig := `{
  "permission": "allow",
  "instructions": [],
  "plugin": [],
  "compaction": {
    "enabled": false
  },
  "provider": {
    "ollama": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Ollama (local)",
      "options": {
        "baseURL": "http://localhost:11434/v1"
      },
      "models": {
        "glm-5.2:cloud": {
          "name": "GLM 5.2 Cloud"
        }
      }
    }
  }
}`
	configPath := filepath.Join(contextDir, "opencode.json")
	if err := os.WriteFile(configPath, []byte(opencodeConfig), 0644); err != nil {
		return fmt.Errorf("writing opencode.json: %w", err)
	}

	// Write AGENTS.md that tells the agent it's in Dev Team, not lobsterdog
	agentsMD := `# Dev Team Agent

You are running inside the Dev Team AIDLC v2 pipeline. This is a dedicated
harness for AI-driven development — NOT the lobsterdog harness.

## Rules

- You are the ` + req.Role + ` agent for feature ` + req.FeatureID + ` in the ` + req.Phase + ` phase.
- Read CONTEXT.md for your full task context, spec artifacts, and repo paths.
- Use the devteam CLI to manage state — do NOT write state files manually.
- Signal completion with: devteam signal <feature-id> pass
- If you need human input: devteam signal <feature-id> needs_feedback
- Submit artifacts with: devteam artifact submit <feature-id> <type> --file <filename>
- Ask questions with: devteam questions ask <feature-id> --file questions.json

## What NOT to do

- Do NOT follow lobsterdog harness conventions (worktrees, git-sync, etc.)
- Do NOT use llmem, cistern, or any lobsterdog-specific tools
- Do NOT follow caveman ruleset or any global rules
- Do NOT create git worktrees or manage branches — the pipeline handles that
- Do NOT run install.sh or any deployment scripts

## Your task

Execute your role for this stage. Produce the required artifacts. Signal
completion when done. The pipeline handles approval gates, stage advancement,
and state persistence.
`
	agentsPath := filepath.Join(contextDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(agentsMD), 0644); err != nil {
		return fmt.Errorf("writing AGENTS.md: %w", err)
	}

	return nil
}

// GetLogPath returns the log file path for a specific stage+agent in a session.
func (m *TmuxSessionManager) GetLogPath(contextDir, stageID, agentRole string) string {
	logDir := filepath.Join(contextDir, "logs")
	logFileName := stageID + "-" + agentRole + ".log"
	if stageID == "" {
		logFileName = agentRole + ".log"
	}
	return filepath.Join(logDir, logFileName)
}

// ReadStageLog reads the log file for a specific stage+agent.
func (m *TmuxSessionManager) ReadStageLog(contextDir, stageID, agentRole string) string {
	logPath := m.GetLogPath(contextDir, stageID, agentRole)
	return readLogFile(logPath)
}

var _ = io.EOF