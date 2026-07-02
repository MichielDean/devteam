import { Badge } from '../ui/primitives';

interface SessionIndicatorProps {
  count: number;
  featureId: string;
}

export default function SessionIndicator({ count, featureId }: SessionIndicatorProps) {
  void featureId;
  return (
    <Badge color="green" data-testid="session-indicator">
      <span className="w-2 h-2 rounded-full bg-green-500 animate-pulse mr-1.5" />
      {count} session{count > 1 ? 's' : ''}
    </Badge>
  );
}