import { type ButtonHTMLAttributes, forwardRef } from 'react';

type Variant = 'primary' | 'secondary' | 'danger' | 'success' | 'warning' | 'ghost';
type Size = 'sm' | 'md' | 'lg';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
  isLoading?: boolean;
}

const variantClasses: Record<Variant, string> = {
  primary: 'bg-[var(--color-accent)] text-white hover:bg-[var(--color-accent-hover)] border border-transparent',
  secondary: 'bg-[var(--color-surface-hover)] text-[var(--color-text-primary)] hover:bg-[var(--color-surface-active)] border border-[var(--color-border-default)]',
  danger: 'bg-[var(--color-danger)] text-white hover:opacity-90 border border-transparent',
  success: 'bg-[var(--color-success)] text-white hover:opacity-90 border border-transparent',
  warning: 'bg-[var(--color-warning)] text-white hover:opacity-90 border border-transparent',
  ghost: 'bg-transparent text-[var(--color-text-secondary)] hover:bg-[var(--color-surface-hover)] hover:text-[var(--color-text-primary)] border border-transparent',
};

const sizeClasses: Record<Size, string> = {
  sm: 'px-3 py-1.5 text-xs',
  md: 'px-4 py-2 text-sm',
  lg: 'px-6 py-2.5 text-sm',
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', isLoading, children, className, disabled, ...props }, ref) => (
    <button
      ref={ref}
      className={`inline-flex items-center justify-center font-medium rounded-[var(--radius-md)] transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${variantClasses[variant]} ${sizeClasses[size]} ${className ?? ''}`}
      style={variant === 'primary' ? { boxShadow: 'var(--shadow-sm)' } : undefined}
      disabled={disabled || isLoading}
      {...props}
    >
      {isLoading && <span className="animate-spin inline-block w-4 h-4 border-2 border-current border-t-transparent rounded-full mr-2" />}
      {children}
    </button>
  ),
);
Button.displayName = 'Button';