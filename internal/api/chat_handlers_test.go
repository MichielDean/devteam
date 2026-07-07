package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/spec"
)

// newChatTestServer constructs a Server with the chat service wired (stub
// dispatcher — no real tmux/opencode in tests). Uses a dedicated
// devteam_test_chat DB (per iac-designs §5) so it doesn't collide with the
// db package's devteam_test_db or the api package's devteam_test_api.
func newChatTestServer(t *testing.T) *Server {
	t.Helper()
	const chatTestDSN = "host=localhost port=5432 user=devteam password=devteam dbname=devteam_test_chat sslmode=disable"
	database, err := db.Open(db.Config{DSN: chatTestDSN}, chatTestDSN)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	db.TruncateAllTables(database)
	t.Cleanup(func() { database.Close() })

	sp := spec.NewSpecProvider("/tmp")
	pipe := pipeline.NewPipeline(nil, sp)
	qs := feature.NewDBQuestionStore(database)
	s := NewServer(":0", sp, pipe, nil, qs, database)
	s.SetChatConfig(&config.Config{}, nil) // stub dispatcher → stub expert answers
	return s
}

func TestChatAPI_CreateAndGetSession(t *testing.T) {
	s := newChatTestServer(t)
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	// Create
	body, _ := json.Marshal(map[string]any{"title": "Test Chat"})
	resp, err := http.Post(ts.URL+"/api/chat/sessions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", resp.StatusCode)
	}
	var sess chatSessionResp
	json.NewDecoder(resp.Body).Decode(&sess)
	resp.Body.Close()
	if sess.Title != "Test Chat" {
		t.Errorf("title = %q, want Test Chat", sess.Title)
	}
	if sess.ID == "" {
		t.Fatal("expected session id")
	}

	// List
	resp, _ = http.Get(ts.URL + "/api/chat/sessions")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d", resp.StatusCode)
	}
	var list []chatSessionResp
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 1 {
		t.Errorf("expected 1 session, got %d", len(list))
	}

	// Get
	resp, _ = http.Get(ts.URL + "/api/chat/sessions/" + sess.ID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d", resp.StatusCode)
	}
	var detail chatSessionDetailResp
	json.NewDecoder(resp.Body).Decode(&detail)
	resp.Body.Close()
	if detail.ID != sess.ID {
		t.Errorf("get id = %q, want %q", detail.ID, sess.ID)
	}
}

func TestChatAPI_ListProviders(t *testing.T) {
	s := newChatTestServer(t)
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/chat/providers")
	if err != nil {
		t.Fatalf("providers: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var provs []chatProviderResp
	json.NewDecoder(resp.Body).Decode(&provs)
	resp.Body.Close()
	if len(provs) < 1 {
		t.Errorf("expected ≥1 provider (default-safe), got %d", len(provs))
	}
	if provs[0].Name != "ollama" {
		t.Errorf("default provider = %q, want ollama", provs[0].Name)
	}
}

func TestChatAPI_SendMessageStream(t *testing.T) {
	s := newChatTestServer(t)
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	// Create session
	body, _ := json.Marshal(map[string]any{"title": "Stream Test"})
	resp, err := http.Post(ts.URL+"/api/chat/sessions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var sess chatSessionResp
	json.NewDecoder(resp.Body).Decode(&sess)
	resp.Body.Close()

	// Send message — SSE stream
	msgBody, _ := json.Marshal(map[string]any{"content": "what are the 5 phases?"})
	resp, err = http.Post(ts.URL+"/api/chat/sessions/"+sess.ID+"/messages", "application/json", bytes.NewReader(msgBody))
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send status = %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", resp.Header.Get("Content-Type"))
	}

	// Read the SSE stream — collect until EOF / "done". A single Read may
	// return only the first chunk (citations, etc.) before the stream has
	// flushed the rest; io.ReadAll drains until the server closes the stream
	// at the end of the response.
	streamBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading stream: %v", err)
	}
	stream := string(streamBytes)
	if !strings.Contains(stream, "data: ") {
		t.Errorf("expected SSE 'data: ' framing, got: %s", stream[:min(200, len(stream))])
	}
	if !strings.Contains(stream, `"type":"done"`) {
		t.Errorf("expected a 'done' chunk in the stream, got: %s", stream[:min(200, len(stream))])
	}
}

func TestChatAPI_DeleteSession(t *testing.T) {
	s := newChatTestServer(t)
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	body, _ := json.Marshal(map[string]any{"title": "to-delete"})
	resp, _ := http.Post(ts.URL+"/api/chat/sessions", "application/json", bytes.NewReader(body))
	var sess chatSessionResp
	json.NewDecoder(resp.Body).Decode(&sess)
	resp.Body.Close()

	req, _ := http.NewRequest("DELETE", ts.URL+"/api/chat/sessions/"+sess.ID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestChatAPI_ErrorShape(t *testing.T) {
	s := newChatTestServer(t)
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	// GET a non-existent session → 404 with {"error","details"}
	resp, _ := http.Get(ts.URL + "/api/chat/sessions/nonexistent-uuid")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	var errResp ErrorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	resp.Body.Close()
	if errResp.Error == "" {
		t.Error("expected non-empty error code")
	}
	if errResp.Details == "" {
		t.Error("expected non-empty details")
	}
}

func TestChatAPI_ListSessionsEmptyArrayNotNull(t *testing.T) {
	// developer-agent failure-mode rule: empty collections serialize as [],
	// not null. Truncate gives 0 sessions.
	s := newChatTestServer(t)
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/api/chat/sessions")
	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	resp.Body.Close()
	got := strings.TrimSpace(string(body[:n]))
	if got == "null" {
		t.Errorf("expected [] not null for empty sessions list")
	}
	if got != "[]" {
		t.Errorf("expected [] for empty sessions, got %q", got)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}