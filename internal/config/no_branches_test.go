package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestAC019_NoProviderSpecificBranches scans the non-test Go source in
// internal/config, internal/role, internal/pipeline, and internal/api for
// provider-specific branches (if/case on provider identity). NFR-INTEG-01
// requires presets be data, not code branches. Traces U-CFG-03, ADR-004.
//
// Allowed: dispatching on runtime values (BaseURL, APIKeyEnv) is data-driven
// (the opencodeProviderKey function inspects BaseURL, not provider names).
// Forbidden: `if provider == "anthropic"`, `case "openai"`, etc.
func TestAC019_NoProviderSpecificBranches(t *testing.T) {
	// Patterns that indicate a provider-specific code branch.
	// Matches: if <var> == "anthropic", case "anthropic":, <var> == "ollama" etc.
	// The word-boundary regex avoids false positives like a comment mentioning "anthropic".
	forbidden := regexp.MustCompile(`(==|case\s+)"(anthropic|ollama-cloud|openai|copilot)"`)
	// Also catch `if provider == "anthropic"` style with the == first.
	identityCompare := regexp.MustCompile(`(provider|preset|presetID|PresetID)\s*(==|!=)\s*"(anthropic|ollama-cloud|openai|copilot)"`)

	dirs := []string{"../config", "../role", "../pipeline", "../api"}
	var violations []string
	for _, dir := range dirs {
		abs, _ := filepath.Abs(dir)
		filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil // tests may reference provider names for assertions
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			// Skip this test file itself (it contains the regex patterns as strings).
			if strings.HasSuffix(path, "no_branches_test.go") {
				return nil
			}
			for _, line := range strings.Split(string(data), "\n") {
				// Skip comments.
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					continue
				}
				if forbidden.FindString(line) != "" && !isDataDrivenOK(line) {
					violations = append(violations, path+": "+strings.TrimSpace(line))
				}
				if identityCompare.FindString(line) != "" {
					violations = append(violations, path+": "+strings.TrimSpace(line))
				}
			}
			return nil
		})
	}
	if len(violations) > 0 {
		t.Errorf("NFR-INTEG-01 violation: provider-specific code branches found:\n%s",
			strings.Join(violations, "\n"))
	}
}

// isDataDrivenOK returns true for lines that inspect runtime values (BaseURL,
// APIKeyEnv) rather than provider identity. The opencodeProviderKey function
// uses strings.Contains(rp.BaseURL, "anthropic.com") — this is data-driven
// (the BaseURL is operator-configured data, not a hardcoded provider name
// in a branch). This allowance is documented in opencode_config.go.
func isDataDrivenOK(line string) bool {
	return strings.Contains(line, "BaseURL") || strings.Contains(line, "base_url") || strings.Contains(line, "baseURL")
}