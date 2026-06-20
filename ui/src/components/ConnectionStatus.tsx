import { useSSE } from '../hooks/useSSE';

interface ConnectionStatusProps {
  featureId?: string;
}

export default function ConnectionStatus({ featureId }: ConnectionStatusProps) {
  const { connected } = useSSE(featureId ?? null);

  if (connected || !featureId) return null;

  return (
    <div
      className="bg-yellow-100 dark:bg-yellow-900 border-b border-yellow-200 dark:border-yellow-800 px-4 py-2 text-center text-sm text-yellow-800 dark:text-yellow-200"
      data-testid="connection-lost-banner"
    >
      Connection lost. Attempting to reconnect...
    </div>
  );
}