package main

//go:generate cd ../../ui && npm run build

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MichielDean/devteam/internal/api"
	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/feature"
	devinit "github.com/MichielDean/devteam/internal/init"
	"github.com/MichielDean/devteam/internal/intake"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/spec"
)

const version = "0.3.0"

func main() {
	// Parse -http flag for web server mode
	httpAddr := flag.String("http", "", "Start web server on specified address (e.g., :8080). If empty, runs in CLI mode.")
	flag.Parse()

	// If -http flag is set, start the web server
	if *httpAddr != "" {
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

		specProvider := spec.NewSpecProvider(baseDir)
		p := pipeline.NewPipeline(cfg, specProvider)
		questionStore := feature.NewFileQuestionStore(baseDir)

		// Serve frontend: use local filesystem (development or after go generate)
		var staticFS fs.FS
		uiDir := filepath.Join(baseDir, "ui", "dist")
		if info, err := os.Stat(uiDir); err == nil && info.IsDir() {
			staticFS = os.DirFS(uiDir)
		}
		// If ui/dist doesn't exist, staticFS is nil — API-only mode (no frontend)

		server := api.NewServer(*httpAddr, specProvider, p, staticFS, questionStore)

		fmt.Printf("Dev Team Web UI starting on %s\n", *httpAddr)
		if err := server.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	baseDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting working directory: %v\n", err)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version":
		fmt.Printf("devteam %s\n", version)
		return
	case "init":
		handleInit()
		return
	}

	cfg, err := config.LoadConfig(filepath.Join(baseDir, "devteam.yaml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "status":
		handleStatus(baseDir)
	case "intake":
		handleIntake(baseDir)
	case "run":
		handleRun(baseDir, cfg)
	case "process":
		handleProcess(baseDir, cfg)
	case "advance":
		handleAdvance(baseDir, cfg)
	case "gate":
		handleGate(baseDir, cfg)
	case "recirculate":
		handleRecirculate(baseDir, cfg)
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
	result, err := p.RunPhaseWithAgent(context.Background(), f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error running phase: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Phase %s completed for %s\n", result.Phase, featureID)
	fmt.Printf("Gate passed: %v\n", result.GateResult.Passed)

	if !result.GateResult.Passed {
		fmt.Println("\nMissing artifacts:")
		for _, art := range result.GateResult.MissingArts {
			fmt.Printf("  - %s\n", art)
		}
		fmt.Println("\nFailed checks:")
		for _, check := range result.GateResult.Checks {
			if !check.Passed {
				fmt.Printf("  [FAIL] %s\n", check.Name)
				if check.Message != "" {
					fmt.Printf("         %s\n", check.Message)
				}
			}
		}
		fmt.Println("\nTo fix: provide the missing artifacts and re-run the gate.")
		fmt.Println("Run 'devteam gate <feature-id>' to re-evaluate.")
		fmt.Println("Run 'devteam recirculate <feature-id> <target-phase>' to go back to a previous phase.")
	} else {
		fmt.Println("\nGate passed! Run 'devteam advance <feature-id>' to move to the next phase.")
	}

	for _, rr := range result.RoleResults {
		status := "SUCCESS"
		if !rr.Success {
			status = "FAILED"
		}
		fmt.Printf("\nRole %s (%s): %s (%v)\n", rr.Role, rr.Phase, status, rr.Duration)
		if rr.Error != "" {
			fmt.Printf("  Error: %s\n", rr.Error)
		}
	}
}

func handleAdvance(baseDir string, cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: devteam advance <feature-id>\n")
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

	gateResult, err := p.EvaluateGate(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error evaluating gate: %v\n", err)
		os.Exit(1)
	}

	if !gateResult.Passed {
		fmt.Printf("Cannot advance: gate for phase %s has not passed.\n", gateResult.Phase)
		fmt.Println("\nMissing artifacts:")
		for _, art := range gateResult.MissingArts {
			fmt.Printf("  - %s\n", art)
		}
		fmt.Println("\nFailed checks:")
		for _, check := range gateResult.Checks {
			if !check.Passed {
				fmt.Printf("  [FAIL] %s\n", check.Name)
			}
		}
		fmt.Println("\nFix the issues above, then re-run 'devteam advance <feature-id>'.")
		os.Exit(1)
	}

	currentPhase := f.CurrentPhase()
	phases := feature.AllPhases()
	currentIdx := -1
	for i, phase := range phases {
		if phase == currentPhase {
			currentIdx = i
			break
		}
	}

	if currentIdx == len(phases)-1 {
		f.MarkDone()
		if err := p.SaveFeature(f); err != nil {
			fmt.Fprintf(os.Stderr, "error saving feature state: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Feature %s completed! All phases passed.\n", featureID)
		return
	}

	f, err = p.AdvanceFeature(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error advancing feature: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Feature %s advanced to phase: %s\n", featureID, f.CurrentPhase())
	fmt.Println("Run 'devteam run <feature-id>' to execute the next phase.")
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
		fmt.Println("  Run 'devteam run <feature-id>' to re-execute the phase.")
		fmt.Println("  Run 'devteam recirculate <feature-id> <target-phase>' to go back to a previous phase.")
		os.Exit(1)
	}

	fmt.Println("\n  Gate passed! Run 'devteam advance <feature-id>' to move to the next phase.")
}

func handleRecirculate(baseDir string, cfg *config.Config) {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: devteam recirculate <feature-id> <target-phase>\n")
		fmt.Fprintf(os.Stderr, "Valid phases: inception, planning, construction, review, testing, delivery\n")
		os.Exit(1)
	}
	featureID := os.Args[2]
	targetPhaseStr := os.Args[3]

	targetPhase := feature.ParsePhase(targetPhaseStr)

	provider := spec.NewSpecProvider(baseDir)
	f, err := provider.LoadFeatureState(featureID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading feature %s: %v\n", featureID, err)
		os.Exit(1)
	}

	currentPhase := f.CurrentPhase()
	reason := fmt.Sprintf("recirculated from %s to %s", currentPhase, targetPhase)

	p := pipeline.NewPipeline(cfg, provider)
	f, err = p.RecirculateFeature(f, targetPhase, reason)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error recirculating feature: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Feature %s recirculated from %s to %s\n", featureID, currentPhase, targetPhase)
	fmt.Printf("Current phase: %s\n", f.CurrentPhase())
	fmt.Printf("Status: %s\n", f.Status)
	fmt.Println("\nRun 'devteam run <feature-id>' to re-execute this phase.")
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
	if len(result.Checks) > 0 {
		fmt.Println("Checks:")
		for _, check := range result.Checks {
			status := "PASS"
			if !check.Passed {
				status = "FAIL"
			}
			fmt.Printf("  [%s] %s\n", status, check.Name)
		}
	}

	if result.Passed {
		fmt.Println("\nBootstrap gate passed! Run 'devteam advance 001-dev-team-platform' to continue.")
	} else {
		fmt.Println("\nBootstrap gate not yet passed. Complete the missing artifacts above.")
	}
}

func handleProcess(baseDir string, cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: devteam process <feature-id> [--max-recirculations N]\n")
		fmt.Fprintf(os.Stderr, "\nAutonomously process a feature through the entire pipeline.\n")
		fmt.Fprintf(os.Stderr, "Runs each phase, evaluates gates, advances on pass, recirculates on failure.\n")
		os.Exit(1)
	}
	featureID := os.Args[2]
	maxRecirculations := 3
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--max-recirculations" && i+1 < len(os.Args) {
			fmt.Sscanf(os.Args[i+1], "%d", &maxRecirculations)
			i++
		}
	}

	provider := spec.NewSpecProvider(baseDir)
	f, err := provider.LoadFeatureState(featureID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading feature %s: %v\n", featureID, err)
		os.Exit(1)
	}

	p := pipeline.NewPipeline(cfg, provider)
	recirculations := 0

	fmt.Printf("Processing feature: %s\n", f.ID)
	fmt.Printf("Title: %s\n", f.Title)
	fmt.Printf("Current phase: %s\n", f.CurrentPhase())
	fmt.Printf("Status: %s\n", f.Status)
	fmt.Println(strings.Repeat("=", 70))

	for {
		// Reload feature from disk each iteration to stay in sync
		f, err = provider.LoadFeatureState(featureID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reloading feature %s: %v\n", featureID, err)
			os.Exit(1)
		}

		currentPhase := f.CurrentPhase()

		// Check if we're already done
		if f.Status == feature.StatusDone {
			fmt.Println("\nFeature already completed!")
			fmt.Printf("  Feature: %s\n", f.ID)
			fmt.Printf("  Title: %s\n", f.Title)
			fmt.Printf("  Status: %s\n", f.Status)
			return
		}

		// Check if delivery gate passes — mark done
		if currentPhase == feature.PhaseDelivery {
			gateResult, err := pipeline.NewGateEvaluator(provider).EvaluateForPhase(f, feature.PhaseDelivery)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error evaluating delivery gate: %v\n", err)
				os.Exit(1)
			}
			if gateResult.Passed {
				f.MarkDone()
				if err := p.SaveFeature(f); err != nil {
					fmt.Fprintf(os.Stderr, "error saving feature: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("\nFeature completed successfully!")
				fmt.Printf("  Feature: %s\n", f.ID)
				fmt.Printf("  Title: %s\n", f.Title)
				fmt.Printf("  Status: %s\n", f.Status)
				return
			}
		}

		fmt.Printf("\n--- Phase: %s ---\n", currentPhase)
		fmt.Printf("Dispatching agents for %s...\n", currentPhase)

		result, err := p.RunPhaseWithAgent(context.Background(), f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error running phase %s: %v\n", currentPhase, err)
			os.Exit(1)
		}

		for _, rr := range result.RoleResults {
			status := "SUCCESS"
			if !rr.Success {
				status = "FAILED"
			}
			fmt.Printf("  Role %s (%s): %s (%v)\n", rr.Role, rr.Phase, status, rr.Duration.Round(time.Second))
			if rr.Error != "" {
				fmt.Printf("    Error: %s\n", truncateError(rr.Error, 200))
			}
		}

		// Evaluate the gate for the phase that was just run, not CurrentPhase()
		// (which may have advanced after the agent saved state to disk)
		fmt.Printf("\nEvaluating gate for %s...\n", result.Phase)
		gateResult, err := pipeline.NewGateEvaluator(provider).EvaluateForPhase(f, result.Phase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error evaluating gate: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Gate %s: %v\n", gateResult.Phase, gateResult.Passed)
		if len(gateResult.Checks) > 0 {
			for _, check := range gateResult.Checks {
				symbol := "✓"
				if !check.Passed {
					symbol = "✗"
				}
				fmt.Printf("  %s %s\n", symbol, check.Name)
			}
		}

		if !gateResult.Passed {
			recirculations++
			if recirculations > maxRecirculations {
				fmt.Printf("\nMaximum recirculations (%d) reached. Stopping.\n", maxRecirculations)
				fmt.Println("Fix the issues above and re-run 'devteam process <feature-id>'.")
				os.Exit(1)
			}

			targetPhase := feature.RecirculationTarget(result.Phase, "gate failed")

			if targetPhase == result.Phase {
				// Same-phase retry: reset phase state to in_progress without recirculating
				fmt.Printf("\nGate failed. Retrying %s (attempt %d/%d)\n", result.Phase, recirculations, maxRecirculations)
				fmt.Println("Fixing issues and retrying...")
				if ps, ok := f.PhaseStates[result.Phase]; ok {
					ps.Status = feature.StatusInProgress
					ps.GateResult = nil
				}
				f.Status = feature.StatusInProgress
				f.UpdatedAt = time.Now()
				if err := p.SaveFeature(f); err != nil {
					fmt.Fprintf(os.Stderr, "error saving feature state: %v\n", err)
					os.Exit(1)
				}
			} else {
				// Recirculate to a different (earlier) phase
				fmt.Printf("\nGate failed. Recirculating from %s to %s (attempt %d/%d)\n", result.Phase, targetPhase, recirculations, maxRecirculations)
				fmt.Println("Fixing issues and retrying...")
				f, err = p.RecirculateFeature(f, targetPhase, fmt.Sprintf("gate failed at %s (attempt %d)", result.Phase, recirculations))
				if err != nil {
					fmt.Fprintf(os.Stderr, "error recirculating: %v\n", err)
					os.Exit(1)
				}
			}
			continue
		}

		// Determine next phase based on the phase that was just run
		phases := feature.AllPhases()
		runPhaseIdx := -1
		for i, phase := range phases {
			if phase == result.Phase {
				runPhaseIdx = i
				break
			}
		}

		if runPhaseIdx == len(phases)-1 {
			f.MarkDone()
			if err := p.SaveFeature(f); err != nil {
				fmt.Fprintf(os.Stderr, "error saving feature: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("\nFeature completed successfully!")
			fmt.Printf("  Feature: %s\n", f.ID)
			fmt.Printf("  Title: %s\n", f.Title)
			fmt.Printf("  Status: %s\n", f.Status)
			return
		}

		nextPhase := phases[runPhaseIdx+1]
		fmt.Printf("\nGate passed! Advancing from %s to %s.\n", result.Phase, nextPhase)
		f, err = p.AdvanceFeatureFrom(f, result.Phase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error advancing: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Advanced to: %s\n", f.CurrentPhase())
		recirculations = 0
	}
}

func truncateError(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func handleInit() {
	initializer := devinit.NewInitializer(".")
	if err := initializer.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "error initializing project: %v\n", err)
		os.Exit(1)
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
	fmt.Fprintf(os.Stderr, "  intake       Submit a new feature (loose idea or external spec)\n")
	fmt.Fprintf(os.Stderr, "  run          Run the current pipeline phase for a feature (dispatches agents)\n")
	fmt.Fprintf(os.Stderr, "  process      Autonomously process a feature through the entire pipeline\n")
	fmt.Fprintf(os.Stderr, "  advance      Advance feature to next phase after gate passes\n")
	fmt.Fprintf(os.Stderr, "  gate         Evaluate the current phase gate for a feature\n")
	fmt.Fprintf(os.Stderr, "  recirculate  Send a feature back to an earlier phase\n")
	fmt.Fprintf(os.Stderr, "  init        Initialize a new devteam project (scaffolds directory structure)\n")
	fmt.Fprintf(os.Stderr, "  status       Show current pipeline status for all features\n")
	fmt.Fprintf(os.Stderr, "  bootstrap    Self-bootstrap: process spec 001 through the pipeline\n")
	fmt.Fprintf(os.Stderr, "  version      Print version\n\n")
	fmt.Fprintf(os.Stderr, "Intake options:\n")
	fmt.Fprintf(os.Stderr, "  --type loose|external   Intake path type\n")
	fmt.Fprintf(os.Stderr, "  --text \"idea\"           Loose idea text\n")
	fmt.Fprintf(os.Stderr, "  --file path             Path to external spec\n")
	fmt.Fprintf(os.Stderr, "  --priority 1|2|3       Feature priority\n\n")
	fmt.Fprintf(os.Stderr, "Pipeline flow (manual):\n")
	fmt.Fprintf(os.Stderr, "  1. devteam intake --type loose --text \"idea\"\n")
	fmt.Fprintf(os.Stderr, "  2. devteam run <feature-id>      # Execute current phase\n")
	fmt.Fprintf(os.Stderr, "  3. devteam gate <feature-id>      # Check if gate passes\n")
	fmt.Fprintf(os.Stderr, "  4. devteam advance <feature-id>   # Move to next phase\n")
	fmt.Fprintf(os.Stderr, "  5. Repeat 2-4 until delivery\n\n")
	fmt.Fprintf(os.Stderr, "Pipeline flow (autonomous):\n")
	fmt.Fprintf(os.Stderr, "  devteam process <feature-id>     # Runs entire pipeline end-to-end\n\n")
	fmt.Fprintf(os.Stderr, "Recirculate options:\n")
	fmt.Fprintf(os.Stderr, "  devteam recirculate <feature-id> <target-phase>\n")
	fmt.Fprintf(os.Stderr, "  Phases: inception, planning, construction, review, testing, delivery\n")
}