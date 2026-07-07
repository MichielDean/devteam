import { create } from 'zustand';
import type {
  ChatSession,
  ChatMessage,
  ChatProvider,
  ChatCitation,
} from '../types';

// ChatStore — the chat UI state. Mirrors the ui-store pattern (zustand,
// minimal, no react-query for the streaming path — streaming is imperative).
//
// The streaming send-message flow is NOT a react-query mutation because it's
// a long-lived SSE stream, not a request/response. The store holds the
// in-flight streaming state + the pending confirm proposal.

interface PendingProposal {
  proposalId: string;
  command: string;
  classification: string;
  consequence?: string;
  needsConfirm: boolean;
}

interface ChatState {
  // Session list + current session
  sessions: ChatSession[];
  currentSessionId: string | null;
  messages: ChatMessage[];
  providers: ChatProvider[];
  selectedProvider: string | null;

  // Streaming state
  isStreaming: boolean;
  streamingContent: string; // accumulates token chunks for the in-flight expert message
  streamingCitations: ChatCitation[]; // accumulates citations chunks
  streamError: string | null;

  // Pending CLI proposal (awaiting user confirm)
  pendingProposal: PendingProposal | null;

  // Actions
  setSessions: (s: ChatSession[]) => void;
  setCurrentSession: (id: string | null) => void;
  setMessages: (m: ChatMessage[]) => void;
  addMessage: (m: ChatMessage) => void;
  setProviders: (p: ChatProvider[]) => void;
  setSelectedProvider: (p: string | null) => void;

  startStream: () => void;
  appendToken: (t: string) => void;
  setCitations: (c: ChatCitation[]) => void;
  setStreamError: (e: string) => void;
  finishStream: () => void;
  clearStream: () => void;

  setPendingProposal: (p: PendingProposal | null) => void;
  clearPendingProposal: () => void;
}

export const useChatStore = create<ChatState>((set) => ({
  sessions: [],
  currentSessionId: null,
  messages: [],
  providers: [],
  selectedProvider: null,
  isStreaming: false,
  streamingContent: '',
  streamingCitations: [],
  streamError: null,
  pendingProposal: null,

  setSessions: (s) => set({ sessions: s }),
  setCurrentSession: (id) => set({ currentSessionId: id, messages: [] }),
  setMessages: (m) => set({ messages: m }),
  addMessage: (m) => set((s) => ({ messages: [...s.messages, m] })),
  setProviders: (p) => set({ providers: p }),
  setSelectedProvider: (p) => set({ selectedProvider: p }),

  startStream: () => set({ isStreaming: true, streamingContent: '', streamingCitations: [], streamError: null }),
  appendToken: (t) => set((s) => ({ streamingContent: s.streamingContent + t })),
  setCitations: (c) => set({ streamingCitations: c }),
  setStreamError: (e) => set({ streamError: e }),
  finishStream: () => set({ isStreaming: false }),
  clearStream: () => set({ streamingContent: '', streamingCitations: [], streamError: null, isStreaming: false }),

  setPendingProposal: (p) => set({ pendingProposal: p }),
  clearPendingProposal: () => set({ pendingProposal: null }),
}));