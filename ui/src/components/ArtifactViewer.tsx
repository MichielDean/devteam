import { useState, useEffect } from 'react';
import ReactMarkdown from 'react-markdown';
import rehypeHighlight from 'rehype-highlight';
import { getArtifact, listArtifacts, type ArtifactMeta } from '../api/client';

interface ArtifactViewerProps {
  featureId: string;
  phaseStates?: Record<string, unknown>;
  stageId?: string;
}

export default function ArtifactViewer({ featureId, phaseStates, stageId }: ArtifactViewerProps) {
  void phaseStates;
  const [artifacts, setArtifacts] = useState<ArtifactMeta[]>([]);
  const [selectedType, setSelectedType] = useState<string | null>(null);
  const [content, setContent] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [contentLoading, setContentLoading] = useState(false);

  // Fetch artifact list
  useEffect(() => {
    setLoading(true);
    listArtifacts(featureId)
      .then((arts) => {
        setArtifacts(arts);
        // Auto-select the first artifact for the current stage, or the first overall
        if (arts.length > 0 && !selectedType) {
          const stageMatch = stageId
            ? arts.find((a) => a.stage_id === stageId)
            : null;
          setSelectedType((stageMatch || arts[0]).artifact_type);
        }
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, [featureId, stageId]);

  // Fetch artifact content when selected
  useEffect(() => {
    if (!selectedType) return;
    setContentLoading(true);
    getArtifact(featureId, selectedType)
      .then((text) => { setContent(text); setContentLoading(false); })
      .catch(() => { setContent(''); setContentLoading(false); });
  }, [featureId, selectedType]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8" data-testid="artifact-loading">
        <div className="animate-spin rounded-full h-6 w-6 border-2 border-t-transparent" style={{ borderColor: 'var(--color-accent)', borderTopColor: 'transparent' }} />
        <span className="ml-3 text-sm text-[var(--color-text-tertiary)]">Loading artifacts...</span>
      </div>
    );
  }

  if (artifacts.length === 0) {
    return (
      <div className="py-8 text-center" data-testid="artifact-empty">
        <p className="text-sm font-medium text-[var(--color-text-secondary)] mb-1">No artifacts yet</p>
        <p className="text-xs text-[var(--color-text-tertiary)]">Artifacts will appear here as stages complete.</p>
      </div>
    );
  }

  return (
    <div data-testid="artifact-viewer">
      {/* Artifact list — show ALL artifacts for the feature */}
      <div className="flex flex-wrap gap-1.5 mb-4" data-testid="artifact-list">
        {artifacts.map((artifact) => (
          <button
            key={artifact.artifact_type}
            onClick={() => setSelectedType(artifact.artifact_type)}
            className={`px-3 py-1.5 rounded-[var(--radius-md)] text-sm font-medium transition-colors flex items-center gap-1.5 ${
              selectedType === artifact.artifact_type
                ? 'bg-[var(--color-accent)] text-white'
                : 'bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)] hover:bg-[var(--color-surface-active)]'
            }`}
            data-testid={`artifact-tab-${artifact.artifact_type}`}
          >
            {artifact.artifact_type}
            {artifact.stage_id && (
              <span className={`text-xs ${selectedType === artifact.artifact_type ? 'text-blue-200' : 'text-[var(--color-text-tertiary)]'}`}>
                {artifact.stage_id}
              </span>
            )}
          </button>
        ))}
      </div>

      {/* Artifact content */}
      {selectedType && (
        <div className="rounded-[var(--radius-md)] overflow-hidden" style={{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border-subtle)' }} data-testid="artifact-content">
          {contentLoading && (
            <div className="flex items-center justify-center py-8">
              <div className="animate-spin rounded-full h-6 w-6 border-2 border-t-transparent" style={{ borderColor: 'var(--color-accent)', borderTopColor: 'transparent' }} />
              <span className="ml-3 text-sm text-[var(--color-text-tertiary)]">Loading...</span>
            </div>
          )}
          {!contentLoading && content && (
            <div className="prose prose-sm dark:prose-invert max-w-none p-4 overflow-auto max-h-[600px]" data-testid="artifact-markdown">
              <ReactMarkdown rehypePlugins={[rehypeHighlight]}>{content}</ReactMarkdown>
            </div>
          )}
          {!contentLoading && !content && (
            <div className="p-6 text-center" data-testid="artifact-not-found">
              <p className="text-sm text-[var(--color-text-tertiary)]">Artifact content not available.</p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}