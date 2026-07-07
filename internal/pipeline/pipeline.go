package pipeline

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/gitops"
	"github.com/MichielDean/devteam/internal/repo"
	"github.com/MichielDean/devteam/internal/role"
	"github.com/MichielDean/devteam/internal/rules"
	"github.com/MichielDean/devteam/internal/spec"
)

// maxConcurrentAgents limits how many agent tmux sessions can run at once.
// Too many concurrent agents will exhaust LLM API quota. The semaphore is
// acquired before dispatch and released after the agent exits.
const maxConcurrentAgents = 4

// Pipeline orchestrates agent dispatch, outcome reading, and feature state.
// One phase per RunPhase call — no recursion, no autopilot loop.
type Pipeline struct {
	config         *config.Config
	specProvider   *spec.SpecProvider
	specWriter     *spec.SpecWriter
	ruleLoader     *rules.RuleLoader
	roleLoader     *role.RoleLoader
	dispatcher     *role.Dispatcher
	questionStore  feature.QuestionStore
	gitClient      *gitops.GitClient
	repoManager    *repo.Manager
	database       *db.DB
	sessionMgr     *SessionManager
	agentSemaphore chan struct{} // limits concurrent agent dispatches
}

func NewPipeline(cfg *config.Config, specProvider *spec.SpecProvider) *Pipeline {
	baseDir := specProvider.BaseDir()
	dispatcher := role.NewDispatcher(baseDir)
	p := &Pipeline{
		config:         cfg,
		specProvider:   specProvider,
		specWriter:     spec.NewSpecWriter(baseDir),
		ruleLoader:     rules.NewRuleLoaderWithConfig(baseDir, cfg),
		roleLoader:     role.NewRoleLoader(baseDir),
		dispatcher:     dispatcher,
		questionStore:  feature.NewFileQuestionStore(baseDir),
		gitClient:      gitops.NewGitClient(baseDir),
		repoManager:    repo.NewManager(baseDir),
		agentSemaphore: make(chan struct{}, maxConcurrentAgents),
	}
	// Wire the batcher config (thresholds) from the loaded Streaming config
	// (U-BK-01). The FlushFunc / ResetFunc are wired in SetDatabase (they
	// need the *db.DB) — U-BK-06. Guard nil cfg — some test paths construct
	// a pipeline with a nil config (chat_handlers_test.go, fr011_invariant_test.go).
	if cfg != nil {
		streamCfg := cfg.Pipeline.Streaming
		dispatcher.TmuxManager().SetStreamConfig(role.StreamConfig{
			FlushIntervalMs: streamCfg.GetFlushIntervalMs(),
			FlushBytes:      streamCfg.GetFlushBytes(),
		})
	}
	return p
}

func (p *Pipeline) SetDatabase(database *db.DB) {
	p.database = database
	p.sessionMgr = NewSessionManager(database, p.dispatcher)
	// Wire the FlushFunc and ResetFunc (U-BK-06 / ADR-7 / ADR-2). These are
	// the DI seams that keep internal/role free of internal/db (AC-5). The
	// closures adapt the batcher's signature to the store methods. Skipped
	// when the dispatcher is nil (some test paths construct a pipeline with
	// a nil dispatcher — e.g. server_test.go's setupTestServer).
	if p.dispatcher == nil {
		return
	}
	tmuxMgr := p.dispatcher.TmuxManager()
	tmuxMgr.SetFlushFn(func(ctx context.Context, featureID, stageID, agentRole string, bolt int, chunk string) error {
		return database.AppendStageLogForBolt(featureID, stageID, bolt, agentRole, chunk)
	})
	tmuxMgr.SetResetFn(func(ctx context.Context, featureID, stageID string, bolt int) error {
		return database.SaveStageLogForBolt(featureID, stageID, bolt, "", "")
	})
}

func (p *Pipeline) SetQuestionStore(qs feature.QuestionStore) {
	p.questionStore = qs
}

// acquireAgentSlot blocks until a concurrent-agent slot is available.
// Releases on return. Call via defer in the dispatch path.
func (p *Pipeline) acquireAgentSlot() func() {
	p.agentSemaphore <- struct{}{}
	return func() { <-p.agentSemaphore }
}

func (p *Pipeline) Dispatcher() *role.Dispatcher {
	return p.dispatcher
}

// Config returns the loaded configuration (or nil if not set — e.g. some test
// paths). Exposed so the API server can read the Streaming config (the
// log_file_fallback flag for the read-path legacy fallback — U-BK-07 / ADR-5).
func (p *Pipeline) Config() *config.Config {
	return p.config
}

func (p *Pipeline) Database() *db.DB {
	return p.database
}

// SessionMgr returns the session manager for tmux session lifecycle management.
func (p *Pipeline) SessionMgr() *SessionManager {
	return p.sessionMgr
}

func NewPipelineWithDispatcher(cfg *config.Config, specProvider *spec.SpecProvider, dispatcher *role.Dispatcher) *Pipeline {
	baseDir := specProvider.BaseDir()
	p := &Pipeline{
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
	if cfg != nil && dispatcher != nil {
		streamCfg := cfg.Pipeline.Streaming
		dispatcher.TmuxManager().SetStreamConfig(role.StreamConfig{
			FlushIntervalMs: streamCfg.GetFlushIntervalMs(),
			FlushBytes:      streamCfg.GetFlushBytes(),
		})
	}
	return p
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

// OutputLineCallback is called for each line of agent output during dispatch.
type OutputLineCallback func(line string, isStderr bool)

// detectAndStoreQuestions reads questions from disk (legacy) or DB and pauses
// the feature if pending questions exist.
func (p *Pipeline) detectAndStoreQuestions(ctx context.Context, f *feature.Feature, phase string) {
	specDir := p.specProvider.FeatureDirFromFeature(f)
	detectedQuestions := feature.DetectQuestions(f.ID, specDir)
	if len(detectedQuestions) == 0 {
		if pending, _ := p.questionStore.PendingCount(ctx, f.ID); pending > 0 {
			if err := f.WaitForHuman(); err != nil {
				log.Printf("warning: cannot transition feature %s to waiting_for_feedback: %v", f.ID, err)
			} else {
				p.saveFeatureState(f)
				log.Printf("RunPhase: feature %s paused for human input (%d questions from DB)", f.ID, pending)
			}
		}
		return
	}

	log.Printf("RunPhase: detected %d questions (disk) for feature %s after %s phase", len(detectedQuestions), f.ID, phase)
	for i := range detectedQuestions {
		detectedQuestions[i].FeatureID = f.ID
		if _, err := p.questionStore.CreateQuestion(ctx, f.ID, detectedQuestions[i]); err != nil {
			log.Printf("warning: failed to create question for feature %s: %v", f.ID, err)
			continue
		}
	}
	os.Remove(filepath.Join(specDir, "questions.json"))

	if err := f.WaitForHuman(); err != nil {
		log.Printf("warning: cannot transition feature %s to waiting_for_feedback: %v", f.ID, err)
	} else {
		p.saveFeatureState(f)
		log.Printf("RunPhase: feature %s paused for human input (%d questions)", f.ID, len(detectedQuestions))
	}
}

// buildContext assembles the full context string for a phase dispatch.
func (p *Pipeline) buildContext(ctx context.Context, f *feature.Feature, phase string, roleName string) (string, error) {
	contextStr, err := p.ruleLoader.BuildContext(phase, roleName, f.Priority)
	if err != nil {
		return "", fmt.Errorf("building context for phase %s role %s: %w", phase, roleName, err)
	}

	if phase == "ideation" || phase == "inception" {
		if inputContent, err := p.specProvider.ReadArtifact(f.ID, feature.ArtifactInputMD); err == nil && inputContent != "" {
			contextStr += "\n\n---\n\n=== Feature Input ===\n" + inputContent
		}
	}

	if p.questionStore != nil {
		questions, qErr := p.questionStore.ListQuestions(ctx, f.ID)
		if qErr == nil && len(questions) > 0 {
			timeoutMinutes := p.config.Pipeline.GetHumanInteractionTimeoutMinutes()
			humanResponses := feature.BuildHumanResponsesContext(questions, timeoutMinutes)
			if humanResponses != "" {
				contextStr += humanResponses
			}
		}
	}

	specContext, err := p.specProvider.BuildCrossRepoContext(f.ID, nil)
	if err == nil && specContext != "" {
		contextStr += "\n\n---\n\n" + specContext
	}

	if p.database != nil {
		revisionNotes := p.buildRevisionNotesContext(f, phase)
		if revisionNotes != "" {
			contextStr += revisionNotes
		}

		notesContext := p.database.BuildNotesContext(f.ID, string(phase))
		if notesContext != "" {
			contextStr += notesContext
		}
	}

	return contextStr, nil
}

func (p *Pipeline) buildRevisionNotesContext(f *feature.Feature, phase string) string {
	if p.database == nil {
		return ""
	}
	notes, err := p.database.GetNotesForPhase(f.ID, string(phase))
	if err != nil || len(notes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n---\n\n# ⚠️ REVISION REQUIRED\n\n")
	b.WriteString("A previous run of this phase was sent back. Address these issues:\n\n")
	for _, n := range notes {
		if n.NoteType == "revision" {
			b.WriteString(fmt.Sprintf("## From %s\n\n%s\n\n", n.Role, n.Content))
		}
	}
	return b.String()
}

// saveFeatureState saves feature state to DB (single source of truth).
func (p *Pipeline) saveFeatureState(f *feature.Feature) {
	if err := p.specProvider.SaveFeatureState(f); err != nil {
		log.Printf("warning: could not save feature state for %s: %v", f.ID, err)
	}
	if p.database != nil {
		p.database.UpdateFeatureStatus(f.ID, string(f.Status), f.CurrentPhaseLegacy())
	}
}

// EnsureSpecWorktree creates a per-feature git worktree if it doesn't exist yet.
func (p *Pipeline) EnsureSpecWorktree(f *feature.Feature) error {
	if f.WorktreeDir != "" {
		if _, err := os.Stat(filepath.Join(f.WorktreeDir, ".git")); err == nil {
			return nil
		}
	}

	worktreeDir := filepath.Join(os.Getenv("HOME"), "worktrees", "devteam-specs", f.ID)
	branchName := "spec/" + f.ID

	cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreeDir, "origin/main")
	cmd.Dir = p.specProvider.BaseDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmd2 := exec.Command("git", "worktree", "add", worktreeDir, branchName)
		cmd2.Dir = p.specProvider.BaseDir()
		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			return fmt.Errorf("creating spec worktree: %w: %s (retry: %s)", err, string(out), string(out2))
		}
	}

	f.WorktreeDir = worktreeDir
	log.Printf("EnsureSpecWorktree: created worktree at %s on branch %s for feature %s", worktreeDir, branchName, f.ID)
	p.saveFeatureState(f)
	return nil
}

func (p *Pipeline) WorktreeDir(f *feature.Feature) string {
	if f.WorktreeDir != "" {
		return f.WorktreeDir
	}
	return p.specProvider.BaseDir()
}

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

func (p *Pipeline) dispatchWorkingDirForPhase(f *feature.Feature, phase string) string {
	if p.isImplPhase(phase) {
		if dirs := p.implRepoDirs(f); len(dirs) > 0 {
			return dirs[0]
		}
	}
	return p.WorktreeDir(f)
}

func (p *Pipeline) isImplPhase(phase string) bool {
	switch phase {
	case "construction", "review", "testing", "delivery":
		return true
	}
	return false
}

func (p *Pipeline) implRepoContext(f *feature.Feature, phase string) string {
	if !p.isImplPhase(phase) || p.database == nil {
		return ""
	}
	repos, err := p.database.GetFeatureRepos(f.ID)
	if err != nil || len(repos) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n---\n\n# Implementation Repositories\n\n")
	b.WriteString("Code changes for this feature must land in the following repository worktrees. Each is already cloned and on the feature branch — DO NOT re-clone, re-branch, or push to main.\n\n")
	for i, pr := range repos {
		flag := ""
		if i == 0 {
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
	return b.String()
}

// PrepareImplRepos clones every repo declared in the feature's repos.yaml
// into a per-feature worktree and records them in feature_repos DB table.
func (p *Pipeline) PrepareImplRepos(f *feature.Feature) error {
	if p.database == nil {
		return nil
	}

	existing, err := p.database.GetFeatureRepos(f.ID)
	if err == nil && len(existing) > 0 {
		allExist := true
		for _, r := range existing {
			if _, err := os.Stat(filepath.Join(r.Dir, ".git")); err != nil {
				allExist = false
				break
			}
		}
		if allExist {
			log.Printf("PrepareImplRepos: %s already has %d prepared repo(s), reusing", f.ID, len(existing))
			return nil
		}
	}

	refs, err := p.specProvider.LoadFeatureRepos(f.ID)
	if err != nil {
		return fmt.Errorf("loading repos.yaml for %s: %w", f.ID, err)
	}
	if len(refs) == 0 {
		log.Printf("PrepareImplRepos: %s has no repos.yaml entries — feature touches only the spec repo", f.ID)
		p.database.DeleteFeatureRepos(f.ID)
		return nil
	}

	workDirs, err := p.repoManager.PrepareRepos(refs, f.ID)
	if err != nil {
		return fmt.Errorf("preparing impl repos for %s: %w", f.ID, err)
	}

	for _, wd := range workDirs {
		if err := p.database.SaveFeatureRepo(f.ID, wd.Name, wd.URL, wd.Dir, wd.Branch); err != nil {
			log.Printf("warning: could not save feature repo %s: %v", wd.Name, err)
		}
	}
	log.Printf("PrepareImplRepos: prepared %d repo(s) for %s", len(workDirs), f.ID)
	return nil
}

func (p *Pipeline) CleanupImplRepos(f *feature.Feature) {
	if err := p.repoManager.RemoveAllWorktreesFor(f.ID); err != nil {
		log.Printf("warning: could not remove impl repo worktrees for %s: %v", f.ID, err)
	}
	if p.database != nil {
		p.database.DeleteFeatureRepos(f.ID)
	}
}

func (p *Pipeline) PushPhaseChanges(f *feature.Feature, phase string) error {
	message := fmt.Sprintf("%s: complete %s phase for %s", phase, f.ID, phase)

	if p.database == nil {
		branchName := "feat/" + f.ID
		return p.gitClient.CommitAndPush(branchName, message)
	}

	repos, err := p.database.GetFeatureRepos(f.ID)
	if err != nil || len(repos) == 0 {
		branchName := "feat/" + f.ID
		return p.gitClient.CommitAndPush(branchName, message)
	}

	workDirs := make([]*repo.RepoWorkDir, 0, len(repos))
	for _, pr := range repos {
		workDirs = append(workDirs, &repo.RepoWorkDir{
			Name: pr.Name, URL: pr.URL, Dir: pr.Dir, Branch: pr.Branch,
		})
	}
	if err := p.repoManager.CommitAcrossRepos(workDirs, f.ID); err != nil {
		return fmt.Errorf("committing across impl repos: %w", err)
	}
	if err := p.repoManager.PushAcrossRepos(workDirs, f.ID); err != nil {
		return fmt.Errorf("pushing across impl repos: %w", err)
	}
	branchName := "feat/" + f.ID
	if err := p.gitClient.CommitAndPush(branchName, message); err != nil {
		log.Printf("warning: could not push spec repo state: %v", err)
	}
	return nil
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

func (p *Pipeline) UpdateFeatureStatus(f *feature.Feature) error {
	return p.specProvider.SaveFeatureState(f)
}

// MergeFeatureToMain merges the feature's worktree branch into main.
// Called after construction (3.1-3.7) completes so code doesn't die in
// a random worktree. In autonomous/guided mode this runs automatically;
// in human mode the user triggers it via the API.
//
// Steps per repo:
// 1. Commit any uncommitted changes in the worktree
// 2. Push the feature branch to origin
// 3. Merge the feature branch into main (fast-forward if possible)
// 4. Push main to origin
// 5. Record audit event
func (p *Pipeline) MergeFeatureToMain(f *feature.Feature) error {
	if p.database == nil {
		return nil
	}

	repos, err := p.database.GetFeatureRepos(f.ID)
	if err != nil || len(repos) == 0 {
		log.Printf("MergeFeatureToMain: %s has no repos — nothing to merge", f.ID)
		return nil
	}

	for _, r := range repos {
		if _, err := os.Stat(r.Dir); err != nil {
			log.Printf("MergeFeatureToMain: worktree dir %s does not exist — skipping %s", r.Dir, r.Name)
			continue
		}

		// 1. Commit uncommitted changes
		commitMsg := fmt.Sprintf("feat: merge %s — construction complete", f.ID)
		if err := p.gitCommitIfChanges(r.Dir, commitMsg); err != nil {
			log.Printf("MergeFeatureToMain: commit failed for %s: %v — continuing", r.Name, err)
		}

		// 2. Push feature branch to origin
		pushCmd := exec.Command("git", "-C", r.Dir, "push", "-u", "origin", r.Branch)
		pushOutput, err := pushCmd.CombinedOutput()
		if err != nil {
			log.Printf("MergeFeatureToMain: push failed for %s branch %s: %v\n%s", r.Name, r.Branch, err, string(pushOutput))
			continue
		}
		log.Printf("MergeFeatureToMain: pushed %s branch %s", r.Name, r.Branch)

		// 3. Merge feature branch into main locally
		// Find the main repo (not the worktree) — use the URL to locate it
		mainDir := p.findMainRepoDir(r.URL)
		if mainDir == "" {
			// Fallback: merge via origin — create a PR and merge
			if err := p.mergeViaPR(r.URL, r.Branch, f.ID); err != nil {
				log.Printf("MergeFeatureToMain: PR merge failed for %s: %v", r.Name, err)
			}
			continue
		}

		// Fetch latest main, merge feature branch, push
		fetchCmd := exec.Command("git", "-C", mainDir, "fetch", "origin")
		fetchCmd.Run()

		mergeCmd := exec.Command("git", "-C", mainDir, "merge", "--no-ff", "-m",
			fmt.Sprintf("Merge %s — construction complete", f.Title),
			r.Branch)
		mergeOutput, err := mergeCmd.CombinedOutput()
		if err != nil {
			log.Printf("MergeFeatureToMain: merge failed for %s: %v\n%s — trying PR", r.Name, err, string(mergeOutput))
			// Merge conflict — fall back to PR
			if err := p.mergeViaPR(r.URL, r.Branch, f.ID); err != nil {
				log.Printf("MergeFeatureToMain: PR merge also failed for %s: %v", r.Name, err)
			}
			continue
		}

		pushMainCmd := exec.Command("git", "-C", mainDir, "push", "origin", "main")
		pushMainOutput, err := pushMainCmd.CombinedOutput()
		if err != nil {
			log.Printf("MergeFeatureToMain: push main failed for %s: %v\n%s", r.Name, err, string(pushMainOutput))
			continue
		}

		log.Printf("MergeFeatureToMain: merged %s branch %s into main and pushed", r.Name, r.Branch)
		p.database.RecordAuditEvent(f.ID, "MERGED_TO_MAIN", "", "", fmt.Sprintf("%s branch %s merged to main", r.Name, r.Branch))
		p.broadcastSSE(f.ID, "merged_to_main",
			fmt.Sprintf(`{"feature_id":%s,"repo":%s,"branch":%s}`, jsonString(f.ID), jsonString(r.Name), jsonString(r.Branch)))
	}

	return nil
}

// gitCommitIfChanges commits uncommitted changes in a directory if any exist.
func (p *Pipeline) gitCommitIfChanges(dir, message string) error {
	statusCmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	output, err := statusCmd.Output()
	if err != nil || len(output) == 0 {
		return nil // no changes or error
	}
	exec.Command("git", "-C", dir, "add", "-A").Run()
	commitCmd := exec.Command("git", "-C", dir, "commit", "-m", message)
	return commitCmd.Run()
}

// findMainRepoDir finds the primary checkout directory for a repo URL.
// Worktrees are under ~/source/<repo>/worktrees/<feature>/<repo>; the
// primary checkout is at ~/source/<repo>.
func (p *Pipeline) findMainRepoDir(repoURL string) string {
	// Extract repo name from URL (last path segment, strip .git)
	parts := strings.Split(repoURL, "/")
	name := parts[len(parts)-1]
	name = strings.TrimSuffix(name, ".git")
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	mainDir := filepath.Join(home, "source", name)
	if _, err := os.Stat(filepath.Join(mainDir, ".git")); err == nil {
		return mainDir
	}
	return ""
}

// mergeViaPR creates a GitHub PR and merges it via gh CLI.
// Used when we can't find the main checkout (e.g. remote-only repos).
func (p *Pipeline) mergeViaPR(repoURL, branch, featureID string) error {
	// Extract owner/repo from URL
	// git@github.com:owner/repo.git → owner/repo
	repoPath := ""
	if strings.Contains(repoURL, "github.com:") {
		repoPath = strings.TrimPrefix(repoURL, "git@github.com:")
	} else if strings.Contains(repoURL, "github.com/") {
		repoPath = strings.TrimPrefix(repoURL, "https://github.com/")
	}
	repoPath = strings.TrimSuffix(repoPath, ".git")

	title := fmt.Sprintf("feat: merge %s — construction complete", featureID)
	prCmd := exec.Command("gh", "pr", "create",
		"--repo", repoPath,
		"--title", title,
		"--body", fmt.Sprintf("Auto-merged from devteam construction phase for feature %s", featureID),
		"--head", branch,
		"--base", "main",
	)
	prOutput, err := prCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("creating PR: %w\n%s", err, string(prOutput))
	}

	// Extract PR number from output (URL contains it)
	prURL := strings.TrimSpace(string(prOutput))
	parts := strings.Split(prURL, "/")
	prNum := parts[len(parts)-1]

	mergeCmd := exec.Command("gh", "pr", "merge", prNum,
		"--repo", repoPath,
		"--squash",
	)
	mergeOutput, err := mergeCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("merging PR %s: %w\n%s", prNum, err, string(mergeOutput))
	}

	log.Printf("mergeViaPR: merged PR %s for %s", prNum, repoPath)
	return nil
}
