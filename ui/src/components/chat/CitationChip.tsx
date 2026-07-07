import type { ChatCitation } from '../../types';

// CitationChip renders one citation as a first-class clickable affordance
// (FR-G2-4, NFR-USA-4). Clicking opens the CitationDrawer with the full
// reference. The chip shows file + section; the drawer could show the body
// (fetched from the RAG inspect endpoint) — for MVS the chip + drawer
// shows file/section/lines, which is enough for SC1/SC2.
interface CitationChipProps {
  citation: ChatCitation;
  onClick?: (c: ChatCitation) => void;
}

export function CitationChip({ citation, onClick }: CitationChipProps) {
  return (
    <button
      type="button"
      onClick={() => onClick?.(citation)}
      className="inline-flex items-center gap-1 px-2 py-0.5 text-xs rounded-full transition-colors"
      style={{
        backgroundColor: 'var(--color-surface-hover)',
        color: 'var(--color-text-secondary)',
        border: '1px solid var(--color-border-subtle)',
      }}
      data-testid="chat-citation-chip"
      title={citation.lines ? `${citation.file} §${citation.section} (${citation.lines})` : `${citation.file} §${citation.section}`}
    >
      <span aria-hidden>📄</span>
      <span className="font-mono">{citation.file}</span>
      {citation.section && <span>§{citation.section}</span>}
    </button>
  );
}

// CitationDrawer is a side panel showing the full citation details. Modeled
// on the existing AuditDrawer pattern (interaction-spec chose a side drawer).
import { useEffect } from 'react';

interface CitationDrawerProps {
  citation: ChatCitation | null;
  onClose: () => void;
}

export function CitationDrawer({ citation, onClose }: CitationDrawerProps) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  if (!citation) return null;
  return (
    <>
      <div
        className="fixed inset-0 z-40"
        style={{ backgroundColor: 'rgba(0,0,0,0.4)' }}
        onClick={onClose}
        data-testid="citation-drawer-overlay"
      />
      <aside
        className="fixed right-0 top-0 bottom-0 z-50 w-full max-w-md overflow-y-auto p-6"
        style={{
          backgroundColor: 'var(--color-surface-raised)',
          borderLeft: '1px solid var(--color-border-subtle)',
          boxShadow: 'var(--shadow-lg)',
        }}
        data-testid="citation-drawer"
      >
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold" style={{ color: 'var(--color-text-primary)' }}>
            Citation
          </h2>
          <button
            onClick={onClose}
            className="text-sm px-2 py-1 rounded"
            style={{ color: 'var(--color-text-secondary)' }}
            aria-label="Close citation"
          >
            ✕
          </button>
        </div>
        <dl className="space-y-3 text-sm">
          <div>
            <dt className="font-semibold mb-0.5" style={{ color: 'var(--color-text-tertiary)' }}>File</dt>
            <dd className="font-mono" style={{ color: 'var(--color-text-primary)' }}>{citation.file}</dd>
          </div>
          {citation.section && (
            <div>
              <dt className="font-semibold mb-0.5" style={{ color: 'var(--color-text-tertiary)' }}>Section</dt>
              <dd style={{ color: 'var(--color-text-primary)' }}>{citation.section}</dd>
            </div>
          )}
          {citation.lines && (
            <div>
              <dt className="font-semibold mb-0.5" style={{ color: 'var(--color-text-tertiary)' }}>Lines</dt>
              <dd className="font-mono" style={{ color: 'var(--color-text-primary)' }}>{citation.lines}</dd>
            </div>
          )}
        </dl>
        <p className="mt-6 text-xs" style={{ color: 'var(--color-text-tertiary)' }}>
          This citation grounds the expert's answer in the AIDLC v2 corpus. Open the file directly to read the full context.
        </p>
      </aside>
    </>
  );
}