import { Drawer } from '../ui/primitives';
import type { AuditEvent } from '../types';

const EVENT_ICONS: Record<string, string> = {
  WORKFLOW_START: '🚀', WORKFLOW_COMPLETE: '🏁',
  STAGE_START: '▶', STAGE_AWAITING_APPROVAL: '❓', STAGE_REVISING: '🔄',
  STAGE_COMPLETED: '✓', STAGE_SKIPPED: '⏭', STAGE_ADVANCED: '→',
  GATE_APPROVED: '✓', GATE_REJECTED: '✗', GATE_ACCEPT_AS_IS: '⚠',
  RULE_LEARNED: '📚', BOLT_STARTED: '🔨', BOLT_COMPLETED: '✓', BOLT_FAILED: '✗',
  LADDER_PROMPT: '🪜', SUBAGENT_COMPLETED: '🔎', JUMP_TO_STAGE: '⇒', HALT_AND_ASK: '⚠',
};

interface AuditDrawerProps {
  open: boolean;
  onClose: () => void;
  events: AuditEvent[];
}

export default function AuditDrawer({ open, onClose, events }: AuditDrawerProps) {
  const displayed = events.slice().reverse();

  return (
    <Drawer open={open} onClose={onClose} title={`Audit Trail (${events.length} events)`} width="450px" data-testid="audit-drawer">
      {events.length === 0 ? (
        <p className="text-sm text-gray-500" data-testid="audit-empty">No events yet.</p>
      ) : (
        <div className="space-y-2" data-testid="audit-event-list">
          {displayed.map((e) => (
            <div key={e.id} className="flex items-start gap-3 py-2 border-b border-gray-100 dark:border-gray-700 last:border-0" data-testid={`audit-event-${e.id}`}>
              <span className="text-lg shrink-0">{EVENT_ICONS[e.event_type] || '•'}</span>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-sm font-medium text-gray-900 dark:text-white">{e.event_type}</span>
                  {e.stage_id && <span className="text-xs px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300">{e.stage_id}</span>}
                  {e.phase && <span className="text-xs px-1.5 py-0.5 rounded bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-200">{e.phase}</span>}
                </div>
                {e.details && <p className="text-xs text-gray-500 mt-0.5 truncate">{e.details}</p>}
                <p className="text-xs text-gray-400 mt-0.5">{new Date(e.created_at).toLocaleString()}</p>
              </div>
            </div>
          ))}
        </div>
      )}
    </Drawer>
  );
}