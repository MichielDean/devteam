import { useSSE } from '../hooks/useSSE';

interface ConnectionStatusProps {
  featureId?: string | null;
}

export default function ConnectionStatus({ featureId }: ConnectionStatusProps) {
  if (!featureId) return null;

  const { connected } = useSSE(featureId);

  if (connected) return null;

  return (
    <div
      className="border-b border-[var(--color-border-subtle)] px-4 py-2 text-center text-sm text-[var(--color-warning)]"
      style={{ backgroundColor: 'var(--color-warning-surface)' }}
      data-testid="connection-lost-banner"
    >
      Connection lost. Attempting to reconnect...
    </div>
  );
}