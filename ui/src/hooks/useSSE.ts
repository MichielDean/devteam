import { useEffect, useRef, useCallback, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import type {
  SSEEventType,
} from '../types';

interface UseSSEReturn {
  connected: boolean;
  lastEvent: SSEEvent | null;
}

export interface SSEEvent {
  type: SSEEventType | 'state_change';
  data: unknown;
}

export function useSSE(featureId: string | null, onEvent?: (event: SSEEvent) => void): UseSSEReturn {
  const [connected, setConnected] = useState(false);
  const [lastEvent, setLastEvent] = useState<SSEEvent | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);
  const reconnectAttempts = useRef(0);
  const queryClient = useQueryClient();
  const onEventRef = useRef(onEvent);
  onEventRef.current = onEvent;

  const handleEvent = useCallback((type: SSEEventType | 'state_change', event: MessageEvent) => {
    try {
      const data = JSON.parse(event.data as string);
      const sseEvent = { type, data };
      setLastEvent(sseEvent);
      if (onEventRef.current) {
        onEventRef.current(sseEvent);
      }

      // Invalidate React Query cache for the relevant feature
      if (data?.feature_id) {
        queryClient.invalidateQueries({ queryKey: ['feature', data.feature_id] });
        queryClient.invalidateQueries({ queryKey: ['questions', data.feature_id] });
      }
      // Always invalidate the feature list when any state changes
      queryClient.invalidateQueries({ queryKey: ['features'] });
      // Invalidate questions cache for question-related events
      if (type === 'waiting_for_feedback' || type === 'questions_answered' || type === 'questions_assumed') {
        if (data?.feature_id) {
          queryClient.invalidateQueries({ queryKey: ['questions', data.feature_id] });
        }
      }
    } catch {
      // Ignore parse errors
    }
  }, [queryClient]);

  const connect = useCallback(() => {
    if (!featureId) return;

    const url = `/api/features/${featureId}/stream`;
    const es = new EventSource(url);
    eventSourceRef.current = es;

    es.onopen = () => {
      setConnected(true);
      reconnectAttempts.current = 0;
    };

    es.onerror = () => {
      setConnected(false);
      es.close();
      eventSourceRef.current = null;

      // Exponential backoff: 1s, 2s, 4s, 8s, max 30s
      const delay = Math.min(1000 * Math.pow(2, reconnectAttempts.current), 30000);
      reconnectAttempts.current++;
      reconnectTimeoutRef.current = window.setTimeout(connect, delay);
    };

    // Handle all event types
    es.addEventListener('phase_change', (e: MessageEvent) => handleEvent('phase_change', e));
    es.addEventListener('gate_result', (e: MessageEvent) => handleEvent('gate_result', e));
    es.addEventListener('agent_dispatch', (e: MessageEvent) => handleEvent('agent_dispatch', e));
    es.addEventListener('agent_complete', (e: MessageEvent) => handleEvent('agent_complete', e));
    es.addEventListener('agent_output', (e: MessageEvent) => handleEvent('agent_output', e));
    es.addEventListener('processing_complete', (e: MessageEvent) => handleEvent('processing_complete', e));
    es.addEventListener('phase_complete', (e: MessageEvent) => handleEvent('phase_complete', e));
    es.addEventListener('error', (e: MessageEvent) => handleEvent('error', e));
    es.addEventListener('state_change', (e: MessageEvent) => handleEvent('state_change', e));
    es.addEventListener('waiting_for_feedback', (e: MessageEvent) => handleEvent('waiting_for_feedback', e));
    es.addEventListener('questions_answered', (e: MessageEvent) => handleEvent('questions_answered', e));
    es.addEventListener('questions_assumed', (e: MessageEvent) => handleEvent('questions_assumed', e));
    es.addEventListener('question_answered', (e: MessageEvent) => handleEvent('question_answered', e));
  }, [featureId, handleEvent]);

  useEffect(() => {
    connect();

    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
      if (reconnectTimeoutRef.current) {
        window.clearTimeout(reconnectTimeoutRef.current);
      }
    };
  }, [connect]);

  return { connected, lastEvent };
}