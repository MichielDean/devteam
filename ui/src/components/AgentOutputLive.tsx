import { useState, useEffect, useRef, useCallback } from 'react';
import { useSSE } from '../hooks/useSSE';

interface AgentOutputLiveProps {
  featureId: string;
  stageId?: string;
  isProcessing: boolean;
}

interface OutputLine {
  line: string;
  isStderr: boolean;
  timestamp: Date;
}

export default function AgentOutputLive({ featureId, stageId, isProcessing }: AgentOutputLiveProps) {
  const [lines, setLines] = useState<OutputLine[]>([]);
  const [isExpanded, setIsExpanded] = useState(true);
  const [isPaused, setIsPaused] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const scrollRef = useRef<HTMLDivElement>(null);
  const { connected, subscribe } = useSSE(featureId);

  // Subscribe to agent_output SSE events
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

  // Auto-scroll on new lines
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
    <div className="bg-gray-900 rounded-lg shadow overflow-hidden" data-testid="agent-output-live">
      <div className="flex items-center justify-between px-4 py-2 bg-gray-950">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-semibold text-gray-300">
            Agent Output
            {stageId && <span className="ml-2 text-xs text-gray-500">· {stageId}</span>}
          </h3>
          <span className="text-xs text-gray-500">{lines.length} lines</span>
          <span className={`w-2 h-2 rounded-full ${connected ? 'bg-green-500' : 'bg-yellow-500'}`} title={connected ? 'Live' : 'Reconnecting'} />
        </div>
        <div className="flex items-center gap-2">
          <input
            type="text"
            placeholder="Search..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="px-2 py-0.5 text-xs bg-gray-800 text-gray-300 rounded border border-gray-700 focus:ring-1 focus:ring-blue-500"
            data-testid="output-search"
          />
          <button onClick={() => setIsPaused(!isPaused)} className="text-xs text-gray-400 hover:text-gray-200" data-testid="output-pause">
            {isPaused ? '▶ Resume' : '⏸ Pause'}
          </button>
          <button onClick={clearOutput} className="text-xs text-gray-400 hover:text-gray-200" data-testid="output-clear">
            Clear
          </button>
          <button onClick={() => setIsExpanded(!isExpanded)} className="text-xs text-gray-400 hover:text-gray-200">
            {isExpanded ? 'Collapse' : 'Expand'}
          </button>
        </div>
      </div>
      {isExpanded && (
        <div
          ref={scrollRef}
          className="font-mono text-xs leading-5 max-h-96 overflow-y-auto bg-gray-950 p-3"
          data-testid="agent-output-lines"
        >
          {filteredLines.length === 0 ? (
            <div className="text-gray-600 italic">{isProcessing ? 'Waiting for output...' : 'No output'}</div>
          ) : (
            filteredLines.map((line, i) => (
              <div
                key={i}
                className={`${line.isStderr ? 'text-red-400' : 'text-gray-300'} whitespace-pre-wrap break-all`}
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