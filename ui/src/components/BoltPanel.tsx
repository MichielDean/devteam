import { Button, Badge, Card } from '../ui/primitives';
import type { Bolt } from '../types';

interface BoltPanelProps {
  bolts: Bolt[];
  onPrepareBolts: () => void;
  onRunBolt: (boltNumber: number) => void;
  onSetLadder: (mode: string) => void;
  autonomyMode?: string;
  showLadderPrompt: boolean;
}

export default function BoltPanel({ bolts, onPrepareBolts, onRunBolt, onSetLadder, autonomyMode, showLadderPrompt }: BoltPanelProps) {
  if (bolts.length === 0 && !showLadderPrompt) {
    return (
      <Card className="p-4 mb-4" data-testid="bolts-panel">
        <div className="flex items-center justify-between mb-2">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Construction Bolts</h3>
          <Button variant="primary" size="sm" onClick={onPrepareBolts} data-testid="prepare-bolts-button">Prepare Bolts</Button>
        </div>
        <p className="text-sm text-gray-500" data-testid="bolts-empty">No Bolts yet. Prepare Bolts from inception output to start construction.</p>
      </Card>
    );
  }

  return (
    <Card className="p-4 mb-4" data-testid="bolts-panel">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Construction Bolts</h3>
        <span className="text-sm text-gray-500">{bolts.length} bolt{bolts.length !== 1 ? 's' : ''}</span>
      </div>

      <div className="space-y-2" data-testid="bolt-list">
        {bolts.map((b) => (
          <div
            key={b.bolt_number}
            className={`flex items-center gap-3 p-3 rounded-lg ${b.is_walking_skeleton ? 'bg-indigo-50 dark:bg-indigo-900/20 border border-indigo-200 dark:border-indigo-800' : 'bg-gray-50 dark:bg-gray-900/30'}`}
            data-testid={`bolt-${b.bolt_number}`}
          >
            <span className="font-mono text-sm font-medium text-gray-900 dark:text-white">Bolt {b.bolt_number}</span>
            {b.is_walking_skeleton && <Badge color="indigo" data-testid={`bolt-walking-skeleton-${b.bolt_number}`}>Walking Skeleton</Badge>}
            <span className={`text-sm ${b.status === 'completed' ? 'text-green-600' : b.status === 'in_progress' ? 'text-blue-600' : b.status === 'failed' ? 'text-red-600' : 'text-gray-500'}`} data-testid={`bolt-status-${b.bolt_number}`}>{b.status}</span>
            <span className="text-xs text-gray-400">{b.unit_ids.length} unit(s)</span>
            <div className="flex-1" />
            {(b.status === 'pending' || b.status === 'failed') && (
              <Button variant="primary" size="sm" onClick={() => onRunBolt(b.bolt_number)} data-testid={`run-bolt-${b.bolt_number}`}>Run</Button>
            )}
          </div>
        ))}
      </div>

      {showLadderPrompt && !autonomyMode && (
        <div className="mt-4 p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg" data-testid="ladder-prompt">
          <p className="text-sm font-medium text-yellow-800 dark:text-yellow-200 mb-2">🪜 Ladder Prompt: Walking skeleton complete. Choose autonomy mode:</p>
          <div className="flex gap-2">
            <Button variant="warning" size="sm" onClick={() => onSetLadder('gated')} data-testid="ladder-gated">Gated (approve each Bolt)</Button>
            <Button variant="primary" size="sm" onClick={() => onSetLadder('autonomous')} data-testid="ladder-autonomous">Autonomous</Button>
          </div>
        </div>
      )}

      {autonomyMode && (
        <div className="mt-3 text-sm text-gray-500" data-testid="autonomy-mode-display">
          Autonomy: <Badge color={autonomyMode === 'autonomous' ? 'indigo' : 'yellow'}>{autonomyMode}</Badge>
        </div>
      )}
    </Card>
  );
}