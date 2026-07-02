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
      <div className="flex flex-wrap gap-1.5 mb-4">
        {ARTIFACT_TYPES.map((artifact) => (
          <button
            key={artifact.key}
            onClick={() => setSelectedArtifact(artifact.key)}
            className={`px-3 py-1.5 rounded-[var(--radius-md)] text-sm font-medium transition-colors ${
              selectedArtifact === artifact.key
                ? 'bg-[var(--color-accent)] text-white'
                : 'bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)] hover:bg-[var(--color-surface-active)]'
            }`}
            data-testid={`artifact-tab-${artifact.key}`}
          >
            {artifact.label}
          </button>
        ))}
      </div>

      {selectedArtifact && (
        <div className="rounded-[var(--radius-md)] overflow-hidden" style={{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border-subtle)' }} data-testid="artifact-content">
          {loading && (
            <div className="flex items-center justify-center py-12">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2" style={{ borderColor: 'var(--color-accent)' }} />
              <span className="ml-3 text-[var(--color-text-tertiary)]">Loading...</span>
            </div>
          )}
          {error && (
            <div className="p-4" style={{ color: 'var(--color-danger)' }} data-testid="artifact-error">Error: {error}</div>
          )}
          {!loading && !error && content && (
            <div className="prose dark:prose-invert max-w-none p-4 overflow-auto max-h-[600px]" data-testid="artifact-markdown">
              <ReactMarkdown rehypePlugins={[rehypeHighlight]}>{content}</ReactMarkdown>
            </div>
          )}
          {!loading && !error && !content && (
            <div className="p-8 text-center" data-testid="artifact-not-generated">
              <p className="text-base font-medium text-[var(--color-text-secondary)] mb-2">Not yet generated</p>
              <p className="text-sm text-[var(--color-text-tertiary)]">This artifact will appear once the producing stage is completed.</p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}