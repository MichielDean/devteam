import { useState, useEffect } from 'react';
import ReactMarkdown from 'react-markdown';
import rehypeHighlight from 'rehype-highlight';
import { getArtifact } from '../api/client';

interface ArtifactViewerProps {
  featureId: string;
  phaseStates?: Record<string, unknown>;
}

const ARTIFACT_TYPES = [
  { key: 'input', label: 'Input', apiPath: 'input' },
  { key: 'spec', label: 'Specification', apiPath: 'spec' },
  { key: 'acceptance', label: 'Acceptance Criteria', apiPath: 'acceptance' },
  { key: 'repos', label: 'Repositories', apiPath: 'repos' },
  { key: 'plan', label: 'Plan', apiPath: 'plan' },
  { key: 'tasks', label: 'Tasks', apiPath: 'tasks' },
  { key: 'review_report', label: 'Review Report', apiPath: 'review_report' },
  { key: 'test_report', label: 'Test Report', apiPath: 'test_report' },
  { key: 'docs', label: 'Documentation', apiPath: 'docs' },
];

export default function ArtifactViewer({ featureId, phaseStates }: ArtifactViewerProps) {
  void phaseStates;
  const [selectedArtifact, setSelectedArtifact] = useState<string | null>(null);
  const [content, setContent] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!selectedArtifact) return;
    const artifact = ARTIFACT_TYPES.find((a) => a.key === selectedArtifact);
    if (!artifact) return;

    setLoading(true);
    setError(null);
    getArtifact(featureId, artifact.apiPath)
      .then((text) => { setContent(text); setLoading(false); })
      .catch((err) => { setError(err.message); setLoading(false); });
  }, [featureId, selectedArtifact]);

  return (
    <div data-testid="artifact-viewer">
      <div className="flex flex-wrap gap-2 mb-4">
        {ARTIFACT_TYPES.map((artifact) => (
          <button
            key={artifact.key}
            onClick={() => setSelectedArtifact(artifact.key)}
            className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
              selectedArtifact === artifact.key
                ? 'bg-blue-600 text-white'
                : 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600'
            }`}
            data-testid={`artifact-tab-${artifact.key}`}
          >
            {artifact.label}
          </button>
        ))}
      </div>

      {selectedArtifact && (
        <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden" data-testid="artifact-content">
          {loading && (
            <div className="flex items-center justify-center py-12">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
              <span className="ml-3 text-gray-500 dark:text-gray-400">Loading...</span>
            </div>
          )}
          {error && (
            <div className="p-4 text-red-600 dark:text-red-400" data-testid="artifact-error">Error: {error}</div>
          )}
          {!loading && !error && content && (
            <div className="prose dark:prose-invert max-w-none p-4 overflow-auto max-h-[600px]" data-testid="artifact-markdown">
              <ReactMarkdown rehypePlugins={[rehypeHighlight]}>{content}</ReactMarkdown>
            </div>
          )}
          {!loading && !error && !content && (
            <div className="p-8 text-center text-gray-500 dark:text-gray-400" data-testid="artifact-not-generated">
              <p className="text-lg font-medium mb-2">Not yet generated</p>
              <p className="text-sm">This artifact will appear once the producing stage is completed.</p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}