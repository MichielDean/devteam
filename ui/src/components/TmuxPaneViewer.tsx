import { useEffect, useRef, useState } from 'react';
import { useParams, Link } from 'react-router';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { Button, Card } from '../ui/primitives';
import { getCapturePane, killSession } from '../api/client';

export default function TmuxPaneViewer() {
  const { id, phase } = useParams<{ id: string; phase: string }>();
  const terminalRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const [alive, setAlive] = useState(true);

  useEffect(() => {
    if (!terminalRef.current || !id || !phase) return;

    const term = new Terminal({
      fontSize: 13,
      fontFamily: 'monospace',
      theme: { background: '#0a0a0a', foreground: '#e0e0e0' },
      scrollback: 10000,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(terminalRef.current);
    fit.fit();
    termRef.current = term;
    fitRef.current = fit;

    let pollInterval: number | undefined;

    const poll = async () => {
      try {
        const output = await getCapturePane(id, phase);
        if (output) {
          term.write(output);
          setAlive(true);
        }
      } catch {
        setAlive(false);
      }
    };

    poll();
    pollInterval = window.setInterval(poll, 500);

    const resizeHandler = () => fit.fit();
    window.addEventListener('resize', resizeHandler);

    return () => {
      window.removeEventListener('resize', resizeHandler);
      if (pollInterval) clearInterval(pollInterval);
      term.dispose();
    };
  }, [id, phase]);

  const handleKill = async () => {
    if (!id || !phase) return;
    if (window.confirm('Kill this tmux session?')) {
      await killSession(id, phase);
      setAlive(false);
    }
  };

  return (
    <div data-testid="tmux-pane-viewer">
      <div className="mb-4">
        <Link to={`/features/${id}/sessions`} className="text-blue-600 dark:text-blue-400 hover:underline text-sm">&larr; Back to Sessions</Link>
      </div>
      <Card className="p-4">
        <div className="flex items-center justify-between mb-3">
          <div>
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">tmux: devteam-{id?.substring(0, 8)}-{phase}</h2>
            <span className={`text-sm ${alive ? 'text-green-600' : 'text-red-600'}`} data-testid="pane-alive-status">
              {alive ? '● Live' : '✗ Session not alive'}
            </span>
          </div>
          {alive && <Button variant="danger" size="sm" onClick={handleKill} data-testid="pane-kill">Kill Session</Button>}
        </div>
        <div ref={terminalRef} className="bg-black rounded-lg p-2" style={{ minHeight: '500px' }} data-testid="xterm-container" />
      </Card>
    </div>
  );
}