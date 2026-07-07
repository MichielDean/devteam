package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 18,
		Name:    "credentials_table",
		Up:      migration018CredentialsTable,
	})
}

// migration018CredentialsTable creates the credentials table for encrypted
// GitHub credentials (feature github-authorization-integration, U-06).
//
// Schema per nfr-design-specs §3.6 / infra-specs §5.1:
//   - kind: 'app_private_key' | 'installation_token' | 'pat'
//   - fingerprint: 16 hex chars (SHA-256 prefix of plaintext) — audit disambiguation, NOT reversible
//   - ciphertext: AES-256-GCM ciphertext (variable length = plaintext + 16 tag bytes)
//   - nonce: 12-byte GCM nonce (per-encrypt random via crypto/rand)
//   - expires_at: NULL for keys/PATs; set for installation_token (restart resilience, BR-CRED-08)
//   - rotated_at: NULL until superseded; the "active" row for a kind has rotated_at IS NULL
//
// The migration version is 018, NOT 017: version 17 ('repos_registry') is already
// applied (architecture-review B1 — the orphan created the legacy `repos` table).
// We leave the orphan alone and start the feature's migrations at 018.
//
// Idempotence (NFR-COMPAT-01, nfr-design-specs §8.3): CREATE TABLE IF NOT EXISTS
// + CREATE INDEX IF NOT EXISTS — re-running on a migrated DB is a no-op.
func migration018CredentialsTable(tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS credentials (
			id              SERIAL PRIMARY KEY,
			kind            TEXT NOT NULL,
			fingerprint     TEXT NOT NULL,
			ciphertext      BYTEA NOT NULL,
			nonce           BYTEA NOT NULL,
			expires_at      TIMESTAMP,
			created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			rotated_at      TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_credentials_kind_active
		 ON credentials(kind, rotated_at) WHERE rotated_at IS NULL`,
	}
	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("executing credentials migration statement: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}