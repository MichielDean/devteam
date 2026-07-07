package chat

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ─── Streaming channel (U-CH-9, C4, NFR-REL-6) ───────────────────────────
//
// StreamingChannel holds per-session stream state for chat send-message.
// It is DISTINCT from the feature-scoped SSE (C4) — keyed by session ID,
// not feature ID, with its own buffer + cleanup.
//
// The interface is transport-agnostic (D6, ADR-011): WriteChunk feeds a
// chunk to the open stream; the HTTP handler decides whether to flush as
// SSE or WebSocket frames. The channel's job is goroutine-safe buffering +
// lifecycle (open/cleanup/close) — not the wire format.
//
// Lifecycle (NFR-REL-6): client disconnect MUST NOT leak goroutines or
// server-side state. The channel's Close cancels the per-session context,
// which the dispatch goroutine observes and aborts on. A unit test asserts
// no leak.

// StreamChunk is one unit of streamed output. Type distinguishes token
// stream, tool-call proposal, citations, and the final message metadata.
type StreamChunk struct {
	Type    string `json:"type"`              // "token" | "tool-call" | "citations" | "done" | "error"
	Content string `json:"content,omitempty"`
	// For tool-call chunks:
	ProposalID    string `json:"proposal_id,omitempty"`
	Command       string `json:"command,omitempty"`
	Classification string `json:"classification,omitempty"`
	Consequence   string `json:"consequence,omitempty"`
	NeedsConfirm  bool   `json:"needs_confirm,omitempty"`
	// For done chunks:
	MessageID     string `json:"message_id,omitempty"`
	ProviderUsed  string `json:"provider_used,omitempty"`
	// For citations (also sent as "citations" type):
	Citations []Citation `json:"citations,omitempty"`
	// For error:
	Error string `json:"error,omitempty"`
}

// StreamSubscriber is the receiver end — typically an HTTP handler that
// forwards chunks to the client (SSE/WebSocket).
type StreamSubscriber struct {
	ID string
	Ch chan StreamChunk
}

// StreamingChannel manages per-session stream state.
type StreamingChannel struct {
	mu          sync.Mutex
	subscribers map[string][]*StreamSubscriber // sessionID → subscribers
	ctxs        map[string]context.Context     // sessionID → cancel-aware ctx
	cancels     map[string]context.CancelFunc
}

// NewStreamingChannel creates the channel manager.
func NewStreamingChannel() *StreamingChannel {
	return &StreamingChannel{
		subscribers: map[string][]*StreamSubscriber{},
		ctxs:        map[string]context.Context{},
		cancels:     map[string]context.CancelFunc{},
	}
}

// Open registers a new subscriber for a session and returns the subscriber +
// the per-session context (canceled on Close/disconnect). The dispatch
// goroutine should observe this ctx to abort on client disconnect (NFR-REL-6).
func (sc *StreamingChannel) Open(sessionID, subscriberID string) (*StreamSubscriber, context.Context) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	// Get or create the per-session context.
	ctx, ok := sc.ctxs[sessionID]
	if !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(context.Background())
		sc.ctxs[sessionID] = ctx
		sc.cancels[sessionID] = cancel
	}
	sub := &StreamSubscriber{ID: subscriberID, Ch: make(chan StreamChunk, 64)}
	sc.subscribers[sessionID] = append(sc.subscribers[sessionID], sub)
	return sub, ctx
}

// WriteChunk sends a chunk to all subscribers of a session (non-blocking —
// a full buffer drops the chunk rather than blocking the dispatch goroutine).
func (sc *StreamingChannel) WriteChunk(sessionID string, chunk StreamChunk) {
	sc.mu.Lock()
	subs := sc.subscribers[sessionID]
	sc.mu.Unlock()
	for _, s := range subs {
		select {
		case s.Ch <- chunk:
		default:
			// Buffer full — drop the chunk. The stream is best-effort for
			// tokens; the final "done" chunk carries the persisted message_id
			// so the client can recover via a GET if it missed tokens.
		}
	}
}

// Close removes a subscriber. When the last subscriber for a session is
// gone, the per-session context is canceled (the dispatch goroutine aborts)
// and the session's state is cleaned up (NFR-REL-6 — no leak).
func (sc *StreamingChannel) Close(sessionID, subscriberID string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	subs := sc.subscribers[sessionID]
	kept := subs[:0]
	for _, s := range subs {
		if s.ID != subscriberID {
			kept = append(kept, s)
		} else {
			close(s.Ch)
		}
	}
	sc.subscribers[sessionID] = kept
	if len(sc.subscribers[sessionID]) == 0 {
		if cancel, ok := sc.cancels[sessionID]; ok {
			cancel()
			delete(sc.cancels, sessionID)
		}
		delete(sc.ctxs, sessionID)
		delete(sc.subscribers, sessionID)
	}
}

// CancelSession cancels the per-session context (e.g. on a hard error or
// explicit abort). Subscribers remain until Close is called.
func (sc *StreamingChannel) CancelSession(sessionID string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if cancel, ok := sc.cancels[sessionID]; ok {
		cancel()
	}
}

// ActiveSessions returns the session IDs with at least one subscriber.
// Useful for diagnostics + the no-leak test.
func (sc *StreamingChannel) ActiveSessions() []string {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	ids := make([]string, 0, len(sc.subscribers))
	for id, subs := range sc.subscribers {
		if len(subs) > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// SessionContext returns the per-session context (for the dispatch goroutine
// to observe). Returns context.Background() if no stream is open.
func (sc *StreamingChannel) SessionContext(sessionID string) context.Context {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if ctx, ok := sc.ctxs[sessionID]; ok {
		return ctx
	}
	return context.Background()
}

// ─── No-leak test helper ──────────────────────────────────────────────────
//
// WaitForCleanup waits up to timeout for a session to be fully cleaned up
// (no subscribers, no context). Used by the NFR-REL-6 unit test.

func (sc *StreamingChannel) WaitForCleanup(sessionID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		sc.mu.Lock()
		_, hasSubs := sc.subscribers[sessionID]
		_, hasCtx := sc.ctxs[sessionID]
		sc.mu.Unlock()
		if !hasSubs && !hasCtx {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("session %s not cleaned up within %s", sessionID, timeout)
}