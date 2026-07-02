import { useState } from 'react';
import { Button, Badge, Card } from '../ui/primitives';

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

export default function GatePanel({
  stageId, stageName, revisionCount, canAcceptAsIs,
  smokeFailures = [], reviewerVerdict, reviewerNotes,
  onApprove, onReject, onAcceptAsIs,
}: GatePanelProps) {
  const [showReject, setShowReject] = useState(false);
  const [rejectNotes, setRejectNotes] = useState('');

  return (
    <Card className="p-4 mb-4 border-2 border-yellow-300 dark:border-yellow-700" data-testid="gate-panel">
      <div className="flex items-center justify-between mb-3">
        <div>
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white" data-testid="gate-title">
            Stage {stageId}: {stageName}
          </h3>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            {revisionCount > 0 ? `${revisionCount} revision${revisionCount > 1 ? 's' : ''} so far` : 'First review — awaiting your approval'}
          </p>
        </div>
        <Badge color="yellow" data-testid="gate-status-badge">Awaiting Approval</Badge>
      </div>

      {reviewerVerdict === 'NOT-READY' && reviewerNotes && (
        <div className="mb-3 p-3 bg-orange-50 dark:bg-orange-900/20 border border-orange-200 dark:border-orange-800 rounded-lg" data-testid="gate-reviewer-rejection">
          <p className="text-sm font-medium text-orange-800 dark:text-orange-200 mb-1">Reviewer: NOT-READY</p>
          <p className="text-sm text-orange-700 dark:text-orange-300 whitespace-pre-wrap">{reviewerNotes}</p>
        </div>
      )}

      {reviewerVerdict === 'READY' && (
        <div className="mb-3 p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg" data-testid="gate-reviewer-approved">
          <p className="text-sm font-medium text-green-800 dark:text-green-200">✓ Reviewer: READY</p>
          {reviewerNotes && <p className="text-sm text-green-700 dark:text-green-300 mt-1">{reviewerNotes}</p>}
        </div>
      )}

      {smokeFailures.length > 0 && (
        <div className="mb-3 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg" data-testid="gate-smoke-failures">
          <p className="text-sm font-medium text-red-800 dark:text-red-200 mb-1">Quality check issues:</p>
          <ul className="list-disc list-inside text-sm text-red-700 dark:text-red-300 space-y-1">
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
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
            What needs to change?
          </label>
          <textarea
            value={rejectNotes}
            onChange={(e) => setRejectNotes(e.target.value)}
            rows={4}
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-orange-500"
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
    </Card>
  );
}