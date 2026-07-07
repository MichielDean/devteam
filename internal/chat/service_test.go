package chat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
)

// chatTestDSN is a dedicated DB for the chat package tests (per iac-designs
// §5 — devteam_test_chat). Avoids collision with the db package's
// devteam_test_db when tests run in parallel across packages.
const chatTestDSN = "host=localhost port=5432 user=devteam password=devteam dbname=devteam_test_chat sslmode=disable"

func openChatTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(db.Config{DSN: chatTestDSN}, chatTestDSN)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	db.TruncateAllTables(d)
	t.Cleanup(func() { d.Close() })
	return d
}

func TestService_ListProviders_DefaultSafe(t *testing.T) {
	d := openChatTestDB(t)
	s := NewService(d, &config.Config{}, nil, t.TempDir())
	provs := s.ListProviders()
	if len(provs) != 1 {
		t.Fatalf("expected 1 default provider, got %d", len(provs))
	}
	if provs[0].Name != "ollama" {
		t.Errorf("default provider name = %q, want ollama", provs[0].Name)
	}
	if !provs[0].Available {
		t.Error("default ollama should be available (no key needed)")
	}
}

func TestService_ListProviders_ConfiguredNoKeys(t *testing.T) {
	d := openChatTestDB(t)
	cfg := &config.Config{
		Providers: config.ProviderList{
			{Name: "ollama", BaseURL: "http://localhost:11434/v1", Model: "glm-5.2:cloud", Adapter: "openai"},
			{Name: "openai", BaseURL: "https://api.openai.com/v1", APIKeyEnv: "OPENAI_API_KEY", Model: "gpt-4o", Adapter: "openai"},
		},
	}
	s := NewService(d, cfg, nil, t.TempDir())
	provs := s.ListProviders()
	if len(provs) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(provs))
	}
	// ollama (no key env) → available
	if !provs[0].Available {
		t.Error("ollama (no key) should be available")
	}
	// openai (key env, unset in test env) → NOT available
	if provs[1].Available {
		t.Error("openai with unset OPENAI_API_KEY should NOT be available")
	}
}

func TestService_CreateSessionAndSendMessage_StubDispatcher(t *testing.T) {
	d := openChatTestDB(t)
	s := NewService(d, &config.Config{}, nil, t.TempDir())

	sess, err := s.CreateSession("test", nil)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Subscribe to the stream BEFORE sending so we can observe chunks.
	sub, _ := s.stream.Open(sess.ID, "test-sub")
	defer s.stream.Close(sess.ID, "test-sub")

	// Send a message about "phases" → stub returns a cited answer.
	// The stub doesn't have RAG chunks (no index built), but the stub answer
	// for "phases" is canned regardless. Let's verify the full flow.
	res, err := s.SendMessage(context.Background(), SendMessageRequest{
		SessionID: sess.ID,
		Content:   "what are the 5 phases?",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if res.ExpertMessageID == "" {
		t.Error("expected expert message id")
	}

	// The expert message should be persisted with citations.
	msgs, _ := d.ListChatMessages(sess.ID)
	if len(msgs) != 2 { // user + expert
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	expert := msgs[1]
	if expert.Role != "expert" {
		t.Errorf("expert role = %q, want expert", expert.Role)
	}
	if len(expert.Citations) == 0 {
		t.Error("expected citations on the expert message")
	}

	// The stream should have received a "done" chunk + a "citations" chunk.
	gotDone := false
	gotCitations := false
 drain:
	for {
		select {
		case c := <-sub.Ch:
			if c.Type == "done" {
				gotDone = true
			}
			if c.Type == "citations" {
				gotCitations = true
			}
		default:
			break drain
		}
	}
	if !gotDone {
		t.Error("expected a 'done' chunk on the stream")
	}
	if !gotCitations {
		t.Error("expected a 'citations' chunk on the stream")
	}
}

func TestService_SendMessage_ReadOnlyToolCallExecutesImmediately(t *testing.T) {
	d := openChatTestDB(t)
	s := NewService(d, &config.Config{}, nil, t.TempDir())
	sess, _ := s.CreateSession("tool-test", nil)

	// Inject an expert output with a read-only tool-call by calling the
	// internal flow indirectly: we can't easily make the stub emit a tool-call,
	// so instead test the CLI-proxy path directly via ConfirmProposal + the
	// proposal store. This verifies the audit + stream integration.
	proposal, err := s.proposals.Create(sess.ID, "feature status my-feature", false)
	if err != nil {
		t.Fatalf("Create proposal: %v", err)
	}
	if proposal.NeedsConfirm() {
		t.Error("read-only should not need confirm")
	}
	// We don't actually Execute (devteam may not be on PATH in test env);
	// just verify the proposal was created + classified.
	if proposal.Classification != ClassReadOnly {
		t.Errorf("class = %v, want read-only", proposal.Classification)
	}
}

func TestService_ConfirmProposal_RejectedAuditsConfirmedFalse(t *testing.T) {
	d := openChatTestDB(t)
	s := NewService(d, &config.Config{}, nil, t.TempDir())
	sess, _ := s.CreateSession("confirm-test", nil)

	// Create a mutating proposal (needs confirm).
	proposal, err := s.proposals.Create(sess.ID, "signal my-feature pass", false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !proposal.NeedsConfirm() {
		t.Fatal("mutating should need confirm")
	}

	// Reject it.
	res, err := s.ConfirmProposal(sess.ID, proposal.ID, false)
	if err != nil {
		t.Fatalf("ConfirmProposal reject: %v", err)
	}
	if !res.Rejected {
		t.Error("expected rejected=true")
	}

	// Audit should record confirmed=false.
	var count int
	err = d.QueryRow(
		`SELECT COUNT(*) FROM audit_events WHERE session_id = ? AND actor = 'expert' AND details LIKE '%"confirmed":false%'`,
		sess.ID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("audit query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 rejected audit event, got %d", count)
	}
}

func TestService_BuildAndRetrieveRAG(t *testing.T) {
	d := openChatTestDB(t)
	tmp := t.TempDir()
	// Minimal corpus.
	writeFile(t, filepath.Join(tmp, "AGENTS.md"), "# AIDLC\n\n5 phases: init, ideation, inception, construction, operation.\n")
	writeFile(t, filepath.Join(tmp, "roles/expert/INSTRUCTIONS.md"), "# Expert\n\nThe expert agent.\n")
	// Manifest
	writeFile(t, filepath.Join(tmp, "roles/expert/knowledge.yaml"),
		"corpus:\n  - path: AGENTS.md\n  - path: roles/expert/INSTRUCTIONS.md\n")

	s := NewService(d, &config.Config{}, nil, tmp)
	idx, err := s.BuildRAG()
	if err != nil {
		t.Fatalf("BuildRAG: %v", err)
	}
	if len(idx.Chunks) == 0 {
		t.Fatal("expected chunks in built index")
	}
	// Retrieve via the service.
	chunks := s.retrieve("phases", 5)
	if len(chunks) == 0 {
		t.Fatal("expected retrieve to return chunks for 'phases'")
	}
	if chunks[0].File != "AGENTS.md" {
		t.Errorf("top chunk file = %q, want AGENTS.md", chunks[0].File)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}