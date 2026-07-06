import { useEffect, useRef } from 'react';

interface AutoApprovePanelProps {
  stageId: string;
  onApprove: () => void;
}

export default function AutoApprovePanel({ stageId, onApprove }: AutoApprovePanelProps) {
  const approved = useRef(false);

  useEffect(() => {
    if (approved.current) return;
    approved.current = true;
    // Small delay so the UI can render the banner first
    const timer = setTimeout(() => onApprove(), 500);
    return () => clearTimeout(timer);
  }, [onApprove]);

  return (
    <div className="p-4 rounded-[var(--radius-md)]" style={{ backgroundColor: 'var(--color-surface-hover)' }} data-testid="auto-approve-panel">
      <div className="flex items-center gap-3">
        <span className="animate-spin rounded-full h-4 w-4 border-2 border-t-transparent" style={{ borderColor: 'var(--color-success)', borderTopColor: 'transparent' }} />
        <div>
          <p className="text-sm font-medium text-[var(--color-text-primary)]">Auto-approving stage {stageId}</p>
          <p className="text-xs text-[var(--color-text-tertiary)]">Autonomous mode — continuing to next stage</p>
        </div>
      </div>
    </div>
  );
}