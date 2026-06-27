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

	"gopkg.in/yaml.v3"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
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
	database      *db.DB
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

	// Save state to the worktree (agents never touch primary checkout)
	p.syncFeatureToDB(f)
	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return fmt.Errorf("saving feature state with worktree dir: %w", err)
	}

	// Also save to primary checkout temporarily so ListFeatures can find it
	// before the worktree scan picks it up
	primaryStateDir := filepath.Join(p.specProvider.BaseDir(), "specs", f.ID)
	os.MkdirAll(primaryStateDir, 0755)
	primaryData, _ := yaml.Marshal(f)
	os.WriteFile(filepath.Join(primaryStateDir, ".devteam-state.yaml"), primaryData, 0644)

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

// recordGitCommit returns the current HEAD commit hash of the worktree.
// Used before agent dispatch so the gate can diff only the agent's changes.
func (p *Pipeline) recordGitCommit(f *feature.Feature) string {
	workDir := p.WorktreeDir(f)
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		log.Printf("recordGitCommit: could not get HEAD: %v", err)
		return ""
	}
	commit := strings.TrimSpace(string(output))
	log.Printf("recordGitCommit: HEAD at %s before agent dispatch", commit[:8])
	return commit
}

// writeGateFailure writes GATE_FAILURE.md when a gate fails, so the next
// agent run can read it and understand what went wrong.
func (p *Pipeline) writeGateFailure(f *feature.Feature, phase feature.Phase, gateResult *feature.GateResult) error {
	if gateResult == nil || gateResult.Passed {
		return nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Gate Failure: %s Phase\n\n", phase))
	b.WriteString(fmt.Sprintf("Feature: %s\n\n", f.ID))
	b.WriteString("## Failed Checks\n\n")

	for _, check := range gateResult.Checks {
		if !check.Passed {
			b.WriteString(fmt.Sprintf("- **FAIL**: %s\n", check.Name))
			if check.Message != "" {
				b.WriteString(fmt.Sprintf("  %s\n", check.Message))
			}
			b.WriteString("\n")
		}
	}

	if len(gateResult.MissingArts) > 0 {
		b.WriteString("## Missing Artifacts\n\n")
		for _, art := range gateResult.MissingArts {
			b.WriteString(fmt.Sprintf("- %s\n", art))
		}
	}

	b.WriteString("\n## Instructions for Re-run\n\n")
	b.WriteString("The previous run of this phase failed the quality gate. Fix the issues above.\n")
	b.WriteString("Do NOT just re-create the same artifacts — address the specific failures.\n")

	content := b.String()
	path := filepath.Join(p.specProvider.FeatureDirFromFeature(f), "GATE_FAILURE.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing GATE_FAILURE.md: %w", err)
	}

	log.Printf("writeGateFailure: wrote GATE_FAILURE.md for %s phase %s (%d failed checks)", f.ID, phase, countFailedChecks(gateResult))
	return nil
}

// writePhaseNote writes a summary note to NOTES.md after a phase passes.
// This is the Cistern notes pattern — each phase leaves a brief for the next.
func (p *Pipeline) writePhaseNote(f *feature.Feature, phase feature.Phase, gateResult *feature.GateResult) {
	notesPath := filepath.Join(p.specProvider.FeatureDirFromFeature(f), "NOTES.md")

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n## [%s] %s — Complete\n", time.Now().Format(time.RFC3339), phase))
	b.WriteString(fmt.Sprintf("**Gate**: Passed (%d/%d checks)\n", countPassedChecks(gateResult), len(gateResult.Checks)))

	// Summarize what was produced
	switch phase {
	case feature.PhaseInception:
		b.WriteString("**Artifacts**: spec.md, acceptance.md, repos.yaml\n")
		b.WriteString("**Key decisions**: See spec.md for user stories, requirements, and assumptions.\n")
	case feature.PhasePlanning:
		b.WriteString("**Artifacts**: plan.md, research.md, data-model.md, contracts/, tasks.md\n")
		b.WriteString("**Key decisions**: See plan.md for technical approach. Tasks.md has implementation order.\n")
	case feature.PhaseConstruction:
		b.WriteString("**Artifacts**: Implementation code\n")
		b.WriteString("**Note**: Verify the implementation matches tasks.md done conditions.\n")
	case feature.PhaseReview:
		b.WriteString("**Artifacts**: review-report.md\n")
		b.WriteString("**Note**: Review findings are in review-report.md. Address any NOT MET criteria.\n")
	case feature.PhaseTesting:
		b.WriteString("**Artifacts**: test-report.md, test files\n")
		b.WriteString("**Note**: Test results are in test-report.md. All tests must pass.\n")
	case feature.PhaseDelivery:
		b.WriteString("**Artifacts**: docs/\n")
		b.WriteString("**Note**: Documentation complete. Feature is done.\n")
	}

	// Append to NOTES.md
	file, err := os.OpenFile(notesPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("warning: could not write phase note: %v", err)
		return
	}
	defer file.Close()
	file.WriteString(b.String())

	// Also write any failed checks as warnings for the next phase
	for _, check := range gateResult.Checks {
		if !check.Passed {
			fmt.Fprintf(file, "\n**WARNING**: %s — %s\n", check.Name, check.Message)
		}
	}
}

func countPassedChecks(gr *feature.GateResult) int {
	count := 0
	for _, c := range gr.Checks {
		if c.Passed {
			count++
		}
	}
	return count
}

func countFailedChecks(gr *feature.GateResult) int {
	count := 0
	for _, c := range gr.Checks {
		if !c.Passed {
			count++
		}
	}
	return count
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

// SetDatabase wires the SQLite database into the pipeline for notes, events, and recirculation tracking.
func (p *Pipeline) SetDatabase(database *db.DB) {
	p.database = database
}

// syncFeatureToDB inserts/updates the feature in SQLite so foreign key constraints work.
func (p *Pipeline) syncFeatureToDB(f *feature.Feature) {
	if p.database == nil {
		return
	}
	p.database.Exec(`INSERT OR REPLACE INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, worktree_dir, created_at, updated_at, recirculation_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.Title, string(f.Current), string(f.Status), f.Priority, string(f.IntakePath), f.SpecDir, f.WorktreeDir, f.CreatedAt, f.UpdatedAt, 0)
}

// SetQuestionStore overrides the default question store (e.g., with DBQuestionStore).
func (p *Pipeline) SetQuestionStore(qs feature.QuestionStore) {
	p.questionStore = qs
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
	p.syncFeatureToDB(f)
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
	p.syncFeatureToDB(f)
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

// cleanPhaseArtifacts removes artifacts for the current phase from the DB
// before running the phase. This ensures the agent starts fresh and doesn't
// skip work because it finds existing artifacts from a previous run.
func (p *Pipeline) cleanPhaseArtifacts(f *feature.Feature, phase feature.Phase) {
	gateDef := feature.GetGateDefinition(phase)
	if gateDef == nil {
		return
	}
	if p.database == nil {
		return
	}
	for _, artType := range gateDef.RequiredArts {
		if err := p.database.DeleteArtifact(f.ID, string(artType)); err != nil {
			log.Printf("warning: could not delete artifact %s: %v", artType, err)
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

	// Include revision notes if present (Cistern recirculation pattern)
	revisionNotesPath := filepath.Join(p.specProvider.FeatureDirFromFeature(f), "REVISION_NOTES.md")
	if revisionContent, err := os.ReadFile(revisionNotesPath); err == nil {
		contextStr = contextStr + "\n\n---\n\n# ⚠️ REVISION REQUIRED\n\n" + string(revisionContent)
	}

	// Include gate failure details if present (fallback safety check)
	gateFailurePath := filepath.Join(p.specProvider.FeatureDir(f.ID), "GATE_FAILURE.md")
	if gateFailureContent, err := os.ReadFile(gateFailurePath); err == nil {
		contextStr = contextStr + "\n\n---\n\n# Gate Failure (Previous Attempt)\n\n" + string(gateFailureContent)
	}

	// Include phase notes from SQLite (Cistern pattern)
	if p.database != nil {
		notesContext := p.database.BuildNotesContext(f.ID, string(currentPhase))
		if notesContext != "" {
			contextStr = contextStr + notesContext
		}
	}

	// Clean up revision notes after they've been included in context
	revisionNotesPath2 := filepath.Join(p.specProvider.FeatureDirFromFeature(f), "REVISION_NOTES.md")
	defer os.Remove(revisionNotesPath2)

	// Clean artifacts from any previous run of this phase so the agent starts fresh
	p.cleanPhaseArtifacts(f, currentPhase)

	// Record git commit before dispatch so gate can diff only agent's changes
	preDispatchCommit := p.recordGitCommit(f)

	var roleResults []*role.DispatchResult
	for _, roleName := range roles {
		roleDef, err := p.roleLoader.Load(roleName)
		if err != nil {
			return nil, fmt.Errorf("loading role %s: %w", roleName, err)
		}

		promptContext := roleDef.Instructions + "\n\n---\n\n" + contextStr

		phaseInstruction := p.phaseInstruction(currentPhase, f)
		if phaseInstruction != "" {
			promptContext = promptContext + "\n\n---\n\n" + phaseInstruction
		}
		// Add outcome instructions (Cistern pattern — agent signals pass/recirculate)
		promptContext = promptContext + outcomeInstructions(currentPhase)

		// Inject impl repo worktree paths so the agent knows where to write.
		promptContext = promptContext + p.implRepoContext(f, currentPhase)

		contextMD := buildContextMD(f.ID, string(currentPhase), roleName, promptContext)
		contextDir := p.specProvider.FeatureDir(f.ID)
		os.MkdirAll(contextDir, 0755)
		contextPath := filepath.Join(contextDir, "CONTEXT.md")
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

		// Resolve per-role provider config (CON-002, CON-006). Fail fast
		// on missing api_key_env before spawning opencode (CON-005).
		resolved, rerr := p.config.ResolveProvider(roleName)
		if rerr != nil {
			result := &role.DispatchResult{
				FeatureID: f.ID,
				Phase:     string(currentPhase),
				Role:      roleName,
				Success:   false,
				Error:     rerr.Error(),
			}
			roleResults = append(roleResults, result)
			continue
		}
		req.Provider = resolved

		result, err := p.dispatcher.Dispatch(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("dispatching role %s for phase %s: %w", roleName, currentPhase, err)
		}
		roleResults = append(roleResults, result)
	}

	gateResult, err := NewGateEvaluatorWithCommit(p.specProvider, p.WorktreeDir(f), preDispatchCommit).EvaluateForPhase(f, currentPhase)
	if err != nil {
		return nil, fmt.Errorf("evaluating gate for phase %s: %w", currentPhase, err)
	}

	ps.GateResult = gateResult
	if ps.GateResult.Passed {
		ps.Status = feature.StatusPassed
		ps.CompletedAt = &now
		// Remove GATE_FAILURE.md on success so it doesn't confuse future phases
		gateFailurePath := filepath.Join(p.specProvider.FeatureDirFromFeature(f), "GATE_FAILURE.md")
		os.Remove(gateFailurePath)
	} else {
		ps.Status = feature.StatusGateBlocked
		// Write GATE_FAILURE.md so the next run knows what failed
		if err := p.writeGateFailure(f, currentPhase, gateResult); err != nil {
			log.Printf("warning: could not write GATE_FAILURE.md: %v", err)
		}
	}

	result := &RunResult{
		Phase:       currentPhase,
		RoleResults: roleResults,
		GateResult:  gateResult,
		Message:     fmt.Sprintf("Phase %s completed. Gate passed: %v", currentPhase, gateResult.Passed),
		Duration:    time.Since(now),
	}

	p.syncFeatureToDB(f)
	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}

	// Commit spec artifacts to git after gate passes
	if ps.GateResult.Passed {
		if err := p.commitSpecArtifacts(f, currentPhase); err != nil {
			log.Printf("warning: could not commit spec artifacts for %s phase %s: %v", f.ID, currentPhase, err)
		}
		// Write a phase note for subsequent phases (Cistern pattern)
		p.writePhaseNote(f, currentPhase, gateResult)
	}

	return result, nil
}

// OutputLineCallback is called for each line of agent output during streaming execution.
type OutputLineCallback func(line string, isStderr bool)

// RunPhaseWithAgentStreaming is the same as RunPhaseWithAgent but streams agent output
// to the callback in real time.
func (p *Pipeline) RunPhaseWithAgentStreaming(ctx context.Context, f *feature.Feature, onOutput OutputLineCallback, autoAdvance bool) (*RunResult, error) {
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

	// Clean artifacts and record commit before dispatch
	p.cleanPhaseArtifacts(f, currentPhase)
	preDispatchCommit := p.recordGitCommit(f)

	var roleResults []*role.DispatchResult
	for _, roleName := range roles {
		roleDef, err := p.roleLoader.Load(roleName)
		if err != nil {
			return nil, fmt.Errorf("loading role %s: %w", roleName, err)
		}

		promptContext := roleDef.Instructions + "\n\n---\n\n" + contextStr

		phaseInstruction := p.phaseInstruction(currentPhase, f)
		if phaseInstruction != "" {
			promptContext = promptContext + "\n\n---\n\n" + phaseInstruction
		}
		// Add outcome instructions (Cistern pattern — agent signals pass/recirculate)
		promptContext = promptContext + outcomeInstructions(currentPhase)

		// Inject impl repo worktree paths so the agent knows where to write.
		promptContext = promptContext + p.implRepoContext(f, currentPhase)

		contextMD := buildContextMD(f.ID, string(currentPhase), roleName, promptContext)
		contextDir := p.specProvider.FeatureDir(f.ID)
		os.MkdirAll(contextDir, 0755)
		contextPath := filepath.Join(contextDir, "CONTEXT.md")
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

		// Resolve per-role provider config (CON-002, CON-006). Fail fast
		// on missing api_key_env before spawning opencode (CON-005).
		resolved, rerr := p.config.ResolveProvider(roleName)
		if rerr != nil {
			roleResults = append(roleResults, &role.DispatchResult{
				FeatureID: f.ID,
				Phase:     string(currentPhase),
				Role:      roleName,
				Success:   false,
				Error:     rerr.Error(),
			})
			continue
		}
		req.Provider = resolved

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

	// Read the agent's outcome (Cistern pattern — agent is the evaluator)
	outcome := p.ParseOutcome(f, currentPhase)
	p.DeleteOutcome(f)

	log.Printf("RunPhaseWithAgentStreaming: agent outcome for %s: %s target=%s notes=%d chars",
		currentPhase, outcome.Result, outcome.Target, len(outcome.Notes))

	// If agent didn't write outcome file, run gate as safety check
	if !outcome.HasFile {
		log.Printf("RunPhaseWithAgentStreaming: no outcome file — running gate as safety check")
		gateResult, err := NewGateEvaluatorWithCommit(p.specProvider, p.WorktreeDir(f), preDispatchCommit).EvaluateForPhase(f, currentPhase)
		if err != nil {
			return nil, fmt.Errorf("evaluating gate for phase %s: %w", currentPhase, err)
		}
		if ps.GateResult.Passed {
			outcome.Result = OutcomePass
		} else {
			outcome.Result = OutcomeRecirculate
			outcome.Notes = formatGateFailureAsNotes(gateResult)
			outcome.Target = string(ResolveRecirculateTarget(currentPhase, ""))
		}
		ps.GateResult = gateResult
	} else {
		ps.GateResult = &feature.GateResult{
			Phase:       currentPhase,
			Passed:      outcome.Result == OutcomePass,
			EvaluatedAt: time.Now(),
		}
	}

	// Write notes to SQLite
	if outcome.Notes != "" && p.database != nil {
		noteType := "summary"
		if outcome.Result == OutcomeRecirculate {
			noteType = "revision"
		}
		p.database.AddNote(f.ID, string(currentPhase), string(p.PrimaryRole(currentPhase)), noteType, outcome.Notes)
	}

	// Record event
	if p.database != nil {
		eventType := db.EventPhaseComplete
		if outcome.Result == OutcomeRecirculate {
			eventType = db.EventRecirculate
		}
		p.database.RecordEvent(f.ID, eventType, string(currentPhase), outcome.Notes)
	}

	if outcome.Result == OutcomePass {
		ps.Status = feature.StatusPassed
		ps.CompletedAt = &now
		gateFailurePath := filepath.Join(p.specProvider.FeatureDirFromFeature(f), "GATE_FAILURE.md")
		os.Remove(gateFailurePath)
	} else if outcome.Result == OutcomeRecirculate {
		ps.Status = feature.StatusGateBlocked
		target := ResolveRecirculateTarget(currentPhase, outcome.Target)
		p.writeRecirculationNotes(f, currentPhase, target, outcome.Notes)
		if p.database != nil {
			p.database.AddRecirculation(f.ID, string(currentPhase), string(target), "agent_recirculate", outcome.Notes)
		}
	} else if outcome.Result == OutcomeNeedsFeedback {
		// Agent wrote questions.json and needs user feedback — NOT a failure
		ps.Status = feature.StatusInProgress
	} else {
		// OutcomeFailed or unknown
		ps.Status = feature.StatusGateBlocked
	}

	result := &RunResult{
		Phase:       currentPhase,
		RoleResults: roleResults,
		GateResult:  ps.GateResult,
		Message:     fmt.Sprintf("Phase %s outcome: %s", currentPhase, outcome.Result),
		Duration:    time.Since(now),
	}

	p.syncFeatureToDB(f)
	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}

	// Commit spec artifacts to git after gate passes.
	// This ensures specs are tracked and survive branch switches / resets.
	if ps.GateResult.Passed {
		if err := p.commitSpecArtifacts(f, currentPhase); err != nil {
			log.Printf("warning: could not commit spec artifacts for %s phase %s: %v", f.ID, currentPhase, err)
		}
	}

	// When delivery gate passes, mark feature done and create a pull request.
	if ps.GateResult.Passed && currentPhase == feature.PhaseDelivery {
		f.MarkDone()
		p.syncFeatureToDB(f)
	if err := p.specProvider.SaveFeatureState(f); err != nil {
			log.Printf("warning: could not save feature state after MarkDone: %v", err)
		}
		if err := p.createPullRequest(f); err != nil {
			log.Printf("warning: could not create pull request for feature %s: %v", f.ID, err)
		}
	}

	// Check for questions after inception/planning phases.
	// Go code checks for questions.json REGARDLESS of gate outcome or agent outcome.
	// The agent may write questions.json without writing outcome.txt — that's fine.
	// Go code is the state machine: it reads the file, stores questions, pauses.
	if currentPhase == feature.PhaseInception || currentPhase == feature.PhasePlanning {
		if p.questionStore != nil {
			// Only check for questions if we haven't already asked questions for this phase
			existingQuestions, _ := p.questionStore.ListQuestions(ctx, f.ID)
			alreadyAskedForPhase := false
			for _, q := range existingQuestions {
				if string(q.Phase) == string(currentPhase) {
					alreadyAskedForPhase = true
					break
				}
			}

			if !alreadyAskedForPhase {
				// Read questions from the worktree (agent's CWD)
				specDir := p.specProvider.FeatureDirFromFeature(f)
				detectedQuestions := feature.DetectQuestions(f.ID, specDir)
				if len(detectedQuestions) > 0 {
					log.Printf("RunPhaseWithAgentStreaming: detected %d questions for feature %s after %s phase", len(detectedQuestions), f.ID, currentPhase)
					for i := range detectedQuestions {
						detectedQuestions[i].FeatureID = f.ID
						if _, err := p.questionStore.CreateQuestion(ctx, f.ID, detectedQuestions[i]); err != nil {
							log.Printf("warning: failed to create question for feature %s: %v", f.ID, err)
							continue
						}
					}
					// Delete questions.json so it's not re-detected on a future re-run
					os.Remove(filepath.Join(specDir, "questions.json"))

					// Pause for human input
					if err := f.WaitForHuman(); err != nil {
						log.Printf("warning: cannot transition feature %s to waiting_for_human: %v", f.ID, err)
					} else {
						p.syncFeatureToDB(f)
	if err := p.specProvider.SaveFeatureState(f); err != nil {
							log.Printf("warning: failed to save feature state for %s: %v", f.ID, err)
						}
						log.Printf("RunPhaseWithAgentStreaming: feature %s paused for human input (%d questions)", f.ID, len(detectedQuestions))
					}
				}
			}
		}
	}

	// Outcome-based routing (Cistern pattern):
	// - pass + autoAdvance → advance to next phase and run it
	// - recirculate + autoAdvance → route to target phase and run it
	// - needs_feedback → stop, let user answer questions (NOT a failure)
	// - failed → stop, notify user
	// - pass + !autoAdvance → stop, let user advance manually
	// - waiting_for_feedback → stop, let user answer questions
	if autoAdvance && f.Status != feature.StatusWaitingFeedback {
		if outcome.Result == OutcomeNeedsFeedback {
			// Don't auto-advance — the question detection block below will handle pausing
		} else if outcome.Result == OutcomePass && currentPhase != feature.PhaseDelivery {
			// Advance to next phase
			nextPhase := feature.NextPhase(currentPhase)
			if nextPhase != "" {
				log.Printf("RunPhaseWithAgentStreaming: auto-advancing %s from %s to %s", f.ID, currentPhase, nextPhase)
				advanced, err := p.AdvanceFeature(f)
				if err != nil {
					log.Printf("warning: could not auto-advance %s: %v", f.ID, err)
				} else {
					f = advanced
					p.specProvider.SaveFeatureState(f)
					log.Printf("RunPhaseWithAgentStreaming: auto-running next phase %s for %s", nextPhase, f.ID)
					nextResult, err := p.RunPhaseWithAgentStreaming(ctx, f, onOutput, true)
					if err != nil {
						log.Printf("warning: auto-advanced phase %s failed: %v", nextPhase, err)
					} else if nextResult != nil {
						result.GateResult = nextResult.GateResult
					}
				}
			}
		} else if outcome.Result == OutcomeRecirculate {
			// Recirculate to target phase (Cistern pattern — send back with notes)
			target := ResolveRecirculateTarget(currentPhase, outcome.Target)
			log.Printf("RunPhaseWithAgentStreaming: recirculating %s from %s to %s", f.ID, currentPhase, target)
			// Move feature back to the target phase
			f.Current = target
			f.PhaseStates[target].Status = feature.StatusDraft
			p.specProvider.SaveFeatureState(f)
			// Run the target phase with the revision notes in context
			log.Printf("RunPhaseWithAgentStreaming: running recirculated phase %s for %s", target, f.ID)
			nextResult, err := p.RunPhaseWithAgentStreaming(ctx, f, onOutput, true)
			if err != nil {
				log.Printf("warning: recirculated phase %s failed: %v", target, err)
			} else if nextResult != nil {
				result.GateResult = nextResult.GateResult
			}
		}
	}

	return result, nil
}
// commitSpecArtifacts is a no-op now that artifacts live in the DB.
// Previously committed spec files to a git branch; with DB storage there's
// nothing on disk to commit. Kept as no-op to avoid changing call sites.
func (p *Pipeline) commitSpecArtifacts(f *feature.Feature, phase feature.Phase) error {
	return nil
}

// createPullRequest opens a GitHub PR from the spec branch to main.
// Called when the delivery gate passes and the feature is marked done.
func (p *Pipeline) createPullRequest(f *feature.Feature) error {
	branchName := "spec/" + f.ID

	// Build PR body from spec artifacts
	body := fmt.Sprintf("## Summary\n\nFeature: %s\n\n", f.Title)
	body += fmt.Sprintf("Pipeline complete — all phases passed (inception → planning → construction → review → testing → delivery).\n\n")
	body += fmt.Sprintf("Spec branch: `%s`\n", branchName)

	// Read spec.md for the description
	specPath := p.specProvider.FeatureDirFromFeature(f) + "/spec.md"
	if specContent, err := os.ReadFile(specPath); err == nil {
		// Extract first few lines as summary
		lines := strings.Split(string(specContent), "\n")
		for i, line := range lines {
			if i > 10 {
				break
			}
			if strings.TrimSpace(line) != "" {
				body += "\n" + line
			}
		}
	}

	// Use gh CLI to create the PR
	cmd := exec.Command("gh", "pr", "create",
		"--title", f.Title,
		"--body", body,
		"--base", "main",
		"--head", branchName,
	)
	cmd.Dir = p.WorktreeDir(f)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("creating PR: %w: %s", err, string(out))
	}

	prURL := strings.TrimSpace(string(out))
	log.Printf("createPullRequest: created PR for feature %s: %s", f.ID, prURL)
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
	p.syncFeatureToDB(f)
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
	p.syncFeatureToDB(f)
	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}
	return f, nil
}

func (p *Pipeline) RecirculateFeature(f *feature.Feature, targetPhase feature.Phase, reason string) (*feature.Feature, error) {
	if err := f.RecirculateTo(targetPhase); err != nil {
		return nil, fmt.Errorf("recirculating from %s to %s: %w", f.CurrentPhase(), targetPhase, err)
	}
	p.syncFeatureToDB(f)
	if err := p.specProvider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}
	return f, nil
}

func (p *Pipeline) EvaluateGate(f *feature.Feature) (*feature.GateResult, error) {
	return NewGateEvaluatorWithWorkDir(p.specProvider, p.WorktreeDir(f)).Evaluate(f)
}

func (p *Pipeline) EvaluateGateForPhase(f *feature.Feature, phase feature.Phase) (*feature.GateResult, error) {
	return NewGateEvaluatorWithWorkDir(p.specProvider, p.WorktreeDir(f)).EvaluateForPhase(f, phase)
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

// UpdateFeatureStatus saves a feature's status and current phase.
func (p *Pipeline) UpdateFeatureStatus(f *feature.Feature) error {
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

func (p *Pipeline) phaseInstruction(phase feature.Phase, f *feature.Feature) string {
	featureID := f.ID
	prefix := fmt.Sprintf("## IMPORTANT: Submit Artifacts via CLI\n\nSpec artifacts (spec.md, plan.md, tasks.md, etc.) are stored in the database, NOT on disk.\nSubmit them using the devteam CLI:\n\n  devteam artifact submit %s <type> --file <filename>\n  devteam artifact submit %s <type> --content \"inline content\"\n\nArtifact types: spec, acceptance, repos, plan, tasks, research, data_model, review_report, test_report, docs, contracts\n\nDo NOT write spec artifacts to disk. Use the CLI.\n", featureID, featureID)
	
	switch phase {
	case feature.PhaseInception:
		return prefix + fmt.Sprintf(`You are in the INCEPTION phase for feature %s.

Your task: Gather requirements through interactive questions, then generate the spec using SpecKit.

## Step 1: Ask Clarifying Questions (AIDLC pattern)

If this is a loose idea (not an external spec), write a questions.json file with 3-8 clarifying questions:
[
  {"phase":"inception","role":"pm","question":"Your question here","type":"multiple_choice","options":["Option A","Option B","Other"]},
]
Every question MUST include "Other" as the last option.

Then submit the questions using the devteam CLI:
  devteam questions ask %s --file questions.json

The pipeline will pause and show these questions to the user. Their answers will be provided to you on the next run.
If you can resolve something by reading existing code, do that instead of asking.

After submitting questions, signal that you need feedback:
  devteam signal %s needs_feedback

When you receive answers, check if you need MORE questions. If so, repeat. If you have enough clarity, proceed to Step 2.

## Step 2: Generate the Spec

When you have enough clarity, use the SpecKit spec template at .specify/templates/spec-template.md to write:
- spec.md — user stories with priorities, acceptance scenarios, functional requirements, success criteria, assumptions
- acceptance.md — acceptance criteria in Given/When/Then format with test levels
- repos.yaml — affected repositories

Submit each artifact via CLI:
  devteam artifact submit %s spec --file spec.md
  devteam artifact submit %s acceptance --file acceptance.md
  devteam artifact submit %s repos --file repos.yaml

If a constitution.md exists, verify compliance.

When the spec is complete, signal pass:
  devteam signal %s pass

Inception should almost never fail — it's just a question-answer loop that ends with a spec.`, featureID, featureID, featureID, featureID, featureID, featureID, featureID)

	case feature.PhasePlanning:
		return prefix + fmt.Sprintf(`You are in the PLANNING phase for feature %s.

Your task: Generate the implementation plan and task list using SpecKit templates.

## Step 1: Ask Clarifying Questions (optional)

If the spec leaves architectural decisions open, write a questions.json file:
[
  {"phase":"planning","role":"architect","question":"...","type":"multiple_choice","options":["A","B","Other"]},
]
Submit via: devteam questions ask %s --file questions.json
Signal: devteam signal %s needs_feedback
If the spec is clear, skip this step.

## Step 2: Generate the Plan

Use the SpecKit plan template at .specify/templates/plan-template.md to write:
- plan.md — technical context, project structure, component design, API contracts, test strategy
- research.md — existing code patterns, library choices, alternatives considered
- data-model.md — entity definitions, attributes, relationships, validation
- contracts/ — one file per API endpoint with request/response schemas

Submit each artifact via CLI:
  devteam artifact submit %s plan --file plan.md
  devteam artifact submit %s research --file research.md
  devteam artifact submit %s data_model --file data-model.md
  devteam artifact submit %s contracts --file contracts/index.md

If a constitution.md exists, perform a constitution check.

## Step 3: Generate the Task List

Use the SpecKit tasks template at .specify/templates/tasks-template.md to write:
- tasks.md — tasks grouped by user story priority, each with file paths, done conditions, dependencies, test levels

Submit via CLI:
  devteam artifact submit %s tasks --file tasks.md

The plan MUST address all acceptance criteria from acceptance.md. Every task must reference specific files.

When done, signal pass: devteam signal %s pass`, featureID, featureID, featureID, featureID, featureID, featureID, featureID, featureID, featureID)

	case feature.PhaseConstruction:
		return fmt.Sprintf(`You are in the CONSTRUCTION phase for feature %s.

Your task: Build the spec. Read the spec, plan, and tasks. Write the code. Commit and push.

1. Read spec.md, acceptance.md, plan.md, tasks.md, data-model.md, contracts/ — understand what to build
2. Read existing code to understand conventions
3. Write the code — implement every task in tasks.md
4. Verify the build succeeds (discover and run the project's build command)
5. Commit all changes: git add -A && git commit -m "feat: implement %s"
6. Push to the current branch: git push origin HEAD
7. Signal pass: devteam signal %s pass

That's it. Build to spec. Commit. Push. Signal.

DO NOT write tests, review code, or write documentation — other phases handle those.`, featureID, featureID, featureID)

	case feature.PhaseReview:
		return fmt.Sprintf(`You are in the REVIEW phase for feature %s.

Your task: Read the code and verify it matches the spec. You are a code reviewer, NOT a tester. Do NOT run tests, start servers, or hit endpoints — that's the Tester's job.

Review process:
1. For each acceptance criterion (AC-NNN) in acceptance.md, find the code that implements it and verify it's correct
2. Check for over-engineering: is the implementation the minimum needed?
3. Check for missing implementations: any spec requirements with no corresponding code?
4. Security review for P1 features: authentication, authorization, input validation

Write your findings to review-report.md and submit via CLI:
  devteam artifact submit %s review_report --file review-report.md

With:
- Per-criterion analysis: every AC-NNN from acceptance.md with MET or NOT MET status
- Quoted evidence: specific code with file path and line number
- Over-engineering findings: line count vs expected
- Missing implementation: user stories with no corresponding code

Format for each criterion:
  AC-NNN: [criterion text]
  Status: MET or NOT MET
  Evidence: [file:line] [quoted code or spec text]
  Explanation: [how the code satisfies or fails the criterion]

DO NOT:
- Run tests — that's the Testing phase's job
- Start the service or hit endpoints — that's the Testing phase's job
- Write test files — that's the Testing phase's job
- Write documentation — that's the Delivery phase's job
- Run build commands — that's the Construction phase's job

No critical findings may remain unresolved.`, featureID, featureID)

	case feature.PhaseTesting:
		return fmt.Sprintf(`You are in the TESTING phase for feature %s.

Your task: Write and run tests. You own testing — no other phase runs tests.

Testing process:
1. Spec-implementation drift: Compare spec against what was built before writing tests
2. Discover the project's test infrastructure: read package.json scripts, Makefile, go.mod, Cargo.toml, etc.
3. Write tests at the appropriate levels for what changed:
   - Smoke tests: verify the service/app starts and responds without panicking
   - Integration tests: full request/response cycles or API interactions
   - E2E tests: if the repo has browser test infrastructure, write and run them
   - Unit tests: business logic, state machine transitions, serialization
4. Run ALL tests that the project supports — discover and use the project's test commands
5. Agent failure mode verification: null pointers, empty collections vs null, phantom methods

Key principles:
- Discover what test commands exist and run them — don't invent new commands
- If the project has browser test infrastructure (Playwright, Cypress, etc.), use it
- If tests need a running server, check if the test framework handles server lifecycle automatically
- If you need to start a server for tests, use a port that is NOT already in use
- If tests fail, fix the TEST if the test is wrong, or report the BUG in test-report.md if the implementation is wrong
- Write real tests with real assertions — not "all tests pass" without evidence

Do NOT manage server processes manually:
- Do NOT run ps, grep for processes, start/stop/kill servers by hand
- Let the test framework handle server lifecycle
- Do NOT run commands in a loop waiting for something to happen — run once, read output, act on it

DO NOT:
- Write implementation code — that's the Construction phase's job
- Review code against acceptance criteria — that's the Review phase's job
- Write documentation — that's the Delivery phase's job
- Run build commands (beyond what's needed to compile tests)

Write your test report to test-report.md and submit via CLI:
  devteam artifact submit %s test_report --file test-report.md

With:
- Spec-implementation drift findings
- Test commands discovered and run (exact commands with output)
- Smoke test results: what was started, what was hit, what status codes returned
- Integration test results: which request/response cycles were verified
- E2E test results (if applicable): which scenarios were tested in a browser
- Unit test results: which logic was tested in isolation
- Null/empty checks: which fields verified to return empty collections not null
- Exact assertions verified
- Anti-fake-report: specific evidence, not "all tests pass"

Quality gate:
- Every acceptance criterion has at least one test
- No null pointer panics, no null-vs-empty-collection mismatches
- All tests pass
- ANY failing test is an automatic recirculate`, featureID, featureID)

	case feature.PhaseDelivery:
		return fmt.Sprintf(`You are in the DELIVERY phase for feature %s.

Your task: Write documentation ONLY. The previous phases already built, reviewed, and tested everything. You do NOT verify, build, test, or deploy anything.

The Testing phase ran the full test suite. The Review phase verified acceptance criteria. The Construction phase built the code. Your job is documentation.

Write documentation to a local docs/ directory, then submit via CLI:
  devteam artifact submit %s docs --file docs/index.md

With:
1. **API documentation** — for every endpoint in the plan: method, path, request/response schemas, error responses
2. **User-facing documentation** — for every user story in the spec, using spec terminology
3. **Changelog** — reference the spec number in every entry
4. **Cross-repo release order** (if applicable) — shared libraries first, consumers second, frontend last
5. **Configuration documentation** — env vars, config files, dependencies

Terminology consistency check: documentation must use the same terms as spec.md, not code-internal names.

DO NOT:
- Run build commands (go build, npm run build, etc.) — Construction already did this
- Run test commands (go test, npm test, npx playwright test, etc.) — Testing already did this
- Start the service or hit endpoints — Testing already did this
- Review code against acceptance criteria — Review already did this
- Write implementation code — Construction already did this
- Commit or push code — the pipeline handles commits and pushes automatically
- Check running processes, verify dependencies, or re-prove anything

Write the docs. That's all.`, featureID, featureID)

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

	// State management instructions — agent MUST use the CLI
	b.WriteString("---\n\n")
	b.WriteString("## State Management — USE THE CLI\n\n")
	b.WriteString(fmt.Sprintf("You are working on feature `%s`. Use the `devteam` CLI to manage state:\n\n", featureID))
	b.WriteString(fmt.Sprintf("- Submit questions: `devteam questions ask %s --file questions.json` then `devteam signal %s needs_feedback`\n", featureID, featureID))
	b.WriteString(fmt.Sprintf("- Signal complete: `devteam signal %s pass`\n", featureID))
	b.WriteString(fmt.Sprintf("- Send work back: `devteam signal %s recirculate:<target> --notes \"what to fix\"`\n", featureID))
	b.WriteString(fmt.Sprintf("- Add notes: `devteam notes add %s --phase %s --content \"what you decided\"`\n", featureID, phase))
	b.WriteString(fmt.Sprintf("- Check status: `devteam feature status %s`\n\n", featureID))
	b.WriteString("Do NOT write outcome.txt or questions.json manually and expect the pipeline to find them. The CLI handles all database operations.\n\n")

	b.WriteString("---\n\n")
	b.WriteString(promptContext)
	return b.String()
}
