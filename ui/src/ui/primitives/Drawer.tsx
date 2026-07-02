import { type ReactNode, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';

interface DrawerProps {
  open: boolean;
  onClose: () => void;
  title?: string;
  children: ReactNode;
  side?: 'right' | 'left';
  width?: string;
  'data-testid'?: string;
}

export function Drawer({ open, onClose, title, children, side = 'right', width = '500px', ...rest }: DrawerProps) {
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [open, onClose]);

  const x = side === 'right' ? '100%' : '-100%';

  return (
    <AnimatePresence>
      {open && (
        <>
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
            className="fixed inset-0 bg-black/60 z-40"
            data-testid="drawer-backdrop"
          />
          <motion.div
            initial={{ x }}
            animate={{ x: 0 }}
            exit={{ x }}
            transition={{ type: 'spring', damping: 30, stiffness: 300 }}
            className={`fixed top-0 ${side === 'right' ? 'right-0' : 'left-0'} bottom-0 bg-[var(--color-surface-raised)] z-50 flex flex-col`}
            style={{ width, boxShadow: 'var(--shadow-xl)' }}
            {...rest}
          >
            {title && (
              <div className="flex items-center justify-between p-4 border-b border-[var(--color-border-subtle)]">
                <h3 className="text-base font-medium text-[var(--color-text-primary)]">{title}</h3>
                <button onClick={onClose} className="text-[var(--color-text-tertiary)] hover:text-[var(--color-text-primary)]" data-testid="drawer-close">
                  ✕
                </button>
              </div>
            )}
            <div className="flex-1 overflow-y-auto p-4">{children}</div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}