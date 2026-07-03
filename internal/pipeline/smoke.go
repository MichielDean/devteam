package pipeline

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MichielDean/devteam/internal/feature"
)

// smokeImplFilesChanged checks that the construction stage actually modified
// or created implementation files. Iterates ALL prepared repos.
func (p *Pipeline) smokeImplFilesChanged(f *feature.Feature, preDispatchCommit string) []string {
	repoDirs := p.implRepoDirs(f)
	if len(repoDirs) == 0 {
		// No impl repos registered — check if ANY repos are registered in DB
		if p.database != nil {
			repos, _ := p.database.GetFeatureRepos(f.ID)
			if len(repos) == 0 {
				// No repos registered at all — this is a spec-only stage, skip smoke check
				return nil
			}
		}
		// Repos registered but dirs missing — that's a real problem
		return []string{"implementation repos are registered but worktree directories are missing — repo preparation failed"}
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