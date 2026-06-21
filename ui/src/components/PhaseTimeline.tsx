import { PHASE_LABELS } from '../types';
import type { PhaseName, PhaseState } from '../types';

interface PhaseTimelineProps {
  phases: readonly PhaseName[];
  currentPhase: string;
  phaseStates: Record<string, PhaseState>;
}

export default function PhaseTimeline({ phases, currentPhase, phaseStates }: PhaseTimelineProps) {
  const currentIndex = phases.indexOf(currentPhase as PhaseName);

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6" data-testid="phase-timeline">
      <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Progress</h3>
      <div className="flex items-center justify-between overflow-x-auto pb-2">
        {phases.map((phase, index) => {
          const state = phaseStates[phase];
          const isCurrent = phase === currentPhase;
          const isCompleted = index < currentIndex || state?.status === 'passed' || state?.status === 'done';
          const isFailed = state?.gate_result && !state.gate_result.passed;
          const isInProgress = isCurrent && (state?.status === 'in_progress' || state?.status === 'gate_blocked');

          let bgColor = 'bg-gray-200 dark:bg-gray-600'; // default: not started
          let textColor = 'text-gray-500 dark:text-gray-400';
          let icon = '';

          if (isCompleted) {
            bgColor = 'bg-green-500';
            textColor = 'text-white';
            icon = '✓';
          } else if (isFailed) {
            bgColor = 'bg-red-500';
            textColor = 'text-white';
            icon = '✗';
          } else if (isInProgress) {
            bgColor = 'bg-blue-500';
            textColor = 'text-white';
            icon = '⟳';
          }

          return (
            <div key={phase} className="flex items-center flex-1 min-w-0">
              <div className="flex flex-col items-center flex-1 min-w-0">
                <div
                  className={`w-8 h-8 rounded-full ${bgColor} ${textColor} flex items-center justify-center text-sm font-bold shrink-0`}
                  data-testid={`phase-dot-${phase}`}
                >
                  {icon || (index + 1)}
                </div>
                <span
                  className={`text-xs mt-1 text-center truncate w-full ${
                    isCurrent ? 'font-semibold text-gray-900 dark:text-white' : 'text-gray-500 dark:text-gray-400'
                  }`}
                >
                  {PHASE_LABELS[phase]}
                </span>
              </div>
              {index < phases.length - 1 && (
                <div className="flex-shrink-0 w-4 h-0.5 bg-gray-300 dark:bg-gray-600 mx-1" />
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}