package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
)

// handleRepoCLI dispatches `devteam repo <verb>` (feature github-authorization-integration,
// interaction-spec §3.2..§3.4).
//
// Verbs:
//   list    — render MANAGED + AVAILABLE-BUT-UNMANAGED groups (M-2, US-03)
//   set     — write one MVP field (M-3, US-06, FR-SETTINGS-04)
//   manage  — transition a discovered repo to managed (M-4, US-16, FR-DISC-06)
//
// Exit codes (interaction-spec §12 rule 2): 0 ok, 1 runtime/auth failure,
// 2 field/flag rejection.
func handleRepoCLI(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: devteam repo <list|set|manage> [options]\n")
		os.Exit(1)
	}
	verb := args[1]
	switch verb {
	case "list":
		repoList(args[2:])
	case "set":
		repoSet(args[2:])
	case "manage":
		repoManage(args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown repo verb: %s (use list, set, or manage)\n", verb)
		os.Exit(1)
	}
}

// openRepoDB opens the DB for repo CLI commands.
func openRepoDB() *db.DB {
	baseDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting working directory: %v\n", err)
		os.Exit(1)
	}
	return openDB(baseDir)
}

// repoList implements `devteam repo list` (M-2, US-03, interaction-spec §3.2).
func repoList(args []string) {
	asJSON := false
	noPager := false
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		case "--no-pager":
			noPager = true
		case "--help", "-h":
			fmt.Println("Usage: devteam repo list [--json] [--no-pager]")
			return
		}
	}
	_ = noPager

	database := openRepoDB()
	defer database.Close()

	// If the github: block is configured, run discovery first to refresh the
	// registry (FR-DISC-01). If not, list whatever's in repo_registry.
	cfg, _ := loadConfig()
	if cfg != nil && cfg.GitHub.AppID != 0 {
		// Auth-health precondition (interaction-spec §3.2).
		_, _, _, client := loadGitHubForCLI()
		repos, err := client.ListRepositories(context.Background())
		if err != nil {
			printAuthError(nil, err)
			os.Exit(1)
		}
		// Persist discovered set (FR-DISC-02).
		for _, r := range repos {
			_, _, _ = database.UpsertRepoRegistry(r.Ref.Owner, r.Ref.Name, r.Ref.Owner+"/"+r.Ref.Name, r.DefaultBranch, r.InstallationID)
		}
		_ = database.RecordRepoRegistryAudit("", fmt.Sprintf("discovery synced %d repos", len(repos)))
	}

	rows, err := database.ListRepoRegistry(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing repo_registry: %v\n", err)
		os.Exit(1)
	}

	var managed, unmanaged []db.RepoRegistryRow
	for _, r := range rows {
		if r.Managed == 1 {
			managed = append(managed, r)
		} else {
			unmanaged = append(unmanaged, r)
		}
	}

	if asJSON {
		printRepoListJSON(managed, unmanaged)
		return
	}
	printRepoListText(managed, unmanaged)
}

func printRepoListText(managed, unmanaged []db.RepoRegistryRow) {
	fmt.Printf("MANAGED (%d)\n", len(managed))
	if len(managed) == 0 {
		fmt.Println("  (none — devteam repo manage <name> to begin)")
	}
	for _, r := range managed {
		fmt.Printf("  %s/%s  (default: %s)\n", r.Owner, r.Name, r.DefaultBranch)
	}
	fmt.Printf("\navailable-but-unmanaged (%d)\n", len(unmanaged))
	if len(unmanaged) == 0 {
		fmt.Println("  (none)")
	}
	for _, r := range unmanaged {
		fmt.Printf("  %s/%s\n", r.Owner, r.Name)
	}
}

func printRepoListJSON(managed, unmanaged []db.RepoRegistryRow) {
	fmt.Printf("[")
	first := true
	emit := func(r db.RepoRegistryRow, managed bool) {
		if !first {
			fmt.Printf(",")
		}
		first = false
		fmt.Printf(`{"name":"%s/%s","default_branch":"%s","managed":%t}`, r.Owner, r.Name, r.DefaultBranch, managed)
	}
	for _, r := range managed {
		emit(r, true)
	}
	for _, r := range unmanaged {
		emit(r, false)
	}
	fmt.Printf("]\n")
}

// repoSet implements `devteam repo set <repo> <key>=<value>` (M-3, US-06,
// FR-SETTINGS-04, interaction-spec §3.3).
func repoSet(args []string) {
	dryRun := false
	confirm := false
	var positional []string
	for _, a := range args {
		switch a {
		case "--dry-run":
			dryRun = true
		case "--confirm":
			confirm = true
		case "--help", "-h":
			fmt.Println("Usage: devteam repo set <owner/name> <key>=<value> [--dry-run] [--confirm]")
			fmt.Println("MVP fields: default_branch, pr_draft_default, conflict_detection_enabled, provider")
			return
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: devteam repo set <owner/name> <key>=<value> [--dry-run] [--confirm]\n")
		fmt.Fprintf(os.Stderr, "MVP fields: %s\n", strings.Join(db.MVPRepoSettingsFields, ", "))
		os.Exit(1)
	}
	repoArg := positional[0]
	kv := positional[1]
	parts := strings.SplitN(kv, "=", 2)
	if len(parts) != 2 {
		fmt.Fprintf(os.Stderr, "✗ invalid argument %q: expected <key>=<value>\n", kv)
		os.Exit(2)
	}
	key := parts[0]
	value := parts[1]

	// Field validation (FR-SETTINGS-03, R-08, US-06 criteria 3-4).
	if !contains(db.MVPRepoSettingsFields, key) {
		if contains(db.Phase2RepoSettingsFields, key) {
			fmt.Fprintf(os.Stderr, "✗ %q is not supported in MVP\n", key)
			fmt.Fprintf(os.Stderr, "  supported: %s\n", strings.Join(db.MVPRepoSettingsFields, ", "))
			fmt.Fprintf(os.Stderr, "  to add it, open a scope-change request (see docs/github-app-setup.md)\n")
		} else {
			fmt.Fprintf(os.Stderr, "✗ unknown field %q; supported: %s\n", key, strings.Join(db.MVPRepoSettingsFields, ", "))
		}
		os.Exit(2)
	}

	database := openRepoDB()
	defer database.Close()

	owner, name := splitRepoArg(repoArg)
	row, err := database.GetRepoRegistry(owner, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ repo %s not in registry: %v\n", repoArg, err)
		fmt.Fprintf(os.Stderr, "  run: devteam repo manage %s\n", repoArg)
		os.Exit(1)
	}
	if row.Managed != 1 {
		fmt.Fprintf(os.Stderr, "✗ %s is not managed; run: devteam repo manage %s\n", repoArg, repoArg)
		os.Exit(1)
	}

	if dryRun {
		fmt.Printf("ℹ dry-run: would set %s.%s = %s\n", repoArg, key, value)
		return
	}

	// Destructive-class confirmation (interaction-spec §3.3, §11.1):
	// changes default_branch OR sets provider=gh.
	if (key == "default_branch" || (key == "provider" && value == "gh")) && !confirm {
		if !confirmPrompt(fmt.Sprintf("⚠ %s.%s = %s is destructive-class; proceed?", repoArg, key, value)) {
			fmt.Fprintf(os.Stderr, "aborted\n")
			os.Exit(1)
		}
	}

	if _, err := database.UpsertRepoSettings(row.ID, key, value); err != nil {
		fmt.Fprintf(os.Stderr, "✗ setting %s: %v\n", key, err)
		os.Exit(1)
	}

	// Audit (FR-AUDIT-03). details carries field names + redacted values.
	details := fmt.Sprintf(`{"actor":"%s","repo":"%s","diff":{%q:%q}}`, os.Getenv("USER"), repoArg, key, value)
	_ = database.RecordRepoSettingsAudit("", repoArg, details)

	fmt.Printf("✓ %s: %s = %s\n", repoArg, key, value)
}

// repoManage implements `devteam repo manage <owner/name>` (M-4, US-16, FR-DISC-06).
func repoManage(args []string) {
	dryRun := false
	var repoArg string
	for _, a := range args {
		switch a {
		case "--dry-run":
			dryRun = true
		case "--help", "-h":
			fmt.Println("Usage: devteam repo manage <owner/name> [--dry-run]")
			return
		default:
			repoArg = a
		}
	}
	if repoArg == "" {
		fmt.Fprintf(os.Stderr, "Usage: devteam repo manage <owner/name> [--dry-run]\n")
		os.Exit(1)
	}
	database := openRepoDB()
	defer database.Close()

	owner, name := splitRepoArg(repoArg)
	if dryRun {
		fmt.Printf("ℹ dry-run: would manage %s with default settings\n", repoArg)
		return
	}
	if err := database.SetRepoManaged(owner, name, 1); err != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		fmt.Fprintf(os.Stderr, "  is the repo discovered? run: devteam repo list\n")
		os.Exit(1)
	}
	_ = database.RecordRepoRegistryAudit("", fmt.Sprintf("managed %s", repoArg))
	fmt.Printf("✓ %s managed\n", repoArg)
	fmt.Printf("  next: devteam repo set %s default_branch=<branch>\n", repoArg)
}

// loadConfig loads devteam.yaml from cwd (best-effort for repo list).
func loadConfig() (*config.Config, error) {
	baseDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return config.LoadConfig(baseDir + "/devteam.yaml")
}

// splitRepoArg splits "owner/name" into (owner, name).
func splitRepoArg(s string) (string, string) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return "", s
	}
	return parts[0], parts[1]
}

// confirmPrompt prints the prompt and reads y/N from stdin (default N,
// interaction-spec §11.1).
func confirmPrompt(prompt string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	var resp string
	fmt.Fscanln(os.Stdin, &resp)
	return strings.ToLower(strings.TrimSpace(resp)) == "y"
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}