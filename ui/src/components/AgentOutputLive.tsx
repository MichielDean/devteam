import { useState, useEffect, useRef } from 'react';
import { API_BASE } from '../api/client';

interface AgentOutputLiveProps {
  featureId: string;
  stageId?: string;
  isProcessing: boolean;
  phase?: string;
}

export default function AgentOutputLive({ featureId, stageId, isProcessing }: AgentOutputLiveProps) {
  const [content, setContent] = useState('');
  const [isExpanded, setIsExpanded] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const scrollRef = useRef<HTMLDivElement>(null);
  const wasAtBottom = useRef(true);

  // Poll the log file endpoint every 2 seconds while processing, every 10s when idle
  useEffect(() => {
    if (!stageId) return;

    const fetchLog = async () => {
      try {
        const res = await fetch(`${API_BASE}/features/${featureId}/log/${stageId}`);
        if (res.ok) {
          const text = await res.text();
          setContent(text);
        }
      } catch {
        // ignore
      }
    };

    fetchLog();
    const interval = isProcessing ? 2000 : 10000;
    const timer = setInterval(fetchLog, interval);
    return () => clearInterval(timer);
  }, [featureId, stageId, isProcessing]);

  // Auto-scroll to bottom if user was at bottom
  useEffect(() => {
    if (scrollRef.current && wasAtBottom.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [content]);

  const handleScroll = () => {
    if (!scrollRef.current) return;
    const el = scrollRef.current;
    wasAtBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 50;
  };

  if (!content && !isProcessing) return null;

  const lines = content.split('\n');
  const filteredLines = searchQuery
    ? lines.filter((l) => l.toLowerCase().includes(searchQuery.toLowerCase()))
    : lines;

  return (
    <div className="rounded-[var(--radius-lg)] overflow-hidden" style={{ backgroundColor: '#000' }} data-testid="agent-output-live">
      <div className="flex items-center justify-between px-4 py-2" style={{ backgroundColor: 'var(--color-surface-hover)' }}>
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-medium text-[var(--color-text-primary)]">
            Agent Output
            {stageId && <span className="ml-2 text-xs text-[var(--color-text-tertiary)]">· {stageId}</span>}
          </h3>
          <span className="text-xs text-[var(--color-text-tertiary)]">{lines.length} lines</span>
          {isProcessing && <span className="w-2 h-2 rounded-full animate-pulse" style={{ backgroundColor: 'var(--color-success)' }} title="Live" />}
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
          <button onClick={() => setIsExpanded(!isExpanded)} className="text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]">
            {isExpanded ? 'Collapse' : 'Expand'}
          </button>
        </div>
      </div>
      {isExpanded && (
        <div
          ref={scrollRef}
          onScroll={handleScroll}
          className="font-mono text-xs leading-5 max-h-96 overflow-y-auto p-3"
          style={{ fontFamily: 'var(--font-mono)', backgroundColor: '#000' }}
          data-testid="agent-output-lines"
        >
          {filteredLines.length === 0 ? (
            <div className="text-[var(--color-text-tertiary)] italic">{isProcessing ? 'Waiting for output...' : 'No output'}</div>
          ) : (
            filteredLines.map((line, i) => (
              <div key={i} className="whitespace-pre-wrap break-all" style={{ color: 'var(--color-text-secondary)' }}>
                {line}
              </div>
            ))
          )}
        </div>
      )}
    </div>
  );
}