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

func (p *Pipeline) SetDatabase(database *db.DB) {
	p.database = database
	p.sessionMgr = NewSessionManager(database, p.dispatcher)
}

func (p *Pipeline) SetQuestionStore(qs feature.QuestionStore) {
	p.questionStore = qs
}

func (p *Pipeline) Dispatcher() *role.Dispatcher {
	return p.dispatcher
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