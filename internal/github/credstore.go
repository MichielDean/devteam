// Package github is the sole GitHub-API boundary for the devteam platform
// (feature github-authorization-integration, ADR-01). All GitHub API access —
// authentication, repository discovery, PR operations, merge-conflict polling —
// goes through this package. No other package imports google/go-github,
// golang.org/x/oauth2, or the App private key material.
//
// The package owns:
//   - GitHubClient interface + domain types (U-01)
//   - NativeClient (go-github, default) + GhCLIClient (gh-CLI fallback adapter) (U-02, U-03)
//   - Encrypted credential store at rest (U-06)
//   - Repository discovery + per-repo settings reader (U-04)
//   - Branch/PR ops + mergeable-state poll (U-08, U-09)
//   - Audit redaction wrapper (U-11)
//
// Build invariant (ADR-18): internal/github/ is the sole importer of
// google/go-github and golang.org/x/oauth2. A test in imports_test.go asserts
// this. AES-GCM uses stdlib crypto/aes + crypto/cipher (nfr-design-specs §3.1
// refinement: x/crypto is NOT imported by this feature).
package github

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// Credstore encrypts and decrypts GitHub credentials at rest using AES-256-GCM
// with a master key sourced from an env-pointed file path (feature
// github-authorization-integration, U-06, FR-CRED-01/02, C-04, C-12).
//
// The master key is loaded ONCE at process startup and held in an unexported
// field (nfr-design-specs §3.2). It is never logged, never in devteam.yaml,
// never in any DB table (BR-CRED-03). Key loss is documented-unrecoverable
// (NFR-SEC-02); there is no escrow.
//
// The credstore is the SOLE reader of decrypted credential values (FR-CRED-03).
// NativeClient calls credstore.Decrypt on JWT mint (ADR-10); no other package
// reads plaintext. Static check: grep internal/ for direct credentials-ciphertext
// reads outside this file → zero hits.
type Credstore struct {
	db       *sql.DB
	masterKey [32]byte // AES-256 key; loaded once at startup, never re-read
}

// CredentialKind enumerates the `kind` values stored in the credentials table.
const (
	CredKindAppPrivateKey    = "app_private_key"
	CredKindInstallationToken = "installation_token"
	CredKindPAT               = "pat"
)

// ErrKeyFileMissing is returned when the master key file (or env var) is not
// found at the configured path (nfr-design-specs §3.2). It is an *AuthError so
// callers can branch via errors.As and the CLI can format the W-10 block.
var ErrKeyFileMissing = errors.New("master key file not found")

// ErrKeyFileInvalid is returned when the master key file is not exactly 32 bytes
// (a trailing newline is the common cause — the operator must `truncate -s 32`).
var ErrKeyFileInvalid = errors.New("master key file is not 32 bytes")

// ErrCiphertextTampered is returned when GCM tag verification fails — the
// ciphertext was modified, or the master key does not match the one used to
// encrypt (NFR-SEC-02 key-loss simulation path, nfr-design-specs §3.4).
var ErrCiphertextTampered = errors.New("ciphertext tampered or master key mismatch")

// ErrNoActiveCredential is returned when no active (rotated_at IS NULL) row
// exists for the requested kind.
var ErrNoActiveCredential = errors.New("no active credential of that kind")

// NewCredstore loads the master key from the env-pointed file path
// (DEVTEAM_MASTER_KEY_FILE) or, as a fallback, the DEVTEAM_MASTER_KEY env var
// (base64 or raw 32 bytes). It returns an error if the key is missing or
// not 32 bytes; the caller (cmd/devteam/main.go) fails fast at startup with
// a runbook pointer (FR-CRED-01, NFR-OPS-01).
//
// db may be nil — a nil-DB credstore can still load the master key (used by
// the auth-health CLI path before the DB is opened in some test scenarios).
// Encrypt/Decrypt require a non-nil DB.
func NewCredstore(db *sql.DB) (*Credstore, error) {
	key, err := loadMasterKey()
	if err != nil {
		return nil, err
	}
	var k [32]byte
	copy(k[:], key)
	// Zero the source slice defensively.
	for i := range key {
		key[i] = 0
	}
	return &Credstore{db: db, masterKey: k}, nil
}

// loadMasterKey reads the master key from DEVTEAM_MASTER_KEY_FILE (preferred,
// 32 bytes raw) or DEVTEAM_MASTER_KEY (fallback, base64 or raw 32 bytes).
// nfr-design-specs §3.2.
func loadMasterKey() ([]byte, error) {
	if path := os.Getenv("DEVTEAM_MASTER_KEY_FILE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrKeyFileMissing, path, err)
		}
		if len(data) != 32 {
			return nil, fmt.Errorf("%w: %s is %d bytes; expected 32 (truncate -s 32 %s)", ErrKeyFileInvalid, path, len(data), path)
		}
		return data, nil
	}
	if v := os.Getenv("DEVTEAM_MASTER_KEY"); v != "" {
		// Try base64 first, then raw.
		if decoded, err := base64.StdEncoding.DecodeString(v); err == nil && len(decoded) == 32 {
			return decoded, nil
		}
		if len(v) == 32 {
			return []byte(v), nil
		}
		return nil, fmt.Errorf("DEVTEAM_MASTER_KEY is set but is %d bytes (not 32 raw or base64 of 32); expected 32", len(v))
	}
	return nil, fmt.Errorf("%w: set DEVTEAM_MASTER_KEY_FILE to the master key file path; see docs/github-app-setup.md §3", ErrKeyFileMissing)
}

// Encrypt encrypts plaintext with AES-256-GCM using a fresh random 12-byte
// nonce. Returns (ciphertext, nonce). The ciphertext includes the GCM auth tag
// (16 bytes appended, stdlib behavior). The caller persists both columns.
//
// The API makes misuse impossible: there is no nonce-input parameter (nfr-design-specs
// §3.3, principle 3 — error prevention). Random nonce via crypto/rand; no reuse.
func (c *Credstore) Encrypt(plaintext []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(c.masterKey[:])
	if err != nil {
		return nil, nil, fmt.Errorf("creating AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("creating GCM: %w", err)
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("generating nonce: %w", err)
	}
	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// Decrypt decrypts ciphertext+nonce with AES-256-GCM. Returns the plaintext.
// On tag mismatch (ciphertext tampered or key mismatch) returns ErrCiphertextTampered.
func (c *Credstore) Decrypt(ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.masterKey[:])
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("nonce is %d bytes; expected %d", len(nonce), gcm.NonceSize())
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCiphertextTampered, err)
	}
	return plain, nil
}

// Fingerprint returns the 16-hex-char SHA-256 prefix of plaintext. This is
// NOT a security boundary — it is audit-disambiguation only ("is this the same
// key as last week?"). It is computationally irreversible (nfr-design-specs §3.6).
func Fingerprint(plaintext []byte) string {
	sum := sha256.Sum256(plaintext)
	return hex.EncodeToString(sum[:8]) // 8 bytes = 16 hex chars
}

// StoreCredential encrypts plaintext, inserts a credentials row, and marks the
// previous active row of the same kind as rotated (sets rotated_at=now). All in
// one transaction (NFR-SEC-03 rotation procedure, nfr-design-specs §3.5).
//
// expiresAt is set for installation_token rows (restart resilience, BR-CRED-08);
// pass a zero time for keys/PATs (NULL expires_at).
//
// Returns the new row's ID. The caller emits the CREDENTIAL_STORED or
// CREDENTIAL_ROTATED audit event (U-11) — this helper does NOT audit directly
// (separation: credstore does crypto + persistence; the audit layer is above).
func (c *Credstore) StoreCredential(kind string, plaintext []byte, expiresAt time.Time) (int64, error) {
	if c.db == nil {
		return 0, fmt.Errorf("credstore: db is nil (cannot store credential)")
	}
	ciphertext, nonce, err := c.Encrypt(plaintext)
	if err != nil {
		return 0, fmt.Errorf("encrypting %s: %w", kind, err)
	}
	fp := Fingerprint(plaintext)

	tx, err := c.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("beginning credential store tx: %w", err)
	}

	// Mark previous active row rotated.
	if _, err := tx.Exec(
		`UPDATE credentials SET rotated_at = CURRENT_TIMESTAMP WHERE kind = ? AND rotated_at IS NULL`,
		kind,
	); err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("rotating previous %s: %w", kind, err)
	}

	var expires interface{}
	if !expiresAt.IsZero() {
		expires = expiresAt
	}
	res, err := tx.Exec(
		`INSERT INTO credentials (kind, fingerprint, ciphertext, nonce, expires_at, created_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		kind, fp, ciphertext, nonce, expires,
	)
	if err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("inserting %s credential: %w", kind, err)
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing credential store tx: %w", err)
	}
	return id, nil
}

// LoadActiveCredential returns the decrypted plaintext of the active (rotated_at
// IS NULL, and for installation_token expires_at > now) credential of the given
// kind. Returns ErrNoActiveCredential if none exists.
func (c *Credstore) LoadActiveCredential(kind string) ([]byte, error) {
	if c.db == nil {
		return nil, fmt.Errorf("credstore: db is nil (cannot load credential)")
	}
	var ciphertext, nonce []byte
	var expiresAt sql.NullTime
	err := c.db.QueryRow(
		`SELECT ciphertext, nonce, expires_at FROM credentials
		 WHERE kind = ? AND rotated_at IS NULL
		 ORDER BY id DESC LIMIT 1`,
		kind,
	).Scan(&ciphertext, &nonce, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, ErrNoActiveCredential
	}
	if err != nil {
		return nil, fmt.Errorf("loading active %s: %w", kind, err)
	}
	// For installation tokens, refuse an expired persisted token.
	if kind == CredKindInstallationToken && expiresAt.Valid && time.Now().After(expiresAt.Time) {
		return nil, ErrNoActiveCredential
	}
	return c.Decrypt(ciphertext, nonce)
}

// LoadInstallationToken returns the persisted installation token if not expired
// (BR-CRED-08 restart resilience). Returns ErrNoActiveCredential if missing/expired.
func (c *Credstore) LoadInstallationToken() (string, error) {
	plain, err := c.LoadActiveCredential(CredKindInstallationToken)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// LoadAppPrivateKey returns the active App private key (PEM bytes).
func (c *Credstore) LoadAppPrivateKey() ([]byte, error) {
	return c.LoadActiveCredential(CredKindAppPrivateKey)
}

// LoadPAT returns the active PAT (fallback auth, FR-AUTH-02).
func (c *Credstore) LoadPAT() (string, error) {
	plain, err := c.LoadActiveCredential(CredKindPAT)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// HasPAT reports whether a PAT is stored (used to decide whether PAT fallback
// can engage — nfr-design-specs §4.3, ADR-09).
func (c *Credstore) HasPAT() bool {
	_, err := c.LoadPAT()
	return err == nil
}

// StoreInstallationToken persists a freshly-exchanged installation token with
// its GitHub-reported expiry. Called by the token cache on every exchange
// (nfr-design-specs §4.1).
func (c *Credstore) StoreInstallationToken(token string, expiresAt time.Time) error {
	_, err := c.StoreCredential(CredKindInstallationToken, []byte(token), expiresAt)
	return err
}

// StoreAppPrivateKey persists the App private key (PEM). Called by
// `devteam auth rotate-key` (NFR-SEC-03, nfr-design-specs §3.5).
func (c *Credstore) StoreAppPrivateKey(pemBytes []byte) error {
	_, err := c.StoreCredential(CredKindAppPrivateKey, pemBytes, time.Time{})
	return err
}

// StorePAT persists a PAT (read from stdin by `devteam auth store-pat`,
// nfr-design-specs §11 finding 6).
func (c *Credstore) StorePAT(pat string) error {
	_, err := c.StoreCredential(CredKindPAT, []byte(pat), time.Time{})
	return err
}