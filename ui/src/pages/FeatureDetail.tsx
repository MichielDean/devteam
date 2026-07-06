import { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, Link } from 'react-router';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getFeature, getFeatureStages, getAuditTrail, getBolts, getRules,
  runStage, resumeStage, forceRerunStage, approveStage, rejectStage, acceptStageAsIs, jumpToStage,
  setScope, setDepth, setTestStrategy, setLadderMode, setExecutionMode, prepareBolts,
  runBolt, cancelFeature, listQuestions, answerQuestion, ApiError,
} from '../api/client';
import { useSSE } from '../hooks/useSSE';
import { useToast } from '../components/Toast';
import { useUIStore } from '../store/ui-store';
import { useKeyboardShortcuts } from '../hooks/useKeyboardShortcuts';
import { Button, Badge, Card } from '../ui/primitives';
import FeatureHeader from '../components/FeatureHeader';
import StageRail from '../components/StageRail';
import MobileStageRail from '../components/MobileStageRail';
import GatePanel from '../components/GatePanel';
import AgentOutputLive from '../components/AgentOutputLive';
import QuestionPanel from '../components/QuestionPanel';
import ControlBar from '../components/ControlBar';
import AuditDrawer from '../components/AuditDrawer';
import ArtifactViewer from '../components/ArtifactViewer';
import type { FeatureDetail as FeatureDetailType } from '../types';
import { STAGE_STATUS_LABELS, AGENT_LABELS } from '../types';

const MAX_REVISIONS = 3;

const STATUS_ICONS_MAP: Record<string, string> = {
  not_started: '○', in_progress: '▶', awaiting_approval: '?', revising: '↻', completed: '✓', skipped: '·',
};
const STATUS_COLOR_MAP: Record<string, string> = {
  not_started: 'var(--color-text-tertiary)', in_progress: 'var(--color-accent)', awaiting_approval: 'var(--color-warning)', revising: 'var(--color-warning)', completed: 'var(--color-success)', skipped: 'var(--color-text-tertiary)',
};
function statusColorForStatus(status: string) { return STATUS_COLOR_MAP[status] || 'var(--color-text-tertiary)'; }
function STATUS_ICONS_FOR_STATUS(status: string) { return STATUS_ICONS_MAP[status] || '○'; }

export default function FeatureDetail() {
  const { id, stageId: routeStageId } = useParams<{ id: string; stageId?: string }>();
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const { selectedStageId, setSelectedStage, auditDrawerOpen, toggleAuditDrawer } = useUIStore();

  const [draft, setDraft] = useState<Record<string, string>>({});
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isProcessing, setIsProcessing] = useState(false);
  const [artifactRefreshKey, setArtifactRefreshKey] = useState(0);

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
    select: (data) => {
      // Detect stage status changes and trigger artifact refresh
      return data;
    },
  });

  // Track previous stage statuses to detect transitions
  const prevStageStatuses = useRef<Record<string, string>>({});
  useEffect(() => {
    if (!stages.length) return;
    const changes: string[] = [];
    for (const s of stages) {
      const prev = prevStageStatuses.current[s.stage_id];
      if (prev && prev !== s.status) {
        changes.push(`${s.stage_id}: ${prev} → ${s.status}`);
      }
      prevStageStatuses.current[s.stage_id] = s.status;
    }
    if (changes.length > 0) {
      // Stage status changed — refetch artifacts and invalidate queries
      setArtifactRefreshKey((k) => k + 1);
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['audit', id!] });
    }
  }, [stages, id, queryClient]);

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

  useEffect(() => {
    if (routeStageId) setSelectedStage(routeStageId);
  }, [routeStageId, setSelectedStage]);

  useEffect(() => {
    if (feature) setIsProcessing(feature.is_processing);
  }, [feature?.is_processing]);

  useSSE(id ?? null, (event) => {
    if (event.type === 'processing_complete' || event.type === 'error') {
      setIsProcessing(false);
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['stages', id!] });
      queryClient.invalidateQueries({ queryKey: ['audit', id!] });
      queryClient.invalidateQueries({ queryKey: ['features'] });
      setArtifactRefreshKey((k) => k + 1);
    } else if (event.type === 'stage_started' || event.type === 'stage_awaiting_approval' || event.type === 'stage_revising' || event.type === 'stage_completed' || event.type === 'gate_approved' || event.type === 'gate_rejected') {
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['stages', id!] });
      queryClient.invalidateQueries({ queryKey: ['audit', id!] });
      setArtifactRefreshKey((k) => k + 1);
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

  const runStageMutation = useMutation({
    mutationFn: (stageId: string) => runStage(id!, stageId),
    onMutate: async (stageId) => {
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

  const executionModeMutation = useMutation({
    mutationFn: (mode: string) => setExecutionMode(id!, mode),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['feature', id!] }); addToast('success', 'Execution mode updated'); },
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

  const resumeMutation = useMutation({
    mutationFn: (stageId: string) => resumeStage(id!, stageId),
    onMutate: async (stageId) => {
      setIsProcessing(true);
      await queryClient.cancelQueries({ queryKey: ['stages', id!] });
      const prev = queryClient.getQueryData(['stages', id!]) as any[] ?? [];
      queryClient.setQueryData(['stages', id!], prev.map((s) => s.stage_id === stageId ? { ...s, status: 'in_progress' } : s));
      return { prev };
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['stages', id!] });
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['audit', id!] });
      addToast('success', 'Stage resumed — re-attaching to session');
    },
    onError: (err: Error, _data, ctx) => {
      setIsProcessing(false);
      if (ctx?.prev) queryClient.setQueryData(['stages', id!], ctx.prev);
      addToast('error', `Resume failed: ${err.message}`);
    },
  });

  const forceRerunMutation = useMutation({
    mutationFn: (stageId: string) => forceRerunStage(id!, stageId),
    onMutate: async (stageId) => {
      setIsProcessing(true);
      await queryClient.cancelQueries({ queryKey: ['stages', id!] });
      const prev = queryClient.getQueryData(['stages', id!]) as any[] ?? [];
      queryClient.setQueryData(['stages', id!], prev.map((s) => s.stage_id === stageId ? { ...s, status: 'in_progress' } : s));
      return { prev };
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['stages', id!] });
      queryClient.invalidateQueries({ queryKey: ['feature', id!] });
      queryClient.invalidateQueries({ queryKey: ['audit', id!] });
      addToast('success', 'Stage force re-run — fresh dispatch');
    },
    onError: (err: Error, _data, ctx) => {
      setIsProcessing(false);
      if (ctx?.prev) queryClient.setQueryData(['stages', id!], ctx.prev);
      addToast('error', `Force re-run failed: ${err.message}`);
    },
  });

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

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12" data-testid="feature-loading">
        <div className="animate-spin rounded-full h-6 w-6 border-2 border-t-transparent" style={{ borderColor: 'var(--color-accent)', borderTopColor: 'transparent' }} />
        <span className="ml-3 text-[var(--color-text-tertiary)] text-sm">Loading feature...</span>
      </div>
    );
  }

  if (error || !feature) {
    return (
      <div className="text-center py-12" data-testid="feature-not-found">
        <h2 className="text-lg font-medium text-[var(--color-text-primary)] mb-2">Feature not found</h2>
        <Link to="/" className="text-sm text-[var(--color-accent)] hover:underline">&larr; Back</Link>
      </div>
    );
  }

  const checkTerminal = (f: FeatureDetailType) => f.status === 'done' || f.status === 'cancelled';
  const terminal = checkTerminal(feature);
  const currentScope = feature.scope || 'feature';
  const currentDepth = feature.depth || 'standard';
  const currentTestStrategy = feature.test_strategy || 'standard';
  const currentExecutionMode = feature.execution_mode || '';
  const selectedStage = stages.find((s) => s.stage_id === (selectedStageId ?? feature.current_stage));
  const awaitingStage = stages.find((s) => s.status === 'awaiting_approval');
  const revisingStage = stages.find((s) => s.status === 'revising');
  const nextStage = stages.find((s) => s.status === 'not_started');
  const activeStage = selectedStage ?? awaitingStage ?? revisingStage ?? stages.find((s) => s.stage_id === feature.current_stage);
  const showLadderPrompt = bolts.length > 0 && bolts[0]?.status === 'completed' && !feature.autonomy_mode;
  const isAutonomous = feature.execution_mode === 'autonomous';
  const isGuided = feature.execution_mode === 'guided';

  return (
    <div className="flex flex-col h-full" data-testid="feature-detail-page">
      <div className="mb-3">
        <Link to="/" className="text-sm text-[var(--color-accent)] hover:underline">&larr; Dashboard</Link>
      </div>

      <FeatureHeader feature={feature} sessionsCount={sessions.length} isTerminal={terminal} onModeChange={(m) => executionModeMutation.mutate(m)} />

      {/* Mobile: horizontal stage rail. Desktop: hidden (sidebar in grid below) */}
      <div className="lg:hidden mb-4">
        <MobileStageRail stages={stages} currentStageId={feature.current_stage} onSelect={(id) => setSelectedStage(id)} />
      </div>

      {/* Desktop: sidebar + content grid. Mobile: content only (full width) */}
      <div className="flex flex-col lg:grid lg:grid-cols-[260px_1fr] gap-4 flex-1 min-h-0">
        {/* Desktop sidebar */}
        <div className="hidden lg:block">
          <StageRail stages={stages} currentStageId={feature.current_stage} bolts={bolts} />
        </div>

        <div className="flex-1 min-w-0 overflow-y-auto space-y-4">
          {!terminal && (
            <Card className="p-4" data-testid="current-stage-panel">
              {isAutonomous ? (
                <div className="flex items-center justify-between p-4 rounded-[var(--radius-md)]" style={{ backgroundColor: 'var(--color-surface-hover)' }} data-testid="autonomous-banner">
                  <div className="flex items-center gap-3">
                    <span className="animate-spin rounded-full h-4 w-4 border-2 border-t-transparent" style={{ borderColor: 'var(--color-accent)', borderTopColor: 'transparent' }} />
                    <div>
                      <p className="text-sm font-medium text-[var(--color-text-primary)]">Running autonomously — stage {feature.current_stage || 'starting...'}</p>
                      <p className="text-xs text-[var(--color-text-tertiary)]">Stages auto-run and auto-approve. Change mode in Settings to regain control.</p>
                    </div>
                  </div>
                  <Button variant="danger" size="sm" onClick={() => {
                    if (window.confirm('Stop autonomous execution? You can resume manually or switch to a different mode.')) {
                      executionModeMutation.mutate('human');
                    }
                  }} data-testid="stop-autonomous-button">
                    ⏹ Stop
                  </Button>
                </div>
              ) : isWaitingForHuman ? (
                <div className="p-3 rounded-[var(--radius-md)]" style={{ backgroundColor: 'var(--color-warning-surface)' }} data-testid="waiting-banner">
                  <p className="text-sm" style={{ color: 'var(--color-warning)' }}>Answer the questions below. The pipeline resumes automatically once all are answered.</p>
                </div>
              ) : isProcessing ? (
                <div className="flex items-center justify-between p-3 rounded-[var(--radius-md)]" style={{ backgroundColor: 'var(--color-surface-hover)' }} data-testid="processing-banner">
                  <div className="flex items-center gap-3">
                    <span className="animate-spin rounded-full h-4 w-4 border-2 border-t-transparent" style={{ borderColor: 'var(--color-accent)', borderTopColor: 'transparent' }} />
                    <div>
                      <p className="text-sm font-medium text-[var(--color-text-primary)]">Agent working on stage {feature.current_stage || '...'}</p>
                      <p className="text-xs text-[var(--color-text-tertiary)]">Output appears below in real time</p>
                    </div>
                  </div>
                  {isGuided ? (
                    <span className="text-xs text-[var(--color-text-tertiary)]">Running in guided mode</span>
                  ) : (
                    <Button variant="danger" size="sm" onClick={() => {
                      if (window.confirm('Force re-run this stage? This kills the current agent session and starts fresh.')) {
                        forceRerunMutation.mutate(feature.current_stage || '');
                      }
                    }} data-testid="force-rerun-button">
                      Force Re-run
                    </Button>
                  )}
                </div>
              ) : awaitingStage ? (
                <div className="p-4 rounded-[var(--radius-md)]" style={{ backgroundColor: 'var(--color-warning-surface)', border: '1px solid var(--color-warning)' }} data-testid="awaiting-approval-banner">
                  <p className="text-sm font-semibold mb-1" style={{ color: 'var(--color-warning)' }}>✓ Stage {awaitingStage.stage_id} complete — review needed</p>
                  <p className="text-xs mb-3" style={{ color: 'var(--color-text-secondary)' }}>
                    {isGuided ? 'Phase-end review gate. Review the artifacts and approve to continue.' : 'The agent finished. Review the artifacts below and approve or request changes.'}
                  </p>
                  <Button variant="primary" onClick={() => setSelectedStage(awaitingStage.stage_id)} data-testid="review-gate-button">Review & Approve</Button>
                </div>
              ) : revisingStage ? (
                <div className="p-4 rounded-[var(--radius-md)]" style={{ backgroundColor: 'var(--color-warning-surface)', border: '1px solid var(--color-warning)' }} data-testid="revising-banner">
                  <p className="text-sm font-semibold mb-1" style={{ color: 'var(--color-warning)' }}>⚠ Stage {revisingStage.stage_id} needs attention</p>
                  <p className="text-xs mb-3" style={{ color: 'var(--color-text-secondary)]' }}>
                    This stage was interrupted. Resume to continue where the agent left off, re-run to start fresh, or approve if the artifacts look good.
                  </p>
                  <div className="flex flex-wrap gap-2">
                    <Button variant="primary" size="sm" onClick={() => resumeMutation.mutate(revisingStage.stage_id)} disabled={resumeMutation.isPending} isLoading={resumeMutation.isPending} data-testid="resume-stage-button">
                      ▶ Resume {revisingStage.stage_id}
                    </Button>
                    <Button variant="secondary" size="sm" onClick={() => runStageMutation.mutate(revisingStage.stage_id)} disabled={runStageMutation.isPending} isLoading={runStageMutation.isPending} data-testid="rerun-stage-button">
                      ↻ Re-run {revisingStage.stage_id}
                    </Button>
                    <Button variant="success" size="sm" onClick={() => approveMutation.mutate(revisingStage.stage_id)} disabled={approveMutation.isPending} data-testid="approve-as-is-button">
                      ✓ Approve as-is
                    </Button>
                  </div>
                </div>
              ) : nextStage ? (
                <div data-testid="next-stage-panel">
                  <p className="text-sm text-[var(--color-text-secondary)] mb-1">Next: <strong className="text-[var(--color-text-primary)]">{nextStage.stage_id}{nextStage.name ? ` · ${nextStage.name}` : ''}</strong></p>
                  {nextStage.description && (
                    <p className="text-xs text-[var(--color-text-tertiary)] mb-3">{nextStage.description}</p>
                  )}
                  {(() => {
                    const isBoltStage = nextStage.stage_id.match(/^3\.[1-5]$/);
                    const pendingBolt = bolts.find(b => b.status === 'pending' || b.status === 'in_progress');
                    if (isBoltStage && pendingBolt) {
                      return (
                        <div>
                          <p className="text-xs text-[var(--color-text-secondary)] mb-2">
                            This runs as <strong>Bolt {pendingBolt.bolt_number}</strong> — stages 3.1-3.5 execute together.
                            {pendingBolt.is_walking_skeleton ? ' This is the walking skeleton — the smallest end-to-end slice.' : ''}
                          </p>
                          <Button variant="primary" onClick={() => runBoltMutation.mutate(pendingBolt.bolt_number)} disabled={runBoltMutation.isPending} isLoading={runBoltMutation.isPending} data-testid="run-stage-button">
                            ▶ Run Bolt {pendingBolt.bolt_number}
                          </Button>
                        </div>
                      );
                    }
                    if (isBoltStage && !pendingBolt) {
                      return (
                        <Button variant="primary" onClick={() => prepareBoltsMutation.mutate()} disabled={prepareBoltsMutation.isPending} isLoading={prepareBoltsMutation.isPending} data-testid="prepare-bolts-button">
                          ▶ Prepare Bolts
                        </Button>
                      );
                    }
                    return (
                      <Button variant="primary" onClick={() => runStageMutation.mutate(nextStage.stage_id)} disabled={runStageMutation.isPending} isLoading={runStageMutation.isPending} data-testid="run-stage-button">
                        ▶ Run {nextStage.stage_id}
                      </Button>
                    );
                  })()}
                </div>
              ) : (
                <p className="text-sm text-[var(--color-text-tertiary)]" data-testid="no-next-stage">All stages complete or in progress.</p>
              )}
            </Card>
          )}

          {terminal && (
            <div className="rounded-[var(--radius-lg)] p-4" style={{ backgroundColor: feature.status === 'done' ? 'var(--color-success-surface)' : 'var(--color-danger-surface)' }} data-testid="terminal-banner">
              <h3 className="text-base font-medium" style={{ color: feature.status === 'done' ? 'var(--color-success)' : 'var(--color-danger)' }}>{feature.status === 'done' ? '✓ Feature Complete' : '✗ Feature Cancelled'}</h3>
            </div>
          )}

          {activeStage && (
            <Card className="p-4" data-testid="stage-detail">
              <div className="flex items-center gap-2 mb-3">
                <h3 className="text-base font-medium text-[var(--color-text-primary)]">
                  {activeStage.stage_id}{activeStage.name ? ` · ${activeStage.name}` : ''}
                </h3>
                <Badge color="blue">{STAGE_STATUS_LABELS[activeStage.status] || activeStage.status}</Badge>
                {activeStage.revision_count > 0 && <Badge color="yellow">×{activeStage.revision_count}</Badge>}
              </div>

              {activeStage.description && (
                <p className="text-sm text-[var(--color-text-secondary)] mb-4 leading-relaxed">{activeStage.description}</p>
              )}

              {activeStage.status === 'awaiting_approval' && (
                <GatePanel
                  stageId={activeStage.stage_id}
                  stageName={activeStage.name || activeStage.stage_id}
                  revisionCount={activeStage.revision_count}
                  canAcceptAsIs={activeStage.revision_count >= MAX_REVISIONS}
                  onApprove={() => approveMutation.mutate(activeStage.stage_id)}
                  onReject={(notes) => rejectMutation.mutate({ stageId: activeStage.stage_id, notes })}
                  onAcceptAsIs={() => acceptAsIsMutation.mutate(activeStage.stage_id)}
                />
              )}

              {/* Always show agent output for the active stage — history for completed stages, live for in-progress */}
              {activeStage && (
                <AgentOutputLive featureId={feature.id} stageId={activeStage.stage_id} isProcessing={isProcessing || feature.is_processing} phase={feature.current_phase || undefined} />
              )}
            </Card>
          )}

          {feature.current_phase === 'construction' && bolts.length > 0 && (
            <div className="rounded-[var(--radius-lg)] p-4" style={{ backgroundColor: 'var(--color-surface-raised)', boxShadow: 'var(--shadow-sm)' }} data-testid="bolt-progress">
              <h3 className="text-base font-medium text-[var(--color-text-primary)] mb-1">Bolts</h3>
              <p className="text-xs text-[var(--color-text-tertiary)] mb-3">Each bolt runs stages 3.1-3.5 together. Review the gate after each bolt completes.</p>
              <div className="space-y-1.5">
                {bolts.map((b) => {
                  const boltStages = stages.filter(s => s.stage_id.match(/^3\.[1-5]$/));
                  return (
                    <div key={b.bolt_number} className="flex items-center gap-3 px-3 py-2 rounded-[var(--radius-md)]" style={{ backgroundColor: 'var(--color-surface)' }} data-testid={`bolt-progress-${b.bolt_number}`}>
                      <span className="text-sm font-medium text-[var(--color-text-primary)]">Bolt {b.bolt_number}</span>
                      {b.is_walking_skeleton && <span className="text-xs px-1.5 py-0.5 rounded" style={{ backgroundColor: 'var(--color-accent)', color: 'white' }}>skeleton</span>}
                      <div className="flex gap-1">
                        {boltStages.map(s => (
                          <span key={s.stage_id} className="text-[10px] font-mono" style={{ color: statusColorForStatus(s.status) }} title={`${s.stage_id} ${s.name || ''}`}>
                            {STATUS_ICONS_FOR_STATUS(s.status)}
                          </span>
                        ))}
                      </div>
                      <span className={`text-xs ${b.status === 'completed' ? 'text-[var(--color-success)]' : b.status === 'in_progress' ? 'text-[var(--color-accent)]' : b.status === 'failed' ? 'text-[var(--color-danger)]' : 'text-[var(--color-text-tertiary)]'}`}>
                        {b.status}
                      </span>
                      {b.unit_ids && b.unit_ids.length > 0 && (
                        <span className="text-xs text-[var(--color-text-tertiary)] ml-auto truncate">{b.unit_ids.join(', ')}</span>
                      )}
                    </div>
                  );
                })}
              </div>
              {showLadderPrompt && (
                <div className="mt-3 p-3 rounded-[var(--radius-md)]" style={{ backgroundColor: 'var(--color-warning-surface)' }}>
                  <p className="text-xs font-medium mb-2" style={{ color: 'var(--color-warning)' }}>Walking skeleton complete — choose mode for remaining bolts:</p>
                  <div className="flex gap-2">
                    <Button variant="secondary" size="sm" onClick={() => ladderMutation.mutate('gated')} data-testid="ladder-gated">Gated (review each)</Button>
                    <Button variant="primary" size="sm" onClick={() => ladderMutation.mutate('autonomous')} data-testid="ladder-autonomous">Autonomous (skip gates)</Button>
                  </div>
                </div>
              )}
              {feature.autonomy_mode && (
                <p className="mt-2 text-xs text-[var(--color-text-tertiary)]">Mode: <span style={{ color: feature.autonomy_mode === 'autonomous' ? 'var(--color-accent)' : 'var(--color-warning)' }}>{feature.autonomy_mode}</span></p>
              )}
            </div>
          )}

          {rules.length > 0 && (
            <Card className="p-4" data-testid="rules-panel">
              <h3 className="text-base font-medium text-[var(--color-text-primary)] mb-3">Learned Rules ({rules.length})</h3>
              <div className="space-y-2" data-testid="rules-list">
                {rules.map((r) => (
                  <div key={r.id} className="p-3 rounded-[var(--radius-md)]" style={{ backgroundColor: 'var(--color-warning-surface)' }} data-testid={`rule-${r.id}`}>
                    <span className="text-xs" style={{ color: 'var(--color-warning)' }}>{AGENT_LABELS[r.agent_name] || r.agent_name} · {r.stage_id || 'global'}</span>
                    <p className="text-sm text-[var(--color-text-primary)] mt-1">{r.rule_text}</p>
                  </div>
                ))}
              </div>
            </Card>
          )}

          {/* Questions — only show for the active stage or if there are pending questions */}
          {(() => {
            const stageQuestions = questions.filter(q =>
              q.status === 'pending' ||
              (activeStage && q.stage_id === activeStage.stage_id)
            );
            if (stageQuestions.length === 0) return null;
            return (
              <QuestionPanel
                questions={stageQuestions}
                drafts={draft}
                onSelect={onSelect}
                onType={onType}
                onSubmitAll={handleSubmitAll}
                isSubmitting={isSubmitting}
                allDrafted={allPendingDrafted}
              />
            );
          })()}

          <Card className="p-4" data-testid="artifacts-panel">
            <h3 className="text-base font-medium text-[var(--color-text-primary)] mb-1">
              Artifacts {activeStage?.name ? `for ${activeStage.stage_id} — ${activeStage.name}` : ''}
            </h3>
            <p className="text-xs text-[var(--color-text-tertiary)] mb-3">
              {activeStage?.key_artifacts && activeStage.key_artifacts.length > 0
                ? `Expected: ${activeStage.key_artifacts.join(', ')}`
                : 'No artifacts expected for this stage.'}
            </p>
            <ArtifactViewer
              featureId={feature.id}
              phaseStates={{}}
              stageId={activeStage?.stage_id}
              keyArtifacts={activeStage?.key_artifacts}
              refreshKey={artifactRefreshKey}
            />
          </Card>
        </div>
      </div>

      <div className="mt-3">
        <ControlBar
          onJumpStage={(stageId) => jumpMutation.mutate({ stageId })}
          onJumpPhase={(phase) => jumpMutation.mutate({ phase })}
          onSetScope={(s) => scopeMutation.mutate(s)}
          onSetDepth={(d) => depthMutation.mutate(d)}
          onSetTestStrategy={(t) => testStrategyMutation.mutate(t)}
          onSetExecutionMode={(m) => executionModeMutation.mutate(m)}
          onCancel={() => cancelMutation.mutate()}
          currentScope={currentScope}
          currentDepth={currentDepth}
          currentTestStrategy={currentTestStrategy}
          currentExecutionMode={currentExecutionMode}
          availableStages={stages}
          isTerminal={terminal}
        />
      </div>

      <AuditDrawer open={auditDrawerOpen} onClose={() => toggleAuditDrawer()} events={auditEvents} />
    </div>
  );
}