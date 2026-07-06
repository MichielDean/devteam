package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 13,
		Name:    "bolt_depends_on",
		Up:      migration013BoltDependsOn,
	})
}

func migration013BoltDependsOn(tx *sql.Tx) error {
	var colExists int
	row := tx.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`, "bolts", "depends_on")
	if err := row.Scan(&colExists); err != nil {
		return fmt.Errorf("checking depends_on column: %w", err)
	}
	if colExists == 0 {
		_, err := tx.Exec(`ALTER TABLE bolts ADD COLUMN depends_on TEXT DEFAULT '[]'`)
		if err != nil {
			return fmt.Errorf("adding depends_on column: %w", err)
		}
	}
	return nil
}