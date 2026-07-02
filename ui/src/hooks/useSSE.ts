import { useEffect, useRef, useCallback, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import type { SSEEventType } from '../types';

interface UseSSEReturn {
  connected: boolean;
  lastEvent: SSEEvent | null;
  subscribe: (type: SSEEventType | 'state_change', handler: (event: SSEEvent) => void) => () => void;
}

export interface SSEEvent {
  type: SSEEventType | 'state_change';
  data: unknown;
}

type EventHandler = (event: SSEEvent) => void;

export function useSSE(featureId: string | null, onEvent?: (event: SSEEvent) => void): UseSSEReturn {
  const [connected, setConnected] = useState(false);
  const [lastEvent, setLastEvent] = useState<SSEEvent | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);
  const reconnectAttempts = useRef(0);
  const queryClient = useQueryClient();
  const onEventRef = useRef(onEvent);
  onEventRef.current = onEvent;
  const subscribersRef = useRef<Map<string, Set<EventHandler>>>(new Map());

  const handleEvent = useCallback((type: SSEEventType | 'state_change', event: MessageEvent) => {
    try {
      const data = JSON.parse(event.data as string);
      const sseEvent = { type, data };
      setLastEvent(sseEvent);
      if (onEventRef.current) onEventRef.current(sseEvent);

      // Notify subscribers for this event type
      const subs = subscribersRef.current.get(type);
      if (subs) {
        subs.forEach((handler) => handler(sseEvent));
      }

      // Invalidate relevant queries
      if (data && typeof data === 'object' && 'feature_id' in data) {
        queryClient.invalidateQueries({ queryKey: ['feature', data.feature_id as string] });
        queryClient.invalidateQueries({ queryKey: ['questions', data.feature_id as string] });
        queryClient.invalidateQueries({ queryKey: ['stages', data.feature_id as string] });
        queryClient.invalidateQueries({ queryKey: ['audit', data.feature_id as string] });
      }
      queryClient.invalidateQueries({ queryKey: ['features'] });
    } catch {
      // Ignore parse errors
    }
  }, [queryClient]);

  const subscribe = useCallback((type: SSEEventType | 'state_change', handler: EventHandler) => {
    if (!subscribersRef.current.has(type)) {
      subscribersRef.current.set(type, new Set());
    }
    subscribersRef.current.get(type)!.add(handler);
    return () => {
      subscribersRef.current.get(type)?.delete(handler);
    };
  }, []);

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
      'stage_started', 'stage_awaiting_approval', 'stage_revising', 'stage_completed',
      'gate_approved', 'gate_rejected', 'gate_result',
      'agent_dispatch', 'agent_complete', 'agent_output',
      'processing_complete', 'error', 'interrupted',
      'waiting_for_feedback', 'question_answered',
      'session_state_change', 'state_change',
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

  return { connected, lastEvent, subscribe };
}