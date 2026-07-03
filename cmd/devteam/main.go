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

	"github.com/MichielDean/devteam/internal/api"
	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	devinit "github.com/MichielDean/devteam/internal/init"
	"github.com/MichielDean/devteam/internal/intake"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/plugins"
	"github.com/MichielDean/devteam/internal/spec"
	"github.com/MichielDean/devteam/internal/stage"
)

const version = "0.4.0"

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

		// Open database for operational data (PostgreSQL)
		// Configure via devteam.yaml:
		//   database:
		//     dsn: "host=localhost port=5432 user=devteam password=devteam dbname=devteam sslmode=disable"
		// Or via environment variable:
		//   DEVTEAM_DB_DSN="host=localhost ..."
		dbCfg := db.Config{
			DSN: cfg.Database.DSN,
		}
		// Environment variable overrides config
		if envDSN := os.Getenv("DEVTEAM_DB_DSN"); envDSN != "" {
			dbCfg.DSN = envDSN
		}
		defaultDSN := "host=localhost port=5432 user=devteam password=devteam dbname=devteam sslmode=disable"
		database, err := db.Open(dbCfg, defaultDSN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		if err := stage.SeedStages(database); err != nil {
			fmt.Fprintf(os.Stderr, "error seeding stage definitions: %v\n", err)
			os.Exit(1)
		}

		// Wire database and DB question store into pipeline
		questionStore := feature.NewDBQuestionStore(database)
		p.SetDatabase(database)
		p.SetQuestionStore(questionStore)
		specProvider.SetDatabase(database)

		// Serve frontend: use local filesystem (development or after go generate)
		var staticFS fs.FS
		uiDir := filepath.Join(baseDir, "ui", "dist")
		if info, err := os.Stat(uiDir); err == nil && info.IsDir() {
			staticFS = os.DirFS(uiDir)
		}
		// If ui/dist doesn't exist, staticFS is nil — API-only mode (no frontend)

		server := api.NewServer(*httpAddr, specProvider, p, staticFS, questionStore, database)
		// Arm rate limiting (v2 — F-15, BR-59). Inserted between NewServer
		// and RestoreActiveProcesses per the setter-based wiring decision
		// (D10/ADR-007). NewServer's signature is UNCHANGED (BR-57). When the
		// rate_limit: block is absent or enabled:false, this is a no-op
		// (BR-33 — passthrough, byte-identical to pre-feature). When invalid,
		// ConfigureRateLimiting logs + leaves the limiter nil (BR-08 —
		// fail-open startup, NOT a crash; the fatal validateConfig path at
		// main.go:42-45 is NOT touched, F-10).
		server.ConfigureRateLimiting(&cfg.RateLimit, filepath.Join(baseDir, "devteam.yaml"))
		server.RestoreActiveProcesses()

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
	case "questions":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: devteam questions <ask|pending|list|answer> <feature-id> [options]\n")
			os.Exit(1)
		}
		handleQuestionsAPICLI(os.Args[2:])
		return
	case "signal":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: devteam signal <feature-id> <outcome> [--notes \"text\"]\n")
			os.Exit(1)
		}
		handleSignalAPICLI(os.Args[2:])
		return
	case "notes":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: devteam notes <add|list> <feature-id> [options]\n")
			os.Exit(1)
		}
		handleNotesAPICLI(os.Args[2:])
		return
	case "artifact":
		if len(os.Args) < 5 {
			fmt.Fprintf(os.Stderr, "Usage: devteam artifact <submit|get> <feature-id> <type> [options]\n")
			os.Exit(1)
		}
		handleArtifactAPICLI(os.Args[2:])
		return
	case "feature":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: devteam feature <status|info> <feature-id>\n")
			os.Exit(1)
		}
		handleFeatureAPICLI(os.Args[2:])
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
	case "run-stage":
		handleRunStage(baseDir, cfg)
	case "approve":
		handleApprove(baseDir, cfg)
	case "reject":
		handleReject(baseDir, cfg)
	case "jump":
		handleJump(baseDir, cfg)
	case "stages":
		handleStages(baseDir, cfg)
	case "audit":
		handleAudit(baseDir, cfg)
	case "plugin":
		handlePlugin(baseDir, cfg)
	case "bootstrap":
		handleBootstrap(baseDir, cfg)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func handleStatus(baseDir string) {
	provider, database := newDBProvider(baseDir)
	defer database.Close()
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
	database := openDB(baseDir)

	switch intakeType {
	case "loose":
		li := intake.NewLooseIdeaIntake(baseDir)
		li.SetDatabase(database)
		f, err := li.Submit(title, title, priority, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error submitting loose idea: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Feature created: %s\n", f.ID)
		fmt.Printf("Phase: %s\n", f.CurrentPhase())
		fmt.Printf("Intake path: loose_idea\n")
	case "external":
		ei := intake.NewExternalSpecIntake(baseDir)
		ei.SetDatabase(database)
		result, err := ei.Submit(title, "External specification", priority, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error submitting external spec: %v\n", err)
			os.Exit(1)
		}
		for _, f := range result.Features {
			fmt.Printf("Feature created: %s\n", f.ID)
			fmt.Printf("Intake path: external_spec\n")
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown intake type: %s (use 'loose' or 'external')\n", intakeType)
		os.Exit(1)
	}
}

// openDB opens the devteam PostgreSQL database for CLI commands that need DB access.
func openDB(baseDir string) *db.DB {
	defaultDSN := "host=localhost port=5432 user=devteam password=devteam dbname=devteam sslmode=disable"
	database, err := db.Open(db.Config{}, defaultDSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	if err := stage.SeedStages(database); err != nil {
		fmt.Fprintf(os.Stderr, "error seeding stage definitions: %v\n", err)
		os.Exit(1)
	}
	return database
}

// newDBProvider creates a SpecProvider wired to the database.
func newDBProvider(baseDir string) (*spec.SpecProvider, *db.DB) {
	provider := spec.NewSpecProvider(baseDir)
	database := openDB(baseDir)
	provider.SetDatabase(database)
	return provider, database
}

// newDBPipeline creates a Pipeline wired to the database.
func newDBPipeline(baseDir string) (*pipeline.Pipeline, *spec.SpecProvider, *db.DB) {
	provider, database := newDBProvider(baseDir)
	cfg, err := config.LoadConfig(filepath.Join(baseDir, "devteam.yaml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}
	p := pipeline.NewPipeline(cfg, provider)
	p.SetDatabase(database)
	return p, provider, database
}

func handleRunStage(baseDir string, cfg *config.Config) {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: devteam run-stage <feature-id> <stage-id>\n")
		fmt.Fprintf(os.Stderr, "Example: devteam run-stage feat-001 1.1\n")
		os.Exit(1)
	}
	featureID := os.Args[2]
	stageID := os.Args[3]

	provider, database := newDBProvider(baseDir)
	defer database.Close()
	f, err := provider.LoadFeatureState(featureID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading feature %s: %v\n", featureID, err)
		os.Exit(1)
	}

	p := pipeline.NewPipeline(cfg, provider)
	p.SetDatabase(database)

	// Initialize stages if needed
	scope := f.Scope
	if scope == "" {
		scope = stage.ScopeFeature
	}
	fstages, _ := database.GetFeatureStages(featureID)
	if len(fstages) == 0 {
		database.InitFeatureStages(featureID, scope)
	}

	fmt.Printf("Running stage %s for feature %s...\n", stageID, featureID)
	result, err := p.RunStage(context.Background(), f, stageID, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error running stage: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Stage %s (%s) completed in %v\n", result.StageID, result.StageName, result.Duration)
	fmt.Printf("Outcome: %s (source: %s)\n", result.Outcome.Outcome, result.OutcomeSource)

	if len(result.SmokeFailures) > 0 {
		fmt.Println("\nSmoke check failures:")
		for _, fail := range result.SmokeFailures {
			fmt.Printf("  [FAIL] %s\n", fail)
		}
	}
	if result.ReviewerResult != nil {
		fmt.Printf("\nReviewer (%s): %s\n", result.ReviewerResult.Reviewer, result.ReviewerResult.Verdict)
		if result.ReviewerResult.Notes != "" {
			fmt.Printf("  Notes: %s\n", result.ReviewerResult.Notes)
		}
	}
	if result.Gate != nil {
		fmt.Printf("\nGate state: %s (revisions: %d)\n", result.Gate.State, result.Gate.RevisionCount)
		if result.Gate.State == "open" {
			fmt.Println("Run 'devteam approve <feature-id> <stage-id>' to approve and advance.")
		}
	}
}

func handleApprove(baseDir string, cfg *config.Config) {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: devteam approve <feature-id> <stage-id>\n")
		os.Exit(1)
	}
	featureID := os.Args[2]
	stageID := os.Args[3]

	provider, database := newDBProvider(baseDir)
	defer database.Close()
	f, err := provider.LoadFeatureState(featureID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading feature %s: %v\n", featureID, err)
		os.Exit(1)
	}

	p := pipeline.NewPipeline(cfg, provider)
	p.SetDatabase(database)

	if err := p.ApproveStage(f, stageID); err != nil {
		fmt.Fprintf(os.Stderr, "error approving stage: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Stage %s approved for feature %s. Advanced to next stage.\n", stageID, featureID)
}

func handleReject(baseDir string, cfg *config.Config) {
	if len(os.Args) < 5 {
		fmt.Fprintf(os.Stderr, "Usage: devteam reject <feature-id> <stage-id> <notes>\n")
		os.Exit(1)
	}
	featureID := os.Args[2]
	stageID := os.Args[3]
	notes := os.Args[4]

	provider, database := newDBProvider(baseDir)
	defer database.Close()
	f, err := provider.LoadFeatureState(featureID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading feature %s: %v\n", featureID, err)
		os.Exit(1)
	}

	p := pipeline.NewPipeline(cfg, provider)
	p.SetDatabase(database)

	if err := p.RejectStage(f, stageID, notes); err != nil {
		fmt.Fprintf(os.Stderr, "error rejecting stage: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Stage %s rejected for feature %s. Rule saved. Re-run the stage to address feedback.\n", stageID, featureID)
}

func handleJump(baseDir string, cfg *config.Config) {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: devteam jump <feature-id> <stage-id|phase:phase-name>\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  devteam jump feat-001 2.3\n")
		fmt.Fprintf(os.Stderr, "  devteam jump feat-001 phase:construction\n")
		os.Exit(1)
	}
	featureID := os.Args[2]
	target := os.Args[3]

	provider, database := newDBProvider(baseDir)
	defer database.Close()
	f, err := provider.LoadFeatureState(featureID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading feature %s: %v\n", featureID, err)
		os.Exit(1)
	}

	p := pipeline.NewPipeline(cfg, provider)
	p.SetDatabase(database)

	if strings.HasPrefix(target, "phase:") {
		phaseName := strings.TrimPrefix(target, "phase:")
		if err := p.JumpToPhase(f, phaseName); err != nil {
			fmt.Fprintf(os.Stderr, "error jumping to phase: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Jumped to phase %s for feature %s\n", phaseName, featureID)
	} else {
		if err := p.JumpToStage(f, target); err != nil {
			fmt.Fprintf(os.Stderr, "error jumping to stage: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Jumped to stage %s for feature %s\n", target, featureID)
	}
}

func handleStages(baseDir string, cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: devteam stages <feature-id>\n")
		os.Exit(1)
	}
	featureID := os.Args[2]

	_, database := newDBProvider(baseDir)
	defer database.Close()

	stages, err := database.GetFeatureStages(featureID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading stages: %v\n", err)
		os.Exit(1)
	}

	if len(stages) == 0 {
		fmt.Printf("No stages initialized for feature %s. Create the feature with scope first.\n", featureID)
		return
	}

	fmt.Printf("Stages for feature %s (%d total):\n\n", featureID, len(stages))
	for _, s := range stages {
		checkbox := stage.StageCheckbox(s.Status)
		rev := ""
		if s.RevisionCount > 0 {
			rev = fmt.Sprintf(" (×%d revisions)", s.RevisionCount)
		}
		fmt.Printf("  %s %s  %s%s\n", checkbox, s.StageID, s.Status, rev)
	}
}

func handleAudit(baseDir string, cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: devteam audit <feature-id>\n")
		os.Exit(1)
	}
	featureID := os.Args[2]

	_, database := newDBProvider(baseDir)
	defer database.Close()

	events, err := database.GetAuditEvents(featureID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading audit trail: %v\n", err)
		os.Exit(1)
	}

	if len(events) == 0 {
		fmt.Printf("No audit events for feature %s\n", featureID)
		return
	}

	fmt.Printf("Audit trail for feature %s (%d events):\n\n", featureID, len(events))
	for _, e := range events {
		stageStr := ""
		if e.StageID != "" {
			stageStr = fmt.Sprintf(" [%s]", e.StageID)
		}
		details := ""
		if e.Details != "" {
			details = " — " + e.Details
		}
		fmt.Printf("  %s  %s%s%s\n", e.CreatedAt.Format("2006-01-02 15:04:05"), e.EventType, stageStr, details)
	}
}

func handlePlugin(baseDir string, cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: devteam plugin <command>\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  update    Fetch latest plugin rules from upstream sources\n")
		fmt.Fprintf(os.Stderr, "  list      Show configured plugins and their status\n")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "update":
		handlePluginUpdate(baseDir, cfg)
	case "list":
		handlePluginList(baseDir, cfg)
	default:
		fmt.Fprintf(os.Stderr, "unknown plugin command: %s\n", os.Args[2])
		fmt.Fprintf(os.Stderr, "Use 'update' or 'list'\n")
		os.Exit(1)
	}
}

func handlePluginUpdate(baseDir string, cfg *config.Config) {
	updater := plugins.NewUpdater(cfg, baseDir)
	if err := updater.UpdateAll(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error updating plugins: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("All plugins updated.")
}

func handlePluginList(baseDir string, cfg *config.Config) {
	if len(cfg.Plugins) == 0 {
		fmt.Println("No plugins configured.")
		return
	}
	fmt.Println("Configured plugins:")
	fmt.Println(strings.Repeat("-", 70))
	for name, plugin := range cfg.Plugins {
		rules, err := plugins.LoadCachedRules(baseDir, name)
		status := "installed"
		if err != nil {
			status = "not installed (run 'devteam plugin update')"
		} else {
			_ = rules
		}
		fmt.Printf("  %-20s  phases: %-20s  roles: %-12s  %s\n", name, strings.Join(plugin.Phases, ","), strings.Join(plugin.Roles, ","), status)
	}
}

func handleBootstrap(baseDir string, cfg *config.Config) {
	featureID := "001-dev-team-platform"
	provider, database := newDBProvider(baseDir)
	defer database.Close()

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
	fmt.Printf("Scope: %s\n", f.Scope)
	fmt.Printf("Current stage: %s\n", f.CurrentStage)
	fmt.Printf("Status: %s\n\n", f.Status)

	// Show stage progress
	stages, _ := database.GetFeatureStages(featureID)
	if len(stages) > 0 {
		fmt.Printf("Stages (%d total):\n", len(stages))
		for _, s := range stages {
			fmt.Printf("  %s %s  %s\n", stage.StageCheckbox(s.Status), s.StageID, s.Status)
		}
	} else {
		fmt.Println("No stages initialized. Use 'devteam intake' to create a feature with scope.")
	}
	fmt.Println("\nUse 'devteam run-stage <feature-id> <stage-id>' to run a stage.")
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
	fmt.Fprintf(os.Stderr, "Server:\n")
	fmt.Fprintf(os.Stderr, "  -http :8080  Start web server\n\n")
	fmt.Fprintf(os.Stderr, "Pipeline commands:\n")
	fmt.Fprintf(os.Stderr, "  intake       Submit a new feature (loose idea or external spec)\n")
	fmt.Fprintf(os.Stderr, "  run          Run the current pipeline phase for a feature (dispatches agents)\n")
	fmt.Fprintf(os.Stderr, "  process      Autonomously process a feature through the entire pipeline\n")
	fmt.Fprintf(os.Stderr, "  advance      Advance feature to next phase after gate passes\n")
	fmt.Fprintf(os.Stderr, "  gate         Evaluate the current phase gate for a feature\n")
	fmt.Fprintf(os.Stderr, "  recirculate  Send a feature back to an earlier phase\n")
	fmt.Fprintf(os.Stderr, "  status       Show current pipeline status for all features\n\n")
	fmt.Fprintf(os.Stderr, "Agent CLI (for use by dispatched agents):\n")
	fmt.Fprintf(os.Stderr, "  questions ask <feature-id> --file questions.json   Submit questions for user feedback\n")
	fmt.Fprintf(os.Stderr, "  questions pending <feature-id>                     List pending questions\n")
	fmt.Fprintf(os.Stderr, "  questions answer <feature-id> <question-id> <ans>  Answer a question\n")
	fmt.Fprintf(os.Stderr, "  signal <feature-id> <outcome> [--notes \"text\"]     Signal phase outcome (pass/recirculate:target/needs_feedback/failed)\n")
	fmt.Fprintf(os.Stderr, "  notes add <feature-id> --phase <phase> --content    Add a note for the next phase\n")
	fmt.Fprintf(os.Stderr, "  notes list <feature-id>                            List all notes for a feature\n")
	fmt.Fprintf(os.Stderr, "  feature status <feature-id>                        Get feature status\n")
	fmt.Fprintf(os.Stderr, "  feature info <feature-id>                          Get feature info\n\n")
	fmt.Fprintf(os.Stderr, "Other:\n")
	fmt.Fprintf(os.Stderr, "  plugin       Manage pipeline plugins (update, list)\n")
	fmt.Fprintf(os.Stderr, "  init         Initialize a new devteam project\n")
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
