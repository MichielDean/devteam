package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/MichielDean/devteam/internal/chat"
	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/role"
)

// ─── Chat HTTP API (B2+B3, FR-G2-2) ─────────────────────────────────────
//
// registerChatRoutes wires the chat route group on the existing mux. Additive
// (C1) — inherits recovery + CORS middleware. Routes:
//
//   POST   /api/chat/sessions                       — create session
//   GET    /api/chat/sessions                       — list sessions
//   GET    /api/chat/sessions/{id}                  — get session + messages
//   DELETE /api/chat/sessions/{id}                  — delete session
//   POST   /api/chat/sessions/{id}/messages         — send message (stream)
//   POST   /api/chat/sessions/{id}/cli-confirm      — confirm/reject proposal
//   GET    /api/chat/providers                      — list providers (picker)
//   GET    /api/chat/rag/corpus                     — inspect RAG index
//
// Error shape: {"error": "<code>", "details": "<msg>"} (NFR-MAINT-3, re AP-5).
// Never bare strings.

func (s *Server) registerChatRoutes(mux *http.ServeMux) {
	// Routes are always registered (additive — C1). The handlers check for
	// chatService == nil at request time and return 503 if chat is not
	// configured. This keeps route registration in NewServer stable while
	// allowing SetChatConfig to wire the service after NewServer.
	mux.HandleFunc("POST /api/chat/sessions", s.chatCreateSession)
	mux.HandleFunc("GET /api/chat/sessions", s.chatListSessions)
	mux.HandleFunc("GET /api/chat/sessions/{id}", s.chatGetSession)
	mux.HandleFunc("DELETE /api/chat/sessions/{id}", s.chatDeleteSession)
	mux.HandleFunc("POST /api/chat/sessions/{id}/messages", s.chatSendMessage)
	mux.HandleFunc("POST /api/chat/sessions/{id}/cli-confirm", s.chatConfirmProposal)
	mux.HandleFunc("GET /api/chat/providers", s.chatListProviders)
	mux.HandleFunc("GET /api/chat/rag/corpus", s.chatInspectRAG)
}

// chatUnavailable is the 503 response when chatService is nil (e.g. in tests
// that don't call SetChatConfig). Returns true if it wrote the response.
func (s *Server) chatUnavailable(w http.ResponseWriter) bool {
	if s.chatService == nil {
		writeError(w, http.StatusServiceUnavailable, "chat_unavailable", "chat service is not configured")
		return true
	}
	return false
}

// ─── DTOs ─────────────────────────────────────────────────────────────────

type chatCreateSessionReq struct {
	Title            string  `json:"title,omitempty"`
	SelectedProvider *string `json:"selected_provider,omitempty"`
}

type chatSessionResp struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	SelectedProvider *string `json:"selected_provider,omitempty"`
	CreatedAt        string  `json:"created_at"`
}

type chatSessionDetailResp struct {
	chatSessionResp
	Messages []chatMessageResp `json:"messages"`
}

type chatMessageResp struct {
	ID            string          `json:"id"`
	Role          string          `json:"role"`
	Content       string          `json:"content"`
	ProviderUsed  *string         `json:"provider_used,omitempty"`
	CreatedAt     string          `json:"created_at"`
	Citations     json.RawMessage `json:"citations,omitempty"`
}

type chatSendMessageReq struct {
	Content  string `json:"content"`
	Provider string `json:"provider,omitempty"`
}

type chatConfirmReq struct {
	ProposalID string `json:"proposal_id"`
	Approved   bool   `json:"approved"`
}

type chatConfirmResp struct {
	Executed bool   `json:"executed"`
	Rejected bool   `json:"rejected,omitempty"`
	Stdout   string `json:"stdout,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
}

type chatProviderResp struct {
	Name      string `json:"name"`
	Model     string `json:"model"`
	Adapter   string `json:"adapter"`
	Available bool   `json:"available"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────

func (s *Server) chatCreateSession(w http.ResponseWriter, r *http.Request) {
	if s.chatUnavailable(w) {
		return
	}
	var req chatCreateSessionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}
	sess, err := s.chatService.CreateSession(req.Title, req.SelectedProvider)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toChatSessionResp(sess))
}

func (s *Server) chatListSessions(w http.ResponseWriter, r *http.Request) {
	if s.chatUnavailable(w) {
		return
	}
	sessions, err := s.chatService.ListSessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]chatSessionResp, 0, len(sessions))
	for i := range sessions {
		out = append(out, toChatSessionResp(&sessions[i]))
	}
	// Serialize as empty array, not null (developer-agent failure-mode rule).
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) chatGetSession(w http.ResponseWriter, r *http.Request) {
	if s.chatUnavailable(w) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Session ID is required")
		return
	}
	sess, msgs, err := s.chatService.GetSession(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session_not_found", fmt.Sprintf("Session %s not found", id))
		return
	}
	resp := chatSessionDetailResp{
		chatSessionResp: toChatSessionResp(sess),
		Messages:        make([]chatMessageResp, 0, len(msgs)),
	}
	for _, m := range msgs {
		resp.Messages = append(resp.Messages, toChatMessageResp(m))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) chatDeleteSession(w http.ResponseWriter, r *http.Request) {
	if s.chatUnavailable(w) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Session ID is required")
		return
	}
	if err := s.chatService.DeleteSession(id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// chatSendMessage streams the expert's response as SSE (D6 = SSE for v1;
// ADR-011 — the channel interface is transport-agnostic, this handler
// implements the SSE wire format). Each chunk is an `application/json`-encoded
// StreamChunk prefixed with "data: " and terminated with \n\n (SSE framing).
func (s *Server) chatSendMessage(w http.ResponseWriter, r *http.Request) {
	if s.chatUnavailable(w) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Session ID is required")
		return
	}
	var req chatSendMessageReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "content is required")
		return
	}

	// SSE headers. The connection stays open until the stream completes or
	// the client disconnects (NFR-REL-6).
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal_error", "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Subscribe to the stream BEFORE dispatching so we don't miss early chunks.
	subscriberID := fmt.Sprintf("http-%d", time.Now().UnixNano())
	sub, streamCtx := s.chatService.Stream().Open(id, subscriberID)
	defer s.chatService.Stream().Close(id, subscriberID)

	// Dispatch in a goroutine; forward chunks to the client as they arrive.
	dispatchDone := make(chan error, 1)
	go func() {
		_, err := s.chatService.SendMessage(streamCtx, chat.SendMessageRequest{
			SessionID: id,
			Content:   req.Content,
			Provider:  req.Provider,
		})
		dispatchDone <- err
	}()

	// Forward chunks until the dispatch completes or the client disconnects.
	clientGone := r.Context().Done()
	for {
		select {
		case chunk := <-sub.Ch:
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			if chunk.Type == "done" || chunk.Type == "error" {
				// Drain the dispatch goroutine.
				select {
				case <-dispatchDone:
				case <-time.After(5 * time.Second):
				}
				return
			}
		case err := <-dispatchDone:
			if err != nil {
				log.Printf("chat send-message dispatch error: %v", err)
			}
			// Drain any remaining chunks briefly before closing.
			for {
				select {
				case chunk := <-sub.Ch:
					data, _ := json.Marshal(chunk)
					fmt.Fprintf(w, "data: %s\n\n", data)
					flusher.Flush()
				default:
					return
				}
			}
		case <-clientGone:
			// Client disconnected — cancel the stream (NFR-REL-6 cleanup).
			s.chatService.Stream().CancelSession(id)
			return
		}
	}
}

func (s *Server) chatConfirmProposal(w http.ResponseWriter, r *http.Request) {
	if s.chatUnavailable(w) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Session ID is required")
		return
	}
	var req chatConfirmReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}
	if req.ProposalID == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "proposal_id is required")
		return
	}
	res, err := s.chatService.ConfirmProposal(id, req.ProposalID, req.Approved)
	if err != nil {
		writeError(w, http.StatusNotFound, "proposal_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, chatConfirmResp{
		Executed: res.Executed,
		Rejected: res.Rejected,
		Stdout:   res.Stdout,
		ExitCode: res.ExitCode,
	})
}

func (s *Server) chatListProviders(w http.ResponseWriter, r *http.Request) {
	if s.chatUnavailable(w) {
		return
	}
	provs := s.chatService.ListProviders()
	out := make([]chatProviderResp, 0, len(provs))
	for _, p := range provs {
		out = append(out, chatProviderResp{
			Name:      p.Name,
			Model:     p.Model,
			Adapter:   p.Adapter,
			Available: p.Available,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) chatInspectRAG(w http.ResponseWriter, r *http.Request) {
	if s.chatUnavailable(w) {
		return
	}
	idx, err := s.chatService.InspectRAG()
	if err != nil {
		// NFR-REL-2: missing index → empty response, not a 500.
		writeJSON(w, http.StatusOK, map[string]any{
			"built_at":     "",
			"corpus_mtime": "",
			"chunks":       []any{},
			"error":        "rag index not built — run `devteam rag build`",
		})
		return
	}
	writeJSON(w, http.StatusOK, idx)
}

// ─── helpers ──────────────────────────────────────────────────────────────

func toChatSessionResp(sess *db.ChatSession) chatSessionResp {
	return chatSessionResp{
		ID:               sess.ID,
		Title:            sess.Title,
		SelectedProvider: sess.SelectedProvider,
		CreatedAt:        sess.CreatedAt.Format(time.RFC3339),
	}
}

func toChatMessageResp(m db.ChatMessage) chatMessageResp {
	resp := chatMessageResp{
		ID:           m.ID,
		Role:         m.Role,
		Content:      m.Content,
		ProviderUsed:  m.ProviderUsed,
		CreatedAt:    m.CreatedAt.Format(time.RFC3339),
	}
	if len(m.Citations) > 0 {
		resp.Citations = json.RawMessage(m.Citations)
	}
	return resp
}

// ─── Chat service construction ────────────────────────────────────────────
//
// SetChatConfig constructs the chat service at server startup. Called by
// main.go after NewServer (the existing NewServer signature is unchanged —
// additive). If the dispatcher is nil (e.g. in unit tests), the chat service
// still works but SendMessage returns stub answers (the stub path).
//
// This is split from NewServer to keep NewServer's signature stable for the
// existing test callers. main.go calls SetChatConfig immediately after
// NewServer.

// SetChatConfig constructs the chat service with the given config + dispatcher.
func (s *Server) SetChatConfig(cfg *config.Config, dispatcher *role.Dispatcher) {
	if s.db == nil {
		return // no DB → no chat service
	}
	s.chatService = chat.NewService(s.db, cfg, dispatcher, s.baseDir)
}

// ChatService returns the chat service (for tests + diagnostics).
func (s *Server) ChatService() *chat.Service { return s.chatService }

// Stream returns the chat streaming channel (used by the SSE handler).
func (s *Server) Stream() *chat.StreamingChannel {
	if s.chatService == nil {
		return nil
	}
	return s.chatService.Stream()
}

// ensureChatContext is a no-op that exists so the `context` import is used
// in this file even when the SSE handler is the only consumer. (Go's unused
// import check is package-level, but keeping the import here documents that
// the handler uses r.Context() for disconnect detection.)
var _ = context.Background

// suppress unused import warnings for strings (used in error messages).
var _ = strings.TrimSpace