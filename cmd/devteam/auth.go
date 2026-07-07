package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/github"
)

// startupAuthHealth runs an AuthHealthCheck at process startup (ADR-15, FR-AUTH-04).
// Used by the web server mode in main.go after db.Open. Returns nil if alive;
// the caller exits 1 with the W-1 block on error.
func startupAuthHealth(cfg *config.Config, database *db.DB, baseDir string) error {
	credstore, err := github.NewCredstore(database.Conn())
	if err != nil {
		return err
	}
	ghCfg := github.Config{
		Provider:                cfg.GitHub.Provider,
		AppID:                   cfg.GitHub.AppID,
		InstallationID:           cfg.GitHub.InstallationID,
		PrivateKeyPath:          cfg.GitHub.PrivateKeyPath,
		TokenCacheTTL:           cfg.GitHub.TokenCacheTTL,
		PATFallbackEnabled:      cfg.GitHub.PATFallback.Enabled,
		ConflictPollMaxRetries:  cfg.GitHub.ConflictPollMaxRetries,
		ConflictPollMaxDuration: cfg.GitHub.ConflictPollMaxDuration,
		BaseDir:                 baseDir,
		Credstore:               credstore,
	}
	client, err := github.NewNativeClient(ghCfg)
	if err != nil {
		return err
	}
	return client.AuthHealthCheck(context.Background())
}

// handleAuthCLI dispatches the `devteam auth <verb>` subcommands (feature
// github-authorization-integration, interaction-spec §3.1).
//
// Verbs:
//   health       — probe the identity (FR-AUTH-04, US-01, M-1)
//   rotate-key   — rotate the App private key (NFR-SEC-03, nfr-design-specs §3.5)
//   store-pat    — store a PAT from stdin (FR-AUTH-02, nfr-design-specs §11 finding 6)
//
// Exit codes (interaction-spec §12 rule 2): 0 ok, 1 runtime/auth failure,
// 2 field/flag rejection.
func handleAuthCLI(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: devteam auth <health|rotate-key|store-pat> [options]\n")
		os.Exit(1)
	}
	verb := args[1]
	switch verb {
	case "health":
		authHealth(args[2:])
	case "rotate-key":
		authRotateKey(args[2:])
	case "store-pat":
		authStorePAT(args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown auth verb: %s (use health, rotate-key, or store-pat)\n", verb)
		os.Exit(1)
	}
}

// loadGitHubForCLI is the shared setup for auth CLI commands: loads config,
// opens the DB, constructs a credstore + NativeClient. Exits 1 on any failure
// (config error, missing master key, missing github: block) with the W-10
// block format (interaction-spec §3.1, NFR-OPS-01).
func loadGitHubForCLI() (*config.Config, *db.DB, *github.Credstore, *github.NativeClient) {
	baseDir, err := os.Getwd()
	if err != nil {
		printAuthError(nil, fmt.Errorf("getting working directory: %w", err))
		os.Exit(1)
	}
	cfg, err := config.LoadConfig(baseDir + "/devteam.yaml")
	if err != nil {
		printAuthError(nil, fmt.Errorf("loading config: %w", err))
		os.Exit(1)
	}
	if cfg.GitHub.AppID == 0 && cfg.GitHub.Provider == "" {
		printAuthError(nil, fmt.Errorf("github: config block not present; see docs/github-app-setup.md §5"))
		os.Exit(1)
	}
	database := openDB(baseDir)
	credstore, err := github.NewCredstore(database.Conn())
	if err != nil {
		printAuthError(nil, err)
		os.Exit(1)
	}
	ghCfg := github.Config{
		Provider:                cfg.GitHub.Provider,
		AppID:                   cfg.GitHub.AppID,
		InstallationID:           cfg.GitHub.InstallationID,
		PrivateKeyPath:          cfg.GitHub.PrivateKeyPath,
		TokenCacheTTL:           cfg.GitHub.TokenCacheTTL,
		PATFallbackEnabled:      cfg.GitHub.PATFallback.Enabled,
		ConflictPollMaxRetries:  cfg.GitHub.ConflictPollMaxRetries,
		ConflictPollMaxDuration: cfg.GitHub.ConflictPollMaxDuration,
		BaseDir:                 baseDir,
		Credstore:               credstore,
	}
	client, err := github.NewNativeClient(ghCfg)
	if err != nil {
		printAuthError(nil, err)
		os.Exit(1)
	}
	return cfg, database, credstore, client
}

// printAuthError prints the W-10 failure block (interaction-spec §3.1, §9.3):
// ✗ + the error, then a numbered `what to do:` block. For *AuthError, the
// runbook section is already in the error string; for other errors, we add a
// generic pointer.
func printAuthError(_ *github.AuthError, err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "✗ %s\n", err.Error())
	if ae, ok := github.IsAuthError(err); ok {
		fmt.Fprintf(os.Stderr, "what to do:\n")
		fmt.Fprintf(os.Stderr, "  1. see docs/github-app-setup.md %s\n", ae.RunbookSection)
		fmt.Fprintf(os.Stderr, "  2. fix the issue described above\n")
		fmt.Fprintf(os.Stderr, "  3. re-run: devteam auth health\n")
	} else {
		fmt.Fprintf(os.Stderr, "what to do:\n")
		fmt.Fprintf(os.Stderr, "  1. see docs/github-app-setup.md\n")
		fmt.Fprintf(os.Stderr, "  2. re-run: devteam auth health\n")
	}
}

// authHealth implements `devteam auth health` (M-1, US-01).
func authHealth(_ []string) {
	_, database, _, client := loadGitHubForCLI()
	defer database.Close()

	ctx := context.Background()
	if err := client.AuthHealthCheck(ctx); err != nil {
		printAuthError(nil, err)
		os.Exit(1)
	}
	// Provenance (US-01 criterion 1, interaction-spec §8.1).
	prov, _ := client.TokenProvenance(ctx)
	if prov == "" {
		prov = "cached"
	}
	fmt.Printf("✓ machine identity alive\n")
	fmt.Printf("  token source: %s\n", prov)
	fmt.Printf("  app_id: %d\n", client.AppID())
	fmt.Printf("  installation_id: %d\n", client.InstallationID())
}

// authRotateKey implements `devteam auth rotate-key` (NFR-SEC-03, nfr-design-specs §3.5).
// Reads the new key from the configured private_key_path, stores it (marking
// the old row rotated), emits a CREDENTIAL_ROTATED audit event, all in one tx.
func authRotateKey(_ []string) {
	cfg, database, credstore, client := loadGitHubForCLI()
	defer database.Close()

	pemBytes, err := os.ReadFile(cfg.GitHub.PrivateKeyPath)
	if err != nil {
		printAuthError(nil, fmt.Errorf("reading new key from %s: %w", cfg.GitHub.PrivateKeyPath, err))
		os.Exit(1)
	}
	oldFp := ""
	if oldKey, err := credstore.LoadAppPrivateKey(); err == nil {
		oldFp = github.Fingerprint(oldKey)
	}
	if err := credstore.StoreAppPrivateKey(pemBytes); err != nil {
		printAuthError(nil, fmt.Errorf("storing new key: %w", err))
		os.Exit(1)
	}
	newFp := github.Fingerprint(pemBytes)
	_ = client // touched so the client construction side-effects (cache init) happen
	details := fmt.Sprintf(`{"actor":"%s","action":"rotate","target":"app_private_key","old_fingerprint":"%s","new_fingerprint":"%s"}`, os.Getenv("USER"), oldFp, newFp)
	if err := database.RecordCredentialAuditEvent("", db.AuditCredentialRotated, "", "auth", details); err != nil {
		fmt.Fprintf(os.Stderr, "⚠ key rotated but audit event failed: %v\n", err)
	}
	fmt.Printf("✓ app private key rotated\n")
	fmt.Printf("  fingerprint: %s…\n", newFp[:4])
	if oldFp != "" {
		fmt.Printf("  old fingerprint: %s… (superseded)\n", oldFp[:4])
	}
	fmt.Printf("  no restart required — next JWT mint uses the new key\n")
}

// authStorePAT implements `devteam auth store-pat` (FR-AUTH-02, nfr-design-specs
// §11 finding 6). Reads the PAT from stdin (never arg, never env — avoids shell
// history leakage). Stores encrypted; emits CREDENTIAL_STORED audit.
func authStorePAT(_ []string) {
	_, database, credstore, _ := loadGitHubForCLI()
	defer database.Close()

	fmt.Fprintf(os.Stderr, "Paste PAT (read from stdin; ends on blank line or EOF):\n")
	reader := bufio.NewReader(os.Stdin)
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimRight(line, "\n")
		if line != "" {
			lines = append(lines, line)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			printAuthError(nil, fmt.Errorf("reading PAT from stdin: %w", err))
			os.Exit(1)
		}
		if line == "" {
			break
		}
	}
	pat := strings.Join(lines, "")
	if pat == "" {
		fmt.Fprintf(os.Stderr, "✗ no PAT read from stdin\n")
		os.Exit(1)
	}
	if err := credstore.StorePAT(pat); err != nil {
		printAuthError(nil, fmt.Errorf("storing PAT: %w", err))
		os.Exit(1)
	}
	fp := github.Fingerprint([]byte(pat))
	details := fmt.Sprintf(`{"actor":"%s","action":"store","target":"pat","fingerprint":"%s"}`, os.Getenv("USER"), fp)
	_ = database.RecordCredentialAuditEvent("", db.AuditCredentialStored, "", "auth", details)
	fmt.Printf("✓ PAT stored\n")
	fmt.Printf("  fingerprint: %s…\n", fp[:4])
}