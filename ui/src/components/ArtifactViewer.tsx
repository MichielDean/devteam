import { useState, useEffect } from 'react';
import ReactMarkdown from 'react-markdown';
import rehypeHighlight from 'rehype-highlight';
import { getArtifact } from '../api/client';
import { ARTIFACT_DISPLAY_NAMES } from '../types';
import type { PhaseState } from '../types';

interface ArtifactViewerProps {
  featureId: string;
  phaseStates: Record<string, PhaseState>;
}

// API path mapping (short names used in URLs)
const ARTIFACT_API_PATHS: Record<string, string> = {
  input_md: 'input',
  spec_md: 'spec',
  acceptance_md: 'acceptance',
  repos_yaml: 'repos',
  plan_md: 'plan',
  tasks_md: 'tasks',
  review_report: 'review_report',
  test_report: 'test_report',
  docs: 'docs',
};

// Expected artifact types per phase
const PHASE_ARTIFACTS: Record<string, string[]> = {
  inception: ['input_md', 'spec_md', 'acceptance_md', 'repos_yaml'],
  planning: ['plan_md', 'tasks_md'],
  construction: [],
  review: ['review_report'],
  testing: ['test_report'],
  delivery: ['docs'],
};

export default function ArtifactViewer({ featureId, phaseStates }: ArtifactViewerProps) {
  const [selectedArtifact, setSelectedArtifact] = useState<string | null>(null);
  const [content, setContent] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Collect all artifacts from all phases
  const allArtifacts: Array<{ type: string; path: string; generatedBy: string; generatedAt: string; phase: string }> = [];
  const generatedTypes = new Set<string>();

  for (const [phase, state] of Object.entries(phaseStates)) {
    for (const artifact of state.artifacts) {
      allArtifacts.push({ type: artifact.type, path: artifact.path, generatedBy: artifact.generated_by, generatedAt: artifact.generated_at, phase });
      generatedTypes.add(artifact.type);
    }
  }

  // Add expected artifacts that haven't been generated yet
  for (const phase of Object.keys(PHASE_ARTIFACTS)) {
    for (const type of PHASE_ARTIFACTS[phase]) {
      if (!generatedTypes.has(type)) {
        allArtifacts.push({ type, path: '', generatedBy: '', generatedAt: '', phase });
      }
    }
  }

  useEffect(() => {
    if (!selectedArtifact) return;

    const apiPath = ARTIFACT_API_PATHS[selectedArtifact];
    if (!apiPath) return;

    setLoading(true);
    setError(null);

    getArtifact(featureId, apiPath)
      .then((text) => {
        setContent(text);
        setLoading(false);
      })
      .catch((err) => {
        setError(err.message);
        setLoading(false);
      });
  }, [featureId, selectedArtifact]);

  return (
    <div data-testid="artifact-viewer">
      {/* Artifact List */}
      <div className="flex flex-wrap gap-2 mb-4">
        {allArtifacts.map((artifact) => {
          const isGenerated = !!artifact.generatedAt;
          const displayName = ARTIFACT_DISPLAY_NAMES[artifact.type] || artifact.type;
          const isSelected = selectedArtifact === artifact.type;

          return (
            <button
              key={artifact.type}
              onClick={() => setSelectedArtifact(artifact.type)}
              className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                isSelected
                  ? 'bg-blue-600 text-white'
                  : isGenerated
                  ? 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600'
                  : 'bg-gray-50 text-gray-400 dark:bg-gray-800 dark:text-gray-500 cursor-default'
              }`}
              disabled={!isGenerated && !isSelected}
              data-testid={`artifact-tab-${artifact.type}`}
            >
              {displayName}
              {!isGenerated && (
                <span className="ml-1 text-xs opacity-60" data-testid="not-generated-badge">—</span>
              )}
            </button>
          );
        })}
      </div>

      {/* Artifact Content */}
      {selectedArtifact && (
        <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden" data-testid="artifact-content">
          {loading && (
            <div className="flex items-center justify-center py-12">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
              <span className="ml-3 text-gray-500 dark:text-gray-400">Loading...</span>
            </div>
          )}

          {error && (
            <div className="p-4 text-red-600 dark:text-red-400" data-testid="artifact-error">
              Error loading artifact: {error}
            </div>
          )}

          {!loading && !error && content && (
            <div className="prose dark:prose-invert max-w-none p-4 overflow-auto max-h-[600px]" data-testid="artifact-markdown">
              <ReactMarkdown rehypePlugins={[rehypeHighlight]}>
                {content}
              </ReactMarkdown>
            </div>
          )}

          {!loading && !error && !content && selectedArtifact && (
            <div className="p-8 text-center text-gray-500 dark:text-gray-400" data-testid="artifact-not-generated">
              <p className="text-lg font-medium mb-2">Not yet generated</p>
              <p className="text-sm">This artifact will appear once the current phase is completed.</p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}