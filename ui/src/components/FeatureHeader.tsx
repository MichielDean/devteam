import { Badge } from '../ui/primitives';
import { STATUS_LABELS, PRIORITY_LABELS, SCOPE_LABELS, DEPTH_LABELS } from '../types';
import type { FeatureDetail } from '../types';
import SessionIndicator from './SessionIndicator';

interface FeatureHeaderProps {
  feature: FeatureDetail;
  sessionsCount?: number;
  isTerminal: boolean;
}

export default function FeatureHeader({ feature, sessionsCount = 0, isTerminal }: FeatureHeaderProps) {
  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4 mb-4" data-testid="feature-header">
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white" data-testid="feature-title">{feature.title}</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5" data-testid="feature-id">{feature.id}</p>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <Badge color={isTerminal ? (feature.status === 'done' ? 'green' : 'red') : 'blue'} data-testid="feature-status">
            {STATUS_LABELS[feature.status] || feature.status}
          </Badge>
          <Badge color="purple" data-testid="feature-priority">{PRIORITY_LABELS[feature.priority] || `P${feature.priority}`}</Badge>
          {feature.scope && <Badge color="indigo" data-testid="feature-scope-badge">{SCOPE_LABELS[feature.scope] || feature.scope}</Badge>}
          {feature.depth && <Badge color="gray" data-testid="feature-depth-badge">{DEPTH_LABELS[feature.depth] || feature.depth}</Badge>}
          {sessionsCount > 0 && <SessionIndicator count={sessionsCount} featureId={feature.id} />}
        </div>
      </div>
      <div className="mt-3 grid grid-cols-2 sm:grid-cols-4 gap-3 text-sm">
        <div>
          <span className="text-gray-500 dark:text-gray-400 text-xs">Current Stage</span>
          <p className="font-medium text-gray-900 dark:text-white" data-testid="feature-current-stage">{feature.current_stage || '—'}</p>
        </div>
        <div>
          <span className="text-gray-500 dark:text-gray-400 text-xs">Phase</span>
          <p className="font-medium text-gray-900 dark:text-white" data-testid="feature-phase">{feature.current_phase || '—'}</p>
        </div>
        <div>
          <span className="text-gray-500 dark:text-gray-400 text-xs">Intake</span>
          <p className="font-medium text-gray-900 dark:text-white">{feature.intake_path === 'loose_idea' ? 'Loose Idea' : 'External Spec'}</p>
        </div>
        <div>
          <span className="text-gray-500 dark:text-gray-400 text-xs">Processing</span>
          <p className="font-medium text-gray-900 dark:text-white" data-testid="feature-processing">{feature.is_processing ? 'Yes' : 'No'}</p>
        </div>
      </div>
    </div>
  );
}