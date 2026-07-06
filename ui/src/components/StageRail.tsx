import { useUIStore } from '../store/ui-store';
import { Badge } from '../ui/primitives';
import { PHASE_LABELS, AGENT_LABELS } from '../types';
import type { FeatureStage, Bolt } from '../types';

interface StageRailProps {
  stages: FeatureStage[];
  currentStageId?: string;
  bolts?: Bolt[];
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

function groupByPhase(stages: FeatureStage[], bolts: Bolt[]): Record<string, (FeatureStage | Bolt)[]> {
  const groups: Record<string, (FeatureStage | Bolt)[]> = {};
  for (const s of stages) {
    const phaseNum = s.stage_id.split('.')[0];
    const phaseName = phaseNum === '0' ? 'initialization' : phaseNum === '1' ? 'ideation' : phaseNum === '2' ? 'inception' : phaseNum === '3' ? 'construction' : phaseNum === '4' ? 'operation' : 'unknown';
    if (!groups[phaseName]) groups[phaseName] = [];

    // During construction: group 3.1-3.5 under bolts, show 3.6-3.7 individually
    if (phaseName === 'construction' && bolts.length > 0) {
      const isBoltStage = s.stage_id.match(/^3\.[1-5]$/);
      if (isBoltStage) {
        continue;
      }
      groups[phaseName].push(s);
    } else {
      groups[phaseName].push(s);
    }
  }

  // Add bolts to construction group
  if (bolts.length > 0 && groups['construction']) {
    // Insert bolts before 3.6/3.7
    const nonBoltStages = groups['construction'];
    groups['construction'] = [...bolts, ...nonBoltStages];
  }

  return groups;
}

function isBolt(item: FeatureStage | Bolt): item is Bolt {
  return 'bolt_number' in item;
}

export default function StageRail({ stages, currentStageId, bolts = [] }: StageRailProps) {
  const { selectedStageId, setSelectedStage } = useUIStore();
  const grouped = groupByPhase(stages, bolts);
  const completed = stages.filter((s) => s.status === 'completed').length;
  const total = stages.length;

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
          {Object.entries(grouped).map(([phase, phaseItems]) => (
            <div key={phase} data-testid={`rail-phase-${phase}`}>
              <h4 className="text-[10px] font-medium text-[var(--color-text-tertiary)] uppercase tracking-wider mb-1 px-1">
                {PHASE_LABELS[phase] || phase}
              </h4>
              <div className="space-y-0.5">
                {phaseItems.map((item) => {
                  if (isBolt(item)) {
                    // Render bolt with its sub-stages (per-bolt rows only).
                    const boltStages = stages.filter(s => s.stage_id.match(/^3\.[1-5]$/) && (s.bolt_number ?? 0) === item.bolt_number);
                    const boltStatus = item.status;
                    const boltIcon = STATUS_ICONS[boltStatus] || '○';
                    const boltColor = statusColor[boltStatus] || 'var(--color-text-tertiary)';
                    const isBoltCurrent = boltStages.some(s => s.stage_id === currentStageId);
                    const isBoltSelected = boltStages.some(s => s.stage_id === selectedStageId);
                    const completedSubSteps = boltStages.filter(s => s.status === 'completed').length;

                    return (
                      <div key={`bolt-${item.bolt_number}`} data-testid={`rail-bolt-${item.bolt_number}`}>
                        <button
                          onClick={() => setSelectedStage(`bolt-${item.bolt_number}`)}
                          className={`w-full flex items-center gap-2 px-2 py-1.5 rounded-[var(--radius-sm)] text-left text-xs transition-colors ${
                            isBoltSelected ? 'bg-[var(--color-surface-active)]' : 'hover:bg-[var(--color-surface-hover)]'
                          }`}
                          style={isBoltSelected ? { borderLeft: `2px solid var(--color-accent)`, paddingLeft: '6px' } : { borderLeft: '2px solid transparent' }}
                          data-testid={`rail-stage-bolt-${item.bolt_number}`}
                        >
                          <span className="w-4 text-center shrink-0 font-mono text-xs" style={{ color: boltColor }}>{boltIcon}</span>
                          <div className="flex-1 min-w-0">
                            <div className={`truncate ${isBoltCurrent ? 'font-medium text-[var(--color-text-primary)]' : 'text-[var(--color-text-secondary)]'}`}>
                              Bolt {item.bolt_number}{item.is_walking_skeleton ? ' · Skeleton' : ''}
                            </div>
                            <div className="text-[10px] text-[var(--color-text-tertiary)]">{completedSubSteps}/5 stages · {item.unit_ids?.join(', ') || ''}</div>
                          </div>
                        </button>
                        {/* Sub-steps */}
                        <div className="ml-6 mt-0.5 space-y-0.5">
                          {boltStages.map(s => {
                            const icon = STATUS_ICONS[s.status] || '○';
                            const color = statusColor[s.status] || 'var(--color-text-tertiary)';
                            const key = `${s.stage_id}-bolt${s.bolt_number ?? 0}`;
                            return (
                              <button
                                key={key}
                                onClick={() => setSelectedStage(s.stage_id)}
                                className="w-full flex items-center gap-1.5 px-2 py-1 rounded-[var(--radius-sm)] text-left text-[11px] transition-colors hover:bg-[var(--color-surface-hover)]"
                                data-testid={`rail-stage-${key}`}
                              >
                                <span className="w-3 text-center shrink-0 font-mono" style={{ color }}>{icon}</span>
                                <span className="truncate text-[var(--color-text-tertiary)]">{s.stage_id} {s.name ? `· ${s.name}` : ''}</span>
                              </button>
                            );
                          })}
                        </div>
                      </div>
                    );
                  }

                  // Render normal stage
                  const s = item;
                  const isCurrent = s.stage_id === currentStageId;
                  const isSelected = s.stage_id === selectedStageId;
                  const icon = STATUS_ICONS[s.status] || '○';
                  const color = statusColor[s.status] || 'var(--color-text-tertiary)';
                  const stageName = s.name || '';
                  const stageDesc = s.description || '';
                  return (
                    <button
                      key={`${s.stage_id}-bolt${s.bolt_number ?? 0}`}
                      onClick={() => setSelectedStage(s.stage_id)}
                      title={stageDesc || undefined}
                      className={`w-full flex items-center gap-2 px-2 py-1.5 rounded-[var(--radius-sm)] text-left text-xs transition-colors ${
                        isSelected ? 'bg-[var(--color-surface-active)]' : 'hover:bg-[var(--color-surface-hover)]'
                      }`}
                      style={isSelected ? { borderLeft: `2px solid var(--color-accent)`, paddingLeft: '6px' } : { borderLeft: '2px solid transparent' }}
                      data-testid={`rail-stage-${s.stage_id}`}
                    >
                      <span className="w-4 text-center shrink-0 font-mono text-xs" style={{ color }} data-testid={`rail-icon-${s.stage_id}`}>{icon}</span>
                      <div className="flex-1 min-w-0">
                        <div className={`truncate ${isCurrent ? 'font-medium text-[var(--color-text-primary)]' : 'text-[var(--color-text-secondary)]'}`}>
                          {s.stage_id}{stageName ? ` · ${stageName}` : ''}
                        </div>
                        {s.lead_agent && (
                          <div className="text-[10px] text-[var(--color-text-tertiary)] truncate">{AGENT_LABELS[s.lead_agent] || s.lead_agent}</div>
                        )}
                      </div>
                      {s.revision_count > 0 && <Badge color="yellow" className="text-[10px] px-1 py-0">×{s.revision_count}</Badge>}
                      {s.reviewer && <span className="text-[10px]" style={{ color: 'var(--color-text-tertiary)' }} title={`Reviewer: ${AGENT_LABELS[s.reviewer]}`}>🔍</span>}
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