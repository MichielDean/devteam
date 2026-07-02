import { useState } from 'react';
import { Button, Badge } from '../ui/primitives';

interface GatePanelProps {
  stageId: string;
  stageName: string;
  revisionCount: number;
  canAcceptAsIs: boolean;
  smokeFailures?: string[];
  reviewerVerdict?: string;
  reviewerNotes?: string;
  onApprove: () => void;
  onReject: (notes: string) => void;
  onAcceptAsIs: () => void;
}

const notesInputClass =
  'w-full px-3 py-2 rounded-[var(--radius-md)] bg-[var(--color-surface)] text-[var(--color-text-primary)] border border-[var(--color-border-subtle)] focus:border-[var(--color-accent)] focus:outline-none text-sm';

export default function GatePanel({
  stageId, stageName, revisionCount, canAcceptAsIs,
  smokeFailures = [], reviewerVerdict, reviewerNotes,
  onApprove, onReject, onAcceptAsIs,
}: GatePanelProps) {
  const [showReject, setShowReject] = useState(false);
  const [rejectNotes, setRejectNotes] = useState('');

  return (
    <div
      className="p-4 mb-4 rounded-[var(--radius-lg)]"
      style={{ backgroundColor: 'var(--color-surface-raised)', boxShadow: 'var(--shadow-md)', borderLeft: '3px solid var(--color-warning)' }}
      data-testid="gate-panel"
    >
      <div className="flex items-center justify-between mb-3">
        <div>
          <h3 className="text-base font-medium text-[var(--color-text-primary)]" data-testid="gate-title">
            Stage {stageId}: {stageName}
          </h3>
          <p className="text-xs text-[var(--color-text-secondary)] mt-0.5">
            {revisionCount > 0 ? `${revisionCount} revision${revisionCount > 1 ? 's' : ''} so far` : 'First review — awaiting your approval'}
          </p>
        </div>
        <Badge color="yellow" data-testid="gate-status-badge">Awaiting Approval</Badge>
      </div>

      {reviewerVerdict === 'NOT-READY' && reviewerNotes && (
        <div className="mb-3 p-3 rounded-[var(--radius-md)]" style={{ backgroundColor: 'var(--color-danger-surface)' }} data-testid="gate-reviewer-rejection">
          <p className="text-sm font-medium mb-1" style={{ color: 'var(--color-danger)' }}>Reviewer: NOT-READY</p>
          <p className="text-sm whitespace-pre-wrap" style={{ color: 'var(--color-danger)' }}>{reviewerNotes}</p>
        </div>
      )}

      {reviewerVerdict === 'READY' && (
        <div className="mb-3 p-3 rounded-[var(--radius-md)]" style={{ backgroundColor: 'var(--color-success-surface)' }} data-testid="gate-reviewer-approved">
          <p className="text-sm font-medium" style={{ color: 'var(--color-success)' }}>✓ Reviewer: READY</p>
          {reviewerNotes && <p className="text-sm mt-1" style={{ color: 'var(--color-success)' }}>{reviewerNotes}</p>}
        </div>
      )}

      {smokeFailures.length > 0 && (
        <div className="mb-3 p-3 rounded-[var(--radius-md)]" style={{ backgroundColor: 'var(--color-danger-surface)' }} data-testid="gate-smoke-failures">
          <p className="text-sm font-medium mb-1" style={{ color: 'var(--color-danger)' }}>Quality check issues:</p>
          <ul className="list-disc list-inside text-sm space-y-1" style={{ color: 'var(--color-danger)' }}>
            {smokeFailures.map((f, i) => <li key={i}>{f}</li>)}
          </ul>
        </div>
      )}

      {!showReject ? (
        <div className="flex flex-col gap-2">
          <Button variant="success" onClick={onApprove} data-testid="gate-approve-button">
            ✓ Approve — move to next stage
          </Button>
          <Button variant="warning" onClick={() => setShowReject(true)} data-testid="gate-reject-button">
            ✗ Request changes — send back for revision
          </Button>
          {canAcceptAsIs && (
            <Button variant="secondary" onClick={onAcceptAsIs} data-testid="gate-accept-as-is-button">
              Accept as-is (escape hatch — {revisionCount} revisions)
            </Button>
          )}
        </div>
      ) : (
        <div className="space-y-3" data-testid="gate-reject-form">
          <label className="block text-sm font-medium text-[var(--color-text-secondary)]">
            What needs to change?
          </label>
          <textarea
            value={rejectNotes}
            onChange={(e) => setRejectNotes(e.target.value)}
            rows={4}
            className={`${notesInputClass} resize-y`}
            placeholder="Describe what needs fixing. This will be saved as a rule for the learning loop."
            data-testid="gate-reject-notes"
          />
          <div className="flex gap-2">
            <Button variant="warning" onClick={() => { onReject(rejectNotes); }} disabled={!rejectNotes.trim()} data-testid="gate-reject-submit">
              Send back for revision
            </Button>
            <Button variant="ghost" onClick={() => setShowReject(false)} data-testid="gate-reject-cancel">
              Back
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}