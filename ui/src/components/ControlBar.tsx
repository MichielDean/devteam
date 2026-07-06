import { useState } from 'react';
import { Button, Modal } from '../ui/primitives';
import { SCOPES, SCOPE_LABELS, SCOPE_DESCRIPTIONS, DEPTHS, DEPTH_LABELS, TEST_STRATEGIES, TEST_STRATEGY_LABELS, EXECUTION_MODES, EXECUTION_MODE_LABELS, EXECUTION_MODE_DESCRIPTIONS } from '../types';

interface ControlBarProps {
  onJumpStage: (stageId: string) => void;
  onJumpPhase: (phase: string) => void;
  onSetScope: (scope: string) => void;
  onSetDepth: (depth: string) => void;
  onSetTestStrategy: (strategy: string) => void;
  onSetExecutionMode: (mode: string) => void;
  onCancel: () => void;
  currentScope: string;
  currentDepth: string;
  currentTestStrategy: string;
  currentExecutionMode: string;
  availableStages: { stage_id: string; status: string }[];
  isTerminal: boolean;
}

const selectClass =
  'w-full px-3 py-2 rounded-[var(--radius-md)] bg-[var(--color-surface-raised)] text-[var(--color-text-primary)] border border-[var(--color-border-subtle)] focus:border-[var(--color-accent)] focus:outline-none text-sm';
const labelClass = 'block text-xs font-medium text-[var(--color-text-secondary)] mb-1.5';

export default function ControlBar({
  onJumpStage, onJumpPhase, onSetScope, onSetDepth, onSetTestStrategy, onSetExecutionMode, onCancel,
  currentScope, currentDepth, currentTestStrategy, currentExecutionMode, availableStages, isTerminal,
}: ControlBarProps) {
  const [jumpOpen, setJumpOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);

  if (isTerminal) return null;

  return (
    <div
      className="flex items-center gap-2 p-2 rounded-[var(--radius-lg)] sticky bottom-4"
      style={{ backgroundColor: 'var(--color-surface-raised)', boxShadow: 'var(--shadow-md)' }}
      data-testid="control-bar"
    >
      <Button variant="ghost" size="sm" onClick={() => setJumpOpen(true)} data-testid="control-jump">
        Jump
      </Button>
      <Button variant="ghost" size="sm" onClick={() => setSettingsOpen(true)} data-testid="control-settings">
        Settings
      </Button>
      <div className="flex-1" />
      <Button variant="danger" size="sm" onClick={() => { if (window.confirm('Cancel this feature?')) onCancel(); }} data-testid="control-cancel">
        Cancel
      </Button>

      <Modal open={jumpOpen} onClose={() => setJumpOpen(false)} title="Jump to Stage or Phase">
        <div className="space-y-4">
          <div>
            <label className={labelClass}>Jump to stage</label>
            <select
              className={selectClass}
              defaultValue=""
              onChange={(e) => { if (e.target.value) { onJumpStage(e.target.value); setJumpOpen(false); } }}
              data-testid="jump-stage-select"
            >
              <option value="">Select a stage...</option>
              {availableStages.filter((s) => s.status === 'not_started' || s.status === 'skipped').map((s) => (
                <option key={s.stage_id} value={s.stage_id}>Stage {s.stage_id}</option>
              ))}
            </select>
          </div>
          <div>
            <label className={labelClass}>Jump to phase</label>
            <div className="flex flex-wrap gap-2">
              {['ideation', 'inception', 'construction', 'operation'].map((phase) => (
                <Button key={phase} variant="secondary" size="sm" onClick={() => { onJumpPhase(phase); setJumpOpen(false); }} data-testid={`jump-phase-${phase}`}>
                  {phase.charAt(0).toUpperCase() + phase.slice(1)}
                </Button>
              ))}
            </div>
          </div>
        </div>
      </Modal>

      <Modal open={settingsOpen} onClose={() => setSettingsOpen(false)} title="Feature Settings">
        <div className="space-y-4">
          <div>
            <label className={labelClass}>Scope</label>
            <select
              value={currentScope}
              onChange={(e) => onSetScope(e.target.value)}
              className={selectClass}
              data-testid="settings-scope"
            >
              {SCOPES.map((s) => <option key={s} value={s}>{SCOPE_LABELS[s]}</option>)}
            </select>
            <p className="text-xs text-[var(--color-text-tertiary)] mt-1">{SCOPE_DESCRIPTIONS[currentScope]}</p>
          </div>
          <div>
            <label className={labelClass}>Depth</label>
            <select
              value={currentDepth}
              onChange={(e) => onSetDepth(e.target.value)}
              className={selectClass}
              data-testid="settings-depth"
            >
              {DEPTHS.map((d) => <option key={d} value={d}>{DEPTH_LABELS[d]}</option>)}
            </select>
          </div>
          <div>
            <label className={labelClass}>Test Strategy</label>
            <select
              value={currentTestStrategy}
              onChange={(e) => onSetTestStrategy(e.target.value)}
              className={selectClass}
              data-testid="settings-test-strategy"
            >
              {TEST_STRATEGIES.map((t) => <option key={t} value={t}>{TEST_STRATEGY_LABELS[t]}</option>)}
            </select>
          </div>
          <div>
            <label className={labelClass}>Execution Mode</label>
            <select
              value={currentExecutionMode || 'human'}
              onChange={(e) => onSetExecutionMode(e.target.value)}
              className={selectClass}
              data-testid="settings-execution-mode"
            >
              {EXECUTION_MODES.map((m) => <option key={m} value={m}>{EXECUTION_MODE_LABELS[m]}</option>)}
            </select>
            <p className="text-xs text-[var(--color-text-tertiary)] mt-1">
              {EXECUTION_MODE_DESCRIPTIONS[currentExecutionMode || 'human']}
            </p>
          </div>
        </div>
      </Modal>
    </div>
  );
}