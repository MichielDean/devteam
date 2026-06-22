package pipeline

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/gitops"
	"github.com/MichielDean/devteam/internal/repo"
	"github.com/MichielDean/devteam/internal/role"
	"github.com/MichielDean/devteam/internal/rules"
	"github.com/MichielDean/devteam/internal/spec"
)

type Pipeline struct {
	config        *config.Config
	specProvider  *spec.SpecProvider
	specWriter    *spec.SpecWriter
	ruleLoader    *rules.RuleLoader
	roleLoader    *role.RoleLoader
	dispatcher    *role.Dispatcher
	questionStore feature.QuestionStore
	gitClient     *gitops.GitClient
	repoManager   *repo.Manager
}

// Dispatcher returns the role dispatcher (for tmux session management).
func (p *Pipeline) Dispatcher() *role.Dispatcher {
	return p.dispatcher
}

// EnsureSpecWorktree creates a per-feature git worktree if it doesn't exist yet.
// All agents dispatch with CWD = the worktree dir. Spec artifacts are written
// there and committed on the spec/<feature-id> branch.
func (p *Pipeline) EnsureSpecWorktree(f *feature.Feature) error {
	if f.WorktreeDir != "" {
		if _, err := os.Stat(filepath.Join(f.WorktreeDir, ".git")); err == nil {
			// Worktree exists — make sure spec dir is present
			wtSpecDir := filepath.Join(f.WorktreeDir, "specs", f.ID)
			if _, err := os.Stat(wtSpecDir); err != nil {
				// Copy spec dir from primary checkout
				primarySpecDir := filepath.Join(p.specProvider.BaseDir(), "specs", f.ID)
				if err := copyDir(primarySpecDir, wtSpecDir); err != nil {
					log.Printf("warning: could not copy spec dir to worktree: %v", err)
				}
			}
			return nil // worktree already exists
		}
	}

	worktreeDir := filepath.Join(os.Getenv("HOME"), "worktrees", "devteam-specs", f.ID)
	branchName := "spec/" + f.ID

	// Create the worktree from origin/main
	cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreeDir, "origin/main")
	cmd.Dir = p.specProvider.BaseDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Branch might already exist — try without -b
		cmd2 := exec.Command("git", "worktree", "add", worktreeDir, branchName)
		cmd2.Dir = p.specProvider.BaseDir()
		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			return fmt.Errorf("creating spec worktree: %w: %s (retry: %s)", err, string(out), string(out2))
		}
	}

	// Copy the spec dir from primary checkout to the worktree
	primarySpecDir := filepath.Join(p.specProvider.BaseDir(), "specs", f.ID)
	wtSpecDir := filepath.Join(worktreeDir, "specs", f.ID)
	if _, err := os.Stat(primarySpecDir); err == nil {
		if err := copyDir(primarySpecDir, wtSpecDir); err != nil {
			log.Printf("warning: could not copy spec dir to worktree: %v", err)
		}
	}

	f.WorktreeDir = worktreeDir
	log.Printf("EnsureSpecWorktree: created worktree at %s on branch %s for feature %s", worktreeDir, branchName, f.ID)

	// Save state with worktree dir (to both primary checkout and worktree)
	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return fmt.Errorf("saving feature state with worktree dir: %w", err)
	}

	return nil
}

// copyDir copies a directory recursively.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

// WorktreeDir returns the worktree directory for a feature, or the base dir if no worktree.
func (p *Pipeline) WorktreeDir(f *feature.Feature) string {
	if f.WorktreeDir != "" {
		return f.WorktreeDir
	}
	return p.specProvider.BaseDir()
}

// dispatchWorkingDirForPhase returns the CWD an agent should run in for the
// given phase. All phases run in the spec worktree if available. Impl phases
// (construction, review, testing, delivery) run in the first prepared impl
// repo worktree so the agent's code changes land in the right tree.
func (p *Pipeline) dispatchWorkingDirForPhase(f *feature.Feature, phase feature.Phase) string {
	switch phase {
	case feature.PhaseConstruction, feature.PhaseReview, feature.PhaseTesting, feature.PhaseDelivery:
		if len(f.PreparedRepos) > 0 {
			return f.PreparedRepos[0].Dir
		}
	}
	// Use the spec worktree if available, otherwise the base dir
	return p.WorktreeDir(f)
}

func NewPipeline(cfg *config.Config, specProvider *spec.SpecProvider) *Pipeline {
	baseDir := specProvider.BaseDir()
	return &Pipeline{
		config:        cfg,
		specProvider:  specProvider,
		specWriter:    spec.NewSpecWriter(baseDir),
		ruleLoader:    rules.NewRuleLoaderWithConfig(baseDir, cfg),
		roleLoader:    role.NewRoleLoader(baseDir),
		dispatcher:    role.NewDispatcher(baseDir),
		questionStore: feature.NewFileQuestionStore(baseDir),
		gitClient:     gitops.NewGitClient(baseDir),
		repoManager:   repo.NewManager(baseDir),
	}
}

func NewPipelineWithDispatcher(cfg *config.Config, specProvider *spec.SpecProvider, dispatcher *role.Dispatcher) *Pipeline {
	baseDir := specProvider.BaseDir()
	return &Pipeline{
		config:        cfg,
		specProvider:  specProvider,
		specWriter:    spec.NewSpecWriter(baseDir),
		ruleLoader:    rules.NewRuleLoaderWithConfig(baseDir, cfg),
		roleLoader:    role.NewRoleLoader(baseDir),
		dispatcher:    dispatcher,
		questionStore: feature.NewFileQuestionStore(baseDir),
		gitClient:     gitops.NewGitClient(baseDir),
		repoManager:   repo.NewManager(baseDir),
	}
}

func NewPipelineWithQuestionStore(cfg *config.Config, specProvider *spec.SpecProvider, questionStore feature.QuestionStore) *Pipeline {
	baseDir := specProvider.BaseDir()
	return &Pipeline{
		config:        cfg,
		specProvider:  specProvider,
		specWriter:    spec.NewSpecWriter(baseDir),
		ruleLoader:    rules.NewRuleLoaderWithConfig(baseDir, cfg),
		roleLoader:    role.NewRoleLoader(baseDir),
		dispatcher:    role.NewDispatcher(baseDir),
		questionStore: questionStore,
		gitClient:     gitops.NewGitClient(baseDir),
		repoManager:   repo.NewManager(baseDir),
	}
}

// CreateFeatureBranch creates a feature branch and draft PR for the feature.
func (p *Pipeline) CreateFeatureBranch(f *feature.Feature) (string, error) {
	branchName := "feat/" + f.ID

	if err := p.gitClient.CreateBranch(branchName); err != nil {
		return "", fmt.Errorf("creating feature branch: %w", err)
	}

	if err := p.gitClient.Push(branchName); err != nil {
		return "", fmt.Errorf("pushing feature branch: %w", err)
	}

	prURL, err := p.gitClient.CreatePullRequest(branchName, f.Title, fmt.Sprintf("Feature: %s\n\nSpec: %s\nPriority: %d", f.Title, f.ID, f.Priority))
	if err != nil {
		log.Printf("warning: could not create draft PR (may already exist): %v", err)
		return branchName, nil
	}

	log.Printf("Created draft PR for feature %s: %s", f.ID, prURL)
	return branchName, nil
}

// MarkPRReady converts the draft PR to ready for review.
func (p *Pipeline) MarkPRReady(f *feature.Feature) error {
	branchName := "feat/" + f.ID
	if err := p.gitClient.ReadyPullRequest(branchName); err != nil {
		return fmt.Errorf("marking PR ready: %w", err)
	}
	log.Printf("Marked PR ready for review for feature %s", f.ID)
	return nil
}

// PrepareImplRepos clones every repo declared in the feature's repos.yaml
// into a per-feature worktree (worktrees/<featureID>/<repoName>) and creates
// the feature/<featureID> branch in each. The resulting work dirs are
// persisted on the feature (PreparedRepos) so subsequent impl phases
// (review, testing, delivery) reuse the same clones without re-preparing.
//
// This is the fix for the "changes lost in branch ether" bug: agents were
// dispatched with CWD = the spec repo, so code they wrote landed in the
// spec repo (or nowhere). Now impl phases dispatch with CWD = a prepared
// impl repo worktree, and PushPhaseChanges pushes each repo's feature
// branch to its own origin.
//
// Safe to call multiple times: if PreparedRepos is already populated and
// the directories still exist, it's a no-op. Call this at the start of
// the construction phase (after inception has produced repos.yaml).
func (p *Pipeline) PrepareImplRepos(f *feature.Feature) error {
	if len(f.PreparedRepos) > 0 && p.preparedReposExist(f) {
		log.Printf("PrepareImplRepos: %s already has %d prepared repo(s), reusing", f.ID, len(f.PreparedRepos))
		return nil
	}

	refs, err := p.specProvider.LoadFeatureRepos(f.ID)
	if err != nil {
		return fmt.Errorf("loading repos.yaml for %s: %w", f.ID, err)
	}
	if len(refs) == 0 {
		log.Printf("PrepareImplRepos: %s has no repos.yaml entries — feature touches only the spec repo", f.ID)
		f.PreparedRepos = nil
		return p.specProvider.SaveFeatureState(f)
	}

	workDirs, err := p.repoManager.PrepareRepos(refs, f.ID)
	if err != nil {
		return fmt.Errorf("preparing impl repos for %s: %w", f.ID, err)
	}

	prepared := make([]feature.PreparedRepo, 0, len(workDirs))
	for _, wd := range workDirs {
		prepared = append(prepared, feature.PreparedRepo{
			Name:   wd.Name,
			URL:    wd.URL,
			Dir:    wd.Dir,
			Branch: wd.Branch,
		})
	}
	f.PreparedRepos = prepared
	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return fmt.Errorf("saving prepared repos on feature state: %w", err)
	}
	log.Printf("PrepareImplRepos: prepared %d repo(s) for %s", len(prepared), f.ID)
	return nil
}

// preparedReposExist returns true if every persisted PreparedRepo still has
// a .git directory on disk. Used to decide whether PrepareImplRepos can
// skip re-cloning.
func (p *Pipeline) preparedReposExist(f *feature.Feature) bool {
	for _, pr := range f.PreparedRepos {
		if _, err := os.Stat(filepath.Join(pr.Dir, ".git")); err != nil {
			return false
		}
	}
	return len(f.PreparedRepos) > 0
}

// CleanupImplRepos removes the per-feature worktrees for a feature. Call
// after a feature is merged or cancelled to avoid accumulating clones.
// Errors are logged, not returned — cleanup is best-effort.
func (p *Pipeline) CleanupImplRepos(f *feature.Feature) {
	if err := p.repoManager.RemoveAllWorktreesFor(f.ID); err != nil {
		log.Printf("warning: could not remove impl repo worktrees for %s: %v", f.ID, err)
	}
	f.PreparedRepos = nil
	_ = p.specProvider.SaveFeatureState(f)
}

// dispatchWorkingDirForPhase returns the CWD an agent should run in for the
// given phase. Spec-only phases (inception, planning) run in the spec repo
// so the agent can read/write spec artifacts. Impl phases (construction,
// review, testing, delivery) run in the first prepared impl repo worktree
// so the agent's code changes land in the right tree.
//
// implRepoContext returns a CONTEXT.md fragment describing the prepared
// impl repo worktrees so agents know where to write code and which branch
// they're on. Empty for spec-only phases (inception, planning) — those
// phases don't touch impl repos, so injecting worktree paths would just
// confuse the PM/Architect.
func (p *Pipeline) implRepoContext(f *feature.Feature, phase feature.Phase) string {
	if len(f.PreparedRepos) == 0 {
		return ""
	}
	if !p.isImplPhase(phase) {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n---\n\n# Implementation Repositories\n\n")
	b.WriteString("Code changes for this feature must land in the following repository worktrees. Each is already cloned and on the feature branch — DO NOT re-clone, re-branch, or push to main.\n\n")
	for i, pr := range f.PreparedRepos {
		flag := ""
		if i == 0 && p.isImplPhase(phase) {
			flag = " (PRIMARY — your CWD)"
		}
		b.WriteString(fmt.Sprintf("- **%s**%s\n  - Path: `%s`\n  - Branch: `%s` (do not switch branches)\n  - Origin: %s\n", pr.Name, flag, pr.Dir, pr.Branch, pr.URL))
	}
	b.WriteString("\n## Commit Discipline\n\n")
	b.WriteString("- Commit changes in the worktree(s) above. The pipeline will push `")
	b.WriteString(repo.FeatureBranchName(f.ID))
	b.WriteString("` to each repo's origin after the phase gate passes.\n")
	b.WriteString("- Do NOT push directly. Do NOT open PRs manually. The pipeline handles push + PR.\n")
	b.WriteString("- Do NOT commit to `main`. Only commit on the feature branch checked out in each worktree.\n")
	b.WriteString("- If the feature spans multiple repos, commit to each repo's worktree with a consistent message referencing the feature ID.\n")
	return b.String()
}

func (p *Pipeline) isImplPhase(phase feature.Phase) bool {
	switch phase {
	case feature.PhaseConstruction, feature.PhaseReview, feature.PhaseTesting, feature.PhaseDelivery:
		return true
	}
	return false
}

// PushPhaseChanges commits and pushes all changes for a completed phase.
// For features with prepared impl repos, commits and pushes each repo's
// feature branch to its own origin. For spec-only features (no repos.yaml),
// falls back to committing/pushing the spec repo's feature branch via
// gitClient (legacy behavior).
func (p *Pipeline) PushPhaseChanges(f *feature.Feature, phase feature.Phase) error {
	message := fmt.Sprintf("%s: complete %s phase for %s", phase, f.ID, phase)

	if len(f.PreparedRepos) > 0 {
		workDirs := make([]*repo.RepoWorkDir, 0, len(f.PreparedRepos))
		for _, pr := range f.PreparedRepos {
			workDirs = append(workDirs, &repo.RepoWorkDir{
				Name:   pr.Name,
				URL:    pr.URL,
				Dir:    pr.Dir,
				Branch: pr.Branch,
			})
		}
		if err := p.repoManager.CommitAcrossRepos(workDirs, f.ID); err != nil {
			return fmt.Errorf("committing across impl repos: %w", err)
		}
		if err := p.repoManager.PushAcrossRepos(workDirs, f.ID); err != nil {
			return fmt.Errorf("pushing across impl repos: %w", err)
		}
		// Also commit/push spec repo state (CONTEXT.md, state file, etc).
		branchName := "feat/" + f.ID
		if err := p.gitClient.CommitAndPush(branchName, message); err != nil {
			log.Printf("warning: could not push spec repo state: %v", err)
		}
		return nil
	}

	// Legacy: spec-only feature.
	branchName := "feat/" + f.ID
	return p.gitClient.CommitAndPush(branchName, message)
}

// cleanPhaseArtifacts removes artifacts from the current phase's spec directory
// before running the phase. This ensures the agent starts fresh and doesn't
// skip work because it finds existing artifacts from a previous run.
func (p *Pipeline) cleanPhaseArtifacts(f *feature.Feature, phase feature.Phase) {
	gateDef := feature.GetGateDefinition(phase)
	if gateDef == nil {
		return
	}

	for _, artType := range gateDef.RequiredArts {
		path := p.specProvider.ArtifactPath(f.ID, artType)
		if _, err := os.Stat(path); err == nil {
			if err := os.Remove(path); err != nil {
				log.Printf("warning: could not remove existing artifact %s: %v", path, err)
			} else {
				log.Printf("cleanPhaseArtifacts: removed existing %s for phase %s", artType, phase)
			}
		}
	}
}

func (p *Pipeline) RunPhase(f *feature.Feature) (*feature.PhaseState, error) {
	currentPhase := f.CurrentPhase()
	phaseConfig, err := p.getPhaseConfig(currentPhase)
	if err != nil {
		return nil, err
	}

	roles := phaseConfig.Roles
	if len(roles) == 0 {
		return nil, fmt.Errorf("no roles configured for phase %s", currentPhase)
	}

	now := time.Now()
	ps, ok := f.PhaseStates[currentPhase]
	if !ok {
		ps = &feature.PhaseState{
			Phase: currentPhase,
		}
		f.PhaseStates[currentPhase] = ps
	}
	ps.Status = feature.StatusInProgress
	ps.StartedAt = &now

	return ps, nil
}

type RunResult struct {
	Phase       feature.Phase
	RoleResults []*role.DispatchResult
	GateResult  *feature.GateResult
	Advanced    bool
	Message     string
	Duration    time.Duration
}

func (p *Pipeline) RunPhaseWithAgent(ctx context.Context, f *feature.Feature) (*RunResult, error) {
	currentPhase := f.CurrentPhase()

	// Ensure spec worktree exists before running any phase
	if err := p.EnsureSpecWorktree(f); err != nil {
		log.Printf("warning: could not create spec worktree: %v — using base dir", err)
	}

	phaseConfig, err := p.getPhaseConfig(currentPhase)
	if err != nil {
		return nil, err
	}

	roles := phaseConfig.Roles
	if len(roles) == 0 {
		return nil, fmt.Errorf("no roles configured for phase %s", currentPhase)
	}

	now := time.Now()
	ps, ok := f.PhaseStates[currentPhase]
	if !ok {
		ps = &feature.PhaseState{
			Phase: currentPhase,
		}
		f.PhaseStates[currentPhase] = ps
	}
	ps.Status = feature.StatusInProgress
	ps.StartedAt = &now

	contextStr, err := p.ruleLoader.BuildContext(string(currentPhase), roles[0], f.Priority)
	if err != nil {
		return nil, fmt.Errorf("building context for phase %s role %s: %w", currentPhase, roles[0], err)
	}

	if currentPhase == feature.PhaseInception {
		inputContent, err := p.specProvider.ReadArtifact(f.ID, feature.ArtifactInputMD)
		if err == nil && inputContent != "" {
			contextStr = contextStr + "\n\n---\n\n=== Feature Input ===\n" + inputContent
		}
	}

	// Inject human responses if there are answered/assumed questions
	if p.questionStore != nil {
		questions, qErr := p.questionStore.ListQuestions(ctx, f.ID)
		if qErr == nil && len(questions) > 0 {
			timeoutMinutes := p.config.Pipeline.GetHumanInteractionTimeoutMinutes()
			humanResponses := feature.BuildHumanResponsesContext(questions, timeoutMinutes)
			if humanResponses != "" {
				contextStr = contextStr + humanResponses
			}
		}
	}

	specContext, err := p.specProvider.BuildCrossRepoContext(f.ID, nil)
	if err == nil && specContext != "" {
		contextStr = contextStr + "\n\n---\n\n" + specContext
	}

	// Include gate failure details if present (for recirculation context)
	gateFailurePath := filepath.Join(p.specProvider.FeatureDir(f.ID), "GATE_FAILURE.md")
	if gateFailureContent, err := os.ReadFile(gateFailurePath); err == nil {
		contextStr = contextStr + "\n\n---\n\n# Gate Failure (Previous Attempt)\n\n" + string(gateFailureContent)
	}

	// Clean artifacts from any previous run of this phase so the agent starts fresh
	p.cleanPhaseArtifacts(f, currentPhase)

	var roleResults []*role.DispatchResult
	for _, roleName := range roles {
		roleDef, err := p.roleLoader.Load(roleName)
		if err != nil {
			return nil, fmt.Errorf("loading role %s: %w", roleName, err)
		}

		promptContext := roleDef.Instructions + "\n\n---\n\n" + contextStr

		phaseInstruction := p.phaseInstruction(currentPhase, f.ID)
		if phaseInstruction != "" {
			promptContext = promptContext + "\n\n---\n\n" + phaseInstruction
		}

		// Inject impl repo worktree paths so the agent knows where to write.
		promptContext = promptContext + p.implRepoContext(f, currentPhase)

		contextMD := buildContextMD(f.ID, string(currentPhase), roleName, promptContext)
		contextPath := filepath.Join(p.specProvider.FeatureDir(f.ID), "CONTEXT.md")
		if err := os.WriteFile(contextPath, []byte(contextMD), 0644); err != nil {
			return nil, fmt.Errorf("writing CONTEXT.md: %w", err)
		}

		req := role.DispatchRequest{
			FeatureID:  f.ID,
			Phase:      string(currentPhase),
			Role:       roleName,
			Context:    promptContext,
			WorkingDir: p.dispatchWorkingDirForPhase(f, currentPhase),
		}

		result, err := p.dispatcher.Dispatch(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("dispatching role %s for phase %s: %w", roleName, currentPhase, err)
		}
		roleResults = append(roleResults, result)
	}

	gateResult, err := NewGateEvaluator(p.specProvider).EvaluateForPhase(f, currentPhase)
	if err != nil {
		return nil, fmt.Errorf("evaluating gate for phase %s: %w", currentPhase, err)
	}

	ps.GateResult = gateResult
	if gateResult.Passed {
		ps.Status = feature.StatusPassed
		ps.CompletedAt = &now
	} else {
		ps.Status = feature.StatusGateBlocked
	}

	result := &RunResult{
		Phase:       currentPhase,
		RoleResults: roleResults,
		GateResult:  gateResult,
		Message:     fmt.Sprintf("Phase %s completed. Gate passed: %v", currentPhase, gateResult.Passed),
		Duration:    time.Since(now),
	}

	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}

	// Commit spec artifacts to git after gate passes
	if gateResult.Passed {
		if err := p.commitSpecArtifacts(f, currentPhase); err != nil {
			log.Printf("warning: could not commit spec artifacts for %s phase %s: %v", f.ID, currentPhase, err)
		}
	}

	return result, nil
}

// OutputLineCallback is called for each line of agent output during streaming execution.
type OutputLineCallback func(line string, isStderr bool)

// RunPhaseWithAgentStreaming is the same as RunPhaseWithAgent but streams agent output
// to the callback in real time.
func (p *Pipeline) RunPhaseWithAgentStreaming(ctx context.Context, f *feature.Feature, onOutput OutputLineCallback) (*RunResult, error) {
	currentPhase := f.CurrentPhase()
	log.Printf("RunPhaseWithAgentStreaming: starting for phase %s, feature %s", currentPhase, f.ID)

	// Ensure spec worktree exists before running any phase
	if err := p.EnsureSpecWorktree(f); err != nil {
		log.Printf("warning: could not create spec worktree: %v — using base dir", err)
	}

	phaseConfig, err := p.getPhaseConfig(currentPhase)
	if err != nil {
		return nil, err
	}

	roles := phaseConfig.Roles
	if len(roles) == 0 {
		return nil, fmt.Errorf("no roles configured for phase %s", currentPhase)
	}
	log.Printf("RunPhaseWithAgentStreaming: roles=%v", roles)

	now := time.Now()
	ps, ok := f.PhaseStates[currentPhase]
	if !ok {
		ps = &feature.PhaseState{
			Phase: currentPhase,
		}
		f.PhaseStates[currentPhase] = ps
	}
	ps.Status = feature.StatusInProgress
	ps.StartedAt = &now

	contextStr, err := p.ruleLoader.BuildContext(string(currentPhase), roles[0], f.Priority)
	if err != nil {
		return nil, fmt.Errorf("building context for phase %s role %s: %w", currentPhase, roles[0], err)
	}

	if currentPhase == feature.PhaseInception {
		inputContent, err := p.specProvider.ReadArtifact(f.ID, feature.ArtifactInputMD)
		if err == nil && inputContent != "" {
			contextStr = contextStr + "\n\n---\n\n=== Feature Input ===\n" + inputContent
		}
	}

	if p.questionStore != nil {
		questions, qErr := p.questionStore.ListQuestions(ctx, f.ID)
		if qErr == nil && len(questions) > 0 {
			timeoutMinutes := p.config.Pipeline.GetHumanInteractionTimeoutMinutes()
			humanResponses := feature.BuildHumanResponsesContext(questions, timeoutMinutes)
			if humanResponses != "" {
				contextStr = contextStr + humanResponses
			}
		}
	}

	specContext, err := p.specProvider.BuildCrossRepoContext(f.ID, nil)
	if err == nil && specContext != "" {
		contextStr = contextStr + "\n\n---\n\n" + specContext
	}

	gateFailurePath := filepath.Join(p.specProvider.FeatureDir(f.ID), "GATE_FAILURE.md")
	if gateFailureContent, err := os.ReadFile(gateFailurePath); err == nil {
		contextStr = contextStr + "\n\n---\n\n# Gate Failure (Previous Attempt)\n\n" + string(gateFailureContent)
	}

	var roleResults []*role.DispatchResult
	for _, roleName := range roles {
		roleDef, err := p.roleLoader.Load(roleName)
		if err != nil {
			return nil, fmt.Errorf("loading role %s: %w", roleName, err)
		}

		promptContext := roleDef.Instructions + "\n\n---\n\n" + contextStr

		phaseInstruction := p.phaseInstruction(currentPhase, f.ID)
		if phaseInstruction != "" {
			promptContext = promptContext + "\n\n---\n\n" + phaseInstruction
		}

		// Inject impl repo worktree paths so the agent knows where to write.
		promptContext = promptContext + p.implRepoContext(f, currentPhase)

		contextMD := buildContextMD(f.ID, string(currentPhase), roleName, promptContext)
		contextPath := filepath.Join(p.specProvider.FeatureDir(f.ID), "CONTEXT.md")
		if err := os.WriteFile(contextPath, []byte(contextMD), 0644); err != nil {
			return nil, fmt.Errorf("writing CONTEXT.md: %w", err)
		}

		req := role.DispatchRequest{
			FeatureID:  f.ID,
			Phase:      string(currentPhase),
			Role:       roleName,
			Context:    promptContext,
			WorkingDir: p.dispatchWorkingDirForPhase(f, currentPhase),
		}

		log.Printf("RunPhaseWithAgentStreaming: dispatching role %s for phase %s", roleName, currentPhase)

		lineCh := make(chan role.OutputLine, 100)
		var streamDone chan struct{}
		if onOutput != nil {
			streamDone = make(chan struct{})
			go func() {
				defer close(streamDone)
				for line := range lineCh {
					onOutput(line.Line, line.IsStderr)
				}
			}()
		}

		// Always close lineCh after dispatch returns so the reader goroutine (if any) exits
		result, err := p.dispatcher.DispatchStreaming(ctx, req, lineCh)
		close(lineCh)
		if streamDone != nil {
			<-streamDone
		}
		log.Printf("RunPhaseWithAgentStreaming: dispatch returned, err=%v", err)
		if streamDone != nil {
			<-streamDone
		}
		if err != nil {
			return nil, fmt.Errorf("dispatching role %s for phase %s: %w", roleName, currentPhase, err)
		}
		roleResults = append(roleResults, result)
	}

	gateResult, err := NewGateEvaluator(p.specProvider).EvaluateForPhase(f, currentPhase)
	if err != nil {
		return nil, fmt.Errorf("evaluating gate for phase %s: %w", currentPhase, err)
	}

	ps.GateResult = gateResult
	if gateResult.Passed {
		ps.Status = feature.StatusPassed
		ps.CompletedAt = &now
	} else {
		ps.Status = feature.StatusGateBlocked
	}

	result := &RunResult{
		Phase:       currentPhase,
		RoleResults: roleResults,
		GateResult:  gateResult,
		Message:     fmt.Sprintf("Phase %s completed. Gate passed: %v", currentPhase, gateResult.Passed),
		Duration:    time.Since(now),
	}

	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}

	// Commit spec artifacts to git after gate passes.
	// This ensures specs are tracked and survive branch switches / resets.
	if gateResult.Passed {
		if err := p.commitSpecArtifacts(f, currentPhase); err != nil {
			log.Printf("warning: could not commit spec artifacts for %s phase %s: %v", f.ID, currentPhase, err)
		}
	}

	return result, nil
}

// commitSpecArtifacts commits spec directory changes to git on the current branch.
// Unlike PushPhaseChanges (which creates feature branches and PRs), this just
// commits to whatever branch is currently checked out — usually main for spec-only features.
func (p *Pipeline) commitSpecArtifacts(f *feature.Feature, phase feature.Phase) error {
	// Use a git client that operates in the worktree (if set) or the base dir
	workDir := p.WorktreeDir(f)
	wtGitClient := gitops.NewGitClient(workDir)

	specDir := p.specProvider.FeatureDirFromFeature(f)
	relPath, err := filepath.Rel(workDir, specDir)
	if err != nil {
		relPath = filepath.Join("specs", f.ID)
	}

	// Stage just the spec directory
	if _, err := wtGitClient.Run("add", relPath); err != nil {
		return fmt.Errorf("staging spec dir %s: %w", relPath, err)
	}

	hasChanges, err := wtGitClient.HasStagedChanges()
	if err != nil {
		return err
	}
	if !hasChanges {
		log.Printf("commitSpecArtifacts: no changes to commit for %s phase %s", f.ID, phase)
		return nil
	}

	message := fmt.Sprintf("spec: %s phase complete for %s", phase, f.ID)
	if err := wtGitClient.Commit(message); err != nil {
		return fmt.Errorf("committing spec artifacts: %w", err)
	}

	// Push the spec branch to origin
	branchName := "spec/" + f.ID
	if err := wtGitClient.Push(branchName); err != nil {
		log.Printf("commitSpecArtifacts: warning: could not push spec branch %s: %v", branchName, err)
	}

	log.Printf("commitSpecArtifacts: committed %s phase artifacts for %s on branch %s", phase, f.ID, branchName)
	return nil
}

func (p *Pipeline) AdvanceFeature(f *feature.Feature) (*feature.Feature, error) {
	fromPhase := f.CurrentPhase()
	phases := feature.AllPhases()
	fromIdx := -1
	for i, phase := range phases {
		if phase == fromPhase {
			fromIdx = i
			break
		}
	}
	if fromIdx < 0 {
		return nil, fmt.Errorf("current phase %s not found", fromPhase)
	}
	if fromIdx >= len(phases)-1 {
		return nil, fmt.Errorf("already at final phase %s, use MarkDone to complete", fromPhase)
	}
	nextPhase := phases[fromIdx+1]
	if err := f.AdvanceTo(nextPhase); err != nil {
		return nil, fmt.Errorf("advancing from %s to %s: %w", fromPhase, nextPhase, err)
	}
	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}
	return f, nil
}

func (p *Pipeline) AdvanceFeatureFrom(f *feature.Feature, fromPhase feature.Phase) (*feature.Feature, error) {
	phases := feature.AllPhases()
	fromIdx := -1
	for i, phase := range phases {
		if phase == fromPhase {
			fromIdx = i
			break
		}
	}
	if fromIdx < 0 {
		return nil, fmt.Errorf("phase %s not found", fromPhase)
	}
	if fromIdx >= len(phases)-1 {
		return nil, fmt.Errorf("already at final phase %s, use MarkDone to complete", fromPhase)
	}
	nextPhase := phases[fromIdx+1]
	if err := f.AdvanceTo(nextPhase); err != nil {
		return nil, fmt.Errorf("advancing from %s to %s: %w", fromPhase, nextPhase, err)
	}
	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}
	return f, nil
}

func (p *Pipeline) RecirculateFeature(f *feature.Feature, targetPhase feature.Phase, reason string) (*feature.Feature, error) {
	if err := f.RecirculateTo(targetPhase); err != nil {
		return nil, fmt.Errorf("recirculating from %s to %s: %w", f.CurrentPhase(), targetPhase, err)
	}
	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}
	return f, nil
}

func (p *Pipeline) EvaluateGate(f *feature.Feature) (*feature.GateResult, error) {
	return NewGateEvaluator(p.specProvider).Evaluate(f)
}

func (p *Pipeline) EvaluateGateForPhase(f *feature.Feature, phase feature.Phase) (*feature.GateResult, error) {
	return NewGateEvaluator(p.specProvider).EvaluateForPhase(f, phase)
}

func (p *Pipeline) ListFeatures() ([]*feature.Feature, error) {
	return p.specProvider.ListFeatures()
}

func (p *Pipeline) GetFeature(featureID string) (*feature.Feature, error) {
	return p.specProvider.LoadFeatureState(featureID)
}

func (p *Pipeline) SaveFeature(f *feature.Feature) error {
	return p.specProvider.SaveFeatureState(f)
}

func (p *Pipeline) getPhaseConfig(phase feature.Phase) (*config.PhaseConfig, error) {
	for i := range p.config.Pipeline.Phases {
		if p.config.Pipeline.Phases[i].Name == string(phase) {
			return &p.config.Pipeline.Phases[i], nil
		}
	}
	return nil, fmt.Errorf("phase %s not found in config", phase)
}

func (p *Pipeline) phaseInstruction(phase feature.Phase, featureID string) string {
	switch phase {
	case feature.PhaseInception:
		return fmt.Sprintf(`You are in the INCEPTION phase for feature %s.

Your task: Explore, clarify, and refine the idea into a structured specification.

Follow the Inception Phase Rules for detailed procedures (request type classification, completeness analysis, error scenario tables, empty state behavior, brownfield analysis). The rules are loaded in your context — use them.

You MUST produce the following artifacts in the spec directory:

1. **spec.md** — Write this file at specs/%s/spec.md with:
   - Feature title and description
   - User stories with priority (P1, P2, P3) — each with independent test
   - Functional requirements (FR-NNN format) — each traced to a user story
   - Key entities and relationships (data model overview)
   - State transitions for entities with lifecycle (valid transitions and invalid transitions)
   - Success criteria (SC-NNN format, measurable — "Given X, When Y, Then Z")
   - Error scenarios table: for each user action, what happens on success AND on each error condition (400, 404, 409, 500)
   - Empty state behavior: what the API/UI returns when collections are empty (200 with [], not 404)
   - Assumptions and scope boundaries — flag every assumption with [ASSUMPTION: ...]
   - No [NEEDS CLARIFICATION] markers may remain — resolve them or convert to assumptions

2. **acceptance.md** — Write this file at specs/%s/acceptance.md with:
   - Acceptance criteria traced to each user story (AC-NNN format)
   - Each criterion in format: AC-NNN: Given [precondition], when [action], then [expected result]
     Test level: [smoke | integration | e2e | unit]
     Verification: [specific assertion or scenario]
   - Every user story has at least one criterion per relevant test level
   - Error paths and empty states explicitly covered
   - No "should work well" or "should be fast" — only "Given X, When Y, Then Z"

3. **repos.yaml** — Write this file at specs/%s/repos.yaml with:
   - Feature ID
   - List of affected repositories with name, URL, and branch
   - At minimum, the devteam repo itself

Do NOT write placeholder content. Every section must contain real, specific content derived from the feature input. If information is missing, make reasonable assumptions and flag them with [ASSUMPTION: ...].`, featureID, featureID, featureID, featureID)

	case feature.PhasePlanning:
		return fmt.Sprintf(`You are in the PLANNING phase for feature %s.

Your task: Design the technical approach with enough specificity that the Developer can implement without making architectural decisions on the fly.

Follow the Planning Phase Rules for detailed procedures (component identification, data modeling, API contracts, NFR design, task decomposition). The rules are loaded in your context — use them.

You MUST produce the following artifacts:

1. **plan.md** — Write this file at specs/%s/plan.md with:
   - Summary of what is being built
   - Technical context (language, framework, dependencies)
   - Project structure (where files go)
   - Component design: for each component, its purpose, responsibilities, interfaces, and dependencies
   - Data model: entities, attributes, relationships, state transitions, data integrity rules
   - API contracts: for each endpoint, method, path, request schema, response schema (including error responses)
   - Test strategy per component: what testing levels are required (smoke, integration, e2e, unit)
   - Agent failure mode checks: which checks apply to which tasks
   - NFR considerations: performance, security, scalability, reliability (as applicable)

2. **tasks.md** — Write this file at specs/%s/tasks.md with:
   - Tasks grouped by user story priority (P1 first, then P2, then P3)
   - Each task has: ID (T001, T002...), description with exact file paths, [P] for parallelizable
   - Done conditions: specific verifiable assertions (not "implement the API" but "implement the API and verify: service starts, GET /api/features returns 200, POST with missing title returns 400")
   - Dependencies between tasks explicitly stated
   - Test level required for each task (smoke, integration, e2e, unit)
   - Agent failure mode checks per task

The plan MUST address all acceptance criteria from acceptance.md. Every task must reference specific files.`, featureID, featureID, featureID)

	case feature.PhaseConstruction:
		return fmt.Sprintf(`You are in the CONSTRUCTION phase for feature %s.

Your task: Implement the code according to the plan and tasks, following the Construction Phase Rules for self-verification, brownfield patterns, and agent failure mode checks.

Before writing any code:
1. Read spec.md and acceptance.md — understand what you're building and why
2. Read plan.md — understand the technical approach and test strategy
3. Read tasks.md — understand what to implement and in what order
4. If brownfield: read existing code to understand conventions

Implementation approach:
- Follow the task list in tasks.md, respecting dependency order
- Write the minimum code needed to satisfy each task's done conditions
- If brownfield: modify existing files in-place, follow existing conventions, do NOT create ClassName_modified.go
- Write tests alongside the code, not after

Self-verification before marking any task complete:
- Build succeeds, binary runs without panicking
- Hit each endpoint, verify no nil pointer panics, proper error codes
- Done conditions from tasks.md are verified
- No TODO, FIXME, HACK, or placeholder implementations remain
- JSON arrays are [] not null (marshal zero-value struct to check)
- Error paths work: 400 for invalid input, 404 for missing resources, 409 for conflicts

Agent failure mode checks:
- Nil pointer chains: initialize struct fields in correct order
- Null vs empty arrays: use json:"fieldname" NOT json:"fieldname,omitempty"
- Recovery middleware first: must be outermost middleware
- Error response structure: {"error": "code", "details": "message"}
- No over-engineering: 500 lines is suspicious, 5000 lines is almost certainly wrong
- No phantom methods: every method called must actually exist

After all tasks are complete:
- go build ./... must succeed
- go test ./... must pass
- Service starts and responds without panicking`, featureID)

	case feature.PhaseReview:
		return fmt.Sprintf(`You are in the REVIEW phase for feature %s.

Your task: Perform adversarial review against the spec acceptance criteria. Follow the Review Phase Rules for the structured review process.

Review process:
1. Spec review: Compare plan against spec — does every user story have corresponding tasks?
2. Code review: For each task, verify done conditions with specific evidence
3. Over-engineering check: Is implementation the minimum needed?
4. Missing implementation check: Any spec requirements not implemented?

Write your findings to specs/%s/review-report.md with:
- Per-criterion analysis: every AC-NNN from acceptance.md with MET or NOT MET status
- Quoted evidence: specific code with file path and line number
- Over-engineering findings: line count vs expected
- Missing implementation: user stories with no corresponding code

Format for each criterion:
  AC-NNN: [criterion text]
  Status: MET or NOT MET
  Evidence: [file:line] [quoted code or spec text]
  Explanation: [how the code satisfies or fails the criterion]

Key checks:
- Null pointer safety: every handler dereferencing pointers, every middleware chain
- JSON serialization: every slice/map field returns [] not null
- Error path coverage: 400, 404, 409, empty state, 500 recovery
- Middleware chain: recovery middleware is outermost, CORS is correct
- Security (P1): authentication, authorization, input validation, no secrets in logs

No critical findings may remain unresolved.`, featureID, featureID)

	case feature.PhaseTesting:
		return fmt.Sprintf(`You are in the TESTING phase for feature %s.

Your task: Verify that what was built actually works. Follow the Testing Phase Rules for the structured testing process.

Testing process:
1. Spec-implementation drift: Compare spec against what was built before writing tests
2. Determine testing levels needed (smoke always, integration for API, E2E for UI, unit for logic)
3. Write and execute smoke tests: start service, hit every endpoint, verify no panics
4. Write and execute integration tests: full HTTP request/response cycles
5. Write and execute E2E tests (if UI changed): load in browser, verify no console errors
6. Write and execute unit tests: business logic, state machine transitions, serialization
7. Agent failure mode verification: nil pointers, null arrays, phantom methods, over-engineering

Write your test report to specs/%s/test-report.md with:
- Spec-implementation drift findings
- Smoke test results: which endpoints were hit, what status codes returned
- Integration test results: which request/response cycles were verified
- E2E test results (if applicable): which pages were loaded, any console errors
- Unit test results: which logic was tested in isolation
- Null/empty checks: which fields verified to return [] not null
- State machine transitions: which transitions were verified
- Exact commands to reproduce each test
- Exact assertions verified
- Anti-fake-report: specific evidence, not "all tests pass"

Quality gate:
- Every acceptance criterion has at least one test
- No nil pointer panics, no null-vs-empty-array mismatches
- All smoke and integration tests pass
- ANY failing test is an automatic recirculate`, featureID, featureID)

	case feature.PhaseDelivery:
		return fmt.Sprintf(`You are in the DELIVERY phase for feature %s.

Your task: Ship and document. Follow the Delivery Phase Rules for documentation, release coordination, and deployment verification.

Documentation:
1. API documentation: for every endpoint in the plan, document method, path, request/response schemas, error responses
2. User-facing documentation: for every user story in the spec, document using spec terminology
3. Changelog: reference the spec number in every entry

Cross-repo release:
- If the feature spans repos, document release order (shared libraries first, consumers second, frontend last)
- Tag all repos with consistent version references

Deployment verification (ALL must pass before marking delivery complete):
- Build the binary: go build -o ~/go/bin/devteam ./cmd/devteam/
- Start the service: verify it starts without panicking
- Hit the endpoints: verify the API responds correctly
- Load the UI: verify the frontend renders without console errors
- Run the test suite: verify all tests pass

Write documentation to specs/%s/docs/ with:
- API documentation per endpoint (method, path, request, response, errors)
- User-facing documentation using spec terminology
- Changelog referencing the spec number
- Cross-repo release order (if applicable)
- Configuration documentation (env vars, config files, dependencies)

Terminology consistency check: documentation must use the same terms as spec.md, not code-internal names.

Pull request:
- Commit all changes with a descriptive message referencing the spec
- Push to the feature branch (feat/%s)
- The pipeline will create a draft PR automatically and mark it ready when delivery completes`, featureID, featureID, featureID)

	default:
		return ""
	}
}

func buildContextMD(featureID, phase, role, promptContext string) string {
	var b strings.Builder
	b.WriteString("# Dev Team Context\n\n")
	b.WriteString(fmt.Sprintf("Feature: %s\n", featureID))
	b.WriteString(fmt.Sprintf("Phase: %s\n", phase))
	b.WriteString(fmt.Sprintf("Role: %s\n\n", role))
	b.WriteString("---\n\n")
	b.WriteString(promptContext)
	return b.String()
}
