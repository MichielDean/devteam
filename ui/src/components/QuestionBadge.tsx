import { Link } from 'react-router';

interface QuestionBadgeProps {
  featureId: string;
  count: number;
}

export default function QuestionBadge({ featureId, count }: QuestionBadgeProps) {
  if (count <= 0) return null;

  return (
    <Link
      to={`/features/${featureId}`}
      className="absolute -top-2 -right-2 text-white text-xs font-bold rounded-[var(--radius-md)] min-w-[20px] h-5 flex items-center justify-center px-1 hover:opacity-90 transition-opacity z-10"
      style={{ backgroundColor: 'var(--color-warning)' }}
      data-testid="question-badge"
      title={`${count} pending question${count !== 1 ? 's' : ''}`}
    >
      {count}
    </Link>
  );
}