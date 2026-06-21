interface EmptyStateProps {
  onCreateClick: () => void;
}

export default function EmptyState({ onCreateClick }: EmptyStateProps) {
  return (
    <div className="text-center py-16" data-testid="empty-state">
      <div className="mx-auto w-24 h-24 rounded-full bg-blue-50 dark:bg-blue-900/30 flex items-center justify-center mb-6">
        <svg className="w-12 h-12 text-blue-500 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
        </svg>
      </div>
      <h3 className="text-xl font-semibold text-gray-900 dark:text-white mb-2">
        No features yet
      </h3>
      <p className="text-gray-500 dark:text-gray-400 max-w-md mx-auto mb-6">
        Describe what you want built. The pipeline will take it from idea through spec, plan, code, review, test, and delivery.
      </p>
      <button
        onClick={onCreateClick}
        className="px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors font-semibold shadow-sm"
        data-testid="empty-state-create-button"
      >
        Start Your First Feature
      </button>
    </div>
  );
}