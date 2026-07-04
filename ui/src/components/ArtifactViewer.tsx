import { useState, useEffect, useMemo } from 'react';
import ReactMarkdown from 'react-markdown';
import rehypeHighlight from 'rehype-highlight';
import { getArtifact, updateArtifact, listArtifacts, type ArtifactMeta } from '../api/client';

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
  const [editedContent, setEditedContent] = useState<string>('');
  const [contentLoading, setContentLoading] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    listArtifacts(featureId).then(setAllArtifacts).catch(() => setAllArtifacts([]));
  }, [featureId]);

  const stageArtifacts = useMemo(() => {
    if (!allArtifacts.length) return [];
    if (keyArtifacts && keyArtifacts.length > 0) {
      const matching = allArtifacts.filter((a) => keyArtifacts.includes(a.artifact_type));
      if (matching.length > 0) return matching;
    }
    if (stageId) {
      const byStage = allArtifacts.filter((a) => a.stage_id === stageId);
      if (byStage.length > 0) return byStage;
    }
    return [];
  }, [allArtifacts, keyArtifacts, stageId]);

  useEffect(() => {
    if (stageArtifacts.length > 0 && !stageArtifacts.find((a) => a.artifact_type === selectedType)) {
      setSelectedType(stageArtifacts[0].artifact_type);
    } else if (stageArtifacts.length === 0) {
      setSelectedType(null);
    }
  }, [stageArtifacts, selectedType]);

  useEffect(() => {
    if (!selectedType) { setContent(''); setEditedContent(''); return; }
    setContentLoading(true);
    setIsEditing(false);
    setDirty(false);
    getArtifact(featureId, selectedType)
      .then((text) => { setContent(text); setEditedContent(text); setContentLoading(false); })
      .catch(() => { setContent(''); setEditedContent(''); setContentLoading(false); });
  }, [featureId, selectedType]);

  const handleEdit = () => {
    setIsEditing(true);
    setEditedContent(content);
    setDirty(false);
  };

  const handleCancel = () => {
    setIsEditing(false);
    setEditedContent(content);
    setDirty(false);
  };

  const handleSave = async () => {
    if (!selectedType || !dirty) return;
    setIsSaving(true);
    try {
      await updateArtifact(featureId, selectedType, editedContent);
      setContent(editedContent);
      setIsEditing(false);
      setDirty(false);
    } catch (err) {
      console.error('Failed to save artifact:', err);
    }
    setIsSaving(false);
  };

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

      {selectedType && (
        <div className="rounded-[var(--radius-md)] overflow-hidden" style={{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border-subtle)' }} data-testid="artifact-content">
          {/* Toolbar */}
          <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--color-border-subtle)]" style={{ backgroundColor: 'var(--color-surface-hover)' }}>
            <span className="text-xs text-[var(--color-text-tertiary)]">{selectedType}</span>
            <div className="flex items-center gap-2">
              {!isEditing ? (
                <>
                  <button onClick={handleEdit} className="text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors" data-testid="artifact-edit-button">
                    ✎ Edit
                  </button>
                </>
              ) : (
                <>
                  {dirty && <span className="text-xs" style={{ color: 'var(--color-warning)' }}>unsaved</span>}
                  <button onClick={handleSave} disabled={!dirty || isSaving} className="text-xs text-[var(--color-success)] hover:opacity-80 disabled:opacity-30 transition-opacity" data-testid="artifact-save-button">
                    {isSaving ? 'Saving...' : '✓ Save'}
                  </button>
                  <button onClick={handleCancel} className="text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors" data-testid="artifact-cancel-edit">
                    Cancel
                  </button>
                </>
              )}
            </div>
          </div>

          {/* Content */}
          {contentLoading && (
            <div className="flex items-center justify-center py-6">
              <div className="animate-spin rounded-full h-5 w-5 border-2 border-t-transparent" style={{ borderColor: 'var(--color-accent)', borderTopColor: 'transparent' }} />
              <span className="ml-2 text-sm text-[var(--color-text-tertiary)]">Loading...</span>
            </div>
          )}
          {!contentLoading && isEditing && (
            <textarea
              value={editedContent}
              onChange={(e) => { setEditedContent(e.target.value); setDirty(true); }}
              className="w-full p-4 font-mono text-sm bg-[var(--color-surface)] text-[var(--color-text-primary)] border-none outline-none resize-y"
              style={{ minHeight: '400px', fontFamily: 'var(--font-mono)' }}
              data-testid="artifact-editor"
            />
          )}
          {!contentLoading && !isEditing && content && (
            <div className="prose prose-sm dark:prose-invert max-w-none p-4 overflow-auto max-h-[500px]" data-testid="artifact-markdown">
              <ReactMarkdown rehypePlugins={[rehypeHighlight]}>{content}</ReactMarkdown>
            </div>
          )}
          {!contentLoading && !isEditing && !content && (
            <div className="p-4 text-center" data-testid="artifact-not-found">
              <p className="text-sm text-[var(--color-text-tertiary)]">Content not available.</p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}