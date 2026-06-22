import { useState } from 'react';
import { useNavigate } from 'react-router';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { listFeatures, createFeature, ApiError } from '../api/client';
import { useToast } from '../components/Toast';
import type { CreateFeatureRequest, FeatureSummary } from '../types';
import FeatureList from '../components/FeatureList';
import IntakeForm from '../components/IntakeForm';
import EmptyState from '../components/EmptyState';
import ViewToggle from '../components/ViewToggle';
import KanbanBoard from '../components/KanbanBoard';

export default function Dashboard() {
  const [showIntakeForm, setShowIntakeForm] = useState(false);
  const [viewMode, setViewMode] = useState<'list' | 'board'>('list');
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const navigate = useNavigate();

  const { data, isLoading, error } = useQuery({
    queryKey: ['features'],
    queryFn: listFeatures,
  });

  const createMutation = useMutation({
    mutationFn: ({ req, startImmediately }: { req: CreateFeatureRequest; startImmediately: boolean }) => {
      req.start_immediately = startImmediately;
      return createFeature(req);
    },
    onSuccess: (data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['features'] });
      setShowIntakeForm(false);
      if (variables.startImmediately) {
        addToast('success', 'Feature created — inception starting');
      } else {
        addToast('success', 'Feature created');
      }
      navigate(`/features/${data.id}`);
    },
    onError: (err: Error) => {
      if (err instanceof ApiError && err.code === 'duplicate_title') {
        addToast('error', `A feature with a similar title already exists: ${err.details}`);
      } else {
        addToast('error', `Failed to create feature: ${err.message}`);
      }
    },
  });

  const features: FeatureSummary[] = data?.features ?? [];
  const totalCount = data?.total_count ?? 0;

  return (
    <div data-testid="dashboard-page">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Features</h2>
            {!isLoading && !error && (
              <span
                data-testid="feature-count-badge"
                aria-label={`Total features: ${totalCount}`}
                className="inline-flex items-center justify-center min-w-[2.5rem] h-6 px-2 rounded-full bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200 text-xs font-bold"
              >
                {totalCount}
              </span>
            )}
          </div>
          <ViewToggle value={viewMode} onChange={setViewMode} />
        </div>
        <button
          onClick={() => setShowIntakeForm(true)}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors text-sm font-semibold shadow-sm"
          data-testid="create-feature-button"
        >
          + New Feature
        </button>
      </div>

      {showIntakeForm && (
        <IntakeForm
          onSubmit={(req, startImmediately) => createMutation.mutate({ req, startImmediately })}
          onCancel={() => setShowIntakeForm(false)}
          isLoading={createMutation.isPending}
        />
      )}

      {viewMode === 'board' ? (
        <KanbanBoard />
      ) : (
        <>
          {isLoading && (
            <div className="flex items-center justify-center py-12" data-testid="features-loading">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
              <span className="ml-3 text-gray-500 dark:text-gray-400">Loading features...</span>
            </div>
          )}

          {error && (
            <div className="text-red-600 dark:text-red-400 py-4" data-testid="features-error">
              Failed to load features: {error.message}
            </div>
          )}

          {!isLoading && !error && features.length === 0 && (
            <EmptyState onCreateClick={() => setShowIntakeForm(true)} />
          )}

          {!isLoading && !error && features.length > 0 && (
            <FeatureList features={features} />
          )}
        </>
      )}
    </div>
  );
}