import { useState, useEffect, useRef, useCallback } from 'react';
import { API_BASE } from '../api/client';
import { useSSE } from './useSSE';

// useOutputStream — the single shared hook for agent output consumption
// (U-UI-10). Owns: one-shot GET /log/{stageId} history read on mount; agent_output
// SSE subscription when isProcessing; append chunks to content state.
//
// Bolt 1 slice: one-shot GET + SSE subscribe + append. The full hook (B5) adds
// reconcile-on-reconnect (FR-17) and the `interrupted` SSE event handler
// (product-review S-3). The footer (U-UI-11) lands in B5.
//
// The read endpoint now returns JSON {content, source} (U-BK-07 / arch-review
// N-1). This hook reads res.json() and uses .content. The source field is
// surfaced for the future footer; the slice does not render it.
//
// No log-polling setInterval — the only residual timer is the flushedAgoMs
// ticker (added in B5). The SSE event `agent_output` is the live transport.

interface UseOutputStreamArgs {
  featureId: string;
  stageId?: string;
  boltNumber?: number;
  isProcessing: boolean;
}

interface UseOutputStreamReturn {
  content: string;
  source: 'db' | 'file-legacy' | '';
  connected: boolean;
}

interface StageLogResponse {
  content: string;
  source: 'db' | 'file-legacy';
}

export function useOutputStream({ featureId, stageId, boltNumber, isProcessing }: UseOutputStreamArgs): UseOutputStreamReturn {
  const [content, setContent] = useState('');
  const [source, setSource] = useState<'db' | 'file-legacy' | ''>('');
  const contentRef = useRef('');
  contentRef.current = content;

  // One-shot history read on mount and when stageId/bolt changes (U-BK-07 read path).
  useEffect(() => {
    if (!stageId) return;
    let cancelled = false;
    const fetchLog = async () => {
      try {
        const boltParam = boltNumber && boltNumber > 0 ? `?bolt=${boltNumber}` : '';
        const res = await fetch(`${API_BASE}/features/${featureId}/log/${stageId}${boltParam}`);
        if (!res.ok) return;
        // JSON response: {content, source} — arch-review N-1 content-type flip.
        const data: StageLogResponse = await res.json();
        if (cancelled) return;
        setContent(data.content || '');
        setSource(data.source || 'db');
      } catch {
        // ignore — SSE will fill in live content
      }
    };
    fetchLog();
    return () => { cancelled = true; };
  }, [featureId, stageId, boltNumber]);

  // SSE subscription for live chunks while processing (U-UI-10 slice).
  const handleAgentOutput = useCallback((event: { type: string; data: unknown }) => {
    if (event.type !== 'agent_output') return;
    const data = event.data as { feature_id?: string; stage_id?: string; line?: string; content?: string };
    // Filter to this stage (the SSE stream is per-feature; multiple stages
    // may emit. Match on stage_id if present, else append — the UI's mount
    // scope guarantees only the active stage is mounted).
    if (data.stage_id && stageId && data.stage_id !== stageId) return;
    const chunk = data.line ?? data.content ?? '';
    if (!chunk) return;
    setContent((prev) => prev + chunk);
  }, [stageId]);

  useSSE(isProcessing ? featureId : null, handleAgentOutput);

  return { content, source, connected: true };
}