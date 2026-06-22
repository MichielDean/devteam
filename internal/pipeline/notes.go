package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MichielDean/devteam/internal/feature"
)

// NotesManager handles per-phase notes that get passed to subsequent phases.
// Inspired by Cistern's cataractae_notes — stages write notes, next stage
// reads filtered subset. This decouples timing, survives crashes, and is auditable.
type NotesManager struct {
	specProvider specProvider
}

type specProvider interface {
	FeatureDirFromFeature(f *feature.Feature) string
}

// PhaseNote is a single note written by a phase.
type PhaseNote struct {
	Phase     string
	Role      string
	Timestamp time.Time
	Content   string
	Type      string // "summary", "finding", "warning", "handoff"
}

// AddNote appends a note to the phase's NOTES.md file.
func (nm *NotesManager) AddNote(f *feature.Feature, note PhaseNote) error {
	notesPath := filepath.Join(nm.specProvider.FeatureDirFromFeature(f), "NOTES.md")

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n## [%s] %s — %s\n", note.Timestamp.Format(time.RFC3339), note.Phase, note.Role))
	if note.Type != "" {
		b.WriteString(fmt.Sprintf("**Type**: %s\n\n", note.Type))
	}
	b.WriteString(note.Content)
	b.WriteString("\n")

	// Append to existing file
	file, err := os.OpenFile(notesPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening NOTES.md: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(b.String())
	return err
}

// BuildNotesContext returns a markdown section containing notes from prior phases.
// This gets appended to CONTEXT.md so the current agent can see what previous
// phases found, decided, and flagged.
func (nm *NotesManager) BuildNotesContext(f *feature.Feature, currentPhase feature.Phase) string {
	notesPath := filepath.Join(nm.specProvider.FeatureDirFromFeature(f), "NOTES.md")
	data, err := os.ReadFile(notesPath)
	if err != nil {
		return "" // No notes yet — fine
	}

	content := string(data)
	if strings.TrimSpace(content) == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n---\n\n# Phase Notes (from prior phases)\n\n")
	b.WriteString("Previous phases recorded the following notes. Use these to understand what was decided, what was found, and what to watch for:\n\n")
	b.WriteString(content)
	return b.String()
}

// BuildGateFailureNotes returns structured notes about the most recent gate failure.
// This is the Cistern "revision cycle" pattern — when a phase fails and recirculates,
// the failing gate's findings are passed as notes to the next run.
func (nm *NotesManager) BuildGateFailureNotes(f *feature.Feature, phase feature.Phase, gateResult *feature.GateResult) string {
	if gateResult == nil || gateResult.Passed {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n---\n\n# Gate Failure (Previous Attempt)\n\n")
	b.WriteString(fmt.Sprintf("**Phase**: %s\n\n", phase))
	b.WriteString("## Failed Checks\n\n")

	for _, check := range gateResult.Checks {
		if !check.Passed {
			b.WriteString(fmt.Sprintf("- **FAIL**: %s\n", check.Name))
			if check.Message != "" {
				b.WriteString(fmt.Sprintf("  %s\n", check.Message))
			}
			b.WriteString("\n")
		}
	}

	if len(gateResult.MissingArts) > 0 {
		b.WriteString("## Missing Artifacts\n\n")
		for _, art := range gateResult.MissingArts {
			b.WriteString(fmt.Sprintf("- %s\n", art))
		}
	}

	b.WriteString("\n## Instructions for Re-run\n\n")
	b.WriteString("The previous run of this phase failed the quality gate. Fix the issues above.\n")
	b.WriteString("Do NOT just re-create the same artifacts — address the specific failures.\n")

	return b.String()
}