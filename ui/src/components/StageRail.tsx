import { useUIStore } from '../store/ui-store';
import { Badge } from '../ui/primitives';
import { PHASE_LABELS, AGENT_LABELS } from '../types';
import type { FeatureStage, StageDefinitionDetail } from '../types';

interface StageRailProps {
  stages: FeatureStage[];
  stageDefinitions?: StageDefinitionDetail[];
  currentStageId?: string;
}

const STATUS_ICONS: Record<string, string> = {
  not_started: '○',
  in_progress: '▶',
  awaiting_approval: '?',
  revising: '↻',
  completed: '✓',
  skipped: '·',
};

const statusColor: Record<string, string> = {
  not_started: 'var(--color-text-tertiary)',
  in_progress: 'var(--color-accent)',
  awaiting_approval: 'var(--color-warning)',
  revising: 'var(--color-warning)',
  completed: 'var(--color-success)',
  skipped: 'var(--color-text-tertiary)',
};

function groupByPhase(stages: FeatureStage[]): Record<string, FeatureStage[]> {
  const groups: Record<string, FeatureStage[]> = {};
  for (const s of stages) {
    const phaseNum = s.stage_id.split('.')[0];
    const phaseName = phaseNum === '0' ? 'initialization' : phaseNum === '1' ? 'ideation' : phaseNum === '2' ? 'inception' : phaseNum === '3' ? 'construction' : phaseNum === '4' ? 'operation' : 'unknown';
    if (!groups[phaseName]) groups[phaseName] = [];
    groups[phaseName].push(s);
  }
  return groups;
}

export default function StageRail({ stages, stageDefinitions, currentStageId }: StageRailProps) {
  const { selectedStageId, setSelectedStage } = useUIStore();
  const grouped = groupByPhase(stages);
  const completed = stages.filter((s) => s.status === 'completed').length;
  const total = stages.length;

  const getStageDef = (stageId: string): StageDefinitionDetail | undefined =>
    stageDefinitions?.find((d) => d.id === stageId);

  return (
    <div className="w-64 shrink-0 rounded-[var(--radius-lg)] overflow-y-auto h-full" style={{ backgroundColor: 'var(--color-surface-raised)', boxShadow: 'var(--shadow-sm)' }} data-testid="stage-rail">
      <div className="p-3 border-b border-[var(--color-border-subtle)]">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-medium text-[var(--color-text-primary)]">Stages</h3>
          <span className="text-xs text-[var(--color-text-tertiary)]" data-testid="rail-progress">{completed}/{total}</span>
        </div>
        <div className="mt-2 h-1 rounded-full overflow-hidden" style={{ backgroundColor: 'var(--color-border-subtle)' }}>
          <div className="h-full transition-all" style={{ width: `${total > 0 ? (completed / total) * 100 : 0}%`, backgroundColor: 'var(--color-accent)' }} />
        </div>
      </div>

      {stages.length === 0 ? (
        <p className="text-xs text-[var(--color-text-tertiary)] p-3" data-testid="rail-empty">No stages initialized.</p>
      ) : (
        <div className="p-2 space-y-3" data-testid="rail-stages">
          {Object.entries(grouped).map(([phase, phaseStages]) => (
            <div key={phase} data-testid={`rail-phase-${phase}`}>
              <h4 className="text-[10px] font-medium text-[var(--color-text-tertiary)] uppercase tracking-wider mb-1 px-1">
                {PHASE_LABELS[phase] || phase}
              </h4>
              <div className="space-y-0.5">
                {phaseStages.map((s) => {
                  const def = getStageDef(s.stage_id);
                  const isCurrent = s.stage_id === currentStageId;
                  const isSelected = s.stage_id === selectedStageId;
                  const icon = STATUS_ICONS[s.status] || '○';
                  const color = statusColor[s.status] || 'var(--color-text-tertiary)';
                  return (
                    <button
                      key={s.stage_id}
                      onClick={() => setSelectedStage(s.stage_id)}
                      title={def?.description || undefined}
                      className={`w-full flex items-center gap-2 px-2 py-1.5 rounded-[var(--radius-sm)] text-left text-xs transition-colors ${
                        isSelected
                          ? 'bg-[var(--color-surface-active)]'
                          : 'hover:bg-[var(--color-surface-hover)]'
                      }`}
                      style={isSelected ? { borderLeft: `2px solid var(--color-accent)`, paddingLeft: '6px' } : { borderLeft: '2px solid transparent' }}
                      data-testid={`rail-stage-${s.stage_id}`}
                    >
                      <span className="w-4 text-center shrink-0 font-mono text-xs" style={{ color }} data-testid={`rail-icon-${s.stage_id}`}>{icon}</span>
                      <div className="flex-1 min-w-0">
                        <div className={`truncate ${isCurrent ? 'font-medium text-[var(--color-text-primary)]' : 'text-[var(--color-text-secondary)]'}`}>
                          {s.stage_id} {def?.name ? `· ${def.name}` : ''}
                        </div>
                        {def && (
                          <div className="text-[10px] text-[var(--color-text-tertiary)] truncate">{AGENT_LABELS[def.lead_agent] || def.lead_agent}</div>
                        )}
                      </div>
                      {s.revision_count > 0 && <Badge color="yellow" className="text-[10px] px-1 py-0">×{s.revision_count}</Badge>}
                      {def?.reviewer && <span className="text-[10px]" style={{ color: 'var(--color-text-tertiary)' }} title={`Reviewer: ${AGENT_LABELS[def.reviewer]}`}>🔍</span>}
                    </button>
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