import { api } from '@/lib/api'

export interface MemberCredential {
  id: string
  type: 'API_TOKEN' | 'SSO_IDENTITY' | 'PASSWORD'
  label?: string
  expires_at?: string
  metadata?: Record<string, unknown>
  created_at: string
}

export interface CreatedToken extends MemberCredential {
  token: string
}

interface PolicyInput {
  gate_id: string
  permission_code: string
}

export interface MyEffectiveAuthConfig {
  api_token: boolean
}

/** Workspace member self-service: calls /api/workspaces/{ws_id}/members/me/* — cookie auto-attached */
export const workspaceCredApi = {
  getMyAuthConfig: (wsId: string): Promise<MyEffectiveAuthConfig> =>
    api
      .get<MyEffectiveAuthConfig>(`/workspaces/${wsId}/members/me/auth-config`)
      .then((r) => r.data),

  listTokens: (wsId: string): Promise<MemberCredential[]> =>
    api
      .get<MemberCredential[]>(`/workspaces/${wsId}/members/me/credentials`)
      .then((r) => (Array.isArray(r.data) ? r.data : []))
      .then((data) => data.filter((c) => c.type === 'API_TOKEN')),

  createToken: (
    wsId: string,
    label: string,
    expiresAt?: string,
    policies?: PolicyInput[],
    scheduleId?: string,
  ): Promise<CreatedToken> =>
    api
      .post<CreatedToken>(`/workspaces/${wsId}/members/me/api-tokens`, {
        label,
        ...(expiresAt ? { expires_at: expiresAt + 'T23:59:59Z' } : {}),
        ...(policies?.length ? { policies } : {}),
        ...(scheduleId ? { schedule_id: scheduleId } : {}),
      })
      .then((r) => r.data),

  deleteToken: (wsId: string, credId: string): Promise<void> =>
    api.delete(`/workspaces/${wsId}/members/me/credentials/${credId}`).then(() => {}),
}

/** Local member self-service: calls /api/auth/local/me/* — cookie auto-attached */
export const memberCredApi = {
  listTokens: (): Promise<MemberCredential[]> =>
    api
      .get<MemberCredential[]>('/auth/local/me/credentials')
      .then((r) => (Array.isArray(r.data) ? r.data : []))
      .then((data) => data.filter((c) => c.type === 'API_TOKEN')),

  createToken: (
    label: string,
    expiresAt?: string,
    policies?: PolicyInput[],
    scheduleId?: string,
  ): Promise<CreatedToken> =>
    api
      .post<CreatedToken>(
        '/auth/local/me/api-tokens',
        {
          label,
          ...(expiresAt ? { expires_at: expiresAt + 'T23:59:59Z' } : {}),
          ...(policies?.length ? { policies } : {}),
          ...(scheduleId ? { schedule_id: scheduleId } : {}),
        },
      )
      .then((r) => r.data),

  deleteToken: (credId: string): Promise<void> =>
    api.delete(`/auth/local/me/credentials/${credId}`).then(() => {}),
}
