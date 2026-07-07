import type {
  FeatureListResponse,
  FeatureDetail,
  CreateFeatureRequest,
  StageRunResult,
  FeatureStage,
  AuditEvent,
  Bolt,
  TeamKnowledge,
  Rule,
  Question,
  CreateQuestionRequest,
  AnswerQuestionRequest,
  RejectStageRequest,
  JumpRequest,
  SetScopeRequest,
  SetDepthRequest,
  SetTestStrategyRequest,
  SetLadderRequest,
  SetExecutionModeRequest,
  SaveKnowledgeRequest,
  ErrorResponse,
  TmuxSession,
} from '../types';

export const API_BASE = '/api';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });

  if (!response.ok) {
    const errorData: ErrorResponse = await response.json().catch(() => ({
      error: 'unknown_error',
      details: response.statusText,
    }));
    throw new ApiError(response.status, errorData.error, errorData.details);
  }

  if (response.status === 204) return undefined as T;
  return response.json();
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    public details?: string,
  ) {
    super(details || code);
    this.name = 'ApiError';
  }
}

// ─── Features ───
export async function listFeatures(): Promise<FeatureListResponse> {
  return request<FeatureListResponse>('/features');
}

export async function getFeature(id: string): Promise<FeatureDetail> {
  return request<FeatureDetail>(`/features/${id}`);
}

export async function createFeature(req: CreateFeatureRequest): Promise<FeatureDetail> {
  return request<FeatureDetail>('/features', {
    method: 'POST',
    body: JSON.stringify(req),
  });
}

export async function cancelFeature(id: string): Promise<FeatureDetail> {
  return request<FeatureDetail>(`/features/${id}/cancel`, { method: 'POST' });
}

// ─── Stage Workflow ───
export async function runStage(featureId: string, stageId: string): Promise<StageRunResult> {
  return request<StageRunResult>(`/features/${featureId}/run-stage`, {
    method: 'POST',
    body: JSON.stringify({ stage_id: stageId }),
  });
}

export async function resumeStage(featureId: string, stageId: string): Promise<{ status: string; stage_id: string; session_alive: boolean }> {
  return request(`/features/${featureId}/stages/${stageId}/resume`, {
    method: 'POST',
  });
}

export async function forceRerunStage(featureId: string, stageId: string): Promise<{ status: string; stage_id: string }> {
  return request(`/features/${featureId}/stages/${stageId}/force-rerun`, {
    method: 'POST',
  });
}

export async function approveStage(featureId: string, stageId: string): Promise<void> {
  await request<void>(`/features/${featureId}/stages/${stageId}/approve`, { method: 'POST' });
}

export async function rejectStage(featureId: string, stageId: string, notes: string): Promise<void> {
  const body: RejectStageRequest = { notes };
  await request<void>(`/features/${featureId}/stages/${stageId}/reject`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function acceptStageAsIs(featureId: string, stageId: string): Promise<void> {
  await request<void>(`/features/${featureId}/stages/${stageId}/accept-as-is`, { method: 'POST' });
}

export async function jumpToStage(featureId: string, stageId?: string, phase?: string): Promise<void> {
  const body: JumpRequest = { stage_id: stageId, phase };
  await request<void>(`/features/${featureId}/jump`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function getFeatureStages(featureId: string): Promise<FeatureStage[]> {
  return request<FeatureStage[]>(`/features/${featureId}/stages`);
}

export async function getAuditTrail(featureId: string): Promise<AuditEvent[]> {
  return request<AuditEvent[]>(`/features/${featureId}/audit`);
}

// ─── Scope/Depth/Test Strategy ───
export async function setScope(featureId: string, scope: string): Promise<void> {
  const body: SetScopeRequest = { scope };
  await request<void>(`/features/${featureId}/scope`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function setDepth(featureId: string, depth: string): Promise<void> {
  const body: SetDepthRequest = { depth };
  await request<void>(`/features/${featureId}/depth`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function setTestStrategy(featureId: string, testStrategy: string): Promise<void> {
  const body: SetTestStrategyRequest = { test_strategy: testStrategy };
  await request<void>(`/features/${featureId}/test-strategy`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function setLadderMode(featureId: string, mode: string): Promise<void> {
  const body: SetLadderRequest = { mode };
  await request<void>(`/features/${featureId}/ladder`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function setExecutionMode(featureId: string, mode: string): Promise<void> {
  const body: SetExecutionModeRequest = { mode };
  await request<void>(`/features/${featureId}/execution-mode`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

// ─── Bolts ───
export async function getBolts(featureId: string): Promise<Bolt[]> {
  return request<Bolt[]>(`/features/${featureId}/bolts`);
}

export async function prepareBolts(featureId: string): Promise<void> {
  await request<void>(`/features/${featureId}/prepare-bolts`, { method: 'POST' });
}

export async function runBolt(featureId: string, boltNumber: number): Promise<unknown> {
  return request<unknown>(`/features/${featureId}/run-bolt/${boltNumber}`, { method: 'POST' });
}

// ─── Rules ───
export async function getRules(featureId: string): Promise<Rule[]> {
  return request<Rule[]>(`/features/${featureId}/rules`);
}

export async function deleteRule(featureId: string, ruleId: number): Promise<void> {
  await request<void>(`/features/${featureId}/rules/${ruleId}`, { method: 'DELETE' });
}

// ─── Team Knowledge ───
export async function listAllKnowledge(): Promise<Record<string, TeamKnowledge[]>> {
  return request<Record<string, TeamKnowledge[]>>('/knowledge');
}

export async function getKnowledge(agent: string): Promise<TeamKnowledge[]> {
  return request<TeamKnowledge[]>(`/knowledge/${agent}`);
}

export async function saveKnowledge(agent: string, topic: string, content: string): Promise<void> {
  const body: SaveKnowledgeRequest = { topic, content };
  await request<void>(`/knowledge/${agent}`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function deleteKnowledge(agent: string, topic: string): Promise<void> {
  await request<void>(`/knowledge/${agent}/${topic}`, { method: 'DELETE' });
}

// ─── Artifacts ───
export async function getArtifact(id: string, type: string): Promise<string> {
  const response = await fetch(`${API_BASE}/features/${id}/artifacts/${type}`);
  if (!response.ok) {
    if (response.status === 404) return '';
    throw new ApiError(response.status, 'unknown_error', response.statusText);
  }
  return response.text();
}

export async function updateArtifact(featureId: string, type: string, content: string): Promise<void> {
  await request<void>(`/features/${featureId}/artifacts/${type}`, {
    method: 'PATCH',
    body: JSON.stringify({ content }),
  });
}

export interface ArtifactMeta {
  artifact_type: string;
  stage_id: string;
  size: number;
  updated_at: string;
}

export async function listArtifacts(featureId: string): Promise<ArtifactMeta[]> {
  return request<ArtifactMeta[]>(`/features/${featureId}/artifacts`);
}

// ─── Questions ───
export async function listQuestions(featureId: string): Promise<Question[]> {
  return request<Question[]>(`/features/${featureId}/questions`);
}

export async function createQuestion(featureId: string, req: CreateQuestionRequest): Promise<Question> {
  return request<Question>(`/features/${featureId}/questions`, {
    method: 'POST',
    body: JSON.stringify(req),
  });
}

export async function answerQuestion(featureId: string, questionId: string, answer: string): Promise<Question> {
  const body: AnswerQuestionRequest = { answer };
  return request<Question>(`/features/${featureId}/questions/${questionId}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  });
}

export async function listPendingQuestions(featureId: string): Promise<Question[]> {
  return request<Question[]>(`/features/${featureId}/questions/pending`);
}

// ─── Output ───
export async function getCapturedOutput(featureId: string): Promise<{ is_processing: boolean; output: string }> {
  return request<{ is_processing: boolean; output: string }>(`/features/${featureId}/output`);
}

// ─── Tmux Sessions ───
export async function listSessions(featureId: string): Promise<TmuxSession[]> {
  return request<TmuxSession[]>(`/features/${featureId}/sessions`);
}

export async function resumeSession(featureId: string, phase: string): Promise<StageRunResult> {
  return request<StageRunResult>(`/features/${featureId}/sessions/${phase}/resume`, { method: 'POST' });
}

export async function killSession(featureId: string, phase: string): Promise<void> {
  await request<void>(`/features/${featureId}/sessions/${phase}/kill`, { method: 'POST' });
}

export async function getSessionOutput(featureId: string, phase: string, stageId?: string): Promise<string> {
  const params = stageId ? `?stage_id=${stageId}` : '';
  const response = await fetch(`${API_BASE}/features/${featureId}/sessions/${phase}/output${params}`);
  if (!response.ok) return '';
  return response.text();
}

export async function getCapturePane(featureId: string, phase: string): Promise<string> {
  const response = await fetch(`${API_BASE}/features/${featureId}/sessions/${phase}/pane`);
  if (!response.ok) return '';
  return response.text();
}

export async function listActiveSessions(): Promise<TmuxSession[]> {
  return request<TmuxSession[]>('/sessions/active');
}

// ─── Repos ───
export interface AvailableRepo {
  name: string;
  url: string;
  description: string;
  primary: boolean;
}

export async function listRepos(): Promise<AvailableRepo[]> {
  return request<AvailableRepo[]>('/repos');
}
// ─── Chat (AIDLC Expert Agent and Chat UI) ───────────────────────────────

import type {
  ChatSession,
  ChatSessionDetail,
  ChatProvider,
  ChatStreamChunk,
  ChatCliConfirmRequest,
  ChatCliConfirmResponse,
} from '../types';

export async function listChatSessions(): Promise<ChatSession[]> {
  return request<ChatSession[]>('/chat/sessions');
}

export async function createChatSession(title?: string, selectedProvider?: string): Promise<ChatSession> {
  return request<ChatSession>('/chat/sessions', {
    method: 'POST',
    body: JSON.stringify({ title, selected_provider: selectedProvider }),
  });
}

export async function getChatSession(id: string): Promise<ChatSessionDetail> {
  return request<ChatSessionDetail>(`/chat/sessions/${id}`);
}

export async function deleteChatSession(id: string): Promise<void> {
  await request<void>(`/chat/sessions/${id}`, { method: 'DELETE' });
}

export async function listChatProviders(): Promise<ChatProvider[]> {
  return request<ChatProvider[]>('/chat/providers');
}

export async function confirmChatCliOp(
  sessionId: string,
  req: ChatCliConfirmRequest,
): Promise<ChatCliConfirmResponse> {
  return request<ChatCliConfirmResponse>(`/chat/sessions/${sessionId}/cli-confirm`, {
    method: 'POST',
    body: JSON.stringify(req),
  });
}

// sendChatMessage opens an SSE stream to POST /chat/sessions/{id}/messages
// and invokes onChunk for each chunk received. Returns when the stream
// closes (done/error) or the AbortSignal aborts (client disconnect).
export async function sendChatMessage(
  sessionId: string,
  content: string,
  provider: string | undefined,
  onChunk: (chunk: ChatStreamChunk) => void,
  signal?: AbortSignal,
): Promise<void> {
  const resp = await fetch(`${API_BASE}/chat/sessions/${sessionId}/messages`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content, provider }),
    signal,
  });
  if (!resp.ok || !resp.body) {
    const err = await resp.json().catch(() => ({ error: 'unknown', details: resp.statusText }));
    throw new ApiError(resp.status, err.error, err.details);
  }
  const reader = resp.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    // SSE framing: events separated by \n\n; each event is "data: <json>\n\n"
    let idx: number;
    while ((idx = buffer.indexOf('\n\n')) >= 0) {
      const event = buffer.slice(0, idx);
      buffer = buffer.slice(idx + 2);
      const line = event.startsWith('data: ') ? event.slice(6) : event;
      const trimmed = line.trim();
      if (!trimmed) continue;
      try {
        const chunk = JSON.parse(trimmed) as ChatStreamChunk;
        onChunk(chunk);
      } catch {
        // Malformed chunk — skip (the stream is best-effort).
      }
    }
  }
}
