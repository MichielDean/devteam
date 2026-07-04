import { useState, useEffect, useMemo } from 'react';
import ReactMarkdown from 'react-markdown';
import rehypeHighlight from 'rehype-highlight';
import { getArtifact, listArtifacts, type ArtifactMeta } from '../api/client';

interface ArtifactViewerProps {
  featureId: string;
  phaseStates?: Record<string, unknown>;
  stageId?: string;
  keyArtifacts?: string[];
}

export default function ArtifactViewer({ featureId, phaseStates, stageId, keyArtifacts }: ArtifactViewerProps) {
  void phaseStates;
  const [allArtifacts, setAllArtifacts] = useState<ArtifactMeta[]>([]);
  const [selectedType, setSelectedType] = useState<string | null>(null);
  const [content, setContent] = useState<string>('');
  const [contentLoading, setContentLoading] = useState(false);

  // Fetch all artifacts for the feature
  useEffect(() => {
    listArtifacts(featureId)
      .then(setAllArtifacts)
      .catch(() => setAllArtifacts([]));
  }, [featureId]);

  // Filter: show artifacts relevant to the current stage
  const stageArtifacts = useMemo(() => {
    if (!allArtifacts.length) return [];

    // If we have key_artifacts for this stage, filter to those
    if (keyArtifacts && keyArtifacts.length > 0) {
      const matching = allArtifacts.filter((a) => keyArtifacts.includes(a.artifact_type));
      if (matching.length > 0) return matching;
    }

    // Fall back: filter by stage_id in the artifact record
    if (stageId) {
      const byStage = allArtifacts.filter((a) => a.stage_id === stageId);
      if (byStage.length > 0) return byStage;
    }

    // No stage-specific artifacts found — return empty (not all artifacts)
    return [];
  }, [allArtifacts, keyArtifacts, stageId]);

  // Auto-select first artifact when list changes
  useEffect(() => {
    if (stageArtifacts.length > 0 && !stageArtifacts.find((a) => a.artifact_type === selectedType)) {
      setSelectedType(stageArtifacts[0].artifact_type);
    } else if (stageArtifacts.length === 0) {
      setSelectedType(null);
    }
  }, [stageArtifacts, selectedType]);

  // Fetch content when selection changes
  useEffect(() => {
    if (!selectedType) { setContent(''); return; }
    setContentLoading(true);
    getArtifact(featureId, selectedType)
      .then((text) => { setContent(text); setContentLoading(false); })
      .catch(() => { setContent(''); setContentLoading(false); });
  }, [featureId, selectedType]);

  if (stageArtifacts.length === 0) {
    return (
      <div className="py-6 text-center" data-testid="artifact-empty">
        <p className="text-sm text-[var(--color-text-tertiary)]">
          {keyArtifacts && keyArtifacts.length > 0
            ? `Expected: ${keyArtifacts.join(', ')} — not yet produced`
            : 'No artifacts for this stage.'}
        </p>
      </div>
    );
  }

  return (
    <div data-testid="artifact-viewer">
      {/* Artifact tabs — only for current stage */}
      <div className="flex flex-wrap gap-1.5 mb-3" data-testid="artifact-list">
        {stageArtifacts.map((artifact) => (
          <button
            key={artifact.artifact_type}
            onClick={() => setSelectedType(artifact.artifact_type)}
            className={`px-3 py-1.5 rounded-[var(--radius-md)] text-sm font-medium transition-colors ${
              selectedType === artifact.artifact_type
                ? 'bg-[var(--color-accent)] text-white'
                : 'bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)] hover:bg-[var(--color-surface-active)]'
            }`}
            data-testid={`artifact-tab-${artifact.artifact_type}`}
          >
            {artifact.artifact_type}
            <span className={`ml-1.5 text-xs ${selectedType === artifact.artifact_type ? 'text-blue-200' : 'text-[var(--color-text-tertiary)]'}`}>
              {(artifact.size / 1024).toFixed(1)}KB
            </span>
          </button>
        ))}
      </div>

      {/* Artifact content */}
      {selectedType && (
        <div className="rounded-[var(--radius-md)] overflow-hidden" style={{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border-subtle)' }} data-testid="artifact-content">
          {contentLoading && (
            <div className="flex items-center justify-center py-6">
              <div className="animate-spin rounded-full h-5 w-5 border-2 border-t-transparent" style={{ borderColor: 'var(--color-accent)', borderTopColor: 'transparent' }} />
              <span className="ml-2 text-sm text-[var(--color-text-tertiary)]">Loading...</span>
            </div>
          )}
          {!contentLoading && content && (
            <div className="prose prose-sm dark:prose-invert max-w-none p-4 overflow-auto max-h-[500px]" data-testid="artifact-markdown">
              <ReactMarkdown rehypePlugins={[rehypeHighlight]}>{content}</ReactMarkdown>
            </div>
          )}
          {!contentLoading && !content && (
            <div className="p-4 text-center" data-testid="artifact-not-found">
              <p className="text-sm text-[var(--color-text-tertiary)]">Content not available.</p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}