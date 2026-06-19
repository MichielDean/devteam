package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Manager struct {
	baseDir string
}

func NewManager(baseDir string) *Manager {
	return &Manager{baseDir: baseDir}
}

func (m *Manager) CloneRepo(url, branch, destDir string) error {
	if _, err := os.Stat(destDir); err == nil {
		return nil
	}
	cmd := exec.Command("git", "clone", "--branch", branch, url, destDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w\noutput: %s", err, string(output))
	}
	return nil
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("git commit failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

func (m *Manager) GetWorkDir(repoName string) string {
	return filepath.Join(m.baseDir, "worktrees", repoName)
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
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %w\noutput: %s", err, string(output))
	}
	return nil
}