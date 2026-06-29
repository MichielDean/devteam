import { Link } from 'react-router';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { FeatureSummary } from '../types';
import { STATUS_LABELS, PHASE_LABELS, PRIORITY_LABELS } from '../types';
import type { PhaseName } from '../types';
import QuestionBadge from './QuestionBadge';
import { deleteFeature, ApiError } from '../api/client';
import { useToast } from './Toast';

interface FeatureCardProps {
  feature: FeatureSummary;
}

// US-003 / AC-025: deletable from the dashboard card. Mirrors the detail-page
// status guard — only draft / terminal features show the affordance.
const NON_DELETABLE_STATUSES = ['in_progress', 'waiting_for_feedback', 'gate_blocked'];

export default function FeatureCard({ feature }: FeatureCardProps) {
  const queryClient = useQueryClient();
  const { addToast } = useToast();

  const deleteMutation = useMutation({
    mutationFn: () => deleteFeature(feature.id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['features'] });
      addToast('success', 'Feature deleted');
    },
    onError: (err: Error) => {
      const apiErr = err instanceof ApiError ? err : null;
      addToast('error', apiErr?.details || `Failed to delete: ${err.message}`);
    },
  });

  const phaseLabel = PHASE_LABELS[feature.current_phase as PhaseName] || feature.current_phase;
  const statusLabel = STATUS_LABELS[feature.status] || feature.status;
  const priorityLabel = PRIORITY_LABELS[feature.priority] || `P${feature.priority}`;
  const deletable = !NON_DELETABLE_STATUSES.includes(feature.status);

  const statusColors: Record<string, string> = {
    in_progress: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
    done: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
    cancelled: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
    draft: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200',
    gate_blocked: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
    passed: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
    failed: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
    recirculated: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200',
    waiting_for_human: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
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
        <span
          className={`px-2 py-0.5 rounded-full text-xs font-medium ${statusColors[feature.status] || 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'}`}
          data-testid="feature-card-status"
        >
          {statusLabel}
        </span>
        <span
          className="px-2 py-0.5 rounded-full text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200"
          data-testid="feature-card-phase"
        >
          {phaseLabel}
        </span>
        <span
          className="px-2 py-0.5 rounded-full text-xs font-medium bg-indigo-100 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200"
          data-testid="feature-card-priority"
        >
          {priorityLabel}
        </span>
      </div>

      {feature.gate_result && (
        <div className="mt-2 flex items-center gap-1" data-testid="feature-card-gate">
          {feature.gate_result.passed ? (
            <span className="text-xs text-green-600 dark:text-green-400">✓ Gate passed</span>
          ) : (
            <span className="text-xs text-red-600 dark:text-red-400">✗ Gate failed</span>
          )}
        </div>
      )}

      <div className="mt-2 text-xs text-gray-500 dark:text-gray-400" data-testid="feature-card-updated">
        Updated {new Date(feature.updated_at).toLocaleDateString()}
      </div>

      {/* US-003 / AC-025: delete affordance on deletable cards. */}
      {deletable && (
        <button
          type="button"
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            if (window.confirm(`Delete "${feature.title}"? This removes the feature and all related data. This cannot be undone.`)) {
              deleteMutation.mutate();
            }
          }}
          disabled={deleteMutation.isPending}
          title="Delete this feature"
          className="absolute top-2 right-2 px-2 py-0.5 text-xs rounded bg-red-100 text-red-700 hover:bg-red-200 dark:bg-red-900 dark:text-red-200 dark:hover:bg-red-800 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          data-testid="feature-card-delete"
        >
          {deleteMutation.isPending ? '...' : 'Delete'}
        </button>
      )}
    </Link>
  );
}