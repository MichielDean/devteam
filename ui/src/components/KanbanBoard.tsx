import { useQuery } from '@tanstack/react-query';
import { listFeatures } from '../api/client';
import {
  groupFeaturesByColumn,
  COLUMN_KEYS,
  COLUMN_LABELS,
} from '../lib/groupFeaturesByColumn';
import KanbanColumn from './KanbanColumn';

export default function KanbanBoard() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['features'],
    queryFn: listFeatures,
  });

  const grouped = groupFeaturesByColumn(data?.features ?? []);

  return (
    <div data-testid="kanban-board" className="flex flex-col gap-3">
      {error && !data && (
        <div
          data-testid="kanban-error"
          className="text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg px-4 py-3"
          role="alert"
        >
          Failed to load features: {error.message}
        </div>
      )}
      {isLoading && !data && (
        <div className="flex items-center justify-center py-12" data-testid="kanban-loading">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
          <span className="ml-3 text-gray-500 dark:text-gray-400">Loading features...</span>
        </div>
      )}
      <div className="flex gap-3 overflow-x-auto pb-2">
        {COLUMN_KEYS.map(key => (
          <KanbanColumn
            key={key}
            columnKey={key}
            label={COLUMN_LABELS[key]}
            features={grouped[key]}
          />
        ))}
      </div>
    </div>
  );
}