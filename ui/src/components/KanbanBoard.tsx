import { PHASES, PHASE_LABELS, type FeatureSummary } from '../types';
import FeatureCard from './FeatureCard';

export interface PhaseColumn {
  phase: string;
  label: string;
  features: FeatureSummary[];
}

/**
 * Group features into six phase columns (in PHASES order) plus a defensive
 * "Other" column for unknown current_phase values. API order preserved within
 * each column (FR-003). Never throws (CON-011).
 */
export function groupFeaturesByPhase(features: FeatureSummary[] | null | undefined): PhaseColumn[] {
  const known = new Set<string>(PHASES);
  const buckets = new Map<string, FeatureSummary[]>();
  for (const p of PHASES) buckets.set(p, []);
  let other: FeatureSummary[] = [];

  for (const f of features ?? []) {
    if (known.has(f.current_phase)) {
      buckets.get(f.current_phase)!.push(f);
    } else {
      other.push(f);
    }
  }

  const columns: PhaseColumn[] = PHASES.map(p => ({
    phase: p,
    label: PHASE_LABELS[p],
    features: buckets.get(p)!,
  }));
  if (other.length > 0) {
    // ponytail: "Other" is a UI fallback, not a pipeline phase; intentionally not in PHASE_LABELS.
    columns.push({ phase: 'other', label: 'Other', features: other });
  }
  return columns;
}

interface KanbanBoardProps {
  features: FeatureSummary[];
}

export default function KanbanBoard({ features }: KanbanBoardProps) {
  const columns = groupFeaturesByPhase(features);

  return (
    <div data-testid="kanban-board" className="flex gap-4 overflow-x-auto min-w-max pb-2">
      {columns.map(col => (
        <section
          key={col.phase}
          data-testid={`kanban-column-${col.phase}`}
          className="flex flex-col min-w-[16rem] w-64 shrink-0 bg-gray-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700"
        >
          <header
            data-testid={`kanban-column-header-${col.phase}`}
            className="px-3 py-2 border-b border-gray-200 dark:border-gray-700 shrink-0"
          >
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white">{col.label}</h3>
          </header>
          <div className="flex flex-col gap-2 p-2 flex-1">
            {col.features.length === 0 ? (
              <p
                data-testid={`kanban-column-empty-${col.phase}`}
                className="text-xs text-gray-400 dark:text-gray-500 text-center py-4"
              >
                No features
              </p>
            ) : (
              col.features.map(f => <FeatureCard key={f.id} feature={f} />)
            )}
          </div>
        </section>
      ))}
    </div>
  );
}