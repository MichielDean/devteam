import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams, Link } from 'react-router';
import { listSessions, killSession, resumeSession, getSessionOutput } from '../api/client';
import { Button, Badge, Card } from '../ui/primitives';
import { useToast } from './Toast';
import { useState, useEffect } from 'react';

const STATE_COLORS: Record<string, 'gray' | 'blue' | 'green' | 'yellow' | 'red' | 'orange' | 'indigo'> = {
  created: 'gray',
  running: 'blue',
  awaiting_gate: 'yellow',
  awaiting_question: 'yellow',
  resuming: 'indigo',
  done: 'green',
  failed: 'red',
  expired: 'gray',
};

export default function SessionView() {
  const { id } = useParams<{ id: string }>();
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [outputForSession, setOutputForSession] = useState<string | null>(null);
  const [viewingPhase, setViewingPhase] = useState<string | null>(null);

  const { data: sessions = [] } = useQuery({
    queryKey: ['sessions', id!],
    queryFn: () => listSessions(id!),
    enabled: !!id,
    refetchInterval: 2000,
  });

  const killMutation = useMutation({
    mutationFn: ({ phase }: { phase: string }) => killSession(id!, phase),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['sessions', id!] }); addToast('success', 'Session killed'); },
    onError: (err: Error) => addToast('error', err.message),
  });

  const resumeMutation = useMutation({
    mutationFn: ({ phase }: { phase: string }) => resumeSession(id!, phase),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['sessions', id!] }); queryClient.invalidateQueries({ queryKey: ['feature', id!] }); addToast('success', 'Session resuming'); },
    onError: (err: Error) => addToast('error', err.message),
  });

  const viewOutput = async (phase: string) => {
    const output = await getSessionOutput(id!, phase);
    setOutputForSession(output);
    setViewingPhase(phase);
  };

  useEffect(() => {
    if (viewingPhase) {
      const interval = setInterval(async () => {
        const output = await getSessionOutput(id!, viewingPhase);
        setOutputForSession(output);
      }, 2000);
      return () => clearInterval(interval);
    }
  }, [viewingPhase, id]);

  return (
    <div data-testid="session-view">
      <div className="mb-4">
        <Link to={`/features/${id}`} className="text-blue-600 dark:text-blue-400 hover:underline text-sm">&larr; Back to Feature</Link>
      </div>
      <h2 className="text-2xl font-bold text-gray-900 dark:text-white mb-4">Tmux Sessions</h2>

      {sessions.length === 0 ? (
        <p className="text-sm text-gray-500" data-testid="sessions-empty">No sessions yet.</p>
      ) : (
        <div className="space-y-3" data-testid="sessions-list">
          {sessions.map((s) => (
            <Card key={s.id} className="p-4" data-testid={`session-${s.phase}`}>
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-semibold text-gray-900 dark:text-white">
                    {s.phase} {s.bolt_number > 0 && <span className="text-gray-400">· Bolt {s.bolt_number}</span>}
                  </h3>
                  <div className="flex items-center gap-2 mt-1">
                    <Badge color={STATE_COLORS[s.state] || 'gray'} data-testid={`session-state-${s.phase}`}>{s.state}</Badge>
                    {s.is_alive && <Badge color="green">Alive</Badge>}
                    {s.last_agent && <Badge color="blue">{s.last_agent}</Badge>}
                    {s.stage_id && <Badge color="gray">{s.stage_id}</Badge>}
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  {s.is_alive && s.state === 'awaiting_question' && (
                    <Button variant="primary" size="sm" onClick={() => resumeMutation.mutate({ phase: s.phase })} data-testid={`session-resume-${s.phase}`}>Resume</Button>
                  )}
                  <Link to={`/features/${id}/sessions/${s.phase}/pane`}>
                    <Button variant="ghost" size="sm" data-testid={`session-pane-${s.phase}`}>View Pane</Button>
                  </Link>
                  <Button variant="ghost" size="sm" onClick={() => viewOutput(s.phase)} data-testid={`session-output-${s.phase}`}>Output</Button>
                  {s.is_alive && (
                    <Button variant="danger" size="sm" onClick={() => killMutation.mutate({ phase: s.phase })} data-testid={`session-kill-${s.phase}`}>Kill</Button>
                  )}
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}

      {outputForSession !== null && (
        <Card className="p-4 mt-4" data-testid="session-output-view">
          <div className="flex items-center justify-between mb-2">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Output: {viewingPhase}</h3>
            <Button variant="ghost" size="sm" onClick={() => { setOutputForSession(null); setViewingPhase(null); }}>Close</Button>
          </div>
          <pre className="bg-gray-950 text-gray-300 p-3 rounded text-xs overflow-x-auto max-h-96 overflow-y-auto" data-testid="session-output-content">{outputForSession || 'No output'}</pre>
        </Card>
      )}
    </div>
  );
}