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

const inputClass =
  'w-full px-3 py-1.5 text-sm rounded-[var(--radius-md)] bg-[var(--color-surface-raised)] text-[var(--color-text-primary)] border border-[var(--color-border-subtle)] focus:border-[var(--color-accent)] focus:outline-none transition-colors';

export default function QuestionPanel({ questions, drafts, onSelect, onType, onSubmitAll, isSubmitting, allDrafted, isWaitingForHuman }: QuestionPanelProps) {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const pending = questions.filter((q) => q.status === 'pending');
  const answered = questions.filter((q) => q.status !== 'pending');

  if (questions.length === 0) return null;

  const grouped = groupByRole(questions);

  return (
    <div className="rounded-[var(--radius-lg)] p-4" style={{ backgroundColor: 'var(--color-surface-raised)', boxShadow: 'var(--shadow-sm)' }} data-testid="question-panel">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-base font-medium text-[var(--color-text-primary)]">Questions</h3>
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
                <span className="text-sm font-medium text-[var(--color-text-secondary)]">
                  {isCollapsed ? '▶' : '▼'} {role}
                </span>
                {rolePending > 0 && <Badge color="yellow">{rolePending} pending</Badge>}
              </button>
              {!isCollapsed && (
                <div className="mt-2 space-y-2 ml-4">
                  {roleQuestions.map((q) => (
                    <div key={q.id} className="p-3 rounded-[var(--radius-md)]" style={{ backgroundColor: 'var(--color-surface-hover)' }} data-testid={`question-card-${q.id}`}>
                      <div className="text-xs text-[var(--color-text-tertiary)] mb-1">Stage {q.phase}</div>
                      <p className="text-sm text-[var(--color-text-primary)] mb-2">{q.question}</p>
                      {q.status === 'pending' ? (
                        <>
                          {q.options.length > 0 && (
                            <div className="flex flex-wrap gap-1.5 mb-2">
                              {q.options.map((opt) => (
                                <button
                                  key={opt}
                                  onClick={() => onSelect(q.id, opt)}
                                  className={`px-3 py-1.5 text-xs rounded-[var(--radius-md)] border transition-colors ${drafts[q.id] === opt ? 'bg-[var(--color-accent)] text-white border-transparent' : 'bg-[var(--color-surface-raised)] text-[var(--color-text-secondary)] border-[var(--color-border-subtle)] hover:bg-[var(--color-surface-active)]'}`}
                                  data-testid={`question-option-${q.id}-${opt}`}
                                >
                                  {opt}
                                </button>
                              ))}
                              <button
                                onClick={() => onSelect(q.id, 'Other')}
                                className={`px-3 py-1.5 text-xs rounded-[var(--radius-md)] border transition-colors ${drafts[q.id] === '' ? 'bg-[var(--color-accent)] text-white border-transparent' : 'bg-[var(--color-surface-raised)] text-[var(--color-text-secondary)] border-[var(--color-border-subtle)] hover:bg-[var(--color-surface-active)]'}`}
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
                            className={inputClass}
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
        <div className="mt-4 border-t border-[var(--color-border-subtle)] pt-3">
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