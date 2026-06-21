package gitops

import (
	"fmt"
	"os/exec"
	"strings"
)

type GitClient struct {
	baseDir string
}

func NewGitClient(baseDir string) *GitClient {
	return &GitClient{baseDir: baseDir}
}

func (g *GitClient) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.baseDir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Run is the exported version of run for use by other packages.
func (g *GitClient) Run(args ...string) (string, error) {
	return g.run(args...)
}

func (g *GitClient) CurrentBranch() (string, error) {
	out, err := g.run("branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("getting current branch: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (g *GitClient) HasRemoteBranch(branch string) bool {
	out, err := g.run("ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}

func (g *GitClient) CreateBranch(branch string) error {
	_, err := g.run("checkout", "-b", branch)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			_, err = g.run("checkout", branch)
			if err != nil {
				return fmt.Errorf("checking out existing branch %s: %w", branch, err)
			}
			return nil
		}
		return fmt.Errorf("creating branch %s: %w", branch, err)
	}
	return nil
}

func (g *GitClient) StageAll() error {
	_, err := g.run("add", "-A")
	if err != nil {
		return fmt.Errorf("staging changes: %w", err)
	}
	return nil
}

func (g *GitClient) HasStagedChanges() (bool, error) {
	out, err := g.run("diff", "--cached", "--stat")
	if err != nil {
		return false, fmt.Errorf("checking staged changes: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}

func (g *GitClient) Commit(message string) error {
	_, err := g.run("commit", "-m", message)
	if err != nil {
		return fmt.Errorf("committing: %w", err)
	}
	return nil
}

func (g *GitClient) Push(branch string) error {
	if g.HasRemoteBranch(branch) {
		_, err := g.run("push", "origin", branch)
		if err != nil {
			return fmt.Errorf("pushing to origin/%s: %w", branch, err)
		}
		return nil
	}
	_, err := g.run("push", "-u", "origin", branch)
	if err != nil {
		return fmt.Errorf("pushing -u origin/%s: %w", branch, err)
	}
	return nil
}

func (g *GitClient) CreatePullRequest(branch string, title string, body string) (string, error) {
	cmd := exec.Command("gh", "pr", "create",
		"--base", "main",
		"--head", branch,
		"--title", title,
		"--body", body,
		"--draft",
	)
	cmd.Dir = g.baseDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("creating PR: %s: %w", string(out), err)
	}
	url := strings.TrimSpace(string(out))
	return url, nil
}

func (g *GitClient) ReadyPullRequest(branch string) error {
	cmd := exec.Command("gh", "pr", "ready", branch)
	cmd.Dir = g.baseDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("marking PR ready: %s: %w", string(out), err)
	}
	return nil
}

func (g *GitClient) AddPullRequestComment(branch string, comment string) error {
	prNumber, err := g.GetPRNumber(branch)
	if err != nil {
		return err
	}
	cmd := exec.Command("gh", "pr", "comment", prNumber, "--body", comment)
	cmd.Dir = g.baseDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("commenting on PR: %s: %w", string(out), err)
	}
	return nil
}

func (g *GitClient) GetPRNumber(branch string) (string, error) {
	cmd := exec.Command("gh", "pr", "list", "--head", branch, "--json", "number", "--limit", "1")
	cmd.Dir = g.baseDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("finding PR for %s: %s: %w", branch, string(out), err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return "", fmt.Errorf("no PR found for branch %s", branch)
	}
	parts := strings.Split(lines[0], ":")
	if len(parts) < 2 {
		return "", fmt.Errorf("unexpected PR list format: %s", lines[0])
	}
	return strings.TrimSpace(parts[1]), nil
}

func (g *GitClient) CommitAndPush(branch string, message string) error {
	if err := g.StageAll(); err != nil {
		return err
	}
	hasChanges, err := g.HasStagedChanges()
	if err != nil {
		return err
	}
	if !hasChanges {
		return nil
	}
	if err := g.Commit(message); err != nil {
		return err
	}
	return g.Push(branch)
}