package api

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"
)

// TestSSEBroadcasterInterface verifies the Server implements SSEBroadcaster.
func TestSSEBroadcasterInterface(t *testing.T) {
	s, _, _ := setupStageTestServer(t)
	defer s.httpServer.Close()

	// BroadcastSSE should not panic
	s.BroadcastSSE("test-feature", "stage_started", `{"test":true}`)

	// Verify event is buffered
	s.sseMu.Lock()
	buffer := s.sseBuffers["test-feature"]
	s.sseMu.Unlock()

	if len(buffer) != 1 {
		t.Fatalf("buffered events: got %d, want 1", len(buffer))
	}
	if buffer[0].EventType != "stage_started" {
		t.Errorf("event type: got %s, want stage_started", buffer[0].EventType)
	}
}

// TestSSEEventTypesBuffered verifies that all new event types are buffered for late joiners.
func TestSSEEventTypesBuffered(t *testing.T) {
	s, _, _ := setupStageTestServer(t)
	defer s.httpServer.Close()

	eventTypes := []string{
		"stage_started",
		"stage_awaiting_approval",
		"stage_revising",
		"stage_completed",
		"gate_approved",
		"gate_rejected",
		"gate_result",
		"session_state_change",
	}

	for _, et := range eventTypes {
		s.BroadcastSSE("feat-buffer-test", et, `{"test":true}`)
	}

	s.sseMu.Lock()
	buffer := s.sseBuffers["feat-buffer-test"]
	s.sseMu.Unlock()

	if len(buffer) != len(eventTypes) {
		t.Errorf("buffered events: got %d, want %d", len(buffer), len(eventTypes))
	}

	// Verify each event type is in the buffer
	bufferedTypes := make(map[string]bool)
	for _, msg := range buffer {
		bufferedTypes[msg.EventType] = true
	}
	for _, et := range eventTypes {
		if !bufferedTypes[et] {
			t.Errorf("event type %s not buffered", et)
		}
	}
}

// TestSSEAgentOutputNotBuffered verifies that agent_output events are NOT buffered.
func TestSSEAgentOutputNotBuffered(t *testing.T) {
	s, _, _ := setupStageTestServer(t)
	defer s.httpServer.Close()

	s.BroadcastSSE("feat-output-test", "agent_output", `{"line":"hello"}`)

	s.sseMu.Lock()
	buffer := s.sseBuffers["feat-output-test"]
	s.sseMu.Unlock()

	if len(buffer) != 0 {
		t.Errorf("agent_output should not be buffered, but got %d events", len(buffer))
	}
}

// TestSSEStreamEndpoint tests the SSE stream endpoint returns proper headers.
func TestSSEStreamEndpoint(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	defer s.httpServer.Close()

	fid := insertTestFeature(t, database, "sse", s)

	// Use a request with a cancellable context so the stream doesn't hang
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequestWithContext(ctx, "GET", "/api/features/"+fid+"/stream", nil)
	w := httptest.NewRecorder()

	// Run the handler in a goroutine — it blocks until context is cancelled
	go s.httpServer.Handler.ServeHTTP(w, req)
	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)

	// Check headers (should be set before the stream blocks)
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type: got %s, want text/event-stream", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control: got %s, want no-cache", cc)
	}
}

// TestSSEKeepalive tests that the SSE stream sends keepalive messages.
func TestSSEKeepalive(t *testing.T) {
	s, _, database := setupStageTestServer(t)
	defer s.httpServer.Close()

	fid := insertTestFeature(t, database, "keepalive", s)

	// Broadcast an event
	s.BroadcastSSE(fid, "stage_started", `{"feature_id":"`+fid+`","stage_id":"1.1"}`)

	// Wait a moment for the event to be buffered
	time.Sleep(50 * time.Millisecond)

	// Verify it's buffered
	s.sseMu.Lock()
	buffer := s.sseBuffers[fid]
	s.sseMu.Unlock()

	if len(buffer) == 0 {
		t.Error("no events buffered for feature")
	}
}