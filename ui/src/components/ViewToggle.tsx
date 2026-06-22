interface ViewToggleProps {
  value: 'list' | 'board';
  onChange: (value: 'list' | 'board') => void;
}

export default function ViewToggle({ value, onChange }: ViewToggleProps) {
  const base =
    'px-3 py-1 text-sm font-medium rounded-md transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500';
  const active = 'bg-blue-600 text-white';
  const inactive = 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600';

  return (
    <div className="inline-flex gap-1 p-1 bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700" role="group" aria-label="View toggle">
      <button
        type="button"
        data-testid="view-toggle-list"
        onClick={() => onChange('list')}
        className={`${base} ${value === 'list' ? active : inactive}`}
        aria-pressed={value === 'list'}
      >
        List
      </button>
      <button
        type="button"
        data-testid="view-toggle-board"
        onClick={() => onChange('board')}
        className={`${base} ${value === 'board' ? active : inactive}`}
        aria-pressed={value === 'board'}
      >
        Board
      </button>
    </div>
  );
}