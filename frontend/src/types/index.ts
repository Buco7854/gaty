export type WorkspaceRole = 'OWNER' | 'ADMIN' | 'MEMBER'
export type GateStatus = 'online' | 'offline' | 'unknown' | 'open' | 'closed' | string
export type GateIntegrationType = 'MQTT' | 'POLLING' | 'WEBHOOK'
export type CredentialType = 'PASSWORD' | 'SSO_IDENTITY' | 'API_TOKEN'

export interface User {
  id: string
  email: string
  created_at: string
}

export interface Workspace {
  id: string
  name: string
  owner_id: string
  sso_settings: Record<string, unknown>
  member_auth_config: Record<string, unknown>
  created_at: string
}

export interface WorkspaceWithRole extends Workspace {
  role: WorkspaceRole
}

/** Maps raw device status values (stringified) to application status strings. */
export type StatusValueMap = Record<string, string>

/** Defines how to extract the gate status from an incoming payload field. */
export interface StatusFieldMapping {
  /** Dot-notated path to the status value in the payload (e.g. "state", "data.status"). */
  field: string
  /** Maps raw device values to app status strings. If empty, the raw string is used as-is. */
  values?: StatusValueMap
}

/** Configures how to parse an incoming status payload. */
export interface PayloadMapping {
  /** Payload format — only "json" is currently supported. */
  format?: 'json'
  status: StatusFieldMapping
}

export interface ActionConfig {
  type: 'MQTT_GATIE' | 'MQTT_CUSTOM' | 'HTTP' | 'HTTP_INBOUND' | 'HTTP_WEBHOOK' | 'NONE'
  config?: Record<string, unknown>
}

/** Maps a raw status-payload key to a display label. */
export interface MetaField {
  /** Dot-notated key in the gate's status payload meta object. */
  key: string
  /** Human-readable label shown in the UI. */
  label: string
  /** Optional unit suffix (e.g. "dB", "%"). */
  unit?: string
}

/** Condition evaluated against incoming metadata to override the gate status. */
export interface StatusRule {
  /** Dot-notated key in the status payload's meta object. */
  key: string
  /** Comparison operator: "eq" | "ne" | "gt" | "gte" | "lt" | "lte" */
  op: string
  /** Threshold value as a string (numeric comparisons convert to float64). */
  value: string
  /** Gate status to set when this rule matches. */
  set_status: string
}

/** Automatic status transition after a timeout. */
export interface StatusTransition {
  from: string
  to: string
  after_seconds: number
  persist_on_change?: boolean
}

export interface Gate {
  id: string
  workspace_id: string
  name: string
  integration_type: GateIntegrationType
  integration_config: Record<string, unknown>
  open_config?: ActionConfig | null
  close_config?: ActionConfig | null
  status_config?: ActionConfig | null
  status: GateStatus
  last_seen_at?: string
  created_at: string
  /** Last received metadata from the gate (sensor data, signal info, etc.) */
  status_metadata?: Record<string, unknown>
  /** Display mapping: which metadata keys to show and how to label them. */
  meta_config?: MetaField[]
  /** Rules evaluated against metadata to auto-override the reported status. */
  status_rules?: StatusRule[]
  /** User-defined statuses in addition to the defaults (open, closed, unavailable). */
  custom_statuses?: string[]
  /** Per-gate inactivity threshold in seconds. null = use global default (30s). */
  ttl_seconds?: number | null
  /** Automatic status transitions after a timeout. */
  status_transitions?: StatusTransition[]
  /** Gate authentication token — only populated on create and rotate-token responses. */
  gate_token?: string
}

export interface WorkspaceMembership {
  id: string
  workspace_id: string
  user_id?: string
  user_email?: string
  local_username?: string
  display_name?: string
  role: WorkspaceRole
  auth_config: Record<string, unknown>
  invited_by?: string
  created_at: string
}

export interface ScheduleRule {
  type: 'time_range' | 'weekdays_range' | 'date_range' | 'day_of_month_range' | 'month_range'
  // time_range
  days?: number[]       // 0=Sun…6=Sat
  start_time?: string   // HH:MM
  end_time?: string     // HH:MM
  // weekdays_range
  start_day?: number    // 0..6
  end_day?: number      // 0..6
  // date_range
  start_date?: string   // YYYY-MM-DD
  end_date?: string     // YYYY-MM-DD
  // day_of_month_range
  start_dom?: number    // 1..31
  end_dom?: number      // 1..31
  // month_range
  start_month?: number  // 1..12
  end_month?: number    // 1..12
}

/** A node in a boolean expression tree for schedule conditions. */
export interface ExprNode {
  op: 'and' | 'or' | 'not' | 'rule'
  children?: ExprNode[]  // for op = "and" | "or" | "not"
  rule?: ScheduleRule    // for op = "rule"
}

export interface AccessSchedule {
  id: string
  workspace_id: string
  /** Present only for member personal schedules; absent for workspace-level schedules. */
  membership_id?: string
  name: string
  description?: string
  expr: ExprNode | null  // null = always allowed
  created_at: string
}

export interface GatePin {
  id: string
  gate_id: string
  label: string
  schedule_id?: string
  metadata: Record<string, unknown>
  created_at: string
}

export interface CustomDomain {
  id: string
  gate_id: string
  domain: string
  dns_challenge_token: string
  verified_at?: string
  created_at: string
}

export interface MembershipPolicy {
  membership_id: string
  gate_id: string
  permission_code: string
}

export interface Credential {
  id: string
  type: CredentialType
  label?: string
  expires_at?: string
  metadata?: Record<string, unknown>
  created_at: string
}

/** Response from global login/register — tokens in cookies, metadata in body. */
export interface GlobalAuthResponse {
  type: 'global'
  user: User
}

/** Response from local login — tokens in cookies, metadata in body. */
export interface LocalAuthResponse {
  type: 'local'
  membership_id: string
  workspace_id: string
  role: WorkspaceRole
  display_name?: string
}

/** Response from refresh — tokens in cookies, type-dependent metadata in body. */
export interface RefreshResponse {
  type: 'global' | 'local' | 'pin_session'
  user?: User
  membership_id?: string
  workspace_id?: string
  role?: WorkspaceRole
  display_name?: string
  gate_id?: string
  permissions?: string[]
}

export interface DomainResolveResult {
  gate_id: string
  gate_name: string
  workspace_id: string
  workspace_name: string
  has_open_action: boolean
  has_close_action: boolean
  status: GateStatus
  meta_config?: MetaField[]
  status_metadata?: Record<string, unknown>
}
