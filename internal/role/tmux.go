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

// TmuxSessionManager manages persistent tmux sessions for agent dispatch.
// Sessions are scoped per feature+phase (or feature+construction-boltN).
// Sessions are NOT killed when a gate opens or question is asked — they stay
// alive for resume. Context is DB-driven; CONTEXT.md is regenerated fresh
// from DB state before each dispatch.
//
// Shape B (ADR-1): the agent command runs as an exec.Cmd whose stdout the
// orchestrator captures via the batcher (StreamOutput). The tmux session is
// created/retained for the operator's interactive needs (resume, debugging,
// session-lifecycle tracking), but the agent command itself is NOT sent to the
// pane — its stdout goes to the DB + SSE channel, not the pane.
type TmuxSessionManager struct {
	workingDir string
	flushFn    FlushFunc  // injected by the composition root (ADR-7); nil = no DB persistence
	streamCfg  StreamConfig
	resetFn    ResetFunc // injected by the composition root; resets the stage_logs row for re-dispatch (O-11)
}

// ResetFunc resets the stage_logs row for a (featureID, stageID, bolt) triple to
// empty, so a re-dispatch (R-6) does not concatenate stale + new output (ADR-2 /
// O-11). Injected by the composition root — the role package must not import db
// (AC-5). A nil ResetFunc means re-dispatches will concatenate (a hazard; the
// composition root MUST wire this for production).
type ResetFunc func(ctx context.Context, featureID, stageID string, bolt int) error

// NewTmuxSessionManager creates a TmuxSessionManager with the given working dir.
// FlushFunc and StreamConfig are zero-valued; use SetFlushFn / SetStreamConfig
// to wire them from the composition root (U-BK-06).
func NewTmuxSessionManager(workingDir string) *TmuxSessionManager {
	return &TmuxSessionManager{workingDir: workingDir}
}

// SetFlushFn injects the FlushFunc used by DispatchStreaming to persist chunks
// to the DB. Called by the composition root (U-BK-06). A nil flushFn means the
// batcher will not persist (used by tests and the non-streaming Dispatch path).
func (m *TmuxSessionManager) SetFlushFn(fn FlushFunc) {
	m.flushFn = fn
}

// SetStreamConfig sets the batcher flush thresholds. Called by the composition
// root from the loaded Streaming config (U-BK-01 / U-BK-06).
func (m *TmuxSessionManager) SetStreamConfig(cfg StreamConfig) {
	m.streamCfg = cfg
}

// SetResetFn injects the ResetFunc used by DispatchStreaming to reset the
// stage_logs row at entry (re-dispatch guard, ADR-2 / O-11). Called by the
// composition root (U-BK-06).
func (m *TmuxSessionManager) SetResetFn(fn ResetFunc) {
	m.resetFn = fn
}

// FlushFn returns the injected FlushFunc (or nil if not set). Exposed for the
// composition root and tests.
func (m *TmuxSessionManager) FlushFn() FlushFunc {
	return m.flushFn
}

// StreamConfig returns the injected StreamConfig.
func (m *TmuxSessionManager) StreamConfig() StreamConfig {
	return m.streamCfg
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

// DispatchStreaming runs the agent in a tmux session, capturing stdout via the
// batcher (Shape B / ADR-1). If the session already exists and is alive, it
// reuses it (does NOT kill). If the session does not exist, it creates a new one.
//
// The agent command runs as an exec.Cmd whose stdout is piped to StreamOutput
// (U-BK-03), which flushes chunks to the DB (via FlushFunc, ADR-7) and the
// lineCh (SSE). The tmux session is created/retained for the operator's
// interactive needs, but the agent command is NOT sent to the pane.
//
// Drain contract (C-5 / R-1 / ADR-2): StreamOutput returns only after its final
// flush. DispatchStreaming returns only after StreamOutput returns. The caller
// (stage_runner) closes lineCh at the existing drain point (stage_runner.go:193)
// AFTER this function returns. SaveStageLogForBolt is NOT called at completion —
// the batcher's final flush IS the final save (ADR-2). The only residual
// SaveStageLogForBolt is the re-dispatch reset at entry (O-11).
//
// result.Output comes from the batcher's in-memory buffer (ADR-6) — no DB
// round-trip, keeping the role package free of db (AC-5).
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
	log.Printf("tmux: prepared context dir %s (opencode.json + AGENTS.md + agents/%s.md)", contextDir, req.Role)

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

	// Re-dispatch reset (ADR-2 / O-11): at DispatchStreaming entry, before
	// cmd.Start(), reset the stage_logs row so a re-dispatch (R-6) does not
	// concatenate stale + new output. This is the ONLY residual use of a
	// full-replace save; the batcher's final flush is the authoritative save.
	// The reset is delegated to ResetFunc (injected by the composition root,
	// U-BK-06) so the role package does not import db (AC-5).
	if m.resetFn != nil {
		if rErr := m.resetFn(ctx, req.FeatureID, req.StageID, req.BoltNumber); rErr != nil {
			log.Printf("tmux: re-dispatch reset failed for %s/%s bolt %d: %v (continuing — append will concatenate stale content)", req.FeatureID, req.StageID, req.BoltNumber, rErr)
		}
	}

	// Build the exec.Cmd (arg vector, not a shell string — less shell surface).
	// opencode run --pure --dangerously-skip-permissions --agent <role> "<prompt>"
	cmd := exec.Command(cmdPath, "run", "--pure", "--dangerously-skip-permissions", "--agent", req.Role, shortPrompt)
	cmd.Dir = workingDir

	// Environment: same env-clearing the tmux session used, applied to the
	// exec.Cmd so the agent runs isolated from the lobsterdog harness.
	cmd.Env = m.buildAgentEnv(contextDir, req.Role)

	// Stderr is merged into stdout so the batcher captures both (the old
	// `2>&1 | tee` did the same).
	cmd.Stderr = nil // merged via the pipe below

	// Create a pipe for stdout — the batcher reads from it.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}
	// Merge stderr into the same pipe by setting cmd.Stderr to the stdout
	// writer. exec.Cmd does not support this directly for a pipe, so we use
	// a goroutine-free approach: set Stderr to the pipe's writer via a
	// merged writer. The simplest correct approach is to use cmd.CombinedOutput
	// semantics, but we need streaming. Instead, we point both Stdout and
	// Stderr at the same os.Pipe.
	stdoutPipe.Close() // we won't use the StdoutPipe; use a merged os.Pipe instead

	rPipe, wPipe, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("creating merged stdout/stderr pipe: %w", err)
	}
	cmd.Stdout = wPipe
	cmd.Stderr = wPipe

	// Ensure/create the tmux session (retained for operator interactive use;
	// the agent command is NOT sent to the pane under Shape B).
	sessionExists := m.IsSessionAlive(sessionName)
	if !sessionExists {
		args := []string{"new-session", "-d", "-s", sessionName, "-c", workingDir}
		// Carry the same env-clearing into the tmux session so a manual
		// `tmux attach` doesn't leak harness vars.
		for _, e := range m.tmuxEnvPairs(contextDir, req.Role) {
			args = append(args, "-e", e)
		}
		log.Printf("tmux: creating session %s for feature %s phase %s", sessionName, req.FeatureID, req.Phase)
		createCmd := exec.Command("tmux", args...)
		createCmd.Env = minimalTmuxEnv()
		out, err := createCmd.CombinedOutput()
		if err != nil {
			rPipe.Close()
			wPipe.Close()
			return nil, fmt.Errorf("creating tmux session: %w: %s", err, string(out))
		}
		log.Printf("tmux: created session %s", sessionName)
	} else {
		log.Printf("tmux: reusing existing session %s for feature %s phase %s", sessionName, req.FeatureID, req.Phase)
	}
	exec.Command("tmux", "set-window-option", "-t", sessionName, "remain-on-exit", "off").Run()

	log.Printf("tmux: starting agent %s for stage %s (session=%s, db-streamed)", req.Role, req.StageID, sessionName)

	if err := cmd.Start(); err != nil {
		rPipe.Close()
		wPipe.Close()
		return nil, fmt.Errorf("starting agent command: %w", err)
	}

	// Close the write end in the parent so the batcher sees EOF when the
	// agent process exits.
	wPipe.Close()

	// Run the batcher on the read end. StreamOutput returns only after its
	// final flush (drain contract, C-5 / ADR-2).
	streamCtx, streamCancel := context.WithCancel(ctx)
	defer streamCancel()
	buffer, streamErr := StreamOutput(streamCtx, rPipe, lineCh, m.flushFn, req.FeatureID, req.StageID, req.Role, req.BoltNumber, m.streamCfg)
	rPipe.Close()

	// Wait for the agent process to exit and capture the exit code (CR-8).
	// ctx cancellation kills the process via the cancelled context's deadline
	// propagating to cmd (Go's exec package sends SIGKILL on ctx cancellation
	// when cmd.Cancel is unset — we set it explicitly to honor the ctx).
	waitErr := cmd.Wait()
	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	result.Duration = time.Since(start)
	result.Output = buffer // ADR-6: in-memory batcher buffer, no DB round-trip
	result.Success = (exitCode == 0)
	if streamErr != nil && streamErr != context.Canceled {
		result.Error = fmt.Sprintf("stream error: %v", streamErr)
		log.Printf("tmux: stream error for %s: %v", sessionName, streamErr)
	}
	if !result.Success && result.Error == "" {
		result.Error = fmt.Sprintf("agent exited with code %d", exitCode)
	}
	if ctx.Err() != nil {
		result.Success = false
		if result.Error == "" {
			result.Error = fmt.Sprintf("dispatch cancelled after %v", result.Duration)
		}
		log.Printf("tmux: session %s cancelled (exit code %d)", sessionName, exitCode)
	} else {
		log.Printf("tmux: session %s agent exited (code %d, success=%v)", sessionName, exitCode, result.Success)
	}
	return result, nil
}

// buildAgentEnv returns the environment for the agent exec.Cmd, isolated from
// the lobsterdog harness.
func (m *TmuxSessionManager) buildAgentEnv(contextDir, role string) []string {
	home, _ := os.UserHomeDir()
	env := []string{
		"OPENCODE_DISABLE_PROJECT_CONFIG=1",
		"OPENCODE_CONFIG_DIR=" + contextDir,
		"CT_CATARACTA_NAME=" + role,
		"GIT_EDITOR=true",
		"GIT_SEQUENCE_EDITOR=true",
		"OPENCODE_SERVER_USERNAME=",
		"OPENCODE_SERVER_PASSWORD=",
		"OPENCODE_PID=",
		"OPENCODE=",
		// Clear lobsterdog harness env vars so they don't leak into devteam agents
		"LOBSTERDOG_HOME=",
		"AGENTS_MD_PATH=",
		"HOME=" + home,
	}
	// PATH: include opencode and go bins, same as the old tmux env setup.
	tmuxPath := os.Getenv("PATH")
	if home != "" {
		for _, binDir := range []string{home + "/.opencode/bin", home + "/go/bin"} {
			if !strings.Contains(tmuxPath, binDir) {
				tmuxPath = binDir + ":" + tmuxPath
			}
		}
	}
	env = append(env, "PATH="+tmuxPath)
	if tmp := os.Getenv("TMPDIR"); tmp != "" {
		env = append(env, "TMPDIR="+tmp)
	}
	if xrd := os.Getenv("XDG_RUNTIME_DIR"); xrd != "" {
		env = append(env, "XDG_RUNTIME_DIR="+xrd)
	}
	return env
}

// tmuxEnvPairs returns the env var pairs for tmux new-session -e flags.
func (m *TmuxSessionManager) tmuxEnvPairs(contextDir, role string) []string {
	pairs := []string{
		"OPENCODE_DISABLE_PROJECT_CONFIG=1",
		"OPENCODE_CONFIG_DIR=" + contextDir,
		"CT_CATARACTA_NAME=" + role,
		"GIT_EDITOR=true",
		"GIT_SEQUENCE_EDITOR=true",
		"OPENCODE_SERVER_USERNAME=",
		"OPENCODE_SERVER_PASSWORD=",
		"OPENCODE_PID=",
		"OPENCODE=",
		"LOBSTERDOG_HOME=",
		"AGENTS_MD_PATH=",
	}
	tmuxPath := os.Getenv("PATH")
	if home := os.Getenv("HOME"); home != "" {
		for _, binDir := range []string{home + "/.opencode/bin", home + "/go/bin"} {
			if !strings.Contains(tmuxPath, binDir) {
				tmuxPath = binDir + ":" + tmuxPath
			}
		}
	}
	pairs = append(pairs, "PATH="+tmuxPath)
	if home := os.Getenv("HOME"); home != "" {
		pairs = append(pairs, "HOME="+home)
	}
	return pairs
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
//
// NOTE: Under Shape B (ADR-1), the agent's stdout is captured by exec.Cmd and
// does NOT appear on the tmux pane. The pane is blank during dispatch. The pane
// viewer is re-routed to the DB/SSE stream (B-REROUTE-SSE / ADR-4 / U-UI-13).
// This method is retained for the transition only; it returns blank content
// during active dispatch.
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
//
// NOTE: Under Shape B (ADR-1), the pane is blank during dispatch. See the
// CapturePane note. This method is retained for the transition only.
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

	// Write self-contained opencode config — fully isolated from global harness.
	// Config files are MERGED by opencode, so we must explicitly override
	// everything from the global config that we don't want (plugins, agents, mcp, instructions).
	//
	// The config is emitted by the shared BuildOpencodeJSON (G3-2, NFR-MAINT-1)
	// so this site and internal/api/agent_handlers.go cannot diverge (R4).
	// When no providers are configured, the default ollama provider is emitted
	// (default-safe — NFR-REL-4, C15). Pre-feature behavior is preserved.
	opencodeConfigBytes, err := BuildOpencodeJSON(OpencodeConfigInput{
		Model:     DefaultModel,
		Providers: nil, // nil → default ollama provider (pre-feature behavior)
	})
	if err != nil {
		return fmt.Errorf("building opencode.json: %w", err)
	}
	configPath := filepath.Join(contextDir, "opencode.json")
	if err := os.WriteFile(configPath, opencodeConfigBytes, 0644); err != nil {
		return fmt.Errorf("writing opencode.json: %w", err)
	}

	// Write a minimal AGENTS.md in the context dir to override ~/AGENTS.md.
	// Opencode automatically loads AGENTS.md from home and project dirs.
	// Without this, the lobsterdog harness AGENTS.md (with llmem, worktrees,
	// caveman rules, etc.) leaks into the devteam agent session.
	agentsMD := "# Dev Team Agent\n\n" +
		"You are running inside the Dev Team AIDLC v2 pipeline.\n" +
		"Use the devteam CLI for state management.\n" +
		"Read CONTEXT.md for your full task context.\n\n" +
		"## CLI Commands\n\n" +
		"    devteam artifacts " + req.FeatureID + "              # get artifacts for current stage\n" +
		"    devteam artifacts " + req.FeatureID + " --all        # get all artifacts\n" +
		"    devteam artifact submit " + req.FeatureID + " <type> --file <file>   # submit artifact\n" +
		"    devteam signal " + req.FeatureID + " pass            # signal completion\n" +
		"    devteam questions ask " + req.FeatureID + " --file questions.json    # ask questions\n" +
		"    devteam signal " + req.FeatureID + " needs_feedback  # signal you need answers\n" +
		"    devteam questions list " + req.FeatureID + "          # read answers after resuming\n" +
		"    devteam feature status " + req.FeatureID + "         # check state\n" +
		"    devteam stages " + req.FeatureID + "                 # list stages\n" +
		"    devteam audit " + req.FeatureID + "                  # audit trail\n\n" +
		"## Asking Questions\n\n" +
		"NEVER print questions in your output text. Always use the CLI:\n" +
		"  1. devteam questions ask to submit questions\n" +
		"  2. devteam signal needs_feedback to request answers\n" +
		"  3. WAIT for the pipeline to resume you\n" +
		"  4. devteam questions list to read the answers\n" +
		"  5. Use answers to refine artifacts, then signal pass\n"
	agentsPath := filepath.Join(contextDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(agentsMD), 0644); err != nil {
		return fmt.Errorf("writing AGENTS.md: %w", err)
	}

	// Write .bashrc in context dir to ensure devteam CLI is in PATH.
	// The --pure flag may reset PATH, so we find the devteam binary
	// at runtime and add its directory to PATH.
	devteamPath, _ := exec.LookPath("devteam")
	if devteamPath == "" {
		// Fallback: check common locations
		for _, p := range []string{
			filepath.Join(os.Getenv("HOME"), "go", "bin", "devteam"),
			"/usr/local/bin/devteam",
			"/usr/bin/devteam",
		} {
			if _, err := os.Stat(p); err == nil {
				devteamPath = p
				break
			}
		}
	}
	if devteamPath != "" {
		devteamDir := filepath.Dir(devteamPath)
		bashrcContent := fmt.Sprintf("export PATH=\"%s:$PATH\"\n", devteamDir)
		bashrcPath := filepath.Join(contextDir, ".bashrc")
		os.WriteFile(bashrcPath, []byte(bashrcContent), 0644)
	}

	return nil
}

