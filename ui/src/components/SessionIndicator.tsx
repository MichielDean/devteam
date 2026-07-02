import { Badge } from '../ui/primitives';

interface SessionIndicatorProps {
  count: number;
  featureId: string;
}

export default function SessionIndicator({ count, featureId }: SessionIndicatorProps) {
  void featureId;
  return (
    <Badge color="green" data-testid="session-indicator">
      <span className="w-2 h-2 rounded-full mr-1.5 animate-pulse" style={{ backgroundColor: 'var(--color-success)' }} />
      {count} session{count > 1 ? 's' : ''}
    </Badge>
  );
}