// TypeScript types matching API responses from the Dev Team backend

export interface FeatureSummary {
  id: string;
  title: string;
  status: string;
  priority: number;
  current_phase: string;
  updated_at: string;
  gate_result: GateResult | null;
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
  created_at: string;
  updated_at: string;
  phase_states: Record<string, PhaseState>;
  dependencies: string[];
  repos: RepoRef[];
  is_processing: boolean;
  processing_mode: string;
}

export interface PhaseState {
  phase: string;
  status: string;
  started_at: string | null;
  completed_at: string | null;
  artifacts: Artifact[];
  gate_result: GateResult | null;
}

export interface Artifact {
  type: string;
  path: string;
  generated_by: string;
  generated_at: string;
}

export interface GateResult {
  phase: string;
  passed: boolean;
  checks: CheckResult[];
}

export interface CheckResult {
  name: string;
  passed: boolean;
  message: string;
}

export interface RepoRef {
  name: string;
  url: string;
  branch: string;
}

// Request DTOs
export interface CreateFeatureRequest {
  type: 'loose_idea' | 'external_spec';
  title: string;
  description: string;
  priority: number;
  file_content?: string; // base64-encoded for external_spec
  start_immediately?: boolean;
}

export interface RecirculateRequest {
  target_phase: string;
}

export interface ErrorResponse {
  error: string;
  details?: string;
}

// SSE Event Types
export type SSEEventType =
  | 'phase_change'
  | 'gate_result'
  | 'agent_dispatch'
  | 'agent_complete'
  | 'agent_output'
  | 'processing_complete'
  | 'phase_complete'
  | 'error'
  | 'waiting_for_feedback'
  | 'questions_answered'
  | 'questions_assumed'
  | 'question_answered';

export interface PhaseChangeEvent {
  feature_id: string;
  phase: string;
  status: string;
  timestamp: string;
}

export interface GateResultEvent {
  feature_id: string;
  phase: string;
  passed: boolean;
  checks: CheckResult[];
}

export interface AgentDispatchEvent {
  feature_id: string;
  phase: string;
  role: string;
  status: string;
  timestamp: string;
}

export interface AgentCompleteEvent {
  feature_id: string;
  phase: string;
  role: string;
  status: string;
  duration_ms: number;
}

export interface ProcessingCompleteEvent {
  feature_id: string;
  status: string;
  timestamp: string;
}

export interface ErrorEvent {
  feature_id: string;
  message: string;
  timestamp: string;
}

// Artifact type mapping for API paths
export const ARTIFACT_TYPE_MAP: Record<string, string> = {
  input: 'input_md',
  spec: 'spec_md',
  acceptance: 'acceptance_md',
  repos: 'repos_yaml',
  plan: 'plan_md',
  tasks: 'tasks_md',
  review_report: 'review_report',
  test_report: 'test_report',
  docs: 'docs',
};

export const ARTIFACT_DISPLAY_NAMES: Record<string, string> = {
  input_md: 'Input',
  spec_md: 'Specification',
  acceptance_md: 'Acceptance Criteria',
  repos_yaml: 'Repositories',
  plan_md: 'Plan',
  tasks_md: 'Tasks',
  review_report: 'Review Report',
  test_report: 'Test Report',
  docs: 'Documentation',
};

// Phase display helpers
export const PHASES = ['inception', 'planning', 'construction', 'review', 'testing', 'delivery'] as const;
export type PhaseName = typeof PHASES[number];

export const PHASE_LABELS: Record<PhaseName, string> = {
  inception: 'Inception',
  planning: 'Planning',
  construction: 'Construction',
  review: 'Review',
  testing: 'Testing',
  delivery: 'Delivery',
};

// User-friendly action verb for each phase — what the user is actually doing
export const PHASE_ACTIONS: Record<PhaseName, string> = {
  inception: 'Start Inception',
  planning: 'Start Planning',
  construction: 'Start Construction',
  review: 'Start Review',
  testing: 'Start Testing',
  delivery: 'Start Delivery',
};

// User-friendly description of each phase — what happens when you run it
export const PHASE_DESCRIPTIONS: Record<PhaseName, string> = {
  inception: 'Turn your idea into a clear specification with requirements and acceptance criteria',
  planning: 'Design the technical approach — architecture, tasks, and test strategy',
  construction: 'Write the code according to the plan',
  review: 'Adversarial review against acceptance criteria to catch gaps',
  testing: 'Verify everything works — smoke tests, integration tests, unit tests',
  delivery: 'Ship it — documentation, PR, and deployment verification',
};

// What each phase produces — shown to the user so they know what to expect
export const PHASE_OUTPUTS: Record<PhaseName, string> = {
  inception: 'Specification, acceptance criteria, and repository list',
  planning: 'Technical plan, task breakdown, and test strategy',
  construction: 'Working code implementation',
  review: 'Review report identifying any gaps or issues',
  testing: 'Test report verifying all acceptance criteria pass',
  delivery: 'Documentation, changelog, and deployment verification',
};

export const STATUS_LABELS: Record<string, string> = {
  draft: 'Draft',
  in_progress: 'In Progress',
  gate_blocked: 'Gate Blocked',
  passed: 'Passed',
  failed: 'Failed',
  done: 'Done',
  recirculated: 'Recirculated',
  cancelled: 'Cancelled',
  waiting_for_feedback: 'Waiting for Human',
};

export const PRIORITY_LABELS: Record<number, string> = {
  1: 'P1 - Critical',
  2: 'P2 - Medium',
  3: 'P3 - Low',
};

// Question types
export interface Question {
  id: string;
  feature_id: string;
  phase: 'inception' | 'planning';
  role: 'pm' | 'architect';
  question: string;
  type: 'clarification' | 'decision' | 'priority';
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