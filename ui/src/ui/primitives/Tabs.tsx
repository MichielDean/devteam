import { type ReactNode } from 'react';

type Tab = { id: string; label: string; icon?: ReactNode };

interface TabsProps {
  tabs: Tab[];
  activeId: string;
  onChange: (id: string) => void;
  className?: string;
  'data-testid'?: string;
}

export function Tabs({ tabs, activeId, onChange, className, ...rest }: TabsProps) {
  return (
    <div className={`flex gap-1 border-b border-gray-200 dark:border-gray-700 ${className ?? ''}`} {...rest}>
      {tabs.map((tab) => (
        <button
          key={tab.id}
          onClick={() => onChange(tab.id)}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            activeId === tab.id
              ? 'border-blue-600 text-blue-600 dark:text-blue-400'
              : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
          }`}
          data-testid={`tab-${tab.id}`}
        >
          {tab.icon && <span className="mr-1.5">{tab.icon}</span>}
          {tab.label}
        </button>
      ))}
    </div>
  );
}