import { useState } from 'react';
import { Badge, Button } from '../ui/primitives';
import type { Question } from '../types';

interface QuestionPanelProps {
  questions: Question[];
  drafts: Record<string, string>;
  onSelect: (qid: string, option: string) => void;
  onType: (qid: string, text: string) => void;
  onSubmitAll: () => void;
  isSubmitting: boolean;
  allDrafted: boolean;
  isWaitingForHuman: boolean;
}

function groupByRole(questions: Question[]): Record<string, Question[]> {
  const groups: Record<string, Question[]> = {};
  for (const q of questions) {
    if (!groups[q.role]) groups[q.role] = [];
    groups[q.role].push(q);
  }
  return groups;
}

export default function QuestionPanel({ questions, drafts, onSelect, onType, onSubmitAll, isSubmitting, allDrafted, isWaitingForHuman }: QuestionPanelProps) {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const pending = questions.filter((q) => q.status === 'pending');
  const answered = questions.filter((q) => q.status !== 'pending');

  if (questions.length === 0) return null;

  const grouped = groupByRole(questions);

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4" data-testid="question-panel">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Questions</h3>
        <Badge color="blue" data-testid="question-progress">{answered.length}/{questions.length} answered</Badge>
      </div>

      <div className="space-y-3">
        {Object.entries(grouped).map(([role, roleQuestions]) => {
          const isCollapsed = collapsed[role] ?? false;
          const rolePending = roleQuestions.filter((q) => q.status === 'pending').length;
          return (
            <div key={role} data-testid={`question-group-${role}`}>
              <button
                onClick={() => setCollapsed((prev) => ({ ...prev, [role]: !isCollapsed }))}
                className="w-full flex items-center justify-between text-left"
                data-testid={`question-group-toggle-${role}`}
              >
                <span className="text-sm font-semibold text-gray-700 dark:text-gray-300">
                  {isCollapsed ? '▶' : '▼'} {role}
                </span>
                {rolePending > 0 && <Badge color="yellow">{rolePending} pending</Badge>}
              </button>
              {!isCollapsed && (
                <div className="mt-2 space-y-3 ml-4">
                  {roleQuestions.map((q) => (
                    <div key={q.id} className="p-3 bg-gray-50 dark:bg-gray-900/30 rounded-lg" data-testid={`question-card-${q.id}`}>
                      <div className="text-xs text-gray-500 mb-1">Stage {q.phase}</div>
                      <p className="text-sm text-gray-900 dark:text-white mb-2">{q.question}</p>
                      {q.status === 'pending' ? (
                        <>
                          {q.options.length > 0 && (
                            <div className="flex flex-wrap gap-2 mb-2">
                              {q.options.map((opt) => (
                                <button
                                  key={opt}
                                  onClick={() => onSelect(q.id, opt)}
                                  className={`px-3 py-1.5 text-xs rounded-lg border transition-colors ${drafts[q.id] === opt ? 'bg-blue-600 text-white border-blue-600' : 'bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300 border-gray-300 dark:border-gray-600 hover:bg-blue-50 dark:hover:bg-blue-900/30'}`}
                                  data-testid={`question-option-${q.id}-${opt}`}
                                >
                                  {opt}
                                </button>
                              ))}
                              <button
                                onClick={() => onSelect(q.id, 'Other')}
                                className={`px-3 py-1.5 text-xs rounded-lg border transition-colors ${drafts[q.id] === '' ? 'bg-blue-600 text-white border-blue-600' : 'bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300 border-gray-300 dark:border-gray-600'}`}
                                data-testid={`question-other-${q.id}`}
                              >
                                Other
                              </button>
                            </div>
                          )}
                          <input
                            type="text"
                            value={drafts[q.id] ?? ''}
                            onChange={(e) => onType(q.id, e.target.value)}
                            placeholder="Type answer..."
                            className="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
                            data-testid={`question-input-${q.id}`}
                          />
                        </>
                      ) : (
                        <Badge color="green" data-testid={`question-answered-${q.id}`}>
                          {q.answer || q.assumption || 'Answered'}
                        </Badge>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {isWaitingForHuman && pending.length > 0 && (
        <div className="mt-4 border-t border-gray-200 dark:border-gray-700 pt-3">
          <Button
            variant="primary"
            size="lg"
            onClick={onSubmitAll}
            disabled={!allDrafted || isSubmitting}
            isLoading={isSubmitting}
            className="w-full"
            data-testid="submit-answers"
          >
            {isSubmitting ? 'Submitting...' : 'Submit Answers & Resume'}
          </Button>
        </div>
      )}
    </div>
  );
}