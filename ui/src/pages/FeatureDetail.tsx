import { useState, useEffect, useRef, useCallback } from 'react';
import { useParams, Link } from 'react-router';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getFeature, getFeatureStages, getAuditTrail, getBolts, getRules,
  runStage, approveStage, rejectStage, acceptStageAsIs, jumpToStage,
  setScope, setDepth, setTestStrategy, setLadderMode, prepareBolts,
  cancelFeature, listQuestions, answerQuestion, ApiError,
} from '../api/client';
import { useSSE } from '../hooks/useSSE';
import { useToast } from '../components/Toast';
import StageProgress from '../components/StageProgress';
import GateModal from '../components/GateModal';
import AuditTimeline from '../components/AuditTimeline';
import ArtifactViewer from '../components/ArtifactViewer';
import AgentOutput from '../components/AgentOutput';
import QuestionCard from '../components/QuestionCard';
import type { FeatureDetail } from '../types';
import {
  SCOPES, SCOPE_LABELS, SCOPE_DESCRIPTIONS, DEPTHS, DEPTH_LABELS,
  TEST_STRATEGIES, TEST_STRATEGY_LABELS, STATUS_LABELS, PRIORITY_LABELS,
  STAGE_STATUS_LABELS,
} from '../types';

const MAX_REVISIONS = 3;

export default function FeatureDetail() {
  const { id } = useParams<{ id: string }>();
  const queryClient = useQueryClient();
  const { addToast } = useToast();

  const [draft, setDraft] = useState<Record<string, string>>({});
  const [isSubmitting, setIsSubmitting] = useState(false);
  const questionCardRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const summaryRef = useRef<HTMLDivElement | null>(null);
  const [gateModalStage, setGateModalStage] = useState<string | null>(null);
  const [isProcessing, setIsProcessing] = useState(false);

  const { data: feature, isLoading, error } = useQuery({
    queryKey: ['feature', id!],
    queryFn: () => getFeature(id!),
    enabled: !!id,
    refetchInterval: isProcessing ? 2000 : false,
  });

  const { connected: sseConnected, lastEvent } = useSSE(id ?? null);
  void sseConnected;

  const { data: questions = [] } = useQuery({
    queryKey: ['questions', id!],
    queryFn: () => listQuestions(id!),
    enabled: !!id,
  });

  const { data: stages = [] } = useQuery({
    queryKey: ['stages', id!],
    queryFn: () => getFeatureStages(id!),
    enabled: !!id,
    refetchInterval: isProcessing ? 2000 : false,
  });

  const { data: auditEvents = [] } = useQuery({
    queryKey: ['audit', id!],
    queryFn: () => getAuditTrail(id!),
    enabled: !!id,
    refetchInterval: isProcessing ? 3000 : false,
  });

  const { data: bolts = [] } = useQuery({
    queryKey: ['bolts', id!],
    queryFn: () => getBolts(id!),
    enabled: !!id,
  });

  const { data: rules = [] } = useQuery({
    queryKey: ['rules', id!],
    queryFn: () => getRules(id!),
    enabled: !!id,
  });

  const isWaitingForHuman = feature?.status === 'waiting_for_feedback';
  const pendingQuestions = questions.filter((q) => q.status === 'pending');

  useEffect(() => {
    if (!isWaitingForHuman || pendingQuestions.length === 0) return;
    const nextEmpty = pendingQuestions.find((q) => !(draft[q.id]?.trim()));
    if (nextEmpty) {
      const el = questionCardRefs.current[nextEmpty.id];
      if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' });
    } else if (summaryRef.current) {
      summaryRef.current.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  }, [draft, isWaitingForHuman]);

  useEffect(() => {
    if (!isWaitingForHuman) setDraft({});
  }, [isWaitingForHuman]);

  const onSelect = useCallback((qid: string, option: string) => {
    if (option === 'Other') {
      setDraft((prev) => ({ ...prev, [qid]: '' }));
    } else {
      setDraft((prev) => ({ ...prev, [qid]: option }));
    }
  }, []);
  const onType = useCallback((qid: string, text: string) => {
    setDraft((prev) => ({ ...prev, [qid]: text }));
  }, []);

  const setCardRef = useCallback(
    (qid: string) => (el: HTMLDivElement | null) => {
      questionCardRefs.current[qid] = el;
    },
    [],
  );

  const allPendingDrafted = pendingQuestions.every((q) => (draft[q.id]?.trim() ?? '').length > 0);

  const handleSubmitAll = async () => {
    if (!id || !allPendingDrafted) return;
    setIsSubmitting(true);
    let aborted = false;
    for (const q of pendingQuestions) {
      const answer = (draft[q.id] ?? '').trim();
      if (!answer) continue;
      try {
        await answerQuestion(id, q.id, answer);
      } catch (err) {
        const apiErr = err instanceof ApiError ? err : null;
        const code = apiErr?.code ?? 'unknown_error';
        const details = apiErr?.details ?? (err instanceof Error ? err.message : 'Failed to answer question');
        if (code === 'conflict') {
          addToast('error', details || 'Question already answered');
        } else {
          addToast('error', details || `Failed to answer question (${code})`);
          aborted = true;
          break;
        }
      }
    }
    setIsSubmitting(false);
    if (!aborted) {
      setDraft({});
      queryClient.invalidateQueries({ queryKey: ['questions', id!] });
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['features'] });
      addToast('success', 'Answers submitted — resuming');
    }
  };

  useEffect(() => {
    if (feature) setIsProcessing(feature.is_processing);
  }, [feature?.is_processing]);

  useEffect(() => {
    if (!lastEvent) return;
    if (lastEvent.type === 'processing_complete' || lastEvent.type === 'error') {
      setIsProcessing(false);
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['stages', id!] });
      queryClient.invalidateQueries({ queryKey: ['audit', id!] });
      queryClient.invalidateQueries({ queryKey: ['features'] });
    } else if (lastEvent.type === 'agent_dispatch' || lastEvent.type === 'stage_change') {
      setIsProcessing(true);
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['stages', id!] });
    } else if (lastEvent.type === 'question_answered') {
      queryClient.invalidateQueries({ queryKey: ['questions', id!] });
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
    }
  }, [lastEvent, id, queryClient]);

  // ─── Mutations ───
  const runStageMutation = useMutation({
    mutationFn: (stageId: string) => runStage(id!, stageId),
    onSuccess: (data) => {
      setIsProcessing(true);
      queryClient.invalidateQueries({ queryKey: ['stages', id!] });
      queryClient.invalidateQueries({ queryKey: ['audit', id!] });
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      if (data.gate?.state === 'open' || data.outcome_source === 'agent_signal') {
        addToast('success', `Stage ${data.stage_id} complete — review the gate`);
      } else {
        addToast('success', `Stage ${data.stage_id} dispatched`);
      }
    },
    onError: (err: Error) => {
      setIsProcessing(false);
      addToast('error', `Failed to run stage: ${err.message}`);
    },
  });

  const approveMutation = useMutation({
    mutationFn: (stageId: string) => approveStage(id!, stageId),
    onSuccess: () => {
      setGateModalStage(null);
      queryClient.invalidateQueries({ queryKey: ['stages', id!] });
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['audit', id!] });
      addToast('success', 'Stage approved — advancing');
    },
    onError: (err: Error) => addToast('error', `Approve failed: ${err.message}`),
  });

  const rejectMutation = useMutation({
    mutationFn: ({ stageId, notes }: { stageId: string; notes: string }) => rejectStage(id!, stageId, notes),
    onSuccess: () => {
      setGateModalStage(null);
      queryClient.invalidateQueries({ queryKey: ['stages', id!] });
      queryClient.invalidateQueries({ queryKey: ['rules', id!] });
      queryClient.invalidateQueries({ queryKey: ['audit', id!] });
      addToast('success', 'Sent back for revision — rule saved');
    },
    onError: (err: Error) => addToast('error', `Reject failed: ${err.message}`),
  });

  const acceptAsIsMutation = useMutation({
    mutationFn: (stageId: string) => acceptStageAsIs(id!, stageId),
    onSuccess: () => {
      setGateModalStage(null);
      queryClient.invalidateQueries({ queryKey: ['stages', id!] });
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['audit', id!] });
      addToast('success', 'Accepted as-is — advancing');
    },
    onError: (err: Error) => addToast('error', `Accept failed: ${err.message}`),
  });

  const jumpMutation = useMutation({
    mutationFn: ({ stageId, phase }: { stageId?: string; phase?: string }) => jumpToStage(id!, stageId, phase),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['stages', id!] });
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['audit', id!] });
      addToast('success', 'Jumped to stage');
    },
    onError: (err: Error) => addToast('error', `Jump failed: ${err.message}`),
  });

  const scopeMutation = useMutation({
    mutationFn: (newScope: string) => setScope(id!, newScope),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['stages', id!] });
      addToast('success', 'Scope updated');
    },
    onError: (err: Error) => addToast('error', `Scope change failed: ${err.message}`),
  });

  const depthMutation = useMutation({
    mutationFn: (newDepth: string) => setDepth(id!, newDepth),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      addToast('success', 'Depth updated');
    },
    onError: (err: Error) => addToast('error', `Depth change failed: ${err.message}`),
  });

  const testStrategyMutation = useMutation({
    mutationFn: (newStrategy: string) => setTestStrategy(id!, newStrategy),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      addToast('success', 'Test strategy updated');
    },
    onError: (err: Error) => addToast('error', `Test strategy failed: ${err.message}`),
  });

  const ladderMutation = useMutation({
    mutationFn: (mode: string) => setLadderMode(id!, mode),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      addToast('success', 'Autonomy mode set');
    },
    onError: (err: Error) => addToast('error', `Ladder failed: ${err.message}`),
  });

  const prepareBoltsMutation = useMutation({
    mutationFn: () => prepareBolts(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bolts', id!] });
      addToast('success', 'Bolts prepared from inception output');
    },
    onError: (err: Error) => addToast('error', `Prepare bolts failed: ${err.message}`),
  });

  const cancelMutation = useMutation({
    mutationFn: () => cancelFeature(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      addToast('success', 'Feature cancelled');
    },
    onError: (err: Error) => addToast('error', `Cancel failed: ${err.message}`),
  });

  // ─── Loading/Error ───
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
        <p className="text-gray-500 dark:text-gray-400 mb-4">The feature you're looking for doesn't exist.</p>
        <Link to="/" className="text-blue-600 dark:text-blue-400 hover:underline">&larr; Back to Dashboard</Link>
      </div>
    );
  }

  const isTerminal = feature.status === 'done' || feature.status === 'cancelled';
  const currentScope = feature.scope || 'feature';
  const currentDepth = feature.depth || 'standard';
  const currentTestStrategy = feature.test_strategy || 'standard';
  const currentStage = stages.find((s) => s.stage_id === feature.current_stage);
  void currentStage;
  const awaitingStage = stages.find((s) => s.status === 'awaiting_approval');
  const revisingStage = stages.find((s) => s.status === 'revising');
  const gateStageId = gateModalStage || awaitingStage?.stage_id || revisingStage?.stage_id;
  const gateStage = stages.find((s) => s.stage_id === gateStageId);

  // Find the next not_started stage to run
  const nextStage = stages.find((s) => s.status === 'not_started');

  return (
    <div data-testid="feature-detail-page">
      <div className="mb-6">
        <Link to="/" className="text-blue-600 dark:text-blue-400 hover:underline text-sm">&larr; Back to Dashboard</Link>
      </div>

      {/* Header */}
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white" data-testid="feature-title">{feature.title}</h1>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1" data-testid="feature-id">{feature.id}</p>
          </div>
          <div className="flex items-center gap-2">
            <span className={`px-3 py-1 rounded-full text-sm font-medium ${isTerminal ? (feature.status === 'done' ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200') : 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'}`} data-testid="feature-status">{STATUS_LABELS[feature.status] || feature.status}</span>
            <span className="px-3 py-1 rounded-full text-sm font-medium bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200" data-testid="feature-priority">{PRIORITY_LABELS[feature.priority] || `P${feature.priority}`}</span>
          </div>
        </div>
        <div className="mt-4 grid grid-cols-2 sm:grid-cols-4 gap-4 text-sm">
          <div>
            <span className="text-gray-500 dark:text-gray-400">Scope</span>
            <p className="font-medium text-gray-900 dark:text-white" data-testid="feature-scope">{SCOPE_LABELS[currentScope] || currentScope}</p>
          </div>
          <div>
            <span className="text-gray-500 dark:text-gray-400">Depth</span>
            <p className="font-medium text-gray-900 dark:text-white" data-testid="feature-depth">{DEPTH_LABELS[currentDepth] || currentDepth}</p>
          </div>
          <div>
            <span className="text-gray-500 dark:text-gray-400">Current Stage</span>
            <p className="font-medium text-gray-900 dark:text-white" data-testid="feature-current-stage">{feature.current_stage || '—'}</p>
          </div>
          <div>
            <span className="text-gray-500 dark:text-gray-400">Intake</span>
            <p className="font-medium text-gray-900 dark:text-white">{feature.intake_path === 'loose_idea' ? 'Loose Idea' : 'External Spec'}</p>
          </div>
        </div>
      </div>

      {/* Stage Progress */}
      <StageProgress stages={stages} currentStageId={feature.current_stage} />

      {/* Current Stage Actions */}
      {!isTerminal && (
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6" data-testid="current-stage-panel">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">Current Stage</h3>

          {isWaitingForHuman ? (
            <div className="p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg" data-testid="waiting-banner">
              <p className="text-sm text-yellow-800 dark:text-yellow-200">Answer the questions below. The pipeline resumes automatically once all are answered.</p>
            </div>
          ) : isProcessing ? (
            <div className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400" data-testid="processing-banner">
              <span className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600"></span>
              Agent working on stage {feature.current_stage || '...'} — watch output below
            </div>
          ) : awaitingStage ? (
            <div className="p-4 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg" data-testid="awaiting-approval-banner">
              <p className="text-sm font-medium text-yellow-800 dark:text-yellow-200 mb-2">Stage {awaitingStage.stage_id} is awaiting your approval</p>
              <button onClick={() => setGateModalStage(awaitingStage.stage_id)} className="px-4 py-2 bg-yellow-600 text-white rounded-lg hover:bg-yellow-700 text-sm font-medium" data-testid="review-gate-button">Review Gate</button>
            </div>
          ) : revisingStage ? (
            <div className="p-4 bg-orange-50 dark:bg-orange-900/20 border border-orange-200 dark:border-orange-800 rounded-lg" data-testid="revising-banner">
              <p className="text-sm font-medium text-orange-800 dark:text-orange-200 mb-2">Stage {revisingStage.stage_id} needs revision ({revisingStage.revision_count} revisions)</p>
              <p className="text-xs text-orange-700 dark:text-orange-300 mb-2">The agent was sent back. Re-run the stage to address the feedback.</p>
              <button onClick={() => runStageMutation.mutate(revisingStage.stage_id)} disabled={runStageMutation.isPending} className="px-4 py-2 bg-orange-600 text-white rounded-lg hover:bg-orange-700 text-sm font-medium" data-testid="rerun-stage-button">Re-run Stage {revisingStage.stage_id}</button>
            </div>
          ) : nextStage ? (
            <div data-testid="next-stage-panel">
              <p className="text-sm text-gray-600 dark:text-gray-400 mb-3">Next stage: <strong>{nextStage.stage_id}</strong> ({STAGE_STATUS_LABELS[nextStage.status]})</p>
              <button onClick={() => runStageMutation.mutate(nextStage.stage_id)} disabled={runStageMutation.isPending} className="px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-sm font-semibold shadow-sm" data-testid="run-stage-button">
                {runStageMutation.isPending ? 'Starting...' : `▶ Run Stage ${nextStage.stage_id}`}
              </button>
            </div>
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400" data-testid="no-next-stage">All stages complete or in progress.</p>
          )}

          {/* Jump controls */}
          <details className="mt-4" data-testid="jump-controls">
            <summary className="text-sm text-gray-500 dark:text-gray-400 cursor-pointer hover:text-gray-700 dark:hover:text-gray-300 select-none">Jump to stage or phase</summary>
            <div className="mt-3 flex flex-wrap gap-3 pt-3 border-t border-gray-200 dark:border-gray-700">
              <select className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm" defaultValue="" onChange={(e) => { if (e.target.value) jumpMutation.mutate({ stageId: e.target.value }); }} data-testid="jump-stage-select">
                <option value="">Jump to stage...</option>
                {stages.filter((s) => s.status === 'not_started' || s.status === 'skipped').map((s) => <option key={s.stage_id} value={s.stage_id}>Stage {s.stage_id}</option>)}
              </select>
              <select className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm" defaultValue="" onChange={(e) => { if (e.target.value) jumpMutation.mutate({ phase: e.target.value }); }} data-testid="jump-phase-select">
                <option value="">Jump to phase...</option>
                <option value="ideation">Ideation</option>
                <option value="inception">Inception</option>
                <option value="construction">Construction</option>
                <option value="operation">Operation</option>
              </select>
            </div>
          </details>

          {/* Scope/Depth/Test Strategy controls */}
          <details className="mt-2" data-testid="scope-controls">
            <summary className="text-sm text-gray-500 dark:text-gray-400 cursor-pointer hover:text-gray-700 dark:hover:text-gray-300 select-none">Scope, depth & test strategy</summary>
            <div className="mt-3 grid grid-cols-1 sm:grid-cols-3 gap-3 pt-3 border-t border-gray-200 dark:border-gray-700">
              <div>
                <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Scope</label>
                <select value={currentScope} onChange={(e) => scopeMutation.mutate(e.target.value)} className="w-full px-2 py-1.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm" data-testid="scope-change-select">
                  {SCOPES.map((s) => <option key={s} value={s}>{SCOPE_LABELS[s]}</option>)}
                </select>
                <p className="text-xs text-gray-400 mt-1">{SCOPE_DESCRIPTIONS[currentScope]}</p>
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Depth</label>
                <select value={currentDepth} onChange={(e) => depthMutation.mutate(e.target.value)} className="w-full px-2 py-1.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm" data-testid="depth-change-select">
                  {DEPTHS.map((d) => <option key={d} value={d}>{DEPTH_LABELS[d]}</option>)}
                </select>
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Test Strategy</label>
                <select value={currentTestStrategy} onChange={(e) => testStrategyMutation.mutate(e.target.value)} className="w-full px-2 py-1.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm" data-testid="test-strategy-change-select">
                  {TEST_STRATEGIES.map((t) => <option key={t} value={t}>{TEST_STRATEGY_LABELS[t]}</option>)}
                </select>
              </div>
            </div>
          </details>

          {/* Cancel */}
          <details className="mt-2" data-testid="cancel-controls">
            <summary className="text-sm text-gray-500 dark:text-gray-400 cursor-pointer hover:text-gray-700 dark:hover:text-gray-300 select-none">Cancel feature</summary>
            <div className="mt-3 pt-3 border-t border-gray-200 dark:border-gray-700">
              <button onClick={() => { if (window.confirm('Cancel this feature?')) cancelMutation.mutate(); }} disabled={cancelMutation.isPending} className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 text-sm" data-testid="cancel-button">Cancel Feature</button>
            </div>
          </details>
        </div>
      )}

      {/* Terminal state */}
      {isTerminal && (
        <div className={`rounded-lg shadow p-6 mb-6 ${feature.status === 'done' ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800' : 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800'}`} data-testid="terminal-banner">
          <h3 className={`text-lg font-semibold ${feature.status === 'done' ? 'text-green-800 dark:text-green-200' : 'text-red-800 dark:text-red-200'}`}>{feature.status === 'done' ? '✓ Feature Complete' : '✗ Feature Cancelled'}</h3>
        </div>
      )}

      {/* Agent Output */}
      {(isProcessing || feature.is_processing) && (
        <AgentOutput featureId={feature.id} isProcessing={isProcessing || feature.is_processing} />
      )}

      {/* Gate Modal */}
      {gateModalStage && gateStage && (
        <GateModal
          stageId={gateStage.stage_id}
          stageName={`Stage ${gateStage.stage_id}`}
          revisionCount={gateStage.revision_count}
          canAcceptAsIs={gateStage.revision_count >= MAX_REVISIONS}
          onApprove={() => approveMutation.mutate(gateStage.stage_id)}
          onReject={(notes) => rejectMutation.mutate({ stageId: gateStage.stage_id, notes })}
          onAcceptAsIs={() => acceptAsIsMutation.mutate(gateStage.stage_id)}
          onClose={() => setGateModalStage(null)}
        />
      )}

      {/* Construction Bolts */}
      {feature.current_phase === 'construction' && (
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6" data-testid="bolts-panel">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Construction Bolts</h3>
            {bolts.length === 0 && (
              <button onClick={() => prepareBoltsMutation.mutate()} disabled={prepareBoltsMutation.isPending} className="px-3 py-1.5 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 text-sm" data-testid="prepare-bolts-button">Prepare Bolts</button>
            )}
          </div>
          {bolts.length === 0 ? (
            <p className="text-sm text-gray-500 dark:text-gray-400">No Bolts yet. Prepare Bolts from the inception output to start construction.</p>
          ) : (
            <div className="space-y-2" data-testid="bolt-list">
              {bolts.map((b) => (
                <div key={b.bolt_number} className={`flex items-center gap-3 p-3 rounded-lg ${b.is_walking_skeleton ? 'bg-indigo-50 dark:bg-indigo-900/20 border border-indigo-200 dark:border-indigo-800' : 'bg-gray-50 dark:bg-gray-900/30'}`} data-testid={`bolt-${b.bolt_number}`}>
                  <span className="font-mono text-sm font-medium text-gray-900 dark:text-white">Bolt {b.bolt_number}</span>
                  {b.is_walking_skeleton && <span className="text-xs px-1.5 py-0.5 rounded bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-200" data-testid="walking-skeleton-badge">Walking Skeleton</span>}
                  <span className={`text-sm ${b.status === 'completed' ? 'text-green-600' : b.status === 'in_progress' ? 'text-blue-600' : b.status === 'failed' ? 'text-red-600' : 'text-gray-500'}`} data-testid={`bolt-status-${b.bolt_number}`}>{b.status}</span>
                  <span className="text-xs text-gray-400">{b.unit_ids.length} unit(s)</span>
                </div>
              ))}
              {/* Ladder prompt */}
              {bolts[0]?.status === 'completed' && !feature.autonomy_mode && (
                <div className="mt-4 p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg" data-testid="ladder-prompt">
                  <p className="text-sm font-medium text-yellow-800 dark:text-yellow-200 mb-2">🪜 Ladder Prompt: Walking skeleton complete. Choose autonomy mode for remaining Bolts:</p>
                  <div className="flex gap-2">
                    <button onClick={() => ladderMutation.mutate('gated')} className="px-3 py-1.5 bg-yellow-600 text-white rounded-lg hover:bg-yellow-700 text-sm" data-testid="ladder-gated">Gated (approve each Bolt)</button>
                    <button onClick={() => ladderMutation.mutate('autonomous')} className="px-3 py-1.5 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 text-sm" data-testid="ladder-autonomous">Autonomous (skip Bolt gates)</button>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* Audit Timeline */}
      <AuditTimeline events={auditEvents} />

      {/* Learned Rules */}
      {rules.length > 0 && (
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6" data-testid="rules-panel">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Learned Rules ({rules.length})</h3>
          <div className="space-y-2" data-testid="rules-list">
            {rules.map((r) => (
              <div key={r.id} className="p-3 bg-orange-50 dark:bg-orange-900/20 border border-orange-200 dark:border-orange-800 rounded-lg" data-testid={`rule-${r.id}`}>
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <span className="text-xs text-orange-700 dark:text-orange-300">Agent: {r.agent_name} · Stage: {r.stage_id || 'global'}</span>
                    <p className="text-sm text-gray-900 dark:text-white mt-1">{r.rule_text}</p>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Questions */}
      {questions.length > 0 && (
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6" data-testid="questions-section">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Questions</h3>
            <span className="px-3 py-1 rounded-full text-sm bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200" data-testid="question-progress">
              {questions.filter((q) => q.status !== 'pending').length}/{questions.length} answered
            </span>
          </div>
          <div className="space-y-4">
            {questions.map((q) => q.status === 'pending' ? <QuestionCard key={q.id} question={q} featureId={feature.id} draft={draft[q.id]} onSelect={(opt) => onSelect(q.id, opt)} onType={(text) => onType(q.id, text)} ref={setCardRef(q.id)} /> : <QuestionCard key={q.id} question={q} featureId={feature.id} />)}
          </div>
          {(isWaitingForHuman || pendingQuestions.length > 0) && (
            <div className="mt-6 border-t border-gray-200 dark:border-gray-700 pt-4">
              <div ref={summaryRef} className="mb-4 p-4 bg-gray-50 dark:bg-gray-900/30 rounded-lg" data-testid="answer-summary">
                <h4 className="text-sm font-semibold text-gray-900 dark:text-white mb-3">Answer summary</h4>
                <ul className="space-y-2">
                  {questions.map((q) => {
                    const answer = q.status === 'answered' ? q.answer : q.status === 'assumed' ? q.assumption : draft[q.id] ?? '';
                    return (
                      <li key={q.id}>
                        <button type="button" onClick={() => { const el = questionCardRefs.current[q.id]; if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' }); }} className="w-full text-left p-2 rounded hover:bg-gray-100 dark:hover:bg-gray-700" data-testid={`summary-row-${q.id}`}>
                          <span className="text-xs text-gray-500">{q.phase} · {q.role}</span>
                          <span className="block text-sm text-gray-900 dark:text-white">{q.question}</span>
                          <span className="block text-sm text-gray-700 dark:text-gray-300">{answer || <span className="italic text-gray-400">Not answered yet</span>}</span>
                        </button>
                      </li>
                    );
                  })}
                </ul>
              </div>
              <button type="button" onClick={handleSubmitAll} disabled={!allPendingDrafted || isSubmitting || pendingQuestions.length === 0} className="px-6 py-3 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed text-sm font-semibold" data-testid="submit-answers">
                {isSubmitting ? 'Submitting...' : 'Submit Answers & Resume'}
              </button>
            </div>
          )}
        </div>
      )}

      {/* Artifacts */}
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Artifacts</h3>
        <ArtifactViewer featureId={feature.id} phaseStates={{}} />
      </div>
    </div>
  );
}