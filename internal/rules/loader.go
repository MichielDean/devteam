package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type RuleLoader struct {
	baseDir string
}

func NewRuleLoader(baseDir string) *RuleLoader {
	return &RuleLoader{baseDir: baseDir}
}

func (rl *RuleLoader) PhaseRules(phase string) ([]string, error) {
	ruleDir := filepath.Join(rl.baseDir, "rules", "pipeline", phase)
	return rl.loadMarkdownFiles(ruleDir)
}

func (rl *RuleLoader) RoleRules(roleName string) (string, error) {
	path := filepath.Join(rl.baseDir, "roles", roleName, "INSTRUCTIONS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading role instructions for %s: %w", roleName, err)
	}
	return string(data), nil
}

func (rl *RuleLoader) CoreWorkflow() (string, error) {
	path := filepath.Join(rl.baseDir, "rules", "pipeline", "core-workflow.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading core workflow: %w", err)
	}
	return string(data), nil
}

func (rl *RuleLoader) ExtensionRules(extensionName string) (string, error) {
	extDir := filepath.Join(rl.baseDir, "rules", "pipeline", "extensions", extensionName)
	mds, err := rl.loadMarkdownFiles(extDir)
	if err != nil {
		return "", fmt.Errorf("extension %s not found: %w", extensionName, err)
	}
	if len(mds) > 0 {
		return mds[0], nil
	}
	return "", fmt.Errorf("extension %s not found", extensionName)
}

func (rl *RuleLoader) BuildContext(phase string, roleName string, priority int) (string, error) {
	var parts []string

	core, err := rl.CoreWorkflow()
	if err != nil {
		return "", err
	}
	parts = append(parts, "=== Core Workflow ===\n"+core)

	role, err := rl.RoleRules(roleName)
	if err != nil {
		return "", err
	}
	parts = append(parts, fmt.Sprintf("=== Role: %s ===\n%s", roleName, role))

	phaseRules, err := rl.PhaseRules(phase)
	if err != nil {
		return "", err
	}
	if len(phaseRules) > 0 {
		parts = append(parts, "=== Phase Rules ===\n"+strings.Join(phaseRules, "\n\n"))
	}

	alwaysExtensions := []string{"error-recovery", "overconfidence-prevention"}
	for _, ext := range alwaysExtensions {
		extRules, err := rl.ExtensionRules(ext)
		if err == nil {
			parts = append(parts, fmt.Sprintf("=== Extension: %s ===\n%s", ext, extRules))
		}
	}

	if priority == 1 {
		for _, ext := range []string{"security", "resiliency"} {
			extRules, err := rl.ExtensionRules(ext)
			if err == nil {
				parts = append(parts, fmt.Sprintf("=== Extension: %s ===\n%s", ext, extRules))
			}
		}
	} else if priority == 2 {
		extRules, err := rl.ExtensionRules("security")
		if err == nil {
			parts = append(parts, "=== Extension: security ===\n"+extRules)
		}
	}

	return strings.Join(parts, "\n\n---\n\n"), nil
}

func (rl *RuleLoader) loadMarkdownFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading rule directory %s: %w", dir, err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		files = append(files, string(data))
	}
	return files, nil
}
