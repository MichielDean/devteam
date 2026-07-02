import { useState } from 'react';
import { useNavigate } from 'react-router';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { listFeatures, createFeature, ApiError } from '../api/client';
import { useToast } from '../components/Toast';
import type { CreateFeatureRequest, FeatureSummary } from '../types';
import IntakeForm from '../components/IntakeForm';
import FeatureCard from '../components/FeatureCard';

export default function Dashboard() {
  const [showIntakeForm, setShowIntakeForm] = useState(false);
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
    onSuccess: (feature, variables) => {
      queryClient.invalidateQueries({ queryKey: ['features'] });
      setShowIntakeForm(false);
      if (variables.startImmediately) {
        addToast('success', 'Feature created — inception starting');
      } else {
        addToast('success', 'Feature created');
      }
      navigate(`/features/${feature.id}`);
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
          <h2 className="text-xl font-medium text-[var(--color-text-primary)]">Features</h2>
          {!isLoading && !error && (
            <span
              data-testid="feature-count-badge"
              aria-label={`Total features: ${totalCount}`}
              className="inline-flex items-center justify-center min-w-[1.75rem] h-5 px-1.5 rounded-[var(--radius-md)] text-xs font-medium"
              style={{ backgroundColor: 'var(--color-surface-active)', color: 'var(--color-text-secondary)' }}
            >
              {totalCount}
            </span>
          )}
        </div>
        <button
          onClick={() => setShowIntakeForm(true)}
          className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-[var(--radius-md)] text-sm font-medium text-white bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] transition-colors"
          style={{ boxShadow: 'var(--shadow-sm)' }}
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

      {isLoading && (
        <div className="flex items-center justify-center py-20" data-testid="features-loading">
          <div className="animate-spin rounded-full h-6 w-6 border-2 border-t-transparent" style={{ borderColor: 'var(--color-accent)', borderTopColor: 'transparent' }} />
          <span className="ml-3 text-[var(--color-text-tertiary)] text-sm">Loading features...</span>
        </div>
      )}

      {error && (
        <div className="py-4 text-sm" style={{ color: 'var(--color-danger)' }} data-testid="features-error">
          Failed to load features: {error.message}
        </div>
      )}

      {!isLoading && !error && features.length === 0 && !showIntakeForm && (
        <div className="flex flex-col items-center justify-center py-20 text-center" data-testid="empty-state">
          <div className="w-12 h-12 rounded-[var(--radius-lg)] flex items-center justify-center mb-4 text-[var(--color-text-tertiary)]" style={{ backgroundColor: 'var(--color-surface-raised)' }}>
            <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
            </svg>
          </div>
          <h3 className="text-base font-medium text-[var(--color-text-primary)] mb-1">No features yet</h3>
          <p className="text-sm text-[var(--color-text-tertiary)] mb-4">Create your first feature to get started.</p>
          <button
            onClick={() => setShowIntakeForm(true)}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-[var(--radius-md)] text-sm font-medium text-white bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] transition-colors"
            style={{ boxShadow: 'var(--shadow-sm)' }}
            data-testid="empty-create-button"
          >
            + New Feature
          </button>
        </div>
      )}

      {!isLoading && !error && features.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4" data-testid="feature-grid">
          {features.map((f) => (
            <FeatureCard key={f.id} feature={f} />
          ))}
        </div>
      )}
    </div>
  );
}