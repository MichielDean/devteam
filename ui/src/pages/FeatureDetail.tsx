import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getFeature, runPhase, advanceFeature, recirculateFeature, cancelFeature, processFeature, evaluateGate, listQuestions } from '../api/client';
import { useSSE } from '../hooks/useSSE';
import { useToast } from '../components/Toast';
import type { FeatureDetail, PhaseName } from '../types';
import { PHASES, PHASE_LABELS, PHASE_ACTIONS, PHASE_DESCRIPTIONS, PHASE_OUTPUTS, STATUS_LABELS, PRIORITY_LABELS } from '../types';
import PhaseTimeline from '../components/PhaseTimeline';
import ArtifactViewer from '../components/ArtifactViewer';
import GateResult from '../components/GateResult';
import ProcessView from '../components/ProcessView';
import AgentOutput from '../components/AgentOutput';
import QuestionCard from '../components/QuestionCard';

export default function FeatureDetail() {
  const { id } = useParams<{ id: string }>();
  const queryClient = useQueryClient();
  const { addToast } = useToast();

  const { data: feature, isLoading, error } = useQuery({
    queryKey: ['feature', id!],
    queryFn: () => getFeature(id!),
    enabled: !!id,
  });

  const { connected: sseConnected, lastEvent } = useSSE(id ?? null);
  void sseConnected;

  const { data: questions = [] } = useQuery({
    queryKey: ['questions', id!],
    queryFn: () => listQuestions(id!),
    enabled: !!id,
  });

  const [isProcessing, setIsProcessing] = useState(feature?.is_processing ?? false);
  const [processingMode, setProcessingMode] = useState<'autopilot' | 'single-phase' | null>(
    (feature?.processing_mode as 'autopilot' | 'single-phase' | null) ?? null
  );

  // Sync isProcessing and processingMode from server response (handles page refresh)
  useEffect(() => {
    if (feature) {
      setIsProcessing(feature.is_processing);
      if (feature.processing_mode === 'autopilot' || feature.processing_mode === 'single-phase') {
        setProcessingMode(feature.processing_mode);
      } else if (!feature.is_processing) {
        setProcessingMode(null);
      }
    }
  }, [feature?.is_processing, feature?.processing_mode]);

  useEffect(() => {
    if (!lastEvent) return;
    if (lastEvent.type === 'processing_complete' || lastEvent.type === 'error' || lastEvent.type === 'phase_complete') {
      setIsProcessing(false);
      setProcessingMode(null);
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['features'] });
    } else if (lastEvent.type === 'agent_dispatch' || lastEvent.type === 'phase_change' || lastEvent.type === 'gate_result') {
      setIsProcessing(true);
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
    }
  }, [lastEvent, id, queryClient]);

  const runPhaseMutation = useMutation({
    mutationFn: () => runPhase(id!),
    onSuccess: () => {
      setIsProcessing(true);
      setProcessingMode('single-phase');
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      addToast('success', `${PHASE_LABELS[currentPhase as PhaseName] || 'Step'} started — watch the progress below`);
    },
    onError: (err: Error) => {
      setIsProcessing(false);
      setProcessingMode(null);
      if (err.message.includes('already')) {
        addToast('error', 'This feature is already being worked on');
      } else {
        addToast('error', `Failed to start: ${err.message}`);
      }
    },
  });

  const advanceMutation = useMutation({
    mutationFn: () => advanceFeature(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      addToast('success', `Moved to ${nextPhaseLabel || 'next step'}`);
    },
    onError: (err: Error) => {
      if (err instanceof Error) addToast('error', `Couldn't move forward: ${err.message}`);
    },
  });

  const recirculateMutation = useMutation({
    mutationFn: (targetPhase: string) => recirculateFeature(id!, targetPhase),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      addToast('success', 'Went back to redo that step');
    },
    onError: (err: Error) => addToast('error', `Failed to redo: ${err.message}`),
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
      setIsProcessing(true);
      setProcessingMode('autopilot');
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      addToast('success', 'Running everything automatically — sit back and watch');
    },
    onError: (err: Error) => {
      setProcessingMode(null);
      if (err.message.includes('already')) {
        addToast('error', 'This feature is already being worked on');
      } else {
        addToast('error', `Couldn't start: ${err.message}`);
      }
    },
  });

  const gateMutation = useMutation({
    mutationFn: () => evaluateGate(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      addToast('success', 'Quality check complete');
    },
    onError: (err: Error) => addToast('error', `Quality check failed: ${err.message}`),
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
          &larr; Back to Dashboard
        </Link>
      </div>
    );
  }

  const isTerminal = feature.status === 'done' || feature.status === 'cancelled';
  const currentPhase = feature.current_phase as PhaseName;
  const currentPhaseState = feature.phase_states[currentPhase];
  const gatePassed = currentPhaseState?.gate_result?.passed ?? false;
  const currentPhaseIndex = PHASES.indexOf(currentPhase);
  const recirculationTargets = PHASES.slice(0, currentPhaseIndex > 0 ? currentPhaseIndex : 0);
  const isDeliveryPassed = currentPhase === 'delivery' && gatePassed;
  const showProcessView = isProcessing || processMutation.isPending;

  const phaseDescriptions = PHASE_DESCRIPTIONS;

  const nextPhase = currentPhaseIndex < PHASES.length - 1 ? PHASES[currentPhaseIndex + 1] : null;
  const nextPhaseLabel = nextPhase ? PHASE_LABELS[nextPhase] : null;

  return (
    <div data-testid="feature-detail-page">
      <div className="mb-6">
        <Link to="/" className="text-blue-600 dark:text-blue-400 hover:underline text-sm">
          &larr; Back to Dashboard
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
                  : feature.status === 'waiting_for_human'
                  ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
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
            <span className="text-gray-500 dark:text-gray-400">Current Phase</span>
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

      {/* Current Phase Context */}
      {!isTerminal && (
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6" data-testid="current-phase-context">
          <div className="flex items-center gap-2 mb-2">
            <span className="text-2xl">
              {feature.status === 'waiting_for_human' ? '🙋' : '⚙️'}
            </span>
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
              {feature.status === 'waiting_for_human'
                ? 'We need your input to continue'
                : isDeliveryPassed
                ? 'All done!'
                : `Working on ${PHASE_LABELS[currentPhase as PhaseName] || currentPhase}`}
            </h3>
          </div>
          <p className="text-sm text-gray-600 dark:text-gray-400 mb-2" data-testid="phase-description">
            {feature.status === 'waiting_for_human'
              ? 'Answer the questions below so we can keep going.'
              : isDeliveryPassed
              ? 'All steps are complete. This feature is ready.'
              : phaseDescriptions[currentPhase as PhaseName] || 'Working on this step.'}
          </p>
          {!feature.status.match('waiting_for_human|done|cancelled') && !isDeliveryPassed && (
            <p className="text-xs text-gray-500 dark:text-gray-500 mb-4">
              This step produces: {PHASE_OUTPUTS[currentPhase as PhaseName] || 'deliverables'}
            </p>
          )}

          {/* Gate status line */}
          {currentPhaseState?.gate_result && (
            <div className="flex items-center gap-2 text-sm mb-4" data-testid="gate-status-line">
              {currentPhaseState.gate_result.passed ? (
                <>
                  <span className="text-green-600 dark:text-green-400 font-medium">✓ Quality check passed</span>
                  {nextPhaseLabel && (
                    <span className="text-gray-500 dark:text-gray-400">
                      — ready to move to {nextPhaseLabel}
                    </span>
                  )}
                </>
              ) : (
                <span className="text-red-600 dark:text-red-400 font-medium">✗ Quality check failed — this step needs to be redone or fixed</span>
              )}
            </div>
          )}

          {/* Primary Action: Autopilot */}
          {!isDeliveryPassed && feature.status !== 'waiting_for_human' && (
            <div className="mb-4" data-testid="primary-action">
              <button
                onClick={() => processMutation.mutate()}
                disabled={processMutation.isPending || isProcessing}
                className="px-6 py-3 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm font-semibold shadow-sm border-2 border-indigo-400 dark:border-indigo-400"
                title={isProcessing ? 'Work is in progress...' : 'Run all steps automatically: inception through delivery. Hands-off until done.'}
                data-testid="process-button"
              >
                {isProcessing ? (
                  <span className="flex items-center gap-2">
                    <span className="animate-spin rounded-full h-4 w-4 border-b-2 border-white"></span>
                    Working...
                  </span>
                ) : (
                  <span className="flex items-center gap-2">
                    ▶ Run Everything Automatically
                  </span>
                )}
              </button>
              <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
                Runs every step from start to finish: inception, planning, construction, review, testing, and delivery — all hands-off.
              </p>
            </div>
          )}

          {/* Waiting for human actions */}
          {feature.status === 'waiting_for_human' && (
            <div className="p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg" data-testid="waiting-banner">
              <p className="text-sm text-yellow-800 dark:text-yellow-200">
                Answer the questions below. The pipeline will resume automatically once all questions are answered.
              </p>
            </div>
          )}

          {/* Delivery complete action */}
          {isDeliveryPassed && (
            <div className="p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg" data-testid="delivery-complete-banner">
              <p className="text-sm text-green-800 dark:text-green-200 font-medium">
                ✓ All phases complete. Feature is ready.
              </p>
            </div>
          )}

          {/* Advanced manual controls (collapsible) */}
          <details className="mt-4" data-testid="advanced-controls">
            <summary className="text-sm text-gray-500 dark:text-gray-400 cursor-pointer hover:text-gray-700 dark:hover:text-gray-300 select-none">
              Step-by-step controls
            </summary>
            <div className="mt-3 flex flex-wrap gap-3 pt-3 border-t border-gray-200 dark:border-gray-700">
              <button
                onClick={() => runPhaseMutation.mutate()}
                disabled={runPhaseMutation.isPending}
                className="px-4 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
                title={`Run only the current step (${PHASE_LABELS[currentPhase as PhaseName]}) and check if it passes. You'll need to advance manually.`}
                data-testid="run-phase-button"
              >
                {PHASE_ACTIONS[currentPhase as PhaseName] || 'Run Current Step'}
              </button>

              <button
                onClick={() => gateMutation.mutate()}
                disabled={gateMutation.isPending}
                className="px-4 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
                title="Check whether this step meets its quality bar."
                data-testid="evaluate-gate-button"
              >
                Check Quality
              </button>

              {!isDeliveryPassed && (
                <button
                  onClick={() => advanceMutation.mutate()}
                  disabled={!gatePassed || advanceMutation.isPending}
                  className="px-4 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
                  title={!gatePassed ? 'Quality check hasn\'t passed yet — run "Check Quality" first' : `Move to the next step: ${nextPhaseLabel}`}
                  data-testid="advance-button"
                >
                  {nextPhaseLabel ? `Go to ${nextPhaseLabel}` : 'Advance'}
                </button>
              )}

              {recirculationTargets.length > 0 && (
                <select
                  className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"
                  defaultValue=""
                  onChange={(e) => {
                    if (e.target.value && window.confirm(`Go back to ${PHASE_LABELS[e.target.value as PhaseName]} and redo that step?`)) {
                      recirculateMutation.mutate(e.target.value);
                    }
                  }}
                  data-testid="recirculate-select"
                >
                  <option value="">Redo a step...</option>
                  {recirculationTargets.map((phase) => (
                    <option key={phase} value={phase}>
                      Redo {PHASE_LABELS[phase]}
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
            </div>
          </details>
        </div>
      )}

      {/* Terminal state banner */}
      {isTerminal && (
        <div className={`rounded-lg shadow p-6 mb-6 ${
          feature.status === 'done'
            ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800'
            : 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800'
        }`} data-testid="terminal-banner">
          <h3 className={`text-lg font-semibold ${
            feature.status === 'done' ? 'text-green-800 dark:text-green-200' : 'text-red-800 dark:text-red-200'
          }`}>
            {feature.status === 'done' ? '✓ Feature Complete' : '✗ Feature Cancelled'}
          </h3>
          <p className="text-sm text-gray-600 dark:text-gray-400 mt-1">
            {feature.status === 'done'
              ? 'This feature has passed all pipeline phases and is delivered.'
              : 'This feature was cancelled and will not proceed further.'}
          </p>
        </div>
      )}

      {/* Process View (shown during processing) */}
      {showProcessView && feature.status === 'in_progress' && (
        <ProcessView featureId={feature.id} mode={processingMode} />
      )}

      {/* Agent Output (shown during processing) */}
      {(showProcessView || feature.status === 'in_progress') && (
        <AgentOutput featureId={feature.id} isProcessing={isProcessing || feature.is_processing} />
      )}

      {/* Gate Results */}
      {currentPhaseState?.gate_result && (
        <GateResult gateResult={currentPhaseState.gate_result} />
      )}

      {/* Questions Section */}
      {questions.length > 0 && (
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Questions</h3>
            {feature.status === 'waiting_for_human' && (
              <span className="px-3 py-1 rounded-full text-sm font-medium bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200">
                {questions.filter((q) => q.status === 'pending').length} pending
              </span>
            )}
          </div>
          {feature.status === 'waiting_for_human' && (
            <div className="mb-4 p-4 bg-yellow-50 dark:bg-yellow-900/20 border-2 border-yellow-300 dark:border-yellow-700 rounded-lg" data-testid="waiting-for-human-banner">
              <p className="text-sm text-yellow-800 dark:text-yellow-200 font-medium">
                The pipeline is paused waiting for your input.
              </p>
              <p className="text-xs text-yellow-700 dark:text-yellow-300 mt-1">
                Answer all questions below — the pipeline resumes automatically once every question is answered.
              </p>
            </div>
          )}
          <div className="space-y-4">
            {questions.map((q) => (
              <QuestionCard key={q.id} question={q} featureId={feature.id} />
            ))}
          </div>
          {questions.every((q) => q.status !== 'pending') && questions.length > 0 && (
            <div className="mt-4 p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg" data-testid="all-questions-answered">
              <p className="text-sm text-green-600 dark:text-green-400 font-medium">
                ✓ All questions answered. Pipeline will resume automatically.
              </p>
            </div>
          )}
        </div>
      )}

      {/* Artifacts */}
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Artifacts</h3>
        <ArtifactViewer featureId={feature.id} phaseStates={feature.phase_states} />
      </div>
    </div>
  );
}