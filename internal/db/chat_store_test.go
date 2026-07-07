package db

import (
	"encoding/json"
	"testing"
)

func TestChatSession_CreateGetList(t *testing.T) {
	d := setupTestDB(t)

	prov := "ollama"
	s, err := d.CreateChatSession("My Chat", &prov)
	if err != nil {
		t.Fatalf("CreateChatSession: %v", err)
	}
	if s.Title != "My Chat" {
		t.Errorf("title = %q, want My Chat", s.Title)
	}
	if s.SelectedProvider == nil || *s.SelectedProvider != "ollama" {
		t.Errorf("selected_provider = %v, want ollama", s.SelectedProvider)
	}

	// Get
	got, err := d.GetChatSession(s.ID)
	if err != nil {
		t.Fatalf("GetChatSession: %v", err)
	}
	if got.ID != s.ID {
		t.Errorf("GetChatSession id mismatch")
	}

	// List — at least one
	list, err := d.ListChatSessions()
	if err != nil {
		t.Fatalf("ListChatSessions: %v", err)
	}
	if len(list) < 1 {
		t.Errorf("expected ≥1 session, got %d", len(list))
	}
}

func TestChatSession_UpdateTitleAndProvider(t *testing.T) {
	d := setupTestDB(t)
	s, _ := d.CreateChatSession("old", nil)

	if err := d.UpdateChatSession(s.ID, "new title", strPtr("openai")); err != nil {
		t.Fatalf("UpdateChatSession: %v", err)
	}
	got, _ := d.GetChatSession(s.ID)
	if got.Title != "new title" {
		t.Errorf("title = %q, want new title", got.Title)
	}
	if got.SelectedProvider == nil || *got.SelectedProvider != "openai" {
		t.Errorf("provider = %v, want openai", got.SelectedProvider)
	}

	// Clear provider (nil) — keep title
	if err := d.UpdateChatSession(s.ID, "", nil); err != nil {
		t.Fatalf("UpdateChatSession clear: %v", err)
	}
	got, _ = d.GetChatSession(s.ID)
	if got.Title != "new title" {
		t.Errorf("title was changed when empty passed; got %q", got.Title)
	}
	if got.SelectedProvider != nil {
		t.Errorf("provider = %v, want nil", got.SelectedProvider)
	}
}

func TestChatSession_DeleteCascadesMessages(t *testing.T) {
	d := setupTestDB(t)
	s, _ := d.CreateChatSession("to-delete", nil)
	d.InsertChatMessage(s.ID, "user", "hello", nil, nil)
	d.InsertChatMessage(s.ID, "expert", "hi back", nil, nil)

	msgs, _ := d.ListChatMessages(s.ID)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages before delete, got %d", len(msgs))
	}

	if err := d.DeleteChatSession(s.ID); err != nil {
		t.Fatalf("DeleteChatSession: %v", err)
	}

	if _, err := d.GetChatSession(s.ID); err == nil {
		t.Error("session should be gone")
	}
	// Messages cascade-deleted
	msgs, _ = d.ListChatMessages(s.ID)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after session delete (cascade), got %d", len(msgs))
	}
}

func TestChatMessage_InsertAndListChronological(t *testing.T) {
	d := setupTestDB(t)
	s, _ := d.CreateChatSession("chrono", nil)

	prov := "anthropic"
	m1, _ := d.InsertChatMessage(s.ID, "user", "first", nil, nil)
	m2, _ := d.InsertChatMessage(s.ID, "expert", "second", &prov, []byte(`[{"file":"AGENTS.md","section":"Phases"}]`))

	msgs, err := d.ListChatMessages(s.ID)
	if err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].ID != m1.ID {
		t.Errorf("first message should be m1, got %s", msgs[0].ID)
	}
	if msgs[1].ID != m2.ID {
		t.Errorf("second message should be m2, got %s", msgs[1].ID)
	}
	if msgs[1].Role != "expert" {
		t.Errorf("role = %q, want expert", msgs[1].Role)
	}
	if msgs[1].ProviderUsed == nil || *msgs[1].ProviderUsed != "anthropic" {
		t.Errorf("provider_used = %v, want anthropic", msgs[1].ProviderUsed)
	}
	if len(msgs[1].Citations) == 0 {
		t.Error("expected citations jsonb to be non-empty")
	}
	// Decode citations to verify shape
	var cits []map[string]interface{}
	if err := json.Unmarshal(msgs[1].Citations, &cits); err != nil {
		t.Fatalf("unmarshalling citations: %v", err)
	}
	if len(cits) != 1 || cits[0]["file"] != "AGENTS.md" {
		t.Errorf("citations shape wrong: %v", cits)
	}
}

func TestChatMessage_RoleCheckConstraint(t *testing.T) {
	d := setupTestDB(t)
	s, _ := d.CreateChatSession("role-test", nil)

	// Invalid role should fail
	_, err := d.InsertChatMessage(s.ID, "assistant", "bad", nil, nil)
	if err == nil {
		t.Error("expected error for invalid role 'assistant', got nil")
	}
}

func TestRecordAuditEventChat(t *testing.T) {
	d := setupTestDB(t)
	s, _ := d.CreateChatSession("audit-test", nil)

	err := d.RecordAuditEventChat(
		"__chat__", AuditChatCliExec, "", "operation",
		`{"command":"devteam feature status","confirmed":true}`, s.ID, "expert",
	)
	if err != nil {
		t.Fatalf("RecordAuditEventChat: %v", err)
	}

	// Verify the row exists with the chat-specific columns populated
	var count int
	err = d.QueryRow(
		`SELECT COUNT(*) FROM audit_events WHERE session_id = ? AND actor = 'expert'`,
		s.ID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("querying chat audit: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 chat audit event, got %d", count)
	}
}

func strPtr(s string) *string { return &s }