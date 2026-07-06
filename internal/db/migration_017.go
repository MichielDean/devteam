package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 17,
		Name:    "provider_config",
		Up:      migration017ProviderConfig,
	})
}

// migration017ProviderConfig creates the provider/tier config tables, seeds the
// 4 provider presets + tier rows, and inserts the sentinel __platform__ feature
// row that platform-global audit events (RecordAuditEvent) reference.
//
// Resolves architecture-review (stage 3.3) F-01 (BLOCKING): both audit tables
// (events, audit_events) FK-constrain feature_id to features(id); empty-string
// feature_id violates the FK. The sentinel row lets RecordAuditEvent("__platform__",
// ...) succeed for config mutations without a new audit table.
//
// Postgres-native DDL. Additive (no DROP/ALTER on existing tables). Idempotent
// (CREATE TABLE IF NOT EXISTS + ON CONFLICT DO NOTHING). Runs in one transaction
// (the migration runner wraps Up in tx.Begin/Commit, migrations.go:66-83).
func migration017ProviderConfig(tx *sql.Tx) error {
	statements := []string{
		// providers — the provider registry
		`CREATE TABLE IF NOT EXISTS providers (
			name              TEXT PRIMARY KEY,
			display_name      TEXT NOT NULL,
			enabled           INTEGER NOT NULL DEFAULT 0,
			base_url          TEXT NOT NULL DEFAULT '',
			api_key_env       TEXT NOT NULL DEFAULT '',
			default_model_id  TEXT NOT NULL DEFAULT '',
			npm_adapter       TEXT NOT NULL DEFAULT '@ai-sdk/openai-compatible',
			env_var_supported INTEGER NOT NULL DEFAULT 1,
			preset_id         TEXT NOT NULL DEFAULT 'custom'
		)`,

		// provider_models — models per provider
		`CREATE TABLE IF NOT EXISTS provider_models (
			provider_name  TEXT NOT NULL,
			model_id        TEXT NOT NULL,
			friendly_name   TEXT NOT NULL,
			PRIMARY KEY (provider_name, model_id),
			FOREIGN KEY (provider_name) REFERENCES providers(name) ON DELETE CASCADE
		)`,

		// tier_models — tier → model per provider
		`CREATE TABLE IF NOT EXISTS tier_models (
			tier            TEXT NOT NULL,
			provider_name   TEXT NOT NULL,
			model_id        TEXT NOT NULL,
			PRIMARY KEY (tier, provider_name),
			FOREIGN KEY (provider_name) REFERENCES providers(name) ON DELETE CASCADE
		)`,

		// role_overrides — per-role explicit provider+model
		`CREATE TABLE IF NOT EXISTS role_overrides (
			role            TEXT PRIMARY KEY,
			provider_name   TEXT NOT NULL,
			model_id        TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (provider_name) REFERENCES providers(name) ON DELETE CASCADE
		)`,

		// Seed: 4 provider presets.
		// Anthropic + Ollama enabled (MVP per human input H2).
		// OpenAI + Copilot disabled (fast-follow per H2).
		// $VAR reference convention (ADR-003): api_key_env stores "$ANTHROPIC_API_KEY", not the key.
		// Copilot: env_var_supported=0 → reduced UI card (ADR-005), no key entry.
		`INSERT INTO providers (name, display_name, enabled, base_url, api_key_env, default_model_id, npm_adapter, env_var_supported, preset_id)
		VALUES
			('anthropic',    'Anthropic',     1, 'https://api.anthropic.com/v1', '$ANTHROPIC_API_KEY', 'claude-opus-4',   '@ai-sdk/openai-compatible', 1, 'anthropic'),
			('ollama-cloud', 'Ollama Cloud',   1, '',                              '$OLLAMA_API_KEY',    'glm-5.2:cloud',  '@ai-sdk/openai-compatible', 1, 'ollama-cloud'),
			('openai',       'OpenAI',         0, 'https://api.openai.com/v1',    '$OPENAI_API_KEY',    'gpt-4o',         '@ai-sdk/openai-compatible', 1, 'openai'),
			('copilot',      'GitHub Copilot', 0, '',                              '',                   '',               '@ai-sdk/openai-compatible', 0, 'copilot')
		ON CONFLICT (name) DO NOTHING`,

		// Seed: tier rows (opus → anthropic/claude-opus-4, sonnet → ollama-cloud/glm-5.2:cloud).
		// These match the existing agentRoster tiers (role.go:20-36, opus/sonnet).
		// opus→anthropic because Anthropic is the higher-tier provider; sonnet→ollama-cloud
		// preserves the as-is dispatch behavior (every agent currently gets glm-5.2:cloud).
		`INSERT INTO tier_models (tier, provider_name, model_id)
		VALUES
			('opus',   'anthropic',    'claude-opus-4'),
			('sonnet', 'ollama-cloud', 'glm-5.2:cloud')
		ON CONFLICT (tier, provider_name) DO NOTHING`,

		// Seed: provider_models for the presets (so the TierMatrix UI has model options).
		`INSERT INTO provider_models (provider_name, model_id, friendly_name)
		VALUES
			('anthropic',    'claude-opus-4',  'Claude Opus 4'),
			('ollama-cloud', 'glm-5.2:cloud', 'GLM 5.2 Cloud'),
			('openai',       'gpt-4o',        'GPT-4o')
		ON CONFLICT (provider_name, model_id) DO NOTHING`,

		// Sentinel feature row for platform-global audit events.
		// Resolves architecture-review F-01: RecordAuditEvent("__platform__", ...) must
		// satisfy the audit_events.feature_id FK to features(id). The sentinel is inert:
		//   - priority=0 (sorts below real features, which default to 3)
		//   - status='draft', current_phase='construction' (never dispatched/advanced)
		//   - spec_dir='platform' (non-path placeholder; no spec artifacts)
		// ON CONFLICT DO NOTHING: re-running migration 017 (if ever re-applied) is a no-op.
		`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, worktree_dir, created_at, updated_at, recirculation_count)
		VALUES ('__platform__', 'Platform Configuration', 'construction', 'draft', 0, 'platform', 'platform', '', NOW(), NOW(), 0)
		ON CONFLICT (id) DO NOTHING`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migration 017 executing statement: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}