package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/feature"
)

type Manager struct {
	baseDir string
	worktreesDir string
}

func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir:       baseDir,
		worktreesDir:  filepath.Join(baseDir, "worktrees"),
	}
}

func (m *Manager) CloneRepo(url, branch, destDir string) error {
	if _, err := os.Stat(destDir); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(destDir), 0755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}
	cmd := exec.Command("git", "clone", "--branch", branch, url, destDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

func (m *Manager) CloneForFeature(repo *config.RepoEntry, featureID string) (string, error) {
	workDir := m.GetWorkDir(repo.Name)
	if _, err := os.Stat(filepath.Join(workDir, ".git")); err == nil {
		if err := m.FetchOrigin(workDir); err != nil {
			return "", fmt.Errorf("fetching origin for %s: %w", repo.Name, err)
		}
		return workDir, nil
	}
	branch := "main"
	if err := m.CloneRepo(repo.URL, branch, workDir); err != nil {
		return "", fmt.Errorf("cloning %s: %w", repo.Name, err)
	}
	return workDir, nil
}

func (m *Manager) CreateFeatureBranch(repoDir, branchName string) error {
	cmd := exec.Command("git", "checkout", "-b", branchName)
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "already exists") {
			cmd := exec.Command("git", "checkout", branchName)
			cmd.Dir = repoDir
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("git checkout existing branch failed: %w\noutput: %s", err, string(output))
			}
			return nil
		}
		return fmt.Errorf("git checkout -b failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

func (m *Manager) CommitAll(repoDir, message string) error {
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %w\noutput: %s", err, string(output))
	}

	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Lobsterdog Contributors",
		"GIT_AUTHOR_EMAIL=noreply@lobsterdog.dev",
		"GIT_COMMITTER_NAME=Lobsterdog Contributors",
		"GIT_COMMITTER_EMAIL=noreply@lobsterdog.dev",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("git commit failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

func (m *Manager) CommitAcrossRepos(repos []*RepoWorkDir, featureID string) error {
	message := fmt.Sprintf("feat(%s): implement changes", featureID)
	for _, rwd := range repos {
		if err := m.CommitAll(rwd.Dir, message); err != nil {
			return fmt.Errorf("committing in %s: %w", rwd.Name, err)
		}
	}
	return nil
}

func (m *Manager) GetWorkDir(repoName string) string {
	return filepath.Join(m.worktreesDir, repoName)
}

func (m *Manager) IsRepoCloned(repoName string) bool {
	dir := m.GetWorkDir(repoName)
	gitDir := filepath.Join(dir, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

func (m *Manager) FetchOrigin(repoDir string) error {
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

func (m *Manager) IsBuildable(repoDir string) bool {
	for _, buildFile := range []string{"go.mod", "package.json", "Makefile", "Cargo.toml"} {
		if _, err := os.Stat(filepath.Join(repoDir, buildFile)); err == nil {
			return true
		}
	}
	return false
}

type RepoWorkDir struct {
	Name string
	URL  string
	Dir  string
}

func (m *Manager) PrepareRepos(repos []feature.RepoRef, featureID string) ([]*RepoWorkDir, error) {
	var workDirs []*RepoWorkDir
	for _, repo := range repos {
		cfgRepo := &config.RepoEntry{
			Name: repo.Name,
			URL:  repo.URL,
		}
		dir, err := m.CloneForFeature(cfgRepo, featureID)
		if err != nil {
			return nil, fmt.Errorf("preparing repo %s: %w", repo.Name, err)
		}
		branchName := fmt.Sprintf("feature/%s", featureID)
		if err := m.CreateFeatureBranch(dir, branchName); err != nil {
			return nil, fmt.Errorf("creating feature branch in %s: %w", repo.Name, err)
		}
		workDirs = append(workDirs, &RepoWorkDir{
			Name: repo.Name,
			URL:  repo.URL,
			Dir:  dir,
		})
	}
	return workDirs, nil
}

func (m *Manager) LoadReposConfig() (*config.ReposConfig, error) {
	path := filepath.Join(m.baseDir, "repos.yaml")
	return config.LoadRepos(path)
}