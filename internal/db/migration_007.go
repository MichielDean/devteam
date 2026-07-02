package db

import (
	"database/sql"
	"fmt"
)

func init() {
	RegisterMigration(Migration{
		Version: 7,
		Name:    "stage_id_on_questions_artifacts",
		Up:      migration007StageID,
	})
}

// migration007StageID adds stage_id columns to questions and spec_artifacts
// so they can be associated with the specific stage that produced them.
func migration007StageID(tx *sql.Tx) error {
	tables := []struct {
		table string
		col   string
	}{
		{"questions", "stage_id"},
		{"spec_artifacts", "stage_id"},
	}

	for _, t := range tables {
		var colExists int
		row := tx.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, t.table, t.col)
		if err := row.Scan(&colExists); err != nil {
			return fmt.Errorf("checking %s.%s: %w", t.table, t.col, err)
		}
		if colExists == 0 {
			_, err := tx.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT DEFAULT ''", t.table, t.col))
			if err != nil {
				return fmt.Errorf("adding %s.%s: %w", t.table, t.col, err)
			}
		}
	}
	return nil
}