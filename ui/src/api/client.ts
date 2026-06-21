import type {
  FeatureListResponse,
  FeatureDetail,
  CreateFeatureRequest,
  RecirculateRequest,
  GateResult,
  ErrorResponse,
  Question,
  CreateQuestionRequest,
  AnswerQuestionRequest,
} from '../types';

const API_BASE = '/api';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: {
      'Content-Type': 'application/json',
    },
    ...options,
  });

  if (!response.ok) {
    const errorData: ErrorResponse = await response.json().catch(() => ({
      error: 'unknown_error',
      details: response.statusText,
    }));
    throw new ApiError(response.status, errorData.error, errorData.details);
  }

  // 204 No Content
  if (response.status === 204) {
    return undefined as T;
  }

  return response.json();
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    public details?: string
  ) {
    super(details || code);
    this.name = 'ApiError';
  }
}

// Feature list
export async function listFeatures(): Promise<FeatureListResponse> {
  return request<FeatureListResponse>('/features');
}

// Feature detail
export async function getFeature(id: string): Promise<FeatureDetail> {
  return request<FeatureDetail>(`/features/${id}`);
}

// Create feature
export async function createFeature(req: CreateFeatureRequest): Promise<FeatureDetail> {
  return request<FeatureDetail>('/features', {
    method: 'POST',
    body: JSON.stringify(req),
  });
}

// Run phase
export async function runPhase(id: string): Promise<FeatureDetail> {
  return request<FeatureDetail>(`/features/${id}/run`, {
    method: 'POST',
  });
}

// Advance feature
export async function advanceFeature(id: string): Promise<FeatureDetail> {
  return request<FeatureDetail>(`/features/${id}/advance`, {
    method: 'POST',
  });
}

// Recirculate feature
export async function recirculateFeature(id: string, targetPhase: string): Promise<FeatureDetail> {
  const body: RecirculateRequest = { target_phase: targetPhase };
  return request<FeatureDetail>(`/features/${id}/recirculate`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

// Cancel feature
export async function cancelFeature(id: string): Promise<FeatureDetail> {
  return request<FeatureDetail>(`/features/${id}/cancel`, {
    method: 'POST',
  });
}

// Process feature (autonomous pipeline)
export async function processFeature(id: string): Promise<FeatureDetail> {
  return request<FeatureDetail>(`/features/${id}/process`, {
    method: 'POST',
  });
}

// Evaluate gate
export async function evaluateGate(id: string): Promise<GateResult> {
  return request<GateResult>(`/features/${id}/gate`);
}

// Get artifact content
export async function getArtifact(id: string, type: string): Promise<string> {
  const response = await fetch(`${API_BASE}/features/${id}/artifacts/${type}`);
  if (!response.ok) {
    if (response.status === 404) {
      return '';
    }
    const errorData: ErrorResponse = await response.json().catch(() => ({
      error: 'unknown_error',
      details: response.statusText,
    }));
    throw new ApiError(response.status, errorData.error, errorData.details);
  }
  return response.text();
}

// Question API functions

// List questions for a feature
export async function listQuestions(featureId: string): Promise<Question[]> {
  return request<Question[]>(`/features/${featureId}/questions`);
}

// Create a question for a feature
export async function createQuestion(featureId: string, req: CreateQuestionRequest): Promise<Question> {
  return request<Question>(`/features/${featureId}/questions`, {
    method: 'POST',
    body: JSON.stringify(req),
  });
}

// Answer a question
export async function answerQuestion(featureId: string, questionId: string, answer: string): Promise<Question> {
  const body: AnswerQuestionRequest = { answer };
  return request<Question>(`/features/${featureId}/questions/${questionId}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  });
}

// List pending questions for a feature
export async function listPendingQuestions(featureId: string): Promise<Question[]> {
  return request<Question[]>(`/features/${featureId}/questions/pending`);
}