import { useState } from 'react';

interface GateModalProps {
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
  onClose: () => void;
}

export default function GateModal({
  stageId,
  stageName,
  revisionCount,
  canAcceptAsIs,
  smokeFailures = [],
  reviewerVerdict,
  reviewerNotes,
  onApprove,
  onReject,
  onAcceptAsIs,
  onClose,
}: GateModalProps) {
  const [rejectNotes, setRejectNotes] = useState('');
  const [showReject, setShowReject] = useState(false);

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4" data-testid="gate-modal">
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-lg w-full p-6" data-testid="gate-modal-content">
        <div className="flex items-start justify-between mb-4">
          <div>
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white" data-testid="gate-modal-title">
              Stage {stageId}: {stageName}
            </h3>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              {revisionCount > 0 ? `${revisionCount} revision${revisionCount > 1 ? 's' : ''} so far` : 'First review'}
            </p>
          </div>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200" data-testid="gate-modal-close">
            ✕
          </button>
        </div>

        {smokeFailures.length > 0 && (
          <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg" data-testid="gate-smoke-failures">
            <p className="text-sm font-medium text-red-800 dark:text-red-200 mb-1">Quality check issues:</p>
            <ul className="list-disc list-inside text-sm text-red-700 dark:text-red-300 space-y-1">
              {smokeFailures.map((f, i) => <li key={i}>{f}</li>)}
            </ul>
          </div>
        )}

        {reviewerVerdict === 'NOT-READY' && reviewerNotes && (
          <div className="mb-4 p-3 bg-orange-50 dark:bg-orange-900/20 border border-orange-200 dark:border-orange-800 rounded-lg" data-testid="gate-reviewer-rejection">
            <p className="text-sm font-medium text-orange-800 dark:text-orange-200 mb-1">Reviewer verdict: NOT-READY</p>
            <p className="text-sm text-orange-700 dark:text-orange-300 whitespace-pre-wrap">{reviewerNotes}</p>
          </div>
        )}

        {reviewerVerdict === 'READY' && (
          <div className="mb-4 p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg" data-testid="gate-reviewer-approved">
            <p className="text-sm font-medium text-green-800 dark:text-green-200">✓ Reviewer: READY</p>
          </div>
        )}

        {!showReject ? (
          <div className="flex flex-col gap-3">
            <button
              onClick={onApprove}
              className="px-4 py-3 bg-green-600 text-white rounded-lg hover:bg-green-700 transition-colors text-sm font-semibold"
              data-testid="gate-approve-button"
            >
              ✓ Approve — move to next stage
            </button>
            <button
              onClick={() => setShowReject(true)}
              className="px-4 py-3 bg-orange-600 text-white rounded-lg hover:bg-orange-700 transition-colors text-sm font-semibold"
              data-testid="gate-reject-button"
            >
              ✗ Request changes — send back for revision
            </button>
            {canAcceptAsIs && (
              <button
                onClick={onAcceptAsIs}
                className="px-4 py-3 bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors text-sm font-medium"
                data-testid="gate-accept-as-is-button"
              >
                Accept as-is (escape hatch — {revisionCount} revisions)
              </button>
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
            <div className="flex gap-3">
              <button
                onClick={() => { onReject(rejectNotes); }}
                disabled={!rejectNotes.trim()}
                className="px-4 py-2 bg-orange-600 text-white rounded-lg hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm font-medium"
                data-testid="gate-reject-submit"
              >
                Send back for revision
              </button>
              <button
                onClick={() => setShowReject(false)}
                className="px-4 py-2 text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 transition-colors text-sm"
                data-testid="gate-reject-cancel"
              >
                Back
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}