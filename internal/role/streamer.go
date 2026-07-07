package role

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"time"
)

// FlushFunc appends a chunk of agent output to the persistent store. It is
// injected by the composition root (the pipeline) so the role package does NOT
// import internal/db (AC-5 / ADR-7). The ctx governs the batcher's lifecycle;
// the store call itself is not context-cancellable today (arch-review S-2).
//
// On error, the batcher retries once with a short backoff, then logs and
// continues — the channel send (lineCh) still happens, so the UI stream does
// not lose a chunk because the DB had a transient (O-10 / ADR-8).
type FlushFunc func(ctx context.Context, featureID, stageID, agentRole string, bolt int, chunk string) error

// StreamConfig holds the tunable batcher flush thresholds. The line-boundary
// flush is always on and is not configurable.
type StreamConfig struct {
	FlushIntervalMs int // default 200 (NFR-1: bounds live latency)
	FlushBytes      int // default 8192 (8KB)
}

// EffectiveFlushIntervalMs returns the configured interval or the default (200) if <= 0.
func (c StreamConfig) EffectiveFlushIntervalMs() time.Duration {
	ms := c.FlushIntervalMs
	if ms <= 0 {
		ms = 200
	}
	return time.Duration(ms) * time.Millisecond
}

// EffectiveFlushBytes returns the configured size threshold or the default (8192) if <= 0.
func (c StreamConfig) EffectiveFlushBytes() int {
	if c.FlushBytes <= 0 {
		return 8192
	}
	return c.FlushBytes
}

// retryBackoff is the pause before a single retry on a FlushFunc error (O-10).
const retryBackoff = 50 * time.Millisecond

// StreamOutput reads r, buffers chunks, and on each flush calls flushFn AND
// sends the chunk to lineCh. It returns the full accumulated in-memory buffer
// (for result.Output — ADR-6) and blocks until r returns io.EOF or ctx is
// cancelled. The final flush occurs before return (C-12 / FR-6).
//
// Drain contract (C-5 / R-1 / ADR-2): StreamOutput returns ONLY after its final
// flush completes. The caller (stage_runner) closes lineCh at the existing
// drain point (stage_runner.go:193) AFTER this function returns. There is
// therefore no window for a late append against a second writer.
//
// Dual fan-out (C-4 / FR-4 / ADR-8): the DB flush and the lineCh send occur in
// the SAME flush, in the SAME goroutine, with persist-then-push ordering —
// flushFn first, then the channel send. If the DB flush fails (after retry),
// the channel send STILL happens so the UI does not lose the chunk.
//
// Flush triggers: line boundary (always on) OR flush_interval_ms OR flush_bytes
// — whichever comes first. The interval/size triggers MAY split a line
// ([PRODUCT CALL] R-1); the line-boundary trigger flushes the complete line.
//
// lineCh is NOT closed here — the caller closes it. A nil lineCh is tolerated
// (the dispatcher uses Dispatch() with lineCh=nil for non-streaming callers).
func StreamOutput(ctx context.Context, r io.Reader, lineCh chan<- OutputLine,
	flushFn FlushFunc, featureID, stageID, agentRole string, bolt int,
	cfg StreamConfig) (buffer string, err error) {

	var buf bytes.Buffer
	flushInterval := cfg.EffectiveFlushIntervalMs()
	flushBytes := cfg.EffectiveFlushBytes()

	// flush sends chunk to the DB (persist-then-push) and the channel.
	flush := func(chunk string) {
		if chunk == "" {
			return
		}
		// Persist-then-push (ADR-8): DB flush FIRST, then channel send.
		// On FlushFn error: retry once with backoff, then log-and-continue.
		// The channel send STILL happens (O-10).
		if flushFn != nil {
			if fErr := flushFn(ctx, featureID, stageID, agentRole, bolt, chunk); fErr != nil {
				time.Sleep(retryBackoff)
				if fErr2 := flushFn(ctx, featureID, stageID, agentRole, bolt, chunk); fErr2 != nil {
					log.Printf("streamer: FlushFn failed after retry (chunk %d bytes) for %s/%s: %v — channel send still happens",
						len(chunk), featureID, stageID, fErr2)
				}
			}
		}
		// Channel send — synchronous, applies backpressure to the batcher.
		if lineCh != nil {
			select {
			case lineCh <- OutputLine{Line: chunk, IsStderr: false}:
			case <-ctx.Done():
				// Context cancelled while blocked on the channel — drop the
				// chunk to honor cancellation. The DB has it (persist-then-push).
				return
			}
		}
	}

	// finalFlush flushes the buffered tail. Called on EOF and ctx cancellation.
	finalFlush := func(pending *bytes.Buffer) {
		if pending.Len() > 0 {
			flush(pending.String())
			pending.Reset()
		}
	}

	// Read loop with a flush ticker. We use a small read buffer and accumulate
	// into a pending buffer; on each tick, line boundary, or size threshold, we
	// flush. A line is flushed when a '\n' is encountered (the chunk includes
	// the '\n').
	var pending bytes.Buffer
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	readBuf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			finalFlush(&pending)
			return buf.String(), ctx.Err()
		case <-ticker.C:
			// Time trigger — flush whatever is pending (may split a line, R-1 product call).
			if pending.Len() > 0 {
				chunk := pending.String()
				buf.WriteString(chunk)
				pending.Reset()
				flush(chunk)
			}
		default:
			n, rErr := r.Read(readBuf)
			if n > 0 {
				data := readBuf[:n]
				buf.Write(data)
				// Walk the read chunk, flushing on each line boundary.
				start := 0
				for i := 0; i < len(data); i++ {
					if data[i] == '\n' {
						// Append the line (including '\n') to pending and flush.
						pending.Write(data[start : i+1])
						start = i + 1
						chunk := pending.String()
						pending.Reset()
						flush(chunk)
						// Reset the ticker so a line flush doesn't immediately
						// trigger a redundant time flush.
						ticker.Reset(flushInterval)
					}
				}
				// Append the trailing fragment (no '\n') to pending.
				if start < len(data) {
					pending.Write(data[start:])
				}
				// Size trigger — flush if pending exceeds the byte threshold
				// (may split a line, R-1 product call).
				if pending.Len() >= flushBytes {
					chunk := pending.String()
					pending.Reset()
					flush(chunk)
					ticker.Reset(flushInterval)
				}
			}
			if rErr != nil {
				if rErr == io.EOF {
					finalFlush(&pending)
					return buf.String(), nil
				}
				// Non-EOF read error — flush what we have, return the error.
				finalFlush(&pending)
				return buf.String(), fmt.Errorf("streamer: read error: %w", rErr)
			}
		}
	}
}