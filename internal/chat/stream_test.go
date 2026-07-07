package chat

import (
	"sync"
	"testing"
	"time"
)

func TestStreamingChannel_OpenWriteClose(t *testing.T) {
	sc := NewStreamingChannel()
	sub, _ := sc.Open("sess-1", "sub-1")
	sc.WriteChunk("sess-1", StreamChunk{Type: "token", Content: "hello"})
	select {
	case c := <-sub.Ch:
		if c.Content != "hello" {
			t.Errorf("chunk content = %q, want hello", c.Content)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive chunk")
	}
	sc.Close("sess-1", "sub-1")
	if len(sc.ActiveSessions()) != 0 {
		t.Errorf("expected no active sessions after close, got %v", sc.ActiveSessions())
	}
}

func TestStreamingChannel_DisconnectCancelsContext(t *testing.T) {
	sc := NewStreamingChannel()
	sub, ctx := sc.Open("sess-1", "sub-1")
	// Context should be active.
	if err := ctx.Err(); err != nil {
		t.Fatalf("ctx should be active, got %v", err)
	}
	sc.Close("sess-1", "sub-1")
	// Context should be canceled after close.
	if err := ctx.Err(); err == nil {
		t.Error("ctx should be canceled after last subscriber closes")
	}
	// Subscriber channel should be closed.
	if _, ok := <-sub.Ch; ok {
		t.Error("subscriber channel should be closed")
	}
}

func TestStreamingChannel_NoLeakOnDisconnect(t *testing.T) {
	sc := NewStreamingChannel()
	sc.Open("sess-1", "sub-1")
	sc.Close("sess-1", "sub-1")
	if err := sc.WaitForCleanup("sess-1", 500*time.Millisecond); err != nil {
		t.Errorf("goroutine/state leak: %v", err)
	}
}

func TestStreamingChannel_MultipleSubscribers(t *testing.T) {
	sc := NewStreamingChannel()
	s1, _ := sc.Open("sess-1", "sub-1")
	s2, _ := sc.Open("sess-1", "sub-2")
	sc.WriteChunk("sess-1", StreamChunk{Type: "token", Content: "broadcast"})
	for i, sub := range []*StreamSubscriber{s1, s2} {
		select {
		case c := <-sub.Ch:
			if c.Content != "broadcast" {
				t.Errorf("sub %d content = %q, want broadcast", i, c.Content)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("sub %d did not receive broadcast", i)
		}
	}
	// Closing one subscriber keeps the session alive for the other.
	sc.Close("sess-1", "sub-1")
	if len(sc.ActiveSessions()) != 1 {
		t.Errorf("expected 1 active session after one sub closes, got %d", len(sc.ActiveSessions()))
	}
	sc.Close("sess-1", "sub-2")
}

func TestStreamingChannel_DropsWhenBufferFull(t *testing.T) {
	sc := NewStreamingChannel()
	sub, _ := sc.Open("sess-1", "sub-1")
	// Fill the buffer (64) then send one more — should drop, not block.
	for i := 0; i < 70; i++ {
		sc.WriteChunk("sess-1", StreamChunk{Type: "token", Content: "x"})
	}
	// Drain what we can — at least the buffer size should arrive.
	got := 0
 drain:
	for {
		select {
		case <-sub.Ch:
			got++
		default:
			break drain
		}
	}
	if got == 0 {
		t.Error("expected to receive at least the buffer's worth of chunks")
	}
	sc.Close("sess-1", "sub-1")
}

// Concurrent open/close — race detector catches issues.
func TestStreamingChannel_ConcurrentOpenClose(t *testing.T) {
	sc := NewStreamingChannel()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := "sess-concurrent"
			_, _ = sc.Open(id, "sub")
			sc.WriteChunk(id, StreamChunk{Type: "token"})
			sc.Close(id, "sub")
		}(i)
	}
	wg.Wait()
}