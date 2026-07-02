package intake

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/spec"
)

type LooseIdeaIntake struct {
	specWriter *spec.SpecWriter
	specDir   string
	baseDir   string
	provider  *spec.SpecProvider
}

func NewLooseIdeaIntake(baseDir string) *LooseIdeaIntake {
	return &LooseIdeaIntake{
		specWriter: spec.NewSpecWriter(baseDir),
		specDir:    filepath.Join(baseDir, "specs"),
		baseDir:    baseDir,
		provider:   spec.NewSpecProvider(baseDir),
	}
}

// SetDatabase wires the database for artifact and state storage.
func (li *LooseIdeaIntake) SetDatabase(database *db.DB) {
	li.provider.SetDatabase(database)
	li.specWriter.SetProvider(li.provider)
}

func (li *LooseIdeaIntake) Submit(title string, description string, priority int, repos []feature.RepoRef) (*feature.Feature, error) {
	id := generateFeatureID(title)
	f := feature.NewFeature(id, title, priority, feature.IntakeLooseIdea)
	f.Repos = repos

	f.Status = feature.StatusInProgress
	f.UpdatedAt = time.Now()
	if err := li.provider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}

	inputContent := li.generateInputContent(f, description)
	if err := li.specWriter.WriteArtifact(f.ID, feature.ArtifactInputMD, []byte(inputContent)); err != nil {
		return nil, fmt.Errorf("writing input.md: %w", err)
	}

	if len(repos) > 0 {
		reposContent := li.generateReposContent(f, repos)
		if err := li.specWriter.WriteArtifact(f.ID, feature.ArtifactReposYAML, []byte(reposContent)); err != nil {
			return nil, fmt.Errorf("writing repos.yaml: %w", err)
		}
	}

	return f, nil
}

func (li *LooseIdeaIntake) generateInputContent(f *feature.Feature, description string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Feature Input: %s\n\n", f.Title))
	b.WriteString(fmt.Sprintf("**Feature ID**: %s\n", f.ID))
	b.WriteString(fmt.Sprintf("**Created**: %s\n", time.Now().Format("2006-01-02")))
	b.WriteString(fmt.Sprintf("**Intake Path**: Loose Idea\n"))
	b.WriteString(fmt.Sprintf("**Priority**: P%d\n\n", f.Priority))
	b.WriteString("## Idea\n\n")
	b.WriteString(description)
	b.WriteString("\n\n---\n\n")
	b.WriteString("This feature was submitted as a loose idea. The PM role will explore, clarify, and refine this into a structured specification with:\n")
	b.WriteString("- `spec.md` with user stories and requirements\n")
	b.WriteString("- `acceptance.md` with verifiable acceptance criteria\n")
	b.WriteString("- `repos.yaml` identifying affected repositories\n\n")
	b.WriteString(fmt.Sprintf("Run `devteam run %s` to start the inception phase and let the PM produce these artifacts.\n", f.ID))
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