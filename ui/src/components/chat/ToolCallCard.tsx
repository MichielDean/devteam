import { useState } from 'react';
import type { ChatStreamChunk } from '../../types';

interface ToolCallCardProps {
  chunk: ChatStreamChunk;
  onConfirm: (proposalId: string, approved: boolean) => void;
  isConfirming: boolean;
}

// ToolCallCard renders a proposed CLI op from the expert (interaction-spec
// S7/S8/S9). Mutating ops show Approve/Cancel; destructive ops require
// typed-confirm naming the consequence (SC9). The exact command is shown
// with no truncation (NFR-USA-2).
export function ToolCallCard({ chunk, onConfirm, isConfirming }: ToolCallCardProps) {
  const isDestructive = chunk.classification === 'destructive';
  const [typedConfirm, setTypedConfirm] = useState('');
  const [decision, setDecision] = useState<'approved' | 'rejected' | null>(null);

  // For destructive ops, the confirm button is disabled until the user types
  // the consequence-acknowledgement. SC9: the dialog names the consequence.
  const canConfirmDestructive = !isDestructive || typedConfirm.length > 0;

  if (decision) {
    return (
      <div
        className="rounded-[var(--radius-md)] p-3 text-sm"
        style={{
          backgroundColor: 'var(--color-surface-hover)',
          border: '1px solid var(--color-border-subtle)',
        }}
        data-testid="chat-toolcall-resolved"
      >
        <div className="font-mono text-xs mb-1" style={{ color: 'var(--color-text-tertiary)' }}>
          {chunk.command}
        </div>
        <div style={{ color: 'var(--color-text-secondary)' }}>
          {decision === 'approved' ? '✓ Approved — executing…' : '✗ Rejected'}
        </div>
      </div>
    );
  }

  return (
    <div
      className="rounded-[var(--radius-md)] p-3"
      style={{
        backgroundColor: 'var(--color-surface-raised)',
        border: `1px solid ${isDestructive ? 'var(--color-border-error)' : 'var(--color-border-subtle)'}`,
      }}
      data-testid="chat-toolcall-card"
    >
      <div className="flex items-center justify-between mb-2">
        <span
          className="text-xs px-2 py-0.5 rounded-full font-semibold"
          style={{
            backgroundColor: isDestructive ? 'var(--color-surface-error)' : 'var(--color-surface-hover)',
            color: isDestructive ? 'var(--color-text-error)' : 'var(--color-text-secondary)',
          }}
        >
          {isDestructive ? 'DESTRUCTIVE' : chunk.classification?.toUpperCase() || 'CLI OP'}
        </span>
        <span className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>
          Expert proposes:
        </span>
      </div>
      <pre
        className="font-mono text-sm whitespace-pre-wrap break-all mb-3 p-2 rounded"
        style={{ backgroundColor: 'var(--color-surface)', color: 'var(--color-text-primary)' }}
        data-testid="chat-toolcall-command"
      >
        $ devteam {chunk.command}
      </pre>
      {isDestructive && chunk.consequence && (
        <div
          className="text-sm mb-3 p-2 rounded"
          style={{ backgroundColor: 'var(--color-surface-error)', color: 'var(--color-text-error)' }}
          data-testid="chat-toolcall-consequence"
        >
          ⚠ {chunk.consequence}
        </div>
      )}
      {isDestructive && (
        <input
          type="text"
          value={typedConfirm}
          onChange={(e) => setTypedConfirm(e.target.value)}
          placeholder="Type to acknowledge the consequence…"
          className="w-full mb-3 px-2 py-1 text-sm rounded"
          style={{
            backgroundColor: 'var(--color-surface)',
            color: 'var(--color-text-primary)',
            border: '1px solid var(--color-border-subtle)',
          }}
          data-testid="chat-toolcall-typed-confirm"
        />
      )}
      {chunk.needs_confirm && (
        <div className="flex gap-2 justify-end">
          <button
            type="button"
            onClick={() => {
              setDecision('rejected');
              onConfirm(chunk.proposal_id!, false);
            }}
            disabled={isConfirming}
            className="px-3 py-1.5 text-sm rounded-[var(--radius-md)]"
            style={{
              color: 'var(--color-text-primary)',
              border: '1px solid var(--color-border-subtle)',
            }}
            data-testid="chat-toolcall-reject"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={() => {
              setDecision('approved');
              onConfirm(chunk.proposal_id!, true);
            }}
            disabled={isConfirming || !canConfirmDestructive}
            className="px-3 py-1.5 text-sm rounded-[var(--radius-md)] font-semibold"
            style={{
              backgroundColor: isDestructive ? 'var(--color-text-error)' : 'var(--color-accent)',
              color: '#fff',
              opacity: canConfirmDestructive ? 1 : 0.5,
            }}
            data-testid="chat-toolcall-approve"
          >
            {isDestructive ? 'Confirm (destructive)' : 'Approve'}
          </button>
        </div>
      )}
    </div>
  );
}