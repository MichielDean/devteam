// AIDLC v2 TypeScript types matching API responses from the Dev Team backend

// ─── Scopes ───
export const SCOPES = ['enterprise', 'feature', 'mvp', 'poc', 'bugfix', 'refactor', 'infra', 'security-patch', 'workshop'] as const;
export type ScopeName = typeof SCOPES[number];

export const SCOPE_LABELS: Record<string, string> = {
  enterprise: 'Enterprise',
  feature: 'Feature',
  mvp: 'MVP',
  poc: 'Proof of Concept',
  bugfix: 'Bug Fix',
  refactor: 'Refactor',
  infra: 'Infrastructure',
  'security-patch': 'Security Patch',
  workshop: 'Workshop',
};

export const SCOPE_DESCRIPTIONS: Record<string, string> = {
  enterprise: 'Regulated enterprise feature, full audit trail (32 stages)',
  feature: 'Default for new features (32 stages)',
  mvp: 'Greenfield, skip late operations (22 stages)',
  poc: 'Prove feasibility fast (8 stages)',
  bugfix: 'Fix a specific bug (7 stages)',
  refactor: 'Clean up existing code (8 stages)',
  infra: 'Infrastructure change (13 stages)',
  'security-patch': 'CVE response (9 stages)',
  workshop: 'AI-DLC workshop or training (25 stages)',
};

// ─── Phases ───
export const PHASES = ['initialization', 'ideation', 'inception', 'construction', 'operation'] as const;
export type PhaseName = typeof PHASES[number];

export const PHASE_LABELS: Record<string, string> = {
  initialization: 'Initialization',
  ideation: 'Ideation',
  inception: 'Inception',
  construction: 'Construction',
  operation: 'Operation',
};

// ─── Depth ───
export const DEPTHS = ['minimal', 'standard', 'comprehensive'] as const;
export type DepthName = typeof DEPTHS[number];

export const DEPTH_LABELS: Record<string, string> = {
  minimal: 'Minimal',
  standard: 'Standard',
  comprehensive: 'Comprehensive',
};

// ─── Test Strategy ───
export const TEST_STRATEGIES = ['minimal', 'standard', 'comprehensive'] as const;
export type TestStrategyName = typeof TEST_STRATEGIES[number];

export const TEST_STRATEGY_LABELS: Record<string, string> = {
  minimal: 'Minimal',
  standard: 'Standard',
  comprehensive: 'Comprehensive',
};

// ─── Stage Status (6-state checkbox) ───
export const STAGE_STATUSES = ['not_started', 'in_progress', 'awaiting_approval', 'revising', 'completed', 'skipped'] as const;
export type StageStatus = typeof STAGE_STATUSES[number];

export const STAGE_STATUS_LABELS: Record<string, string> = {
  not_started: 'Not Started',
  in_progress: 'In Progress',
  awaiting_approval: 'Awaiting Approval',
  revising: 'Revising',
  completed: 'Completed',
  skipped: 'Skipped',
};

export const STAGE_CHECKBOX: Record<string, string> = {
  not_started: '[ ]',
  in_progress: '[-]',
  awaiting_approval: '[?]',
  revising: '[R]',
  completed: '[x]',
  skipped: '[S]',
};

// ─── Autonomy Mode ───
export const AUTONOMY_MODES = ['gated', 'autonomous'] as const;
export type AutonomyMode = typeof AUTONOMY_MODES[number];

// ─── Feature ───
export interface FeatureSummary {
  id: string;
  title: string;
  status: string;
  priority: number;
  current_phase: string;
  scope?: string;
  current_stage?: string;
  updated_at: string;
  pending_questions_count: number;
}

export interface FeatureListResponse {
  features: FeatureSummary[];
  total_count: number;
}

export interface FeatureDetail {
  id: string;
  title: string;
  status: string;
  priority: number;
  intake_path: string;
  current_phase: string;
  scope?: string;
  depth?: string;
  test_strategy?: string;
  autonomy_mode?: string;
  current_stage?: string;
  created_at: string;
  updated_at: string;
  dependencies: string[];
  repos: RepoRef[];
  is_processing: boolean;
  processing_mode?: string;
}

export interface RepoRef {
  name: string;
  url: string;
  branch: string;
}

// ─── Stages ───
export interface StageDefinition {
  id: string;
  phase: string;
  name: string;
  description?: string;
  lead_agent: string;
  supporting_agents: string[];
  key_artifacts: string[];
  condition: string;
  scopes: string[];
  reviewer: string;
  sort_order: number;
}

export interface FeatureStage {
  id: number;
  feature_id: string;
  stage_id: string;
  status: string;
  revision_count: number;
  started_at?: string;
  completed_at?: string;
  name?: string;
  description?: string;
  phase?: string;
  lead_agent?: string;
  key_artifacts?: string[];
  reviewer?: string;
}

// ─── Audit Events ───
export interface AuditEvent {
  id: number;
  feature_id: string;
  event_type: string;
  stage_id?: string;
  phase?: string;
  details?: string;
  created_at: string;
}

// ─── Bolts ───
export interface Bolt {
  id: number;
  feature_id: string;
  bolt_number: number;
  unit_ids: string[];
  status: string;
  is_walking_skeleton: boolean;
  created_at: string;
}

// ─── Team Knowledge ───
export interface TeamKnowledge {
  id: number;
  agent_name: string;
  topic: string;
  content: string;
  created_at: string;
  updated_at: string;
}

// ─── Rules (Learning Loop) ───
export interface Rule {
  id: number;
  feature_id: string;
  agent_name: string;
  stage_id: string;
  rule_text: string;
  source_rejection: string;
  created_at: string;
}

// ─── Stage Run Result ───
export interface StageRunResult {
  stage_id: string;
  phase: string;
  stage_name: string;
  smoke_failures: string[];
  outcome_source: string;
  gate?: {
    feature_id: string;
    stage_id: string;
    state: string;
    revision_count: number;
    revision_notes: string;
  };
  reviewer_result?: {
    reviewer: string;
    verdict: string;
    notes: string;
    iterations: number;
  };
  duration: number;
}

// ─── Request DTOs ───
export interface CreateFeatureRequest {
  type: 'loose_idea' | 'external_spec';
  title: string;
  description: string;
  priority: number;
  file_content?: string;
  start_immediately?: boolean;
  scope?: string;
  depth?: string;
  test_strategy?: string;
  repos?: RepoRef[];
}

export interface RunStageRequest {
  stage_id: string;
}

export interface RejectStageRequest {
  notes: string;
}

export interface JumpRequest {
  stage_id?: string;
  phase?: string;
}

export interface SetScopeRequest {
  scope: string;
}

export interface SetDepthRequest {
  depth: string;
}

export interface SetTestStrategyRequest {
  test_strategy: string;
}

export interface SetLadderRequest {
  mode: string;
}

export interface SaveKnowledgeRequest {
  topic: string;
  content: string;
}

export interface ErrorResponse {
  error: string;
  details?: string;
}

// ─── SSE Event Types ───
export type SSEEventType =
  | 'stage_started'
  | 'stage_awaiting_approval'
  | 'stage_revising'
  | 'stage_completed'
  | 'gate_approved'
  | 'gate_rejected'
  | 'gate_result'
  | 'agent_dispatch'
  | 'agent_complete'
  | 'agent_output'
  | 'processing_complete'
  | 'error'
  | 'interrupted'
  | 'waiting_for_feedback'
  | 'question_answered'
  | 'session_state_change'
  | 'state_change';

export interface SSEMessage {
  type: SSEEventType;
  data: string;
  timestamp: string;
}

// ─── Tmux Sessions ───
export interface TmuxSession {
  id: number;
  feature_id: string;
  phase: string;
  bolt_number: number;
  stage_id: string;
  session_name: string;
  state: string;
  context_dir: string;
  last_agent: string;
  last_output_at: string | null;
  created_at: string;
  updated_at: string;
  is_alive: boolean;
}

// ─── Stage Detail ───
export interface StageDefinitionDetail {
  id: string;
  phase: string;
  name: string;
  description?: string;
  lead_agent: string;
  supporting_agents: string[];
  key_artifacts: string[];
  condition: string;
  scopes: string[];
  reviewer: string;
  sort_order: number;
}

// ─── Artifact ───
export interface Artifact {
  type: string;
  path: string;
  generated_by: string;
  generated_at: string;
}

// ─── Questions ───
export interface Question {
  id: string;
  feature_id: string;
  phase: string;
  stage_id: string;
  role: string;
  question: string;
  type: string;
  options: string[];
  answer: string | null;
  assumption: string | null;
  status: 'pending' | 'answered' | 'assumed';
  created_at: string;
  answered_at: string | null;
}

export interface CreateQuestionRequest {
  phase: string;
  role: string;
  question: string;
  type: string;
  options?: string[];
}

export interface AnswerQuestionRequest {
  answer: string;
}

// ─── Status/Priority Labels ───
export const STATUS_LABELS: Record<string, string> = {
  draft: 'Draft',
  in_progress: 'In Progress',
  gate_blocked: 'Gate Blocked',
  passed: 'Passed',
  failed: 'Failed',
  done: 'Done',
  cancelled: 'Cancelled',
  waiting_for_feedback: 'Waiting for Human',
};

export const PRIORITY_LABELS: Record<number, string> = {
  1: 'P1 - Critical',
  2: 'P2 - Medium',
  3: 'P3 - Low',
};

// ─── Agents ───
export const AGENTS = [
  'product', 'design', 'delivery', 'architect', 'platform',
  'devsecops', 'developer', 'quality', 'pipeline-deploy', 'operations',
] as const;

export const REVIEWERS = ['product-lead', 'architecture-reviewer'] as const;

export const AGENT_LABELS: Record<string, string> = {
  product: 'Product',
  design: 'Design',
  delivery: 'Delivery',
  architect: 'Architect',
  platform: 'Platform',
  devsecops: 'DevSecOps',
  developer: 'Developer',
  quality: 'Quality',
  'pipeline-deploy': 'Pipeline & Deploy',
  operations: 'Operations',
  'product-lead': 'Product Lead (Reviewer)',
  'architecture-reviewer': 'Architecture Reviewer',
};