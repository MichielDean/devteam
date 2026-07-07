package db

import (
	"database/sql"
	"fmt"
	"time"
)

// ChatSession is one chat conversation row.
type ChatSession struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	SelectedProvider *string    `json:"selected_provider,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// ChatMessage is one message in a chat session.
// Citations is the raw jsonb ([]byte) — callers decode as needed.
type ChatMessage struct {
	ID            string    `json:"id"`
	SessionID     string    `json:"session_id"`
	Role          string    `json:"role"` // user | expert | tool
	Content       string    `json:"content"`
	ProviderUsed   *string   `json:"provider_used,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	Citations     []byte    `json:"citations,omitempty"` // jsonb: [{file, section, lines?}]
}

// CreateChatSession inserts a new chat session and returns it.
// If title is empty, the default "New Chat" applies at the DB level.
func (db *DB) CreateChatSession(title string, selectedProvider *string) (*ChatSession, error) {
	var (
		id        string
		outTitle  string
		outProv   sql.NullString
		createdAt time.Time
	)
	err := db.QueryRow(
		`INSERT INTO chat_sessions (title, selected_provider) VALUES (?, ?)
		 RETURNING id, title, selected_provider, created_at`,
		title, selectedProvider,
	).Scan(&id, &outTitle, &outProv, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("creating chat session: %w", err)
	}
	s := &ChatSession{ID: id, Title: outTitle, CreatedAt: createdAt}
	if outProv.Valid {
		v := outProv.String
		s.SelectedProvider = &v
	}
	return s, nil
}

// ListChatSessions returns all sessions, newest first.
func (db *DB) ListChatSessions() ([]ChatSession, error) {
	rows, err := db.Query(
		`SELECT id, title, selected_provider, created_at
		 FROM chat_sessions ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing chat sessions: %w", err)
	}
	defer rows.Close()
	var sessions []ChatSession
	for rows.Next() {
		var (
			s   ChatSession
			np  sql.NullString
			ct  time.Time
		)
		if err := rows.Scan(&s.ID, &s.Title, &np, &ct); err != nil {
			return nil, fmt.Errorf("scanning chat session: %w", err)
		}
		s.CreatedAt = ct
		if np.Valid {
			v := np.String
			s.SelectedProvider = &v
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// GetChatSession returns one session by id.
func (db *DB) GetChatSession(id string) (*ChatSession, error) {
	var (
		s     ChatSession
		np    sql.NullString
		ct    time.Time
	)
	err := db.QueryRow(
		`SELECT id, title, selected_provider, created_at
		 FROM chat_sessions WHERE id = ?`, id,
	).Scan(&s.ID, &s.Title, &np, &ct)
	if err != nil {
		return nil, fmt.Errorf("getting chat session %s: %w", id, err)
	}
	s.CreatedAt = ct
	if np.Valid {
		v := np.String
		s.SelectedProvider = &v
	}
	return &s, nil
}

// UpdateChatSession sets title and/or selected provider.
// Pass nil for selectedProvider to clear it; pass "" title to keep current.
func (db *DB) UpdateChatSession(id string, title string, selectedProvider *string) error {
	// Coalesce: keep current title if empty; selected_provider NULL if nil.
	_, err := db.Exec(
		`UPDATE chat_sessions
		    SET title = COALESCE(NULLIF(?, ''), title),
		        selected_provider = ?
		  WHERE id = ?`,
		title, selectedProvider, id,
	)
	if err != nil {
		return fmt.Errorf("updating chat session %s: %w", id, err)
	}
	return nil
}

// DeleteChatSession removes a session and (via cascade) its messages.
func (db *DB) DeleteChatSession(id string) error {
	_, err := db.Exec(`DELETE FROM chat_sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting chat session %s: %w", id, err)
	}
	return nil
}

// InsertChatMessage inserts one message and returns it.
// citations is the raw jsonb bytes (may be nil for no citations).
func (db *DB) InsertChatMessage(sessionID, role, content string, providerUsed *string, citations []byte) (*ChatMessage, error) {
	var (
		id         string
		outRole    string
		outContent string
		outProv    sql.NullString
		createdAt  time.Time
		outCit     []byte
	)
	err := db.QueryRow(
		`INSERT INTO chat_messages (session_id, role, content, provider_used, citations)
		 VALUES (?, ?, ?, ?, ?)
		 RETURNING id, session_id, role, content, provider_used, created_at, citations`,
		sessionID, role, content, providerUsed, citations,
	).Scan(&id, &sessionID, &outRole, &outContent, &outProv, &createdAt, &outCit)
	if err != nil {
		return nil, fmt.Errorf("inserting chat message: %w", err)
	}
	m := &ChatMessage{
		ID:        id,
		SessionID: sessionID,
		Role:      outRole,
		Content:   outContent,
		CreatedAt: createdAt,
		Citations:  outCit,
	}
	if outProv.Valid {
		v := outProv.String
		m.ProviderUsed = &v
	}
	return m, nil
}

// ListChatMessages returns messages for a session in chronological order.
func (db *DB) ListChatMessages(sessionID string) ([]ChatMessage, error) {
	rows, err := db.Query(
		`SELECT id, session_id, role, content, provider_used, created_at, citations
		 FROM chat_messages WHERE session_id = ? ORDER BY created_at ASC, id ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing chat messages for %s: %w", sessionID, err)
	}
	defer rows.Close()
	var messages []ChatMessage
	for rows.Next() {
		var (
			m        ChatMessage
			np       sql.NullString
			ct       time.Time
			cit      []byte
		)
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &np, &ct, &cit); err != nil {
			return nil, fmt.Errorf("scanning chat message: %w", err)
		}
		m.CreatedAt = ct
		if np.Valid {
			v := np.String
			m.ProviderUsed = &v
		}
		m.Citations = cit
		messages = append(messages, m)
	}
	return messages, nil
}