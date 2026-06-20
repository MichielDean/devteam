import { useState, useEffect } from 'react';
import { useSSE } from '../hooks/useSSE';
import { PHASE_LABELS } from '../types';
import type { PhaseName } from '../types';

interface ProcessViewProps {
  featureId: string;
}

interface ProcessStep {
  phase: string;
  type: string;
  data: unknown;
  timestamp: Date;
}

export default function ProcessView({ featureId }: ProcessViewProps) {
  const { lastEvent } = useSSE(featureId);
  const [steps, setSteps] = useState<ProcessStep[]>([]);
  const [startTime] = useState(Date.now());
  const [elapsed, setElapsed] = useState<string>('');
  const [isComplete, setIsComplete] = useState(false);

  useEffect(() => {
    if (!lastEvent) return;

    setSteps((prev) => [
      ...prev,
      {
        phase: (lastEvent.data as Record<string, string>).phase ?? '',
        type: lastEvent.type,
        data: lastEvent.data,
        timestamp: new Date(),
      },
    ]);

    if (lastEvent.type === 'processing_complete' || lastEvent.type === 'error') {
      setIsComplete(true);
    }
  }, [lastEvent]);

  // Update elapsed time every second
  useEffect(() => {
    if (isComplete) return;

    const interval = setInterval(() => {
      const diff = Date.now() - startTime;
      const seconds = Math.floor(diff / 1000);
      const minutes = Math.floor(seconds / 60);
      const secs = seconds % 60;
      setElapsed(minutes > 0 ? `${minutes}m ${secs}s` : `${secs}s`);
    }, 1000);

    return () => clearInterval(interval);
  }, [startTime, isComplete]);

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6" data-testid="process-view">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
          {isComplete ? 'Processing Complete' : 'Processing...'}
        </h3>
        {!isComplete && elapsed && (
          <span className="text-sm text-gray-500 dark:text-gray-400" data-testid="process-elapsed">
            {elapsed}
          </span>
        )}
      </div>

      {steps.length === 0 && !isComplete && (
        <div className="flex items-center gap-2 text-gray-500 dark:text-gray-400">
          <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600"></div>
          <span>Starting processing...</span>
        </div>
      )}

      <div className="space-y-2 max-h-64 overflow-y-auto" data-testid="process-steps">
        {steps.map((step, index) => (
          <div
            key={index}
            className="flex items-start gap-2 p-2 rounded-md bg-gray-50 dark:bg-gray-700 text-sm"
            data-testid={`process-step-${index}`}
          >
            <span className="shrink-0">
              {step.type === 'phase_change' && '🔄'}
              {step.type === 'gate_result' && ((step.data as Record<string, boolean>).passed ? '✅' : '❌')}
              {step.type === 'agent_dispatch' && '📤'}
              {step.type === 'agent_complete' && '📥'}
              {step.type === 'processing_complete' && '🎉'}
              {step.type === 'error' && '⚠️'}
            </span>
            <div className="flex-1 min-w-0">
              <p className="text-gray-900 dark:text-white">
                {step.type === 'phase_change' && `Phase changed to ${PHASE_LABELS[step.phase as PhaseName] || step.phase}`}
                {step.type === 'gate_result' && `Gate ${((step.data as Record<string, boolean>).passed) ? 'passed' : 'failed'} for ${PHASE_LABELS[step.phase as PhaseName] || step.phase}`}
                {step.type === 'agent_dispatch' && `Agent dispatched for ${PHASE_LABELS[step.phase as PhaseName] || step.phase}`}
                {step.type === 'agent_complete' && `Agent completed for ${PHASE_LABELS[step.phase as PhaseName] || step.phase}`}
                {step.type === 'processing_complete' && 'Processing complete!'}
                {step.type === 'error' && `Error: ${(step.data as Record<string, string>).message || 'Unknown error'}`}
              </p>
            </div>
            <span className="text-xs text-gray-400 dark:text-gray-500 shrink-0">
              {step.timestamp.toLocaleTimeString()}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}