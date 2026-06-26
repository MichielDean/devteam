package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
)

// handleQuestionsCLI handles: devteam questions <subcommand> <feature-id> [options]
//
//	devteam questions ask <feature-id> --file questions.json
//	devteam questions pending <feature-id>
//	devteam questions list <feature-id>
//	devteam questions answer <feature-id> <question-id> "answer text"
func handleQuestionsCLI(baseDir string, args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: devteam questions <ask|pending|list|answer> <feature-id> [options]\n")
		os.Exit(1)
	}

	subcommand := args[0]
	featureID := args[1]

	database := openDB(baseDir)
	defer database.Close()

	store := feature.NewDBQuestionStore(database)

	switch subcommand {
	case "ask":
		// devteam questions ask <feature-id> --file questions.json
		filePath := ""
		for i, arg := range args[2:] {
			if arg == "--file" && i+1 < len(args[2:]) {
				filePath = args[2:][i+1]
			}
		}
		if filePath == "" {
			filePath = "questions.json"
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", filePath, err)
			os.Exit(1)
		}

		var rawQuestions []struct {
			Phase    string   `json:"phase"`
			Role     string   `json:"role"`
			Question string   `json:"question"`
			Type     string   `json:"type"`
			Options  []string `json:"options"`
		}
		if err := json.Unmarshal(data, &rawQuestions); err != nil {
			fmt.Fprintf(os.Stderr, "error parsing questions.json: %v\n", err)
			os.Exit(1)
		}

		// Ensure feature exists in SQLite for FK constraint
		ensureFeatureInDB(database, featureID)

		ctx := context.Background()
		created := 0
		for i, rq := range rawQuestions {
			q := feature.Question{
				FeatureID: featureID,
				Phase:     rq.Phase,
				Role:      rq.Role,
				Question:  rq.Question,
				Type:      rq.Type,
				Options:   rq.Options,
				Status:    feature.QuestionStatusPending,
			}
			if q.Options == nil {
				q.Options = []string{}
			}
			if q.Type == "" {
				q.Type = "multiple_choice"
			}

			result, err := store.CreateQuestion(ctx, featureID, q)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error creating question %d: %v\n", i, err)
				continue
			}
			created++
			fmt.Printf("Created question %d: %s\n", i, result.ID)
		}

		// Delete the questions.json file so it's not re-detected
		os.Remove(filePath)

		fmt.Printf("Successfully created %d questions for feature %s\n", created, featureID)

	case "pending":
		ctx := context.Background()
		questions, err := store.ListPendingQuestions(ctx, featureID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error listing pending questions: %v\n", err)
			os.Exit(1)
		}
		if len(questions) == 0 {
			fmt.Println("No pending questions")
			return
		}
		for _, q := range questions {
			fmt.Printf("%s\t%s\n", q.ID, q.Question)
		}

	case "list":
		ctx := context.Background()
		questions, err := store.ListQuestions(ctx, featureID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error listing questions: %v\n", err)
			os.Exit(1)
		}
		for _, q := range questions {
			status := string(q.Status)
			answer := ""
			if q.Answer != nil {
				answer = *q.Answer
			}
			fmt.Printf("%s\t%s\t%s\t%s\n", q.ID, status, q.Question, answer)
		}

	case "answer":
		if len(args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: devteam questions answer <feature-id> <question-id> <answer>\n")
			os.Exit(1)
		}
		questionID := args[2]
		answer := strings.Join(args[3:], " ")

		ctx := context.Background()
		result, err := store.AnswerQuestion(ctx, featureID, questionID, answer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error answering question: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Answered question %s: %s\n", result.ID, answer)

	default:
		fmt.Fprintf(os.Stderr, "unknown questions subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

// handleSignalCLI handles: devteam signal <feature-id> <outcome> [notes]
//
//	devteam signal <feature-id> pass
//	devteam signal <feature-id> recirculate:construction --notes "missing error handling"
//	devteam signal <feature-id> needs_feedback
//	devteam signal <feature-id> failed
func handleSignalCLI(baseDir string, args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: devteam signal <feature-id> <pass|recirculate:target|needs_feedback|failed> [--notes \"text\"]\n")
		os.Exit(1)
	}

	featureID := args[0]
	outcome := args[1]

	notes := ""
	for i, arg := range args[2:] {
		if arg == "--notes" && i+1 < len(args[2:]) {
			notes = strings.Join(args[2:][i+1:], " ")
			break
		}
	}

	// Write outcome.txt to the spec directory
	specDir := findSpecDir(baseDir, featureID)
	if specDir == "" {
		fmt.Fprintf(os.Stderr, "error: could not find spec directory for feature %s\n", featureID)
		os.Exit(1)
	}

	content := outcome
	if notes != "" {
		content = outcome + "\n" + notes
	}

	outcomePath := filepath.Join(specDir, "outcome.txt")
	if err := os.WriteFile(outcomePath, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing outcome: %v\n", err)
		os.Exit(1)
	}

	// Also record in SQLite
	database := openDB(baseDir)
	defer database.Close()

	ensureFeatureInDB(database, featureID)

	eventType := "phase_complete"
	if outcome == "recirculate" || strings.HasPrefix(outcome, "recirculate:") {
		eventType = "recirculate"
	}
	database.RecordEvent(featureID, eventType, "", notes)

	fmt.Printf("Signal recorded: %s for feature %s\n", outcome, featureID)
	if notes != "" {
		fmt.Printf("Notes: %s\n", notes)
	}
}

// handleNotesCLI handles: devteam notes <add|list> <feature-id> [options]
//
//	devteam notes add <feature-id> --phase inception --content "Spec complete"
//	devteam notes list <feature-id>
func handleNotesCLI(baseDir string, args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: devteam notes <add|list> <feature-id> [options]\n")
		os.Exit(1)
	}

	subcommand := args[0]
	featureID := args[1]

	database := openDB(baseDir)
	defer database.Close()

	ensureFeatureInDB(database, featureID)

	switch subcommand {
	case "add":
		phase := ""
		content := ""
		for i, arg := range args[2:] {
			if arg == "--phase" && i+1 < len(args[2:]) {
				phase = args[2:][i+1]
			}
			if arg == "--content" && i+1 < len(args[2:]) {
				content = strings.Join(args[2:][i+1:], " ")
			}
		}
		if content == "" {
			fmt.Fprintf(os.Stderr, "error: --content is required\n")
			os.Exit(1)
		}

		database.AddNote(featureID, phase, "agent", "summary", content)
		fmt.Printf("Note added for feature %s (phase: %s)\n", featureID, phase)

	case "list":
		notes, err := database.GetNotes(featureID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error listing notes: %v\n", err)
			os.Exit(1)
		}
		for _, n := range notes {
			fmt.Printf("[%s] %s/%s: %s\n", n.CreatedAt.Format("2006-01-02 15:04:05"), n.Phase, n.Role, n.Content)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown notes subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

// handleFeatureCLI handles: devteam feature <status|info> <feature-id>
func handleFeatureCLI(baseDir string, args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: devteam feature <status|info> <feature-id>\n")
		os.Exit(1)
	}

	subcommand := args[0]
	featureID := args[1]

	switch subcommand {
	case "status":
		database := openDB(baseDir)
		defer database.Close()

		var status, phase string
		err := database.QueryRow(
			"SELECT status, current_phase FROM features WHERE id = ?",
			featureID,
		).Scan(&status, &phase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: feature %s not found in database\n", featureID)
			os.Exit(1)
		}
		fmt.Printf("%s\t%s\t%s\n", featureID, status, phase)

	case "info":
		database := openDB(baseDir)
		defer database.Close()

		row := database.QueryRow(
			"SELECT id, title, current_phase, status, priority, worktree_dir FROM features WHERE id = ?",
			featureID,
		)
		var id, title, phase, status, worktreeDir string
		var priority int
		err := row.Scan(&id, &title, &phase, &status, &priority, &worktreeDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: feature %s not found\n", featureID)
			os.Exit(1)
		}
		fmt.Printf("ID: %s\nTitle: %s\nPhase: %s\nStatus: %s\nPriority: %d\nWorktree: %s\n",
			id, title, phase, status, priority, worktreeDir)

	default:
		fmt.Fprintf(os.Stderr, "unknown feature subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

// --- Helpers ---

func openDB(baseDir string) *db.DB {
	// Agent CLI commands always use the primary checkout's DB, not the worktree's.
	// The DEVTEAM_DB_PATH env var can override the location.
	dbPath := os.Getenv("DEVTEAM_DB_PATH")
	if dbPath == "" {
		// Default: primary checkout's .devteam.db
		dbPath = filepath.Join(os.Getenv("HOME"), "source", "devteam", ".devteam.db")
	}
	// If the baseDir has a .devteam.db, prefer that (for dev/testing)
	localDB := filepath.Join(baseDir, ".devteam.db")
	if _, err := os.Stat(localDB); err == nil {
		dbPath = localDB
	}

	database, err := db.Open(db.Config{Driver: "sqlite3"}, dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database at %s: %v\n", dbPath, err)
		os.Exit(1)
	}
	return database
}

func ensureFeatureInDB(database *db.DB, featureID string) {
	// Insert a minimal feature row if it doesn't exist (for FK constraints)
	database.Exec(
		`INSERT OR IGNORE INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count) VALUES (?, ?, 'inception', 'in_progress', 3, 'loose_idea', '', ?, ?, 0)`,
		featureID, featureID, time.Now().UTC(), time.Now().UTC(),
	)
}

func findSpecDir(baseDir, featureID string) string {
	// Check worktree first
	wtBase := filepath.Join(os.Getenv("HOME"), "worktrees", "devteam-specs", featureID)
	wtSpecDir := filepath.Join(wtBase, "specs", featureID)
	if _, err := os.Stat(wtSpecDir); err == nil {
		return wtSpecDir
	}
	// Fallback to primary checkout
	primarySpecDir := filepath.Join(baseDir, "specs", featureID)
	if _, err := os.Stat(primarySpecDir); err == nil {
		return primarySpecDir
	}
	return ""
}