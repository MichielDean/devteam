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

// TmuxSessionManager manages tmux sessions for agent dispatch.
type TmuxSessionManager struct {
	workingDir string
}

func NewTmuxSessionManager(workingDir string) *TmuxSessionManager {
	return &TmuxSessionManager{workingDir: workingDir}
}

func (m *TmuxSessionManager) SessionName(featureID string) string {
	safe := strings.NewReplacer(" ", "-", ":", "-", ".", "-", "/", "-", "\\", "-").Replace(featureID)
	return "devteam-" + safe
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// DispatchStreaming runs the agent in a tmux session, capturing output to a log
// file. Output is streamed by tailing the log file. The session's exit code
// determines Success — no more guessing from capture-pane.
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

	sessionName := m.SessionName(req.FeatureID)
	m.KillSession(sessionName)

	workingDir := m.workingDir
	if req.WorkingDir != "" {
		workingDir = req.WorkingDir
	}

	// Log file: full output audit trail. Read by getCapturedOutput and streamed to SSE.
	logDir := filepath.Join(workingDir, "logs")
	os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, req.Phase+"-"+req.Role+".log")
	os.Truncate(logPath, 0)

	// Exit code marker: written by the wrapper command when opencode exits.
	exitCodePath := filepath.Join(contextDir, "exit_code")

	// Build the shell command: run opencode, tee output to log, capture exit code.
	agentCmd := fmt.Sprintf(
		"%s run --dangerously-skip-permissions --agent %s %s 2>&1 | tee %s; echo ${PIPESTATUS[0]} > %s",
		cmdPath, req.Role, shellQuote(shortPrompt), shellQuote(logPath), shellQuote(exitCodePath),
	)

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

	exec.Command("tmux", "set-window-option", "-t", sessionName, "remain-on-exit", "off").Run()

	// Stream output by tailing the log file. Stop when session dies and exit code appears.
	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		m.tailLog(ctx, logPath, lineCh)
	}()

	// Wait for session to end. Only kill on context cancellation — no stale timeout.
	// LLM calls legitimately produce no stdout for minutes; killing mid-thought is wrong.
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.KillSession(sessionName)
			<-streamDone
			result.Duration = time.Since(start)
			result.Output = readLogFile(logPath)
			result.Success = false
			result.Error = fmt.Sprintf("dispatch cancelled after %v", result.Duration)
			return result, nil

		case <-ticker.C:
			if !m.IsSessionAlive(sessionName) {
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

// tailLog reads the log file line by line, sending each line to lineCh.
// Blocks until ctx is cancelled or the file reader hits EOF (tee closes the file).
func (m *TmuxSessionManager) tailLog(ctx context.Context, logPath string, lineCh chan<- OutputLine) {
	// Wait for the file to appear
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

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
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

func (m *TmuxSessionManager) CaptureOutput(sessionName string) (string, error) {
	cmd := exec.Command("tmux", "capture-pane", "-p", "-t", sessionName, "-S", "-500")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("capturing tmux output: %w", err)
	}
	return string(out), nil
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

	return contextDir, nil
}

var _ = io.EOF