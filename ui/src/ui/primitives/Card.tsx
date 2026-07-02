import { type ReactNode } from 'react';

interface CardProps {
  children: ReactNode;
  className?: string;
  'data-testid'?: string;
}

export function Card({ children, className, ...rest }: CardProps) {
  return (
    <div
      className={`bg-[var(--color-surface-raised)] rounded-[var(--radius-lg)] ${className ?? ''}`}
      style={{ boxShadow: 'var(--shadow-sm)' }}
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
    <div className={`flex items-center justify-between p-4 border-b border-[var(--color-border-subtle)] ${className ?? ''}`} {...rest}>
      <h3 className="text-base font-medium text-[var(--color-text-primary)]">{title}</h3>
      {action}
    </div>
  );
}