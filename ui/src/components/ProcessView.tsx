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

const STEP_DESCRIPTIONS: Record<string, string> = {
  phase_change: 'Phase changed',
  gate_result: 'Gate evaluated',
  agent_dispatch: 'Agent started work',
  agent_complete: 'Agent finished work',
  processing_complete: 'Pipeline complete',
  error: 'Error',
  waiting_for_human: 'Paused for input',
  questions_answered: 'All questions answered',
  questions_assumed: 'Questions auto-assumed',
};

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

  const currentPhase = steps.length > 0 ? steps[steps.length - 1].phase : '';
  const currentPhaseLabel = PHASE_LABELS[currentPhase as PhaseName] || currentPhase;

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6 border-l-4 border-indigo-500" data-testid="process-view">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          {!isComplete && (
            <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-indigo-600"></div>
          )}
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
            {isComplete ? 'Pipeline Complete' : `Autopilot Running — ${currentPhaseLabel}`}
          </h3>
        </div>
        {!isComplete && elapsed && (
          <span className="text-sm text-gray-500 dark:text-gray-400 font-mono" data-testid="process-elapsed">
            {elapsed}
          </span>
        )}
      </div>

      {!isComplete && steps.length === 0 && (
        <div className="flex items-center gap-2 text-gray-500 dark:text-gray-400">
          <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600"></div>
          <span>Starting autopilot...</span>
        </div>
      )}

      <div className="space-y-2 max-h-64 overflow-y-auto" data-testid="process-steps">
        {steps.map((step, index) => {
          const data = step.data as Record<string, unknown>;
          const isLast = index === steps.length - 1;
          return (
            <div
              key={index}
              className={`flex items-start gap-2 p-2 rounded-md text-sm ${
                isLast
                  ? 'bg-indigo-50 dark:bg-indigo-900/20 border border-indigo-200 dark:border-indigo-800'
                  : 'bg-gray-50 dark:bg-gray-700'
              }`}
              data-testid={`process-step-${index}`}
            >
              <span className="shrink-0 mt-0.5">
                {step.type === 'phase_change' && '🔄'}
                {step.type === 'gate_result' && ((data as Record<string, boolean>).passed ? '✅' : '❌')}
                {step.type === 'agent_dispatch' && '📤'}
                {step.type === 'agent_complete' && '📥'}
                {step.type === 'processing_complete' && '🎉'}
                {step.type === 'error' && '⚠️'}
                {step.type === 'waiting_for_human' && '🙋'}
                {step.type === 'questions_answered' && '💬'}
                {step.type === 'questions_assumed' && '⏰'}
              </span>
              <div className="flex-1 min-w-0">
                <p className="text-gray-900 dark:text-white font-medium">
                  {PHASE_LABELS[step.phase as PhaseName] || step.phase}
                </p>
                <p className="text-gray-600 dark:text-gray-400 text-xs">
                  {STEP_DESCRIPTIONS[step.type] || step.type}
                  {step.type === 'gate_result' && (
                    (data as Record<string, boolean>).passed
                      ? ' — passed'
                      : ' — failed'
                  )}
                  {step.type === 'agent_dispatch' && ` (${(data as Record<string, string>).role || ''})`}
                  {step.type === 'error' && `: ${(data as Record<string, string>).message || 'Unknown error'}`}
                </p>
              </div>
              <span className="text-xs text-gray-400 dark:text-gray-500 shrink-0">
                {step.timestamp.toLocaleTimeString()}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}