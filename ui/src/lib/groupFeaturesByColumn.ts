import { PHASES, PHASE_LABELS, type PhaseName } from '../types';
import type { FeatureSummary } from '../types';

export type ColumnKey = 'backlog' | PhaseName;

export const COLUMN_KEYS = ['backlog', ...PHASES] as const;

export const COLUMN_LABELS: Record<ColumnKey, string> = {
  backlog: 'Backlog',
  ...PHASE_LABELS,
};

function emptyColumns(): Record<ColumnKey, FeatureSummary[]> {
  return {
    backlog: [],
    inception: [],
    planning: [],
    construction: [],
    review: [],
    testing: [],
    delivery: [],
  };
}

export function groupFeaturesByColumn(
  features: FeatureSummary[],
): Record<ColumnKey, FeatureSummary[]> {
  const cols = emptyColumns();
  for (const f of features) {
    if (f.status === 'draft' && f.current_phase === 'inception') {
      cols.backlog.push(f);
    } else if (PHASES.includes(f.current_phase as PhaseName)) {
      cols[f.current_phase as ColumnKey].push(f);
    }
    // else: unknown phase — drop (defensive; not in canonical PHASES).
  }
  return cols;
}