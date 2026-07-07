// Admin UI DTO types — mirror the Go handler response shapes
// (internal/api/settings_handlers.go).

export interface RepoRow {
  name: string;
  url: string;
  branch: string;
  description: string;
  primary: boolean;
  created_at: string;
  updated_at: string;
  reference_count: number;
}

export interface RepoInput {
  name: string;
  url: string;
  branch?: string;
  description?: string;
  primary: boolean;
}

export interface DefaultsRow {
  scope?: string;
  depth?: string;
  test_strategy?: string;
  execution_mode?: string;
  repo?: string;
}

export interface DefaultsResponse {
  global: DefaultsRow;
  per_repo: DefaultsRow[];
}

export interface AuditEventRow {
  id: number;
  feature_id: string;
  event_type: string;
  stage_id?: string;
  phase?: string;
  details?: string;
  actor?: string;
  created_at: string;
}

export interface AuditListResponse {
  events: AuditEventRow[];
  total: number;
  page: number;
  page_size: number;
}