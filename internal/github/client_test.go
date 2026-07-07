package github

import (
	"database/sql"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestCredstoreEncryptDecryptRoundTrip verifies AES-256-GCM round-trip
// (FR-CRED-01, U-06 acceptance). Uses an in-memory master key (no DB needed
// for the crypto round-trip).
func TestCredstoreEncryptDecryptRoundTrip(t *testing.T) {
	c := &Credstore{}
	// Set a 32-byte master key directly (bypasses env-var loading for unit test).
	copy(c.masterKey[:], []byte("0123456789ABCDEF0123456789ABCDEF"))

	plaintext := []byte("super-secret-app-private-key-PEM")
	ciphertext, nonce, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if len(nonce) != 12 {
		t.Errorf("nonce len = %d; want 12", len(nonce))
	}
	if len(ciphertext) != len(plaintext)+16 {
		t.Errorf("ciphertext len = %d; want %d (plaintext + 16 GCM tag)", len(ciphertext), len(plaintext)+16)
	}

	decrypted, err := c.Decrypt(ciphertext, nonce)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("round-trip mismatch: got %q; want %q", decrypted, plaintext)
	}
}

// TestCredstoreTamperedCiphertext verifies GCM tag mismatch surfaces as
// ErrCiphertextTampered (NFR-SEC-02 key-loss simulation, nfr-design-specs §3.4).
func TestCredstoreTamperedCiphertext(t *testing.T) {
	c := &Credstore{}
	copy(c.masterKey[:], []byte("0123456789ABCDEF0123456789ABCDEF"))

	ct, nonce, _ := c.Encrypt([]byte("plaintext"))
	// Flip a byte in the ciphertext.
	ct[0] ^= 0xFF
	_, err := c.Decrypt(ct, nonce)
	if !errors.Is(err, ErrCiphertextTampered) {
		t.Errorf("expected ErrCiphertextTampered; got %v", err)
	}
}

// TestCredstoreKeyMismatch verifies a different master key fails to decrypt
// (the key-loss recovery path: a re-generated master key cannot read old rows).
func TestCredstoreKeyMismatch(t *testing.T) {
	c1 := &Credstore{}
	copy(c1.masterKey[:], []byte("0123456789ABCDEF0123456789ABCDEF"))
	c2 := &Credstore{}
	copy(c2.masterKey[:], []byte("FEDCBA9876543210FEDCBA9876543210"))

	ct, nonce, _ := c1.Encrypt([]byte("plaintext"))
	_, err := c2.Decrypt(ct, nonce)
	if !errors.Is(err, ErrCiphertextTampered) {
		t.Errorf("expected ErrCiphertextTampered on key mismatch; got %v", err)
	}
}

// TestFingerprintIs16HexChars verifies the audit-disambiguation fingerprint
// (nfr-design-specs §3.6 — 16 hex chars, SHA-256 prefix).
func TestFingerprintIs16HexChars(t *testing.T) {
	fp := Fingerprint([]byte("some-credential"))
	if len(fp) != 16 {
		t.Errorf("fingerprint len = %d; want 16", len(fp))
	}
	for _, c := range fp {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("fingerprint contains non-hex char %q in %q", c, fp)
		}
	}
}

// TestRedactTokenURLs verifies the token-URL scrubbing (CR-03, FR-AUDIT-05,
// nfr-design-specs §2.2).
func TestRedactTokenURLs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "PAT in URL",
			input: "error cloning https://ghp_abcdef1234567890@github.com/owner/repo",
			want:  "error cloning https://[redacted]@github.com/owner/repo",
		},
		{
			name:  "installation token in URL",
			input: "failed: https://ghs_abcdefghijklmnop@github.com/org/repo.git",
			want:  "failed: https://[redacted]@github.com/org/repo.git",
		},
		{
			name:  "token query param",
			input: "GET https://api.github.com/?token=gho_secretvalue123",
			want:  "GET https://api.github.com/?token=[redacted]",
		},
		{
			name:  "bare ghp_ prefix",
			input: "leaked: ghp_1234567890abcdefghijklmnopqrstuv",
			want:  "leaked: ghp_[redacted]",
		},
		{
			name:  "no token",
			input: "ordinary error: connection refused",
			want:  "ordinary error: connection refused",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := redactString(tc.input)
			if got != tc.want {
				t.Errorf("redactString(%q)\n  got:  %q\n  want: %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestRedactTokenURLsIdempotent verifies double-application is identical to
// single (nfr-design-specs §2.2 — defensive against double-wrapping).
func TestRedactTokenURLsIdempotent(t *testing.T) {
	input := "https://ghp_secret@github.com/repo"
	once := redactString(input)
	twice := redactString(once)
	if once != twice {
		t.Errorf("redaction not idempotent:\n  once:  %q\n  twice: %q", once, twice)
	}
}

// TestRedactTokenURLsOnError verifies the error-wrapper form preserves
// errors.Is/As unwrapping (nfr-design-specs §2.2).
func TestRedactTokenURLsOnError(t *testing.T) {
	original := fmt.Errorf("clone failed: https://ghp_secret@github.com/repo: %w", sql.ErrConnDone)
	redacted := redactTokenURLs(original)
	if !errors.Is(redacted, sql.ErrConnDone) {
		t.Errorf("errors.Is broken by redaction: %v", redacted)
	}
	redactedStr := redacted.Error()
	if strings.Contains(redactedStr, "ghp_secret") {
		t.Errorf("token leaked in redacted error: %q", redactedStr)
	}
	if !strings.Contains(redactedStr, "[redacted]") {
		t.Errorf("redaction marker missing in: %q", redactedStr)
	}
}

// TestMergeableStateString verifies the lowercase banner form (interaction-spec §8.4).
func TestMergeableStateString(t *testing.T) {
	tests := []struct {
		state MergeableState
		want  string
	}{
		{Mergeable, "mergeable"},
		{Conflicting, "conflicting"},
		{MergeableUnknown, "unknown"},
	}
	for _, tc := range tests {
		if got := tc.state.String(); got != tc.want {
			t.Errorf("%d.String() = %q; want %q", tc.state, got, tc.want)
		}
	}
}

// TestAuthErrorCodeRunbookSection verifies every code maps to a section
// (BR-AUTH-05, nfr-design-specs §5.1).
func TestAuthErrorCodeRunbookSection(t *testing.T) {
	codes := []AuthErrorCode{
		ErrCodeInstallationNotFound,
		ErrCodeInstallationSuspended,
		ErrCodeKeyRejected,
		ErrCodeKeyFileMissing,
		ErrCodeKeyFileInvalid,
		ErrCodePATFallbackExhausted,
		ErrCodeCiphertextTampered,
		ErrCodeGhCLINotFound,
	}
	for _, c := range codes {
		section := c.RunbookSection()
		if section == "" || !strings.HasPrefix(section, "§") {
			t.Errorf("code %q: runbook section = %q; want non-empty §-prefixed", c, section)
		}
	}
}

// TestAuthErrorFormatting verifies the Error() string includes the runbook pointer.
func TestAuthErrorFormatting(t *testing.T) {
	ae := &AuthError{Code: ErrCodeInstallationNotFound, Message: "installation #42 not reachable"}
	s := ae.Error()
	if !strings.Contains(s, "installation_not_found") {
		t.Errorf("error string missing code: %q", s)
	}
	if !strings.Contains(s, "docs/github-app-setup.md") {
		t.Errorf("error string missing runbook pointer: %q", s)
	}
	if !strings.Contains(s, "§10") {
		t.Errorf("error string missing section: %q", s)
	}
}

// TestIsAuthError verifies the errors.As-based extraction.
func TestIsAuthError(t *testing.T) {
	original := authErrorFromCode(ErrCodeKeyRejected, "bad key")
	wrapped := fmt.Errorf("wrap: %w", original)

	ae, ok := IsAuthError(wrapped)
	if !ok {
		t.Fatalf("IsAuthError returned false for wrapped *AuthError")
	}
	if ae.Code != ErrCodeKeyRejected {
		t.Errorf("extracted code = %q; want %q", ae.Code, ErrCodeKeyRejected)
	}
}

// TestGhCLIClientErrUnsupported verifies the adapter returns ErrUnsupported for
// the 4 net-new methods (ADR-08, FR-IFACE-04, U-03 acceptance).
func TestGhCLIClientErrUnsupported(t *testing.T) {
	g := &GhCLIClient{baseDir: t.TempDir()}
	ctx := t.Context()

	if _, err := g.ListRepositories(ctx); !errors.Is(err, ErrUnsupported) {
		t.Errorf("ListRepositories: expected ErrUnsupported; got %v", err)
	}
	if _, err := g.GetMergeableState(ctx, RepoRef{Owner: "o", Name: "n"}, 1); !errors.Is(err, ErrUnsupported) {
		t.Errorf("GetMergeableState: expected ErrUnsupported; got %v", err)
	}
	if err := g.CreateBranch(ctx, RepoRef{Owner: "o", Name: "n"}, "b", "main"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("CreateBranch: expected ErrUnsupported; got %v", err)
	}
}

// TestGhCLIClientAuthHealthCheckGhAbsent verifies NFR-PORT-03: when provider=gh
// and gh is not on PATH, AuthHealthCheck returns a typed ErrCodeGhCLINotFound
// error (not a panic, not an exec error leak).
func TestGhCLIClientAuthHealthCheckGhAbsent(t *testing.T) {
	// Sanitize PATH so gh is not findable. This test only runs if gh is NOT
	// installed at the test host's PATH (most CI). If gh IS installed, skip.
	if _, err := exec.LookPath("gh"); err == nil {
		t.Skip("gh is on PATH; cannot test gh-absent detection on this host")
	}
	g := &GhCLIClient{baseDir: t.TempDir()}
	err := g.AuthHealthCheck(t.Context())
	ae, ok := IsAuthError(err)
	if !ok {
		t.Fatalf("expected *AuthError; got %T: %v", err, err)
	}
	if ae.Code != ErrCodeGhCLINotFound {
		t.Errorf("code = %q; want %q", ae.Code, ErrCodeGhCLINotFound)
	}
}

// TestTokenCacheTTL verifies the cache returns fresh hits and expires.
func TestTokenCacheTTL(t *testing.T) {
	tc := newTokenCache(50 * time.Millisecond)
	tc.set("token-A", "refreshed", 50*time.Millisecond)
	if got, ok := tc.get(); !ok || got != "token-A" {
		t.Errorf("fresh get: got %q ok=%v; want token-A true", got, ok)
	}
	time.Sleep(60 * time.Millisecond)
	if _, ok := tc.get(); ok {
		t.Errorf("expired get: expected ok=false")
	}
}

// TestTokenCacheProvenance verifies the "cached" downgrade on warm reads
// (interaction-spec §8.1).
func TestTokenCacheProvenance(t *testing.T) {
	tc := newTokenCache(time.Minute)
	tc.set("tok", "refreshed", time.Minute)
	// First read within 1s → "refreshed".
	if got := tc.provenance(); got != "refreshed" {
		t.Errorf("immediate provenance = %q; want refreshed", got)
	}
	// Wait past 1s → "cached".
	time.Sleep(1100 * time.Millisecond)
	if got := tc.provenance(); got != "cached" {
		t.Errorf("warm provenance = %q; want cached", got)
	}
}