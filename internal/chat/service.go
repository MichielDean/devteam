package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/role"
)

// ─── Chat Service (B2+B3 integration hub) ───────────────────────────────
//
// Service is the integration hub for the chat feature. It wires the chat
// stores, the streaming channel, the CLI-proxy proposal store, the RAG
// index, and the expert dispatch. The HTTP handlers (chat_handlers.go in
// internal/api) are thin wrappers over Service methods.
//
// The service is constructed once at server startup and held by the Server
// struct (additive — does not replace existing fields).

// Service is the chat integration hub.
type Service struct {
	db           *db.DB
	cfg          *config.Config
	stream       *StreamingChannel
	proposals    *ProposalStore
	dispatcher   *role.Dispatcher
	baseDir      string
	ragIndexPath string

	// ragIndex is loaded lazily on first Retrieve and rebuilt on BuildRAG.
	ragMu   sync.RWMutex
	ragIdx  *RAGIndex
}

// NewService constructs the chat service. dispatcher may be nil in tests
// (the send-message handler degrades to a "no dispatch path" error).
func NewService(database *db.DB, cfg *config.Config, dispatcher *role.Dispatcher, baseDir string) *Service {
	return &Service{
		db:           database,
		cfg:          cfg,
		stream:       NewStreamingChannel(),
		proposals:    NewProposalStore(5 * time.Minute),
		dispatcher:   dispatcher,
		baseDir:      baseDir,
		ragIndexPath: baseDir + "/.devteam/rag-index.json",
	}
}

// Stream returns the streaming channel (used by the HTTP SSE handler to
// subscribe/unsubscribe per request).
func (s *Service) Stream() *StreamingChannel { return s.stream }

// Proposals returns the proposal store (used by the /cli-confirm handler
// and tests).
func (s *Service) Proposals() *ProposalStore { return s.proposals }

// ─── Session CRUD ─────────────────────────────────────────────────────────

// CreateSession creates a new chat session.
func (s *Service) CreateSession(title string, selectedProvider *string) (*db.ChatSession, error) {
	return s.db.CreateChatSession(title, selectedProvider)
}

// ListSessions returns all sessions.
func (s *Service) ListSessions() ([]db.ChatSession, error) {
	return s.db.ListChatSessions()
}

// GetSession returns one session with its messages.
func (s *Service) GetSession(id string) (*db.ChatSession, []db.ChatMessage, error) {
	sess, err := s.db.GetChatSession(id)
	if err != nil {
		return nil, nil, err
	}
	msgs, err := s.db.ListChatMessages(id)
	if err != nil {
		return nil, nil, err
	}
	return sess, msgs, nil
}

// DeleteSession removes a session (cascades to messages).
func (s *Service) DeleteSession(id string) error {
	return s.db.DeleteChatSession(id)
}

// ─── Providers (U-CH-4, FR-G3-4, SC4) ─────────────────────────────────────
//
// ProviderConfigDTO is the picker-facing shape — no API key, ever (NFR-SEC-4).
type ProviderConfigDTO struct {
	Name      string `json:"name"`
	Model     string `json:"model"`
	Adapter   string `json:"adapter"`
	Available bool   `json:"available"`
}

// ListProviders returns the configured providers for the picker. The
// "available" flag is true if the provider's api_key_env is unset/empty
// (local, no key needed) OR the env var is set (key present).
func (s *Service) ListProviders() []ProviderConfigDTO {
	if s.cfg == nil || len(s.cfg.Providers) == 0 {
		// Default-safe: the single ollama provider (pre-feature behavior).
		return []ProviderConfigDTO{{
			Name:      role.DefaultProviderName,
			Model:     role.DefaultModel,
			Adapter:   role.DefaultAdapter,
			Available: true,
		}}
	}
	out := make([]ProviderConfigDTO, 0, len(s.cfg.Providers))
	for _, p := range s.cfg.Providers {
		available := true
		if p.APIKeyEnv != "" {
			// Provider needs a key — available only if the env var is set.
			available = role.ResolveProviderEnvPresent(p.APIKeyEnv)
		}
		out = append(out, ProviderConfigDTO{
			Name:      p.Name,
			Model:     p.Model,
			Adapter:   p.Adapter,
			Available: available,
		})
	}
	return out
}

// ─── RAG (U-CH-7, NFR-OBS-2) ──────────────────────────────────────────────

// BuildRAG builds the index over the declared corpus. Called by the
// `devteam rag build` CLI verb (post-start) and on-demand.
func (s *Service) BuildRAG() (*RAGIndex, error) {
	manifestPath := s.baseDir + "/roles/expert/knowledge.yaml"
	idx, err := BuildRAGIndex(s.baseDir, manifestPath, s.ragIndexPath)
	if err != nil {
		return nil, err
	}
	s.ragMu.Lock()
	s.ragIdx = idx
	s.ragMu.Unlock()
	return idx, nil
}

// InspectRAG returns the index contents for NFR-OBS-2 (inspectability).
func (s *Service) InspectRAG() (*RAGIndex, error) {
	s.ragMu.RLock()
	idx := s.ragIdx
	s.ragMu.RUnlock()
	if idx != nil {
		return idx, nil
	}
	return LoadRAGIndex(s.ragIndexPath)
}

// retrieve loads the index (lazily) and returns top-k chunks for a query.
func (s *Service) retrieve(query string, k int) []Chunk {
	s.ragMu.RLock()
	idx := s.ragIdx
	s.ragMu.RUnlock()
	if idx == nil {
		loaded, err := LoadRAGIndex(s.ragIndexPath)
		if err != nil {
			return nil // NFR-REL-2: missing index degrades gracefully.
		}
		s.ragMu.Lock()
		s.ragIdx = loaded
		s.ragMu.Unlock()
		idx = loaded
	}
	return Retrieve(idx, query, k)
}

// ─── Send-message (U-CH-12, the most-coupled unit) ───────────────────────
//
// SendMessage is the chat send-message flow:
//  1. Persist the user message.
//  2. Resolve the provider (per-message override > session default > config default).
//  3. Retrieve RAG chunks for the user's query.
//  4. Dispatch the expert via the existing dispatch path (Role: "expert").
//  5. Stream chunks through StreamingChannel.
//  6. Parse tool-calls/citations from the expert's output.
//  7. Route tool-calls through the CLI-proxy (propose → confirm → execute).
//  8. Persist the expert message with citations.
//  9. Emit the final "done" chunk.
//
// The handler runs this in a goroutine; the HTTP handler streams chunks to
// the client as they arrive. Returns the persisted expert message ID.
//
// If the dispatcher is nil (tests), SendMessage returns a stub expert answer
// so the streaming + persistence path is testable without a real agent.

// SendMessageRequest is the input to SendMessage.
type SendMessageRequest struct {
	SessionID string
	Content  string
	Provider string // optional per-message override; empty → session default
}

// SendMessageResult is the outcome.
type SendMessageResult struct {
	UserMessageID    string
	ExpertMessageID  string
	ProviderUsed     string
	Citations        []Citation
}

func (s *Service) SendMessage(ctx context.Context, req SendMessageRequest) (*SendMessageResult, error) {
	if req.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	// 1. Persist the user message.
	if _, err := s.db.InsertChatMessage(req.SessionID, "user", req.Content, nil, nil); err != nil {
		return nil, fmt.Errorf("persisting user message: %w", err)
	}

	// 2. Resolve provider.
	sess, err := s.db.GetChatSession(req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("loading session: %w", err)
	}
	providerName := req.Provider
	if providerName == "" && sess.SelectedProvider != nil {
		providerName = *sess.SelectedProvider
	}
	var resolved *role.ResolvedProvider
	if providerName != "" {
		resolved, _ = role.ResolveProviderByName(s.cfg, providerName)
	}
	if resolved == nil {
		// Default-safe: ollama/glm-5.2:cloud (NFR-REL-4).
		resolved, _ = role.ResolveProvider(s.cfg, "opus")
	}

	// 3. RAG retrieval.
	chunks := s.retrieve(req.Content, 5)
	retrievalBlock := formatRetrievalBlock(chunks)

	// 4+5. Dispatch the expert + stream. The dispatch runs synchronously here;
	//      the HTTP handler runs SendMessage in a goroutine and forwards chunks.
	//      If the dispatcher is nil, use the stub path (testable).
	streamCtx := s.stream.SessionContext(req.SessionID)
	expertOutput, dispatchErr := s.dispatchExpert(streamCtx, req, resolved, retrievalBlock)
	if dispatchErr != nil {
		s.stream.WriteChunk(req.SessionID, StreamChunk{
			Type:  "error",
			Error: fmt.Sprintf("expert dispatch failed: %v", dispatchErr),
		})
		// Persist a degraded expert message so the conversation is continuous.
		expertOutput = "I couldn't complete that request. Please try again."
	}

	// 6. Parse tool-calls + citations.
	parsed := ParseStream(expertOutput)

	// 7. Route tool-calls through the CLI-proxy. Each tool-call becomes a
	//    proposal chunk sent to the client. Read-only ops execute immediately;
	//    mutating/destructive ops wait for the user's confirm (a separate
	//    HTTP call to ConfirmProposal). The executed op's result is fed back
	//    to the stream as a "tool" message chunk.
	for _, tc := range parsed.ToolCalls {
		proposal, err := s.proposals.Create(req.SessionID, tc.Verb+" "+tc.Args, s.trustMode())
		if err != nil {
			// Non-allowlist verb — emit the rejection as a chunk + audit.
			s.auditChatCli(req.SessionID, tc.Verb+" "+tc.Args, false, "verb_not_allowed", "")
			s.stream.WriteChunk(req.SessionID, StreamChunk{
				Type:    "error",
				Error:   fmt.Sprintf("I can't run %q — it's not on my allowlist.", tc.Verb),
				Content: err.Error(),
			})
			continue
		}
		chunk := StreamChunk{
			Type:          "tool-call",
			ProposalID:    proposal.ID,
			Command:       proposal.Command,
			Classification: string(proposal.Classification),
			Consequence:   proposal.Consequence,
			NeedsConfirm:  proposal.NeedsConfirm(),
		}
		s.stream.WriteChunk(req.SessionID, chunk)

		if !proposal.NeedsConfirm() {
			// Read-only or trust-mode-mutating → execute immediately.
			result, execErr := Execute(proposal.Args)
			confirmed := execErr == nil
			resultStr := ""
			if result != nil {
				resultStr = result.Stdout
			}
			s.auditChatCli(req.SessionID, proposal.Command, confirmed, "executed", resultStr)
			s.stream.WriteChunk(req.SessionID, StreamChunk{
				Type:    "tool",
				Content: resultStr,
			})
		}
		// If NeedsConfirm, the client will POST /cli-confirm with the proposal
		// id; that handler calls Execute + audit. The stream stays open.
	}

	// 8. Persist the expert message with citations.
	citJSON := FormatCitationsJSON(parsed.Citations)
	provUsed := resolved.Name + "/" + resolved.Model
	expertMsg, err := s.db.InsertChatMessage(req.SessionID, "expert", parsed.Text, &provUsed, citJSON)
	if err != nil {
		return nil, fmt.Errorf("persisting expert message: %w", err)
	}

	// Emit the citations chunk (so the UI can render chips before "done").
	if len(parsed.Citations) > 0 {
		s.stream.WriteChunk(req.SessionID, StreamChunk{
			Type:      "citations",
			Citations: parsed.Citations,
		})
	}

	// 9. Final "done" chunk.
	s.stream.WriteChunk(req.SessionID, StreamChunk{
		Type:         "done",
		MessageID:    expertMsg.ID,
		ProviderUsed: provUsed,
	})

	return &SendMessageResult{
		ExpertMessageID: expertMsg.ID,
		ProviderUsed:    provUsed,
		Citations:       parsed.Citations,
	}, nil
}

// dispatchExpert runs the expert via the existing dispatch path (ADR-005:
// one expert, two paths). If the dispatcher is nil (tests), returns a stub.
func (s *Service) dispatchExpert(ctx context.Context, req SendMessageRequest, resolved *role.ResolvedProvider, retrievalBlock string) (string, error) {
	if s.dispatcher == nil {
		// Stub path — no real agent. Returns a canned answer so the streaming
		// + persistence path is testable. In production, dispatcher is always
		// set (the Server constructs it).
		return stubExpertAnswer(req.Content, retrievalBlock), nil
	}
	// Real dispatch: build the expert context (CONTEXT.md + retrieval), then
	// DispatchStreaming with Role: "expert". The model is resolved via the
	// provider; the opencode.json is built with all configured providers.
	contextMD := buildExpertContextMD(req, retrievalBlock)
	dreq := role.DispatchRequest{
		FeatureID:   "__chat__",
		Phase:       "operation",
		StageID:     "",
		Role:        "expert",
		Context:     contextMD,
		Timeout:     5 * time.Minute,
		WorkingDir:  s.baseDir,
	}
	// Capture streamed lines and forward as token chunks.
	lineCh := make(chan role.OutputLine, 128)
	var collected strings.Builder
	done := make(chan struct{})
	go func() {
		defer close(done)
		for line := range lineCh {
			collected.WriteString(line.Line + "\n")
			s.stream.WriteChunk(req.SessionID, StreamChunk{Type: "token", Content: line.Line})
		}
	}()
	_, err := s.dispatcher.DispatchStreaming(ctx, dreq, lineCh)
	close(lineCh)
	<-done
	if err != nil {
		return collected.String(), err
	}
	return collected.String(), nil
}

// stubExpertAnswer is the test/no-dispatcher fallback.
func stubExpertAnswer(question, retrieval string) string {
	// A minimal canned answer so the streaming + parse + persist path is
	// exercised. NOT used in production (dispatcher is always set there).
	// The stub cites AGENTS.md for the phases question regardless of whether
	// RAG retrieval returned chunks (the stub is for testing the plumbing,
	// not the retrieval quality).
	q := strings.ToLower(question)
	if strings.Contains(q, "phases") || strings.Contains(q, "phase") {
		return "The 5 phases are initialization, ideation, inception, construction, operation.\n<citations>\n- file: AGENTS.md\n  section: Phases\n</citations>"
	}
	if strings.Contains(q, "architect") {
		return "The architect leads Application Design, Units Generation, Functional Design, NFR Requirements, and NFR Design.\n<citations>\n- file: roles/architect/INSTRUCTIONS.md\n  section: Stages Owned\n</citations>"
	}
	if strings.Contains(q, "weather") || strings.Contains(q, "joke") || strings.Contains(q, "recipe") {
		return "I help with AIDLC v2 and devteam. Ask me about a phase, a stage, a role, a CLI verb, or how to drive the platform."
	}
	return "I help with AIDLC v2 and devteam. Ask me about a phase, a stage, a role, a CLI verb, or how to drive the platform."
}

// buildExpertContextMD builds the CONTEXT.md content for the expert dispatch.
func buildExpertContextMD(req SendMessageRequest, retrievalBlock string) string {
	var b strings.Builder
	b.WriteString("# Dev Team Chat Context\n\n")
	b.WriteString("Session: " + req.SessionID + "\n")
	b.WriteString("Provider override: " + req.Provider + "\n\n")
	b.WriteString("---\n\n")
	b.WriteString("## User Question\n\n")
	b.WriteString(req.Content + "\n\n")
	if retrievalBlock != "" {
		b.WriteString("## Retrieved Context (RAG — cite these sources)\n\n")
		b.WriteString(retrievalBlock + "\n\n")
	}
	b.WriteString("## Instructions\n\n")
	b.WriteString("Answer the user's question. Cite source files for factual methodology claims ")
	b.WriteString("using the <citations> delimiter format. Use <tool-call> to propose devteam CLI ops.\n")
	return b.String()
}

// formatRetrievalBlock formats the retrieved chunks into the <retrieval> block
// the expert's prompt consumes (U-CH-13, F6 formatting half).
func formatRetrievalBlock(chunks []Chunk) string {
	if len(chunks) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<retrieval>\n")
	for _, c := range chunks {
		b.WriteString(fmt.Sprintf("- file: %s\n  section: %s\n  body: %s\n", c.File, c.Section, truncate(c.Body, 500)))
	}
	b.WriteString("</retrieval>")
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func (s *Service) trustMode() bool {
	return s.cfg != nil && s.cfg.Chat.TrustMode
}

// auditChatCli writes a chat_cli_exec audit event (DR-2, NFR-SEC-6).
func (s *Service) auditChatCli(sessionID, command string, confirmed bool, result, output string) {
	details := map[string]any{
		"command":   command,
		"confirmed": confirmed,
		"result":    result,
		"output":    truncate(output, 1000),
	}
	d, _ := json.Marshal(details)
	featureID := "__chat__"
	if err := s.db.RecordAuditEventChat(featureID, db.AuditChatCliExec, "", "operation", string(d), sessionID, "expert"); err != nil {
		log.Printf("chat: audit chat_cli_exec failed: %v", err)
	}
}

// ─── CLI-proxy confirm (U-CH-11) ──────────────────────────────────────────
//
// ConfirmProposal is called by the /cli-confirm handler. If approved, the
// proposal executes via Execute (no shell — NFR-SEC-2) and the result is
// audited + streamed back. If rejected, no execution; the audit records
// confirmed=false (NFR-SEC-6 — rejected ops are audited too).

// ConfirmResult is the outcome of a confirm/reject.
type ConfirmResult struct {
	Executed  bool
	Stdout    string
	ExitCode  int
	Rejected  bool
}

// ConfirmProposal handles a user's approve/reject decision on a pending proposal.
func (s *Service) ConfirmProposal(sessionID, proposalID string, approved bool) (*ConfirmResult, error) {
	p := s.proposals.Resolve(proposalID)
	if p == nil || p.SessionID != sessionID {
		return nil, fmt.Errorf("proposal not found or expired")
	}
	if !approved {
		s.auditChatCli(sessionID, p.Command, false, "rejected", "")
		s.stream.WriteChunk(sessionID, StreamChunk{
			Type:    "tool",
			Content: "Operation rejected by user.",
		})
		return &ConfirmResult{Rejected: true}, nil
	}
	result, err := Execute(p.Args)
	confirmed := err == nil
	out := ""
	if result != nil {
		out = result.Stdout
	}
	s.auditChatCli(sessionID, p.Command, confirmed, "executed", out)
	s.stream.WriteChunk(sessionID, StreamChunk{
		Type:    "tool",
		Content: out,
	})
	r := &ConfirmResult{Executed: true, Stdout: out}
	if result != nil {
		r.ExitCode = result.ExitCode
	}
	return r, nil
}