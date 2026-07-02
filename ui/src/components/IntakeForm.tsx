import { useState, useMemo } from 'react';
import type { CreateFeatureRequest, ScopeName } from '../types';
import { SCOPES, SCOPE_LABELS, SCOPE_DESCRIPTIONS } from '../types';

interface IntakeFormProps {
  onSubmit: (req: CreateFeatureRequest, startImmediately: boolean) => void;
  onCancel: () => void;
  isLoading: boolean;
}

// Client-side scope auto-detection (mirrors backend stage.DetectScope)
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
  const [fileContent, setFileContent] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [startImmediately, setStartImmediately] = useState(false);

  // Auto-detect scope from title + description
  const detectedScope = useMemo(() => {
    if (scope) return null; // user overrode
    const text = title + ' ' + description;
    if (!text.trim()) return null;
    const detected = detectScope(text);
    return detected;
  }, [title, description, scope]);

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};
    if (!title.trim()) newErrors.title = 'Title is required';
    else if (title.length > 200) newErrors.title = 'Title must be 200 characters or less';
    if (!description.trim()) newErrors.description = 'Description is required';
    else if (description.length > 10000) newErrors.description = 'Description must be 10,000 characters or less';
    if (type === 'external_spec' && !fileContent) newErrors.file = 'File is required for external spec';
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
    if (type === 'external_spec' && fileContent) req.file_content = fileContent;
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

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6" data-testid="intake-form">
      <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">What do you want built?</h3>
      <form onSubmit={handleSubmit} className="space-y-4">
        {/* Type */}
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Type</label>
          <div className="flex gap-2">
            <button type="button" onClick={() => setType('loose_idea')} className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${type === 'loose_idea' ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'}`} data-testid="type-loose-idea">Loose Idea</button>
            <button type="button" onClick={() => setType('external_spec')} className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${type === 'external_spec' ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'}`} data-testid="type-external-spec">External Spec</button>
          </div>
        </div>

        {/* Title */}
        <div>
          <label htmlFor="title" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Title</label>
          <input id="title" type="text" value={title} onChange={(e) => setTitle(e.target.value)} maxLength={200} className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent" placeholder="Feature title..." data-testid="title-input" />
          {errors.title && <p className="mt-1 text-sm text-red-600" data-testid="title-error">{errors.title}</p>}
          <p className="mt-1 text-xs text-gray-500">{title.length}/200</p>
        </div>

        {/* Description */}
        <div>
          <label htmlFor="description" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Description</label>
          <textarea id="description" value={description} onChange={(e) => setDescription(e.target.value)} maxLength={10000} rows={5} className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-y" placeholder="Describe your feature idea..." data-testid="description-input" />
          {errors.description && <p className="mt-1 text-sm text-red-600" data-testid="description-error">{errors.description}</p>}
          <p className="mt-1 text-xs text-gray-500">{description.length}/10000</p>
        </div>

        {/* Scope Selector with Auto-Detect */}
        <div>
          <label htmlFor="scope" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Scope</label>
          <select id="scope" value={scope} onChange={(e) => setScope(e.target.value as ScopeName | '')} className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500" data-testid="scope-select">
            <option value="">Auto-detect{detectedScope ? ` (${SCOPE_LABELS[detectedScope]})` : ''}</option>
            {SCOPES.map((s) => <option key={s} value={s}>{SCOPE_LABELS[s]}</option>)}
          </select>
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400" data-testid="scope-hint">
            {scope ? SCOPE_DESCRIPTIONS[scope] : detectedScope ? `Auto-detected: ${SCOPE_DESCRIPTIONS[detectedScope]}` : 'Scope determines how many stages run. Type a description to see auto-detection.'}
          </p>
        </div>

        {/* Depth */}
        <div>
          <label htmlFor="depth" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Depth (optional)</label>
          <select id="depth" value={depth} onChange={(e) => setDepth(e.target.value)} className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500" data-testid="depth-select">
            <option value="">Default for scope</option>
            <option value="minimal">Minimal — core essentials</option>
            <option value="standard">Standard — complete artifacts</option>
            <option value="comprehensive">Comprehensive — full enterprise detail</option>
          </select>
        </div>

        {/* Priority */}
        <div>
          <label htmlFor="priority" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Priority</label>
          <select id="priority" value={priority} onChange={(e) => setPriority(Number(e.target.value))} className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500" data-testid="priority-select">
            <option value={1}>P1 - Critical</option>
            <option value={2}>P2 - Medium</option>
            <option value={3}>P3 - Low</option>
          </select>
        </div>

        {/* File upload */}
        {type === 'external_spec' && (
          <div>
            <label htmlFor="file" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Spec File</label>
            <input id="file" type="file" onChange={handleFileChange} accept=".md,.txt,.markdown" className="w-full text-sm text-gray-500 dark:text-gray-400 file:mr-4 file:py-2 file:px-4 file:rounded-lg file:border-0 file:text-sm file:font-medium file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100 dark:file:bg-blue-900 dark:file:text-blue-200" data-testid="file-input" />
            {errors.file && <p className="mt-1 text-sm text-red-600" data-testid="file-error">{errors.file}</p>}
          </div>
        )}

        {/* Actions */}
        <div className="flex items-center gap-3">
          <button type="submit" onClick={() => setStartImmediately(false)} disabled={isLoading} className="px-4 py-2.5 bg-gray-600 text-white rounded-lg hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm font-medium" data-testid="add-button">{isLoading && !startImmediately ? 'Adding...' : 'Add'}</button>
          <button type="submit" onClick={() => setStartImmediately(true)} disabled={isLoading} className="px-6 py-2.5 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm font-semibold shadow-sm" data-testid="submit-button">{isLoading && startImmediately ? 'Creating...' : 'Add & Start'}</button>
          <button type="button" onClick={onCancel} className="px-4 py-2 text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 transition-colors text-sm" data-testid="cancel-button">Cancel</button>
        </div>
        <p className="text-xs text-gray-500 dark:text-gray-400 mt-2"><strong>Add</strong> creates the feature. <strong>Add & Start</strong> runs the first stage immediately.</p>
      </form>
    </div>
  );
}