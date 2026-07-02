import { createContext, useContext, useState, useCallback, type ReactNode } from 'react';

interface Toast {
  id: number;
  type: 'success' | 'error';
  message: string;
}

interface ToastContextType {
  addToast: (type: 'success' | 'error', message: string) => void;
}

const ToastContext = createContext<ToastContextType>({
  addToast: () => {},
});

let nextId = 0;

const typeStyle: Record<Toast['type'], React.CSSProperties> = {
  success: { backgroundColor: 'var(--color-success)' },
  error: { backgroundColor: 'var(--color-danger)' },
};

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const addToast = useCallback((type: 'success' | 'error', message: string) => {
    const id = nextId++;
    setToasts((prev) => [...prev, { id, type, message }]);
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, 5000);
  }, []);

  const removeToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ addToast }}>
      {children}
      <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2" data-testid="toast-container">
        {toasts.map((toast) => (
          <div
            key={toast.id}
            className="px-4 py-3 rounded-[var(--radius-md)] text-white max-w-sm"
            style={{ ...typeStyle[toast.type], boxShadow: 'var(--shadow-lg)' }}
            role="alert"
            data-testid={`toast-${toast.type}`}
          >
            <div className="flex items-center justify-between">
              <span className="text-sm">{toast.message}</span>
              <button
                onClick={() => removeToast(toast.id)}
                className="ml-2 text-white/70 hover:text-white"
                aria-label="Dismiss"
              >
                ×
              </button>
            </div>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  return useContext(ToastContext);
}