import { useEffect, useRef, useCallback, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import type { SSEEventType } from '../types';

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
      if (onEventRef.current) onEventRef.current(sseEvent);

      if (data?.feature_id) {
        queryClient.invalidateQueries({ queryKey: ['feature', data.feature_id] });
        queryClient.invalidateQueries({ queryKey: ['questions', data.feature_id] });
        queryClient.invalidateQueries({ queryKey: ['stages', data.feature_id] });
        queryClient.invalidateQueries({ queryKey: ['audit', data.feature_id] });
      }
      queryClient.invalidateQueries({ queryKey: ['features'] });
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
      const delay = Math.min(1000 * Math.pow(2, reconnectAttempts.current), 30000);
      reconnectAttempts.current++;
      reconnectTimeoutRef.current = window.setTimeout(connect, delay);
    };

    const eventTypes: (SSEEventType | 'state_change')[] = [
      'stage_change', 'gate_result', 'agent_dispatch', 'agent_complete',
      'agent_output', 'processing_complete', 'error', 'state_change',
      'waiting_for_feedback', 'question_answered',
    ];
    for (const type of eventTypes) {
      es.addEventListener(type, (e: MessageEvent) => handleEvent(type, e));
    }
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