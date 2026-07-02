import { Button, Badge } from '../ui/primitives';
import type { Color } from '../ui/primitives/Badge';
import type { Bolt } from '../types';

interface BoltPanelProps {
  bolts: Bolt[];
  onPrepareBolts: () => void;
  onRunBolt: (boltNumber: number) => void;
  onSetLadder: (mode: string) => void;
  autonomyMode?: string;
  showLadderPrompt: boolean;
}

const statusColor: Record<string, Color> = {
  completed: 'green',
  in_progress: 'blue',
  failed: 'red',
  pending: 'gray',
};

export default function BoltPanel({ bolts, onPrepareBolts, onRunBolt, onSetLadder, autonomyMode, showLadderPrompt }: BoltPanelProps) {
  if (bolts.length === 0 && !showLadderPrompt) {
    return (
      <div className="p-4 mb-4 rounded-[var(--radius-lg)]" style={{ backgroundColor: 'var(--color-surface-raised)', boxShadow: 'var(--shadow-sm)' }} data-testid="bolts-panel">
        <div className="flex items-center justify-between mb-2">
          <h3 className="text-base font-medium text-[var(--color-text-primary)]">Construction Bolts</h3>
          <Button variant="primary" size="sm" onClick={onPrepareBolts} data-testid="prepare-bolts-button">Prepare Bolts</Button>
        </div>
        <p className="text-sm text-[var(--color-text-tertiary)]" data-testid="bolts-empty">No Bolts yet. Prepare Bolts from inception output to start construction.</p>
      </div>
    );
  }

  return (
    <div className="p-4 mb-4 rounded-[var(--radius-lg)]" style={{ backgroundColor: 'var(--color-surface-raised)', boxShadow: 'var(--shadow-sm)' }} data-testid="bolts-panel">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-base font-medium text-[var(--color-text-primary)]">Construction Bolts</h3>
        <span className="text-sm text-[var(--color-text-tertiary)]">{bolts.length} bolt{bolts.length !== 1 ? 's' : ''}</span>
      </div>

      <div className="space-y-2" data-testid="bolt-list">
        {bolts.map((b) => (
          <div
            key={b.bolt_number}
            className="flex items-center gap-3 p-3 rounded-[var(--radius-md)]"
            style={{
              backgroundColor: b.is_walking_skeleton ? 'var(--color-surface-active)' : 'var(--color-surface-hover)',
              borderLeft: b.is_walking_skeleton ? '2px solid var(--color-accent)' : undefined,
            }}
            data-testid={`bolt-${b.bolt_number}`}
          >
            <span className="font-mono text-sm font-medium text-[var(--color-text-primary)]">Bolt {b.bolt_number}</span>
            {b.is_walking_skeleton && <Badge color="blue" data-testid={`bolt-walking-skeleton-${b.bolt_number}`}>Walking Skeleton</Badge>}
            <Badge color={statusColor[b.status] || 'gray'} data-testid={`bolt-status-${b.bolt_number}`}>{b.status}</Badge>
            <span className="text-xs text-[var(--color-text-tertiary)]">{b.unit_ids.length} unit(s)</span>
            <div className="flex-1" />
            {(b.status === 'pending' || b.status === 'failed') && (
              <Button variant="primary" size="sm" onClick={() => onRunBolt(b.bolt_number)} data-testid={`run-bolt-${b.bolt_number}`}>Run</Button>
            )}
          </div>
        ))}
      </div>

      {showLadderPrompt && !autonomyMode && (
        <div className="mt-4 p-3 rounded-[var(--radius-md)]" style={{ backgroundColor: 'var(--color-warning-surface)' }} data-testid="ladder-prompt">
          <p className="text-sm font-medium mb-2" style={{ color: 'var(--color-warning)' }}>🪜 Ladder Prompt: Walking skeleton complete. Choose autonomy mode:</p>
          <div className="flex gap-2">
            <Button variant="warning" size="sm" onClick={() => onSetLadder('gated')} data-testid="ladder-gated">Gated (approve each Bolt)</Button>
            <Button variant="primary" size="sm" onClick={() => onSetLadder('autonomous')} data-testid="ladder-autonomous">Autonomous</Button>
          </div>
        </div>
      )}

      {autonomyMode && (
        <div className="mt-3 text-sm text-[var(--color-text-tertiary)]" data-testid="autonomy-mode-display">
          Autonomy: <Badge color={autonomyMode === 'autonomous' ? 'blue' : 'yellow'}>{autonomyMode}</Badge>
        </div>
      )}
    </div>
  );
}