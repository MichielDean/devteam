import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { listAllKnowledge, saveKnowledge, deleteKnowledge } from '../api/client';
import { Button, Badge, Card } from '../ui/primitives';
import { useToast } from '../components/Toast';
import { AGENTS, REVIEWERS, AGENT_LABELS } from '../types';

export default function KnowledgePage() {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [selectedAgent, setSelectedAgent] = useState<string>('product');
  const [topic, setTopic] = useState('');
  const [content, setContent] = useState('');

  const { data: allKnowledge = {} } = useQuery({
    queryKey: ['knowledge-all'],
    queryFn: listAllKnowledge,
  });

  const saveMutation = useMutation({
    mutationFn: ({ agent, topic, content }: { agent: string; topic: string; content: string }) => saveKnowledge(agent, topic, content),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['knowledge-all'] });
      addToast('success', 'Knowledge saved');
      setTopic('');
      setContent('');
    },
    onError: (err: Error) => addToast('error', err.message),
  });

  const deleteMutation = useMutation({
    mutationFn: ({ agent, topic }: { agent: string; topic: string }) => deleteKnowledge(agent, topic),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['knowledge-all'] });
      addToast('success', 'Deleted');
    },
    onError: (err: Error) => addToast('error', err.message),
  });

  const agentKnowledge = allKnowledge[selectedAgent] ?? [];

  return (
    <div data-testid="knowledge-page">
      <h2 className="text-2xl font-bold text-gray-900 dark:text-white mb-4">Team Knowledge</h2>
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <Card className="p-4 lg:col-span-1">
          <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Agent</h3>
          <select
            value={selectedAgent}
            onChange={(e) => setSelectedAgent(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"
            data-testid="knowledge-agent-select"
          >
            {[...AGENTS, ...REVIEWERS].map((a) => <option key={a} value={a}>{AGENT_LABELS[a] || a}</option>)}
          </select>

          <div className="mt-4 space-y-3">
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">Topic</label>
              <input
                type="text"
                value={topic}
                onChange={(e) => setTopic(e.target.value)}
                placeholder="e.g. coding-standards"
                className="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                data-testid="knowledge-topic-input"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">Content</label>
              <textarea
                value={content}
                onChange={(e) => setContent(e.target.value)}
                rows={6}
                placeholder="Knowledge content..."
                className="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                data-testid="knowledge-content-input"
              />
            </div>
            <Button variant="primary" size="sm" onClick={() => saveMutation.mutate({ agent: selectedAgent, topic, content })} disabled={!topic || !content} data-testid="knowledge-save-button">
              Save
            </Button>
          </div>
        </Card>

        <Card className="p-4 lg:col-span-2">
          <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
            {AGENT_LABELS[selectedAgent] || selectedAgent} — {agentKnowledge.length} entries
          </h3>
          {agentKnowledge.length === 0 ? (
            <p className="text-sm text-gray-500" data-testid="knowledge-empty">No knowledge entries yet.</p>
          ) : (
            <div className="space-y-3" data-testid="knowledge-list">
              {agentKnowledge.map((k) => (
                <div key={k.id} className="p-3 bg-gray-50 dark:bg-gray-900/30 rounded-lg" data-testid={`knowledge-entry-${k.id}`}>
                  <div className="flex items-center justify-between mb-1">
                    <Badge color="blue">{k.topic}</Badge>
                    <Button variant="danger" size="sm" onClick={() => deleteMutation.mutate({ agent: selectedAgent, topic: k.topic })} data-testid={`knowledge-delete-${k.id}`}>Delete</Button>
                  </div>
                  <p className="text-sm text-gray-700 dark:text-gray-300 whitespace-pre-wrap">{k.content}</p>
                  <p className="text-xs text-gray-400 mt-1">Updated: {new Date(k.updated_at).toLocaleString()}</p>
                </div>
              ))}
            </div>
          )}
        </Card>
      </div>
    </div>
  );
}