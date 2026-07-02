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
        <p className="text-sm text-[var(--color-text-tertiary)]" data-testid="audit-empty">No events yet.</p>
      ) : (
        <div className="space-y-3" data-testid="audit-event-list">
          {displayed.map((e) => (
            <div key={e.id} className="flex items-start gap-3 pb-3 border-b border-[var(--color-border-subtle)] last:border-0 last:pb-0" data-testid={`audit-event-${e.id}`}>
              <span className="text-base shrink-0">{EVENT_ICONS[e.event_type] || '•'}</span>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-sm font-medium text-[var(--color-text-primary)]">{e.event_type}</span>
                  {e.stage_id && <span className="text-[10px] px-1.5 py-0.5 rounded-[var(--radius-sm)] text-[var(--color-text-secondary)]" style={{ backgroundColor: 'var(--color-surface-active)' }}>{e.stage_id}</span>}
                  {e.phase && <span className="text-[10px] px-1.5 py-0.5 rounded-[var(--radius-sm)]" style={{ backgroundColor: 'var(--color-accent)', color: '#fff' }}>{e.phase}</span>}
                </div>
                {e.details && <p className="text-xs text-[var(--color-text-secondary)] mt-1 truncate">{e.details}</p>}
                <p className="text-xs text-[var(--color-text-tertiary)] mt-1">{new Date(e.created_at).toLocaleString()}</p>
              </div>
            </div>
          ))}
        </div>
      )}
    </Drawer>
  );
}