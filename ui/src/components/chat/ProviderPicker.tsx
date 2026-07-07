import { useState, useRef, useEffect } from 'react';
import type { ChatProvider } from '../../types';

interface ProviderPickerProps {
  providers: ChatProvider[];
  selected: string | null;
  onSelect: (name: string) => void;
}

// ProviderPicker is a dropdown that lets the user switch the LLM provider
// mid-session (FR-G3-4, SC4). ≤2 clicks to switch (NFR-USA-3). The picker
// lives in the chat surface, NOT the management console (C17/ADR-010).
export function ProviderPicker({ providers, selected, onSelect }: ProviderPickerProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', onClick);
    return () => document.removeEventListener('mousedown', onClick);
  }, []);

  const current = providers.find((p) => p.name === selected) || providers[0];
  if (!current) {
    return (
      <div
        className="text-xs px-3 py-1.5 rounded-[var(--radius-md)]"
        style={{ backgroundColor: 'var(--color-surface-error)', color: 'var(--color-text-error)' }}
        data-testid="chat-providers-empty"
      >
        No providers configured — add to devteam.yaml
      </div>
    );
  }

  return (
    <div className="relative" ref={ref} data-testid="chat-provider-picker">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex items-center gap-2 px-3 py-1.5 text-sm rounded-[var(--radius-md)] transition-colors"
        style={{
          backgroundColor: 'var(--color-surface-raised)',
          color: 'var(--color-text-primary)',
          border: '1px solid var(--color-border-subtle)',
        }}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span aria-hidden>🤖</span>
        <span className="font-mono">{current.name}</span>
        <span style={{ color: 'var(--color-text-tertiary)' }}>·</span>
        <span className="font-mono text-xs" style={{ color: 'var(--color-text-tertiary)' }}>{current.model}</span>
        <span aria-hidden>▾</span>
      </button>
      {open && (
        <ul
          role="listbox"
          className="absolute right-0 mt-1 min-w-[240px] rounded-[var(--radius-md)] shadow-lg z-30 overflow-hidden"
          style={{
            backgroundColor: 'var(--color-surface-raised)',
            border: '1px solid var(--color-border-subtle)',
          }}
        >
          {providers.map((p) => (
            <li key={p.name}>
              <button
                type="button"
                role="option"
                aria-selected={p.name === selected}
                disabled={!p.available}
                onClick={() => {
                  onSelect(p.name);
                  setOpen(false);
                }}
                className="w-full text-left px-3 py-2 text-sm flex items-center justify-between transition-colors disabled:opacity-50"
                style={{
                  color: 'var(--color-text-primary)',
                  backgroundColor: p.name === selected ? 'var(--color-surface-hover)' : 'transparent',
                }}
              >
                <span className="flex flex-col">
                  <span className="font-mono">{p.name}</span>
                  <span className="font-mono text-xs" style={{ color: 'var(--color-text-tertiary)' }}>{p.model}</span>
                </span>
                {!p.available && (
                  <span className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>no key</span>
                )}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}