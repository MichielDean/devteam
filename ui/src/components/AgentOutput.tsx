import { useState, useEffect, useRef } from 'react';
import { useSSE } from '../hooks/useSSE';
import { getCapturedOutput } from '../api/client';
import { PHASE_LABELS } from '../types';
import type { PhaseName } from '../types';

interface AgentOutputProps {
  featureId: string;
}

interface OutputLine {
  line: string;
  isStderr: boolean;
  timestamp: Date;
}

export default function AgentOutput({ featureId }: AgentOutputProps) {
  const [lines, setLines] = useState<OutputLine[]>([]);
  const [isExpanded, setIsExpanded] = useState(true);
  const scrollRef = useRef<HTMLDivElement>(null);
  const pendingLinesRef = useRef<OutputLine[]>([]);
  const flushTimerRef = useRef<number | null>(null);

  const flushLines = () => {
    flushTimerRef.current = null;
    if (pendingLinesRef.current.length === 0) return;
    setLines((prev) => {
      const combined = [...prev, ...pendingLinesRef.current];
      pendingLinesRef.current = [];
      return combined.length > 500 ? combined.slice(-500) : combined;
    });
  };

  const addLine = (line: string, isStderr: boolean) => {
    pendingLinesRef.current.push({ line, isStderr, timestamp: new Date() });
    if (flushTimerRef.current === null) {
      flushTimerRef.current = window.setTimeout(flushLines, 100);
    }
  };

  useEffect(() => {
    return () => {
      if (flushTimerRef.current) window.clearTimeout(flushTimerRef.current);
    };
  }, []);

  // Fetch existing tmux output on mount (for page refresh recovery)
  useEffect(() => {
    getCapturedOutput(featureId).then((data) => {
      if (data.output && data.output.trim()) {
        const existingLines = data.output.split('\n').filter((l) => l.trim()).map((l) => ({
          line: l,
          isStderr: false,
          timestamp: new Date(),
        }));
        setLines(existingLines);
      }
    }).catch(() => {});
  }, [featureId]);

  // Use onEvent callback to capture ALL events in real-time
  useSSE(featureId, (event) => {
    if (event.type === 'agent_output') {
      const data = event.data as Record<string, unknown>;
      const line = (data as Record<string, string>).line ?? '';
      const isStderr = (data as Record<string, boolean>).stderr ?? false;
      addLine(line, isStderr);
    } else if (event.type === 'agent_dispatch') {
      const data = event.data as Record<string, string>;
      addLine(`→ Agent dispatched for ${PHASE_LABELS[data.phase as PhaseName] || data.phase}`, false);
    } else if (event.type === 'agent_complete') {
      const data = event.data as Record<string, unknown>;
      const durationMs = (data as Record<string, number>).duration_ms ?? 0;
      const duration = durationMs > 0 ? ` (${Math.round(durationMs / 1000)}s)` : '';
      addLine(`✓ Agent completed${duration}`, false);
    } else if (event.type === 'gate_result') {
      const data = event.data as Record<string, boolean>;
      addLine(data.passed ? '✓ Quality check passed' : '✗ Quality check failed', !data.passed);
    } else if (event.type === 'phase_change') {
      const data = event.data as Record<string, string>;
      addLine(`Step changed to ${PHASE_LABELS[data.phase as PhaseName] || data.phase}`, false);
    } else if (event.type === 'phase_complete' || event.type === 'processing_complete') {
      addLine(event.type === 'processing_complete' ? '🎉 All done!' : '✓ Step complete', false);
    } else if (event.type === 'error') {
      const data = event.data as Record<string, string>;
      addLine(`⚠ Error: ${data.message ?? 'Unknown error'}`, true);
    } else if (event.type === 'waiting_for_human') {
      addLine('🙋 Waiting for your input', false);
    }
  });

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