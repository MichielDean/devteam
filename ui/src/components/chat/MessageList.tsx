import { useEffect, useRef, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import rehypeHighlight from 'rehype-highlight';
import type { ChatMessage, ChatCitation } from '../../types';
import { CitationChip } from './CitationChip';

interface MessageListProps {
  messages: ChatMessage[];
  streamingContent: string;
  streamingCitations: ChatCitation[];
  isStreaming: boolean;
  streamError: string | null;
}

// MessageList renders the chat message history + the in-flight streaming
// expert message. Auto-scrolls to bottom unless the user scrolled up
// (jump-to-latest pill appears, interaction-spec S22).
export function MessageList({
  messages,
  streamingContent,
  streamingCitations,
  isStreaming,
  streamError,
}: MessageListProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const wasAtBottom = useRef(true);
  const [showJump, setShowJump] = useState(false);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    if (wasAtBottom.current) {
      el.scrollTop = el.scrollHeight;
    } else {
      setShowJump(true);
    }
  }, [messages, streamingContent]);

  const handleScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
    wasAtBottom.current = atBottom;
    if (atBottom) setShowJump(false);
  };

  const jumpToLatest = () => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
      wasAtBottom.current = true;
      setShowJump(false);
    }
  };

  return (
    <div className="relative flex-1 overflow-y-auto" ref={scrollRef} onScroll={handleScroll} data-testid="chat-message-list">
      <div className="max-w-3xl mx-auto px-4 py-6 space-y-6">
        {messages.length === 0 && !isStreaming && (
          <div className="text-center py-12" style={{ color: 'var(--color-text-tertiary)' }}>
            Ask the expert about AIDLC v2 — phases, stages, roles, CLI verbs, or how to drive the platform.
          </div>
        )}
        {messages.map((m) => (
          <MessageBubble key={m.id} message={m} />
        ))}
        {isStreaming && (
          <MessageBubble
            message={{
              id: 'streaming',
              role: 'expert',
              content: streamingContent,
              created_at: new Date().toISOString(),
              citations: streamingCitations,
            }}
            streaming
          />
        )}
        {isStreaming && streamingContent === '' && !streamError && (
          <div className="flex items-center gap-2 text-sm" style={{ color: 'var(--color-text-tertiary)' }}>
            <span className="inline-block w-2 h-2 rounded-full animate-pulse" style={{ backgroundColor: 'var(--color-accent)' }} />
            Expert is thinking…
          </div>
        )}
        {streamError && (
          <div
            className="rounded-[var(--radius-md)] p-3 text-sm"
            style={{ backgroundColor: 'var(--color-surface-error)', color: 'var(--color-text-error)' }}
            data-testid="chat-stream-error"
          >
            {streamError}
          </div>
        )}
      </div>
      {showJump && (
        <button
          onClick={jumpToLatest}
          className="sticky bottom-4 left-1/2 -translate-x-1/2 px-3 py-1.5 text-sm rounded-full shadow"
          style={{
            backgroundColor: 'var(--color-surface-raised)',
            color: 'var(--color-text-primary)',
            border: '1px solid var(--color-border-subtle)',
          }}
          data-testid="chat-jump-latest"
        >
          ↓ Jump to latest
        </button>
      )}
    </div>
  );
}

function MessageBubble({ message, streaming }: { message: ChatMessage; streaming?: boolean }) {
  const isUser = message.role === 'user';
  const isTool = message.role === 'tool';
  return (
    <div className={isUser ? 'flex justify-end' : 'flex justify-start'} data-testid={`chat-message-${message.role}`}>
      <div
        className="max-w-[85%] rounded-[var(--radius-md)] px-4 py-3"
        style={{
          backgroundColor: isUser
            ? 'var(--color-accent)'
            : isTool
              ? 'var(--color-surface-hover)'
              : 'var(--color-surface-raised)',
          color: isUser ? '#fff' : 'var(--color-text-primary)',
          border: isUser || isTool ? 'none' : '1px solid var(--color-border-subtle)',
        }}
      >
        {!isUser && (
          <div className="text-xs mb-1 font-semibold" style={{ color: 'var(--color-text-tertiary)' }}>
            {isTool ? 'Tool' : 'Expert'}
            {streaming && ' (typing…)'}
          </div>
        )}
        <div className="prose prose-sm max-w-none chat-markdown" style={{ color: isUser ? '#fff' : 'var(--color-text-primary)' }}>
          <ReactMarkdown rehypePlugins={[rehypeHighlight]}>{message.content || '…'}</ReactMarkdown>
        </div>
        {!isUser && message.citations && message.citations.length > 0 && (
          <div className="mt-3 flex flex-wrap gap-1.5">
            {message.citations.map((c, i) => (
              <CitationChip key={i} citation={c} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}