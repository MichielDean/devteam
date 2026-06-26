import { forwardRef } from 'react';
import type { Question } from '../types';

interface QuestionCardProps {
  question: Question;
  featureId: string;
  draft?: string;
  onSelect?: (option: string) => void;
  onType?: (text: string) => void;
}

const typeColors: Record<string, { bg: string; text: string }> = {
  clarification: { bg: 'bg-blue-100 dark:bg-blue-900', text: 'text-blue-800 dark:text-blue-200' },
  decision: { bg: 'bg-orange-100 dark:bg-orange-900', text: 'text-orange-800 dark:text-orange-200' },
  priority: { bg: 'bg-purple-100 dark:bg-purple-900', text: 'text-purple-800 dark:text-purple-200' },
};

const cardBase =
  'bg-white dark:bg-gray-800 rounded-lg shadow border border-gray-200 dark:border-gray-700 p-4';

const QuestionCard = forwardRef<HTMLDivElement, QuestionCardProps>(function QuestionCard(
  { question, featureId, draft, onSelect, onType },
  ref
) {
  void featureId;
  const typeColor =
    typeColors[question.type] || { bg: 'bg-gray-100 dark:bg-gray-700', text: 'text-gray-800 dark:text-gray-200' };

  const phaseRoleLabel = (
    <span className="text-xs text-gray-500 dark:text-gray-400">{question.phase} · {question.role}</span>
  );

  const badge = (
    <span
      className={`px-2 py-0.5 rounded-full text-xs font-medium ${typeColor.bg} ${typeColor.text}`}
      data-testid="question-type-badge"
    >
      {question.type}
    </span>
  );

  // Answered state
  if (question.status === 'answered') {
    return (
      <div className={cardBase} data-testid={`question-card-${question.id}`} ref={ref}>
        <div className="flex items-start justify-between mb-2">
          <div className="flex items-center gap-2">
            {badge}
            {phaseRoleLabel}
          </div>
          <span className="text-green-600 dark:text-green-400 text-lg" data-testid="question-checkmark">✓</span>
        </div>
        <p className="text-sm text-gray-900 dark:text-white mb-2" data-testid="question-text">{question.question}</p>
        <div className="bg-green-50 dark:bg-green-900/20 rounded p-2">
          <p className="text-sm text-gray-700 dark:text-gray-300" data-testid="question-answer">{question.answer}</p>
        </div>
      </div>
    );
  }

  // Assumed state
  if (question.status === 'assumed') {
    return (
      <div className={`${cardBase} opacity-75`} data-testid={`question-card-${question.id}`} ref={ref}>
        <div className="flex items-start justify-between mb-2">
          <div className="flex items-center gap-2">
            {badge}
            {phaseRoleLabel}
          </div>
          <span
            className="text-xs bg-yellow-100 dark:bg-yellow-900 text-yellow-800 dark:text-yellow-200 px-2 py-0.5 rounded-full"
            data-testid="question-auto-assumed-label"
          >
            auto-assumed
          </span>
        </div>
        <p className="text-sm text-gray-900 dark:text-white mb-2" data-testid="question-text">{question.question}</p>
        <div className="bg-yellow-50 dark:bg-yellow-900/20 rounded p-2">
          <p className="text-sm text-gray-700 dark:text-gray-300" data-testid="question-assumption">{question.assumption}</p>
        </div>
      </div>
    );
  }

  // Pending state
  const hasOptions = question.options && question.options.length > 0;
  const otherSelected = hasOptions && draft !== undefined && draft !== null && !question.options.includes(draft);

  return (
    <div className={`${cardBase} border-blue-300 dark:border-blue-700`} data-testid={`question-card-${question.id}`} ref={ref}>
      <div className="flex items-center gap-2 mb-2">
        {badge}
        {phaseRoleLabel}
        <span className="text-xs font-medium text-blue-700 dark:text-blue-300 ml-auto">Pending</span>
      </div>
      <p className="text-sm text-gray-900 dark:text-white mb-3" data-testid="question-text">{question.question}</p>

      {hasOptions ? (
        <>
          <div className="flex flex-col gap-2" data-testid="question-options">
            {question.options.map((option, idx) => {
              const selected = draft === option || (option === 'Other' && otherSelected);
              return (
                <button
                  key={idx}
                  onClick={() => onSelect?.(option)}
                  className={`px-4 py-3 text-left text-sm rounded-lg border transition-colors break-words whitespace-normal ${
                    selected
                      ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/40 text-blue-900 dark:text-blue-100 ring-2 ring-blue-400'
                      : 'border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300'
                  }`}
                  data-testid={`question-option-${idx}`}
                  aria-pressed={selected}
                  data-selected={selected ? 'true' : 'false'}
                >
                  {option}
                </button>
              );
            })}
          </div>
          {otherSelected && (
            <textarea
              value={draft}
              onChange={(e) => onType?.(e.target.value)}
              placeholder="Please specify..."
              rows={3}
              autoFocus
              className="mt-3 w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500 resize-y"
              data-testid="question-other-input"
            />
          )}
        </>
      ) : (
        <textarea
          value={draft ?? ''}
          onChange={(e) => onType?.(e.target.value)}
          placeholder="Type your answer..."
          rows={4}
          className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500 resize-y"
          data-testid="question-answer-input"
        />
      )}
    </div>
  );
});

export default QuestionCard;