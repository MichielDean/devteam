interface EmptyStateProps {
  onCreateClick: () => void;
}

export default function EmptyState({ onCreateClick }: EmptyStateProps) {
  return (
    <div className="text-center py-12" data-testid="empty-state">
      <svg
        className="mx-auto h-24 w-24 text-gray-300 dark:text-gray-600"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={1}
          d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
        />
      </svg>
      <h3 className="mt-4 text-lg font-medium text-gray-900 dark:text-white">
        No features yet
      </h3>
      <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
        Get started by submitting your first feature idea.
      </p>
      <div className="mt-6">
        <button
          onClick={onCreateClick}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors text-sm font-medium"
          data-testid="empty-state-create-button"
        >
          Create First Feature
        </button>
      </div>
    </div>
  );
}