package role

import (
	"strings"
	"testing"
)

func TestFormatKnowledgeAndRules(t *testing.T) {
	knowledge := []TeamKnowledgeEntry{
		{Topic: "coding-standards", Content: "Use tabs not spaces."},
		{Topic: "api-conventions", Content: "REST endpoints plural."},
	}
	rules := []RuleEntry{
		{StageID: "2.3", RuleText: "Always include error codes in responses."},
		{StageID: "", RuleText: "Never return null for collections."},
	}

	result := FormatKnowledgeAndRules(knowledge, rules)

	if !strings.Contains(result, "## Team Knowledge") {
		t.Error("missing Team Knowledge section")
	}
	if !strings.Contains(result, "coding-standards") {
		t.Error("missing coding-standards topic")
	}
	if !strings.Contains(result, "## Learned Rules") {
		t.Error("missing Learned Rules section")
	}
	if !strings.Contains(result, "[2.3]") {
		t.Error("missing stage ID in rules")
	}
	if !strings.Contains(result, "Never return null") {
		t.Error("missing global rule")
	}
}

func TestFormatKnowledgeAndRulesEmpty(t *testing.T) {
	result := FormatKnowledgeAndRules(nil, nil)
	if result != "" {
		t.Errorf("expected empty string for nil input, got %s", result)
	}
}

func TestFormatKnowledgeAndRulesKnowledgeOnly(t *testing.T) {
	knowledge := []TeamKnowledgeEntry{
		{Topic: "style", Content: "Be concise."},
	}
	result := FormatKnowledgeAndRules(knowledge, nil)
	if !strings.Contains(result, "## Team Knowledge") {
		t.Error("missing Team Knowledge section")
	}
	if strings.Contains(result, "## Learned Rules") {
		t.Error("should not have Learned Rules section")
	}
}

func TestFormatKnowledgeAndRulesRulesOnly(t *testing.T) {
	rules := []RuleEntry{
		{StageID: "3.5", RuleText: "Test before marking done."},
	}
	result := FormatKnowledgeAndRules(nil, rules)
	if strings.Contains(result, "## Team Knowledge") {
		t.Error("should not have Team Knowledge section")
	}
	if !strings.Contains(result, "## Learned Rules") {
		t.Error("missing Learned Rules section")
	}
}