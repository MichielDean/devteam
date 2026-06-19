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

	return &RoleDefinition{
		Name:         roleName,
		Description:  description,
		Instructions: string(data),
		PhaseRules:   "", // Filled from config
	}, nil
}

func (rl *RoleLoader) LoadAll() (map[string]*RoleDefinition, error) {
	roles := map[string]*RoleDefinition{}
	for _, roleName := range []string{"pm", "architect", "developer", "reviewer", "tester", "ops"} {
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
	expected := []string{"pm", "architect", "developer", "reviewer", "tester", "ops"}
	for _, name := range expected {
		if _, ok := roles[name]; !ok {
			return fmt.Errorf("missing required role: %s", name)
		}
	}
	return nil
}
