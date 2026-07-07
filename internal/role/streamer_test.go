package role

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordingFlush records every flushed chunk and the order it was called.
type recordingFlush struct {
	mu     sync.Mutex
	chunks []string
	err    error // if set, returned on every call (for error-injection tests)
	calls  int32
}

func (f *recordingFlush) flush(ctx context.Context, featureID, stageID, agentRole string, bolt int, chunk string) error {
	atomic.AddInt32(&f.calls, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.chunks = append(f.chunks, chunk)
	return nil
}

func (f *recordingFlush) joined() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.chunks, "")
}

// TestStreamOutputLineBoundaryFlush verifies the always-on line-boundary flush
// trigger: each '\n'-terminated line is flushed as its own chunk (U-BK-03).
func TestStreamOutputLineBoundaryFlush(t *testing.T) {
	flush := &recordingFlush{}
	lineCh := make(chan OutputLine, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := "line one\nline two\nline three\n"
	r := strings.NewReader(in)

	cfg := StreamConfig{FlushIntervalMs: 10000, FlushBytes: 1024 * 1024} // disable time/size triggers
	buf, err := StreamOutput(ctx, r, lineCh, flush.flush, "feat", "1.1", "product", 0, cfg)
	if err != nil {
		t.Fatalf("StreamOutput: %v", err)
	}
	if buf != in {
		t.Errorf("buffer = %q, want %q", buf, in)
	}
	if got := flush.joined(); got != in {
		t.Errorf("flushed = %q, want %q", got, in)
	}
	// Each line should be its own chunk (3 lines).
	flush.mu.Lock()
	chunkCount := len(flush.chunks)
	flush.mu.Unlock()
	if chunkCount != 3 {
		t.Errorf("chunk count = %d, want 3 (one per line)", chunkCount)
	}
}

// TestStreamOutputSizeTriggerFlush verifies the flush_bytes threshold: when
// reads accumulate past the threshold without a line boundary, the pending
// buffer is flushed (U-BK-03 / R-1 product call). A single read that exceeds
// the threshold flushes as one chunk (the threshold bounds a single append's
// payload, not a hard split size); accumulation across reads is exercised here
// with a slow reader.
func TestStreamOutputSizeTriggerFlush(t *testing.T) {
	flush := &recordingFlush{}
	lineCh := make(chan OutputLine, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Emit 512-byte chunks with no newlines — the 1KB threshold is crossed
	// after the second read (512 + 512 = 1024 >= 1024).
	slow := &slowReader{
		chunks: []string{
			strings.Repeat("a", 512),
			strings.Repeat("b", 512),
			strings.Repeat("c", 512),
		},
		delay: 1 * time.Millisecond,
	}

	cfg := StreamConfig{FlushIntervalMs: 10000, FlushBytes: 1024}
	_, err := StreamOutput(ctx, slow, lineCh, flush.flush, "feat", "3.5", "developer", 1, cfg)
	if err != nil {
		t.Fatalf("StreamOutput: %v", err)
	}

	// The 1536 bytes (no newlines) should all be flushed — by the size trigger
	// during the run and the final flush at EOF.
	flush.mu.Lock()
	chunkCount := len(flush.chunks)
	totalLen := 0
	for _, c := range flush.chunks {
		totalLen += len(c)
	}
	flush.mu.Unlock()
	if totalLen != 1536 {
		t.Errorf("flushed total length = %d, want 1536", totalLen)
	}
	if chunkCount < 2 {
		t.Errorf("chunk count = %d, want >= 2 (size trigger should fire as reads accumulate past 1KB)", chunkCount)
	}
}

// TestStreamOutputTimeTriggerFlush verifies the flush_interval_ms threshold: a
// single long line with no '\n' is flushed by the ticker (U-BK-03 / R-1 product call).
func TestStreamOutputTimeTriggerFlush(t *testing.T) {
	flush := &recordingFlush{}
	lineCh := make(chan OutputLine, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// A slow reader that emits 10 bytes every 5ms with no newlines, so the
	// time trigger (50ms) must fire before EOF.
	slow := &slowReader{chunks: []string{
		strings.Repeat("a", 10),
		strings.Repeat("b", 10),
		strings.Repeat("c", 10),
		strings.Repeat("d", 10),
	}, delay: 5 * time.Millisecond}

	cfg := StreamConfig{FlushIntervalMs: 50, FlushBytes: 1024 * 1024}
	_, err := StreamOutput(ctx, slow, lineCh, flush.flush, "feat", "3.5", "developer", 1, cfg)
	if err != nil {
		t.Fatalf("StreamOutput: %v", err)
	}

	// The 40 bytes (no newlines) should all be flushed — by the time trigger
	// during the run, and the final flush at EOF.
	flush.mu.Lock()
	totalLen := 0
	for _, c := range flush.chunks {
		totalLen += len(c)
	}
	flush.mu.Unlock()
	if totalLen != 40 {
		t.Errorf("flushed total length = %d, want 40", totalLen)
	}
}

// TestStreamOutputFinalFlushOnEOF verifies the drain contract: the final flush
// occurs before StreamOutput returns (C-12 / FR-6 / ADR-2).
func TestStreamOutputFinalFlushOnEOF(t *testing.T) {
	flush := &recordingFlush{}
	lineCh := make(chan OutputLine, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Input with a trailing fragment that has no '\n' — must be flushed at EOF.
	in := "line one\nline two\ntrailing-fragment-no-newline"
	r := strings.NewReader(in)

	cfg := StreamConfig{FlushIntervalMs: 10000, FlushBytes: 1024 * 1024}
	buf, err := StreamOutput(ctx, r, lineCh, flush.flush, "feat", "1.1", "product", 0, cfg)
	if err != nil {
		t.Fatalf("StreamOutput: %v", err)
	}
	if buf != in {
		t.Errorf("buffer = %q, want %q", buf, in)
	}
	// The trailing fragment MUST be in the flushed output (final flush).
	if got := flush.joined(); got != in {
		t.Errorf("flushed = %q, want %q (trailing fragment must be final-flushed)", got, in)
	}
}

// TestStreamOutputNoDropOnCancel verifies cancellation triggers the final flush
// — no buffered tail is lost when the context is cancelled (C-12 / FR-6 / ADR-2).
func TestStreamOutputNoDropOnCancel(t *testing.T) {
	flush := &recordingFlush{}
	lineCh := make(chan OutputLine, 100)
	ctx, cancel := context.WithCancel(context.Background())

	// A slow reader that emits content then blocks — we cancel mid-read.
	slow := &slowReader{
		chunks: []string{"hello\n", "world"},
		delay:  50 * time.Millisecond,
	}

	cfg := StreamConfig{FlushIntervalMs: 10000, FlushBytes: 1024 * 1024}
	done := make(chan struct{})
	var buf string
	go func() {
		defer close(done)
		buf, _ = StreamOutput(ctx, slow, lineCh, flush.flush, "feat", "3.5", "developer", 1, cfg)
	}()

	// Wait for the first line to be flushed (it has a '\n', so it flushes immediately).
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	// "hello\n" should have been flushed before cancellation; "world" should
	// be flushed by the final flush on cancellation.
	flush.mu.Lock()
	got := strings.Join(flush.chunks, "")
	flush.mu.Unlock()
	if !strings.Contains(got, "hello\n") {
		t.Errorf("flushed = %q, missing %q (pre-cancel line)", got, "hello\\n")
	}
	if !strings.Contains(got, "world") {
		t.Errorf("flushed = %q, missing %q (final flush on cancel)", got, "world")
	}
	if buf != "hello\nworld" {
		t.Errorf("buffer = %q, want %q", buf, "hello\\nworld")
	}
}

// TestStreamOutputDualFanOutCoLocation verifies the DB flush and the channel
// send occur in the SAME flush, in persist-then-push order (C-4 / ADR-8).
func TestStreamOutputDualFanOutCoLocation(t *testing.T) {
	var flushCalls int32
	var chanReceives int32

	// A flush that blocks until the test signals it — so we can assert the
	// channel send has NOT happened yet (persist-then-push).
	blockFlush := make(chan struct{})
	flushReturned := make(chan struct{})
	flushFn := func(ctx context.Context, featureID, stageID, agentRole string, bolt int, chunk string) error {
		atomic.AddInt32(&flushCalls, 1)
		// Signal that we're inside the flush, then block.
		select {
		case <-blockFlush:
		case <-ctx.Done():
		}
		close(flushReturned)
		return nil
	}

	lineCh := make(chan OutputLine, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := "line\n"
	r := strings.NewReader(in)
	cfg := StreamConfig{FlushIntervalMs: 10000, FlushBytes: 1024 * 1024}

	done := make(chan struct{})
	go func() {
		defer close(done)
		StreamOutput(ctx, r, lineCh, flushFn, "feat", "1.1", "product", 0, cfg)
	}()

	// Wait for the flush to be called (the line has a '\n', so it flushes immediately).
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&flushCalls) != 1 {
		t.Fatalf("flush not called: calls=%d", atomic.LoadInt32(&flushCalls))
	}
	// The channel send should NOT have happened yet (persist-then-push: flush first).
	select {
	case <-lineCh:
		t.Fatal("channel received before flush returned — violates persist-then-push (ADR-8)")
	default:
		// good — channel is empty while flush is in flight
	}
	// Release the flush; the channel send should now happen.
	close(blockFlush)
	<-flushReturned
	select {
	case <-lineCh:
		atomic.AddInt32(&chanReceives, 1)
	case <-time.After(1 * time.Second):
		t.Fatal("channel did not receive after flush returned")
	}
	<-done

	if atomic.LoadInt32(&chanReceives) != 1 {
		t.Errorf("chanReceives = %d, want 1", atomic.LoadInt32(&chanReceives))
	}
}

// TestStreamOutputRetryOnceOnFlushError verifies the retry-once-with-backoff policy
// on a FlushFunc error: the second call is attempted, and the channel send still
// happens (O-10 / ADR-8).
func TestStreamOutputRetryOnceOnFlushError(t *testing.T) {
	var calls int32
	// Fail every odd call (the first attempt for each flush), succeed every
	// even call (the retry). This models "first attempt fails, retry succeeds"
	// for every flushed chunk.
	flushFn := func(ctx context.Context, featureID, stageID, agentRole string, bolt int, chunk string) error {
		n := atomic.AddInt32(&calls, 1)
		if n%2 == 1 {
			return errors.New("transient DB error")
		}
		return nil
	}

	lineCh := make(chan OutputLine, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := "line one\nline two\n"
	r := strings.NewReader(in)
	cfg := StreamConfig{FlushIntervalMs: 10000, FlushBytes: 1024 * 1024}
	_, err := StreamOutput(ctx, r, lineCh, flushFn, "feat", "1.1", "product", 0, cfg)
	if err != nil {
		t.Fatalf("StreamOutput: %v", err)
	}

	// 2 lines → 2 flush attempts → 4 calls (1 fail + 1 retry per line).
	if got := atomic.LoadInt32(&calls); got != 4 {
		t.Errorf("flush calls = %d, want 4 (2 lines × 2 calls each: fail + retry)", got)
	}
	// Both lines should have been received on the channel (channel send happens even after a failed retry).
	received := 0
	for {
		select {
		case <-lineCh:
			received++
		default:
			if received != 2 {
				t.Errorf("channel received = %d, want 2 (channel send still happens after retry failure)", received)
			}
			return
		}
	}
}

// TestStreamOutputPersistBeforePushOrder verifies persist-then-push ordering
// within a single flush: the DB flush call completes before the channel send
// (U-BK-03 / ADR-8). The unbuffered channel forces the send to block until the
// receiver consumes, so the recorded order is strict.
func TestStreamOutputPersistBeforePushOrder(t *testing.T) {
	order := make([]string, 0, 4)
	var mu sync.Mutex
	flushFn := func(ctx context.Context, featureID, stageID, agentRole string, bolt int, chunk string) error {
		mu.Lock()
		order = append(order, "flush:"+chunk)
		mu.Unlock()
		return nil
	}

	// Unbuffered channel — the send blocks until the receiver consumes, so
	// the next flush cannot start until the previous channel send completes.
	lineCh := make(chan OutputLine)
	go func() {
		for line := range lineCh {
			mu.Lock()
			order = append(order, "chan:"+line.Line)
			mu.Unlock()
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in := "a\nb\n"
	r := strings.NewReader(in)
	cfg := StreamConfig{FlushIntervalMs: 10000, FlushBytes: 1024 * 1024}
	_, err := StreamOutput(ctx, r, lineCh, flushFn, "feat", "1.1", "product", 0, cfg)
	if err != nil {
		t.Fatalf("StreamOutput: %v", err)
	}
	close(lineCh)

	// Wait briefly for the receiver to observe the last send + the close.
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	// Expected: flush:a, chan:a, flush:b, chan:b — persist-then-push per flush.
	want := []string{"flush:a\n", "chan:a\n", "flush:b\n", "chan:b\n"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i, w := range want {
		if order[i] != w {
			t.Errorf("order[%d] = %q, want %q", i, order[i], w)
		}
	}
}

// TestStreamOutputNilLineCh verifies a nil lineCh is tolerated (the non-streaming
// Dispatch() path passes nil — U-BK-03).
func TestStreamOutputNilLineCh(t *testing.T) {
	flush := &recordingFlush{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := "line one\nline two\n"
	r := strings.NewReader(in)
	cfg := StreamConfig{FlushIntervalMs: 10000, FlushBytes: 1024 * 1024}
	buf, err := StreamOutput(ctx, r, nil, flush.flush, "feat", "1.1", "product", 0, cfg)
	if err != nil {
		t.Fatalf("StreamOutput with nil lineCh: %v", err)
	}
	if buf != in {
		t.Errorf("buffer = %q, want %q", buf, in)
	}
	if got := flush.joined(); got != in {
		t.Errorf("flushed = %q, want %q", got, in)
	}
}

// TestStreamOutputConfigDefaults verifies that zero-value StreamConfig fields
// fall back to the documented defaults (200ms / 8192 bytes) (U-BK-03).
func TestStreamOutputConfigDefaults(t *testing.T) {
	cfg := StreamConfig{}
	if got := cfg.EffectiveFlushIntervalMs(); got != 200*time.Millisecond {
		t.Errorf("EffectiveFlushIntervalMs() = %v, want 200ms (default)", got)
	}
	if got := cfg.EffectiveFlushBytes(); got != 8192 {
		t.Errorf("EffectiveFlushBytes() = %d, want 8192 (default)", got)
	}
}

// slowReader is a test helper that emits chunks with a delay between them.
type slowReader struct {
	chunks []string
	delay  time.Duration
	idx    int
}

func (s *slowReader) Read(p []byte) (int, error) {
	if s.idx >= len(s.chunks) {
		return 0, io.EOF
	}
	time.Sleep(s.delay)
	c := s.chunks[s.idx]
	s.idx++
	n := copy(p, c)
	return n, nil
}