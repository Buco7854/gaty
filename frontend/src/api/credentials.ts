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

function authHeader(bearerToken: string) {
  return { headers: { Authorization: `Bearer ${bearerToken}` } }
}

/** Member self-service: calls /api/auth/local/me/* with the member's own JWT */
export const memberCredApi = {
  listTokens: (bearerToken: string): Promise<MemberCredential[]> =>
    api
      .get<MemberCredential[]>('/auth/local/me/credentials', authHeader(bearerToken))
      .then((r) => (Array.isArray(r.data) ? r.data : []))
      .then((data) => data.filter((c) => c.type === 'API_TOKEN')),

  createToken: (bearerToken: string, label: string): Promise<CreatedToken> =>
    api
      .post<CreatedToken>('/auth/local/me/api-tokens', { label }, authHeader(bearerToken))
      .then((r) => r.data),

  deleteToken: (bearerToken: string, credId: string): Promise<void> =>
    api.delete(`/auth/local/me/credentials/${credId}`, authHeader(bearerToken)).then(() => {}),
}
