import { useState, useEffect, useRef } from 'react';
import { getCapturedOutput } from '../api/client';

interface AgentOutputProps {
  featureId: string;
  isProcessing: boolean;
}

interface OutputLine {
  line: string;
  isStderr: boolean;
  timestamp: Date;
}

export default function AgentOutput({ featureId, isProcessing }: AgentOutputProps) {
  const [lines, setLines] = useState<OutputLine[]>([]);
  const [isExpanded, setIsExpanded] = useState(true);
  const scrollRef = useRef<HTMLDivElement>(null);
  const lastLengthRef = useRef(0);

  // Poll /output endpoint every 2 seconds while processing.
  // Fetches the full tmux capture-pane output and appends only new lines.
  // This avoids the multiple-EventSource problem where useSSE events
  // get split across connections and lost.
  useEffect(() => {
    let cancelled = false;

    const poll = async () => {
      try {
        const data = await getCapturedOutput(featureId);
        if (cancelled || !data.output) return;

        const allLines = data.output.split('\n').filter((l) => l.trim());
        if (allLines.length > lastLengthRef.current) {
          const newLines = allLines.slice(lastLengthRef.current).map((l) => ({
            line: l,
            isStderr: false,
            timestamp: new Date(),
          }));
          setLines((prev) => {
            const combined = [...prev, ...newLines];
            return combined.length > 500 ? combined.slice(-500) : combined;
          });
          lastLengthRef.current = allLines.length;
        }
      } catch {
        // ignore
      }
    };

    // Initial fetch
    poll();

    // Poll every 2 seconds while processing
    if (isProcessing) {
      const interval = setInterval(poll, 2000);
      return () => {
        cancelled = true;
        clearInterval(interval);
      };
    }
  }, [featureId, isProcessing]);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [lines]);

  if (lines.length === 0) return null;

  return (
    <div className="bg-gray-900 rounded-lg shadow p-4 mb-6" data-testid="agent-output">
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-sm font-semibold text-gray-300">
          Agent Output
          <span className="ml-2 text-xs text-gray-500">{lines.length} lines</span>
        </h3>
        <button
          onClick={() => setIsExpanded(!isExpanded)}
          className="text-xs text-gray-400 hover:text-gray-200"
        >
          {isExpanded ? 'Collapse' : 'Expand'}
        </button>
      </div>
      {isExpanded && (
        <div
          ref={scrollRef}
          className="font-mono text-xs leading-5 max-h-80 overflow-y-auto rounded bg-gray-950 p-3"
          data-testid="agent-output-lines"
        >
          {lines.map((line, i) => (
            <div
              key={i}
              className={`${line.isStderr ? 'text-red-400' : 'text-gray-300'} whitespace-pre-wrap break-all`}
            >
              {line.line}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}