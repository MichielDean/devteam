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
      className="absolute -top-2 -right-2 bg-yellow-500 text-white text-xs font-bold rounded-full min-w-[20px] h-5 flex items-center justify-center px-1 hover:bg-yellow-600 transition-colors z-10"
      data-testid="question-badge"
      title={`${count} pending question${count !== 1 ? 's' : ''}`}
    >
      {count}
    </Link>
  );
}