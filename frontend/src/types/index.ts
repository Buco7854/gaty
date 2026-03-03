export type WorkspaceRole = 'OWNER' | 'ADMIN' | 'MEMBER'
export type GateStatus = 'online' | 'offline' | 'unknown'
export type GateIntegrationType = 'MQTT' | 'POLLING' | 'WEBHOOK'
export type CredentialType = 'PASSWORD' | 'SSO_IDENTITY' | 'API_TOKEN'

export interface User {
  id: string
  email: string
  created_at: string
}

export interface Workspace {
  id: string
  slug: string
  name: string
  owner_id: string
  sso_settings: Record<string, unknown>
  member_auth_config: Record<string, unknown>
  created_at: string
}

export interface WorkspaceWithRole extends Workspace {
  role: WorkspaceRole
}

export interface Gate {
  id: string
  workspace_id: string
  name: string
  integration_type: GateIntegrationType
  integration_config: Record<string, unknown>
  status: GateStatus
  last_seen_at?: string
  created_at: string
}

export interface WorkspaceMembership {
  id: string
  workspace_id: string
  user_id?: string
  local_username?: string
  display_name?: string
  role: WorkspaceRole
  auth_config: Record<string, unknown>
  invited_by?: string
  created_at: string
}

export interface GatePin {
  id: string
  gate_id: string
  label?: string
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

export interface AuthResponse {
  access_token: string
  refresh_token: string
  user: User
}

export interface RefreshResponse {
  access_token: string
  refresh_token: string
}

export interface DomainResolveResult {
  gate_id: string
  gate_name: string
  workspace_id: string
  workspace_slug: string
  workspace_name: string
}
