package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// apiURL returns the Dev Team API URL from env var or default
func apiURL() string {
	url := os.Getenv("DEVTEAM_API_URL")
	if url == "" {
		url = "http://localhost:8765"
	}
	return strings.TrimRight(url, "/")
}

// apiPost sends a POST request to the API and returns the response body
func apiPost(path string, body interface{}) (string, error) {
	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post(apiURL()+path, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return string(respBody), nil
}

// apiPatch sends a PATCH request to the API and returns the response body
func apiPatch(path string, body interface{}) (string, error) {
	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPatch, apiURL()+path, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("building PATCH request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return string(respBody), nil
}

// apiGet sends a GET request and returns the response body
func apiGet(path string) (string, error) {
	resp, err := http.Get(apiURL() + path)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return string(respBody), nil
}

// handleQuestionsAPICLI handles: devteam questions <ask|pending|list|answer> <feature-id> [options]
func handleQuestionsAPICLI(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: devteam questions <ask|pending|list|answer> <feature-id> [options]\n")
		os.Exit(1)
	}

	subcommand := args[0]
	featureID := args[1]

	switch subcommand {
	case "ask":
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

		// Parse the questions and submit each via API
		var questions []struct {
			Phase    string   `json:"phase"`
			Role     string   `json:"role"`
			Question string   `json:"question"`
			Type     string   `json:"type"`
			Options  []string `json:"options"`
		}
		if err := json.Unmarshal(data, &questions); err != nil {
			fmt.Fprintf(os.Stderr, "error parsing questions.json: %v\n", err)
			os.Exit(1)
		}

		created := 0
		for i, q := range questions {
			body := map[string]interface{}{
				"phase":    q.Phase,
				"role":     q.Role,
				"question": q.Question,
				"type":     q.Type,
				"options":  q.Options,
			}
			_, err = apiPost(fmt.Sprintf("/api/features/%s/questions", featureID), body)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error creating question %d: %v\n", i, err)
				continue
			}
			created++
			fmt.Printf("Created question %d\n", i)
		}

		os.Remove(filePath)
		fmt.Printf("Successfully created %d questions for feature %s\n", created, featureID)

	case "pending":
		resp, err := apiGet(fmt.Sprintf("/api/features/%s/questions/pending", featureID))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		var questions []map[string]interface{}
		json.Unmarshal([]byte(resp), &questions)
		if len(questions) == 0 {
			fmt.Println("No pending questions")
			return
		}
		for _, q := range questions {
			fmt.Printf("%s\t%s\n", q["id"], q["question"])
		}

	case "list":
		resp, err := apiGet(fmt.Sprintf("/api/features/%s/questions", featureID))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(resp)

	case "answer":
		if len(args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: devteam questions answer <feature-id> <question-id> <answer>\n")
			os.Exit(1)
		}
		questionID := args[2]
		answer := strings.Join(args[3:], " ")

		body := map[string]interface{}{"answer": answer}
		resp, err := apiPatch(fmt.Sprintf("/api/features/%s/questions/%s", featureID, questionID), body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error answering question: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Answered: %s\n", resp)

	default:
		fmt.Fprintf(os.Stderr, "unknown questions subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

// handleSignalAPICLI handles: devteam signal <feature-id> <outcome> [--notes "text"]
func handleSignalAPICLI(args []string) {
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

	target := ""
	if strings.HasPrefix(outcome, "recirculate:") {
		parts := strings.SplitN(outcome, ":", 2)
		target = parts[1]
	}

	body := map[string]interface{}{
		"outcome": outcome,
		"target":  target,
		"notes":   notes,
	}
	resp, err := apiPost(fmt.Sprintf("/api/features/%s/signal", featureID), body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error signaling: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Signal recorded: %s\n", resp)
}

// handleNotesAPICLI handles: devteam notes <add|list> <feature-id> [options]
func handleNotesAPICLI(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: devteam notes <add|list> <feature-id> [options]\n")
		os.Exit(1)
	}

	subcommand := args[0]
	featureID := args[1]

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

		body := map[string]interface{}{"phase": phase, "content": content}
		resp, err := apiPost(fmt.Sprintf("/api/features/%s/notes", featureID), body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error adding note: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Note added: %s\n", resp)

	case "list":
		resp, err := apiGet(fmt.Sprintf("/api/features/%s/notes", featureID))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(resp)

	default:
		fmt.Fprintf(os.Stderr, "unknown notes subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

// handleArtifactAPICLI handles: devteam artifact <submit|get> <feature-id> <type> [options]
func handleArtifactAPICLI(args []string) {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: devteam artifact <submit|get> <feature-id> <type> [--file path | --content \"text\"]\n")
		os.Exit(1)
	}

	subcommand := args[0]
	featureID := args[1]
	artType := args[2]

	switch subcommand {
	case "submit":
		content := ""
		filePath := ""
		for i, arg := range args[3:] {
			if arg == "--file" && i+1 < len(args[3:]) {
				filePath = args[3:][i+1]
			}
			if arg == "--content" && i+1 < len(args[3:]) {
				content = strings.Join(args[3:][i+1:], " ")
			}
		}

		if filePath != "" {
			data, err := os.ReadFile(filePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
				os.Exit(1)
			}
			content = string(data)
		}

		if content == "" {
			fmt.Fprintf(os.Stderr, "error: --file or --content is required\n")
			os.Exit(1)
		}

		body := map[string]interface{}{"content": content}
		resp, err := apiPost(fmt.Sprintf("/api/features/%s/artifacts/%s", featureID, artType), body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error submitting artifact: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Artifact saved: %s\n", resp)

	case "get":
		resp, err := apiGet(fmt.Sprintf("/api/features/%s/artifacts/%s", featureID, artType))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(resp)

	default:
		fmt.Fprintf(os.Stderr, "unknown artifact subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

// handleArtifactsListCLI handles: devteam artifacts <feature-id> [--stage 1.4 | --all]
// Fetches all artifacts for the current stage (or specified stage, or all) and
// prints them as a single concatenated markdown document.
func handleArtifactsListCLI(args []string) {
	featureID := args[0]
	stageID := ""
	allArtifacts := false

	for i, arg := range args[1:] {
		if arg == "--stage" && i+1 < len(args[1:]) {
			stageID = args[1:][i+1]
		}
		if arg == "--all" {
			allArtifacts = true
		}
	}

	// If no stage specified and not --all, look up the feature's current stage
	if stageID == "" && !allArtifacts {
		resp, err := apiGet(fmt.Sprintf("/api/features/%s", featureID))
		if err == nil {
			var f map[string]interface{}
			json.Unmarshal([]byte(resp), &f)
			if cs, ok := f["current_stage"].(string); ok && cs != "" {
				stageID = cs
			}
		}
	}

	// Fetch the artifact list
	resp, err := apiGet(fmt.Sprintf("/api/features/%s/artifacts", featureID))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing artifacts: %v\n", err)
		os.Exit(1)
	}

	var artifacts []struct {
		ArtifactType string `json:"artifact_type"`
		StageID      string `json:"stage_id"`
		Size         int    `json:"size"`
	}
	json.Unmarshal([]byte(resp), &artifacts)

	if len(artifacts) == 0 {
		fmt.Println("No artifacts found.")
		return
	}

	// Filter to current stage if not --all
	var filtered []struct {
		ArtifactType string `json:"artifact_type"`
		StageID      string `json:"stage_id"`
		Size         int    `json:"size"`
	}
	for _, a := range artifacts {
		if allArtifacts || a.StageID == stageID {
			filtered = append(filtered, a)
		}
	}

	// Also check stage definition for key_artifacts — fetch those too even if stage_id not set
	if !allArtifacts && stageID != "" {
		// Get stage definition to know expected key_artifacts
		stageResp, err := apiGet(fmt.Sprintf("/api/features/%s/stages", featureID))
		if err == nil {
			var stages []struct {
				StageID     string   `json:"stage_id"`
				KeyArtifacts []string `json:"key_artifacts"`
			}
			json.Unmarshal([]byte(stageResp), &stages)
			for _, s := range stages {
				if s.StageID == stageID {
					for _, expected := range s.KeyArtifacts {
						// Check if we already have it
						found := false
						for _, f := range filtered {
							if f.ArtifactType == expected {
								found = true
								break
							}
						}
						if !found {
							// Try to fetch it
							for _, a := range artifacts {
								if a.ArtifactType == expected {
									filtered = append(filtered, a)
									break
								}
							}
						}
					}
				}
			}
		}
	}

	if len(filtered) == 0 {
		fmt.Printf("No artifacts found for stage %s.\n", stageID)
		fmt.Println("Available artifacts:")
		for _, a := range artifacts {
			fmt.Printf("  %s (stage: %s, size: %d)\n", a.ArtifactType, a.StageID, a.Size)
		}
		return
	}

	// Fetch and print each artifact
	for i, a := range filtered {
		if i > 0 {
			fmt.Print("\n---\n\n")
		}
		fmt.Printf("# Artifact: %s (stage: %s)\n\n", a.ArtifactType, a.StageID)
		content, err := apiGet(fmt.Sprintf("/api/features/%s/artifacts/%s", featureID, a.ArtifactType))
		if err != nil {
			fmt.Printf("Error fetching %s: %v\n", a.ArtifactType, err)
			continue
		}
		fmt.Println(content)
	}
}
func handleFeatureAPICLI(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: devteam feature <status|info> <feature-id>\n")
		os.Exit(1)
	}

	subcommand := args[0]
	featureID := args[1]

	switch subcommand {
	case "status":
		resp, err := apiGet(fmt.Sprintf("/api/features/%s", featureID))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		var f map[string]interface{}
		json.Unmarshal([]byte(resp), &f)
		fmt.Printf("%s\t%s\t%s\n", featureID, f["status"], f["current_phase"])

	case "info":
		resp, err := apiGet(fmt.Sprintf("/api/features/%s", featureID))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(resp)

	default:
		fmt.Fprintf(os.Stderr, "unknown feature subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}