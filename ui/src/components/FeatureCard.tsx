import { Link } from 'react-router';
import type { FeatureSummary } from '../types';
import { STATUS_LABELS, SCOPE_LABELS } from '../types';
import { Badge } from '../ui/primitives';
import type { Color } from '../ui/primitives/Badge';
import QuestionBadge from './QuestionBadge';

interface FeatureCardProps {
  feature: FeatureSummary;
}

const TOTAL_STAGES = 32;

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

function stageToProgress(currentStage?: string): number {
  if (!currentStage) return 0;
  const [major, minor] = currentStage.split('.');
  const majorNum = Number(major) || 0;
  const minorNum = Number(minor) || 0;
  const idx = majorNum * 8 + minorNum;
  return Math.min(100, (idx / TOTAL_STAGES) * 100);
}

export default function FeatureCard({ feature }: FeatureCardProps) {
  const statusLabel = STATUS_LABELS[feature.status] || feature.status;
  const scopeLabel = SCOPE_LABELS[feature.scope || 'feature'] || 'Feature';
  const progress = stageToProgress(feature.current_stage);

  return (
    <Link
      to={`/features/${feature.id}`}
      className="block bg-[var(--color-surface-raised)] rounded-[var(--radius-lg)] p-4 relative transition-all hover:bg-[var(--color-surface-hover)]"
      style={{ boxShadow: 'var(--shadow-sm)' }}
      data-testid={`feature-card-${feature.id}`}
      onMouseEnter={(e) => { e.currentTarget.style.boxShadow = 'var(--shadow-md)'; }}
      onMouseLeave={(e) => { e.currentTarget.style.boxShadow = 'var(--shadow-sm)'; }}
    >
      {feature.pending_questions_count > 0 && (
        <QuestionBadge featureId={feature.id} count={feature.pending_questions_count} />
      )}

      <h3 className="text-sm font-medium text-[var(--color-text-primary)] truncate mb-2" data-testid="feature-card-title">
        {feature.title}
      </h3>

      <div className="flex items-center gap-1.5 flex-wrap mb-3">
        <Badge color={statusColor[feature.status] || 'gray'} data-testid="feature-card-status">
          {statusLabel}
        </Badge>
        <Badge color="gray" data-testid="feature-card-scope">
          {scopeLabel}
        </Badge>
        {feature.current_stage && (
          <Badge color="blue" data-testid="feature-card-stage">
            {feature.current_stage}
          </Badge>
        )}
      </div>

      {/* Progress bar */}
      <div className="mb-3">
        <div className="h-1 rounded-full overflow-hidden" style={{ backgroundColor: 'var(--color-border-subtle)' }}>
          <div
            className="h-full transition-all"
            style={{ width: `${progress}%`, backgroundColor: 'var(--color-accent)' }}
            data-testid="feature-card-progress"
          />
        </div>
      </div>

      <div className="text-xs text-[var(--color-text-tertiary)]" data-testid="feature-card-updated">
        Updated {new Date(feature.updated_at).toLocaleDateString()}
      </div>
    </Link>
  );
}