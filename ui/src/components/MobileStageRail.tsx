import { useRef, useEffect } from 'react';
import { useUIStore } from '../store/ui-store';
import type { FeatureStage } from '../types';

interface MobileStageRailProps {
  stages: FeatureStage[];
  currentStageId?: string;
  onSelect: (stageId: string) => void;
}

const STATUS_ICONS: Record<string, string> = {
  not_started: '○',
  in_progress: '▶',
  awaiting_approval: '?',
  revising: 'R',
  completed: '✓',
  skipped: 'S',
};

const STATUS_COLORS: Record<string, string> = {
  not_started: 'var(--color-text-tertiary)',
  in_progress: 'var(--color-accent)',
  awaiting_approval: 'var(--color-warning)',
  revising: 'var(--color-warning)',
  completed: 'var(--color-success)',
  skipped: 'var(--color-text-tertiary)',
};

export default function MobileStageRail({ stages, currentStageId, onSelect }: MobileStageRailProps) {
  const { selectedStageId, setSelectedStage } = useUIStore();
  const scrollRef = useRef<HTMLDivElement>(null);

  // Scroll current stage into view on mount
  useEffect(() => {
    if (scrollRef.current && currentStageId) {
      const el = scrollRef.current.querySelector(`[data-stage="${currentStageId}"]`);
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', inline: 'center', block: 'nearest' });
      }
    }
  }, [currentStageId]);

  const completed = stages.filter((s) => s.status === 'completed').length;

  return (
    <div className="rounded-[var(--radius-lg)] p-3" style={{ backgroundColor: 'var(--color-surface-raised)' }} data-testid="mobile-stage-rail">
      <div className="flex items-center justify-between mb-2">
        <span className="text-xs font-medium text-[var(--color-text-secondary)]">{completed}/{stages.length} stages</span>
        <div className="h-1 flex-1 mx-3 rounded-full" style={{ backgroundColor: 'var(--color-border-subtle)' }}>
          <div className="h-full rounded-full transition-all" style={{ width: `${stages.length > 0 ? (completed / stages.length) * 100 : 0}%`, backgroundColor: 'var(--color-accent)' }} />
        </div>
      </div>
      <div ref={scrollRef} className="flex gap-1.5 overflow-x-auto pb-1" style={{ scrollbarWidth: 'thin' }}>
        {stages.map((s) => {
          const isCurrent = s.stage_id === currentStageId;
          const isSelected = s.stage_id === selectedStageId;
          const icon = STATUS_ICONS[s.status] || '○';
          const color = STATUS_COLORS[s.status] || 'var(--color-text-tertiary)';
          const key = `${s.stage_id}-bolt${s.bolt_number ?? 0}`;
          return (
            <button
              key={key}
              data-stage={key}
              onClick={() => { setSelectedStage(s.stage_id); onSelect(s.stage_id); }}
              className={`flex items-center gap-1 px-2.5 py-1.5 rounded-[var(--radius-md)] text-xs font-medium whitespace-nowrap shrink-0 transition-colors ${
                isSelected
                  ? 'bg-[var(--color-accent)] text-white'
                  : isCurrent
                  ? 'text-white'
                  : 'text-[var(--color-text-secondary)]'
              }`}
              style={!isSelected && !isCurrent ? { backgroundColor: 'var(--color-surface-hover)' } : isCurrent && !isSelected ? { backgroundColor: 'var(--color-surface-active)', border: '1px solid var(--color-accent)' } : undefined}
              data-testid={`mobile-rail-${key}`}
            >
              <span style={{ color: isSelected ? 'white' : color }}>{icon}</span>
              {s.stage_id}
              {s.bolt_number > 0 && <span className="text-[10px] opacity-60">·B{s.bolt_number}</span>}
              {s.name && <span className="text-[10px] opacity-70 hidden sm:inline">· {s.name.split(' ')[0]}</span>}
            </button>
          );
        })}
      </div>
    </div>
  );
}