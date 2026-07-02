import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { listAllKnowledge, saveKnowledge, deleteKnowledge, ApiError } from '../api/client';
import { useToast } from './Toast';
import { AGENTS, REVIEWERS, AGENT_LABELS } from '../types';
import type { TeamKnowledge } from '../types';

const ALL_AGENTS = [...AGENTS, ...REVIEWERS];

export default function KnowledgeEditor() {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [selectedAgent, setSelectedAgent] = useState<string>('');
  const [topic, setTopic] = useState('');
  const [content, setContent] = useState('');
  const [editingTopic, setEditingTopic] = useState<string | null>(null);

  const { data: knowledge, isLoading } = useQuery({
    queryKey: ['knowledge'],
    queryFn: listAllKnowledge,
  });

  const saveMutation = useMutation({
    mutationFn: ({ agent, topic, content }: { agent: string; topic: string; content: string }) =>
      saveKnowledge(agent, topic, content),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['knowledge'] });
      setTopic('');
      setContent('');
      setEditingTopic(null);
      addToast('success', 'Knowledge saved');
    },
    onError: (err: Error) => {
      const msg = err instanceof ApiError ? err.details || err.message : err.message;
      addToast('error', `Save failed: ${msg}`);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: ({ agent, topic }: { agent: string; topic: string }) => deleteKnowledge(agent, topic),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['knowledge'] });
      addToast('success', 'Knowledge deleted');
    },
    onError: (err: Error) => addToast('error', `Delete failed: ${err.message}`),
  });

  const handleEdit = (agent: string, entry: TeamKnowledge) => {
    setSelectedAgent(agent);
    setTopic(entry.topic);
    setContent(entry.content);
    setEditingTopic(entry.topic);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedAgent || !topic.trim() || !content.trim()) return;
    saveMutation.mutate({ agent: selectedAgent, topic: topic.trim(), content: content.trim() });
  };

  const agentEntries = selectedAgent ? (knowledge?.[selectedAgent] ?? []) : [];

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6" data-testid="knowledge-editor">
      <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Team Knowledge</h3>

      <form onSubmit={handleSubmit} className="space-y-4 mb-6">
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Agent</label>
          <select
            value={selectedAgent}
            onChange={(e) => { setSelectedAgent(e.target.value); setEditingTopic(null); }}
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"
            data-testid="knowledge-agent-select"
          >
            <option value="">Select an agent...</option>
            {ALL_AGENTS.map((a) => <option key={a} value={a}>{AGENT_LABELS[a] || a}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Topic</label>
          <input
            type="text"
            value={topic}
            onChange={(e) => setTopic(e.target.value)}
            placeholder="e.g. coding-standards, api-conventions"
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"
            data-testid="knowledge-topic-input"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Content</label>
          <textarea
            value={content}
            onChange={(e) => setContent(e.target.value)}
            rows={5}
            placeholder="Knowledge content..."
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm resize-y"
            data-testid="knowledge-content-input"
          />
        </div>
        <div className="flex gap-2">
          <button
            type="submit"
            disabled={!selectedAgent || !topic.trim() || !content.trim() || saveMutation.isPending}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-sm font-medium"
            data-testid="knowledge-save-button"
          >
            {editingTopic ? 'Update' : 'Add'} Knowledge
          </button>
          {editingTopic && (
            <button
              type="button"
              onClick={() => { setTopic(''); setContent(''); setEditingTopic(null); }}
              className="px-4 py-2 text-gray-600 dark:text-gray-400 text-sm"
              data-testid="knowledge-cancel-edit"
            >
              Cancel
            </button>
          )}
        </div>
      </form>

      {selectedAgent && agentEntries.length > 0 && (
        <div className="space-y-2" data-testid="knowledge-list">
          <h4 className="text-sm font-semibold text-gray-700 dark:text-gray-300">Existing entries for {AGENT_LABELS[selectedAgent]}</h4>
          {agentEntries.map((entry) => (
            <div
              key={entry.id}
              className="flex items-start justify-between p-3 bg-gray-50 dark:bg-gray-900/30 rounded-lg border border-gray-200 dark:border-gray-700"
              data-testid={`knowledge-entry-${entry.topic}`}
            >
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-gray-900 dark:text-white">{entry.topic}</p>
                <p className="text-xs text-gray-500 dark:text-gray-400 mt-1 line-clamp-2">{entry.content}</p>
                <p className="text-xs text-gray-400 mt-1">Updated {new Date(entry.updated_at).toLocaleDateString()}</p>
              </div>
              <div className="flex gap-1 ml-2 shrink-0">
                <button
                  onClick={() => handleEdit(selectedAgent, entry)}
                  className="px-2 py-1 text-xs text-blue-600 dark:text-blue-400 hover:underline"
                  data-testid={`knowledge-edit-${entry.topic}`}
                >
                  Edit
                </button>
                <button
                  onClick={() => deleteMutation.mutate({ agent: selectedAgent, topic: entry.topic })}
                  className="px-2 py-1 text-xs text-red-600 dark:text-red-400 hover:underline"
                  data-testid={`knowledge-delete-${entry.topic}`}
                >
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {isLoading && <p className="text-sm text-gray-500">Loading...</p>}
    </div>
  );
}