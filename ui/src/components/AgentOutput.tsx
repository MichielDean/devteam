import { useState, useEffect, useRef } from 'react';
import { useSSE } from '../hooks/useSSE';
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
  const { lastEvent } = useSSE(featureId);
  const [lines, setLines] = useState<OutputLine[]>([]);
  const [isExpanded, setIsExpanded] = useState(true);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!lastEvent) return;

    if (lastEvent.type === 'agent_output') {
      const data = lastEvent.data as Record<string, unknown>;
      const line = (data as Record<string, string>).line ?? '';
      const isStderr = (data as Record<string, boolean>).stderr ?? false;
      setLines((prev) => [...prev, { line, isStderr, timestamp: new Date() }]);
    } else if (lastEvent.type === 'agent_dispatch') {
      setLines((prev) => [...prev, {
        line: `→ Agent dispatched for ${PHASE_LABELS[(lastEvent.data as Record<string, string>).phase as PhaseName] || (lastEvent.data as Record<string, string>).phase}`,
        isStderr: false,
        timestamp: new Date(),
      }]);
    } else if (lastEvent.type === 'agent_complete') {
      const data = lastEvent.data as Record<string, unknown>;
      const durationMs = (data as Record<string, number>).duration_ms ?? 0;
      const duration = durationMs > 0 ? ` (${Math.round(durationMs / 1000)}s)` : '';
      setLines((prev) => [...prev, {
        line: `✓ Agent completed${duration}`,
        isStderr: false,
        timestamp: new Date(),
      }]);
    } else if (lastEvent.type === 'gate_result') {
      const data = lastEvent.data as Record<string, unknown>;
      const passed = (data as Record<string, boolean>).passed;
      setLines((prev) => [...prev, {
        line: passed ? '✓ Gate passed' : '✗ Gate failed',
        isStderr: !passed,
        timestamp: new Date(),
      }]);
    } else if (lastEvent.type === 'phase_change') {
      const data = lastEvent.data as Record<string, string>;
      setLines((prev) => [...prev, {
        line: `Phase changed to ${PHASE_LABELS[data.phase as PhaseName] || data.phase}`,
        isStderr: false,
        timestamp: new Date(),
      }]);
    } else if (lastEvent.type === 'phase_complete' || lastEvent.type === 'processing_complete') {
      setLines((prev) => [...prev, {
        line: lastEvent.type === 'processing_complete' ? '🎉 Pipeline complete' : '✓ Phase complete',
        isStderr: false,
        timestamp: new Date(),
      }]);
    } else if (lastEvent.type === 'error') {
      const data = lastEvent.data as Record<string, string>;
      setLines((prev) => [...prev, {
        line: `⚠ Error: ${data.message ?? 'Unknown error'}`,
        isStderr: true,
        timestamp: new Date(),
      }]);
    } else if (lastEvent.type === 'waiting_for_human') {
      setLines((prev) => [...prev, {
        line: '🙋 Waiting for human input',
        isStderr: false,
        timestamp: new Date(),
      }]);
    }
  }, [lastEvent]);

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