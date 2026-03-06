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

function authHeader(bearerToken: string) {
  return { headers: { Authorization: `Bearer ${bearerToken}` } }
}

/** Workspace member self-service: calls /api/workspaces/{ws_id}/members/me/* — JWT auto-attached by axios interceptor */
export const workspaceCredApi = {
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

/** Local member self-service: calls /api/auth/local/me/* with the member's own JWT */
export const memberCredApi = {
  listTokens: (bearerToken: string): Promise<MemberCredential[]> =>
    api
      .get<MemberCredential[]>('/auth/local/me/credentials', authHeader(bearerToken))
      .then((r) => (Array.isArray(r.data) ? r.data : []))
      .then((data) => data.filter((c) => c.type === 'API_TOKEN')),

  createToken: (bearerToken: string, label: string, expiresAt?: string): Promise<CreatedToken> =>
    api
      .post<CreatedToken>(
        '/auth/local/me/api-tokens',
        { label, ...(expiresAt ? { expires_at: expiresAt + 'T23:59:59Z' } : {}) },
        authHeader(bearerToken),
      )
      .then((r) => r.data),

  deleteToken: (bearerToken: string, credId: string): Promise<void> =>
    api.delete(`/auth/local/me/credentials/${credId}`, authHeader(bearerToken)).then(() => {}),
}
