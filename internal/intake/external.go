package intake

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/spec"
)

type ExternalSpecIntake struct {
	specWriter *spec.SpecWriter
	specDir    string
}

func NewExternalSpecIntake(baseDir string) *ExternalSpecIntake {
	return &ExternalSpecIntake{
		specWriter: spec.NewSpecWriter(baseDir),
		specDir:    filepath.Join(baseDir, "specs"),
	}
}

type DecompositionResult struct {
	Features     []*feature.Feature
	Dependencies map[string][]string
}

func (es *ExternalSpecIntake) Submit(title string, documentContent string, priority int, repos []feature.RepoRef) (*DecompositionResult, error) {
	id := generateFeatureID(title)
	f := feature.NewFeature(id, title, priority, feature.IntakeExternalSpec)
	f.Repos = repos

	if err := es.specWriter.CreateFeatureDir(f.ID); err != nil {
		return nil, fmt.Errorf("creating feature directory: %w", err)
	}

	specContent := es.generateSpecFromExternal(f, documentContent)
	if err := es.specWriter.WriteArtifact(f.ID, feature.ArtifactSpecMD, []byte(specContent)); err != nil {
		return nil, fmt.Errorf("writing spec.md: %w", err)
	}

	acceptanceContent := es.generateAcceptanceFromExternal(f, documentContent)
	if err := es.specWriter.WriteArtifact(f.ID, feature.ArtifactAcceptanceMD, []byte(acceptanceContent)); err != nil {
		return nil, fmt.Errorf("writing acceptance.md: %w", err)
	}

	reposContent := es.generateReposContent(f, repos)
	if err := es.specWriter.WriteArtifact(f.ID, feature.ArtifactReposYAML, []byte(reposContent)); err != nil {
		return nil, fmt.Errorf("writing repos.yaml: %w", err)
	}

	f.Status = feature.StatusInProgress
	f.PhaseStates[feature.PhaseInception].Status = feature.StatusInProgress
	now := time.Now()
	f.PhaseStates[feature.PhaseInception].StartedAt = &now

	provider := spec.NewSpecProvider(filepath.Dir(es.specDir))
	if err := provider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}

	result := &DecompositionResult{
		Features:     []*feature.Feature{f},
		Dependencies: map[string][]string{},
	}

	return result, nil
}

func (es *ExternalSpecIntake) generateSpecFromExternal(f *feature.Feature, content string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Feature Specification: %s\n\n", f.Title))
	b.WriteString(fmt.Sprintf("**Feature Branch**: `%s`\n\n", f.ID))
	b.WriteString(fmt.Sprintf("**Created**: %s\n\n", time.Now().Format("2006-01-02")))
	b.WriteString("**Status**: Draft\n\n")
	b.WriteString(fmt.Sprintf("**Input**: External specification (decomposed from roadmap)\n\n"))

	sections := es.parseSections(content)
	if len(sections) > 0 {
		b.WriteString("## Source Document Summary\n\n")
		limit := 5
		if len(sections) < limit {
			limit = len(sections)
		}
		for _, section := range sections[:limit] {
			b.WriteString(fmt.Sprintf("- %s\n", section))
		}
		b.WriteString("\n")
	}

	b.WriteString("## User Scenarios & Testing *(mandatory)*\n\n")
	b.WriteString("### User Story 1 - [To be refined from external spec] (Priority: P1)\n\n")
	b.WriteString("[To be refined from external specification]\n\n")
	b.WriteString("## Requirements *(mandatory)*\n\n")
	b.WriteString("### Functional Requirements\n\n")
	b.WriteString("- **FR-001**: [To be refined from external specification]\n\n")
	b.WriteString("## Success Criteria *(mandatory)*\n\n")
	b.WriteString("### Measurable Outcomes\n\n")
	b.WriteString("- **SC-001**: [To be refined from external specification]\n\n")
	b.WriteString("## Assumptions\n\n")
	b.WriteString("- [To be refined during PM decomposition]\n")
	return b.String()
}

func (es *ExternalSpecIntake) generateAcceptanceFromExternal(f *feature.Feature, content string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Acceptance Criteria: %s\n\n", f.Title))
	b.WriteString(fmt.Sprintf("**Spec**: %s\n", f.ID))
	b.WriteString(fmt.Sprintf("**Created**: %s\n\n", time.Now().Format("2006-01-02")))
	b.WriteString("## Acceptance Criteria\n\n")
	b.WriteString("- **AC-001**: [To be refined from external specification]\n")
	return b.String()
}

func (es *ExternalSpecIntake) generateReposContent(f *feature.Feature, repos []feature.RepoRef) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("feature: %s\n", f.ID))
	b.WriteString("repos:\n")
	if len(repos) == 0 {
		b.WriteString("  - name: devteam\n")
		b.WriteString(fmt.Sprintf("    branch: feature/%s\n", f.ID))
	} else {
		for _, repo := range repos {
			b.WriteString(fmt.Sprintf("  - name: %s\n", repo.Name))
			if repo.URL != "" {
				b.WriteString(fmt.Sprintf("    url: %s\n", repo.URL))
			}
			b.WriteString(fmt.Sprintf("    branch: feature/%s\n", f.ID))
		}
	}
	return b.String()
}

func (es *ExternalSpecIntake) parseSections(content string) []string {
	var sections []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			heading := strings.TrimLeft(trimmed, "# ")
			heading = strings.TrimSpace(heading)
			if heading != "" {
				sections = append(sections, heading)
			}
		}
	}
	return sections
}
