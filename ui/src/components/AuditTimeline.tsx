import { useState } from 'react';
import type { AuditEvent } from '../types';

interface AuditTimelineProps {
  events: AuditEvent[];
}

const EVENT_ICONS: Record<string, string> = {
  WORKFLOW_START: '🚀',
  WORKFLOW_COMPLETE: '🏁',
  STAGE_START: '▶',
  STAGE_AWAITING_APPROVAL: '❓',
  STAGE_REVISING: '🔄',
  STAGE_COMPLETED: '✓',
  STAGE_SKIPPED: '⏭',
  STAGE_ADVANCED: '→',
  GATE_APPROVED: '✓',
  GATE_REJECTED: '✗',
  GATE_ACCEPT_AS_IS: '⚠',
  RULE_LEARNED: '📚',
  BOLT_STARTED: '🔨',
  BOLT_COMPLETED: '✓',
  BOLT_FAILED: '✗',
  LADDER_PROMPT: '🪜',
  SUBAGENT_COMPLETED: '🔎',
  JUMP_TO_STAGE: '⇒',
  HALT_AND_ASK: '⚠',
  ERROR_STAGE: '⚠',
  // Chat (AIDLC Expert Agent and Chat UI) — expert-driven CLI ops.
  CHAT_CLI_EXEC: '💬',
};

export default function AuditTimeline({ events }: AuditTimelineProps) {
  const [expanded, setExpanded] = useState(false);
  const displayed = expanded ? events : events.slice(-20).reverse();

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6" data-testid="audit-timeline">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Audit Trail</h3>
        <span className="text-sm text-gray-500 dark:text-gray-400" data-testid="audit-count">
          {events.length} events
        </span>
      </div>

      {events.length === 0 ? (
        <p className="text-sm text-gray-500 dark:text-gray-400" data-testid="no-audit-events">
          No events yet. Events appear as stages run.
        </p>
      ) : (
        <div className="space-y-2" data-testid="audit-event-list">
          {displayed.map((e) => {
            const icon = EVENT_ICONS[e.event_type] || '•';
            return (
              <div
                key={e.id}
                className="flex items-start gap-3 py-2 border-b border-gray-100 dark:border-gray-700 last:border-0"
                data-testid={`audit-event-${e.id}`}
              >
                <span className="text-lg shrink-0" data-testid={`audit-icon-${e.id}`}>{icon}</span>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="text-sm font-medium text-gray-900 dark:text-white" data-testid={`audit-type-${e.id}`}>
                      {e.event_type}
                    </span>
                    {e.stage_id && (
                      <span className="text-xs px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300" data-testid={`audit-stage-${e.id}`}>
                        {e.stage_id}
                      </span>
                    )}
                    {e.phase && (
                      <span className="text-xs px-1.5 py-0.5 rounded bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-200" data-testid={`audit-phase-${e.id}`}>
                        {e.phase}
                      </span>
                    )}
                  </div>
                  {e.details && (
                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5 truncate" data-testid={`audit-details-${e.id}`}>
                      {e.details}
                    </p>
                  )}
                  <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5" data-testid={`audit-time-${e.id}`}>
                    {new Date(e.created_at).toLocaleString()}
                  </p>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {events.length > 20 && !expanded && (
        <button
          onClick={() => setExpanded(true)}
          className="mt-3 text-sm text-blue-600 dark:text-blue-400 hover:underline"
          data-testid="audit-show-all"
        >
          Show all {events.length} events
        </button>
      )}
      {expanded && (
        <button
          onClick={() => setExpanded(false)}
          className="mt-3 text-sm text-blue-600 dark:text-blue-400 hover:underline"
          data-testid="audit-show-recent"
        >
          Show recent only
        </button>
      )}
    </div>
  );
}