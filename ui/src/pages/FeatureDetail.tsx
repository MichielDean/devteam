import { useState, useEffect, useCallback } from 'react';
import { useParams, Link } from 'react-router';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getFeature, getFeatureStages, getAuditTrail, getBolts, getRules,
  runStage, approveStage, rejectStage, acceptStageAsIs, jumpToStage,
  setScope, setDepth, setTestStrategy, setLadderMode, prepareBolts,
  runBolt, cancelFeature, listQuestions, answerQuestion, ApiError,
} from '../api/client';
import { useSSE } from '../hooks/useSSE';
import { useToast } from '../components/Toast';
import { useUIStore } from '../store/ui-store';
import { useKeyboardShortcuts } from '../hooks/useKeyboardShortcuts';
import { Button, Badge, Card } from '../ui/primitives';
import FeatureHeader from '../components/FeatureHeader';
import StageRail from '../components/StageRail';
import GatePanel from '../components/GatePanel';
import AgentOutputLive from '../components/AgentOutputLive';
import QuestionPanel from '../components/QuestionPanel';
import BoltPanel from '../components/BoltPanel';
import ControlBar from '../components/ControlBar';
import AuditDrawer from '../components/AuditDrawer';
import ArtifactViewer from '../components/ArtifactViewer';
import type { FeatureDetail as FeatureDetailType } from '../types';
import { STAGE_STATUS_LABELS, AGENT_LABELS } from '../types';

const MAX_REVISIONS = 3;

export default function FeatureDetail() {
  const { id, stageId: routeStageId } = useParams<{ id: string; stageId?: string }>();
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const { selectedStageId, setSelectedStage, auditDrawerOpen, toggleAuditDrawer } = useUIStore();

  const [draft, setDraft] = useState<Record<string, string>>({});
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isProcessing, setIsProcessing] = useState(false);

  const { data: feature, isLoading, error } = useQuery({
    queryKey: ['feature', id!],
    queryFn: () => getFeature(id!),
    enabled: !!id,
    refetchInterval: isProcessing ? 2000 : false,
  });

  const { connected: sseConnected } = useSSE(id ?? null);
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

  const { data: sessions = [] } = useQuery({
    queryKey: ['sessions', id!],
    queryFn: () => fetch(`/api/features/${id}/sessions`).then((r) => r.json()).catch(() => []),
    enabled: !!id,
  });

  const isWaitingForHuman = feature?.status === 'waiting_for_feedback';
  const pendingQuestions = questions.filter((q) => q.status === 'pending');

  // Sync selected stage with route
  useEffect(() => {
    if (routeStageId) setSelectedStage(routeStageId);
  }, [routeStageId, setSelectedStage]);

  // Sync processing state
  useEffect(() => {
    if (feature) setIsProcessing(feature.is_processing);
  }, [feature?.is_processing]);

  // SSE event handling
  useSSE(id ?? null, (event) => {
    if (event.type === 'processing_complete' || event.type === 'error') {
      setIsProcessing(false);
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['stages', id!] });
      queryClient.invalidateQueries({ queryKey: ['audit', id!] });
      queryClient.invalidateQueries({ queryKey: ['features'] });
    } else if (event.type === 'stage_started' || event.type === 'stage_awaiting_approval' || event.type === 'stage_revising' || event.type === 'stage_completed' || event.type === 'gate_approved' || event.type === 'gate_rejected') {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['stages', id!] });
      queryClient.invalidateQueries({ queryKey: ['audit', id!] });
    }
  });

  const onSelect = useCallback((qid: string, option: string) => {
    setDraft((prev) => ({ ...prev, [qid]: option === 'Other' ? '' : option }));
  }, []);
  const onType = useCallback((qid: string, text: string) => {
    setDraft((prev) => ({ ...prev, [qid]: text }));
  }, []);

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
        const details = apiErr?.details ?? (err instanceof Error ? err.message : 'Failed');
        if (code === 'conflict') {
          addToast('error', details || 'Question already answered');
        } else {
          addToast('error', details || `Failed (${code})`);
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

  // ─── Mutations ───
  const runStageMutation = useMutation({
    mutationFn: (stageId: string) => runStage(id!, stageId),
    onMutate: async (stageId) => {
      // Optimistic: mark stage in_progress
      await queryClient.cancelQueries({ queryKey: ['stages', id!] });
      const prev = queryClient.getQueryData(['stages', id!]) as any[] ?? [];
      queryClient.setQueryData(['stages', id!], prev.map((s) => s.stage_id === stageId ? { ...s, status: 'in_progress' } : s));
      return { prev };
    },
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
    onError: (err: Error, _data, ctx) => {
      setIsProcessing(false);
      if (ctx?.prev) queryClient.setQueryData(['stages', id!], ctx.prev);
      addToast('error', `Failed: ${err.message}`);
    },
  });

  const approveMutation = useMutation({
    mutationFn: (stageId: string) => approveStage(id!, stageId),
    onMutate: async (stageId) => {
      await queryClient.cancelQueries({ queryKey: ['stages', id!] });
      const prev = queryClient.getQueryData(['stages', id!]) as any[] ?? [];
      queryClient.setQueryData(['stages', id!], prev.map((s) => s.stage_id === stageId ? { ...s, status: 'completed' } : s));
      return { prev };
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['stages', id!] });
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['audit', id!] });
      addToast('success', 'Stage approved — advancing');
    },
    onError: (err: Error, _data, ctx) => {
      if (ctx?.prev) queryClient.setQueryData(['stages', id!], ctx.prev);
      addToast('error', `Approve failed: ${err.message}`);
    },
  });

  const rejectMutation = useMutation({
    mutationFn: ({ stageId, notes }: { stageId: string; notes: string }) => rejectStage(id!, stageId, notes),
    onSuccess: () => {
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
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['feature', id!] }); queryClient.invalidateQueries({ queryKey: ['stages', id!] }); addToast('success', 'Scope updated'); },
    onError: (err: Error) => addToast('error', err.message),
  });

  const depthMutation = useMutation({
    mutationFn: (newDepth: string) => setDepth(id!, newDepth),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['feature', id!] }); addToast('success', 'Depth updated'); },
    onError: (err: Error) => addToast('error', err.message),
  });

  const testStrategyMutation = useMutation({
    mutationFn: (newStrategy: string) => setTestStrategy(id!, newStrategy),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['feature', id!] }); addToast('success', 'Test strategy updated'); },
    onError: (err: Error) => addToast('error', err.message),
  });

  const ladderMutation = useMutation({
    mutationFn: (mode: string) => setLadderMode(id!, mode),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['feature', id!] }); addToast('success', 'Autonomy mode set'); },
    onError: (err: Error) => addToast('error', err.message),
  });

  const prepareBoltsMutation = useMutation({
    mutationFn: () => prepareBolts(id!),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['bolts', id!] }); addToast('success', 'Bolts prepared'); },
    onError: (err: Error) => addToast('error', err.message),
  });

  const runBoltMutation = useMutation({
    mutationFn: (boltNumber: number) => runBolt(id!, boltNumber),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['bolts', id!] }); addToast('success', 'Bolt started'); },
    onError: (err: Error) => addToast('error', err.message),
  });

  const cancelMutation = useMutation({
    mutationFn: () => cancelFeature(id!),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['feature', id!] }); addToast('success', 'Feature cancelled'); },
    onError: (err: Error) => addToast('error', err.message),
  });

  // ─── Keyboard Shortcuts ───
  useKeyboardShortcuts({
    shortcuts: [
      { key: 'a', handler: () => { const s = stages.find((s) => s.status === 'awaiting_approval'); if (s) approveMutation.mutate(s.stage_id); }, description: 'Approve gate' },
      { key: 'r', handler: () => { const el = document.querySelector('[data-testid="gate-reject-button"]') as HTMLButtonElement; if (el) el.click(); }, description: 'Reject gate' },
      { key: 'Enter', handler: () => { const s = stages.find((s) => s.status === 'not_started'); if (s && !isProcessing) runStageMutation.mutate(s.stage_id); }, description: 'Run next stage' },
      { key: 'j', handler: () => { const el = document.querySelector('[data-testid="control-jump"]') as HTMLButtonElement; if (el) el.click(); }, description: 'Jump menu' },
      { key: 'g', handler: () => toggleAuditDrawer(), description: 'Toggle audit drawer' },
    ],
    enabled: !(feature?.status === 'done' || feature?.status === 'cancelled'),
  });

  // ─── Loading/Error ───
  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12" data-testid="feature-loading">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
        <span className="ml-3 text-gray-500">Loading feature...</span>
      </div>
    );
  }

  if (error || !feature) {
    return (
      <div className="text-center py-12" data-testid="feature-not-found">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-2">Feature not found</h2>
        <Link to="/" className="text-blue-600 dark:text-blue-400 hover:underline">&larr; Back</Link>
      </div>
    );
  }

  const checkTerminal = (f: FeatureDetailType) => f.status === 'done' || f.status === 'cancelled';
  const terminal = checkTerminal(feature);
  const currentScope = feature.scope || 'feature';
  const currentDepth = feature.depth || 'standard';
  const currentTestStrategy = feature.test_strategy || 'standard';
  const selectedStage = stages.find((s) => s.stage_id === (selectedStageId ?? feature.current_stage));
  const awaitingStage = stages.find((s) => s.status === 'awaiting_approval');
  const revisingStage = stages.find((s) => s.status === 'revising');
  const nextStage = stages.find((s) => s.status === 'not_started');
  const activeStage = selectedStage ?? awaitingStage ?? revisingStage ?? stages.find((s) => s.stage_id === feature.current_stage);
  const showLadderPrompt = bolts.length > 0 && bolts[0]?.status === 'completed' && !feature.autonomy_mode;

  return (
    <div className="flex flex-col h-full" data-testid="feature-detail-page">
      <div className="mb-3">
        <Link to="/" className="text-blue-600 dark:text-blue-400 hover:underline text-sm">&larr; Dashboard</Link>
      </div>

      <FeatureHeader feature={feature} sessionsCount={sessions.length} isTerminal={terminal} />

      <div className="flex gap-4 flex-1 min-h-0">
        <StageRail stages={stages} currentStageId={feature.current_stage} />

        <div className="flex-1 min-w-0 overflow-y-auto space-y-4">
          {/* Current Stage Actions */}
          {!terminal && (
            <Card className="p-4" data-testid="current-stage-panel">
              {isWaitingForHuman ? (
                <div className="p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg" data-testid="waiting-banner">
                  <p className="text-sm text-yellow-800 dark:text-yellow-200">Answer the questions below. The pipeline resumes automatically once all are answered.</p>
                </div>
              ) : isProcessing ? (
                <div className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400" data-testid="processing-banner">
                  <span className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600" />
                  Agent working on stage {feature.current_stage || '...'}
                </div>
              ) : awaitingStage ? (
                <div data-testid="awaiting-approval-banner">
                  <p className="text-sm font-medium text-yellow-800 dark:text-yellow-200 mb-2">Stage {awaitingStage.stage_id} awaiting approval</p>
                  <Button variant="warning" onClick={() => setSelectedStage(awaitingStage.stage_id)} data-testid="review-gate-button">Review Gate</Button>
                </div>
              ) : revisingStage ? (
                <div className="p-3 bg-orange-50 dark:bg-orange-900/20 border border-orange-200 dark:border-orange-800 rounded-lg" data-testid="revising-banner">
                  <p className="text-sm font-medium text-orange-800 dark:text-orange-200 mb-2">Stage {revisingStage.stage_id} needs revision ({revisingStage.revision_count} revisions)</p>
                  <Button variant="warning" size="sm" onClick={() => runStageMutation.mutate(revisingStage.stage_id)} disabled={runStageMutation.isPending} data-testid="rerun-stage-button">Re-run Stage</Button>
                </div>
              ) : nextStage ? (
                <div data-testid="next-stage-panel">
                  <p className="text-sm text-gray-600 dark:text-gray-400 mb-2">Next: <strong>{nextStage.stage_id}</strong></p>
                  <Button variant="primary" onClick={() => runStageMutation.mutate(nextStage.stage_id)} disabled={runStageMutation.isPending} isLoading={runStageMutation.isPending} data-testid="run-stage-button">
                    ▶ Run Stage {nextStage.stage_id}
                  </Button>
                </div>
              ) : (
                <p className="text-sm text-gray-500" data-testid="no-next-stage">All stages complete or in progress.</p>
              )}
            </Card>
          )}

          {terminal && (
            <div className={`rounded-lg shadow p-4 ${feature.status === 'done' ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800' : 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800'}`} data-testid="terminal-banner">
              <h3 className={`text-lg font-semibold ${feature.status === 'done' ? 'text-green-800 dark:text-green-200' : 'text-red-800 dark:text-red-200'}`}>{feature.status === 'done' ? '✓ Feature Complete' : '✗ Feature Cancelled'}</h3>
            </div>
          )}

          {/* Stage Detail */}
          {activeStage && (
            <Card className="p-4" data-testid="stage-detail">
              <div className="flex items-center gap-2 mb-3">
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{activeStage.stage_id}</h3>
                <Badge color="blue">{STAGE_STATUS_LABELS[activeStage.status] || activeStage.status}</Badge>
                {activeStage.revision_count > 0 && <Badge color="orange">×{activeStage.revision_count}</Badge>}
              </div>

              {/* Gate Panel inline */}
              {activeStage.status === 'awaiting_approval' && (
                <GatePanel
                  stageId={activeStage.stage_id}
                  stageName={activeStage.stage_id}
                  revisionCount={activeStage.revision_count}
                  canAcceptAsIs={activeStage.revision_count >= MAX_REVISIONS}
                  onApprove={() => approveMutation.mutate(activeStage.stage_id)}
                  onReject={(notes) => rejectMutation.mutate({ stageId: activeStage.stage_id, notes })}
                  onAcceptAsIs={() => acceptAsIsMutation.mutate(activeStage.stage_id)}
                />
              )}

              {/* Agent Output */}
              {(isProcessing || feature.is_processing) && (
                <AgentOutputLive featureId={feature.id} stageId={activeStage.stage_id} isProcessing={isProcessing || feature.is_processing} />
              )}
            </Card>
          )}

          {/* Construction Bolts */}
          {feature.current_phase === 'construction' && (
            <BoltPanel
              bolts={bolts}
              onPrepareBolts={() => prepareBoltsMutation.mutate()}
              onRunBolt={(n) => runBoltMutation.mutate(n)}
              onSetLadder={(m) => ladderMutation.mutate(m)}
              autonomyMode={feature.autonomy_mode}
              showLadderPrompt={showLadderPrompt}
            />
          )}

          {/* Learned Rules */}
          {rules.length > 0 && (
            <Card className="p-4" data-testid="rules-panel">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">Learned Rules ({rules.length})</h3>
              <div className="space-y-2" data-testid="rules-list">
                {rules.map((r) => (
                  <div key={r.id} className="p-3 bg-orange-50 dark:bg-orange-900/20 border border-orange-200 dark:border-orange-800 rounded-lg" data-testid={`rule-${r.id}`}>
                    <span className="text-xs text-orange-700 dark:text-orange-300">{AGENT_LABELS[r.agent_name] || r.agent_name} · {r.stage_id || 'global'}</span>
                    <p className="text-sm text-gray-900 dark:text-white mt-1">{r.rule_text}</p>
                  </div>
                ))}
              </div>
            </Card>
          )}

          {/* Questions */}
          <QuestionPanel
            questions={questions}
            drafts={draft}
            onSelect={onSelect}
            onType={onType}
            onSubmitAll={handleSubmitAll}
            isSubmitting={isSubmitting}
            allDrafted={allPendingDrafted}
            isWaitingForHuman={isWaitingForHuman ?? false}
          />

          {/* Artifacts */}
          <Card className="p-4" data-testid="artifacts-panel">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">Artifacts</h3>
            <ArtifactViewer featureId={feature.id} phaseStates={{}} />
          </Card>
        </div>
      </div>

      {/* Control Bar */}
      <div className="mt-3">
        <ControlBar
          onJumpStage={(stageId) => jumpMutation.mutate({ stageId })}
          onJumpPhase={(phase) => jumpMutation.mutate({ phase })}
          onSetScope={(s) => scopeMutation.mutate(s)}
          onSetDepth={(d) => depthMutation.mutate(d)}
          onSetTestStrategy={(t) => testStrategyMutation.mutate(t)}
          onCancel={() => cancelMutation.mutate()}
          currentScope={currentScope}
          currentDepth={currentDepth}
          currentTestStrategy={currentTestStrategy}
          availableStages={stages}
          isTerminal={terminal}
        />
      </div>

      {/* Audit Drawer */}
      <AuditDrawer open={auditDrawerOpen} onClose={() => toggleAuditDrawer()} events={auditEvents} />
    </div>
  );
}