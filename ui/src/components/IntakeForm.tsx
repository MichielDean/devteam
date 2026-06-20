import { useState } from 'react';
import type { CreateFeatureRequest } from '../types';

interface IntakeFormProps {
  onSubmit: (req: CreateFeatureRequest) => void;
  onCancel: () => void;
  isLoading: boolean;
}

export default function IntakeForm({ onSubmit, onCancel, isLoading }: IntakeFormProps) {
  const [type, setType] = useState<'loose_idea' | 'external_spec'>('loose_idea');
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [priority, setPriority] = useState(2);
  const [fileContent, setFileContent] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!title.trim()) {
      newErrors.title = 'Title is required';
    } else if (title.length > 200) {
      newErrors.title = 'Title must be 200 characters or less';
    }

    if (!description.trim()) {
      newErrors.description = 'Description is required';
    } else if (description.length > 10000) {
      newErrors.description = 'Description must be 10,000 characters or less';
    }

    if (type === 'external_spec' && !fileContent) {
      newErrors.file = 'File is required for external spec';
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

    if (type === 'external_spec' && fileContent) {
      req.file_content = fileContent;
    }

    onSubmit(req);
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = () => {
      const result = reader.result as string;
      // Remove data URL prefix (e.g., "data:text/markdown;base64,")
      const base64 = result.split(',')[1] || result;
      setFileContent(base64);
    };
    reader.readAsDataURL(file);
  };

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6" data-testid="intake-form">
      <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Create New Feature</h3>

      <form onSubmit={handleSubmit} className="space-y-4">
        {/* Type Toggle */}
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
            Type
          </label>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setType('loose_idea')}
              className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                type === 'loose_idea'
                  ? 'bg-blue-600 text-white'
                  : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
              }`}
              data-testid="type-loose-idea"
            >
              Loose Idea
            </button>
            <button
              type="button"
              onClick={() => setType('external_spec')}
              className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                type === 'external_spec'
                  ? 'bg-blue-600 text-white'
                  : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
              }`}
              data-testid="type-external-spec"
            >
              External Spec
            </button>
          </div>
        </div>

        {/* Title */}
        <div>
          <label htmlFor="title" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Title
          </label>
          <input
            id="title"
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            maxLength={200}
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            placeholder="Feature title..."
            data-testid="title-input"
          />
          {errors.title && <p className="mt-1 text-sm text-red-600" data-testid="title-error">{errors.title}</p>}
          <p className="mt-1 text-xs text-gray-500">{title.length}/200</p>
        </div>

        {/* Description */}
        <div>
          <label htmlFor="description" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Description
          </label>
          <textarea
            id="description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            maxLength={10000}
            rows={5}
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-y"
            placeholder="Describe your feature idea..."
            data-testid="description-input"
          />
          {errors.description && <p className="mt-1 text-sm text-red-600" data-testid="description-error">{errors.description}</p>}
          <p className="mt-1 text-xs text-gray-500">{description.length}/10000</p>
        </div>

        {/* Priority */}
        <div>
          <label htmlFor="priority" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Priority
          </label>
          <select
            id="priority"
            value={priority}
            onChange={(e) => setPriority(Number(e.target.value))}
            className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
            data-testid="priority-select"
          >
            <option value={1}>P1 - Critical</option>
            <option value={2}>P2 - Medium</option>
            <option value={3}>P3 - Low</option>
          </select>
        </div>

        {/* File Upload (for external_spec) */}
        {type === 'external_spec' && (
          <div>
            <label htmlFor="file" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Spec File
            </label>
            <input
              id="file"
              type="file"
              onChange={handleFileChange}
              accept=".md,.txt,.markdown"
              className="w-full text-sm text-gray-500 dark:text-gray-400 file:mr-4 file:py-2 file:px-4 file:rounded-lg file:border-0 file:text-sm file:font-medium file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100 dark:file:bg-blue-900 dark:file:text-blue-200"
              data-testid="file-input"
            />
            {errors.file && <p className="mt-1 text-sm text-red-600" data-testid="file-error">{errors.file}</p>}
          </div>
        )}

        {/* Actions */}
        <div className="flex items-center gap-3">
          <button
            type="submit"
            disabled={isLoading}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm font-medium"
            data-testid="submit-button"
          >
            {isLoading ? 'Creating...' : 'Create Feature'}
          </button>
          <button
            type="button"
            onClick={onCancel}
            className="px-4 py-2 text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 transition-colors text-sm"
            data-testid="cancel-button"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}