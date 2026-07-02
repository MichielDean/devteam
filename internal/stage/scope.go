package stage

import (
	"strings"
)

// Scope names (AIDLC v2 9 scopes).
const (
	ScopeEnterprise     = "enterprise"
	ScopeFeature        = "feature"
	ScopeMVP            = "mvp"
	ScopePOC            = "poc"
	ScopeBugfix         = "bugfix"
	ScopeRefactor       = "refactor"
	ScopeInfra          = "infra"
	ScopeSecurityPatch  = "security-patch"
	ScopeWorkshop       = "workshop"
)

// ScopeInfo describes a scope.
type ScopeInfo struct {
	Name            string
	Stages          int   // number of stages executed
	DefaultDepth    string
	DefaultTestStr  string
	UseCase         string
}

// AllScopes returns the 9 scopes with metadata.
func AllScopes() []ScopeInfo {
	return []ScopeInfo{
		{ScopeEnterprise, 32, DepthComprehensive, TestStrategyComprehensive, "Regulated enterprise feature, full audit trail"},
		{ScopeFeature, 32, DepthStandard, TestStrategyStandard, "Default for new features"},
		{ScopeMVP, 22, DepthStandard, TestStrategyStandard, "Greenfield, skip late operations"},
		{ScopePOC, 8, DepthMinimal, TestStrategyMinimal, "Prove feasibility fast"},
		{ScopeBugfix, 7, DepthMinimal, TestStrategyMinimal, "Fix a specific bug"},
		{ScopeRefactor, 8, DepthMinimal, TestStrategyMinimal, "Clean up existing code"},
		{ScopeInfra, 13, DepthStandard, TestStrategyStandard, "Infrastructure change"},
		{ScopeSecurityPatch, 9, DepthMinimal, TestStrategyMinimal, "CVE response"},
		{ScopeWorkshop, 25, DepthStandard, TestStrategyMinimal, "AI-DLC workshop or training session"},
	}
}

// IsValidScope checks if a string is a valid scope name.
func IsValidScope(s string) bool {
	for _, sc := range AllScopes() {
		if sc.Name == s {
			return true
		}
	}
	return false
}

// GetScopeInfo returns metadata for a scope, or nil if invalid.
func GetScopeInfo(name string) *ScopeInfo {
	for _, sc := range AllScopes() {
		if sc.Name == name {
			return &sc
		}
	}
	return nil
}

// DetectScope auto-detects the appropriate scope from freeform intent text.
// Returns the detected scope and whether auto-detection was confident.
// If no keyword matches, defaults to "feature".
func DetectScope(intent string) (scope string, confident bool) {
	lower := strings.ToLower(intent)

	// Order matters: most specific first.
	keywordScopes := []struct {
		keywords []string
		scope    string
	}{
		{[]string{"cve", "vulnerability", "security patch", "security-patch"}, ScopeSecurityPatch},
		{[]string{"proof of concept", "prototype", "poc", "spike"}, ScopePOC},
		{[]string{"mvp", "minimum viable"}, ScopeMVP},
		{[]string{"workshop", "lab", "training"}, ScopeWorkshop},
		{[]string{"infrastructure", "deploy", "infra"}, ScopeInfra},
		{[]string{"refactor", "clean up", "simplify", "restructure"}, ScopeRefactor},
		{[]string{"fix", "bug", "broken", "error", "crash", "panic"}, ScopeBugfix},
	}

	wordCount := len(strings.Fields(intent))

	for _, ks := range keywordScopes {
		for _, kw := range ks.keywords {
			if strings.Contains(lower, kw) {
				// Disambiguation: if intent contains a scope keyword AND is >=5 words,
				// default to "feature" unless the keyword is very specific.
				if wordCount >= 5 {
					// Specific keywords that override the disambiguation rule
					if ks.scope == ScopeSecurityPatch || ks.scope == ScopePOC || ks.scope == ScopeMVP || ks.scope == ScopeWorkshop {
						return ks.scope, true
					}
					return ScopeFeature, false
				}
				return ks.scope, true
			}
		}
	}

	return ScopeFeature, false
}