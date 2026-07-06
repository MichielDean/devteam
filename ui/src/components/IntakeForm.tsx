import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { listRepos } from '../api/client';
import type { CreateFeatureRequest, ScopeName } from '../types';
import { SCOPES, SCOPE_LABELS, SCOPE_DESCRIPTIONS, DEPTH_DESCRIPTIONS } from '../types';

interface IntakeFormProps {
  onSubmit: (req: CreateFeatureRequest, startImmediately: boolean) => void;
  onCancel: () => void;
  isLoading: boolean;
}

const inputClass =
  'w-full px-3 py-2 rounded-[var(--radius-md)] bg-[var(--color-surface-raised)] text-[var(--color-text-primary)] border border-[var(--color-border-subtle)] focus:border-[var(--color-accent)] focus:outline-none transition-colors text-sm';
const labelClass = 'block text-xs font-medium text-[var(--color-text-secondary)] mb-1.5';

// Scopes that require implementation repos
const REPO_REQUIRED_SCOPES = ['feature', 'enterprise', 'mvp', 'infra', 'security-patch'];

function detectScope(text: string): ScopeName {
  const lower = text.toLowerCase();
  const wordCount = text.trim().split(/\s+/).length;

  const checks: { keywords: string[]; scope: ScopeName; specific: boolean }[] = [
    { keywords: ['cve', 'vulnerability', 'security patch', 'security-patch'], scope: 'security-patch', specific: true },
    { keywords: ['proof of concept', 'prototype', 'poc', 'spike'], scope: 'poc', specific: true },
    { keywords: ['mvp', 'minimum viable'], scope: 'mvp', specific: true },
    { keywords: ['workshop', 'lab', 'training'], scope: 'workshop', specific: true },
    { keywords: ['infrastructure', 'deploy', 'infra'], scope: 'infra', specific: false },
    { keywords: ['refactor', 'clean up', 'simplify', 'restructure'], scope: 'refactor', specific: false },
    { keywords: ['fix', 'bug', 'broken', 'error', 'crash', 'panic'], scope: 'bugfix', specific: false },
  ];

  for (const check of checks) {
    for (const kw of check.keywords) {
      if (lower.includes(kw)) {
        if (wordCount >= 5 && !check.specific) return 'feature';
        return check.scope;
      }
    }
  }
  return 'feature';
}

export default function IntakeForm({ onSubmit, onCancel, isLoading }: IntakeFormProps) {
  const [type, setType] = useState<'loose_idea' | 'external_spec'>('loose_idea');
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [priority, setPriority] = useState(2);
  const [scope, setScope] = useState<ScopeName | ''>('');
  const [depth, setDepth] = useState('');
  const [executionMode, setExecutionMode] = useState('');
  const [fileContent, setFileContent] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [startImmediately, setStartImmediately] = useState(false);
  const [selectedRepos, setSelectedRepos] = useState<Set<string>>(new Set());

  const { data: repos = [] } = useQuery({
    queryKey: ['repos'],
    queryFn: listRepos,
  });

  const detectedScope = useMemo(() => {
    if (scope) return null;
    const text = title + ' ' + description;
    if (!text.trim()) return null;
    return detectScope(text);
  }, [title, description, scope]);

  const effectiveScope = scope || detectedScope || 'feature';
  const needsRepos = REPO_REQUIRED_SCOPES.includes(effectiveScope);

  const toggleRepo = (name: string) => {
    setSelectedRepos((prev) => {
      const next = new Set(prev);
      if (next.has(name)) {
        next.delete(name);
      } else {
        next.add(name);
      }
      return next;
    });
  };

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};
    if (!title.trim()) newErrors.title = 'Title is required';
    else if (title.length > 200) newErrors.title = 'Title must be 200 characters or less';
    if (!description.trim()) newErrors.description = 'Description is required';
    else if (description.length > 10000) newErrors.description = 'Description must be 10,000 characters or less';
    if (type === 'external_spec' && !fileContent) newErrors.file = 'File is required for external spec';
    if (needsRepos && selectedRepos.size === 0) {
      newErrors.repos = `This scope (${effectiveScope}) requires at least one implementation repository. Select a repo below.`;
    }
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    const req: CreateFeatureRequest = {
      type,
      title: title.trim(),
      description: description.trim(),
      priority,
    };
    if (scope) req.scope = scope;
    if (depth) req.depth = depth;
    if (executionMode) req.execution_mode = executionMode;
    if (type === 'external_spec' && fileContent) req.file_content = fileContent;

    // Add selected repos
    if (selectedRepos.size > 0) {
      req.repos = repos
        .filter((r) => selectedRepos.has(r.name))
        .map((r) => ({ name: r.name, url: r.url, branch: '' }));
    }

    onSubmit(req, startImmediately);
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      const result = reader.result as string;
      const base64 = result.split(',')[1] || result;
      setFileContent(base64);
    };
    reader.readAsDataURL(file);
  };

  const typeBtn = (active: boolean) =>
    `px-4 py-2 rounded-[var(--radius-md)] text-sm font-medium transition-colors ${
      active
        ? 'bg-[var(--color-accent)] text-white'
        : 'bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)] hover:bg-[var(--color-surface-active)]'
    }`;

  return (
    <div className="rounded-[var(--radius-lg)] p-6 mb-6" style={{ backgroundColor: 'var(--color-surface-raised)', boxShadow: 'var(--shadow-md)' }} data-testid="intake-form">
      <h3 className="text-lg font-medium text-[var(--color-text-primary)] mb-4">What do you want built?</h3>
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className={labelClass}>Type</label>
          <div className="flex gap-2">
            <button type="button" onClick={() => setType('loose_idea')} className={typeBtn(type === 'loose_idea')} data-testid="type-loose-idea">Loose Idea</button>
            <button type="button" onClick={() => setType('external_spec')} className={typeBtn(type === 'external_spec')} data-testid="type-external-spec">External Spec</button>
          </div>
        </div>

        <div>
          <label htmlFor="title" className={labelClass}>Title</label>
          <input id="title" type="text" value={title} onChange={(e) => setTitle(e.target.value)} maxLength={200} className={inputClass} placeholder="Feature title..." data-testid="title-input" />
          {errors.title && <p className="mt-1 text-sm" style={{ color: 'var(--color-danger)' }} data-testid="title-error">{errors.title}</p>}
          <p className="mt-1 text-xs text-[var(--color-text-tertiary)]">{title.length}/200</p>
        </div>

        <div>
          <label htmlFor="description" className={labelClass}>Description</label>
          <textarea id="description" value={description} onChange={(e) => setDescription(e.target.value)} maxLength={10000} rows={5} className={`${inputClass} resize-y`} placeholder="Describe your feature idea..." data-testid="description-input" />
          {errors.description && <p className="mt-1 text-sm" style={{ color: 'var(--color-danger)' }} data-testid="description-error">{errors.description}</p>}
          <p className="mt-1 text-xs text-[var(--color-text-tertiary)]">{description.length}/10000</p>
        </div>

        {/* Implementation Repositories */}
        <div>
          <label className={labelClass}>
            Implementation Repositories {needsRepos && <span style={{ color: 'var(--color-warning)' }}>*</span>}
          </label>
          <p className="text-xs text-[var(--color-text-tertiary)] mb-2">
            {needsRepos
              ? 'This scope requires at least one repo — agents will write code here.'
              : 'Optional — select repos if code changes are needed. Spec-only features can skip this.'}
          </p>
          <div className="space-y-1.5" data-testid="repo-selector">
            {repos.length === 0 && (
              <p className="text-xs text-[var(--color-text-tertiary)]">No repos found in repos.yaml</p>
            )}
            {repos.map((repo) => {
              const isSelected = selectedRepos.has(repo.name);
              return (
                <label
                  key={repo.name}
                  className={`flex items-start gap-3 p-3 rounded-[var(--radius-md)] cursor-pointer transition-colors ${
                    isSelected
                      ? 'bg-[var(--color-surface-active)] border border-[var(--color-accent)]'
                      : 'bg-[var(--color-surface)] border border-[var(--color-border-subtle)] hover:bg-[var(--color-surface-hover)]'
                  }`}
                  data-testid={`repo-option-${repo.name}`}
                >
                  <input
                    type="checkbox"
                    checked={isSelected}
                    onChange={() => toggleRepo(repo.name)}
                    className="mt-0.5 accent-[var(--color-accent)]"
                    data-testid={`repo-checkbox-${repo.name}`}
                  />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium text-[var(--color-text-primary)]">{repo.name}</span>
                      {repo.primary && (
                        <span className="text-xs px-1.5 py-0.5 rounded bg-[var(--color-accent)] text-white" data-testid={`repo-primary-${repo.name}`}>primary</span>
                      )}
                    </div>
                    <p className="text-xs text-[var(--color-text-tertiary)] truncate">{repo.description}</p>
                    <p className="text-xs text-[var(--color-text-tertiary)] font-mono truncate">{repo.url}</p>
                  </div>
                </label>
              );
            })}
          </div>
          {errors.repos && <p className="mt-1 text-sm" style={{ color: 'var(--color-danger)' }} data-testid="repos-error">{errors.repos}</p>}
          {selectedRepos.size > 0 && (
            <p className="mt-1 text-xs text-[var(--color-text-tertiary)]">
              {selectedRepos.size} repo{selectedRepos.size > 1 ? 's' : ''} selected
            </p>
          )}
        </div>

        <div>
          <label htmlFor="scope" className={labelClass}>Scope — controls which stages run</label>
          <select id="scope" value={scope} onChange={(e) => setScope(e.target.value as ScopeName | '')} className={inputClass} data-testid="scope-select">
            <option value="">Auto-detect{detectedScope ? ` (${SCOPE_LABELS[detectedScope]})` : ''}</option>
            {SCOPES.map((s) => <option key={s} value={s}>{SCOPE_LABELS[s]} — {SCOPE_DESCRIPTIONS[s]}</option>)}
          </select>
          <p className="mt-1.5 text-xs text-[var(--color-text-tertiary)]" data-testid="scope-hint">
            {scope ? SCOPE_DESCRIPTIONS[scope] : detectedScope ? `Auto-detected: ${SCOPE_DESCRIPTIONS[detectedScope]}` : 'Scope determines which stages execute. Auto-detects from your description.'}
          </p>
        </div>

        <div>
          <label htmlFor="depth" className={labelClass}>Depth — controls artifact detail level</label>
          <select id="depth" value={depth} onChange={(e) => setDepth(e.target.value)} className={inputClass} data-testid="depth-select">
            <option value="">Default for scope</option>
            <option value="minimal">Minimal — 1-2 page artifacts, key decisions only</option>
            <option value="standard">Standard — Complete artifacts, all required sections</option>
            <option value="comprehensive">Comprehensive — Full enterprise detail, compliance matrices</option>
          </select>
          <p className="mt-1.5 text-xs text-[var(--color-text-tertiary)]" data-testid="depth-hint">
            {depth ? DEPTH_DESCRIPTIONS[depth] : 'Depth controls how detailed each artifact is. Does NOT skip stages — use Scope for that.'}
          </p>
        </div>

        <div>
          <label htmlFor="execution-mode" className={labelClass}>Execution Mode (optional)</label>
          <select
            id="execution-mode"
            value={executionMode}
            onChange={(e) => setExecutionMode(e.target.value)}
            className={inputClass}
            data-testid="execution-mode-select"
          >
            <option value="">Use default (Human in the Loop)</option>
            <option value="human">Human in the Loop — approve every stage</option>
            <option value="guided">Partially Autonomous — auto-run, review at phase gates</option>
            <option value="autonomous">Fully Autonomous — no human interaction</option>
          </select>
        </div>

        <div>
          <label htmlFor="priority" className={labelClass}>Priority</label>
          <select id="priority" value={priority} onChange={(e) => setPriority(Number(e.target.value))} className={inputClass} data-testid="priority-select">
            <option value={1}>P1 - Critical</option>
            <option value={2}>P2 - Medium</option>
            <option value={3}>P3 - Low</option>
          </select>
        </div>

        {type === 'external_spec' && (
          <div>
            <label htmlFor="file" className={labelClass}>Spec File</label>
            <input id="file" type="file" onChange={handleFileChange} accept=".md,.txt,.markdown" className="w-full text-sm text-[var(--color-text-tertiary)] file:mr-3 file:py-1.5 file:px-3 file:rounded-[var(--radius-md)] file:border-0 file:text-sm file:font-medium file:bg-[var(--color-accent)] file:text-white hover:file:opacity-90" data-testid="file-input" />
            {errors.file && <p className="mt-1 text-sm" style={{ color: 'var(--color-danger)' }} data-testid="file-error">{errors.file}</p>}
          </div>
        )}

        <div className="flex items-center gap-3 pt-2">
          <button type="submit" onClick={() => setStartImmediately(false)} disabled={isLoading} className="px-4 py-2.5 rounded-[var(--radius-md)] text-sm font-medium bg-[var(--color-surface-hover)] text-[var(--color-text-primary)] hover:bg-[var(--color-surface-active)] disabled:opacity-50 disabled:cursor-not-allowed transition-colors" data-testid="add-button">{isLoading && !startImmediately ? 'Adding...' : 'Add'}</button>
          <button type="submit" onClick={() => setStartImmediately(true)} disabled={isLoading} className="px-5 py-2.5 rounded-[var(--radius-md)] text-sm font-semibold text-white bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] disabled:opacity-50 disabled:cursor-not-allowed transition-colors" style={{ boxShadow: 'var(--shadow-sm)' }} data-testid="submit-button">{isLoading && startImmediately ? 'Creating...' : 'Add & Start'}</button>
          <button type="button" onClick={onCancel} className="px-4 py-2 text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors" data-testid="cancel-button">Cancel</button>
        </div>
        <p className="text-xs text-[var(--color-text-tertiary)] mt-2"><strong className="text-[var(--color-text-secondary)]">Add</strong> creates the feature. <strong className="text-[var(--color-text-secondary)]">Add & Start</strong> runs the first stage immediately.</p>
      </form>
    </div>
  );
}