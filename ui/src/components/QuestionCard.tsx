import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { answerQuestion } from '../api/client';
import type { Question } from '../types';
import { useToast } from '../components/Toast';

interface QuestionCardProps {
  question: Question;
  featureId: string;
}

const typeColors: Record<string, { bg: string; text: string }> = {
  clarification: { bg: 'bg-blue-100 dark:bg-blue-900', text: 'text-blue-800 dark:text-blue-200' },
  decision: { bg: 'bg-orange-100 dark:bg-orange-900', text: 'text-orange-800 dark:text-orange-200' },
  priority: { bg: 'bg-purple-100 dark:bg-purple-900', text: 'text-purple-800 dark:text-purple-200' },
};

export default function QuestionCard({ question, featureId }: QuestionCardProps) {
  const [answerText, setAnswerText] = useState('');
  const queryClient = useQueryClient();
  const { addToast } = useToast();

  const answerMutation = useMutation({
    mutationFn: (answer: string) => answerQuestion(featureId, question.id, answer),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['questions', featureId] });
      queryClient.invalidateQueries({ queryKey: ['feature', featureId] });
      queryClient.invalidateQueries({ queryKey: ['features'] });
      setAnswerText('');
      addToast('success', 'Question answered');
    },
    onError: (err: Error) => {
      if (err.message.includes('already answered') || err.message.includes('already assumed') || err.message.includes('409')) {
        addToast('error', 'Question already answered');
      } else {
        addToast('error', `Failed to answer question: ${err.message}`);
      }
    },
  });

  const typeColor = typeColors[question.type] || { bg: 'bg-gray-100 dark:bg-gray-700', text: 'text-gray-800 dark:text-gray-200' };

  const handleOptionClick = (option: string) => {
    setAnswerText(option);
  };

  const handleSubmit = () => {
    const trimmed = answerText.trim();
    if (!trimmed) return;
    answerMutation.mutate(trimmed);
  };

  // Answered state
  if (question.status === 'answered') {
    return (
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow border border-gray-200 dark:border-gray-700 p-4" data-testid={`question-card-${question.id}`}>
        <div className="flex items-start justify-between mb-2">
          <div className="flex items-center gap-2">
            <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${typeColor.bg} ${typeColor.text}`} data-testid="question-type-badge">
              {question.type}
            </span>
            <span className="text-xs text-gray-500 dark:text-gray-400">
              {question.phase} · {question.role}
            </span>
          </div>
          <span className="text-green-600 dark:text-green-400 text-lg" data-testid="question-checkmark">✓</span>
        </div>
        <p className="text-sm text-gray-900 dark:text-white mb-2" data-testid="question-text">{question.question}</p>
        <div className="bg-green-50 dark:bg-green-900/20 rounded p-2">
          <p className="text-sm text-gray-700 dark:text-gray-300" data-testid="question-answer">
            {question.answer}
          </p>
        </div>
      </div>
    );
  }

  // Assumed state
  if (question.status === 'assumed') {
    return (
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow border border-gray-200 dark:border-gray-700 p-4 opacity-75" data-testid={`question-card-${question.id}`}>
        <div className="flex items-start justify-between mb-2">
          <div className="flex items-center gap-2">
            <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${typeColor.bg} ${typeColor.text}`} data-testid="question-type-badge">
              {question.type}
            </span>
            <span className="text-xs text-gray-500 dark:text-gray-400">
              {question.phase} · {question.role}
            </span>
          </div>
          <span className="text-xs bg-yellow-100 dark:bg-yellow-900 text-yellow-800 dark:text-yellow-200 px-2 py-0.5 rounded-full" data-testid="question-auto-assumed-label">auto-assumed</span>
        </div>
        <p className="text-sm text-gray-900 dark:text-white mb-2" data-testid="question-text">{question.question}</p>
        <div className="bg-yellow-50 dark:bg-yellow-900/20 rounded p-2">
          <p className="text-sm text-gray-700 dark:text-gray-300" data-testid="question-assumption">
            {question.assumption}
          </p>
        </div>
      </div>
    );
  }

  // Pending state
  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow border border-gray-200 dark:border-gray-700 p-4" data-testid={`question-card-${question.id}`}>
      <div className="flex items-center gap-2 mb-2">
        <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${typeColor.bg} ${typeColor.text}`} data-testid="question-type-badge">
          {question.type}
        </span>
        <span className="text-xs text-gray-500 dark:text-gray-400">
          {question.phase} · {question.role}
        </span>
      </div>
      <p className="text-sm text-gray-900 dark:text-white mb-3" data-testid="question-text">{question.question}</p>
      
      {question.options && question.options.length > 0 && (
        <div className="flex flex-wrap gap-2 mb-3" data-testid="question-options">
          {question.options.map((option, idx) => (
            <button
              key={idx}
              onClick={() => handleOptionClick(option)}
              className="px-3 py-1 text-sm rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 transition-colors"
              data-testid={`question-option-${idx}`}
            >
              {option}
            </button>
          ))}
        </div>
      )}
      
      <div className="flex gap-2">
        <input
          type="text"
          value={answerText}
          onChange={(e) => setAnswerText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && answerText.trim()) {
              handleSubmit();
            }
          }}
          placeholder="Type your answer..."
          className="flex-1 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
          disabled={answerMutation.isPending}
          data-testid="question-answer-input"
        />
        <button
          onClick={handleSubmit}
          disabled={answerMutation.isPending || !answerText.trim()}
          className="px-4 py-2 text-sm bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          data-testid="question-answer-submit"
        >
          {answerMutation.isPending ? 'Submitting...' : 'Submit'}
        </button>
      </div>
    </div>
  );
}