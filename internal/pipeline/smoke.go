package pipeline

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
)

// SmokeCheck runs after the agent signals pass. Catches the obvious failure
// mode: agent claimed pass but did nothing. Returns a list of failure reasons;
// empty list means the smoke check passed.
//
// This replaces the 1000-line substring-matching gate. The agent is trusted
// as the primary evaluator — the smoke check is just "did the agent actually
// do something?"
func (p *Pipeline) SmokeCheck(f *feature.Feature, phase feature.Phase, preDispatchCommit string) []string {
	switch phase {
	case feature.PhaseInception:
		return p.smokeArtifactsExist(f, []feature.ArtifactType{
			feature.ArtifactSpecMD, feature.ArtifactAcceptanceMD, feature.ArtifactReposYAML,
		})
	case feature.PhasePlanning:
		return p.smokeArtifactsExist(f, []feature.ArtifactType{
			feature.ArtifactPlanMD, feature.ArtifactTasksMD, feature.ArtifactDataModelMD,
			feature.ArtifactResearchMD, feature.ArtifactContractsDir,
		})
	case feature.PhaseConstruction:
		return p.smokeImplFilesChanged(f, preDispatchCommit)
	case feature.PhaseReview:
		return p.smokeReviewReport(f)
	case feature.PhaseTesting:
		return p.smokeTestFilesCreated(f, preDispatchCommit)
	case feature.PhaseDelivery:
		return p.smokeArtifactsExist(f, []feature.ArtifactType{feature.ArtifactDocs})
	}
	return nil
}

// smokeArtifactsExist checks that each required artifact is present in the DB
// and non-empty.
func (p *Pipeline) smokeArtifactsExist(f *feature.Feature, arts []feature.ArtifactType) []string {
	var failures []string
	for _, art := range arts {
		content, err := p.specProvider.ReadArtifact(f.ID, art)
		if err != nil || strings.TrimSpace(content) == "" {
			failures = append(failures, fmt.Sprintf("artifact %s missing or empty", art))
		}
	}
	return failures
}

// smokeImplFilesChanged checks that the construction phase actually modified
// or created implementation files (not just spec artifacts). Iterates ALL
// prepared repos — a feature spanning multiple repos must produce code in
// at least one.
func (p *Pipeline) smokeImplFilesChanged(f *feature.Feature, preDispatchCommit string) []string {
	repoDirs := p.implRepoDirs(f)
	if len(repoDirs) == 0 {
		// No prepared repos — spec-only feature. Check the spec worktree.
		repoDirs = []string{p.WorktreeDir(f)}
	}

	totalImplFiles := 0
	for _, dir := range repoDirs {
		changed := p.changedFiles(dir, preDispatchCommit)
		implCount := countImplFiles(changed)
		totalImplFiles += implCount
	}

	if totalImplFiles == 0 {
		return []string{"no implementation files were modified or created — agent did no coding work"}
	}
	return nil
}

// smokeReviewReport checks that the review report exists and references at
// least one file path (evidence the reviewer actually looked at code).
func (p *Pipeline) smokeReviewReport(f *feature.Feature) []string {
	content, err := p.specProvider.ReadArtifact(f.ID, feature.ArtifactReviewReport)
	if err != nil || strings.TrimSpace(content) == "" {
		return []string{"review_report artifact missing or empty"}
	}
	lower := strings.ToLower(content)
	hasFilePath := strings.Contains(lower, ".go:") || strings.Contains(lower, ".ts:") ||
		strings.Contains(lower, ".tsx:") || strings.Contains(lower, ".py:") ||
		strings.Contains(lower, ".rs:") || strings.Contains(lower, "file:") || strings.Contains(lower, "line")
	if !hasFilePath {
		return []string{"review_report contains no file path evidence — reviewer did not inspect code"}
	}
	return nil
}

// smokeTestFilesCreated checks that the testing phase created actual test
// files (not just a report). Iterates ALL prepared repos.
func (p *Pipeline) smokeTestFilesCreated(f *feature.Feature, preDispatchCommit string) []string {
	// Report must exist
	content, err := p.specProvider.ReadArtifact(f.ID, feature.ArtifactTestReport)
	if err != nil || strings.TrimSpace(content) == "" {
		return []string{"test_report artifact missing or empty"}
	}

	repoDirs := p.implRepoDirs(f)
	if len(repoDirs) == 0 {
		repoDirs = []string{p.WorktreeDir(f)}
	}

	totalTestFiles := 0
	for _, dir := range repoDirs {
		changed := p.changedFiles(dir, preDispatchCommit)
		for _, line := range changed {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "_test.go") || strings.Contains(lower, "test.ts") ||
				strings.Contains(lower, "test.tsx") || strings.Contains(lower, ".spec.ts") ||
				strings.Contains(lower, ".spec.tsx") || strings.Contains(lower, "test_") ||
				strings.Contains(lower, "_test.") || strings.Contains(lower, "/tests/") ||
				strings.Contains(lower, "/e2e/") || strings.Contains(lower, "/test/") {
				if !strings.HasPrefix(line, "specs/") {
					totalTestFiles++
				}
			}
		}
	}

	if totalTestFiles == 0 {
		return []string{"no test files were created or modified — tester wrote a report but no tests"}
	}
	return nil
}

// implRepoDirs returns the worktree dirs of all prepared impl repos.
func (p *Pipeline) implRepoDirs(f *feature.Feature) []string {
	if p.database == nil {
		return nil
	}
	repos, err := p.database.GetFeatureRepos(f.ID)
	if err != nil {
		return nil
	}
	dirs := make([]string, 0, len(repos))
	for _, r := range repos {
		if _, err := os.Stat(filepath.Join(r.Dir, ".git")); err == nil {
			dirs = append(dirs, r.Dir)
		}
	}
	return dirs
}

// changedFiles returns the list of files changed since preDispatchCommit.
func (p *Pipeline) changedFiles(workDir, preDispatchCommit string) []string {
	if preDispatchCommit == "" {
		out, err := exec.Command("git", "-C", workDir, "status", "--porcelain").Output()
		if err != nil {
			return nil
		}
		return parseGitStatus(string(out))
	}
	out, err := exec.Command("git", "-C", workDir, "diff", "--name-only", preDispatchCommit).Output()
	if err != nil {
		// Fallback to uncommitted changes
		out, err := exec.Command("git", "-C", workDir, "status", "--porcelain").Output()
		if err != nil {
			return nil
		}
		return parseGitStatus(string(out))
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

func parseGitStatus(s string) []string {
	var files []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if len(line) > 3 {
			files = append(files, strings.TrimSpace(line[2:]))
		}
	}
	return files
}

func countImplFiles(files []string) int {
	count := 0
	for _, line := range files {
		if strings.HasPrefix(line, "specs/") || strings.Contains(line, ".devteam-state") ||
			strings.Contains(line, "CONTEXT.md") || strings.Contains(line, "NOTES.md") ||
			strings.Contains(line, "questions.json") || strings.HasSuffix(line, "go.sum") ||
			strings.HasSuffix(line, "package-lock.json") || strings.Contains(line, "node_modules") ||
			strings.HasPrefix(line, "logs/") {
			continue
		}
		count++
	}
	return count
}

// BuildAllRepos builds all prepared repos. Returns failure messages.
// Used by the testing smoke check to verify code compiles across all repos.
func (p *Pipeline) BuildAllRepos(f *feature.Feature) []string {
	repoDirs := p.implRepoDirs(f)
	var failures []string
	for _, dir := range repoDirs {
		if !buildRepo(dir) {
			failures = append(failures, fmt.Sprintf("build failed in %s", dir))
		}
	}
	return failures
}

func buildRepo(workDir string) bool {
	if _, err := os.Stat(filepath.Join(workDir, "go.mod")); err == nil {
		goPath, err := exec.LookPath("go")
		if err != nil {
			goPath = "/usr/local/go/bin/go"
		}
		cmd := exec.Command(goPath, "build", "./...")
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":"+"/usr/local/go/bin")
		out, err := cmd.CombinedOutput()
		if err != nil {
			_ = out
			return false
		}
		return true
	}
	if _, err := os.Stat(filepath.Join(workDir, "package.json")); err == nil {
		pkg, _ := os.ReadFile(filepath.Join(workDir, "package.json"))
		if !strings.Contains(string(pkg), "\"build\"") {
			return true
		}
		cmd := exec.Command("npm", "run", "build")
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH"))
		return cmd.Run() == nil
	}
	if _, err := os.Stat(filepath.Join(workDir, "Cargo.toml")); err == nil {
		cmd := exec.Command("cargo", "build")
		cmd.Dir = workDir
		return cmd.Run() == nil
	}
	if _, err := os.Stat(filepath.Join(workDir, "Makefile")); err == nil {
		cmd := exec.Command("make")
		cmd.Dir = workDir
		return cmd.Run() == nil
	}
	return true
}

var _ = db.EventPhaseComplete