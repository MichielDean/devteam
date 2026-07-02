package role

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type RoleDefinition struct {
	Name         string `yaml:"name" json:"name"`
	Description  string `yaml:"description" json:"description"`
	Instructions string `yaml:"instructions" json:"instructions"`
	PhaseRules   string `yaml:"phase_rules" json:"phase_rules"`
	ModelTier    string `yaml:"model_tier" json:"model_tier"` // "opus", "sonnet", ""
	IsReviewer   bool   `yaml:"is_reviewer" json:"is_reviewer"`
}

// agentRoster maps agent slug → model tier. Reviewers marked separately.
var agentRoster = map[string]struct {
	tier      string
	reviewer  bool
}{
	"product":              {"opus", false},
	"design":               {"opus", false},
	"delivery":             {"sonnet", false},
	"architect":            {"opus", false},
	"platform":             {"opus", false},
	"devsecops":            {"opus", false},
	"developer":            {"opus", false},
	"quality":              {"opus", false},
	"pipeline-deploy":      {"sonnet", false},
	"operations":           {"sonnet", false},
	"product-lead":         {"sonnet", true},
	"architecture-reviewer": {"sonnet", true},
}

func AgentRoster() map[string]struct {
	tier     string
	reviewer bool
} {
	return agentRoster
}

type RoleLoader struct {
	baseDir string
}

func NewRoleLoader(baseDir string) *RoleLoader {
	return &RoleLoader{baseDir: baseDir}
}

func (rl *RoleLoader) Load(roleName string) (*RoleDefinition, error) {
	roleDir := filepath.Join(rl.baseDir, "roles", roleName)
	instructionsPath := filepath.Join(roleDir, "INSTRUCTIONS.md")

	data, err := os.ReadFile(instructionsPath)
	if err != nil {
		return nil, fmt.Errorf("loading role %s: %w", roleName, err)
	}

	lines := strings.Split(string(data), "\n")
	description := ""
	if len(lines) > 0 {
		desc := strings.TrimLeft(lines[0], "# ")
		if desc != "" && desc != strings.TrimLeft(lines[0], "#") {
			description = desc
		}
	}
	if description == "" && len(lines) > 2 {
		for _, line := range lines[1:] {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				description = trimmed
				break
			}
		}
	}

	info, ok := agentRoster[roleName]
	tier := ""
	reviewer := false
	if ok {
		tier = info.tier
		reviewer = info.reviewer
	}

	return &RoleDefinition{
		Name:         roleName,
		Description:  description,
		Instructions: string(data),
		PhaseRules:   "",
		ModelTier:    tier,
		IsReviewer:   reviewer,
	}, nil
}

func (rl *RoleLoader) LoadAll() (map[string]*RoleDefinition, error) {
	roles := map[string]*RoleDefinition{}
	for roleName := range agentRoster {
		rd, err := rl.Load(roleName)
		if err != nil {
			return nil, fmt.Errorf("loading role %s: %w", roleName, err)
		}
		roles[roleName] = rd
	}
	return roles, nil
}

func (rl *RoleLoader) Validate() error {
	roles, err := rl.LoadAll()
	if err != nil {
		return err
	}
	for name := range agentRoster {
		if _, ok := roles[name]; !ok {
			return fmt.Errorf("missing required role: %s", name)
		}
	}
	return nil
}

// Agents returns the ordered list of non-reviewer agent names.
func Agents() []string {
	return []string{"product", "design", "delivery", "architect", "platform", "devsecops", "developer", "quality", "pipeline-deploy", "operations"}
}

// Reviewers returns the list of reviewer agent names.
func Reviewers() []string {
	return []string{"product-lead", "architecture-reviewer"}
}

// AllRoles returns all 12 agent names (10 agents + 2 reviewers).
func AllRoles() []string {
	return append(Agents(), Reviewers()...)
}