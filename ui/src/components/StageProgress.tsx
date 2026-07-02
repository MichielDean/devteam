import { PHASE_LABELS, STAGE_CHECKBOX } from '../types';
import type { FeatureStage } from '../types';

interface StageProgressProps {
  stages: FeatureStage[];
  currentStageId?: string;
}

// Group stages by phase for display
function groupByPhase(stages: FeatureStage[]): Record<string, FeatureStage[]> {
  const groups: Record<string, FeatureStage[]> = {};
  for (const s of stages) {
    // Extract phase from stage_id: "1.1" → phase "1" → ideation
    const phaseNum = s.stage_id.split('.')[0];
    let phaseName = '';
    switch (phaseNum) {
      case '0': phaseName = 'initialization'; break;
      case '1': phaseName = 'ideation'; break;
      case '2': phaseName = 'inception'; break;
      case '3': phaseName = 'construction'; break;
      case '4': phaseName = 'operation'; break;
      default: phaseName = 'unknown';
    }
    if (!groups[phaseName]) groups[phaseName] = [];
    groups[phaseName].push(s);
  }
  return groups;
}

const STATUS_COLORS: Record<string, string> = {
  not_started: 'text-gray-400',
  in_progress: 'text-blue-600 dark:text-blue-400',
  awaiting_approval: 'text-yellow-600 dark:text-yellow-400',
  revising: 'text-orange-600 dark:text-orange-400',
  completed: 'text-green-600 dark:text-green-400',
  skipped: 'text-gray-400',
};

export default function StageProgress({ stages, currentStageId }: StageProgressProps) {
  const grouped = groupByPhase(stages);
  const completed = stages.filter((s) => s.status === 'completed').length;
  const skipped = stages.filter((s) => s.status === 'skipped').length;
  const total = stages.length;

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6" data-testid="stage-progress">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Progress</h3>
        <span className="text-sm text-gray-500 dark:text-gray-400" data-testid="stage-count">
          {completed}/{total - skipped} completed ({skipped} skipped)
        </span>
      </div>

      {stages.length === 0 ? (
        <p className="text-sm text-gray-500 dark:text-gray-400" data-testid="no-stages">
          No stages initialized. Stages will appear when the feature starts.
        </p>
      ) : (
        <div className="space-y-4" data-testid="stage-list">
          {Object.entries(grouped).map(([phase, phaseStages]) => (
            <div key={phase} data-testid={`phase-group-${phase}`}>
              <h4 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2" data-testid={`phase-label-${phase}`}>
                {PHASE_LABELS[phase] || phase}
              </h4>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
                {phaseStages.map((s) => {
                  const isCurrent = s.stage_id === currentStageId;
                  const color = STATUS_COLORS[s.status] || 'text-gray-400';
                  return (
                    <div
                      key={s.stage_id}
                      className={`flex items-center gap-2 px-3 py-2 rounded-lg text-sm ${
                        isCurrent ? 'bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800' : 'bg-gray-50 dark:bg-gray-900/30'
                      }`}
                      data-testid={`stage-item-${s.stage_id}`}
                    >
                      <span className={`font-mono text-sm ${color}`} data-testid={`stage-checkbox-${s.stage_id}`}>
                        {STAGE_CHECKBOX[s.status] || '[ ]'}
                      </span>
                      <span className={`flex-1 truncate ${isCurrent ? 'font-semibold text-gray-900 dark:text-white' : 'text-gray-600 dark:text-gray-400'}`}>
                        {s.stage_id}
                      </span>
                      {s.revision_count > 0 && (
                        <span className="text-xs text-orange-500" data-testid={`stage-revisions-${s.stage_id}`}>
                          ×{s.revision_count}
                        </span>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}