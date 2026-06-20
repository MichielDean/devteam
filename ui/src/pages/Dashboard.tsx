import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { listFeatures, createFeature, ApiError } from '../api/client';
import { useToast } from '../components/Toast';
import type { CreateFeatureRequest, FeatureSummary } from '../types';
import FeatureList from '../components/FeatureList';
import IntakeForm from '../components/IntakeForm';
import EmptyState from '../components/EmptyState';

export default function Dashboard() {
  const [showIntakeForm, setShowIntakeForm] = useState(false);
  const queryClient = useQueryClient();
  const { addToast } = useToast();

  const { data, isLoading, error } = useQuery({
    queryKey: ['features'],
    queryFn: listFeatures,
  });

  const createMutation = useMutation({
    mutationFn: (req: CreateFeatureRequest) => createFeature(req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['features'] });
      setShowIntakeForm(false);
      addToast('success', 'Feature created successfully');
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

  return (
    <div data-testid="dashboard-page">
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Features</h2>
        <button
          onClick={() => setShowIntakeForm(true)}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors text-sm font-medium"
          data-testid="create-feature-button"
        >
          + New Feature
        </button>
      </div>

      {showIntakeForm && (
        <IntakeForm
          onSubmit={(req) => createMutation.mutate(req)}
          onCancel={() => setShowIntakeForm(false)}
          isLoading={createMutation.isPending}
        />
      )}

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
    </div>
  );
}