import { useEffect, useState, useCallback, useRef } from 'react';
import { useChatStore } from '../store/chat-store';
import {
  listChatSessions,
  createChatSession,
  getChatSession,
  deleteChatSession,
  listChatProviders,
  sendChatMessage,
  confirmChatCliOp,
} from '../api/client';
import type { ChatCitation, ChatStreamChunk } from '../types';
import { MessageList } from '../components/chat/MessageList';
import { CitationDrawer } from '../components/chat/CitationChip';
import { ProviderPicker } from '../components/chat/ProviderPicker';
import { ToolCallCard } from '../components/chat/ToolCallCard';

// Chat is the /chat route (FR-G2-1). One route, one process (C1/C9/NG4).
// Renders the session list sidebar + the message stream + the input + the
// provider picker + the confirm affordance (interaction-spec S1–S22).
export default function Chat() {
  const store = useChatStore();
  const [input, setInput] = useState('');
  const [drawerCitation, setDrawerCitation] = useState<ChatCitation | null>(null);
  const [confirmingProposal, setConfirmingProposal] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  // Load sessions + providers on mount.
  useEffect(() => {
    listChatSessions().then((s) => store.setSessions(s)).catch(() => {});
    listChatProviders().then((p) => {
      store.setProviders(p);
      if (p.length > 0 && !store.selectedProvider) store.setSelectedProvider(p[0].name);
    }).catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Load messages when switching sessions.
  useEffect(() => {
    if (!store.currentSessionId) return;
    getChatSession(store.currentSessionId)
      .then((detail) => store.setMessages(detail.messages))
      .catch(() => store.setMessages([]));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [store.currentSessionId]);

  const handleNewChat = useCallback(async () => {
    const sess = await createChatSession('New Chat');
    store.setSessions(await listChatSessions());
    store.setCurrentSession(sess.id);
    setInput('');
  }, [store]);

  const handleSelectSession = useCallback((id: string) => {
    store.setCurrentSession(id);
  }, [store]);

  const handleDeleteSession = useCallback(async (id: string) => {
    await deleteChatSession(id);
    store.setSessions(await listChatSessions());
    if (store.currentSessionId === id) store.setCurrentSession(null);
  }, [store]);

  const handleSend = useCallback(async () => {
    const content = input.trim();
    if (!content || store.isStreaming) return;
    if (!store.currentSessionId) {
      // Auto-create a session on first message.
      const sess = await createChatSession('New Chat', store.selectedProvider || undefined);
      store.setSessions(await listChatSessions());
      store.setCurrentSession(sess.id);
    }
    const sessionId = store.currentSessionId || (await createChatSession('New Chat')).id;
    setInput('');
    store.addMessage({
      id: `user-${Date.now()}`,
      role: 'user',
      content,
      created_at: new Date().toISOString(),
    });
    store.startStream();
    const controller = new AbortController();
    abortRef.current = controller;
    try {
      await sendChatMessage(
        sessionId,
        content,
        store.selectedProvider || undefined,
        (chunk: ChatStreamChunk) => handleChunk(chunk, sessionId),
        controller.signal,
      );
    } catch (err: any) {
      if (err.name !== 'AbortError') {
        store.setStreamError(err.message || String(err));
      }
    } finally {
      store.finishStream();
      abortRef.current = null;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [input, store]);

  const handleChunk = useCallback((chunk: ChatStreamChunk, sessionId: string) => {
    switch (chunk.type) {
      case 'token':
        store.appendToken(chunk.content || '');
        break;
      case 'citations':
        if (chunk.citations) store.setCitations(chunk.citations);
        break;
      case 'tool-call':
        if (chunk.needs_confirm) {
          store.setPendingProposal({
            proposalId: chunk.proposal_id || '',
            command: chunk.command || '',
            classification: chunk.classification || '',
            consequence: chunk.consequence,
            needsConfirm: true,
          });
        }
        break;
      case 'done':
        // Persist the final expert message from the server's record.
        if (chunk.message_id) {
          getChatSession(sessionId).then((detail) => store.setMessages(detail.messages)).catch(() => {});
        }
        store.clearStream();
        break;
      case 'error':
        store.setStreamError(chunk.error || 'Unknown error');
        break;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [store]);

  const handleConfirm = useCallback(async (proposalId: string, approved: boolean) => {
    if (!store.currentSessionId) return;
    setConfirmingProposal(proposalId);
    try {
      await confirmChatCliOp(store.currentSessionId, { proposal_id: proposalId, approved });
      store.clearPendingProposal();
    } catch (err: any) {
      store.setStreamError(`Confirm failed: ${err.message}`);
    } finally {
      setConfirmingProposal(null);
    }
  }, [store]);

  const handleStop = useCallback(() => {
    abortRef.current?.abort();
    store.finishStream();
  }, [store]);

  return (
    <div className="flex h-[calc(100vh-120px)] gap-4" data-testid="chat-page">
      {/* Session sidebar */}
      <aside
        className="w-64 flex-shrink-0 rounded-[var(--radius-md)] overflow-hidden flex flex-col"
        style={{ backgroundColor: 'var(--color-surface-raised)', border: '1px solid var(--color-border-subtle)' }}
        data-testid="chat-sidebar"
      >
        <div className="p-3 border-b" style={{ borderColor: 'var(--color-border-subtle)' }}>
          <button
            onClick={handleNewChat}
            className="w-full px-3 py-2 text-sm font-semibold rounded-[var(--radius-md)]"
            style={{ backgroundColor: 'var(--color-accent)', color: '#fff' }}
            data-testid="chat-new-session"
          >
            + New Chat
          </button>
        </div>
        <div className="flex-1 overflow-y-auto p-2 space-y-1">
          {store.sessions.length === 0 && (
            <div className="text-xs text-center py-4" style={{ color: 'var(--color-text-tertiary)' }}>
              No chats yet
            </div>
          )}
          {store.sessions.map((s) => (
            <div
              key={s.id}
              className={`group flex items-center justify-between px-2 py-1.5 rounded-[var(--radius-md)] cursor-pointer text-sm ${
                s.id === store.currentSessionId ? '' : 'hover:bg-[var(--color-surface-hover)]'
              }`}
              style={{
                backgroundColor: s.id === store.currentSessionId ? 'var(--color-surface-hover)' : 'transparent',
                color: 'var(--color-text-primary)',
              }}
              onClick={() => handleSelectSession(s.id)}
              data-testid="chat-session-item"
            >
              <span className="truncate flex-1">{s.title}</span>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  handleDeleteSession(s.id);
                }}
                className="opacity-0 group-hover:opacity-100 text-xs px-1"
                style={{ color: 'var(--color-text-tertiary)' }}
                aria-label="Delete chat"
              >
                ✕
              </button>
            </div>
          ))}
        </div>
      </aside>

      {/* Main chat column */}
      <div
        className="flex-1 flex flex-col rounded-[var(--radius-md)] overflow-hidden"
        style={{ backgroundColor: 'var(--color-surface-raised)', border: '1px solid var(--color-border-subtle)' }}
      >
        {/* Header: title + provider picker */}
        <div
          className="flex items-center justify-between px-4 py-3 border-b"
          style={{ borderColor: 'var(--color-border-subtle)' }}
        >
          <h1 className="text-base font-semibold" style={{ color: 'var(--color-text-primary)' }}>
            AIDLC Expert
          </h1>
          <ProviderPicker
            providers={store.providers}
            selected={store.selectedProvider}
            onSelect={(name) => store.setSelectedProvider(name)}
          />
        </div>

        {/* Messages */}
        <MessageList
          messages={store.messages}
          streamingContent={store.streamingContent}
          streamingCitations={store.streamingCitations}
          isStreaming={store.isStreaming}
          streamError={store.streamError}
        />

        {/* Pending proposal (confirm gate) */}
        {store.pendingProposal && (
          <div className="px-4 py-3 border-t" style={{ borderColor: 'var(--color-border-subtle)' }}>
            <ToolCallCard
              chunk={{
                type: 'tool-call',
                proposal_id: store.pendingProposal.proposalId,
                command: store.pendingProposal.command,
                classification: store.pendingProposal.classification,
                consequence: store.pendingProposal.consequence,
                needs_confirm: store.pendingProposal.needsConfirm,
              }}
              onConfirm={handleConfirm}
              isConfirming={confirmingProposal === store.pendingProposal.proposalId}
            />
          </div>
        )}

        {/* Input */}
        <div className="px-4 py-3 border-t" style={{ borderColor: 'var(--color-border-subtle)' }}>
          <div className="flex gap-2 items-end">
            <textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  handleSend();
                }
              }}
              placeholder="Ask about AIDLC v2 — phases, stages, roles, CLI verbs…"
              rows={1}
              className="flex-1 px-3 py-2 text-sm rounded-[var(--radius-md)] resize-none"
              style={{
                backgroundColor: 'var(--color-surface)',
                color: 'var(--color-text-primary)',
                border: '1px solid var(--color-border-subtle)',
                minHeight: '40px',
                maxHeight: '160px',
              }}
              data-testid="chat-input"
              disabled={store.isStreaming}
            />
            {store.isStreaming ? (
              <button
                onClick={handleStop}
                className="px-4 py-2 text-sm font-semibold rounded-[var(--radius-md)]"
                style={{ backgroundColor: 'var(--color-text-error)', color: '#fff' }}
                data-testid="chat-stop"
              >
                Stop
              </button>
            ) : (
              <button
                onClick={handleSend}
                disabled={!input.trim()}
                className="px-4 py-2 text-sm font-semibold rounded-[var(--radius-md)]"
                style={{
                  backgroundColor: input.trim() ? 'var(--color-accent)' : 'var(--color-surface-hover)',
                  color: input.trim() ? '#fff' : 'var(--color-text-tertiary)',
                }}
                data-testid="chat-send"
              >
                Send
              </button>
            )}
          </div>
          <p className="text-xs mt-1.5" style={{ color: 'var(--color-text-tertiary)' }}>
            The expert grounds factual answers in the AIDLC corpus. Mutating ops require your confirm.
          </p>
        </div>
      </div>

      <CitationDrawer citation={drawerCitation} onClose={() => setDrawerCitation(null)} />
    </div>
  );
}