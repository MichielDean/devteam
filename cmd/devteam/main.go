package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/intake"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/spec"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	baseDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting working directory: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(filepath.Join(baseDir, "devteam.yaml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version":
		fmt.Printf("devteam %s\n", version)
	case "status":
		handleStatus(baseDir)
	case "intake":
		handleIntake(baseDir)
	case "run":
		handleRun(baseDir, cfg)
	case "gate":
		handleGate(baseDir, cfg)
	case "bootstrap":
		handleBootstrap(baseDir, cfg)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func handleStatus(baseDir string) {
	provider := spec.NewSpecProvider(baseDir)
	features, err := provider.ListFeatures()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing features: %v\n", err)
		os.Exit(1)
	}
	if len(features) == 0 {
		fmt.Println("No features in progress.")
		fmt.Println("\nCreate a feature with: devteam intake --type loose --text \"your idea\"")
		return
	}
	fmt.Println("Dev Team Status:")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("  %-35s %-12s %-8s %s\n", "ID", "Phase", "Priority", "Status")
	fmt.Println(strings.Repeat("-", 70))
	for _, f := range features {
		phase := f.CurrentPhase()
		fmt.Printf("  %-35s %-12s %-8d %s\n", f.ID, phase, f.Priority, f.Status)
	}
}

func handleIntake(baseDir string) {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: devteam intake --type loose --text \"idea\"\n")
		fmt.Fprintf(os.Stderr, "       devteam intake --type external --file path/to/prd.md\n")
		os.Exit(1)
	}

	intakeType, title, priority := parseIntakeArgs()

	switch intakeType {
	case "loose":
		li := intake.NewLooseIdeaIntake(baseDir)
		f, err := li.Submit(title, title, priority, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error submitting loose idea: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Feature created: %s\n", f.ID)
		fmt.Printf("Spec directory: %s\n", filepath.Join("specs", f.ID))
		fmt.Printf("Phase: %s\n", f.CurrentPhase())
		fmt.Printf("Intake path: loose_idea\n")
	case "external":
		ei := intake.NewExternalSpecIntake(baseDir)
		result, err := ei.Submit(title, "External specification", priority, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error submitting external spec: %v\n", err)
			os.Exit(1)
		}
		for _, f := range result.Features {
			fmt.Printf("Feature created: %s\n", f.ID)
			fmt.Printf("Spec directory: %s\n", filepath.Join("specs", f.ID))
			fmt.Printf("Intake path: external_spec\n")
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown intake type: %s (use 'loose' or 'external')\n", intakeType)
		os.Exit(1)
	}
}

func handleRun(baseDir string, cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: devteam run <feature-id>\n")
		os.Exit(1)
	}
	featureID := os.Args[2]

	provider := spec.NewSpecProvider(baseDir)
	f, err := provider.LoadFeatureState(featureID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading feature %s: %v\n", featureID, err)
		os.Exit(1)
	}

	currentPhase := f.CurrentPhase()
	fmt.Printf("Running phase %s for feature %s...\n", currentPhase, featureID)

	p := pipeline.NewPipeline(cfg, provider)
	ps, err := p.RunPhase(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error running phase: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Phase %s started for %s\n", ps.Phase, featureID)
	fmt.Printf("Status: %s\n", ps.Status)

	if err := p.SaveFeature(f); err != nil {
		fmt.Fprintf(os.Stderr, "error saving feature state: %v\n", err)
		os.Exit(1)
	}
}

func handleGate(baseDir string, cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: devteam gate <feature-id>\n")
		os.Exit(1)
	}
	featureID := os.Args[2]

	provider := spec.NewSpecProvider(baseDir)
	f, err := provider.LoadFeatureState(featureID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading feature %s: %v\n", featureID, err)
		os.Exit(1)
	}

	p := pipeline.NewPipeline(cfg, provider)
	result, err := p.EvaluateGate(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error evaluating gate: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Gate evaluation for %s (phase: %s):\n", featureID, result.Phase)
	fmt.Printf("  Passed: %v\n", result.Passed)
	if len(result.MissingArts) > 0 {
		fmt.Println("  Missing artifacts:")
		for _, art := range result.MissingArts {
			fmt.Printf("    - %s\n", art)
		}
	}
	if len(result.Checks) > 0 {
		fmt.Println("  Checks:")
		for _, check := range result.Checks {
			status := "PASS"
			if !check.Passed {
				status = "FAIL"
			}
			fmt.Printf("    [%s] %s\n", status, check.Name)
			if check.Message != "" {
				fmt.Printf("           %s\n", check.Message)
			}
		}
	}

	if !result.Passed {
		fmt.Println("\n  To fix: provide the missing artifacts listed above.")
		fmt.Println("  Run 'devteam run <feature-id>' to execute the next phase.")
		os.Exit(1)
	}

	fmt.Println("\n  Gate passed! Run 'devteam run <feature-id>' to advance to the next phase.")
}

func handleBootstrap(baseDir string, cfg *config.Config) {
	featureID := "001-dev-team-platform"
	provider := spec.NewSpecProvider(baseDir)

	f, err := provider.LoadFeatureState(featureID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading feature %s: %v\n", featureID, err)
		fmt.Fprintf(os.Stderr, "Make sure spec 001 exists in %s\n", filepath.Join("specs", featureID))
		os.Exit(1)
	}

	fmt.Println("Dev Team Self-Bootstrap")
	fmt.Println("======================")
	fmt.Printf("Feature: %s\n", f.ID)
	fmt.Printf("Title: %s\n", f.Title)
	fmt.Printf("Current phase: %s\n", f.CurrentPhase())
	fmt.Printf("Status: %s\n\n", f.Status)

	p := pipeline.NewPipeline(cfg, provider)
	result, err := p.EvaluateGate(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error evaluating gate: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Gate: %s\n", result.Phase)
	fmt.Printf("Passed: %v\n", result.Passed)
	if len(result.MissingArts) > 0 {
		fmt.Println("Missing artifacts:")
		for _, art := range result.MissingArts {
			fmt.Printf("  - %s\n", art)
		}
	}
}

func parseIntakeArgs() (string, string, int) {
	intakeType := "loose"
	title := ""
	priority := 2

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--type":
			if i+1 < len(os.Args) {
				intakeType = os.Args[i+1]
				i++
			}
		case "--text":
			if i+1 < len(os.Args) {
				title = os.Args[i+1]
				i++
			}
		case "--file":
			if i+1 < len(os.Args) {
				title = filepath.Base(os.Args[i+1])
				i++
			}
		case "--priority":
			if i+1 < len(os.Args) {
				fmt.Sscanf(os.Args[i+1], "%d", &priority)
				i++
			}
		}
	}

	if title == "" {
		title = "untitled-feature"
	}

	return intakeType, title, priority
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "devteam %s - multi-agent development platform\n\n", version)
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  devteam <command> [args]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  intake     Submit a new feature (loose idea or external spec)\n")
	fmt.Fprintf(os.Stderr, "  run        Run the next pipeline phase for a feature\n")
	fmt.Fprintf(os.Stderr, "  gate       Evaluate the current phase gate for a feature\n")
	fmt.Fprintf(os.Stderr, "  status     Show current pipeline status for all features\n")
	fmt.Fprintf(os.Stderr, "  bootstrap   Self-bootstrap: process spec 001 through the pipeline\n")
	fmt.Fprintf(os.Stderr, "  version    Print version\n\n")
	fmt.Fprintf(os.Stderr, "Intake options:\n")
	fmt.Fprintf(os.Stderr, "  --type loose|external   Intake path type\n")
	fmt.Fprintf(os.Stderr, "  --text \"idea\"           Loose idea text\n")
	fmt.Fprintf(os.Stderr, "  --file path             Path to external spec\n")
	fmt.Fprintf(os.Stderr, "  --priority 1|2|3       Feature priority\n")
}
