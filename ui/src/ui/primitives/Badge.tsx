import { type ReactNode } from 'react';

export type Color = 'gray' | 'blue' | 'green' | 'yellow' | 'red' | 'orange' | 'purple' | 'indigo';

const colorClasses: Record<Color, string> = {
  gray: 'bg-[var(--color-surface-active)] text-[var(--color-text-secondary)]',
  blue: 'bg-[var(--color-accent)]/10 text-[var(--color-accent)]',
  green: 'bg-[var(--color-success-surface)] text-[var(--color-success)]',
  yellow: 'bg-[var(--color-warning-surface)] text-[var(--color-warning)]',
  red: 'bg-[var(--color-danger-surface)] text-[var(--color-danger)]',
  orange: 'bg-[var(--color-warning-surface)] text-[var(--color-warning)]',
  purple: 'bg-[var(--color-accent)]/10 text-[var(--color-accent)]',
  indigo: 'bg-[var(--color-accent)]/10 text-[var(--color-accent)]',
};

interface BadgeProps {
  color?: Color;
  children: ReactNode;
  className?: string;
  'data-testid'?: string;
}

export function Badge({ color = 'gray', children, className, ...rest }: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded-[var(--radius-md)] text-xs font-medium ${colorClasses[color]} ${className ?? ''}`}
      {...rest}
    >
      {children}
    </span>
  );
}