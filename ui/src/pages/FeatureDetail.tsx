import { useParams, Link } from 'react-router';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getFeature, runPhase, advanceFeature, recirculateFeature, cancelFeature, processFeature, evaluateGate } from '../api/client';
import { useSSE } from '../hooks/useSSE';
import { useToast } from '../components/Toast';
import type { FeatureDetail, PhaseName } from '../types';
import { PHASES, PHASE_LABELS, STATUS_LABELS, PRIORITY_LABELS } from '../types';
import PhaseTimeline from '../components/PhaseTimeline';
import ArtifactViewer from '../components/ArtifactViewer';
import GateResult from '../components/GateResult';
import ProcessView from '../components/ProcessView';

export default function FeatureDetail() {
  const { id } = useParams<{ id: string }>();
  const queryClient = useQueryClient();
  const { addToast } = useToast();

  const { data: feature, isLoading, error } = useQuery({
    queryKey: ['feature', id!],
    queryFn: () => getFeature(id!),
    enabled: !!id,
  });

  // SSE connection for real-time updates
  const { connected: sseConnected } = useSSE(id ?? null);
  void sseConnected; // Used for connection status banner

  // Mutations for pipeline actions
  const runPhaseMutation = useMutation({
    mutationFn: () => runPhase(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      addToast('success', 'Phase execution started');
    },
    onError: (err: Error) => addToast('error', `Failed to run phase: ${err.message}`),
  });

  const advanceMutation = useMutation({
    mutationFn: () => advanceFeature(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      addToast('success', 'Feature advanced to next phase');
    },
    onError: (err: Error) => {
      if (err instanceof Error) addToast('error', `Failed to advance: ${err.message}`);
    },
  });

  const recirculateMutation = useMutation({
    mutationFn: (targetPhase: string) => recirculateFeature(id!, targetPhase),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      addToast('success', 'Feature recirculated');
    },
    onError: (err: Error) => addToast('error', `Failed to recirculate: ${err.message}`),
  });

  const cancelMutation = useMutation({
    mutationFn: () => cancelFeature(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      addToast('success', 'Feature cancelled');
    },
    onError: (err: Error) => addToast('error', `Failed to cancel: ${err.message}`),
  });

  const processMutation = useMutation({
    mutationFn: () => processFeature(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      addToast('success', 'Processing started');
    },
    onError: (err: Error) => {
      if (err.message.includes('already')) {
        addToast('error', 'Feature is already being processed');
      } else {
        addToast('error', `Failed to start processing: ${err.message}`);
      }
    },
  });

  const gateMutation = useMutation({
    mutationFn: () => evaluateGate(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      addToast('success', 'Gate evaluated');
    },
    onError: (err: Error) => addToast('error', `Failed to evaluate gate: ${err.message}`),
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12" data-testid="feature-loading">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
        <span className="ml-3 text-gray-500 dark:text-gray-400">Loading feature...</span>
      </div>
    );
  }

  if (error || !feature) {
    return (
      <div className="text-center py-12" data-testid="feature-not-found">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-2">Feature not found</h2>
        <p className="text-gray-500 dark:text-gray-400 mb-4">
          The feature you're looking for doesn't exist.
        </p>
        <Link to="/" className="text-blue-600 dark:text-blue-400 hover:underline">
          ← Back to Dashboard
        </Link>
      </div>
    );
  }

  const isTerminal = feature.status === 'done' || feature.status === 'cancelled';
  const currentPhase = feature.current_phase as PhaseName;
  const currentPhaseState = feature.phase_states[currentPhase];
  const gatePassed = currentPhaseState?.gate_result?.passed ?? false;

  // Determine available recirculation targets (phases earlier than current)
  const currentPhaseIndex = PHASES.indexOf(currentPhase);
  const recirculationTargets = PHASES.slice(0, currentPhaseIndex > 0 ? currentPhaseIndex : 0);

  // Determine if at delivery with passed gate
  const isDeliveryPassed = currentPhase === 'delivery' && gatePassed;

  return (
    <div data-testid="feature-detail-page">
      <div className="mb-6">
        <Link to="/" className="text-blue-600 dark:text-blue-400 hover:underline text-sm">
          ← Back to Dashboard
        </Link>
      </div>

      {/* Feature Header */}
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white" data-testid="feature-title">
              {feature.title}
            </h1>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1" data-testid="feature-id">
              {feature.id}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <span
              className={`px-3 py-1 rounded-full text-sm font-medium ${
                feature.status === 'done'
                  ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                  : feature.status === 'cancelled'
                  ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                  : feature.status === 'in_progress'
                  ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
                  : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'
              }`}
              data-testid="feature-status"
            >
              {STATUS_LABELS[feature.status] || feature.status}
            </span>
            <span
              className="px-3 py-1 rounded-full text-sm font-medium bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200"
              data-testid="feature-priority"
            >
              {PRIORITY_LABELS[feature.priority] || `P${feature.priority}`}
            </span>
          </div>
        </div>

        <div className="mt-4 grid grid-cols-2 sm:grid-cols-4 gap-4 text-sm">
          <div>
            <span className="text-gray-500 dark:text-gray-400">Phase</span>
            <p className="font-medium text-gray-900 dark:text-white" data-testid="feature-phase">
              {PHASE_LABELS[currentPhase as PhaseName] || currentPhase}
            </p>
          </div>
          <div>
            <span className="text-gray-500 dark:text-gray-400">Intake</span>
            <p className="font-medium text-gray-900 dark:text-white">
              {feature.intake_path === 'loose_idea' ? 'Loose Idea' : 'External Spec'}
            </p>
          </div>
          <div>
            <span className="text-gray-500 dark:text-gray-400">Created</span>
            <p className="font-medium text-gray-900 dark:text-white">
              {new Date(feature.created_at).toLocaleDateString()}
            </p>
          </div>
          <div>
            <span className="text-gray-500 dark:text-gray-400">Updated</span>
            <p className="font-medium text-gray-900 dark:text-white">
              {new Date(feature.updated_at).toLocaleDateString()}
            </p>
          </div>
        </div>
      </div>

      {/* Phase Timeline */}
      <PhaseTimeline phases={PHASES} currentPhase={currentPhase} phaseStates={feature.phase_states} />

      {/* Process View (shown during processing) */}
      {(feature.status === 'in_progress' && processMutation.isPending) && (
        <ProcessView featureId={feature.id} />
      )}

      {/* Action Buttons */}
      {!isTerminal && (
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Actions</h3>
          <div className="flex flex-wrap gap-3">
            <button
              onClick={() => runPhaseMutation.mutate()}
              disabled={runPhaseMutation.isPending}
              className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
              data-testid="run-phase-button"
            >
              Run Phase
            </button>

            <button
              onClick={() => gateMutation.mutate()}
              disabled={gateMutation.isPending}
              className="px-4 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
              data-testid="evaluate-gate-button"
            >
              Evaluate Gate
            </button>

            {!isDeliveryPassed && (
              <button
                onClick={() => advanceMutation.mutate()}
                disabled={!gatePassed || advanceMutation.isPending}
                className="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
                title={!gatePassed ? 'Gate has not passed' : 'Advance to next phase'}
                data-testid="advance-button"
              >
                Advance
              </button>
            )}

            {isDeliveryPassed && (
              <span className="px-4 py-2 bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200 rounded-lg text-sm font-medium" data-testid="mark-done-indicator">
                ✓ Ready to Mark Done
              </span>
            )}

            {recirculationTargets.length > 0 && (
              <select
                className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"
                defaultValue=""
                onChange={(e) => {
                  if (e.target.value && window.confirm(`Recirculate to ${PHASE_LABELS[e.target.value as PhaseName]}?`)) {
                    recirculateMutation.mutate(e.target.value);
                  }
                }}
                data-testid="recirculate-select"
              >
                <option value="">Recirculate to...</option>
                {recirculationTargets.map((phase) => (
                  <option key={phase} value={phase}>
                    {PHASE_LABELS[phase]}
                  </option>
                ))}
              </select>
            )}

            <button
              onClick={() => {
                if (window.confirm('Are you sure you want to cancel this feature?')) {
                  cancelMutation.mutate();
                }
              }}
              disabled={cancelMutation.isPending}
              className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
              data-testid="cancel-button"
            >
              Cancel
            </button>

            <button
              onClick={() => processMutation.mutate()}
              disabled={processMutation.isPending}
              className="px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
              title={feature.status === 'in_progress' ? 'Feature is already being processed' : 'Process entire pipeline'}
              data-testid="process-button"
            >
              Process
            </button>
          </div>
        </div>
      )}

      {/* Gate Results */}
      {currentPhaseState?.gate_result && (
        <GateResult gateResult={currentPhaseState.gate_result} />
      )}

      {/* Artifacts */}
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Artifacts</h3>
        <ArtifactViewer featureId={feature.id} phaseStates={feature.phase_states} />
      </div>
    </div>
  );
}