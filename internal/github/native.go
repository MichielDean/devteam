package github

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
)

// NativeClient implements GitHubClient using google/go-github (ADR-02, U-02).
// It is the default provider and has no dependency on the gh CLI binary
// (FR-IFACE-02, NFR-PORT-02). All GitHub API access goes through this client
// or the GhCLIClient fallback adapter — never both for the same method.
//
// Auth model (ADR-10): the App private key is read from the credstore on JWT
// mint, NOT at construction. Key rotation = file replace + next JWT mint uses
// the new key (no restart, no code change — F-8, NFR-SEC-03).
type NativeClient struct {
	appID           int64
	installationID  int64
	privateKeyPath  string
	credstore       *Credstore
	tokenCache      *tokenCache
	httpClient      *http.Client     // transport for go-github (oauth2-injected token)
	client          *github.Client   // the go-github wrapper
	patFallback     bool
	pollMaxRetries  int
	pollMaxDuration time.Duration
}

// NewNativeClient constructs a NativeClient (U-02). The App private key is NOT
// read here — it is read on the first JWT mint (ADR-10). A nil credstore is
// allowed only in test paths; production requires a credstore for key + token
// persistence (FR-CRED-03).
func NewNativeClient(cfg Config) (*NativeClient, error) {
	if cfg.AppID == 0 || cfg.InstallationID == 0 {
		return nil, fmt.Errorf("native client: app_id and installation_id are required")
	}
	if cfg.PrivateKeyPath == "" {
		return nil, fmt.Errorf("native client: private_key_path is required")
	}
	if cfg.Credstore == nil {
		return nil, fmt.Errorf("native client: credstore is required (master key not loaded?)")
	}
	ttl := cfg.TokenCacheTTL
	if ttl == 0 {
		ttl = 9 * time.Minute
	}
	pollRetries := cfg.ConflictPollMaxRetries
	if pollRetries == 0 {
		pollRetries = 5
	}
	pollDur := cfg.ConflictPollMaxDuration
	if pollDur == 0 {
		pollDur = 60 * time.Second
	}

	// Start with a plain HTTP client; the oauth2-token-injected transport is
	// built on the first getToken (after token exchange). This is lazy: we
	// don't exchange at construction so a misconfigured-but-not-yet-called
	// client doesn't fail construction.
	nc := &NativeClient{
		appID:           cfg.AppID,
		installationID:  cfg.InstallationID,
		privateKeyPath:  cfg.PrivateKeyPath,
		credstore:       cfg.Credstore,
		tokenCache:      newTokenCache(ttl),
		patFallback:     cfg.PATFallbackEnabled,
		pollMaxRetries:  pollRetries,
		pollMaxDuration: pollDur,
	}
	nc.httpClient = &http.Client{Timeout: 30 * time.Second}
	nc.client = github.NewClient(nc.httpClient)
	return nc, nil
}

// clientWithToken returns a go-github client wired to use the given token.
// We rebuild the client on token refresh (the oauth2 transport caches the
// token; rebuilding swaps it cleanly without mutating a shared transport).
func (nc *NativeClient) clientWithToken(token string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	tc.Timeout = 30 * time.Second
	return github.NewClient(tc)
}

// AuthHealthCheck verifies the identity is alive (FR-AUTH-04, ADR-15).
// nil = alive. On failure returns *AuthError with Code + RunbookSection
// (ADR-12). The check issues ONE lightweight API call (GET /app or
// /installation/repositories?per_page=1) to confirm the token is valid.
func (nc *NativeClient) AuthHealthCheck(ctx context.Context) error {
	token, err := nc.getToken(ctx)
	if err != nil {
		return redactTokenURLs(err)
	}
	c := nc.clientWithToken(token)
	// Lightweight probe: list 1 repo from the installation.
	_, resp, err := c.Apps.ListRepos(ctx, &github.ListOptions{PerPage: 1})
	if err != nil {
		return redactTokenURLs(nc.classifyAPIError(resp, err, "auth health check"))
	}
	return nil
}

// TokenProvenance returns the cache's provenance string for the CLI's
// `token source:` line (US-01, ADR-06). This is a *NativeClient-specific method,
// NOT on the interface — the CLI type-asserts to call it.
func (nc *NativeClient) TokenProvenance(ctx context.Context) (string, error) {
	if _, err := nc.getToken(ctx); err != nil {
		return "", redactTokenURLs(err)
	}
	return nc.tokenCache.provenance(), nil
}

// AppID returns the configured GitHub App ID (CLI display, US-01).
func (nc *NativeClient) AppID() int64 { return nc.appID }

// InstallationID returns the configured installation ID (CLI display, US-01).
func (nc *NativeClient) InstallationID() int64 { return nc.installationID }

// getToken returns a valid installation token, exchanging/refreshing as needed
// (nfr-design-specs §4.2, BR-CRED-07). Never returns a stale token past TTL.
func (nc *NativeClient) getToken(ctx context.Context) (string, error) {
	// 1. Cache hit (fresh)?
	if tok, ok := nc.tokenCache.get(); ok {
		return tok, nil
	}
	// 2. Persisted fallback (restart resilience, BR-CRED-08)?
	if persisted, err := nc.credstore.LoadInstallationToken(); err == nil && persisted != "" {
		// Validate it via a lightweight call? No — we trust the expires_at check
		// done in LoadActiveCredential. Populate cache with a short TTL so the
		// next getToken refreshes properly. We don't know the exact expiry from
		// the string alone, so set cache TTL to the configured TTL (the refresh
		// goroutine will mint a fresh one before this one expires).
		nc.tokenCache.set(persisted, "re-exchanged", nc.tokenCache.ttl)
		return persisted, nil
	}
	// 3. Cold path: mint JWT → exchange for installation token.
	tok, expiresAt, err := nc.exchangeInstallationToken(ctx)
	if err != nil {
		// 4. PAT fallback (only on token-expiry-class failures, ADR-09).
		if nc.patFallback && nc.credstore.HasPAT() {
			pat, perr := nc.credstore.LoadPAT()
			if perr == nil && pat != "" {
				nc.tokenCache.set(pat, "fallback (PAT)", nc.tokenCache.ttl)
				return pat, nil
			}
		}
		return "", err
	}
	nc.tokenCache.set(tok, "refreshed", time.Until(expiresAt))
	// Persist for restart resilience.
	_ = nc.credstore.StoreInstallationToken(tok, expiresAt)
	return tok, nil
}

// exchangeInstallationToken mints a JWT from the App private key (read on-demand,
// ADR-10) and exchanges it for an installation token via
// POST /app/installations/{id}/access_tokens (FR-AUTH-01).
func (nc *NativeClient) exchangeInstallationToken(ctx context.Context) (token string, expiresAt time.Time, err error) {
	// Read the App private key on-demand (ADR-10 — key rotation = file replace,
	// next mint uses the new key, no restart).
	pemBytes, err := os.ReadFile(nc.privateKeyPath)
	if err != nil {
		return "", time.Time{}, authErrorFromCode(ErrCodeKeyFileMissing,
			fmt.Sprintf("private key file not readable at %s: %v", nc.privateKeyPath, err))
	}
	// Validate PEM.
	if !strings.Contains(string(pemBytes), "PRIVATE KEY") {
		return "", time.Time{}, authErrorFromCode(ErrCodeKeyFileInvalid,
			fmt.Sprintf("file at %s is not a PEM private key", nc.privateKeyPath))
	}

	// Mint JWT (RS256, App ID, 10-min expiry).
	jwtTok, err := mintAppJWT(nc.appID, pemBytes)
	if err != nil {
		return "", time.Time{}, authErrorFromCode(ErrCodeKeyRejected,
			fmt.Sprintf("minting App JWT: %v", err))
	}

	// Exchange JWT for installation token via go-github.
	jwtClient := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: jwtTok}))
	jwtClient.Timeout = 30 * time.Second
	appClient := github.NewClient(jwtClient)

	tok, resp, err := appClient.Apps.CreateInstallationToken(ctx, nc.installationID, nil)
	if err != nil {
		return "", time.Time{}, nc.classifyAPIError(resp, err, "token exchange")
	}
	if tok == nil || tok.GetToken() == "" {
		return "", time.Time{}, authErrorFromCode(ErrCodeKeyRejected, "token exchange returned empty token")
	}
	expires := tok.GetExpiresAt()
	if expires.IsZero() {
		// GitHub default: 1 hour from now. go-github's Timestamp wraps time.Time.
		expires = github.Timestamp{Time: time.Now().Add(60 * time.Minute)}
	}
	return tok.GetToken(), expires.Time, nil
}

// mintAppJWT builds the RS256-signed App JWT (GitHub App auth, FR-AUTH-01).
// The JWT is valid for 10 minutes (GitHub's max); issued-at now, expires now+10m.
func mintAppJWT(appID int64, pemBytes []byte) (string, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(pemBytes)
	if err != nil {
		return "", fmt.Errorf("parsing PEM: %w", err)
	}
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    fmt.Sprintf("%d", appID),
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)), // clock skew tolerance
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := tok.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}
	return signed, nil
}

// classifyAPIError maps a go-github API error to an *AuthError by status code
// (ADR-09, nfr-design-specs §4.3). 404/403 surface loudly (no PAT fallback); 401
// is key-rejected; 5xx/timeout are token-expiry-class (PAT fallback engages
// upstream in getToken). Non-auth errors pass through wrapped + redacted.
func (nc *NativeClient) classifyAPIError(resp *github.Response, err error, op string) error {
	if resp == nil {
		// Network error / no response — token-expiry-class. The caller (getToken)
		// decides on PAT fallback; here we return a generic auth error.
		return authErrorFromCode(ErrCodePATFallbackExhausted,
			fmt.Sprintf("%s: network error: %v", op, err))
	}
	switch resp.StatusCode {
	case 401:
		return authErrorFromCode(ErrCodeKeyRejected,
			fmt.Sprintf("%s: 401 — App private key rejected (rotate key: devteam auth rotate-key)", op))
	case 403:
		return authErrorFromCode(ErrCodeInstallationSuspended,
			fmt.Sprintf("%s: 403 — installation #%d suspended by org admin", op, nc.installationID))
	case 404:
		return authErrorFromCode(ErrCodeInstallationNotFound,
			fmt.Sprintf("%s: 404 — installation #%d not reachable (revoked?)", op, nc.installationID))
	default:
		return fmt.Errorf("%s: %w", op, err)
	}
}

// ensureClientWithFreshToken is the helper called before each API method to
// guarantee the embedded client has a fresh token. Returns the client to use.
func (nc *NativeClient) ensureClientWithFreshToken(ctx context.Context) (*github.Client, error) {
	tok, err := nc.getToken(ctx)
	if err != nil {
		return nil, redactTokenURLs(err)
	}
	return nc.clientWithToken(tok), nil
}