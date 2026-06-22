import FeatureCard from './FeatureCard';
import type { FeatureSummary } from '../types';
import type { ColumnKey } from '../lib/groupFeaturesByColumn';

interface KanbanColumnProps {
  columnKey: ColumnKey;
  label: string;
  features: FeatureSummary[];
}

export default function KanbanColumn({ columnKey, label, features }: KanbanColumnProps) {
  const emptyMessage =
    columnKey === 'backlog' ? 'No features waiting to start' : 'No features in this phase';

  return (
    <section
      data-testid={`kanban-column-${columnKey}`}
      className="flex flex-col w-72 shrink-0 bg-gray-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700"
    >
      <header className="flex items-center justify-between px-3 py-2 border-b border-gray-200 dark:border-gray-700">
        <h3 className="text-sm font-semibold text-gray-900 dark:text-white">{label}</h3>
        <span
          className="text-xs font-bold text-gray-600 dark:text-gray-300"
          data-testid={`kanban-column-count-${columnKey}`}
        >
          {features.length}
        </span>
      </header>
      <div className="flex flex-col gap-2 p-2 overflow-y-auto">
        {features.length === 0 ? (
          <p className="text-xs text-gray-500 dark:text-gray-400 text-center py-4" data-testid={`kanban-column-empty-${columnKey}`}>
            {emptyMessage}
          </p>
        ) : (
          features.map(f => <FeatureCard key={f.id} feature={f} />)
        )}
      </div>
    </section>
  );
}