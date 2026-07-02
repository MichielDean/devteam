package db

import (
	"fmt"
	"strings"
	"time"
)

// NoteRow is the database representation of an inter-phase note.
type NoteRow struct {
	ID        int64     `json:"id"`
	FeatureID string    `json:"feature_id"`
	Phase     string    `json:"phase"`
	Role      string    `json:"role"`
	NoteType  string    `json:"note_type"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// AddNote inserts a note for a feature.
func (db *DB) AddNote(featureID, phase, role, noteType, content string) (int64, error) {
	var id int64
	err := db.QueryRow(
		`INSERT INTO notes (feature_id, phase, role, note_type, content, created_at) VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		featureID, phase, role, noteType, content, time.Now().UTC(),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("adding note: %w", err)
	}
	return id, nil
}

// GetNotes retrieves all notes for a feature, ordered by time.
func (db *DB) GetNotes(featureID string) ([]NoteRow, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, phase, role, note_type, content, created_at
		 FROM notes WHERE feature_id = ? ORDER BY created_at ASC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting notes: %w", err)
	}
	defer rows.Close()

	var notes []NoteRow
	for rows.Next() {
		var n NoteRow
		if err := rows.Scan(&n.ID, &n.FeatureID, &n.Phase, &n.Role, &n.NoteType, &n.Content, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning note: %w", err)
		}
		notes = append(notes, n)
	}
	return notes, nil
}

// GetNotesForPhase retrieves notes from a specific phase.
func (db *DB) GetNotesForPhase(featureID, phase string) ([]NoteRow, error) {
	rows, err := db.Query(
		`SELECT id, feature_id, phase, role, note_type, content, created_at
		 FROM notes WHERE feature_id = ? AND phase = ? ORDER BY created_at ASC`,
		featureID, phase,
	)
	if err != nil {
		return nil, fmt.Errorf("getting notes for phase: %w", err)
	}
	defer rows.Close()

	var notes []NoteRow
	for rows.Next() {
		var n NoteRow
		if err := rows.Scan(&n.ID, &n.FeatureID, &n.Phase, &n.Role, &n.NoteType, &n.Content, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning note: %w", err)
		}
		notes = append(notes, n)
	}
	return notes, nil
}

// BuildNotesContext returns a markdown section containing notes from prior phases.
// This gets appended to CONTEXT.md so the current agent can see what previous
// phases found, decided, and flagged. (Cistern pattern)
func (db *DB) BuildNotesContext(featureID, currentPhase string) string {
	notes, err := db.GetNotes(featureID)
	if err != nil || len(notes) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n---\n\n# Phase Notes (from prior phases)\n\n")
	b.WriteString("Previous phases recorded the following notes. Use these to understand what was decided, what was found, and what to watch for:\n\n")

	for _, n := range notes {
		if n.Phase == currentPhase {
			continue // Don't show notes from the current phase
		}
		b.WriteString(fmt.Sprintf("## [%s] %s — %s (%s)\n\n%s\n\n", n.CreatedAt.Format(time.RFC3339), n.Phase, n.Role, n.NoteType, n.Content))
	}

	return b.String()
}