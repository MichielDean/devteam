import { useState, useEffect, useRef, useCallback } from 'react';
import { useSSE } from '../hooks/useSSE';
import { getSessionOutput } from '../api/client';

interface AgentOutputLiveProps {
  featureId: string;
  stageId?: string;
  isProcessing: boolean;
  phase?: string;
}

interface OutputLine {
  line: string;
  isStderr: boolean;
  timestamp: Date;
}

export default function AgentOutputLive({ featureId, stageId, isProcessing, phase }: AgentOutputLiveProps) {
  const [lines, setLines] = useState<OutputLine[]>([]);
  const [isExpanded, setIsExpanded] = useState(true);
  const [isPaused, setIsPaused] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const scrollRef = useRef<HTMLDivElement>(null);
  const { connected, subscribe } = useSSE(featureId);

  // Load existing log content on mount — restores output after page refresh
  useEffect(() => {
    if (!phase) return;
    getSessionOutput(featureId, phase, stageId)
      .then((output) => {
        if (output && output.trim()) {
          const existingLines = output.split('\n').filter((l) => l.trim());
          if (existingLines.length > 0) {
            setLines(existingLines.map((line) => ({ line, isStderr: false, timestamp: new Date() })));
          }
        }
      })
      .catch(() => {});
  }, [featureId, phase, stageId]);

  // SSE subscription for live agent output — appends to existing log content
  useEffect(() => {
    const unsubscribe = subscribe('agent_output', (event) => {
      if (isPaused) return;
      const data = event.data as { line?: string; stderr?: boolean };
      if (data?.line) {
        const line = data.line;
        setLines((prev) => {
          const combined = [...prev, { line, isStderr: data.stderr ?? false, timestamp: new Date() }];
          return combined.length > 500 ? combined.slice(-500) : combined;
        });
      }
    });
    return unsubscribe;
  }, [subscribe, isPaused]);

  // No polling fallback — log content is loaded on mount and SSE handles live updates

  useEffect(() => {
    if (scrollRef.current && !isPaused) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [lines, isPaused]);

  const filteredLines = searchQuery
    ? lines.filter((l) => l.line.toLowerCase().includes(searchQuery.toLowerCase()))
    : lines;

  const clearOutput = useCallback(() => setLines([]), []);

  if (lines.length === 0 && !isProcessing) return null;

  return (
    <div className="rounded-[var(--radius-lg)] overflow-hidden" style={{ backgroundColor: '#000' }} data-testid="agent-output-live">
      <div className="flex items-center justify-between px-4 py-2" style={{ backgroundColor: 'var(--color-surface-hover)' }}>
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-medium text-[var(--color-text-primary)]">
            Agent Output
            {stageId && <span className="ml-2 text-xs text-[var(--color-text-tertiary)]">· {stageId}</span>}
          </h3>
          <span className="text-xs text-[var(--color-text-tertiary)]">{lines.length} lines</span>
          <span className="w-2 h-2 rounded-full" style={{ backgroundColor: connected ? 'var(--color-success)' : 'var(--color-warning)' }} title={connected ? 'Live' : 'Reconnecting'} />
        </div>
        <div className="flex items-center gap-2">
          <input
            type="text"
            placeholder="Search..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="px-2 py-0.5 text-xs rounded-[var(--radius-sm)] text-[var(--color-text-primary)] border border-[var(--color-border-default)] focus:border-[var(--color-accent)] focus:outline-none"
            style={{ backgroundColor: 'var(--color-surface-raised)' }}
            data-testid="output-search"
          />
          <button onClick={() => setIsPaused(!isPaused)} className="text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]" data-testid="output-pause">
            {isPaused ? '▶ Resume' : '⏸ Pause'}
          </button>
          <button onClick={clearOutput} className="text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]" data-testid="output-clear">
            Clear
          </button>
          <button onClick={() => setIsExpanded(!isExpanded)} className="text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]">
            {isExpanded ? 'Collapse' : 'Expand'}
          </button>
        </div>
      </div>
      {isExpanded && (
        <div
          ref={scrollRef}
          className="font-mono text-xs leading-5 max-h-96 overflow-y-auto p-3"
          style={{ fontFamily: 'var(--font-mono)', backgroundColor: '#000' }}
          data-testid="agent-output-lines"
        >
          {filteredLines.length === 0 ? (
            <div className="text-[var(--color-text-tertiary)] italic">{isProcessing ? 'Waiting for output...' : 'No output'}</div>
          ) : (
            filteredLines.map((line, i) => (
              <div
                key={i}
                className="whitespace-pre-wrap break-all"
                style={{ color: line.isStderr ? 'var(--color-danger)' : 'var(--color-text-secondary)' }}
              >
                {line.line}
              </div>
            ))
          )}
        </div>
      )}
    </div>
  );
}