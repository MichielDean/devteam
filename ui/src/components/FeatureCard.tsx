import { Link } from 'react-router';
import type { FeatureSummary } from '../types';
import { STATUS_LABELS, SCOPE_LABELS, PRIORITY_LABELS } from '../types';
import QuestionBadge from './QuestionBadge';

interface FeatureCardProps {
  feature: FeatureSummary;
}

export default function FeatureCard({ feature }: FeatureCardProps) {
  const statusLabel = STATUS_LABELS[feature.status] || feature.status;
  const scopeLabel = SCOPE_LABELS[feature.scope || 'feature'] || 'Feature';
  const priorityLabel = PRIORITY_LABELS[feature.priority] || `P${feature.priority}`;

  const statusColors: Record<string, string> = {
    in_progress: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
    done: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
    cancelled: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
    draft: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200',
    gate_blocked: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
    passed: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
    failed: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
    waiting_for_feedback: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
  };

  return (
    <Link
      to={`/features/${feature.id}`}
      className="block bg-white dark:bg-gray-800 rounded-lg shadow hover:shadow-md transition-shadow border border-gray-200 dark:border-gray-700 p-4 relative"
      data-testid={`feature-card-${feature.id}`}
    >
      {feature.pending_questions_count > 0 && (
        <QuestionBadge featureId={feature.id} count={feature.pending_questions_count} />
      )}
      <div className="flex items-start justify-between mb-2">
        <h3 className="text-sm font-semibold text-gray-900 dark:text-white truncate" data-testid="feature-card-title">
          {feature.title}
        </h3>
        <span className="text-xs text-gray-500 dark:text-gray-400 ml-2 shrink-0" data-testid="feature-card-id">
          {feature.id.slice(0, 20)}{feature.id.length > 20 ? '...' : ''}
        </span>
      </div>
      <div className="flex items-center gap-2 flex-wrap">
        <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${statusColors[feature.status] || 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'}`} data-testid="feature-card-status">
          {statusLabel}
        </span>
        <span className="px-2 py-0.5 rounded-full text-xs font-medium bg-teal-100 text-teal-800 dark:bg-teal-900 dark:text-teal-200" data-testid="feature-card-scope">
          {scopeLabel}
        </span>
        {feature.current_stage && (
          <span className="px-2 py-0.5 rounded-full text-xs font-medium bg-indigo-100 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200" data-testid="feature-card-stage">
            Stage {feature.current_stage}
          </span>
        )}
        <span className="px-2 py-0.5 rounded-full text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200" data-testid="feature-card-priority">
          {priorityLabel}
        </span>
      </div>
      <div className="mt-2 text-xs text-gray-500 dark:text-gray-400" data-testid="feature-card-updated">
        Updated {new Date(feature.updated_at).toLocaleDateString()}
      </div>
    </Link>
  );
}