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
	var ruleDir string
	switch phase {
	case "inception":
		ruleDir = filepath.Join(rl.baseDir, "rules", "aidlc-rule-details", "inception")
	case "planning", "construction", "review":
		ruleDir = filepath.Join(rl.baseDir, "rules", "aidlc-rule-details", "construction")
	case "testing", "delivery":
		ruleDir = filepath.Join(rl.baseDir, "rules", "aidlc-rule-details", "operations")
	default:
		return nil, fmt.Errorf("unknown phase: %s", phase)
	}
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
	path := filepath.Join(rl.baseDir, "rules", "aidlc", "core-workflow.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading core workflow: %w", err)
	}
	return string(data), nil
}

func (rl *RuleLoader) ExtensionRules(extensionName string) (string, error) {
	extDirs := []string{
		filepath.Join(rl.baseDir, "rules", "aidlc-rule-details", "extensions", "security", "baseline"),
		filepath.Join(rl.baseDir, "rules", "aidlc-rule-details", "extensions", "resiliency", "baseline"),
		filepath.Join(rl.baseDir, "rules", "aidlc-rule-details", "extensions", "testing", "property-based"),
	}
	for _, dir := range extDirs {
		matcher := fmt.Sprintf("%s/", extensionName)
		if strings.Contains(dir, matcher) {
			mds, err := rl.loadMarkdownFiles(dir)
			if err != nil {
				continue
			}
			if len(mds) > 0 {
				return mds[0], nil
			}
		}
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

	if priority == 1 {
		for _, ext := range []string{"security", "resiliency"} {
			extRules, err := rl.ExtensionRules(ext)
			if err == nil {
				parts = append(parts, fmt.Sprintf("=== Extension: %s ===\n%s", ext, extRules))
			}
		}
	} else if priority == 2 {
		extRules, err := rl.ExtensionRules("resiliency")
		if err == nil {
			parts = append(parts, "=== Extension: resiliency ===\n"+extRules)
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
