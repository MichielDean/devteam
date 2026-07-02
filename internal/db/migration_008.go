package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 8,
		Name:    "team_knowledge_and_rules",
		Up:      migration008KnowledgeRules,
	})
}

// migration008KnowledgeRules creates:
//   - team_knowledge: two-tier knowledge system (methodology in role files,
//     team knowledge in DB). Per-agent customization loaded into context at dispatch.
//   - rules: learning loop output. Gate rejections become behavioral rules
//     that constrain future agent behavior.
func migration008KnowledgeRules(tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS team_knowledge (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_name TEXT NOT NULL,
			topic TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			UNIQUE(agent_name, topic)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_team_knowledge_agent ON team_knowledge(agent_name)`,

		`CREATE TABLE IF NOT EXISTS rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT DEFAULT '',
			agent_name TEXT NOT NULL,
			stage_id TEXT DEFAULT '',
			rule_text TEXT NOT NULL,
			source_rejection TEXT DEFAULT '',
			created_at TIMESTAMP NOT NULL,
			FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rules_agent ON rules(agent_name)`,
		`CREATE INDEX IF NOT EXISTS idx_rules_feature ON rules(feature_id)`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("executing statement: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}