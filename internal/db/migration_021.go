package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 21,
		Name:    "chat_persistence",
		Up:      migration021ChatPersistence,
	})
}

// migration021ChatPersistence adds the chat persistence schema (DR-1, DR-2):
//   - chat_sessions: one row per chat conversation
//   - chat_messages: messages within a session (user/expert/tool roles)
//   - audit_events.session_id, actor, feature_id_*: nullable columns so
//     chat_cli_exec audit events can link to a chat session and record the
//     "expert" actor, with an optional __chat__ sentinel feature row when
//     no real feature is involved.
//
// Additive only (C1): new tables + nullable columns. Existing audit reads
// are unaffected. Forward-only (R-DEP-1): no Down. A failed 021 rolls back
// transactionally to pre-021 state.
//
// Version 21 (not 18): versions 18-20 were already claimed on the live DB
// by sibling features (github-authorization-integration: 18/20;
// settings-and-admin-ui: 19) before this feature merged. Renumbered from
// the constructor's original 18 to the next free integer to avoid the
// migration-runner silent-skip (D-3.6-1).
func migration021ChatPersistence(tx *sql.Tx) error {
	statements := []string{
		// chat_sessions — UUID PK, selected_provider nullable (no provider = default)
		`CREATE TABLE IF NOT EXISTS chat_sessions (
			id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			title             TEXT NOT NULL DEFAULT 'New Chat',
			selected_provider TEXT,
			created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,

		// chat_messages — FK to chat_sessions with cascade delete; role is constrained
		// to user|expert|tool; citations is a jsonb array of {file, section, lines?}
		`CREATE TABLE IF NOT EXISTS chat_messages (
			id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			session_id     UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
			role           TEXT NOT NULL CHECK (role IN ('user','expert','tool')),
			content        TEXT NOT NULL,
			provider_used  TEXT,
			created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
			citations      JSONB
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_session_created
			ON chat_messages(session_id, created_at)`,

		// audit_events additive columns — nullable so existing rows/reads unaffected.
		// session_id links a chat_cli_exec event to the chat session that triggered it.
		// actor records "expert" for chat-driven ops (vs pipeline-driven ops which leave it NULL).
		// feature_id_chat allows chat_cli_exec events with no real feature to use __chat__.
		// We use a separate column name (feature_id_chat) to avoid clashing with the
		// existing feature_id NOT NULL column on audit_events.
		`ALTER TABLE audit_events
			ADD COLUMN IF NOT EXISTS session_id UUID,
			ADD COLUMN IF NOT EXISTS actor TEXT,
			ADD COLUMN IF NOT EXISTS feature_id_chat TEXT`,

		`CREATE INDEX IF NOT EXISTS idx_audit_events_session
			ON audit_events(session_id) WHERE session_id IS NOT NULL`,

		// __chat__ sentinel feature row — parent for chat_cli_exec audit events
		// that have no real feature (pure methodology Q&A in the chat UI).
		// Uses a fixed text id. ON CONFLICT DO NOTHING makes the migration idempotent.
		`INSERT INTO features (id, title, current_phase, status, priority, intake_path, spec_dir, created_at, updated_at, recirculation_count, scope, depth, test_strategy)
		 VALUES ('__chat__', '__chat__ sentinel', 'operation', 'sentinel', 0, 'loose_idea', '',
		         now(), now(), 0, 'feature', 'minimal', 'standard')
		 ON CONFLICT (id) DO NOTHING`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("executing statement: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}