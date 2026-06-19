package intake

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/spec"
)

type LooseIdeaIntake struct {
	specWriter *spec.SpecWriter
	specDir    string
}

func NewLooseIdeaIntake(baseDir string) *LooseIdeaIntake {
	return &LooseIdeaIntake{
		specWriter: spec.NewSpecWriter(baseDir),
		specDir:    filepath.Join(baseDir, "specs"),
	}
}

func (li *LooseIdeaIntake) Submit(title string, description string, priority int, repos []feature.RepoRef) (*feature.Feature, error) {
	id := generateFeatureID(title)
	f := feature.NewFeature(id, title, priority, feature.IntakeLooseIdea)
	f.Repos = repos

	if err := li.specWriter.CreateFeatureDir(f.ID); err != nil {
		return nil, fmt.Errorf("creating feature directory: %w", err)
	}

	specContent := li.generateSpecContent(f, description)
	if err := li.specWriter.WriteArtifact(f.ID, feature.ArtifactSpecMD, []byte(specContent)); err != nil {
		return nil, fmt.Errorf("writing spec.md: %w", err)
	}

	acceptanceContent := li.generateAcceptanceContent(f, description)
	if err := li.specWriter.WriteArtifact(f.ID, feature.ArtifactAcceptanceMD, []byte(acceptanceContent)); err != nil {
		return nil, fmt.Errorf("writing acceptance.md: %w", err)
	}

	reposContent := li.generateReposContent(f, repos)
	if err := li.specWriter.WriteArtifact(f.ID, feature.ArtifactReposYAML, []byte(reposContent)); err != nil {
		return nil, fmt.Errorf("writing repos.yaml: %w", err)
	}

	f.Status = feature.StatusInProgress
	f.PhaseStates[feature.PhaseInception].Status = feature.StatusInProgress
	now := time.Now()
	f.PhaseStates[feature.PhaseInception].StartedAt = &now

	provider := spec.NewSpecProvider(filepath.Dir(li.specDir))
	if err := provider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}

	return f, nil
}

func (li *LooseIdeaIntake) generateSpecContent(f *feature.Feature, description string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Feature Specification: %s\n\n", f.Title))
	b.WriteString(fmt.Sprintf("**Feature Branch**: `%s`\n\n", f.ID))
	b.WriteString(fmt.Sprintf("**Created**: %s\n\n", time.Now().Format("2006-01-02")))
	b.WriteString("**Status**: Draft\n\n")
	b.WriteString(fmt.Sprintf("**Input**: Loose idea: \"%s\"\n\n", description))
	b.WriteString("## User Scenarios & Testing *(mandatory)*\n\n")
	b.WriteString(fmt.Sprintf("### User Story 1 - [Title] (Priority: P1)\n\n"))
	b.WriteString(fmt.Sprintf("[To be refined from: \"%s\"]\n\n", description))
	b.WriteString("## Requirements *(mandatory)*\n\n")
	b.WriteString("### Functional Requirements\n\n")
	b.WriteString("- **FR-001**: [To be refined from loose idea]\n\n")
	b.WriteString("## Success Criteria *(mandatory)*\n\n")
	b.WriteString("### Measurable Outcomes\n\n")
	b.WriteString("- **SC-001**: [To be refined from loose idea]\n\n")
	b.WriteString("## Assumptions\n\n")
	b.WriteString("- [To be refined during PM exploration]\n")
	return b.String()
}

func (li *LooseIdeaIntake) generateAcceptanceContent(f *feature.Feature, description string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Acceptance Criteria: %s\n\n", f.Title))
	b.WriteString(fmt.Sprintf("**Spec**: %s\n", f.ID))
	b.WriteString(fmt.Sprintf("**Created**: %s\n\n", time.Now().Format("2006-01-02")))
	b.WriteString("## Acceptance Criteria\n\n")
	b.WriteString("- **AC-001**: [To be refined during PM exploration]\n")
	return b.String()
}

func (li *LooseIdeaIntake) generateReposContent(f *feature.Feature, repos []feature.RepoRef) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("feature: %s\n", f.ID))
	b.WriteString("repos:\n")
	if len(repos) == 0 {
		b.WriteString("  - name: devteam\n")
		b.WriteString("    branch: feature/" + f.ID + "\n")
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

func generateFeatureID(title string) string {
	s := strings.ToLower(title)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			s = strings.ReplaceAll(s, string(r), "")
		}
	}
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}