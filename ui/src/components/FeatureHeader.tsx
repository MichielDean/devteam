import { Badge } from '../ui/primitives';
import type { Color } from '../ui/primitives/Badge';
import { STATUS_LABELS, PRIORITY_LABELS, SCOPE_LABELS, DEPTH_LABELS } from '../types';
import type { FeatureDetail } from '../types';
import SessionIndicator from './SessionIndicator';

interface FeatureHeaderProps {
  feature: FeatureDetail;
  sessionsCount?: number;
  isTerminal: boolean;
}

const statusColor: Record<string, Color> = {
  in_progress: 'blue',
  done: 'green',
  cancelled: 'red',
  draft: 'gray',
  gate_blocked: 'yellow',
  passed: 'green',
  failed: 'red',
  waiting_for_feedback: 'yellow',
};

export default function FeatureHeader({ feature, sessionsCount = 0, isTerminal }: FeatureHeaderProps) {
  return (
    <div className="rounded-[var(--radius-lg)] p-4 mb-4" style={{ backgroundColor: 'var(--color-surface-raised)', boxShadow: 'var(--shadow-sm)' }} data-testid="feature-header">
      <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-lg sm:text-xl font-medium text-[var(--color-text-primary)] truncate" data-testid="feature-title">{feature.title}</h1>
          <p className="text-xs text-[var(--color-text-tertiary)] mt-1" data-testid="feature-id">{feature.id}</p>
        </div>
        <div className="flex items-center gap-1.5 flex-wrap shrink-0">
          <Badge color={isTerminal ? (feature.status === 'done' ? 'green' : 'red') : (statusColor[feature.status] || 'gray')} data-testid="feature-status">
            {STATUS_LABELS[feature.status] || feature.status}
          </Badge>
          <Badge color="gray" data-testid="feature-priority">{PRIORITY_LABELS[feature.priority] || `P${feature.priority}`}</Badge>
          {feature.scope && <Badge color="blue" data-testid="feature-scope-badge">{SCOPE_LABELS[feature.scope] || feature.scope}</Badge>}
          {feature.depth && <Badge color="gray" data-testid="feature-depth-badge">{DEPTH_LABELS[feature.depth] || feature.depth}</Badge>}
          {sessionsCount > 0 && <SessionIndicator count={sessionsCount} featureId={feature.id} />}
        </div>
      </div>
      <div className="mt-4 grid grid-cols-2 sm:grid-cols-4 gap-4">
        <div>
          <span className="text-[var(--color-text-tertiary)] text-xs uppercase tracking-wide">Current Stage</span>
          <p className="text-sm font-medium text-[var(--color-text-primary)] mt-0.5" data-testid="feature-current-stage">{feature.current_stage || '—'}</p>
        </div>
        <div>
          <span className="text-[var(--color-text-tertiary)] text-xs uppercase tracking-wide">Phase</span>
          <p className="text-sm font-medium text-[var(--color-text-primary)] mt-0.5" data-testid="feature-phase">{feature.current_phase || '—'}</p>
        </div>
        <div>
          <span className="text-[var(--color-text-tertiary)] text-xs uppercase tracking-wide">Intake</span>
          <p className="text-sm font-medium text-[var(--color-text-primary)] mt-0.5">{feature.intake_path === 'loose_idea' ? 'Loose Idea' : 'External Spec'}</p>
        </div>
        <div>
          <span className="text-[var(--color-text-tertiary)] text-xs uppercase tracking-wide">Processing</span>
          <p className="text-sm font-medium text-[var(--color-text-primary)] mt-0.5" data-testid="feature-processing">{feature.is_processing ? 'Yes' : 'No'}</p>
        </div>
      </div>
    </div>
  );
}