package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MichielDean/devteam/internal/feature"
)

// Outcome is the result of a phase execution, decided by the agent (Cistern pattern).
// The agent writes an outcome.txt file in the spec directory.
type Outcome string

const (
	OutcomePass          Outcome = "pass"
	OutcomeRecirculate   Outcome = "recirculate"
	OutcomeFailed        Outcome = "failed"
	OutcomeNeedsFeedback Outcome = "needs_feedback"
)

// ParsedOutcome is the parsed result from the agent's outcome file.
type ParsedOutcome struct {
	Result  Outcome
	Target  string // for recirculate: which phase to send back to
	Notes   string // agent's notes about why
	HasFile bool   // true if outcome.txt was found
}

// ParseOutcome reads the outcome.txt file from the spec directory and parses it.
// Format:
//   pass
//   recirculate:construction
//   recirculate:construction
//   Missing error handling in handler.go:42
//   recirculate:inception
//   Spec doesn't define what happens when user is not authenticated
//   pool
//   Blocked on external dependency
func (p *Pipeline) ParseOutcome(f *feature.Feature, phase feature.Phase) ParsedOutcome {
	// Read from the worktree only (agent's CWD)
	outcomePath := filepath.Join(p.specProvider.FeatureDirFromFeature(f), "outcome.txt")
	data, err := os.ReadFile(outcomePath)
	if err != nil {
		return ParsedOutcome{Result: OutcomePass, HasFile: false}
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return ParsedOutcome{Result: OutcomePass, HasFile: false}
	}

	lines := strings.Split(content, "\n")
	firstLine := strings.TrimSpace(lines[0])

	po := ParsedOutcome{HasFile: true}

	// Parse first line: pass, recirculate, recirculate:target, or pool
	if firstLine == "pass" {
		po.Result = OutcomePass
	} else if firstLine == "failed" {
		po.Result = OutcomeFailed
	} else if firstLine == "needs_feedback" {
		po.Result = OutcomeNeedsFeedback
	} else if strings.HasPrefix(firstLine, "recirculate") {
		po.Result = OutcomeRecirculate
		// Check for target: recirculate:construction
		if idx := strings.Index(firstLine, ":"); idx >= 0 {
			po.Target = strings.TrimSpace(firstLine[idx+1:])
		}
	} else {
		// Unknown outcome — default to pass (agent didn't signal properly)
		po.Result = OutcomePass
	}

	// Rest of the lines are notes
	if len(lines) > 1 {
		po.Notes = strings.TrimSpace(strings.Join(lines[1:], "\n"))
	}

	return po
}

// DeleteOutcome removes the outcome file from the worktree.
func (p *Pipeline) DeleteOutcome(f *feature.Feature) {
	outcomePath := filepath.Join(p.specProvider.FeatureDirFromFeature(f), "outcome.txt")
	os.Remove(outcomePath)
}

// ResolveRecirculateTarget determines which phase to recirculate to.
// If the agent specified a target (recirculate:construction), use that.
// Otherwise, use the default routing: each phase has a default recirculate target.
func ResolveRecirculateTarget(phase feature.Phase, explicitTarget string) feature.Phase {
	if explicitTarget != "" {
		// Validate the target is a valid phase
		p := feature.ParsePhase(explicitTarget)
		if p != "" {
			return p
		}
	}

	// Default recirculate routing (Cistern OnRecirculate pattern)
	switch phase {
	case feature.PhaseReview:
		return feature.PhaseConstruction // review sends back to construction
	case feature.PhaseTesting:
		return feature.PhaseConstruction // testing sends back to construction
	case feature.PhaseDelivery:
		return feature.PhaseConstruction // delivery sends back to construction
	case feature.PhasePlanning:
		return feature.PhaseInception // planning sends back to inception (spec gaps)
	case feature.PhaseConstruction:
		return feature.PhasePlanning // construction sends back to planning (design gaps)
	default:
		return phase // fallback: re-run current (shouldn't happen)
	}
}

// writeOutcomeContext adds the outcome instructions to the phase instruction.
// This tells the agent HOW to signal pass/recirculate.
func outcomeInstructions(phase feature.Phase) string {
	// Determine what this phase can recirculate to
	var recirculateTarget string
	switch phase {
	case feature.PhaseReview:
		recirculateTarget = "construction"
	case feature.PhaseTesting:
		recirculateTarget = "construction"
	case feature.PhaseDelivery:
		recirculateTarget = "construction"
	case feature.PhasePlanning:
		recirculateTarget = "inception"
	case feature.PhaseConstruction:
		recirculateTarget = "planning"
	default:
		recirculateTarget = ""
	}

	var b strings.Builder
	b.WriteString("\n\n---\n\n## Outcome Signal (MANDATORY)\n\n")
	b.WriteString("After completing your work, write a file called `outcome.txt` in the spec directory (`specs/<feature-id>/outcome.txt`).\n\n")
	b.WriteString("The FIRST line must be one of:\n")
	b.WriteString("- `pass` — your work is complete and verified\n")
	if recirculateTarget != "" {
		b.WriteString(fmt.Sprintf("- `recirculate:%s` — you found issues that need to be fixed by the %s phase\n", recirculateTarget, recirculateTarget))
	}
	b.WriteString("- `needs_feedback` — you have written questions.json and need the user to answer them\n")
	b.WriteString("- `failed` — you are blocked and cannot proceed\n\n")

	if recirculateTarget != "" {
		b.WriteString(fmt.Sprintf("When recirculating to %s, write the reason on subsequent lines:\n", recirculateTarget))
		b.WriteString("```\n")
		b.WriteString(fmt.Sprintf("recirculate:%s\n", recirculateTarget))
		b.WriteString("Missing error handling in handler.go:42 — returns 500 instead of 400 for invalid input\n")
		b.WriteString("Null pointer in FeatureList.tsx when features array is empty\n")
		b.WriteString("```\n\n")
		b.WriteString(fmt.Sprintf("These notes will be passed to the %s agent so they know exactly what to fix.\n", recirculateTarget))
	} else {
		b.WriteString("Write `pass` when your work is complete. Nothing else needed.\n")
	}

	b.WriteString("\nThe pipeline reads this file to decide what to do next. If you don't write it, the pipeline will assume `pass`.\n")

	return b.String()
}

// formatGateFailureAsNotes converts a gate result into notes for recirculation.
// Used when the agent didn't write an outcome file and the gate failed.
func formatGateFailureAsNotes(gr *feature.GateResult) string {
	if gr == nil || gr.Passed {
		return ""
	}
	var b strings.Builder
	b.WriteString("Gate safety check failed:\n")
	for _, check := range gr.Checks {
		if !check.Passed {
			b.WriteString(fmt.Sprintf("- %s: %s\n", check.Name, check.Message))
		}
	}
	if len(gr.MissingArts) > 0 {
		b.WriteString("Missing artifacts:\n")
		for _, art := range gr.MissingArts {
			b.WriteString(fmt.Sprintf("- %s\n", art))
		}
	}
	return b.String()
}

// writeRecirculationNotes writes notes for the target phase so the agent
// knows what to fix (Cistern revision cycle pattern).
func (p *Pipeline) writeRecirculationNotes(f *feature.Feature, fromPhase, toPhase feature.Phase, notes string) error {
	notesPath := filepath.Join(p.specProvider.FeatureDirFromFeature(f), "REVISION_NOTES.md")

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Revision Required: %s → %s\n\n", fromPhase, toPhase))
	b.WriteString(fmt.Sprintf("The %s phase found issues that need to be fixed.\n\n", fromPhase))
	b.WriteString("## Issues Found\n\n")
	b.WriteString(notes)
	b.WriteString("\n\n## Instructions\n\n")
	b.WriteString(fmt.Sprintf("Address ALL issues above before proceeding with your normal %s work.\n", toPhase))
	b.WriteString("Do NOT skip or ignore any of these issues.\n")

	return os.WriteFile(notesPath, []byte(b.String()), 0644)
}