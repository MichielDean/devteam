import { useState } from 'react';
import type { FeatureSummary } from '../types';
import FeatureCard from './FeatureCard';

type SortField = 'phase' | 'priority' | 'status' | 'updated_at';
type SortDirection = 'asc' | 'desc';

interface FeatureListProps {
  features: FeatureSummary[];
}

export default function FeatureList({ features }: FeatureListProps) {
  const [sortField, setSortField] = useState<SortField>('updated_at');
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc');

  const sortedFeatures = [...features].sort((a, b) => {
    let comparison = 0;
    switch (sortField) {
      case 'phase':
        comparison = a.current_phase.localeCompare(b.current_phase);
        break;
      case 'priority':
        comparison = a.priority - b.priority;
        break;
      case 'status':
        comparison = a.status.localeCompare(b.status);
        break;
      case 'updated_at':
        comparison = new Date(a.updated_at).getTime() - new Date(b.updated_at).getTime();
        break;
    }
    return sortDirection === 'asc' ? comparison : -comparison;
  });

  const toggleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDirection((prev) => (prev === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortField(field);
      setSortDirection('desc');
    }
  };

  const sortIcon = (field: SortField) => {
    if (sortField !== field) return '↕';
    return sortDirection === 'asc' ? '↑' : '↓';
  };

  return (
    <div data-testid="feature-list">
      <div className="flex items-center gap-4 mb-4 text-sm">
        <span className="text-gray-500 dark:text-gray-400">Sort by:</span>
        <button
          onClick={() => toggleSort('phase')}
          className={`px-3 py-1 rounded-md transition-colors ${
            sortField === 'phase' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200' : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700'
          }`}
          data-testid="sort-by-phase"
        >
          Phase {sortIcon('phase')}
        </button>
        <button
          onClick={() => toggleSort('priority')}
          className={`px-3 py-1 rounded-md transition-colors ${
            sortField === 'priority' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200' : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700'
          }`}
          data-testid="sort-by-priority"
        >
          Priority {sortIcon('priority')}
        </button>
        <button
          onClick={() => toggleSort('status')}
          className={`px-3 py-1 rounded-md transition-colors ${
            sortField === 'status' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200' : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700'
          }`}
          data-testid="sort-by-status"
        >
          Status {sortIcon('status')}
        </button>
        <button
          onClick={() => toggleSort('updated_at')}
          className={`px-3 py-1 rounded-md transition-colors ${
            sortField === 'updated_at' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200' : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700'
          }`}
          data-testid="sort-by-updated"
        >
          Updated {sortIcon('updated_at')}
        </button>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {sortedFeatures.map((feature) => (
          <FeatureCard key={feature.id} feature={feature} />
        ))}
      </div>
    </div>
  );
}