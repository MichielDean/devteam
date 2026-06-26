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

type ExternalSpecIntake struct {
	specWriter *spec.SpecWriter
	specDir    string
	baseDir    string
	provider   *spec.SpecProvider
}

func NewExternalSpecIntake(baseDir string) *ExternalSpecIntake {
	return &ExternalSpecIntake{
		specWriter: spec.NewSpecWriter(baseDir),
		specDir:    filepath.Join(baseDir, "specs"),
		baseDir:    baseDir,
		provider:   spec.NewSpecProvider(baseDir),
	}
}

// SetDatabase wires the database for artifact and state storage.
func (es *ExternalSpecIntake) SetDatabase(database *db.DB) {
	es.provider.SetDatabase(database)
	es.specWriter.SetProvider(es.provider)
}

type DecompositionResult struct {
	Features     []*feature.Feature
	Dependencies map[string][]string
}

func (es *ExternalSpecIntake) Submit(title string, documentContent string, priority int, repos []feature.RepoRef) (*DecompositionResult, error) {
	id := generateFeatureID(title)
	f := feature.NewFeature(id, title, priority, feature.IntakeExternalSpec)
	f.Repos = repos

	f.Start()
	if err := es.provider.SaveFeatureState(f); err != nil {
		return nil, fmt.Errorf("saving feature state: %w", err)
	}

	inputContent := es.generateInputFromExternal(f, documentContent)
	if err := es.specWriter.WriteArtifact(f.ID, feature.ArtifactInputMD, []byte(inputContent)); err != nil {
		return nil, fmt.Errorf("writing input.md: %w", err)
	}

	if len(repos) > 0 {
		reposContent := es.generateReposContent(f, repos)
		if err := es.specWriter.WriteArtifact(f.ID, feature.ArtifactReposYAML, []byte(reposContent)); err != nil {
			return nil, fmt.Errorf("writing repos.yaml: %w", err)
		}
	}

	result := &DecompositionResult{
		Features:     []*feature.Feature{f},
		Dependencies: map[string][]string{},
	}

	return result, nil
}

func (es *ExternalSpecIntake) generateInputFromExternal(f *feature.Feature, content string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Feature Input: %s\n\n", f.Title))
	b.WriteString(fmt.Sprintf("**Feature ID**: %s\n", f.ID))
	b.WriteString(fmt.Sprintf("**Created**: %s\n", time.Now().Format("2006-01-02")))
	b.WriteString(fmt.Sprintf("**Intake Path**: External Specification\n"))
	b.WriteString(fmt.Sprintf("**Priority**: P%d\n\n", f.Priority))
	b.WriteString("## Source Document\n\n")

	sections := parseSections(content)
	if len(sections) > 0 {
		b.WriteString("Detected sections:\n")
		for _, section := range sections {
			b.WriteString(fmt.Sprintf("- %s\n", section))
		}
		b.WriteString("\n")
	}

	b.WriteString("### Full Document\n\n")
	b.WriteString(content)
	b.WriteString("\n\n---\n\n")
	b.WriteString("This feature was submitted as an external specification. The PM role will decompose it into:\n")
	b.WriteString("- `spec.md` with user stories and requirements\n")
	b.WriteString("- `acceptance.md` with verifiable acceptance criteria\n")
	b.WriteString("- `repos.yaml` identifying affected repositories\n\n")
	b.WriteString(fmt.Sprintf("Run `devteam run %s` to start the inception phase and let the PM produce these artifacts.\n", f.ID))
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

func parseSections(content string) []string {
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