import { type ReactNode } from 'react';

interface CardProps {
  children: ReactNode;
  className?: string;
  'data-testid'?: string;
}

export function Card({ children, className, ...rest }: CardProps) {
  return (
    <div
      className={`bg-white dark:bg-gray-800 rounded-lg shadow ${className ?? ''}`}
      {...rest}
    >
      {children}
    </div>
  );
}

interface CardHeaderProps {
  title: string;
  action?: ReactNode;
  className?: string;
  'data-testid'?: string;
}

export function CardHeader({ title, action, className, ...rest }: CardHeaderProps) {
  return (
    <div className={`flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700 ${className ?? ''}`} {...rest}>
      <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{title}</h3>
      {action}
    </div>
  );
}