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
  RunStageRequest,
  RejectStageRequest,
  JumpRequest,
  SetScopeRequest,
  SetDepthRequest,
  SetTestStrategyRequest,
  SetLadderRequest,
  SaveKnowledgeRequest,
  ErrorResponse,
  TmuxSession,
} from '../types';

const API_BASE = '/api';

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
  const body: RunStageRequest = { stage_id: stageId };
  return request<StageRunResult>(`/features/${featureId}/run-stage`, {
    method: 'POST',
    body: JSON.stringify(body),
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